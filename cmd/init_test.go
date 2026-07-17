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
