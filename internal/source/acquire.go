package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmeacham/speclib/internal/manifest"
	"github.com/jmeacham/speclib/internal/spec"
)

type Resolved struct {
	Version string
	Commit  string
}

func Acquire(ref Ref, constraint, explicit string) (Resolved, *manifest.Library, *spec.Spec, error) {
	// Each call to Acquire is one logical resolve of ref.Location: forget any
	// fetch-once memoization ensureMirror recorded for it during an earlier,
	// independent resolve so this one observes upstream changes (e.g. newly
	// pushed tags) instead of silently reusing a stale mirror.
	forgetFetch(ref.Location)
	if ref.IsLocal {
		// A local path that is a git repo with semver tags resolves to those
		// tags (real version + commit SHA), giving version pinning without a
		// remote. Non-git dirs, or git repos with no semver tags, fall back to
		// an unversioned working-tree read.
		if isGitRepoWithTags(ref.Location) {
			return acquireGit(ref, constraint, explicit)
		}
		return acquireLocal(ref)
	}
	return acquireGit(ref, constraint, explicit)
}

// isGitRepoWithTags reports whether path is a git repository that has at least
// one semver tag. The rev-parse gate avoids attempting a mirror clone of a
// non-git directory.
func isGitRepoWithTags(path string) bool {
	if _, err := gitRun(path, "rev-parse", "--git-dir"); err != nil {
		return false
	}
	vs, _, err := gitTags(path)
	return err == nil && len(vs) > 0
}

func acquireLocal(ref Ref) (Resolved, *manifest.Library, *spec.Spec, error) {
	root := ref.Location
	libData, err := os.ReadFile(filepath.Join(root, "speclib.toml"))
	if err != nil {
		return Resolved{}, nil, nil, fmt.Errorf("read library manifest: %w", err)
	}
	lib, err := manifest.ParseLibrary(libData)
	if err != nil {
		return Resolved{}, nil, nil, err
	}
	read := func(p string) ([]byte, error) { return os.ReadFile(filepath.Join(root, p)) }
	listFixtures := func(sub string) ([]spec.File, error) { return localFixtures(root, sub) }
	sp, err := assembleSpec(lib.Files, read, listFixtures)
	if err != nil {
		return Resolved{}, nil, nil, err
	}
	// Local sources have no git commit; use a sentinel so the resolution/
	// generation state machine (which compares Commit to GeneratedCommit) works.
	return Resolved{Version: "0.0.0+local", Commit: "local"}, lib, sp, nil
}

func acquireGit(ref Ref, constraint, explicit string) (Resolved, *manifest.Library, *spec.Spec, error) {
	vs, refs, err := gitTags(ref.Location)
	if err != nil {
		return Resolved{}, nil, nil, err
	}
	var chosen, tag string
	if explicit != "" {
		tag = "v" + strings.TrimPrefix(explicit, "v")
		chosen = strings.TrimPrefix(explicit, "v")
	} else {
		v, err := PickVersion(constraint, vs)
		if err != nil {
			return Resolved{}, nil, nil, err
		}
		chosen = v.String()
		tag = refs[chosen]
	}
	commit, err := gitResolveCommit(ref.Location, tag)
	if err != nil {
		return Resolved{}, nil, nil, err
	}
	lib, sp, err := readLibAndSpec(ref.Location, tag)
	if err != nil {
		return Resolved{}, nil, nil, err
	}
	return Resolved{Version: chosen, Commit: commit}, lib, sp, nil
}

// AcquireAtCommit reads the spec-library at an immutable pinned commit,
// bypassing tag resolution entirely. sync/Materialize uses this instead of
// Acquire: the lockfile pins a commit SHA (Package.Commit), and if the
// constraint's tag has since been force-moved (e.g. `git tag -f`) to
// different content, re-resolving via the tag — as Acquire does — would
// silently generate against that different content instead of the content
// the lockfile actually records.
//
// The git-vs-local branch mirrors Acquire's: a non-git local source has no
// commits to pin against (acquireLocal always reports the "local" sentinel),
// so commit is ignored and this falls back to reading the current working
// tree, same as Acquire does for that source.
func AcquireAtCommit(ref Ref, commit string) (*manifest.Library, *spec.Spec, error) {
	forgetFetch(ref.Location)
	if ref.IsLocal && !isGitRepoWithTags(ref.Location) {
		_, lib, sp, err := acquireLocal(ref)
		return lib, sp, err
	}
	return readLibAndSpec(ref.Location, commit)
}

// readLibAndSpec reads speclib.toml and the spec files (prompt/spec/fixtures)
// out of location at commitish, which git accepts as either a tag or a full
// commit SHA. acquireGit and AcquireAtCommit share this: they differ only in
// how commitish is obtained (resolved from a version constraint vs. taken
// directly from a lockfile pin), not in how the library is read once it is
// known.
func readLibAndSpec(location, commitish string) (*manifest.Library, *spec.Spec, error) {
	libData, err := gitReadFile(location, commitish, "speclib.toml")
	if err != nil {
		return nil, nil, fmt.Errorf("read library manifest at %s: %w", commitish, err)
	}
	lib, err := manifest.ParseLibrary(libData)
	if err != nil {
		return nil, nil, err
	}
	read := func(p string) ([]byte, error) { return gitReadFile(location, commitish, p) }
	listFixtures := func(sub string) ([]spec.File, error) {
		paths, err := gitListFiles(location, commitish, sub)
		if err != nil {
			return nil, err
		}
		return readFixtureList(sub, paths, read)
	}
	sp, err := assembleSpec(lib.Files, read, listFixtures)
	if err != nil {
		return nil, nil, err
	}
	return lib, sp, nil
}

// SpecDiff returns a unified diff of the spec files (prompt, spec doc, and the
// fixtures path) between fromCommit and toCommit, computed against the source's
// git mirror. If either commit is empty or the "local" sentinel there is no
// diff to compute and it returns ("", nil).
func SpecDiff(ref Ref, fromCommit, toCommit string, files manifest.Files) (string, error) {
	if fromCommit == "" || toCommit == "" || fromCommit == "local" || toCommit == "local" {
		return "", nil
	}
	paths := []string{files.Prompt, files.Spec}
	if files.Fixtures != "" {
		paths = append(paths, strings.TrimSuffix(files.Fixtures, "/"))
	}
	return gitDiff(ref.Location, fromCommit, toCommit, paths)
}

func assembleSpec(files manifest.Files, read func(string) ([]byte, error), listFixtures func(string) ([]spec.File, error)) (*spec.Spec, error) {
	prompt, err := read(files.Prompt)
	if err != nil {
		return nil, fmt.Errorf("read prompt: %w", err)
	}
	specDoc, err := read(files.Spec)
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	sp := &spec.Spec{Prompt: prompt, SpecDoc: specDoc}
	if files.Fixtures != "" {
		fx, err := listFixtures(strings.TrimSuffix(files.Fixtures, "/"))
		if err != nil {
			return nil, fmt.Errorf("read fixtures: %w", err)
		}
		sp.Fixtures = fx
	}
	return sp, nil
}

// localFixtures walks a fixtures file-or-directory on disk.
func localFixtures(root, sub string) ([]spec.File, error) {
	full := filepath.Join(root, sub)
	info, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, err
		}
		return []spec.File{{Path: sub, Data: data}}, nil
	}
	var out []spec.File
	err = filepath.WalkDir(full, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, spec.File{Path: filepath.ToSlash(rel), Data: data})
		return nil
	})
	return out, err
}

// readFixtureList reads each git path. If `sub` is a single file, git ls-tree returns just it.
func readFixtureList(sub string, paths []string, read func(string) ([]byte, error)) ([]spec.File, error) {
	var out []spec.File
	for _, p := range paths {
		data, err := read(p)
		if err != nil {
			return nil, err
		}
		out = append(out, spec.File{Path: p, Data: data})
	}
	return out, nil
}
