package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

type functionDeclaration struct {
	callable Callable
	token    parser.Token
	file     string
}

type Analyzer struct {
	environment       Environment
	diagnostics       diagnostics.Bag
	functions         map[string]functionDeclaration
	classes           map[string]Class
	classTokens       map[string]functionDeclaration
	interfaces        map[string]Interface
	interfaceTokens   map[string]functionDeclaration
	file              string
	currentClass      string
	currentReturnType typesystem.Type
	suppressUndefined int
}

func Analyze(units []SourceUnit, environment Environment) []diagnostics.Diagnostic {
	a := &Analyzer{
		environment:       environment,
		functions:         make(map[string]functionDeclaration),
		classes:           make(map[string]Class),
		classTokens:       make(map[string]functionDeclaration),
		interfaces:        make(map[string]Interface),
		interfaceTokens:   make(map[string]functionDeclaration),
		currentReturnType: typesystem.Type{Kind: typesystem.Unknown},
	}
	for name, class := range environment.Classes {
		a.classes[name] = class
	}
	for name, iface := range environment.Interfaces {
		a.interfaces[name] = iface
	}
	a.collectDeclarations(units)
	a.analyzeUnits(units)
	items := a.diagnostics.Items()
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].File != items[j].File {
			return items[i].File < items[j].File
		}
		if items[i].Range.Start.Line != items[j].Range.Start.Line {
			return items[i].Range.Start.Line < items[j].Range.Start.Line
		}
		if items[i].Range.Start.Column != items[j].Range.Start.Column {
			return items[i].Range.Start.Column < items[j].Range.Start.Column
		}
		return items[i].Code < items[j].Code
	})
	return items
}

func (a *Analyzer) collectDeclarations(units []SourceUnit) {
	for _, unit := range units {
		if unit.Program == nil {
			continue
		}
		for _, statement := range unit.Program.Statements {
			switch node := statement.(type) {
			case *parser.MethodStatement:
				a.declareFunction(node, unit.Path)
			case *parser.ClassStatement:
				a.declareClass(node, unit.Path)
			case *parser.InterfaceStatement:
				a.declareInterface(node, unit.Path)
			}
		}
	}
}

func (a *Analyzer) declareFunction(method *parser.MethodStatement, file string) {
	if method == nil || method.Name == nil {
		return
	}
	name := method.Name.Value
	if previous, exists := a.functions[name]; exists {
		a.add("JOSS-DECL-001", diagnostics.SeverityError, file, method.Name.Token,
			fmt.Sprintf("Function `%s` is already declared at %s:%d.", name, previous.file, previous.token.Line),
			"Each project-level function must have a unique name.", "Rename or remove one declaration.")
		return
	}
	callable := callableFromMethod(method)
	callable.File = file
	a.functions[name] = functionDeclaration{callable: callable, token: method.Name.Token, file: file}
}

func (a *Analyzer) declareInterface(ifaceNode *parser.InterfaceStatement, file string) {
	if ifaceNode == nil || ifaceNode.Name == nil {
		return
	}
	name := ifaceNode.Name.Value
	if previous, exists := a.interfaces[name]; exists {
		where := "the runtime environment"
		if declaration, ok := a.interfaceTokens[name]; ok {
			where = fmt.Sprintf("%s:%d", declaration.file, declaration.token.Line)
		} else if previous.Name != "" {
			where = "a declared interface"
		}
		a.add("JOSS-DECL-004", diagnostics.SeverityError, file, ifaceNode.Name.Token,
			fmt.Sprintf("Interface `%s` conflicts with %s.", name, where),
			"Interface names share one project-wide namespace.", "Choose a unique interface name.")
		return
	}
	if classDecl, exists := a.classTokens[name]; exists {
		a.add("JOSS-DECL-004", diagnostics.SeverityError, file, ifaceNode.Name.Token,
			fmt.Sprintf("Interface `%s` conflicts with class at %s:%d.", name, classDecl.file, classDecl.token.Line),
			"Interfaces and classes share one project-wide type namespace.", "Choose a unique name.")
		return
	}
	iface := Interface{
		Name:       name,
		Extends:    make([]string, 0, len(ifaceNode.Extends)),
		Methods:    make(map[string]Callable),
		Visibility: ifaceNode.Visibility,
		File:       file,
	}
	for _, ext := range ifaceNode.Extends {
		if ext != nil {
			iface.Extends = append(iface.Extends, ext.Value)
		}
	}
	for _, method := range ifaceNode.Methods {
		if method == nil || method.Name == nil {
			continue
		}
		methodName := method.Name.Value
		callable := callableFromMethod(method)
		callable.Owner = name
		callable.File = file
		if _, exists := iface.Methods[methodName]; exists {
			a.add("JOSS-DECL-003", diagnostics.SeverityError, file, method.Name.Token,
				fmt.Sprintf("Method `%s::%s` is declared more than once.", name, methodName),
				"An interface cannot contain duplicate method names.", "Rename or remove one method.")
			continue
		}
		iface.Methods[methodName] = callable
	}
	a.interfaces[name] = iface
	a.interfaceTokens[name] = functionDeclaration{token: ifaceNode.Name.Token, file: file}
}

func (a *Analyzer) declareClass(classNode *parser.ClassStatement, file string) {
	if classNode == nil || classNode.Name == nil {
		return
	}
	name := classNode.Name.Value
	if previous, exists := a.classes[name]; exists {
		where := "the runtime environment"
		if declaration, ok := a.classTokens[name]; ok {
			where = fmt.Sprintf("%s:%d", declaration.file, declaration.token.Line)
		} else if previous.Name != "" {
			where = "a native or plugin class"
		}
		a.add("JOSS-DECL-002", diagnostics.SeverityError, file, classNode.Name.Token,
			fmt.Sprintf("Class `%s` conflicts with %s.", name, where),
			"Class names share one project-wide namespace.", "Choose a unique class name.")
		return
	}
	if ifaceDecl, exists := a.interfaceTokens[name]; exists {
		a.add("JOSS-DECL-004", diagnostics.SeverityError, file, classNode.Name.Token,
			fmt.Sprintf("Class `%s` conflicts with interface at %s:%d.", name, ifaceDecl.file, ifaceDecl.token.Line),
			"Interfaces and classes share one project-wide type namespace.", "Choose a unique name.")
		return
	}
	class := Class{Name: name, Methods: make(map[string]Callable), Fields: make(map[string]Field), Visibility: classNode.Visibility, File: file}
	if classNode.SuperClass != nil {
		class.SuperClass = classNode.SuperClass.Value
	}
	for _, iface := range classNode.Interfaces {
		if iface != nil {
			class.Interfaces = append(class.Interfaces, iface.Value)
		}
	}
	if classNode.Body != nil {
		for _, member := range classNode.Body.Statements {
			if declaration, ok := member.(*parser.LetStatement); ok && declaration.Name != nil {
				class.Fields[declaration.Name.Value] = Field{Type: typeFromToken(declaration.Token), Constant: declaration.IsConst, Visibility: declaration.Visibility, Owner: name}
				continue
			}
			if declarations, ok := member.(*parser.MultiLetStatement); ok {
				for _, declaration := range declarations.Declarations {
					if declaration.Name != nil {
						class.Fields[declaration.Name.Value] = Field{Type: typeFromToken(declarations.TypeToken), Visibility: declarations.Visibility, Owner: name}
					}
				}
				continue
			}
			var methodName string
			var callable Callable
			var token parser.Token
			switch method := member.(type) {
			case *parser.MethodStatement:
				if method.Name == nil {
					continue
				}
				methodName, callable, token = method.Name.Value, callableFromMethod(method), method.Name.Token
				callable.Owner = name
				callable.File = file
			case *parser.InitStatement:
				if method.Name == nil {
					continue
				}
				methodName = method.Name.Value
				callable = Callable{Name: methodName, Parameters: parametersFromAST(method.Parameters), ReturnType: typesystem.Type{Kind: typesystem.Unknown}}
				token = method.Name.Token
			default:
				continue
			}
			if _, exists := class.Methods[methodName]; exists {
				a.add("JOSS-DECL-003", diagnostics.SeverityError, file, token,
					fmt.Sprintf("Method `%s::%s` is declared more than once.", name, methodName),
					"A class cannot contain duplicate method names.", "Rename or remove one method.")
				continue
			}
			class.Methods[methodName] = callable
		}
	}
	a.classes[name] = class
	a.classTokens[name] = functionDeclaration{token: classNode.Name.Token, file: file}
}

func callableFromMethod(method *parser.MethodStatement) Callable {
	return Callable{
		Name:       method.Name.Value,
		Parameters: parametersFromAST(method.Parameters),
		ReturnType: typeFromToken(method.ReturnType),
		Visibility: method.Visibility,
	}
}

func typeFromToken(token parser.Token) typesystem.Type {
	if token.Literal == "" {
		return typesystem.Type{Kind: typesystem.Unknown}
	}
	return typesystem.Parse(token.Literal)
}

func parametersFromAST(parameters []*parser.Parameter) []Parameter {
	result := make([]Parameter, 0, len(parameters))
	for _, parameter := range parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		parameterType := typesystem.Type{Kind: typesystem.Mixed}
		if parameter.Type.Literal != "" && parameter.Type.Type != parser.VAR {
			parameterType = typesystem.Parse(parameter.Type.Literal)
		}
		result = append(result, Parameter{Name: parameter.Name.Value, Type: parameterType, HasDefault: parameter.DefaultValue != nil, ByReference: parameter.ByReference})
	}
	return result
}

func (a *Analyzer) analyzeUnits(units []SourceUnit) {
	global := newScope(nil)
	for name, valueType := range a.environment.Globals {
		global.put(&symbol{Name: name, Type: valueType, Kind: symbolImplicit, Used: true, Synthetic: true})
	}
	for _, unit := range units {
		if unit.Program == nil {
			continue
		}
		a.file = unit.Path
		fileScope := newScope(global)
		for _, statement := range unit.Program.Statements {
			switch node := statement.(type) {
			case *parser.ClassStatement:
				a.analyzeClass(node, global)
			case *parser.InterfaceStatement:
				a.analyzeInterface(node)
			case *parser.MethodStatement:
				returnType := typeFromToken(node.ReturnType)
				a.validateDeclaredType(returnType, node.ReturnType, "return annotation")
				a.analyzeCallable(node.Parameters, node.Body, global, "", returnType)
			default:
				a.analyzeStatement(statement, fileScope)
			}
		}
		a.reportUnused(fileScope)
	}
}

type ifaceMethodRequirement struct {
	method    Callable
	ifaceName string
}

func (a *Analyzer) collectInterfaceMethods(interfaceName string, visited map[string]bool) []ifaceMethodRequirement {
	if visited[interfaceName] {
		return nil
	}
	visited[interfaceName] = true
	iface, exists := a.interfaces[interfaceName]
	if !exists {
		return nil
	}
	var reqs []ifaceMethodRequirement
	for _, method := range iface.Methods {
		reqs = append(reqs, ifaceMethodRequirement{method: method, ifaceName: interfaceName})
	}
	for _, ext := range iface.Extends {
		reqs = append(reqs, a.collectInterfaceMethods(ext, visited)...)
	}
	return reqs
}

func (a *Analyzer) analyzeInterface(ifaceNode *parser.InterfaceStatement) {
	if ifaceNode == nil || ifaceNode.Name == nil {
		return
	}
	name := ifaceNode.Name.Value
	visited := map[string]bool{name: true}
	for _, ext := range ifaceNode.Extends {
		if ext == nil {
			continue
		}
		if _, exists := a.interfaces[ext.Value]; !exists {
			a.add("JOSS-SYM-007", diagnostics.SeverityError, a.file, ext.Token,
				fmt.Sprintf("Extended interface `%s` does not exist.", ext.Value),
				"An interface can only extend existing interfaces.",
				"Check the interface name or declare it.")
			continue
		}
		if a.hasInterfaceCycle(ext.Value, visited) {
			a.add("JOSS-SYM-008", diagnostics.SeverityError, a.file, ext.Token,
				fmt.Sprintf("Cyclic inheritance detected in interface `%s` via `%s`.", name, ext.Value),
				"Interface inheritance hierarchy cannot form cycles.",
				"Remove the circular inheritance reference.")
		}
	}
	for _, method := range ifaceNode.Methods {
		if method == nil {
			continue
		}
		returnType := typeFromToken(method.ReturnType)
		a.validateDeclaredType(returnType, method.ReturnType, "return annotation")
		for _, param := range method.Parameters {
			if param == nil || param.Name == nil {
				continue
			}
			if param.Type.Literal == "" || param.Type.Type == parser.VAR {
				a.add("JOSS-TYPE-011", diagnostics.SeverityError, a.file, param.Name.Token,
					fmt.Sprintf("Parameter `$%s` requires an explicit type.", param.Name.Value),
					"Joss does not create implicit `mixed` parameters.",
					"Declare a concrete/class/union type, or write `mixed` explicitly.")
			} else {
				pType := typesystem.Parse(param.Type.Literal)
				a.validateDeclaredType(pType, param.Type, "parameter type")
			}
		}
	}
}

func (a *Analyzer) hasInterfaceCycle(current string, visited map[string]bool) bool {
	if visited[current] {
		return true
	}
	visited[current] = true
	defer func() { delete(visited, current) }()
	iface, exists := a.interfaces[current]
	if !exists {
		return false
	}
	for _, ext := range iface.Extends {
		if a.hasInterfaceCycle(ext, visited) {
			return true
		}
	}
	return false
}

func (a *Analyzer) analyzeClass(classNode *parser.ClassStatement, global *scope) {
	if classNode == nil || classNode.Name == nil {
		return
	}
	previousClass := a.currentClass
	a.currentClass = classNode.Name.Value
	defer func() { a.currentClass = previousClass }()
	if classNode.SuperClass != nil {
		if _, exists := a.classes[classNode.SuperClass.Value]; !exists {
			a.add("JOSS-SYM-005", diagnostics.SeverityError, a.file, classNode.SuperClass.Token,
				fmt.Sprintf("Base class `%s` does not exist.", classNode.SuperClass.Value),
				"Inheritance requires a class known to the project, runtime or loaded plugins.", "Check the class name or plugin configuration.")
		}
	}
	for _, ifaceNode := range classNode.Interfaces {
		if ifaceNode == nil {
			continue
		}
		if _, exists := a.interfaces[ifaceNode.Value]; !exists {
			a.add("JOSS-SYM-007", diagnostics.SeverityError, a.file, ifaceNode.Token,
				fmt.Sprintf("Interface `%s` does not exist.", ifaceNode.Value),
				"Classes can only implement declared interfaces.",
				"Check the interface name or declare the interface.")
		}
	}
	checkedMethods := map[string]bool{}
	for _, ifaceNode := range classNode.Interfaces {
		if ifaceNode == nil {
			continue
		}
		if _, exists := a.interfaces[ifaceNode.Value]; !exists {
			continue
		}
		reqs := a.collectInterfaceMethods(ifaceNode.Value, map[string]bool{})
		for _, req := range reqs {
			if checkedMethods[req.method.Name] {
				continue
			}
			checkedMethods[req.method.Name] = true
			classMethod, exists := a.lookupMethod(classNode.Name.Value, req.method.Name)
			if !exists {
				a.add("JOSS-DECL-005", diagnostics.SeverityError, a.file, classNode.Name.Token,
					fmt.Sprintf("Class `%s` does not implement method `%s` required by interface `%s`.",
						classNode.Name.Value, req.method.Name, req.ifaceName),
					"A class implementing an interface must implement all of its methods.",
					fmt.Sprintf("Implement `public func %s(...)` in class `%s`.", req.method.Name, classNode.Name.Value))
				continue
			}
			if classMethod.Visibility != "" && classMethod.Visibility != "public" {
				a.add("JOSS-DECL-005", diagnostics.SeverityError, a.file, classNode.Name.Token,
					fmt.Sprintf("Class `%s` implements interface method `%s` with visibility `%s`; must be `public`.",
						classNode.Name.Value, req.method.Name, classMethod.Visibility),
					"Interface methods define public contracts and must be implemented as public.",
					"Change method visibility to `public`.")
			}
			if len(classMethod.Parameters) != len(req.method.Parameters) {
				a.add("JOSS-DECL-005", diagnostics.SeverityError, a.file, classNode.Name.Token,
					fmt.Sprintf("Class `%s` method `%s` has %d parameter(s), but interface `%s` declares %d.",
						classNode.Name.Value, req.method.Name, len(classMethod.Parameters), req.ifaceName, len(req.method.Parameters)),
					"Method implementation must match the interface signature.",
					"Adjust the method parameters to match the interface contract.")
			} else {
				for i, ifaceParam := range req.method.Parameters {
					clsParam := classMethod.Parameters[i]
					if clsParam.Type != ifaceParam.Type {
						a.add("JOSS-DECL-005", diagnostics.SeverityError, a.file, classNode.Name.Token,
							fmt.Sprintf("Class `%s` method `%s` parameter `$%s` has type `%s`, but interface `%s` declares type `%s`.",
								classNode.Name.Value, req.method.Name, clsParam.Name, clsParam.Type.String(), req.ifaceName, ifaceParam.Type.String()),
							"Method parameter types must match the interface definition.",
							"Change parameter type to match the interface.")
						break
					}
				}
			}
			if req.method.ReturnType.Kind != typesystem.Unknown {
				if classMethod.ReturnType != req.method.ReturnType && !typesystem.Assignable(req.method.ReturnType, classMethod.ReturnType) {
					a.add("JOSS-DECL-005", diagnostics.SeverityError, a.file, classNode.Name.Token,
						fmt.Sprintf("Class `%s` method `%s` returns `%s`, but interface `%s` requires `%s`.",
							classNode.Name.Value, req.method.Name, classMethod.ReturnType.String(), req.ifaceName, req.method.ReturnType.String()),
						"Method return type must be compatible with the interface contract.",
						"Adjust the return type to match the interface.")
				}
			}
		}
	}
	classScope := newScope(global)
	classScope.put(&symbol{Name: "this", Type: typesystem.Type{Kind: typesystem.Class, Name: classNode.Name.Value}, Kind: symbolImplicit, Used: true, Synthetic: true})
	if classNode.Body == nil {
		return
	}
	for _, member := range classNode.Body.Statements {
		switch node := member.(type) {
		case *parser.MethodStatement:
			returnType := typeFromToken(node.ReturnType)
			a.validateDeclaredType(returnType, node.ReturnType, "return annotation")
			a.analyzeCallable(node.Parameters, node.Body, classScope, classNode.Name.Value, returnType)
		case *parser.InitStatement:
			a.analyzeCallable(node.Parameters, node.Body, classScope, classNode.Name.Value, typesystem.Type{Kind: typesystem.Unknown})
		case *parser.LetStatement:
			a.analyzeDeclaration(node, classScope, false)
		case *parser.MultiLetStatement:
			a.analyzeMultiDeclaration(node, classScope, false)
		}
	}
}

func (a *Analyzer) analyzeCallable(parameters []*parser.Parameter, body *parser.BlockStatement, parent *scope, className string, returnType typesystem.Type) {
	previousReturnType := a.currentReturnType
	a.currentReturnType = returnType
	defer func() { a.currentReturnType = previousReturnType }()
	local := newScope(parent)
	if className != "" {
		local.put(&symbol{Name: "this", Type: typesystem.Type{Kind: typesystem.Class, Name: className}, Kind: symbolImplicit, Used: true, Synthetic: true})
	}
	for _, parameter := range parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		if parameter.ByReference && parameter.DefaultValue != nil {
			a.add("JOSS-REF-006", diagnostics.SeverityError, a.file, parameter.Name.Token,
				fmt.Sprintf("Reference parameter `$%s` cannot have a default value.", parameter.Name.Value),
				"A reference must alias a concrete mutable caller binding.", "Remove the default and require an explicit `ref` argument.")
		}
		parameterType := typesystem.Type{Kind: typesystem.Mixed}
		if parameter.Type.Literal == "" || parameter.Type.Type == parser.VAR {
			a.add("JOSS-TYPE-011", diagnostics.SeverityError, a.file, parameter.Name.Token,
				fmt.Sprintf("Parameter `$%s` requires an explicit type.", parameter.Name.Value),
				"Joss does not create implicit `mixed` parameters.", "Declare a concrete/class/union type, or write `mixed` explicitly when dynamism is intentional.")
		}
		if parameter.Type.Literal != "" && parameter.Type.Type != parser.VAR {
			parameterType = typesystem.Parse(parameter.Type.Literal)
			a.validateDeclaredType(parameterType, parameter.Type, "parameter type")
		}
		if _, exists := local.local(parameter.Name.Value); exists {
			a.redeclaration(parameter.Name.Value, parameter.Name.Token)
			continue
		}
		local.put(&symbol{Name: parameter.Name.Value, Type: parameterType, Kind: symbolParameter, Token: parameter.Name.Token, File: a.file, Dynamic: parameterType.IsDynamic()})
		if parameter.DefaultValue != nil {
			valueType := a.inferExpression(parameter.DefaultValue, local)
			if !a.assignableExpression(parameterType, valueType, parameter.DefaultValue) {
				a.typeMismatch("JOSS-TYPE-002", parameter.Name.Value, parameterType, valueType, parameter.Name.Token, "default value")
			}
		}
	}
	if body != nil {
		a.analyzeBlock(body, local)
		if returnType.Kind != typesystem.Unknown && !blockTerminatesCallable(body) {
			a.add("JOSS-TYPE-010", diagnostics.SeverityError, a.file, body.Token,
				fmt.Sprintf("Not every control-flow path returns the declared type `%s`.", returnType.String()),
				"A callable with an explicit return type must return or throw on every reachable path.",
				"Add an explicit return to the missing path or make the return type nullable when `null` is intentional.")
		}
	}
	a.reportUnused(local)
}

func (a *Analyzer) analyzeBlock(block *parser.BlockStatement, current *scope) bool {
	terminated := false
	for _, statement := range block.Statements {
		if terminated {
			a.add("JOSS-FLOW-001", diagnostics.SeverityWarning, a.file, tokenOfStatement(statement),
				"Unreachable statement after an unconditional control-flow exit.",
				"Execution already returned or threw before this statement.", "Remove the statement or restructure the preceding control flow.")
			continue
		}
		terminated = a.analyzeStatement(statement, current)
	}
	return terminated
}

// analyzeStatement returns true when the statement unconditionally terminates
// the current block.
func (a *Analyzer) analyzeStatement(statement parser.Statement, current *scope) bool {
	if statement == nil {
		return false
	}
	switch node := statement.(type) {
	case *parser.LetStatement:
		a.analyzeDeclaration(node, current, true)
	case *parser.MultiLetStatement:
		a.analyzeMultiDeclaration(node, current, true)
	case *parser.ExpressionStatement:
		a.inferExpression(node.Expression, current)
		return expressionTerminatesCallable(node.Expression)
	case *parser.EchoStatement:
		a.inferExpression(node.Value, current)
	case *parser.ReturnStatement:
		actualType := typesystem.Type{Kind: typesystem.Null}
		if node.ReturnValue != nil {
			actualType = a.inferExpression(node.ReturnValue, current)
		}
		if a.currentReturnType.Kind != typesystem.Unknown && !a.assignableExpression(a.currentReturnType, actualType, node.ReturnValue) {
			a.add("JOSS-TYPE-008", diagnostics.SeverityError, a.file, node.Token,
				fmt.Sprintf("Return value has type `%s`; callable requires `%s`.", actualType.String(), a.currentReturnType.String()),
				"Declared return types apply to every explicit return in the callable.", "Return a compatible value or correct the return annotation.")
		}
		return true
	case *parser.ThrowStatement:
		a.inferExpression(node.Value, current)
		return true
	case *parser.WhileStatement:
		a.inferExpression(node.Condition, current)
		if node.Body != nil {
			a.analyzeBlock(node.Body, current)
		}
	case *parser.DoWhileStatement:
		if node.Body != nil {
			a.analyzeBlock(node.Body, current)
		}
		a.inferExpression(node.Condition, current)
	case *parser.ForeachStatement:
		a.inferExpression(node.Iterable, current)
		if node.Key != "" {
			keyName := cleanName(node.Key)
			if existing, exists := current.local(keyName); exists {
				existing.Type = typesystem.Type{Kind: typesystem.Unknown}
				existing.Kind = symbolIteration
			} else {
				current.put(&symbol{Name: keyName, Type: typesystem.Type{Kind: typesystem.Unknown}, Kind: symbolIteration, Token: node.Token, File: a.file, Inferred: true})
			}
		}
		name := cleanName(node.Value)
		if existing, exists := current.local(name); exists {
			// Reusing the iteration binding in a later foreach is assignment-like
			// in the runtime and does not redeclare a typed local.
			existing.Type = typesystem.Type{Kind: typesystem.Unknown}
			existing.Kind = symbolIteration
		} else {
			current.put(&symbol{Name: name, Type: typesystem.Type{Kind: typesystem.Unknown}, Kind: symbolIteration, Token: node.Token, File: a.file, Inferred: true})
		}
		if node.Body != nil {
			a.analyzeBlock(node.Body, current)
		}
	case *parser.DeferStatement:
		if node.Body != nil {
			a.analyzeStatement(node.Body, current)
		}
	case *parser.TryCatchStatement:
		tryTerminates := false
		if node.TryBlock != nil {
			tryTerminates = a.analyzeBlock(node.TryBlock, current)
		}
		if node.CatchVar != "" {
			name := cleanName(node.CatchVar)
			if _, exists := current.local(name); !exists {
				current.put(&symbol{Name: name, Type: typesystem.Type{Kind: typesystem.Object}, Kind: symbolCatch, Token: node.CatchToken, File: a.file})
			}
		}
		catchTerminates := false
		if node.CatchBlock != nil {
			catchTerminates = a.analyzeBlock(node.CatchBlock, current)
		}
		return tryTerminates && catchTerminates
	case *parser.BreakStatement, *parser.ContinueStatement:
		return true
	}
	return false
}

func (a *Analyzer) analyzeDeclaration(node *parser.LetStatement, current *scope, warnUnused bool) {
	if node == nil || node.Name == nil {
		return
	}
	name := cleanName(node.Name.Value)
	if _, exists := current.local(name); exists {
		a.redeclaration(name, node.Name.Token)
		return
	}
	declaredType := typesystem.Parse(node.Token.Literal)
	a.validateDeclaredType(declaredType, node.Token, "variable type")
	inferred := strings.EqualFold(node.Token.Literal, "var")
	dynamic := declaredType.Kind == typesystem.Mixed
	valueType := typesystem.Type{Kind: typesystem.Unknown}
	if node.Value != nil {
		valueType = a.inferExpression(node.Value, current)
	}
	if inferred {
		declaredType = typesystem.MergeInference(declaredType, valueType)
	} else if node.Value != nil && !a.assignableExpression(declaredType, valueType, node.Value) {
		a.typeMismatch("JOSS-TYPE-002", name, declaredType, valueType, node.Name.Token, "initializer")
	}
	kind := symbolVariable
	if !warnUnused {
		kind = symbolImplicit
	}
	current.put(&symbol{Name: name, Type: declaredType, Kind: kind, Token: node.Name.Token, File: a.file, Dynamic: dynamic, Inferred: inferred, Constant: node.IsConst, Used: !warnUnused})
}

func (a *Analyzer) validateDeclaredType(declaredType typesystem.Type, token parser.Token, context string) {
	if token.Literal == "" {
		return
	}
	for _, member := range declaredType.Members() {
		if member.Kind != typesystem.Class {
			continue
		}
		if _, exists := a.classes[member.Name]; exists {
			continue
		}
		if _, exists := a.interfaces[member.Name]; exists {
			continue
		}
		a.add("JOSS-TYPE-009", diagnostics.SeverityError, a.file, token,
			fmt.Sprintf("Unknown type `%s` in %s.", member.Name, context),
			"Only canonical primitive types or declared/native class names are valid.",
			"Use a canonical type name or declare the referenced class.")
	}
}

func (a *Analyzer) analyzeMultiDeclaration(node *parser.MultiLetStatement, current *scope, warnUnused bool) {
	if node == nil {
		return
	}
	for _, declaration := range node.Declarations {
		single := &parser.LetStatement{Token: node.TypeToken, Name: declaration.Name, Value: declaration.Value}
		a.analyzeDeclaration(single, current, warnUnused)
	}
}

func (a *Analyzer) reportUnused(current *scope) {
	names := make([]string, 0, len(current.symbols))
	for name := range current.symbols {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := current.symbols[name]
		if value.Used || value.Synthetic || value.Kind == symbolParameter || value.Kind == symbolCatch || strings.HasPrefix(name, "_") {
			continue
		}
		a.add("JOSS-LINT-001", diagnostics.SeverityWarning, value.File, value.Token,
			fmt.Sprintf("Variable `$%s` is declared but never used.", name),
			"Unused variables often indicate obsolete or incomplete code.", "Remove it or use it; prefix intentionally ignored names with `_`.")
	}
}

func (a *Analyzer) redeclaration(name string, token parser.Token) {
	a.add("JOSS-SYM-002", diagnostics.SeverityError, a.file, token,
		fmt.Sprintf("Symbol `$%s` is already declared in this scope.", cleanName(name)),
		"Redeclarations make the inferred or explicit type ambiguous.", "Assign to the existing variable or choose a different name.")
}

func (a *Analyzer) typeMismatch(code, name string, destination, source typesystem.Type, token parser.Token, context string) {
	a.add(code, diagnostics.SeverityError, a.file, token,
		fmt.Sprintf("Cannot use `%s` as %s for `$%s` of type `%s`.", source.String(), context, cleanName(name), destination.String()),
		"Joss variables keep their explicit or first inferred type.", "Convert the value explicitly or use `let $name` only when dynamic typing is intentional.")
}

func (a *Analyzer) add(code string, severity diagnostics.Severity, file string, token parser.Token, message, explanation, suggestion string) {
	a.diagnostics.Add(diagnostics.Diagnostic{
		Code: code, Severity: severity, File: file, Message: message,
		Range:       diagnostics.Range{Start: diagnostics.Position{Line: token.Line, Column: token.Column}, End: diagnostics.Position{Line: token.Line, Column: token.Column}},
		Explanation: explanation, Suggestion: suggestion,
	})
}

func tokenOfStatement(statement parser.Statement) parser.Token {
	switch node := statement.(type) {
	case *parser.LetStatement:
		return node.Token
	case *parser.MultiLetStatement:
		return node.TypeToken
	case *parser.ExpressionStatement:
		return node.Token
	case *parser.EchoStatement:
		return node.Token
	case *parser.ReturnStatement:
		return node.Token
	case *parser.ThrowStatement:
		return node.Token
	case *parser.WhileStatement:
		return node.Token
	case *parser.DoWhileStatement:
		return node.Token
	case *parser.ForeachStatement:
		return node.Token
	case *parser.TryCatchStatement:
		return node.Token
	case *parser.BreakStatement:
		return node.Token
	case *parser.ContinueStatement:
		return node.Token
	case *parser.DeferStatement:
		return node.Token
	case *parser.ClassStatement:
		return node.Token
	case *parser.MethodStatement:
		return node.Token
	case *parser.InitStatement:
		return node.Token
	default:
		return parser.Token{}
	}
}
