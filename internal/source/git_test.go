package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// makeRepo creates a git repo with two tagged versions and returns its path.
func makeRepo(t *testing.T) string {
	t.Helper()
	// Sandbox the git mirror cache. On Linux (this project's target platform)
	// os.UserCacheDir() honors $XDG_CACHE_HOME, so this keeps tests out of the
	// developer's real ~/.cache. (os.UserCacheDir ignores XDG on macOS/Windows.)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	run("init", "-q")
	write("SPEC.md", "v1.0 spec")
	write("fixtures/a.json", "1")
	run("add", "-A")
	run("commit", "-qm", "v1")
	run("tag", "v1.0.0")
	write("SPEC.md", "v1.1 spec")
	run("add", "-A")
	run("commit", "-qm", "v1.1")
	run("tag", "v1.1.0")
	return dir
}

func TestGitTagsAndRead(t *testing.T) {
	repo := makeRepo(t)

	vs, refs, err := gitTags(repo)
	require.NoError(t, err)
	require.Len(t, vs, 2)
	require.Equal(t, "v1.1.0", refs["1.1.0"])

	commit, err := gitResolveCommit(repo, "v1.1.0")
	require.NoError(t, err)
	require.Len(t, commit, 40)

	data, err := gitReadFile(repo, "v1.0.0", "SPEC.md")
	require.NoError(t, err)
	require.Equal(t, "v1.0 spec", string(data))

	files, err := gitListFiles(repo, "v1.1.0", "fixtures")
	require.NoError(t, err)
	require.Equal(t, []string{"fixtures/a.json"}, files)
}

func writeAndTag(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	write("speclib.toml", "[library]\nname = \"demo\"\n[files]\nprompt = \"PROMPT.md\"\nspec = \"SPEC.md\"\nfixtures = \"fixtures/\"\n")
	write("PROMPT.md", "prompt")
	write("SPEC.md", "spec v1.1")
	run("add", "-A")
	run("commit", "-qm", "full lib")
	run("tag", "-f", "v1.1.0")
}

// makeRepoTwoLibs creates a git repo where two tags (v1.0.0, v2.0.0) each carry
// a complete spec-library (speclib.toml + PROMPT.md + SPEC.md + fixtures/a.json),
// differing only in SPEC.md content ("spec v1.0" vs "spec v2.0"). Used to test
// acquireGit's explicit-version selection (the `explicit` argument to Acquire),
// which otherwise has no coverage: both existing Acquire tests pass explicit="".
func makeRepoTwoLibs(t *testing.T) string {
	t.Helper()
	// Sandbox the git mirror cache; see makeRepo above for why.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	libToml := "[library]\nname = \"demo\"\n[files]\nprompt = \"PROMPT.md\"\nspec = \"SPEC.md\"\nfixtures = \"fixtures/\"\n"

	run("init", "-q")
	write("speclib.toml", libToml)
	write("PROMPT.md", "prompt")
	write("SPEC.md", "spec v1.0")
	write("fixtures/a.json", "1")
	run("add", "-A")
	run("commit", "-qm", "v1.0.0")
	run("tag", "v1.0.0")

	write("SPEC.md", "spec v2.0")
	run("add", "-A")
	run("commit", "-qm", "v2.0.0")
	run("tag", "v2.0.0")

	return dir
}
