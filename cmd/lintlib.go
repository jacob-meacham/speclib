package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmeacham/speclib/internal/manifest"
	"github.com/jmeacham/speclib/internal/paths"
)

// lintResult is the outcome of validating the spec-library manifest and
// files rooted at the current directory.
type lintResult struct {
	Name     string
	Problems []string
}

// lintLibrary validates the spec-library manifest (paths.Manifest) and the
// files it references, relative to the current directory. It returns a hard
// error only when speclib.toml itself is missing or unparseable, since
// nothing else can be checked in that case. Otherwise every problem found is
// collected into the result so callers (lint, release) can report them all
// at once rather than stopping at the first one.
func lintLibrary() (*lintResult, error) {
	data, err := os.ReadFile(paths.Manifest)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", paths.Manifest, err)
	}
	lib, err := manifest.ParseLibrary(data)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", paths.Manifest, err)
	}

	res := &lintResult{Name: lib.Meta.Name}
	if lib.Meta.Name == "" {
		res.Problems = append(res.Problems, "[library].name must not be empty")
	}

	res.requireFile("prompt", lib.Files.Prompt)
	res.requireFile("spec", lib.Files.Spec)

	if lib.Files.Fixtures != "" {
		res.checkFixtures(lib.Files.Fixtures)
	}

	return res, nil
}

// requireFile records a problem if field is unset, or if it's set but the
// path it names does not exist.
func (r *lintResult) requireFile(field, rel string) {
	if rel == "" {
		r.Problems = append(r.Problems, fmt.Sprintf("[files].%s must be set", field))
		return
	}
	if _, err := os.Stat(rel); err != nil {
		r.Problems = append(r.Problems, fmt.Sprintf("[files].%s: %s does not exist", field, rel))
	}
}

// checkFixtures verifies rel exists (file or directory) and, if it's a
// .json file, that it contains valid JSON.
func (r *lintResult) checkFixtures(rel string) {
	info, err := os.Stat(rel)
	if err != nil {
		r.Problems = append(r.Problems, fmt.Sprintf("[files].fixtures: %s does not exist", rel))
		return
	}
	if info.IsDir() || filepath.Ext(rel) != ".json" {
		return
	}
	data, err := os.ReadFile(rel)
	if err != nil {
		r.Problems = append(r.Problems, fmt.Sprintf("[files].fixtures: %s: %v", rel, err))
		return
	}
	if !json.Valid(data) {
		r.Problems = append(r.Problems, fmt.Sprintf("[files].fixtures: %s is not valid JSON", rel))
	}
}
