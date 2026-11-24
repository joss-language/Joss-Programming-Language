package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type coreRateBucket struct {
	windowStart time.Time
	count       int64
}

var (
	coreRateMu      sync.Mutex
	coreRateBuckets = make(map[string]coreRateBucket)
	coreSessMu      sync.Mutex
	coreSessStore   = make(map[string]map[string]interface{})
)

// appendNativeHTTPHeader inserts a transport header without interpreting the
// response body as UTF-8; RAW responses may contain arbitrary binary bytes.
func appendNativeHTTPHeader(response, header string) string {
	separator := strings.Index(response, "\r\n\r\n")
	if separator < 0 || header == "" {
		return response
	}
	return response[:separator] + "\r\n" + header + "\r\n\r\n" + response[separator+4:]
}

func (r *Runtime) callBuiltinIO(name string, args []interface{}) (interface{}, bool) {
	switch name {
	case "env", "config":
		if len(args) > 0 {
			if key, ok := args[0].(string); ok {
				if val, exists := r.Env[key]; exists && val != "" {
					return val, true
				}
				if len(args) > 1 {
					return args[1], true
				}
				if val, exists := r.Env[key]; exists {
					return val, true
				}
				return "", true
			}
		}
		return "", true

	case "view":
		return r.executeViewMethod(nil, "render", args), true

	case "__joss_http_dispatch":
		if len(args) >= 3 {
			method, methodOK := args[0].(string)
			path, pathOK := args[1].(string)
			if methodOK && pathOK {
				if method == "__websocket__" {
					if len(args) >= 3 {
						if socket, ok := args[2].(*Instance); ok {
							return r.startNativeWebSocket(path, socket), true
						}
					}
					return false, true
				}
				requestData, requestOK := args[2].(map[string]interface{})
				if !requestOK {
					if rawRequest, ok := args[2].(string); ok {
						requestData = r.parseHTTPWithStdlib(rawRequest)
						requestOK = requestData != nil
					}
				}
				if !requestOK {
					return "", true
				}
				if requestPath, ok := requestData["_path"].(string); ok && requestPath != "" {
					path = requestPath
				}
				sessData := map[string]interface{}{}
				if len(args) >= 4 {
					if suppliedSession, ok := args[3].(map[string]interface{}); ok && suppliedSession != nil {
						sessData = suppliedSession
					}
				}
				resp, err := r.Dispatch(method, path, requestData, sessData)
				if err == nil && resp != nil {
					formatted := formatNativeHTTPResponse(resp)
					if len(args) >= 5 {
						if sessionCookie, ok := args[4].(string); ok && sessionCookie != "" {
							formatted = appendNativeHTTPHeader(formatted, sessionCookie)
						}
					}
					return formatted, true
				}
			}
		}
		return "", true

	case "__joss_timezone_data":
		content, err := os.ReadFile("/etc/localtime")
		if err != nil {
			return []interface{}{}, true
		}
		bytes := make([]interface{}, len(content))
		for index, value := range content {
			bytes[index] = int64(value)
		}
		return bytes, true

	case "__joss_get_embedded_asset":
		if GlobalFileSystem != nil && len(args) == 1 {
			urlPath, _ := args[0].(string)
			name := strings.TrimPrefix(urlPath, "/")
			if strings.HasPrefix(name, "public/") && fs.ValidPath(name) && !strings.ContainsAny(name, "\\:") {
				f, err := GlobalFileSystem.Open(name)
				if err == nil {
					defer f.Close()
					if data, err := io.ReadAll(f); err == nil {
						return string(data), true
					}
				}
			}
		}
		return "", true

	case "__joss_socket_peer_ip":
		if len(args) >= 1 {
			if inst, ok := args[0].(*Instance); ok && inst != nil {
				if conn, okC := inst.Fields["_conn"].(net.Conn); okC && conn != nil && conn.RemoteAddr() != nil {
					host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
					if err == nil && host != "" {
						return host, true
					}
					return conn.RemoteAddr().String(), true
				}
			}
		}
		return "127.0.0.1", true

	case "__joss_http_read_request":
		if len(args) >= 1 {
			if inst, ok := args[0].(*Instance); ok && inst != nil {
				var chunks bytes.Buffer
				for i := 0; i < 64; i++ {
					part, _ := r.executeSocketMethod(inst, "receive", []interface{}{int64(65536)}).(string)
					if part == "" {
						break
					}
					chunks.WriteString(part)
					rawStr := chunks.String()
					if idx := strings.Index(rawStr, "\r\n\r\n"); idx >= 0 {
						cl := 0
						for _, line := range strings.Split(rawStr[:idx], "\r\n") {
							if strings.HasPrefix(strings.ToLower(line), "content-length:") {
								cl, _ = strconv.Atoi(strings.TrimSpace(line[len("content-length:"):]))
							}
						}
						if len(rawStr) >= idx+4+cl {
							break
						}
					}
				}
				return chunks.String(), true
			}
		}
		return "", true

	case "__joss_http_serve_concurrent":
		if len(args) >= 2 {
			if srvInst, ok := args[0].(*Instance); ok && srvInst != nil {
				handlerFn := args[1]
				for {
					client := r.executeSocketMethod(srvInst, "accept", nil)
					if client == nil {
						break
					}
					go func(c interface{}) {
						fork := r.Fork()
						defer fork.Free()
						fork.applyFunction(handlerFn, []interface{}{c})
					}(client)
				}
			}
		}
		return nil, true

	case "__joss_http_parse_multipart":
		if len(args) >= 2 {
			rawStr, _ := args[0].(string)
			reqMap, ok := args[1].(map[string]interface{})
			if ok && rawStr != "" {
				if idx := strings.Index(rawStr, "\r\n\r\n"); idx >= 0 {
					ct := ""
					for _, line := range strings.Split(rawStr[:idx], "\r\n") {
						if strings.HasPrefix(strings.ToLower(line), "content-type:") {
							ct = strings.TrimSpace(line[len("content-type:"):])
						}
					}
					if _, params, err := mime.ParseMediaType(ct); err == nil && params["boundary"] != "" {
						mr := multipart.NewReader(strings.NewReader(rawStr[idx+4:]), params["boundary"])
						files := map[string]interface{}{}
						for {
							p, pErr := mr.NextPart()
							if pErr != nil {
								break
							}
							b, _ := io.ReadAll(io.LimitReader(p, 32<<20))
							_ = p.Close()
							if p.FileName() != "" {
								tmpF, tErr := os.CreateTemp("", "joss_up_*")
								tmpPath := ""
								if tErr == nil {
									_, _ = tmpF.Write(b)
									tmpPath = tmpF.Name()
									_ = tmpF.Close()
								}
								files[p.FormName()] = map[string]interface{}{
									"name":     p.FileName(),
									"path":     tmpPath,
									"tmp_name": tmpPath,
									"type":     p.Header.Get("Content-Type"),
									"size":     int64(len(b)),
									"content":  string(b),
									"error":    int64(0),
								}
							} else if p.FormName() != "" {
								reqMap[p.FormName()] = string(b)
							}
						}
						reqMap["_files"] = files
					}
				}
			}
		}
		return nil, true

	case "__joss_rate_limit_check":
		if len(args) < 3 {
			return int64(0), true
		}
		ip := fmt.Sprintf("%v", args[0])
		maxReq := toInt64(args[1])
		winSec := toInt64(args[2])
		if maxReq <= 0 {
			maxReq = 60
		}
		if winSec <= 0 {
			winSec = 60
		}
		now := time.Now()
		coreRateMu.Lock()
		b, exists := coreRateBuckets[ip]
		if !exists || now.Sub(b.windowStart) > time.Duration(winSec)*time.Second {
			b = coreRateBucket{windowStart: now, count: 0}
		}
		b.count++
		coreRateBuckets[ip] = b
		coreRateMu.Unlock()
		if b.count > maxReq {
			return -winSec, true
		}
		return maxReq - b.count, true

	case "__joss_http_check_middleware":
		if len(args) < 3 {
			return "", true
		}
		mwSpec := fmt.Sprintf("%v", args[0])
		reqData, _ := args[1].(map[string]interface{})
		sessData, _ := args[2].(map[string]interface{})
		for _, rawMW := range strings.Split(mwSpec, "\n") {
			mw := strings.TrimSpace(rawMW)
			switch {
			case strings.HasPrefix(mw, "throttle:"):
				spec := strings.TrimPrefix(mw, "throttle:")
				parts := strings.Split(spec, ",")
				maxReq := int64(60)
				winSec := int64(60)
				if len(parts) >= 1 {
					if v, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64); err == nil && v > 0 {
						maxReq = v
					}
				}
				if len(parts) >= 2 {
					if v, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil && v > 0 {
						winSec = v
					}
				}
				ip := fmt.Sprintf("%v", reqData["_ip"])
				if ip == "" || ip == "<nil>" {
					ip = "127.0.0.1"
				}
				rem, _ := r.callBuiltinIO("__joss_rate_limit_check", []interface{}{ip, maxReq, winSec})
				if toInt64(rem) < 0 {
					return fmt.Sprintf("HTTP/1.1 429 Too Many Requests\r\nContent-Type: text/html; charset=utf-8\r\nRetry-After: %d\r\nConnection: close\r\n\r\n<h1>429 Too Many Requests</h1>", winSec), true
				}
			case mw == "auth":
				if toInt64(sessData["user_id"]) <= 0 {
					return "HTTP/1.1 302 Found\r\nLocation: /login\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", true
				}
			case mw == "guest":
				if toInt64(sessData["user_id"]) > 0 {
					return "HTTP/1.1 302 Found\r\nLocation: /dashboard\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", true
				}
			case mw == "admin":
				if fmt.Sprintf("%v", sessData["user_role"]) != "admin" {
					return "HTTP/1.1 302 Found\r\nLocation: /\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", true
				}
			case mw == "auth_api":
				tok := ""
				if hdrs, ok := reqData["_headers"].(map[string]interface{}); ok {
					for k, v := range hdrs {
						if strings.EqualFold(k, "authorization") {
							tok = strings.TrimPrefix(fmt.Sprintf("%v", v), "Bearer ")
						}
					}
				}
				if tok == "" {
					if cks, ok := reqData["_cookies"].(map[string]interface{}); ok {
						if v, okC := cks["joss_token"].(string); okC {
							tok = v
						}
					}
				}
				if tok == "" {
					if ut, okU := sessData["user_token"].(string); okU {
						tok = ut
					}
				}
				if tok == "" {
					return "HTTP/1.1 401 Unauthorized\r\nContent-Type: application/json; charset=utf-8\r\nConnection: close\r\n\r\n{\"error\":\"Unauthorized: Missing Token\"}", true
				}
				claims, valid := r.ValidateJWT(tok)
				if !valid {
					return "HTTP/1.1 401 Unauthorized\r\nContent-Type: application/json; charset=utf-8\r\nConnection: close\r\n\r\n{\"error\":\"Unauthorized: Invalid Token\"}", true
				}
				sessData["user_id"] = claimUserID(claims["user_id"])
				sessData["user_email"] = claims["email"]
				sessData["user_name"] = claims["name"]
				sessData["user_role"] = claims["role"]
			}
		}
		return "", true

	case "__joss_session_load":
		if len(args) < 1 {
			return map[string]interface{}{}, true
		}
		sid := fmt.Sprintf("%v", args[0])
		coreSessMu.Lock()
		src := coreSessStore[sid]
		out := make(map[string]interface{}, len(src))
		for k, v := range src {
			out[k] = v
		}
		coreSessMu.Unlock()
		return out, true

	case "__joss_session_save":
		if len(args) < 2 {
			return int64(0), true
		}
		sid := fmt.Sprintf("%v", args[0])
		data, _ := args[1].(map[string]interface{})
		cp := make(map[string]interface{}, len(data))
		for k, v := range data {
			cp[k] = v
		}
		coreSessMu.Lock()
		coreSessStore[sid] = cp
		coreSessMu.Unlock()
		return int64(1), true

	case "__joss_session_destroy":
		if len(args) >= 1 {
			sid := fmt.Sprintf("%v", args[0])
			coreSessMu.Lock()
			delete(coreSessStore, sid)
			coreSessMu.Unlock()
		}
		return int64(1), true

	case "__joss_session_last_error":
		return "", true

	case "__joss_http_inject_header":
		if len(args) >= 2 {
			respStr := fmt.Sprintf("%v", args[0])
			hdr := fmt.Sprintf("%v", args[1])
			if hdr != "" && strings.HasPrefix(respStr, "HTTP/") && !strings.Contains(respStr, "joss_session=") {
				return appendNativeHTTPHeader(respStr, hdr), true
			}
			return args[0], true
		}
		if len(args) == 1 {
			return args[0], true
		}
		return "", true

	case "__joss_jwt_parse_verified":
		if len(args) >= 1 {
			claims, valid := r.parseJWT(fmt.Sprintf("%v", args[0]))
			if valid && claims != nil {
				return claims, true
			}
		}
		return map[string]interface{}{}, true

	case "json":
		return r.executeResponseMethod(nil, "json", args), true

	case "back":
		return r.executeResponseMethod(nil, "back", nil), true

	case "response":
		return r.executeResponseMethod(nil, "raw", args), true

	case "request":
		if len(args) == 0 {
			return r.executeRequestMethod(nil, "all", nil), true
		}
		return r.executeRequestMethod(nil, "input", args), true

	case "session":
		if len(args) == 0 {
			if sessVal, ok := r.Variables["$__session"]; ok {
				return sessVal, true
			}
			return nil, true
		}
		return r.executeSessionMethod(nil, "get", args), true

	case "redirect":
		return r.executeResponseMethod(nil, "redirect", args), true

	case "file_exists":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) == 1 {
			if path, ok := args[0].(string); ok {
				_, err := os.Stat(path)
				return err == nil, true
			}
		}
		return false, true

	case "file_get_contents":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) == 1 {
			if path, ok := args[0].(string); ok {
				content, err := ReadFileBytes(path)
				if err != nil {
					return nil, true
				}
				return string(content), true
			}
		}
		return nil, true

	case "__joss_file_get_bytes":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) != 1 {
			return []interface{}{}, true
		}
		path, ok := args[0].(string)
		if !ok || path == "" {
			return []interface{}{}, true
		}
		content, err := ReadFileBytes(path)
		if err != nil {
			return []interface{}{}, true
		}
		bytes := make([]interface{}, len(content))
		for i, value := range content {
			bytes[i] = int64(value)
		}
		return bytes, true

	case "file_put_contents":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) == 2 {
			path, ok1 := args[0].(string)
			content, ok2 := args[1].(string)
			if ok1 && ok2 {
				err := os.WriteFile(path, []byte(content), 0644)
				if err != nil {
					return false, true
				}
				return true, true
			}
		}
		return false, true

	case "unlink", "file_delete":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) > 0 {
			path := fmt.Sprintf("%v", args[0])
			err := os.Remove(path)
			return err == nil, true
		}
		return false, true

	case "mkdir":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) > 0 {
			path := fmt.Sprintf("%v", args[0])
			err := os.MkdirAll(path, 0755)
			return err == nil, true
		}
		return false, true

	case "is_dir":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) > 0 {
			path := fmt.Sprintf("%v", args[0])
			fi, err := os.Stat(path)
			return err == nil && fi.IsDir(), true
		}
		return false, true

	case "is_file":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) > 0 {
			path := fmt.Sprintf("%v", args[0])
			fi, err := os.Stat(path)
			return err == nil && !fi.IsDir(), true
		}
		return false, true

	case "toon_encode":
		if len(args) == 1 {
			return ToonEncode(args[0]), true
		}
		return "", true

	case "toon_decode":
		if len(args) == 1 {
			if str, ok := args[0].(string); ok {
				return ToonDecode(str), true
			}
		}
		return nil, true

	case "toon_verify":
		if len(args) == 1 {
			if str, ok := args[0].(string); ok {
				return ToonVerify(str), true
			}
		}
		return false, true

	case "hive_read_box":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) < 1 {
			return nil, true
		}
		filePath := fmt.Sprintf("%v", args[0])
		entries, err := ReadHiveBox(filePath)
		if err != nil {
			fmt.Printf("[hive_read_box Error] %v\n", err)
			return nil, true
		}
		result := make([]interface{}, len(entries))
		for i, entry := range entries {
			result[i] = entry
		}
		return result, true

	case "run":
		if len(args) > 0 {
			if err := r.RequireCapability("process"); err != nil {
				panic(err)
			}
			scriptPath, ok := args[0].(string)
			if !ok {
				return "", true
			}

			allow, ok := r.Env["ALLOW_SYSTEM_RUN"]
			if !ok || (allow != "true" && allow != "1") {
				fmt.Println("[Security] Error: Ejecución de scripts bloqueada. Configure ALLOW_SYSTEM_RUN=true en su entorno.")
				return "", true
			}

			runner := ""
			if strings.HasSuffix(scriptPath, ".py") {
				runner = "python"
			} else if strings.HasSuffix(scriptPath, ".php") {
				runner = "php"
			} else {
				fmt.Println("[Error] Tipo de archivo no soportado para 'run'. Use .py o .php")
				return "", true
			}

			cmdArgs := []string{scriptPath}
			if len(args) > 1 {
				for _, arg := range args[1:] {
					cmdArgs = append(cmdArgs, fmt.Sprintf("%v", arg))
				}
			}

			cmd := exec.Command(runner, cmdArgs...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("[Run] Error ejecutando script: %v\n", err)
			}
			return string(output), true
		}
		return "", true

	case "__joss_db_query":
		if len(args) == 0 {
			return []interface{}{}, true
		}
		sqlStr, ok := args[0].(string)
		if !ok {
			return []interface{}{}, true
		}
		var bindings []interface{}
		if len(args) > 1 {
			if bArr, isArr := args[1].([]interface{}); isArr {
				bindings = bArr
			}
		}
		return r.executeRawDBQuery(sqlStr, bindings), true

	case "__joss_db_execute":
		if len(args) == 0 {
			return int64(0), true
		}
		sqlStr, ok := args[0].(string)
		if !ok {
			return int64(0), true
		}
		var bindings []interface{}
		if len(args) > 1 {
			if bArr, isArr := args[1].([]interface{}); isArr {
				bindings = bArr
			}
		}
		return r.executeRawDBExecute(sqlStr, bindings), true

	case "__joss_db_get_table":
		if len(args) == 0 {
			return []interface{}{}, true
		}
		tblName := fmt.Sprintf("%v", args[0])
		if r.Variables == nil {
			r.Variables = make(map[string]interface{})
		}
		tables, ok := r.Variables["__joss_db_tables"].(map[string]interface{})
		if !ok {
			tables = make(map[string]interface{})
			r.Variables["__joss_db_tables"] = tables
		}
		if rows, exists := tables[tblName]; exists {
			return rows, true
		}
		newRows := []interface{}{}
		tables[tblName] = newRows
		return newRows, true

	case "__joss_db_set_table":
		if len(args) < 2 {
			return true, true
		}
		tblName := fmt.Sprintf("%v", args[0])
		rows := args[1]
		if r.Variables == nil {
			r.Variables = make(map[string]interface{})
		}
		tables, ok := r.Variables["__joss_db_tables"].(map[string]interface{})
		if !ok {
			tables = make(map[string]interface{})
			r.Variables["__joss_db_tables"] = tables
		}
		tables[tblName] = rows
		return true, true

	case "__joss_db_is_connected":
		return r.GetDB() != nil, true

	case "__joss_storage_get":
		if len(args) < 3 {
			return nil, true
		}
		userId := fmt.Sprintf("%v", args[1])
		objectPath := fmt.Sprintf("%v", args[2])
		localPath := filepath.Join("storage", "user_storage", userId, objectPath)
		if data, err := ReadFileBytes(localPath); err == nil {
			return string(data), true
		}
		return nil, true

	case "__joss_storage_put":
		if len(args) < 4 {
			return false, true
		}
		userId := fmt.Sprintf("%v", args[1])
		objectPath := fmt.Sprintf("%v", args[2])
		content := fmt.Sprintf("%v", args[3])
		localPath := filepath.Join("storage", "user_storage", userId, objectPath)
		_ = os.MkdirAll(filepath.Dir(localPath), 0755)
		err := os.WriteFile(localPath, []byte(content), 0644)
		return err == nil, true

	case "__joss_storage_delete":
		if len(args) < 3 {
			return false, true
		}
		userId := fmt.Sprintf("%v", args[1])
		objectPath := fmt.Sprintf("%v", args[2])
		localPath := filepath.Join("storage", "user_storage", userId, objectPath)
		_ = os.Remove(localPath)
		return true, true
	}

	return nil, false
}

func formatNativeHTTPResponse(value interface{}) string {
	status := http.StatusOK
	contentType := "text/html; charset=utf-8"
	location := ""
	body := ""
	headers := make(map[string]string)
	cookies := make(map[string]interface{})
	if instance, ok := value.(*Instance); ok && instance != nil {
		fields := instance.Fields
		if rawStatus, exists := fields["status_code"]; exists {
			status = toInt(rawStatus)
		} else if rawStatus, exists := fields["status"]; exists {
			status = toInt(rawStatus)
		}
		if rawHeaders, ok := fields["headers"].(map[string]interface{}); ok {
			for key, val := range rawHeaders {
				headers[key] = fmt.Sprintf("%v", val)
			}
		}
		if rawCookies, ok := fields["cookies"].(map[string]interface{}); ok {
			cookies = rawCookies
		}
		switch fmt.Sprintf("%v", fields["_type"]) {
		case "REDIRECT":
			if _, hasStatus := fields["status_code"]; !hasStatus {
				status = http.StatusFound
			}
			location = fmt.Sprintf("%v", fields["url"])
			if location == "<nil>" {
				location = "/"
			}
			body = "Redirecting to " + location
		case "JSON":
			contentType = "application/json; charset=utf-8"
			encoded, err := json.Marshal(fields["data"])
			if err != nil {
				status = http.StatusInternalServerError
				body = `{"error":"response encoding failed"}`
			} else {
				body = string(encoded)
			}
		case "RAW":
			if rawType, ok := fields["content_type"].(string); ok && rawType != "" {
				contentType = rawType
			}
			body = fmt.Sprintf("%v", fields["data"])
		default:
			body = fmt.Sprintf("%v", fields["data"])
		}
	} else {
		body = fmt.Sprintf("%v", value)
	}
	if status < 100 || status > 599 {
		status = http.StatusInternalServerError
	}
	statusText := http.StatusText(status)
	if statusText == "" {
		statusText = "Response"
	}
	var response strings.Builder
	fmt.Fprintf(&response, "HTTP/1.1 %d %s\r\nContent-Type: %s\r\nConnection: close\r\n", status, statusText, contentType)
	if location != "" {
		fmt.Fprintf(&response, "Location: %s\r\n", location)
	}
	for key, val := range headers {
		fmt.Fprintf(&response, "%s: %s\r\n", key, val)
	}
	for key, val := range cookies {
		cookie := &http.Cookie{Name: key, Value: fmt.Sprintf("%v", val), Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode}
		if cookie.Value == "" {
			cookie.MaxAge = -1
		}
		fmt.Fprintf(&response, "Set-Cookie: %s\r\n", cookie.String())
	}
	fmt.Fprintf(&response, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return response.String()
}
