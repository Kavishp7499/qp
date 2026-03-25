package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const daemonNudgeInterval = 24 * time.Hour

func daemonNudgePath(homeDir string) string {
	return filepath.Join(homeDir, ".qp", "daemon", "nudge.txt")
}

func shouldShowDaemonNudge(homeDir string, now time.Time) bool {
	path := daemonNudgePath(homeDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	lastShown, err := time.Parse(time.RFC3339, strings.TrimSpace(string(raw)))
	if err != nil {
		return true
	}
	return now.Sub(lastShown) >= daemonNudgeInterval
}

func markDaemonNudgeShown(homeDir string, now time.Time) error {
	path := daemonNudgePath(homeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(now.UTC().Format(time.RFC3339)), 0o644)
}

func maybePrintDaemonNudge(stderr io.Writer, homeDir string, now time.Time) {
	if stderr == nil || !shouldShowDaemonNudge(homeDir, now) {
		return
	}
	fmt.Fprintln(stderr, "Tip: run 'qp setup --windows' for faster execution (~2ms vs ~1000ms)")
	_ = markDaemonNudgeShown(homeDir, now)
}
