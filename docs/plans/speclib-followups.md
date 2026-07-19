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

## Self-hosting dogfood — SHIPPED (2026-07-17)

Proved the whole thesis on speclib itself: authored a spec-library **of
speclib's own core** and regenerated it in a second language, fixture-validated.

- **`speclib-core`** (`/code/jacob/speclib-core`, local git, tag **v0.1.0**) —
  a spec of speclib's four deterministic pure functions (`spec_hash`,
  `select_version`, `package_state`, `compute_plan`). `test_fixtures.json` (22
  cases) was **generated from the Go reference** (a throwaway generator that
  called the real `spec.Hash` / `source.PickVersion` / `lockfile.Package.State`
  / `syncplan.Compute`, then deleted — the fixtures are the durable artifact).
  Authored, `speclib lint`-clean, and `speclib release 0.1.0`-tagged entirely
  through the CLI (dogfooding the author tooling).
- **`speclib-core-rs`** (`/code/jacob/speclib-core-rs`, commit `a6abd95`) — a
  Rust port generated via `speclib add`/`sync` against the spec, validated with
  `cargo test` against the **byte-identical** Go fixtures: all 22 cases pass,
  incl. `spec_hash` reproducing `sha256:d89f4179…` exactly. `clippy -D warnings`
  clean; `speclib verify → PASS`; `status → up-to-date pass clean`.
- **Learning — a spec must pin cross-language semantics that libraries disagree
  on.** Go's Masterminds semver treats a bare `"1.4.0"` as **exact**; Rust's
  `semver` crate treats it as **caret**. The fixtures encode Go's behavior, so
  SPEC §3.2 had to state "bare = exact (use `=1.4.0` in Rust)" explicitly, and
  the Rust impl rewrites leading-digit constraints to `=`. Same class of thing
  as the byte-exact hash encoding: **behavioral fixtures catch dialect drift the
  prose would otherwise paper over.** This is the strongest argument yet for
  fixtures-as-contract over signature conformance.

## External adoption review — the liveness/provenance gap (2026-07-19)

An independent review of the three roku-deeplink adoptions (graded against a
code constitution) found the sharpest failure mode of the whole exercise.
**Generation was correct everywhere** — every regex/channel-ID/delay
byte-faithful, all three fixture suites pass. **Wiring was not.** In 2 of 3
repos the spec-tracked artifact is not the code that runs (dead code):

- **blockbuster (A−)** — model adoption; both generated functions live on the
  runtime path (the catalog collapsed five hand-written channel impls into one
  data table). Only smell: `RokuEcp.kt:11` banner still says `v1.1.0` while
  `speclib.lock` pins `1.2.0` (verified) + a `!!` in the fixture test.
- **roku-mcp (C+)** — Function 1 live + excellent regression tests for the two
  reconciled regex drifts; but the entire Function-2 surface
  (`build_playback_command` + dataclasses) has **zero non-test callers**
  (verified) — the live client hand-rolls launch→sleep→keypress, so the delay
  exists twice and the fixture-tested copy is dead. Recorded `fixture_status =
  pass` while skipping the Emby fixtures.
- **cueso (C−)** — `deeplink.py` has **zero importers** (verified); the live
  path `streaming.py` re-implements the same spec regexes with
  `# Per roku-deeplink-spec:` comments as the only sync and had **drifted past
  the spec** (Hulu 2285, Apple TV+ 551012 — verified). speclib was tracking the
  dead copy while the live copy drifted untracked.

**The defect, precisely:** speclib verifies "does the generated code match the
spec?" but never "is the generated code the code that runs?" Its value over
"paste the spec into your agent" is provenance/drift-tracking; a tracked-but-dead
artifact makes `fixture_status = pass` a claim about a *file*, not the repo's
behavior. The pattern that predicts the grade: **generation-as-replacement wires
itself (blockbuster); generation-alongside-an-incumbent needs an explicit
integrate-and-decommission step** — which neither the tool nor the driver
enforced.

**Decision — fix in generation discipline, not a tool gate.** A `verify`
caller-grep is a blunt instrument, and retrofitting a library over existing
hand-written code is an unusual migration that inherently needs a decommission
step. So the fix lives in the sync skill, not the CLI:

- **SHIPPED** (`internal/scaffold/templates/sync-instructions.md`): a **Retrofit
  rule** (find the live implementation, reconcile *it* against the spec, make it
  the tracked target, delete the duplicate — "the generated code must be the
  code that runs"), a **liveness confirmation** in the verify step (the
  generated code must be reachable outside tests), **honest `fixture_status`**
  (`skip`, never `pass`, when any fixture is excluded), and **provenance-header
  ownership** (stamp/refresh the resolved version so the file never disagrees
  with the lock).
- **SHIPPED — spec response:** `roku-deeplink` **v1.3.0** adds Hulu + Apple TV+,
  conforming the canonical catalog to cueso's *live* `streaming.py` (the truly
  most-exercised path). This makes cueso's live code spec-conformant, so its
  reconciliation is now "track `streaming.py`, delete dead `deeplink.py`" rather
  than a wire-the-dead-module contortion.
- **SHIPPED — consumer reconciliations** (all upgraded to v1.3.0, each on its
  repo's `speclib-roku-deeplink` branch, verified):
  - **blockbuster** (`179c3c9`) — data-driven catalog gained Hulu/Apple TV+;
    banner fixed `v1.1.0`→`v1.3.0`; `!!` removed (`requireNotNull`). Generated
    core already live. `fixture_status=pass` (full catalog incl. Emby; all 35
    fixtures run). 294 tests + detekt green.
  - **roku-mcp** (`db27d26`) — Function 2 was dead; **wired** it into the live
    play path (`ecp_client.launch_with_deeplink → execute_playback_command(
    build_playback_command(...))`), removing the duplicated 2000ms delay so the
    generated code now runs. Function 1 resynced (Hulu/Apple TV+). Honest
    `fixture_status=skip` (no Emby). Recorded test scoped to the deeplink tests
    so `verify` isn't tripped by a pre-existing `test_config` env failure.
  - **cueso** (`a2c4a1e`) — **retargeted** speclib from the dead `deeplink.py`
    to the live `streaming.py` (Function 1 `match_url_full`) + `search_and_play.py`
    (Function 2 `launch_on_roku`, executed inline); **deleted** `deeplink.py`
    (171 lines) + `test_deeplink.py` (372). New fixture test validates the live
    path via a documented internal-name→spec-channel adapter. Honest
    `fixture_status=skip` (no Emby). 227→200 tests green (−61 dead, +34 live).
  All three now satisfy the retrofit rule: the tracked code is the code that
  runs, and the lock's status is honest. (Note: cueso's `config.yml` gates Hulu
  off at runtime — a feature-flag selection, not dead code; the tracked code
  supports all six channels.)
- **DEFERRED (optional tool hardening):** `sync --record` could refuse `pass`
  when fewer fixtures ran than the library ships; a liveness *heuristic* in
  `verify` remains possible but is deprioritized in favor of the discipline fix.

## Design deviations noted (intentional, consistent with the P0 plan)

- `sync --plan` stands in for the design doc's `sync --dry-run`.
- `verify` runs only the recorded `test_command`; the design's "check
  pins/hashes" and `status`'s "whether newer tags exist" are not implemented
  (both need network/hash work — deferred).
