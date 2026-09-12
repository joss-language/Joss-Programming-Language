package main

import (
	"fmt"
	"os"

	"github.com/jossecurity/joss/pkg/fixer"
	"github.com/jossecurity/joss/pkg/i18n"
)

func handleFixCommand(args []string) {
	dryRun := false
	var targetPath string

	for _, arg := range args {
		switch arg {
		case "--dry-run", "-d":
			dryRun = true
		default:
			if targetPath == "" {
				targetPath = arg
			}
		}
	}

	if targetPath == "" {
		targetPath = "."
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		fmt.Println(i18n.Tr("fixAccessError", i18n.M{"path": targetPath, "error": err.Error()}))
		os.Exit(1)
	}

	f := fixer.NewFixer(dryRun)

	if !info.IsDir() {
		res, err := f.FixFile(targetPath)
		if err != nil {
			fmt.Println(i18n.Tr("fixFileError", i18n.M{"path": targetPath, "error": err.Error()}))
			os.Exit(1)
		}
		if res.Changed {
			if dryRun {
				fmt.Println(i18n.Tr("fixDryRunFile", i18n.M{"path": targetPath, "count": res.FixesApplied}))
			} else {
				fmt.Println(i18n.Tr("fixAppliedFile", i18n.M{"path": targetPath, "count": res.FixesApplied}))
			}
		} else {
			fmt.Println(i18n.Tr("fixNoChanges", map[string]interface{}{"file": targetPath}))
		}
		return
	}

	results, err := f.FixDirectory(targetPath)
	if err != nil {
		fmt.Println(i18n.Tr("fixDirError", i18n.M{"path": targetPath, "error": err.Error()}))
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Println(i18n.Tr("fixAllClean"))
		return
	}

	totalFixes := 0
	for _, res := range results {
		totalFixes += res.FixesApplied
		if dryRun {
			fmt.Println(i18n.Tr("fixDryRunItem", i18n.M{"file": res.File, "count": res.FixesApplied}))
		} else {
			fmt.Println(i18n.Tr("fixAppliedItem", i18n.M{"file": res.File, "count": res.FixesApplied}))
		}
	}

	if dryRun {
		fmt.Printf("\n%s\n", i18n.Tr("fixDryRunTotal", i18n.M{"files": len(results), "fixes": totalFixes}))
	} else {
		fmt.Printf("\n%s\n", i18n.Tr("fixAppliedTotal", i18n.M{"files": len(results), "fixes": totalFixes}))
	}
}
