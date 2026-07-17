package lockfile

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestState(t *testing.T) {
	require.Equal(t, Pending, Package{Commit: "aaa"}.State())
	require.Equal(t, UpgradePending, Package{Commit: "bbb", GeneratedCommit: "aaa"}.State())
	require.Equal(t, UpToDate, Package{Commit: "aaa", GeneratedCommit: "aaa"}.State())
}

func TestUpsertAndFind(t *testing.T) {
	l := &Lockfile{}
	l.Upsert(Package{Name: "a", Version: "1.0.0"})
	l.Upsert(Package{Name: "a", Version: "1.1.0"}) // replaces
	require.Len(t, l.Packages, 1)
	p, ok := l.Find("a")
	require.True(t, ok)
	require.Equal(t, "1.1.0", p.Version)
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "speclib.lock")
	l := &Lockfile{Packages: []Package{{Name: "a", Commit: "c1", GeneratedCommit: "c1", TestCommand: "npm test"}}}
	require.NoError(t, l.Save(path))

	got, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, l.Packages, got.Packages)
	require.Equal(t, UpToDate, got.Packages[0].State())
}

func TestRemove(t *testing.T) {
	l := &Lockfile{Packages: []Package{{Name: "a"}, {Name: "b"}}}
	l.Remove("a")
	_, ok := l.Find("a")
	require.False(t, ok)
	require.Len(t, l.Packages, 1)
}
