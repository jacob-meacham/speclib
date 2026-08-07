package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// HeadlessClaude drives generation non-interactively via the adapter's print
// mode (`claude -p`), streaming one tool-use progress line to Progress per
// event so the run is observable while it happens. The real claude binary is
// exercised via dogfood only; tests must always front PATH with a scripted
// fake.
type HeadlessClaude struct {
	Adapter     Adapter
	Permissions []string // replaces the adapter's default permission args when non-empty
	Progress    io.Writer
}

// streamEvent is the subset of `--output-format stream-json` events the
// headless path needs: assistant tool_use blocks (progress) and the final
// result event, whose text carries the RESULT line.
type streamEvent struct {
	Type    string `json:"type"`
	Result  string `json:"result"`
	Message struct {
		Content []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Input struct {
				FilePath string `json:"file_path"`
				Command  string `json:"command"`
			} `json:"input"`
		} `json:"content"`
	} `json:"message"`
}

func (h HeadlessClaude) Generate(ctx context.Context, req Request) (Result, error) {
	ad := h.Adapter
	if ad.Bin == "" {
		ad, _ = Lookup("")
	}
	progress := h.Progress
	if progress == nil {
		progress = io.Discard
	}
	if _, err := exec.LookPath(ad.Bin); err != nil {
		return Result{}, fmt.Errorf("%s not found on PATH", ad.Bin)
	}
	cmd := exec.CommandContext(ctx, ad.Bin, ad.HeadlessArgs(buildPrompt(req), h.Permissions)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("start %s: %w", ad.Bin, err)
	}
	// Keep the last 40 raw lines so every failure mode can show what the
	// agent actually said, without buffering a whole transcript.
	var tail []string
	keep := func(line string) {
		if len(tail) == 40 {
			tail = tail[1:]
		}
		tail = append(tail, line)
	}
	resultText := ""
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 10*1024*1024) // stream-json lines can be large
	for sc.Scan() {
		line := sc.Text()
		keep(line)
		var ev streamEvent
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue // non-JSON noise stays in the tail only
		}
		switch ev.Type {
		case "assistant":
			for _, c := range ev.Message.Content {
				if c.Type == "tool_use" {
					fmt.Fprintln(progress, progressLine(c.Name, c.Input.FilePath, c.Input.Command))
				}
			}
		case "result":
			resultText = ev.Result
		}
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return Result{}, fmt.Errorf("%s timed out; last output:\n%s", ad.Bin, transcriptTail(tail, &stderr))
	}
	if waitErr != nil {
		return Result{}, fmt.Errorf("%s: %v; last output:\n%s", ad.Bin, waitErr, transcriptTail(tail, &stderr))
	}
	tc, fs := parseResultLine(resultText)
	if tc == "" {
		return Result{}, fmt.Errorf("could not parse RESULT line from agent output; last output:\n%s", transcriptTail(tail, &stderr))
	}
	return Result{TestCommand: tc, FixtureStatus: fs}, nil
}

func transcriptTail(lines []string, stderr *bytes.Buffer) string {
	out := strings.Join(lines, "\n")
	if s := strings.TrimSpace(stderr.String()); s != "" {
		out += "\nstderr: " + s
	}
	return out
}

// progressLine renders one tool_use event as a single observable line:
// commands read as "Name: cmd", file edits as "Name path".
func progressLine(name, filePath, command string) string {
	switch {
	case command != "":
		return fmt.Sprintf("  [tool] %s: %s", name, command)
	case filePath != "":
		return fmt.Sprintf("  [tool] %s %s", name, filePath)
	default:
		return fmt.Sprintf("  [tool] %s", name)
	}
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
