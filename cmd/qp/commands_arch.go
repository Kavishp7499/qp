package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/neural-chilli/qp/internal/archcheck"
)

func runArchCheck(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("arch-check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--json": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	report, err := archcheck.Run(cfg, repoRoot)
	if err != nil {
		printError(stderr, err)
		return 1
	}

	if *jsonOut {
		_ = printJSON(stdout, report)
	} else {
		if report.ViolationsCount == 0 {
			fmt.Fprintf(stdout, "arch-check passed (%d imports checked)\n", report.TotalImports)
		} else {
			for _, violation := range report.Violations {
				fmt.Fprintf(stdout, "%s:%d -> %s [%s] %s\n",
					violation.Source.File,
					violation.Source.Line,
					violation.Target.File,
					violation.Rule,
					violation.Message,
				)
			}
			fmt.Fprintf(stdout, "arch-check failed (%d violations, %d imports checked)\n", report.ViolationsCount, report.TotalImports)
		}
	}

	if report.ViolationsCount > 0 {
		return 1
	}
	return 0
}
