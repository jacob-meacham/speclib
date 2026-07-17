package syncplan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmeacham/speclib/internal/lockfile"
	"github.com/jmeacham/speclib/internal/manifest"
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

func TestComputeReturnsPending(t *testing.T) {
	m := &manifest.Manifest{Dependencies: map[string]manifest.Dependency{
		"demo": {Path: "gen/demo"}, "done": {Path: "gen/done"},
	}}
	l := &lockfile.Lockfile{Packages: []lockfile.Package{
		{Name: "demo", Commit: "c1"},                        // pending
		{Name: "done", Commit: "c2", GeneratedCommit: "c2"}, // up-to-date
	}}
	pend, err := Compute(m, l, "")
	require.NoError(t, err)
	require.Len(t, pend, 1)
	require.Equal(t, "demo", pend[0].Name)

	// only filter: named pending package returns just that package.
	only, err := Compute(m, l, "demo")
	require.NoError(t, err)
	require.Len(t, only, 1)
	require.Equal(t, "demo", only[0].Name)

	// only filter: naming an up-to-date package still filters it out.
	upToDate, err := Compute(m, l, "done")
	require.NoError(t, err)
	require.Empty(t, upToDate)

	// only filter: naming a package that doesn't exist yields nothing.
	missing, err := Compute(m, l, "nonexistent")
	require.NoError(t, err)
	require.Empty(t, missing)
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
}
