package core

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/viewtemplate"
)

var (
	GlobalViewSharedData = make(map[string]interface{})
	globalViewSharedMu   sync.RWMutex
)

func resolveSafeViewSubpath(viewName string) (string, bool) {
	return viewtemplate.ResolveSafeSubpath(viewName)
}

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
		viewPath, safe := resolveSafeViewSubpath(viewName)
		if !safe {
			return false
		}
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
				globalViewSharedMu.Lock()
				GlobalViewSharedData[key] = args[1]
				globalViewSharedMu.Unlock()
				return true
			}
		} else if len(args) == 1 {
			if m, ok := args[0].(map[string]interface{}); ok {
				globalViewSharedMu.Lock()
				for k, v := range m {
					GlobalViewSharedData[k] = v
				}
				globalViewSharedMu.Unlock()
				return true
			}
		}
		return false
	}

	if method == "render" {
		if len(args) >= 1 {
			viewName, ok := args[0].(string)
			if !ok {
				return ""
			}
			data := make(map[string]interface{})
			// Inject globally shared data
			globalViewSharedMu.RLock()
			for k, v := range GlobalViewSharedData {
				data[k] = v
			}
			globalViewSharedMu.RUnlock()
			if len(args) > 1 {
				if d, ok := args[1].(map[string]interface{}); ok {
					for k, v := range d {
						data[k] = v
					}
				}
			}
			fmt.Printf("[View] Rendering %s\n", viewName)

			// Inject Global Auth & Flash Variables
			data["auth_check"] = false
			data["auth_guest"] = true
			data["auth_user"] = ""
			data["auth_role"] = ""
			data["auth_email"] = ""
			if _, ok := data["title"]; !ok {
				data["title"] = ""
			}
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
			viewPath, safeView := resolveSafeViewSubpath(viewName)
			if !safeView {
				return fmt.Sprintf("Error: Vista '%s' inválida", viewName)
			}

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

			readViewFile := func(name string) (string, bool) {
				subPath, safe := resolveSafeViewSubpath(name)
				if !safe {
					return "", false
				}
				if GlobalFileSystem != nil {
					p1 := path.Join("app", "views", subPath+".joss.html")
					if f, errOpen := GlobalFileSystem.Open(p1); errOpen == nil {
						stat, _ := f.Stat()
						buf := make([]byte, stat.Size())
						f.Read(buf)
						f.Close()
						return string(buf), true
					}
					p2 := path.Join("app", "views", subPath+".html")
					if f, errOpen := GlobalFileSystem.Open(p2); errOpen == nil {
						stat, _ := f.Stat()
						buf := make([]byte, stat.Size())
						f.Read(buf)
						f.Close()
						return string(buf), true
					}
					return "", false
				}
				p1 := filepath.Join("app", "views", subPath+".joss.html")
				if buf, errRead := os.ReadFile(p1); errRead == nil {
					return string(buf), true
				}
				p2 := filepath.Join("app", "views", subPath+".html")
				if buf, errRead := os.ReadFile(p2); errRead == nil {
					return string(buf), true
				}
				return "", false
			}

			finalHtml, err := viewtemplate.PreprocessDirectives(viewContent, readViewFile, viewtemplate.PreprocessOptions{
				FailOnMissingLayout:  true,
				AnnotateIncludeError: true,
			})
			if err != nil {
				if strings.HasPrefix(err.Error(), "Layout ") {
					if GlobalFileSystem != nil && strings.HasSuffix(err.Error(), "no encontrado") {
						return fmt.Sprintf("Error: %s (VFS)", err.Error())
					}
					return fmt.Sprintf("Error: %s", err.Error())
				}
				return fmt.Sprintf("Error compiling view directive: %v", err)
			}

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
					if r.topHasReturn {
						result = r.topReturnValue
						r.topHasReturn = false
						r.topReturnValue = nil
						break
					}
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

// CompileViewToJOSS translates HTML view to JOSS script via the canonical viewtemplate compiler.
func CompileViewToJOSS(htmlStr string) (string, error) {
	return viewtemplate.CompileHTMLToJoss(htmlStr)
}
