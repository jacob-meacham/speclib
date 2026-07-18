package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Validate the spec-library manifest and files in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := lintLibrary()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(res.Problems) > 0 {
				for _, p := range res.Problems {
					fmt.Fprintln(out, "- "+p)
				}
				return fmt.Errorf("%d problem(s) found", len(res.Problems))
			}
			fmt.Fprintf(out, "ok: %s\n", res.Name)
			return nil
		},
	}
}
