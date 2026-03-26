package scope

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/neural-chilli/qp/internal/config"
)

func TestComputeCoverage(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	for _, dir := range []string{"internal/api", "internal/orphan"} {
		if err := os.MkdirAll(filepath.Join(repoRoot, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "internal/api/handler.go"), []byte("package api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "internal/orphan/job.go"), []byte("package orphan\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Scopes: map[string]config.Scope{
			"api": {Desc: "API", Paths: []string{"internal/api/"}},
		},
	}

	coverage, err := ComputeCoverage(cfg, repoRoot)
	if err != nil {
		t.Fatalf("ComputeCoverage() error = %v", err)
	}
	if !slices.Contains(coverage.Covered, "internal/api") {
		t.Fatalf("Covered = %#v, want internal/api", coverage.Covered)
	}
	if !slices.Contains(coverage.Orphaned, "internal/orphan") {
		t.Fatalf("Orphaned = %#v, want internal/orphan", coverage.Orphaned)
	}
}

func TestOverlappingScopePaths(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	for _, dir := range []string{"internal/api", "internal/api/v2"} {
		if err := os.MkdirAll(filepath.Join(repoRoot, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "internal/api/handler.go"), []byte("package api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "internal/api/v2/handler.go"), []byte("package v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Scopes: map[string]config.Scope{
			"api":   {Desc: "API", Paths: []string{"internal/api/"}},
			"apiv2": {Desc: "API v2", Paths: []string{"internal/api/v2/"}},
		},
	}

	coverage, err := ComputeCoverage(cfg, repoRoot)
	if err != nil {
		t.Fatalf("ComputeCoverage() error = %v", err)
	}
	// Both directories should be covered (not orphaned).
	if slices.Contains(coverage.Orphaned, "internal/api") {
		t.Fatalf("internal/api should be covered, not orphaned")
	}
	if slices.Contains(coverage.Orphaned, "internal/api/v2") {
		t.Fatalf("internal/api/v2 should be covered, not orphaned")
	}
}

func TestEmptyScopeDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Scopes: map[string]config.Scope{
			"empty": {Desc: "empty scope"},
		},
	}

	result, err := Get(cfg, "empty")
	if err != nil {
		t.Fatalf("Get(empty) error = %v", err)
	}
	if result.Desc != "empty scope" {
		t.Fatalf("Desc = %q, want empty scope", result.Desc)
	}
	if len(result.Paths) != 0 {
		t.Fatalf("Paths = %v, want empty", result.Paths)
	}
}

func TestFormatPromptWithDescription(t *testing.T) {
	t.Parallel()

	result := FormatPrompt("api", "HTTP API layer", []string{"internal/api/"})
	if result == "" {
		t.Fatal("FormatPrompt() returned empty string")
	}
	if !slices.Contains([]byte(result), []byte("api")[0]) {
		t.Fatal("FormatPrompt should mention scope name")
	}
}

func TestFormatPromptWithoutDescription(t *testing.T) {
	t.Parallel()

	result := FormatPrompt("api", "", []string{"internal/api/"})
	if result == "" {
		t.Fatal("FormatPrompt() returned empty string")
	}
}
