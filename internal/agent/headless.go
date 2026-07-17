package agent

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// HeadlessClaude drives generation non-interactively via `claude -p`.
// It is exercised via dogfood, not unit tests; it must never be used by tests.
type HeadlessClaude struct{}

func (HeadlessClaude) Generate(ctx context.Context, req Request) (Result, error) {
	prompt := fmt.Sprintf(
		"Read the spec in %s (PROMPT.md, SPEC.md, fixtures). Generate a %s "+
			"implementation into %s, write a fixture-driven test, run it until it "+
			"passes, then print a final line exactly: RESULT <test-command> || <pass|skip|fail>.",
		req.SpecDir, req.Language, req.TargetPath)
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Result{}, fmt.Errorf("claude: %v: %s", err, string(out))
	}
	tc, fs := parseResultLine(string(out))
	if tc == "" {
		return Result{}, fmt.Errorf("could not parse RESULT line from agent output")
	}
	return Result{TestCommand: tc, FixtureStatus: fs}, nil
}

func parseResultLine(out string) (testCmd, fixtureStatus string) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if rest, ok := strings.CutPrefix(line, "RESULT "); ok {
			parts := strings.SplitN(rest, " || ", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			}
		}
	}
	return "", ""
}
