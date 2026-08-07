package cmd

import (
	"os"

	"github.com/jacob-meacham/speclib/internal/agent"
	"github.com/spf13/cobra"
)

var chdir string

// version is stamped by goreleaser at release time via
// -ldflags "-X github.com/jacob-meacham/speclib/cmd.version=...".
var version = "0.0.0-dev"

func newRootCmd() *cobra.Command {
	// nil backend: sync --headless builds the configured HeadlessClaude from
	// the manifest's [agent] section at run time. Tests inject stubs here.
	return newRootCmdWithBackend(nil)
}

func newRootCmdWithBackend(backend agent.Backend) *cobra.Command {
	root := &cobra.Command{
		Use:           "speclib",
		Short:         "A package manager for spec-driven libraries",
		Version:       version,
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
	root.AddCommand(newNewCmd())
	root.AddCommand(newLintCmd())
	root.AddCommand(newReleaseCmd())
	return root
}

// Execute runs the root command. main() calls this.
func Execute() error { return newRootCmd().Execute() }
