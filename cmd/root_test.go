package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRootCommandHasName(t *testing.T) {
	root := newRootCmd()
	require.Equal(t, "speclib", root.Name())
}

func TestRootVersionPrintsInjectedVersion(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})
	require.NoError(t, root.Execute())
	require.Equal(t, "speclib version "+version+"\n", out.String())
	require.Equal(t, "0.0.0-dev", version) // release builds override via ldflags
}

func TestRootHelpRuns(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "speclib")
}
