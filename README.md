# Herdr Background Activity

See when a Codex agent has **background shells, a prompt draft, or a pending reply**.
Also shows **Claude prompt drafts** in the agent and workspace sidebars.
A small, dependency-free [Herdr](https://herdr.dev) plugin publishes a separate
sidebar badge while preserving Herdr's agent state, notifications, and waits.

```text
Example agent
project · codex · Idle
1 background shell
```

Runs on macOS/Linux with Herdr **0.8.2+** and Go **1.24+** at install time.

| Harness | Prompt drafts | Background-shell counts | Question labels |
| --- | --- | --- | --- |
| Codex | Yes | Yes | Async `Reply needed` and native blocked input |
| Claude | Yes | Not yet | Native blocked `Input needed` only |
| Pi | Not yet | Not yet | Not yet |

## Install

From a terminal inside Herdr:

```sh
herdr plugin install nicbat/herdr-background-activity --yes
```

For a reproducible install, add `--ref <commit>`.

Add a row containing `$bg_activity` to your existing agent sidebar rows in
`~/.config/herdr/config.toml`. For example:

```toml
[ui.sidebar.agents]
rows = [
  ["state_icon", "pane"],
  ["workspace", "agent", "state_text"],
  [{ token = "$bg_activity", fg = "#f9e2af", dim = false }],
]
```

You can add the same row to `ui.sidebar.spaces.rows` for a workspace total.
Merge these entries into your existing tables; do not create duplicate tables
or discard your current sidebar configuration. Then reload and start:

```sh
herdr server reload-config
herdr plugin action invoke herdr.background-activity.start
```

The plugin also starts automatically with Herdr. It can coexist with Auto Title;
it does not write Auto Title's `topic`, `counts`, or `activity` tokens.

## What the badge means

| Badge | Meaning |
| --- | --- |
| `1 background shell` | One terminal in the recognizable current Codex footer |
| `N background shells` | N terminals in that footer |
| `Draft prompt` | Unsubmitted text in a recognized Codex or Claude composer; Codex queue hints also detect attachments |
| `Reply needed` | An expanded or collapsed Codex question, even while the model keeps working |
| `Input needed` | Herdr reports blocked, but the specific input UI is not recognized |
| `Menu open` / `Viewing history` | A recognizable view is obscuring the live footer |
| `N+ background shells` | Workspace has N confirmed shells and at least one unknown Codex pane |
| No badge | No confirmed activity to display; this does **not** prove zero shells |

Interaction labels and visible shell counts can appear together, such as
`Draft prompt · 1 background shell`. Workspace rows summarize panes with pending
replies and drafts, with replies first. `1 reply needed` means one pane needs a
reply; that pane can contain multiple questions.

As of v0.2.0, uncertain shell counts remain `unknown` internally without a
`background ?` badge. Check `inspect` or `$bg_state` for diagnostic detail. This
keeps drafting and ordinary work quiet without inventing a zero count.

Workspace **shell totals cover Codex panes only**. Draft and native blocked-input
totals include both Codex and Claude. These are not a machine-wide process inventory. A long-lived
dev server counts if Codex includes it in its background-terminal summary.

The plugin reads Codex and Claude panes' **visible screens** approximately every two
seconds. The Codex adapter parses the live composer/footer layout examined in Codex
`rust-v0.160.0`. The supported footer contains `Context N% used` or
`N% context left`. Empty composers show `? for shortcuts`; drafts can replace it
with a queue hint or hide it. ANSI styling distinguishes dim placeholder text
from typed input, so a missing shortcut hint alone is not treated as a draft.
The background summary, when present, reads:

```text
N background terminal(s) running · /ps to view · /stop to close
```

Counts can lag by a polling interval, or longer when Herdr is busy. This is a
best-effort UI adapter, not a supported structured background-job API. Custom
status lines, new Codex layouts, dialogs, history views, scrolling, clipped
summaries, and unreadable screens can leave the count unknown. An active working
row without a readable count stays unknown because that row can clip it.
The complete `N background terminal(s) running` phrase still establishes a count
when only the trailing shortcut help is clipped. Working tips below that row
do not invalidate it.
The normal supported idle footer without a summary establishes zero. The parser
anchors to the live footer instead of searching transcript history, but a
verbatim imitation of that layout in rendered text can still be ambiguous.
Question labels use the dedicated submit/skip/main-prompt controls or collapsed
`? N questions` and `to answer` markers, not question marks in ordinary prose.
Key remappings or layouts that hide those markers may remain unrecognized.

The Claude draft adapter requires a `❯` prompt between two horizontal rules,
followed by a recognizable `shift+tab to cycle` or `? for shortcuts` footer.
It supports plain/styled and multiline drafts, ignores empty or dim placeholder
text, and rejects known menu/permission/history controls. Blocked, scrolled,
truncated, or unreadable views do not establish a draft. Alternate layouts and
unrecognized placeholder styling can still be ambiguous. It does not install
Claude hooks or infer any Claude background-shell count.

## Controls

```sh
herdr plugin action invoke herdr.background-activity.status
herdr plugin action invoke herdr.background-activity.restart
herdr plugin action invoke herdr.background-activity.stop
```

**Stop before disabling or uninstalling**: Herdr startup hooks are unsupervised,
so disabling a plugin alone does not stop an already-running reporter.

```sh
herdr plugin action invoke herdr.background-activity.stop
herdr plugin disable herdr.background-activity
```

One daemon runs per Herdr socket and state directory. Start is idempotent;
restart waits for the previous reporter to finish clearing its metadata. The
daemon exits when its Herdr server socket disappears or is replaced. A private
control socket handles stop requests; no PID-file process killing is used.
State and startup errors live under `HERDR_PLUGIN_STATE_DIR`, in a socket-specific
subdirectory. Socket path length limits still apply to unusually long config paths.
When run directly from a shell, the binary uses
`${XDG_STATE_HOME:-~/.local/state}/herdr/plugins/herdr.background-activity`.
Set `HERDR_PLUGIN_STATE_DIR` explicitly if your Herdr installation uses a
different state directory.

## Data and scope

The runtime uses only Herdr's local Unix socket. It does not send agent input,
change agent lifecycle state, traverse process trees, call a model, or persist
terminal contents. The read-only `inspect` command prints pane IDs, harness/state,
and parsed observations; no terminal text or command descriptions are printed.

Owned pane/workspace metadata:

| Token | Value |
| --- | --- |
| `$bg_activity` | Confirmed shell/interaction badge; absent for a bare unknown count |
| `$bg_shells` | Exact decimal count, including `0`; absent when unknown/partial/unsupported |
| `$bg_state` | Shell-count confidence: `known`, `unknown`, `partial` (workspace), or `unsupported` |

A Claude pane can report `bg_state=unsupported` alongside `Draft prompt`: its
draft is visible but its shell inventory is unsupported. A workspace with only
Claude interactions likewise has no exact shell count.

Reports refresh about every six seconds at the default poll interval and expire
after **15 seconds** without renewal. Graceful shutdown clears the owned keys.
Failed reads clear exact counts on the next successful report. Identity and
view state are rechecked after reads to reject observations of replaced panes.
An agent replacement with the same terminal, harness, and no reported session
identity cannot always be detected between polls.

## Develop

```sh
make check                  # formatting, vet, race tests, build
make build
./herdr-background-activity inspect  # read-only; run inside Herdr
herdr plugin link "$PWD"
herdr plugin action invoke herdr.background-activity.start
```

The runtime uses only Go's standard library. Tests use synthetic terminal text
and fake Unix-socket servers. Coverage includes footer boundaries, unknowns,
completion clearing, reconciliation, expiry settings, failed-report retry,
session changes, protocol errors, and real subprocess start/stop/restart.
Draft/question tests cover ANSI placeholders, multiline input, collapsed and
expanded asynchronous questions, working tips, and clearing submitted drafts.
Claude tests cover bordered composers, permission choices, transcript mentions,
draft clearing, mixed-harness rollups, and replacement/view-state guards.

Future adapters can use structured signals when a supported connection to an
existing agent is available. Claude needs a validated task-completion refresh
path before its hook snapshots can support a reliable badge.

## References

- [Codex background-terminal footer](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/tui/src/bottom_pane/unified_exec_footer.rs)
- [Codex footer placement](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/tui/src/bottom_pane/mod.rs)
- [Codex draft footer modes](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/tui/src/bottom_pane/footer.rs)
- [Codex collapsed questions](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/tui/src/bottom_pane/questions.rs)
- [Codex asynchronous question snapshots](https://github.com/openai/codex/tree/rust-v0.160.0/codex-rs/tui/src/chatwidget/tests/snapshots)
- [Herdr 0.8.2 socket API](https://github.com/herdrdev/herdr/blob/v0.8.2/docs/next/website/src/content/docs/socket-api.mdx)
- [Herdr 0.8.2 plugins](https://github.com/herdrdev/herdr/blob/v0.8.2/docs/next/website/src/content/docs/plugins.mdx)

MIT licensed.
