package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/jacob-meacham/speclib/internal/agent"
	"github.com/jacob-meacham/speclib/internal/fingerprint"
	"github.com/jacob-meacham/speclib/internal/lockfile"
	"github.com/jacob-meacham/speclib/internal/manifest"
	"github.com/jacob-meacham/speclib/internal/paths"
	"github.com/jacob-meacham/speclib/internal/syncplan"
	"github.com/spf13/cobra"
)

func newSyncCmd(backend agent.Backend) *cobra.Command {
	var plan, asJSON bool
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
			default:
				return runHeadless(cmd, only, backend)
			}
		},
	}
	cmd.Flags().BoolVar(&plan, "plan", false, "print the work plan and materialize specs (no generation)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "with --plan, emit JSON")
	cmd.Flags().StringVar(&record, "record", "", "record generation provenance for this dependency")
	cmd.Flags().StringVar(&testCmd, "test-command", "", "with --record, the test command to re-run in verify")
	cmd.Flags().StringVar(&fixtureStatus, "fixture-status", "pass", "with --record: pass|skip|fail")
	cmd.Flags().StringVar(&generatedCommit, "generated-commit", "", "with --record: spec commit generated from (defaults to resolved commit)")
	cmd.Flags().StringVar(&selections, "selections", "", "with --record: generation choices to honor on future upgrades")
	cmd.MarkFlagsMutuallyExclusive("plan", "record")
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

func runHeadless(cmd *cobra.Command, only string, backend agent.Backend) error {
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
	for _, p := range pending {
		item, err := syncplan.Materialize(paths.WorkDir, m.Dependencies[p.Name], p, m.Project.Checks)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Generating %s...\n", p.Name)
		res, err := backend.Generate(context.Background(), agent.Request{
			Name: item.Name, TargetPath: item.TargetPath, Language: item.Language,
			ContextFile: item.ContextFile, SpecDir: item.SpecDir, Checks: item.Checks,
		})
		if err != nil {
			return fmt.Errorf("generate %s: %w", p.Name, err)
		}
		// Recording gate: never trust the agent's claim alone. Re-run the
		// reported test command; a sync whose test fails records nothing.
		if out, testErr := exec.Command("sh", "-c", res.TestCommand).CombinedOutput(); testErr != nil {
			return fmt.Errorf("%s: generated, but test command %q failed — nothing recorded: %v\n%s",
				p.Name, res.TestCommand, testErr, out)
		}
		if err := runRecord(cmd, p.Name, res.TestCommand, res.FixtureStatus, "", ""); err != nil {
			return err
		}
	}
	return nil
}
