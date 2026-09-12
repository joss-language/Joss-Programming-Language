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
		fmt.Printf("Error: No se pudo acceder a '%s': %v\n", targetPath, err)
		os.Exit(1)
	}

	f := fixer.NewFixer(dryRun)

	if !info.IsDir() {
		res, err := f.FixFile(targetPath)
		if err != nil {
			fmt.Printf("Error aplicando correcciones en %s: %v\n", targetPath, err)
			os.Exit(1)
		}
		if res.Changed {
			if dryRun {
				fmt.Printf("[FIX DRY-RUN] %s: %d corrección(es) propuestas.\n", targetPath, res.FixesApplied)
			} else {
				fmt.Printf("[FIX OK] %s: %d corrección(es) aplicadas con éxito.\n", targetPath, res.FixesApplied)
			}
		} else {
			fmt.Println(i18n.Tr("fixNoChanges", map[string]interface{}{"file": targetPath}))
		}
		return
	}

	results, err := f.FixDirectory(targetPath)
	if err != nil {
		fmt.Printf("Error procesando directorio %s: %v\n", targetPath, err)
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
			fmt.Printf("[FIX DRY-RUN] %s (%d correcciones)\n", res.File, res.FixesApplied)
		} else {
			fmt.Printf("[FIX APPLIED] %s (%d correcciones)\n", res.File, res.FixesApplied)
		}
	}

	if dryRun {
		fmt.Printf("\n[FIX DRY-RUN] Total: %d archivo(s) con %d corrección(es) listas para aplicar.\n", len(results), totalFixes)
	} else {
		fmt.Printf("\n[FIX OK] Total: %d archivo(s) actualizados con %d corrección(es).\n", len(results), totalFixes)
	}
}
