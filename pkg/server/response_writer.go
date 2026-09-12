package server

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jossecurity/joss/pkg/core"
)

type responseSession struct {
	id     string
	driver string
	data   map[string]interface{}
}

// writeExecutionResult is the HTTP adaptation boundary for values returned by
// Joss. It returns false when the value has no published HTTP representation.
func writeExecutionResult(w http.ResponseWriter, request *http.Request, runtime *core.Runtime, result interface{}, session responseSession) bool {
	if fields, ok := result.(map[string]interface{}); ok {
		return writeResponseFields(w, request, runtime, fields, responseSession{}, false)
	}
	if instance, ok := result.(*core.Instance); ok {
		if instance == nil {
			return false
		}
		writeResponseCookies(w, request, instance.Fields)
		writeResponseHeaders(w, instance.Fields)
		return writeResponseFields(w, request, runtime, instance.Fields, session, true)
	}
	if text, ok := result.(string); ok {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		_, _ = w.Write([]byte(text))
		if strings.Contains(w.Header().Get("Content-Type"), "text/html") {
			_, _ = fmt.Fprint(w, getHotReloadScript())
		}
		return true
	}
	return false
}

func writeResponseFields(w http.ResponseWriter, request *http.Request, runtime *core.Runtime, fields map[string]interface{}, session responseSession, instance bool) bool {
	kind, _ := fields["_type"].(string)
	switch kind {
	case "JSON":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus(fields["status_code"], http.StatusOK))
		_ = json.NewEncoder(w).Encode(fields["data"])
		return true
	case "RAW":
		contentType, _ := fields["content_type"].(string)
		if contentType == "" {
			contentType = "text/plain"
		}
		w.Header().Set("Content-Type", contentType)
		writeResponseHeaders(w, fields)
		w.WriteHeader(httpStatus(fields["status_code"], http.StatusOK))
		switch value := fields["data"].(type) {
		case string:
			_, _ = w.Write([]byte(value))
		case []byte:
			_, _ = w.Write(value)
		default:
			_, _ = fmt.Fprintf(w, "%v", value)
		}
		return true
	case "FILE":
		if !instance {
			return false
		}
		filePath, _ := fields["data"].(string)
		if filePath == "" {
			return false
		}
		absolute, err := filepath.Abs(filePath)
		if err != nil {
			absolute = filePath
		}
		content, err := os.ReadFile(absolute)
		if err != nil {
			http.Error(w, "File read error", http.StatusInternalServerError)
			return true
		}
		downloadName, _ := fields["download_name"].(string)
		if downloadName == "" {
			downloadName = filepath.Base(absolute)
		}
		contentType, _ := fields["content_type"].(string)
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(downloadName))
			if contentType == "" {
				contentType = http.DetectContentType(content)
			}
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadName))
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprint(len(content)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
		return true
	case "STREAM":
		if !instance {
			return false
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		stream := core.NewStreamInstance(runtime, w)
		if stream != nil {
			if callback, ok := fields["callback"]; ok {
				runtime.CallFunction(callback, []interface{}{stream})
			}
		}
		return true
	case "REDIRECT":
		if instance {
			persistRedirectFlash(runtime, session, fields)
		}
		url := fields["url"].(string)
		status := http.StatusFound
		if instance {
			status = httpStatus(fields["status_code"], http.StatusFound)
		}
		http.Redirect(w, request, url, status)
		return true
	}
	return false
}

func writeResponseCookies(w http.ResponseWriter, request *http.Request, fields map[string]interface{}) {
	cookies, _ := fields["cookies"].(map[string]interface{})
	for name, value := range cookies {
		text := fmt.Sprint(value)
		maxAge := 86400 * 30
		if text == "" {
			maxAge = -1
		}
		http.SetCookie(w, &http.Cookie{Name: name, Value: text, Path: "/", HttpOnly: true, Secure: request.TLS != nil, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
	}
}

func writeResponseHeaders(w http.ResponseWriter, fields map[string]interface{}) {
	headers, _ := fields["headers"].(map[string]interface{})
	for name, value := range headers {
		w.Header().Set(name, fmt.Sprint(value))
	}
}

func httpStatus(value interface{}, fallback int) int {
	switch code := value.(type) {
	case int:
		return code
	case int64:
		return int(code)
	case float64:
		return int(code)
	default:
		return fallback
	}
}

func persistRedirectFlash(runtime *core.Runtime, session responseSession, fields map[string]interface{}) {
	flash, ok := fields["flash"].(map[string]interface{})
	if !ok || session.id == "" {
		return
	}
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if session.driver == "redis" {
		for key, value := range flash {
			session.data[key] = value
		}
		data, _ := json.Marshal(session.data)
		core.GlobalRedis.Set(core.Ctx, "session:"+session.id, data, 24*time.Hour)
		return
	}
	if sessionStore[session.id] == nil {
		sessionStore[session.id] = make(map[string]interface{})
	}
	for key, value := range flash {
		sessionStore[session.id][key] = value
	}
	if session.driver == "file" {
		if err := persistFileSessions(runtime.Env); err != nil {
			fmt.Printf("[Session] Error persistiendo flash: %v\n", err)
		}
	}
}
