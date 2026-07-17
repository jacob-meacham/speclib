package cmd

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "speclib",
		Short:         "A package manager for spec-driven libraries",
		Version:       "0.0.0-dev",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	return root
}

// Execute runs the root command. main() calls this.
func Execute() error {
	return newRootCmd().Execute()
}
