package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAgentBriefTaskJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
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
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"agent-brief", "--task", "check", "--json"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(agent-brief --task check --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{`"task": "check"`, `"context":`, `"agent": true`, `"markdown":`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunAgentBriefFilesMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cmd/qp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd/qp/main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
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
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"agent-brief", "--file", "cmd/qp/main.go"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(agent-brief --file cmd/qp/main.go) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{"# qp agent brief", "## Change Plan", "Matching Scopes", "Relevant Tasks"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}
