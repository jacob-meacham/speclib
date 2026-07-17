package scaffold

import (
	"embed"
	"os"
	"path/filepath"
)

//go:embed templates/*
var templates embed.FS

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
