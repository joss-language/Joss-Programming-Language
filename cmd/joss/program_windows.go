//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jchv/go-webview2"
	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/server"
)

func startProgram() {
	// Bloquear el hilo principal del sistema operativo para Win32 / COM (STA)
	runtime.LockOSThread()

	os.Setenv("JOSS_GUI", "true")
	os.Setenv("JOSS_PROGRAM_START", "true")

	fmt.Println("[Desktop GUI] Inicializando aplicación en modo ventana nativa...")

	serverReady := make(chan string, 1)
	server.ServerReadyCallback = func(port string) {
		select {
		case serverReady <- port:
		default:
		}
	}

	go launchProgramRuntime()

	fmt.Println("[Desktop GUI] Esperando a que el backend de la aplicación esté listo...")
	// Esperar reactivamente a que la aplicación y el servidor terminen de cargar completamente
	port := <-serverReady
	fmt.Printf("[Desktop GUI] Backend listo en puerto %s. Creando ventana nativa...\n", port)

	launchGUIWindow(port)
}

func launchProgramRuntime() {
	if _, err := os.Stat("main.joss"); err != nil && server.GlobalFileSystem == nil {
		fmt.Println(i18n.Tr("programNoMainStarting"))
		fmt.Println("  main.joss")
		server.Start(nil)
		return
	}

	fmt.Println(i18n.Tr("programRunningMain"))
	fmt.Println("  main.joss")

	units, parseDiagnostics := semanticanalyzer.LoadProject("main.joss", "app")
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

	r := core.NewRuntime()
	r.CurrentFile = "main.joss"
	r.LoadEnv(server.GlobalFileSystem)

	defer func() {
		if rec := recover(); rec != nil {
			fmt.Printf("\n[Error de Ejecución JOSS]\n%s\n", core.FormatPanicAsError(rec))
			os.Exit(1)
		}
	}()

	r.ExecutePrepared(report.Prepared)
}

func readMainJossData() ([]byte, error) {
	if server.GlobalFileSystem != nil {
		content, errOpen := server.GlobalFileSystem.Open("main.joss")
		if errOpen == nil {
			defer content.Close()
			stat, _ := content.Stat()
			data := make([]byte, stat.Size())
			_, errRead := content.Read(data)
			return data, errRead
		}
		return nil, errOpen
	}
	return os.ReadFile("main.joss")
}

func launchGUIWindow(port string) {
	title := "Joss Application"
	envRes := core.DetectAndLoadEnv(server.GlobalFileSystem)
	if appName := envRes.EnvMap["APP_NAME"]; appName != "" {
		title = appName
	}

	dataDir := filepath.Join(os.TempDir(), "joss_gui_"+port)
	_ = os.MkdirAll(dataDir, 0755)

	targetURL := "http://127.0.0.1:" + port
	fmt.Printf("[Desktop GUI] Configurando ventana WebView2 (Título: '%s', Destino: %s, DataPath: %s)...\n", title, targetURL, dataDir)

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     true,
		DataPath:  dataDir,
		AutoFocus: true,
	})
	if w == nil {
		log.Println(i18n.Tr("programWebViewError"))
		return
	}
	defer w.Destroy()

	// Enlazar bridge de logging para recibir eventos de JavaScript de la interfaz
	_ = w.Bind("__joss_bridge_log__", func(level string, message string) error {
		fmt.Printf("[UI Console %s] %s\n", level, message)
		return nil
	})

	w.Init(`
		(function() {
			try {
				window.__joss_bridge_log__("INFO", "WebView2 inicializado");
				window.addEventListener("DOMContentLoaded", function() {
					window.__joss_bridge_log__("DOM", "DOMContentLoaded en " + window.location.href);
				});
				window.addEventListener("load", function() {
					window.__joss_bridge_log__("LOAD", "Página completamente cargada: " + window.location.href);
				});
				window.addEventListener("error", function(e) {
					window.__joss_bridge_log__("JS_ERROR", (e.message || "Error") + " en " + (e.filename || "") + ":" + (e.lineno || 0));
				});
				var origError = console.error;
				console.error = function() {
					var msg = Array.prototype.slice.call(arguments).join(" ");
					try { window.__joss_bridge_log__("JS_CONSOLE_ERROR", msg); } catch(_) {}
					if (origError) origError.apply(console, arguments);
				};
			} catch(e) {}
		})();
	`)

	w.SetTitle(title)
	w.SetSize(1120, 760, webview2.HintNone)

	fmt.Printf("[Desktop GUI] Navegando a %s...\n", targetURL)
	w.Navigate(targetURL)

	fmt.Println("[Desktop GUI] Bucle de mensajes activo (w.Run()). Presione Alt+F4 o cierre la ventana para salir.")
	w.Run()
	fmt.Println("[Desktop GUI] Ventana cerrada por el usuario. Proceso terminado.")
	os.Exit(0)
}
