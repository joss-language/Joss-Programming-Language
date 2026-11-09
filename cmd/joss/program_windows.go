//go:build windows

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jchv/go-webview2"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/i18n"
)

func startProgram() {
	// Bloquear el hilo principal del sistema operativo para Win32 / COM (STA)
	runtime.LockOSThread()

	os.Setenv("JOSS_GUI", "true")
	os.Setenv("JOSS_PROGRAM_START", "true")

	if _, err := os.Stat("main.joss"); os.IsNotExist(err) {
		fmt.Println(i18n.Tr("cliErrNoMain"))
		fmt.Println("  main.joss")
		fmt.Println(i18n.Tr("cliErrRequireMain"))
		os.Exit(1)
	}

	fmt.Println("⚡ [Desktop GUI] Compilando aplicación a binario nativo AOT...")
	exePath, err := buildNativeProgram("main.joss", "", "", false, false, false, true, "auto")
	if err != nil {
		fmt.Printf("❌ [Desktop GUI] Error compilando binario nativo: %v\n", err)
		os.Exit(1)
	}

	// Resolver puerto configurado desde .env / env.joss o variable de entorno
	port := "8000"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	} else {
		envRes := core.DetectAndLoadEnv(nil)
		if envPort := envRes.EnvMap["PORT"]; envPort != "" {
			port = envPort
		}
	}

	fmt.Printf("[Desktop GUI] Iniciando backend nativo (%s) en puerto %s...\n", exePath, port)
	cmd := exec.Command(exePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		fmt.Printf("❌ [Desktop GUI] Error iniciando proceso nativo: %v\n", err)
		os.Exit(1)
	}
	defer killChildProcess(cmd)

	// Esperar reactivamente a que el socket del backend nativo responda
	targetAddr := "127.0.0.1:" + port
	fmt.Printf("[Desktop GUI] Esperando enlace TCP en %s...\n", targetAddr)
	connected := false
	for i := 0; i < 40; i++ {
		conn, dialErr := net.DialTimeout("tcp", targetAddr, 100*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			connected = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !connected {
		fmt.Printf("⚠️  [Desktop GUI] Tiempo de espera agotado conectando a %s, abriendo ventana igualmente...\n", targetAddr)
	} else {
		fmt.Printf("[Desktop GUI] Backend nativo enlazado en %s. Abriendo ventana...\n", targetAddr)
	}

	launchGUIWindow(port)
}

func launchGUIWindow(port string) {
	title := "Joss Application"
	envRes := core.DetectAndLoadEnv(nil)
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
}
