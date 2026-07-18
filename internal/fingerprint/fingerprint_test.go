package fingerprint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
}

func TestHashDirOrderIndependent(t *testing.T) {
	a := t.TempDir()
	writeFiles(t, a, map[string]string{
		"a.go":        "package a",
		"sub/b.go":    "package b",
		"sub/c/d.txt": "hello",
	})
	b := t.TempDir()
	// write in a different order than a
	writeFiles(t, b, map[string]string{
		"sub/c/d.txt": "hello",
		"sub/b.go":    "package b",
		"a.go":        "package a",
	})

	ha, err := HashDir(a)
	require.NoError(t, err)
	hb, err := HashDir(b)
	require.NoError(t, err)
	require.Equal(t, ha, hb)
	require.True(t, strings.HasPrefix(ha, "sha256:"))
}

func TestHashDirChangesWithContent(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a.go": "package a"})
	h1, err := HashDir(dir)
	require.NoError(t, err)

	writeFiles(t, dir, map[string]string{"a.go": "package a2"})
	h2, err := HashDir(dir)
	require.NoError(t, err)
	require.NotEqual(t, h1, h2)
}

func TestHashDirChangesWithFilename(t *testing.T) {
	dir1 := t.TempDir()
	writeFiles(t, dir1, map[string]string{"a.go": "same"})
	dir2 := t.TempDir()
	writeFiles(t, dir2, map[string]string{"b.go": "same"})

	h1, err := HashDir(dir1)
	require.NoError(t, err)
	h2, err := HashDir(dir2)
	require.NoError(t, err)
	require.NotEqual(t, h1, h2)
}

func TestHashDirMissingReturnsEmpty(t *testing.T) {
	h, err := HashDir(filepath.Join(t.TempDir(), "does-not-exist"))
	require.NoError(t, err)
	require.Equal(t, "", h)
}
