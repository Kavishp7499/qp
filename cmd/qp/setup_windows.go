//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func installWindowsPowerShellShim() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	profilePath := filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	block := strings.Join([]string{
		"# qp daemon shim",
		"function Set-QPExitCode {",
		"    param([int]$Code)",
		"    & cmd /c \"exit $Code\" | Out-Null",
		"    $global:LASTEXITCODE = $Code",
		"}",
		"function qp {",
		"    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)",
		"    $pipe = $null",
		"    $writer = $null",
		"    $reader = $null",
		"    try {",
		"        $pipe = [System.IO.Pipes.NamedPipeClientStream]::new('.', 'qp-daemon', [System.IO.Pipes.PipeDirection]::InOut)",
		"        $pipe.Connect(1000)",
		"        $writer = [System.IO.StreamWriter]::new($pipe)",
		"        $writer.AutoFlush = $true",
		"        $reader = [System.IO.StreamReader]::new($pipe)",
		"        $payload = @{ args = $Args; cwd = (Get-Location).Path } | ConvertTo-Json -Compress",
		"        $writer.WriteLine($payload)",
		"        while (($line = $reader.ReadLine()) -ne $null) {",
		"            $evt = $line | ConvertFrom-Json",
		"            if ($null -ne $evt.error -and [string]$evt.error -ne '') {",
		"                [Console]::Error.Write([string]$evt.error)",
		"                [Console]::Error.Write(\"`n\")",
		"                Set-QPExitCode ([int]$evt.exit_code)",
		"                return",
		"            }",
		"            if ($evt.stream -eq 'stdout') { [Console]::Out.Write([string]$evt.data) }",
		"            elseif ($evt.stream -eq 'stderr') { [Console]::Error.Write([string]$evt.data) }",
		"            if ($evt.done) {",
		"                Set-QPExitCode ([int]$evt.exit_code)",
		"                return",
		"            }",
		"        }",
		"        [Console]::Error.WriteLine('qp daemon connection closed unexpectedly.')",
		"        Set-QPExitCode 1",
		"    } catch {",
		"        [Console]::Error.WriteLine(\"qp daemon error: $($_.Exception.Message)\")",
		"        Set-QPExitCode 1",
		"    } finally {",
		"        if ($reader -ne $null) { $reader.Dispose() }",
		"        if ($writer -ne $null) { $writer.Dispose() }",
		"        if ($pipe -ne $null) { $pipe.Dispose() }",
		"    }",
		"}",
	}, "\n")
	if _, err := writeManagedBlock(profilePath, "# >>> qp daemon >>>", "# <<< qp daemon <<<", block); err != nil {
		return "", fmt.Errorf("install powershell shim: %w", err)
	}
	return profilePath, nil
}
