package syncplan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jacob-meacham/speclib/internal/lockfile"
	"github.com/jacob-meacham/speclib/internal/manifest"
	"github.com/jacob-meacham/speclib/internal/source"
	"github.com/jacob-meacham/speclib/internal/spec"
)

type Item struct {
	Name        string   `json:"name"`
	State       string   `json:"state"`
	TargetPath  string   `json:"target_path"`
	Language    string   `json:"language"`
	ContextFile string   `json:"context_file,omitempty"`
	SpecDir     string   `json:"spec_dir"`
	Checks      []string `json:"checks,omitempty"`
	// upgrade-pending only
	FromCommit   string `json:"from_commit,omitempty"`
	ToVersion    string `json:"to_version,omitempty"`
	ToCommit     string `json:"to_commit,omitempty"`
	Selections   string `json:"selections,omitempty"`
	SpecDiffPath string `json:"spec_diff_path,omitempty"`
}

// Compute returns packages needing generation: both pending (never generated)
// and upgrade-pending (resolution moved ahead of the generated commit).
// Lockfile packages with no matching manifest entry are excluded: they are
// orphaned (e.g. hand-edited lockfile state) and have no source to acquire.
func Compute(m *manifest.Manifest, l *lockfile.Lockfile, only string) ([]lockfile.Package, error) {
	var out []lockfile.Package
	for _, p := range l.Packages {
		if only != "" && p.Name != only {
			continue
		}
		if _, ok := m.Dependencies[p.Name]; !ok {
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
// the from/to provenance (and prior selections) on the Item. checks is the
// consumer project's declared check commands, copied onto the Item verbatim.
func Materialize(workRoot string, dep manifest.Dependency, pkg lockfile.Package, checks []string) (Item, error) {
	ref := source.Parse(dep.Source)
	// Materialize generates from the lockfile's pinned commit, not from
	// re-resolving dep.Version's tag: if the tag has since been force-moved
	// upstream, resolving it here would generate against different content
	// than the lockfile records.
	lib, sp, err := source.AcquireAtCommit(ref, pkg.Commit)
	if err != nil {
		return Item{}, fmt.Errorf("acquire %s: %w", pkg.Name, err)
	}
	specDir := filepath.Join(workRoot, pkg.Name)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return Item{}, fmt.Errorf("create spec dir for %s: %w", pkg.Name, err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "PROMPT.md"), sp.Prompt, 0o644); err != nil {
		return Item{}, fmt.Errorf("write PROMPT.md for %s: %w", pkg.Name, err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "SPEC.md"), sp.SpecDoc, 0o644); err != nil {
		return Item{}, fmt.Errorf("write SPEC.md for %s: %w", pkg.Name, err)
	}
	for _, f := range sp.Fixtures {
		dst, err := fixtureDest(specDir, f)
		if err != nil {
			return Item{}, fmt.Errorf("materialize %s: %w", pkg.Name, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return Item{}, fmt.Errorf("create fixture dir for %s (%s): %w", pkg.Name, f.Path, err)
		}
		if err := os.WriteFile(dst, f.Data, 0o644); err != nil {
			return Item{}, fmt.Errorf("write fixture %s (%s): %w", pkg.Name, f.Path, err)
		}
	}
	item := Item{
		Name:        pkg.Name,
		State:       pkg.State().String(),
		TargetPath:  pkg.Path,
		Language:    pkg.Language,
		ContextFile: dep.Context,
		SpecDir:     specDir,
		Checks:      checks,
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
			return Item{}, fmt.Errorf("write SPEC.diff for %s: %w", pkg.Name, err)
		}
		item.FromCommit = pkg.GeneratedCommit
		item.ToVersion = pkg.Version
		item.ToCommit = pkg.Commit
		item.Selections = pkg.Selections
		item.SpecDiffPath = diffPath
	}
	return item, nil
}

// fixtureDest returns the on-disk path for fixture f within specDir, erroring
// if f's declared path would resolve outside specDir (e.g. via a "../"
// component) rather than silently writing there. This guards against a
// malformed or malicious library manifest whose fixtures list escapes root.
func fixtureDest(specDir string, f spec.File) (string, error) {
	dst := filepath.Join(specDir, f.Path)
	rel, err := filepath.Rel(specDir, dst)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fixture path %q escapes spec directory", f.Path)
	}
	return dst, nil
}
