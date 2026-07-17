package syncplan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmeacham/speclib/internal/lockfile"
	"github.com/jmeacham/speclib/internal/manifest"
	"github.com/jmeacham/speclib/internal/spec"
	"github.com/stretchr/testify/require"
)

func writeDemoLib(t *testing.T, dir string) {
	t.Helper()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	write("speclib.toml", "[library]\nname = \"demo\"\n[files]\nprompt = \"PROMPT.md\"\nspec = \"SPEC.md\"\nfixtures = \"fixtures/\"\n")
	write("PROMPT.md", "prompt")
	write("SPEC.md", "spec")
	write("fixtures/a.json", "1")
}

func TestComputeReturnsPendingAndUpgradePending(t *testing.T) {
	m := &manifest.Manifest{Dependencies: map[string]manifest.Dependency{
		"demo": {Path: "gen/demo"}, "upg": {Path: "gen/upg"}, "done": {Path: "gen/done"},
	}}
	l := &lockfile.Lockfile{Packages: []lockfile.Package{
		{Name: "demo", Commit: "c1"},                        // pending
		{Name: "upg", Commit: "c2", GeneratedCommit: "c1"},  // upgrade-pending
		{Name: "done", Commit: "c2", GeneratedCommit: "c2"}, // up-to-date
	}}
	work, err := Compute(m, l, "")
	require.NoError(t, err)
	require.Len(t, work, 2)
	names := []string{work[0].Name, work[1].Name}
	require.ElementsMatch(t, []string{"demo", "upg"}, names)

	// only filter: named pending package returns just that package.
	only, err := Compute(m, l, "demo")
	require.NoError(t, err)
	require.Len(t, only, 1)
	require.Equal(t, "demo", only[0].Name)

	// only filter: an upgrade-pending package is included.
	upg, err := Compute(m, l, "upg")
	require.NoError(t, err)
	require.Len(t, upg, 1)
	require.Equal(t, "upg", upg[0].Name)

	// only filter: naming an up-to-date package still filters it out.
	upToDate, err := Compute(m, l, "done")
	require.NoError(t, err)
	require.Empty(t, upToDate)

	// only filter: naming a package that doesn't exist yields nothing.
	missing, err := Compute(m, l, "nonexistent")
	require.NoError(t, err)
	require.Empty(t, missing)
}

// TestComputeExcludesOrphanLockfileEntries covers a lockfile package with no
// matching manifest entry (e.g. hand-edited state). It can never be
// generated (there is no source to fetch), so Compute must exclude it rather
// than hand it to Materialize, which would try to acquire an empty source.
func TestComputeExcludesOrphanLockfileEntries(t *testing.T) {
	m := &manifest.Manifest{Dependencies: map[string]manifest.Dependency{
		"demo": {Path: "gen/demo"},
	}}
	l := &lockfile.Lockfile{Packages: []lockfile.Package{
		{Name: "demo", Commit: "c1"},   // pending, in manifest
		{Name: "orphan", Commit: "c2"}, // pending, but no manifest entry
	}}
	work, err := Compute(m, l, "")
	require.NoError(t, err)
	require.Len(t, work, 1)
	require.Equal(t, "demo", work[0].Name)

	// Naming the orphan directly must not surface it either.
	only, err := Compute(m, l, "orphan")
	require.NoError(t, err)
	require.Empty(t, only)
}

// makeGitLibRepo builds a git repo whose v1.0.0 and v2.0.0 tags each carry a
// full spec-library differing only in SPEC.md ("spec v1.0" vs "spec v2.0"),
// and returns the repo path plus the two tag commit SHAs. The XDG cache is
// sandboxed so the source package's git mirror stays out of the real ~/.cache.
func makeGitLibRepo(t *testing.T) (repo, v1, v2 string) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := t.TempDir()
	git := func(args ...string) string {
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

	git("init", "-q")
	write("speclib.toml", libToml)
	write("PROMPT.md", "prompt")
	write("SPEC.md", "spec v1.0")
	write("fixtures/a.json", "1")
	git("add", "-A")
	git("commit", "-qm", "v1.0.0")
	git("tag", "v1.0.0")
	v1 = git("rev-list", "-n", "1", "v1.0.0")

	write("SPEC.md", "spec v2.0")
	git("add", "-A")
	git("commit", "-qm", "v2.0.0")
	git("tag", "v2.0.0")
	v2 = git("rev-list", "-n", "1", "v2.0.0")

	return dir, v1, v2
}

func TestMaterializeUpgradePending(t *testing.T) {
	repo, v1, v2 := makeGitLibRepo(t)
	work := t.TempDir()

	dep := manifest.Dependency{Source: repo, Version: "*", Path: "gen/demo", Language: "go"}
	// GeneratedCommit at v1, resolved Commit at v2 => upgrade-pending.
	pkg := lockfile.Package{
		Name: "demo", Source: repo, Version: "2.0.0", Commit: v2,
		GeneratedCommit: v1, Selections: "channels=roku", Language: "go", Path: "gen/demo",
	}
	require.Equal(t, lockfile.UpgradePending, pkg.State())

	item, err := Materialize(work, dep, pkg)
	require.NoError(t, err)
	require.Equal(t, "upgrade-pending", item.State)
	require.Equal(t, v1, item.FromCommit)
	require.Equal(t, "2.0.0", item.ToVersion)
	require.Equal(t, v2, item.ToCommit)
	require.Equal(t, "channels=roku", item.Selections)

	// Full new spec is materialized (SPEC.md is the v2 content).
	specMd, err := os.ReadFile(filepath.Join(item.SpecDir, "SPEC.md"))
	require.NoError(t, err)
	require.Equal(t, "spec v2.0", string(specMd))

	// SPEC.diff is written and reflects the spec change.
	require.Equal(t, filepath.Join(item.SpecDir, "SPEC.diff"), item.SpecDiffPath)
	diff, err := os.ReadFile(item.SpecDiffPath)
	require.NoError(t, err)
	require.Contains(t, string(diff), "spec v1.0")
	require.Contains(t, string(diff), "spec v2.0")
}

// TestFixtureDestRejectsEscape guards against a malicious or malformed local
// library whose fixtures path yields a spec.File escaping specDir (e.g. via
// "../"), which would otherwise let Materialize write outside .speclib/work.
func TestFixtureDestRejectsEscape(t *testing.T) {
	specDir := filepath.Join(t.TempDir(), "demo")

	_, err := fixtureDest(specDir, spec.File{Path: "../escape.json"})
	require.Error(t, err)

	_, err = fixtureDest(specDir, spec.File{Path: "fixtures/../../escape.json"})
	require.Error(t, err)

	dst, err := fixtureDest(specDir, spec.File{Path: "fixtures/a.json"})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(specDir, "fixtures/a.json"), dst)
}

func TestMaterialize(t *testing.T) {
	libDir := t.TempDir()
	writeDemoLib(t, libDir)
	work := t.TempDir()

	dep := manifest.Dependency{Source: libDir, Version: "*", Path: "gen/demo", Language: "go", Context: "speclib/demo.md"}
	pkg := lockfile.Package{Name: "demo", Source: libDir, Version: "0.0.0+local", Language: "go", Path: "gen/demo"}

	item, err := Materialize(work, dep, pkg)
	require.NoError(t, err)
	require.Equal(t, "demo", item.Name)
	require.Equal(t, "pending", item.State)
	require.Equal(t, "gen/demo", item.TargetPath)
	require.Equal(t, "go", item.Language)
	require.Equal(t, "speclib/demo.md", item.ContextFile)
	require.Equal(t, "demo", filepath.Base(item.SpecDir))
	require.FileExists(t, filepath.Join(item.SpecDir, "PROMPT.md"))
	require.FileExists(t, filepath.Join(item.SpecDir, "SPEC.md"))
	require.FileExists(t, filepath.Join(item.SpecDir, "fixtures", "a.json"))
	// Pending items carry no upgrade metadata and produce no diff.
	require.Empty(t, item.FromCommit)
	require.Empty(t, item.ToVersion)
	require.Empty(t, item.ToCommit)
	require.Empty(t, item.SpecDiffPath)
	require.NoFileExists(t, filepath.Join(item.SpecDir, "SPEC.diff"))
}
