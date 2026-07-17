package source

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

// ensureMirror clones location into a bare mirror cache (or updates it).
func ensureMirror(location string) (string, error) {
	dir, err := gitCacheDir(location)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return "", err
		}
		if out, err := gitRun("", "clone", "--mirror", location, dir); err != nil {
			return "", fmt.Errorf("git clone %s: %v: %s", location, err, out)
		}
		return dir, nil
	}
	if out, err := gitRun(dir, "fetch", "--tags", "--prune", "origin"); err != nil {
		return "", fmt.Errorf("git fetch: %v: %s", err, out)
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
