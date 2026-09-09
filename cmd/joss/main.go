package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	_ "modernc.org/sqlite"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/parser"
	_ "github.com/jossecurity/joss/pkg/server"
	"github.com/jossecurity/joss/pkg/template"
	"github.com/jossecurity/joss/pkg/version"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) >= 2 {
		cmd := os.Args[1]
		if cmd == "server" || cmd == "program" {
			// Listener global en background para terminar con la tecla "q" sin requerir Enter
			go func() {
				fd := int(os.Stdin.Fd())
				if term.IsTerminal(fd) {
					state, err := term.MakeRaw(fd)
					if err == nil {
						defer term.Restore(fd, state)
						var buf [1]byte
						for {
							n, err := os.Stdin.Read(buf[:])
							if err != nil || n == 0 {
								break
							}
							char := buf[0]
							// Si se presiona 'q' o 'Q', salimos
							if char == 'q' || char == 'Q' {
								term.Restore(fd, state)
								fmt.Println("\n[Joss] Terminando ejecución por petición del usuario (tecla 'q')...")
								os.Exit(0)
							}
							// Soportar Ctrl+C (ASCII 3) para interrupción estándar
							if char == 3 {
								term.Restore(fd, state)
								os.Exit(0)
							}
						}
					}
				}

				// Fallback si no es una terminal o falla MakeRaw
				reader := bufio.NewReader(os.Stdin)
				for {
					text, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if strings.TrimSpace(text) == "q" {
						fmt.Println("\n[Joss] Terminando ejecución por petición del usuario (tecla 'q')...")
						os.Exit(0)
					}
				}
			}()
		}
	}

	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	// Trigger non-blocking background update check
	checkUpdateBackground()

	command := os.Args[1]

	switch command {
	case "plugin":
		runPluginCommand(os.Args[2:])
	case "update":
		handleUpdateCommand(os.Args[2:])
	case "server":
		if len(os.Args) >= 3 && os.Args[2] == "start" {
			// Always require main.joss
			if _, err := os.Stat("main.joss"); err == nil {
				fmt.Println("[CLI] Ejecutando script de inicio (main.joss)...")
				executeScript("main.joss")
			} else {
				fmt.Println("Error: No se encontró 'main.joss'.")
				fmt.Println("Todos los proyectos deben tener un punto de entrada 'main.joss' que inicie el servidor.")
				os.Exit(1)
			}
		} else {
			fmt.Println("Uso: joss server start")
		}
	case "program":
		if len(os.Args) >= 3 && os.Args[2] == "start" {
			startProgram()
		} else {
			fmt.Println("Uso: joss program start")
		}
	case "format":
		handleFormatCommand(os.Args[2:])
	case "lint":
		handleLintCommand(os.Args[2:])
	case "fix":
		handleFixCommand(os.Args[2:])
	case "test":
		handleTestCommand(os.Args[2:])
	case "check":
		handleCheckCommand(os.Args[2:])
	case "analyze":
		filename := "main.joss"
		if len(os.Args) >= 3 {
			filename = os.Args[2]
		}
		analyzeScript(filename)
	case "run":
		if len(os.Args) < 3 {
			fmt.Println("Uso: joss run [archivo.joss]")
			return
		}
		filename := os.Args[2]
		executeScript(filename)

	case "build":
		target := "web"
		if len(os.Args) >= 3 {
			target = os.Args[2]
		}
		switch target {
		case "native":
			targetOS, targetArch := "", ""
			enableGUI := false
			for _, arg := range os.Args[3:] {
				if arg == "--gui" {
					enableGUI = true
				} else if targetOS == "" {
					targetOS = arg
				} else if targetArch == "" {
					targetArch = arg
				}
			}
			buildNative(targetOS, targetArch, enableGUI)
		case "program":
			targetOS, targetArch := "", ""
			enableGUI := false
			for _, arg := range os.Args[3:] {
				if arg == "--gui" {
					enableGUI = true
				} else if targetOS == "" {
					targetOS = arg
				} else if targetArch == "" {
					targetArch = arg
				}
			}
			if targetOS != "" {
				buildNative(targetOS, targetArch, enableGUI)
			} else {
				buildProgram()
			}
		case "package":
			if len(os.Args) < 4 {
				fmt.Println("Uso: joss build package [ruta_del_paquete]")
				return
			}
			buildPackage(os.Args[3])
		default:
			buildWeb()
		}
	case "make:controller":
		if len(os.Args) < 3 {
			fmt.Println("Uso: joss make:controller [Nombre]")
			return
		}
		createController(os.Args[2])
	case "make:middleware":
		if len(os.Args) < 3 {
			fmt.Println("Uso: joss make:middleware [Nombre]")
			return
		}
		createMiddleware(os.Args[2])
	case "make:model":
		if len(os.Args) < 3 {
			fmt.Println("Uso: joss make:model [Nombre]")
			return
		}
		createModel(os.Args[2])
	case "make:view":
		if len(os.Args) < 3 {
			fmt.Println("Uso: joss make:view [Nombre]")
			return
		}
		createView(os.Args[2])
	case "make:mvc":
		if len(os.Args) < 3 {
			fmt.Println("Uso: joss make:mvc [Nombre]")
			return
		}
		createMVC(os.Args[2])
	case "make:crud":
		if len(os.Args) < 3 {
			fmt.Println("Uso: joss make:crud [Tabla]")
			return
		}
		if err := createCRUD(os.Args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "remove:crud":
		if len(os.Args) < 3 {
			fmt.Println("Uso: joss remove:crud [Tabla]")
			return
		}
		removeCRUD(os.Args[2])
	case "make:migration":
		if len(os.Args) < 3 {
			fmt.Println("Uso: joss make:migration [Nombre]")
			return
		}
		if err := createMigration(os.Args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "db:seed":
		runSeeders()
	case "migrate":
		runMigrations()
	case "migrate:fresh":
		runMigrateFresh()
	case "new":
		if len(os.Args) < 3 {
			fmt.Println("Uso: joss new [web|console|package|plugin] [ruta/nombre]")
			fmt.Println("  joss new [ruta]            - Crea proyecto web (default)")
			fmt.Println("  joss new console [ruta]    - Crea proyecto de consola")
			fmt.Println("  joss new web [ruta]        - Crea proyecto web (explícito)")
			fmt.Println("  joss new package [nombre]  - Crea un nuevo paquete optimizado para Joss")
			fmt.Println("  joss new plugin [ruta]     - Crea un nuevo plugin oficial (.jp Bytecode puro)")
			return
		}

		// Detectar tipo de proyecto
		switch os.Args[2] {
		case "console":
			if len(os.Args) < 4 {
				fmt.Println("Uso: joss new console [ruta]")
				return
			}
			template.CreateConsoleProject(os.Args[3])
		case "web":
			if len(os.Args) < 4 {
				fmt.Println("Uso: joss new web [ruta]")
				return
			}
			template.CreateBibleProject(os.Args[3])
		case "package":
			if len(os.Args) < 4 {
				fmt.Println("Uso: joss new package [nombre]")
				return
			}
			createNewPackage(os.Args[3])
		case "plugin":
			if len(os.Args) < 4 {
				fmt.Println("Uso: joss new plugin [ruta/nombre]")
				return
			}
			createNewPluginProject(os.Args[3])
		default:
			// Default: web project
			template.CreateBibleProject(os.Args[2])
		}
	case "userstorage":
		if len(os.Args) < 3 {
			fmt.Println("Uso: joss userstorage [local | oci | sync-oci | sync-local]")
			return
		}
		handleUserStorage(os.Args[2])
	case "brevo:config":
		handleBrevoConfig()
	case "version":
		fmt.Printf("%s v%s (%s)\n", version.Name, version.Version, version.NameVersion)
	case "pub":
		handlePubCli(os.Args[2:])
	case "package":
		if len(os.Args) == 4 && os.Args[2] == "inspect" {
			inspectPackage(os.Args[3])
		} else {
			fmt.Println("Uso: joss package inspect archivo.jp")
		}
	case "ai:activate":
		activateAI()
	case "change":
		if len(os.Args) < 4 || os.Args[2] != "db" {
			fmt.Println("Uso: joss change db [motor] o joss change db prefix [nuevo_prefijo]")
			return
		}

		switch os.Args[3] {
		case "migrate":
			changeDatabaseMigrate()
		case "prefix":
			if len(os.Args) < 5 {
				fmt.Println("Uso: joss change db prefix [nuevo_prefijo]")
				return
			}
			newPrefix := os.Args[4]
			changeDatabasePrefix(newPrefix)
		default:
			targetEngine := os.Args[3]
			changeDatabaseEngine(targetEngine)
		}
	case "help", "--help", "-h":
		printHelp(os.Args[2:]...)
	default:
		if tryDispatchPluginCommand(command, os.Args[2:]) {
			return
		}
		fmt.Printf("Comando desconocido: %s\n", command)
		if command == "make:miggrate" {
			fmt.Println("¿Quisiste decir 'joss make:migration [Nombre]'?")
		}
		printHelp()
		os.Exit(1)
	}
}

func analyzeScript(filename string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Printf("Error: No se encontró el archivo '%s'.\n", filename)
		if filename == "main.joss" {
			fmt.Println("Todos los proyectos deben tener un punto de entrada 'main.joss' o especificar un archivo: joss analyze [archivo.joss]")
		}
		os.Exit(1)
	}

	fmt.Printf("🔍 Analizando proyecto Joss (%s)...\n", filename)

	units, parseDiagnostics := semanticanalyzer.LoadProject(filename, "app")
	if len(parseDiagnostics) > 0 {
		report := core.AnalysisReportFromDiagnostics(parseDiagnostics)
		report.PrintReport()
		os.Exit(1)
	}

	report := core.AnalyzeSourceUnits(units)
	report.PrintReport()

	if report.HasErrors() {
		os.Exit(1)
	}
}

func executeScript(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error leyendo archivo: %v\n", err)
		return
	}

	l := parser.NewLexer(string(data))
	p := parser.NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		fmt.Println("Errores de parseo:")
		for _, msg := range p.Errors() {
			fmt.Printf("\t%s\n", msg)
		}
		os.Exit(1)
	}

	// Analyze the same project surface that the runtime will preload. Semantic
	// errors are blocking; warnings remain visible but do not prevent execution.
	units, parseDiagnostics := semanticanalyzer.LoadProject(filename, "app")
	if len(parseDiagnostics) > 0 {
		core.AnalysisReportFromDiagnostics(parseDiagnostics).PrintReport()
		os.Exit(1)
	}
	report := core.AnalyzeSourceUnits(units)
	if report.HasIssues() {
		report.PrintReport()
	}
	if report.HasErrors() {
		os.Exit(1)
	}

	rt := core.NewRuntime()
	rt.CurrentFile = filename
	rt.LoadEnv(nil)

	// Preload all .joss files in app/ and subfolders (controllers, models, services, middleware, etc.)
	if _, err := os.Stat("app"); err == nil {
		rt.PreloadAppFiles("app")
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("\n[Error de Ejecución JOSS]\n%s\n", core.FormatPanicAsError(r))
			os.Exit(1)
		}
	}()

	rt.Execute(program)
}

func createNewPackage(name string) {
	fmt.Printf("[Package] Creando nuevo paquete '%s'...\n", name)

	// Create root directory
	if err := os.MkdirAll(name, 0755); err != nil {
		fmt.Printf("Error al crear directorio: %v\n", err)
		return
	}

	// Create src directory
	srcDir := filepath.Join(name, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		fmt.Printf("Error al crear directorio src: %v\n", err)
		return
	}

	// Create joss.yaml
	manifestContent := fmt.Sprintf(`name: %s
version: 1.0.0
description: Libreria optimizada para Joss
repository: ""
license: MIT
type: joss
environment:
  joss: ">=3.6.7"
entry:
  main: src/plugin.joss
dependencies:
`, name)
	if err := os.WriteFile(filepath.Join(name, "joss.yaml"), []byte(manifestContent), 0644); err != nil {
		fmt.Printf("Error al escribir joss.yaml: %v\n", err)
		return
	}

	// Create src/plugin.joss
	className := packageClassName(name)
	pluginContent := fmt.Sprintf(`// plugin.joss
// Se carga automaticamente al declarar %s en joss.yaml.

public class %s {
    public func version(): string {
        return "1.0.0"
    }
}
`, name, className)
	if err := os.WriteFile(filepath.Join(srcDir, "plugin.joss"), []byte(pluginContent), 0644); err != nil {
		fmt.Printf("Error al escribir src/plugin.joss: %v\n", err)
		return
	}

	// Create README.md
	readmeContent := fmt.Sprintf("# %s\n\nPlugin para el lenguaje de programación Joss.\n\n## Compilar\n\n```bash\njoss plugin compile .\njoss plugin inspect %s.jp\n```\n\nEl JP v2 resultante contiene bytecode compilado optimizado (main.jbc) y metadatos de símbolos en `META-INF/joss-symbols.json`. Para compilar plugins escritos en Java, Python, PHP o Rust/Wasm a bytecode nativo de Joss, usa `joss plugin compile <archivo> --lang=<lenguaje>`; consulta `docs/PLUGINS.md`.\n\n## Instalación\n\n```bash\njoss pub add %s\n```\n\nJoss lo carga automáticamente desde `joss.yaml`.\n\n## Uso\n\n```joss\n$plugin = new %s()\n```\n", name, name, name, className)
	if err := os.WriteFile(filepath.Join(name, "README.md"), []byte(readmeContent), 0644); err != nil {
		fmt.Printf("Error al escribir README.md: %v\n", err)
		return
	}

	fmt.Printf("[Package] Paquete '%s' inicializado exitosamente.\n", name)
}

func packageClassName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	var result strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		result.WriteString(strings.ToUpper(part[:1]))
		result.WriteString(part[1:])
	}
	if result.Len() == 0 {
		return "Plugin"
	}
	return result.String()
}
