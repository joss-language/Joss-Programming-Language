package ir

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jossecurity/joss/pkg/analyzer"
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
	if l.reachGraph != nil && !l.reachGraph.HasCapability(analyzer.CapServer) && !l.reachGraph.HasCapability(analyzer.CapHTTP) {
		return nil
	}
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
		relPath, safe := viewtemplate.ResolveSafeSubpath(viewName)
		if !safe {
			return "", false
		}
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
		dotName := strings.ReplaceAll(rel, "/", ".")
		viewContent := rawViews[rel]

		finalHTML, errPre := viewtemplate.PreprocessDirectives(viewContent, readViewFile, viewtemplate.PreprocessOptions{})
		if errPre != nil {
			finalHTML = viewContent
		}

		// Translate HTML view to Joss source code and parse into AST via the shared compiler
		jossScript, _ := viewtemplate.CompileHTMLToJoss(finalHTML)
		lex := parser.NewLexer(jossScript)
		p := parser.NewParser(lex)
		program := p.ParseProgram()

		fnName := fmt.Sprintf("__joss_view_%d", idx)
		if err := l.lowerSingleViewAST(fnName, program, len(p.Errors()) == 0, finalHTML); err != nil {
			return nil, err
		}

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
		targetFnName := rt.controller + "_" + rt.action
		targetFn, exists := l.prog.Functions[targetFnName]
		if !exists || len(targetFn.Params) < 1 {
			continue
		}

		numPlaceholders := strings.Count(rt.path, "{")
		expectedParams := len(targetFn.Params) - 1
		if numPlaceholders > expectedParams {
			continue
		}

		matchBlk := fn.NewBlock(fmt.Sprintf("route_match_%d", idx))
		nextBlk := fn.NewBlock(fmt.Sprintf("route_next_%d", idx))

		cmpTemp := fn.NewTemp(TypeBool, fmt.Sprintf("rcmp_%d", idx))
		if numPlaceholders == 0 {
			curr.AddInstruction(&CompareInst{
				Op:    OpCmpEq,
				Dest:  cmpTemp,
				Left:  pathParam,
				Right: NewConstString(rt.path),
			})
		} else {
			curr.AddInstruction(&CallInst{
				Dest:   cmpTemp,
				Callee: "__joss_rt_route_matches",
				Args:   []Value{pathParam, NewConstString(rt.path)},
			})
		}
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

		callArgs := []Value{ctrlTemp}
		for pIdx := 0; pIdx < numPlaceholders; pIdx++ {
			pTemp := fn.NewTemp(TypeString, fmt.Sprintf("parg_%d_%d", idx, pIdx))
			matchBlk.AddInstruction(&CallInst{
				Dest:   pTemp,
				Callee: "__joss_rt_route_param",
				Args:   []Value{pathParam, NewConstString(rt.path), NewConstInt(int64(pIdx), TypeI64)},
			})
			callArgs = append(callArgs, pTemp)
		}
		for pIdx := numPlaceholders; pIdx < expectedParams; pIdx++ {
			callArgs = append(callArgs, NewConstString(""))
		}

		retTemp := fn.NewTemp(TypeString, fmt.Sprintf("rret_%d", idx))
		matchBlk.AddInstruction(&CallInst{
			Dest:   retTemp,
			Callee: targetFnName,
			Args:   callArgs,
		})
		matchBlk.SetTerminator(&ReturnTerminator{Val: retTemp})

		curr = nextBlk
		dispatched++
	}

	if dispatched == 0 {
		return nil
	}

	curr.SetTerminator(&ReturnTerminator{
		Val: NewConstString("<!DOCTYPE html><html lang=\"es\"><head><meta charset=\"utf-8\"><title>404 Not Found</title></head><body><h1>404 No Encontrado</h1><p>La página solicitada no existe.</p></body></html>"),
	})

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
