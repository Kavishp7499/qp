//go:build !windows

package main

import "fmt"

func installWindowsPowerShellShim() (string, error) {
	return "", fmt.Errorf("windows only")
}

func registerWindowsTaskScheduler(exePath string) (string, error) {
	_ = exePath
	return "", fmt.Errorf("windows only")
}
