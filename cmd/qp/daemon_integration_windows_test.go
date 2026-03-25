//go:build windows

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestWindowsDaemonRoundTripHarness(t *testing.T) {
	if os.Getenv("QP_WINDOWS_INTEGRATION") != "1" {
		t.Skip("set QP_WINDOWS_INTEGRATION=1 to run daemon integration harness")
	}
	exePath := os.Getenv("QP_EXE_PATH")
	if exePath == "" {
		t.Skip("set QP_EXE_PATH to a built qp executable")
	}

	_, _, _ = runQPExe(t, exePath, "daemon", "stop")

	stdout, stderr, err := runQPExe(t, exePath, "daemon", "start")
	if err != nil {
		t.Fatalf("daemon start error = %v; stdout=%s stderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "qp daemon running") {
		t.Fatalf("daemon start stdout = %q, want running status", stdout)
	}

	stdout, stderr, err = runQPExe(t, exePath, "version")
	if err != nil {
		t.Fatalf("version error = %v; stdout=%s stderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "qp ") {
		t.Fatalf("version stdout = %q, want version output", stdout)
	}

	stdout, stderr, err = runQPExe(t, exePath, "daemon", "stop")
	if err != nil {
		t.Fatalf("daemon stop error = %v; stdout=%s stderr=%s", err, stdout, stderr)
	}
}

func runQPExe(t *testing.T, exePath string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(exePath, args...)
	out, err := cmd.Output()
	stderr := ""
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
	}
	return string(out), stderr, err
}
