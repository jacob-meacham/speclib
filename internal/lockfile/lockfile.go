package lockfile

import (
	"errors"
	"io/fs"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

// State classifies a package's generation status relative to its resolved
// commit.
type State int

const (
	// Pending means the package has never been generated.
	Pending State = iota
	// UpgradePending means the resolved commit has moved ahead of the commit
	// the package was last generated from.
	UpgradePending
	// UpToDate means the package was generated from the currently resolved
	// commit.
	UpToDate
)

// String renders State as its lowercase, hyphenated name (e.g.
// "upgrade-pending"), as used in status output and Item.State.
func (s State) String() string {
	switch s {
	case Pending:
		return "pending"
	case UpgradePending:
		return "upgrade-pending"
	default:
		return "up-to-date"
	}
}

// Package is one dependency's resolved (and, once generated, generation)
// state as recorded in the lockfile.
type Package struct {
	Name   string `toml:"name"`
	Source string `toml:"source"`

	// resolution (add / lock)
	Version  string `toml:"version"`
	Commit   string `toml:"commit"`
	SpecHash string `toml:"spec_hash"`
	Language string `toml:"language"`
	Path     string `toml:"path"`

	// generation provenance (sync)
	GeneratedCommit string `toml:"generated_commit,omitempty"`
	GeneratedAt     string `toml:"generated_at,omitempty"`
	Generator       string `toml:"generator,omitempty"`
	FixtureStatus   string `toml:"fixture_status,omitempty"`
	TestCommand     string `toml:"test_command,omitempty"`
	Selections      string `toml:"selections,omitempty"`
	// GeneratedHash is the fingerprint.HashDir digest of Path recorded at
	// generation time, used to detect drift from hand-edits.
	GeneratedHash string `toml:"generated_hash,omitempty"`
}

// State derives the package's generation state by comparing its resolved
// Commit to its GeneratedCommit.
func (p Package) State() State {
	switch {
	case p.GeneratedCommit == "":
		return Pending
	case p.GeneratedCommit != p.Commit:
		return UpgradePending
	default:
		return UpToDate
	}
}

// Lockfile is the on-disk record (speclib.lock) of every resolved, and
// optionally generated, dependency package.
type Lockfile struct {
	Packages []Package `toml:"package"`
}

// Load reads and parses the lockfile at path, returning an empty Lockfile
// (not an error) if the file does not yet exist.
func Load(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Lockfile{}, nil
	}
	if err != nil {
		return nil, err
	}
	var l Lockfile
	if err := toml.Unmarshal(data, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

// Save serializes the lockfile as TOML and writes it to path.
func (l *Lockfile) Save(path string) error {
	data, err := toml.Marshal(l)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Find returns a pointer to the package named name and true, or (nil, false)
// if no such package is present.
func (l *Lockfile) Find(name string) (*Package, bool) {
	for i := range l.Packages {
		if l.Packages[i].Name == name {
			return &l.Packages[i], true
		}
	}
	return nil, false
}

// Upsert replaces the existing package with the same name as p, or appends p
// if no such package is present.
func (l *Lockfile) Upsert(p Package) {
	for i := range l.Packages {
		if l.Packages[i].Name == p.Name {
			l.Packages[i] = p
			return
		}
	}
	l.Packages = append(l.Packages, p)
}

// Remove deletes the package named name, if present; it is a no-op
// otherwise.
func (l *Lockfile) Remove(name string) {
	out := l.Packages[:0]
	for _, p := range l.Packages {
		if p.Name != name {
			out = append(out, p)
		}
	}
	l.Packages = out
}
