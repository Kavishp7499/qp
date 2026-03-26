package archcheck

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/neural-chilli/qp/internal/config"
)

type Location struct {
	File   string `json:"file"`
	Line   int    `json:"line,omitempty"`
	Domain string `json:"domain,omitempty"`
	Layer  string `json:"layer,omitempty"`
}

type Violation struct {
	Source  Location `json:"source"`
	Target  Location `json:"target"`
	Rule    string   `json:"rule"`
	Message string   `json:"message"`
}

type Report struct {
	Violations      []Violation `json:"violations"`
	TotalImports    int         `json:"total_imports"`
	ViolationsCount int         `json:"violations_count"`
	Status          string      `json:"status"`
}

type classifier struct {
	domains map[string]config.ArchitectureDomain
}

type ruleSet struct {
	directionForward bool
	crossDomain      string
	crossCutting     string
	layerIndex       map[string]int
}

func Run(cfg *config.Config, repoRoot string) (Report, error) {
	if len(cfg.Architecture.Domains) == 0 {
		return Report{}, fmt.Errorf("architecture is not configured in qp.yaml")
	}

	rules := buildRules(cfg.Architecture)
	modulePath := detectModulePath(repoRoot)
	if modulePath == "" {
		return Report{}, fmt.Errorf("architecture checking currently requires a Go project (no go.mod found in %s); consider using `qp validate --suggest` with scope coverage as a language-agnostic alternative", repoRoot)
	}
	classify := classifier{domains: cfg.Architecture.Domains}

	report := Report{
		Violations: []Violation{},
		Status:     "pass",
	}

	for _, domain := range cfg.Architecture.Domains {
		root := filepath.Join(repoRoot, domain.Root)
		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				switch d.Name() {
				case ".git", ".qp", "node_modules", "vendor":
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}

			imports, parseErr := parseImports(path)
			if parseErr != nil {
				return parseErr
			}
			sourceRel, relErr := filepath.Rel(repoRoot, path)
			if relErr != nil {
				return relErr
			}
			sourceRel = filepath.ToSlash(sourceRel)
			sourceDomain, sourceLayer, ok := classify.pathInfo(sourceRel)
			if !ok {
				return nil
			}
			for _, imp := range imports {
				targetRel, isLocal := normalizeImportPath(imp.path, modulePath)
				if !isLocal {
					continue
				}
				targetDomain, targetLayer, targetOK := classify.pathInfo(targetRel)
				if !targetOK {
					continue
				}
				report.TotalImports++
				if violation, violated := evaluateRules(rules, sourceRel, sourceDomain, sourceLayer, targetRel, targetDomain, targetLayer, imp.line); violated {
					report.Violations = append(report.Violations, violation)
				}
			}
			return nil
		})
		if walkErr != nil {
			return Report{}, walkErr
		}
	}

	report.ViolationsCount = len(report.Violations)
	if report.ViolationsCount > 0 {
		report.Status = "fail"
	}
	return report, nil
}

type parsedImport struct {
	path string
	line int
}

func parseImports(filePath string) ([]parsedImport, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	imports := make([]parsedImport, 0, len(file.Imports))
	for _, spec := range file.Imports {
		importPath := strings.Trim(spec.Path.Value, "\"")
		line := fset.Position(spec.Pos()).Line
		imports = append(imports, parsedImport{path: importPath, line: line})
	}
	return imports, nil
}

func detectModulePath(repoRoot string) string {
	goModPath := filepath.Join(repoRoot, "go.mod")
	f, err := os.Open(goModPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func normalizeImportPath(importPath, modulePath string) (string, bool) {
	if modulePath == "" {
		return "", false
	}
	if importPath == modulePath {
		return "", false
	}
	prefix := modulePath + "/"
	if !strings.HasPrefix(importPath, prefix) {
		return "", false
	}
	rel := strings.TrimPrefix(importPath, prefix)
	return filepath.ToSlash(filepath.Clean(rel)), true
}

func buildRules(arch config.ArchitectureConfig) ruleSet {
	set := ruleSet{
		crossDomain: "allow",
		layerIndex:  map[string]int{},
	}
	for idx, layer := range arch.Layers {
		set.layerIndex[layer] = idx
	}
	for _, rule := range arch.Rules {
		if rule.Direction == "forward" {
			set.directionForward = true
		}
		if rule.CrossDomain != "" {
			set.crossDomain = rule.CrossDomain
		}
		if rule.CrossCutting != "" {
			set.crossCutting = rule.CrossCutting
		}
	}
	return set
}

func (c classifier) pathInfo(path string) (string, string, bool) {
	path = filepath.ToSlash(filepath.Clean(path))
	for domainName, domain := range c.domains {
		root := filepath.ToSlash(filepath.Clean(domain.Root))
		if path != root && !strings.HasPrefix(path, root+"/") {
			continue
		}
		remainder := strings.TrimPrefix(path, root)
		remainder = strings.TrimPrefix(remainder, "/")
		if remainder == "" {
			return domainName, "", true
		}
		layer := strings.Split(remainder, "/")[0]
		if len(domain.Layers) > 0 && !contains(domain.Layers, layer) {
			return domainName, "", true
		}
		return domainName, layer, true
	}
	return "", "", false
}

func evaluateRules(rules ruleSet, sourceFile, sourceDomain, sourceLayer, targetFile, targetDomain, targetLayer string, line int) (Violation, bool) {
	if sourceDomain != targetDomain {
		if rules.crossDomain == "deny" {
			return Violation{
				Source: Location{File: sourceFile, Line: line, Domain: sourceDomain, Layer: sourceLayer},
				Target: Location{File: targetFile, Domain: targetDomain, Layer: targetLayer},
				Rule:   "cross_domain",
				Message: fmt.Sprintf("cross-domain import is denied: %s -> %s",
					sourceDomain, targetDomain),
			}, true
		}
		if rules.crossCutting != "" && targetLayer != rules.crossCutting {
			return Violation{
				Source: Location{File: sourceFile, Line: line, Domain: sourceDomain, Layer: sourceLayer},
				Target: Location{File: targetFile, Domain: targetDomain, Layer: targetLayer},
				Rule:   "cross_domain",
				Message: fmt.Sprintf("cross-domain imports must go through %s layer",
					rules.crossCutting),
			}, true
		}
	}

	if rules.directionForward && sourceLayer != "" && targetLayer != "" {
		sourceIndex, sourceOK := rules.layerIndex[sourceLayer]
		targetIndex, targetOK := rules.layerIndex[targetLayer]
		if sourceOK && targetOK && targetIndex < sourceIndex {
			return Violation{
				Source: Location{File: sourceFile, Line: line, Domain: sourceDomain, Layer: sourceLayer},
				Target: Location{File: targetFile, Domain: targetDomain, Layer: targetLayer},
				Rule:   "layer_direction",
				Message: fmt.Sprintf("forward layer rule violated: %s cannot import %s",
					sourceLayer, targetLayer),
			}, true
		}
	}

	return Violation{}, false
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
