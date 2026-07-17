package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseResultLineCommandContainingOrOr covers a test command that itself
// contains " || " (e.g. a shell fallback like "go test ./... || echo fail").
// The split must anchor on the last " || ", which separates the command from
// the trailing pass|skip|fail status, not the first one found in the command.
func TestParseResultLineCommandContainingOrOr(t *testing.T) {
	out := "some preamble\nRESULT go test ./... || echo fail || pass\n"
	tc, fs := parseResultLine(out)
	require.Equal(t, "go test ./... || echo fail", tc)
	require.Equal(t, "pass", fs)
}

func TestParseResultLineSimple(t *testing.T) {
	tc, fs := parseResultLine("RESULT go test ./... || pass")
	require.Equal(t, "go test ./...", tc)
	require.Equal(t, "pass", fs)
}

func TestParseResultLineNoMatchReturnsEmpty(t *testing.T) {
	tc, fs := parseResultLine("no result line here")
	require.Empty(t, tc)
	require.Empty(t, fs)
}
