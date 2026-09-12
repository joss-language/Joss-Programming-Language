package main

import (
	"fmt"
	"os"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/formatter"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/linter"
)

func handleCheckCommand(args []string) {
	var targetPath string
	if len(args) > 0 {
		targetPath = args[0]
	} else {
		targetPath = "."
	}

	fmt.Println(i18n.Tr("checkVerifying", map[string]interface{}{"path": targetPath}))
	fmt.Println()

	hasErrors := false

	// 1. Format check
	unformatted, err := formatter.FormatDirectory(targetPath, false, true)
	if err != nil {
		fmt.Println(i18n.Tr("checkFormatScanError", i18n.M{"error": err.Error()}))
		hasErrors = true
	} else if len(unformatted) > 0 {
		fmt.Println(i18n.Tr("checkFormatWarning", i18n.M{"count": len(unformatted)}))
		for _, f := range unformatted {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Println()
	} else {
		fmt.Printf("  %s\n", i18n.Tr("checkFormatOk"))
	}

	// 2. Lint & Semantic Analysis check
	l := linter.NewLinter()
	issues, err := l.LintPath(targetPath)
	if err != nil {
		fmt.Println(i18n.Tr("checkLintAnalysisError", i18n.M{"error": err.Error()}))
		hasErrors = true
	} else {
		errCount := 0
		warnCount := 0
		for _, issue := range issues {
			if issue.Severity == diagnostics.SeverityError {
				errCount++
				hasErrors = true
			} else {
				warnCount++
			}
		}

		if len(issues) > 0 {
			fmt.Printf("\n%s\n", i18n.Tr("checkDiagnosticsSummary", i18n.M{"errors": errCount, "warnings": warnCount}))
			for _, issue := range issues {
				fmt.Printf("  %s\n", issue.String())
				if issue.Suggestion != "" {
					fmt.Printf("    %s\n", i18n.Tr("checkSuggestionLabel", i18n.M{"suggestion": issue.Suggestion}))
				}
			}
		} else {
			fmt.Printf("  %s\n", i18n.Tr("checkSemanticOk"))
			fmt.Printf("  %s\n", i18n.Tr("checkLintOk"))
		}
	}

	fmt.Println()
	if hasErrors {
		fmt.Println(i18n.Tr("checkResultFail"))
		os.Exit(1)
	}

	fmt.Println(i18n.Tr("checkResultOk"))
}
