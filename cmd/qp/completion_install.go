package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type completionInstallResult struct {
	Message    string
	Paths      []string
	ReloadHint string
}

type completionInstaller struct {
	shell string
	home  string
}

func newCompletionInstaller(shell string) (*completionInstaller, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &completionInstaller{shell: shell, home: home}, nil
}

func (c *completionInstaller) Install() (*completionInstallResult, error) {
	switch c.shell {
	case "bash":
		return c.installBash()
	case "zsh":
		return c.installZsh()
	case "fish":
		return c.installFish()
	case "powershell":
		return c.installPowerShell()
	default:
		return nil, fmt.Errorf("unsupported shell %q", c.shell)
	}
}

func (c *completionInstaller) installBash() (*completionInstallResult, error) {
	script, err := completionScript("bash")
	if err != nil {
		return nil, err
	}
	completionPath := filepath.Join(c.home, ".local", "share", "bash-completion", "completions", "qp")
	if err := writeFileWithParents(completionPath, script); err != nil {
		return nil, err
	}
	rcPath := filepath.Join(c.home, ".bashrc")
	block := strings.Join([]string{
		"# qp completion",
		fmt.Sprintf("if [ -f %q ]; then", completionPath),
		fmt.Sprintf("  source %q", completionPath),
		"fi",
	}, "\n")
	if _, err := writeManagedBlock(rcPath, "# >>> qp completion >>>", "# <<< qp completion <<<", block); err != nil {
		return nil, err
	}
	return &completionInstallResult{
		Message:    "Installed bash completion for qp.",
		Paths:      []string{completionPath, rcPath},
		ReloadHint: "Restart bash or run `source ~/.bashrc`.",
	}, nil
}

func (c *completionInstaller) installZsh() (*completionInstallResult, error) {
	script, err := completionScript("zsh")
	if err != nil {
		return nil, err
	}
	completionDir := filepath.Join(c.home, ".zsh", "completions")
	completionPath := filepath.Join(completionDir, "_qp")
	if err := writeFileWithParents(completionPath, script); err != nil {
		return nil, err
	}
	rcPath := filepath.Join(c.home, ".zshrc")
	block := strings.Join([]string{
		"# qp completion",
		fmt.Sprintf("fpath=(%q $fpath)", completionDir),
		"autoload -Uz compinit",
		"compinit",
	}, "\n")
	if _, err := writeManagedBlock(rcPath, "# >>> qp completion >>>", "# <<< qp completion <<<", block); err != nil {
		return nil, err
	}
	return &completionInstallResult{
		Message:    "Installed zsh completion for qp.",
		Paths:      []string{completionPath, rcPath},
		ReloadHint: "Restart zsh or run `source ~/.zshrc`.",
	}, nil
}

func (c *completionInstaller) installFish() (*completionInstallResult, error) {
	script, err := completionScript("fish")
	if err != nil {
		return nil, err
	}
	completionPath := filepath.Join(c.home, ".config", "fish", "completions", "qp.fish")
	if err := writeFileWithParents(completionPath, script); err != nil {
		return nil, err
	}
	return &completionInstallResult{
		Message:    "Installed fish completion for qp.",
		Paths:      []string{completionPath},
		ReloadHint: "Restart fish or run `source ~/.config/fish/completions/qp.fish`.",
	}, nil
}

func (c *completionInstaller) installPowerShell() (*completionInstallResult, error) {
	script, err := completionScript("powershell")
	if err != nil {
		return nil, err
	}
	scriptPath := filepath.Join(c.home, "Documents", "PowerShell", "qp-completion.ps1")
	if err := writeFileWithParents(scriptPath, script); err != nil {
		return nil, err
	}
	profilePath := filepath.Join(c.home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	block := strings.Join([]string{
		"# qp completion",
		fmt.Sprintf(". %q", scriptPath),
	}, "\n")
	if _, err := writeManagedBlock(profilePath, "# >>> qp completion >>>", "# <<< qp completion <<<", block); err != nil {
		return nil, err
	}
	return &completionInstallResult{
		Message:    "Installed PowerShell completion for qp.",
		Paths:      []string{scriptPath, profilePath},
		ReloadHint: "Restart PowerShell or run `. $PROFILE`.",
	}, nil
}

func completionScript(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashCompletionScript(), nil
	case "zsh":
		return zshCompletionScript(), nil
	case "fish":
		return fishCompletionScript(), nil
	case "powershell":
		return powershellCompletionScript(), nil
	default:
		return "", fmt.Errorf("unsupported shell %q", shell)
	}
}

func bashCompletionScript() string {
	return `# qp bash completion
_qp_completion() {
  local IFS=$'\n'
  local args=("${COMP_WORDS[@]:1}")
  if [[ $COMP_CWORD -ge ${#args[@]} ]]; then
    args+=("")
  fi
  COMPREPLY=($(qp __complete "${args[@]}"))
}
complete -F _qp_completion qp
`
}

func zshCompletionScript() string {
	return `#compdef qp
_qp_completion() {
  local -a args
  args=("${words[@]:2}")
  if [[ -z "${words[CURRENT]}" ]]; then
    args+=("")
  fi
  local -a completions
  completions=("${(@f)$(qp __complete "${args[@]}")}")
  _describe 'values' completions
}
compdef _qp_completion qp
`
}

func fishCompletionScript() string {
	return `function __qp_complete
    set -l words (commandline -opc)
    if test (count $words) -gt 0
        set -e words[1]
    end
    if string match -q -- '* ' (commandline)
        set words $words ""
    end
    qp __complete $words
end

complete -c qp -f -a "(__qp_complete)"
`
}

func powershellCompletionScript() string {
	return `Register-ArgumentCompleter -Native -CommandName qp -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $elements = @()
    foreach ($element in ($commandAst.CommandElements | Select-Object -Skip 1)) {
        if ($null -ne $element.Value) {
            $elements += $element.Value
        }
    }
    if ([string]::IsNullOrEmpty($wordToComplete)) {
        $elements += ""
    }

    qp __complete @elements | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`
}

func writeFileWithParents(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeManagedBlock(path, start, end, body string) (bool, error) {
	block := strings.Join([]string{start, body, end, ""}, "\n")
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	content := string(raw)
	updated := false
	if strings.Contains(content, start) && strings.Contains(content, end) {
		startIdx := strings.Index(content, start)
		endIdx := strings.Index(content, end)
		if startIdx >= 0 && endIdx >= startIdx {
			endIdx += len(end)
			content = content[:startIdx] + block + strings.TrimLeft(content[endIdx:], "\n")
			updated = true
		}
	} else if strings.TrimSpace(content) == "" {
		content = block
		updated = true
	} else {
		content = strings.TrimRight(content, "\n") + "\n\n" + block
		updated = true
	}

	if err := writeFileWithParents(path, content); err != nil {
		return false, err
	}
	return updated, nil
}
