package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmeacham/speclib/internal/agent"
	"github.com/jmeacham/speclib/internal/lockfile"
	"github.com/jmeacham/speclib/internal/manifest"
	"github.com/jmeacham/speclib/internal/paths"
	"github.com/jmeacham/speclib/internal/syncplan"
	"github.com/spf13/cobra"
)

func newSyncCmd(backend agent.Backend) *cobra.Command {
	var plan, asJSON bool
	var record, testCmd, fixtureStatus, generatedCommit string
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
				return runRecord(cmd, record, testCmd, fixtureStatus, generatedCommit)
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

func runPlan(cmd *cobra.Command, only string, asJSON bool) error {
	m, l, err := loadState()
	if err != nil {
		return err
	}
	pending, err := syncplan.Compute(m, l, only)
	if err != nil {
		return err
	}
	items := make([]syncplan.Item, 0, len(pending))
	for _, p := range pending {
		item, err := syncplan.Materialize(paths.WorkDir, m.Dependencies[p.Name], p)
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

func runRecord(cmd *cobra.Command, name, testCmd, fixtureStatus, generatedCommit string) error {
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
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to sync.")
		return nil
	}
	for _, p := range pending {
		item, err := syncplan.Materialize(paths.WorkDir, m.Dependencies[p.Name], p)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Generating %s...\n", p.Name)
		res, err := backend.Generate(context.Background(), agent.Request{
			Name: item.Name, TargetPath: item.TargetPath, Language: item.Language,
			ContextFile: item.ContextFile, SpecDir: item.SpecDir,
		})
		if err != nil {
			return fmt.Errorf("generate %s: %w", p.Name, err)
		}
		if err := runRecord(cmd, p.Name, res.TestCommand, res.FixtureStatus, ""); err != nil {
			return err
		}
	}
	return nil
}
