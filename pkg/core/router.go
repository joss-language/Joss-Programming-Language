package core

import (
	"fmt"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
)

// executeRouterMethod handles Router class methods (get, post, api, match)
// This fixes the bug where Router methods were not implemented
func (r *Runtime) executeRouterMethod(instance *Instance, method string, args []interface{}) interface{} {
	// Initialize Routes map if needed
	if r.Routes == nil {
		r.Routes = make(map[string]map[string]interface{})
	}

	// Helper to add route
	addRoute := func(method, path string, handler interface{}) {
		if r.Routes[method] == nil {
			r.Routes[method] = make(map[string]interface{})
		}

		// Store as RouteInfo map
		routeInfo := map[string]interface{}{
			"handler":    handler,
			"middleware": []string{},
			"source":     r.CurrentSource,
		}

		// Add current middleware if any
		if len(r.CurrentMiddleware) > 0 {
			mwCopy := make([]string, len(r.CurrentMiddleware))
			copy(mwCopy, r.CurrentMiddleware)
			routeInfo["middleware"] = mwCopy
		}

		r.Routes[method][path] = routeInfo
	}

	switch method {
	case "middleware":
		if len(args) >= 1 {
			if mw, ok := args[0].(string); ok {
				// Start middleware group
				if r.CurrentMiddleware == nil {
					r.CurrentMiddleware = []string{}
				}
				r.CurrentMiddleware = append(r.CurrentMiddleware, mw)
			}
		}
		return nil

	case "registerMiddleware":
		if len(args) >= 2 {
			name := args[0].(string)
			handler := args[1]

			if r.CustomMiddlewares == nil {
				r.CustomMiddlewares = make(map[string]interface{})
			}
			r.CustomMiddlewares[name] = handler
			fmt.Printf("[DEBUG] Middleware registered: %s\n", name)
		}
		return nil

	case "end":
		// End middleware group (pop last)
		if len(r.CurrentMiddleware) > 0 {
			r.CurrentMiddleware = r.CurrentMiddleware[:len(r.CurrentMiddleware)-1]
		}
		return nil

	case "get":
		if len(args) >= 2 {
			path := args[0].(string)
			handler := args[1]
			addRoute("GET", path, handler)
			fmt.Printf("[DEBUG] executeRouterMethod called: get (%s)\n", path)
		}
		return nil

	case "post":
		if len(args) >= 2 {
			path := args[0].(string)
			handler := args[1]
			addRoute("POST", path, handler)
			fmt.Printf("[DEBUG] executeRouterMethod called: post (%s)\n", path)
		}
		return nil

	case "put":
		if len(args) >= 2 {
			path := args[0].(string)
			handler := args[1]
			addRoute("PUT", path, handler)
			fmt.Printf("[DEBUG] executeRouterMethod called: put (%s)\n", path)
		}
		return nil

	case "delete":
		if len(args) >= 2 {
			path := args[0].(string)
			handler := args[1]
			addRoute("DELETE", path, handler)
			fmt.Printf("[DEBUG] executeRouterMethod called: delete (%s)\n", path)
		}
		return nil

	case "patch":
		if len(args) >= 2 {
			path := args[0].(string)
			handler := args[1]
			addRoute("PATCH", path, handler)
			fmt.Printf("[DEBUG] executeRouterMethod called: patch (%s)\n", path)
		}
		return nil

	case "options":
		if len(args) >= 2 {
			path := args[0].(string)
			handler := args[1]
			addRoute("OPTIONS", path, handler)
			fmt.Printf("[DEBUG] executeRouterMethod called: options (%s)\n", path)
		}
		return nil

	case "head":
		if len(args) >= 2 {
			path := args[0].(string)
			handler := args[1]
			addRoute("HEAD", path, handler)
			fmt.Printf("[DEBUG] executeRouterMethod called: head (%s)\n", path)
		}
		return nil

	case "query":
		if len(args) >= 2 {
			path := args[0].(string)
			handler := args[1]
			addRoute("QUERY", path, handler)
			fmt.Printf("[DEBUG] executeRouterMethod called: query (%s)\n", path)
		}
		return nil

	case "any":
		// Router::any("/path", handler)
		// Registers route for ALL HTTP verbs: GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD, QUERY
		if len(args) >= 2 {
			path := args[0].(string)
			handler := args[1]
			methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD", "QUERY"}

			if handlerStr, ok := handler.(string); ok {
				handlerParts := strings.Split(handlerStr, "@")
				if len(handlerParts) > 2 {
					controller := handlerParts[0]
					methodHandlers := handlerParts[1:]
					for i, m := range methods {
						if i < len(methodHandlers) {
							fullHandler := controller + "@" + methodHandlers[i]
							addRoute(m, path, fullHandler)
						} else {
							fullHandler := controller + "@" + methodHandlers[len(methodHandlers)-1]
							addRoute(m, path, fullHandler)
						}
					}
				} else {
					for _, m := range methods {
						addRoute(m, path, handlerStr)
					}
				}
			} else {
				for _, m := range methods {
					addRoute(m, path, handler)
				}
			}
			fmt.Printf("[DEBUG] executeRouterMethod called: any (%s)\n", path)
		}
		return nil

	case "match":
		// Router::match("GET|POST", "/path", "Controller@method1@method2")
		if len(args) >= 3 {
			methodsStr := args[0].(string)
			path := args[1].(string)
			handlerStr := args[2].(string)

			methods := strings.Split(methodsStr, "|")
			handlerParts := strings.Split(handlerStr, "@")

			// Case 1: Controller@method (Same for all)
			if len(handlerParts) == 2 {
				for _, m := range methods {
					addRoute(strings.ToUpper(strings.TrimSpace(m)), path, handlerStr)
				}
			} else if len(handlerParts) > 2 {
				// Case 2: Controller@method1@method2 (Map to methods)
				controller := handlerParts[0]
				methodHandlers := handlerParts[1:]

				for i, m := range methods {
					if i < len(methodHandlers) {
						fullHandler := controller + "@" + methodHandlers[i]
						addRoute(strings.ToUpper(strings.TrimSpace(m)), path, fullHandler)
					}
				}
			}
			fmt.Printf("[DEBUG] executeRouterMethod called: match (%s)\n", path)
		}
		return nil

	case "api":
		// API routes can be GET or POST, register for both
		if len(args) >= 2 {
			path := args[0].(string)
			handler := args[1]
			addRoute("GET", path, handler)
			addRoute("POST", path, handler)
			fmt.Printf("[DEBUG] executeRouterMethod called: api (%s)\n", path)
		}
		return nil

	case "ws":
		// Router.ws("/path", "Controller@method")
		if len(args) >= 2 {
			path := args[0].(string)
			handler := args[1]
			// Internally we treat WS as a special method "WS"
			addRoute("WS", path, handler)
			fmt.Printf("[DEBUG] executeRouterMethod called: ws (%s)\n", path)
		}
		return nil

	case "group":
		// Router::group("middleware", func() { ... })
		if len(args) >= 2 {
			mwName := args[0].(string)
			callback := args[1]

			// Push Middleware
			if r.CurrentMiddleware == nil {
				r.CurrentMiddleware = []string{}
			}
			r.CurrentMiddleware = append(r.CurrentMiddleware, mwName)

			if captured, ok := callback.(*CapturedFunction); ok {
				r.callCapturedFunction(captured, nil)
			} else if fn, ok := callback.(*parser.FunctionLiteral); ok {
				r.executeBlock(fn.Body)
			} else {
				fmt.Printf("[ERROR] Router.group callback is not a function: %T\n", callback)
			}

			// Pop Middleware
			if len(r.CurrentMiddleware) > 0 {
				r.CurrentMiddleware = r.CurrentMiddleware[:len(r.CurrentMiddleware)-1]
			}
			fmt.Printf("[DEBUG] executeRouterMethod called: group (%s)\n", mwName)
		}
		return nil

	}

	return nil
}
