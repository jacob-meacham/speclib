package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/jacob-meacham/speclib/internal/scaffold"
	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a new spec-library in ./<name>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dir := name

			empty, err := dirEmpty(dir)
			if err != nil {
				return err
			}
			if !empty {
				return fmt.Errorf("%s already exists and is not empty", dir)
			}

			if err := scaffold.WriteLibrary(dir, name); err != nil {
				return err
			}
			if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
				return fmt.Errorf("git init: %v: %s", err, out)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Scaffolded spec-library %q in ./%s\n\nNext steps:\n  cd %s\n  edit SPEC.md, PROMPT.md, and test_fixtures.json\n  speclib lint\n  speclib release 0.1.0\n", name, dir, dir)
			return nil
		},
	}
}

// dirEmpty reports whether dir is either absent or contains no entries.
func dirEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}
