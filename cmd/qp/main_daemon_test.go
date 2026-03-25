package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunSetupNoopOnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-windows behavior only")
	}

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)
	code := run([]string{"setup"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(setup) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "No setup required on this platform") {
		t.Fatalf("stdout = %q, want non-windows noop message", got)
	}
}

func TestRunSetupWindowsFlagNoopOnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-windows behavior only")
	}

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)
	code := run([]string{"setup", "--windows"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(setup --windows) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "only available on Windows") {
		t.Fatalf("stdout = %q, want windows-only message", got)
	}
}

func TestRunDaemonStatusWhenNotRunning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)
	code := run([]string{"daemon", "status"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(daemon status) code = %d, want 0; stderr=%s", code, readStderr())
	}
	out := readStdout()
	if !strings.Contains(out, "qp daemon is not running") {
		t.Fatalf("stdout = %q, want not-running status", out)
	}
	if !strings.Contains(out, filepath.Join(home, ".qp", "daemon", "daemon.log")) {
		t.Fatalf("stdout = %q, want daemon log path", out)
	}
}

func TestDaemonNudgeShownOncePerInterval(t *testing.T) {
	home := t.TempDir()
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)

	var first bytes.Buffer
	maybePrintDaemonNudge(&first, home, now)
	if !strings.Contains(first.String(), "qp setup --windows") {
		t.Fatalf("first nudge = %q, want setup tip", first.String())
	}

	var second bytes.Buffer
	maybePrintDaemonNudge(&second, home, now.Add(time.Hour))
	if second.String() != "" {
		t.Fatalf("second nudge = %q, want suppressed nudge", second.String())
	}

	var third bytes.Buffer
	maybePrintDaemonNudge(&third, home, now.Add(25*time.Hour))
	if !strings.Contains(third.String(), "qp setup --windows") {
		t.Fatalf("third nudge = %q, want nudge after interval", third.String())
	}
}

func TestShouldShowDaemonNudgeWhenMarkerCorrupt(t *testing.T) {
	home := t.TempDir()
	path := daemonNudgePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-a-timestamp"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !shouldShowDaemonNudge(home, time.Now()) {
		t.Fatal("shouldShowDaemonNudge() = false, want true for corrupt marker")
	}
}
