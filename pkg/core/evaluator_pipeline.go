package core

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/parser"
)

func (r *Runtime) evaluatePipeline(expression *parser.InfixExpression) interface{} {
	left := r.evaluateExpression(expression.Left)
	switch right := expression.Right.(type) {
	case *parser.CallExpression:
		arguments := make([]interface{}, 0, len(right.Arguments)+1)
		arguments = append(arguments, left)
		for _, argument := range right.Arguments {
			arguments = append(arguments, r.evaluateCallArgument(argument))
		}
		var callable interface{}
		if identifier, ok := right.Function.(*parser.Identifier); ok {
			if result, ok := r.callBuiltin(identifier.Value, arguments); ok {
				return result
			}
			if function, ok := r.Functions[identifier.Value]; ok {
				callable = function
			} else if value, resolved, initialized := r.localValue(identifier); resolved && initialized {
				callable = value
			} else if value, ok := r.Variables[identifier.Value]; ok && r.sourceMapVisible(identifier.Value) {
				callable = value
			}
		} else {
			callable = r.evaluateExpression(right.Function)
		}
		if callable == nil {
			panic(&JossError{Type: "NotCallable", Message: "Función de pipeline no encontrada o nula", File: r.CurrentFile})
		}
		return r.applyFunction(callable, arguments)
	case *parser.Identifier:
		if result, ok := r.callBuiltin(right.Value, []interface{}{left}); ok {
			return result
		}
		var callable interface{}
		if function, ok := r.Functions[right.Value]; ok {
			callable = function
		} else if value, resolved, initialized := r.localValue(right); resolved && initialized {
			callable = value
		} else if value, ok := r.Variables[right.Value]; ok && r.sourceMapVisible(right.Value) {
			callable = value
		}
		if callable == nil {
			panic(&JossError{Type: "UndefinedFunction", Message: fmt.Sprintf("Función '%s' no encontrada", right.Value), File: r.CurrentFile, Line: right.Token.Line})
		}
		return r.applyFunction(callable, []interface{}{left})
	case *parser.FunctionLiteral:
		return r.callCapturedFunction(r.captureFunction(right), []interface{}{left})
	default:
		return r.applyFunction(r.evaluateExpression(expression.Right), []interface{}{left})
	}
}
