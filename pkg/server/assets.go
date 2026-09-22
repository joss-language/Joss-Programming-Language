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

// optimizeCSS removes CSS blocks that are not used in views
func optimizeCSS(cssContent string) string {
	fmt.Println("  [Optimizer] Analizando uso de CSS...")

	// 1. Collect all Tokens from Views
	viewTokens := make(map[string]bool)
	viewFiles, _ := filepath.Glob("app/views/**/*.joss.html")
	// Also check root views
	rootViews, _ := filepath.Glob("app/views/*.joss.html")
	viewFiles = append(viewFiles, rootViews...)

	for _, file := range viewFiles {
		data, err := os.ReadFile(file)
		if err == nil {
			content := string(data)
			// Simple Regex to capture class="foo bar" and id="baz"
			// and also clean strings that might be used in JS: "foo-bar"
			reToken := regexp.MustCompile(`[\w-]+`)
			matches := reToken.FindAllString(content, -1)
			for _, m := range matches {
				viewTokens[m] = true
			}
		}
	}

	// Always whitelist standard HTML tags
	htmlTags := []string{"body", "html", "div", "span", "a", "p", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "form", "input", "button", "nav", "header", "footer", "main", "section", "article", "*"}
	for _, tag := range htmlTags {
		viewTokens[tag] = true
	}

	// 2. Parse CSS Blocks
	// We matched balanced braces approx: selector { content }
	// Note: This regex is simple and might fail on nested media queries.
	// For production a real CSS parser is better, but this fits the "interpreter" vibe.
	// Matches:  selector { ... }
	// We split by "}" to process blocks? No, regex replace is safer.

	// Strategy: Split into blocks "selector { body }"
	// We use a simple state machine or regex split.
	// Let's use Regex for simple top-level blocks.
	// RE: ([^{]+)\{([^}]+)\}

	reBlock := regexp.MustCompile(`(?s)([^{}]+)\{([^{}]+)\}`)

	optimized := reBlock.ReplaceAllStringFunc(cssContent, func(block string) string {
		parts := reBlock.FindStringSubmatch(block)
		if len(parts) < 3 {
			return block
		}
		selectorLine := strings.TrimSpace(parts[1])
		// body := parts[2] // Unused

		// Check if selector is used
		// Selectors: .card, #header, div > span, .btn.active
		// We tokenize the selector and check if ALL *meaningful* tokens exist.
		// Meaningful: starts with . or #, or is a tag.

		// Split selector by comma for multiple selectors: .a, .b { }
		subSelectors := strings.Split(selectorLine, ",")
		keepBlock := false

		for _, sel := range subSelectors {
			sel = strings.TrimSpace(sel)
			// Tokenize selector: .btn -> btn, #id -> id, div -> div
			// Ignore >, +, ~, :hover, ::before
			reSelToken := regexp.MustCompile(`([.#]?[\w-]+)`)
			selTokens := reSelToken.FindAllString(sel, -1)

			selectorMatch := true
			hastokens := false

			for _, t := range selTokens {
				if t == "" {
					continue
				}
				cleanToken := strings.TrimLeft(t, ".#")
				// Pseudo-classes check
				if strings.HasPrefix(t, ":") {
					continue
				}

				hastokens = true
				if !viewTokens[cleanToken] {
					selectorMatch = false
					break
				}
			}

			if hastokens && selectorMatch {
				keepBlock = true
				break
			}
			// Keep At-Rules (@media, @keyframes, @import)
			if strings.HasPrefix(sel, "@") {
				keepBlock = true
				break
			}
		}

		if keepBlock {
			return block
		}

		// fmt.Printf("  [Optimizer] Eliminado: %s\n", strings.Split(selectorLine, "\n")[0])
		return "" // Remove block
	})

	// Clean up empty lines left behind
	reEmptyLines := regexp.MustCompile(`\n\s*\n`)
	optimized = reEmptyLines.ReplaceAllString(optimized, "\n")

	initialSize := len(cssContent)
	finalSize := len(optimized)
	saved := initialSize - finalSize
	fmt.Printf("  [Optimizer] Reducción: %d bytes (%.2f%%)\n", saved, float64(saved)/float64(initialSize)*100)

	return optimized
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
