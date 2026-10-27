package ir

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/viewtemplate"
)

type compiledViewEntry struct {
	dotName   string
	slashName string
	fnName    string
}

// lowerProjectViewsAndRoutes scans app/views (if present in the working directory)
// and routes.joss / api.joss in l.prep.Units to synthesize:
//  1. __joss_view_<idx>(__data): compiled native IR functions for each .joss.html template
//  2. view(viewName, dataMap): dispatches viewName to the compiled view IR function
//  3. __joss_http_dispatch(path): dispatches HTTP request paths to compiled native Controller_method functions
func (l *Lowerer) lowerProjectViewsAndRoutes() error {
	views, err := l.compileProjectViewsToIR("app/views")
	if err != nil {
		return err
	}
	if err := l.synthesizeViewDispatcherFunction(views); err != nil {
		return err
	}
	if err := l.synthesizeHTTPDispatchFunction(); err != nil {
		return err
	}
	return nil
}

// compileProjectViewsToIR walks app/views, preprocesses @extends/@section/@yield/@include/@json,
// translates each template to Joss source code via compileViewHTMLToJossScript, parses it into
// a *parser.Program AST, and lowers it into a native IR function receiving $__data.
func (l *Lowerer) compileProjectViewsToIR(viewsRoot string) ([]compiledViewEntry, error) {
	info, err := os.Stat(viewsRoot)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	readViewFile := func(viewName string) (string, bool) {
		relPath := strings.ReplaceAll(strings.TrimSpace(viewName), ".", "/")
		p1 := filepath.Join(viewsRoot, filepath.FromSlash(relPath)+".joss.html")
		if data, err := os.ReadFile(p1); err == nil {
			return string(data), true
		}
		p2 := filepath.Join(viewsRoot, filepath.FromSlash(relPath)+".html")
		if data, err := os.ReadFile(p2); err == nil {
			return string(data), true
		}
		return "", false
	}

	reExtends := regexp.MustCompile(`@extends\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	reSection := regexp.MustCompile(`@section\s*\(\s*['"]([^'"]+)['"]\s*\)([\s\S]*?)@endsection`)
	reYield := regexp.MustCompile(`@yield\s*\(\s*['"][^'"]+['"]\s*\)`)
	reInclude := regexp.MustCompile(`@include\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	reBladeComments := regexp.MustCompile(`\{\{--[\s\S]*?--\}\}`)
	reCsrfPre := regexp.MustCompile(`\{\{\s*csrf_field\(\)\s*\}\}`)

	var relPaths []string
	rawViews := make(map[string]string)

	_ = filepath.Walk(viewsRoot, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil || fi == nil || fi.IsDir() {
			return nil
		}
		slashPath := filepath.ToSlash(path)
		var rel string
		if strings.HasSuffix(slashPath, ".joss.html") {
			rel = strings.TrimSuffix(strings.TrimPrefix(slashPath, filepath.ToSlash(viewsRoot)+"/"), ".joss.html")
		} else if strings.HasSuffix(slashPath, ".html") {
			rel = strings.TrimSuffix(strings.TrimPrefix(slashPath, filepath.ToSlash(viewsRoot)+"/"), ".html")
		} else {
			return nil
		}
		if strings.HasPrefix(rel, "layouts/") {
			return nil
		}
		rawBytes, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		relPaths = append(relPaths, rel)
		rawViews[rel] = string(rawBytes)
		return nil
	})

	sort.Strings(relPaths)

	var entries []compiledViewEntry
	for idx, rel := range relPaths {
		viewContent := rawViews[rel]

		// 1. Handle @extends and @section -> @yield
		finalHTML := viewContent
		if strings.HasPrefix(strings.TrimSpace(viewContent), "@extends") {
			if m := reExtends.FindStringSubmatch(viewContent); len(m) > 1 {
				if layoutContent, ok := readViewFile(m[1]); ok {
					sections := make(map[string]string)
					for _, sm := range reSection.FindAllStringSubmatch(viewContent, -1) {
						sections[sm[1]] = sm[2]
					}
					finalHTML = layoutContent
					for secName, secBody := range sections {
						finalHTML = strings.ReplaceAll(finalHTML, fmt.Sprintf("@yield('%s')", secName), secBody)
						finalHTML = strings.ReplaceAll(finalHTML, fmt.Sprintf("@yield(\"%s\")", secName), secBody)
					}
					finalHTML = reYield.ReplaceAllString(finalHTML, "")
				}
			}
		}

		// 2. Expand @include directives
		for depth := 0; depth < 8; depth++ {
			m := reInclude.FindStringSubmatch(finalHTML)
			if m == nil {
				break
			}
			incContent, ok := readViewFile(m[1])
			if !ok {
				incContent = ""
			}
			finalHTML = strings.Replace(finalHTML, m[0], incContent, 1)
		}

		// 3. Strip Blade comments {{-- ... --}}
		finalHTML = reBladeComments.ReplaceAllString(finalHTML, "")

		// 4. Shared directive rewrite (@json)
		if rewritten, err := viewtemplate.RewriteJSON(finalHTML); err == nil {
			finalHTML = rewritten
		}

		// 5. csrf_field() outputs raw HTML input
		finalHTML = reCsrfPre.ReplaceAllString(finalHTML, `{{! csrf_field() }}`)

		// 6. Translate HTML view to Joss source code and parse into AST
		jossScript := compileViewHTMLToJossScript(finalHTML)
		lex := parser.NewLexer(jossScript)
		p := parser.NewParser(lex)
		program := p.ParseProgram()

		fnName := fmt.Sprintf("__joss_view_%d", idx)
		if err := l.lowerSingleViewAST(fnName, program, len(p.Errors()) == 0, finalHTML); err != nil {
			return nil, err
		}

		dotName := strings.ReplaceAll(rel, "/", ".")
		entries = append(entries, compiledViewEntry{
			dotName:   dotName,
			slashName: rel,
			fnName:    fnName,
		})
	}

	return entries, nil
}

func (l *Lowerer) lowerSingleViewAST(fnName string, program *parser.Program, parsedOK bool, fallbackHTML string) error {
	fn := NewFunction(fnName, TypeString)
	dataParam := fn.AddParam("__data", PtrType(StructType("map", nil)))

	savedFn := l.currentFn
	savedBlock := l.currBlock
	savedLocals := l.varLocals
	savedLoopStack := l.loopStack
	savedViewData := l.viewDataParam

	l.currentFn = fn
	l.currBlock = fn.EntryBlock
	l.varLocals = make(map[string]Value)
	l.loopStack = nil
	l.viewDataParam = dataParam

	defer func() {
		l.currentFn = savedFn
		l.currBlock = savedBlock
		l.varLocals = savedLocals
		l.loopStack = savedLoopStack
		l.viewDataParam = savedViewData
	}()

	if parsedOK && program != nil {
		for _, stmt := range program.Statements {
			if err := l.lowerStatement(stmt); err != nil {
				// Fallback if a statement inside a view uses an unsupported construct
				l.currBlock.SetTerminator(&ReturnTerminator{Val: NewConstString(fallbackHTML)})
				l.prog.AddFunction(fn)
				return nil
			}
		}
	}

	if l.currBlock.Terminator == nil {
		if ptr, ok := l.varLocals["__output"]; ok {
			outVal := fn.NewTemp(TypeString, "view_out")
			l.currBlock.AddInstruction(&LoadInst{Dest: outVal, Ptr: ptr})
			l.currBlock.SetTerminator(&ReturnTerminator{Val: outVal})
		} else {
			l.currBlock.SetTerminator(&ReturnTerminator{Val: NewConstString(fallbackHTML)})
		}
	}

	l.prog.AddFunction(fn)
	return nil
}

// compileViewHTMLToJossScript translates a preprocessed .joss.html template into Joss source code,
// matching pkg/core.CompileViewToJOSS.
func compileViewHTMLToJossScript(htmlStr string) string {
	var jossCode strings.Builder

	escapeJossString := func(s string) string {
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		s = strings.ReplaceAll(s, "\n", "\\n")
		s = strings.ReplaceAll(s, "\r", "\\r")
		s = strings.ReplaceAll(s, "${", "\\${")
		return s
	}

	reForeachStart := regexp.MustCompile(`^@foreach\s*\(\s*(.*?)\s+as\s+\$([a-zA-Z0-9_]+)(?:\s*=>\s*\$([a-zA-Z0-9_]+))?\s*\)`)
	reExprRaw := regexp.MustCompile(`(?s)^\{\{!(.*?)\}\}`)
	reExprEscaped := regexp.MustCompile(`(?s)^\{\{(.*?)\}\}`)
	reDot := regexp.MustCompile(`\$([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)`)

	translateExpr := func(expr string) string {
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
			// 1. @foreach
			if strings.HasPrefix(str[i:], "@foreach") {
				if m := reForeachStart.FindStringSubmatch(str[i:]); m != nil {
					flushText(lastIdx, i)

					listExpr := translateExpr(m[1])
					firstVar := m[2]
					secondVar := m[3]
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
						innerHTML := str[i+fullMatchLen : endIdx]
						compiledInner := compileRange(innerHTML)

						if secondVar != "" {
							code.WriteString(fmt.Sprintf("foreach (%s as $%s => $%s) {\n", listExpr, firstVar, secondVar))
						} else {
							code.WriteString(fmt.Sprintf("foreach (%s as $%s) {\n", listExpr, firstVar))
						}
						code.WriteString(compiledInner)
						code.WriteString("}\n")

						i = endIdx + 11
						lastIdx = i
						continue
					}
				}
			}

			// 2. Block Ternary: {{ (cond) ? { trueBody } : { falseBody } }} or {{ (cond) ? { trueBody } }}
			if i+4 < n && str[i] == '{' && str[i+1] == '{' {
				k := i + 2
				for k < n && (str[k] == ' ' || str[k] == '\t' || str[k] == '\n' || str[k] == '\r') {
					k++
				}
				if k < n && str[k] == '(' {
					condStart := k + 1
					condEnd := findMatchingRunePair(str, k, '(', ')')
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
								tbEnd := findMatchingRunePair(str, tbStart, '{', '}')
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
											fbEnd := findMatchingRunePair(str, fbStart, '{', '}')
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

			// 3. Raw Expression: {{! expr }}
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

			// 4. Escaped Expression: {{ expr }}
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

	jossCode.WriteString("var $__output = \"\";\n")
	jossCode.WriteString(compileRange(htmlStr))
	jossCode.WriteString("return $__output;\n")
	return jossCode.String()
}

func findMatchingRunePair(s string, startPos int, startChar, endChar rune) int {
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

// synthesizeViewDispatcherFunction creates the IR function `view(name, data)` (`fn_view`)
// which dispatches `name` to the compiled view IR function `__joss_view_<idx>(data)`.
func (l *Lowerer) synthesizeViewDispatcherFunction(views []compiledViewEntry) error {
	if len(views) == 0 || l.prog.Functions["view"] != nil {
		return nil
	}

	fn := NewFunction("view", TypeString)
	nameParam := fn.AddParam("name", TypeString)
	dataParam := fn.AddParam("data", PtrType(StructType("map", nil)))

	curr := fn.EntryBlock

	for idx, v := range views {
		names := []string{v.dotName}
		if v.slashName != v.dotName {
			names = append(names, v.slashName)
		}
		for subIdx, candidateName := range names {
			matchBlk := fn.NewBlock(fmt.Sprintf("view_match_%d_%d", idx, subIdx))
			nextBlk := fn.NewBlock(fmt.Sprintf("view_next_%d_%d", idx, subIdx))

			cmpTemp := fn.NewTemp(TypeBool, fmt.Sprintf("vcmp_%d_%d", idx, subIdx))
			curr.AddInstruction(&CompareInst{
				Op:    OpCmpEq,
				Dest:  cmpTemp,
				Left:  nameParam,
				Right: NewConstString(candidateName),
			})
			curr.SetTerminator(&BranchTerminator{
				Cond:       cmpTemp,
				TrueBlock:  matchBlk,
				FalseBlock: nextBlk,
			})

			resTemp := fn.NewTemp(TypeString, fmt.Sprintf("vres_%d_%d", idx, subIdx))
			matchBlk.AddInstruction(&CallInst{
				Dest:   resTemp,
				Callee: v.fnName,
				Args:   []Value{dataParam},
			})
			matchBlk.SetTerminator(&ReturnTerminator{Val: resTemp})
			curr = nextBlk
		}
	}

	curr.SetTerminator(&ReturnTerminator{
		Val: NewConstString("<!DOCTYPE html><html><body><h1>Vista no encontrada</h1></body></html>"),
	})

	l.prog.AddFunction(fn)
	return nil
}

type compiledRouteEntry struct {
	method     string
	path       string
	controller string
	action     string
}

// synthesizeHTTPDispatchFunction inspects routes.joss / api.joss AST in l.prep.Units,
// extracts Router::get / Router::post / Router::match registrations, and synthesizes
// the IR function `__joss_http_dispatch(path)` which instantiates the target controller
// and calls its compiled native method (`Controller_action`).
func (l *Lowerer) synthesizeHTTPDispatchFunction() error {
	if l.prog.Functions["__joss_http_dispatch"] != nil {
		return nil
	}

	routes := l.extractProjectRoutesFromAST()
	if len(routes) == 0 {
		return nil
	}

	fn := NewFunction("__joss_http_dispatch", TypeString)
	pathParam := fn.AddParam("path", TypeString)

	curr := fn.EntryBlock
	dispatched := 0

	for idx, rt := range routes {
		if rt.method != "GET" && rt.method != "ANY" {
			continue
		}
		if strings.Contains(rt.path, "{") {
			continue
		}
		targetFnName := rt.controller + "_" + rt.action
		targetFn, exists := l.prog.Functions[targetFnName]
		if !exists || len(targetFn.Params) != 1 {
			continue
		}

		matchBlk := fn.NewBlock(fmt.Sprintf("route_match_%d", idx))
		nextBlk := fn.NewBlock(fmt.Sprintf("route_next_%d", idx))

		cmpTemp := fn.NewTemp(TypeBool, fmt.Sprintf("rcmp_%d", idx))
		curr.AddInstruction(&CompareInst{
			Op:    OpCmpEq,
			Dest:  cmpTemp,
			Left:  pathParam,
			Right: NewConstString(rt.path),
		})
		curr.SetTerminator(&BranchTerminator{
			Cond:       cmpTemp,
			TrueBlock:  matchBlk,
			FalseBlock: nextBlk,
		})

		objType := PtrType(StructType(rt.controller, nil))
		ctrlTemp := fn.NewTemp(objType, fmt.Sprintf("ctrl_%d", idx))
		matchBlk.AddInstruction(&NewObjectInst{
			Dest:      ctrlTemp,
			ClassName: rt.controller,
		})

		retTemp := fn.NewTemp(TypeString, fmt.Sprintf("rret_%d", idx))
		matchBlk.AddInstruction(&CallInst{
			Dest:   retTemp,
			Callee: targetFnName,
			Args:   []Value{ctrlTemp},
		})
		matchBlk.SetTerminator(&ReturnTerminator{Val: retTemp})

		curr = nextBlk
		dispatched++
	}

	if dispatched == 0 {
		return nil
	}

	if homeFn, ok := l.prog.Functions["HomeController_index"]; ok && len(homeFn.Params) == 1 {
		objType := PtrType(StructType("HomeController", nil))
		ctrlTemp := fn.NewTemp(objType, "ctrl_default")
		curr.AddInstruction(&NewObjectInst{
			Dest:      ctrlTemp,
			ClassName: "HomeController",
		})
		retTemp := fn.NewTemp(TypeString, "rret_default")
		curr.AddInstruction(&CallInst{
			Dest:   retTemp,
			Callee: "HomeController_index",
			Args:   []Value{ctrlTemp},
		})
		curr.SetTerminator(&ReturnTerminator{Val: retTemp})
	} else {
		curr.SetTerminator(&ReturnTerminator{
			Val: NewConstString("<!DOCTYPE html><html><body><h1>404 Not Found</h1></body></html>"),
		})
	}

	l.prog.AddFunction(fn)
	return nil
}

func (l *Lowerer) extractProjectRoutesFromAST() []compiledRouteEntry {
	var routes []compiledRouteEntry
	if l.prep == nil {
		return routes
	}

	var walkStmts func(stmts []parser.Statement)
	walkStmts = func(stmts []parser.Statement) {
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case *parser.ExpressionStatement:
				if call, ok := s.Expression.(*parser.CallExpression); ok {
					routes = append(routes, extractRouteFromCallExpr(call, walkStmts)...)
				}
			case *parser.BlockStatement:
				walkStmts(s.Statements)
			}
		}
	}

	for _, unit := range l.prep.Units {
		if unit.Program != nil {
			walkStmts(unit.Program.Statements)
		}
	}
	return routes
}

func extractRouteFromCallExpr(call *parser.CallExpression, walkStmts func([]parser.Statement)) []compiledRouteEntry {
	var out []compiledRouteEntry
	member, ok := call.Function.(*parser.MemberExpression)
	if !ok || member.Property == nil {
		return out
	}
	ident, ok := member.Left.(*parser.Identifier)
	if !ok || ident.Value != "Router" {
		return out
	}

	method := member.Property.Value
	switch method {
	case "group":
		for _, arg := range call.Arguments {
			if fnLit, ok := arg.(*parser.FunctionLiteral); ok && fnLit.Body != nil {
				walkStmts(fnLit.Body.Statements)
			}
		}
	case "get", "post":
		if len(call.Arguments) >= 2 {
			pathLit, ok1 := call.Arguments[0].(*parser.StringLiteral)
			targetLit, ok2 := call.Arguments[1].(*parser.StringLiteral)
			if ok1 && ok2 && strings.Contains(targetLit.Value, "@") {
				parts := strings.Split(targetLit.Value, "@")
				if len(parts) >= 2 {
					out = append(out, compiledRouteEntry{
						method:     strings.ToUpper(method),
						path:       pathLit.Value,
						controller: parts[0],
						action:     parts[1],
					})
				}
			}
		}
	case "match":
		if len(call.Arguments) >= 3 {
			methodsLit, _ := call.Arguments[0].(*parser.StringLiteral)
			pathLit, ok1 := call.Arguments[1].(*parser.StringLiteral)
			targetLit, ok2 := call.Arguments[2].(*parser.StringLiteral)
			if ok1 && ok2 && strings.Contains(targetLit.Value, "@") {
				parts := strings.Split(targetLit.Value, "@")
				if len(parts) == 2 {
					out = append(out, compiledRouteEntry{
						method:     "ANY",
						path:       pathLit.Value,
						controller: parts[0],
						action:     parts[1],
					})
				} else if len(parts) > 2 {
					controller := parts[0]
					actions := parts[1:]
					methods := []string{"ANY"}
					if methodsLit != nil && methodsLit.Value != "" {
						methods = strings.Split(methodsLit.Value, "|")
					}
					for i, action := range actions {
						m := "ANY"
						if i < len(methods) {
							m = strings.ToUpper(strings.TrimSpace(methods[i]))
						}
						out = append(out, compiledRouteEntry{
							method:     m,
							path:       pathLit.Value,
							controller: controller,
							action:     action,
						})
					}
				}
			}
		}
	}
	return out
}
