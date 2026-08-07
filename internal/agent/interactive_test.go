package agent

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLaunchInteractivePassesInstructionsAndWiresStdio(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args.txt")
	t.Setenv("FAKE_ARGS", argsFile)
	writeFakeClaude(t, `printf '%s' "$1" > "$FAKE_ARGS"
echo "agent ui"
`)
	a, _ := Lookup("claude")
	var out, errOut strings.Builder
	err := a.LaunchInteractive("THE INSTRUCTIONS", strings.NewReader(""), &out, &errOut)
	require.NoError(t, err)
	require.Equal(t, "agent ui\n", out.String())
	got, readErr := os.ReadFile(argsFile)
	require.NoError(t, readErr)
	require.Equal(t, "THE INSTRUCTIONS", string(got))
}

func TestLaunchInteractiveMissingBinaryErrors(t *testing.T) {
	a := Adapter{Name: "claude", Bin: "no-such-agent-xyz"}
	err := a.LaunchInteractive("x", strings.NewReader(""), io.Discard, io.Discard)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no-such-agent-xyz not found on PATH")
	require.Contains(t, err.Error(), "--headless")
}
