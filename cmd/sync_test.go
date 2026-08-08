package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-meacham/speclib/internal/agent"
	"github.com/jacob-meacham/speclib/internal/fingerprint"
	"github.com/jacob-meacham/speclib/internal/lockfile"
	"github.com/jacob-meacham/speclib/internal/manifest"
	"github.com/stretchr/testify/require"
)

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

func runSyncWithStub(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	root := newRootCmdWithBackend(agent.StubBackend{})
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"--chdir", dir}, args...))
	err := root.Execute()
	return out.String(), err
}

func TestSyncPlanJSON(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	out, err := runSyncWithStub(t, dir, "sync", "--plan", "--json")
	require.NoError(t, err)

	var plan struct{ Items []map[string]any }
	require.NoError(t, json.Unmarshal([]byte(out), &plan))
	require.Len(t, plan.Items, 1)
	require.Equal(t, "demo", plan.Items[0]["name"])
	_, hasChecks := plan.Items[0]["checks"]
	require.False(t, hasChecks, "checks key must be absent when the manifest declares none")
	require.FileExists(t, filepath.Join(dir, ".speclib", "work", "demo", "SPEC.md"))
}

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

func TestSyncRecord(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)
	// give it a concrete commit so generated_commit can match
	l, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p, _ := l.Find("demo")
	p.Commit = "abc123"
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))

	_, err := runSyncWithStub(t, dir, "sync", "--record", "demo", "--test-command", "go test ./gen/demo", "--fixture-status", "pass")
	require.NoError(t, err)

	l2, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p2, _ := l2.Find("demo")
	require.Equal(t, "abc123", p2.GeneratedCommit)
	require.Equal(t, "pass", p2.FixtureStatus)
	require.Equal(t, lockfile.UpToDate, p2.State())
}

func TestSyncRecordComputesGeneratedHash(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)
	genDir := filepath.Join(dir, "gen", "demo")
	require.NoError(t, os.MkdirAll(genDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "demo.go"), []byte("package demo"), 0o644))

	_, err := runSyncWithStub(t, dir, "sync", "--record", "demo", "--test-command", "go test ./gen/demo", "--fixture-status", "pass")
	require.NoError(t, err)

	want, err := fingerprint.HashDir(genDir)
	require.NoError(t, err)
	require.NotEmpty(t, want)

	l2, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p2, _ := l2.Find("demo")
	require.Equal(t, want, p2.GeneratedHash)
}

func TestSyncRecordMissingDirLeavesHashEmpty(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)
	// gen/demo does not exist.

	_, err := runSyncWithStub(t, dir, "sync", "--record", "demo", "--test-command", "true", "--fixture-status", "pass")
	require.NoError(t, err)

	l2, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p2, _ := l2.Find("demo")
	require.Empty(t, p2.GeneratedHash)
}

func TestSyncRecordSelections(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)
	l, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p, _ := l.Find("demo")
	p.Commit = "abc123"
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))

	_, err := runSyncWithStub(t, dir, "sync", "--record", "demo",
		"--test-command", "go test ./gen/demo", "--fixture-status", "pass",
		"--selections", "channels=roku,fire")
	require.NoError(t, err)

	l2, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p2, _ := l2.Find("demo")
	require.Equal(t, "channels=roku,fire", p2.Selections)
}

func TestSyncPlanUnknownDepErrors(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	_, err := runSyncWithStub(t, dir, "sync", "--plan", "nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no such dependency: nonexistent")
}

func TestSyncHeadlessUnknownDepErrors(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	_, err := runSyncWithStub(t, dir, "sync", "--headless", "nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no such dependency: nonexistent")
}

func TestSyncKnownDepAlreadyUpToDateDoesNotError(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)
	// Mark demo up-to-date: GeneratedCommit == Commit.
	l, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p, _ := l.Find("demo")
	p.Commit = "local"
	p.GeneratedCommit = "local"
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))

	out, err := runSyncWithStub(t, dir, "sync", "--plan", "demo")
	require.NoError(t, err)
	require.Contains(t, out, "Nothing to sync.")
}

func TestSyncPlanAndRecordAreMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	_, err := runSyncWithStub(t, dir, "sync", "--plan", "--record", "demo")
	require.Error(t, err)
}

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
	root.SetArgs([]string{"--chdir", dir, "sync", "--headless"})
	require.NoError(t, root.Execute())

	require.Equal(t, []string{"go build ./...", "go vet ./..."}, got.Checks)
}

func TestSyncHeadlessGeneratesAndRecords(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	_, err := runSyncWithStub(t, dir, "sync", "--headless")
	require.NoError(t, err)

	l, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p, _ := l.Find("demo")
	require.NotEmpty(t, p.GeneratedCommit)
	require.Equal(t, "true", p.TestCommand)
	require.FileExists(t, filepath.Join(dir, "gen", "demo", "GENERATED.md"))
}

// failingTestBackend generates like the stub but reports a test command that
// fails, which must block recording.
type failingTestBackend struct{}

func (b failingTestBackend) Generate(ctx context.Context, req agent.Request) (agent.Result, error) {
	_, err := agent.StubBackend{}.Generate(ctx, req)
	if err != nil {
		return agent.Result{}, err
	}
	return agent.Result{TestCommand: "false", FixtureStatus: "pass"}, nil
}

// inSessionRecordBackend mimics the real agent: the sync instructions have it
// record provenance itself via `speclib sync --record ... --selections ...`
// during the session, before the driver's recording gate runs.
type inSessionRecordBackend struct {
	t   *testing.T
	dir string
}

func (b inSessionRecordBackend) Generate(ctx context.Context, req agent.Request) (agent.Result, error) {
	if _, err := (agent.StubBackend{}).Generate(ctx, req); err != nil {
		return agent.Result{}, err
	}
	_, err := runSyncWithStub(b.t, b.dir, "sync", "--record", req.Name, "--test-command", "true",
		"--selections", "language: go; flavor: minimal")
	require.NoError(b.t, err)
	return agent.Result{TestCommand: "true", FixtureStatus: "pass"}, nil
}

func TestSyncHeadlessGatePreservesInSessionSelections(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	root := newRootCmdWithBackend(inSessionRecordBackend{t: t, dir: dir})
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--chdir", dir, "sync", "--headless"})
	require.NoError(t, root.Execute())

	l, err := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	require.NoError(t, err)
	p, ok := l.Find("demo")
	require.True(t, ok)
	require.Equal(t, "language: go; flavor: minimal", p.Selections)
}

func TestSyncHeadlessFailingTestCommandRecordsNothing(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	root := newRootCmdWithBackend(failingTestBackend{})
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--chdir", dir, "sync", "--headless"})
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), `test command "false" failed`)
	require.Contains(t, err.Error(), "nothing recorded")

	l, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p, _ := l.Find("demo")
	require.Empty(t, p.GeneratedCommit, "a failing test command must not be recorded")
}

// TestSyncHeadlessNilBackendUsesManifestAgentConfig covers the backend == nil
// branch of runHeadless — the only path where the manifest's [agent] section
// (permissions in particular) actually reaches HeadlessClaude. Every other
// headless test injects a stub/recording backend and never exercises this
// wiring.
func TestSyncHeadlessNilBackendUsesManifestAgentConfig(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	m, err := manifest.Load(filepath.Join(dir, "speclib.toml"))
	require.NoError(t, err)
	m.Agent = &manifest.Agent{Permissions: []string{"--dangerously-skip-permissions"}}
	require.NoError(t, m.Save(filepath.Join(dir, "speclib.toml")))

	argsFile := filepath.Join(dir, "args.txt")
	t.Setenv("FAKE_ARGS", argsFile)
	binDir := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "claude"),
		[]byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$FAKE_ARGS\"\n"+
			"printf '{\"type\":\"result\",\"result\":\"RESULT true || skip\"}\\n'\n"), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := newRootCmdWithBackend(nil)
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--chdir", dir, "sync", "--headless"})
	require.NoError(t, root.Execute())

	argv, readErr := os.ReadFile(argsFile)
	require.NoError(t, readErr)
	require.Contains(t, string(argv), "--dangerously-skip-permissions")
	require.NotContains(t, string(argv), "--allowedTools")

	l2, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p2, _ := l2.Find("demo")
	require.Equal(t, "true", p2.TestCommand)
}

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

// The Materializing lines exist so sync is never silent while it acquires
// sources: materialization may fetch over the network, and a slow fetch with
// no output reads as a hang.
func TestSyncHeadlessPrintsMaterializeProgress(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	out, err := runSyncWithStub(t, dir, "sync", "--headless")
	require.NoError(t, err)
	require.Contains(t, out, "Materializing demo...")
}

func TestSyncInteractivePrintsMaterializeProgress(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)
	stubTTY(t, true)
	binDir := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	out, err := runSyncWithStub(t, dir, "sync")
	require.NoError(t, err)
	require.Contains(t, out, "Materializing demo...")
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

func TestSyncHeadlessFlagIsExclusiveWithRecord(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	_, err := runSyncWithStub(t, dir, "sync", "--headless", "--record", "demo")
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
	// A poison claude on PATH: if the nothing-pending guard ever regresses
	// and launches the agent anyway, this fails loudly instead of silently
	// invoking whatever "claude" happens to be on the real PATH.
	binDir := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "claude"),
		[]byte("#!/bin/sh\necho SHOULD-NOT-RUN >&2\nexit 97\n"), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	out, err := runSyncWithStub(t, dir, "sync")
	require.NoError(t, err)
	require.Contains(t, out, "Nothing to sync.")
}
