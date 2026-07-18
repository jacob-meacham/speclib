package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func gitRelease(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

// makeCommittedLib scaffolds a spec-library named demo in dir/demo, commits
// it, and returns the library directory.
func makeCommittedLib(t *testing.T, dir string) string {
	t.Helper()
	_, err := runCmd(t, dir, "new", "demo")
	require.NoError(t, err)
	libDir := filepath.Join(dir, "demo")
	gitRelease(t, libDir, "add", "-A")
	gitRelease(t, libDir, "commit", "-qm", "initial")
	return libDir
}

func TestReleaseCreatesTag(t *testing.T) {
	dir := t.TempDir()
	libDir := makeCommittedLib(t, dir)

	out, err := runCmd(t, libDir, "release", "0.1.0")
	require.NoError(t, err)
	require.Contains(t, out, "v0.1.0")

	tagOut, err := exec.Command("git", "-C", libDir, "tag", "--list", "v0.1.0").CombinedOutput()
	require.NoError(t, err)
	require.Contains(t, string(tagOut), "v0.1.0")
}

func TestReleaseErrorsIfTagExists(t *testing.T) {
	dir := t.TempDir()
	libDir := makeCommittedLib(t, dir)

	_, err := runCmd(t, libDir, "release", "0.1.0")
	require.NoError(t, err)

	_, err = runCmd(t, libDir, "release", "0.1.0")
	require.Error(t, err)
}

func TestReleaseErrorsOnInvalidSemver(t *testing.T) {
	dir := t.TempDir()
	libDir := makeCommittedLib(t, dir)

	_, err := runCmd(t, libDir, "release", "notsemver")
	require.Error(t, err)
}

func TestReleaseErrorsOnDirtyTree(t *testing.T) {
	dir := t.TempDir()
	libDir := makeCommittedLib(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "SPEC.md"), []byte("dirty change"), 0o644))

	_, err := runCmd(t, libDir, "release", "0.1.0")
	require.Error(t, err)
}

func TestReleaseErrorsOnLintFailure(t *testing.T) {
	dir := t.TempDir()
	libDir := makeCommittedLib(t, dir)
	require.NoError(t, os.Remove(filepath.Join(libDir, "SPEC.md")))
	gitRelease(t, libDir, "add", "-A")
	gitRelease(t, libDir, "commit", "-qm", "remove spec")

	_, err := runCmd(t, libDir, "release", "0.1.0")
	require.Error(t, err)
}
