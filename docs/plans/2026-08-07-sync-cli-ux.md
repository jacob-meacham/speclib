# sync CLI UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `speclib sync` on a terminal launches the coding agent's own interactive UI preloaded with the sync instructions; `speclib sync --headless` becomes a working non-interactive path with per-adapter permission defaults, streamed progress, a timeout, and a recording gate that re-runs the reported test command before touching the lockfile.

**Architecture:** An adapter table in `internal/agent` maps agent name → binary + default headless permission args (claude only today); a new optional `[agent]` manifest section overrides both. `cmd/sync.go` grows mode selection (TTY → interactive handoff, `--headless` → rewritten print-mode backend, non-TTY without `--headless` → error). The headless backend parses claude's `stream-json` events for live progress and the final `RESULT` line.

**Tech Stack:** Go 1.25, cobra, pelletier/go-toml v2, testify. No new dependencies.

**Spec:** `docs/specs/2026-08-07-sync-cli-ux-design.md` — read it before starting.

## Global Constraints

- No new Go module dependencies; TTY detection uses `os.ModeCharDevice`, not `x/term`.
- The real `claude` binary must NEVER run in tests — tests use scripted fake binaries prepended to `PATH` (constitution rule already noted in `internal/agent/headless.go`).
- All test assertions are exact (constitution §12) — exact argv slices, exact struct equality, exact strings, not just "no error".
- claude adapter defaults, verbatim from the spec: headless args `-p <prompt> --output-format stream-json --verbose --allowedTools Write,Edit,Bash`; interactive argv is just `<instructions>`.
- Default headless timeout: `15m`, flag `--timeout`, per-dependency.
- Non-TTY error text: `not a terminal; pass --headless for non-interactive use`.
- Tests are plain `go test` + testify `require`; run `go build ./... && go vet ./...` before every commit.
- Linux/macOS only (Windows targets were dropped); `sh -c` is available.

---

### Task 1: `[agent]` manifest section + scaffold example

**Files:**
- Modify: `internal/manifest/manifest.go`
- Modify: `internal/scaffold/templates/speclib.toml.tmpl`
- Test: `internal/manifest/manifest_test.go` (create if absent, else extend)
- Test: `internal/scaffold/scaffold_test.go` (extend)

**Interfaces:**
- Consumes: existing `manifest.Manifest`, `Load`, `Save`.
- Produces: `type Agent struct { Command string; Permissions []string }`; `Manifest.Agent *Agent` (nil when the section is absent); `func (m *Manifest) AgentCommand() string` and `func (m *Manifest) AgentPermissions() []string` (nil-safe accessors returning zero values when unset). Tasks 5–6 call the accessors.

- [ ] **Step 1: Write the failing tests**

In `internal/manifest/manifest_test.go`:

```go
func TestManifestAgentRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "speclib.toml")
	m := &Manifest{
		Agent:        &Agent{Command: "claude", Permissions: []string{"--dangerously-skip-permissions"}},
		Dependencies: map[string]Dependency{},
	}
	require.NoError(t, m.Save(path))
	got, err := Load(path)
	require.NoError(t, err)
	require.NotNil(t, got.Agent)
	require.Equal(t, "claude", got.AgentCommand())
	require.Equal(t, []string{"--dangerously-skip-permissions"}, got.AgentPermissions())
}

func TestManifestAgentAbsentYieldsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "speclib.toml")
	require.NoError(t, os.WriteFile(path, []byte("[project]\nlanguage = \"go\"\n"), 0o644))
	m, err := Load(path)
	require.NoError(t, err)
	require.Nil(t, m.Agent)
	require.Equal(t, "", m.AgentCommand())
	require.Nil(t, m.AgentPermissions())
}

func TestManifestSaveWithoutAgentOmitsSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "speclib.toml")
	m := &Manifest{Dependencies: map[string]Dependency{}}
	require.NoError(t, m.Save(path))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(data), "[agent]")
}
```

In `internal/scaffold/scaffold_test.go`:

```go
func TestWriteManifestIncludesAgentExample(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, WriteManifest(dir))
	data, err := os.ReadFile(filepath.Join(dir, "speclib.toml"))
	require.NoError(t, err)
	require.Contains(t, string(data), "# [agent]")
	require.Contains(t, string(data), `# permissions = ["--allowedTools", "Write,Edit,Bash"]`)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/manifest/ ./internal/scaffold/ -run 'Agent' -v`
Expected: FAIL — `undefined: Agent`, missing template content.

- [ ] **Step 3: Implement**

In `internal/manifest/manifest.go`, after the `Project` type:

```go
// Agent configures which coding agent drives generation and, for headless
// sync, the permission args its print mode is launched with. A nil Agent (no
// [agent] section) means the defaults: the claude adapter with its built-in
// permission allowlist.
type Agent struct {
	Command     string   `toml:"command,omitempty"`
	Permissions []string `toml:"permissions,omitempty"`
}
```

Add the field to `Manifest` (pointer so an unset section round-trips as absent, not as an empty `[agent]` table):

```go
type Manifest struct {
	Project      Project               `toml:"project"`
	Agent        *Agent                `toml:"agent,omitempty"`
	Dependencies map[string]Dependency `toml:"dependencies"`
}
```

Add nil-safe accessors after `LanguageFor`:

```go
// AgentCommand returns the configured [agent].command, or "" (the default
// adapter) when the section is absent.
func (m *Manifest) AgentCommand() string {
	if m.Agent == nil {
		return ""
	}
	return m.Agent.Command
}

// AgentPermissions returns the configured [agent].permissions, or nil (use
// the adapter's defaults) when the section is absent.
func (m *Manifest) AgentPermissions() []string {
	if m.Agent == nil {
		return nil
	}
	return m.Agent.Permissions
}
```

Append to `internal/scaffold/templates/speclib.toml.tmpl` (after the `[dependencies]` block, matching the file's commented-example style):

```toml

# Which coding agent drives `speclib sync` and, in --headless mode, the
# permission args its print mode gets. These are the defaults; permissions
# replaces the adapter's default args verbatim (e.g. a tighter allowlist, or
# ["--dangerously-skip-permissions"] inside sandboxed CI).
# [agent]
# command = "claude"
# permissions = ["--allowedTools", "Write,Edit,Bash"]
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/manifest/ ./internal/scaffold/ -v`
Expected: PASS (all, including pre-existing tests).

- [ ] **Step 5: Commit**

```bash
go build ./... && go vet ./...
git add internal/manifest/ internal/scaffold/
git commit -m "feat: [agent] manifest section with nil-safe accessors"
```

---

### Task 2: agent adapter table

**Files:**
- Create: `internal/agent/adapter.go`
- Test: `internal/agent/adapter_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `type Adapter struct { Name, Bin string; DefaultPermissions []string }`; `func Lookup(name string) (Adapter, error)` ("" → claude; unknown → error listing supported); `func (a Adapter) HeadlessArgs(prompt string, permissions []string) []string`; `func (a Adapter) InteractiveArgs(instructions string) []string`. Tasks 3, 5, 6 consume all of these.

- [ ] **Step 1: Write the failing tests**

Create `internal/agent/adapter_test.go`:

```go
package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLookupDefaultsToClaude(t *testing.T) {
	a, err := Lookup("")
	require.NoError(t, err)
	require.Equal(t, "claude", a.Name)
	require.Equal(t, "claude", a.Bin)
	require.Equal(t, []string{"--allowedTools", "Write,Edit,Bash"}, a.DefaultPermissions)
}

func TestLookupUnknownListsSupported(t *testing.T) {
	_, err := Lookup("cursor")
	require.Error(t, err)
	require.Equal(t, `unknown agent "cursor": supported agents are claude`, err.Error())
}

func TestHeadlessArgsWithDefaultPermissions(t *testing.T) {
	a, _ := Lookup("claude")
	require.Equal(t,
		[]string{"-p", "PROMPT", "--output-format", "stream-json", "--verbose", "--allowedTools", "Write,Edit,Bash"},
		a.HeadlessArgs("PROMPT", nil))
}

func TestHeadlessArgsWithOverriddenPermissions(t *testing.T) {
	a, _ := Lookup("claude")
	require.Equal(t,
		[]string{"-p", "PROMPT", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"},
		a.HeadlessArgs("PROMPT", []string{"--dangerously-skip-permissions"}))
}

func TestInteractiveArgs(t *testing.T) {
	a, _ := Lookup("claude")
	require.Equal(t, []string{"INSTRUCTIONS"}, a.InteractiveArgs("INSTRUCTIONS"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run 'Lookup|Args' -v`
Expected: FAIL — `undefined: Lookup`.

- [ ] **Step 3: Implement**

Create `internal/agent/adapter.go`:

```go
package agent

import (
	"fmt"
	"sort"
	"strings"
)

// Adapter describes one supported coding agent: the binary speclib launches
// and the permission args its headless print mode gets by default. The table
// below is the seam for supporting agents beyond claude; the manifest's
// [agent] section selects an adapter and can replace its permission args.
type Adapter struct {
	Name               string
	Bin                string
	DefaultPermissions []string
}

var adapters = map[string]Adapter{
	"claude": {
		Name:               "claude",
		Bin:                "claude",
		DefaultPermissions: []string{"--allowedTools", "Write,Edit,Bash"},
	},
}

// Lookup resolves an [agent].command value to an adapter. An empty name
// selects the default (claude); an unknown name errors, naming the supported
// adapters.
func Lookup(name string) (Adapter, error) {
	if name == "" {
		name = "claude"
	}
	a, ok := adapters[name]
	if !ok {
		names := make([]string, 0, len(adapters))
		for n := range adapters {
			names = append(names, n)
		}
		sort.Strings(names)
		return Adapter{}, fmt.Errorf("unknown agent %q: supported agents are %s", name, strings.Join(names, ", "))
	}
	return a, nil
}

// HeadlessArgs builds the print-mode argv (minus the binary): the generation
// prompt, streamed JSON output, and the permission args — permissions
// verbatim when non-empty (the manifest override), else the adapter's
// defaults.
func (a Adapter) HeadlessArgs(prompt string, permissions []string) []string {
	if len(permissions) == 0 {
		permissions = a.DefaultPermissions
	}
	args := []string{"-p", prompt, "--output-format", "stream-json", "--verbose"}
	return append(args, permissions...)
}

// InteractiveArgs builds the argv that opens the agent's own interactive UI
// with instructions as the initial prompt.
func (a Adapter) InteractiveArgs(instructions string) []string {
	return []string{instructions}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
go build ./... && go vet ./...
git add internal/agent/adapter.go internal/agent/adapter_test.go
git commit -m "feat: agent adapter table with per-adapter permission defaults"
```

---

### Task 3: headless backend rewrite — streamed progress, RESULT from the result event

**Files:**
- Modify: `internal/agent/headless.go` (keep `buildPrompt` and `parseResultLine` unchanged)
- Test: `internal/agent/headless_test.go` (extend; existing tests must keep passing)

**Interfaces:**
- Consumes: `Adapter`, `Lookup`, `HeadlessArgs` from Task 2; existing `Request`, `Result`, `buildPrompt`, `parseResultLine`.
- Produces: `HeadlessClaude` becomes `struct { Adapter Adapter; Permissions []string; Progress io.Writer }` (zero value still works: empty Adapter falls back to claude, nil Progress discards). `Generate(ctx, req)` respects ctx cancellation/timeout. Task 6 constructs it as `agent.HeadlessClaude{Adapter: ad, Permissions: m.AgentPermissions(), Progress: cmd.ErrOrStderr()}`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/agent/headless_test.go`:

```go
// writeFakeClaude installs a scripted `claude` at the front of PATH. The
// real claude must never run in tests; every headless test goes through a
// fake like this.
func writeFakeClaude(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\n"+script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestHeadlessGenerateStreamsProgressAndParsesResult(t *testing.T) {
	writeFakeClaude(t, `
echo '{"type":"system","subtype":"init"}'
echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"gen/demo/demo.go"}}]}}'
echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./gen/demo"}}]}}'
echo '{"type":"result","subtype":"success","result":"done\nRESULT go test ./gen/demo || pass"}'
`)
	var progress strings.Builder
	h := HeadlessClaude{Progress: &progress}
	res, err := h.Generate(context.Background(), Request{SpecDir: "s", Language: "go", TargetPath: "t"})
	require.NoError(t, err)
	require.Equal(t, Result{TestCommand: "go test ./gen/demo", FixtureStatus: "pass"}, res)
	require.Equal(t, "  [tool] Write gen/demo/demo.go\n  [tool] Bash: go test ./gen/demo\n", progress.String())
}

func TestHeadlessGenerateTimesOut(t *testing.T) {
	writeFakeClaude(t, "sleep 5\n")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := HeadlessClaude{}.Generate(ctx, Request{SpecDir: "s", Language: "go", TargetPath: "t"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "timed out")
}

func TestHeadlessGenerateNoResultLineErrorsWithTail(t *testing.T) {
	writeFakeClaude(t, `
echo '{"type":"result","subtype":"success","result":"I could not finish"}'
`)
	_, err := HeadlessClaude{}.Generate(context.Background(), Request{SpecDir: "s", Language: "go", TargetPath: "t"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not parse RESULT line")
	require.Contains(t, err.Error(), "I could not finish")
}

func TestHeadlessGenerateMissingBinaryErrors(t *testing.T) {
	_, err := HeadlessClaude{Adapter: Adapter{Name: "claude", Bin: "no-such-agent-xyz"}}.
		Generate(context.Background(), Request{SpecDir: "s", Language: "go", TargetPath: "t"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no-such-agent-xyz not found on PATH")
}

func TestProgressLine(t *testing.T) {
	require.Equal(t, "  [tool] Bash: go test", progressLine("Bash", "", "go test"))
	require.Equal(t, "  [tool] Write a/b.go", progressLine("Write", "a/b.go", ""))
	require.Equal(t, "  [tool] WebSearch", progressLine("WebSearch", "", ""))
}
```

Add imports the file now needs: `context`, `os`, `path/filepath`, `strings`, `time`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run 'Headless|ProgressLine' -v`
Expected: FAIL — `unknown field Progress`, `undefined: progressLine`.

- [ ] **Step 3: Rewrite `internal/agent/headless.go`**

Replace the `HeadlessClaude` type and `Generate` (keep `buildPrompt` and `parseResultLine` exactly as they are):

```go
package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// HeadlessClaude drives generation non-interactively via the adapter's print
// mode (`claude -p`), streaming one tool-use progress line to Progress per
// event so the run is observable while it happens. The real claude binary is
// exercised via dogfood only; tests must always front PATH with a scripted
// fake.
type HeadlessClaude struct {
	Adapter     Adapter
	Permissions []string // replaces the adapter's default permission args when non-empty
	Progress    io.Writer
}

// streamEvent is the subset of `--output-format stream-json` events the
// headless path needs: assistant tool_use blocks (progress) and the final
// result event, whose text carries the RESULT line.
type streamEvent struct {
	Type    string `json:"type"`
	Result  string `json:"result"`
	Message struct {
		Content []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Input struct {
				FilePath string `json:"file_path"`
				Command  string `json:"command"`
			} `json:"input"`
		} `json:"content"`
	} `json:"message"`
}

func (h HeadlessClaude) Generate(ctx context.Context, req Request) (Result, error) {
	ad := h.Adapter
	if ad.Bin == "" {
		ad, _ = Lookup("")
	}
	progress := h.Progress
	if progress == nil {
		progress = io.Discard
	}
	if _, err := exec.LookPath(ad.Bin); err != nil {
		return Result{}, fmt.Errorf("%s not found on PATH", ad.Bin)
	}
	cmd := exec.CommandContext(ctx, ad.Bin, ad.HeadlessArgs(buildPrompt(req), h.Permissions)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("start %s: %w", ad.Bin, err)
	}
	// Keep the last 40 raw lines so every failure mode can show what the
	// agent actually said, without buffering a whole transcript.
	var tail []string
	keep := func(line string) {
		if len(tail) == 40 {
			tail = tail[1:]
		}
		tail = append(tail, line)
	}
	resultText := ""
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 10*1024*1024) // stream-json lines can be large
	for sc.Scan() {
		line := sc.Text()
		keep(line)
		var ev streamEvent
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue // non-JSON noise stays in the tail only
		}
		switch ev.Type {
		case "assistant":
			for _, c := range ev.Message.Content {
				if c.Type == "tool_use" {
					fmt.Fprintln(progress, progressLine(c.Name, c.Input.FilePath, c.Input.Command))
				}
			}
		case "result":
			resultText = ev.Result
		}
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return Result{}, fmt.Errorf("%s timed out; last output:\n%s", ad.Bin, transcriptTail(tail, &stderr))
	}
	if waitErr != nil {
		return Result{}, fmt.Errorf("%s: %v; last output:\n%s", ad.Bin, waitErr, transcriptTail(tail, &stderr))
	}
	tc, fs := parseResultLine(resultText)
	if tc == "" {
		return Result{}, fmt.Errorf("could not parse RESULT line from agent output; last output:\n%s", transcriptTail(tail, &stderr))
	}
	return Result{TestCommand: tc, FixtureStatus: fs}, nil
}

func transcriptTail(lines []string, stderr *bytes.Buffer) string {
	out := strings.Join(lines, "\n")
	if s := strings.TrimSpace(stderr.String()); s != "" {
		out += "\nstderr: " + s
	}
	return out
}

// progressLine renders one tool_use event as a single observable line:
// commands read as "Name: cmd", file edits as "Name path".
func progressLine(name, filePath, command string) string {
	switch {
	case command != "":
		return fmt.Sprintf("  [tool] %s: %s", name, command)
	case filePath != "":
		return fmt.Sprintf("  [tool] %s %s", name, filePath)
	default:
		return fmt.Sprintf("  [tool] %s", name)
	}
}
```

`buildPrompt` and `parseResultLine` (and their existing tests) stay untouched below this.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/ -v`
Expected: PASS, including the pre-existing `TestParseResultLine*` and `TestBuildPrompt*` tests. The timeout test must finish in well under a second.

- [ ] **Step 5: Commit**

```bash
go build ./... && go vet ./...
git add internal/agent/headless.go internal/agent/headless_test.go
git commit -m "feat: headless sync streams stream-json progress with timeout-aware errors"
```

---

### Task 4: recording gate — re-run the reported test command before recording

**Files:**
- Modify: `cmd/sync.go` (`runHeadless`)
- Test: `cmd/sync_test.go`

**Interfaces:**
- Consumes: existing `runHeadless`, `runRecord`, `agent.Backend`.
- Produces: `runHeadless` runs `sh -c <res.TestCommand>` from the project root after `Generate` and before `runRecord`; nonzero exit → error `"<dep>: generated, but test command %q failed — nothing recorded"` (plus command output), lockfile untouched. Task 6 preserves this behavior when it re-signatures `runHeadless`.

- [ ] **Step 1: Write the failing test**

Add to `cmd/sync_test.go`:

```go
// failingTestBackend generates like the stub but reports a test command that
// fails, which must block recording.
type failingTestBackend struct{}

func (failingTestBackend) Generate(ctx context.Context, req agent.Request) (agent.Result, error) {
	if _, err := agent.StubBackend{}.Generate(ctx, req); err != nil {
		return agent.Result{}, err
	}
	return agent.Result{TestCommand: "false", FixtureStatus: "pass"}, nil
}

func TestSyncHeadlessFailingTestCommandRecordsNothing(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	root := newRootCmdWithBackend(failingTestBackend{})
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--chdir", dir, "sync"})
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), `test command "false" failed`)
	require.Contains(t, err.Error(), "nothing recorded")

	l, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p, _ := l.Find("demo")
	require.Empty(t, p.GeneratedCommit, "a failing test command must not be recorded")
}
```

(Task 6 will add `--headless` to these args when mode selection lands; bare `sync` still routes to `runHeadless` at this point.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestSyncHeadlessFailingTestCommandRecordsNothing -v`
Expected: FAIL — sync currently records despite the failing command, so `p.GeneratedCommit` is non-empty.

- [ ] **Step 3: Implement the gate**

In `cmd/sync.go` `runHeadless`, between the `backend.Generate` error check and the `runRecord` call, insert:

```go
		// Recording gate: never trust the agent's claim alone. Re-run the
		// reported test command; a sync whose test fails records nothing.
		if out, testErr := exec.Command("sh", "-c", res.TestCommand).CombinedOutput(); testErr != nil {
			return fmt.Errorf("%s: generated, but test command %q failed — nothing recorded: %v\n%s",
				p.Name, res.TestCommand, testErr, out)
		}
```

Add `"os/exec"` to the file's imports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -v`
Expected: PASS — the new test, and the existing `TestSyncHeadlessGeneratesAndRecords` / e2e (stub reports `true`, which passes the gate).

- [ ] **Step 5: Commit**

```bash
go build ./... && go vet ./...
git add cmd/sync.go cmd/sync_test.go
git commit -m "feat: headless sync re-runs the reported test command before recording"
```

---

### Task 5: interactive launcher — scaffold.SyncInstructions + Adapter.LaunchInteractive

**Files:**
- Modify: `internal/scaffold/scaffold.go`
- Create: `internal/agent/interactive.go`
- Test: `internal/scaffold/scaffold_test.go`, `internal/agent/interactive_test.go`

**Interfaces:**
- Consumes: `Adapter`, `InteractiveArgs` from Task 2; scaffold's embedded `templates/sync-instructions.md`.
- Produces: `func SyncInstructions() (string, error)` in scaffold (the canonical instructions body); `func (a Adapter) LaunchInteractive(instructions string, stdin io.Reader, stdout, stderr io.Writer) error`. Task 6 calls both.

- [ ] **Step 1: Write the failing tests**

In `internal/scaffold/scaffold_test.go`:

```go
func TestSyncInstructionsExposesCanonicalBody(t *testing.T) {
	body, err := SyncInstructions()
	require.NoError(t, err)
	require.Contains(t, body, "speclib sync --plan --json")
	require.Contains(t, body, "speclib sync --record")
}
```

Create `internal/agent/interactive_test.go`:

```go
package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLaunchInteractivePassesInstructionsAndWiresStdio(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args.txt")
	t.Setenv("FAKE_ARGS", argsFile)
	writeFakeClaude(t, `printf '%s' "$1" > "$FAKE_ARGS"
echo "agent ui"
`)
	a, _ := Lookup("claude")
	var out, errOut strings.Builder
	err := a.LaunchInteractive("THE INSTRUCTIONS", strings.NewReader(""), &out, &errOut)
	require.NoError(t, err)
	require.Equal(t, "agent ui\n", out.String())
	got, readErr := os.ReadFile(argsFile)
	require.NoError(t, readErr)
	require.Equal(t, "THE INSTRUCTIONS", string(got))
}

func TestLaunchInteractiveMissingBinaryErrors(t *testing.T) {
	a := Adapter{Name: "claude", Bin: "no-such-agent-xyz"}
	err := a.LaunchInteractive("x", strings.NewReader(""), io.Discard, io.Discard)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no-such-agent-xyz not found on PATH")
	require.Contains(t, err.Error(), "--headless")
}
```

(`writeFakeClaude` comes from Task 3's test file — same package. Add the `io` import for the second test.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scaffold/ ./internal/agent/ -run 'SyncInstructions|LaunchInteractive' -v`
Expected: FAIL — `undefined: SyncInstructions`, `undefined` method.

- [ ] **Step 3: Implement**

In `internal/scaffold/scaffold.go`, after the `syncInstructionsPath` const:

```go
// SyncInstructions returns the canonical speclib-sync workflow body all agent
// integrations share. The interactive `speclib sync` handoff passes it as the
// agent's initial prompt, so the flow works even when no integration file was
// ever installed.
func SyncInstructions() (string, error) {
	raw, err := templates.ReadFile(syncInstructionsPath)
	return string(raw), err
}
```

Also simplify `WriteAgent` to reuse it (replace its first three lines):

```go
func WriteAgent(dir, agent string) error {
	body, err := SyncInstructions()
	if err != nil {
		return err
	}
```

Create `internal/agent/interactive.go`:

```go
package agent

import (
	"fmt"
	"io"
	"os/exec"
)

// LaunchInteractive opens the adapter's own interactive UI with instructions
// as the initial prompt, wiring the given stdio through. When the caller
// passes *os.File values (the real CLI passes the process's stdin/stdout/
// stderr), the child inherits the terminal file descriptors directly, so the
// agent's full-screen UI, permission prompts, and questions all work.
func (a Adapter) LaunchInteractive(instructions string, stdin io.Reader, stdout, stderr io.Writer) error {
	if _, err := exec.LookPath(a.Bin); err != nil {
		return fmt.Errorf("%s not found on PATH — install it, or use `speclib sync --headless`", a.Bin)
	}
	c := exec.Command(a.Bin, a.InteractiveArgs(instructions)...)
	c.Stdin, c.Stdout, c.Stderr = stdin, stdout, stderr
	return c.Run()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scaffold/ ./internal/agent/ -v`
Expected: PASS (including existing scaffold tests — `WriteAgent` behavior is unchanged).

- [ ] **Step 5: Commit**

```bash
go build ./... && go vet ./...
git add internal/scaffold/ internal/agent/interactive.go internal/agent/interactive_test.go
git commit -m "feat: interactive launcher and exported sync instructions"
```

---

### Task 6: mode selection — TTY handoff default, --headless, --timeout

**Files:**
- Modify: `cmd/sync.go`, `cmd/root.go`
- Test: `cmd/sync_test.go`, `cmd/e2e_test.go` (update existing bare-`sync` calls)

**Interfaces:**
- Consumes: everything from Tasks 1–5: `agent.Lookup`, `agent.HeadlessClaude{Adapter, Permissions, Progress}`, `Adapter.LaunchInteractive`, `scaffold.SyncInstructions`, `m.AgentCommand()`, `m.AgentPermissions()`.
- Produces: `speclib sync` final behavior — TTY → `runInteractive`; `--headless` → `runHeadless(cmd, only, backend, timeout)`; non-TTY without `--headless` → error. `newRootCmd` passes a nil backend (built lazily from the manifest); tests keep injecting stubs via `newRootCmdWithBackend`. Package-level `var isInteractiveTTY func() bool` is swappable in tests.

- [ ] **Step 1: Write the failing tests**

Add to `cmd/sync_test.go`:

```go
// stubTTY forces the interactive-TTY answer for one test.
func stubTTY(t *testing.T, val bool) {
	t.Helper()
	old := isInteractiveTTY
	isInteractiveTTY = func() bool { return val }
	t.Cleanup(func() { isInteractiveTTY = old })
}

func TestSyncNonTTYWithoutHeadlessErrors(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)
	stubTTY(t, false)

	_, err := runSyncWithStub(t, dir, "sync")
	require.Error(t, err)
	require.Equal(t, "not a terminal; pass --headless for non-interactive use", err.Error())
}

func TestSyncInteractiveLaunchesAgentAndSummarizes(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)
	stubTTY(t, true)
	argsFile := filepath.Join(dir, "args.txt")
	t.Setenv("FAKE_ARGS", argsFile)
	binDir := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "claude"),
		[]byte("#!/bin/sh\nprintf '%s' \"$1\" > \"$FAKE_ARGS\"\n"), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	out, err := runSyncWithStub(t, dir, "sync")
	require.NoError(t, err)

	instructions, readErr := os.ReadFile(argsFile)
	require.NoError(t, readErr)
	require.Contains(t, string(instructions), "speclib sync --plan --json")
	require.Contains(t, out, "Launching claude for 1 pending dependency(ies)...")
	require.Contains(t, out, "demo: pending") // the fake agent recorded nothing
	require.FileExists(t, filepath.Join(dir, ".speclib", "work", "demo", "SPEC.md"))
}

func TestSyncInteractiveSingleDepAppendsOnlyInstruction(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)
	stubTTY(t, true)
	argsFile := filepath.Join(dir, "args.txt")
	t.Setenv("FAKE_ARGS", argsFile)
	binDir := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "claude"),
		[]byte("#!/bin/sh\nprintf '%s' \"$1\" > \"$FAKE_ARGS\"\n"), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := runSyncWithStub(t, dir, "sync", "demo")
	require.NoError(t, err)
	instructions, readErr := os.ReadFile(argsFile)
	require.NoError(t, readErr)
	require.Contains(t, string(instructions), `Sync only the dependency named "demo".`)
}

func TestSyncInteractiveNonzeroExitPropagatesAfterSummary(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)
	stubTTY(t, true)
	binDir := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "claude"),
		[]byte("#!/bin/sh\nexit 3\n"), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	out, err := runSyncWithStub(t, dir, "sync")
	require.Error(t, err, "the agent session's failure must propagate")
	require.Contains(t, out, "demo: pending", "the summary must still print on failure")
}

func TestSyncHeadlessFlagIsExclusiveWithPlan(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	_, err := runSyncWithStub(t, dir, "sync", "--headless", "--plan")
	require.Error(t, err)
}

func TestSyncNothingPendingSkipsLaunch(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)
	stubTTY(t, true)
	l, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p, _ := l.Find("demo")
	p.GeneratedCommit = p.Commit
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))
	// No fake claude on PATH is needed: with nothing pending, nothing may launch.

	out, err := runSyncWithStub(t, dir, "sync")
	require.NoError(t, err)
	require.Contains(t, out, "Nothing to sync.")
}
```

Update the existing headless-path tests to opt in explicitly — in `cmd/sync_test.go`:
- `TestSyncHeadlessUnknownDepErrors`: args become `"sync", "--headless", "nonexistent"`.
- `TestSyncHeadlessPassesChecksToBackend`: `root.SetArgs([]string{"--chdir", dir, "sync", "--headless"})`.
- `TestSyncHeadlessGeneratesAndRecords`: args become `"sync", "--headless"`.
- `TestSyncHeadlessFailingTestCommandRecordsNothing` (Task 4): `root.SetArgs([]string{"--chdir", dir, "sync", "--headless"})`.

In `cmd/e2e_test.go`, both `runSyncWithStub(t, dir, "sync")` calls become `runSyncWithStub(t, dir, "sync", "--headless")`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -v`
Expected: FAIL — `undefined: isInteractiveTTY`, unknown flag `--headless`.

- [ ] **Step 3: Implement mode selection in `cmd/sync.go`**

At the top of the file (after imports), add:

```go
// isInteractiveTTY reports whether both stdin and stdout are terminals — the
// gate for handing off to the agent's interactive UI. Swappable in tests.
var isInteractiveTTY = func() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout} {
		info, err := f.Stat()
		if err != nil || info.Mode()&os.ModeCharDevice == 0 {
			return false
		}
	}
	return true
}
```

Rework `newSyncCmd`:

```go
func newSyncCmd(backend agent.Backend) *cobra.Command {
	var plan, asJSON, headless bool
	var timeout time.Duration
	var record, testCmd, fixtureStatus, generatedCommit, selections string
	cmd := &cobra.Command{
		Use:   "sync [dep]",
		Short: "Generate code for pending dependencies (one at a time)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			only := ""
			if len(args) == 1 {
				only = args[0]
			}
			switch {
			case record != "":
				return runRecord(cmd, record, testCmd, fixtureStatus, generatedCommit, selections)
			case plan:
				return runPlan(cmd, only, asJSON)
			case headless:
				return runHeadless(cmd, only, backend, timeout)
			default:
				if !isInteractiveTTY() {
					return errors.New("not a terminal; pass --headless for non-interactive use")
				}
				return runInteractive(cmd, only)
			}
		},
	}
	cmd.Flags().BoolVar(&plan, "plan", false, "print the work plan and materialize specs (no generation)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "with --plan, emit JSON")
	cmd.Flags().BoolVar(&headless, "headless", false, "generate non-interactively via the agent's print mode (for CI)")
	cmd.Flags().DurationVar(&timeout, "timeout", 15*time.Minute, "with --headless, per-dependency generation timeout")
	cmd.Flags().StringVar(&record, "record", "", "record generation provenance for this dependency")
	cmd.Flags().StringVar(&testCmd, "test-command", "", "with --record, the test command to re-run in verify")
	cmd.Flags().StringVar(&fixtureStatus, "fixture-status", "pass", "with --record: pass|skip|fail")
	cmd.Flags().StringVar(&generatedCommit, "generated-commit", "", "with --record: spec commit generated from (defaults to resolved commit)")
	cmd.Flags().StringVar(&selections, "selections", "", "with --record: generation choices to honor on future upgrades")
	cmd.MarkFlagsMutuallyExclusive("plan", "record", "headless")
	return cmd
}
```

Add `runInteractive`:

```go
// runInteractive materializes every pending spec, then hands the terminal to
// the agent's own UI — full streaming, permission prompts, and questions —
// seeded with the canonical sync instructions. Recording happens inside the
// session via `speclib sync --record`, exactly as the installed skill does.
func runInteractive(cmd *cobra.Command, only string) error {
	m, l, err := loadState()
	if err != nil {
		return err
	}
	pending, err := syncplan.Compute(m, l, only)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		if only != "" {
			if err := requireKnownDep(m, l, only); err != nil {
				return err
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to sync.")
		return nil
	}
	names := make([]string, 0, len(pending))
	for _, p := range pending {
		if _, err := syncplan.Materialize(paths.WorkDir, m.Dependencies[p.Name], p, m.Project.Checks); err != nil {
			return err
		}
		names = append(names, p.Name)
	}
	ad, err := agent.Lookup(m.AgentCommand())
	if err != nil {
		return err
	}
	instructions, err := scaffold.SyncInstructions()
	if err != nil {
		return err
	}
	if only != "" {
		instructions += fmt.Sprintf("\n\nSync only the dependency named %q.", only)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Launching %s for %d pending dependency(ies)...\n", ad.Bin, len(pending))
	runErr := ad.LaunchInteractive(instructions, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
	// Partial progress is durable (--record ran inside the session), so
	// summarize per-dependency state even when the session exited nonzero.
	if l2, loadErr := lockfile.Load(paths.Lock); loadErr == nil {
		for _, name := range names {
			if p, ok := l2.Find(name); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", name, p.State())
			}
		}
	}
	return runErr
}
```

Re-signature `runHeadless` to take the timeout and build the backend lazily (keep the Task 4 recording gate in place):

```go
func runHeadless(cmd *cobra.Command, only string, backend agent.Backend, timeout time.Duration) error {
	m, l, err := loadState()
	if err != nil {
		return err
	}
	pending, err := syncplan.Compute(m, l, only)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		if only != "" {
			if err := requireKnownDep(m, l, only); err != nil {
				return err
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to sync.")
		return nil
	}
	if backend == nil {
		ad, err := agent.Lookup(m.AgentCommand())
		if err != nil {
			return err
		}
		backend = agent.HeadlessClaude{Adapter: ad, Permissions: m.AgentPermissions(), Progress: cmd.ErrOrStderr()}
	}
	for _, p := range pending {
		item, err := syncplan.Materialize(paths.WorkDir, m.Dependencies[p.Name], p, m.Project.Checks)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Generating %s...\n", p.Name)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		res, err := backend.Generate(ctx, agent.Request{
			Name: item.Name, TargetPath: item.TargetPath, Language: item.Language,
			ContextFile: item.ContextFile, SpecDir: item.SpecDir, Checks: item.Checks,
		})
		cancel()
		if err != nil {
			return fmt.Errorf("generate %s: %w", p.Name, err)
		}
		// Recording gate: never trust the agent's claim alone. Re-run the
		// reported test command; a sync whose test fails records nothing.
		if out, testErr := exec.Command("sh", "-c", res.TestCommand).CombinedOutput(); testErr != nil {
			return fmt.Errorf("%s: generated, but test command %q failed — nothing recorded: %v\n%s",
				p.Name, res.TestCommand, testErr, out)
		}
		if err := runRecord(cmd, p.Name, res.TestCommand, res.FixtureStatus, "", ""); err != nil {
			return err
		}
	}
	return nil
}
```

Update the file's imports to: `context`, `encoding/json`, `errors`, `fmt`, `os`, `os/exec`, `time`, the existing internal imports, plus `"github.com/jacob-meacham/speclib/internal/scaffold"` (lockfile is already imported).

In `cmd/root.go`, make the default backend lazy:

```go
func newRootCmd() *cobra.Command {
	// nil backend: sync --headless builds the configured HeadlessClaude from
	// the manifest's [agent] section at run time. Tests inject stubs here.
	return newRootCmdWithBackend(nil)
}
```

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: PASS everywhere. Specifically: the two interactive tests, non-TTY error, exclusivity, nothing-pending skip, all updated `--headless` tests, and the untouched packages.

- [ ] **Step 5: Commit**

```bash
go build ./... && go vet ./...
git add cmd/
git commit -m "feat: speclib sync hands off to the agent UI; --headless for CI"
```

---

### Task 7: docs — tutorial and README

**Files:**
- Modify: `docs/tutorials/first-spec-library.md`
- Modify: `README.md:37-39`

**Interfaces:** none — prose only, but the claims must match Task 6's shipped behavior exactly.

- [ ] **Step 1: Update the tutorial's generation step**

In `docs/tutorials/first-spec-library.md`, replace the paragraph beginning `Now generate. Inside Claude Code, say **"sync my speclib dependencies"**` (currently lines 150–154) with:

```markdown
Now generate. Run `speclib sync` — it launches Claude Code preloaded with the
sync instructions. You watch generation live, answer any questions it asks,
and it records provenance as it finishes each dependency. (Equivalently, open
Claude Code yourself and say **"sync my speclib dependencies"** — the
installed skill follows the same instructions.) For slugify the generated
code looks like:
```

- [ ] **Step 2: Add a headless/CI subsection**

Immediately before the `## Part 3` heading, insert:

```markdown
### Headless (CI)

`speclib sync --headless` generates without a UI: the agent runs in print
mode with the adapter's default permissions (`--allowedTools Write,Edit,Bash`
for claude — override via the `[agent]` section in `speclib.toml`), streams
one-line tool progress to stderr, and times out per dependency after 15
minutes (`--timeout`). Before recording, speclib re-runs the reported test
command itself; a failing test records nothing.
```

- [ ] **Step 3: Update the README command list**

In `README.md`, replace lines 37–39:

```markdown
- `speclib sync [dep]` — generate code for pending dependencies, one at a time.
  Inside Claude Code, the installed skill drives this interactively; the CLI
  exposes `sync --plan` / `sync --record` as its plumbing.
```

with:

```markdown
- `speclib sync [dep]` — generate code for pending dependencies. On a
  terminal it launches your coding agent's own UI preloaded with the sync
  instructions; `--headless` generates non-interactively for CI. The CLI
  exposes `sync --plan` / `sync --record` as the plumbing agents drive.
```

- [ ] **Step 4: Verify docs match behavior**

Run: `grep -n "speclib sync" README.md docs/tutorials/first-spec-library.md`
Expected: no remaining claim that bare `sync` is headless or that generation only happens inside a manually-opened Claude Code session.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/tutorials/first-spec-library.md
git commit -m "docs: sync launches the agent UI; document --headless for CI"
```

---

## Final verification (after all tasks)

- [ ] `go build ./... && go vet ./... && go test ./...` — all green.
- [ ] Manual dogfood (not automated; requires real `claude`): in a scratch dir, `speclib new demo && cd demo && git init && git add -A && git commit -m x && speclib release 0.1.0`, then in a sibling app dir `speclib init && speclib add ../demo@0.1.0 --path gen/demo --lang python`, then (a) `speclib sync` from a real terminal → Claude Code UI opens seeded with the instructions; (b) `speclib sync --headless` → progress lines stream, a failing/absent test records nothing, success records provenance and `speclib verify` passes.
