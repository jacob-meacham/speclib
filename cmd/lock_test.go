package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-meacham/speclib/internal/lockfile"
	"github.com/jacob-meacham/speclib/internal/manifest"
	"github.com/stretchr/testify/require"
)

func TestLockResolvesMissing(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib")
	writeDemoLib(t, lib)

	// Hand-write a manifest with a dep but no lockfile entry.
	m := &manifest.Manifest{
		Project:      manifest.Project{Language: "go"},
		Dependencies: map[string]manifest.Dependency{"demo": {Source: lib, Version: "*", Path: "gen/demo"}},
	}
	require.NoError(t, m.Save(filepath.Join(dir, "speclib.toml")))

	_, err := runCmd(t, dir, "lock")
	require.NoError(t, err)

	l, err := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	require.NoError(t, err)
	p, ok := l.Find("demo")
	require.True(t, ok)
	require.NotEmpty(t, p.SpecHash)
	require.Equal(t, "go", p.Language)

	// Lock again: idempotent, entry unchanged.
	before, _ := os.ReadFile(filepath.Join(dir, "speclib.lock"))
	_, err = runCmd(t, dir, "lock")
	require.NoError(t, err)
	after, _ := os.ReadFile(filepath.Join(dir, "speclib.lock"))
	require.Equal(t, before, after)
}
