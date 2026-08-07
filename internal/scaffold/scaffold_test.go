package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteAgentClaude(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, WriteAgent(dir, "claude"))

	skill := filepath.Join(dir, ".claude", "skills", "speclib-sync", "SKILL.md")
	data, err := os.ReadFile(skill)
	require.NoError(t, err)

	body := string(data)
	require.Contains(t, body, "name: speclib-sync")
	require.Contains(t, body, "speclib sync --plan")
	require.Contains(t, body, "run the project's build/compile")
	require.Contains(t, body, "linter")
	require.Contains(t, body, "`checks`")
	require.Contains(t, body, "exits 0")
	require.Contains(t, body, "if a check failed or was skipped")
}

func TestWriteManifestIncludesChecksExample(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, WriteManifest(dir))

	data, err := os.ReadFile(filepath.Join(dir, "speclib.toml"))
	require.NoError(t, err)
	require.Contains(t, string(data), `# checks = ["go build ./...", "golangci-lint run"]`)
}

func TestWriteAgentCursor(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, WriteAgent(dir, "cursor"))

	rule := filepath.Join(dir, ".cursor", "rules", "speclib-sync.mdc")
	data, err := os.ReadFile(rule)
	require.NoError(t, err)

	body := string(data)
	require.Contains(t, body, "alwaysApply: false")
	require.Contains(t, body, "speclib sync --plan")
	require.Contains(t, body, "run the project's build/compile")
	require.Contains(t, body, "linter")
}

func TestWriteAgentAgentsFresh(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, WriteAgent(dir, "agents"))

	dst := filepath.Join(dir, "AGENTS.md")
	data, err := os.ReadFile(dst)
	require.NoError(t, err)

	body := string(data)
	require.Contains(t, body, "## speclib")
	require.Contains(t, body, "speclib sync --plan")
	require.Contains(t, body, "run the project's build/compile")
	require.Contains(t, body, "linter")
}

func TestWriteAgentAgentsAppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "AGENTS.md")
	existing := "# AGENTS.md\n\n## Some other tool\n\nDo the other thing.\n"
	require.NoError(t, os.WriteFile(dst, []byte(existing), 0o644))

	require.NoError(t, WriteAgent(dir, "agents"))

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	body := string(data)

	require.Contains(t, body, "## Some other tool")
	require.Contains(t, body, "Do the other thing.")
	require.Contains(t, body, "## speclib")
	require.Contains(t, body, "speclib sync --plan")
}

func TestWriteAgentAgentsIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, WriteAgent(dir, "agents"))

	dst := filepath.Join(dir, "AGENTS.md")
	first, err := os.ReadFile(dst)
	require.NoError(t, err)

	// Running again must not duplicate the section.
	require.NoError(t, WriteAgent(dir, "agents"))
	second, err := os.ReadFile(dst)
	require.NoError(t, err)

	require.Equal(t, string(first), string(second))
	require.Equal(t, 1, strings.Count(string(second), "## speclib"))
}

func TestWriteAgentUnknown(t *testing.T) {
	dir := t.TempDir()
	err := WriteAgent(dir, "foo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "foo")
	require.Contains(t, err.Error(), "claude")
	require.Contains(t, err.Error(), "cursor")
	require.Contains(t, err.Error(), "agents")
}

func TestWriteClaudeAgentStillWorks(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, WriteClaudeAgent(dir))
	require.FileExists(t, filepath.Join(dir, ".claude", "skills", "speclib-sync", "SKILL.md"))
}

func TestWriteManifestIncludesAgentExample(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, WriteManifest(dir))
	data, err := os.ReadFile(filepath.Join(dir, "speclib.toml"))
	require.NoError(t, err)
	require.Contains(t, string(data), "# [agent]")
	require.Contains(t, string(data), `# permissions = ["--allowedTools", "Write,Edit,Bash"]`)
}

func TestSyncInstructionsExposesCanonicalBody(t *testing.T) {
	body, err := SyncInstructions()
	require.NoError(t, err)
	require.Contains(t, body, "speclib sync --plan --json")
	require.Contains(t, body, "speclib sync --record")
}
