package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-meacham/speclib/internal/manifest"
	"github.com/stretchr/testify/require"
)

func writeLib(t *testing.T, dir string) {
	t.Helper()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	write("speclib.toml", `
[library]
name = "demo"
[files]
prompt = "PROMPT.md"
spec = "SPEC.md"
fixtures = "fixtures/"
`)
	write("PROMPT.md", "prompt")
	write("SPEC.md", "spec body")
	write("fixtures/a.json", "1")
	write("fixtures/b.json", "2")
}

func TestAcquireLocal(t *testing.T) {
	dir := t.TempDir()
	writeLib(t, dir)

	res, lib, sp, err := Acquire(Parse(dir), "*", "")
	require.NoError(t, err)
	require.Equal(t, "demo", lib.Meta.Name)
	require.Equal(t, "local", res.Commit)
	require.Equal(t, "prompt", string(sp.Prompt))
	require.Equal(t, "spec body", string(sp.SpecDoc))
	require.Len(t, sp.Fixtures, 2)
}

func TestAcquireGit(t *testing.T) {
	repo := makeRepoWithLib(t) // defined below in this file
	// NOTE: deliberately not Parse(repo). Parse's contract (Task 5) treats any
	// absolute filesystem path as a local source, which would route this
	// through acquireLocal and never exercise git tag resolution. Building the
	// Ref directly exercises acquireGit against a local repo standing in for a
	// remote (git itself supports local paths as clone sources, as git_test.go
	// already relies on for gitTags/gitReadFile/etc).
	ref := Ref{Raw: repo, IsLocal: false, Location: repo}
	res, lib, sp, err := Acquire(ref, "^1", "")
	require.NoError(t, err)
	require.Equal(t, "1.1.0", res.Version)
	require.Len(t, res.Commit, 40)
	require.Equal(t, "demo", lib.Meta.Name)
	require.Equal(t, "spec v1.1", string(sp.SpecDoc))
}

// makeRepoWithLib builds a git repo containing a full spec-library at v1.0.0 and v1.1.0.
func makeRepoWithLib(t *testing.T) string {
	t.Helper()
	dir := makeRepo(t) // from git_test.go, already has SPEC.md + fixtures + tags
	// add a library manifest + PROMPT on top and retag as v1.1.0 content
	// (makeRepo already tagged v1.1.0 with SPEC.md="v1.1 spec"; overwrite for a complete lib)
	writeAndTag(t, dir)
	return dir
}

// TestAcquireGitExplicit covers acquireGit's explicit != "" branch, which has
// no other coverage: TestAcquireGit above passes explicit="". This branch
// matters because later tasks depend on it — `add <src>@<ver>` and, critically,
// `sync` re-fetching every git dep via Acquire(ref, dep.Version, pkg.Version)
// with the previously resolved version passed as explicit.
func TestAcquireGitExplicit(t *testing.T) {
	repo := makeRepoTwoLibs(t) // defined in git_test.go
	// Deliberately not Parse(repo) — see the comment in TestAcquireGit above.
	ref := Ref{Raw: repo, IsLocal: false, Location: repo}

	// explicit overrides the "highest available" default, which would pick v2.0.0.
	res, lib, sp, err := Acquire(ref, "*", "1.0.0")
	require.NoError(t, err)
	require.Equal(t, "1.0.0", res.Version)
	require.Equal(t, "demo", lib.Meta.Name)
	require.Equal(t, "spec v1.0", string(sp.SpecDoc))

	// A "v"-prefixed explicit version normalizes the same way ("v"+TrimPrefix(explicit, "v")).
	res2, _, sp2, err := Acquire(ref, "*", "v1.0.0")
	require.NoError(t, err)
	require.Equal(t, "1.0.0", res2.Version)
	require.Equal(t, "spec v1.0", string(sp2.SpecDoc))
}

// TestSpecDiff checks that SpecDiff returns a unified diff of the spec files
// between two commits, and that an empty/"local" commit yields no diff.
func TestSpecDiff(t *testing.T) {
	repo := makeRepoTwoLibs(t) // SPEC.md: "spec v1.0" @v1.0.0 -> "spec v2.0" @v2.0.0
	ref := Ref{Raw: repo, IsLocal: false, Location: repo}

	from, err := gitResolveCommit(repo, "v1.0.0")
	require.NoError(t, err)
	to, err := gitResolveCommit(repo, "v2.0.0")
	require.NoError(t, err)

	files := manifest.Files{Prompt: "PROMPT.md", Spec: "SPEC.md", Fixtures: "fixtures/"}
	diff, err := SpecDiff(ref, from, to, files)
	require.NoError(t, err)
	require.Contains(t, diff, "SPEC.md")
	require.Contains(t, diff, "spec v1.0") // removed line
	require.Contains(t, diff, "spec v2.0") // added line

	// No diff available when either commit is empty or the local sentinel.
	empty, err := SpecDiff(ref, "", to, files)
	require.NoError(t, err)
	require.Empty(t, empty)
	local, err := SpecDiff(ref, "local", to, files)
	require.NoError(t, err)
	require.Empty(t, local)
}

// TestAcquireAtCommitIgnoresMovedTag proves the whole reason AcquireAtCommit
// exists: resolution pins an immutable commit SHA, so if the source's tag is
// later force-moved to different content (e.g. an upstream rewrite between
// `lock`/`update` and `sync`), reading at the pinned commit must still return
// the original content, not whatever the tag currently points at.
func TestAcquireAtCommitIgnoresMovedTag(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
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
	write("SPEC.md", "spec A")
	write("fixtures/a.json", "1")
	run("add", "-A")
	run("commit", "-qm", "v1.0.0")
	run("tag", "v1.0.0")
	pinned := run("rev-list", "-n", "1", "v1.0.0")

	// Force-move v1.0.0 to a new commit whose SPEC.md differs.
	write("SPEC.md", "spec B")
	run("add", "-A")
	run("commit", "-qm", "moved")
	run("tag", "-f", "v1.0.0")

	// Not Parse(dir) — see the comment in TestAcquireGit above for why: this
	// exercises acquireGit's read path against a local repo standing in for a
	// remote, same as the existing Acquire tests.
	ref := Ref{Raw: dir, IsLocal: false, Location: dir}
	lib, sp, err := AcquireAtCommit(ref, pinned)
	require.NoError(t, err)
	require.Equal(t, "demo", lib.Meta.Name)
	require.Equal(t, "spec A", string(sp.SpecDoc))
}

// TestAcquireAtCommitServesCachedCommitWithoutFetch: a pinned commit is
// immutable, so when the mirror cache already contains it AcquireAtCommit
// must serve reads from the cache without touching the origin. Removing the
// origin after warming the cache makes any clone/fetch attempt fail loudly —
// this is the regression guard for `speclib sync` silently refetching every
// source (and hanging on a slow network) before producing any output.
func TestAcquireAtCommitServesCachedCommitWithoutFetch(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	repo := makeRepoWithLib(t)
	ref := Ref{Raw: repo, IsLocal: false, Location: repo}

	res, _, _, err := Acquire(ref, "^1", "") // warms the mirror cache
	require.NoError(t, err)

	require.NoError(t, os.RemoveAll(repo)) // origin gone: any fetch now fails
	forgetFetch(repo)                      // a fresh process has no in-process fetch mark

	lib, sp, err := AcquireAtCommit(ref, res.Commit)
	require.NoError(t, err)
	require.Equal(t, "demo", lib.Meta.Name)
	require.Equal(t, "spec v1.1", string(sp.SpecDoc))
}

// TestAcquireAtCommitLocalFallback covers the non-git local source case: with
// no commits to pin against (acquireLocal always reports the "local"
// sentinel), AcquireAtCommit ignores the given commit and reads the current
// working tree, same as Acquire does for that source.
func TestAcquireAtCommitLocalFallback(t *testing.T) {
	dir := t.TempDir()
	writeLib(t, dir)

	lib, sp, err := AcquireAtCommit(Parse(dir), "local")
	require.NoError(t, err)
	require.Equal(t, "demo", lib.Meta.Name)
	require.Equal(t, "spec body", string(sp.SpecDoc))
}

// TestAcquireLocalGitRepoUsesTags covers the local-versioning fix: a path that
// Parse classifies as local but which is a git repo with semver tags resolves
// to the tag version + a real commit SHA (not "0.0.0+local"/"local"), giving
// version pinning from a local path without needing a remote.
func TestAcquireLocalGitRepoUsesTags(t *testing.T) {
	repo := makeRepoWithLib(t) // git repo whose v1.1.0 tag carries a full library
	ref := Parse(repo)
	require.True(t, ref.IsLocal) // sanity: an absolute path is classified local...

	res, lib, sp, err := Acquire(ref, "*", "")
	require.NoError(t, err)
	// ...yet because it is a git repo with tags, it resolves via git tags.
	require.Equal(t, "1.1.0", res.Version)
	require.Len(t, res.Commit, 40)
	require.NotEqual(t, "local", res.Commit)
	require.Equal(t, "demo", lib.Meta.Name)
	require.Equal(t, "spec v1.1", string(sp.SpecDoc))
}
