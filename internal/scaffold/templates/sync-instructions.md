# speclib sync

You generate implementation code from spec-driven libraries. The `speclib` CLI
handles all bookkeeping; you do the code generation. Work **one dependency at a
time**.

## Steps

1. Run `speclib sync --plan --json` (or `speclib sync --plan --json <dep>` for a
   single dependency). It prints the work items that need generating.
2. For **each** item, in order:
   a. Read the spec files in `spec_dir` (`PROMPT.md`, `SPEC.md`, and the
      `fixtures/` tree). Follow `PROMPT.md` — it tells you how to interpret the
      spec and fixtures. The materialized spec is always **authoritative**.
   b. Read the consumer repo's `AGENTS.md` / `CLAUDE.md` for local conventions,
      and the per-dependency `context_file` if present, to adapt the code to
      this project's shape (its types, persistence, framework).
   c. Generate the implementation into `target_path` in the item's `language`.
      - `state: "pending"` — first generation. If code already exists there (a
        resumed run), reconcile against it rather than starting over.
      - `state: "upgrade-pending"` — the spec moved from `from_commit` to
        `to_version`/`to_commit`. Read `SPEC.diff` in `spec_dir` to see exactly
        what changed, then reconcile against the **existing code already on
        disk** at `target_path`: apply the minimal change the diff implies and
        **preserve local edits**. Honor the recorded `selections` — the same
        generation choices as last time (e.g. the channel subset) — unless the
        user asks to change them. If `SPEC.diff` is empty, fall back to the full
        `SPEC.md` as the source of truth.
   d. Verify cheaply first, then thoroughly: run the project's build/compile
      step and its linter, and fix any errors they report — they're faster to
      catch than fixture failures. Only once those are clean, write a test in
      `language` that exercises the fixtures, and run it. Fix and repeat until
      the fixture test passes. If the library ships no fixtures, note that.
   e. If the agent needs a decision the spec doesn't cover (e.g. a field the
      local types lack), **ask the user** before guessing.
   f. Record provenance:
      `speclib sync --record <name> --test-command "<cmd>" --fixture-status <pass|skip>`
      Also pass `--selections "<choices>"` capturing any generation choices you
      made (e.g. which channels/options), so future upgrades honor them.
3. Stop after the last item. Tell the user what was generated and how to
   `speclib verify`.

Never edit `speclib.toml` or `speclib.lock` by hand — the CLI owns them.
