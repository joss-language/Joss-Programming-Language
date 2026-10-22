package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jossecurity/joss/pkg/core"
)

// isDeceptionEnabled returns true if SECURITY_DECEPTION=true in env or environment variables.
func isDeceptionEnabled(env map[string]string) bool {
	if env != nil {
		val := strings.ToLower(strings.TrimSpace(env["SECURITY_DECEPTION"]))
		if val == "true" || val == "1" || val == "yes" || val == "on" {
			return true
		}
	}
	return false
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
			<strong>Host:</strong> %s &bull; Joss Web Engine
		</div>
	</div>
</body>
</html>`, statusCode, title, statusCode, svgIcon, title, message, host)
}

func getErrorSVG(status int) string {
	switch status {
	case 403, 401:
		// Shield / Lock
		return `<svg class="danger icon-large" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>`
	case 404:
		// Compass / Search
		return `<svg class="warning icon-large" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polygon points="16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76"/></svg>`
	case 500, 502, 503:
		// Server / Alert
		return `<svg class="danger icon-large" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>`
	default:
		// General alert
		return `<svg class="warning icon-large" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>`
	}
}
