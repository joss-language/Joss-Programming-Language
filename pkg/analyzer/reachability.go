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
	LiveMethods         map[string]map[string]bool // className -> methodName -> true
	LiveFunctions       map[string]bool
	LiveSymbols         map[string]bool
	RuntimeCapabilities map[RuntimeCapability]bool
}

// NewReachabilityGraph allocates an initialized reachability graph container.
func NewReachabilityGraph(entrypoint string) *ReachabilityGraph {
	return &ReachabilityGraph{
		Entrypoint:          canonicalSourcePath(entrypoint),
		LiveFiles:           make(map[string]bool),
		LiveClasses:         make(map[string]bool),
		LiveMethods:         make(map[string]map[string]bool),
		LiveFunctions:       make(map[string]bool),
		LiveSymbols:         make(map[string]bool),
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

	// Index source units by path and class definitions
	unitByPath := make(map[string]SourceUnit, len(prepared.Units))
	unitByClass := make(map[string]string)
	classDeclMap := make(map[string]*parser.ClassStatement)

	for _, unit := range prepared.Units {
		cPath := canonicalSourcePath(unit.Path)
		unitByPath[cPath] = unit
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			if cs, ok := stmt.(*parser.ClassStatement); ok && cs.Name != nil {
				unitByClass[cs.Name.Value] = cPath
				classDeclMap[cs.Name.Value] = cs
			}
		}
	}

	// Mark explicitly kept classes and symbols
	for _, c := range opts.KeepClasses {
		graph.markClassLive(c, unitByClass, classDeclMap)
	}
	for _, s := range opts.KeepSymbols {
		graph.LiveSymbols[s] = true
		graph.LiveFunctions[s] = true
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
			graph.scanRouteUnit(unit, unitByClass, classDeclMap)
			if unit.Program != nil {
				graph.scanASTStatements(unit.Program.Statements, unitByClass, classDeclMap)
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
				graph.markClassLive(cs.Name.Value, unitByClass, classDeclMap)
				graph.markMethodLive(cs.Name.Value, "Init", unitByClass, classDeclMap)
				graph.markMethodLive(cs.Name.Value, "main", unitByClass, classDeclMap)
			}
		}
		graph.scanASTStatements(entryUnit.Program.Statements, unitByClass, classDeclMap)
	}

	// Transitive expansion for all marked classes
	graph.expandTransitiveDependencies(unitByClass, classDeclMap)

	return graph
}

func (g *ReachabilityGraph) markClassLive(className string, unitByClass map[string]string, classDeclMap map[string]*parser.ClassStatement) {
	if g.LiveClasses[className] {
		return
	}
	g.LiveClasses[className] = true
	if path, found := unitByClass[className]; found {
		g.LiveFiles[path] = true
	}
	if g.LiveMethods[className] == nil {
		g.LiveMethods[className] = make(map[string]bool)
	}

	// If class extends Model or other special framework class, activate capabilities
	if decl, found := classDeclMap[className]; found {
		if decl.SuperClass != nil {
			parentName := decl.SuperClass.Value
			if parentName == "Model" {
				g.RuntimeCapabilities[CapDatabase] = true
				g.RuntimeCapabilities[CapSQLite] = true
			}
			g.markClassLive(parentName, unitByClass, classDeclMap)
		}
		// In models, keep accessors, mutators and hooks live
		if decl.Body != nil {
			for _, member := range decl.Body.Statements {
				if ms, ok := member.(*parser.MethodStatement); ok && ms.Name != nil {
					mName := ms.Name.Value
					if strings.HasPrefix(mName, "get") || strings.HasPrefix(mName, "set") ||
						mName == "beforeSave" || mName == "afterSave" || mName == "beforeCreate" || mName == "afterCreate" {
						g.LiveMethods[className][mName] = true
					}
				}
			}
		}
	}
}

func (g *ReachabilityGraph) markMethodLive(className, methodName string, unitByClass map[string]string, classDeclMap map[string]*parser.ClassStatement) {
	g.markClassLive(className, unitByClass, classDeclMap)
	if g.LiveMethods[className] == nil {
		g.LiveMethods[className] = make(map[string]bool)
	}
	g.LiveMethods[className][methodName] = true
}

func (g *ReachabilityGraph) scanRouteUnit(unit SourceUnit, unitByClass map[string]string, classDeclMap map[string]*parser.ClassStatement) {
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
					g.markMethodLive(ctrl, action, unitByClass, classDeclMap)
				}
			}
		}
	}
}

func (g *ReachabilityGraph) scanASTStatements(stmts []parser.Statement, unitByClass map[string]string, classDeclMap map[string]*parser.ClassStatement) {
	for _, stmt := range stmts {
		g.scanASTNode(stmt, unitByClass, classDeclMap)
	}
}

func (g *ReachabilityGraph) scanASTNode(node parser.Node, unitByClass map[string]string, classDeclMap map[string]*parser.ClassStatement) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *parser.ExpressionStatement:
		g.scanASTNode(n.Expression, unitByClass, classDeclMap)
	case *parser.EchoStatement:
		g.RuntimeCapabilities[CapIO] = true
		if n.Value != nil {
			g.scanASTNode(n.Value, unitByClass, classDeclMap)
		}
	case *parser.LetStatement:
		g.scanASTNode(n.Value, unitByClass, classDeclMap)
	case *parser.MultiLetStatement:
		for _, decl := range n.Declarations {
			if decl.Value != nil {
				g.scanASTNode(decl.Value, unitByClass, classDeclMap)
			}
		}
	case *parser.ReturnStatement:
		g.scanASTNode(n.ReturnValue, unitByClass, classDeclMap)
	case *parser.BlockStatement:
		for _, s := range n.Statements {
			g.scanASTNode(s, unitByClass, classDeclMap)
		}
	case *parser.BlockExpression:
		if n.Block != nil {
			g.scanASTNode(n.Block, unitByClass, classDeclMap)
		}
	case *parser.GuardStatement:
		g.scanASTNode(n.Condition, unitByClass, classDeclMap)
		if n.Body != nil {
			g.scanASTNode(n.Body, unitByClass, classDeclMap)
		}
	case *parser.ForeachStatement:
		g.scanASTNode(n.Iterable, unitByClass, classDeclMap)
		if n.Body != nil {
			g.scanASTNode(n.Body, unitByClass, classDeclMap)
		}
	case *parser.WhileStatement:
		g.scanASTNode(n.Condition, unitByClass, classDeclMap)
		if n.Body != nil {
			g.scanASTNode(n.Body, unitByClass, classDeclMap)
		}
	case *parser.DoWhileStatement:
		g.scanASTNode(n.Condition, unitByClass, classDeclMap)
		if n.Body != nil {
			g.scanASTNode(n.Body, unitByClass, classDeclMap)
		}
	case *parser.TryCatchStatement:
		if n.TryBlock != nil {
			g.scanASTNode(n.TryBlock, unitByClass, classDeclMap)
		}
		if n.CatchBlock != nil {
			g.scanASTNode(n.CatchBlock, unitByClass, classDeclMap)
		}
	case *parser.ThrowStatement:
		g.scanASTNode(n.Value, unitByClass, classDeclMap)
	case *parser.DeferStatement:
		g.scanASTNode(n.Body, unitByClass, classDeclMap)
	case *parser.DestructureStatement:
		g.scanASTNode(n.Value, unitByClass, classDeclMap)
	case *parser.InitStatement:
		if n.Body != nil {
			g.scanASTNode(n.Body, unitByClass, classDeclMap)
		}
	case *parser.AssignExpression:
		g.scanASTNode(n.Left, unitByClass, classDeclMap)
		g.scanASTNode(n.Value, unitByClass, classDeclMap)
	case *parser.IndexExpression:
		g.scanASTNode(n.Left, unitByClass, classDeclMap)
		g.scanASTNode(n.Index, unitByClass, classDeclMap)
	case *parser.MatchExpression:
		g.scanASTNode(n.Subject, unitByClass, classDeclMap)
		for _, arm := range n.Arms {
			for _, k := range arm.Keys {
				g.scanASTNode(k, unitByClass, classDeclMap)
			}
			g.scanASTNode(arm.Value, unitByClass, classDeclMap)
		}
	case *parser.TernaryExpression:
		g.scanASTNode(n.Condition, unitByClass, classDeclMap)
		g.scanASTNode(n.True, unitByClass, classDeclMap)
		g.scanASTNode(n.False, unitByClass, classDeclMap)
	case *parser.CallExpression:
		g.inspectCall(n, unitByClass, classDeclMap)
		for _, arg := range n.Arguments {
			g.scanASTNode(arg, unitByClass, classDeclMap)
		}
	case *parser.NewExpression:
		if n.Class != nil {
			g.detectNativeCapability(n.Class.Value, "Init")
			g.markClassLive(n.Class.Value, unitByClass, classDeclMap)
			g.markMethodLive(n.Class.Value, "Init", unitByClass, classDeclMap)
		}
		for _, arg := range n.Arguments {
			g.scanASTNode(arg, unitByClass, classDeclMap)
		}
	case *parser.MemberExpression:
		g.scanASTNode(n.Left, unitByClass, classDeclMap)
	case *parser.InfixExpression:
		g.scanASTNode(n.Left, unitByClass, classDeclMap)
		g.scanASTNode(n.Right, unitByClass, classDeclMap)
	case *parser.PrefixExpression:
		g.scanASTNode(n.Right, unitByClass, classDeclMap)
	case *parser.PostfixExpression:
		g.scanASTNode(n.Left, unitByClass, classDeclMap)
	case *parser.ArrayLiteral:
		for _, el := range n.Elements {
			g.scanASTNode(el, unitByClass, classDeclMap)
		}
	case *parser.MapLiteral:
		for k, v := range n.Pairs {
			g.scanASTNode(k, unitByClass, classDeclMap)
			g.scanASTNode(v, unitByClass, classDeclMap)
		}
	case *parser.ClassStatement:
		// Scanned when marked live
	case *parser.MethodStatement:
		if n.Body != nil {
			g.scanASTNode(n.Body, unitByClass, classDeclMap)
		}
	case *parser.FunctionLiteral:
		if n.Body != nil {
			g.scanASTNode(n.Body, unitByClass, classDeclMap)
		}
	}
}

func (g *ReachabilityGraph) inspectCall(call *parser.CallExpression, unitByClass map[string]string, classDeclMap map[string]*parser.ClassStatement) {
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
					for _, arg := range call.Arguments {
						if lit, ok := arg.(*parser.StringLiteral); ok {
							g.LiveSymbols[lit.Value] = true
						}
					}
				}
				g.detectNativeCapability(className, methodName)
				g.markMethodLive(className, methodName, unitByClass, classDeclMap)
				return
			}
		}
	}

	// 2. Global function: print(...), die(...), etc.
	if ident, ok := call.Function.(*parser.Identifier); ok {
		fnName := ident.Value
		g.LiveFunctions[fnName] = true
		if fnName == "print" || fnName == "println" || fnName == "printf" {
			g.RuntimeCapabilities[CapIO] = true
		}
	}
}

func (g *ReachabilityGraph) detectNativeCapability(className, methodName string) {
	switch className {
	case "GranDB", "Database", "DB", "Schema", "SQLite":
		g.RuntimeCapabilities[CapDatabase] = true
		g.RuntimeCapabilities[CapSQLite] = true
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

func (g *ReachabilityGraph) expandTransitiveDependencies(unitByClass map[string]string, classDeclMap map[string]*parser.ClassStatement) {
	// Repeatedly scan bodies of newly live methods until fixpoint
	scannedMethods := make(map[string]bool)

	for {
		newWork := false
		for className, methods := range g.LiveMethods {
			decl, ok := classDeclMap[className]
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

				for _, member := range decl.Body.Statements {
					if ms, ok := member.(*parser.MethodStatement); ok && ms.Name != nil && ms.Name.Value == mName {
						if ms.Body != nil {
							g.scanASTNode(ms.Body, unitByClass, classDeclMap)
						}
					} else if initStmt, ok := member.(*parser.InitStatement); ok {
						initName := "Init"
						if initStmt.Name != nil {
							initName = initStmt.Name.Value
						}
						if mName == "Init" || mName == initName || mName == "main" {
							if initStmt.Body != nil {
								g.scanASTNode(initStmt.Body, unitByClass, classDeclMap)
							}
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
	// Entrypoint statements remain preserved
	if cPath == graph.Entrypoint {
		return unit
	}

	var prunedStmts []parser.Statement

	for _, stmt := range unit.Program.Statements {
		switch s := stmt.(type) {
		case *parser.ClassStatement:
			if s.Name != nil && !graph.IsClassReachable(s.Name.Value) {
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
							// Retain Init constructor and marked live methods
							if mName == "Init" || liveMethods[mName] {
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
			if s.Name != nil && !graph.IsFunctionReachable(s.Name.Value) {
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
