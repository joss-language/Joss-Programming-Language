package core

import (
	"fmt"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
)

// parseHTTPWithStdlib delegates raw HTTP decoding to the canonical Joss
// Standard Library parser. This adapter only invokes the source implementation;
// it does not maintain a second request parsing policy in the interpreter.
func (r *Runtime) parseHTTPWithStdlib(raw string) map[string]interface{} {
	class := r.Classes["HttpRequestParser"]
	if class == nil || class.Body == nil {
		return nil
	}
	for _, statement := range class.Body.Statements {
		method, ok := statement.(*parser.MethodStatement)
		if !ok || method.Name == nil || method.Name.Value != "parse" {
			continue
		}
		value := r.CallMethodEvaluated(method, nil, []interface{}{raw})
		request, ok := value.(map[string]interface{})
		if !ok {
			return nil
		}
		return request
	}
	return nil
}

// executeRequestMethod handles Request methods
func (r *Runtime) executeRequestMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {

	case "file":
		if len(args) > 0 {
			key, ok := args[0].(string)
			if !ok {
				return nil
			}

			// Access $__request variable
			if reqVal, ok := r.Variables["$__request"]; ok {
				if reqInstance, ok := reqVal.(*Instance); ok {
					// Check _files map
					if filesVal, ok := reqInstance.Fields["_files"]; ok {
						if filesMap, ok := filesVal.(map[string]interface{}); ok {
							if file, ok := filesMap[key]; ok {
								return file // Returns the map {name, content, ...}
							}
						}
					}
				}
			}
			return nil
		}

	case "input", "post":
		if len(args) > 0 {
			key, ok := args[0].(string)
			if !ok {
				return nil
			}

			// Access $__request variable injected by Dispatch
			if reqVal, ok := r.Variables["$__request"]; ok {
				if reqInstance, ok := reqVal.(*Instance); ok {
					if val, ok := reqInstance.Fields[key]; ok {
						return val
					}
				}
			}
			if len(args) > 1 {
				return args[1]
			}
			return nil
		}
	case "all":
		if reqVal, ok := r.Variables["$__request"]; ok {
			if reqInstance, ok := reqVal.(*Instance); ok {
				// Filter out internal fields
				result := make(map[string]interface{})
				for k, v := range reqInstance.Fields {
					// exclude internal fields starting with _
					if k != "_headers" && k != "_host" && k != "_scheme" && k != "_files" && k != "_cookies" && k != "_method" && k != "_referer" && k != "_token" {
						result[k] = v
					}
				}
				return result
			}
		}
		return make(map[string]interface{})

	case "except":
		if len(args) > 0 {
			excludeMap := make(map[string]bool)
			if list, ok := args[0].([]interface{}); ok {
				for _, item := range list {
					excludeMap[fmt.Sprintf("%v", item)] = true
				}
			}

			if reqVal, ok := r.Variables["$__request"]; ok {
				if reqInstance, ok := reqVal.(*Instance); ok {
					result := make(map[string]interface{})
					for k, v := range reqInstance.Fields {
						// exclude internal fields
						if !excludeMap[k] && k != "_headers" && k != "_host" && k != "_scheme" && k != "_files" && k != "_cookies" && k != "_method" && k != "_referer" && k != "_token" {
							result[k] = v
						}
					}
					return result
				}
			}
		}
		return make(map[string]interface{})

	case "root":
		// Return scheme://host
		if reqVal, ok := r.Variables["$__request"]; ok {
			if reqInstance, ok := reqVal.(*Instance); ok {
				scheme := "http"
				if s, ok := reqInstance.Fields["_scheme"].(string); ok {
					scheme = s
				}
				host := "localhost"
				if h, ok := reqInstance.Fields["_host"].(string); ok {
					host = h
				}
				return scheme + "://" + host
			}
		}
		return ""

	case "cookie":
		if len(args) > 0 {
			key, ok := args[0].(string)
			if !ok {
				return ""
			}
			if reqVal, ok := r.Variables["$__request"]; ok {
				if reqInstance, ok := reqVal.(*Instance); ok {
					// Check _cookies map
					if cookieVal, ok := reqInstance.Fields["_cookies"]; ok {
						if cookieMap, ok := cookieVal.(map[string]interface{}); ok {
							if val, ok := cookieMap[key]; ok {
								return fmt.Sprintf("%v", val)
							}
						}
					}
				}
			}
			if len(args) > 1 {
				return args[1]
			}
		}
		return ""

	case "header":
		if len(args) > 0 {
			key, ok := args[0].(string)
			if !ok {
				return nil
			}
			if reqVal, ok := r.Variables["$__request"]; ok {
				if reqInstance, ok := reqVal.(*Instance); ok {
					// 1. Check Top-Level Overrides (e.g. Authorization)
					if val, ok := reqInstance.Fields[key]; ok {
						return fmt.Sprintf("%v", val)
					}
					// 2. Check _headers map
					if headersVal, ok := reqInstance.Fields["_headers"]; ok {
						if headersMap, ok := headersVal.(map[string]interface{}); ok {
							if val, ok := headersMap[key]; ok {
								return fmt.Sprintf("%v", val)
							}
							for hk, hv := range headersMap {
								if strings.EqualFold(hk, key) {
									return fmt.Sprintf("%v", hv)
								}
							}
						}
					}
					for fk, fv := range reqInstance.Fields {
						if strings.EqualFold(fk, key) {
							return fmt.Sprintf("%v", fv)
						}
					}
				}
			}
			return nil
		}

	case "hasFile", "hasfile":
		if len(args) > 0 {
			key, ok := args[0].(string)
			if !ok {
				return false
			}
			if reqVal, ok := r.Variables["$__request"]; ok {
				if reqInstance, ok := reqVal.(*Instance); ok {
					if filesVal, ok := reqInstance.Fields["_files"]; ok {
						if filesMap, ok := filesVal.(map[string]interface{}); ok {
							if file, ok := filesMap[key]; ok && file != nil {
								return true
							}
						}
					}
				}
			}
		}
		return false

	case "has":
		if len(args) > 0 {
			key, ok := args[0].(string)
			if !ok {
				return false
			}
			if reqVal, ok := r.Variables["$__request"]; ok {
				if reqInstance, ok := reqVal.(*Instance); ok {
					if val, ok := reqInstance.Fields[key]; ok && val != nil && val != "" {
						return true
					}
				}
			}
		}
		return false

	case "method":
		if reqVal, ok := r.Variables["$__request"]; ok {
			if reqInstance, ok := reqVal.(*Instance); ok {
				if m, ok := reqInstance.Fields["_method"].(string); ok {
					return m
				}
			}
		}
		return "GET"

	case "isMethod", "ismethod":
		if len(args) > 0 {
			targetMethod, ok := args[0].(string)
			if !ok {
				return false
			}
			if reqVal, ok := r.Variables["$__request"]; ok {
				if reqInstance, ok := reqVal.(*Instance); ok {
					if m, ok := reqInstance.Fields["_method"].(string); ok {
						return fmt.Sprintf("%v", m) == targetMethod
					}
				}
			}
		}
		return false

	case "path", "url", "uri":
		if reqVal, ok := r.Variables["$__request"]; ok {
			if reqInstance, ok := reqVal.(*Instance); ok {
				if method == "uri" {
					if u, ok := reqInstance.Fields["_uri"].(string); ok && u != "" {
						return u
					}
				}
				if p, ok := reqInstance.Fields["_path"].(string); ok && p != "" {
					return p
				}
				if u, ok := reqInstance.Fields["_url"].(string); ok && u != "" {
					return u
				}
				if u, ok := reqInstance.Fields["_uri"].(string); ok && u != "" {
					return u
				}
			}
		}
		return "/"

	case "ip":
		if reqVal, ok := r.Variables["$__request"]; ok {
			if reqInstance, ok := reqVal.(*Instance); ok {
				if ip, ok := reqInstance.Fields["_ip"].(string); ok {
					return ip
				}
			}
		}
		return "127.0.0.1"

	case "userAgent", "useragent":
		return r.executeRequestMethod(instance, "header", []interface{}{"User-Agent"})

	case "bearerToken", "bearertoken":
		authHeader := r.executeRequestMethod(instance, "header", []interface{}{"Authorization"})
		if authStr, ok := authHeader.(string); ok {
			if len(authStr) > 7 && (authStr[:7] == "Bearer " || authStr[:7] == "bearer ") {
				return authStr[7:]
			}
		}
		return ""
	}
	return nil
}
