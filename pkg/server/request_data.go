package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxMultipartMemory = 10 << 20
	maxJSONBodyBytes   = 10 << 20
	maxUploadFileBytes = 50 << 20
)

// decodeRequestData is the transport boundary between net/http and the map
// shape consumed by the Joss runtime. Keeping it isolated makes request
// decoding testable without constructing or dispatching a language runtime.
func decodeRequestData(request *http.Request, host, scheme string) map[string]interface{} {
	data := make(map[string]interface{})
	for key, values := range request.URL.Query() {
		if len(values) > 0 {
			data[key] = values[0]
		}
	}

	if strings.Contains(request.Header.Get("Content-Type"), "application/json") {
		decodeJSONBody(request, data)
	} else {
		decodeFormBody(request, data)
	}
	addRequestMetadata(request, data, host, scheme)
	return data
}

func decodeJSONBody(request *http.Request, data map[string]interface{}) {
	if request.Body == nil {
		return
	}
	var body map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(request.Body, maxJSONBodyBytes)).Decode(&body); err != nil {
		return
	}
	for key, value := range body {
		data[key] = value
	}
}

func sanitizeTempUploadName(filename string) string {
	base := filepath.Base(strings.ReplaceAll(filename, "\\", "/"))
	var b strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" || out == "." || out == ".." {
		return "upload.bin"
	}
	if len(out) > 64 {
		out = out[len(out)-64:]
	}
	return out
}

func decodeFormBody(request *http.Request, data map[string]interface{}) {
	_ = request.ParseMultipartForm(maxMultipartMemory)
	for key, values := range request.PostForm {
		if len(values) > 0 {
			data[key] = values[0]
		}
	}
	if request.MultipartForm == nil || request.MultipartForm.File == nil {
		return
	}

	files := make(map[string]interface{})
	for field, headers := range request.MultipartForm.File {
		if len(headers) == 0 {
			continue
		}
		header := headers[0]
		if header.Size < 0 || header.Size > maxUploadFileBytes {
			continue
		}
		file, err := header.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(file, maxUploadFileBytes))
		_ = file.Close()
		if err != nil {
			continue
		}

		path := ""
		safeSuffix := sanitizeTempUploadName(header.Filename)
		temporary, err := os.CreateTemp("", "joss_upload_*_"+safeSuffix)
		if err == nil {
			_, _ = temporary.Write(content)
			path = temporary.Name()
			_ = temporary.Close()
		}
		files[field] = map[string]interface{}{
			"name": header.Filename, "path": path, "type": header.Header.Get("Content-Type"),
			"size": int64(len(content)), "content": string(content),
		}
	}
	data["_files"] = files
}

func cleanupRequestTempFiles(reqData map[string]interface{}, r *http.Request) {
	if reqData != nil {
		if filesMap, ok := reqData["_files"].(map[string]interface{}); ok {
			for _, v := range filesMap {
				if entry, ok := v.(map[string]interface{}); ok {
					if p, ok := entry["path"].(string); ok && p != "" {
						_ = os.Remove(p)
					}
				}
			}
		}
	}
	if r != nil && r.MultipartForm != nil {
		_ = r.MultipartForm.RemoveAll()
	}
}

func addRequestMetadata(request *http.Request, data map[string]interface{}, host, scheme string) {
	headers := make(map[string]interface{}, len(request.Header))
	for key, values := range request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	data["_headers"] = headers
	if authorization := request.Header.Get("Authorization"); authorization != "" {
		data["Authorization"] = authorization
	}
	data["_method"] = request.Method
	data["_path"] = request.URL.Path
	data["_uri"] = request.URL.RequestURI()
	data["_url"] = request.URL.String()
	data["_ip"] = request.RemoteAddr
	data["_referer"] = request.Referer()
	data["_host"] = host
	data["_scheme"] = scheme

	cookies := make(map[string]interface{})
	for _, cookie := range request.Cookies() {
		cookies[cookie.Name] = cookie.Value
	}
	data["_cookies"] = cookies
}
