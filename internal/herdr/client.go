// Package herdr contains only the read and display-metadata operations this plugin uses.
package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"
)

type API interface {
	Call(context.Context, string, any, any) error
}

type Client struct {
	Socket string
	seq    atomic.Uint64
}

func (c *Client) Call(ctx context.Context, method string, params any, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", c.Socket)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if params == nil {
		params = struct{}{}
	}
	req := struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params"`
	}{
		fmt.Sprintf("background-%d", c.seq.Add(1)), method, params,
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("send %s: %w", method, err)
	}
	// Bound responses without storing any screen text on disk.
	line, err := bufio.NewReader(io.LimitReader(conn, 4<<20)).ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("read %s: %w", method, err)
	}
	var frame struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(line, &frame); err != nil {
		return fmt.Errorf("decode %s: %w", method, err)
	}
	if frame.Error != nil {
		return fmt.Errorf("%s: %s", method, frame.Error.Code)
	}
	if out != nil {
		return json.Unmarshal(frame.Result, out)
	}
	return nil
}

type Pane struct {
	ID          string          `json:"pane_id"`
	TerminalID  string          `json:"terminal_id"`
	WorkspaceID string          `json:"workspace_id"`
	Agent       string          `json:"agent"`
	Status      string          `json:"agent_status"`
	Session     json.RawMessage `json:"agent_session"`
	Scroll      struct {
		Offset int `json:"offset_from_bottom"`
	} `json:"scroll"`
}
type Workspace struct {
	ID string `json:"workspace_id"`
}
type Snapshot struct {
	Panes      []Pane      `json:"panes"`
	Workspaces []Workspace `json:"workspaces"`
}
type SnapshotResult struct {
	Snapshot Snapshot `json:"snapshot"`
}
type ReadResult struct {
	Read struct {
		Text      string `json:"text"`
		Truncated bool   `json:"truncated"`
	} `json:"read"`
}

const Source = "herdr.background-activity"
const TTL = 15000

type Tokens map[string]*string

type Metadata struct {
	PaneID      string `json:"pane_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Source      string `json:"source"`
	Tokens      Tokens `json:"tokens"`
	TTL         int    `json:"ttl_ms"`
}

func String(s string) *string { return &s }
func Clear() Tokens           { return Tokens{"bg_activity": nil, "bg_shells": nil, "bg_state": nil} }
