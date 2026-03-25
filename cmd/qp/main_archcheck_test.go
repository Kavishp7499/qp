package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunArchCheckJSONFail(t *testing.T) {
	dir := t.TempDir()
	writeArchCheckFixture(t, dir)

	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"arch-check", "--json"}, stdout, stderr)
	if code == 0 {
		t.Fatalf("run(arch-check --json) code = 0, want failure; stderr=%s", readStderr())
	}
	output := readStdout()
	for _, want := range []string{`"status": "fail"`, `"violations_count": 1`, `"rule": "cross_domain"`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunArchCheckPass(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src", "auth", "service"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "auth", "service", "auth.go"), []byte(`package service
import "example.com/demo/src/auth/repo"
func Check() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src", "auth", "repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "auth", "repo", "repo.go"), []byte("package repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  check:
    desc: Run checks
    cmd: echo ok
architecture:
  layers: [repo, service]
  domains:
    auth:
      root: src/auth
      layers: [repo, service]
  rules:
    - cross_domain: deny
`), 0o644); err != nil {
		t.Fatal(err)
	}

	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"arch-check"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(arch-check) code = %d, want success; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), "arch-check passed") {
		t.Fatalf("stdout = %q, want pass output", readStdout())
	}
}

func writeArchCheckFixture(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(dir, "src", "auth", "service"),
		filepath.Join(dir, "src", "payments", "repo"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "src", "auth", "service", "auth.go"), []byte(`package service
import "example.com/demo/src/payments/repo"
func Check() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "payments", "repo", "store.go"), []byte("package repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  check:
    desc: Run checks
    cmd: echo ok
architecture:
  layers: [repo, service]
  domains:
    auth:
      root: src/auth
      layers: [repo, service]
    payments:
      root: src/payments
      layers: [repo, service]
  rules:
    - cross_domain: deny
`), 0o644); err != nil {
		t.Fatal(err)
	}
}
