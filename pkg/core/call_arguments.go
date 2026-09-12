package core

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
)

// NamedValue is the evaluated representation of a named source argument.
type NamedValue struct {
	Name  string
	Value interface{}
}

// defaultValueMarker postpones default expression evaluation until the callee
// frame is active. Defaults therefore observe the same lexical context as before.
type defaultValueMarker struct{}

// bindArguments is the canonical positional/named/default binding rule for Joss
// methods, functions and closures. Type and reference validation happens later,
// when values are installed in the callee frame.
func (r *Runtime) bindArguments(method *parser.MethodStatement, args []interface{}) []interface{} {
	hasNamed := false
	for _, argument := range args {
		if _, ok := argument.(*NamedValue); ok {
			hasNamed = true
			break
		}
	}
	if !hasNamed {
		return args
	}

	positional := make([]interface{}, 0, len(args))
	named := make(map[string]interface{})
	for _, argument := range args {
		if value, ok := argument.(*NamedValue); ok {
			if _, duplicate := named[value.Name]; duplicate {
				panic(fmt.Sprintf("ArgumentError: Parámetro '$%s' proporcionado más de una vez en %s()", value.Name, method.Name.Value))
			}
			named[value.Name] = value.Value
		} else {
			positional = append(positional, argument)
		}
	}

	resolved := make([]interface{}, len(method.Parameters))
	for index, parameter := range method.Parameters {
		cleanName := strings.TrimPrefix(parameter.Name.Value, "$")
		if index < len(positional) {
			if _, duplicate := named[cleanName]; duplicate {
				panic(fmt.Sprintf("ArgumentError: Parámetro '$%s' proporcionado por posición y nombre en %s()", cleanName, method.Name.Value))
			}
			resolved[index] = positional[index]
		} else if value, ok := named[cleanName]; ok {
			resolved[index] = value
			delete(named, cleanName)
		} else if parameter.DefaultValue != nil {
			resolved[index] = defaultValueMarker{}
		} else {
			panic(fmt.Sprintf("ArgumentError: Falta el argumento requerido '$%s' en %s()", cleanName, method.Name.Value))
		}
	}

	if len(named) > 0 {
		for extra := range named {
			panic(fmt.Sprintf("ArgumentError: Parámetro desconocido '%s' en %s()", extra, method.Name.Value))
		}
	}

	return resolved
}

func (r *Runtime) evaluateCallArguments(rawArguments []parser.Expression) []interface{} {
	arguments := make([]interface{}, 0, len(rawArguments))
	for _, argument := range rawArguments {
		if spread, ok := argument.(*parser.SpreadExpression); ok {
			value := r.evaluateExpression(spread.Expression)
			if slice, ok := value.([]interface{}); ok {
				arguments = append(arguments, slice...)
				continue
			}
			reflected := reflect.ValueOf(value)
			if reflected.IsValid() && reflected.Kind() == reflect.Slice {
				for index := 0; index < reflected.Len(); index++ {
					arguments = append(arguments, reflected.Index(index).Interface())
				}
				continue
			}
			if values, ok := value.(map[string]interface{}); ok {
				for name, item := range values {
					arguments = append(arguments, &NamedValue{Name: name, Value: item})
				}
				continue
			}
			if values, ok := value.(map[interface{}]interface{}); ok {
				for name, item := range values {
					arguments = append(arguments, &NamedValue{Name: fmt.Sprintf("%v", name), Value: item})
				}
				continue
			}
		}
		arguments = append(arguments, r.evaluateCallArgument(argument))
	}
	return arguments
}

func (r *Runtime) evaluateCallArgument(argument parser.Expression) interface{} {
	if named, ok := argument.(*parser.NamedArgument); ok {
		value := r.evaluateCallArgument(named.Value)
		return &NamedValue{Name: named.Name, Value: value}
	}
	reference, ok := argument.(*parser.ReferenceExpression)
	if !ok {
		return r.evaluateExpression(argument)
	}
	identifier, ok := reference.Target.(*parser.Identifier)
	if !ok {
		panic(&JossError{Type: "InvalidReference", Message: "ref requiere una variable mutable", File: r.CurrentFile, Line: reference.Token.Line})
	}
	if exists, initialized := r.localBindingExists(identifier); exists {
		if !initialized {
			panic(&JossError{Type: "UndefinedVariable", Message: fmt.Sprintf("Variable '%s' no inicializada", identifier.Value), File: r.CurrentFile, Line: identifier.Token.Line})
		}
		slot, _ := r.slotForIdentifier(identifier)
		if slot.Constant {
			panic(&JossError{Type: "ConstantAssignment", Message: fmt.Sprintf("La constante '%s' no puede pasarse mediante ref", identifier.Value), File: r.CurrentFile, Line: identifier.Token.Line})
		}
		return r.referenceToIdentifier(identifier)
	}
	if _, exists := r.Variables[identifier.Value]; !exists || !r.sourceMapVisible(identifier.Value) {
		panic(&JossError{Type: "UndefinedVariable", Message: fmt.Sprintf("Variable '%s' no definida", identifier.Value), File: r.CurrentFile, Line: identifier.Token.Line})
	}
	if r.Constants[identifier.Value] {
		panic(&JossError{Type: "ConstantAssignment", Message: fmt.Sprintf("La constante '%s' no puede pasarse mediante ref", identifier.Value), File: r.CurrentFile, Line: identifier.Token.Line})
	}
	return r.referenceToIdentifier(identifier)
}

func hasVariableReference(arguments []interface{}) bool {
	for _, argument := range arguments {
		if _, ok := argument.(*VariableReference); ok {
			return true
		}
	}
	return false
}
