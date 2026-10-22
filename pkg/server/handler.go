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
	return `<script>(function(){if(window.__hr_init)return;window.__hr_init=1;function mD(t,s){if(t.nodeType!==s.nodeType||t.nodeName!==s.nodeName){if(t.parentNode)t.parentNode.replaceChild(s.cloneNode(true),t);return}if(t.nodeType===3||t.nodeType===8){if(t.nodeValue!==s.nodeValue)t.nodeValue=s.nodeValue;return}if(t.nodeType===1){var a=t.attributes,b=s.attributes;for(var i=a.length-1;i>=0;i--){if(!s.hasAttribute(a[i].name))t.removeAttribute(a[i].name)}for(var j=0;j<b.length;j++){if(t.getAttribute(b[j].name)!==b[j].value)t.setAttribute(b[j].name,b[j].value)}if((t.nodeName==='INPUT'||t.nodeName==='TEXTAREA'||t.nodeName==='SELECT')&&t!==document.activeElement){if(s.value!==undefined&&t.value!==s.value)t.value=s.value}var tc=Array.from(t.childNodes),sc=Array.from(s.childNodes),ml=Math.max(tc.length,sc.length);for(var k=0;k<ml;k++){if(!tc[k]&&sc[k])t.appendChild(sc[k].cloneNode(true));else if(tc[k]&&!sc[k])t.removeChild(tc[k]);else if(tc[k]&&sc[k])mD(tc[k],sc[k])}}}function rC(){var l=document.getElementsByTagName("link");for(var i=0;i<l.length;i++){if(l[i].rel==="stylesheet"&&l[i].href){var u=new URL(l[i].href,location.href);u.searchParams.set("_h",Date.now());l[i].href=u.toString()}}}function sR(){var sx=window.scrollX,sy=window.scrollY;fetch(location.href,{headers:{'X-Hot-Reload':'1'},cache:'no-cache'}).then(function(r){return r.text()}).then(function(h){var d=new DOMParser().parseFromString(h,'text/html');if(d&&d.body){mD(document.body,d.body);if(d.title&&document.title!==d.title)document.title=d.title;window.scrollTo(sx,sy)}else{location.reload()}}).catch(function(){location.reload()})}function c(){var p=location.protocol==="https:"?"wss://":"ws://",ws=new WebSocket(p+location.host+"/__hot_reload");ws.onmessage=function(e){try{var m=JSON.parse(e.data);if(m.type==="css")rC();else sR()}catch(err){sR()}};ws.onclose=function(){setTimeout(c,2000)}}c();})();</script>`
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
