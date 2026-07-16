# speclib — design

**Status:** approved design, pre-implementation
**Date:** 2026-07-16
**Author:** Jacob Meacham (with Claude)

## Summary

`speclib` is a package manager for **spec-driven libraries**: git repos that
ship a specification, a prompt, and test fixtures instead of code (see
[Spec-Driven Libraries](https://jemonjam.com/2026/02/04/spec-driven-library) and
the `roku-deeplink-spec` example). You declare spec-library dependencies in a
manifest, pin them to versions, and `speclib` produces working implementation
code — **in your project's language, adapted to your project's shape** — by
driving a coding agent.

It has `uv`-style semantics (`add` / `lock` / `sync`), with one crucial
difference from a normal package manager: producing a dependency's code is an
**interactive, potentially hours-long agent session** — not a download. So the
fast bookkeeping (`add`, `lock`) is cleanly separated from the slow generation
(`sync`), and the design is built around making that separation ergonomic.

## Why

Three motivations, in increasing order of importance:

1. **Don't reverse-engineer the same protocol N times.** Undocumented protocols
   and integrations get reimplemented from scratch in every language. Capture the
   knowledge once as a spec; generate an implementation on demand.

2. **Maintain a library once, not once-per-language.** A spec library is a single
   source of truth. Fixing a bug or adding a feature means editing the spec and
   cutting a release — not porting the change across a Kotlin, Python, and
   TypeScript codebase and keeping them in sync forever.

3. **Library-fication across differing contexts (the big one).** The same spec
   can be consumed by codebases that need *different shapes*. Two servers
   integrating the same API might use different databases, different domain
   types, and different frameworks. speclib generates code that **fits each
   consumer's context** — its types, its persistence layer, its conventions —
   transparently. The spec is a behavioral contract; the generated code is a
   context-adapted implementation of it.

A consequence of (3): generated code is **not** expected to be identical across
consumers. The lockfile makes *your* checkout reproducible and verifiable; it
does not make two different projects produce the same code. That's a feature.

## Core principle: a clean seam

Two halves, one hard boundary between them:

- **CLI (Go) — deterministic plumbing.** Resolves versions, fetches specs at a
  git tag, hashes spec content, computes spec diffs, runs fixtures, reads/writes
  the manifest and lockfile. **The CLI never calls an LLM.**
- **Agent integration — the LLM work.** Generates implementation code from a
  spec, migrates code across a spec diff, writes a fixture-driven test. **The
  agent never touches the lockfile.**

This is the spec-kit integration model: `speclib init --agent claude` drops
skill/command files into the repo; the agent drives the LLM-heavy steps and
calls back into the CLI for plumbing. Claude Code is the first-class integration
in v1; the agent backend is an interface so other agents slot in later without
touching the core.

**Why the seam matters:** it keeps the reproducible, testable, CI-runnable parts
(pins, hashes, fixtures) free of non-determinism, and confines the
non-deterministic part (code generation) behind a single interface with a
fixture gate in front of it.

## Language & distribution

- CLI implemented in **Go**: single static binary, trivial cross-compile,
  goreleaser + Homebrew tap for painless install, fast startup and compile. The
  tool is I/O- and orchestration-bound (git, hashing, diffing, shelling out), so
  there is no hot compute path — Go is more than fast enough, and its quick
  iteration helps both humans and coding agents extend it.

## The spec-library format (author's "package")

A spec-library is a git repo. **Versions are semver git tags** (`v1.4.0`) — that
tag *is* the version; it is not duplicated anywhere in the repo. A library
contains:

```
roku-deeplink-spec/
  speclib.toml        # library manifest (no version — the tag is the version)
  PROMPT.md           # entry point for the LLM
  SPEC.md             # the specification
  fixtures/           # test cases — file OR directory, library-defined format
  CHANGELOG.md
```

Library manifest — `speclib.toml`:

```toml
[library]
name    = "roku-deeplink"
summary = "Roku External Control Protocol deep-linking"

[files]
prompt   = "PROMPT.md"
spec     = "SPEC.md"
fixtures = "fixtures/"          # file or directory; omit if none

[hints]
languages = ["kotlin", "typescript", "python", "ruby"]   # non-binding hints
```

## Local context & adaptation

Because generated code must fit the consumer's shape (motivation 3), the agent
needs to know that shape. Two layers, simplest first:

- **Repo agent instructions (default).** The agent already reads the consumer
  repo's `AGENTS.md` / `CLAUDE.md`. Ambient conventions ("models live in
  `internal/models`", "persistence goes through `repo` interfaces") come from
  there for free — no speclib-specific config.
- **Per-dependency context (escape hatch).** For targeted integration guidance,
  a dependency may point at a context file (default location
  `speclib/<dep>.md`): "map the API's `device_id` to our `Device.ID`; persist
  via `deviceRepo.Save`." This is handed to the agent during generation and
  migration.

Fixtures test the spec's **behavioral contract**; local context shapes the
**integration surfaces**. The two are orthogonal: adapting persistence to Mongo
vs. Postgres doesn't change whether the deep-link URL is built correctly, and it
is the latter that fixtures pin.

## The consumer side

### Manifest — `speclib.toml`

One target per dependency (`path` + optional `language`), with an optional
project-level default language:

```toml
[project]
language = "typescript"        # default target language for dependencies

[dependencies.roku-deeplink]
source  = "https://github.com/jmeacham/roku-deeplink-spec"
version = "^1.4"               # semver range
path    = "src/gen/roku"       # where generated code goes
# language = "kotlin"          # optional: override the project default
# context  = "speclib/roku.md" # optional: targeted integration guidance
```

### Lockfile — `speclib.lock`

Each entry has two distinct parts: **resolution** (written by `add` / `lock`,
fast, no LLM) and **generation provenance** (written by `sync` after the agent
finishes, slow). Their relationship *is* the dependency's state:

```toml
[[package]]
name    = "roku-deeplink"
source  = "https://github.com/jmeacham/roku-deeplink-spec"

# --- resolution (from add / lock) ---
version   = "1.4.0"            # resolved semver (from the git tag)
commit    = "a1b2c3d…"         # exact tagged commit — the immutable pin
spec_hash = "sha256:…"         # hash over prompt+spec+fixtures at that commit
language  = "typescript"
path      = "src/gen/roku"

# --- generation provenance (from sync) ---
generated_commit = "a1b2c3d…"  # spec commit the on-disk code was built from
generated_at     = "2026-07-16T12:00:00Z"
generator        = "claude-code / claude-opus-4-8"
fixture_status   = "pass"      # pass | skip (no fixtures) | fail
test_command     = "npm test -- roku"
```

Dependency state falls straight out of comparing the two parts:

- **generation provenance absent** → *pending* (added but never synced).
- **`generated_commit` != `commit`** → *upgrade pending* (`sync` will migrate
  from `generated_commit` → `commit`).
- **`generated_commit` == `commit`**, code present, fixtures pass → *up to date*
  (`sync` skips it — no agent call).

`version` and `commit` are not redundant: `version` is the human-facing semver,
`commit` is the immutable SHA (tags can move; the SHA is the real pin). Generated
**code is checked in** (it is your source); local edits to it are expected and
preserved across upgrades.

## Commands (uv-style)

CLI = deterministic and fast; agent = the slow, interactive LLM work. `add` and
`lock` never touch the agent; `sync` is the only command that generates code.

| Command | CLI (deterministic, fast) | Agent (interactive, slow) |
|---|---|---|
| `speclib init [--agent claude]` | scaffold `speclib.toml`; drop agent skill/command files | — |
| `speclib add <src>[@<ver>] --path <dir> [--lang L] [--context F]` | add manifest entry, resolve, write lockfile resolution. **Never generates** — prints the `sync` next-step. | — |
| `speclib remove <dep>` | drop from manifest + lockfile (optionally delete generated dir) | — |
| `speclib lock [--upgrade \| --upgrade-package <dep>]` | resolve constraints to exact versions; write lockfile resolution. **No code changes.** | — |
| `speclib sync [<dep>] [--dry-run]` | **no arg → all deps that need it** (pending + upgrade-pending), **`<dep>` → just that one**. Drives generation/migration **one dep at a time, sequentially, resumably**; records provenance per dep as it finishes. `--dry-run` prints the plan (no agent). | generate / migrate interactively, loop until fixtures green |
| `speclib verify [dep]` | re-run each dep's `test_command`; check pins/hashes | — |
| `speclib status` | show deps and their state (**pending / upgrade-pending / up-to-date**), locked versions, whether newer tags exist, fixture status | — |

Author-side (v1, share the same format/core):

| Command | Does |
|---|---|
| `speclib new <name>` | scaffold a spec-library repo (manifest, PROMPT/SPEC/fixtures stubs, CHANGELOG) |
| `speclib lint` | validate the library manifest and file references; sanity-check fixtures |
| `speclib release <version>` | lint, then create/confirm the git tag `v<version>` |

There is deliberately **no `bump` command**. Upgrading is uv-shaped:
`speclib lock --upgrade-package roku-deeplink` moves the resolution, then
`speclib sync` performs the spec-diff migration.

### How the CLI reaches the agent

Generation is an **interactive, potentially hours-long** session in which the
agent may ask *you* clarifying questions ("your `Device` type has no `serial`
field — derive it from `id`?"). That shapes the two backends:

- **In-session (primary).** Inside a running Claude Code session, the installed
  skill *drives* generation and migration — asking you questions, writing code,
  running fixtures, looping — and calls `speclib` subcommands for plumbing
  (`speclib sync --plan` to learn what's pending + get spec files/diff/context;
  record provenance when a dep finishes). This is the real path for anything
  non-trivial.
- **Headless (secondary).** Outside a session, the CLI can shell out to the
  agent's headless mode (e.g. `claude -p`). Suitable only for **simple specs
  that need no clarification** and for CI regen — it can't comfortably conduct an
  interactive Q&A.

The CLI→agent contract for one dep: `{ task: generate | migrate, spec files (or
spec diff + current code), target: {language, path}, context: repo agent files +
optional per-dep context file }` → agent returns written files + the
`test_command` to record.

## Performance & dev ergonomics

Generation is the one expensive, human-in-the-loop operation, so the design
confines it, skips it whenever possible, and makes the unavoidable part
observable and resumable:

- **Confine the cost.** Generation is paid **once, by whoever adds/upgrades a
  dep**; the result is checked in. Clones, CI, and `speclib verify` are
  **100% LLM-free and fast** — they only re-run recorded `test_command`s and
  check hashes. Slowness is an authoring-time cost, never an everyone-every-time
  cost.
- **`add` is instant.** `add` only writes manifest + lockfile resolution and
  tells you to `sync` when ready — it never generates. You batch up several
  `add`s and do the slow work deliberately, later.
- **Incremental `sync`.** Up-to-date deps (`generated_commit == commit`, fixtures
  pass) are skipped with **zero agent calls**. The agent runs only for pending or
  upgrade-pending deps.
- **Sequential, one at a time, resumable.** Because sessions are interactive,
  `sync` (no arg) works through **every** dep that needs it — pending and
  upgrade-pending — **one dep at a time**, pausing between them. `sync <dep>`
  does **only** that dependency. Each finished dep is committed to the lockfile,
  so an interrupted multi-hour `sync` resumes at the next pending dep — completed
  deps are never redone. Within a single dep there is no separate checkpoint: on
  re-run the agent reconciles against whatever partial code already exists on
  disk and continues from there — the partial code *is* the checkpoint.
- **`sync --dry-run`.** Fast, deterministic plan of exactly which deps would
  generate vs. migrate (and the size of each spec diff) — decide before paying.
- **Cheaper inner loop.** Prompt-cache the spec/context across the
  generate→run-fixtures→retry iterations so retries are faster and cheaper.
- **`status` surfaces debt.** Because `add` defers generation, `status` clearly
  flags deps that are *pending* — declared but not yet built.

## Sync & migration

`speclib sync` is the one place code changes, working through deps sequentially:

- **Pending (no generation provenance)** → generate from the spec at the locked
  commit.
- **Upgrade-pending (`generated_commit` != `commit`)** → the CLI computes the
  **spec diff** between `generated_commit` and `commit`, hands the agent the diff
  + your current code, and the agent applies the *minimal* migration — preserving
  local edits. Fixtures gate the result.
- **Up to date** → skip.

This is why upgrades feel like a real package manager rather than
"regenerate-and-pray": the spec diff *is* the changelog, and the migration is a
patch, not a rewrite.

## Scope

**v1 ships all three phases:**

- **P0 — consumer round trip:** format, git/tag/local resolver, manifest +
  lockfile (resolution ⁄ generation split), `init` / `add` / `lock` / `sync`
  (interactive, sequential, resumable) / `verify` / `status`, Claude generate
  skill, fixture-gated generation, context injection. Dogfood against the real
  `roku-deeplink-spec`.
- **P1 — sync-driven migration:** spec-diff engine + migrate skill +
  `lock --upgrade[-package]`.
- **P2 — author tooling:** `new` / `lint` / `release`.

**Non-goals for v1 (future / P3):**

- A hosted **registry** (install-by-name, discovery). Git + local paths only for
  now; the resolver is designed so a named registry can slot in later without
  breaking manifests or lockfiles.
- Agents **beyond Claude Code**. The backend interface exists so others can be
  added later; only the Claude integration is built in v1.
- Byte-identical vendored regeneration. The lockfile records provenance and
  fixture status; it does not cache generated output for byte-restore (and, per
  motivation 3, byte-identical output isn't even the goal across consumers).

## Self-hosting dogfood (post-v1, pinned goal)

Once v1 is built enough, we dogfood speclib on **itself**: author a
spec-library *of speclib* (`PROMPT.md` / `SPEC.md` / a scenario `fixtures/` tree
describing the CLI's behavior — resolution, hashing, spec-diff, manifest/lockfile
semantics, command contracts), then use speclib to generate a **Rust**
reimplementation of the CLI from that spec.

This is the ultimate validation: a spec-driven library regenerating a real,
non-trivial tool in a second language, with the Go implementation's behavior
captured as fixtures the Rust port must pass. It also stress-tests the format on
something far larger than a protocol adapter — and is a primary reason fixtures
must support complex, scenario-based structure rather than flat JSON. Not in v1
scope, but the design must not paint us into a corner that makes it hard.

## Testing strategy

- **CLI unit/integration tests** cover the deterministic core: version
  resolution against fixture git repos, spec hashing, spec-diff computation,
  manifest/lockfile read-modify-write, sync state transitions
  (pending → generated → up-to-date), `verify` running a recorded
  `test_command`. No LLM involved — fully deterministic and CI-safe.
- **Agent-backend tests** stub the generation/migration request/response
  contract so orchestration (loop-until-green, per-dep provenance commit,
  resume-after-interrupt, error paths) is tested without a live model.
- **End-to-end dogfood:** generate and sync the real `roku-deeplink-spec`,
  asserting its fixtures pass. Run manually / behind a flag since it needs a live
  agent.

## Open questions (for the plan, not blockers)

1. Exact headless invocation and how (if at all) it surfaces the agent's
   clarifying questions in a non-interactive context.
2. Per-dependency context: settle the default file location and whether globs of
   local files (for the agent to conform to) are worth supporting beyond a prose
   context file.
3. How `verify` / `status` should treat intentional local edits to generated
   code (report informationally vs. warn) — drift is allowed by design, but we
   may want to surface it.
