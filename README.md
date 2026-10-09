# Herdr Background Activity

See when a Codex agent is idle **and still has background shells running**.
A small, dependency-free [Herdr](https://herdr.dev) plugin publishes a separate
sidebar badge while preserving Herdr's agent state, notifications, and waits.

```text
Example agent
project · codex · Idle
1 background shell
```

This first release supports **Codex**, macOS/Linux, and Herdr **0.8.2+**.
Claude and Pi are not yet supported. Go **1.24+** is required at install time.

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
| `background ?` | The supported agent's current signal is unavailable or ambiguous |
| `N+ background shells` | Workspace has N confirmed shells and at least one unknown Codex pane |
| No badge | Confirmed zero, an unsupported harness, or no current report |

Workspace totals cover **Codex panes only**, including workspaces that also have
Claude or Pi panes. They are not a machine-wide process inventory. A long-lived
dev server counts if Codex includes it in its background-terminal summary.

The plugin reads each Codex pane's **visible screen** approximately every two
seconds and parses the live composer/footer layout examined in Codex
`rust-v0.160.0`. The supported footer contains `Context N% used`,
`? for shortcuts`, and, when present:

```text
N background terminal(s) running · /ps to view · /stop to close
```

Counts can lag by a polling interval, or longer when Herdr is busy. This is a
best-effort UI adapter, not a supported structured background-job API. Custom
status lines, new Codex layouts, dialogs, history views, scrolling, clipped
summaries, and unreadable screens can produce `background ?`. An active working
row without a complete summary also stays unknown because that row can clip it.
The normal supported idle footer without a summary establishes zero. The parser
anchors to the live footer instead of searching transcript history, but a
verbatim imitation of that layout in rendered text can still be ambiguous.

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

## Data and scope

The runtime uses only Herdr's local Unix socket. It does not send agent input,
change agent lifecycle state, traverse process trees, call a model, or persist
terminal contents. The read-only `inspect` command prints pane IDs, harness/state,
and parsed observations; no terminal text or command descriptions are printed.

Owned pane/workspace metadata:

| Token | Value |
| --- | --- |
| `$bg_activity` | Human-readable badge, cleared at known zero |
| `$bg_shells` | Exact decimal count, including `0`; absent when unknown/partial/unsupported |
| `$bg_state` | `known`, `unknown`, `partial` (workspace), or `unsupported` (pane) |

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

Future adapters can use structured signals when a supported connection to an
existing agent is available. Claude needs a validated task-completion refresh
path before its hook snapshots can support a reliable badge.

## References

- [Codex background-terminal footer](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/tui/src/bottom_pane/unified_exec_footer.rs)
- [Codex footer placement](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/tui/src/bottom_pane/mod.rs)
- [Herdr 0.8.2 socket API](https://github.com/herdrdev/herdr/blob/v0.8.2/docs/next/website/src/content/docs/socket-api.mdx)
- [Herdr 0.8.2 plugins](https://github.com/herdrdev/herdr/blob/v0.8.2/docs/next/website/src/content/docs/plugins.mdx)

MIT licensed.
