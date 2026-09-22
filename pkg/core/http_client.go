package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (r *Runtime) executeHttpMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "get":
		return r.quickHttpRequest("GET", args)

	case "post":
		return r.quickHttpRequestWithBody("POST", args)

	case "put":
		return r.quickHttpRequestWithBody("PUT", args)

	case "patch":
		return r.quickHttpRequestWithBody("PATCH", args)

	case "query":
		return r.quickHttpRequestWithBody("QUERY", args)

	case "delete":
		return r.quickHttpRequest("DELETE", args)

	case "head":
		if len(args) == 0 {
			return make(map[string]interface{})
		}
		urlStr := fmt.Sprintf("%v", args[0])
		headers := parseHeaderMap(args, 1)
		res := r.performFullHttpRequest("HEAD", urlStr, "", headers, nil, 15, true)
		if hMap, ok := res["headers"].(map[string]interface{}); ok {
			return hMap
		}
		return make(map[string]interface{})

	case "options":
		if len(args) == 0 {
			return make(map[string]interface{})
		}
		urlStr := fmt.Sprintf("%v", args[0])
		headers := parseHeaderMap(args, 1)
		res := r.performFullHttpRequest("OPTIONS", urlStr, "", headers, nil, 15, true)
		if hMap, ok := res["headers"].(map[string]interface{}); ok {
			return hMap
		}
		return make(map[string]interface{})

	case "json":
		if len(args) < 2 {
			return nil
		}
		httpMethod := strings.ToUpper(fmt.Sprintf("%v", args[0]))
		urlStr := fmt.Sprintf("%v", args[1])

		var dataObj interface{}
		if len(args) > 2 {
			dataObj = args[2]
		}

		headers := parseHeaderMap(args, 3)
		if _, ok := headers["Content-Type"]; !ok {
			headers["Content-Type"] = "application/json"
		}
		if _, ok := headers["Accept"]; !ok {
			headers["Accept"] = "application/json"
		}

		var bodyStr string
		if dataObj != nil {
			if bBytes, err := json.Marshal(dataObj); err == nil {
				bodyStr = string(bBytes)
			}
		}

		res := r.performFullHttpRequest(httpMethod, urlStr, bodyStr, headers, nil, 30, true)
		bodyRaw, _ := res["body"].(string)

		if errStr, ok := res["error"].(string); ok && errStr != "" {
			fmt.Printf("[Http::json Error] %s %s -> %s\n", httpMethod, urlStr, errStr)
			if bodyRaw == "" {
				return map[string]interface{}{
					"error": map[string]interface{}{
						"message": fmt.Sprintf("HTTP Network Error: %s", errStr),
					},
				}
			}
		}

		if bodyRaw != "" {
			var parsedJSON interface{}
			if err := json.Unmarshal([]byte(bodyRaw), &parsedJSON); err == nil {
				return parsedJSON
			}
			return map[string]interface{}{
				"error": map[string]interface{}{
					"message": fmt.Sprintf("Raw HTTP %v: %s", res["status"], bodyRaw),
				},
			}
		}

		return map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("Empty response from %s (HTTP %v)", urlStr, res["status"]),
			},
		}

	case "request":
		if len(args) < 2 {
			return map[string]interface{}{"status": 0, "status_text": "", "body": "", "success": false}
		}
		httpMethod := strings.ToUpper(fmt.Sprintf("%v", args[0]))
		urlStr := fmt.Sprintf("%v", args[1])

		headers := make(map[string]string)
		queryParams := make(map[string]string)
		bodyStr := ""
		timeoutSec := 15
		followRedirects := true

		if len(args) > 2 {
			if opts, ok := args[2].(map[string]interface{}); ok {
				if hMap, ok := opts["headers"].(map[string]interface{}); ok {
					for k, v := range hMap {
						headers[k] = fmt.Sprintf("%v", v)
					}
				}
				if qMap, ok := opts["query"].(map[string]interface{}); ok {
					for k, v := range qMap {
						queryParams[k] = fmt.Sprintf("%v", v)
					}
				}
				if b, ok := opts["body"]; ok && b != nil {
					bodyStr = fmt.Sprintf("%v", b)
				} else if jData, ok := opts["json"]; ok && jData != nil {
					if jBytes, err := json.Marshal(jData); err == nil {
						bodyStr = string(jBytes)
						if _, hasCT := headers["Content-Type"]; !hasCT {
							headers["Content-Type"] = "application/json"
						}
					}
				} else if fData, ok := opts["form"].(map[string]interface{}); ok {
					formValues := url.Values{}
					for k, v := range fData {
						formValues.Set(k, fmt.Sprintf("%v", v))
					}
					bodyStr = formValues.Encode()
					if _, hasCT := headers["Content-Type"]; !hasCT {
						headers["Content-Type"] = "application/x-www-form-urlencoded"
					}
				}

				if t, ok := opts["timeout"]; ok {
					if tInt, ok := t.(int64); ok {
						timeoutSec = int(tInt)
					} else if tFloat, ok := t.(float64); ok {
						timeoutSec = int(tFloat)
					} else if tIntVal, ok := t.(int); ok {
						timeoutSec = tIntVal
					}
				}

				if fr, ok := opts["follow_redirects"].(bool); ok {
					followRedirects = fr
				}
			}
		}

		return r.performFullHttpRequest(httpMethod, urlStr, bodyStr, headers, queryParams, timeoutSec, followRedirects)
	}

	return ""
}

func parseHeaderMap(args []interface{}, index int) map[string]string {
	headers := make(map[string]string)
	if len(args) > index {
		if hMap, ok := args[index].(map[string]interface{}); ok {
			for k, v := range hMap {
				headers[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	return headers
}

func (r *Runtime) quickHttpRequest(method string, args []interface{}) string {
	if len(args) == 0 {
		return ""
	}
	urlStr := fmt.Sprintf("%v", args[0])
	headers := parseHeaderMap(args, 1)
	res := r.performFullHttpRequest(method, urlStr, "", headers, nil, 15, true)
	if body, ok := res["body"].(string); ok {
		return body
	}
	return ""
}

func (r *Runtime) quickHttpRequestWithBody(method string, args []interface{}) string {
	if len(args) == 0 {
		return ""
	}
	urlStr := fmt.Sprintf("%v", args[0])
	headers := parseHeaderMap(args, 2)
	bodyStr := ""
	if len(args) > 1 && args[1] != nil {
		if m, ok := args[1].(map[string]interface{}); ok {
			contentType, _ := headers["Content-Type"]
			if strings.Contains(contentType, "application/x-www-form-urlencoded") {
				form := url.Values{}
				for k, v := range m {
					form.Set(k, fmt.Sprintf("%v", v))
				}
				bodyStr = form.Encode()
			} else {
				bBytes, _ := json.Marshal(m)
				bodyStr = string(bBytes)
			}
		} else {
			bodyStr = fmt.Sprintf("%v", args[1])
		}
	}
	res := r.performFullHttpRequest(method, urlStr, bodyStr, headers, nil, 15, true)
	if body, ok := res["body"].(string); ok {
		return body
	}
	return ""
}

func (r *Runtime) performFullHttpRequest(method, urlStr, bodyStr string, headers map[string]string, queryParams map[string]string, timeoutSec int, followRedirects bool) map[string]interface{} {
	if err := r.RequireCapability("network"); err != nil {
		panic(err)
	}
	if timeoutSec <= 0 {
		timeoutSec = 15
	}

	if len(queryParams) > 0 {
		parsedURL, err := url.Parse(urlStr)
		if err == nil {
			q := parsedURL.Query()
			for k, v := range queryParams {
				q.Set(k, v)
			}
			parsedURL.RawQuery = q.Encode()
			urlStr = parsedURL.String()
		}
	}

	client := &http.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
	}

	if !followRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	var reqBody io.Reader
	if bodyStr != "" {
		reqBody = bytes.NewBufferString(bodyStr)
	}

	req, err := http.NewRequest(method, urlStr, reqBody)
	if err != nil {
		return map[string]interface{}{
			"status":      0,
			"status_text": "",
			"body":        "",
			"headers":     make(map[string]interface{}),
			"success":     false,
			"error":       err.Error(),
		}
	}
	if r.executionContext != nil {
		req = req.WithContext(r.executionContext)
	}

	req.Header.Set("User-Agent", "Joss/3.6 Runtime")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		r.checkExecutionCancelled()
		return map[string]interface{}{
			"status":      0,
			"status_text": "",
			"body":        "",
			"headers":     make(map[string]interface{}),
			"success":     false,
			"error":       err.Error(),
		}
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		r.checkExecutionCancelled()
		return map[string]interface{}{
			"status":      resp.StatusCode,
			"status_text": resp.Status,
			"body":        "",
			"headers":     make(map[string]interface{}),
			"success":     false,
			"error":       err.Error(),
		}
	}

	respHeaders := make(map[string]interface{})
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	var parsedJSON interface{}
	bodyRaw := string(respBytes)
	if strings.HasPrefix(strings.TrimSpace(bodyRaw), "{") || strings.HasPrefix(strings.TrimSpace(bodyRaw), "[") {
		_ = json.Unmarshal([]byte(bodyRaw), &parsedJSON)
	}

	return map[string]interface{}{
		"status":      resp.StatusCode,
		"status_text": resp.Status,
		"body":        bodyRaw,
		"json":        parsedJSON,
		"headers":     respHeaders,
		"success":     resp.StatusCode >= 200 && resp.StatusCode < 300,
	}
}
