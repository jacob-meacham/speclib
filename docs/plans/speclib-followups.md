# speclib — deferred follow-ups (from P0 reviews)

Captured during the P0 subagent-driven build. None block P0 merge (final
whole-branch review verdict: READY TO MERGE). Ordered by priority for P1.

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

## Design deviations noted (intentional, consistent with the P0 plan)

- `sync --plan` stands in for the design doc's `sync --dry-run`.
- `verify` runs only the recorded `test_command`; the design's "check
  pins/hashes" and `status`'s "whether newer tags exist" are not implemented
  (both need network/hash work — deferred).
