package config

import (
	"slices"
	"strings"
	"testing"
)

func TestSuggestIncludesCoreConfigHints(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: map[string]Task{
			"lint": {Desc: "lint", Cmd: "echo lint"},
			"test": {Desc: "test", Cmd: "echo test", Scope: "backend"},
		},
		Guards: map[string]Guard{
			"default": {Steps: []string{"lint"}},
		},
		Scopes: map[string]Scope{
			"backend": {Paths: []string{"internal/backend/"}},
		},
		Codemap: CodemapConfig{
			Packages: map[string]CodemapPackage{
				"cmd/qp": {Desc: "CLI layer"},
			},
		},
	}

	got := Suggest(cfg)
	assertContainsSuggestion(t, got, "Tasks without scope")
	assertContainsSuggestion(t, got, "Scopes without description")
	assertContainsSuggestion(t, got, "Scoped tasks not covered by guards")
	assertContainsSuggestion(t, got, "Codemap packages outside scope paths")
}

func TestSuggestHandlesGuardCoverageViaPipelineSteps(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: map[string]Task{
			"test":  {Desc: "test", Cmd: "echo test", Scope: "backend"},
			"check": {Desc: "check", Steps: []string{"test"}},
		},
		Guards: map[string]Guard{
			"default": {Steps: []string{"check"}},
		},
		Scopes: map[string]Scope{
			"backend": {Desc: "backend", Paths: []string{"internal/backend/"}},
		},
	}

	got := Suggest(cfg)
	if slices.ContainsFunc(got, func(item string) bool {
		return strings.Contains(item, "Scoped tasks not covered by guards")
	}) {
		t.Fatalf("Suggest() = %#v, want no uncovered scoped tasks", got)
	}
}

func assertContainsSuggestion(t *testing.T, suggestions []string, want string) {
	t.Helper()
	if !slices.ContainsFunc(suggestions, func(item string) bool {
		return strings.Contains(item, want)
	}) {
		t.Fatalf("suggestions = %#v, want %q", suggestions, want)
	}
}
