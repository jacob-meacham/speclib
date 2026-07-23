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
	cmd := exec.CommandContext(ctx, "claude", "-p", buildPrompt(req))
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

// buildPrompt renders the headless generation prompt. Kept separate from
// Generate so the prompt contract is testable without spawning claude.
func buildPrompt(req Request) string {
	checks := ""
	if len(req.Checks) > 0 {
		checks = fmt.Sprintf(" Before writing the test, run each project check"+
			" in order and fix failures until every one exits 0: %s.",
			strings.Join(req.Checks, "; "))
	}
	return fmt.Sprintf(
		"Read the spec in %s (PROMPT.md, SPEC.md, fixtures). Generate a %s "+
			"implementation into %s.%s Write a fixture-driven test, run it until it "+
			"passes, then print a final line exactly: RESULT <test-command> || <pass|skip|fail>.",
		req.SpecDir, req.Language, req.TargetPath, checks)
}

func parseResultLine(out string) (testCmd, fixtureStatus string) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if rest, ok := strings.CutPrefix(line, "RESULT "); ok {
			// Split on the last " || " so a test command that itself
			// contains " || " (e.g. a shell fallback) is not mis-split.
			idx := strings.LastIndex(rest, " || ")
			if idx >= 0 {
				return strings.TrimSpace(rest[:idx]), strings.TrimSpace(rest[idx+len(" || "):])
			}
		}
	}
	return "", ""
}
