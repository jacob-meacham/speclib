package cmd

import (
	"os"

	"github.com/jmeacham/speclib/internal/agent"
	"github.com/spf13/cobra"
)

var chdir string

func newRootCmd() *cobra.Command {
	return newRootCmdWithBackend(agent.HeadlessClaude{})
}

func newRootCmdWithBackend(backend agent.Backend) *cobra.Command {
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
	root.AddCommand(newLockCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newRemoveCmd())
	root.AddCommand(newVerifyCmd())
	root.AddCommand(newSyncCmd(backend))
	return root
}

// Execute runs the root command. main() calls this.
func Execute() error { return newRootCmd().Execute() }
