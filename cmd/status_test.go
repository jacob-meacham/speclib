package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmeacham/speclib/internal/fingerprint"
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
	require.Contains(t, out, "CODE")
	// Neither package has a GeneratedHash recorded.
	require.Contains(t, out, "-")
}

func TestStatusCodeColumnNeverGenerated(t *testing.T) {
	dir := t.TempDir()
	l := &lockfile.Lockfile{Packages: []lockfile.Package{
		{Name: "pending-dep", Version: "1.0.0", Commit: "c1", Path: "gen/pending-dep"},
	}}
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))

	out, err := runCmd(t, dir, "status")
	require.NoError(t, err)
	require.Regexp(t, `pending-dep\s+1\.0\.0\s+pending\s+-\s+-\s*\n`, out)
}

func TestStatusCodeColumnClean(t *testing.T) {
	dir := t.TempDir()
	genDir := filepath.Join(dir, "gen", "demo")
	require.NoError(t, os.MkdirAll(genDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "demo.go"), []byte("package demo"), 0o644))
	hash, err := fingerprint.HashDir(genDir)
	require.NoError(t, err)

	l := &lockfile.Lockfile{Packages: []lockfile.Package{
		{Name: "demo", Version: "1.0.0", Commit: "c1", GeneratedCommit: "c1", Path: "gen/demo", GeneratedHash: hash},
	}}
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))

	out, err := runCmd(t, dir, "status")
	require.NoError(t, err)
	require.Contains(t, out, "clean")
	require.NotContains(t, out, "edited")
}

func TestStatusCodeColumnEdited(t *testing.T) {
	dir := t.TempDir()
	genDir := filepath.Join(dir, "gen", "demo")
	require.NoError(t, os.MkdirAll(genDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "demo.go"), []byte("package demo"), 0o644))
	hash, err := fingerprint.HashDir(genDir)
	require.NoError(t, err)

	l := &lockfile.Lockfile{Packages: []lockfile.Package{
		{Name: "demo", Version: "1.0.0", Commit: "c1", GeneratedCommit: "c1", Path: "gen/demo", GeneratedHash: hash},
	}}
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))

	// Hand-edit the generated file after recording.
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "demo.go"), []byte("package demo // edited"), 0o644))

	out, err := runCmd(t, dir, "status")
	require.NoError(t, err)
	require.Contains(t, out, "edited")
}

func TestStatusCodeColumnMissing(t *testing.T) {
	dir := t.TempDir()
	l := &lockfile.Lockfile{Packages: []lockfile.Package{
		{Name: "demo", Version: "1.0.0", Commit: "c1", GeneratedCommit: "c1", Path: "gen/demo", GeneratedHash: "sha256:deadbeef"},
	}}
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))
	// gen/demo does not exist.

	out, err := runCmd(t, dir, "status")
	require.NoError(t, err)
	require.Contains(t, out, "missing")
}
