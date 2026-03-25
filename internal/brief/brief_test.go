package brief

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neural-chilli/qp/internal/config"
)

func TestGenerateTaskBriefIncludesAgentContext(t *testing.T) {
	dir := t.TempDir()
	writeBriefConfig(t, dir, `
project: demo
tasks:
  check:
    desc: Run checks
    cmd: printf ok
    scope: cli
scopes:
  cli:
    desc: CLI command surface
    paths:
      - cmd/qp/
`)

	cfg, err := config.Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	out, err := New(cfg, dir).Generate(Options{Task: "check"})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Context.Agent || out.Context.Task != "check" {
		t.Fatalf("context = %+v, want agent task brief", out.Context)
	}
	if !strings.Contains(out.Markdown, "# qp agent brief") {
		t.Fatalf("markdown = %q, want agent brief heading", out.Markdown)
	}
}

func TestGenerateFileBriefIncludesPlan(t *testing.T) {
	dir := t.TempDir()
	writeBriefConfig(t, dir, `
project: demo
tasks:
  check:
    desc: Run checks
    cmd: printf ok
    scope: cli
guards:
  default:
    steps:
      - check
scopes:
  cli:
    desc: CLI command surface
    paths:
      - cmd/qp/
codemap:
  packages:
    cmd/qp:
      desc: CLI entrypoint
`)

	cfg, err := config.Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	out, err := New(cfg, dir).Generate(Options{Files: []string{"cmd/qp/main.go"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Plan == nil {
		t.Fatal("plan = nil, want change plan")
	}
	if len(out.Files) != 1 || out.Files[0] != "cmd/qp/main.go" {
		t.Fatalf("files = %v, want normalized plan files", out.Files)
	}
	if !strings.Contains(out.Markdown, "## Change Plan") {
		t.Fatalf("markdown = %q, want change plan section", out.Markdown)
	}
}

func writeBriefConfig(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "cmd/qp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd/qp/main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
