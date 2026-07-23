# sync checks — design

**Status:** approved design, pre-implementation
**Date:** 2026-07-22
**Author:** Jacob Meacham (with Claude)

## Summary

A consumer project declares its build/lint/format-check commands once in
`speclib.toml`. `speclib sync --plan` surfaces them verbatim on each work item,
and the sync skill requires the generating agent to run them and get them clean
**before** writing the fixture test, then report honestly what it ran. The CLI
carries the list but never executes it: checks are **advisory**, enforced by
the agent workflow, not by `speclib`.

## Why

The sync instructions already tell the agent to "run the project's build step
and its linter", but nothing names the commands, so the instruction is open to
interpretation. In practice that gap ships real defects: speclib-core-rs was
generated with `fixture_status = 'pass'` recorded while `cargo fmt --check`
fails and `clippy::pedantic` was never configured. Naming the exact commands in
the manifest removes the interpretation gap; the agent no longer decides what
"the linter" means for this project.

Enforcement alternatives (a hard gate in `sync --record`, or a recorded
`checks_status` in the lock) were considered and rejected for now: the CLI
stays out of running checks entirely. The declaration format is chosen so that
upgrading to CLI enforcement later is purely additive.

## Data model

Three field additions. **No lockfile changes.**

- `manifest.Project` gains `Checks []string` (`toml:"checks,omitempty"`):

  ```toml
  [project]
  language = "rust"
  checks = [
    "cargo build",
    "cargo clippy --all-targets -- -D warnings",
    "cargo fmt --check",
  ]
  ```

  Project-level only. No per-dependency override until a real need appears.

- `syncplan.Item` gains `Checks []string` (`json:"checks,omitempty"`).
- `agent.Request` gains `Checks []string` (headless path).

## Data flow

1. `syncplan.Materialize` gains a `checks []string` parameter and copies it
   onto the returned `Item`. Both callers (`runPlan`, `runHeadless` in
   `cmd/sync.go`) already hold the manifest and pass `m.Project.Checks`.
2. `sync --plan --json` emits `checks` with no further changes — it already
   serializes Items. `omitempty` keeps the key absent when undeclared.
3. `runHeadless` threads `item.Checks` into `agent.Request`.

The CLI never executes a check command.

## Sync skill (`internal/scaffold/templates/sync-instructions.md`)

Step 2d becomes an explicit gate sequence:

- If the item has `checks`: run each command in order after generating; every
  one must exit 0 before the fixture test is written. On failure, fix and
  re-run until clean.
- If `checks` is absent: fall back to the current generic guidance (run the
  project's build/compile step and its linter).
- Either way, the final wrap-up (step 3) lists each check command run and its
  result. A sync is never described as clean if a check failed or was skipped.
- A check that cannot be run (missing tool, broken command) is a step-2e
  "ask the user" situation, never a silent skip.

## Headless prompt (`internal/agent/headless.go`)

When `req.Checks` is non-empty, the prompt names the commands and requires them
clean before the fixture test. The `RESULT <test-command> || <status>` line is
unchanged — advisory means no new recorded status.

## Scaffolding

`internal/scaffold/templates/speclib.toml.tmpl` gains a commented example:

```toml
# Commands the sync agent must run clean (exit 0) before recording a sync —
# build, lint, format-check. Run by the agent during sync, never by the CLI.
# checks = ["go build ./...", "golangci-lint run"]
```

## Error handling

- Empty or absent `checks`: key omitted from plan JSON; skill falls back to
  generic guidance. Not an error.
- Malformed manifest TOML already fails at `manifest.Load`; no new paths.
- Un-runnable check commands are the agent's problem to surface to the user
  (skill rule above); the CLI has no execution path to fail.

## Testing

All exact assertions (constitution §12):

- `manifest`: `Load`/`Save` round-trips `checks` (exact slice equality);
  absent field yields empty slice.
- `syncplan`: `Materialize` populates `Item.Checks` verbatim.
- `cmd`: `sync --plan --json` emits the declared commands verbatim (decoded
  JSON compared exactly); the `checks` key is absent when undeclared.
- `agent`: headless prompt construction includes each declared command when
  `req.Checks` is non-empty (prompt string assertion); `Request` receives
  `item.Checks` in the headless flow.
- `scaffold`: the scaffolded `speclib.toml` contains the commented `checks`
  example.

## Out of scope

- CLI execution of checks (`sync --record` gate) and `speclib verify`
  integration.
- Any `speclib.lock` schema change.
- Per-dependency check overrides.
- Language-based auto-detection of check commands.
