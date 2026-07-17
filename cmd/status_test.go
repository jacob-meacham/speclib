package cmd

import (
	"path/filepath"
	"testing"

	"github.com/jmeacham/speclib/internal/lockfile"
	"github.com/stretchr/testify/require"
)

func TestStatusShowsState(t *testing.T) {
	dir := t.TempDir()
	l := &lockfile.Lockfile{Packages: []lockfile.Package{
		{Name: "pending-dep", Version: "1.0.0", Commit: "c1"},
		{Name: "done-dep", Version: "2.0.0", Commit: "c2", GeneratedCommit: "c2", FixtureStatus: "pass"},
	}}
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))

	out, err := runCmd(t, dir, "status")
	require.NoError(t, err)
	require.Contains(t, out, "pending-dep")
	require.Contains(t, out, "pending")
	require.Contains(t, out, "done-dep")
	require.Contains(t, out, "up-to-date")
	require.Contains(t, out, "pass")
}
