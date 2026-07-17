package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndToEndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib")
	writeDemoLib(t, lib)

	_, err := runSyncWithStub(t, dir, "init", "--agent", "claude")
	require.NoError(t, err)

	_, err = runSyncWithStub(t, dir, "add", lib, "--path", "gen/demo", "--lang", "go")
	require.NoError(t, err)

	// pending before sync
	out, err := runSyncWithStub(t, dir, "status")
	require.NoError(t, err)
	require.Contains(t, out, "pending")

	_, err = runSyncWithStub(t, dir, "sync")
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(dir, "gen", "demo", "GENERATED.md"))

	// up-to-date after sync
	out, err = runSyncWithStub(t, dir, "status")
	require.NoError(t, err)
	require.Contains(t, out, "up-to-date")

	// verify passes (stub records `true`)
	_, err = runSyncWithStub(t, dir, "verify")
	require.NoError(t, err)

	// second sync is a no-op
	out, err = runSyncWithStub(t, dir, "sync")
	require.NoError(t, err)
	require.Contains(t, out, "Nothing to sync")
}
