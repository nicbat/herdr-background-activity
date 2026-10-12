// Package service manages one reporter per Herdr socket, without a supervisor.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/nicbat/herdr-background-activity/internal/app"
	"github.com/nicbat/herdr-background-activity/internal/buildinfo"
	"github.com/nicbat/herdr-background-activity/internal/herdr"
)

type Config struct{ Socket, StateDir string }

func FromEnv() (Config, error) {
	c := Config{Socket: os.Getenv("HERDR_SOCKET_PATH"), StateDir: os.Getenv("HERDR_PLUGIN_STATE_DIR")}
	if os.Getenv("HERDR_ENV") != "1" || c.Socket == "" {
		return c, errors.New("run inside Herdr (HERDR_ENV=1 and HERDR_SOCKET_PATH required)")
	}
	if c.StateDir == "" {
		state := os.Getenv("XDG_STATE_HOME")
		if state == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return c, err
			}
			state = filepath.Join(home, ".local", "state")
		}
		c.StateDir = filepath.Join(state, "herdr", "plugins", herdr.Source)
	}
	return c, nil
}

func (c Config) dir() string {
	return filepath.Join(c.StateDir, fmt.Sprintf("%x", sha256.Sum256([]byte(c.Socket)))[:12])
}
func (c Config) control() string { return filepath.Join(c.dir(), "ctl") }

type Status struct {
	Running     bool      `json:"running"`
	Version     string    `json:"version"`
	Build       string    `json:"build"`
	PID         int       `json:"pid,omitempty"`
	Server      string    `json:"server,omitempty"`
	LastSuccess time.Time `json:"last_success"`
	Healthy     bool      `json:"healthy"`
}

func serverID(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return "", errors.New("Herdr path is not a socket")
	}
	stat := info.Sys().(*syscall.Stat_t)
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), nil
}

func (c Config) request(command string) (Status, error) {
	var status Status
	conn, err := net.DialTimeout("unix", c.control(), 500*time.Millisecond)
	if err != nil {
		return status, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if err := json.NewEncoder(conn).Encode(command); err != nil {
		return status, err
	}
	err = json.NewDecoder(io.LimitReader(conn, 4096)).Decode(&status)
	return status, err
}

func (c Config) Status() (Status, error) { return c.request("status") }

// File locks survive crashes correctly. Only our private control socket can stop us.
func lock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func waitLock(ctx context.Context, path string) (*os.File, error) {
	for {
		f, err := lock(path)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// Manage serializes start/stop/restart, including the old reporter's cleanup.
func (c Config) Manage(ctx context.Context, command string) error {
	if command != "start" && command != "stop" && command != "restart" {
		return errors.New("invalid service command")
	}
	if err := os.MkdirAll(c.dir(), 0700); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	manager, err := waitLock(ctx, filepath.Join(c.dir(), "manager.lock"))
	if err != nil {
		return err
	}
	defer manager.Close()
	if command == "stop" || command == "restart" {
		if err := c.stop(ctx); err != nil {
			return err
		}
	}
	if command == "stop" {
		return nil
	}
	server, err := serverID(c.Socket)
	if err != nil {
		return err
	}
	build, err := buildinfo.Digest()
	if err != nil {
		return err
	}
	if status, err := c.Status(); err == nil && status.Running {
		if status.Server == server && status.Version == buildinfo.Version && status.Build == build {
			return nil
		}
		if err := c.stop(ctx); err != nil {
			return err
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(c.dir(), "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(exe, "serve")
	cmd.Env = append(os.Environ(), "HERDR_ENV=1", "HERDR_SOCKET_PATH="+c.Socket, "HERDR_PLUGIN_STATE_DIR="+c.StateDir)
	// Do not retain plugin-action pipes or its session/controlling terminal.
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		if status, err := c.Status(); err == nil && status.Running && status.Server == server && status.Version == buildinfo.Version && status.Build == build {
			return nil
		}
		select {
		case err := <-done:
			return fmt.Errorf("reporter exited (%v); see %s", err, filepath.Join(c.dir(), "daemon.log"))
		case <-ctx.Done():
			return fmt.Errorf("start: %w", ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (c Config) stop(ctx context.Context) error {
	// A startup may not yet have opened its control socket. Do not infer death
	// from that alone: wait for either a control reply or the singleton lock.
	for {
		if _, err := c.request("stop"); err == nil {
			break
		}
		f, err := lock(filepath.Join(c.dir(), "daemon.lock"))
		if err == nil {
			f.Close()
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	f, err := waitLock(ctx, filepath.Join(c.dir(), "daemon.lock"))
	if err != nil {
		return err
	}
	return f.Close()
}

func (c Config) Serve(ctx context.Context) error {
	if err := os.MkdirAll(c.dir(), 0700); err != nil {
		return err
	}
	guard, err := lock(filepath.Join(c.dir(), "daemon.lock"))
	if err != nil {
		return err
	}
	defer guard.Close()
	server, err := serverID(c.Socket)
	if err != nil {
		return err
	}
	if err := os.Remove(c.control()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	listener, err := net.Listen("unix", c.control())
	if err != nil {
		return err
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	build, err := buildinfo.Digest()
	if err != nil {
		return err
	}
	status := Status{Running: true, Version: buildinfo.Version, Build: build, PID: os.Getpid(), Server: server}
	var mu sync.Mutex
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			var command string
			if json.NewDecoder(io.LimitReader(conn, 128)).Decode(&command) == nil && (command == "status" || command == "stop") {
				mu.Lock()
				reply := status
				mu.Unlock()
				_ = json.NewEncoder(conn).Encode(reply)
				if command == "stop" {
					cancel()
				}
			}
			conn.Close()
		}
	}()
	runner := app.Runner{API: &herdr.Client{Socket: c.Socket}}
	defer func() {
		// Do not clear a new server's reporter if the old server was replaced.
		if current, err := serverID(c.Socket); err == nil && current == server {
			cleanup, done := context.WithTimeout(context.Background(), 3*time.Second)
			defer done()
			runner.Clear(cleanup)
		}
	}()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if current, err := serverID(c.Socket); err != nil || current != server {
			return nil
		}
		poll, done := context.WithTimeout(ctx, 10*time.Second)
		err := runner.Tick(poll, time.Now())
		done()
		mu.Lock()
		status.Healthy = err == nil
		if err == nil {
			status.LastSuccess = time.Now()
		}
		mu.Unlock()
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
