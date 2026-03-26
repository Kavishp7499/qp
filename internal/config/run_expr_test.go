package config

import "testing"

func TestParseRunExprParsesNestedGraph(t *testing.T) {
	t.Parallel()

	expr, err := ParseRunExpr("par(lint, test) -> build")
	if err != nil {
		t.Fatalf("ParseRunExpr() error = %v", err)
	}
	refs := RunExprRefs(expr)
	want := map[string]bool{"lint": true, "test": true, "build": true}
	if len(refs) != len(want) {
		t.Fatalf("RunExprRefs() = %#v, want %#v", refs, want)
	}
	for _, ref := range refs {
		if !want[ref] {
			t.Fatalf("unexpected ref %q in %#v", ref, refs)
		}
	}
}

func TestParseRunExprRejectsInvalidSyntax(t *testing.T) {
	t.Parallel()

	_, err := ParseRunExpr("par(lint test)")
	if err == nil {
		t.Fatal("ParseRunExpr() error = nil, want parse error")
	}
}

func TestParseRunExprWhenExpression(t *testing.T) {
	t.Parallel()

	expr, err := ParseRunExpr(`when(branch() == "main", deploy, notify)`)
	if err != nil {
		t.Fatalf("ParseRunExpr() error = %v", err)
	}
	refs := RunExprRefs(expr)
	want := map[string]bool{"deploy": true, "notify": true}
	if len(refs) != len(want) {
		t.Fatalf("RunExprRefs() = %#v, want %#v", refs, want)
	}
	for _, ref := range refs {
		if !want[ref] {
			t.Fatalf("unexpected ref %q in %#v", ref, refs)
		}
	}
}

func TestParseRunExprSwitchExpression(t *testing.T) {
	t.Parallel()

	expr, err := ParseRunExpr(`switch(env("TARGET"), "api": build-api -> deploy-api, "web": build-web)`)
	if err != nil {
		t.Fatalf("ParseRunExpr() error = %v", err)
	}
	refs := RunExprRefs(expr)
	want := map[string]bool{"build-api": true, "deploy-api": true, "build-web": true}
	if len(refs) != len(want) {
		t.Fatalf("RunExprRefs() = %#v, want %#v", refs, want)
	}
	for _, ref := range refs {
		if !want[ref] {
			t.Fatalf("unexpected ref %q in %#v", ref, refs)
		}
	}
}

func TestRunExprRenameRefs(t *testing.T) {
	t.Parallel()

	rename := map[string]string{
		"lint":  "ml:lint",
		"test":  "ml:test",
		"build": "ml:build",
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"seq", "lint -> test -> build", "ml:lint -> ml:test -> ml:build"},
		{"par", "par(lint, test)", "par(ml:lint, ml:test)"},
		{"mixed", "par(lint, test) -> build", "par(ml:lint, ml:test) -> ml:build"},
		{"partial rename", "lint -> deploy", "ml:lint -> deploy"},
		{"when", `when(branch() == "main", build, test)`, `when(branch() == "main", ml:build, ml:test)`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expr, err := ParseRunExpr(tt.input)
			if err != nil {
				t.Fatalf("ParseRunExpr(%q) error = %v", tt.input, err)
			}
			got := RunExprString(RunExprRenameRefs(expr, rename))
			if got != tt.want {
				t.Fatalf("RunExprString(RunExprRenameRefs(%q)) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRunExprStringRoundTrips(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple ref", "build", "build"},
		{"seq", "lint -> test -> build", "lint -> test -> build"},
		{"par", "par(lint, test)", "par(lint, test)"},
		{"when no else", `when(branch() == "main", deploy)`, `when(branch() == "main", deploy)`},
		{"when with else", `when(branch() == "main", deploy, notify)`, `when(branch() == "main", deploy, notify)`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expr, err := ParseRunExpr(tt.input)
			if err != nil {
				t.Fatalf("ParseRunExpr(%q) error = %v", tt.input, err)
			}
			got := RunExprString(expr)
			if got != tt.want {
				t.Fatalf("RunExprString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
