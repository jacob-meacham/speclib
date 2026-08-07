# sync CLI UX — design

**Status:** approved design, pre-implementation
**Date:** 2026-08-07
**Author:** Jacob Meacham (with Claude)

## Summary

`speclib sync` run from a terminal launches the coding agent's own interactive
UI, preloaded with the canonical sync instructions — the user watches
generation live, answers permission prompts and agent questions natively, and
the session records provenance through the existing `sync --record` plumbing.
`speclib sync --headless` becomes a working non-interactive path for CI:
per-adapter permission grants, streamed progress, a timeout, and a recording
gate that re-runs the reported test command before anything lands in the
lockfile. No custom TUI is built; the agent's shipped UI is the UI.

## Why

Today, bare `speclib sync` (the exact command `speclib add`'s hint recommends)
takes the headless path, which is broken four ways at once:

1. **It cannot succeed.** `claude -p` is spawned with no permission flags, and
   a non-interactive session cannot grant permission prompts. Probed against
   claude v2.1.224: every write path (Write tool, shell redirection, `tee`,
   `python3`) was denied; nothing was generated.
2. **It looks frozen.** `Generate` uses `CombinedOutput()`, so agent output is
   buffered until exit. The user sees `Generating <dep>...` then silence.
3. **It hangs unbounded.** The context is `context.Background()` — no timeout.
4. **It can record false success.** The same probe showed the agent printing
   the demanded `RESULT ... || pass` line despite generating nothing;
   `runHeadless` would have recorded that sync as complete.

The tutorial's supported path (the installed skill inside Claude Code) works,
but the CLI's default steers users away from it and into the broken path.

## Mode selection (`cmd/sync.go`)

- `speclib sync [dep]` with stdin **and** stdout both TTYs → interactive
  handoff (an interactive UI needs both).
- `speclib sync --headless [dep]` → headless path (any environment).
- Non-TTY without `--headless` → error:
  `not a terminal; pass --headless for non-interactive use`.
  Failing loudly beats silently spending tokens from a pipe.
- `--plan`, `--record`, `--json` are unchanged; the installed skill keeps
  working exactly as today. `--headless` is mutually exclusive with `--plan`
  and `--record`.
- TTY detection is injectable (a `func() bool` on the command constructor) so
  mode selection is unit-testable.

## Agent adapters (`internal/agent`)

An adapter table maps agent name → binary, default headless permission args,
and how to build the headless and interactive invocations. `claude` is the
only entry today; the table is the seam for cursor/others later.

claude defaults:

- headless: `claude -p <prompt> --allowedTools Write,Edit,Bash
  --output-format stream-json --verbose` — the agent can generate code and
  iterate on tests; network-facing tools stay denied.
- interactive: `claude <instructions>` with no permission args — the UI
  prompts natively.

## Configuration (`[agent]` in `speclib.toml`)

Reasonable defaults per adapter, overridable to anything:

```toml
[agent]
# Which agent adapter drives generation (default "claude").
# command = "claude"

# Replaces the adapter's default permission args for headless sync, verbatim.
# permissions = ["--allowedTools", "Write,Edit,Bash"]
```

- `command` selects the adapter. An unknown value errors, listing supported
  adapters.
- `permissions` is a raw argv slice substituted for the adapter's default
  permission args in headless mode (e.g.
  `["--dangerously-skip-permissions"]` for sandboxed CI, or a tighter
  allowlist). Unset → adapter default. Interactive mode is unaffected — its
  UI handles permissions.
- `manifest.Manifest` gains an `Agent` struct (`command`, `permissions`);
  `internal/scaffold/templates/speclib.toml.tmpl` gains the commented example
  above.

## Interactive handoff (`internal/agent/interactive.go`)

1. Compute pending and materialize specs exactly as `--plan` does; if nothing
   is pending, print `Nothing to sync.` and exit without launching anything.
2. Launch the adapter's UI **once for the whole batch**, with the embedded
   `templates/sync-instructions.md` body as the initial prompt — the handoff
   works even when `init --agent` was never run. stdin/stdout/stderr are
   inherited from the terminal; the child's exit code is propagated.
3. After the session ends, reload the lockfile and print a summary: which
   deps were recorded, which remain pending.
4. Adapter binary missing from PATH → actionable error naming the binary,
   never a hang.

## Headless path (`internal/agent/headless.go`)

- Parse `stream-json` events as they arrive and render one-line progress to
  stderr (`[tool] Write src/slugify/slugify.py`); the `RESULT` line is parsed
  from the final result event, not a buffered dump.
- `--timeout <duration>` flag, default `15m`, via `context.WithTimeout`. On
  expiry or an exit without a parseable `RESULT` line, fail with the last
  lines of the agent transcript.
- **Recording gate:** before `runRecord`, speclib re-runs the reported test
  command itself (the same runner `verify` uses). Nonzero exit → error, no
  record. This closes the false-success hole. The bare `--record` plumbing
  stays ungated: the interactive skill runs tests itself and `verify`
  backstops it.

## Error handling

- Unknown `[agent].command` → error listing supported adapters.
- Adapter binary not on PATH → error naming it (both modes).
- Non-TTY without `--headless` → error with the hint above.
- Headless timeout / missing `RESULT` / failing test command → error with
  transcript tail; lockfile untouched.
- Interactive session exits nonzero → propagate the code after printing the
  recorded-vs-pending summary (partial progress is already durable via
  `--record`).

## Testing

All exact assertions (constitution §12). The headless and interactive paths
are exercised against a scripted fake `claude` on PATH (as real `claude` must
never run in tests):

- `cmd`: mode selection — TTY→interactive, `--headless`→headless, non-TTY
  without `--headless` errors (injected TTY detector); flag exclusivity.
- `agent`: adapter arg construction with default and overridden
  `permissions` (exact argv); stream-json event → progress-line rendering;
  `RESULT` parsed from the result event; timeout kills the child and errors;
  missing-binary error message.
- `cmd`: recording gate — a fake agent whose `RESULT` test command fails
  leaves the lockfile unchanged and exits nonzero; a passing one records.
- `manifest`: `[agent]` round-trip; absent section yields defaults.
- `scaffold`: template contains the commented `[agent]` example.
- Existing stub-backend e2e unchanged; one manual dogfood run of each mode.

## Docs

- Tutorial Part 2: the generation step becomes `speclib sync` (launches the
  agent UI); "say 'sync my speclib dependencies' inside Claude Code" stays as
  the alternative. A short CI subsection shows `--headless`.
- `speclib add`'s hint text is unchanged — it becomes truthful.

## Out of scope

- A custom TUI or any wrapping of agent UIs.
- Non-claude adapters (the adapter table is the seam; no cursor entry yet).
- Permission configuration for interactive mode.
- Gating the bare `--record` plumbing.
- Lockfile schema changes.
