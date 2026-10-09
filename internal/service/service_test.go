package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Manage launches the current executable; this entry point makes the test
// executable act as the real daemon in child processes.
func TestMain(m *testing.M) {
	if os.Getenv("HBA_TEST_SERVICE") == "1" {
		c, err := FromEnv()
		if err == nil {
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
			err = c.Serve(ctx)
			cancel()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type testServer struct {
	listener net.Listener
	mu       sync.Mutex
	clears   int
}

func serveAPI(t *testing.T, socket string) *testServer {
	t.Helper()
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	s := &testServer{listener: ln}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(time.Second))
				var req struct {
					Method string
					Params struct{ Tokens map[string]*string }
				}
				if json.NewDecoder(conn).Decode(&req) != nil {
					return
				}
				var result any = map[string]any{}
				if req.Method == "session.snapshot" {
					result = map[string]any{"snapshot": map[string]any{"panes": []any{map[string]any{"pane_id": "p1", "workspace_id": "w1", "terminal_id": "t1", "agent": "codex", "agent_status": "idle"}}}}
				} else if req.Method == "pane.read" {
					result = map[string]any{"read": map[string]any{"text": "\n\n1 background terminal running · /ps to view · /stop to close\n\n\n› Prompt\nContext 42% used\n? for shortcuts"}}
				} else if req.Method == "pane.report_metadata" && len(req.Params.Tokens) == 3 && req.Params.Tokens["bg_state"] == nil {
					s.mu.Lock()
					s.clears++
					s.mu.Unlock()
				}
				_ = json.NewEncoder(conn).Encode(map[string]any{"result": result})
			}()
		}
	}()
	return s
}

func eventually(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition did not become true")
}

func TestLifecycle(t *testing.T) {
	dir, err := os.MkdirTemp("", "hba-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	t.Setenv("HBA_TEST_SERVICE", "1")
	c := Config{Socket: filepath.Join(dir, "s"), StateDir: filepath.Join(dir, "st")}
	s := serveAPI(t, c.Socket)
	ctx := context.Background()
	defer c.Manage(ctx, "stop")
	if err := c.Manage(ctx, "start"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { status, err := c.Status(); return err == nil && status.Healthy })
	first, _ := c.Status()
	if err := c.Manage(ctx, "start"); err != nil {
		t.Fatal(err)
	}
	second, _ := c.Status()
	if first.PID != second.PID {
		t.Fatal("duplicate daemon")
	}
	if sid, err := syscall.Getpgid(second.PID); err != nil || sid != second.PID {
		t.Fatalf("not detached: %d %v", sid, err)
	}
	if err := c.Manage(ctx, "restart"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { status, err := c.Status(); return err == nil && status.Healthy })
	third, _ := c.Status()
	if third.PID == first.PID {
		t.Fatal("did not restart")
	}
	s.mu.Lock()
	cleared := s.clears
	s.mu.Unlock()
	if cleared == 0 {
		t.Fatal("restart did not clear previous metadata")
	}
	// A second Herdr socket uses an independent singleton.
	c2 := Config{Socket: filepath.Join(dir, "s2"), StateDir: c.StateDir}
	serveAPI(t, c2.Socket)
	defer c2.Manage(ctx, "stop")
	if err := c2.Manage(ctx, "start"); err != nil {
		t.Fatal(err)
	}
	other, _ := c2.Status()
	if other.PID == third.PID {
		t.Fatal("sockets share daemon")
	}
	// Preserve the old inode while binding a replacement at the original path.
	if err := os.Rename(c.Socket, c.Socket+".old"); err != nil {
		t.Fatal(err)
	}
	serveAPI(t, c.Socket)
	eventually(t, func() bool { _, err := c.Status(); return err != nil })
	if err := c.Manage(ctx, "start"); err != nil {
		t.Fatal(err)
	}
	if err := c.Manage(ctx, "stop"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Status(); err == nil {
		t.Fatal("still running")
	}
	if _, err := c2.Status(); err != nil {
		t.Fatal("stopped other socket")
	}
	if err := c.Manage(ctx, "stop"); err != nil {
		t.Fatal("stop not idempotent", err)
	}
}
