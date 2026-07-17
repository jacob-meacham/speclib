package manifest

import (
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
