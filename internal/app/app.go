// Package app polls fresh pane observations and reports expiring display-only counts.
package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nicbat/herdr-background-activity/internal/activity"
	"github.com/nicbat/herdr-background-activity/internal/herdr"
)

type Result struct {
	Pane        herdr.Pane           `json:"pane"`
	Observation activity.Observation `json:"observation"`
}

// Observe never changes a pane or retains its screen contents.
func Observe(ctx context.Context, api herdr.API, panes []herdr.Pane) []Result {
	results := make([]Result, len(panes))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(4, len(panes)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				p := panes[i]
				o := activity.Observation{State: "unsupported"}
				if p.Agent == "codex" {
					o = activity.Unknown("agent not in live view")
					if p.Scroll.Offset != 0 {
						o.Interaction = "history"
					}
					if p.Scroll.Offset == 0 {
						var read herdr.ReadResult
						params := map[string]any{"pane_id": p.ID, "source": "visible", "format": "ansi", "strip_ansi": false}
						if err := api.Call(ctx, "pane.read", params, &read); err != nil {
							o = activity.Unknown("screen unavailable")
						} else if read.Read.Truncated {
							o = activity.Unknown("screen truncated")
						} else {
							o = activity.Codex(read.Read.Text)
							if p.Status != "idle" && p.Status != "done" && o.State == "known" && o.Count == 0 {
								o.State, o.Reason = "unknown", "active view may hide the summary"
							}
						}
					}
					if p.Status == "blocked" && o.Interaction == "" {
						o.Interaction = "input"
					}
				}
				results[i] = Result{p, o}
			}
		}()
	}
	for i := range panes {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}

func PaneTokens(o activity.Observation) herdr.Tokens {
	tokens := herdr.Clear()
	tokens["bg_state"] = herdr.String(o.State)
	if label := o.Label(); label != "" {
		tokens["bg_activity"] = herdr.String(label)
	}
	if o.State == "known" {
		tokens["bg_shells"] = herdr.String(strconv.Itoa(o.Count))
	}
	return tokens
}

func WorkspaceTokens(results []Result, id string) herdr.Tokens {
	var supported, unknown, count, replies, drafts, inputs int
	for _, r := range results {
		if r.Pane.WorkspaceID != id || r.Pane.Agent != "codex" {
			continue
		}
		supported++
		switch r.Observation.Interaction {
		case "question":
			replies++
		case "draft":
			drafts++
		case "input":
			inputs++
		}
		if r.Observation.State != "known" {
			unknown++
		} else {
			count += r.Observation.Count
		}
	}
	if supported == 0 {
		return herdr.Clear()
	}
	o := activity.Observation{State: "known", Count: count}
	tokens := PaneTokens(o)
	if unknown > 0 {
		tokens["bg_shells"] = nil // A partial sum is not an exact total.
		tokens["bg_state"] = herdr.String("unknown")
		label := ""
		if count > 0 {
			label = fmt.Sprintf("%d+ background shells", count)
			tokens["bg_state"] = herdr.String("partial")
		}
		if label != "" {
			tokens["bg_activity"] = herdr.String(label)
		} else {
			tokens["bg_activity"] = nil
		}
	}
	labels := []string{}
	if replies > 0 {
		noun := "replies needed"
		if replies == 1 {
			noun = "reply needed"
		}
		labels = append(labels, fmt.Sprintf("%d %s", replies, noun))
	}
	if inputs > 0 {
		labels = append(labels, fmt.Sprintf("%d awaiting input", inputs))
	}
	if tokens["bg_activity"] != nil {
		labels = append(labels, *tokens["bg_activity"])
	}
	if drafts > 0 {
		noun := "drafts"
		if drafts == 1 {
			noun = "draft"
		}
		labels = append(labels, fmt.Sprintf("%d %s", drafts, noun))
	}
	if len(labels) > 0 {
		tokens["bg_activity"] = herdr.String(strings.Join(labels, " · "))
	}
	return tokens
}

type reported struct {
	Fingerprint string
	At          time.Time
}
type Runner struct {
	API     herdr.API
	reports map[string]reported
}

func (r *Runner) snapshot(ctx context.Context) (herdr.Snapshot, error) {
	var out herdr.SnapshotResult
	err := r.API.Call(ctx, "session.snapshot", nil, &out)
	return out.Snapshot, err
}

func identity(p herdr.Pane) string { return p.TerminalID + "/" + p.Agent + "/" + string(p.Session) }

// Tick rechecks identity after reads; replacements never inherit the previous occupant's result.
func (r *Runner) Tick(ctx context.Context, now time.Time) error {
	if r.reports == nil {
		r.reports = make(map[string]reported)
	}
	snapshot, err := r.snapshot(ctx)
	if err != nil {
		return err
	}
	results := Observe(ctx, r.API, snapshot.Panes)
	current, err := r.snapshot(ctx)
	if err != nil {
		return err
	}
	byID := make(map[string]Result)
	for _, result := range results {
		byID[result.Pane.ID] = result
	}
	validated := make([]Result, 0, len(current.Panes))
	live := make(map[string]bool)
	var firstErr error
	for _, pane := range current.Panes {
		key := "pane/" + pane.ID
		live[key] = true
		result, ok := byID[pane.ID]
		if !ok || identity(result.Pane) != identity(pane) || result.Pane.Status != pane.Status || result.Pane.Scroll.Offset != pane.Scroll.Offset {
			result.Observation = activity.Unknown("pane changed while reading")
		}
		if pane.Scroll.Offset != 0 {
			result.Observation = activity.Unknown("history view")
			result.Observation.Interaction = "history"
		}
		if pane.Status == "blocked" && result.Observation.Interaction == "" {
			result.Observation.Interaction = "input"
		}
		result.Pane = pane
		if pane.Agent != "codex" {
			result.Observation = activity.Observation{State: "unsupported"}
		}
		validated = append(validated, result)
		tokens := PaneTokens(result.Observation)
		// Do not add a visible badge to harnesses this version cannot observe.
		if pane.Agent == "" {
			tokens = herdr.Clear()
		}
		if err := r.report(ctx, key, identity(pane), tokens, now); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for _, workspace := range current.Workspaces {
		key := "workspace/" + workspace.ID
		live[key] = true
		if err := r.report(ctx, key, "", WorkspaceTokens(validated, workspace.ID), now); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for key := range r.reports {
		if !live[key] {
			delete(r.reports, key)
		}
	}
	return firstErr
}

func fingerprint(tokens herdr.Tokens) string {
	keys := make([]string, 0, len(tokens))
	for key := range tokens {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := ""
	for _, key := range keys {
		out += key + "="
		if tokens[key] != nil {
			out += *tokens[key]
		}
		out += ";"
	}
	return out
}

func (r *Runner) report(ctx context.Context, key, id string, tokens herdr.Tokens, now time.Time) error {
	fp := id + "/" + fingerprint(tokens)
	if previous, ok := r.reports[key]; ok && previous.Fingerprint == fp && now.Sub(previous.At) < 5*time.Second {
		return nil
	}
	params := herdr.Metadata{Source: herdr.Source, Tokens: tokens, TTL: herdr.TTL}
	method := "pane.report_metadata"
	if len(key) > 5 && key[:5] == "pane/" {
		params.PaneID = key[5:]
	} else {
		method = "workspace.report_metadata"
		params.WorkspaceID = key[len("workspace/"):]
	}
	if err := r.API.Call(ctx, method, params, nil); err != nil {
		return err
	}
	r.reports[key] = reported{fp, now}
	return nil
}

// Clear is best-effort on a clean shutdown; TTL also removes data after a crash.
func (r *Runner) Clear(ctx context.Context) {
	for key := range r.reports {
		_ = r.report(ctx, key, "clear", herdr.Clear(), time.Now())
	}
}
