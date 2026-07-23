package cmd

import (
	"fmt"

	"github.com/jacob-meacham/speclib/internal/lockfile"
	"github.com/jacob-meacham/speclib/internal/manifest"
	"github.com/jacob-meacham/speclib/internal/paths"
	"github.com/jacob-meacham/speclib/internal/source"
	"github.com/spf13/cobra"
)

func newLockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lock",
		Short: "Resolve manifest dependencies into the lockfile",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := manifest.Load(paths.Manifest)
			if err != nil {
				return err
			}
			l, err := lockfile.Load(paths.Lock)
			if err != nil {
				return err
			}
			changed := 0
			for name, dep := range m.Dependencies {
				if _, ok := l.Find(name); ok {
					continue // P0: don't re-resolve existing entries
				}
				res, _, sp, err := source.Acquire(source.Parse(dep.Source), dep.Version, "")
				if err != nil {
					return fmt.Errorf("resolve %s: %w", name, err)
				}
				l.Upsert(lockfile.Package{
					Name: name, Source: dep.Source,
					Version: res.Version, Commit: res.Commit, SpecHash: sp.Hash(),
					Language: m.LanguageFor(name), Path: dep.Path,
				})
				changed++
			}
			if changed > 0 {
				if err := l.Save(paths.Lock); err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Resolved %d dependency(ies).\n", changed)
			return nil
		},
	}
}
