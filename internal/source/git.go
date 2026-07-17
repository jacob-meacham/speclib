package source

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/Masterminds/semver/v3"
)

func gitCacheDir(location string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(location))
	return filepath.Join(base, "speclib", "git", hex.EncodeToString(sum[:])), nil
}

// validateSource rejects locations that are dangerous to pass to `git
// clone`/`git fetch` as a positional argument. In particular:
//   - a location starting with "-" could be interpreted as a command-line
//     flag instead of a source (flag injection);
//   - git's "ext::" and "fd::" transport helpers can execute arbitrary
//     commands or read/write arbitrary file descriptors, so they are
//     rejected outright rather than merely defused.
//
// http(s), git, ssh, scp-like (user@host:path), file:// and plain local
// paths are all left untouched.
func validateSource(location string) error {
	if strings.HasPrefix(location, "-") {
		return fmt.Errorf("invalid git source %q: must not start with '-'", location)
	}
	lower := strings.ToLower(location)
	for _, scheme := range []string{"ext::", "fd::"} {
		if strings.HasPrefix(lower, scheme) {
			return fmt.Errorf("invalid git source %q: %q transport is not allowed", location, strings.TrimSuffix(scheme, "::"))
		}
	}
	return nil
}

// gitCloneArgs builds the argument list used to clone location into dir as a
// bare mirror. protocol.ext.allow and protocol.fd.allow are pinned to
// "never" to disable git's smart-transport helpers (which can run arbitrary
// commands or accept arbitrary file descriptors); the "file" protocol is
// deliberately left enabled since local-path sources rely on it. The "--"
// separator ensures location is always treated as a positional argument,
// never as a flag, even if validateSource's leading-dash check were somehow
// bypassed.
func gitCloneArgs(location, dir string) []string {
	return []string{
		"-c", "protocol.ext.allow=never",
		"-c", "protocol.fd.allow=never",
		"clone", "--mirror", "--", location, dir,
	}
}

// fetchedLocations tracks which source locations have already had a
// successful clone or fetch during the current resolve. ensureMirror uses
// it to avoid redundant network round-trips when a single logical resolve
// operation calls it multiple times (tags, then commit, then file reads,
// ...). Acquire (the sole entry point that starts a resolve) calls
// forgetFetch at the start of each call, so memoization never outlives the
// resolve it belongs to: a later, independent resolve of the same source
// still observes upstream changes such as newly pushed tags.
var fetchedLocations sync.Map // location(string) -> struct{}{}

func markFetched(location string) {
	fetchedLocations.Store(location, struct{}{})
}

// forgetFetch clears the fetch-once memoization for location. Callers
// should invoke this at the start of each independent resolve of a source
// (see fetchedLocations) so that ensureMirror performs a fresh fetch rather
// than reusing a mark left over from an earlier, unrelated resolve.
func forgetFetch(location string) {
	fetchedLocations.Delete(location)
}

// mirrorReady reports whether location's mirror has already been
// fetched/cloned in this process and its cache dir is still present on
// disk.
func mirrorReady(location, dir string) bool {
	if _, ok := fetchedLocations.Load(location); !ok {
		return false
	}
	_, err := os.Stat(dir)
	return err == nil
}

// lockFilePath returns the path of the advisory lock file guarding
// concurrent clone/fetch of dir. It lives alongside dir (same parent), so
// it does not need its own directory setup beyond dir's parent.
func lockFilePath(dir string) string {
	return dir + ".lock"
}

// withMirrorLock acquires an exclusive, advisory file lock scoped to dir
// before running fn, releasing it afterward. This serializes concurrent
// clone/fetch attempts against the same cache dir, whether they come from
// goroutines within this process or from other speclib processes racing on
// the same source.
func withMirrorLock(dir string, fn func() error) error {
	f, err := os.OpenFile(lockFilePath(dir), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock %s: %w", lockFilePath(dir), err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}

// ensureMirror clones location into a bare mirror cache (or updates it),
// returning the cache dir. It fetches/clones at most once per location per
// resolve (see fetchedLocations) and serializes concurrent clone/fetch
// attempts against the same cache dir via an advisory file lock (see
// withMirrorLock).
func ensureMirror(location string) (string, error) {
	if err := validateSource(location); err != nil {
		return "", err
	}
	dir, err := gitCacheDir(location)
	if err != nil {
		return "", err
	}
	if mirrorReady(location, dir) {
		return dir, nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	err = withMirrorLock(dir, func() error {
		// Re-check now that we hold the lock: another goroutine or
		// process may have completed the clone/fetch while we waited.
		if mirrorReady(location, dir) {
			return nil
		}
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			if out, cloneErr := gitRun("", gitCloneArgs(location, dir)...); cloneErr != nil {
				return fmt.Errorf("git clone %s: %v: %s", location, cloneErr, out)
			}
		} else if statErr != nil {
			return statErr
		} else {
			if out, fetchErr := gitRun(dir, "fetch", "--tags", "--prune", "origin"); fetchErr != nil {
				return fmt.Errorf("git fetch: %v: %s", fetchErr, out)
			}
		}
		markFetched(location)
		return nil
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

func gitRun(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func gitOut(dir string, args ...string) (string, error) {
	out, err := gitRun(dir, args...)
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return out, nil
}

func gitTags(location string) ([]*semver.Version, map[string]string, error) {
	dir, err := ensureMirror(location)
	if err != nil {
		return nil, nil, err
	}
	out, err := gitOut(dir, "tag", "--list", "v*")
	if err != nil {
		return nil, nil, err
	}
	var vs []*semver.Version
	refs := map[string]string{}
	for _, line := range strings.Fields(out) {
		v, err := semver.NewVersion(strings.TrimPrefix(line, "v"))
		if err != nil {
			continue // ignore non-semver tags
		}
		vs = append(vs, v)
		refs[v.String()] = line
	}
	return vs, refs, nil
}

func gitResolveCommit(location, ref string) (string, error) {
	dir, err := ensureMirror(location)
	if err != nil {
		return "", err
	}
	out, err := gitOut(dir, "rev-list", "-n", "1", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func gitReadFile(location, ref, path string) ([]byte, error) {
	dir, err := ensureMirror(location)
	if err != nil {
		return nil, err
	}
	out, err := gitOut(dir, "show", ref+":"+path)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

func gitDiff(location, fromRef, toRef string, paths []string) (string, error) {
	dir, err := ensureMirror(location)
	if err != nil {
		return "", err
	}
	args := append([]string{"diff", fromRef + ".." + toRef, "--"}, paths...)
	return gitOut(dir, args...)
}

func gitListFiles(location, ref, sub string) ([]string, error) {
	dir, err := ensureMirror(location)
	if err != nil {
		return nil, err
	}
	out, err := gitOut(dir, "ls-tree", "-r", "--name-only", ref, "--", sub)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
