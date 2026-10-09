package analyzer

import (
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
)

// RuntimeCapability flags features required from the Go runtime environment.
type RuntimeCapability string

const (
	CapCore       RuntimeCapability = "core"
	CapIO         RuntimeCapability = "io"
	CapServer     RuntimeCapability = "server"
	CapHTTP       RuntimeCapability = "http"
	CapDatabase   RuntimeCapability = "database"
	CapSQLite     RuntimeCapability = "sqlite"
	CapPostgres   RuntimeCapability = "postgres"
	CapMySQL      RuntimeCapability = "mysql"
	CapSQLServer  RuntimeCapability = "sqlserver"
	CapOCI        RuntimeCapability = "oci"
	CapGUI        RuntimeCapability = "gui"
	CapWebSocket  RuntimeCapability = "websocket"
	CapJSON       RuntimeCapability = "json"
	CapMarkdown   RuntimeCapability = "markdown"
	CapFilesystem RuntimeCapability = "filesystem"
)

// ReachabilityOptions governs symbols and files retention.
type ReachabilityOptions struct {
	Entrypoint  string
	KeepClasses []string
	KeepSymbols []string
	KeepFiles   []string
	RouteFiles  []string
}

// ReachabilityGraph tracks files, classes, methods, and functions that are live.
type ReachabilityGraph struct {
	Entrypoint          string
	LiveFiles           map[string]bool
	LiveClasses         map[string]bool
	InstantiatedClasses map[string]bool
	LiveMethods         map[string]map[string]bool // className -> methodName -> true
	LiveFunctions       map[string]bool
	LiveSymbols         map[string]bool
	LivePluginSymbols   map[string]map[string]bool // pluginName -> symbolName -> true
	RuntimeCapabilities map[RuntimeCapability]bool
}

// NewReachabilityGraph allocates an initialized reachability graph container.
func NewReachabilityGraph(entrypoint string) *ReachabilityGraph {
	return &ReachabilityGraph{
		Entrypoint:          canonicalSourcePath(entrypoint),
		LiveFiles:           make(map[string]bool),
		LiveClasses:         make(map[string]bool),
		InstantiatedClasses: make(map[string]bool),
		LiveMethods:         make(map[string]map[string]bool),
		LiveFunctions:       make(map[string]bool),
		LiveSymbols:         make(map[string]bool),
		LivePluginSymbols:   make(map[string]map[string]bool),
		RuntimeCapabilities: map[RuntimeCapability]bool{CapCore: true},
	}
}

// IsFileReachable returns true if the source file was determined to be live.
func (g *ReachabilityGraph) IsFileReachable(filePath string) bool {
	if g == nil {
		return true
	}
	clean := canonicalSourcePath(filePath)
	return g.LiveFiles[clean]
}

// IsClassReachable returns true if the class declaration is live.
func (g *ReachabilityGraph) IsClassReachable(className string) bool {
	if g == nil {
		return true
	}
	return g.LiveClasses[className]
}

// IsMethodReachable returns true if a specific class method is live.
func (g *ReachabilityGraph) IsMethodReachable(className, methodName string) bool {
	if g == nil {
		return true
	}
	if methods, exists := g.LiveMethods[className]; exists {
		return methods[methodName]
	}
	return false
}

// IsFunctionReachable returns true if a named function is live.
func (g *ReachabilityGraph) IsFunctionReachable(funcName string) bool {
	if g == nil {
		return true
	}
	return g.LiveFunctions[funcName]
}

// RecordPluginSymbolUse notes that a plugin symbol is referenced.
func (g *ReachabilityGraph) RecordPluginSymbolUse(pluginName, symbolName string) {
	if g == nil {
		return
	}
	if g.LivePluginSymbols == nil {
		g.LivePluginSymbols = make(map[string]map[string]bool)
	}
	if g.LivePluginSymbols[pluginName] == nil {
		g.LivePluginSymbols[pluginName] = make(map[string]bool)
	}
	g.LivePluginSymbols[pluginName][symbolName] = true
	g.LiveSymbols[pluginName] = true
	g.LiveSymbols[symbolName] = true
}

// IsPluginSymbolReachable returns true if a specific plugin symbol is needed.
func (g *ReachabilityGraph) IsPluginSymbolReachable(pluginName, symbolName string) bool {
	if g == nil {
		return true
	}
	if symbols, exists := g.LivePluginSymbols[pluginName]; exists {
		return symbols[symbolName]
	}
	return g.LiveSymbols[pluginName]
}

// HasCapability checks whether a specific runtime subsystem is required.
func (g *ReachabilityGraph) HasCapability(cap RuntimeCapability) bool {
	if g == nil {
		return true
	}
	return g.RuntimeCapabilities[cap]
}

// ReachableFiles returns a sorted list of all source files marked live.
func (g *ReachabilityGraph) ReachableFiles() []string {
	if g == nil {
		return nil
	}
	files := make([]string, 0, len(g.LiveFiles))
	for f := range g.LiveFiles {
		files = append(files, f)
	}
	return files
}

// BuildReachabilityGraph performs static reachability analysis on a PreparedProgram.
func BuildReachabilityGraph(prepared *PreparedProgram, opts ReachabilityOptions) *ReachabilityGraph {
	graph := NewReachabilityGraph(opts.Entrypoint)
	if prepared == nil {
		return graph
	}

	// Always mark entrypoint as live
	if graph.Entrypoint != "" {
		graph.LiveFiles[graph.Entrypoint] = true
	}

	// Mark explicitly kept files
	for _, f := range opts.KeepFiles {
		graph.LiveFiles[canonicalSourcePath(f)] = true
	}

	// Index source units by path, class definitions, and top-level functions
	unitByPath := make(map[string]SourceUnit, len(prepared.Units))
	ctx := &reachabilityContext{
		graph:           graph,
		unitByClass:     make(map[string]string),
		classDeclMap:    make(map[string]*parser.ClassStatement),
		unitByFunc:      make(map[string]string),
		funcDeclMap:     make(map[string]*parser.MethodStatement),
		seenMethodCalls: make(map[string]bool),
	}

	for _, unit := range prepared.Units {
		cPath := canonicalSourcePath(unit.Path)
		unitByPath[cPath] = unit
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			if cs, ok := stmt.(*parser.ClassStatement); ok && cs.Name != nil {
				ctx.unitByClass[cs.Name.Value] = cPath
				ctx.classDeclMap[cs.Name.Value] = cs
			}
			if ms, ok := stmt.(*parser.MethodStatement); ok && ms.Name != nil {
				ctx.unitByFunc[ms.Name.Value] = cPath
				ctx.funcDeclMap[ms.Name.Value] = ms
			}
		}
	}

	// Mark explicitly kept classes and symbols
	for _, c := range opts.KeepClasses {
		ctx.markClassLive(c)
	}
	for _, s := range opts.KeepSymbols {
		graph.LiveSymbols[s] = true
		ctx.markFunctionLive(s)
	}

	// Inspect route files if provided or matching routes.joss / api.joss
	for _, unit := range prepared.Units {
		cPath := canonicalSourcePath(unit.Path)
		isRoute := cPath == "routes.joss" || cPath == "api.joss" || strings.HasSuffix(cPath, "/routes.joss") || strings.HasSuffix(cPath, "/api.joss")
		for _, rf := range opts.RouteFiles {
			if canonicalSourcePath(rf) == cPath {
				isRoute = true
				break
			}
		}
		if isRoute {
			graph.LiveFiles[cPath] = true
			graph.RuntimeCapabilities[CapServer] = true
			graph.RuntimeCapabilities[CapHTTP] = true
			ctx.scanRouteUnit(unit)
			if unit.Program != nil {
				ctx.scanStatements(unit.Program.Statements)
			}
		}
	}

	// Traverse from Entrypoint
	entryUnit, exists := unitByPath[graph.Entrypoint]
	if !exists && len(prepared.Units) > 0 {
		entryUnit = prepared.Units[0]
		graph.Entrypoint = canonicalSourcePath(entryUnit.Path)
		graph.LiveFiles[graph.Entrypoint] = true
	}

	if entryUnit.Program != nil {
		for _, stmt := range entryUnit.Program.Statements {
			if cs, ok := stmt.(*parser.ClassStatement); ok && cs.Name != nil {
				ctx.markClassLive(cs.Name.Value)
				ctx.markMethodLive(cs.Name.Value, "Init")
				ctx.markMethodLive(cs.Name.Value, "main")
			}
			if ms, ok := stmt.(*parser.MethodStatement); ok && ms.Name != nil {
				if ms.Name.Value == "main" || ms.Name.Value == "Main" {
					ctx.markFunctionLive(ms.Name.Value)
				}
			}
		}
		ctx.scanStatements(entryUnit.Program.Statements)
	}

	// Transitive expansion for all marked classes and functions
	ctx.expandTransitive()

	return graph
}

type reachabilityContext struct {
	graph           *ReachabilityGraph
	unitByClass     map[string]string
	classDeclMap    map[string]*parser.ClassStatement
	unitByFunc      map[string]string
	funcDeclMap     map[string]*parser.MethodStatement
	seenMethodCalls map[string]bool
	currentClass    string
}

func (ctx *reachabilityContext) markClassLive(className string) {
	if ctx.graph.LiveClasses[className] {
		return
	}
	ctx.graph.LiveClasses[className] = true
	if path, found := ctx.unitByClass[className]; found {
		ctx.graph.LiveFiles[path] = true
	}
	if ctx.graph.LiveMethods[className] == nil {
		ctx.graph.LiveMethods[className] = make(map[string]bool)
	}

	// If class extends Model or other framework class, activate capabilities
	if decl, found := ctx.classDeclMap[className]; found {
		if decl.SuperClass != nil {
			parentName := decl.SuperClass.Value
			if parentName == "Model" {
				ctx.graph.RuntimeCapabilities[CapDatabase] = true
				ctx.graph.RuntimeCapabilities[CapSQLite] = true
			}
			ctx.markClassLive(parentName)
		}
		// In models, keep accessors, mutators and hooks live
		if decl.Body != nil {
			for _, member := range decl.Body.Statements {
				if ms, ok := member.(*parser.MethodStatement); ok && ms.Name != nil {
					mName := ms.Name.Value
					if strings.HasPrefix(mName, "get") || strings.HasPrefix(mName, "set") ||
						mName == "beforeSave" || mName == "afterSave" || mName == "beforeCreate" || mName == "afterCreate" {
						ctx.graph.LiveMethods[className][mName] = true
					}
				}
			}
		}
	}
}

func (ctx *reachabilityContext) markClassInstantiated(className string) {
	ctx.markClassLive(className)
	if ctx.graph.InstantiatedClasses == nil {
		ctx.graph.InstantiatedClasses = make(map[string]bool)
	}
	ctx.graph.InstantiatedClasses[className] = true
	ctx.markMethodLive(className, "Init")
	ctx.markMethodLive(className, "constructor")
	if decl, found := ctx.classDeclMap[className]; found && decl.SuperClass != nil {
		ctx.markClassInstantiated(decl.SuperClass.Value)
	}
}

func (ctx *reachabilityContext) markMethodLive(className, methodName string) {
	ctx.markClassLive(className)
	if ctx.graph.LiveMethods[className] == nil {
		ctx.graph.LiveMethods[className] = make(map[string]bool)
	}
	ctx.graph.LiveMethods[className][methodName] = true
}

func (ctx *reachabilityContext) markFunctionLive(fnName string) {
	if ctx.graph.LiveFunctions[fnName] {
		return
	}
	ctx.graph.LiveFunctions[fnName] = true
	if path, found := ctx.unitByFunc[fnName]; found {
		ctx.graph.LiveFiles[path] = true
	}
}

func (ctx *reachabilityContext) scanRouteUnit(unit SourceUnit) {
	if unit.Program == nil {
		return
	}
	for _, stmt := range unit.Program.Statements {
		exprStmt, ok := stmt.(*parser.ExpressionStatement)
		if !ok {
			continue
		}
		call, ok := exprStmt.Expression.(*parser.CallExpression)
		if !ok {
			continue
		}
		// Pattern: Router::get("/path", "Controller@action") or Router::post(...)
		for _, arg := range call.Arguments {
			if strLit, ok := arg.(*parser.StringLiteral); ok {
				if strings.Contains(strLit.Value, "@") {
					parts := strings.SplitN(strLit.Value, "@", 2)
					ctrl := parts[0]
					action := parts[1]
					ctx.markClassInstantiated(ctrl)
					ctx.markMethodLive(ctrl, action)
				}
			}
		}
	}
}

func (ctx *reachabilityContext) scanStatements(stmts []parser.Statement) {
	for _, stmt := range stmts {
		ctx.scanNode(stmt)
	}
}

func (ctx *reachabilityContext) scanNode(node parser.Node) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *parser.ExpressionStatement:
		ctx.scanNode(n.Expression)
	case *parser.EchoStatement:
		ctx.graph.RuntimeCapabilities[CapIO] = true
		if n.Value != nil {
			ctx.scanNode(n.Value)
		}
	case *parser.LetStatement:
		ctx.scanNode(n.Value)
	case *parser.MultiLetStatement:
		for _, decl := range n.Declarations {
			if decl.Value != nil {
				ctx.scanNode(decl.Value)
			}
		}
	case *parser.ReturnStatement:
		ctx.scanNode(n.ReturnValue)
	case *parser.BlockStatement:
		for _, s := range n.Statements {
			ctx.scanNode(s)
		}
	case *parser.BlockExpression:
		if n.Block != nil {
			ctx.scanNode(n.Block)
		}
	case *parser.GuardStatement:
		ctx.scanNode(n.Condition)
		if n.Body != nil {
			ctx.scanNode(n.Body)
		}
	case *parser.ForeachStatement:
		ctx.scanNode(n.Iterable)
		if n.Body != nil {
			ctx.scanNode(n.Body)
		}
	case *parser.WhileStatement:
		ctx.scanNode(n.Condition)
		if n.Body != nil {
			ctx.scanNode(n.Body)
		}
	case *parser.DoWhileStatement:
		ctx.scanNode(n.Condition)
		if n.Body != nil {
			ctx.scanNode(n.Body)
		}
	case *parser.TryCatchStatement:
		if n.TryBlock != nil {
			ctx.scanNode(n.TryBlock)
		}
		if n.CatchBlock != nil {
			ctx.scanNode(n.CatchBlock)
		}
	case *parser.ThrowStatement:
		ctx.scanNode(n.Value)
	case *parser.DeferStatement:
		ctx.scanNode(n.Body)
	case *parser.DestructureStatement:
		ctx.scanNode(n.Value)
	case *parser.InitStatement:
		if n.Body != nil {
			ctx.scanNode(n.Body)
		}
	case *parser.AssignExpression:
		ctx.scanNode(n.Left)
		ctx.scanNode(n.Value)
	case *parser.IndexExpression:
		ctx.scanNode(n.Left)
		ctx.scanNode(n.Index)
	case *parser.MatchExpression:
		ctx.scanNode(n.Subject)
		for _, arm := range n.Arms {
			for _, k := range arm.Keys {
				ctx.scanNode(k)
			}
			ctx.scanNode(arm.Value)
		}
	case *parser.TernaryExpression:
		ctx.scanNode(n.Condition)
		ctx.scanNode(n.True)
		ctx.scanNode(n.False)
	case *parser.CallExpression:
		ctx.inspectCall(n)
		ctx.scanNode(n.Function)
		for _, arg := range n.Arguments {
			ctx.scanNode(arg)
		}
	case *parser.NewExpression:
		if n.Class != nil {
			ctx.graph.detectNativeCapability(n.Class.Value, "Init")
			ctx.markClassInstantiated(n.Class.Value)
		}
		for _, arg := range n.Arguments {
			ctx.scanNode(arg)
		}
	case *parser.MemberExpression:
		ctx.scanNode(n.Left)
	case *parser.InfixExpression:
		ctx.scanNode(n.Left)
		ctx.scanNode(n.Right)
	case *parser.PrefixExpression:
		ctx.scanNode(n.Right)
	case *parser.PostfixExpression:
		ctx.scanNode(n.Left)
	case *parser.ArrayLiteral:
		for _, el := range n.Elements {
			ctx.scanNode(el)
		}
	case *parser.MapLiteral:
		for k, v := range n.Pairs {
			ctx.scanNode(k)
			ctx.scanNode(v)
		}
	case *parser.ClassStatement:
		// Scanned when marked live
	case *parser.MethodStatement:
		if n.Body != nil {
			ctx.scanNode(n.Body)
		}
	case *parser.FunctionLiteral:
		if n.Body != nil {
			ctx.scanNode(n.Body)
		}
	}
}

func (ctx *reachabilityContext) inspectCall(call *parser.CallExpression) {
	if call == nil {
		return
	}

	// 1. Static call: Class::method(...)
	if member, ok := call.Function.(*parser.MemberExpression); ok {
		if member.Token.Literal == "::" {
			if ident, ok := member.Left.(*parser.Identifier); ok {
				className := ident.Value
				methodName := ""
				if member.Property != nil {
					methodName = member.Property.Value
				}
				if className == "Plugin" && (methodName == "call" || methodName == "stream") {
					if len(call.Arguments) >= 2 {
						pName := ""
						sName := ""
						if lit, ok := call.Arguments[0].(*parser.StringLiteral); ok {
							pName = lit.Value
						}
						if lit, ok := call.Arguments[1].(*parser.StringLiteral); ok {
							sName = lit.Value
						}
						if pName != "" && sName != "" {
							ctx.graph.RecordPluginSymbolUse(pName, sName)
						}
					}
					for _, arg := range call.Arguments {
						if lit, ok := arg.(*parser.StringLiteral); ok {
							ctx.graph.LiveSymbols[lit.Value] = true
						}
					}
				}
				ctx.graph.detectNativeCapability(className, methodName)
				ctx.markMethodLive(className, methodName)
				return
			}
		} else if member.Token.Literal == "->" {
			methodName := ""
			if member.Property != nil {
				methodName = member.Property.Value
			}
			if methodName != "" {
				// Record this instance method call name for transitive resolution
				ctx.seenMethodCalls[methodName] = true

				// A. Direct new expression: (new ClassName())->method(...)
				if newExpr, ok := member.Left.(*parser.NewExpression); ok && newExpr.Class != nil {
					targetClass := newExpr.Class.Value
					ctx.markClassInstantiated(targetClass)
					ctx.markMethodLive(targetClass, methodName)
					return
				}

				// B. $this->method(...)
				if ident, ok := member.Left.(*parser.Identifier); ok && (ident.Value == "$this" || ident.Value == "this") {
					if ctx.currentClass != "" {
						ctx.markMethodLive(ctx.currentClass, methodName)
						return
					}
				}

				// C. Instance call: $var->method(...)
				// Match against any live class that declares this method
				for cName := range ctx.graph.LiveClasses {
					if decl, ok := ctx.classDeclMap[cName]; ok && decl.Body != nil {
						for _, m := range decl.Body.Statements {
							if ms, ok := m.(*parser.MethodStatement); ok && ms.Name != nil && ms.Name.Value == methodName {
								ctx.markMethodLive(cName, methodName)
							}
						}
					}
				}
			}
		}
	}

	// 2. Global function: print(...), helper(...), etc.
	if ident, ok := call.Function.(*parser.Identifier); ok {
		fnName := ident.Value
		ctx.markFunctionLive(fnName)
		if fnName == "print" || fnName == "println" || fnName == "printf" {
			ctx.graph.RuntimeCapabilities[CapIO] = true
		}
	}
}

func (g *ReachabilityGraph) detectNativeCapability(className, methodName string) {
	switch className {
	case "GranDB", "Database", "DB", "Schema", "SQLite":
		g.RuntimeCapabilities[CapDatabase] = true
		g.RuntimeCapabilities[CapSQLite] = true
	case "MySQL":
		g.RuntimeCapabilities[CapDatabase] = true
		g.RuntimeCapabilities[CapMySQL] = true
	case "Server", "Router", "Response", "Request":
		g.RuntimeCapabilities[CapServer] = true
		g.RuntimeCapabilities[CapHTTP] = true
	case "WebSocket":
		g.RuntimeCapabilities[CapServer] = true
		g.RuntimeCapabilities[CapWebSocket] = true
	case "Window", "GUI", "WebView":
		g.RuntimeCapabilities[CapGUI] = true
	case "JSON":
		g.RuntimeCapabilities[CapJSON] = true
	case "Markdown":
		g.RuntimeCapabilities[CapMarkdown] = true
	case "Console":
		g.RuntimeCapabilities[CapIO] = true
	}
}

func (ctx *reachabilityContext) expandTransitive() {
	// Repeatedly scan bodies of newly live methods and functions until fixpoint
	scannedMethods := make(map[string]bool)
	scannedFunctions := make(map[string]bool)

	for {
		newWork := false

		// 1. Expand class methods
		for className, methods := range ctx.graph.LiveMethods {
			decl, ok := ctx.classDeclMap[className]
			if !ok || decl.Body == nil {
				continue
			}
			for mName := range methods {
				key := className + "::" + mName
				if scannedMethods[key] {
					continue
				}
				scannedMethods[key] = true
				newWork = true

				ctx.currentClass = className
				for _, member := range decl.Body.Statements {
					if ms, ok := member.(*parser.MethodStatement); ok && ms.Name != nil && ms.Name.Value == mName {
						if ms.Body != nil {
							ctx.scanNode(ms.Body)
						}
					} else if initStmt, ok := member.(*parser.InitStatement); ok {
						initName := "Init"
						if initStmt.Name != nil {
							initName = initStmt.Name.Value
						}
						if mName == "Init" || mName == initName || mName == "main" {
							if initStmt.Body != nil {
								ctx.scanNode(initStmt.Body)
							}
						}
					}
				}
				ctx.currentClass = ""
			}
		}

		// 2. Expand top-level functions (dead code elimination for library functions)
		for fnName := range ctx.graph.LiveFunctions {
			if scannedFunctions[fnName] {
				continue
			}
			scannedFunctions[fnName] = true
			if decl, ok := ctx.funcDeclMap[fnName]; ok && decl.Body != nil {
				newWork = true
				ctx.scanNode(decl.Body)
			}
		}

		// 3. Match any seen method calls against current live classes
		for methodName := range ctx.seenMethodCalls {
			for cName := range ctx.graph.LiveClasses {
				if ctx.graph.LiveMethods[cName] != nil && ctx.graph.LiveMethods[cName][methodName] {
					continue
				}
				if decl, ok := ctx.classDeclMap[cName]; ok && decl.Body != nil {
					for _, m := range decl.Body.Statements {
						if ms, ok := m.(*parser.MethodStatement); ok && ms.Name != nil && ms.Name.Value == methodName {
							ctx.markMethodLive(cName, methodName)
							newWork = true
						}
					}
				}
			}
		}

		if !newWork {
			break
		}
	}
}

// PruneProgramAST removes unreferenced top-level functions and unreferenced class methods from a SourceUnit.
func PruneProgramAST(unit SourceUnit, graph *ReachabilityGraph) SourceUnit {
	if graph == nil || unit.Program == nil {
		return unit
	}

	cPath := canonicalSourcePath(unit.Path)
	isEntrypoint := (cPath == graph.Entrypoint)

	var prunedStmts []parser.Statement

	for _, stmt := range unit.Program.Statements {
		switch s := stmt.(type) {
		case *parser.ClassStatement:
			if !isEntrypoint && s.Name != nil && !graph.IsClassReachable(s.Name.Value) {
				// Entire class is dead, drop it
				continue
			}
			// Class is reachable: prune methods inside if symbol-level pruning applies
			if s.Name != nil && s.Body != nil {
				className := s.Name.Value
				liveMethods := graph.LiveMethods[className]
				if liveMethods != nil {
					var retainedBody []parser.Statement
					for _, member := range s.Body.Statements {
						if ms, ok := member.(*parser.MethodStatement); ok && ms.Name != nil {
							mName := ms.Name.Value
							// Retain Init / constructor and marked live methods
							if mName == "Init" || mName == "constructor" || liveMethods[mName] {
								retainedBody = append(retainedBody, member)
							}
						} else {
							// Properties, constants, etc. are retained
							retainedBody = append(retainedBody, member)
						}
					}
					s.Body.Statements = retainedBody
				}
			}
			prunedStmts = append(prunedStmts, s)

		case *parser.MethodStatement:
			if s.Name != nil {
				if s.Name.Value == "main" || s.Name.Value == "Main" || graph.IsFunctionReachable(s.Name.Value) {
					prunedStmts = append(prunedStmts, s)
				}
				// Unreferenced function dropped
				continue
			}
			prunedStmts = append(prunedStmts, s)

		default:
			prunedStmts = append(prunedStmts, stmt)
		}
	}

	unit.Program.Statements = prunedStmts
	return unit
}
