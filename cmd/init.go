package cmd

import (
	"fmt"

	"github.com/jacob-meacham/speclib/internal/scaffold"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var agent string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold speclib.toml and agent integration in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := scaffold.WriteManifest("."); err != nil {
				return err
			}
			if agent != "" {
				if err := scaffold.WriteAgent(".", agent); err != nil {
					return err
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Initialized speclib. Add a dependency with `speclib add`.")
			return nil
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "", "install integration for this agent (claude, cursor, agents)")
	return cmd
}
