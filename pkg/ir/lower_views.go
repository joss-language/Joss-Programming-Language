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
//  1. __joss_http_dispatch(path): dispatches HTTP request paths to compiled native Controller_method functions
//  2. __joss_view_<idx>(__data): compiled native IR functions for each reachable .joss.html template
//  3. view(viewName, dataMap): dispatches viewName to the compiled view IR function
func (l *Lowerer) lowerProjectViewsAndRoutes() error {
	if l.reachGraph != nil && !l.reachGraph.HasCapability(analyzer.CapServer) && !l.reachGraph.HasCapability(analyzer.CapHTTP) {
		return nil
	}
	if err := l.synthesizeHTTPDispatchFunction(); err != nil {
		return err
	}
	views, err := l.compileProjectViewsToIR("app/views")
	if err != nil {
		return err
	}
	if err := l.synthesizeViewDispatcherFunction(views); err != nil {
		return err
	}
	if err := l.synthesizeEmbeddedAssetsFunction(); err != nil {
		return err
	}
	return nil
}

// collectReachableViewNames inspects all functions reachable from `main` and `__joss_http_dispatch`
// in l.prog.Functions and extracts the string literal arguments passed to `view(...)` or `View::render(...)`.
// If any reachable call to `view` passes a non-literal dynamic name, it returns (nil, true) so all views are kept.
func (l *Lowerer) collectReachableViewNames() (map[string]bool, bool) {
	if l.prog == nil || len(l.prog.Functions) == 0 {
		return nil, true
	}

	reachableFns := make(map[string]bool)
	var worklist []string
	if l.prog.Functions["main"] != nil {
		reachableFns["main"] = true
		worklist = append(worklist, "main")
	}
	if l.prog.Functions["__joss_http_dispatch"] != nil {
		reachableFns["__joss_http_dispatch"] = true
		worklist = append(worklist, "__joss_http_dispatch")
	}
	if len(worklist) == 0 {
		for fnName := range l.prog.Functions {
			reachableFns[fnName] = true
			worklist = append(worklist, fnName)
		}
	}

	usedViews := make(map[string]bool)
	hasDynamicViewCall := false

	recordViewArg := func(args []Value) {
		if len(args) == 0 {
			return
		}
		if cStr, ok := args[0].(*ConstString); ok && cStr.Val != "" {
			norm := strings.ReplaceAll(strings.TrimSpace(cStr.Val), ".", "/")
			usedViews[norm] = true
			usedViews[strings.TrimSpace(cStr.Val)] = true
		} else {
			hasDynamicViewCall = true
		}
	}

	var scanBlocks func(blocks []*BasicBlock)
	scanBlocks = func(blocks []*BasicBlock) {
		for _, blk := range blocks {
			for _, inst := range blk.Instructions {
				switch i := inst.(type) {
				case *CallInst:
					if i.Callee == "view" || i.Callee == "View_render" {
						recordViewArg(i.Args)
					} else if l.prog.Functions[i.Callee] != nil && !reachableFns[i.Callee] {
						reachableFns[i.Callee] = true
						worklist = append(worklist, i.Callee)
					}
					for _, arg := range i.Args {
						if fp, ok := arg.(*ConstFnPtr); ok && l.prog.Functions[fp.FnName] != nil && !reachableFns[fp.FnName] {
							reachableFns[fp.FnName] = true
							worklist = append(worklist, fp.FnName)
						}
					}
				case *CallBuiltinInst:
					if i.BuiltinID == "view" {
						recordViewArg(i.Args)
					}
				case *CallHostServiceInst:
					if i.ClassName == "View" && i.MethodName == "render" {
						recordViewArg(i.Args)
					}
				case *TryCatchInst:
					scanBlocks(i.TryBlocks)
					scanBlocks(i.CatchBlocks)
				}
			}
		}
	}

	for len(worklist) > 0 {
		fnName := worklist[0]
		worklist = worklist[1:]
		if fn := l.prog.Functions[fnName]; fn != nil {
			scanBlocks(fn.Blocks)
		}
	}

	return usedViews, hasDynamicViewCall
}

// compactStaticViewHTML collapses runs of indentation whitespace between HTML tags
// and strips HTML comments (outside <pre>, <code>, <textarea>, <script>, <style>)
// to minimize .rdata size in standalone native binaries without altering rendered layout.
func compactStaticViewHTML(htmlStr string) string {
	var out strings.Builder
	out.Grow(len(htmlStr))
	n := len(htmlStr)
	inProtected := 0
	i := 0

	hasCiPrefix := func(s, prefix string) bool {
		return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
	}

	for i < n {
		// Preserve Joss template interpolation blocks {{ ... }} and {!! ... !!} verbatim
		if strings.HasPrefix(htmlStr[i:], "{{") {
			if end := strings.Index(htmlStr[i+2:], "}}"); end != -1 {
				out.WriteString(htmlStr[i : i+2+end+2])
				i += 2 + end + 2
				continue
			}
		}
		if strings.HasPrefix(htmlStr[i:], "{!!") {
			if end := strings.Index(htmlStr[i+3:], "!!}"); end != -1 {
				out.WriteString(htmlStr[i : i+3+end+3])
				i += 3 + end + 3
				continue
			}
		}

		// Strip standard HTML comments <!-- ... --> outside protected blocks
		if inProtected == 0 && strings.HasPrefix(htmlStr[i:], "<!--") && !strings.HasPrefix(htmlStr[i:], "<!--[if") && !strings.HasPrefix(htmlStr[i:], "<!-- JOSS_ASSETS -->") {
			if end := strings.Index(htmlStr[i+4:], "-->"); end != -1 {
				i += 4 + end + 3
				continue
			}
		}

		// Track entry/exit of whitespace-sensitive tags (<pre, <code, <textarea, <script, <style)
		if htmlStr[i] == '<' {
			rest := htmlStr[i:]
			for _, tag := range []string{"pre", "code", "textarea", "script", "style"} {
				if hasCiPrefix(rest, "<"+tag) && (len(rest) == 1+len(tag) || rest[1+len(tag)] == ' ' || rest[1+len(tag)] == '>' || rest[1+len(tag)] == '\t' || rest[1+len(tag)] == '\n' || rest[1+len(tag)] == '\r') {
					inProtected++
					break
				}
				if hasCiPrefix(rest, "</"+tag) && (len(rest) == 2+len(tag) || rest[2+len(tag)] == ' ' || rest[2+len(tag)] == '>' || rest[2+len(tag)] == '\t' || rest[2+len(tag)] == '\n' || rest[2+len(tag)] == '\r') {
					if inProtected > 0 {
						inProtected--
					}
					break
				}
			}
		}

		if inProtected == 0 && (htmlStr[i] == ' ' || htmlStr[i] == '\t' || htmlStr[i] == '\n' || htmlStr[i] == '\r') {
			j := i
			hasNewline := false
			for j < n && (htmlStr[j] == ' ' || htmlStr[j] == '\t' || htmlStr[j] == '\n' || htmlStr[j] == '\r') {
				if htmlStr[j] == '\n' || htmlStr[j] == '\r' {
					hasNewline = true
				}
				j++
			}
			if hasNewline {
				prevIsTagEnd := i > 0 && htmlStr[i-1] == '>'
				nextIsTagStart := j < n && htmlStr[j] == '<'
				if prevIsTagEnd && nextIsTagStart {
					out.WriteByte('\n')
				} else {
					out.WriteByte(' ')
				}
				i = j
				continue
			} else if j-i > 1 {
				out.WriteByte(' ')
				i = j
				continue
			}
		}

		out.WriteByte(htmlStr[i])
		i++
	}
	return out.String()
}

// compileProjectViewsToIR walks app/views, filters to reachable views, preprocesses @extends/@section/@yield/@include/@json,
// translates each template to Joss source code via compileViewHTMLToJossScript, parses it into
// a *parser.Program AST, and lowers it into a native IR function receiving $__data.
func (l *Lowerer) compileProjectViewsToIR(viewsRoot string) ([]compiledViewEntry, error) {
	info, err := os.Stat(viewsRoot)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	usedViews, hasDynamicViewCall := l.collectReachableViewNames()
	if !hasDynamicViewCall && len(usedViews) == 0 {
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
		dotName := strings.ReplaceAll(rel, "/", ".")
		if !hasDynamicViewCall && !usedViews[rel] && !usedViews[dotName] {
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
		finalHTML = compactStaticViewHTML(finalHTML)

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

// PruneUnreachableFunctions eliminates functions in l.prog.Functions that are not reachable
// from `main`, `__joss_http_dispatch`, or `view`.
func (l *Lowerer) PruneUnreachableFunctions() {
	if l.prog == nil || len(l.prog.Functions) == 0 {
		return
	}

	reachable := make(map[string]bool)
	var worklist []string

	for _, root := range []string{"main", "__joss_http_dispatch", "view", "__joss_get_embedded_asset"} {
		if l.prog.Functions[root] != nil {
			reachable[root] = true
			worklist = append(worklist, root)
		}
	}
	if len(worklist) == 0 {
		return
	}

	var scanBlocks func(blocks []*BasicBlock)
	scanBlocks = func(blocks []*BasicBlock) {
		for _, blk := range blocks {
			for _, inst := range blk.Instructions {
				switch i := inst.(type) {
				case *CallInst:
					if l.prog.Functions[i.Callee] != nil && !reachable[i.Callee] {
						reachable[i.Callee] = true
						worklist = append(worklist, i.Callee)
					}
					for _, arg := range i.Args {
						if fp, ok := arg.(*ConstFnPtr); ok && l.prog.Functions[fp.FnName] != nil && !reachable[fp.FnName] {
							reachable[fp.FnName] = true
							worklist = append(worklist, fp.FnName)
						}
					}
				case *CallBuiltinInst:
					if l.prog.Functions[i.BuiltinID] != nil && !reachable[i.BuiltinID] {
						reachable[i.BuiltinID] = true
						worklist = append(worklist, i.BuiltinID)
					}
					for _, arg := range i.Args {
						if fp, ok := arg.(*ConstFnPtr); ok && l.prog.Functions[fp.FnName] != nil && !reachable[fp.FnName] {
							reachable[fp.FnName] = true
							worklist = append(worklist, fp.FnName)
						}
					}
				case *CallHostServiceInst:
					mName := i.ClassName + "_" + i.MethodName
					if l.prog.Functions[mName] != nil && !reachable[mName] {
						reachable[mName] = true
						worklist = append(worklist, mName)
					}
					if i.ClassName == "View" && i.MethodName == "render" && l.prog.Functions["view"] != nil && !reachable["view"] {
						reachable["view"] = true
						worklist = append(worklist, "view")
					}
				case *StoreInst:
					if fp, ok := i.Val.(*ConstFnPtr); ok && l.prog.Functions[fp.FnName] != nil && !reachable[fp.FnName] {
						reachable[fp.FnName] = true
						worklist = append(worklist, fp.FnName)
					}
				case *MoveInst:
					if fp, ok := i.Src.(*ConstFnPtr); ok && l.prog.Functions[fp.FnName] != nil && !reachable[fp.FnName] {
						reachable[fp.FnName] = true
						worklist = append(worklist, fp.FnName)
					}
				case *TryCatchInst:
					scanBlocks(i.TryBlocks)
					scanBlocks(i.CatchBlocks)
				}
			}
		}
	}

	for len(worklist) > 0 {
		fnName := worklist[0]
		worklist = worklist[1:]
		if fn := l.prog.Functions[fnName]; fn != nil {
			scanBlocks(fn.Blocks)
		}
	}

	for fnName := range l.prog.Functions {
		if !reachable[fnName] {
			delete(l.prog.Functions, fnName)
		}
	}
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
	methodParam := fn.AddParam("method", TypeString)
	pathParam := fn.AddParam("path", TypeString)
	requestParam := fn.AddParam("request", PtrType(StructType("map", nil)))
	requestGlobal := l.prog.AddGlobal("__joss_request_params", requestParam.Type(), nil, false)
	fn.EntryBlock.AddInstruction(&GlobalStoreInst{GlobalName: requestGlobal.Name, Val: requestParam})

	curr := fn.EntryBlock
	dispatched := 0

	for idx, rt := range routes {
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

		pathMatch := fn.NewTemp(TypeBool, fmt.Sprintf("rpath_%d", idx))
		if numPlaceholders == 0 {
			curr.AddInstruction(&CompareInst{
				Op:    OpCmpEq,
				Dest:  pathMatch,
				Left:  pathParam,
				Right: NewConstString(rt.path),
			})
		} else {
			curr.AddInstruction(&CallInst{
				Dest:   pathMatch,
				Callee: "__joss_rt_route_matches",
				Args:   []Value{pathParam, NewConstString(rt.path)},
			})
		}
		cmpTemp := pathMatch
		if rt.method != "ANY" {
			methodMatch := fn.NewTemp(TypeBool, fmt.Sprintf("rmethod_%d", idx))
			curr.AddInstruction(&CompareInst{
				Op:    OpCmpEq,
				Dest:  methodMatch,
				Left:  methodParam,
				Right: NewConstString(strings.ToUpper(rt.method)),
			})
			cmpTemp = fn.NewTemp(TypeBool, fmt.Sprintf("rcmp_%d", idx))
			curr.AddInstruction(&BinaryInst{Dest: cmpTemp, Op: OpAnd, Left: methodMatch, Right: pathMatch})
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
	case "get", "post", "put", "patch", "delete", "options":
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

// synthesizeEmbeddedAssetsFunction walks static asset directories (such as public/ and assets/)
// and any project subdirectories containing static files (.css, .js, .svg, .png, .jpg, .ico, etc.)
// and synthesizes the native IR function `__joss_get_embedded_asset(path: string): string`
// which serves the files directly from the standalone binary (.rodata section).
func (l *Lowerer) synthesizeEmbeddedAssetsFunction() error {
	if l.prog.Functions["__joss_get_embedded_asset"] != nil {
		return nil
	}

	type assetEntry struct {
		aliases []string
		content string
	}

	var assets []assetEntry
	ignoredDirs := map[string]bool{
		".git": true, ".vscode": true, ".idea": true, "build": true, "dist": true,
		"vendor": true, "node_modules": true, ".gemini": true, ".codex": true,
		".agents": true, ".github": true, "tests": true, "test": true,
	}

	staticExts := map[string]bool{
		".css": true, ".js": true, ".json": true, ".svg": true, ".png": true,
		".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true,
		".woff": true, ".woff2": true, ".ttf": true, ".txt": true, ".xml": true,
	}

	searchRoots := []string{"public", "assets"}
	for _, root := range searchRoots {
		if _, err := os.Stat(root); err != nil {
			continue
		}
		_ = filepath.Walk(root, func(path string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil || fi == nil || fi.IsDir() {
				return nil
			}
			parts := strings.Split(filepath.ToSlash(path), "/")
			if len(parts) > 0 && ignoredDirs[parts[0]] {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if !staticExts[ext] || fi.Size() > 2<<20 {
				return nil
			}
			contentBytes, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			slashPath := filepath.ToSlash(path) // e.g. "public/css/landing.css"
			aliases := []string{
				slashPath,
				"/" + slashPath,
			}
			// Also support path without the root prefix (e.g. "/css/landing.css" and "css/landing.css")
			withoutRoot := strings.TrimPrefix(slashPath, root+"/")
			if withoutRoot != slashPath {
				aliases = append(aliases, withoutRoot, "/"+withoutRoot)
			}
			assets = append(assets, assetEntry{
				aliases: aliases,
				content: string(contentBytes),
			})
			return nil
		})
	}

	fn := NewFunction("__joss_get_embedded_asset", TypeString)
	pathParam := fn.AddParam("path", TypeString)
	curr := fn.EntryBlock

	for idx, a := range assets {
		matchBlk := fn.NewBlock(fmt.Sprintf("asset_match_%d", idx))
		matchBlk.SetTerminator(&ReturnTerminator{Val: NewConstString(a.content)})

		for subIdx, alias := range a.aliases {
			nextBlk := fn.NewBlock(fmt.Sprintf("asset_next_%d_%d", idx, subIdx))
			cmpTemp := fn.NewTemp(TypeBool, fmt.Sprintf("acmp_%d_%d", idx, subIdx))
			curr.AddInstruction(&CompareInst{
				Op:    OpCmpEq,
				Dest:  cmpTemp,
				Left:  pathParam,
				Right: NewConstString(alias),
			})
			curr.SetTerminator(&BranchTerminator{
				Cond:       cmpTemp,
				TrueBlock:  matchBlk,
				FalseBlock: nextBlk,
			})
			curr = nextBlk
		}
	}

	curr.SetTerminator(&ReturnTerminator{Val: NewConstString("")})
	l.prog.AddFunction(fn)
	return nil
}
