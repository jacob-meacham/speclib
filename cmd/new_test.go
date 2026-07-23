package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-meacham/speclib/internal/manifest"
	"github.com/stretchr/testify/require"
)

func TestNewScaffoldsLibrary(t *testing.T) {
	dir := t.TempDir()
	out, err := runCmd(t, dir, "new", "demo")
	require.NoError(t, err)
	require.Contains(t, out, "demo")

	libDir := filepath.Join(dir, "demo")
	for _, f := range []string{"speclib.toml", "PROMPT.md", "SPEC.md", "test_fixtures.json", "README.md", "CHANGELOG.md"} {
		require.FileExists(t, filepath.Join(libDir, f))
	}
	require.DirExists(t, filepath.Join(libDir, ".git"))

	data, err := os.ReadFile(filepath.Join(libDir, "speclib.toml"))
	require.NoError(t, err)
	lib, err := manifest.ParseLibrary(data)
	require.NoError(t, err)
	require.Equal(t, "demo", lib.Meta.Name)
	require.Equal(t, "PROMPT.md", lib.Files.Prompt)
	require.Equal(t, "SPEC.md", lib.Files.Spec)
	require.Equal(t, "test_fixtures.json", lib.Files.Fixtures)
}

func TestNewErrorsOnExistingNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "demo")
	require.NoError(t, os.MkdirAll(libDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "keep.txt"), []byte("x"), 0o644))

	_, err := runCmd(t, dir, "new", "demo")
	require.Error(t, err)
}

func TestNewIntoExistingEmptyDirSucceeds(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "demo")
	require.NoError(t, os.MkdirAll(libDir, 0o755))

	_, err := runCmd(t, dir, "new", "demo")
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(libDir, "speclib.toml"))
}
