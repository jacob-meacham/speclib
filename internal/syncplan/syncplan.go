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
}

// Compute returns packages needing generation (P0: pending only).
func Compute(m *manifest.Manifest, l *lockfile.Lockfile, only string) ([]lockfile.Package, error) {
	var out []lockfile.Package
	for _, p := range l.Packages {
		if only != "" && p.Name != only {
			continue
		}
		if p.State() == lockfile.Pending {
			out = append(out, p)
		}
	}
	return out, nil
}

// Materialize fetches the spec and writes it under workRoot/<name>/ for the agent.
func Materialize(workRoot string, dep manifest.Dependency, pkg lockfile.Package) (Item, error) {
	_, _, sp, err := source.Acquire(source.Parse(dep.Source), dep.Version, pkg.Version)
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
	return Item{
		Name:        pkg.Name,
		State:       pkg.State().String(),
		TargetPath:  pkg.Path,
		Language:    pkg.Language,
		ContextFile: dep.Context,
		SpecDir:     specDir,
	}, nil
}

// ErrNoWork is returned by callers when there is nothing to do.
var ErrNoWork = fmt.Errorf("nothing to sync")
