package lockfile

import (
	"errors"
	"io/fs"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

type State int

const (
	Pending State = iota
	UpgradePending
	UpToDate
)

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
}

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

type Lockfile struct {
	Packages []Package `toml:"package"`
}

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

func (l *Lockfile) Save(path string) error {
	data, err := toml.Marshal(l)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (l *Lockfile) Find(name string) (*Package, bool) {
	for i := range l.Packages {
		if l.Packages[i].Name == name {
			return &l.Packages[i], true
		}
	}
	return nil, false
}

func (l *Lockfile) Upsert(p Package) {
	for i := range l.Packages {
		if l.Packages[i].Name == p.Name {
			l.Packages[i] = p
			return
		}
	}
	l.Packages = append(l.Packages, p)
}

func (l *Lockfile) Remove(name string) {
	out := l.Packages[:0]
	for _, p := range l.Packages {
		if p.Name != name {
			out = append(out, p)
		}
	}
	l.Packages = out
}
