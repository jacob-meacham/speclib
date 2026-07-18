package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// syncInstructionsPath is the single, canonical source of the speclib-sync
// workflow: read the plan, generate per dependency, build/lint then run
// fixtures, and record provenance. Every supported agent gets this same body
// wrapped in its own frontmatter and dropped at its own conventional path.
const syncInstructionsPath = "templates/sync-instructions.md"

// SupportedAgents lists the values accepted by WriteAgent (and `speclib init
// --agent`).
var SupportedAgents = []string{"claude", "cursor", "agents"}

const claudeFrontmatter = `---
name: speclib-sync
description: Generate or update code for speclib spec-library dependencies. Use when the user runs ` + "`speclib sync`" + `, asks to generate a spec-library dependency, or after ` + "`speclib add`" + `.
---

`

const cursorFrontmatter = `---
description: Drive speclib code generation/upgrades
alwaysApply: false
---

`

// agentsSectionHeading delimits the speclib block within a shared AGENTS.md
// so repeated `speclib init --agent agents` runs are idempotent.
const agentsSectionHeading = "## speclib"

// WriteAgent installs the speclib-sync instructions for the named coding
// agent into dir. All agents share one canonical instructions body
// (templates/sync-instructions.md); this composes it with the agent's own
// frontmatter and writes it to the agent's conventional path:
//
//   - claude -> .claude/skills/speclib-sync/SKILL.md (YAML frontmatter)
//   - cursor -> .cursor/rules/speclib-sync.mdc (MDC frontmatter)
//   - agents -> AGENTS.md at dir's root (plain markdown, appended
//     idempotently under a "## speclib" section)
func WriteAgent(dir, agent string) error {
	raw, err := templates.ReadFile(syncInstructionsPath)
	if err != nil {
		return err
	}
	body := string(raw)

	switch agent {
	case "claude":
		return writeFile(dir, filepath.Join(".claude", "skills", "speclib-sync", "SKILL.md"), claudeFrontmatter+body)
	case "cursor":
		return writeFile(dir, filepath.Join(".cursor", "rules", "speclib-sync.mdc"), cursorFrontmatter+body)
	case "agents":
		return writeAgentsMD(dir, body)
	default:
		return fmt.Errorf("unknown agent %q: supported agents are %s", agent, strings.Join(SupportedAgents, ", "))
	}
}

// WriteClaudeAgent installs the Claude Code skill integration. It is a thin
// wrapper around WriteAgent, kept for existing callers.
func WriteClaudeAgent(dir string) error {
	return WriteAgent(dir, "claude")
}

func writeFile(dir, rel, content string) error {
	dst := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, []byte(content), 0o644)
}

// writeAgentsMD creates or appends to AGENTS.md at dir's root. If AGENTS.md
// already has a "## speclib" section, it is left untouched (idempotent);
// otherwise the section is appended to any existing content, or a fresh file
// is created with a short top heading plus the section.
func writeAgentsMD(dir, body string) error {
	dst := filepath.Join(dir, "AGENTS.md")
	existing, err := os.ReadFile(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		content := "# AGENTS.md\n\n" + agentsSectionHeading + "\n\n" + body
		return os.WriteFile(dst, []byte(content), 0o644)
	}
	if strings.Contains(string(existing), agentsSectionHeading) {
		return nil
	}
	content := strings.TrimRight(string(existing), "\n") + "\n\n" + agentsSectionHeading + "\n\n" + body
	return os.WriteFile(dst, []byte(content), 0o644)
}
