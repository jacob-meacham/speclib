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
	Language string `toml:"language,omitempty"`
}

type Manifest struct {
	Project      Project               `toml:"project"`
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
