package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/i18n"
)

var (
	currentRuntime   *core.Runtime
	mutex            sync.RWMutex
	GlobalFileSystem http.FileSystem // Exposed for hotreload.go
)

func init() {
	core.ServerStartCallback = Start
}

// Start initializes and starts the Joss HTTP server with Hot Reload
// fs can be nil, in which case it defaults to http.Dir("public")
func Start(fileSystem http.FileSystem) {
	GlobalFileSystem = fileSystem
	core.SetFileSystem(fileSystem)

	// Initial Load
	reloadApp("")

	// Start File Watcher (disabled in production or if explicitly set)
	disableWatch := false
	if currentRuntime != nil {
		if env := currentRuntime.Env["APP_ENV"]; env == "production" || env == "prod" {
			disableWatch = true
		}
		if env := currentRuntime.Env["DISABLE_WATCHDOG"]; env == "true" {
			disableWatch = true
		}
	}
	if os.Getenv("DISABLE_WATCHDOG") == "true" || os.Getenv("APP_ENV") == "production" {
		disableWatch = true
	}

	if !disableWatch {
		go watchChanges()
	} else {
		fmt.Println("[Server] File watcher (Hot Reload) deshabilitado.")
	}

	port := "8000"
	if val, ok := currentRuntime.Env["PORT"]; ok && val != "" {
		port = val
	} else if val, ok := currentRuntime.Env["JOSS_PORT"]; ok && val != "" {
		port = val
	} else if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	// Static Files
	if GlobalFileSystem != nil {
		// VFS Mode (Root FS)
		fsHandler := http.FileServer(GlobalFileSystem)

		// /public/ -> maps to public/ in VFS (no strip prefix)
		http.Handle("/public/", fsHandler)

		// /assets/ -> maps to public/ in VFS
		http.Handle("/assets/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = strings.Replace(r.URL.Path, "/assets/", "/public/", 1)
			fsHandler.ServeHTTP(w, r)
		}))
	} else {
		// Disk Mode (Public FS)
		if fileSystem == nil {
			fileSystem = http.Dir("public")
		}
		fsHandler := http.FileServer(fileSystem)
		http.Handle("/public/", http.StripPrefix("/public/", fsHandler))
		http.Handle("/assets/", http.StripPrefix("/assets/", fsHandler))
	}

	// WebSocket Endpoint for Hot Reload
	http.HandleFunc("/__hot_reload", hotReloadHandler)

	// Vendor Assets (Node Modules) - Must be more specific than /assets/
	http.HandleFunc("/assets/vendor/", func(w http.ResponseWriter, r *http.Request) {
		// Only allowed in Disk Mode (Development)
		if GlobalFileSystem != nil {
			http.NotFound(w, r)
			return
		}

		relPath := strings.TrimPrefix(r.URL.Path, "/assets/vendor/")
		// Security Check: simple traversal prevention
		if strings.Contains(relPath, "..") {
			http.Error(w, "Invalid path", http.StatusForbidden)
			return
		}

		fullPath := filepath.Join("node_modules", relPath)
		if _, err := os.Stat(fullPath); err == nil {
			// serve
			http.ServeFile(w, r, fullPath)
			return
		}

		fmt.Printf("[Server] Vendor Asset 404: %s\n", fullPath)
		http.NotFound(w, r)
	})

	// WebSocket Endpoint
	InitWebSocket()
	core.BroadcastFunc = Broadcast
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ServeWs(GlobalHub, w, r)
	})

	// Main Handler
	http.HandleFunc("/", MainHandler)

	// Start Server with Timeouts
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      nil, // Use DefaultServeMux
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	certFile := strings.TrimSpace(currentRuntime.Env["TLS_CERT_FILE"])
	keyFile := strings.TrimSpace(currentRuntime.Env["TLS_KEY_FILE"])
	if (certFile == "") != (keyFile == "") {
		fmt.Println("Error iniciando servidor: TLS_CERT_FILE y TLS_KEY_FILE deben configurarse juntos")
		return
	}

	fmt.Println(i18n.Tr("serverStarting", map[string]interface{}{
		"host": "localhost",
		"port": port,
	}))
	var err error
	if certFile != "" {
		err = srv.ListenAndServeTLS(certFile, keyFile)
	} else {
		err = srv.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		fmt.Printf("FATAL: Error iniciando servidor: %v\n", err)
		os.Exit(1)
	}
	if err == http.ErrServerClosed {
		fmt.Println(i18n.Tr("serverStopped"))
	}
}
