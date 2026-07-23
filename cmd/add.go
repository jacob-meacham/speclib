package cmd

import (
	"fmt"
	"strings"

	"github.com/jacob-meacham/speclib/internal/lockfile"
	"github.com/jacob-meacham/speclib/internal/manifest"
	"github.com/jacob-meacham/speclib/internal/paths"
	"github.com/jacob-meacham/speclib/internal/source"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var path, lang, context string
	cmd := &cobra.Command{
		Use:   "add <source>[@<version>]",
		Short: "Add a spec-library dependency (does not generate; run `speclib sync`)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				return fmt.Errorf("--path is required")
			}
			src, explicit := splitVersion(args[0])
			constraint := "*"
			if explicit != "" {
				constraint = "^" + explicit
			}

			res, lib, sp, err := source.Acquire(source.Parse(src), constraint, explicit)
			if err != nil {
				return err
			}
			name := lib.Meta.Name

			m, err := manifest.Load(paths.Manifest)
			if err != nil {
				return err
			}
			verConstraint := constraint
			m.Dependencies[name] = manifest.Dependency{
				Source: src, Version: verConstraint, Path: path, Language: lang, Context: context,
			}
			if err := m.Save(paths.Manifest); err != nil {
				return err
			}

			l, err := lockfile.Load(paths.Lock)
			if err != nil {
				return err
			}
			l.Upsert(lockfile.Package{
				Name: name, Source: src,
				Version: res.Version, Commit: res.Commit, SpecHash: sp.Hash(),
				Language: m.LanguageFor(name), Path: path,
			})
			if err := l.Save(paths.Lock); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added %s@%s. Generate it with `speclib sync %s`.\n", name, res.Version, name)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "output directory for generated code (required)")
	cmd.Flags().StringVar(&lang, "lang", "", "target language (overrides project default)")
	cmd.Flags().StringVar(&context, "context", "", "path to a per-dependency context file")
	return cmd
}

func splitVersion(arg string) (src, version string) {
	// Split on the last '@' that is not part of a scheme or scp-like URL.
	if i := strings.LastIndex(arg, "@"); i > 0 && !strings.Contains(arg[i:], "/") {
		return arg[:i], arg[i+1:]
	}
	return arg, ""
}
