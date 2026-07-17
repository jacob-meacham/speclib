package syncplan

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmeacham/speclib/internal/lockfile"
	"github.com/jmeacham/speclib/internal/manifest"
	"github.com/jmeacham/speclib/internal/source"
)

type Item struct {
	Name        string `json:"name"`
	State       string `json:"state"`
	TargetPath  string `json:"target_path"`
	Language    string `json:"language"`
	ContextFile string `json:"context_file,omitempty"`
	SpecDir     string `json:"spec_dir"`
	// upgrade-pending only
	FromCommit   string `json:"from_commit,omitempty"`
	ToVersion    string `json:"to_version,omitempty"`
	ToCommit     string `json:"to_commit,omitempty"`
	Selections   string `json:"selections,omitempty"`
	SpecDiffPath string `json:"spec_diff_path,omitempty"`
}

// Compute returns packages needing generation: both pending (never generated)
// and upgrade-pending (resolution moved ahead of the generated commit).
func Compute(m *manifest.Manifest, l *lockfile.Lockfile, only string) ([]lockfile.Package, error) {
	var out []lockfile.Package
	for _, p := range l.Packages {
		if only != "" && p.Name != only {
			continue
		}
		switch p.State() {
		case lockfile.Pending, lockfile.UpgradePending:
			out = append(out, p)
		}
	}
	return out, nil
}

// Materialize fetches the spec and writes it under workRoot/<name>/ for the
// agent. For an upgrade-pending package it also writes a SPEC.diff of the spec
// files between the generated commit and the newly resolved commit, and records
// the from/to provenance (and prior selections) on the Item.
func Materialize(workRoot string, dep manifest.Dependency, pkg lockfile.Package) (Item, error) {
	ref := source.Parse(dep.Source)
	_, lib, sp, err := source.Acquire(ref, dep.Version, pkg.Version)
	if err != nil {
		return Item{}, err
	}
	specDir := filepath.Join(workRoot, pkg.Name)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return Item{}, err
	}
	if err := os.WriteFile(filepath.Join(specDir, "PROMPT.md"), sp.Prompt, 0o644); err != nil {
		return Item{}, err
	}
	if err := os.WriteFile(filepath.Join(specDir, "SPEC.md"), sp.SpecDoc, 0o644); err != nil {
		return Item{}, err
	}
	for _, f := range sp.Fixtures {
		dst := filepath.Join(specDir, f.Path)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return Item{}, err
		}
		if err := os.WriteFile(dst, f.Data, 0o644); err != nil {
			return Item{}, err
		}
	}
	item := Item{
		Name:        pkg.Name,
		State:       pkg.State().String(),
		TargetPath:  pkg.Path,
		Language:    pkg.Language,
		ContextFile: dep.Context,
		SpecDir:     specDir,
	}
	if pkg.State() == lockfile.UpgradePending {
		// Best-effort spec diff. A diff we cannot compute (e.g. a non-git local
		// source pinned at the "local" sentinel, or a read failure) must not
		// fail the whole plan: fall back to an empty SPEC.diff.
		diff, err := source.SpecDiff(ref, pkg.GeneratedCommit, pkg.Commit, lib.Files)
		if err != nil {
			diff = ""
		}
		diffPath := filepath.Join(specDir, "SPEC.diff")
		if err := os.WriteFile(diffPath, []byte(diff), 0o644); err != nil {
			return Item{}, err
		}
		item.FromCommit = pkg.GeneratedCommit
		item.ToVersion = pkg.Version
		item.ToCommit = pkg.Commit
		item.Selections = pkg.Selections
		item.SpecDiffPath = diffPath
	}
	return item, nil
}

// ErrNoWork is returned by callers when there is nothing to do.
var ErrNoWork = fmt.Errorf("nothing to sync")
