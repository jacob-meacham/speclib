package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func runCmd(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"--chdir", dir}, args...))
	err := root.Execute()
	return out.String(), err
}

func TestInitCreatesManifestAndAgent(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, dir, "init", "--agent", "claude")
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(dir, "speclib.toml"))
	skill := filepath.Join(dir, ".claude", "skills", "speclib-sync", "SKILL.md")
	require.FileExists(t, skill)

	data, _ := os.ReadFile(skill)
	require.Contains(t, string(data), "speclib sync --plan")
}

func TestInitWithoutAgentOnlyScaffoldsManifest(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, dir, "init")
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(dir, "speclib.toml"))
	require.NoFileExists(t, filepath.Join(dir, ".claude", "skills", "speclib-sync", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(dir, ".cursor", "rules", "speclib-sync.mdc"))
	require.NoFileExists(t, filepath.Join(dir, "AGENTS.md"))
}

func TestInitCreatesCursorAgent(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, dir, "init", "--agent", "cursor")
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(dir, "speclib.toml"))
	rule := filepath.Join(dir, ".cursor", "rules", "speclib-sync.mdc")
	require.FileExists(t, rule)

	data, _ := os.ReadFile(rule)
	require.Contains(t, string(data), "alwaysApply: false")
	require.Contains(t, string(data), "speclib sync --plan")
}

func TestInitCreatesAgentsMD(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, dir, "init", "--agent", "agents")
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(dir, "speclib.toml"))
	agentsFile := filepath.Join(dir, "AGENTS.md")
	require.FileExists(t, agentsFile)

	data, _ := os.ReadFile(agentsFile)
	require.Contains(t, string(data), "## speclib")
	require.Contains(t, string(data), "speclib sync --plan")
}

func TestInitAgentsMDIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	agentsFile := filepath.Join(dir, "AGENTS.md")
	existing := "# AGENTS.md\n\n## Existing conventions\n\nUse tabs.\n"
	require.NoError(t, os.WriteFile(agentsFile, []byte(existing), 0o644))

	_, err := runCmd(t, dir, "init", "--agent", "agents")
	require.NoError(t, err)

	data, err := os.ReadFile(agentsFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "## Existing conventions")
	require.Contains(t, string(data), "## speclib")

	// Running init again must not duplicate the section.
	_, err = runCmd(t, dir, "init", "--agent", "agents")
	require.NoError(t, err)
	after, err := os.ReadFile(agentsFile)
	require.NoError(t, err)
	require.Equal(t, string(data), string(after))
}

func TestInitUnknownAgentErrors(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, dir, "init", "--agent", "foo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "foo")
	require.Contains(t, err.Error(), "claude")
	require.Contains(t, err.Error(), "cursor")
	require.Contains(t, err.Error(), "agents")
}
