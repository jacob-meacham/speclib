package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "speclib.toml")

	m := &Manifest{
		Project: Project{Language: "typescript"},
		Dependencies: map[string]Dependency{
			"roku-deeplink": {Source: "https://x/roku", Version: "^1.4", Path: "src/gen/roku"},
		},
	}
	require.NoError(t, m.Save(path))

	got, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, m.Dependencies["roku-deeplink"], got.Dependencies["roku-deeplink"])
	require.Equal(t, "typescript", got.Project.Language)
}

func TestChecksRoundTripAndDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "speclib.toml")

	m := &Manifest{
		Project: Project{Language: "rust", Checks: []string{
			"cargo build",
			"cargo clippy --all-targets -- -D warnings",
			"cargo fmt --check",
		}},
		Dependencies: map[string]Dependency{},
	}
	require.NoError(t, m.Save(path))

	got, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, []string{
		"cargo build",
		"cargo clippy --all-targets -- -D warnings",
		"cargo fmt --check",
	}, got.Project.Checks)

	// A manifest that never mentions checks loads with none.
	bare := filepath.Join(dir, "bare.toml")
	require.NoError(t, os.WriteFile(bare, []byte("[project]\nlanguage = \"go\"\n"), 0o644))
	got2, err := Load(bare)
	require.NoError(t, err)
	require.Empty(t, got2.Project.Checks)
}

func TestLoadMissingReturnsEmpty(t *testing.T) {
	m, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Empty(t, m.Dependencies)
}

func TestLanguageForFallsBackToProject(t *testing.T) {
	m := &Manifest{
		Project:      Project{Language: "go"},
		Dependencies: map[string]Dependency{"a": {}, "b": {Language: "rust"}},
	}
	require.Equal(t, "go", m.LanguageFor("a"))
	require.Equal(t, "rust", m.LanguageFor("b"))
}

func TestParseLibrary(t *testing.T) {
	lib, err := ParseLibrary([]byte(`
[library]
name = "roku-deeplink"
summary = "Roku ECP deep-linking"
[files]
prompt = "PROMPT.md"
spec = "SPEC.md"
fixtures = "fixtures/"
[hints]
languages = ["kotlin", "python"]
`))
	require.NoError(t, err)
	require.Equal(t, "roku-deeplink", lib.Meta.Name)
	require.Equal(t, "PROMPT.md", lib.Files.Prompt)
	require.Equal(t, []string{"kotlin", "python"}, lib.Hints.Languages)
}

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
