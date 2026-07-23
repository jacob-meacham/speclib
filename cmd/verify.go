package cmd

import (
	"fmt"
	"os/exec"

	"github.com/jacob-meacham/speclib/internal/lockfile"
	"github.com/jacob-meacham/speclib/internal/paths"
	"github.com/spf13/cobra"
)

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify [dep]",
		Short: "Re-run recorded fixture tests for generated dependencies",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := lockfile.Load(paths.Lock)
			if err != nil {
				return err
			}
			only := ""
			if len(args) == 1 {
				only = args[0]
			}
			if only != "" {
				if _, ok := l.Find(only); !ok {
					return fmt.Errorf("no such dependency: %s", only)
				}
			}
			var failed []string
			for _, p := range l.Packages {
				if only != "" && p.Name != only {
					continue
				}
				out := cmd.OutOrStdout()
				if p.TestCommand == "" {
					fmt.Fprintf(out, "SKIP %s (no fixture test)\n", p.Name)
					continue
				}
				c := exec.Command("sh", "-c", p.TestCommand)
				if b, err := c.CombinedOutput(); err != nil {
					fmt.Fprintf(out, "FAIL %s\n%s\n", p.Name, string(b))
					failed = append(failed, p.Name)
				} else {
					fmt.Fprintf(out, "PASS %s\n", p.Name)
				}
			}
			if len(failed) > 0 {
				return fmt.Errorf("%d dependency(ies) failed verification", len(failed))
			}
			return nil
		},
	}
}
