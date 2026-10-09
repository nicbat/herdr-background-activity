# Background Activity

A display-only Herdr plugin. Follow the user's reuse-first worktree policy.
Keep one writer per checkout and preserve unrelated changes.

- Never send input to agents, alter lifecycle state, or inspect process trees.
- Read only the current visible terminal screen; never persist terminal text.
- A missing or unreadable signal is unknown, not zero.
- Own only `bg_activity`, `bg_shells`, and `bg_state` metadata keys.
- Keep the runtime dependency-free and support macOS and Linux.
- Run `make check` before committing. Tests must exercise detector boundaries,
  stale data, session changes, and protocol behavior.
- Use Conventional Commits. Keep public fixtures synthetic and free of local
  paths, transcripts, and internal project information.
