package scaffold

import (
	"bytes"
	"embed"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed templates/*
var templates embed.FS

// libraryTemplates maps embedded template paths (under templates/new/) to
// the filename they render as in a scaffolded spec-library. Files ending in
// ".tmpl" are rendered through text/template with a {{.Name}} field; the
// fixtures file is static and copied verbatim.
var libraryTemplates = map[string]string{
	"templates/new/speclib.toml.tmpl": "speclib.toml",
	"templates/new/PROMPT.md.tmpl":    "PROMPT.md",
	"templates/new/SPEC.md.tmpl":      "SPEC.md",
	"templates/new/README.md.tmpl":    "README.md",
	"templates/new/CHANGELOG.md.tmpl": "CHANGELOG.md",
}

// WriteLibrary scaffolds a new spec-library into dir: speclib.toml (with
// name substituted into [library].name), PROMPT.md/SPEC.md stubs, a minimal
// test_fixtures.json, and README.md/CHANGELOG.md stubs. dir is created if it
// does not already exist.
func WriteLibrary(dir, name string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data := struct{ Name string }{Name: name}
	for src, dst := range libraryTemplates {
		raw, err := templates.ReadFile(src)
		if err != nil {
			return err
		}
		tmpl, err := template.New(dst).Parse(string(raw))
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, dst), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	fixtures, err := templates.ReadFile("templates/new/test_fixtures.json")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "test_fixtures.json"), fixtures, 0o644)
}

func WriteManifest(dir string) error {
	dst := filepath.Join(dir, "speclib.toml")
	if _, err := os.Stat(dst); err == nil {
		return nil // don't clobber an existing manifest
	}
	data, err := templates.ReadFile("templates/speclib.toml.tmpl")
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func WriteClaudeAgent(dir string) error {
	dstDir := filepath.Join(dir, ".claude", "skills", "speclib-sync")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	data, err := templates.ReadFile("templates/claude-sync.md")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dstDir, "SKILL.md"), data, 0o644)
}
