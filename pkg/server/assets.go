package server

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func CompileStyles() {
	compileStyles()
}

func compileStyles() {
	fmt.Println("Compilando estilos...")
	// Find .scss files
	files, _ := filepath.Glob("assets/css/*.scss")
	for _, file := range files {
		// Only compile files that don't start with _
		if strings.HasPrefix(filepath.Base(file), "_") {
			continue
		}

		content, err := resolveImports(file)
		if err != nil {
			fmt.Printf("Error compilando %s: %v\n", file, err)
			continue
		}

		// 1. Extract Variables
		vars := make(map[string]string)
		// Support hyphens in variable names: $my-var
		reVar := regexp.MustCompile(`(\$[a-zA-Z0-9_-]+):\s*(.+?);`)
		content = reVar.ReplaceAllStringFunc(content, func(match string) string {
			parts := reVar.FindStringSubmatch(match)
			vars[parts[1]] = parts[2]
			return "" // Remove variable definition from output
		})

		// 2. Replace complete variable tokens. Replacing map keys one by one
		// corrupts names that share a prefix, such as $text and $text-dim.
		content = replaceSCSSVariables(content, vars)
		// Optimize CSS (Purge Unused)
		// content = optimizeCSS(content) // DISABLED: Moving to Dynamic Runtime Delivery

		// Change extension to .css
		outFile := filepath.Join("public", "css", strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))+".css")
		os.MkdirAll(filepath.Dir(outFile), 0755)
		os.WriteFile(outFile, []byte(content), 0644)
	}
}

var scssVariableReference = regexp.MustCompile(`\$[a-zA-Z0-9_-]+`)

func replaceSCSSVariables(content string, variables map[string]string) string {
	return scssVariableReference.ReplaceAllStringFunc(content, func(name string) string {
		if value, ok := variables[name]; ok {
			return value
		}
		return name
	})
}

func resolveImports(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(data)

	reImport := regexp.MustCompile(`@import\s+"([^"]+)";`)
	content = reImport.ReplaceAllStringFunc(content, func(match string) string {
		parts := reImport.FindStringSubmatch(match)
		importName := parts[1]

		// Normalize import name for OS
		importName = filepath.FromSlash(importName)

		// Split import name into dir and file
		importDir := filepath.Dir(importName)
		importFile := filepath.Base(importName)

		// Current file directory
		currentDir := filepath.Dir(path)

		candidates := []string{
			// Relative: currentDir/importDir/_importFile.scss
			filepath.Join(currentDir, importDir, "_"+importFile+".scss"),
			// Relative: currentDir/importDir/importFile.scss
			filepath.Join(currentDir, importDir, importFile+".scss"),

			// Node Modules: node_modules/importDir/_importFile.scss
			filepath.Join("node_modules", importDir, "_"+importFile+".scss"),
			// Node Modules: node_modules/importDir/importFile.scss
			filepath.Join("node_modules", importDir, importFile+".scss"),

			// Node Modules: node_modules/importName (direct file)
			filepath.Join("node_modules", importName),
			// Node Modules: node_modules/importName.css
			filepath.Join("node_modules", importName+".css"),
		}

		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				importedContent, err := resolveImports(candidate)
				if err == nil {
					return importedContent
				}
			}
		}
		return match // Keep original if not found (or error)
	})

	return content, nil
}
