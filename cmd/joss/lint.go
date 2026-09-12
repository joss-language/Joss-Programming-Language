package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/linter"
)

func handleLintCommand(args []string) {
	jsonOutput := false
	var targetPath string

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		default:
			if targetPath == "" {
				targetPath = arg
			}
		}
	}

	if targetPath == "" {
		targetPath = "."
	}

	l := linter.NewLinter()
	issues, err := l.LintPath(targetPath)
	if err != nil {
		fmt.Printf("Error ejecutando linter en '%s': %v\n", targetPath, err)
		os.Exit(1)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(issues, "", "  ")
		fmt.Println(string(data))
		if hasLintErrors(issues) {
			os.Exit(1)
		}
		return
	}

	if len(issues) == 0 {
		fmt.Printf("[LINT OK] %s\n", i18n.Tr("lintSuccess"))
		return
	}

	fmt.Println("\nResultados del análisis de Lint en Joss:")
	fmt.Println("------------------------------------------------------------")
	errCount := 0
	warnCount := 0

	for _, issue := range issues {
		fmt.Println(issue.String())
		if issue.Explanation != "" {
			fmt.Printf("  info: %s\n", issue.Explanation)
		}
		if issue.Suggestion != "" {
			fmt.Printf("  sugerencia: %s\n", issue.Suggestion)
		}
		if issue.Severity == diagnostics.SeverityError {
			errCount++
		} else {
			warnCount++
		}
	}
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("%d error(es), %d advertencia(s) detectadas.\n", errCount, warnCount)

	if errCount > 0 {
		os.Exit(1)
	}
}

func hasLintErrors(issues []linter.LintIssue) bool {
	for _, i := range issues {
		if i.Severity == diagnostics.SeverityError {
			return true
		}
	}
	return false
}
