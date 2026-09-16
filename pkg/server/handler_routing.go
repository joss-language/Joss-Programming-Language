package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jossecurity/joss/pkg/core"
)

func handleEarlySpecialRoutes(w http.ResponseWriter, r *http.Request, rt *core.Runtime, baseUrl string) bool {
	if r.URL.Path == "/favicon.ico" {
		handleFavicon(w, r)
		return true
	}

	if r.URL.Path == "/sitemap.xml" {
		w.Header().Set(headerContentType, "application/xml; charset=utf-8")
		fmt.Fprintf(w, "%s", rt.GenerateSitemapXML(baseUrl))
		return true
	}

	if r.URL.Path == "/sitemap.xsl" {
		w.Header().Set(headerContentType, "application/xslt+xml; charset=utf-8")
		w.Header().Set(headerCacheControl, headerCacheControlPublic)
		fmt.Fprintf(w, "%s", rt.GenerateSitemapXSL())
		return true
	}

	if indexNowKey := rt.GetIndexNowKey(); indexNowKey != "" && r.URL.Path == "/"+indexNowKey+".txt" {
		w.Header().Set(headerContentType, "text/plain; charset=utf-8")
		w.Header().Set(headerCacheControl, headerCacheControlPublic)
		fmt.Fprint(w, indexNowKey)
		return true
	}

	if strings.HasPrefix(r.URL.Path, "/assets/vendor/") {
		handleVendorAssets(w, r)
		return true
	}

	return false
}

func handleFavicon(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat("public/favicon.ico"); err == nil {
		http.ServeFile(w, r, "public/favicon.ico")
		return
	}
	if _, err := os.Stat("assets/logo.png"); err == nil {
		http.ServeFile(w, r, "assets/logo.png")
		return
	}
	if _, err := os.Stat("assets/logo.ico"); err == nil {
		http.ServeFile(w, r, "assets/logo.ico")
		return
	}
	w.Header().Set(headerContentType, "image/png")
	w.Header().Set(headerCacheControl, headerCacheControlPublic)
	w.Write(DefaultLogo)
}

func handleVendorAssets(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, "/assets/vendor/")
	fullPath := "node_modules/" + relPath

	if strings.Contains(relPath, "..") {
		http.Error(w, "Invalid path", http.StatusForbidden)
		return
	}

	if _, err := os.Stat(fullPath); err == nil {
		http.ServeFile(w, r, fullPath)
		return
	}
	http.NotFound(w, r)
}

func setupRequestLocale(r *http.Request, rt *core.Runtime) {
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang != "" {
		parts := strings.Split(acceptLang, ",")
		if len(parts) > 0 {
			first := strings.TrimSpace(parts[0])
			first = strings.Split(first, ";")[0]
			rt.SetLocale(first)
			return
		}
	}
	rt.SetLocale("en")
}

func handleCORSHeaders(w http.ResponseWriter, r *http.Request, env map[string]string) bool {
	corsPolicy := env["CORS_WEB"]
	if corsPolicy == "" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}

	allowOrigin := ""
	if corsPolicy == "*" {
		allowOrigin = "*"
	} else {
		allowed := strings.Split(corsPolicy, ",")
		for _, a := range allowed {
			if strings.TrimSpace(a) == origin {
				allowOrigin = origin
				break
			}
		}
	}

	if allowOrigin == "" {
		return false
	}

	w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-CSRF-TOKEN")
	if corsPolicy != "*" {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

func startRequestWatchdog(r *http.Request, requestID string) func() {
	requestStartTime := time.Now()
	done := make(chan struct{})
	go func() {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
			strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			return
		}

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fmt.Printf("[WATCHDOG] Request %s still processing (%.0fs)...\n", requestID, time.Since(requestStartTime).Seconds())
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}

func handleWebSocketRoute(w http.ResponseWriter, r *http.Request, rt *core.Runtime, upgraded *bool) bool {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("[WS] Upgrade failed: %v\n", err)
		return true
	}
	*upgraded = true

	maxMessageBytes := int64(envPositiveInt(rt.Env, "WS_MAX_MESSAGE_BYTES", 8*1024*1024))
	idleTimeout := time.Duration(envPositiveInt(rt.Env, "WS_IDLE_TIMEOUT_SECONDS", 120)) * time.Second
	pingInterval := time.Duration(envPositiveInt(rt.Env, "WS_PING_INTERVAL_SECONDS", 30)) * time.Second
	if pingInterval >= idleTimeout {
		pingInterval = idleTimeout / 2
	}
	if pingInterval < time.Second {
		pingInterval = time.Second
	}

	conn.SetReadLimit(maxMessageBytes)
	refreshReadDeadline := func() error {
		return conn.SetReadDeadline(time.Now().Add(idleTimeout))
	}
	_ = refreshReadDeadline()
	conn.SetPongHandler(func(string) error {
		return refreshReadDeadline()
	})

	var writeMu sync.Mutex
	pingDone := make(chan struct{})
	defer close(pingDone)
	defer conn.Close()
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				writeMu.Lock()
				err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second))
				writeMu.Unlock()
				if err != nil {
					_ = conn.Close()
					return
				}
			case <-pingDone:
				return
			}
		}
	}()

	reader := func() (int, []byte, error) {
		return conn.ReadMessage()
	}

	sender := func(v interface{}) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		if str, ok := v.(string); ok {
			return conn.WriteMessage(1, []byte(str))
		}
		return conn.WriteJSON(v)
	}

	rt.DispatchWebSocket(r.URL.Path, conn, reader, sender, conn.Close)
	return true
}

func setupSessionAndSecurity(w http.ResponseWriter, r *http.Request, rt *core.Runtime) (string, string, map[string]interface{}, bool) {
	sessionID := ""
	cookie, err := r.Cookie("joss_session")
	if err != nil {
		sessionID = generateSessionID()
		http.SetCookie(w, &http.Cookie{Name: "joss_session", Value: sessionID, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode})
	} else {
		sessionID = cookie.Value
	}

	sessData, driver, sessionErr := loadSession(rt.Env, sessionID)
	if sessionErr != nil {
		status := http.StatusInternalServerError
		if driver == "redis" {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, "Unable to load session storage", status)
		fmt.Printf("[Session] %v\n", sessionErr)
		return "", "", nil, false
	}

	applyJWTPersistence(r, rt, sessData)
	setSecurityHeaders(w, r)

	if !ensureCSRFToken(w, rt, driver, sessionID, sessData) {
		return "", "", nil, false
	}

	return sessionID, driver, sessData, true
}

func applyJWTPersistence(r *http.Request, rt *core.Runtime, sessData map[string]interface{}) {
	jwtCookie, err := r.Cookie("joss_token")
	if err != nil || jwtCookie.Value == "" {
		return
	}
	jwtClaims, valid := rt.ValidateJWT(jwtCookie.Value)
	if !valid || jwtClaims == nil {
		return
	}
	if _, ok := sessData["user_id"]; ok {
		return
	}

	if uid, ok := jwtClaims["user_id"].(float64); ok {
		sessData["user_id"] = int(uid)
	}
	if email, ok := jwtClaims["email"].(string); ok {
		sessData["user_email"] = email
	}
	if name, ok := jwtClaims["name"].(string); ok {
		sessData["user_name"] = name
	}
	if role, ok := jwtClaims["role"].(string); ok {
		sessData["user_role"] = role
	}
	sessData["user_token"] = jwtCookie.Value
	fmt.Printf("[HANDLER] Session restored from JWT for user: %v\n", sessData["user_email"])
}

func setSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	if r.TLS != nil {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
}

func ensureCSRFToken(w http.ResponseWriter, rt *core.Runtime, driver, sessionID string, sessData map[string]interface{}) bool {
	if val, ok := sessData["csrf_token"]; ok {
		csrfToken, valid := val.(string)
		if !valid || csrfToken == "" {
			http.Error(w, "Unable to load session storage", http.StatusInternalServerError)
			fmt.Printf("[Session] csrf_token has invalid type %T\n", val)
			return false
		}
		return true
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "Unable to initialize session security", http.StatusInternalServerError)
		return false
	}
	csrfToken := hex.EncodeToString(b)
	sessData["csrf_token"] = csrfToken
	if err := saveSession(rt.Env, driver, sessionID, sessData); err != nil {
		http.Error(w, "Unable to persist session storage", http.StatusInternalServerError)
		fmt.Printf("[Session] %v\n", err)
		return false
	}
	return true
}

func validateCSRFToken(w http.ResponseWriter, r *http.Request, reqData, sessData map[string]interface{}, sessionID string) bool {
	if r.Method != "POST" && r.Method != "PUT" && r.Method != "DELETE" && r.Method != "PATCH" {
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		return true
	}

	csrfToken, _ := sessData["csrf_token"].(string)
	reqToken := ""
	if val, ok := reqData["_token"]; ok {
		reqToken = fmt.Sprintf("%v", val)
	} else {
		reqToken = r.Header.Get("X-CSRF-TOKEN")
	}

	fmt.Printf("[CSRF DEBUG] Session: %s | Stored: %s | Received: %s\n", sessionID, csrfToken, reqToken)

	if reqToken == "" || reqToken != csrfToken {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "<h1>419 Page Expired</h1><p>CSRF token mismatch.</p>")
		fmt.Fprintf(w, "<!-- Debug: Stored='%s' Received='%s' -->", csrfToken, reqToken)
		return false
	}
	return true
}
