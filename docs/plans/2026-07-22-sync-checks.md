# Declared Advisory Sync Checks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consumer projects declare build/lint/format-check commands in `speclib.toml` `[project].checks`; `sync --plan` surfaces them verbatim to the generating agent, which must run them clean before the fixture test. The CLI never executes them.

**Architecture:** One field flows through the existing pipeline: `manifest.Project.Checks` → `syncplan.Materialize` (new parameter) → `syncplan.Item.Checks` → plan JSON / `agent.Request.Checks` → headless prompt. The sync skill template turns the list into an explicit gate sequence. No lockfile changes; spec is `docs/specs/2026-07-22-sync-checks-design.md`.

**Tech Stack:** Go, cobra, `pelletier/go-toml/v2`, `stretchr/testify` (`require`), `text/template` + `embed` for scaffolding.

## Global Constraints

- Every commit passes: `gofmt -l .` (no output), `go vet ./...`, `go build ./...`, `go test ./...`.
- Exact terminal assertions (`require.Equal` on full values), never bare `NotEmpty`/`Contains` as the only check on a success path.
- No dead code: every field/function added here is wired to a runtime caller within this plan.
- `checks` is advisory: no task adds CLI execution of check commands, no `speclib.lock` field, no `verify` integration (spec "Out of scope").
- The literal phrase `run the project's build/compile` must survive in `sync-instructions.md` — three existing scaffold tests assert it (`internal/scaffold/scaffold_test.go:23,38,53`).
- Work on branch `speclib-update-command` (the active branch; the spec is committed there).

---

### Task 1: `manifest.Project.Checks`

**Files:**
- Modify: `internal/manifest/manifest.go:19-21` (the `Project` struct)
- Test: `internal/manifest/manifest_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `manifest.Project.Checks []string` (toml tag `checks,omitempty`) — Tasks 2–3 read `m.Project.Checks`.

- [ ] **Step 1: Write the failing test**

Add to `internal/manifest/manifest_test.go` (add `"os"` to the imports):

```go
func TestChecksRoundTripAndDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "speclib.toml")

	m := &Manifest{
		Project: Project{Language: "rust", Checks: []string{
			"cargo build",
			"cargo clippy --all-targets -- -D warnings",
			"cargo fmt --check",
		}},
		Dependencies: map[string]Dependency{},
	}
	require.NoError(t, m.Save(path))

	got, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, []string{
		"cargo build",
		"cargo clippy --all-targets -- -D warnings",
		"cargo fmt --check",
	}, got.Project.Checks)

	// A manifest that never mentions checks loads with none.
	bare := filepath.Join(dir, "bare.toml")
	require.NoError(t, os.WriteFile(bare, []byte("[project]\nlanguage = \"go\"\n"), 0o644))
	got2, err := Load(bare)
	require.NoError(t, err)
	require.Empty(t, got2.Project.Checks)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/manifest/ -run TestChecksRoundTripAndDefault`
Expected: FAIL to compile with `unknown field Checks in struct literal of type Project`.

- [ ] **Step 3: Write minimal implementation**

In `internal/manifest/manifest.go`, change the `Project` struct to:

```go
type Project struct {
	Language string   `toml:"language,omitempty"`
	Checks   []string `toml:"checks,omitempty"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/manifest/`
Expected: PASS (all tests, including the pre-existing ones).

- [ ] **Step 5: Commit**

```bash
git add internal/manifest/manifest.go internal/manifest/manifest_test.go
git commit -m "feat: declare advisory check commands in [project].checks"
```

---

### Task 2: `syncplan.Item.Checks` via a `Materialize` parameter

**Files:**
- Modify: `internal/syncplan/syncplan.go:15-28` (`Item` struct), `:55` (`Materialize` signature), `:87-94` (Item literal)
- Modify: `cmd/sync.go:93` (`runPlan` call site), `cmd/sync.go:166` (`runHeadless` call site)
- Test: `internal/syncplan/syncplan_test.go` (three existing `Materialize` call sites)

**Interfaces:**
- Consumes: `m.Project.Checks` from Task 1.
- Produces: `Materialize(workRoot string, dep manifest.Dependency, pkg lockfile.Package, checks []string) (Item, error)` and `Item.Checks []string` (json tag `checks,omitempty`) — Task 3 asserts the JSON surface, Task 4 reads `item.Checks`.

- [ ] **Step 1: Update tests to the new signature and behavior (failing first)**

In `internal/syncplan/syncplan_test.go`:

In `TestMaterialize` (line 246), change the call and add an exact assertion after the `ContextFile` check:

```go
	item, err := Materialize(work, dep, pkg, []string{"go build ./...", "go vet ./..."})
	require.NoError(t, err)
	// ... existing assertions unchanged ...
	require.Equal(t, []string{"go build ./...", "go vet ./..."}, item.Checks)
```

In `TestMaterializeUpgradePending` (line 146), pass `nil` and assert emptiness with the other item assertions:

```go
	item, err := Materialize(work, dep, pkg, nil)
	...
	require.Empty(t, item.Checks)
```

In `TestMaterializeHonorsPinnedCommitAcrossMovedTag` (line 214):

```go
	item, err := Materialize(work, dep, pkg, nil)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/syncplan/`
Expected: FAIL to compile with `too many arguments in call to Materialize`.

- [ ] **Step 3: Implement**

In `internal/syncplan/syncplan.go`, add the field to `Item` after `SpecDir`:

```go
type Item struct {
	Name        string   `json:"name"`
	State       string   `json:"state"`
	TargetPath  string   `json:"target_path"`
	Language    string   `json:"language"`
	ContextFile string   `json:"context_file,omitempty"`
	SpecDir     string   `json:"spec_dir"`
	Checks      []string `json:"checks,omitempty"`
	// upgrade-pending only
	FromCommit   string `json:"from_commit,omitempty"`
	ToVersion    string `json:"to_version,omitempty"`
	ToCommit     string `json:"to_commit,omitempty"`
	Selections   string `json:"selections,omitempty"`
	SpecDiffPath string `json:"spec_diff_path,omitempty"`
}
```

Change the `Materialize` signature and doc comment (the comment gains one sentence):

```go
// Materialize fetches the spec and writes it under workRoot/<name>/ for the
// agent. For an upgrade-pending package it also writes a SPEC.diff of the spec
// files between the generated commit and the newly resolved commit, and records
// the from/to provenance (and prior selections) on the Item. checks is the
// consumer project's declared check commands, copied onto the Item verbatim.
func Materialize(workRoot string, dep manifest.Dependency, pkg lockfile.Package, checks []string) (Item, error) {
```

In the `Item` literal (around line 87), add `Checks: checks,` after `SpecDir: specDir,`.

In `cmd/sync.go`, update both call sites:

`runPlan` (line 93):
```go
		item, err := syncplan.Materialize(paths.WorkDir, m.Dependencies[p.Name], p, m.Project.Checks)
```

`runHeadless` (line 166):
```go
		item, err := syncplan.Materialize(paths.WorkDir, m.Dependencies[p.Name], p, m.Project.Checks)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./... && go test ./internal/syncplan/ ./cmd/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/syncplan/syncplan.go internal/syncplan/syncplan_test.go cmd/sync.go
git commit -m "feat: carry declared checks onto sync plan items"
```

---

### Task 3: lock the `sync --plan --json` checks surface

**Files:**
- Modify: `cmd/sync_test.go:17-30` (`setupPending` gains a variadic checks parameter), `:43-55` (`TestSyncPlanJSON` gains an absent-key assertion)
- Test: `cmd/sync_test.go` (new `TestSyncPlanJSONIncludesChecks`)

**Interfaces:**
- Consumes: `Item.Checks` JSON emission from Task 2; `setupPending` is reused by Task 4's headless test.
- Produces: `setupPending(t *testing.T, dir string, checks ...string)` — existing zero-checks callers compile unchanged.

- [ ] **Step 1: Extend the helper**

Change `setupPending`'s signature and manifest literal in `cmd/sync_test.go`:

```go
func setupPending(t *testing.T, dir string, checks ...string) {
	t.Helper()
	lib := filepath.Join(dir, "lib")
	writeDemoLib(t, lib)
	m := &manifest.Manifest{
		Project:      manifest.Project{Language: "go", Checks: checks},
		Dependencies: map[string]manifest.Dependency{"demo": {Source: lib, Version: "*", Path: "gen/demo"}},
	}
	require.NoError(t, m.Save(filepath.Join(dir, "speclib.toml")))
	l := &lockfile.Lockfile{Packages: []lockfile.Package{
		{Name: "demo", Source: lib, Version: "0.0.0+local", Commit: "local", SpecHash: "sha256:x", Language: "go", Path: "gen/demo"},
	}}
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))
}
```

- [ ] **Step 2: Write the new test and tighten the old one**

Add to `cmd/sync_test.go`:

```go
func TestSyncPlanJSONIncludesChecks(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir, "go build ./...", "go vet ./...")

	out, err := runSyncWithStub(t, dir, "sync", "--plan", "--json")
	require.NoError(t, err)

	var plan struct{ Items []map[string]any }
	require.NoError(t, json.Unmarshal([]byte(out), &plan))
	require.Len(t, plan.Items, 1)
	require.Equal(t, []any{"go build ./...", "go vet ./..."}, plan.Items[0]["checks"])
}
```

In `TestSyncPlanJSON`, after the existing `require.Equal(t, "demo", ...)` line, add:

```go
	_, hasChecks := plan.Items[0]["checks"]
	require.False(t, hasChecks, "checks key must be absent when the manifest declares none")
```

- [ ] **Step 3: Run the tests**

Run: `go test ./cmd/ -run 'TestSyncPlanJSON'`
Expected: PASS — both tests. These lock in behavior produced by Task 2's wiring; if either fails, fix Task 2's wiring before committing (the emission path, not the tests, is wrong).

- [ ] **Step 4: Commit**

```bash
git add cmd/sync_test.go
git commit -m "test: lock the sync --plan JSON checks surface"
```

---

### Task 4: thread checks into the headless backend

**Files:**
- Modify: `internal/agent/backend.go:9-15` (`Request` struct)
- Modify: `internal/agent/headless.go:14-20` (extract `buildPrompt`, include checks)
- Modify: `cmd/sync.go:171-174` (`runHeadless` Request literal)
- Test: `internal/agent/headless_test.go`, `cmd/sync_test.go`

**Interfaces:**
- Consumes: `item.Checks` from Task 2, `setupPending(t, dir, checks...)` from Task 3.
- Produces: `agent.Request.Checks []string`; unexported `buildPrompt(req Request) string` in package `agent`, called by `HeadlessClaude.Generate`.

- [ ] **Step 1: Write the failing prompt tests**

Add to `internal/agent/headless_test.go`:

```go
func TestBuildPromptIncludesChecksInOrder(t *testing.T) {
	p := buildPrompt(Request{
		SpecDir: ".speclib/work/demo", Language: "go", TargetPath: "gen/demo",
		Checks: []string{"go build ./...", "go vet ./..."},
	})
	require.Contains(t, p, "go build ./...; go vet ./...")
	require.Contains(t, p, "exits 0")
	require.Contains(t, p, "RESULT <test-command> || <pass|skip|fail>")
}

func TestBuildPromptOmitsChecksWhenUndeclared(t *testing.T) {
	p := buildPrompt(Request{SpecDir: "s", Language: "go", TargetPath: "t"})
	require.NotContains(t, p, "exits 0")
	require.Contains(t, p, "RESULT <test-command> || <pass|skip|fail>")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/`
Expected: FAIL to compile with `undefined: buildPrompt`.

- [ ] **Step 3: Implement**

In `internal/agent/backend.go`, add the field:

```go
type Request struct {
	Name        string
	TargetPath  string
	Language    string
	ContextFile string
	SpecDir     string
	Checks      []string
}
```

In `internal/agent/headless.go`, replace the inline prompt in `Generate` with a call to a new function, and add the function below `Generate`:

```go
func (HeadlessClaude) Generate(ctx context.Context, req Request) (Result, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p", buildPrompt(req))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Result{}, fmt.Errorf("claude: %v: %s", err, string(out))
	}
	tc, fs := parseResultLine(string(out))
	if tc == "" {
		return Result{}, fmt.Errorf("could not parse RESULT line from agent output")
	}
	return Result{TestCommand: tc, FixtureStatus: fs}, nil
}

// buildPrompt renders the headless generation prompt. Kept separate from
// Generate so the prompt contract is testable without spawning claude.
func buildPrompt(req Request) string {
	checks := ""
	if len(req.Checks) > 0 {
		checks = fmt.Sprintf(" Before writing the test, run each project check"+
			" in order and fix failures until every one exits 0: %s.",
			strings.Join(req.Checks, "; "))
	}
	return fmt.Sprintf(
		"Read the spec in %s (PROMPT.md, SPEC.md, fixtures). Generate a %s "+
			"implementation into %s.%s Write a fixture-driven test, run it until it "+
			"passes, then print a final line exactly: RESULT <test-command> || <pass|skip|fail>.",
		req.SpecDir, req.Language, req.TargetPath, checks)
}
```

In `cmd/sync.go` `runHeadless`, add `Checks: item.Checks,` to the Request literal:

```go
		res, err := backend.Generate(context.Background(), agent.Request{
			Name: item.Name, TargetPath: item.TargetPath, Language: item.Language,
			ContextFile: item.ContextFile, SpecDir: item.SpecDir, Checks: item.Checks,
		})
```

- [ ] **Step 4: Write the failing threading test**

Add to `cmd/sync_test.go` (add `"context"` to the imports):

```go
// recordingBackend captures the Request runHeadless hands to the backend so
// the plan-item -> Request threading is provable without running claude.
type recordingBackend struct {
	got *agent.Request
}

func (r *recordingBackend) Generate(ctx context.Context, req agent.Request) (agent.Result, error) {
	*r.got = req
	return agent.StubBackend{}.Generate(ctx, req)
}

func TestSyncHeadlessPassesChecksToBackend(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir, "go build ./...", "go vet ./...")

	var got agent.Request
	root := newRootCmdWithBackend(&recordingBackend{got: &got})
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--chdir", dir, "sync"})
	require.NoError(t, root.Execute())

	require.Equal(t, []string{"go build ./...", "go vet ./..."}, got.Checks)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go build ./... && go test ./internal/agent/ ./cmd/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/backend.go internal/agent/headless.go internal/agent/headless_test.go cmd/sync.go cmd/sync_test.go
git commit -m "feat: thread declared checks into the headless generation prompt"
```

---

### Task 5: templates — scaffold example and skill gate sequence

**Files:**
- Modify: `internal/scaffold/templates/speclib.toml.tmpl`
- Modify: `internal/scaffold/templates/sync-instructions.md` (step 2d and step 3)
- Test: `internal/scaffold/scaffold_test.go`

**Interfaces:**
- Consumes: nothing from other tasks (templates are static content embedded via `//go:embed`).
- Produces: the scaffolded `speclib.toml` advertises `checks`; the sync skill mandates the gate sequence.

- [ ] **Step 1: Write the failing tests**

Add to `internal/scaffold/scaffold_test.go`:

```go
func TestWriteManifestIncludesChecksExample(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, WriteManifest(dir))

	data, err := os.ReadFile(filepath.Join(dir, "speclib.toml"))
	require.NoError(t, err)
	require.Contains(t, string(data), `# checks = ["go build ./...", "golangci-lint run"]`)
}
```

In `TestWriteAgentClaude`, after the existing `require.Contains(t, body, "linter")` line, add:

```go
	require.Contains(t, body, "`checks`")
	require.Contains(t, body, "exits 0")
	require.Contains(t, body, "if a check failed or was skipped")
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/`
Expected: FAIL — `TestWriteManifestIncludesChecksExample` and `TestWriteAgentClaude` both fail on the missing content.

- [ ] **Step 3: Update the manifest template**

Replace the full contents of `internal/scaffold/templates/speclib.toml.tmpl` with:

```toml
[project]
# Default target language for generated dependencies.
language = ""

# Commands the sync agent must run clean (exit 0) before recording a sync —
# build, lint, format-check. Run by the agent during sync, never by the CLI.
# checks = ["go build ./...", "golangci-lint run"]

# Declare spec-library dependencies with `speclib add <source> --path <dir>`,
# then generate their code with `speclib sync`.
[dependencies]
```

- [ ] **Step 4: Update the sync instructions**

In `internal/scaffold/templates/sync-instructions.md`, replace step 2d's first paragraph. Old text:

```
   d. Verify cheaply first, then thoroughly: run the project's build/compile
      step and its linter, and fix any errors they report — they're faster to
      catch than fixture failures. Only once those are clean, write a test in
      `language` that exercises the fixtures, and run it. Fix and repeat until
      the fixture test passes. If the library ships no fixtures, note that.
```

New text (the trailing "Then confirm the code you generated…" paragraph of 2d stays as is):

```
   d. Verify cheaply first, then thoroughly. If the plan item has a `checks`
      list, run each command in order and fix any failures until every one
      exits 0 — these are the project's declared gates (build, lint,
      format-check), and all of them must be clean before you write the
      fixture test. If the item has no `checks`, run the project's
      build/compile step and its linter, and fix any errors they report —
      they're faster to catch than fixture failures. Either way, a check you
      cannot run (missing tool, broken command) is a step-2e situation: ask
      the user, never silently skip it. Only once those are clean, write a
      test in `language` that exercises the fixtures, and run it. Fix and
      repeat until the fixture test passes. If the library ships no fixtures,
      note that.
```

Replace step 3. Old text:

```
3. Stop after the last item. Tell the user what was generated and how to
   `speclib verify`.
```

New text:

```
3. Stop after the last item. Tell the user what was generated and how to
   `speclib verify`, and list each check command you ran with its result.
   Never describe a sync as clean if a check failed or was skipped.
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/scaffold/ ./cmd/`
Expected: PASS — including the three pre-existing tests asserting `run the project's build/compile`, which the fallback sentence preserves.

- [ ] **Step 6: Full-suite verification**

Run: `gofmt -l . && go vet ./... && go build ./... && go test ./...`
Expected: `gofmt` prints nothing; everything else passes.

- [ ] **Step 7: Commit**

```bash
git add internal/scaffold/templates/speclib.toml.tmpl internal/scaffold/templates/sync-instructions.md internal/scaffold/scaffold_test.go
git commit -m "feat: advertise checks in init scaffold and require them in the sync skill"
```
