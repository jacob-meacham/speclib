package agent

import (
	"context"
	"os"
	"path/filepath"
)

type Request struct {
	Name        string
	TargetPath  string
	Language    string
	ContextFile string
	SpecDir     string
	Checks      []string
}

type Result struct {
	TestCommand   string
	FixtureStatus string
}

type Backend interface {
	Generate(ctx context.Context, req Request) (Result, error)
}

// StubBackend is a deterministic test backend: it writes a placeholder and
// reports a trivially-passing test command.
type StubBackend struct{}

func (StubBackend) Generate(_ context.Context, req Request) (Result, error) {
	if err := os.MkdirAll(req.TargetPath, 0o755); err != nil {
		return Result{}, err
	}
	body := "Generated placeholder for " + req.Name + " (" + req.Language + ")\n"
	if err := os.WriteFile(filepath.Join(req.TargetPath, "GENERATED.md"), []byte(body), 0o644); err != nil {
		return Result{}, err
	}
	return Result{TestCommand: "true", FixtureStatus: "skip"}, nil
}
