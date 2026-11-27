package ir

import (
	"crypto/rand"
	"encoding/hex"
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
//  1. __joss_http_dispatch(method, path, request, session, cookie): dispatches
//     HTTP requests to compiled native Controller_method functions with their request context.
//  2. __joss_view_<idx>(__data): compiled native IR functions for each reachable .joss.html template
//  3. view(viewName, dataMap): dispatches viewName to the compiled view IR function
func (l *Lowerer) programNeedsEmbeddedAssets() bool {
	if l.reachGraph == nil {
		return true
	}
	if l.reachGraph.HasCapability(analyzer.CapServer) ||
		l.reachGraph.HasCapability(analyzer.CapHTTP) ||
		l.reachGraph.HasCapability(analyzer.CapDatabase) ||
		l.reachGraph.HasCapability(analyzer.CapFilesystem) ||
		l.reachGraph.HasCapability(analyzer.CapOCI) {
		return true
	}
	if l.prog == nil {
		return false
	}
	var scan func(blocks []*BasicBlock) bool
	scan = func(blocks []*BasicBlock) bool {
		for _, blk := range blocks {
			for _, inst := range blk.Instructions {
				switch i := inst.(type) {
				case *CallInst:
					if i.Callee == "env" || i.Callee == "config" || i.Callee == "file_get_contents" || i.Callee == "file_exists" || i.Callee == "is_file" || i.Callee == "is_dir" {
						return true
					}
				case *CallBuiltinInst:
					if i.BuiltinID == "env" || i.BuiltinID == "config" || i.BuiltinID == "file_get_contents" || i.BuiltinID == "file_exists" || i.BuiltinID == "is_file" || i.BuiltinID == "is_dir" || i.BuiltinID == "__joss_file_get_bytes" {
						return true
					}
				case *CallRuntimeInst:
					if i.Func == "env" || i.Func == "joss_env" || i.Func == "joss_config" || strings.HasPrefix(i.Func, "joss_file_") || i.Func == "joss_is_file" || i.Func == "joss_is_dir" {
						return true
					}
				case *TryCatchInst:
					if scan(i.TryBlocks) || scan(i.CatchBlocks) {
						return true
					}
				}
			}
		}
		return false
	}
	for _, fn := range l.prog.Functions {
		if scan(fn.Blocks) {
			return true
		}
	}
	return false
}

func (l *Lowerer) lowerProjectViewsAndRoutes() error {
	if l.programNeedsEmbeddedAssets() {
		if err := l.synthesizeEmbeddedAssetsFunction(); err != nil {
			return err
		}
	}
	if l.reachGraph != nil && !l.reachGraph.HasCapability(analyzer.CapServer) && !l.reachGraph.HasCapability(analyzer.CapHTTP) {
		return l.synthesizePluginDispatchFunction()
	}
	if err := l.synthesizeHTTPDispatchFunction(); err != nil {
		return err
	}
	if err := l.synthesizePluginDispatchFunction(); err != nil {
		return err
	}
	views, err := l.compileProjectViewsToIR("app/views")
	if err != nil {
		return err
	}
	if err := l.synthesizeViewDispatcherFunction(views); err != nil {
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
				case *CallRuntimeInst:
					for _, arg := range i.Args {
						if fp, ok := arg.(*ConstFnPtr); ok && l.prog.Functions[fp.FnName] != nil && !reachableFns[fp.FnName] {
							reachableFns[fp.FnName] = true
							worklist = append(worklist, fp.FnName)
						}
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

	isHotReloadDev := strings.TrimSpace(os.Getenv("JOSS_HOT_RELOAD_URL")) != ""
	devChunksDir := filepath.Join("assets", ".joss_cache", "views")

	var entries []compiledViewEntry
	for idx, rel := range relPaths {
		dotName := strings.ReplaceAll(rel, "/", ".")
		viewContent := rawViews[rel]

		finalHTML, errPre := viewtemplate.PreprocessDirectives(viewContent, readViewFile, viewtemplate.PreprocessOptions{})
		if errPre != nil {
			finalHTML = viewContent
		}

		var jossScript string
		if isHotReloadDev {
			var chunks []string
			jossScript, chunks, _, _ = viewtemplate.CompileHTMLToJossWithSlots(dotName, finalHTML)
			_ = viewtemplate.WriteViewChunksFile(devChunksDir, dotName, chunks)
		} else {
			finalHTML = compactStaticViewHTML(finalHTML)
			jossScript, _ = viewtemplate.CompileHTMLToJoss(finalHTML)
		}
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
// from `main` or the server dispatch entrypoint. View and asset helpers are
// retained only when called from those roots.
func (l *Lowerer) PruneUnreachableFunctions() {
	if l.prog == nil || len(l.prog.Functions) == 0 {
		return
	}

	reachable := make(map[string]bool)
	var worklist []string

	for _, root := range []string{"main", "__joss_http_dispatch", "__joss_plugin_dispatch"} {
		if l.prog.Functions[root] != nil {
			reachable[root] = true
			worklist = append(worklist, root)
		}
	}
	if len(worklist) == 0 {
		return
	}

	pluginFnNames := l.collectDeclaredPluginIRFunctions()
	hasPluginUsage := false

	enqueue := func(fnName string) {
		if fnName != "" && l.prog.Functions[fnName] != nil && !reachable[fnName] {
			reachable[fnName] = true
			worklist = append(worklist, fnName)
		}
	}

	var scanBlocks func(blocks []*BasicBlock)
	scanBlocks = func(blocks []*BasicBlock) {
		for _, blk := range blocks {
			for _, inst := range blk.Instructions {
				switch i := inst.(type) {
				case *CallInst:
					enqueue(i.Callee)
					if isPluginDispatchCallee(i.Callee) {
						hasPluginUsage = true
					}
					for _, arg := range i.Args {
						if fp, ok := arg.(*ConstFnPtr); ok {
							enqueue(fp.FnName)
						}
					}
				case *CallBuiltinInst:
					enqueue(i.BuiltinID)
					for _, arg := range i.Args {
						if fp, ok := arg.(*ConstFnPtr); ok {
							enqueue(fp.FnName)
						}
					}
				case *CallRuntimeInst:
					enqueue(i.Func)
					for _, arg := range i.Args {
						if fp, ok := arg.(*ConstFnPtr); ok {
							enqueue(fp.FnName)
						}
					}
				case *CallHostServiceInst:
					if i.ClassName == "Plugin" && (i.MethodName == "call" || i.MethodName == "stream" || i.MethodName == "load") {
						hasPluginUsage = true
					}
					for _, mName := range []string{i.ClassName + "_" + i.MethodName, "__joss_host_" + i.ClassName + "_" + i.MethodName} {
						enqueue(mName)
					}
					if i.ClassName == "View" && i.MethodName == "render" {
						enqueue("view")
					}
					for _, arg := range i.Args {
						if fp, ok := arg.(*ConstFnPtr); ok {
							enqueue(fp.FnName)
						}
					}
				case *StoreInst:
					if fp, ok := i.Val.(*ConstFnPtr); ok {
						enqueue(fp.FnName)
					}
				case *MoveInst:
					if fp, ok := i.Src.(*ConstFnPtr); ok {
						enqueue(fp.FnName)
					}
				case *TryCatchInst:
					scanBlocks(i.TryBlocks)
					scanBlocks(i.CatchBlocks)
				}
				for _, op := range inst.Operands() {
					if fp, ok := op.(*ConstFnPtr); ok {
						enqueue(fp.FnName)
					}
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
		if hasPluginUsage && len(pluginFnNames) > 0 {
			for pFn := range pluginFnNames {
				enqueue(pFn)
			}
			pluginFnNames = nil
		}
	}

	for fnName := range l.prog.Functions {
		if !reachable[fnName] {
			delete(l.prog.Functions, fnName)
		}
	}
}

// lowerRequestSessionContext threads request-local session state through the
// ordinary IR call graph reachable from the synthesized HTTP dispatcher. This
// avoids process globals and leaves ISA emitters with generic calls and maps.
func (l *Lowerer) lowerRequestSessionContext() {
	if l.prog == nil {
		return
	}
	const dispatcherName = "__joss_http_dispatch"
	const sessionOwner = "__joss_host_Session_get"
	dispatcher := l.prog.Functions[dispatcherName]
	contextType := PtrType(StructType("map", nil))
	if dispatcher != nil && len(dispatcher.Params) >= 4 {
		contextType = dispatcher.Params[3].Type()
	}

	// Find functions reachable from HTTP route dispatch.
	reachable := make(map[string]bool)
	callEdges := make(map[string]map[string]bool)
	requiresSession := make(map[string]bool)
	worklist := []string(nil)
	if dispatcher != nil {
		reachable[dispatcherName] = true
		worklist = append(worklist, dispatcherName)
	}
	var scanBlocks func(string, []*BasicBlock)
	scanBlocks = func(caller string, blocks []*BasicBlock) {
		for _, block := range blocks {
			for _, instruction := range block.Instructions {
				callee := ""
				switch call := instruction.(type) {
				case *CallInst:
					callee = call.Callee
				case *CallBuiltinInst:
					if call.BuiltinID == "view" && l.prog.Functions["view"] != nil {
						callee = "view"
					}
				case *CallHostServiceInst:
					if call.ClassName == "Session" && (call.MethodName == "get" || call.MethodName == "put" || call.MethodName == "has" || call.MethodName == "forget") {
						requiresSession[caller] = true
					}
					for _, candidate := range []string{call.ClassName + "_" + call.MethodName, "__joss_host_" + call.ClassName + "_" + call.MethodName} {
						if l.prog.Functions[candidate] != nil {
							callee = candidate
							break
						}
					}
				case *TryCatchInst:
					scanBlocks(caller, call.TryBlocks)
					scanBlocks(caller, call.CatchBlocks)
				}
				if callee != "" && l.prog.Functions[callee] != nil && !reachable[callee] {
					reachable[callee] = true
					worklist = append(worklist, callee)
				}
				if callee != "" && l.prog.Functions[callee] != nil {
					if callEdges[caller] == nil {
						callEdges[caller] = make(map[string]bool)
					}
					callEdges[caller][callee] = true
				}
				for _, value := range instruction.Operands() {
					if fnPtr, ok := value.(*ConstFnPtr); ok && l.prog.Functions[fnPtr.FnName] != nil && !reachable[fnPtr.FnName] {
						reachable[fnPtr.FnName] = true
						worklist = append(worklist, fnPtr.FnName)
					}
					if fnPtr, ok := value.(*ConstFnPtr); ok && l.prog.Functions[fnPtr.FnName] != nil {
						if callEdges[caller] == nil {
							callEdges[caller] = make(map[string]bool)
						}
						callEdges[caller][fnPtr.FnName] = true
					}
				}
			}
		}
	}
	for len(worklist) > 0 {
		name := worklist[0]
		worklist = worklist[1:]
		if fn := l.prog.Functions[name]; fn != nil {
			scanBlocks(name, fn.Blocks)
		}
	}

	// Add the hidden parameter only to route functions that use Session or call
	// another function that does. Unrelated controller/helpers keep their ABI.
	contextual := make(map[string]bool, len(requiresSession))
	for name := range requiresSession {
		contextual[name] = true
	}
	changed := true
	for changed {
		changed = false
		for caller, callees := range callEdges {
			if contextual[caller] {
				continue
			}
			for callee := range callees {
				if contextual[callee] {
					contextual[caller] = true
					changed = true
					break
				}
			}
		}
	}
	for name := range contextual {
		if name == dispatcherName || name == "__joss_plugin_dispatch" || name == sessionOwner || strings.HasPrefix(name, "__joss_host_Session_") || strings.HasPrefix(name, "__closure_") {
			delete(contextual, name)
			continue
		}
		fn := l.prog.Functions[name]
		if fn == nil {
			continue
		}
		fn.AddParam("__request_session_context", contextType)
		fn.Attributes["request_session_context"] = "true"
		contextual[name] = true
	}

	contextFor := func(fn *Function, emit *[]Instruction) Value {
		if dispatcher != nil && fn.Name == dispatcherName {
			return dispatcher.Params[3]
		}
		if fn.Attributes["request_session_context"] == "true" {
			for _, param := range fn.Params {
				if param.Name == "__request_session_context" {
					return param
				}
			}
		}
		tmp := fn.NewTemp(contextType, "active_sess")
		*emit = append(*emit, &CallRuntimeInst{Dest: tmp, Func: "joss_get_active_session"})
		return tmp
	}
	var rewriteBlocks func(*Function, []*BasicBlock)
	rewriteBlocks = func(fn *Function, blocks []*BasicBlock) {
		for _, block := range blocks {
			instructions := make([]Instruction, 0, len(block.Instructions))
			for _, instruction := range block.Instructions {
				switch call := instruction.(type) {
				case *CallInst:
					if contextual[call.Callee] {
						call.Args = append(call.Args, contextFor(fn, &instructions))
					}
				case *CallBuiltinInst:
					if call.BuiltinID == "view" && contextual["view"] {
						args := append([]Value(nil), call.Args...)
						args = append(args, contextFor(fn, &instructions))
						instructions = append(instructions, &CallInst{Dest: call.Dest, Callee: "view", Args: args})
						continue
					}
				case *CallHostServiceInst:
					if call.ClassName == "Session" && (call.MethodName == "get" || call.MethodName == "put" || call.MethodName == "has" || call.MethodName == "forget") {
						owner := "__joss_host_Session_" + call.MethodName
						if l.prog.Functions[owner] == nil {
							instructions = append(instructions, instruction)
							continue
						}
						args := []Value{contextFor(fn, &instructions)}
						args = append(args, call.Args...)
						instructions = append(instructions, &CallInst{Dest: call.Dest, Callee: owner, Args: args})
						continue
					}
				case *TryCatchInst:
					rewriteBlocks(fn, call.TryBlocks)
					rewriteBlocks(fn, call.CatchBlocks)
				}
				instructions = append(instructions, instruction)
			}
			block.Instructions = instructions
		}
	}
	for _, fn := range l.prog.Functions {
		rewriteBlocks(fn, fn.Blocks)
	}
}

// lowerRequestSEOContext keeps SEO metadata in the request map and threads it
// through only the HTTP call graph that touches SEO. Product formatting remains
// in std/http/seo_host.joss; emitters implement only generic map/string ops.
func (l *Lowerer) lowerRequestSEOContext() {
	if l.prog == nil {
		return
	}
	const dispatcherName = "__joss_http_dispatch"
	dispatcher := l.prog.Functions[dispatcherName]
	if dispatcher == nil || len(dispatcher.Params) < 3 {
		return
	}
	contextType := dispatcher.Params[2].Type()
	seoOwner := func(call *CallHostServiceInst) string {
		method := call.MethodName
		if method == "keywords" && len(call.Args) > 0 && call.Args[0].Type().Kind == TypeKindArray && l.prog.Functions["__joss_host_SEO_keywords_array"] != nil {
			return "__joss_host_SEO_keywords_array"
		}
		return "__joss_host_SEO_" + method
	}
	seoRuntimeOwner := func(name string) string {
		name = strings.TrimPrefix(name, "fn_")
		method := ""
		switch {
		case strings.HasPrefix(name, "SEO_"):
			method = strings.TrimPrefix(name, "SEO_")
		case strings.HasPrefix(name, "SEO::"):
			method = strings.TrimPrefix(name, "SEO::")
		default:
			return ""
		}
		switch method {
		case "title", "description", "keywords", "canonical", "og", "meta", "render":
			return "__joss_host_SEO_" + method
		default:
			return ""
		}
	}
	contextual := make(map[string]bool)
	edges := make(map[string]map[string]bool)
	worklist := []string{dispatcherName}
	reachable := map[string]bool{dispatcherName: true}
	var scan func(string, []*BasicBlock)
	scan = func(caller string, blocks []*BasicBlock) {
		for _, block := range blocks {
			for _, instruction := range block.Instructions {
				callee := ""
				switch call := instruction.(type) {
				case *CallInst:
					callee = call.Callee
					if owner := seoRuntimeOwner(call.Callee); owner != "" && l.prog.Functions[owner] != nil {
						contextual[caller] = true
						callee = owner
					}
				case *CallBuiltinInst:
					if call.BuiltinID == "view" && l.prog.Functions["view"] != nil {
						callee = "view"
					} else if call.BuiltinID == "SEO_render" && l.prog.Functions["__joss_host_SEO_render"] != nil {
						contextual[caller] = true
						callee = "__joss_host_SEO_render"
					}
				case *CallRuntimeInst:
					if owner := seoRuntimeOwner(call.Func); owner != "" && l.prog.Functions[owner] != nil {
						contextual[caller] = true
						callee = owner
					}
				case *CallHostServiceInst:
					if call.ClassName == "SEO" && (call.MethodName == "title" || call.MethodName == "description" || call.MethodName == "keywords" || call.MethodName == "canonical" || call.MethodName == "og" || call.MethodName == "meta" || call.MethodName == "render") {
						contextual[caller] = true
						callee = seoOwner(call)
					}
				case *TryCatchInst:
					scan(caller, call.TryBlocks)
					scan(caller, call.CatchBlocks)
				}
				if callee != "" && l.prog.Functions[callee] != nil {
					if !reachable[callee] {
						reachable[callee] = true
						worklist = append(worklist, callee)
					}
					if edges[caller] == nil {
						edges[caller] = make(map[string]bool)
					}
					edges[caller][callee] = true
				}
				for _, value := range instruction.Operands() {
					if ptr, ok := value.(*ConstFnPtr); ok && l.prog.Functions[ptr.FnName] != nil {
						if !reachable[ptr.FnName] {
							reachable[ptr.FnName] = true
							worklist = append(worklist, ptr.FnName)
						}
						if edges[caller] == nil {
							edges[caller] = make(map[string]bool)
						}
						edges[caller][ptr.FnName] = true
					}
				}
			}
		}
	}
	for len(worklist) > 0 {
		name := worklist[0]
		worklist = worklist[1:]
		if fn := l.prog.Functions[name]; fn != nil {
			scan(name, fn.Blocks)
		}
	}
	changed := true
	for changed {
		changed = false
		for caller, callees := range edges {
			if contextual[caller] {
				continue
			}
			for callee := range callees {
				if contextual[callee] {
					contextual[caller] = true
					changed = true
					break
				}
			}
		}
	}
	delete(contextual, dispatcherName)
	delete(contextual, "__joss_plugin_dispatch")
	delete(contextual, "__joss_host_SEO_title")
	for name := range contextual {
		if strings.HasPrefix(name, "__closure_") {
			delete(contextual, name)
			continue
		}
		if fn := l.prog.Functions[name]; fn != nil {
			fn.AddParam("__request_seo_context", contextType)
			fn.Attributes["request_seo_context"] = "true"
		}
	}
	contextFor := func(fn *Function, emit *[]Instruction) Value {
		if fn.Name == dispatcherName {
			return dispatcher.Params[2]
		}
		if fn.Attributes["request_seo_context"] == "true" {
			for _, param := range fn.Params {
				if param.Name == "__request_seo_context" {
					return param
				}
			}
		}
		if strings.HasPrefix(fn.Name, "__closure_") || fn.Name == "__joss_plugin_dispatch" {
			tmp := fn.NewTemp(contextType, "active_req")
			*emit = append(*emit, &CallRuntimeInst{Dest: tmp, Func: "joss_get_active_request"})
			return tmp
		}
		return NewConstNull(contextType)
	}
	var rewrite func(*Function, []*BasicBlock)
	rewrite = func(fn *Function, blocks []*BasicBlock) {
		for _, block := range blocks {
			items := make([]Instruction, 0, len(block.Instructions))
			for _, instruction := range block.Instructions {
				switch call := instruction.(type) {
				case *CallInst:
					if owner := seoRuntimeOwner(call.Callee); owner != "" && l.prog.Functions[owner] != nil {
						args := []Value{contextFor(fn, &items)}
						args = append(args, call.Args...)
						items = append(items, &CallInst{Dest: call.Dest, Callee: owner, Args: args})
						continue
					}
					if contextual[call.Callee] {
						call.Args = append(call.Args, contextFor(fn, &items))
					}
				case *CallBuiltinInst:
					if call.BuiltinID == "view" && contextual["view"] {
						args := append([]Value(nil), call.Args...)
						args = append(args, contextFor(fn, &items))
						items = append(items, &CallInst{Dest: call.Dest, Callee: "view", Args: args})
						continue
					}
					if call.BuiltinID == "SEO_render" && l.prog.Functions["__joss_host_SEO_render"] != nil {
						items = append(items, &CallInst{Dest: call.Dest, Callee: "__joss_host_SEO_render", Args: []Value{contextFor(fn, &items)}})
						continue
					}
				case *CallRuntimeInst:
					if owner := seoRuntimeOwner(call.Func); owner != "" && l.prog.Functions[owner] != nil {
						args := []Value{contextFor(fn, &items)}
						args = append(args, call.Args...)
						items = append(items, &CallInst{Dest: call.Dest, Callee: owner, Args: args})
						continue
					}
				case *CallHostServiceInst:
					owner := seoOwner(call)
					if call.ClassName == "SEO" && l.prog.Functions[owner] != nil {
						args := []Value{contextFor(fn, &items)}
						args = append(args, call.Args...)
						items = append(items, &CallInst{Dest: call.Dest, Callee: owner, Args: args})
						continue
					}
				case *TryCatchInst:
					rewrite(fn, call.TryBlocks)
					rewrite(fn, call.CatchBlocks)
				}
				items = append(items, instruction)
			}
			block.Instructions = items
		}
	}
	for _, fn := range l.prog.Functions {
		rewrite(fn, fn.Blocks)
	}
}

// lowerGenericCollectionBuiltins converts map deletion into a canonical IR
// operation so the interpreter and each native sink share the same Joss-level
// policy while the emitters only implement the map representation operation.
func (l *Lowerer) lowerGenericCollectionBuiltins() {
	if l.prog == nil {
		return
	}
	var rewrite func([]*BasicBlock)
	rewrite = func(blocks []*BasicBlock) {
		for _, block := range blocks {
			instructions := make([]Instruction, 0, len(block.Instructions))
			for _, instruction := range block.Instructions {
				if call, ok := instruction.(*CallBuiltinInst); ok && call.BuiltinID == "map_delete" && len(call.Args) == 2 {
					instructions = append(instructions, &MapDeleteInst{Map: call.Args[0], Key: call.Args[1]})
					continue
				}
				if nested, ok := instruction.(*TryCatchInst); ok {
					rewrite(nested.TryBlocks)
					rewrite(nested.CatchBlocks)
				}
				instructions = append(instructions, instruction)
			}
			block.Instructions = instructions
		}
	}
	for _, function := range l.prog.Functions {
		rewrite(function.Blocks)
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
	// Inject global auth & flash variables matching pkg/core/view.go
	falseBox := fn.NewTemp(TypeMixed, "view_false_box")
	curr.AddInstruction(&BoxValueInst{Dest: falseBox, Val: NewConstBool(false)})
	trueBox := fn.NewTemp(TypeMixed, "view_true_box")
	curr.AddInstruction(&BoxValueInst{Dest: trueBox, Val: NewConstBool(true)})
	curr.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("auth_check"), Val: falseBox})
	curr.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("auth_guest"), Val: trueBox})
	curr.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("auth_user"), Val: NewConstString("")})
	curr.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("auth_role"), Val: NewConstString("")})
	curr.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("auth_email"), Val: NewConstString("")})

	sessTmp := fn.NewTemp(PtrType(StructType("map", nil)), "view_sess")
	curr.AddInstruction(&CallRuntimeInst{Dest: sessTmp, Func: "joss_get_active_session"})
	sessNotNull := fn.NewTemp(TypeBool, "view_sess_ok")
	curr.AddInstruction(&CallRuntimeInst{Dest: sessNotNull, Func: "joss_is_not_null", Args: []Value{sessTmp}})
	sessBlk := fn.NewBlock("view_sess_inject")
	afterSessBlk := fn.NewBlock("view_dispatch_start")
	curr.SetTerminator(&BranchTerminator{Cond: sessNotNull, TrueBlock: sessBlk, FalseBlock: afterSessBlk})

	uidVal := fn.NewTemp(TypeMixed, "view_uid")
	sessBlk.AddInstruction(&MapGetInst{Dest: uidVal, Map: sessTmp, Key: NewConstString("user_id")})
	uidOk := fn.NewTemp(TypeBool, "view_uid_ok")
	sessBlk.AddInstruction(&CallRuntimeInst{Dest: uidOk, Func: "joss_is_not_null", Args: []Value{uidVal}})
	injectAuthBlk := fn.NewBlock("view_sess_auth")
	flashBlk := fn.NewBlock("view_sess_flash")
	sessBlk.SetTerminator(&BranchTerminator{Cond: uidOk, TrueBlock: injectAuthBlk, FalseBlock: flashBlk})

	injectAuthBlk.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("auth_check"), Val: trueBox})
	injectAuthBlk.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("auth_guest"), Val: falseBox})
	uName := fn.NewTemp(TypeMixed, "view_uname")
	injectAuthBlk.AddInstruction(&MapGetInst{Dest: uName, Map: sessTmp, Key: NewConstString("user_name")})
	injectAuthBlk.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("auth_user"), Val: uName})
	uRole := fn.NewTemp(TypeMixed, "view_urole")
	injectAuthBlk.AddInstruction(&MapGetInst{Dest: uRole, Map: sessTmp, Key: NewConstString("user_role")})
	injectAuthBlk.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("auth_role"), Val: uRole})
	uEmail := fn.NewTemp(TypeMixed, "view_uemail")
	injectAuthBlk.AddInstruction(&MapGetInst{Dest: uEmail, Map: sessTmp, Key: NewConstString("user_email")})
	injectAuthBlk.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("auth_email"), Val: uEmail})
	injectAuthBlk.SetTerminator(&JumpTerminator{Target: flashBlk})

	errVal := fn.NewTemp(TypeMixed, "view_err")
	flashBlk.AddInstruction(&MapGetInst{Dest: errVal, Map: sessTmp, Key: NewConstString("error")})
	errOk := fn.NewTemp(TypeBool, "view_err_ok")
	flashBlk.AddInstruction(&CallRuntimeInst{Dest: errOk, Func: "joss_is_not_null", Args: []Value{errVal}})
	setErrBlk := fn.NewBlock("view_set_err")
	checkSuccBlk := fn.NewBlock("view_check_succ")
	flashBlk.SetTerminator(&BranchTerminator{Cond: errOk, TrueBlock: setErrBlk, FalseBlock: checkSuccBlk})
	setErrBlk.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("error"), Val: errVal})
	setErrBlk.SetTerminator(&JumpTerminator{Target: checkSuccBlk})

	succVal := fn.NewTemp(TypeMixed, "view_succ")
	checkSuccBlk.AddInstruction(&MapGetInst{Dest: succVal, Map: sessTmp, Key: NewConstString("success")})
	succOk := fn.NewTemp(TypeBool, "view_succ_ok")
	checkSuccBlk.AddInstruction(&CallRuntimeInst{Dest: succOk, Func: "joss_is_not_null", Args: []Value{succVal}})
	setSuccBlk := fn.NewBlock("view_set_succ")
	checkCsrfBlk := fn.NewBlock("view_check_csrf")
	checkSuccBlk.SetTerminator(&BranchTerminator{Cond: succOk, TrueBlock: setSuccBlk, FalseBlock: checkCsrfBlk})
	setSuccBlk.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("success"), Val: succVal})
	setSuccBlk.SetTerminator(&JumpTerminator{Target: checkCsrfBlk})

	csrfVal := fn.NewTemp(TypeMixed, "view_csrf")
	checkCsrfBlk.AddInstruction(&MapGetInst{Dest: csrfVal, Map: sessTmp, Key: NewConstString("csrf_token")})
	csrfOk := fn.NewTemp(TypeBool, "view_csrf_ok")
	checkCsrfBlk.AddInstruction(&CallRuntimeInst{Dest: csrfOk, Func: "joss_is_not_null", Args: []Value{csrfVal}})
	setCsrfBlk := fn.NewBlock("view_set_csrf")
	checkCsrfBlk.SetTerminator(&BranchTerminator{Cond: csrfOk, TrueBlock: setCsrfBlk, FalseBlock: afterSessBlk})
	setCsrfBlk.AddInstruction(&MapSetInst{Map: dataParam, Key: NewConstString("csrf_token"), Val: csrfVal})
	setCsrfBlk.SetTerminator(&JumpTerminator{Target: afterSessBlk})
	curr = afterSessBlk

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
	method      string
	path        string
	controller  string
	action      string
	middlewares []string
	closureFn   *parser.FunctionLiteral
	isWS        bool
}

// synthesizeHTTPDispatchFunction inspects routes.joss / api.joss AST in l.prep.Units,
// extracts Router::get / Router::post / Router::match / Router::group / closure registrations,
// and synthesizes `__joss_http_dispatch(...)` with full route middleware enforcement.
func (l *Lowerer) synthesizeHTTPDispatchFunction() error {
	if l.prog.Functions["__joss_http_dispatch"] != nil {
		return nil
	}

	routes := l.extractProjectRoutesFromAST()
	if len(routes) == 0 {
		return nil
	}

	// Lower inline closure routes into concrete IR functions before dispatch emission.
	closureCallees := make([]string, len(routes))
	for idx, rt := range routes {
		if rt.closureFn != nil && !rt.isWS {
			fnPtrVal, err := l.lowerFunctionLiteral(rt.closureFn)
			if err == nil {
				if cfp, ok := fnPtrVal.(*ConstFnPtr); ok {
					closureCallees[idx] = cfp.FnName
				}
			}
		}
	}

	fn := NewFunction("__joss_http_dispatch", TypeString)
	methodParam := fn.AddParam("method", TypeString)
	pathParam := fn.AddParam("path", TypeString)
	requestParam := fn.AddParam("request", PtrType(StructType("map", nil)))
	sessionParam := fn.AddParam("session", PtrType(StructType("map", nil)))
	fn.AddParam("cookieHeader", TypeString)
	requestGlobal := l.prog.AddGlobal("__joss_request_params", requestParam.Type(), nil, false)
	fn.EntryBlock.AddInstruction(&GlobalStoreInst{GlobalName: requestGlobal.Name, Val: requestParam})
	fn.EntryBlock.AddInstruction(&CallRuntimeInst{Func: "joss_set_active_session", Args: []Value{sessionParam}})
	if l.prog.Functions["__joss_get_embedded_asset"] != nil {
		fn.EntryBlock.AddInstruction(&CallInst{
			Callee: "joss_register_embedded_asset_resolver",
			Args:   []Value{NewConstFnPtr("__joss_get_embedded_asset", TypeMixed)},
		})
	}

	curr := fn.EntryBlock
	hasWSDispatch := l.reachGraph != nil && l.reachGraph.HasCapability(analyzer.CapWebSocket)
	if !hasWSDispatch {
		for _, rt := range routes {
			if rt.isWS || strings.EqualFold(rt.method, "WS") {
				hasWSDispatch = true
				break
			}
		}
	}
	if !hasWSDispatch && (l.prog.Functions["Server_start"] != nil || l.prog.Functions["HttpServer_start"] != nil) {
		hasWSDispatch = true
	}
	if hasWSDispatch {
		wsCheck := fn.NewTemp(TypeBool, "is_ws_dispatch")
		curr.AddInstruction(&CompareInst{
			Op:    OpCmpEq,
			Dest:  wsCheck,
			Left:  methodParam,
			Right: NewConstString("__websocket__"),
		})
		wsBlk := fn.NewBlock("ws_dispatch")
		httpBlk := fn.NewBlock("http_dispatch")
		curr.SetTerminator(&BranchTerminator{
			Cond:       wsCheck,
			TrueBlock:  wsBlk,
			FalseBlock: httpBlk,
		})

		// Dispatch registered Router::ws(...) controller/closure handlers with a
		// native WebSocket connection object ($ws) and run its RFC 6455 event loop.
		wsCurr := wsBlk
		wsObj := fn.NewTemp(TypeMixed, "ws_obj")
		wsCurr.AddInstruction(&CallRuntimeInst{
			Dest: wsObj,
			Func: "joss_ws_wrap",
			Args: []Value{requestParam},
		})
		for idx, rt := range routes {
			if !rt.isWS {
				continue
			}
			isClosure := closureCallees[idx] != ""
			targetFnName := ""
			var targetFn *Function
			if isClosure {
				targetFnName = closureCallees[idx]
				targetFn = l.prog.Functions[targetFnName]
			} else {
				targetFnName = rt.controller + "_" + rt.action
				targetFn = l.prog.Functions[targetFnName]
				if targetFn == nil {
					for fnName, fObj := range l.prog.Functions {
						if strings.EqualFold(fnName, targetFnName) {
							targetFnName = fnName
							targetFn = fObj
							break
						}
					}
				}
			}
			if targetFn == nil {
				continue
			}
			numPlaceholders := strings.Count(rt.path, "{")
			wsPathMatch := fn.NewTemp(TypeBool, fmt.Sprintf("ws_rpath_%d", idx))
			if numPlaceholders == 0 {
				wsCurr.AddInstruction(&CompareInst{
					Op:    OpCmpEq,
					Dest:  wsPathMatch,
					Left:  pathParam,
					Right: NewConstString(rt.path),
				})
			} else {
				wsCurr.AddInstruction(&CallInst{
					Dest:   wsPathMatch,
					Callee: "__joss_rt_route_matches",
					Args:   []Value{pathParam, NewConstString(rt.path)},
				})
			}
			wsMatchBlk := fn.NewBlock(fmt.Sprintf("ws_match_%d", idx))
			wsNextBlk := fn.NewBlock(fmt.Sprintf("ws_next_%d", idx))
			wsCurr.SetTerminator(&BranchTerminator{
				Cond:       wsPathMatch,
				TrueBlock:  wsMatchBlk,
				FalseBlock: wsNextBlk,
			})

			var wsArgs []Value
			if !isClosure {
				objType := PtrType(StructType(rt.controller, nil))
				ctrlTemp := fn.NewTemp(objType, fmt.Sprintf("ws_ctrl_%d", idx))
				wsMatchBlk.AddInstruction(&NewObjectInst{
					Dest:      ctrlTemp,
					ClassName: rt.controller,
				})
				wsArgs = append(wsArgs, ctrlTemp)
			}
			wsArgs = append(wsArgs, wsObj)
			for pIdx := 0; pIdx < numPlaceholders; pIdx++ {
				pTemp := fn.NewTemp(TypeString, fmt.Sprintf("ws_parg_%d_%d", idx, pIdx))
				wsMatchBlk.AddInstruction(&CallInst{
					Dest:   pTemp,
					Callee: "__joss_rt_route_param",
					Args:   []Value{pathParam, NewConstString(rt.path), NewConstInt(int64(pIdx), TypeI64)},
				})
				wsArgs = append(wsArgs, pTemp)
			}
			for len(wsArgs) < len(targetFn.Params) {
				wsArgs = append(wsArgs, NewConstString(""))
			}
			if len(wsArgs) > len(targetFn.Params) {
				wsArgs = wsArgs[:len(targetFn.Params)]
			}
			if targetFn.ReturnType.Kind == TypeKindVoid {
				wsMatchBlk.AddInstruction(&CallInst{
					Callee: targetFnName,
					Args:   wsArgs,
				})
			} else {
				ig := fn.NewTemp(targetFn.ReturnType, fmt.Sprintf("ws_hret_%d", idx))
				wsMatchBlk.AddInstruction(&CallInst{
					Dest:   ig,
					Callee: targetFnName,
					Args:   wsArgs,
				})
			}
			wsLoopRet := fn.NewTemp(TypeString, fmt.Sprintf("ws_loop_ret_%d", idx))
			wsMatchBlk.AddInstruction(&CallRuntimeInst{
				Dest: wsLoopRet,
				Func: "joss_ws_run_loop",
				Args: []Value{wsObj},
			})
			wsMatchBlk.SetTerminator(&ReturnTerminator{Val: wsLoopRet})
			wsCurr = wsNextBlk
		}

		wsRet := fn.NewTemp(TypeString, "ws_dispatch_ret")
		wsCurr.AddInstruction(&CallRuntimeInst{
			Dest: wsRet,
			Func: "joss___joss_http_dispatch",
			Args: []Value{methodParam, pathParam, requestParam},
		})
		wsCurr.SetTerminator(&ReturnTerminator{Val: wsRet})
		curr = httpBlk
	}

	// Collect non-Router top-level bootstrap statements from routes.joss / api.joss
	// (e.g. `var $loader = new MiddlewareLoader(); $loader->load()`) so custom
	// middlewares from DB/code are registered at native server startup.
	var bootstrapStmts []parser.Statement
	for _, unit := range l.prep.Units {
		if unit.Program == nil {
			continue
		}
		base := strings.ToLower(filepath.Base(unit.Path))
		if base != "routes.joss" && base != "api.joss" {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			if isRouterRegistrationStmt(stmt) {
				continue
			}
			bootstrapStmts = append(bootstrapStmts, stmt)
		}
	}
	hasBootstrapFn := false
	if len(bootstrapStmts) > 0 {
		bootFn := NewFunction("__joss_http_bootstrap", TypeVoid)
		savedFn := l.currentFn
		savedBlock := l.currBlock
		savedLocals := l.varLocals
		savedLoopStack := l.loopStack
		l.currentFn = bootFn
		l.currBlock = bootFn.EntryBlock
		l.varLocals = make(map[string]Value)
		l.loopStack = nil
		for _, st := range bootstrapStmts {
			if l.currBlock.Terminator != nil {
				break
			}
			_ = l.lowerStatement(st)
		}
		if l.currBlock.Terminator == nil {
			l.currBlock.SetTerminator(&ReturnTerminator{Val: nil})
		}
		l.prog.AddFunction(bootFn)
		l.currentFn = savedFn
		l.currBlock = savedBlock
		l.varLocals = savedLocals
		l.loopStack = savedLoopStack
		hasBootstrapFn = true
	}

	// Synthesize compile-time startup bootstrap & debug reports
	// (__bootstrap__, __debug_summary__, __debug_routes__).
	{
		var fullSB strings.Builder
		mwSet := make(map[string]struct{})
		webCount, apiCount, wsCount := 0, 0, 0
		fullSB.WriteString(fmt.Sprintf("[Debug] Rutas registradas (%d):\n", len(routes)))
		for idx, rt := range routes {
			methodLabel := strings.ToUpper(rt.method)
			if rt.isWS {
				methodLabel = "WS"
				wsCount++
			} else if strings.HasPrefix(rt.path, "/api/") {
				apiCount++
			} else {
				webCount++
			}
			target := rt.controller + "@" + rt.action
			if closureCallees[idx] != "" {
				target = "<closure>"
			}
			if len(rt.middlewares) > 0 {
				for _, mw := range rt.middlewares {
					mwSet[mw] = struct{}{}
				}
				fullSB.WriteString(fmt.Sprintf("  [%-6s] %-38s -> %s  [middleware: %s]\n", methodLabel, rt.path, target, strings.Join(rt.middlewares, ", ")))
			} else {
				fullSB.WriteString(fmt.Sprintf("  [%-6s] %-38s -> %s\n", methodLabel, rt.path, target))
			}
		}
		mwListStr := "ninguno"
		if len(mwSet) > 0 {
			mws := make([]string, 0, len(mwSet))
			for mw := range mwSet {
				mws = append(mws, mw)
			}
			sort.Strings(mws)
			mwListStr = strings.Join(mws, ", ")
		}
		summaryStr := fmt.Sprintf("[Server] Rutas: %d (Web: %d, API: %d, WS: %d) | Middlewares en rutas (%d): %s",
			len(routes), webCount, apiCount, wsCount, len(mwSet), mwListStr)
		fullSB.WriteString(fmt.Sprintf("[Debug] Middlewares en rutas (%d): %s\n", len(mwSet), mwListStr))

		// 1. __bootstrap__: run route-file bootstrap (e.g. MiddlewareLoader) and return registered custom middlewares
		bootCheck := fn.NewTemp(TypeBool, "is_boot_dispatch")
		curr.AddInstruction(&CompareInst{
			Op:    OpCmpEq,
			Dest:  bootCheck,
			Left:  methodParam,
			Right: NewConstString("__bootstrap__"),
		})
		bootBlk := fn.NewBlock("boot_dispatch")
		afterBootBlk := fn.NewBlock("after_boot_dispatch")
		curr.SetTerminator(&BranchTerminator{
			Cond:       bootCheck,
			TrueBlock:  bootBlk,
			FalseBlock: afterBootBlk,
		})
		if hasBootstrapFn {
			bootBlk.AddInstruction(&CallInst{Callee: "__joss_http_bootstrap"})
		}
		mwListVal := fn.NewTemp(TypeString, "custom_mw_list")
		bootBlk.AddInstruction(&CallRuntimeInst{
			Dest: mwListVal,
			Func: "joss_router_list_custom_middlewares",
		})
		bootBlk.SetTerminator(&ReturnTerminator{Val: mwListVal})
		curr = afterBootBlk

		// 2. __debug_summary__: compact startup summary
		sumCheck := fn.NewTemp(TypeBool, "is_dbg_summary")
		curr.AddInstruction(&CompareInst{
			Op:    OpCmpEq,
			Dest:  sumCheck,
			Left:  methodParam,
			Right: NewConstString("__debug_summary__"),
		})
		sumBlk := fn.NewBlock("dbg_summary")
		afterSumBlk := fn.NewBlock("after_dbg_summary")
		curr.SetTerminator(&BranchTerminator{
			Cond:       sumCheck,
			TrueBlock:  sumBlk,
			FalseBlock: afterSumBlk,
		})
		sumBlk.SetTerminator(&ReturnTerminator{Val: NewConstString(summaryStr)})
		curr = afterSumBlk

		// 3. __debug_routes__: full verbose route table (opt-in via JOSS_DEBUG_ROUTES=1)
		dbgCheck := fn.NewTemp(TypeBool, "is_dbg_routes")
		curr.AddInstruction(&CompareInst{
			Op:    OpCmpEq,
			Dest:  dbgCheck,
			Left:  methodParam,
			Right: NewConstString("__debug_routes__"),
		})
		dbgBlk := fn.NewBlock("dbg_routes")
		routesBlk := fn.NewBlock("routes_dispatch")
		curr.SetTerminator(&BranchTerminator{
			Cond:       dbgCheck,
			TrueBlock:  dbgBlk,
			FalseBlock: routesBlk,
		})
		dbgBlk.SetTerminator(&ReturnTerminator{Val: NewConstString(fullSB.String())})
		curr = routesBlk
	}

	dispatched := 0

	for idx, rt := range routes {
		if rt.isWS {
			continue
		}

		isClosure := closureCallees[idx] != ""
		targetFnName := rt.controller + "_" + rt.action
		if isClosure {
			targetFnName = closureCallees[idx]
		}
		targetFn, exists := l.prog.Functions[targetFnName]
		if !exists {
			continue
		}

		numPlaceholders := strings.Count(rt.path, "{")
		expectedParams := len(targetFn.Params)
		if !isClosure {
			if len(targetFn.Params) < 1 {
				continue
			}
			expectedParams = len(targetFn.Params) - 1
		}
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

		execBlk := matchBlk
		if len(rt.middlewares) > 0 {
			mwResp := fn.NewTemp(TypeString, fmt.Sprintf("mw_resp_%d", idx))
			matchBlk.AddInstruction(&CallBuiltinInst{
				Dest:      mwResp,
				BuiltinID: "__joss_http_check_middleware",
				Args: []Value{
					NewConstString(strings.Join(rt.middlewares, "\n")),
					requestParam,
					sessionParam,
				},
			})
			mwEmpty := fn.NewTemp(TypeBool, fmt.Sprintf("mw_empty_%d", idx))
			matchBlk.AddInstruction(&CallRuntimeInst{
				Dest: mwEmpty,
				Func: "is_empty",
				Args: []Value{mwResp},
			})
			mwRejectBlk := fn.NewBlock(fmt.Sprintf("mw_reject_%d", idx))
			mwPassBlk := fn.NewBlock(fmt.Sprintf("mw_pass_%d", idx))
			matchBlk.SetTerminator(&BranchTerminator{
				Cond:       mwEmpty,
				TrueBlock:  mwPassBlk,
				FalseBlock: mwRejectBlk,
			})
			mwRejectBlk.SetTerminator(&ReturnTerminator{Val: mwResp})
			execBlk = mwPassBlk
		}

		var callArgs []Value
		if !isClosure {
			objType := PtrType(StructType(rt.controller, nil))
			ctrlTemp := fn.NewTemp(objType, fmt.Sprintf("ctrl_%d", idx))
			execBlk.AddInstruction(&NewObjectInst{
				Dest:      ctrlTemp,
				ClassName: rt.controller,
			})
			callArgs = []Value{ctrlTemp}
		}
		for pIdx := 0; pIdx < numPlaceholders; pIdx++ {
			pTemp := fn.NewTemp(TypeString, fmt.Sprintf("parg_%d_%d", idx, pIdx))
			execBlk.AddInstruction(&CallInst{
				Dest:   pTemp,
				Callee: "__joss_rt_route_param",
				Args:   []Value{pathParam, NewConstString(rt.path), NewConstInt(int64(pIdx), TypeI64)},
			})
			callArgs = append(callArgs, pTemp)
		}
		for pIdx := numPlaceholders; pIdx < expectedParams; pIdx++ {
			callArgs = append(callArgs, NewConstString(""))
		}

		if targetFn.ReturnType.Kind == TypeKindVoid {
			execBlk.AddInstruction(&CallInst{
				Callee: targetFnName,
				Args:   callArgs,
			})
			execBlk.SetTerminator(&ReturnTerminator{Val: NewConstString("")})
		} else if targetFn.ReturnType.Kind != TypeKindString {
			rawRet := fn.NewTemp(targetFn.ReturnType, fmt.Sprintf("rraw_%d", idx))
			execBlk.AddInstruction(&CallInst{
				Dest:   rawRet,
				Callee: targetFnName,
				Args:   callArgs,
			})
			strRet := fn.NewTemp(TypeString, fmt.Sprintf("rret_%d", idx))
			execBlk.AddInstruction(&CallRuntimeInst{
				Dest: strRet,
				Func: "to_string",
				Args: []Value{rawRet},
			})
			execBlk.SetTerminator(&ReturnTerminator{Val: strRet})
		} else {
			retTemp := fn.NewTemp(TypeString, fmt.Sprintf("rret_%d", idx))
			execBlk.AddInstruction(&CallInst{
				Dest:   retTemp,
				Callee: targetFnName,
				Args:   callArgs,
			})
			execBlk.SetTerminator(&ReturnTerminator{Val: retTemp})
		}

		curr = nextBlk
		dispatched++
	}

	if dispatched == 0 {
		return nil
	}

	// Return empty string on unmatched routes so HttpServer::handle can render
	// canonical HTTP/1.1 404 Not Found (JSON for APIs, custom error view or
	// anti-fingerprinting ErrorPages::render404 for HTML).
	curr.SetTerminator(&ReturnTerminator{
		Val: NewConstString(""),
	})

	l.prog.AddFunction(fn)
	return nil
}

func isRouterRegistrationStmt(stmt parser.Statement) bool {
	es, ok := stmt.(*parser.ExpressionStatement)
	if !ok || es == nil {
		return false
	}
	call, ok := es.Expression.(*parser.CallExpression)
	if !ok || call == nil {
		return false
	}
	member, ok := call.Function.(*parser.MemberExpression)
	if !ok || member == nil || member.Property == nil {
		return false
	}
	ident, ok := member.Left.(*parser.Identifier)
	if !ok || ident == nil || ident.Value != "Router" {
		return false
	}
	// Keep Router::registerMiddleware(...) so dynamic middlewares execute at startup.
	return !strings.EqualFold(member.Property.Value, "registerMiddleware")
}

func extractMiddlewareList(expr parser.Expression) []string {
	switch e := expr.(type) {
	case *parser.StringLiteral:
		if strings.TrimSpace(e.Value) != "" {
			return []string{strings.TrimSpace(e.Value)}
		}
	case *parser.ArrayLiteral:
		var out []string
		for _, el := range e.Elements {
			if sl, ok := el.(*parser.StringLiteral); ok && strings.TrimSpace(sl.Value) != "" {
				out = append(out, strings.TrimSpace(sl.Value))
			}
		}
		return out
	}
	return nil
}

func (l *Lowerer) extractProjectRoutesFromAST() []compiledRouteEntry {
	var routes []compiledRouteEntry
	if l.prep == nil {
		return routes
	}

	for _, unit := range l.prep.Units {
		if unit.Program == nil {
			continue
		}
		var mwStack [][]string
		flattenMW := func() []string {
			var all []string
			for _, frame := range mwStack {
				all = append(all, frame...)
			}
			return all
		}

		var walkStmts func(stmts []parser.Statement)
		walkStmts = func(stmts []parser.Statement) {
			for _, stmt := range stmts {
				switch s := stmt.(type) {
				case *parser.ExpressionStatement:
					if call, ok := s.Expression.(*parser.CallExpression); ok {
						routes = append(routes, extractRouteFromCallExprWithMW(call, &mwStack, flattenMW, walkStmts)...)
					}
				case *parser.BlockStatement:
					walkStmts(s.Statements)
				}
			}
		}
		walkStmts(unit.Program.Statements)
	}
	return routes
}

func appendRouteTarget(out []compiledRouteEntry, httpMethod, path string, targetExpr parser.Expression, mws []string, isWS bool) []compiledRouteEntry {
	if fnLit, ok := targetExpr.(*parser.FunctionLiteral); ok {
		return append(out, compiledRouteEntry{
			method:      httpMethod,
			path:        path,
			middlewares: append([]string(nil), mws...),
			closureFn:   fnLit,
			isWS:        isWS,
		})
	}
	if targetLit, ok := targetExpr.(*parser.StringLiteral); ok && strings.Contains(targetLit.Value, "@") {
		parts := strings.Split(targetLit.Value, "@")
		if len(parts) >= 2 {
			return append(out, compiledRouteEntry{
				method:      httpMethod,
				path:        path,
				controller:  parts[0],
				action:      parts[1],
				middlewares: append([]string(nil), mws...),
				isWS:        isWS,
			})
		}
	}
	return out
}

func extractRouteFromCallExprWithMW(call *parser.CallExpression, mwStack *[][]string, flattenMW func() []string, walkStmts func([]parser.Statement)) []compiledRouteEntry {
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
	case "middleware":
		if len(call.Arguments) >= 1 {
			if list := extractMiddlewareList(call.Arguments[0]); len(list) > 0 {
				*mwStack = append(*mwStack, list)
			}
		}
	case "end":
		if len(*mwStack) > 0 {
			*mwStack = (*mwStack)[:len(*mwStack)-1]
		}
	case "group":
		pushed := false
		if len(call.Arguments) >= 2 {
			if list := extractMiddlewareList(call.Arguments[0]); len(list) > 0 {
				*mwStack = append(*mwStack, list)
				pushed = true
			}
		}
		for _, arg := range call.Arguments {
			if fnLit, okFn := arg.(*parser.FunctionLiteral); okFn && fnLit.Body != nil {
				walkStmts(fnLit.Body.Statements)
			}
		}
		if pushed && len(*mwStack) > 0 {
			*mwStack = (*mwStack)[:len(*mwStack)-1]
		}
	case "get", "post", "put", "patch", "delete", "options":
		if len(call.Arguments) >= 2 {
			if pathLit, ok1 := call.Arguments[0].(*parser.StringLiteral); ok1 {
				out = appendRouteTarget(out, strings.ToUpper(method), pathLit.Value, call.Arguments[1], flattenMW(), false)
			}
		}
	case "any":
		if len(call.Arguments) >= 2 {
			if pathLit, ok1 := call.Arguments[0].(*parser.StringLiteral); ok1 {
				out = appendRouteTarget(out, "ANY", pathLit.Value, call.Arguments[1], flattenMW(), false)
			}
		}
	case "api":
		if len(call.Arguments) >= 2 {
			if pathLit, ok1 := call.Arguments[0].(*parser.StringLiteral); ok1 {
				mws := flattenMW()
				out = appendRouteTarget(out, "GET", pathLit.Value, call.Arguments[1], mws, false)
				out = appendRouteTarget(out, "POST", pathLit.Value, call.Arguments[1], mws, false)
			}
		}
	case "ws":
		if len(call.Arguments) >= 2 {
			if pathLit, ok1 := call.Arguments[0].(*parser.StringLiteral); ok1 {
				out = appendRouteTarget(out, "WS", pathLit.Value, call.Arguments[1], flattenMW(), true)
			}
		}
	case "match":
		if len(call.Arguments) >= 3 {
			methodsLit, _ := call.Arguments[0].(*parser.StringLiteral)
			pathLit, ok1 := call.Arguments[1].(*parser.StringLiteral)
			if !ok1 {
				return out
			}
			mws := flattenMW()
			if fnLit, okFn := call.Arguments[2].(*parser.FunctionLiteral); okFn {
				methods := []string{"ANY"}
				if methodsLit != nil && methodsLit.Value != "" {
					methods = strings.Split(methodsLit.Value, "|")
				}
				for _, m := range methods {
					out = append(out, compiledRouteEntry{
						method:      strings.ToUpper(strings.TrimSpace(m)),
						path:        pathLit.Value,
						middlewares: append([]string(nil), mws...),
						closureFn:   fnLit,
					})
				}
				return out
			}
			targetLit, ok2 := call.Arguments[2].(*parser.StringLiteral)
			if ok2 && strings.Contains(targetLit.Value, "@") {
				parts := strings.Split(targetLit.Value, "@")
				if len(parts) == 2 {
					methods := []string{"ANY"}
					if methodsLit != nil && methodsLit.Value != "" {
						methods = strings.Split(methodsLit.Value, "|")
					}
					for _, m := range methods {
						out = append(out, compiledRouteEntry{
							method:      strings.ToUpper(strings.TrimSpace(m)),
							path:        pathLit.Value,
							controller:  parts[0],
							action:      parts[1],
							middlewares: append([]string(nil), mws...),
						})
					}
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
							method:      m,
							path:        pathLit.Value,
							controller:  controller,
							action:      action,
							middlewares: append([]string(nil), mws...),
						})
					}
				}
			}
		}
	}
	return out
}

func encryptEmbeddedBlob(plain []byte) string {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	cipherBytes := make([]byte, len(plain))
	var state uint64 = 0x9e3779b97f4a7c15
	for i := 0; i < len(key); i++ {
		state ^= uint64(key[i]) << ((uint(i) % 8) * 8)
		state *= 0xbf58476d1ce4e5b9
	}
	for i, b := range plain {
		state ^= state >> 12
		state ^= state << 25
		state ^= state >> 27
		ks := byte((state*0x2545f4914f6cdd1d)>>56) ^ key[i%len(key)]
		cipherBytes[i] = b ^ ks
	}
	return "JOSSENV1:" + hex.EncodeToString(key) + ":" + hex.EncodeToString(cipherBytes)
}

// synthesizeEmbeddedAssetsFunction walks static asset directories (such as public/ and assets/)
// and private project data/secret directories (such as storage/secrets/, config/, and custom folders)
// and synthesizes the native IR function `__joss_get_embedded_asset(path: string): string`
// which serves public assets and decrypted private files directly from the standalone binary (.rodata section).
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
		".md": true, ".html": true,
	}

	isHotReloadDev := strings.TrimSpace(os.Getenv("JOSS_HOT_RELOAD_URL")) != ""
	if !isHotReloadDev {
		searchRoots := []string{"public", "assets"}
		for _, root := range searchRoots {
			if _, err := os.Stat(root); err != nil {
				continue
			}
			_ = filepath.Walk(root, func(path string, fi os.FileInfo, walkErr error) error {
				if walkErr != nil || fi == nil || fi.IsDir() {
					return nil
				}
				slashPath := filepath.ToSlash(path)
				parts := strings.Split(slashPath, "/")
				if len(parts) > 0 && ignoredDirs[parts[0]] {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(path))
				if (ext == ".pem" || ext == ".key" || ext == ".p12" || ext == ".pfx") && fi.Size() <= 2<<20 {
					if pemBytes, err := os.ReadFile(path); err == nil && len(pemBytes) > 0 {
						assets = append(assets, assetEntry{
							aliases: []string{"__joss_priv_" + slashPath},
							content: encryptEmbeddedBlob(pemBytes),
						})
					}
					return nil
				}
				if !staticExts[ext] || fi.Size() > 2<<20 {
					return nil
				}
				// Handle ephemeral compiled CSS under assets/.joss_cache/css/*.css
				if strings.HasPrefix(slashPath, "assets/.joss_cache/css/") && ext == ".css" {
					contentBytes, err := os.ReadFile(path)
					if err != nil {
						return nil
					}
					relCSS := strings.TrimPrefix(slashPath, "assets/.joss_cache/css/")
					canonical := "assets/css/" + relCSS
					assets = append(assets, assetEntry{
						aliases: []string{canonical, "/" + canonical},
						content: string(contentBytes),
					})
					return nil
				}
				// Never embed other ephemeral cache files (e.g., dev view chunks)
				if strings.Contains(slashPath, "/.joss_cache/") || strings.HasPrefix(slashPath, ".joss_cache/") {
					return nil
				}
				contentBytes, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				var aliases []string
				if root == "public" {
					// public/ maps directly to the web root "/" without a /public/ prefix.
					withoutRoot := strings.TrimPrefix(slashPath, "public/")
					if withoutRoot != "" && withoutRoot != slashPath {
						aliases = []string{withoutRoot, "/" + withoutRoot}
					}
				} else {
					// assets/ is served under /assets/...
					aliases = []string{slashPath, "/" + slashPath}
				}
				if len(aliases) > 0 {
					assets = append(assets, assetEntry{
						aliases: aliases,
						content: string(contentBytes),
					})
				}
				return nil
			})
		}
	}

	// Embed private project files and custom folders (e.g. storage/secrets/*, config/*, etc.)
	// encrypted under __joss_priv_<path> so the standalone binary works from any directory
	// without exposing secrets in plaintext or over HTTP.
	skipPrivateTopDirs := map[string]bool{
		".git": true, ".vscode": true, ".idea": true, "build": true, "dist": true,
		"vendor": true, "node_modules": true, ".gemini": true, ".codex": true,
		".agents": true, ".github": true, "tests": true, "test": true, "testdata": true,
		"app": true, "plugins": true, "public": true, "assets": true,
	}
	skipEphemeralPrefixes := []string{
		"storage/sessions/", "storage/logs/", "storage/cache/",
		"storage/framework/", "storage/tmp/", "storage/temp/",
	}
	skipPrivateExts := map[string]bool{
		".joss": true, ".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".o": true, ".obj": true, ".a": true, ".lib": true, ".pdb": true,
		".ilk": true, ".exp": true, ".map": true, ".jp": true, ".db": true,
		".sqlite": true, ".sqlite3": true, ".log": true, ".tmp": true, ".bak": true,
		".zip": true, ".tar": true, ".gz": true, ".7z": true, ".rar": true,
		".mp4": true, ".mov": true, ".avi": true,
	}
	isJossProjectDir := false
	for _, marker := range []string{"joss.yaml", "env.joss", ".env", "routes.joss", "storage"} {
		if _, err := os.Stat(marker); err == nil {
			isJossProjectDir = true
			break
		}
	}
	const maxPrivateFiles = 256
	const maxPrivateTotalBytes int64 = 16 << 20
	privateFilesCount := 0
	var privateTotalBytes int64 = 0
	if isJossProjectDir && !isHotReloadDev {
		if entries, err := os.ReadDir("."); err == nil {
			for _, entry := range entries {
				if privateFilesCount >= maxPrivateFiles || privateTotalBytes >= maxPrivateTotalBytes {
					break
				}
				name := entry.Name()
				if skipPrivateTopDirs[name] || name == "env.joss" || name == ".env" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "build_") || strings.HasPrefix(name, "build-") {
					continue
				}
				if !entry.IsDir() {
					continue
				}
				_ = filepath.Walk(name, func(path string, fi os.FileInfo, walkErr error) error {
					if walkErr != nil || fi == nil {
						return nil
					}
					if privateFilesCount >= maxPrivateFiles || privateTotalBytes >= maxPrivateTotalBytes {
						return filepath.SkipDir
					}
					slashPath := filepath.ToSlash(path)
					if fi.IsDir() {
						for _, ep := range skipEphemeralPrefixes {
							if slashPath+"/" == ep || strings.HasPrefix(slashPath+"/", ep) {
								return filepath.SkipDir
							}
						}
						return nil
					}
					for _, ep := range skipEphemeralPrefixes {
						if strings.HasPrefix(slashPath, ep) {
							return nil
						}
					}
					ext := strings.ToLower(filepath.Ext(path))
					if skipPrivateExts[ext] || fi.Size() <= 0 || fi.Size() > 2<<20 {
						return nil
					}
					if privateTotalBytes+fi.Size() > maxPrivateTotalBytes {
						return nil
					}
					rawBytes, err := os.ReadFile(path)
					if err != nil || len(rawBytes) == 0 {
						return nil
					}
					assets = append(assets, assetEntry{
						aliases: []string{"__joss_priv_" + slashPath},
						content: encryptEmbeddedBlob(rawBytes),
					})
					privateFilesCount++
					privateTotalBytes += int64(len(rawBytes))
					return nil
				})
			}
		}
	}

	// Encrypt and embed project env.joss / .env so standalone binaries carry
	// protected configuration without exposing plaintext secrets in the executable.
	for _, envFile := range []string{"env.joss", ".env"} {
		if envBytes, err := os.ReadFile(envFile); err == nil && len(envBytes) > 0 {
			assets = append(assets, assetEntry{
				aliases: []string{"__joss_embedded_env_enc"},
				content: encryptEmbeddedBlob(envBytes),
			})
			break
		}
	}

	if len(assets) == 0 {
		return nil
	}

	fn := NewFunction("__joss_get_embedded_asset", TypeString)
	pathParam := fn.AddParam("path", TypeString)
	curr := fn.EntryBlock

	for idx, a := range assets {
		matchBlk := fn.NewBlock(fmt.Sprintf("asset_match_%d", idx))
		if strings.IndexByte(a.content, 0) >= 0 {
			bytesTmp := fn.NewTemp(TypeString, fmt.Sprintf("asset_bytes_%d", idx))
			matchBlk.AddInstruction(&CallRuntimeInst{
				Func: "joss_bytes_new",
				Args: []Value{
					NewConstString(a.content),
					NewConstInt(int64(len(a.content)), TypeI64),
				},
				Dest: bytesTmp,
			})
			matchBlk.SetTerminator(&ReturnTerminator{Val: bytesTmp})
		} else {
			matchBlk.SetTerminator(&ReturnTerminator{Val: NewConstString(a.content)})
		}

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
	if mainFn := l.prog.Functions["main"]; mainFn != nil && mainFn.EntryBlock != nil {
		regInst := &CallInst{
			Callee: "joss_register_embedded_asset_resolver",
			Args:   []Value{NewConstFnPtr("__joss_get_embedded_asset", TypeMixed)},
		}
		mainFn.EntryBlock.Instructions = append([]Instruction{regInst}, mainFn.EntryBlock.Instructions...)
	}
	return nil
}

type pluginCallableDecl struct {
	pluginName       string
	unitPath         string
	unitWasProcessed bool
	aliases          []string
	rawIRName        string
	actualIRName     string
	method           *parser.MethodStatement
	classStmt        *parser.ClassStatement
	className        string
	isStatic         bool
	requiresCtorArgs bool
	ctorMethod       *parser.MethodStatement
}

type pluginCallUsage struct {
	hasPluginCall   bool
	dynamicAll      bool
	dynamicPlugin   map[string]bool
	targetedPlugins map[string]bool
	targetedSymbols map[string]map[string]bool
}

func extractPluginNameFromPath(rawPath string) (string, bool) {
	norm := strings.ReplaceAll(rawPath, "\\", "/")
	if !strings.HasPrefix(norm, "<plugin:") {
		return "", false
	}
	rest := strings.TrimPrefix(norm, "<plugin:")
	if slash := strings.Index(rest, "/"); slash != -1 {
		name := strings.TrimSpace(rest[:slash])
		return name, name != ""
	}
	name := strings.TrimSpace(strings.TrimSuffix(rest, ">"))
	return name, name != ""
}

func sanitizePluginIRIdent(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "plugin"
	}
	return b.String()
}

func isPluginDispatchCallee(callee string) bool {
	switch callee {
	case "Plugin_call", "Plugin_stream", "Plugin_load",
		"__joss_host_Plugin_call", "__joss_host_Plugin_stream", "__joss_host_Plugin_load":
		return true
	default:
		return false
	}
}

func (l *Lowerer) collectDeclaredPluginIRFunctions() map[string]bool {
	out := make(map[string]bool)
	if l.prep == nil || l.prog == nil {
		return out
	}
	recordIfPresent := func(name string) {
		if name != "" && l.prog.Functions[name] != nil {
			out[name] = true
		}
	}
	for _, unit := range l.prep.Units {
		pluginName, isPlugin := extractPluginNameFromPath(unit.Path)
		if !isPlugin || unit.Program == nil {
			continue
		}
		prefix := "__joss_plugin_" + sanitizePluginIRIdent(pluginName) + "_"
		for _, stmt := range unit.Program.Statements {
			switch s := stmt.(type) {
			case *parser.MethodStatement:
				if s.Name != nil {
					recordIfPresent(s.Name.Value)
					recordIfPresent(prefix + s.Name.Value)
				}
			case *parser.ClassStatement:
				if s.Name == nil || s.Body == nil {
					continue
				}
				for _, member := range s.Body.Statements {
					if m, ok := member.(*parser.MethodStatement); ok && m.Name != nil {
						irName := s.Name.Value + "_" + m.Name.Value
						recordIfPresent(irName)
						recordIfPresent(prefix + irName)
					} else if initStmt, ok := member.(*parser.InitStatement); ok {
						mName := "Init"
						if initStmt.Name != nil && initStmt.Name.Value != "" {
							mName = initStmt.Name.Value
						}
						irName := s.Name.Value + "_" + mName
						recordIfPresent(irName)
						recordIfPresent(prefix + irName)
					}
				}
			}
		}
	}
	return out
}

func (l *Lowerer) detectPluginCallUsage() pluginCallUsage {
	usage := pluginCallUsage{
		dynamicPlugin:   make(map[string]bool),
		targetedPlugins: make(map[string]bool),
		targetedSymbols: make(map[string]map[string]bool),
	}
	if l.reachGraph != nil && len(l.reachGraph.LivePluginSymbols) > 0 {
		usage.hasPluginCall = true
		for pName, syms := range l.reachGraph.LivePluginSymbols {
			usage.targetedPlugins[pName] = true
			if usage.targetedSymbols[pName] == nil {
				usage.targetedSymbols[pName] = make(map[string]bool)
			}
			for sym := range syms {
				usage.targetedSymbols[pName][sym] = true
			}
		}
	}
	if l.prog == nil {
		return usage
	}
	recordCallArgs := func(method string, args []Value) {
		usage.hasPluginCall = true
		if method == "load" {
			usage.dynamicAll = true
			return
		}
		if len(args) == 0 {
			usage.dynamicAll = true
			return
		}
		plugLit, okPlug := args[0].(*ConstString)
		if !okPlug || plugLit.Val == "" {
			usage.dynamicAll = true
			return
		}
		pName := plugLit.Val
		usage.targetedPlugins[pName] = true
		if len(args) < 2 {
			usage.dynamicPlugin[pName] = true
			return
		}
		fnLit, okFn := args[1].(*ConstString)
		if !okFn || fnLit.Val == "" {
			usage.dynamicPlugin[pName] = true
			return
		}
		if usage.targetedSymbols[pName] == nil {
			usage.targetedSymbols[pName] = make(map[string]bool)
		}
		usage.targetedSymbols[pName][fnLit.Val] = true
	}

	var scanBlocks func([]*BasicBlock)
	scanBlocks = func(blocks []*BasicBlock) {
		for _, blk := range blocks {
			for _, inst := range blk.Instructions {
				switch i := inst.(type) {
				case *CallHostServiceInst:
					if i.ClassName == "Plugin" && (i.MethodName == "call" || i.MethodName == "stream" || i.MethodName == "load") {
						recordCallArgs(i.MethodName, i.Args)
					}
				case *CallInst:
					if isPluginDispatchCallee(i.Callee) {
						method := "call"
						if strings.HasSuffix(i.Callee, "_load") {
							method = "load"
						} else if strings.HasSuffix(i.Callee, "_stream") {
							method = "stream"
						}
						recordCallArgs(method, i.Args)
					}
				case *TryCatchInst:
					scanBlocks(i.TryBlocks)
					scanBlocks(i.CatchBlocks)
				}
			}
		}
	}

	fnNames := make([]string, 0, len(l.prog.Functions))
	for fnName := range l.prog.Functions {
		fnNames = append(fnNames, fnName)
	}
	sort.Strings(fnNames)
	for _, fnName := range fnNames {
		if fn := l.prog.Functions[fnName]; fn != nil {
			scanBlocks(fn.Blocks)
		}
	}
	return usage
}

func pluginClassConstructorInfo(cs *parser.ClassStatement) (bool, *parser.MethodStatement) {
	if cs == nil || cs.Body == nil {
		return false, nil
	}
	requiresCtorArgs := false
	var ctor *parser.MethodStatement
	for _, member := range cs.Body.Statements {
		switch m := member.(type) {
		case *parser.InitStatement:
			req := 0
			for _, p := range m.Parameters {
				if p != nil && p.DefaultValue == nil {
					req++
				}
			}
			if req > 0 {
				requiresCtorArgs = true
			}
			mName := "Init"
			if m.Name != nil && m.Name.Value != "" {
				mName = m.Name.Value
			}
			if ctor == nil {
				ctor = &parser.MethodStatement{
					Token:      m.Token,
					Name:       &parser.Identifier{Value: mName},
					Parameters: m.Parameters,
					Body:       m.Body,
					ReturnType: parser.Token{Literal: "void"},
				}
			}
		case *parser.MethodStatement:
			if m.Name == nil || (m.Name.Value != "Init" && m.Name.Value != "constructor") {
				continue
			}
			req := 0
			for _, p := range m.Parameters {
				if p != nil && p.DefaultValue == nil {
					req++
				}
			}
			if req > 0 {
				requiresCtorArgs = true
			}
			if ctor == nil && m.Body != nil {
				ctor = m
			}
		}
	}
	return requiresCtorArgs, ctor
}

func (l *Lowerer) collectPluginDeclarations() []*pluginCallableDecl {
	if l.prep == nil {
		return nil
	}
	var decls []*pluginCallableDecl
	seenAliasPerPlugin := make(map[string]map[string]bool)

	for _, unit := range l.prep.Units {
		pluginName, isPlugin := extractPluginNameFromPath(unit.Path)
		if !isPlugin || unit.Program == nil {
			continue
		}
		if seenAliasPerPlugin[pluginName] == nil {
			seenAliasPerPlugin[pluginName] = make(map[string]bool)
		}
		unitProcessed := l.reachGraph == nil || l.reachGraph.IsFileReachable(unit.Path)

		// Collect top-level functions first so short-name aliases prefer them over class methods.
		for _, stmt := range unit.Program.Statements {
			m, ok := stmt.(*parser.MethodStatement)
			if !ok || m.Name == nil || m.IsAbstract || m.Body == nil {
				continue
			}
			fnName := m.Name.Value
			seenAliasPerPlugin[pluginName][fnName] = true
			decls = append(decls, &pluginCallableDecl{
				pluginName:       pluginName,
				unitPath:         unit.Path,
				unitWasProcessed: unitProcessed,
				aliases:          []string{fnName},
				rawIRName:        fnName,
				method:           m,
			})
		}

		for _, stmt := range unit.Program.Statements {
			cs, ok := stmt.(*parser.ClassStatement)
			if !ok || cs.Name == nil || cs.Body == nil {
				continue
			}
			requiresCtorArgs, ctorMethod := pluginClassConstructorInfo(cs)
			for _, member := range cs.Body.Statements {
				m, ok := member.(*parser.MethodStatement)
				if !ok || m.Name == nil || m.IsAbstract || m.Body == nil {
					continue
				}
				if m.Name.Value == "Init" || m.Name.Value == "constructor" {
					continue
				}
				qualAlias := cs.Name.Value + "::" + m.Name.Value
				aliases := []string{qualAlias}
				if !seenAliasPerPlugin[pluginName][m.Name.Value] {
					seenAliasPerPlugin[pluginName][m.Name.Value] = true
					aliases = append(aliases, m.Name.Value)
				}
				seenAliasPerPlugin[pluginName][qualAlias] = true
				decls = append(decls, &pluginCallableDecl{
					pluginName:       pluginName,
					unitPath:         unit.Path,
					unitWasProcessed: unitProcessed,
					aliases:          aliases,
					rawIRName:        cs.Name.Value + "_" + m.Name.Value,
					method:           m,
					classStmt:        cs,
					className:        cs.Name.Value,
					isStatic:         m.IsStatic,
					requiresCtorArgs: requiresCtorArgs,
					ctorMethod:       ctorMethod,
				})
			}
		}
	}
	return decls
}

func (l *Lowerer) lowerPluginDeclSafely(decl *pluginCallableDecl, forceUnique bool) (string, error) {
	if decl == nil || decl.method == nil {
		return "", nil
	}
	if !forceUnique && decl.actualIRName != "" && l.prog.Functions[decl.actualIRName] != nil {
		return decl.actualIRName, nil
	}
	if !forceUnique && decl.unitWasProcessed && l.prog.Functions[decl.rawIRName] != nil {
		decl.actualIRName = decl.rawIRName
		return decl.rawIRName, nil
	}

	savedFn := l.currentFn
	savedBlock := l.currBlock
	savedLocals := l.varLocals
	savedLoops := l.loopStack
	defer func() {
		l.currentFn = savedFn
		l.currBlock = savedBlock
		l.varLocals = savedLocals
		l.loopStack = savedLoops
	}()

	existing := l.prog.Functions[decl.rawIRName]
	var err error
	if decl.className != "" {
		err = l.lowerClassMethod(decl.className, decl.method)
	} else {
		err = l.lowerMethod(decl.method)
	}
	if err != nil {
		if existing != nil {
			l.prog.Functions[decl.rawIRName] = existing
		}
		return "", err
	}

	lowered := l.prog.Functions[decl.rawIRName]
	if lowered == nil {
		return "", nil
	}
	if existing != nil {
		// Avoid overwriting an existing function/native owner from another unit or plugin.
		l.prog.Functions[decl.rawIRName] = existing
		uniqueName := "__joss_plugin_" + sanitizePluginIRIdent(decl.pluginName) + "_" + decl.rawIRName
		lowered.Name = uniqueName
		l.prog.Functions[uniqueName] = lowered
		decl.actualIRName = uniqueName
		return uniqueName, nil
	}
	decl.actualIRName = decl.rawIRName
	return decl.rawIRName, nil
}

func (l *Lowerer) collectSameUnitPluginCallees(fn *Function) ([]string, []string) {
	if fn == nil {
		return nil, nil
	}
	var callees []string
	var ctors []string
	var scan func([]*BasicBlock)
	scan = func(blocks []*BasicBlock) {
		for _, blk := range blocks {
			for _, inst := range blk.Instructions {
				switch i := inst.(type) {
				case *CallInst:
					callees = append(callees, i.Callee)
					for _, arg := range i.Args {
						if fp, ok := arg.(*ConstFnPtr); ok {
							callees = append(callees, fp.FnName)
						}
					}
				case *CallHostServiceInst:
					callees = append(callees, i.ClassName+"_"+i.MethodName)
				case *NewObjectInst:
					ctors = append(ctors, i.ClassName+"_Init", i.ClassName+"_constructor")
				case *TryCatchInst:
					scan(i.TryBlocks)
					scan(i.CatchBlocks)
				}
			}
		}
	}
	scan(fn.Blocks)
	return callees, ctors
}

func declMatchesPluginUsage(decl *pluginCallableDecl, usage pluginCallUsage) bool {
	if decl.unitWasProcessed && decl.rawIRName != "" {
		// Already lowered directly by LowerProgram (e.g. direct call or reachGraph == nil).
		return true
	}
	if !usage.hasPluginCall || usage.dynamicAll {
		return true
	}
	if !usage.targetedPlugins[decl.pluginName] {
		return false
	}
	if usage.dynamicPlugin[decl.pluginName] {
		return true
	}
	syms := usage.targetedSymbols[decl.pluginName]
	for _, alias := range decl.aliases {
		if syms[alias] {
			return true
		}
	}
	if syms[decl.rawIRName] {
		return true
	}
	if decl.method != nil && decl.method.Name != nil && syms[decl.method.Name.Value] {
		return true
	}
	return false
}

// synthesizePluginDispatchFunction lowers reachable plugin entrypoints from
// `<plugin:...>` units, synthesizes `__joss_plugin_dispatch(plugin, fn, args)`
// compatible with `JossPluginDispatchFn` in `runtime/native/joss_web_rt.c`, and
// registers it at the start of `main` and `__joss_http_dispatch`.
func (l *Lowerer) synthesizePluginDispatchFunction() error {
	if l.prog == nil || l.prog.Functions["__joss_plugin_dispatch"] != nil {
		return nil
	}

	usage := l.detectPluginCallUsage()
	allDecls := l.collectPluginDeclarations()
	if len(allDecls) == 0 && !usage.hasPluginCall {
		return nil
	}

	// Index declarations by unitPath + rawIRName for transitive closure lowering.
	byUnitAndRaw := make(map[string]map[string]*pluginCallableDecl)
	ctorsByUnitAndName := make(map[string]map[string]*pluginCallableDecl)
	for _, d := range allDecls {
		if byUnitAndRaw[d.unitPath] == nil {
			byUnitAndRaw[d.unitPath] = make(map[string]*pluginCallableDecl)
		}
		byUnitAndRaw[d.unitPath][d.rawIRName] = d
		if d.className != "" && d.ctorMethod != nil {
			if ctorsByUnitAndName[d.unitPath] == nil {
				ctorsByUnitAndName[d.unitPath] = make(map[string]*pluginCallableDecl)
			}
			ctorRaw := d.className + "_" + d.ctorMethod.Name.Value
			if ctorsByUnitAndName[d.unitPath][ctorRaw] == nil {
				ctorsByUnitAndName[d.unitPath][ctorRaw] = &pluginCallableDecl{
					pluginName:       d.pluginName,
					unitPath:         d.unitPath,
					unitWasProcessed: d.unitWasProcessed,
					rawIRName:        ctorRaw,
					method:           d.ctorMethod,
					classStmt:        d.classStmt,
					className:        d.className,
				}
			}
		}
	}

	var dispatchDecls []*pluginCallableDecl
	loweredQueue := make([]*pluginCallableDecl, 0)
	queued := make(map[*pluginCallableDecl]bool)

	enqueueDecl := func(d *pluginCallableDecl) {
		if d == nil || queued[d] {
			return
		}
		queued[d] = true
		loweredQueue = append(loweredQueue, d)
	}

	for _, d := range allDecls {
		if declMatchesPluginUsage(d, usage) || (d.unitWasProcessed && l.prog.Functions[d.rawIRName] != nil) {
			if d.className != "" && !d.isStatic && d.requiresCtorArgs {
				continue
			}
			dispatchDecls = append(dispatchDecls, d)
			enqueueDecl(d)
			if d.className != "" && !d.isStatic && d.ctorMethod != nil && !d.requiresCtorArgs {
				if ctors := ctorsByUnitAndName[d.unitPath]; ctors != nil {
					enqueueDecl(ctors[d.className+"_"+d.ctorMethod.Name.Value])
				}
			}
		}
	}

	if len(dispatchDecls) == 0 && !usage.hasPluginCall {
		return nil
	}

	// Lower selected plugin declarations and their transitive callees within each plugin unit.
	for head := 0; head < len(loweredQueue); head++ {
		d := loweredQueue[head]
		irName, err := l.lowerPluginDeclSafely(d, false)
		if err != nil {
			return err
		}
		if irName == "" {
			continue
		}
		fn := l.prog.Functions[irName]
		callees, ctorNames := l.collectSameUnitPluginCallees(fn)
		unitMap := byUnitAndRaw[d.unitPath]
		ctorMap := ctorsByUnitAndName[d.unitPath]
		for _, cName := range callees {
			if targetDecl := unitMap[cName]; targetDecl != nil {
				enqueueDecl(targetDecl)
			}
		}
		for _, cName := range ctorNames {
			if ctorDecl := ctorMap[cName]; ctorDecl != nil {
				enqueueDecl(ctorDecl)
			}
		}
	}

	fn := NewFunction("__joss_plugin_dispatch", TypeMixed)
	pluginParam := fn.AddParam("plugin", TypeString)
	fnParam := fn.AddParam("fn", TypeString)
	argsParam := fn.AddParam("args", ArrayType(TypeMixed))

	curr := fn.EntryBlock

	for idx, d := range dispatchDecls {
		irFnName := d.actualIRName
		if irFnName == "" {
			irFnName = d.rawIRName
		}
		targetFn := l.prog.Functions[irFnName]
		if targetFn == nil {
			continue
		}
		if d.className != "" && !d.isStatic && d.requiresCtorArgs {
			continue
		}

		plugCmp := fn.NewTemp(TypeBool, fmt.Sprintf("pcmp_%d", idx))
		curr.AddInstruction(&CompareInst{
			Op:    OpCmpEq,
			Dest:  plugCmp,
			Left:  pluginParam,
			Right: NewConstString(d.pluginName),
		})

		var fnMatch Value
		for aIdx, alias := range d.aliases {
			fc := fn.NewTemp(TypeBool, fmt.Sprintf("fcmp_%d_%d", idx, aIdx))
			curr.AddInstruction(&CompareInst{
				Op:    OpCmpEq,
				Dest:  fc,
				Left:  fnParam,
				Right: NewConstString(alias),
			})
			if fnMatch == nil {
				fnMatch = fc
			} else {
				orTmp := fn.NewTemp(TypeBool, fmt.Sprintf("for_%d_%d", idx, aIdx))
				curr.AddInstruction(&BinaryInst{
					Op:    OpOr,
					Dest:  orTmp,
					Left:  fnMatch,
					Right: fc,
				})
				fnMatch = orTmp
			}
		}
		if fnMatch == nil {
			continue
		}

		bothCmp := fn.NewTemp(TypeBool, fmt.Sprintf("pfcmp_%d", idx))
		curr.AddInstruction(&BinaryInst{
			Op:    OpAnd,
			Dest:  bothCmp,
			Left:  plugCmp,
			Right: fnMatch,
		})

		matchBlk := fn.NewBlock(fmt.Sprintf("plugin_match_%d", idx))
		nextBlk := fn.NewBlock(fmt.Sprintf("plugin_next_%d", idx))
		curr.SetTerminator(&BranchTerminator{
			Cond:       bothCmp,
			TrueBlock:  matchBlk,
			FalseBlock: nextBlk,
		})

		var callArgs []Value
		paramStartIdx := 0
		if d.className != "" && !d.isStatic && len(targetFn.Params) > 0 &&
			(targetFn.Params[0].Name == "this" || targetFn.Params[0].Name == "$this") {
			objType := PtrType(StructType(d.className, nil))
			objTemp := fn.NewTemp(objType, fmt.Sprintf("pobj_%d", idx))
			matchBlk.AddInstruction(&NewObjectInst{
				Dest:      objTemp,
				ClassName: d.className,
			})
			l.emitPluginDefaultFields(matchBlk, objTemp, d.classStmt)
			if d.ctorMethod != nil {
				ctorKey := d.className + "_" + d.ctorMethod.Name.Value
				ctorIR := ctorKey
				if ctorDecl := ctorsByUnitAndName[d.unitPath][ctorKey]; ctorDecl != nil && ctorDecl.actualIRName != "" {
					ctorIR = ctorDecl.actualIRName
				}
				if ctorFn := l.prog.Functions[ctorIR]; ctorFn != nil && len(ctorFn.Params) <= 1 {
					var ctorArgs []Value
					if len(ctorFn.Params) == 1 {
						ctorArgs = []Value{objTemp}
					}
					matchBlk.AddInstruction(&CallInst{
						Callee: ctorIR,
						Args:   ctorArgs,
					})
				}
			}
			callArgs = append(callArgs, objTemp)
			paramStartIdx = 1
		}

		execBlk := matchBlk
		for pIdx := paramStartIdx; pIdx < len(targetFn.Params); pIdx++ {
			argIdx := pIdx - paramStartIdx
			paramType := targetFn.Params[pIdx].Type()
			var defVal Value
			if pIdx < len(targetFn.DefaultArgs) {
				defVal = targetFn.DefaultArgs[pIdx]
			}
			argVal, nextExecBlk := l.emitUnpackPluginDispatchArg(fn, execBlk, argsParam, idx, argIdx, paramType, defVal)
			execBlk = nextExecBlk
			callArgs = append(callArgs, argVal)
		}

		if targetFn.ReturnType.Kind == TypeKindVoid {
			execBlk.AddInstruction(&CallInst{
				Callee: irFnName,
				Args:   callArgs,
			})
			execBlk.SetTerminator(&ReturnTerminator{Val: NewConstInt(0, TypeMixed)})
		} else {
			resTemp := fn.NewTemp(targetFn.ReturnType, fmt.Sprintf("pres_%d", idx))
			execBlk.AddInstruction(&CallInst{
				Dest:   resTemp,
				Callee: irFnName,
				Args:   callArgs,
			})
			execBlk.SetTerminator(&ReturnTerminator{Val: resTemp})
		}

		curr = nextBlk
	}

	curr.SetTerminator(&ReturnTerminator{Val: NewConstInt(0, TypeMixed)})
	l.prog.AddFunction(fn)

	for _, entryName := range []string{"main", "__joss_http_dispatch"} {
		if entryFn := l.prog.Functions[entryName]; entryFn != nil && entryFn.EntryBlock != nil {
			regInst := &CallRuntimeInst{
				Func: "joss_register_plugin_dispatcher",
				Args: []Value{NewConstFnPtr("__joss_plugin_dispatch", PtrType(TypeVoid))},
			}
			entryFn.EntryBlock.Instructions = append([]Instruction{regInst}, entryFn.EntryBlock.Instructions...)
		}
	}
	return nil
}

func (l *Lowerer) emitPluginDefaultFields(blk *BasicBlock, objTemp Value, cs *parser.ClassStatement) {
	if blk == nil || cs == nil || cs.Body == nil {
		return
	}
	for _, member := range cs.Body.Statements {
		letStmt, ok := member.(*parser.LetStatement)
		if !ok || letStmt.IsStatic || letStmt.Name == nil || letStmt.Value == nil {
			continue
		}
		propType := l.mapType(letStmt.Token.Literal)
		val := l.lowerConstantDefault(letStmt.Value, propType)
		if val != nil && !isZeroInitConstant(val) {
			blk.AddInstruction(&StoreFieldInst{
				Val:       val,
				Obj:       objTemp,
				FieldName: cleanVarName(letStmt.Name.Value),
			})
		}
	}
}

func hasNonZeroPluginDefault(defVal Value) bool {
	if defVal == nil || isZeroInitConstant(defVal) {
		return false
	}
	if cs, ok := defVal.(*ConstString); ok && cs.Val == "" {
		return false
	}
	return true
}

func (l *Lowerer) emitUnpackPluginDispatchArg(fn *Function, blk *BasicBlock, argsParam Value, branchIdx, argIdx int, paramType Type, defVal Value) (Value, *BasicBlock) {
	extractArg := func(targetBlk *BasicBlock) Value {
		if paramType.Kind == TypeKindString {
			argStr := fn.NewTemp(TypeString, fmt.Sprintf("parg_%d_%d", branchIdx, argIdx))
			targetBlk.AddInstruction(&ArrayGetInst{
				Dest:  argStr,
				Array: argsParam,
				Index: NewConstInt(int64(argIdx), TypeI64),
			})
			return argStr
		}
		raw := fn.NewTemp(TypeI64, fmt.Sprintf("parg_raw_%d_%d", branchIdx, argIdx))
		targetBlk.AddInstruction(&ArrayGetInst{
			Dest:  raw,
			Array: argsParam,
			Index: NewConstInt(int64(argIdx), TypeI64),
		})
		if paramType.Kind == TypeKindMixed {
			return raw
		}
		typedVal := fn.NewTemp(paramType, fmt.Sprintf("parg_%d_%d", branchIdx, argIdx))
		targetBlk.AddInstruction(&UnboxValueInst{
			Dest: typedVal,
			Val:  raw,
		})
		return typedVal
	}

	if !hasNonZeroPluginDefault(defVal) {
		return extractArg(blk), blk
	}

	argPtr := fn.NewTemp(PtrType(paramType), fmt.Sprintf("parg_ptr_%d_%d", branchIdx, argIdx))
	fn.EntryBlock.Instructions = append([]Instruction{&AllocaInst{Dest: argPtr, AllocType: paramType}}, fn.EntryBlock.Instructions...)

	lenTmp := fn.NewTemp(TypeI64, fmt.Sprintf("pargs_len_%d_%d", branchIdx, argIdx))
	blk.AddInstruction(&CallRuntimeInst{
		Dest: lenTmp,
		Func: "joss_arr_len",
		Args: []Value{argsParam},
	})
	hasArg := fn.NewTemp(TypeBool, fmt.Sprintf("phas_arg_%d_%d", branchIdx, argIdx))
	blk.AddInstruction(&CompareInst{
		Op:    OpCmpGt,
		Dest:  hasArg,
		Left:  lenTmp,
		Right: NewConstInt(int64(argIdx), TypeI64),
	})

	haveBlk := fn.NewBlock(fmt.Sprintf("parg_have_%d_%d", branchIdx, argIdx))
	defBlk := fn.NewBlock(fmt.Sprintf("parg_def_%d_%d", branchIdx, argIdx))
	contBlk := fn.NewBlock(fmt.Sprintf("parg_cont_%d_%d", branchIdx, argIdx))
	blk.SetTerminator(&BranchTerminator{
		Cond:       hasArg,
		TrueBlock:  haveBlk,
		FalseBlock: defBlk,
	})

	extracted := extractArg(haveBlk)
	haveBlk.AddInstruction(&StoreInst{Val: extracted, Ptr: argPtr})
	haveBlk.SetTerminator(&JumpTerminator{Target: contBlk})

	defBlk.AddInstruction(&StoreInst{Val: defVal, Ptr: argPtr})
	defBlk.SetTerminator(&JumpTerminator{Target: contBlk})

	loaded := fn.NewTemp(paramType, fmt.Sprintf("parg_val_%d_%d", branchIdx, argIdx))
	contBlk.AddInstruction(&LoadInst{Dest: loaded, Ptr: argPtr})
	return loaded, contBlk
}
