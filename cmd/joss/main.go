package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	_ "modernc.org/sqlite"

	"github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/mobile"
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
								fmt.Println("\n" + i18n.Tr("cliTerminateByKey"))
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
						fmt.Println("\n" + i18n.Tr("cliTerminateByKey"))
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
	case "indexnow":
		handleIndexNowCommand(os.Args[2:])
	case "server":
		if len(os.Args) >= 3 && os.Args[2] == "start" {
			// Always require main.joss
			if _, err := os.Stat("main.joss"); err == nil {
				fmt.Println(i18n.Tr("cliExecMain"))
				fmt.Println("  main.joss")
				executeScript("main.joss")
			} else {
				fmt.Println(i18n.Tr("cliErrNoMain"))
				fmt.Println("  main.joss")
				fmt.Println(i18n.Tr("cliErrRequireMain"))
				os.Exit(1)
			}
		} else {
			fmt.Printf("%s joss server start\n", i18n.Tr("cliUsageLabel"))
		}
	case "program":
		if len(os.Args) >= 3 && os.Args[2] == "start" {
			startProgram()
		} else {
			fmt.Printf("%s joss program start\n", i18n.Tr("cliUsageLabel"))
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
	case "emit-ir":
		filename := "main.joss"
		outFile := ""
		for i := 2; i < len(os.Args); i++ {
			if strings.HasPrefix(os.Args[i], "-o=") {
				outFile = strings.TrimPrefix(os.Args[i], "-o=")
			} else if os.Args[i] == "-o" && i+1 < len(os.Args) {
				outFile = os.Args[i+1]
				i++
			} else if !strings.HasPrefix(os.Args[i], "-") {
				filename = os.Args[i]
			}
		}
		emitIRCommand(filename, outFile)
	case "analyze":
		filename := "main.joss"
		if len(os.Args) >= 3 {
			filename = os.Args[2]
		}
		analyzeScript(filename)
	case "eval":
		if len(os.Args) < 3 {
			fmt.Printf("%s joss eval \"codigo\" [--json]\n", i18n.Tr("cliUsageLabel"))
			return
		}
		asJSON := false
		source := ""
		for _, arg := range os.Args[2:] {
			if arg == "--json" {
				asJSON = true
			} else if source == "" {
				source = arg
			}
		}
		executeEval(source, asJSON)
	case "run":
		if len(os.Args) < 3 {
			fmt.Printf("%s joss run [archivo.joss | -e \"codigo\"]\n", i18n.Tr("cliUsageLabel"))
			return
		}
		if os.Args[2] == "-e" {
			if len(os.Args) < 4 {
				fmt.Printf("%s joss run -e \"codigo\"\n", i18n.Tr("cliUsageLabel"))
				return
			}
			executeEval(os.Args[3], false)
			return
		}
		filename := os.Args[2]
		executeScript(filename)
	case "repl":
		runRepl()

	case "build":
		handleBuildCommand(os.Args[2:])
	case "make:controller":
		if len(os.Args) < 3 {
			fmt.Printf("%s joss make:controller [Nombre]\n", i18n.Tr("cliUsageLabel"))
			return
		}
		createController(os.Args[2])
	case "make:middleware":
		if len(os.Args) < 3 {
			fmt.Printf("%s joss make:middleware [Nombre]\n", i18n.Tr("cliUsageLabel"))
			return
		}
		createMiddleware(os.Args[2])
	case "make:model":
		if len(os.Args) < 3 {
			fmt.Printf("%s joss make:model [Nombre]\n", i18n.Tr("cliUsageLabel"))
			return
		}
		createModel(os.Args[2])
	case "make:view":
		if len(os.Args) < 3 {
			fmt.Printf("%s joss make:view [Nombre]\n", i18n.Tr("cliUsageLabel"))
			return
		}
		createView(os.Args[2])
	case "make:mvc":
		if len(os.Args) < 3 {
			fmt.Printf("%s joss make:mvc [Nombre]\n", i18n.Tr("cliUsageLabel"))
			return
		}
		createMVC(os.Args[2])
	case "make:crud":
		if len(os.Args) < 3 {
			fmt.Printf("%s joss make:crud [Tabla]\n", i18n.Tr("cliUsageLabel"))
			return
		}
		if err := createCRUD(os.Args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "remove:crud":
		if len(os.Args) < 3 {
			fmt.Printf("%s joss remove:crud [Tabla]\n", i18n.Tr("cliUsageLabel"))
			return
		}
		removeCRUD(os.Args[2])
	case "make:migration":
		if len(os.Args) < 3 {
			fmt.Printf("%s joss make:migration [Nombre]\n", i18n.Tr("cliUsageLabel"))
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

	case "change":
		if len(os.Args) < 4 || os.Args[2] != "db" {
			fmt.Printf("%s joss change db [motor] o joss change db prefix [nuevo_prefijo]\n", i18n.Tr("cliUsageLabel"))
			return
		}

		switch os.Args[3] {
		case "migrate":
			changeDatabaseMigrate()
		case "prefix":
			if len(os.Args) < 5 {
				fmt.Printf("%s joss change db prefix [nuevo_prefijo]\n", i18n.Tr("cliUsageLabel"))
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
		fmt.Println(i18n.Tr("cliUnknownCommand", i18n.M{"command": command}))
		if command == "make:miggrate" {
			fmt.Println(i18n.Tr("cliDidYouMean"))
			fmt.Println("  joss make:migration [Nombre]")
		}
		printHelp()
		os.Exit(1)
	}
}

func analyzeScript(filename string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Println(i18n.Tr("cliScriptNotFound", i18n.M{"file": filename}))
		if filename == "main.joss" {
			fmt.Println(i18n.Tr("cliRequireMainOrScript"))
			fmt.Println("  main.joss")
			fmt.Println("  joss analyze [archivo.joss]")
		}
		os.Exit(1)
	}

	fmt.Printf("🔍 %s\n", i18n.Tr("cliAnalyzingProject", i18n.M{"file": filename}))

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

func emitIRCommand(filename, outFile string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Printf("Error: archivo '%s' no encontrado.\n", filename)
		os.Exit(1)
	}

	units, parseDiagnostics := semanticanalyzer.LoadProject(filename, "app")
	if len(parseDiagnostics) > 0 {
		report := core.AnalysisReportFromDiagnostics(parseDiagnostics)
		report.PrintReport()
		os.Exit(1)
	}

	report := core.AnalyzeSourceUnits(units)
	if report.HasErrors() {
		report.PrintReport()
		os.Exit(1)
	}

	lowerer := ir.NewLowerer(report.Prepared, nil)
	progName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	irProg, err := lowerer.LowerProgram(progName)
	if err != nil {
		fmt.Printf("Error generando IR: %v\n", err)
		os.Exit(1)
	}

	verifier := ir.NewVerifier()
	if err := verifier.Verify(irProg); err != nil {
		fmt.Printf("Error de verificación IR: %v\n", err)
		os.Exit(1)
	}

	dump := irProg.Dump()
	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(dump), 0644); err != nil {
			fmt.Printf("Error escribiendo archivo IR '%s': %v\n", outFile, err)
			os.Exit(1)
		}
		fmt.Printf("✓ IR generado exitosamente en: %s\n", outFile)
	} else {
		fmt.Print(dump)
	}
}

func handleBuildCommand(args []string) {
	if len(args) == 0 {
		if _, err := os.Stat("main.joss"); err == nil {
			buildNativeProgram("main.joss", "", "", false, false, false, false, "auto")
			return
		}
		fmt.Printf("%s joss build [archivo.joss] [opciones]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println("\nOpciones:")
		fmt.Println("  -o <salida>          Nombre o ruta del binario nativo de salida")
		fmt.Println("  --target=<os>-<arch> Objetivo de compilación cruzada (ej. windows-amd64, linux-amd64)")
		fmt.Println("  --release            Compilación optimizada sin símbolos de depuración")
		fmt.Println("  --debug              Compilación con información de depuración")
		fmt.Println("  --trace              Emite y conserva artefactos intermedios (.ir, .ll, .standalone.go)")
		fmt.Println("\nSubcomandos de distribución:")
		fmt.Println("  package <ruta>       Empaquetar librería o plugin en formato .jp")
		fmt.Println("  web                  Preparar bundle de archivos y assets para despliegue web")
		return
	}

	first := args[0]
	// Subcomandos de paquetes y bundles existentes
	if first == "bundle" || first == "app" {
		tOS := ""
		tArch := ""
		gui := false
		outPath := ""
		for _, a := range args[1:] {
			if a == "--gui" {
				gui = true
			} else if strings.HasPrefix(a, "--target=") {
				parts := strings.Split(strings.TrimPrefix(a, "--target="), "-")
				if len(parts) == 2 {
					tOS = parts[0]
					tArch = parts[1]
				}
			} else if strings.HasPrefix(a, "-o=") {
				outPath = strings.TrimPrefix(a, "-o=")
			}
		}
		buildNativeWithOutput(tOS, tArch, gui, outPath)
		return
	}
	if first == "package" {
		if len(args) < 2 {
			fmt.Printf("%s joss build package [ruta_del_paquete]\n", i18n.Tr("cliUsageLabel"))
			return
		}
		buildPackage(args[1])
		return
	}
	if first == "web" {
		buildWeb()
		return
	}

	// Subcomandos históricos en deprecación redirigidos de forma transparente
	shift := 0
	if first == "native-backend" || first == "native" {
		fmt.Println("[aviso] 'joss build native' y 'joss build native-backend' están en deprecación. Usa directamente 'joss build [archivo.joss]'")
		shift = 1
	} else if first == "program" {
		fmt.Println("[aviso] 'joss build program' está en deprecación. Usa directamente 'joss build [archivo.joss]'")
		shift = 1
	}

	remaining := args[shift:]
	filename := ""
	outExe := ""
	targetStr := ""
	release := true
	debug := false
	trace := false
	backendName := "auto"

	enableGUI := false

	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]
		if strings.HasPrefix(arg, "-o=") {
			outExe = strings.TrimPrefix(arg, "-o=")
		} else if arg == "-o" && i+1 < len(remaining) {
			outExe = remaining[i+1]
			i++
		} else if strings.HasPrefix(arg, "--target=") {
			targetStr = strings.TrimPrefix(arg, "--target=")
		} else if arg == "--target" && i+1 < len(remaining) {
			targetStr = remaining[i+1]
			i++
		} else if arg == "--release" {
			release = true
		} else if arg == "--debug" {
			debug = true
			release = false
		} else if arg == "--trace" {
			trace = true
		} else if arg == "--gui" {
			enableGUI = true
		} else if strings.HasPrefix(arg, "--backend=") {
			backendName = strings.TrimPrefix(arg, "--backend=")
		} else if !strings.HasPrefix(arg, "-") {
			if filename == "" {
				filename = arg
			}
		}
	}

	if filename == "" {
		if _, err := os.Stat("main.joss"); err == nil {
			filename = "main.joss"
		} else {
			fmt.Printf("Error: debes especificar un archivo fuente para compilar (ej. 'joss build main.joss')\n")
			os.Exit(1)
		}
	}

	buildNativeProgram(filename, outExe, targetStr, release, debug, trace, enableGUI, backendName)
}

func buildNativeProgram(filename, outExe, targetStr string, release, debug, trace, enableGUI bool, backendName string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Printf("Error: archivo '%s' no encontrado.\n", filename)
		os.Exit(1)
	}

	target, err := native.ParseTarget(targetStr)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if outExe == "" {
		base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
		if base == "" {
			base = "app"
		}
		if target.OS == "windows" {
			outExe = base + ".exe"
		} else {
			outExe = base
		}
	} else if target.OS == "windows" && !strings.HasSuffix(strings.ToLower(outExe), ".exe") {
		outExe += ".exe"
	}

	fmt.Printf("🔨 Compilando nativamente: %s -> %s (target: %s)\n", filename, outExe, target.DashString())
	if trace {
		fmt.Println("  [trace] 1. Analizando código fuente con frontend...")
	}
	units, parseDiagnostics := semanticanalyzer.LoadProject(filename, "app")
	if len(parseDiagnostics) > 0 {
		core.AnalysisReportFromDiagnostics(parseDiagnostics).PrintReport()
		os.Exit(1)
	}

	report := core.AnalyzeSourceUnits(units)
	if report.HasErrors() {
		report.PrintReport()
		os.Exit(1)
	}

	if trace {
		fmt.Println("  [trace] 2. Generando Joss Native IR (Lowering)...")
	}
	lowerer := ir.NewLowerer(report.Prepared, nil)
	progName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	irProg, err := lowerer.LowerProgram(progName)
	if err != nil {
		fmt.Printf("Error de compilación nativa AOT (Lowering): %v\n", err)
		fmt.Println("Sugerencia: La compilación nativa AOT requiere soporte en Joss Native IR.")
		fmt.Println("Para empaquetar una aplicación completa con el runtime, usa 'joss package' o 'joss build bundle'.")
		os.Exit(1)
	}

	if trace {
		fmt.Println("  [trace] 3. Verificando integridad de CFG y tipos en IR...")
	}
	verifier := ir.NewVerifier()
	if err := verifier.Verify(irProg); err != nil {
		fmt.Printf("Error de compilación nativa AOT (Verificación de IR): %v\n", err)
		fmt.Println("Sugerencia: El IR generado contiene instrucciones o tipos no válidos para el backend nativo.")
		os.Exit(1)
	}

	bKind := native.BackendAuto
	switch strings.ToLower(backendName) {
	case "llvm":
		bKind = native.BackendLLVM
	case "standalone":
		bKind = native.BackendStandalone
	default:
		bKind = native.BackendAuto
	}

	if trace {
		fmt.Printf("  [trace] 4. Ejecutando backend nativo (solicitado: %s)...\n", bKind)
	}

	res, err := native.BuildProgram(irProg, native.BuildOptions{
		Target:     target,
		Backend:    bKind,
		OutputPath: outExe,
		Trace:      trace,
		Debug:      debug,
		Release:    release,
	})
	if err != nil {
		fmt.Printf("Error generando ejecutable nativo: %v\n", err)
		os.Exit(1)
	}

	sizeKB := float64(res.ExecutableSize) / 1024.0
	if res.IsBootstrap {
		fmt.Printf("  [bootstrap] %s\n", res.ToolchainNotice)
		fmt.Printf("✓ Ejecutable generado con éxito (bootstrap): %s (%.1f KB) [target: %s]\n", outExe, sizeKB, res.Target.DashString())
	} else {
		fmt.Printf("✓ Ejecutable nativo real generado con éxito: %s (%.1f KB) [backend: %s, target: %s]\n", outExe, sizeKB, res.BackendUsed, res.Target.DashString())
	}
	if trace {
		if res.IRDumpPath != "" {
			fmt.Printf("  [trace] Artefacto IR: %s\n", res.IRDumpPath)
		}
		if res.LLVMPath != "" {
			fmt.Printf("  [trace] Artefacto LLVM: %s\n", res.LLVMPath)
		}
		if res.BootstrapPath != "" {
			fmt.Printf("  [trace] Artefacto Bootstrap: %s\n", res.BootstrapPath)
		}
	}
}

func buildNativeBackend(filename, outExe string, backendName string, trace bool) {
	buildNativeProgram(filename, outExe, "", false, false, trace, false, backendName)
}

func executeScript(filename string) {
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
	program := report.Prepared.Entrypoint()
	if program == nil {
		fmt.Println(i18n.Tr("cliReadFileError", i18n.M{"error": "entrypoint was not prepared"}))
		return
	}

	rt := core.NewRuntime()
	rt.CurrentFile = filename
	rt.LoadEnv(nil)

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("\n[Error de Ejecución JOSS]\n%s\n", core.FormatPanicAsError(r))
			os.Exit(1)
		}
	}()

	rt.ExecutePrepared(report.Prepared)
}

func executeEval(source string, asJSON bool) {
	if asJSON {
		fmt.Println(mobile.Run(source, 0))
		return
	}
	res := mobile.RunDirect(source, 0)
	if res.Stdout != "" {
		fmt.Print(res.Stdout)
	}
	if res.Stderr != "" {
		fmt.Fprint(os.Stderr, res.Stderr)
	}
	if !res.Success {
		if res.Error != "" {
			fmt.Fprintln(os.Stderr, res.Error)
		}
		os.Exit(1)
	}
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

func runRepl() {
	fmt.Printf("Joss %s Interactive REPL\n", version.Version)
	fmt.Println("Escribe expresiones o sentencias Joss. Escribe 'exit' o presiona Ctrl+C para salir.")
	fmt.Println()

	rt := core.NewRuntime()
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("joss> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "exit" || trimmed == "quit" {
			break
		}

		lexer := parser.NewLexer(trimmed)
		p := parser.NewParser(lexer)
		prog := p.ParseProgram()

		if len(p.Errors()) > 0 {
			for _, e := range p.Errors() {
				fmt.Printf("Error: %s\n", e)
			}
			continue
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Runtime Error: %v\n", r)
				}
			}()
			for _, stmt := range prog.Statements {
				if exprStmt, ok := stmt.(*parser.ExpressionStatement); ok {
					res := rt.EvaluateExpression(exprStmt.Expression)
					if res != nil {
						fmt.Printf("=> %v\n", res)
					}
				} else {
					rt.ExecuteStatement(stmt)
				}
			}
		}()
	}
}

func handleIndexNowCommand(args []string) {
	if len(args) == 0 || args[0] != "generate" {
		fmt.Println("Uso: joss indexnow generate")
		return
	}

	// Generate 32-character secure hex key
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		fmt.Printf("Error al generar clave criptográfica: %v\n", err)
		return
	}
	key := hex.EncodeToString(bytes)

	// Update or create .env
	envPath := ".env"
	envContent := ""
	if data, err := os.ReadFile(envPath); err == nil {
		envContent = string(data)
	}

	keyLine := fmt.Sprintf("INDEXNOW_KEY=%s", key)
	if strings.Contains(envContent, "INDEXNOW_KEY=") {
		lines := strings.Split(envContent, "\n")
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "INDEXNOW_KEY=") {
				lines[i] = keyLine
				break
			}
		}
		envContent = strings.Join(lines, "\n")
	} else {
		if envContent != "" && !strings.HasSuffix(envContent, "\n") {
			envContent += "\n"
		}
		envContent += keyLine + "\n"
	}

	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		fmt.Printf("Error al escribir en .env: %v\n", err)
		return
	}

	fmt.Println("✓ IndexNow configurado exitosamente:")
	fmt.Printf("  Key: %s\n", key)
	fmt.Println("  Guardado en .env (INDEXNOW_KEY)")
	fmt.Println("  El servidor web nativo de Joss responderá de forma interna y automática en:")
	fmt.Printf("  GET /%s.txt\n", key)
	fmt.Println("  (No requiere crear archivos físicos ni rutas manuales en routes.joss)")
}
