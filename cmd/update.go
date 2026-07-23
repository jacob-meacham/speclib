package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jacob-meacham/speclib/internal/lockfile"
	"github.com/jacob-meacham/speclib/internal/manifest"
	"github.com/jacob-meacham/speclib/internal/paths"
	"github.com/jacob-meacham/speclib/internal/source"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var to string
	cmd := &cobra.Command{
		Use:   "update [dep]",
		Short: "Re-resolve dependencies to newer versions (run `speclib sync` to regenerate)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			only := ""
			if len(args) == 1 {
				only = args[0]
			}
			if to != "" && only == "" {
				return fmt.Errorf("--to requires a dependency name")
			}

			m, err := manifest.Load(paths.Manifest)
			if err != nil {
				return err
			}
			l, err := lockfile.Load(paths.Lock)
			if err != nil {
				return err
			}

			names := updateTargets(m, only)
			if only != "" {
				if _, ok := m.Dependencies[only]; !ok {
					return fmt.Errorf("no such dependency: %s", only)
				}
			}

			lockChanged, manifestChanged := false, false
			out := cmd.OutOrStdout()
			for _, name := range names {
				dep := m.Dependencies[name]
				pkg, ok := l.Find(name)
				if !ok {
					return fmt.Errorf("no lockfile entry for %s; run `speclib add` or `speclib lock` first", name)
				}

				// --to pins an explicit version and rewrites the manifest
				// constraint to ^<version>; otherwise re-resolve the highest
				// version the existing constraint admits.
				res, _, sp, err := source.Acquire(source.Parse(dep.Source), dep.Version, to)
				if err != nil {
					return fmt.Errorf("resolve %s: %w", name, err)
				}
				if to != "" {
					newConstraint := "^" + strings.TrimPrefix(to, "v")
					if dep.Version != newConstraint {
						dep.Version = newConstraint
						m.Dependencies[name] = dep
						manifestChanged = true
					}
				}

				if res.Version == pkg.Version && res.Commit == pkg.Commit {
					fmt.Fprintf(out, "%s: already up to date (%s)\n", name, pkg.Version)
					continue
				}
				// Update resolution only; leave generation provenance intact so
				// State() flips to upgrade-pending until `speclib sync` runs.
				prev := pkg.Version
				pkg.Version = res.Version
				pkg.Commit = res.Commit
				pkg.SpecHash = sp.Hash()
				lockChanged = true
				fmt.Fprintf(out, "%s: %s -> %s (run 'speclib sync')\n", name, prev, res.Version)
			}

			if manifestChanged {
				if err := m.Save(paths.Manifest); err != nil {
					return err
				}
			}
			if lockChanged {
				if err := l.Save(paths.Lock); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "pin a dependency to an explicit version and update its manifest constraint")
	return cmd
}

// updateTargets returns the dependency names to update, sorted for stable
// output. With `only` set it is just that one name.
func updateTargets(m *manifest.Manifest, only string) []string {
	if only != "" {
		return []string{only}
	}
	names := make([]string, 0, len(m.Dependencies))
	for name := range m.Dependencies {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
