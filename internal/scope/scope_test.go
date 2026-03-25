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
