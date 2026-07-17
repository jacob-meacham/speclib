package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var chdir string

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "speclib",
		Short:         "A package manager for spec-driven libraries",
		Version:       "0.0.0-dev",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if chdir != "" {
				return os.Chdir(chdir)
			}
			return nil
		},
	}
	root.PersistentFlags().StringVar(&chdir, "chdir", "", "run as if speclib was started in this directory")
	root.AddCommand(newInitCmd())
	root.AddCommand(newAddCmd())
	return root
}

// Execute runs the root command. main() calls this.
func Execute() error {
	return newRootCmd().Execute()
}
