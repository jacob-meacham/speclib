package manifest

import (
	"errors"
	"io/fs"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

type Dependency struct {
	Source   string `toml:"source"`
	Version  string `toml:"version"`
	Path     string `toml:"path"`
	Language string `toml:"language,omitempty"`
	Context  string `toml:"context,omitempty"`
}

type Project struct {
	Language string   `toml:"language,omitempty"`
	Checks   []string `toml:"checks,omitempty"`
}

// Agent configures which coding agent drives generation and, for headless
// sync, the permission args its print mode is launched with. A nil Agent (no
// [agent] section) means the defaults: the claude adapter with its built-in
// permission allowlist.
type Agent struct {
	Command     string   `toml:"command,omitempty"`
	Permissions []string `toml:"permissions,omitempty"`
}

type Manifest struct {
	Project      Project               `toml:"project"`
	Agent        *Agent                `toml:"agent,omitempty"`
	Dependencies map[string]Dependency `toml:"dependencies"`
}

func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Manifest{Dependencies: map[string]Dependency{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]Dependency{}
	}
	return &m, nil
}

func (m *Manifest) Save(path string) error {
	data, err := toml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (m *Manifest) LanguageFor(dep string) string {
	if d, ok := m.Dependencies[dep]; ok && d.Language != "" {
		return d.Language
	}
	return m.Project.Language
}

// AgentCommand returns the configured [agent].command, or "" (the default
// adapter) when the section is absent.
func (m *Manifest) AgentCommand() string {
	if m.Agent == nil {
		return ""
	}
	return m.Agent.Command
}

// AgentPermissions returns the configured [agent].permissions, or nil (use
// the adapter's defaults) when the section is absent.
func (m *Manifest) AgentPermissions() []string {
	if m.Agent == nil {
		return nil
	}
	return m.Agent.Permissions
}

type LibraryMeta struct {
	Name    string `toml:"name"`
	Summary string `toml:"summary"`
}

type Files struct {
	Prompt   string `toml:"prompt"`
	Spec     string `toml:"spec"`
	Fixtures string `toml:"fixtures"`
}

type Hints struct {
	Languages []string `toml:"languages"`
}

type Library struct {
	Meta  LibraryMeta `toml:"library"`
	Files Files       `toml:"files"`
	Hints Hints       `toml:"hints"`
}

func ParseLibrary(data []byte) (*Library, error) {
	var lib Library
	if err := toml.Unmarshal(data, &lib); err != nil {
		return nil, err
	}
	return &lib, nil
}
