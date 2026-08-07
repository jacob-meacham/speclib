package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifestAgentRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "speclib.toml")
	m := &Manifest{
		Agent:        &Agent{Command: "claude", Permissions: []string{"--dangerously-skip-permissions"}},
		Dependencies: map[string]Dependency{},
	}
	require.NoError(t, m.Save(path))
	got, err := Load(path)
	require.NoError(t, err)
	require.NotNil(t, got.Agent)
	require.Equal(t, "claude", got.AgentCommand())
	require.Equal(t, []string{"--dangerously-skip-permissions"}, got.AgentPermissions())
}

func TestManifestAgentAbsentYieldsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "speclib.toml")
	require.NoError(t, os.WriteFile(path, []byte("[project]\nlanguage = \"go\"\n"), 0o644))
	m, err := Load(path)
	require.NoError(t, err)
	require.Nil(t, m.Agent)
	require.Equal(t, "", m.AgentCommand())
	require.Nil(t, m.AgentPermissions())
}

func TestManifestSaveWithoutAgentOmitsSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "speclib.toml")
	m := &Manifest{Dependencies: map[string]Dependency{}}
	require.NoError(t, m.Save(path))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(data), "[agent]")
}
