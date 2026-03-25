package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPlanMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: go test ./...
    scope: cli
    safety: idempotent
  build:
    desc: Build binary
    cmd: go build ./...
    scope: cli
    safety: idempotent
    needs:
      - test
guards:
  default:
    steps:
      - test
      - build
groups:
  core:
    desc: Core workflows
    tasks:
      - test
      - build
scopes:
  cli:
    desc: CLI surface
    paths:
      - cmd/qp/
codemap:
  packages:
    cmd/qp:
      desc: CLI entrypoint
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"plan", "--file", "cmd/qp/main.go"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(plan) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{"# qp plan", "Matching Scopes", "Relevant Tasks", "Relevant Guards", "Relevant Codemap", "qp help ", "qp guard default"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunPlanJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: go test ./...
    scope: cli
scopes:
  cli:
    desc: CLI surface
    paths:
      - cmd/qp/
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"plan", "--json", "cmd/qp/main.go"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(plan --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{`"files": [`, `"cmd/qp/main.go"`, `"scopes": [`, `"tasks": [`, `"markdown":`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunPlanRequiresFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: go test ./...
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, _ := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"plan"}, stdout, stderr)
	if code == 0 {
		t.Fatal("run(plan) code = 0, want failure")
	}
	if !strings.Contains(readStderr(), "at least one file path is required") {
		t.Fatalf("stderr = %q, want file requirement error", readStderr())
	}
}

func TestRunDiffPlanJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: go test ./...
    scope: cli
scopes:
  cli:
    desc: CLI surface
    paths:
      - cmd/qp/
`), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForTest(t, dir, "init")
	runGitForTest(t, dir, "config", "user.email", "test@example.com")
	runGitForTest(t, dir, "config", "user.name", "Test User")
	if err := os.MkdirAll(filepath.Join(dir, "cmd/qp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd/qp/main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForTest(t, dir, "add", "qp.yaml", "cmd/qp/main.go")
	runGitForTest(t, dir, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(dir, "cmd/qp/main.go"), []byte("package main\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"diff-plan", "--json"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(diff-plan --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{`"files": [`, `"cmd/qp/main.go"`, `"tasks": [`, `"markdown":`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func runGitForTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if raw, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(raw))
	}
}
