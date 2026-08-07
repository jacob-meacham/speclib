package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLookupDefaultsToClaude(t *testing.T) {
	a, err := Lookup("")
	require.NoError(t, err)
	require.Equal(t, "claude", a.Name)
	require.Equal(t, "claude", a.Bin)
	require.Equal(t, []string{"--allowedTools", "Write,Edit,Bash"}, a.DefaultPermissions)
}

func TestLookupUnknownListsSupported(t *testing.T) {
	_, err := Lookup("cursor")
	require.Error(t, err)
	require.Equal(t, `unknown agent "cursor": supported agents are claude`, err.Error())
}

func TestHeadlessArgsWithDefaultPermissions(t *testing.T) {
	a, _ := Lookup("claude")
	require.Equal(t,
		[]string{"-p", "PROMPT", "--output-format", "stream-json", "--verbose", "--allowedTools", "Write,Edit,Bash"},
		a.HeadlessArgs("PROMPT", nil))
}

func TestHeadlessArgsWithOverriddenPermissions(t *testing.T) {
	a, _ := Lookup("claude")
	require.Equal(t,
		[]string{"-p", "PROMPT", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"},
		a.HeadlessArgs("PROMPT", []string{"--dangerously-skip-permissions"}))
}

func TestInteractiveArgs(t *testing.T) {
	a, _ := Lookup("claude")
	require.Equal(t, []string{"INSTRUCTIONS"}, a.InteractiveArgs("INSTRUCTIONS"))
}
