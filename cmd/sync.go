package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/jacob-meacham/speclib/internal/agent"
	"github.com/jacob-meacham/speclib/internal/fingerprint"
	"github.com/jacob-meacham/speclib/internal/lockfile"
	"github.com/jacob-meacham/speclib/internal/manifest"
	"github.com/jacob-meacham/speclib/internal/paths"
	"github.com/jacob-meacham/speclib/internal/scaffold"
	"github.com/jacob-meacham/speclib/internal/syncplan"
	"github.com/spf13/cobra"
)

// isInteractiveTTY reports whether both stdin and stdout are terminals — the
// gate for handing off to the agent's interactive UI. Swappable in tests.
var isInteractiveTTY = func() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout} {
		info, err := f.Stat()
		if err != nil || info.Mode()&os.ModeCharDevice == 0 {
			return false
		}
	}
	return true
}

func newSyncCmd(backend agent.Backend) *cobra.Command {
	var plan, asJSON, headless bool
	var timeout time.Duration
	var record, testCmd, fixtureStatus, generatedCommit, selections string
	cmd := &cobra.Command{
		Use:   "sync [dep]",
		Short: "Generate code for pending dependencies (one at a time)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			only := ""
			if len(args) == 1 {
				only = args[0]
			}
			switch {
			case record != "":
				return runRecord(cmd, record, testCmd, fixtureStatus, generatedCommit, selections)
			case plan:
				return runPlan(cmd, only, asJSON)
			case headless:
				return runHeadless(cmd, only, backend, timeout)
			default:
				if !isInteractiveTTY() {
					return errors.New("not a terminal; pass --headless for non-interactive use")
				}
				return runInteractive(cmd, only)
			}
		},
	}
	cmd.Flags().BoolVar(&plan, "plan", false, "print the work plan and materialize specs (no generation)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "with --plan, emit JSON")
	cmd.Flags().BoolVar(&headless, "headless", false, "generate non-interactively via the agent's print mode (for CI)")
	cmd.Flags().DurationVar(&timeout, "timeout", 15*time.Minute, "with --headless, per-dependency generation timeout")
	cmd.Flags().StringVar(&record, "record", "", "record generation provenance for this dependency")
	cmd.Flags().StringVar(&testCmd, "test-command", "", "with --record, the test command to re-run in verify")
	cmd.Flags().StringVar(&fixtureStatus, "fixture-status", "pass", "with --record: pass|skip|fail")
	cmd.Flags().StringVar(&generatedCommit, "generated-commit", "", "with --record: spec commit generated from (defaults to resolved commit)")
	cmd.Flags().StringVar(&selections, "selections", "", "with --record: generation choices to honor on future upgrades")
	cmd.MarkFlagsMutuallyExclusive("plan", "record", "headless")
	return cmd
}

func loadState() (*manifest.Manifest, *lockfile.Lockfile, error) {
	m, err := manifest.Load(paths.Manifest)
	if err != nil {
		return nil, nil, err
	}
	l, err := lockfile.Load(paths.Lock)
	if err != nil {
		return nil, nil, err
	}
	return m, l, nil
}

// requireKnownDep errors if name names no dependency in either the manifest
// or the lockfile. It distinguishes a typo'd/unknown name (error) from a
// known dependency that simply has no pending work (which callers report as
// "Nothing to sync.").
func requireKnownDep(m *manifest.Manifest, l *lockfile.Lockfile, name string) error {
	if _, ok := m.Dependencies[name]; ok {
		return nil
	}
	if _, ok := l.Find(name); ok {
		return nil
	}
	return fmt.Errorf("no such dependency: %s", name)
}

func runPlan(cmd *cobra.Command, only string, asJSON bool) error {
	m, l, err := loadState()
	if err != nil {
		return err
	}
	pending, err := syncplan.Compute(m, l, only)
	if err != nil {
		return err
	}
	if only != "" && len(pending) == 0 {
		if err := requireKnownDep(m, l, only); err != nil {
			return err
		}
	}
	items := make([]syncplan.Item, 0, len(pending))
	for _, p := range pending {
		item, err := syncplan.Materialize(paths.WorkDir, m.Dependencies[p.Name], p, m.Project.Checks)
		if err != nil {
			return err
		}
		items = append(items, item)
	}
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Items []syncplan.Item `json:"items"`
		}{items})
	}
	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to sync.")
		return nil
	}
	for _, it := range items {
		fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s (%s); spec in %s\n", it.Name, it.TargetPath, it.Language, it.SpecDir)
	}
	return nil
}

func runRecord(cmd *cobra.Command, name, testCmd, fixtureStatus, generatedCommit, selections string) error {
	_, l, err := loadState()
	if err != nil {
		return err
	}
	p, ok := l.Find(name)
	if !ok {
		return fmt.Errorf("no such dependency: %s", name)
	}
	if generatedCommit == "" {
		generatedCommit = p.Commit
	}
	p.GeneratedCommit = generatedCommit
	p.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	p.Generator = "claude-code"
	p.TestCommand = testCmd
	p.FixtureStatus = fixtureStatus
	p.Selections = selections
	// Best-effort: an unreadable generated-code dir shouldn't block recording.
	if hash, hashErr := fingerprint.HashDir(p.Path); hashErr == nil {
		p.GeneratedHash = hash
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not hash %s: %v\n", p.Path, hashErr)
	}
	if err := l.Save(paths.Lock); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Recorded %s.\n", name)
	return nil
}

// runInteractive materializes every pending spec, then hands the terminal to
// the agent's own UI — full streaming, permission prompts, and questions —
// seeded with the canonical sync instructions. Recording happens inside the
// session via `speclib sync --record`, exactly as the installed skill does.
func runInteractive(cmd *cobra.Command, only string) error {
	m, l, err := loadState()
	if err != nil {
		return err
	}
	pending, err := syncplan.Compute(m, l, only)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		if only != "" {
			if err := requireKnownDep(m, l, only); err != nil {
				return err
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to sync.")
		return nil
	}
	names := make([]string, 0, len(pending))
	for _, p := range pending {
		// Announce before acquiring: materialization may fetch the source
		// over the network, and a slow fetch with no output reads as a hang.
		fmt.Fprintf(cmd.OutOrStdout(), "Materializing %s...\n", p.Name)
		if _, err := syncplan.Materialize(paths.WorkDir, m.Dependencies[p.Name], p, m.Project.Checks); err != nil {
			return err
		}
		names = append(names, p.Name)
	}
	ad, err := agent.Lookup(m.AgentCommand())
	if err != nil {
		return err
	}
	instructions, err := scaffold.SyncInstructions()
	if err != nil {
		return err
	}
	if only != "" {
		instructions += fmt.Sprintf("\n\nSync only the dependency named %q.", only)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Launching %s for %d pending dependency(ies)...\n", ad.Bin, len(pending))
	runErr := ad.LaunchInteractive(instructions, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
	// Partial progress is durable (--record ran inside the session), so
	// summarize per-dependency state even when the session exited nonzero.
	if l2, loadErr := lockfile.Load(paths.Lock); loadErr == nil {
		for _, name := range names {
			if p, ok := l2.Find(name); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", name, p.State())
			}
		}
	}
	return runErr
}

func runHeadless(cmd *cobra.Command, only string, backend agent.Backend, timeout time.Duration) error {
	m, l, err := loadState()
	if err != nil {
		return err
	}
	pending, err := syncplan.Compute(m, l, only)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		if only != "" {
			if err := requireKnownDep(m, l, only); err != nil {
				return err
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to sync.")
		return nil
	}
	if backend == nil {
		ad, err := agent.Lookup(m.AgentCommand())
		if err != nil {
			return err
		}
		backend = agent.HeadlessClaude{Adapter: ad, Permissions: m.AgentPermissions(), Progress: cmd.ErrOrStderr()}
	}
	for _, p := range pending {
		// Announce before acquiring: materialization may fetch the source
		// over the network, and a slow fetch with no output reads as a hang.
		fmt.Fprintf(cmd.OutOrStdout(), "Materializing %s...\n", p.Name)
		item, err := syncplan.Materialize(paths.WorkDir, m.Dependencies[p.Name], p, m.Project.Checks)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Generating %s...\n", p.Name)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		res, err := backend.Generate(ctx, agent.Request{
			Name: item.Name, TargetPath: item.TargetPath, Language: item.Language,
			ContextFile: item.ContextFile, SpecDir: item.SpecDir, Checks: item.Checks,
		})
		cancel()
		if err != nil {
			return fmt.Errorf("generate %s: %w", p.Name, err)
		}
		// Recording gate: never trust the agent's claim alone. Re-run the
		// reported test command; a sync whose test fails records nothing.
		// Bound it with the same timeout and process-group hygiene as the
		// generation call itself, so a hung test command can't hang sync.
		tctx, tcancel := context.WithTimeout(context.Background(), timeout)
		tc := exec.CommandContext(tctx, "sh", "-c", res.TestCommand)
		tc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		tc.Cancel = func() error { return syscall.Kill(-tc.Process.Pid, syscall.SIGKILL) }
		tc.WaitDelay = 5 * time.Second
		out, testErr := tc.CombinedOutput()
		tcancel()
		if testErr != nil {
			return fmt.Errorf("%s: generated, but test command %q failed — nothing recorded: %v\n%s",
				p.Name, res.TestCommand, testErr, out)
		}
		// The agent records selections in-session (the sync instructions
		// have it pass --selections); the gate re-record overwrites only
		// what it verifies, so carry those selections forward.
		selections := ""
		if cur, loadErr := lockfile.Load(paths.Lock); loadErr == nil {
			if curPkg, ok := cur.Find(p.Name); ok {
				selections = curPkg.Selections
			}
		}
		if err := runRecord(cmd, p.Name, res.TestCommand, res.FixtureStatus, "", selections); err != nil {
			return err
		}
	}
	return nil
}
