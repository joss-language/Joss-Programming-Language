package core

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
)

// Dispatch handles HTTP requests based on defined routes
func (r *Runtime) Dispatch(method, path string, reqData, sessData map[string]interface{}) (interface{}, error) {
	// Inject Request and Session
	r.Variables["$__request"] = &Instance{Class: r.Classes["Request"], Fields: reqData}
	r.Variables["$__session"] = &Instance{Class: r.Classes["Session"], Fields: sessData}
	r.markHostGlobals("$__request", "$__session")

	// Match Route
	var handler interface{}
	var middleware []string
	method = strings.ToUpper(method)

	// Check exact match
	if r.Routes[method] != nil {
		if routeInfo, ok := r.Routes[method][path].(map[string]interface{}); ok {
			handler = routeInfo["handler"]
			if mw, ok := routeInfo["middleware"].([]string); ok {
				middleware = mw
			}
		}
	}

	// Check dynamic routes if no exact match
	if handler == nil && r.Routes[method] != nil {
		for routePath, routeVal := range r.Routes[method] {
			if strings.Contains(routePath, "{") {
				// Regex matching (simplified)
				regexPath := "^" + regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`).ReplaceAllString(routePath, "([^/]+)") + "$"
				re := regexp.MustCompile(regexPath)
				matches := re.FindStringSubmatch(path)

				if len(matches) > 0 {
					if routeInfo, ok := routeVal.(map[string]interface{}); ok {
						handler = routeInfo["handler"]
						if mw, ok := routeInfo["middleware"].([]string); ok {
							middleware = mw
						}
					}
					break
				}
			}
		}
	}

	if handler == nil {
		fmt.Printf("[DEBUG] DISPATCH FAIL: Route not found: %s %s\n", method, path)
		fmt.Printf("[DEBUG] Available Routes for %s:\n", method)
		if r.Routes[method] != nil {
			for k := range r.Routes[method] {
				fmt.Printf("\t'%s'\n", k)
			}
		} else {
			fmt.Println("\t(None)")
		}
		return nil, fmt.Errorf("route not found: %s %s", method, path)
	}

	// Debug Handler Type
	fmt.Printf("[DEBUG] DISPATCH SUCCESS: Found handler for %s %s\n", method, path)

	// Middleware Execution
	for _, mw := range middleware {
		switch mw {
		case "auth":
			// Check if logged in
			isLoggedIn := false
			if sessInst, ok := r.Variables["$__session"].(*Instance); ok {
				if _, ok := sessInst.Fields["user_id"]; ok {
					isLoggedIn = true
				}
			}
			if !isLoggedIn {
				return &Instance{
					Fields: map[string]interface{}{
						"_type": "REDIRECT",
						"url":   "/login",
						"flash": map[string]interface{}{
							"error": "Debes iniciar sesión.",
						},
					},
				}, nil
			}
		case "guest":
			// Check if NOT logged in
			isLoggedIn := false
			if sessInst, ok := r.Variables["$__session"].(*Instance); ok {
				if _, ok := sessInst.Fields["user_id"]; ok {
					isLoggedIn = true
				}
			}
			if isLoggedIn {
				return &Instance{
					Fields: map[string]interface{}{
						"_type": "REDIRECT",
						"url":   "/dashboard",
					},
				}, nil
			}
		case "admin":
			// Check if admin
			isAdmin := false
			if sessInst, ok := r.Variables["$__session"].(*Instance); ok {
				if role, ok := sessInst.Fields["user_role"]; ok && role == "admin" {
					isAdmin = true
				}
			}
			if !isAdmin {
				return &Instance{
					Fields: map[string]interface{}{
						"_type": "REDIRECT",
						"url":   "/",
						"flash": map[string]interface{}{
							"error": "Acceso denegado.",
						},
					},
				}, nil
			}
		case "auth_api":
			// Check Authorization header
			authHeader := ""
			// We need to access headers securely. ReqData should have it.
			// Assuming reqData["header"] or reqData["headers"]
			if reqInst, ok := r.Variables["$__request"].(*Instance); ok {
				// Try to find headers
				if h, ok := reqInst.Fields["_headers"].(map[string]interface{}); ok {
					if val, k := h["Authorization"].(string); k {
						authHeader = val
					}
				}
				// Fallback or variation
				if authHeader == "" {
					if h, ok := reqData["Authorization"].(string); ok {
						authHeader = h
					}
				}
			}

			// Simple Bearer check
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == "" {
				// Fallback to cookie joss_token from request cookies
				if reqInst, ok := r.Variables["$__request"].(*Instance); ok {
					if c, ok := reqInst.Fields["_cookies"].(map[string]interface{}); ok {
						if val, k := c["joss_token"].(string); k && val != "" {
							tokenString = val
						}
					}
				}
				if tokenString == "" {
					if c, ok := reqData["_cookies"].(map[string]interface{}); ok {
						if val, k := c["joss_token"].(string); k && val != "" {
							tokenString = val
						}
					}
				}
				if tokenString == "" {
					// Fallback to session user_token
					if sessInst, ok := r.Variables["$__session"].(*Instance); ok {
						if ut, ok := sessInst.Fields["user_token"].(string); ok && ut != "" {
							tokenString = ut
						}
					}
				}
			}

			if tokenString == "" {
				return &Instance{Fields: map[string]interface{}{
					"_type":  "JSON",
					"status": 401,
					"data":   map[string]interface{}{"error": "Unauthorized: Missing Token"},
				}}, nil
			}

			// MFA challenge tokens are intentionally rejected by normal API auth.
			claims, valid := r.ValidateJWT(tokenString)
			if !valid {
				return &Instance{Fields: map[string]interface{}{
					"_type":  "JSON",
					"status": 401,
					"data":   map[string]interface{}{"error": "Unauthorized: Invalid Token"},
				}}, nil
			}

			// Success: Inject user info into request?
			if claims != nil {
				// Inject into $__user or similar if needed. For now, pass.
				// Maybe populate $__session with user info for this request context?
				if sessInst, ok := r.Variables["$__session"].(*Instance); ok {
					if uid, ok := claims["user_id"].(float64); ok {
						sessInst.Fields["user_id"] = int(uid)
					} else {
						sessInst.Fields["user_id"] = claims["user_id"]
					}
					sessInst.Fields["user_email"] = claims["email"]
					sessInst.Fields["user_name"] = claims["name"]
					if role, ok := claims["role"].(string); ok {
						sessInst.Fields["user_role"] = role
					}
				}
			}

		default:
			// Custom Middleware
			if r.CustomMiddlewares != nil {
				if handler, ok := r.CustomMiddlewares[mw]; ok {
					// Debug Log
					fmt.Printf("[DEBUG] Executing Custom Middleware: %s\n", mw)
					fmt.Printf("[DEBUG] Middleware Handler Type: %T\n", handler)

					// Execute closure
					res := r.applyFunction(handler, []interface{}{mw})

					// Debug Result
					if res != nil {
						fmt.Printf("[DEBUG] Middleware Result Type: %T\n", res)
					}

					// If returns a Result/Response, stop and return it
					if inst, ok := res.(*Instance); ok {
						// Assuming standard response structure or check existence
						if _, hasType := inst.Fields["_type"]; hasType {
							fmt.Printf("[DEBUG] Middleware %s returned Response (Redirect/JSON), stopping request.\n", mw)
							return inst, nil
						}
					}
				} else {
					fmt.Printf("[DEBUG] Custom Middleware %s NOT FOUND in execution map.\n", mw)
				}
			}
		}
	}

	// Execute Handler
	// Execute Handler
	if handlerName, ok := handler.(string); ok {
		// Controller@Method
		parts := strings.Split(handlerName, "@")
		if len(parts) == 2 {
			controllerName := parts[0]
			methodName := parts[1]

			// Find Controller Class
			if classStmt, ok := r.Classes[controllerName]; ok {
				// Create Instance
				instance := &Instance{Class: classStmt, Fields: make(map[string]interface{})}

				// Find Method
				for _, stmt := range classStmt.Body.Statements {
					if m, ok := stmt.(*parser.MethodStatement); ok {
						if m.Name.Value == methodName {
							// Extract parameters if dynamic route
							args := []interface{}{}
							if strings.Contains(path, "/") {
								for routePath := range r.Routes[method] {
									if strings.Contains(routePath, "{") {
										regexPath := "^" + regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`).ReplaceAllString(routePath, "([^/]+)") + "$"
										re := regexp.MustCompile(regexPath)
										matches := re.FindStringSubmatch(path)
										if len(matches) > 1 {
											for _, param := range matches[1:] {
												args = append(args, param)
											}
											break
										}
									}
								}
							}
							fmt.Printf("[DEBUG] Executing method %s@%s\n", controllerName, methodName)
							return r.CallMethodEvaluated(m, instance, args), nil
						}
					}
				}
				fmt.Printf("[DEBUG] Method %s not found in controller %s\n", methodName, controllerName)
				return nil, fmt.Errorf("method %s not found in controller %s", methodName, controllerName)
			}
			fmt.Printf("[DEBUG] Controller %s not found. Available classes: %d\n", controllerName, len(r.Classes))
			for k := range r.Classes {
				fmt.Printf("\tClass: %s\n", k)
			}
			return nil, fmt.Errorf("controller %s not found", controllerName)
		}
	}

	// Handle FunctionLiteral closures as route handlers
	// e.g. Router::get("/sound/{id}", func ($id) { return Redirect::to(...) })
	if fn, ok := handler.(*parser.FunctionLiteral); ok {
		args := r.extractRouteParams(method, path)
		synth := &parser.MethodStatement{
			Token:      fn.Token,
			Name:       &parser.Identifier{Value: "anonymous"},
			Parameters: fn.Parameters,
			ReturnType: fn.ReturnType,
			Body:       fn.Body,
		}
		fmt.Printf("[DEBUG] Executing closure handler for %s %s\n", method, path)
		return r.CallMethodEvaluated(synth, nil, args), nil
	}

	return nil, nil
}

// extractRouteParams finds the dynamic URL parameters for the given method+path
// by matching against registered {param} routes and returning the captured values.
func (r *Runtime) extractRouteParams(method, path string) []interface{} {
	args := []interface{}{}
	if r.Routes[method] == nil {
		return args
	}
	for routePath := range r.Routes[method] {
		if !strings.Contains(routePath, "{") {
			continue
		}
		regexPath := "^" + regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`).ReplaceAllString(routePath, "([^/]+)") + "$"
		re := regexp.MustCompile(regexPath)
		matches := re.FindStringSubmatch(path)
		if len(matches) > 1 {
			for _, param := range matches[1:] {
				args = append(args, param)
			}
			return args
		}
	}
	return args
}

// DispatchWebSocket handles WebSocket upgrades
func (r *Runtime) DispatchWebSocket(path string, conn interface{}, reader func() (int, []byte, error), sender func(interface{}) error, closer func() error) {
	// Inject a blank session for WS context so Auth can store credentials
	if _, ok := r.Variables["$__session"]; !ok {
		r.Variables["$__session"] = &Instance{
			Class:  r.Classes["Session"],
			Fields: make(map[string]interface{}),
		}
		r.markHostGlobals("$__session")
	}

	// Conn is passed as interface{}, expected to be *websocket.Conn
	// Reader is a closure wrapping conn.ReadMessage()
	// Sender is a closure wrapping conn.WriteJSON()

	// Match Route
	var handler interface{}
	var routeParams []interface{}
	if r.Routes["WS"] != nil {
		if routeInfo, ok := r.Routes["WS"][path].(map[string]interface{}); ok {
			handler = routeInfo["handler"]
		}
		if handler == nil {
			for pattern, rawHandler := range r.Routes["WS"] {
				params, matches := matchRoutePattern(pattern, path)
				if !matches {
					continue
				}
				routeParams = params
				if routeInfo, ok := rawHandler.(map[string]interface{}); ok {
					handler = routeInfo["handler"]
				}
				break
			}
		}
	}

	if handler == nil {
		fmt.Printf("[WS] No handler found for %s\n", path)
		return
	}

	// Create WebSocket Instance
	if _, ok := r.Classes["WebSocket"]; !ok {
		fmt.Println("[WS] Error: WebSocket class not found in runtime")
		return
	}

	wsInstance := &Instance{
		Class: r.Classes["WebSocket"],
		Fields: map[string]interface{}{
			"_conn":   conn,
			"_sender": sender,
		},
	}
	lifecycle := &webSocketLifecycle{runtime: r, instance: wsInstance, reader: reader, closer: closer}
	wsInstance.Fields["_closer"] = lifecycle.close
	defer lifecycle.cleanup()

	// Execute Handler (Controller@Method)
	// This sets up the callbacks (onMessage, etc.)
	handlerExecuted := false
	if handlerName, ok := handler.(string); ok {
		parts := strings.Split(handlerName, "@")
		if len(parts) == 2 {
			controllerName := parts[0]
			methodName := parts[1]

			if classStmt, ok := r.Classes[controllerName]; ok {
				instance := &Instance{Class: classStmt, Fields: make(map[string]interface{})}
				for _, stmt := range classStmt.Body.Statements {
					if m, ok := stmt.(*parser.MethodStatement); ok {
						if m.Name.Value == methodName {
							fmt.Printf("[WS] Executing Setup %s@%s\n", controllerName, methodName)
							callArgs := append([]interface{}{wsInstance}, routeParams...)
							r.CallMethodEvaluated(m, instance, callArgs)
							handlerExecuted = true
							break // Found
						}
					}
				}
			}
		}
	}
	if fn, ok := handler.(*parser.FunctionLiteral); ok {
		method := &parser.MethodStatement{Token: fn.Token, Name: &parser.Identifier{Value: "anonymous"}, Parameters: fn.Parameters, ReturnType: fn.ReturnType, Body: fn.Body}
		callArgs := append([]interface{}{wsInstance}, routeParams...)
		r.CallMethodEvaluated(method, nil, callArgs)
		handlerExecuted = true
	}

	if !handlerExecuted {
		fmt.Println("[WS] Handler execution failed (method not found?)")
		return
	}

	// Blocking Event Loop
	fmt.Println("[WS] Starting Event Loop")
	lifecycle.readMessages()
}

func (r *Runtime) callWebSocketCallback(event string, callback interface{}, args []interface{}) (completed bool) {
	completed = true
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Printf("[WS] %s callback panic: %v\n", event, recovered)
			completed = false
		}
	}()
	r.CallFunction(callback, args)
	return completed
}

func matchRoutePattern(pattern, path string) ([]interface{}, bool) {
	if pattern == path {
		return nil, true
	}
	if !strings.Contains(pattern, "{") {
		return nil, false
	}
	placeholder := regexp.MustCompile(`\{[a-zA-Z_][a-zA-Z0-9_]*\}`)
	locations := placeholder.FindAllStringIndex(pattern, -1)
	if len(locations) == 0 {
		return nil, false
	}
	var expression strings.Builder
	expression.WriteString("^")
	last := 0
	for _, location := range locations {
		expression.WriteString(regexp.QuoteMeta(pattern[last:location[0]]))
		expression.WriteString("([^/]+)")
		last = location[1]
	}
	expression.WriteString(regexp.QuoteMeta(pattern[last:]))
	expression.WriteString("$")
	matches := regexp.MustCompile(expression.String()).FindStringSubmatch(path)
	if len(matches) != len(locations)+1 {
		return nil, false
	}
	params := make([]interface{}, 0, len(locations))
	for _, value := range matches[1:] {
		params = append(params, value)
	}
	return params, true
}
