package guard

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/neural-chilli/qp/internal/config"
	"github.com/neural-chilli/qp/internal/runner"
)

func TestRunAllowsPipelineGuardSteps(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {
				Desc: "Run tests",
				Cmd:  "printf test",
			},
			"build": {
				Desc: "Build",
				Cmd:  "printf build",
			},
			"check": {
				Desc:  "Run checks",
				Steps: []string{"test", "build"},
			},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"check"}},
		},
	}

	report, err := New(cfg, repoRoot, runner.New(cfg, repoRoot)).Run("default", runner.Options{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Overall != runner.StatusPass {
		t.Fatalf("Overall = %q, want pass", report.Overall)
	}
	if len(report.Steps) != 1 {
		t.Fatalf("len(report.Steps) = %d, want 1", len(report.Steps))
	}
	if report.Steps[0].Name != "check" {
		t.Fatalf("step name = %q, want check", report.Steps[0].Name)
	}
	if report.Steps[0].Status != runner.StatusPass {
		t.Fatalf("step status = %q, want pass", report.Steps[0].Status)
	}
}

func TestRunIncludesStructuredErrorsInGuardReport(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {
				Desc:        "Run tests",
				Cmd:         `printf '%s\n' 'internal/guard/guard_test.go:42: guard failed' >&2; exit 1`,
				ErrorFormat: "go_test",
			},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"test"}},
		},
	}

	report, err := New(cfg, repoRoot, runner.New(cfg, repoRoot)).Run("default", runner.Options{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(report.Steps) != 1 {
		t.Fatalf("len(report.Steps) = %d, want 1", len(report.Steps))
	}
	if len(report.Steps[0].Errors) != 1 {
		t.Fatalf("len(report.Steps[0].Errors) = %d, want 1", len(report.Steps[0].Errors))
	}
	if got := report.Steps[0].Errors[0]; got.File != "internal/guard/guard_test.go" || got.Line != 42 || got.Message != "guard failed" {
		t.Fatalf("Errors[0] = %+v, want parsed guard error", got)
	}
}

func TestRunContinuesAfterMidPipelineFailure(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"pass1": {Desc: "pass first", Cmd: "echo pass1"},
			"fail":  {Desc: "fail", Cmd: "exit 1"},
			"pass2": {Desc: "pass second", Cmd: "echo pass2"},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"pass1", "fail", "pass2"}},
		},
	}

	report, err := New(cfg, repoRoot, runner.New(cfg, repoRoot)).Run("default", runner.Options{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Overall != runner.StatusFail {
		t.Fatalf("Overall = %q, want fail", report.Overall)
	}
	// All three steps should have run (guard continues on failure).
	if len(report.Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(report.Steps))
	}
	if report.Steps[0].Status != runner.StatusPass {
		t.Fatalf("step 0 status = %q, want pass", report.Steps[0].Status)
	}
	if report.Steps[1].Status != runner.StatusFail {
		t.Fatalf("step 1 status = %q, want fail", report.Steps[1].Status)
	}
	if report.Steps[2].Status != runner.StatusPass {
		t.Fatalf("step 2 status = %q, want pass", report.Steps[2].Status)
	}
}

func TestLastGuardJSONWrittenCorrectly(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {Desc: "test", Cmd: "echo ok"},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"test"}},
		},
	}

	_, err := New(cfg, repoRoot, runner.New(cfg, repoRoot)).Run("default", runner.Options{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	guardPath := runner.LastGuardPath(repoRoot)
	raw, err := os.ReadFile(guardPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", guardPath, err)
	}
	var cache CacheReport
	if err := json.Unmarshal(raw, &cache); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if cache.Guard != "default" {
		t.Fatalf("cache.Guard = %q, want default", cache.Guard)
	}
	if len(cache.Steps) != 1 {
		t.Fatalf("len(cache.Steps) = %d, want 1", len(cache.Steps))
	}
}

func TestCorruptedLastGuardJSONHandled(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	// Write corrupted cache file.
	guardPath := runner.LastGuardPath(repoRoot)
	if err := os.MkdirAll(runner.CacheDir(repoRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write to the .qp dir so the file exists.
	dotQP := runner.LastGuardPath(repoRoot)
	if err := os.MkdirAll(dotQP[:len(dotQP)-len("/last-guard.json")], 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guardPath, []byte("not json{{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {Desc: "test", Cmd: "echo ok"},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"test"}},
		},
	}

	// Should succeed even with corrupted cache — the old cache is just overwritten.
	report, err := New(cfg, repoRoot, runner.New(cfg, repoRoot)).Run("default", runner.Options{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Overall != runner.StatusPass {
		t.Fatalf("Overall = %q, want pass", report.Overall)
	}
}
