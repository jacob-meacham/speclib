# speclib

A package manager for spec-driven libraries. Declare a spec-library dependency,
and `speclib` generates a context-adapted implementation in your language by
driving a coding agent.

## Install

With Go:

```sh
go install github.com/jacob-meacham/speclib@latest
```

Or grab a prebuilt binary from the
[releases page](https://github.com/jacob-meacham/speclib/releases) — e.g. for
Linux on amd64:

```sh
curl -sL https://github.com/jacob-meacham/speclib/releases/download/v0.1.0/speclib_0.1.0_linux_amd64.tar.gz \
  | tar xz speclib
./speclib --version
```

Binaries are published for Linux and macOS (amd64 and arm64). Building from
source works anywhere Go does: `go build .` in a checkout.

## Quick start

The tutorial walks the whole loop — authoring a spec-library, releasing it,
generating an implementation into a consumer project, and verifying it in CI:
[docs/tutorials/first-spec-library.md](docs/tutorials/first-spec-library.md).

## Commands

Consuming spec-libraries:

- `speclib init [--agent claude|cursor|agents]` — scaffold `speclib.toml` and
  agent integration.
- `speclib add <source>[@<version>] --path <dir> [--lang L] [--context F]` —
  add a dependency (resolution only; does not generate).
- `speclib lock` — resolve any unresolved manifest dependencies.
- `speclib sync [dep]` — generate code for pending dependencies, one at a time.
  Inside Claude Code, the installed skill drives this interactively; the CLI
  exposes `sync --plan` / `sync --record` as its plumbing.
- `speclib update [dep] [--to <version>]` — re-resolve to newer versions.
- `speclib verify [dep]` — re-run recorded fixture tests (LLM-free,
  CI-friendly).
- `speclib status` — show dependency versions and generation state.
- `speclib remove <dep>` — drop a dependency (leaves generated code in place).

Authoring spec-libraries:

- `speclib new <name>` — scaffold a new spec-library in `./<name>`.
- `speclib lint` — validate the library manifest and file references.
- `speclib release <version>` — lint, then tag a release.

Generated code is checked in. `add`/`lock` are fast and never call an LLM; only
`sync` generates. See `docs/specs/2026-07-16-speclib-design.md` for the design.
