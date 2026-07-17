package cmd

import (
	"path/filepath"
	"testing"

	"github.com/jmeacham/speclib/internal/lockfile"
	"github.com/stretchr/testify/require"
)

func TestVerifyRunsTestCommands(t *testing.T) {
	dir := t.TempDir()
	l := &lockfile.Lockfile{Packages: []lockfile.Package{
		{Name: "ok", Commit: "c", GeneratedCommit: "c", TestCommand: "true"},
	}}
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))
	out, err := runCmd(t, dir, "verify")
	require.NoError(t, err)
	require.Contains(t, out, "ok")

	l2 := &lockfile.Lockfile{Packages: []lockfile.Package{
		{Name: "bad", Commit: "c", GeneratedCommit: "c", TestCommand: "false"},
	}}
	require.NoError(t, l2.Save(filepath.Join(dir, "speclib.lock")))
	_, err = runCmd(t, dir, "verify")
	require.Error(t, err)
}
