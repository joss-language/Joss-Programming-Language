package core

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
)

// Helper to evaluate an expression string within the current runtime context
func (r *Runtime) evaluateViewExpression(expr string, data map[string]interface{}) (result interface{}) {
	defer func() {
		if rec := recover(); rec != nil {
			result = ""
		}
	}()

	// Inject data into variables (support both raw key and $key)
	for k, v := range data {
		r.Variables[k] = v
		if !strings.HasPrefix(k, "$") {
			r.Variables["$"+k] = v
		} else {
			r.Variables[strings.TrimPrefix(k, "$")] = v
		}
	}

	l := parser.NewLexer(expr)
	p := parser.NewParser(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		return fmt.Sprintf("Error parsing view expression: %s | Details: %v", expr, p.Errors())
	}

	if len(program.Statements) == 0 {
		return ""
	}

	// Evaluate the statements safely
	for _, stmt := range program.Statements {
		if exprStmt, ok := stmt.(*parser.ExpressionStatement); ok {
			result = r.evaluateExpression(exprStmt.Expression)
		} else {
			result = r.executeStatement(stmt)
		}
	}
	return result
}

var (
	GlobalViewSharedData = make(map[string]interface{})
)

// View Implementation
func (r *Runtime) executeViewMethod(instance *Instance, method string, args []interface{}) interface{} {
	// Initialize AssetManager on first use
	am := GetAssetManager()
	am.Initialize()

	if method == "exists" {
		if len(args) == 0 {
			return false
		}
		viewName, ok := args[0].(string)
		if !ok {
			return false
		}
		viewPath := strings.ReplaceAll(viewName, ".", "/")
		if GlobalFileSystem != nil {
			p1 := path.Join("app", "views", viewPath+".joss.html")
			if _, err := GlobalFileSystem.Open(p1); err == nil {
				return true
			}
			p2 := path.Join("app", "views", viewPath+".html")
			if _, err := GlobalFileSystem.Open(p2); err == nil {
				return true
			}
		} else {
			p1 := filepath.Join("app", "views", viewPath+".joss.html")
			if _, err := os.Stat(p1); err == nil {
				return true
			}
			p2 := filepath.Join("app", "views", viewPath+".html")
			if _, err := os.Stat(p2); err == nil {
				return true
			}
		}
		return false
	}

	if method == "share" {
		if len(args) >= 2 {
			if key, ok := args[0].(string); ok {
				GlobalViewSharedData[key] = args[1]
				return true
			}
		} else if len(args) == 1 {
			if m, ok := args[0].(map[string]interface{}); ok {
				for k, v := range m {
					GlobalViewSharedData[k] = v
				}
				return true
			}
		}
		return false
	}

	if method == "render" {
		if len(args) >= 1 {
			viewName := args[0].(string)
			data := make(map[string]interface{})
			// Inject globally shared data
			for k, v := range GlobalViewSharedData {
				data[k] = v
			}
			if len(args) > 1 {
				if d, ok := args[1].(map[string]interface{}); ok {
					for k, v := range d {
						data[k] = v
					}
				}
			}
			fmt.Printf("[View] Rendering %s with data: %v\n", viewName, data)

			// Inject Global Auth & Flash Variables
			data["auth_check"] = false
			data["auth_guest"] = true
			data["auth_user"] = ""
			data["auth_role"] = ""
			data["auth_email"] = ""
			if _, ok := data["error"]; !ok {
				data["error"] = ""
			}
			if _, ok := data["success"]; !ok {
				data["success"] = ""
			}

			if sessVal, ok := r.Variables["$__session"]; ok {
				// fmt.Println("[View DEBUG] Found $__session")
				if sessInst, ok := sessVal.(*Instance); ok {
					// fmt.Printf("[View DEBUG] Session keys: %v\n", sessInst.Fields)
					if _, ok := sessInst.Fields["user_id"]; ok {
						data["auth_check"] = true
						data["auth_guest"] = false
						if name, ok := sessInst.Fields["user_name"]; ok {
							data["auth_user"] = name
						}
						if role, ok := sessInst.Fields["user_role"]; ok {
							data["auth_role"] = role
						}
						if email, ok := sessInst.Fields["user_email"]; ok {
							data["auth_email"] = email
						}
					}
					// Inject Flash Messages
					if errVal, ok := sessInst.Fields["error"]; ok {
						data["error"] = errVal
						delete(sessInst.Fields, "error")
					}
					if successVal, ok := sessInst.Fields["success"]; ok {
						data["success"] = successVal
						delete(sessInst.Fields, "success")
					}
					// Inject CSRF Token
					if csrfVal, ok := sessInst.Fields["csrf_token"]; ok {
						// fmt.Printf("[View DEBUG] Injecting CSRF token: %v\n", csrfVal)
						data["csrf_token"] = csrfVal
					} else {
						fmt.Println("[View DEBUG] CSRF token NOT FOUND in $__session fields")
						// Print all keys for debugging
						for k := range sessInst.Fields {
							fmt.Printf("[View DEBUG] Available Key: %s\n", k)
						}
					}
				} else {
					fmt.Println("[View DEBUG] $__session is not an Instance")
				}
			} else {
				fmt.Println("[View DEBUG] $__session variable NOT FOUND in Runtime")
			}

			// 1. Read View Content
			viewPath := strings.ReplaceAll(viewName, ".", "/")

			var content []byte
			var err error

			if GlobalFileSystem != nil {
				// VFS Mode
				// Use path.Join for forward slashes
				pathStr := path.Join("app", "views", viewPath+".joss.html")
				f, errOpen := GlobalFileSystem.Open(pathStr)
				if errOpen == nil {
					stat, _ := f.Stat()
					content = make([]byte, stat.Size())
					f.Read(content)
					f.Close()
				} else {
					// Try .html
					pathStr = path.Join("app", "views", viewPath+".html")
					f, errOpen = GlobalFileSystem.Open(pathStr)
					if errOpen == nil {
						stat, _ := f.Stat()
						content = make([]byte, stat.Size())
						f.Read(content)
						f.Close()
					} else {
						return fmt.Sprintf("Error: Vista '%s' no encontrada (VFS)", viewName)
					}
				}
			} else {
				// Disk Mode
				path := filepath.Join("app", "views", viewPath+".joss.html")
				content, err = os.ReadFile(path)
				if err != nil {
					path = filepath.Join("app", "views", viewPath+".html")
					content, err = os.ReadFile(path)
					if err != nil {
						return fmt.Sprintf("Error: Vista '%s' no encontrada", viewName)
					}
				}
			}

			viewContent := string(content)

			// 2. Handle Inheritance (@extends)
			var layoutContent string
			sections := make(map[string]string)

			if strings.HasPrefix(strings.TrimSpace(viewContent), "@extends") {
				// Extract layout name
				// Support both ' and " quotes
				reExtends := regexp.MustCompile(`@extends\s*\(\s*['"]([^'"]+)['"]\s*\)`)
				match := reExtends.FindStringSubmatch(viewContent)
				if len(match) > 1 {
					layoutName := match[1]
					layoutPath := strings.ReplaceAll(layoutName, ".", "/")

					if GlobalFileSystem != nil {
						// VFS Layout
						lPath := path.Join("app", "views", layoutPath+".joss.html")
						f, err := GlobalFileSystem.Open(lPath)
						if err == nil {
							stat, _ := f.Stat()
							lContent := make([]byte, stat.Size())
							f.Read(lContent)
							f.Close()
							layoutContent = string(lContent)
						} else {
							// Try .html
							lPath = path.Join("app", "views", layoutPath+".html")
							f, err = GlobalFileSystem.Open(lPath)
							if err == nil {
								stat, _ := f.Stat()
								lContent := make([]byte, stat.Size())
								f.Read(lContent)
								f.Close()
								layoutContent = string(lContent)
							} else {
								return fmt.Sprintf("Error: Layout '%s' no encontrado (VFS)", layoutName)
							}
						}
					} else {
						// Disk Layout
						lPath := filepath.Join("app", "views", layoutPath+".joss.html")
						lContent, err := os.ReadFile(lPath)
						if err == nil {
							layoutContent = string(lContent)
						} else {
							// Try .html
							lPath = filepath.Join("app", "views", layoutPath+".html")
							lContent, err = os.ReadFile(lPath)
							if err == nil {
								layoutContent = string(lContent)
							} else {
								return fmt.Sprintf("Error: Layout '%s' no encontrado", layoutName)
							}
						}
					}
				}

				// Extract Sections
				// @section('name') ... @endsection
				// Support both ' and " quotes
				reSection := regexp.MustCompile(`@section\s*\(\s*['"]([^'"]+)['"]\s*\)([\s\S]*?)@endsection`)
				sectionMatches := reSection.FindAllStringSubmatch(viewContent, -1)
				for _, sm := range sectionMatches {
					sections[sm[1]] = sm[2]
				}
			}

			// 3. Merge Layout and View
			finalHtml := viewContent
			if layoutContent != "" {
				finalHtml = layoutContent
				// Replace @yield('name') with section content
				for name, content := range sections {
					// Make placeholder regex-safe or try both quote types
					finalHtml = strings.ReplaceAll(finalHtml, fmt.Sprintf("@yield('%s')", name), content)
					finalHtml = strings.ReplaceAll(finalHtml, fmt.Sprintf("@yield(\"%s\")", name), content)
				}
				// Remove any remaining @yields
				reYield := regexp.MustCompile(`@yield\s*\(\s*['"][^'"]+['"]\s*\)`)
				finalHtml = reYield.ReplaceAllString(finalHtml, "")
			}

			// 3.4 Handle @include('view.name')
			reInclude := regexp.MustCompile(`@include\s*\(\s*['"]([^'"]+)['"]\s*\)`)
			for {
				match := reInclude.FindStringSubmatch(finalHtml)
				if match == nil {
					break
				}
				fullMatch := match[0]
				includeName := match[1]
				includePath := strings.ReplaceAll(includeName, ".", "/")
				var includeContent []byte

				// Resolve Path (reuse logic or simplify)
				// Note: We are repeating file reading logic here. In a full refactor we should extract 'readView(name)' helper.
				// For now, inline to be safe.

				if GlobalFileSystem != nil {
					// VFS
					iPath := path.Join("app", "views", includePath+".joss.html")
					f, err := GlobalFileSystem.Open(iPath)
					if err == nil {
						stat, _ := f.Stat()
						c := make([]byte, stat.Size())
						f.Read(c)
						f.Close()
						includeContent = c
					} else {
						// Try .html
						iPath = path.Join("app", "views", includePath+".html")
						f, err = GlobalFileSystem.Open(iPath)
						if err == nil {
							stat, _ := f.Stat()
							c := make([]byte, stat.Size())
							f.Read(c)
							f.Close()
							includeContent = c
						} else {
							includeContent = []byte(fmt.Sprintf("<!-- Error: Include '%s' not found -->", includeName))
						}
					}
				} else {
					// Disk
					iPath := filepath.Join("app", "views", includePath+".joss.html")
					c, err := os.ReadFile(iPath)
					if err == nil {
						includeContent = c
					} else {
						iPath = filepath.Join("app", "views", includePath+".html")
						c, err := os.ReadFile(iPath)
						if err == nil {
							includeContent = c
						} else {
							includeContent = []byte(fmt.Sprintf("<!-- Error: Include '%s' not found -->", includeName))
						}
					}
				}

				finalHtml = strings.Replace(finalHtml, fullMatch, string(includeContent), 1)
			}

			// Pre-process Blade comments: {{-- comment content --}}
			reBladeComments := regexp.MustCompile(`\{\{--[\s\S]*?--\}\}`)
			finalHtml = reBladeComments.ReplaceAllString(finalHtml, "")

			// Pre-process @json(expr) directive to raw {{! json_encode(expr) }}
			reJsonDirective := regexp.MustCompile(`@json\s*\((.*?)\)`)
			finalHtml = reJsonDirective.ReplaceAllString(finalHtml, `{{! json_encode($1) }}`)

			// Pre-process csrf_field() to be raw output
			reCsrfPre := regexp.MustCompile(`\{\{\s*csrf_field\(\)\s*\}\}`)
			finalHtml = reCsrfPre.ReplaceAllString(finalHtml, `{{! csrf_field() }}`)

			// Compile HTML to JOSS script
			jossScript, errCompile := CompileViewToJOSS(finalHtml)
			if errCompile != nil {
				return fmt.Sprintf("Error compiling view: %v", errCompile)
			}

			// Parse and execute JOSS script
			l := parser.NewLexer(jossScript)
			p := parser.NewParser(l)
			program := p.ParseProgram()

			if len(p.Errors()) > 0 {
				return fmt.Sprintf("Error parsing compiled view JOSS script: %v\nScript:\n%s", p.Errors(), jossScript)
			}

			// Inject variables from data map (support both raw key and $key)
			for k, v := range data {
				r.Variables[k] = v
				if !strings.HasPrefix(k, "$") {
					r.Variables["$"+k] = v
				} else {
					r.Variables[strings.TrimPrefix(k, "$")] = v
				}
			}

			var result interface{}
			func() {
				previousFrame := r.currentFrame
				r.currentFrame = nil
				defer func() { r.currentFrame = previousFrame }()
				defer func() {
					if rec := recover(); rec != nil {
						if rp, ok := rec.(*ReturnPanic); ok {
							result = rp.Value
						} else {
							if jErr, ok := rec.(*JossError); ok {
								jErr.Message = fmt.Sprintf("Error en vista '%s': %s", viewName, jErr.Message)
							}
							panic(rec) // Bubble up database or other execution panics
						}
					}
				}()

				for _, stmt := range program.Statements {
					r.executeStatement(stmt)
				}
			}()

			finalHtml = ""
			if result != nil {
				finalHtml = fmt.Sprintf("%v", result)
			}

			// 5. Asset Injection (Node Modules Only)
			vendorCSS, vendorJS := am.GetVendorIncludes()

			// REVERT: Dynamic CSS removed by user request.
			// System relies on static <link href="/public/css/app.css"> in layout.

			// Inject CSS (Vendor Only)
			if strings.Contains(finalHtml, "<!-- JOSS_ASSETS -->") {
				// Custom placeholder
				finalHtml = strings.Replace(finalHtml, "<!-- JOSS_ASSETS -->", vendorCSS, 1)
			} else if strings.Contains(finalHtml, "</head>") {
				// Inject before head close
				finalHtml = strings.Replace(finalHtml, "</head>", vendorCSS+"</head>", 1)
			} else {
				// Just prepend
				finalHtml = vendorCSS + finalHtml
			}

			// Inject JS
			if strings.Contains(finalHtml, "</body>") {
				finalHtml = strings.Replace(finalHtml, "</body>", vendorJS+"</body>", 1)
			} else {
				finalHtml = finalHtml + vendorJS
			}

			return finalHtml
		}
	}
	return nil
}

// CompileViewToJOSS translates HTML view to JOSS script
func CompileViewToJOSS(htmlStr string) (string, error) {
	var jossCode strings.Builder

	escapeJossString := func(s string) string {
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		s = strings.ReplaceAll(s, "\n", "\\n")
		s = strings.ReplaceAll(s, "\r", "\\r")
		s = strings.ReplaceAll(s, "${", "\\${")
		return s
	}

	reForeachStart := regexp.MustCompile(`^@foreach\s*\(\s*(.*?)\s+as\s+\$([a-zA-Z0-9_]+)\s*\)`)
	reExprRaw := regexp.MustCompile(`^\{\{!(.*?)\}\}`)
	reExprEscaped := regexp.MustCompile(`^\{\{(.*?)\}\}`)

	translateExpr := func(expr string) string {
		reDot := regexp.MustCompile(`\$([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)`)
		for reDot.MatchString(expr) {
			expr = reDot.ReplaceAllString(expr, "$$$1->$2")
		}
		return expr
	}

	var compileRange func(str string) string
	compileRange = func(str string) string {
		var code strings.Builder
		i := 0
		n := len(str)

		flushText := func(start, end int) {
			if start >= end {
				return
			}
			txt := str[start:end]
			code.WriteString(fmt.Sprintf("$__output = $__output . \"%s\";\n", escapeJossString(txt)))
		}

		lastIdx := 0

		for i < n {
			// 1. Check @foreach
			if strings.HasPrefix(str[i:], "@foreach") {
				if m := reForeachStart.FindStringSubmatch(str[i:]); m != nil {
					flushText(lastIdx, i)

					listExpr := translateExpr(m[1])
					itemVar := m[2]
					fullMatchLen := len(m[0])

					depth := 1
					j := i + fullMatchLen
					endIdx := -1
					for j < n {
						if strings.HasPrefix(str[j:], "@foreach") {
							depth++
							j += 8
						} else if strings.HasPrefix(str[j:], "@endforeach") {
							depth--
							if depth == 0 {
								endIdx = j
								break
							}
							j += 11
						} else {
							j++
						}
					}

					if endIdx != -1 {
						innerHtml := str[i+fullMatchLen : endIdx]
						compiledInner := compileRange(innerHtml)

						code.WriteString(fmt.Sprintf("foreach (%s as $%s) {\n", listExpr, itemVar))
						code.WriteString(compiledInner)
						code.WriteString("}\n")

						i = endIdx + 11
						lastIdx = i
						continue
					}
				}
			}

			// 2. Check Block Ternary
			if i+4 < n && str[i] == '{' && str[i+1] == '{' {
				k := i + 2
				for k < n && (str[k] == ' ' || str[k] == '\t' || str[k] == '\n' || str[k] == '\r') {
					k++
				}
				if k < n && str[k] == '(' {
					condStart := k + 1
					condEnd := findMatchingPair(str, k, '(', ')')
					if condEnd != -1 {
						q := condEnd + 1
						for q < n && (str[q] == ' ' || str[q] == '\t' || str[q] == '\n' || str[q] == '\r') {
							q++
						}
						if q < n && str[q] == '?' {
							tbStart := q + 1
							for tbStart < n && (str[tbStart] == ' ' || str[tbStart] == '\t' || str[tbStart] == '\n' || str[tbStart] == '\r') {
								tbStart++
							}
							if tbStart < n && str[tbStart] == '{' {
								tbEnd := findMatchingPair(str, tbStart, '{', '}')
								if tbEnd != -1 {
									col := tbEnd + 1
									for col < n && (str[col] == ' ' || str[col] == '\t' || str[col] == '\n' || str[col] == '\r') {
										col++
									}
									if col < n && str[col] == ':' {
										fbStart := col + 1
										for fbStart < n && (str[fbStart] == ' ' || str[fbStart] == '\t' || str[fbStart] == '\n' || str[fbStart] == '\r') {
											fbStart++
										}
										if fbStart < n && str[fbStart] == '{' {
											fbEnd := findMatchingPair(str, fbStart, '{', '}')
											if fbEnd != -1 {
												closeBraces := fbEnd + 1
												for closeBraces < n && (str[closeBraces] == ' ' || str[closeBraces] == '\t' || str[closeBraces] == '\n' || str[closeBraces] == '\r') {
													closeBraces++
												}
												if closeBraces+1 < n && str[closeBraces] == '}' && str[closeBraces+1] == '}' {
													flushText(lastIdx, i)

													condExpr := str[condStart:condEnd]
													trueBody := str[tbStart+1 : tbEnd]
													falseBody := str[fbStart+1 : fbEnd]

													code.WriteString(fmt.Sprintf("(%s) ? {\n", translateExpr(condExpr)))
													code.WriteString(compileRange(trueBody))
													code.WriteString("} : {\n")
													code.WriteString(compileRange(falseBody))
													code.WriteString("};\n")

													i = closeBraces + 2
													lastIdx = i
													continue
												}
											}
										}
									} else if col+1 < n && str[col] == '}' && str[col+1] == '}' {
										flushText(lastIdx, i)

										condExpr := str[condStart:condEnd]
										trueBody := str[tbStart+1 : tbEnd]

										code.WriteString(fmt.Sprintf("(%s) ? {\n", translateExpr(condExpr)))
										code.WriteString(compileRange(trueBody))
										code.WriteString("};\n")

										i = col + 2
										lastIdx = i
										continue
									}
								}
							}
						}
					}
				}
			}

			// 3. Check Raw Expression: {{! expr }}
			if strings.HasPrefix(str[i:], "{{!") {
				if m := reExprRaw.FindStringSubmatch(str[i:]); m != nil {
					flushText(lastIdx, i)

					expr := strings.TrimSpace(m[1])
					fullMatchLen := len(m[0])

					code.WriteString(fmt.Sprintf("$__output = $__output . (%s);\n", translateExpr(expr)))

					i += fullMatchLen
					lastIdx = i
					continue
				}
			}

			// 4. Check Escaped Expression: {{ expr }}
			if strings.HasPrefix(str[i:], "{{") {
				if m := reExprEscaped.FindStringSubmatch(str[i:]); m != nil {
					flushText(lastIdx, i)

					expr := strings.TrimSpace(m[1])
					fullMatchLen := len(m[0])

					code.WriteString(fmt.Sprintf("$__output = $__output . html_escape(%s);\n", translateExpr(expr)))

					i += fullMatchLen
					lastIdx = i
					continue
				}
			}

			i++
		}

		flushText(lastIdx, n)
		return code.String()
	}

	jossCode.WriteString("let string $__output = \"\";\n")
	jossCode.WriteString(compileRange(htmlStr))
	jossCode.WriteString("return $__output;\n")

	return jossCode.String(), nil
}

func findMatchingPair(s string, startPos int, startChar, endChar rune) int {
	depth := 0
	for idx, r := range s[startPos:] {
		if r == startChar {
			depth++
		} else if r == endChar {
			depth--
			if depth == 0 {
				return startPos + idx
			}
		}
	}
	return -1
}
