package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/jacob-meacham/speclib/internal/fingerprint"
	"github.com/jacob-meacham/speclib/internal/lockfile"
	"github.com/jacob-meacham/speclib/internal/paths"
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
			fmt.Fprintln(w, "NAME\tVERSION\tSTATE\tFIXTURES\tCODE")
			for _, p := range l.Packages {
				fx := p.FixtureStatus
				if fx == "" {
					fx = "-"
				}
				code, err := codeStatus(p)
				if err != nil {
					return err
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", p.Name, p.Version, p.State(), fx, code)
			}
			return w.Flush()
		},
	}
}

// codeStatus reports whether a package's generated code on disk still
// matches the hash recorded at generation time: "-" if it was never
// generated (no GeneratedHash), "missing" if Path no longer exists,
// "edited" if its content has drifted, or "clean" otherwise.
func codeStatus(p lockfile.Package) (string, error) {
	if p.GeneratedHash == "" {
		return "-", nil
	}
	current, err := fingerprint.HashDir(p.Path)
	if err != nil {
		return "", err
	}
	switch {
	case current == "":
		return "missing", nil
	case current != p.GeneratedHash:
		return "edited", nil
	default:
		return "clean", nil
	}
}
