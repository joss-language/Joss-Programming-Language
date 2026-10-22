package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/jossecurity/joss/pkg/core"

	_ "embed"
)

//go:embed default_logo.png
var DefaultLogo []byte

const (
	headerContentType        = "Content-Type"
	headerCacheControl       = "Cache-Control"
	headerCacheControlPublic = "public, max-age=86400"
)

var (
	sessionStore = make(map[string]map[string]interface{})
	sessionMu    sync.Mutex
)

func MainHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[Server REQ] %s %s (desde %s)\n", r.Method, r.URL.Path, r.RemoteAddr)
	requestID := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	stopWatchdog := startRequestWatchdog(r, requestID)
	defer stopWatchdog()

	// 1. Runtime Fork (Isolation)
	mutex.RLock()
	if currentRuntime == nil {
		mutex.RUnlock()
		fmt.Printf("[Server WARN] Servidor iniciando... Runtime no listo para %s\n", r.URL.Path)
		http.Error(w, "Server starting up...", http.StatusServiceUnavailable)
		return
	}
	rt := currentRuntime.Fork()
	mutex.RUnlock()
	webSocketUpgraded := false

	defer func() {
		if recovered := recover(); recovered != nil {
			errMsg := core.FormatPanicAsError(recovered)
			fmt.Printf("[SERVER PANIC] %s\n", errMsg)
			if !webSocketUpgraded {
				msg := "Ocurrió un error inesperado al procesar la solicitud."
				if isDebugEnabled(rt.Env) {
					msg = errMsg
				}
				renderErrorPage(w, r, rt, http.StatusInternalServerError, "Error interno del servidor", msg)
			}
		}
		rt.Free()
	}()

	setupRequestLocale(r, rt)

	if !enforceRateLimit(w, r, rt.Env) || handleCORSHeaders(w, r, rt.Env) {
		return
	}

	// Detect Scheme and Host for Dynamic URLs
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	baseUrl := scheme + "://" + host

	if handleEarlySpecialRoutes(w, r, rt, baseUrl) {
		return
	}

	if handleWebSocketRoute(w, r, rt, &webSocketUpgraded) {
		return
	}

	// Translate the HTTP request into the stable map exposed to Joss code.
	reqData := decodeRequestData(r, host, scheme)

	sessionID, driver, sessData, ok := setupSessionAndSecurity(w, r, rt)
	if !ok {
		return
	}

	if !validateCSRFToken(w, r, reqData, sessData, sessionID) {
		return
	}

	// 7. Dispatch
	result, err := rt.Dispatch(r.Method, r.URL.Path, reqData, sessData)

	// 8. Save Session
	if err := saveSession(rt.Env, driver, sessionID, sessData); err != nil {
		fmt.Printf("[Session ERROR] %v\n", err)
		renderErrorPage(w, r, rt, http.StatusInternalServerError, "Error interno del servidor", "No se pudo persistir el almacenamiento de sesión.")
		return
	}

	if err == nil {
		if writeExecutionResult(w, r, rt, result, responseSession{id: sessionID, driver: driver, data: sessData}) {
			fmt.Printf("[Server RESP] %s %s -> 200 OK\n", r.Method, r.URL.Path)
			return
		}
		fmt.Printf("[Server WARN] writeExecutionResult no pudo representar el resultado para %s %s (tipo: %T)\n", r.Method, r.URL.Path, result)
	} else {
		fmt.Printf("[Server ERROR] Dispatch falló para %s %s: %v\n", r.Method, r.URL.Path, err)
		if !strings.Contains(err.Error(), "route not found") {
			// Es un error interno de ejecución 500
			msg := "Ocurrió un error inesperado al procesar la solicitud."
			if isDebugEnabled(rt.Env) {
				msg = fmt.Sprintf("Error de ejecución: %v", err)
			}
			renderErrorPage(w, r, rt, http.StatusInternalServerError, "Error interno del servidor", msg)
			return
		}
	}

	// Try serving static files from public/ directory before 404 (e.g. /ads.txt, /robots.txt, etc.)
	if servePublicFile(w, r) {
		fmt.Printf("[Server RESP] %s %s -> Archivo público servido\n", r.Method, r.URL.Path)
		return
	}

	// Fallback / 404
	if r.URL.Path == "/" {
		w.Header().Set(headerContentType, "text/html")
		fmt.Fprintf(w, "<h1>JosSecurity Server Running</h1>")
		fmt.Fprintf(w, "<p>Environment: %s</p>", rt.Env["APP_ENV"])
		fmt.Fprintf(w, `<link rel="stylesheet" href="/public/css/app.css">`)
		fmt.Printf("[Server RESP] %s %s -> Fallback HTML por defecto\n", r.Method, r.URL.Path)
		return
	}

	fmt.Printf("[Server RESP] %s %s -> 404 Not Found\n", r.Method, r.URL.Path)
	renderErrorPage(w, r, rt, http.StatusNotFound, "Página no encontrada", "El recurso o página que intentas acceder no existe en este servidor.")
}

func servePublicFile(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != "GET" && r.Method != "HEAD" {
		return false
	}
	cleanPath := strings.TrimPrefix(filepath.Clean(r.URL.Path), string(filepath.Separator))
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	cleanPath = strings.TrimPrefix(cleanPath, "\\")
	if cleanPath == "" || cleanPath == "." || strings.Contains(cleanPath, "..") {
		return false
	}

	// 1. VFS Mode
	if GlobalFileSystem != nil {
		vfsPath := "/public/" + cleanPath
		f, err := GlobalFileSystem.Open(vfsPath)
		if err != nil {
			vfsPath = cleanPath
			f, err = GlobalFileSystem.Open(vfsPath)
		}
		if err == nil {
			defer f.Close()
			stat, err := f.Stat()
			if err == nil && !stat.IsDir() {
				http.ServeContent(w, r, stat.Name(), stat.ModTime(), f)
				return true
			}
		}
		return false
	}

	// 2. Disk Mode
	diskPath := filepath.Join("public", cleanPath)
	stat, err := os.Stat(diskPath)
	if err == nil && !stat.IsDir() {
		http.ServeFile(w, r, diskPath)
		return true
	}

	return false
}

func getHotReloadScript() string {
	return ""
}

func envPositiveInt(env map[string]string, key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(env[key]))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
