package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/plugincompiler"
	"github.com/jossecurity/joss/pkg/pluginpkg"
)

func runPluginCommand(args []string) {
	if len(args) < 1 {
		printPluginUsage()
		return
	}

	subCmd := args[0]
	switch subCmd {
	case "compile":
		handlePluginCompile(args[1:])
	case "inspect":
		handlePluginInspect(args[1:])
	case "verify":
		handlePluginVerify(args[1:])
	default:
		fmt.Printf("Comando de plugin desconocido: %s\n\n", subCmd)
		printPluginUsage()
	}
}

func printPluginUsage() {
	fmt.Println(i18n.Tr("pluginUsageTitle"))
	fmt.Println("  joss plugin compile <dir|archivo> [--lang=java|python|php|wasm] [--name=nombre] [--ver=1.0.0] [--exports=f1,f2]")
	fmt.Println("  joss plugin inspect <plugin.jp>")
	fmt.Println("  joss plugin verify <plugin.jp>")
}

func handlePluginCompile(args []string) {
	if len(args) < 1 {
		fmt.Println(i18n.Tr("pluginSpecifySourceError"))
		fmt.Println("Ejemplo: joss plugin compile MiPlugin.jar --lang=java --name=music-plugin --exports=searchSong,getSong")
		return
	}

	sourcePath := args[0]
	lang := "java"
	name := ""
	version := "1.0.0"
	var exports []string
	var permissions []string

	for _, flag := range args[1:] {
		if strings.HasPrefix(flag, "--lang=") {
			lang = strings.TrimPrefix(flag, "--lang=")
		} else if strings.HasPrefix(flag, "--name=") {
			name = strings.TrimPrefix(flag, "--name=")
		} else if strings.HasPrefix(flag, "--ver=") {
			version = strings.TrimPrefix(flag, "--ver=")
		} else if strings.HasPrefix(flag, "--exports=") {
			expStr := strings.TrimPrefix(flag, "--exports=")
			exports = strings.Split(expStr, ",")
		} else if strings.HasPrefix(flag, "--permissions=") {
			permStr := strings.TrimPrefix(flag, "--permissions=")
			permissions = strings.Split(permStr, ",")
		}
	}

	// Auto-detect parameters from joss.yaml if present
	manifestPath := filepath.Join(sourcePath, "joss.yaml")
	if info, err := os.Stat(sourcePath); err == nil && !info.IsDir() {
		manifestPath = filepath.Join(filepath.Dir(sourcePath), "joss.yaml")
	}

	if data, err := os.ReadFile(manifestPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "name:") && name == "" {
				name = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "name:")), "\"'")
			} else if strings.HasPrefix(trimmed, "version:") {
				version = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "version:")), "\"'")
			} else if strings.HasPrefix(trimmed, "language:") {
				lang = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "language:")), "\"'")
			}
		}
	}

	if name == "" {
		base := filepath.Base(sourcePath)
		ext := filepath.Ext(base)
		name = strings.TrimSuffix(base, ext)
	}

	// If it's a pure Joss plugin project (joss.yaml type: joss or src/plugin.joss exists)
	jossEntry := filepath.Join(sourcePath, "src", "plugin.joss")
	if _, err := os.Stat(jossEntry); err == nil {
		buildPackage(sourcePath)
		return
	}
	if info, err := os.Stat(sourcePath); err == nil && !info.IsDir() && strings.HasSuffix(sourcePath, ".joss") {
		buildPackage(filepath.Dir(sourcePath))
		return
	}

	fmt.Println(i18n.Tr("pluginCompiling", i18n.M{"path": sourcePath, "lang": lang}))

	opts := plugincompiler.Options{
		SourceDir:   filepath.Dir(sourcePath),
		Language:    lang,
		EntryFile:   sourcePath,
		Name:        name,
		Version:     version,
		Exports:     exports,
		Permissions: permissions,
		MaxSizeMB:   1.0,
	}

	outPath, result, err := plugincompiler.CompileProject(opts)
	if err != nil {
		fmt.Println(i18n.Tr("pluginCompileError", i18n.M{"error": err.Error()}))
		return
	}

	fi, _ := os.Stat(outPath)
	sizeKB := float64(fi.Size()) / 1024.0

	fmt.Println(i18n.Tr("pluginCompileSuccess"))
	fmt.Println("  " + i18n.Tr("pluginGenerated", i18n.M{"path": outPath}))
	fmt.Printf("  %s\n", i18n.Tr("pluginPackageSize", i18n.M{"size": fmt.Sprintf("%.2f", sizeKB)}))
	fmt.Println("  " + i18n.Tr("pluginTreeShaking", i18n.M{"retained": result.OptimizedFuncs, "removed": result.RemovedFuncs}))
	fmt.Println("  " + i18n.Tr("pluginStructuresRetained", i18n.M{"retained": result.OptimizedStructs, "removed": result.RemovedStructs}))
}

func handlePluginInspect(args []string) {
	if len(args) < 1 {
		fmt.Println(i18n.Tr("pluginInspectSpecifyError"))
		fmt.Println("Ejemplo: joss plugin inspect music-plugin.jp")
		return
	}

	jpPath := args[0]
	archive, err := os.ReadFile(jpPath)
	if err != nil {
		fmt.Println(i18n.Tr("pluginOpenError", i18n.M{"path": jpPath, "error": err.Error()}))
		return
	}

	pkg, err := pluginpkg.Read(archive)
	if err != nil {
		fmt.Println(i18n.Tr("pluginDecodeError", i18n.M{"path": jpPath, "error": err.Error()}))
		return
	}

	fi, _ := os.Stat(jpPath)
	sizeKB := float64(fi.Size()) / 1024.0

	fmt.Println("========================================")
	fmt.Println(" " + i18n.Tr("pluginInspectTitle", i18n.M{"name": pkg.Metadata.Name}))
	fmt.Println(" " + i18n.Tr("pluginInspectVersion", i18n.M{"version": pkg.Metadata.Version}))
	fmt.Println(" " + i18n.Tr("pluginBytecodeTarget"))
	fmt.Printf("  %s\n", i18n.Tr("pluginPackageSize", i18n.M{"size": fmt.Sprintf("%.2f", sizeKB)}))
	if pkg.Metadata.Signature != "" {
		fmt.Println(" " + i18n.Tr("pluginSignatureVerifiedStatus", i18n.M{"algo": pkg.Metadata.SignatureAlgorithm, "key": pkg.Metadata.KeyID}))
	}
	fmt.Println("----------------------------------------")
	fmt.Println(" " + i18n.Tr("pluginExportedFunctions"))
	for _, exp := range pkg.Metadata.Exports {
		fmt.Printf("   - %s()\n", exp)
	}
	if pkg.Metadata.Symbols != "" {
		if symData, ok := pkg.Files[pkg.Metadata.Symbols]; ok {
			var symbols pluginpkg.SymbolIndex
			if err := json.Unmarshal(symData, &symbols); err == nil {
				if len(symbols.Classes) > 0 {
					fmt.Println(" " + i18n.Tr("pluginDeclaredClasses"))
					for _, cls := range symbols.Classes {
						fmt.Printf("   - class %s\n", cls.Name)
						for _, m := range cls.Methods {
							fmt.Printf("       method %s()\n", m.Name)
						}
					}
				}
			}
		}
	}
	fmt.Println(" " + i18n.Tr("pluginDeclaredPermissions"))
	if len(pkg.Metadata.Permissions) == 0 {
		fmt.Println("   " + i18n.Tr("pluginPermissionsNone"))
	} else {
		for _, perm := range pkg.Metadata.Permissions {
			fmt.Printf("   - %s\n", perm)
		}
	}
	fmt.Println("========================================")
}

func handlePluginVerify(args []string) {
	if len(args) < 1 {
		fmt.Println(i18n.Tr("pluginVerifySpecifyError"))
		fmt.Println("Ejemplo: joss plugin verify mi_plugin.jp")
		return
	}

	jpPath := args[0]
	archive, err := os.ReadFile(jpPath)
	if err != nil {
		fmt.Println("❌ " + i18n.Tr("pluginOpenError", i18n.M{"path": jpPath, "error": err.Error()}))
		os.Exit(1)
	}

	pkg, err := pluginpkg.ReadVerified(archive)
	if err != nil {
		fmt.Println("❌ " + i18n.Tr("pluginVerificationError", i18n.M{"path": jpPath, "error": err.Error()}))
		os.Exit(1)
	}

	fmt.Println("========================================")
	fmt.Println(" " + i18n.Tr("pluginInspectTitle", i18n.M{"name": pkg.Metadata.Name}))
	fmt.Println(" " + i18n.Tr("pluginInspectVersion", i18n.M{"version": pkg.Metadata.Version}))
	fmt.Println(" " + i18n.Tr("pluginSignatureValid"))
	if pkg.Metadata.SignatureAlgorithm != "" {
		fmt.Println(" " + i18n.Tr("pluginSignatureAlgorithm", i18n.M{"algo": pkg.Metadata.SignatureAlgorithm, "key": pkg.Metadata.KeyID}))
	}
	fmt.Println("========================================")
}
