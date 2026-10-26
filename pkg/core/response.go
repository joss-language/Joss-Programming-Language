package core

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/parser"
)

// executeResponseMethod handles Response methods
func (r *Runtime) executeResponseMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "error":
		if len(args) > 0 {
			statusCode := 400
			if len(args) > 1 {
				statusCode = toInt(args[1])
			}
			return r.createWebResponse("JSON", "", map[string]interface{}{"error": fmt.Sprintf("%v", args[0])}, statusCode)
		}

	case "redirect":
		if len(args) > 0 {
			url, ok := args[0].(string)
			if !ok {
				url = fmt.Sprintf("%v", args[0])
			}
			statusCode := 302
			if len(args) > 1 {
				statusCode = toInt(args[1])
			}
			return r.createWebResponse("REDIRECT", url, nil, statusCode)
		}
	case "back":
		referer := "/"
		if reqVal, ok := r.Variables["$__request"]; ok {
			if reqInstance, ok := reqVal.(*Instance); ok {
				if ref, ok := reqInstance.Fields["_referer"]; ok && ref != "" {
					if refStr, okStr := ref.(string); okStr {
						referer = refStr
					}
				}
			}
		}
		return r.createWebResponse("REDIRECT", referer, nil, 302)

	case "json":
		if len(args) > 0 {
			statusCode := 200
			if len(args) > 1 {
				statusCode = toInt(args[1])
			}
			return r.createWebResponse("JSON", "", args[0], statusCode)
		}

	case "raw":
		if len(args) > 0 {
			content := args[0]
			statusCode := 200
			if len(args) > 1 {
				statusCode = toInt(args[1])
			}
			res := r.createWebResponse("RAW", "", content, statusCode)

			// Content Type
			if len(args) > 2 {
				if ct, ok := args[2].(string); ok {
					res.Fields["content_type"] = ct
				}
			}
			// Headers
			if len(args) > 3 {
				if headers, ok := args[3].(map[string]interface{}); ok {
					if resHeaders, ok := res.Fields["headers"].(map[string]interface{}); ok {
						for k, v := range headers {
							resHeaders[k] = v
						}
					}
				}
			}
			return res
		}

	case "stream":
		// Response::stream(callback)
		if len(args) > 0 {
			callback := args[0] // Is a *parser.FunctionLiteral or closure
			res := r.createWebResponse("STREAM", "", nil, 200)
			res.Fields["callback"] = callback
			return res
		}

	case "download":
		// Response::download(filePath, fileName)
		if len(args) > 0 {
			filePath := fmt.Sprintf("%v", args[0])
			res := r.createWebResponse("FILE", "", filePath, 200)
			if len(args) > 1 {
				res.Fields["download_name"] = fmt.Sprintf("%v", args[1])
			}
			return res
		}
	}
	return nil
}

func (r *Runtime) createWebResponse(resType string, url string, data interface{}, status int) *Instance {
	// We need a class for this to support method calls
	if _, ok := r.Classes["WebResponse"]; !ok {
		r.registerClass(&parser.ClassStatement{
			Name: &parser.Identifier{Value: "WebResponse"},
			Body: &parser.BlockStatement{},
		})
	}

	instance := &Instance{
		Class: r.Classes["WebResponse"],
		Fields: map[string]interface{}{
			"_type":       resType,
			"url":         url,
			"data":        data,
			"status_code": status,
			"flash":       make(map[string]interface{}),
			"cookies":     make(map[string]interface{}),
			"headers":     make(map[string]interface{}),
		},
	}

	// Default Content Type for RAW
	if resType == "RAW" {
		instance.Fields["content_type"] = "text/plain"
	}

	return instance
}

// executeWebResponseMethod handles methods on the WebResponse instance (like ->withCookie())
func (r *Runtime) executeWebResponseMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "with":
		if len(args) >= 2 {
			key := fmt.Sprintf("%v", args[0])
			val := args[1]
			if flash, ok := instance.Fields["flash"].(map[string]interface{}); ok {
				flash[key] = val
			}
			return instance
		}

	case "withCookie":
		// ->withCookie(name, value)
		if len(args) >= 2 {
			key := fmt.Sprintf("%v", args[0])
			val := args[1]
			if cookies, ok := instance.Fields["cookies"].(map[string]interface{}); ok {
				cookies[key] = val
			}
			return instance
		}

	case "withHeader":
		// ->withHeader(key, value)
		if len(args) >= 2 {
			key := fmt.Sprintf("%v", args[0])
			val := args[1]
			if headers, ok := instance.Fields["headers"].(map[string]interface{}); ok {
				headers[key] = val
			}
			return instance
		}

	case "status":
		// ->status(404)
		if len(args) >= 1 {
			instance.Fields["status_code"] = toInt(args[0])
			return instance
		}
	}
	return instance
}

// executeRedirectMethod handles Redirect::to($url, $status) — PHP-style redirect helper
func (r *Runtime) executeRedirectMethod(instance *Instance, method string, args []interface{}) interface{} {
	if method == "to" && len(args) >= 1 {
		url := fmt.Sprintf("%v", args[0])
		status := 302
		if len(args) >= 2 {
			status = toInt(args[1])
		}
		res := r.createWebResponse("REDIRECT", url, nil, status)
		res.Fields["status_code"] = status
		return res
	}
	return nil
}

func toInt(val interface{}) int {
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 200
	}
}
