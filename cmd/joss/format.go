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
			fmt.Fprintln(os.Stderr, i18n.Tr("formatStdinError", i18n.M{"error": err.Error()}))
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
		fmt.Println(i18n.Tr("formatAccessError", i18n.M{"path": targetPath, "error": err.Error()}))
		os.Exit(1)
	}

	if !info.IsDir() {
		changed, err := formatter.FormatFile(targetPath, write || !check)
		if err != nil {
			fmt.Println(i18n.Tr("formatFileError", i18n.M{"path": targetPath, "error": err.Error()}))
			os.Exit(1)
		}
		if check {
			if changed {
				fmt.Println(i18n.Tr("formatNotCanonical", i18n.M{"path": targetPath}))
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
		fmt.Println(i18n.Tr("formatDirError", i18n.M{"path": targetPath, "error": err.Error()}))
		os.Exit(1)
	}

	if check {
		if len(unformatted) > 0 {
			fmt.Println(i18n.Tr("formatUnformattedFound", i18n.M{"count": len(unformatted)}))
			for _, file := range unformatted {
				fmt.Printf("  - %s\n", file)
			}
			os.Exit(1)
		}
		fmt.Println(i18n.Tr("formatAllOk"))
		return
	}

	if write {
		fmt.Println(i18n.Tr("formatModifiedCount", i18n.M{"count": len(unformatted)}))
	} else {
		if len(unformatted) > 0 {
			fmt.Println(i18n.Tr("formatRequireFormat", i18n.M{"count": len(unformatted)}))
			for _, file := range unformatted {
				fmt.Printf("  - %s\n", file)
			}
		} else {
			fmt.Println(i18n.Tr("formatAllFilesClean"))
		}
	}
}
