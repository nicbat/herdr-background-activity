package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nicbat/herdr-background-activity/internal/activity"
	"github.com/nicbat/herdr-background-activity/internal/app"
	"github.com/nicbat/herdr-background-activity/internal/buildinfo"
	"github.com/nicbat/herdr-background-activity/internal/herdr"
	"github.com/nicbat/herdr-background-activity/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	command := "start"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command == "--help" || command == "help" {
		fmt.Println("herdr-background-activity [start|stop|restart|status|inspect|serve|version]\nRun inside Herdr. inspect is read-only; serve runs in the foreground.")
		return nil
	}
	if command == "version" || command == "--version" {
		fmt.Println(buildinfo.Version)
		return nil
	}
	c, err := service.FromEnv()
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	switch command {
	case "start", "stop", "restart":
		if err := c.Manage(ctx, command); err != nil {
			return err
		}
		fmt.Println("Background Activity: " + command + " complete")
	case "serve":
		return c.Serve(ctx)
	case "status":
		s, err := c.Status()
		if err != nil {
			return fmt.Errorf("reporter unavailable: %w", err)
		}
		return json.NewEncoder(os.Stdout).Encode(s)
	case "inspect":
		ctx, done := context.WithTimeout(ctx, 15*time.Second)
		defer done()
		api := &herdr.Client{Socket: c.Socket}
		var snapshot herdr.SnapshotResult
		if err := api.Call(ctx, "session.snapshot", nil, &snapshot); err != nil {
			return err
		}
		type inspection struct {
			PaneID      string               `json:"pane_id"`
			Agent       string               `json:"agent"`
			Status      string               `json:"agent_status"`
			Observation activity.Observation `json:"observation"`
		}
		out := []inspection{}
		for _, r := range app.Observe(ctx, api, snapshot.Snapshot.Panes) {
			if r.Pane.Agent != "" {
				out = append(out, inspection{r.Pane.ID, r.Pane.Agent, r.Pane.Status, r.Observation})
			}
		}
		return json.NewEncoder(os.Stdout).Encode(out)
	default:
		return fmt.Errorf("unknown command %q (see --help)", command)
	}
	return nil
}
