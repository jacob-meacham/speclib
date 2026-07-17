package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmeacham/speclib/internal/lockfile"
	"github.com/jmeacham/speclib/internal/manifest"
	"github.com/stretchr/testify/require"
)

func TestAddLocalSource(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib")
	writeDemoLib(t, lib) // helper below

	_, err := runCmd(t, dir, "init")
	require.NoError(t, err)

	out, err := runCmd(t, dir, "add", lib, "--path", "gen/demo", "--lang", "go")
	require.NoError(t, err)
	require.Contains(t, out, "speclib sync")

	m, err := manifest.Load(filepath.Join(dir, "speclib.toml"))
	require.NoError(t, err)
	dep, ok := m.Dependencies["demo"]
	require.True(t, ok)
	require.Equal(t, "gen/demo", dep.Path)
	require.Equal(t, "go", dep.Language)

	l, err := lockfile.Load(filepath.Join(dir, "speclib.lock"))
	require.NoError(t, err)
	p, ok := l.Find("demo")
	require.True(t, ok)
	require.NotEmpty(t, p.SpecHash)
	require.Equal(t, lockfile.Pending, p.State()) // resolved but not generated
}

func TestSplitVersion(t *testing.T) {
	tests := []struct {
		in          string
		wantSrc     string
		wantVersion string
	}{
		{"lib@1.4.0", "lib", "1.4.0"},
		{"https://host/o/r", "https://host/o/r", ""},
		{"https://host/o/r@1.4.0", "https://host/o/r", "1.4.0"},
		// scp-like git remote: the '@' suffix contains '/', so it must NOT be
		// treated as a version separator.
		{"git@github.com:o/r.git", "git@github.com:o/r.git", ""},
		{"../local@1.0", "../local", "1.0"},
		// leading '@' is guarded by the i>0 check, so it is never split.
		{"@", "@", ""},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			src, version := splitVersion(tc.in)
			require.Equal(t, tc.wantSrc, src)
			require.Equal(t, tc.wantVersion, version)
		})
	}
}

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
