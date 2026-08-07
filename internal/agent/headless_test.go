package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// writeFakeClaude installs a scripted `claude` at the front of PATH. The
// real claude must never run in tests; every headless test goes through a
// fake like this.
func writeFakeClaude(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\n"+script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestHeadlessGenerateStreamsProgressAndParsesResult(t *testing.T) {
	writeFakeClaude(t, `
echo '{"type":"system","subtype":"init"}'
echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"gen/demo/demo.go"}}]}}'
echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./gen/demo"}}]}}'
printf '{"type":"result","subtype":"success","result":"done\\nRESULT go test ./gen/demo || pass"}\n'
`)
	var progress strings.Builder
	h := HeadlessClaude{Progress: &progress}
	res, err := h.Generate(context.Background(), Request{SpecDir: "s", Language: "go", TargetPath: "t"})
	require.NoError(t, err)
	require.Equal(t, Result{TestCommand: "go test ./gen/demo", FixtureStatus: "pass"}, res)
	require.Equal(t, "  [tool] Write gen/demo/demo.go\n  [tool] Bash: go test ./gen/demo\n", progress.String())
}

func TestHeadlessGenerateTimesOut(t *testing.T) {
	writeFakeClaude(t, "sleep 5 &\nsleep 5\n")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := HeadlessClaude{}.Generate(ctx, Request{SpecDir: "s", Language: "go", TargetPath: "t"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "timed out")
	require.Less(t, time.Since(start), 2*time.Second, "timeout must bound wall-clock time")
}

func TestHeadlessGenerateNoResultLineErrorsWithTail(t *testing.T) {
	writeFakeClaude(t, `
echo '{"type":"result","subtype":"success","result":"I could not finish"}'
`)
	_, err := HeadlessClaude{}.Generate(context.Background(), Request{SpecDir: "s", Language: "go", TargetPath: "t"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not parse RESULT line")
	require.Contains(t, err.Error(), "I could not finish")
}

// TestHeadlessGenerateScanErrorKillsProcessAndErrors covers a scan failure
// (e.g. bufio.ErrTooLong from a line past the scanner's max buffer) that
// previously fell through the loop silently and surfaced as a misleading
// timeout or "could not parse RESULT line" error. It must instead be
// reported for what it is, and the child process group must be killed
// promptly rather than left to the trailing sleep below.
func TestHeadlessGenerateScanErrorKillsProcessAndErrors(t *testing.T) {
	writeFakeClaude(t, `
dd if=/dev/zero bs=11000000 count=1 2>/dev/null | tr '\0' 'a'
echo
sleep 5
`)
	start := time.Now()
	_, err := HeadlessClaude{}.Generate(context.Background(), Request{SpecDir: "s", Language: "go", TargetPath: "t"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "reading claude output")
	require.Less(t, time.Since(start), 3*time.Second, "a scan error must kill the process promptly, not wait out the trailing sleep")
}

func TestHeadlessGenerateMissingBinaryErrors(t *testing.T) {
	_, err := HeadlessClaude{Adapter: Adapter{Name: "claude", Bin: "no-such-agent-xyz"}}.
		Generate(context.Background(), Request{SpecDir: "s", Language: "go", TargetPath: "t"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no-such-agent-xyz not found on PATH")
}

func TestProgressLine(t *testing.T) {
	require.Equal(t, "  [tool] Bash: go test", progressLine("Bash", "", "go test"))
	require.Equal(t, "  [tool] Write a/b.go", progressLine("Write", "a/b.go", ""))
	require.Equal(t, "  [tool] WebSearch", progressLine("WebSearch", "", ""))
}

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

func TestBuildPromptIncludesChecksInOrder(t *testing.T) {
	p := buildPrompt(Request{
		SpecDir: ".speclib/work/demo", Language: "go", TargetPath: "gen/demo",
		Checks: []string{"go build ./...", "go vet ./..."},
	})
	require.Contains(t, p, "go build ./...; go vet ./...")
	require.Contains(t, p, "exits 0")
	require.Contains(t, p, "RESULT <test-command> || <pass|skip|fail>")
}

func TestBuildPromptOmitsChecksWhenUndeclared(t *testing.T) {
	p := buildPrompt(Request{SpecDir: "s", Language: "go", TargetPath: "t"})
	require.NotContains(t, p, "exits 0")
	require.Contains(t, p, "RESULT <test-command> || <pass|skip|fail>")
}
