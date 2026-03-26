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

func TestSuggestNamespacedTasksNotInGuards(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: map[string]Task{
			"check":       {Desc: "check", Steps: []string{"ml:train"}},
			"ml:train":    {Desc: "train", Cmd: "echo train"},
			"ml:evaluate": {Desc: "evaluate", Cmd: "echo evaluate"},
		},
		Guards: map[string]Guard{
			"default": {Steps: []string{"check"}},
		},
	}

	got := Suggest(cfg)
	// ml:train is covered via check -> ml:train, but ml:evaluate is not.
	assertContainsSuggestion(t, got, "Namespaced tasks not covered by guards")
	for _, s := range got {
		if strings.Contains(s, "Namespaced tasks not covered") && strings.Contains(s, "ml:train") {
			t.Fatalf("Suggest() should not flag ml:train as uncovered namespaced task, got %#v", got)
		}
	}
}

func TestSuggestNoNamespacedWarningWhenAllCovered(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: map[string]Task{
			"check":    {Desc: "check", Steps: []string{"ml:train"}},
			"ml:train": {Desc: "train", Cmd: "echo train"},
		},
		Guards: map[string]Guard{
			"default": {Steps: []string{"check"}},
		},
	}

	got := Suggest(cfg)
	if slices.ContainsFunc(got, func(item string) bool {
		return strings.Contains(item, "Namespaced tasks not covered by guards")
	}) {
		t.Fatalf("Suggest() = %#v, want no uncovered namespaced tasks", got)
	}
}

func TestSuggestSurfacesMalformedRunExpr(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: map[string]Task{
			"check":   {Desc: "check", Steps: []string{"broken"}},
			"broken":  {Desc: "broken", Run: "-> -> invalid"},
			"covered": {Desc: "covered", Cmd: "echo ok", Scope: "backend"},
		},
		Guards: map[string]Guard{
			"default": {Steps: []string{"check"}},
		},
		Scopes: map[string]Scope{
			"backend": {Desc: "backend", Paths: []string{"internal/backend/"}},
		},
	}

	got := Suggest(cfg)
	assertContainsSuggestion(t, got, "failed to parse run expression")
}

func assertContainsSuggestion(t *testing.T, suggestions []string, want string) {
	t.Helper()
	if !slices.ContainsFunc(suggestions, func(item string) bool {
		return strings.Contains(item, want)
	}) {
		t.Fatalf("suggestions = %#v, want %q", suggestions, want)
	}
}
