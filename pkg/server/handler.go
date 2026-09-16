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
	requestID := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	stopWatchdog := startRequestWatchdog(r, requestID)
	defer stopWatchdog()

	// 1. Runtime Fork (Isolation)
	mutex.RLock()
	if currentRuntime == nil {
		mutex.RUnlock()
		http.Error(w, "Server starting up...", http.StatusServiceUnavailable)
		return
	}
	rt := currentRuntime.Fork()
	mutex.RUnlock()
	webSocketUpgraded := false

	defer func() {
		if recovered := recover(); recovered != nil {
			errMsg := core.FormatPanicAsError(recovered)
			fmt.Printf("[SERVER ERROR] %s\n", errMsg)
			if !webSocketUpgraded {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "<h1>500 Internal Server Error</h1><pre>%s</pre>", errMsg)
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
		http.Error(w, "Unable to persist session storage", http.StatusInternalServerError)
		fmt.Printf("[Session] %v\n", err)
		return
	}

	if err == nil {
		if writeExecutionResult(w, r, rt, result, responseSession{id: sessionID, driver: driver, data: sessData}) {
			return
		}
	} else {
		fmt.Printf("[DEBUG] Dispatch error: %v\n", err)
	}

	// Try serving static files from public/ directory before 404 (e.g. /ads.txt, /robots.txt, etc.)
	if servePublicFile(w, r) {
		return
	}

	// Fallback / 404
	if r.URL.Path == "/" {
		w.Header().Set(headerContentType, "text/html")
		fmt.Fprintf(w, "<h1>JosSecurity Server Running</h1>")
		fmt.Fprintf(w, "<p>Environment: %s</p>", rt.Env["APP_ENV"])
		fmt.Fprintf(w, `<link rel="stylesheet" href="/public/css/app.css">`)
		// Hot Reload Script (WebSocket)
		fmt.Fprint(w, getHotReloadScript())
		return
	}

	http.NotFound(w, r)
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
	return `<script>
	(function() {
		if (window.__joss_hot_reload_initialized) return;
		window.__joss_hot_reload_initialized = true;

		function showToast(message) {
			var toast = document.getElementById('__joss_hot_reload_toast');
			if (!toast) {
				toast = document.createElement('div');
				toast.id = '__joss_hot_reload_toast';
				toast.style.cssText = 'position:fixed;bottom:20px;right:20px;z-index:999999;background:rgba(18,18,24,0.92);color:#fff;padding:8px 14px;border-radius:20px;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;font-size:13px;font-weight:500;box-shadow:0 4px 14px rgba(0,0,0,0.25);border:1px solid rgba(255,255,255,0.1);display:flex;align-items:center;gap:6px;transition:opacity 0.3s,transform 0.3s;opacity:0;transform:translateY(10px);pointer-events:none;';
				if (document.body) {
					document.body.appendChild(toast);
				}
			}
			if (toast) {
				toast.innerHTML = '<span style="color:#38bdf8;">⚡</span> ' + message;
				toast.style.opacity = '1';
				toast.style.transform = 'translateY(0)';
				clearTimeout(toast.__timer);
				toast.__timer = setTimeout(function() {
					toast.style.opacity = '0';
					toast.style.transform = 'translateY(10px)';
				}, 1800);
			}
		}

		function reloadCSS() {
			var links = document.getElementsByTagName("link");
			var updated = false;
			for (var i = 0; i < links.length; i++) {
				var link = links[i];
				if (link.rel === "stylesheet" && link.href) {
					var url = new URL(link.href, location.href);
					url.searchParams.set("_hr", Date.now());
					link.href = url.toString();
					updated = true;
				}
			}
			showToast("Estilos actualizados");
		}

		function morphDOM(target, source) {
			if (target.nodeType !== source.nodeType || target.nodeName !== source.nodeName) {
				if (target.parentNode) {
					target.parentNode.replaceChild(source.cloneNode(true), target);
				}
				return;
			}

			if (target.nodeType === Node.TEXT_NODE || target.nodeType === Node.COMMENT_NODE) {
				if (target.nodeValue !== source.nodeValue) {
					target.nodeValue = source.nodeValue;
				}
				return;
			}

			if (target.nodeType === Node.ELEMENT_NODE) {
				if (target.id === '__joss_hot_reload_toast') return;

				var tAttrs = target.attributes;
				var sAttrs = source.attributes;

				for (var i = tAttrs.length - 1; i >= 0; i--) {
					var attrName = tAttrs[i].name;
					if (!source.hasAttribute(attrName)) {
						target.removeAttribute(attrName);
					}
				}

				for (var i = 0; i < sAttrs.length; i++) {
					var attr = sAttrs[i];
					if (target.getAttribute(attr.name) !== attr.value) {
						target.setAttribute(attr.name, attr.value);
					}
				}

				if ((target.nodeName === 'INPUT' || target.nodeName === 'TEXTAREA' || target.nodeName === 'SELECT') && target !== document.activeElement) {
					if (source.value !== undefined && target.value !== source.value) {
						target.value = source.value;
					}
				}

				var tChildren = Array.from(target.childNodes);
				var sChildren = Array.from(source.childNodes);

				var maxLen = Math.max(tChildren.length, sChildren.length);
				for (var i = 0; i < maxLen; i++) {
					var tChild = tChildren[i];
					var sChild = sChildren[i];

					if (!tChild && sChild) {
						target.appendChild(sChild.cloneNode(true));
					} else if (tChild && !sChild) {
						target.removeChild(tChild);
					} else if (tChild && sChild) {
						morphDOM(tChild, sChild);
					}
				}
			}
		}

		function softReload() {
			var scrollX = window.scrollX;
			var scrollY = window.scrollY;

			fetch(location.href, {
				headers: { 'X-Hot-Reload': '1' },
				cache: 'no-cache'
			})
			.then(function(res) { return res.text(); })
			.then(function(html) {
				var parser = new DOMParser();
				var newDoc = parser.parseFromString(html, 'text/html');

				if (newDoc && newDoc.body) {
					morphDOM(document.body, newDoc.body);
					if (newDoc.title && document.title !== newDoc.title) {
						document.title = newDoc.title;
					}
					window.scrollTo(scrollX, scrollY);
					showToast("Hot Reload aplicado");
				} else {
					location.reload();
				}
			})
			.catch(function(err) {
				console.error("[HotReload] Error en recarga suave:", err);
				location.reload();
			});
		}

		function connect() {
			var protocol = location.protocol === "https:" ? "wss://" : "ws://";
			var conn = new WebSocket(protocol + location.host + "/__hot_reload");

			conn.onmessage = function(evt) {
				var data = evt.data;
				try {
					var msg = JSON.parse(data);
					if (msg.type === "css") {
						reloadCSS();
					} else if (msg.type === "soft_reload") {
						softReload();
					} else if (msg.type === "full_reload") {
						location.reload();
					} else {
						softReload();
					}
				} catch(e) {
					if (data === "reload") {
						softReload();
					} else {
						location.reload();
					}
				}
			};

			conn.onclose = function() {
				console.log("[HotReload] Conexión cerrada. Reconectando en 2s...");
				setTimeout(connect, 2000);
			};
		}

		connect();
	})();
</script>`
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
