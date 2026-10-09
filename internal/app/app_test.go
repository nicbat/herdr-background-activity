package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nicbat/herdr-background-activity/internal/activity"
	"github.com/nicbat/herdr-background-activity/internal/herdr"
)

const zeroScreen = "• Example complete.\n\n\n› Ask Codex\n\nContext 42% used\n? for shortcuts\n"
const oneScreen = "\n\n1 background terminal running · /ps to view · /stop to close\n\n\n› Ask Codex\n\nContext 42% used\n? for shortcuts\n"

type fakeAPI struct {
	mu                 sync.Mutex
	snapshot           herdr.Snapshot
	replacement        *herdr.Snapshot
	snapshots, reads   int
	screen             string
	readErr, reportErr bool
	truncated          bool
	reports            []herdr.Metadata
}

func (f *fakeAPI) Call(_ context.Context, method string, params, out any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch method {
	case "session.snapshot":
		f.snapshots++
		s := f.snapshot
		if f.replacement != nil && f.snapshots%2 == 0 {
			s = *f.replacement
		}
		out.(*herdr.SnapshotResult).Snapshot = s
	case "pane.read":
		f.reads++
		p := params.(map[string]any)
		if p["source"] != "visible" || p["format"] != "ansi" || p["strip_ansi"] != false {
			return errors.New("unsafe read parameters")
		}
		if f.readErr {
			return errors.New("unavailable")
		}
		out.(*herdr.ReadResult).Read.Text = f.screen
		out.(*herdr.ReadResult).Read.Truncated = f.truncated
	case "pane.report_metadata", "workspace.report_metadata":
		if f.reportErr {
			return errors.New("unavailable")
		}
		f.reports = append(f.reports, params.(herdr.Metadata))
	default:
		return errors.New("unexpected method: " + method)
	}
	return nil
}

func pane() herdr.Pane {
	return herdr.Pane{ID: "p1", WorkspaceID: "w1", TerminalID: "t1", Agent: "codex", Status: "idle", Session: json.RawMessage(`{"id":"session1"}`)}
}
func fixture() *fakeAPI {
	return &fakeAPI{snapshot: herdr.Snapshot{Panes: []herdr.Pane{pane()}, Workspaces: []herdr.Workspace{{ID: "w1"}}}, screen: oneScreen}
}
func token(tokens herdr.Tokens, key string) string {
	if tokens[key] == nil {
		return "<nil>"
	}
	return *tokens[key]
}

func TestReconcileRefreshClearAndRetry(t *testing.T) {
	f := fixture()
	r := Runner{API: f}
	ctx := context.Background()
	now := time.Now()
	if err := r.Tick(ctx, now); err != nil {
		t.Fatal(err)
	}
	if len(f.reports) != 2 {
		t.Fatal(f.reports)
	}
	for _, report := range f.reports {
		if report.Source != herdr.Source || report.TTL != 15000 || len(report.Tokens) != 3 {
			t.Fatal(report)
		}
		for key := range report.Tokens {
			if !strings.HasPrefix(key, "bg_") {
				t.Fatal(key)
			}
		}
		if token(report.Tokens, "bg_shells") != "1" {
			t.Fatal(report)
		}
	}
	_ = r.Tick(ctx, now.Add(time.Second))
	if len(f.reports) != 2 {
		t.Fatal("not deduplicated")
	}
	_ = r.Tick(ctx, now.Add(5*time.Second))
	if len(f.reports) != 4 {
		t.Fatal("not refreshed")
	}
	f.screen = zeroScreen
	_ = r.Tick(ctx, now.Add(6*time.Second))
	if token(f.reports[4].Tokens, "bg_shells") != "0" || f.reports[4].Tokens["bg_activity"] != nil {
		t.Fatal("finished job not cleared")
	}
	f.readErr = true
	_ = r.Tick(ctx, now.Add(7*time.Second))
	if token(f.reports[6].Tokens, "bg_state") != "unknown" || f.reports[6].Tokens["bg_shells"] != nil {
		t.Fatal("stale count retained")
	}
	f.readErr = false
	f.reportErr = true
	if r.Tick(ctx, now.Add(8*time.Second)) == nil {
		t.Fatal("expected failed write")
	}
	f.reportErr = false
	_ = r.Tick(ctx, now.Add(9*time.Second))
	if len(f.reports) != 10 {
		t.Fatal("failed report was not retried")
	}
	r.Clear(ctx)
	for _, report := range f.reports[10:] {
		for _, value := range report.Tokens {
			if value != nil {
				t.Fatal("shutdown did not clear")
			}
		}
	}
	f.snapshot = herdr.Snapshot{}
	_ = r.Tick(ctx, now.Add(10*time.Second))
	if len(r.reports) != 0 {
		t.Fatal("removed resources retained")
	}
}

func TestIdentityAndStatusChange(t *testing.T) {
	for _, change := range []string{"terminal", "session", "harness", "blocked", "working", "new"} {
		t.Run(change, func(t *testing.T) {
			f := fixture()
			p := pane()
			switch change {
			case "terminal":
				p.TerminalID = "t2"
			case "session":
				p.Session = json.RawMessage(`{"id":"session2"}`)
			case "harness":
				p.Agent = "claude"
			case "blocked":
				p.Status = "blocked"
			case "working":
				p.Status = "working"
			case "new":
				p.ID = "p2"
			}
			f.replacement = &herdr.Snapshot{Panes: []herdr.Pane{p}}
			r := Runner{API: f}
			if err := r.Tick(context.Background(), time.Now()); err != nil {
				t.Fatal(err)
			}
			if f.reports[0].Tokens["bg_shells"] != nil {
				t.Fatal("replacement inherited count")
			}
		})
	}
}

func TestObserveUncertainViews(t *testing.T) {
	for _, test := range []string{"blocked", "unknown", "scrolled", "truncated", "read error", "working hidden", "unsupported"} {
		t.Run(test, func(t *testing.T) {
			f := fixture()
			p := pane()
			switch test {
			case "blocked", "unknown":
				p.Status = test
				f.screen = zeroScreen
			case "scrolled":
				p.Scroll.Offset = 2
			case "truncated":
				f.truncated = true
			case "read error":
				f.readErr = true
			case "working hidden":
				p.Status = "working"
				f.screen = zeroScreen
			case "unsupported":
				p.Agent = "claude"
			}
			o := Observe(context.Background(), f, []herdr.Pane{p})[0].Observation
			if o.State == "known" {
				t.Fatal(o)
			}
			if test == "unsupported" && (o.State != "unsupported" || f.reads != 0) {
				t.Fatal(o)
			}
		})
	}
}

func TestQuestionAndDraftReconciliation(t *testing.T) {
	f := fixture()
	r := Runner{API: f}
	now := time.Now()
	ctx := context.Background()
	question := "• Queued follow-up inputs\n\nChoose an option\n\nType your answer\n\nenter submit   ctrl+] skip   shift+→ main prompt\n"
	for i, status := range []string{"working", "blocked", "idle", "unknown"} {
		f.snapshot.Panes[0].Status = status
		f.screen = question
		if err := r.Tick(ctx, now.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
		last := f.reports[len(f.reports)-1]
		if token(last.Tokens, "bg_activity") != "1 reply needed" || last.Tokens["bg_shells"] != nil {
			t.Fatal("question lost or reported false zero", last)
		}
	}
	f.snapshot.Panes[0].Status = "working"
	f.screen = "• Complete.\n\n\n\x1b[0m› A draft\x1b[0m\n\nContext 42% used\ntab to queue message\n"
	if err := r.Tick(ctx, now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	last := f.reports[len(f.reports)-1]
	if token(last.Tokens, "bg_activity") != "1 draft" || last.Tokens["bg_shells"] != nil {
		t.Fatal("draft should survive uncertainty", last)
	}
	f.snapshot.Panes[0].Status = "idle"
	f.screen = zeroScreen
	if err := r.Tick(ctx, now.Add(6*time.Second)); err != nil {
		t.Fatal(err)
	}
	last = f.reports[len(f.reports)-1]
	if last.Tokens["bg_activity"] != nil || token(last.Tokens, "bg_shells") != "0" {
		t.Fatal("answered/submitted labels not cleared", last)
	}
}

func TestUnknownRollupIsQuietWithoutInventingZero(t *testing.T) {
	tokens := WorkspaceTokens([]Result{{pane(), activity.Unknown("hidden")}}, "w1")
	if tokens["bg_activity"] != nil || tokens["bg_shells"] != nil || token(tokens, "bg_state") != "unknown" {
		t.Fatal(tokens)
	}
}

func TestWorkspaceAggregation(t *testing.T) {
	p := pane()
	other := p
	other.WorkspaceID = "elsewhere"
	claude := p
	claude.Agent = "claude"
	results := []Result{{p, activity.Observation{State: "known", Count: 2}}, {other, activity.Observation{State: "known", Count: 20}}, {claude, activity.Observation{State: "unsupported"}}}
	if token(WorkspaceTokens(results, "w1"), "bg_shells") != "2" {
		t.Fatal("wrong aggregate")
	}
	results = append(results, Result{p, activity.Unknown("hidden")})
	tokens := WorkspaceTokens(results, "w1")
	if token(tokens, "bg_activity") != "2+ background shells" || tokens["bg_shells"] != nil || token(tokens, "bg_state") != "partial" {
		t.Fatal(tokens)
	}
	if token(WorkspaceTokens(nil, "w1"), "bg_activity") != "<nil>" {
		t.Fatal("unsupported must be blank")
	}
}
