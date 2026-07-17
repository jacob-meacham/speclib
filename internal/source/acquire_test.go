package source

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmeacham/speclib/internal/manifest"
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
