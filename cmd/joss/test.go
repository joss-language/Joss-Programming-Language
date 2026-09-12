package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/tester"
)

func handleTestCommand(args []string) {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	filter := fs.String("filter", "", "Filtra pruebas por nombre")
	fs.StringVar(filter, "f", "", "Filtra pruebas por nombre")

	_ = fs.Parse(args)

	targetPath := "."
	posArgs := fs.Args()
	if len(posArgs) > 0 {
		targetPath = posArgs[0]
	}

	runner := tester.NewRunner()
	runner.Filter = *filter

	fmt.Println(i18n.Tr("testRunning", map[string]interface{}{"path": targetPath}))

	report, err := runner.Run(targetPath)
	if err != nil {
		fmt.Printf("Error ejecutando pruebas: %v\n", err)
		os.Exit(1)
	}

	if len(report.Suites) == 0 {
		fmt.Println(i18n.Tr("testNoFilesFound"))
		return
	}

	report.PrintSummary()

	if report.TotalFailed > 0 {
		os.Exit(1)
	}
}
