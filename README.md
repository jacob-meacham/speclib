# speclib

A package manager for spec-driven libraries. Declare a spec-library dependency,
and `speclib` generates a context-adapted implementation in your language by
driving a coding agent.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/jacob-meacham/speclib/main/install.sh | sh
```

This detects your platform, verifies the release checksum, and installs the
latest binary to `/usr/local/bin` (using sudo only if that isn't writable).
Set `SPECLIB_INSTALL_DIR` to install elsewhere, or `SPECLIB_VERSION` to pin a
version.

Alternatives: `go install github.com/jacob-meacham/speclib@latest`, a manual
download from the [releases page](https://github.com/jacob-meacham/speclib/releases)
(Linux and macOS, amd64 and arm64), or `go build .` in a checkout.

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
