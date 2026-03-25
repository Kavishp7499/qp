package main

import (
	"os"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/lipgloss"
)

var forceNoColor atomic.Bool

var diagnosticPattern = regexp.MustCompile(`([A-Za-z0-9_./\\-]+\.[A-Za-z0-9_./\\-]*):([0-9]+)(?::([0-9]+))?`)

type outputStyler struct {
	enabled   bool
	bold      lipgloss.Style
	dim       lipgloss.Style
	green     lipgloss.Style
	red       lipgloss.Style
	yellow    lipgloss.Style
	blue      lipgloss.Style
	filePath  lipgloss.Style
	lineNo    lipgloss.Style
	errorHead lipgloss.Style
}

func stripGlobalFlags(args []string) ([]string, bool) {
	out := make([]string, 0, len(args))
	noColor := false
	for _, arg := range args {
		switch arg {
		case "--no-color":
			noColor = true
		default:
			out = append(out, arg)
		}
	}
	return out, noColor
}

func newOutputStyler(file *os.File) outputStyler {
	enabled := shouldUseColor(file)
	base := lipgloss.NewStyle()
	if !enabled {
		base = base.UnsetForeground().UnsetBackground().UnsetBold().UnsetItalic().UnsetUnderline().UnsetFaint()
	}
	return outputStyler{
		enabled:   enabled,
		bold:      base.Bold(true),
		dim:       base.Faint(true),
		green:     base.Foreground(lipgloss.Color("10")),
		red:       base.Foreground(lipgloss.Color("9")),
		yellow:    base.Foreground(lipgloss.Color("11")),
		blue:      base.Foreground(lipgloss.Color("12")),
		filePath:  base.Foreground(lipgloss.Color("14")).Bold(true),
		lineNo:    base.Foreground(lipgloss.Color("11")).Bold(true),
		errorHead: base.Foreground(lipgloss.Color("9")).Bold(true),
	}
}

func shouldUseColor(file *os.File) bool {
	if forceNoColor.Load() || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" || file == nil {
		return false
	}
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func (s outputStyler) taskName(name string) string {
	return s.bold.Render(name)
}

func (s outputStyler) duration(text string) string {
	return s.dim.Render(text)
}

func (s outputStyler) statusBadge(status string) string {
	switch status {
	case "pass":
		return s.green.Render("✓ PASS")
	case "fail":
		return s.red.Render("✗ FAIL")
	case "cancelled":
		return s.yellow.Render("⏭ CANCELLED")
	case "skipped":
		return s.yellow.Render("⏭ SKIPPED")
	case "timeout":
		return s.red.Render("✗ TIMEOUT")
	default:
		return strings.ToUpper(status)
	}
}

func (s outputStyler) running(text string) string {
	return s.blue.Render("⟳ " + text)
}

func (s outputStyler) finalStatus(passed bool) string {
	if passed {
		return s.green.Render("✓ PASSED")
	}
	return s.red.Render("✗ FAILED")
}

func (s outputStyler) errorPrefix() string {
	return s.errorHead.Render("error:")
}

func (s outputStyler) highlightDiagnostics(text string) string {
	return diagnosticPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := diagnosticPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		path := s.filePath.Render(parts[1])
		line := s.lineNo.Render(parts[2])
		if parts[3] == "" {
			return path + ":" + line
		}
		col := s.lineNo.Render(parts[3])
		return path + ":" + line + ":" + col
	})
}
