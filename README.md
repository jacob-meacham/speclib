# speclib

A package manager for spec-driven libraries. Declare a spec-library dependency,
and `speclib` generates a context-adapted implementation in your language by
driving a coding agent.

## Commands (P0)

- `speclib init [--agent claude]` — scaffold `speclib.toml` and agent integration.
- `speclib add <source>[@<version>] --path <dir> [--lang L] [--context F]` — add a
  dependency (resolution only; does not generate).
- `speclib lock` — resolve any unresolved manifest dependencies.
- `speclib sync [dep]` — generate code for pending dependencies, one at a time.
  Inside Claude Code, the installed skill drives this interactively; the CLI
  exposes `sync --plan` / `sync --record` as its plumbing.
- `speclib verify [dep]` — re-run recorded fixture tests (LLM-free, CI-friendly).
- `speclib status` — show dependency versions and generation state.
- `speclib remove <dep>` — drop a dependency (leaves generated code in place).

Generated code is checked in. `add`/`lock` are fast and never call an LLM; only
`sync` generates. See `docs/specs/2026-07-16-speclib-design.md` for the design.
