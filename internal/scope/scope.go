package scope

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neural-chilli/qp/internal/config"
)

type Result struct {
	Scope string   `json:"scope"`
	Desc  string   `json:"desc,omitempty"`
	Paths []string `json:"paths"`
}

type Coverage struct {
	SourceDirs []string `json:"source_dirs"`
	Covered    []string `json:"covered"`
	Orphaned   []string `json:"orphaned"`
}

func Get(cfg *config.Config, name string) (Result, error) {
	scopeDef, ok := cfg.Scopes[name]
	if !ok {
		return Result{}, fmt.Errorf("unknown scope %q", name)
	}
	return Result{
		Scope: name,
		Desc:  scopeDef.Desc,
		Paths: append([]string(nil), scopeDef.Paths...),
	}, nil
}

func FormatPrompt(name, desc string, paths []string) string {
	if desc != "" && len(paths) == 0 {
		return fmt.Sprintf("Only modify files in the declared scope `%s`. Scope intent: %s", name, desc)
	}
	if desc != "" {
		return fmt.Sprintf("Only modify files in the declared scope `%s`: %s. Scope intent: %s", name, strings.Join(paths, ", "), desc)
	}
	if len(paths) == 0 {
		return fmt.Sprintf("Only modify files in the declared scope `%s`.", name)
	}
	return fmt.Sprintf("Only modify files in the declared scope `%s`: %s", name, strings.Join(paths, ", "))
}

func ComputeCoverage(cfg *config.Config, repoRoot string) (Coverage, error) {
	sourceDirs := map[string]bool{}
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".qp", "node_modules", "vendor", "dist", "build":
				return filepath.SkipDir
			}
			return nil
		}
		if !isSourceFile(path) {
			return nil
		}
		relDir, err := filepath.Rel(repoRoot, filepath.Dir(path))
		if err != nil {
			return err
		}
		relDir = filepath.ToSlash(relDir)
		if relDir == "." {
			return nil
		}
		sourceDirs[relDir] = true
		return nil
	})
	if err != nil {
		return Coverage{}, err
	}

	var all []string
	var covered []string
	var orphaned []string
	for dir := range sourceDirs {
		all = append(all, dir)
		if matchesAnyScope(dir, cfg.Scopes) {
			covered = append(covered, dir)
		} else {
			orphaned = append(orphaned, dir)
		}
	}
	sort.Strings(all)
	sort.Strings(covered)
	sort.Strings(orphaned)

	return Coverage{
		SourceDirs: all,
		Covered:    covered,
		Orphaned:   orphaned,
	}, nil
}

func isSourceFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".rs", ".java", ".c", ".cc", ".cpp", ".h", ".hpp":
		return true
	default:
		return false
	}
}

func matchesAnyScope(dir string, scopes map[string]config.Scope) bool {
	for _, scopeDef := range scopes {
		for _, raw := range scopeDef.Paths {
			scopePath := strings.TrimSuffix(raw, "/")
			if scopePath == "" {
				continue
			}
			if dir == scopePath || strings.HasPrefix(dir, scopePath+"/") || strings.HasPrefix(scopePath, dir+"/") {
				return true
			}
		}
	}
	return false
}
