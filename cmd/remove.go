package cmd

import (
	"fmt"

	"github.com/jmeacham/speclib/internal/lockfile"
	"github.com/jmeacham/speclib/internal/manifest"
	"github.com/jmeacham/speclib/internal/paths"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <dep>",
		Short: "Remove a dependency from the manifest and lockfile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			m, err := manifest.Load(paths.Manifest)
			if err != nil {
				return err
			}
			l, err := lockfile.Load(paths.Lock)
			if err != nil {
				return err
			}
			genPath := ""
			if p, ok := l.Find(name); ok {
				genPath = p.Path
			}
			delete(m.Dependencies, name)
			l.Remove(name)
			if err := m.Save(paths.Manifest); err != nil {
				return err
			}
			if err := l.Save(paths.Lock); err != nil {
				return err
			}
			if genPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed %s. Generated code at %s was left in place; delete it if you no longer need it.\n", name, genPath)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed %s.\n", name)
			}
			return nil
		},
	}
}
