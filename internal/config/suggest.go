package config

import (
	"fmt"
	"sort"
	"strings"
)

func Suggest(cfg *Config) []string {
	var suggestions []string

	if tasks := tasksWithoutScope(cfg); len(tasks) > 0 {
		suggestions = append(suggestions, fmt.Sprintf("Tasks without scope: %s", strings.Join(tasks, ", ")))
	}
	if scopes := scopesWithoutDescription(cfg); len(scopes) > 0 {
		suggestions = append(suggestions, fmt.Sprintf("Scopes without description: %s", strings.Join(scopes, ", ")))
	}
	if uncovered := scopedTasksNotInGuards(cfg); len(uncovered) > 0 {
		suggestions = append(suggestions, fmt.Sprintf("Scoped tasks not covered by guards: %s", strings.Join(uncovered, ", ")))
	}
	if orphaned := codemapPackagesWithoutScope(cfg); len(orphaned) > 0 {
		suggestions = append(suggestions, fmt.Sprintf("Codemap packages outside scope paths: %s", strings.Join(orphaned, ", ")))
	}

	return suggestions
}

func tasksWithoutScope(cfg *Config) []string {
	var out []string
	for name, task := range cfg.Tasks {
		if task.Scope == "" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func scopesWithoutDescription(cfg *Config) []string {
	var out []string
	for name, scope := range cfg.Scopes {
		if strings.TrimSpace(scope.Desc) == "" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func scopedTasksNotInGuards(cfg *Config) []string {
	covered := map[string]bool{}
	visited := map[string]bool{}
	for _, guardCfg := range cfg.Guards {
		for _, step := range guardCfg.Steps {
			coverTask(cfg, step, covered, visited)
		}
	}
	var out []string
	for name, task := range cfg.Tasks {
		if task.Scope == "" {
			continue
		}
		if !covered[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func coverTask(cfg *Config, name string, covered, visited map[string]bool) {
	if visited[name] {
		return
	}
	visited[name] = true
	task, ok := cfg.Tasks[name]
	if !ok {
		return
	}
	covered[name] = true
	for _, step := range task.Steps {
		coverTask(cfg, step, covered, visited)
	}
	for _, need := range task.Needs {
		coverTask(cfg, need, covered, visited)
	}
	if task.Run != "" {
		expr, err := ParseRunExpr(task.Run)
		if err == nil {
			for _, ref := range RunExprRefs(expr) {
				coverTask(cfg, ref, covered, visited)
			}
		}
	}
}

func codemapPackagesWithoutScope(cfg *Config) []string {
	var out []string
	for pkg := range cfg.Codemap.Packages {
		if !packageMatchesAnyScope(pkg, cfg.Scopes) {
			out = append(out, pkg)
		}
	}
	sort.Strings(out)
	return out
}

func packageMatchesAnyScope(pkg string, scopes map[string]Scope) bool {
	for _, scope := range scopes {
		for _, raw := range scope.Paths {
			scopePath := strings.TrimSuffix(raw, "/")
			if scopePath == "" {
				continue
			}
			if pkg == scopePath || strings.HasPrefix(pkg, scopePath+"/") || strings.HasPrefix(scopePath, pkg+"/") {
				return true
			}
		}
	}
	return false
}
