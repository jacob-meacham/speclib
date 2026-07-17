package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/jmeacham/speclib/internal/lockfile"
	"github.com/jmeacham/speclib/internal/paths"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show dependency versions and generation state",
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := lockfile.Load(paths.Lock)
			if err != nil {
				return err
			}
			if len(l.Packages) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No dependencies. Add one with `speclib add`.")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVERSION\tSTATE\tFIXTURES")
			for _, p := range l.Packages {
				fx := p.FixtureStatus
				if fx == "" {
					fx = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Version, p.State(), fx)
			}
			return w.Flush()
		},
	}
}
