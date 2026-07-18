package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLintCleanLibraryPasses(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, dir, "new", "demo")
	require.NoError(t, err)

	out, err := runCmd(t, filepath.Join(dir, "demo"), "lint")
	require.NoError(t, err)
	require.Contains(t, out, "ok")
	require.Contains(t, out, "demo")
}

func TestLintReportsMissingSpecFile(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, dir, "new", "demo")
	require.NoError(t, err)
	libDir := filepath.Join(dir, "demo")
	require.NoError(t, os.Remove(filepath.Join(libDir, "SPEC.md")))

	out, err := runCmd(t, libDir, "lint")
	require.Error(t, err)
	require.Contains(t, out, "SPEC.md")
}

func TestLintReportsEmptyName(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, dir, "new", "demo")
	require.NoError(t, err)
	libDir := filepath.Join(dir, "demo")
	toml := "[library]\nname = \"\"\n\n[files]\nprompt = \"PROMPT.md\"\nspec = \"SPEC.md\"\nfixtures = \"test_fixtures.json\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "speclib.toml"), []byte(toml), 0o644))

	out, err := runCmd(t, libDir, "lint")
	require.Error(t, err)
	require.Contains(t, out, "name")
}

func TestLintReportsMalformedFixtures(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, dir, "new", "demo")
	require.NoError(t, err)
	libDir := filepath.Join(dir, "demo")
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "test_fixtures.json"), []byte("{not json"), 0o644))

	out, err := runCmd(t, libDir, "lint")
	require.Error(t, err)
	require.Contains(t, out, "not valid JSON")
}

func TestLintErrorsWhenManifestMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, dir, "lint")
	require.Error(t, err)
}
