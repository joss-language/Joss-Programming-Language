package core

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/parser"
)

// executeCall is the source-level invocation coordinator. It evaluates source
// arguments once, resolves the callable once, and delegates execution to the
// canonical callable dispatcher.
func (r *Runtime) executeCall(call *parser.CallExpression) interface{} {
	arguments := r.evaluateCallArguments(call.Arguments)

	// Built-ins are considered only when no source function or local shadows the
	// canonical built-in name.
	if identifier, ok := call.Function.(*parser.Identifier); ok {
		_, hasUserFunction := r.Functions[identifier.Value]
		_, hasLocal, _ := r.localValue(identifier)
		if !hasUserFunction && !hasLocal {
			if hasVariableReference(arguments) && IsBuiltin(identifier.Value) {
				panic(&JossError{Type: "ReferenceEscape", Message: fmt.Sprintf("La función nativa '%s' no declara parámetros ref", identifier.Value), File: r.CurrentFile, Line: identifier.Token.Line})
			}
			if result, ok := r.callBuiltin(identifier.Value, arguments); ok {
				return result
			}
		}
	}

	// Plain identifiers use the source function table before lexical and global
	// values. Other expressions (members and closures) use ordinary evaluation.
	var callable interface{}
	if identifier, ok := call.Function.(*parser.Identifier); ok {
		if function, ok := r.Functions[identifier.Value]; ok {
			callable = function
		} else if value, resolved, initialized := r.localValue(identifier); resolved && initialized {
			callable = value
		} else if value, ok := r.Variables[identifier.Value]; ok && r.sourceMapVisible(identifier.Value) {
			callable = value
		}
	} else {
		callable = r.evaluateExpression(call.Function)
	}

	if callable == nil {
		if member, ok := call.Function.(*parser.MemberExpression); ok && (member.NullSafe || member.Token.Type == parser.NULL_SAFE_ARROW) {
			return nil
		}
		if identifier, ok := call.Function.(*parser.Identifier); ok {
			panic(&JossError{
				Type:    "UndefinedFunction",
				Message: fmt.Sprintf("Función '%s' no encontrada", identifier.Value),
				File:    r.CurrentFile,
				Line:    identifier.Token.Line,
			})
		}
		panic(&JossError{Type: "NotCallable", Message: "Intento de invocar un objeto nulo o no invocable", File: r.CurrentFile})
	}

	return r.applyFunction(callable, arguments)
}

// callBuiltin dispatches canonical built-in definitions to their domain handler.
func (r *Runtime) callBuiltin(name string, arguments []interface{}) (interface{}, bool) {
	definition, exists := builtinDefinitionFor(name)
	if !exists {
		return nil, false
	}

	var result interface{}
	var handled bool
	switch definition.domain {
	case builtinCollections:
		result, handled = r.callBuiltinArray(name, arguments)
	case builtinAsync:
		result, handled = r.callBuiltinAsync(name, arguments)
	case builtinDate:
		result, handled = r.callBuiltinDate(name, arguments)
	case builtinIO:
		result, handled = r.callBuiltinIO(name, arguments)
	case builtinSerialization:
		result, handled = r.callBuiltinSerialization(name, arguments)
	case builtinString:
		result, handled = r.callBuiltinString(name, arguments)
	}
	if handled {
		return result, true
	}

	panic(fmt.Sprintf("internal error: builtin %q is catalogued without a runtime handler", name))
}
