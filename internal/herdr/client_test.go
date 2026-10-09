package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProtocol(t *testing.T) {
	for _, mode := range []string{"success", "error", "oversize", "cancel", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "hba-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			path := filepath.Join(dir, "s")
			ln, err := net.Listen("unix", path)
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				var req struct {
					Method string
					Params map[string]any
				}
				if err := json.NewDecoder(conn).Decode(&req); err != nil {
					t.Error(err)
					return
				}
				if req.Method != "session.snapshot" || req.Params == nil {
					t.Error("missing method or params")
				}
				switch mode {
				case "success":
					_, _ = conn.Write([]byte("{\"result\":{\"snapshot\":{\"panes\":[{\"pane_id\":\"p1\"}]}}}\n"))
				case "error":
					_, _ = conn.Write([]byte("{\"error\":{\"code\":\"not_found\",\"message\":\"do not log screen content\"}}\n"))
				case "oversize":
					_, _ = conn.Write([]byte(strings.Repeat("x", (4<<20)+1) + "\n"))
				case "cancel":
					_, _ = bufio.NewReader(conn).ReadByte() // Client cancellation closes connection.
				case "malformed":
					_, _ = conn.Write([]byte("invalid\n"))
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			var result SnapshotResult
			err = (&Client{Socket: path}).Call(ctx, "session.snapshot", nil, &result)
			if mode == "success" {
				if err != nil || len(result.Snapshot.Panes) != 1 {
					t.Fatalf("%+v %v", result, err)
				}
			} else if err == nil {
				t.Fatal("expected error")
			}
			if mode == "error" && strings.Contains(err.Error(), "screen content") {
				t.Fatal("server message leaked")
			}
			<-done
		})
	}
}
