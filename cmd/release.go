package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

func newReleaseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "release <version>",
		Short: "Lint, then tag a release of the spec-library in the current directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version := args[0]
			if _, err := semver.NewVersion(version); err != nil {
				return fmt.Errorf("invalid version %q: not semver: %w", version, err)
			}
			tag := "v" + version

			res, err := lintLibrary()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(res.Problems) > 0 {
				for _, p := range res.Problems {
					fmt.Fprintln(out, "- "+p)
				}
				return fmt.Errorf("lint failed: %d problem(s) found", len(res.Problems))
			}

			dirty, err := gitWorkingTreeDirty()
			if err != nil {
				return err
			}
			if dirty {
				return fmt.Errorf("git working tree is not clean; commit your changes before releasing")
			}

			exists, err := gitTagExists(tag)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("tag %s already exists", tag)
			}

			if b, err := exec.Command("git", "tag", tag).CombinedOutput(); err != nil {
				return fmt.Errorf("git tag %s: %v: %s", tag, err, b)
			}

			fmt.Fprintf(out, "Released %s %s as tag %s\n", res.Name, version, tag)
			return nil
		},
	}
}

// gitWorkingTreeDirty reports whether the git working tree rooted at the
// current directory has any uncommitted changes (including untracked
// files).
func gitWorkingTreeDirty() (bool, error) {
	out, err := exec.Command("git", "status", "--porcelain").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git status: %v: %s", err, out)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// gitTagExists reports whether tag already exists in the current git repo.
func gitTagExists(tag string) (bool, error) {
	out, err := exec.Command("git", "tag", "--list", tag).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git tag --list: %v: %s", err, out)
	}
	return strings.TrimSpace(string(out)) == tag, nil
}
