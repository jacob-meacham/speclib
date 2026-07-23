# Your first spec-library

This tutorial walks the entire speclib loop with a deliberately tiny example:
a `slugify` function. You will author a spec-library, release it, consume it
from a project, generate an implementation, verify it LLM-free, and take an
upgrade. Every command and its output below was captured from a real run.

**Prerequisites:** `speclib` [installed](../../README.md#install), `git`, and —
only for the generation step — a coding agent (Claude Code is the first-class
integration).

## Part 1 — Author the spec-library

A spec-library is a git repo that ships a specification, a prompt, and test
fixtures instead of code. Scaffold one:

```
$ speclib new slugify-spec
Scaffolded spec-library "slugify-spec" in ./slugify-spec

Next steps:
  cd slugify-spec
  edit SPEC.md, PROMPT.md, and test_fixtures.json
  speclib lint
  speclib release 0.1.0
```

Fill in the three files. The spec is behavioral — algorithm and examples, no
implementation:

`SPEC.md`:

```markdown
# slugify Specification

Turn an arbitrary string into a URL-safe slug.

## 1. Problem Statement

One pure function. Given the same input, every implementation must produce
an identical output string.

`slugify(input: string) -> string`

## 2. Algorithm

1. Lowercase the input (ASCII case folding).
2. Replace each run of spaces, tabs, underscores, or hyphens with a single
   hyphen (`-`).
3. Remove every character that is not `a-z`, `0-9`, or `-`.
4. Collapse any remaining runs of consecutive hyphens into one.
5. Trim leading and trailing hyphens.

## 3. Examples

- `slugify("Hello, World!")` -> `"hello-world"`
- `slugify("  Already--Slugged  ")` -> `"already-slugged"`
- `slugify("Émigré café")` -> `"migr-caf"` (non-ASCII letters are removed)
- `slugify("___")` -> `""`

## 4. Test Fixtures

`test_fixtures.json` has one key, `slugify`; each case is
`{ "input": string, "expected": string }`. Your implementation must
reproduce every `expected` exactly.
```

`PROMPT.md` is the agent's entry point — keep it short and point at the spec:

```markdown
# slugify: Implementation Prompt

You are implementing **slugify** — a URL-safe slug function. The complete
specification is in SPEC.md in this directory.

Implement the single pure function from SPEC.md's contract in the consumer's
requested language, following the algorithm exactly. Write a test that loads
`test_fixtures.json` and asserts your implementation reproduces every
`expected` value exactly.
```

`test_fixtures.json` is the contract every implementation must reproduce.
Cover the edge cases your algorithm section implies:

```json
{
  "slugify": [
    { "input": "Hello, World!", "expected": "hello-world" },
    { "input": "  Already--Slugged  ", "expected": "already-slugged" },
    { "input": "snake_case_name", "expected": "snake-case-name" },
    { "input": "Émigré café", "expected": "migr-caf" },
    { "input": "100% Pure Go", "expected": "100-pure-go" },
    { "input": "___", "expected": "" },
    { "input": "", "expected": "" }
  ]
}
```

Fill in `summary` in `speclib.toml`, then lint, commit, and release:

```
$ speclib lint
ok: slugify-spec
$ git add -A && git commit -m "slugify spec v0.1.0"
$ speclib release 0.1.0
Released slugify-spec 0.1.0 as tag v0.1.0
```

The git tag **is** the version — it is not duplicated anywhere in the repo.

## Part 2 — Consume it

In your application repo:

```
$ speclib init --agent claude
Initialized speclib. Add a dependency with `speclib add`.
```

This writes `speclib.toml` and installs the `speclib-sync` skill at
`.claude/skills/speclib-sync/SKILL.md`. While you're in `speclib.toml`,
declare your project's check commands — the sync agent must run these clean
before it records a generation:

```toml
[project]
language = "python"
checks = ["ruff check .", "ruff format --check ."]
```

Add the dependency (any git URL works; a local path is fine for this
tutorial):

```
$ speclib add ../slugify-spec@0.1.0 --path src/slugify --lang python
Added slugify-spec@0.1.0. Generate it with `speclib sync slugify-spec`.
$ speclib status
NAME          VERSION  STATE    FIXTURES  CODE
slugify-spec  0.1.0    pending  -         -
```

`add` resolves the version to a pinned commit in `speclib.lock` and hashes the
spec content — fast, deterministic, no LLM. Generation is a separate step:

```
$ speclib sync --plan
slugify-spec -> src/slugify (python); spec in .speclib/work/slugify-spec
```

Now generate. Inside Claude Code, say **"sync my speclib dependencies"** — the
installed skill reads the plan, generates the implementation into
`src/slugify/` adapted to your project's conventions, runs your declared
`checks` until clean, writes a fixture-driven test and runs it until it
passes, then records provenance. For slugify the generated code looks like:

```python
# Generated from slugify-spec v0.1.0 by speclib.
import re


def slugify(text: str) -> str:
    text = text.lower()
    text = re.sub(r"[ \t_-]+", "-", text)
    text = re.sub(r"[^a-z0-9-]", "", text)
    text = re.sub(r"-{2,}", "-", text)
    return text.strip("-")
```

plus a `test_slugify.py` that loads the fixtures and asserts every expected
value. The recording step the agent runs is plain CLI plumbing:

```
$ speclib sync --record slugify-spec \
    --test-command "cd src/slugify && python3 test_slugify.py" \
    --fixture-status pass
Recorded slugify-spec.
```

Generated code is checked in — commit `src/slugify/`, `speclib.toml`, and
`speclib.lock` together.

## Part 3 — Verify, forever, without an LLM

The lockfile now remembers how to prove the implementation still honors the
spec. `verify` re-runs the recorded fixture test — in CI, on a teammate's
machine, anywhere:

```
$ speclib verify
PASS slugify-spec
$ speclib status
NAME          VERSION  STATE       FIXTURES  CODE
slugify-spec  0.1.0    up-to-date  pass      clean
```

`CODE clean` means the generated files still match the fingerprint recorded at
generation time — local edits would show as drift.

## Part 4 — Take an upgrade

The spec author adds a rule (say, capping slugs at 80 characters) and cuts
`0.2.0`. In the consumer, a plain `update` respects your semver constraint —
`add foo@0.1.0` pinned `^0.1.0`, and for `0.x` versions caret treats a minor
bump as breaking:

```
$ speclib update
slugify-spec: already up to date (0.1.0)
```

Take the breaking upgrade deliberately:

```
$ speclib update slugify-spec --to 0.2.0
slugify-spec: 0.1.0 -> 0.2.0 (run 'speclib sync')
$ speclib status
NAME          VERSION  STATE            FIXTURES  CODE
slugify-spec  0.2.0    upgrade-pending  pass      clean
```

`sync --plan` now materializes a `SPEC.diff` alongside the full spec, so the
agent migrates your existing code with the minimal change instead of
regenerating from scratch — preserving any local adaptations:

```
$ speclib sync --plan
slugify-spec -> src/slugify (python); spec in .speclib/work/slugify-spec
$ head .speclib/work/slugify-spec/SPEC.diff
diff --git a/SPEC.md b/SPEC.md
...
 5. Trim leading and trailing hyphens.
+6. Truncate the result to at most 80 characters, then trim trailing hyphens again.
```

Run the sync in your agent again, and `verify` keeps guarding the result.

## Where to go next

- The full design and rationale: [`docs/specs/2026-07-16-speclib-design.md`](../specs/2026-07-16-speclib-design.md)
- A real spec-library extracted from this repo's own core algorithms:
  the `speclib-core` pattern (four pure functions + byte-exact fixtures) shows
  what a production-grade spec looks like.
