package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jacob-meacham/speclib/internal/lockfile"
	"github.com/jacob-meacham/speclib/internal/manifest"
	"github.com/stretchr/testify/require"
)

func gitLib(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

// makeVersionedLib creates a git repo carrying a full spec-library tagged
// v1.0.0 then v1.1.0 (SPEC.md content differs per tag). Returns the repo path.
func makeVersionedLib(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	lib := "[library]\nname = \"demo\"\n[files]\nprompt = \"PROMPT.md\"\nspec = \"SPEC.md\"\nfixtures = \"fixtures/\"\n"
	gitLib(t, dir, "init", "-q")
	write("speclib.toml", lib)
	write("PROMPT.md", "prompt")
	write("SPEC.md", "spec v1.0")
	write("fixtures/a.json", "1")
	gitLib(t, dir, "add", "-A")
	gitLib(t, dir, "commit", "-qm", "v1.0.0")
	gitLib(t, dir, "tag", "v1.0.0")
	write("SPEC.md", "spec v1.1")
	gitLib(t, dir, "add", "-A")
	gitLib(t, dir, "commit", "-qm", "v1.1.0")
	gitLib(t, dir, "tag", "v1.1.0")
	return dir
}

func tagVersion(t *testing.T, dir, tag, spec string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte(spec), 0o644))
	gitLib(t, dir, "add", "-A")
	gitLib(t, dir, "commit", "-qm", tag)
	gitLib(t, dir, "tag", tag)
}

func TestUpdateMovesResolutionKeepsProvenance(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := t.TempDir()
	repo := makeVersionedLib(t)

	_, err := runCmd(t, dir, "init")
	require.NoError(t, err)
	_, err = runCmd(t, dir, "add", repo+"@1.1.0", "--path", "gen/demo", "--lang", "go")
	require.NoError(t, err)

	l, err := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	require.NoError(t, err)
	p, ok := l.Find("demo")
	require.True(t, ok)
	require.Equal(t, "1.1.0", p.Version)

	// Simulate a completed sync at the current commit.
	p.GeneratedCommit = p.Commit
	p.TestCommand = "go test ./gen/demo"
	p.Selections = "channels=roku"
	require.NoError(t, l.Save(filepath.Join(dir, "speclib.lock")))
	genCommit := p.Commit

	// Publish a newer version that ^1.1.0 still admits.
	tagVersion(t, repo, "v1.2.0", "spec v1.2")

	out, err := runCmd(t, dir, "update")
	require.NoError(t, err)
	require.Contains(t, out, "1.1.0 -> 1.2.0")
	require.Contains(t, out, "speclib sync")

	l2, err := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	require.NoError(t, err)
	p2, _ := l2.Find("demo")
	require.Equal(t, "1.2.0", p2.Version)           // resolution advanced
	require.NotEqual(t, genCommit, p2.Commit)       // ...to a new commit
	require.Equal(t, genCommit, p2.GeneratedCommit) // provenance untouched
	require.Equal(t, "go test ./gen/demo", p2.TestCommand)
	require.Equal(t, "channels=roku", p2.Selections)
	require.Equal(t, lockfile.UpgradePending, p2.State())

	// Running again is a no-op: already at the newest.
	out, err = runCmd(t, dir, "update")
	require.NoError(t, err)
	require.Contains(t, out, "already up to date")
}

func TestUpdateToUpdatesManifestConstraint(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := t.TempDir()
	repo := makeVersionedLib(t)

	_, err := runCmd(t, dir, "init")
	require.NoError(t, err)
	_, err = runCmd(t, dir, "add", repo+"@1.0.0", "--path", "gen/demo", "--lang", "go")
	require.NoError(t, err)

	m, err := manifest.Load(filepath.Join(dir, "speclib.toml"))
	require.NoError(t, err)
	require.Equal(t, "^1.0.0", m.Dependencies["demo"].Version)

	out, err := runCmd(t, dir, "update", "demo", "--to", "1.1.0")
	require.NoError(t, err)
	require.Contains(t, out, "1.0.0 -> 1.1.0")

	m2, err := manifest.Load(filepath.Join(dir, "speclib.toml"))
	require.NoError(t, err)
	require.Equal(t, "^1.1.0", m2.Dependencies["demo"].Version)

	l, err := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	require.NoError(t, err)
	p, _ := l.Find("demo")
	require.Equal(t, "1.1.0", p.Version)
}
