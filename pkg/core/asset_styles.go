package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CompileStyles compiles .scss style files from assets/css into public/css.
func CompileStyles() {
	compileStyles()
}

func compileStyles() {
	fmt.Println("Compilando estilos...")
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
		reVar := regexp.MustCompile(`(\$[a-zA-Z0-9_-]+):\s*(.+?);`)
		content = reVar.ReplaceAllStringFunc(content, func(match string) string {
			parts := reVar.FindStringSubmatch(match)
			vars[parts[1]] = parts[2]
			return "" // Remove variable definition from output
		})

		// 2. Replace complete variable tokens
		content = replaceSCSSVariables(content, vars)

		// 3. Strip standalone SCSS single-line comments (// ...) without touching URLs
		content = stripSCSSLineComments(content)
		content = optimizeCSS(content)

		// Change extension to .css
		outFile := filepath.Join("public", "css", strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))+".css")
		_ = os.MkdirAll(filepath.Dir(outFile), 0755)
		newBytes := []byte(content)
		if existing, readErr := os.ReadFile(outFile); readErr == nil && string(existing) == content {
			continue
		}
		_ = os.WriteFile(outFile, newBytes, 0644)
	}

	// Also copy plain .css files from assets/css into public/css
	cssFiles, _ := filepath.Glob("assets/css/*.css")
	for _, file := range cssFiles {
		if strings.HasPrefix(filepath.Base(file), "_") {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		outFile := filepath.Join("public", "css", filepath.Base(file))
		_ = os.MkdirAll(filepath.Dir(outFile), 0755)
		_ = os.WriteFile(outFile, content, 0644)
	}
}

var scssVariableReference = regexp.MustCompile(`\$[a-zA-Z0-9_-]+`)
var scssStandaloneLineComment = regexp.MustCompile(`(?m)^[ \t]*//[^\r\n]*\r?\n?`)
var cssBlockComment = regexp.MustCompile(`/\*[\s\S]*?\*/`)
var cssMultiWhitespace = regexp.MustCompile(`[ \t\r\n]+`)

func optimizeCSS(content string) string {
	cleaned := cssBlockComment.ReplaceAllString(content, "")
	cleaned = cssMultiWhitespace.ReplaceAllString(cleaned, " ")
	return strings.TrimSpace(cleaned)
}

func stripSCSSLineComments(content string) string {
	return scssStandaloneLineComment.ReplaceAllString(content, "")
}

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

		importName = filepath.FromSlash(importName)
		importDir := filepath.Dir(importName)
		importFile := filepath.Base(importName)
		currentDir := filepath.Dir(path)

		candidates := []string{
			filepath.Join(currentDir, importDir, "_"+importFile+".scss"),
			filepath.Join(currentDir, importDir, importFile+".scss"),
			filepath.Join("node_modules", importDir, "_"+importFile+".scss"),
			filepath.Join("node_modules", importDir, importFile+".scss"),
			filepath.Join("node_modules", importName),
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
		return match
	})

	return content, nil
}
