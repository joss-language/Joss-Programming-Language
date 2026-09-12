package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jossecurity/joss/pkg/formatter"
	"github.com/jossecurity/joss/pkg/i18n"
)

func handleFormatCommand(args []string) {
	write := false
	check := false
	var targetPath string

	for _, arg := range args {
		switch arg {
		case "--write", "-w":
			write = true
		case "--check", "-c":
			check = true
		default:
			if targetPath == "" {
				targetPath = arg
			}
		}
	}

	if targetPath == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error leyendo stdin: %v\n", err)
			os.Exit(1)
		}
		formatted, err := formatter.FormatSource(string(data))
		if err != nil {
			// Si hay un error de sintaxis, devolver el código original para no romper el buffer
			fmt.Print(string(data))
			return
		}
		fmt.Print(formatted)
		return
	}

	if targetPath == "" {
		targetPath = "."
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		fmt.Printf("Error: No se pudo acceder a '%s': %v\n", targetPath, err)
		os.Exit(1)
	}

	if !info.IsDir() {
		changed, err := formatter.FormatFile(targetPath, write || !check)
		if err != nil {
			fmt.Printf("Error formateando %s: %v\n", targetPath, err)
			os.Exit(1)
		}
		if check {
			if changed {
				fmt.Printf("[FORMAT ERROR] %s no cumple con el formato canónico de Joss.\n", targetPath)
				os.Exit(1)
			}
			fmt.Println(i18n.Tr("formatFileOk", map[string]interface{}{"file": targetPath}))
			return
		}
		if changed {
			fmt.Println(i18n.Tr("formatFileFormatted", map[string]interface{}{"file": targetPath}))
		} else {
			fmt.Println(i18n.Tr("formatFileAlready", map[string]interface{}{"file": targetPath}))
		}
		return
	}

	unformatted, err := formatter.FormatDirectory(targetPath, write, check)
	if err != nil {
		fmt.Printf("Error procesando directorio %s: %v\n", targetPath, err)
		os.Exit(1)
	}

	if check {
		if len(unformatted) > 0 {
			fmt.Printf("[FORMAT ERROR] Se encontraron %d archivo(s) sin formatear:\n", len(unformatted))
			for _, file := range unformatted {
				fmt.Printf("  - %s\n", file)
			}
			os.Exit(1)
		}
		fmt.Println(i18n.Tr("formatAllOk"))
		return
	}

	if write {
		fmt.Printf("[FORMAT] %d archivo(s) modificados y formateados.\n", len(unformatted))
	} else {
		if len(unformatted) > 0 {
			fmt.Printf("[FORMAT] %d archivo(s) requieren formato (usa --write para aplicar):\n", len(unformatted))
			for _, file := range unformatted {
				fmt.Printf("  - %s\n", file)
			}
		} else {
			fmt.Println("[FORMAT] Todos los archivos .joss están correctamente formateados.")
		}
	}
}
