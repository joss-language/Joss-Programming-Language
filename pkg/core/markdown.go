package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// executeMarkdownMethod handles Markdown native methods
func (r *Runtime) executeMarkdownMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "toHtml":
		// Markdown::toHtml(string $content) - Convert markdown string to HTML
		if len(args) != 1 {
			fmt.Println("Error: Markdown::toHtml requiere 1 argumento (content)")
			return nil
		}

		content, ok := args[0].(string)
		if !ok {
			fmt.Println("Error: El argumento de Markdown::toHtml debe ser un string")
			return nil
		}

		return markdownToHTML(content)

	case "readFile":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		// Markdown::readFile(string $path) - Read markdown file and return HTML
		if len(args) != 1 {
			fmt.Println("Error: Markdown::readFile requiere 1 argumento (path)")
			return nil
		}

		path, ok := args[0].(string)
		if !ok {
			fmt.Println("Error: El argumento de Markdown::readFile debe ser un string")
			return nil
		}

		return readMarkdownFile(path)

	default:
		fmt.Printf("Error: Método desconocido Markdown::%s\n", method)
		return nil
	}
}

// markdownToHTML converts markdown content to HTML
func markdownToHTML(content string) string {
	// Create markdown parser with extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.Footnotes
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse([]byte(content))

	// Create HTML renderer with options
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{
		Flags: htmlFlags,
	}
	renderer := html.NewRenderer(opts)

	// Render to HTML
	htmlContent := markdown.Render(doc, renderer)

	return string(htmlContent)
}

// readMarkdownFile reads a markdown file and converts it to HTML
func readMarkdownFile(path string) string {
	// Security: Prevent directory traversal
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		fmt.Println("Error de seguridad: Ruta inválida (directory traversal detectado)")
		return ""
	}

	// Read file
	content, err := os.ReadFile(cleanPath)
	if err != nil {
		fmt.Printf("Error al leer archivo markdown: %v\n", err)
		return ""
	}

	// Convert to HTML
	return markdownToHTML(string(content))
}
