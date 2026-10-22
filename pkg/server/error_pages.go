package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jossecurity/joss/pkg/core"
)

// isDeceptionEnabled returns true unless SECURITY_DECEPTION=false (enabled by default for stealth).
func isDeceptionEnabled(env map[string]string) bool {
	if env != nil {
		val := strings.ToLower(strings.TrimSpace(env["SECURITY_DECEPTION"]))
		if val == "false" || val == "0" || val == "no" || val == "off" {
			return false
		}
	}
	return true
}

// isDebugEnabled returns true unless APP_DEBUG=false or DEBUG=false.
func isDebugEnabled(env map[string]string) bool {
	if env != nil {
		if val := strings.ToLower(strings.TrimSpace(env["APP_DEBUG"])); val != "" {
			return val == "true" || val == "1" || val == "yes" || val == "on"
		}
		if val := strings.ToLower(strings.TrimSpace(env["DEBUG"])); val != "" {
			return val == "true" || val == "1" || val == "yes" || val == "on"
		}
	}
	return true
}

// renderErrorPage renders a clean, styled error page matching the Joss design language.
// If the project has a custom view in app/views/errors/{status}.joss.html, it attempts to render it.
func renderErrorPage(w http.ResponseWriter, r *http.Request, rt *core.Runtime, statusCode int, title, message string) {
	// If the client expects JSON or the route is under /api/, respond with JSON.
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") || strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(statusCode)
		fmt.Fprintf(w, `{"status":%d,"error":%q,"message":%q}`+"\n", statusCode, http.StatusText(statusCode), message)
		return
	}

	// Try custom project error view: app/views/errors/{statusCode}.joss.html
	if rt != nil {
		customViewName := fmt.Sprintf("errors.%d", statusCode)
		viewRes := rt.CallFunction("view", []interface{}{customViewName, map[string]interface{}{
			"status":  statusCode,
			"title":   title,
			"message": message,
		}})
		if viewRes != nil {
			if wr, ok := viewRes.(*core.Instance); ok && wr.Class != nil && wr.Class.Name.Value == "WebResponse" {
				if html, ok := wr.Fields["content"].(string); ok && html != "" {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(statusCode)
					_, _ = w.Write([]byte(html))
					return
				}
			}
		}
	}

	// Default embedded clean error page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	html := buildDefaultErrorHTML(statusCode, title, message, r.Host)
	_, _ = w.Write([]byte(html))
}

func buildDefaultErrorHTML(statusCode int, title, message, host string) string {
	svgIcon := getErrorSVG(statusCode)
	return fmt.Sprintf(`<!doctype html>
<html lang="es">
<head>
	<meta charset="utf-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<title>%d - %s</title>
	<style>
		body {
			background-color: #f8fafc;
			margin-top: 8%%;
			color: #334155;
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
			text-align: center;
			padding: 0 1rem;
		}
		.container {
			max-width: 600px;
			margin-right: auto;
			margin-left: auto;
		}
		.animate__fadeIn {
			animation: fadeIn 0.6s ease-in-out both;
		}
		.icon-large {
			height: 110px;
			width: 110px;
			margin-bottom: 1.5rem;
		}
		.warning { color: #f59e0b; fill: #f59e0b; }
		.danger  { color: #ef4444; fill: #ef4444; }
		.info    { color: #3b82f6; fill: #3b82f6; }
		h1 {
			font-size: 2.2rem;
			font-weight: 700;
			color: #1e293b;
			letter-spacing: -0.02em;
			margin: 0 0 15px 0;
		}
		.description-text {
			color: #64748b;
			font-size: 1.15rem;
			line-height: 1.6;
			margin-bottom: 2rem;
		}
		.badge {
			display: inline-block;
			padding: 4px 12px;
			font-size: 0.85rem;
			font-weight: 600;
			background: #e2e8f0;
			color: #475569;
			border-radius: 9999px;
			margin-bottom: 1rem;
		}
		.footer {
			margin-top: 40px;
			font-size: 0.8rem;
			color: #94a3b8;
		}
		.btn {
			display: inline-block;
			margin-top: 10px;
			padding: 10px 24px;
			background: #2563eb;
			color: #ffffff;
			text-decoration: none;
			font-weight: 500;
			border-radius: 8px;
			transition: background 0.2s ease;
		}
		.btn:hover {
			background: #1d4ed8;
		}
		@keyframes fadeIn {
			from { opacity: 0; transform: translateY(8px); }
			to { opacity: 1; transform: translateY(0); }
		}
	</style>
</head>
<body>
	<div class="container animate__fadeIn">
		<div class="badge">Error %d</div>
		<div>%s</div>
		<h1>%s</h1>
		<div class="description-text">
			<p>%s</p>
		</div>
		<a href="/" class="btn">Volver al inicio</a>
		<div class="footer">
			<strong>Host:</strong> %s
		</div>
	</div>
</body>
</html>`, statusCode, title, statusCode, svgIcon, title, message, host)
}

func getErrorSVG(status int) string {
	switch status {
	case 403, 401:
		// Shield / Lock (Yellow/Amber fa-shield-alt style)
		return `<svg class="warning icon-large" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" fill="currentColor"><path d="M466.5 83.7l-192-80a48.15 48.15 0 0 0-36.9 0l-192 80C27.7 91.1 16 108.6 16 128c0 198.5 114.5 335.7 221.5 380.3 11.8 4.9 25.1 4.9 36.9 0C360.1 472.6 496 334.3 496 128c0-19.4-11.7-36.9-29.5-44.3zM256 461.3C161.7 419.6 64 294.6 64 128l192-80 192 80c0 166.6-97.7 291.6-192 333.3z"/></svg>`
	case 404:
		// Hard hat / Under construction (Warning icon from index.html)
		return `<svg class="warning icon-large" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" fill="currentColor"><path d="M480 288c0-80.25-49.28-148.92-119.19-177.62L320 192V80a16 16 0 0 0-16-16h-96a16 16 0 0 0-16 16v112l-40.81-81.62C81.28 139.08 32 207.75 32 288v64h448zm16 96H16a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h480a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16z"></path></svg>`
	case 500, 502, 503:
		// Server / Alert (Amber warning style)
		return `<svg class="warning icon-large" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" fill="currentColor"><path d="M448 64H64A64 64 0 0 0 0 128v64a64 64 0 0 0 64 64h384a64 64 0 0 0 64-64v-64a64 64 0 0 0-64-64zm-48 88a24 24 0 1 1 24-24 24 24 0 0 1-24 24zm48 104H64a64 64 0 0 0-64 64v64a64 64 0 0 0 64 64h384a64 64 0 0 0 64-64v-64a64 64 0 0 0-64-64zm-48 88a24 24 0 1 1 24-24 24 24 0 0 1-24 24z"/></svg>`
	default:
		// General alert circle
		return `<svg class="warning icon-large" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" fill="currentColor"><path d="M256 8C119 8 8 119 8 256s111 248 248 248 248-111 248-248S393 8 256 8zm0 448c-110.5 0-200-89.5-200-200S145.5 56 256 56s200 89.5 200 200-89.5 200-200 200zm-32-316v152c0 6.6 5.4 12 12 12h40c6.6 0 12-5.4 12-12V140c0-6.6-5.4-12-12-12h-40c-6.6 0-12 5.4-12 12zm32 260c17.7 0 32-14.3 32-32s-14.3-32-32-32-32 14.3-32 32 14.3 32 32 32z"/></svg>`
	}
}
