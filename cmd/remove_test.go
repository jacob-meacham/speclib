package cmd

import (
	"path/filepath"
	"testing"

	"github.com/jacob-meacham/speclib/internal/lockfile"
	"github.com/jacob-meacham/speclib/internal/manifest"
	"github.com/stretchr/testify/require"
)

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	m := &manifest.Manifest{Dependencies: map[string]manifest.Dependency{"demo": {Source: "x", Path: "gen/demo"}}}
	require.NoError(t, m.Save(filepath.Join(dir, "speclib.toml")))
	l := &lockfile.Lockfile{Packages: []lockfile.Package{{Name: "demo", Path: "gen/demo"}}}
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))

	out, err := runCmd(t, dir, "remove", "demo")
	require.NoError(t, err)
	require.Contains(t, out, "gen/demo")

	m2, _ := manifest.Load(filepath.Join(dir, "speclib.toml"))
	_, ok := m2.Dependencies["demo"]
	require.False(t, ok)
	l2, _ := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	_, ok = l2.Find("demo")
	require.False(t, ok)
}
