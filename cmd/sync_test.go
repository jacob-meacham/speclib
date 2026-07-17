package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmeacham/speclib/internal/agent"
	"github.com/jmeacham/speclib/internal/lockfile"
	"github.com/jmeacham/speclib/internal/manifest"
	"github.com/stretchr/testify/require"
)

func setupPending(t *testing.T, dir string) {
	t.Helper()
	lib := filepath.Join(dir, "lib")
	writeDemoLib(t, lib)
	m := &manifest.Manifest{
		Project:      manifest.Project{Language: "go"},
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
	require.FileExists(t, filepath.Join(dir, ".speclib", "work", "demo", "SPEC.md"))
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

func TestSyncHeadlessGeneratesAndRecords(t *testing.T) {
	dir := t.TempDir()
	setupPending(t, dir)

	_, err := runSyncWithStub(t, dir, "sync")
	require.NoError(t, err)

	l, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	p, _ := l.Find("demo")
	require.NotEmpty(t, p.GeneratedCommit)
	require.Equal(t, "true", p.TestCommand)
	require.FileExists(t, filepath.Join(dir, "gen", "demo", "GENERATED.md"))
}
