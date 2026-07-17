package agent

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStubBackendWritesFile(t *testing.T) {
	dir := t.TempDir()
	res, err := StubBackend{}.Generate(context.Background(), Request{
		Name: "demo", TargetPath: filepath.Join(dir, "gen"), Language: "go",
	})
	require.NoError(t, err)
	require.Equal(t, "true", res.TestCommand)
	require.FileExists(t, filepath.Join(dir, "gen", "GENERATED.md"))
}
