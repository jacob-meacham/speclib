# speclib — deferred follow-ups (from P0 reviews)

Captured during the P0 subagent-driven build. None block P0 merge (final
whole-branch review verdict: READY TO MERGE). Ordered by priority for P1.

## Shipped — P0/P1 hardening (branch `speclib-update-command`, 2026-07-17)

- **git-clone injection hardening** + **cache concurrency lock** (`flock`) +
  **fetch-once per resolve** — item 1 + the concurrency item + item 11 (`aa80e07`).
- **Orphan lockfile entry** no longer crashes `sync`; **`verify`/`sync
  <unknown-dep>`** now error; **fixture-path traversal guard**;
  **`parseResultLine`** splits on the last `||`; removed dead `ErrNoWork`;
  `sync` `--plan`/`--record` mutually exclusive; lockfile godoc + `Materialize`
  error wrapping — items 3, 4, 5, 6, 7, 8, 10 (`86a6cc2`).
- **Materialize at the pinned commit** (not the re-resolved tag) — item 2
  (`47c31b7`), proven with a moved-tag test.
- **Upgrade workflow** (`update` + upgrade-aware `sync` + `SPEC.diff`) and
  **selections serialization** — the P1 core (`a9f2b08`).

**Still open:** `--chdir` global-state hazard (deferred — latent-only, needs a
working-dir refactor); drift-fingerprint / staged compile-gate (P1 enhancements,
deferred); item 9 `sort.SliceStable` (cosmetic, unreachable); item 12 Linux-only
test sandbox (accepted).

## P1 priority

1. **Harden `git clone` of user-declared sources** (`internal/source/git.go`,
   `ensureMirror`). `git clone --mirror <location>` passes the source verbatim:
   `ext::sh -c '…'` → RCE, a `-`-prefixed value can inject git flags. Add a URL
   scheme allow-list, pass `--` before the URL, and set
   `-c protocol.ext.allow=never`. Author-time + user-chosen source makes this
   acceptable for P0, but it's the top hardening item for a package manager.

2. **Materialize at the pinned commit, not the tag** (`internal/syncplan`
   `Materialize` → `source.Acquire(..., pkg.Version)`). Generation resolves the
   spec by tag `v<version>` and never uses `pkg.Commit`, though the design names
   the commit SHA the immutable pin. If a tag force-moves between `lock` and
   `sync`, the agent generates against different content than the lockfile
   records. Acquire by `pkg.Commit` instead.

## Correctness / robustness (low priority)

3. **Orphan lockfile entry crashes `sync`** — a package in the lockfile but not
   the manifest makes `m.Dependencies[name]` a zero `Dependency{}`, so
   `Acquire(Parse(""), …)` tries `git clone ""`. Only reachable via hand-edited
   state (normal `add`/`remove` keep them in sync). Have `syncplan.Compute`
   cross-check its `m` param (currently unused) and skip/warn on orphans.
4. **`parseResultLine` greedy `" || "` split** (`internal/agent/headless.go`) —
   a recorded command containing `||` folds its tail into `fixture_status`.
   Dogfood-only path.
5. **`Materialize` fixture-path traversal guard** — local library whose
   `[files].fixtures` escapes root (`../..`) writes outside `.speclib/work`.
   Add a `filepath.Rel` containment check. (Git sources are protected by
   `ls-tree`.)
6. **`verify <unknown-dep>` / `sync <unknown-dep>` silently succeed** — return
   an error on an unmatched dependency name instead of a no-op success.

## Cleanup / cosmetic

7. Remove unused exported `syncplan.ErrNoWork` (dead code).
8. `sync` modes: add `MarkFlagsMutuallyExclusive("plan","record")` for CLI UX
   (precedence is currently safe: record > plan > headless).
9. `spec.Hash` uses `sort.Slice` (unstable) — unreachable today (fs walk /
   ls-tree can't yield duplicate paths); switch to `sort.SliceStable` if fixture
   paths ever come from a less-strict source.
10. Godoc on `internal/lockfile` exported API; consistent error wrapping in
    `assembleSpec` / `Materialize`; `Generator` label is hardcoded
    `"claude-code"` even on the stub path.
11. Per-resolve git overhead: `gitTags`/`gitResolveCommit`/`gitReadFile`/
    `gitListFiles` each call `ensureMirror` → up to 4 `git fetch` per resolve.
    Fetch once per resolve.
12. `XDG_CACHE_HOME` test-cache sandbox is Linux-only (fine — project targets
    Linux; revisit if macOS/Windows support is added).

## Ideas from PDD / promptdriven.ai (triaged 2026-07-17)

Comparison notes: PDD applies "regenerate-not-patch" to a single codebase
(prompts are project-local source, no versioning/registry/cross-project reuse);
speclib applies it at the dependency layer (a library maintained once, consumed
and context-adapted everywhere). Ideas worth pursuing:

- **Staged verification in `sync` (PDD's `crash` → `verify` → `test` → `fix`).**
  Add a **compile/build gate** before the fixture gate — for generation into a
  real project, "does it compile + pass the project's build/lint" is a cheaper,
  earlier check than fixtures. (Already reflected in the blockbuster Phase-2
  plan: gate on `./gradlew` build + detekt before/with the fixture test.)
  Optional: generate a runnable **usage example** per dependency (docs +
  smoke test); the spec's worked examples are a natural source.
- **Finer-grained fingerprinting + local-drift detection** (PDD skips per
  sub-step by fingerprint). Add a **generated-code fingerprint** to the lockfile
  so `sync`/`verify` can detect local edits to generated code and re-run only
  what's needed, instead of whole-dep up-to-date/skip.
- **Batch / background non-interactive mode.** For the non-interactive,
  fixture-gated regeneration path, make the headless backend first-class:
  "define, launch, walk away," optionally via LLM batch APIs for cost. Aligns
  with the existing resumable, one-at-a-time headless design.

Considered and NOT pursued:

- **Interface/signature conformance gate** (PDD's `architecture.json`
  conformance) — parked. speclib's context-adaptation means the generated
  *shape* is intentionally flexible; the behavioral contract (fixtures) is the
  right invariant, not the syntactic signature. Revisit only if a spec wants an
  opt-in exact-signature mode.
- **Back-propagation (`update`)** — declined. Generated code is checked in and
  may be hand-edited, but edits are not fed back into the (often third-party)
  spec.
- **Auto-deps context discovery** — belongs in the agent generation prompt
  (auto-gather the consumer's relevant types/schemas), not in the deterministic
  CLI. Keep the clean seam; make it a step in the generate skill.

## From the multi-consumer rollout (2026-07-17)

Dogfooded roku-deeplink across three real consumers (blockbuster/Kotlin,
cueso/Python, roku-mcp/Python). Learnings:

- **`update` command — SHIPPED** (branch `speclib-update-command`). uv-inspired
  re-resolve: `speclib update [<dep>] [--to <version>]` moves the lockfile
  resolution ahead (→ `upgrade-pending`); `speclib sync` then regenerates,
  materializing the new spec **plus** a computed `SPEC.diff` (git diff of the
  spec files old→new) and the from/to commits so the agent reconciles against
  on-disk code. Closes the "no upgrade command" P0 gap.
- **Selections serialization — SHIPPED.** `sync --record --selections "…"`
  persists generation choices (e.g. channel subset) into the lockfile
  provenance; `sync --plan` feeds them back on every regenerate so upgrades
  honor the same choices. Baking the choice into the PROMPT (the agent picks)
  is fine — this just serializes the result.
- **Git mirror cache is not concurrency-safe — OPEN (elevate to P0 hardening).**
  The git backend clones each source into `~/.cache/speclib/git/<sha256(source)>`
  (then `git fetch`). Multiple speclib processes resolving the *same* source
  concurrently race on that dir (clone/fetch lock errors) — had to run the three
  consumer subagents sequentially. Fix: a per-cache-dir lock file (`flock`), or a
  per-run temp clone. Needed before any parallel/CI use.
- **`--chdir` global-state hazard — OPEN** (P0-review Important, plan-mandated).
  The root command's `chdir` is a package var + a process-wide `os.Chdir` in
  `PersistentPreRunE` with no restore. Harmless today (every cmd test passes
  `--chdir` to a fresh temp dir), but latent for any `cmd` test doing relative
  I/O or using `t.Parallel()`. Fix: thread a working dir instead of chdir, or
  restore cwd after Execute.
- **Reconciliation direction confirmed:** when consumers drift, the spec should
  conform to the most-exercised consumer (here: `post_launch_key` on the
  extraction result), not the reverse. The `update`/regenerate loop then pulls
  the other consumers back to canonical.

## Design deviations noted (intentional, consistent with the P0 plan)

- `sync --plan` stands in for the design doc's `sync --dry-run`.
- `verify` runs only the recorded `test_command`; the design's "check
  pins/hashes" and `status`'s "whether newer tags exist" are not implemented
  (both need network/hash work — deferred).
