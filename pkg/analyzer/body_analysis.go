package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

func (a *Analyzer) projectScope() *scope {
	global := newScope(nil)
	for name, valueType := range a.environment.Globals {
		global.put(&symbol{Name: name, Type: valueType, Kind: symbolImplicit, Used: true, Synthetic: true})
	}
	return global
}

func (a *Analyzer) validateNominalContracts(units []SourceUnit, global *scope) {
	for _, unit := range units {
		if unit.Program == nil {
			continue
		}
		a.withSourceFile(unit.Path, func() {
			for _, statement := range unit.Program.Statements {
				switch node := statement.(type) {
				case *parser.ClassStatement:
					a.validateClassContract(node, global)
				case *parser.InterfaceStatement:
					a.analyzeInterface(node)
				}
			}
		})
	}
}

func (a *Analyzer) analyzeSourceBodies(units []SourceUnit, global *scope) {
	for _, unit := range units {
		if unit.Program == nil {
			continue
		}
		a.withSourceFile(unit.Path, func() {
			fileScope := newScope(global)
			for _, statement := range unit.Program.Statements {
				switch node := statement.(type) {
				case *parser.ClassStatement:
					a.analyzeClassBody(node, global)
				case *parser.InterfaceStatement:
					continue
				case *parser.MethodStatement:
					for _, tp := range node.TypeParameters {
						if tp != nil {
							a.currentTypeParams[tp.Value] = true
							defer delete(a.currentTypeParams, tp.Value)
						}
					}
					returnType := typeFromToken(node.ReturnType)
					a.validateDeclaredType(returnType, node.ReturnType, "return annotation")
					a.analyzeCallable(node.Parameters, node.Body, global, "", returnType, false)
				default:
					a.analyzeStatement(statement, fileScope)
				}
			}
			a.reportUnused(fileScope)
		})
	}
}

func (a *Analyzer) analyzeCallable(parameters []*parser.Parameter, body *parser.BlockStatement, parent *scope, className string, returnType typesystem.Type, inConstructor bool) {
	previousReturnType := a.currentReturnType
	a.currentReturnType = returnType
	defer func() { a.currentReturnType = previousReturnType }()
	previousInConstructor := a.inConstructor
	a.inConstructor = inConstructor
	defer func() { a.inConstructor = previousInConstructor }()
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
		local.put(&symbol{Name: parameter.Name.Value, Type: parameterType, Kind: symbolParameter, Token: parameter.Name.Token, File: a.file, Dynamic: parameterType.IsDynamic(), Initialized: true})
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
	case *parser.DestructureStatement:
		a.analyzeDestructure(node, current)
	case *parser.ExpressionStatement:
		a.inferExpression(node.Expression, current)
		if te, ok := node.Expression.(*parser.TernaryExpression); ok {
			if te.True != nil && expressionTerminatesCallable(te.True) {
				_, falseScope := a.narrowScopeFromCondition(te.Condition, current)
				a.applyNarrowedScope(current, falseScope)
			} else if te.False != nil && expressionTerminatesCallable(te.False) {
				trueScope, _ := a.narrowScopeFromCondition(te.Condition, current)
				a.applyNarrowedScope(current, trueScope)
			}
		}
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
	case *parser.GuardStatement:
		a.inferExpression(node.Condition, current)
		if node.Body != nil {
			_, elseScope := a.narrowScopeFromCondition(node.Condition, current)
			a.analyzeBlock(node.Body, elseScope)
			if !blockTerminatesCallable(node.Body) {
				a.add("JOSS-FLOW-005", diagnostics.SeverityError, a.file, node.Token,
					"El cuerpo `else` de una sentencia `guard` debe terminar el flujo del callable con `return` o `throw`.",
					"La sentencia `guard` garantiza la salida anticipada si la condición falla.",
					"Agrega `return` o `throw` dentro del bloque `else`.")
			}
			trueScope, _ := a.narrowScopeFromCondition(node.Condition, current)
			a.applyNarrowedScope(current, trueScope)
		}
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
				existing.Initialized = true
				current.markInitialized(keyName)
			} else {
				current.put(&symbol{Name: keyName, Type: typesystem.Type{Kind: typesystem.Unknown}, Kind: symbolIteration, Token: node.Token, File: a.file, Inferred: true, Initialized: true})
			}
		}
		name := cleanName(node.Value)
		if existing, exists := current.local(name); exists {
			// Reusing the iteration binding in a later foreach is assignment-like
			// in the runtime and does not redeclare a typed local.
			existing.Type = typesystem.Type{Kind: typesystem.Unknown}
			existing.Kind = symbolIteration
			existing.Initialized = true
			current.markInitialized(name)
		} else {
			current.put(&symbol{Name: name, Type: typesystem.Type{Kind: typesystem.Unknown}, Kind: symbolIteration, Token: node.Token, File: a.file, Inferred: true, Initialized: true})
		}
		if node.Body != nil {
			a.analyzeBlock(node.Body, current)
		}
	case *parser.DeferStatement:
		if node.Body != nil {
			a.analyzeStatement(node.Body, current)
		}
	case *parser.TryCatchStatement:
		tryScope := newScope(current)
		tryTerminates := false
		if node.TryBlock != nil {
			tryTerminates = a.analyzeBlock(node.TryBlock, tryScope)
		}
		catchScope := newScope(current)
		if node.CatchVar != "" {
			name := cleanName(node.CatchVar)
			if _, exists := catchScope.local(name); !exists {
				catchScope.put(&symbol{Name: name, Type: typesystem.Type{Kind: typesystem.Object}, Kind: symbolCatch, Token: node.CatchToken, File: a.file, Initialized: true})
			}
		}
		catchTerminates := false
		if node.CatchBlock != nil {
			catchTerminates = a.analyzeBlock(node.CatchBlock, catchScope)
		}
		for name, sym := range current.symbols {
			if !current.isInitialized(name) && sym.Kind == symbolVariable {
				tryInit := tryTerminates || tryScope.isInitialized(name)
				catchInit := catchTerminates || catchScope.isInitialized(name)
				if tryInit && catchInit {
					current.markInitialized(name)
					sym.Initialized = true
				}
			}
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
	initialized := node.Value != nil || !warnUnused || kind != symbolVariable
	current.put(&symbol{Name: name, Type: declaredType, Kind: kind, Token: node.Name.Token, File: a.file, Dynamic: dynamic, Inferred: inferred, Constant: node.IsConst, Used: !warnUnused, Initialized: initialized})
}

func (a *Analyzer) validateDeclaredType(declaredType typesystem.Type, token parser.Token, context string) {
	if token.Literal == "" {
		return
	}
	for _, member := range declaredType.Members() {
		if member.Kind != typesystem.Class {
			continue
		}
		baseName := member.Name
		if idx := strings.Index(baseName, "<"); idx != -1 {
			baseName = strings.TrimSpace(baseName[:idx])
		}
		if _, exists := a.classes[baseName]; exists {
			continue
		}
		if _, exists := a.interfaces[baseName]; exists {
			continue
		}
		if _, exists := a.enums[baseName]; exists {
			continue
		}
		if a.currentTypeParams != nil && a.currentTypeParams[baseName] {
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

func (a *Analyzer) analyzeDestructure(node *parser.DestructureStatement, current *scope) {
	if node == nil {
		return
	}
	if node.Value != nil {
		a.inferExpression(node.Value, current)
	}
	for _, name := range node.Names {
		if name == nil {
			continue
		}
		varName := cleanName(name.Value)
		if _, exists := current.local(varName); exists {
			a.redeclaration(varName, name.Token)
			continue
		}
		current.put(&symbol{
			Name:        varName,
			Kind:        symbolVariable,
			Type:        typesystem.Type{Kind: typesystem.Mixed},
			Token:       name.Token,
			File:        a.file,
			Dynamic:     true,
			Initialized: true,
			Used:        false,
		})
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
	case *parser.DestructureStatement:
		return node.Token
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
	case *parser.GuardStatement:
		return node.Token
	default:
		return parser.Token{}
	}
}

func (a *Analyzer) applyNarrowedScope(target *scope, source *scope) {
	if target == nil || source == nil {
		return
	}
	for name, sym := range source.symbols {
		if sym.Synthetic {
			if existing, ok := target.resolve(name); ok {
				existing.Type = sym.Type
			}
		}
	}
}
