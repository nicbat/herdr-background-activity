# Herdr Background Activity

See **background shells, waiting subagents, prompt drafts, and input requests**
in the agent and workspace sidebars, with adapters for Codex, Claude, and stock Pi.
A small, dependency-free [Herdr](https://herdr.dev) plugin publishes a separate
sidebar badge while preserving Herdr's agent state, notifications, and waits.

```text
Example agent
project · codex · Idle
1 background shell
```

Runs on macOS/Linux with Herdr **0.8.2+** and Go **1.24+** at install time.

| Harness | Prompt drafts | Background shells | Background agents | Input labels |
| --- | --- | --- | --- | --- |
| Codex | Yes | Visible footer | Unavailable | Async replies, recognized approvals, generic blocked input |
| Claude | Yes | Visible/wrapped footer | Explicit live waiting summary | Recognized questions/approvals, generic blocked input |
| Pi | Stock editor | Unavailable in stock Pi | Extension-specific; not implemented | Stock text-input dialogs and generic selectors |

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

The plugin also starts automatically with Herdr. After upgrading, invoke `start`
once: it compares the running version and executable digest, replaces an older
reporter, and waits for its metadata cleanup. Starting the same build is idempotent.
`status` shows the running version and digest; `version` shows the CLI version.
It also works when upgrading from releases that reported no build identity. It can coexist with Auto Title;
it does not write Auto Title's `topic`, `counts`, or `activity` tokens.

## What the badge means

| Badge | Meaning |
| --- | --- |
| `1 background shell` | One shell in a recognized Codex or Claude footer |
| `N background shells` | N terminals in that footer |
| `Draft prompt` | Unsubmitted text in a recognized Codex, Claude, or stock Pi composer; Codex queue hints also detect attachments |
| `Reply needed` | A recognized Codex/Claude question or stock Pi text-input dialog |
| `Approval needed` | A recognized live Codex or Claude approval dialog |
| `N background agents` | Claude explicitly says it is waiting for N background agents |
| `Input needed` | Herdr reports blocked, but the specific input UI is not recognized |
| `Menu open` / `Viewing history` | A recognizable view is obscuring the live footer |
| `N+ background shells` | Workspace has N confirmed shells and at least one unknown Codex/Claude pane |
| No badge | No confirmed activity to display; this does **not** prove zero shells |

Interaction labels and visible shell counts can appear together, such as
`Draft prompt · 1 background shell`. Workspace rows summarize approvals, replies, generic input, shells, background
agents, and drafts, in that order. `1 reply needed` means one pane needs a
reply; that pane can contain multiple questions.

As of v0.2.0, uncertain shell counts remain `unknown` internally without a
`background ?` badge. Check `inspect` or `$bg_state` for diagnostic detail. This
keeps drafting and ordinary work quiet without inventing a zero count.

Workspace shell totals include **Codex and Claude**. Draft/input totals also
include **Pi**. These are not a machine-wide process inventory. A long-lived dev
server counts if its agent includes it in the live shell summary.

The plugin reads supported panes' **visible screens** approximately every two
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
The normal supported Codex idle footer without a summary establishes zero. The parser
anchors to the live footer instead of searching transcript history, but a
verbatim imitation of that layout in rendered text can still be ambiguous.
Question labels use the dedicated submit/skip/main-prompt controls or collapsed
`? N questions` and `to answer` markers, not question marks in ordinary prose.
Key remappings or layouts that hide those markers may remain unrecognized.

The Claude adapter requires a `❯` prompt between two horizontal rules,
followed by a recognizable `shift+tab to cycle`, `? for shortcuts`, or
`← for agents` footer.
Unicode whitespace after the prompt glyph is supported, including Claude's
non-breaking space (fixed in v0.3.1).
It supports plain/styled and multiline drafts, ignores empty or dim placeholder
text, and rejects known menu/permission/history controls. Blocked, scrolled,
truncated, or unreadable views do not establish a draft. Alternate layouts and
unrecognized placeholder styling can still be ambiguous.

Since v0.4.0, Claude counts come from a complete `N shell(s)` component in the
native control row, as observed in Claude Code **2.1.287**:

```text
⏵⏵ auto mode on · 2 shells · ← for agents
```

The footer updates independently of agent turns. Counts clear when the shell
component disappears. Transcript summaries such as `1 shell still running`,
custom status-line text, and generic task/agent counts are ignored. A missing
Claude count remains **unknown**, since narrow layouts may hide the component;
it never establishes zero. Workspace sums with unknown panes display `N+`.
Wrapped native control rows are joined within a bounded footer region. Complete
counts remain usable when trailing shortcut help is clipped. Custom status
lines above the controls are not merged into the count. A clipped count itself
stays unknown. No Claude hooks, extra model calls, or agent restarts are required.

Claude background-agent counts come from `Waiting for N background agent(s) to
finish` immediately above the live composer, validated in **2.1.287**. Historical
waiting messages and retained completed/failed panel rows are ignored. This
covers Claude's waiting view; it is not a full inventory while the parent is doing
other work. Workspace agent sums always use `N+` because other panes may hide
children. Codex and extension-specific Pi agent inventories remain unavailable.

Claude blocked forms with multi-question navigation show `Reply needed`; known
permission forms show `Approval needed`. Codex's recognizable approval choices
also get `Approval needed`. Generic selectors stay `Input needed`. These labels
do not approve anything or change Herdr's native blocked state.

The Pi adapter follows **v0.87.1** source: its stock bordered editor, cwd/context
footer, and built-in extension text-input/select dialogs. Pi's bash tool runs in
the foreground and publishes no background-job inventory. Consequently Pi never
reports a background-shell count, even for a command containing `&`. Custom
editors/footers, autocomplete lists, remapped controls, and third-party background
tools may be unrecognized. Pi does not need an additional extension for supported
draft/input labels. Its adapter is source-verified and covered by synthetic
fixtures; live Pi validation remains pending.

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

A Claude pane can report `bg_state=unknown` alongside `Draft prompt` when its
draft is visible but its shell count is hidden. Positive visible counts use
`known`; Pi uses `unsupported` for its shell inventory, independently of draft/input
labels. Background-agent observations affect the badge, not `$bg_shells` or
`$bg_state`; both tokens retain their shell-only meanings.

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
draft clearing, shell-count completion, mixed-harness rollups, and
replacement/view-state guards. Pi fixtures cover stock editors, styling,
multiline drafts, dialogs, custom/hidden footers, and unsupported counts. Service
tests verify version upgrades and rebuilds of the same version replace only the
reporter, while starting the same build preserves it.

Future adapters can use structured signals when a supported connection to an
existing agent is available. Claude needs a validated task-completion refresh
path before its hook snapshots can support a reliable badge.

## References

- [Pi v0.87.1 editor](https://github.com/earendil-works/pi/blob/v0.87.1/packages/tui/src/components/editor.ts)
- [Pi v0.87.1 footer and dialogs](https://github.com/earendil-works/pi/tree/v0.87.1/packages/coding-agent/src/modes/interactive/components)
- [Pi v0.87.1 foreground Bash tool](https://github.com/earendil-works/pi/blob/v0.87.1/packages/coding-agent/src/core/tools/bash.ts)
- [Claude background subagent lifecycle](https://code.claude.com/docs/en/sub-agents#run-subagents-in-foreground-or-background)

- [Codex background-terminal footer](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/tui/src/bottom_pane/unified_exec_footer.rs)
- [Codex footer placement](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/tui/src/bottom_pane/mod.rs)
- [Codex draft footer modes](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/tui/src/bottom_pane/footer.rs)
- [Codex collapsed questions](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/tui/src/bottom_pane/questions.rs)
- [Codex asynchronous question snapshots](https://github.com/openai/codex/tree/rust-v0.160.0/codex-rs/tui/src/chatwidget/tests/snapshots)
- [Herdr 0.8.2 socket API](https://github.com/herdrdev/herdr/blob/v0.8.2/docs/next/website/src/content/docs/socket-api.mdx)
- [Herdr 0.8.2 plugins](https://github.com/herdrdev/herdr/blob/v0.8.2/docs/next/website/src/content/docs/plugins.mdx)

MIT licensed.
