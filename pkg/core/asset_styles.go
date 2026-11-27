package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// AssetCacheRoot is the ephemeral cache directory under assets/ managed by the
// dev server supervisor and AOT build pipeline. It is removed when the server exits.
var AssetCacheRoot = filepath.Join("assets", ".joss_cache")

// StylesCacheDir holds compiled .css artifacts generated from assets/css/*.scss.
var StylesCacheDir = filepath.Join("assets", ".joss_cache", "css")

// CleanupAssetCache removes the ephemeral assets/.joss_cache directory.
func CleanupAssetCache() {
	_ = os.RemoveAll(AssetCacheRoot)
}

// CompileStyles compiles .scss style files from assets/css into the ephemeral
// cache directory assets/.joss_cache/css without touching public/.
func CompileStyles() {
	compileStyles()
}

var reSCSSVarDecl = regexp.MustCompile(`(\$[a-zA-Z0-9_-]+):\s*(.+?);`)

func compileStyles() {
	files, _ := filepath.Glob("assets/css/*.scss")
	if len(files) == 0 {
		return
	}
	start := time.Now()
	compiledCount := 0
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
		reVar := reSCSSVarDecl
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

		// Write compiled output into ephemeral cache under assets/.joss_cache/css/
		outFile := filepath.Join(StylesCacheDir, strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))+".css")
		_ = os.MkdirAll(filepath.Dir(outFile), 0755)
		compiledCount++
		newBytes := []byte(content)
		if existing, readErr := os.ReadFile(outFile); readErr == nil && string(existing) == content {
			continue
		}
		_ = os.WriteFile(outFile, newBytes, 0644)
	}
	if compiledCount > 0 {
		fmt.Printf("🎨 Estilos compilados en caché (%d archivo(s), %dms)\n", compiledCount, time.Since(start).Milliseconds())
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
