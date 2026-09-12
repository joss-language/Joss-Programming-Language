package core

import (
	"fmt"
	"reflect"

	"github.com/jossecurity/joss/pkg/parser"
)

// ApplyFunction, CallCallable and CallFunction are compatibility entry points
// over one implementation. Callable kinds must not duplicate argument binding.
func (r *Runtime) ApplyFunction(callable interface{}, arguments []interface{}) interface{} {
	return r.applyFunction(callable, arguments)
}

func (r *Runtime) CallCallable(callable interface{}, arguments []interface{}) interface{} {
	return r.applyFunction(callable, arguments)
}

func (r *Runtime) CallFunction(callable interface{}, arguments []interface{}) interface{} {
	return r.applyFunction(callable, arguments)
}

func (r *Runtime) applyFunction(callable interface{}, arguments []interface{}) interface{} {
	if pluginCallable, ok := callable.(*PluginCallable); ok {
		if hasVariableReference(arguments) {
			panic(&JossError{Type: "ReferenceEscape", Message: "Las referencias mutables no pueden cruzar la frontera de plugins", File: r.CurrentFile})
		}
		if r.PluginRegistry != nil {
			if pluginCallable.ClassName != "" {
				result, err := r.PluginRegistry.CallMethod(pluginCallable.PluginName, pluginCallable.ClassName, pluginCallable.Function, nil, arguments)
				if err != nil {
					panic(fmt.Sprintf("Error en metodo de plugin %s::%s.%s: %v", pluginCallable.PluginName, pluginCallable.ClassName, pluginCallable.Function, err))
				}
				return result
			}
			result, err := r.PluginRegistry.CallFunction(pluginCallable.PluginName, pluginCallable.Function, arguments)
			if err != nil {
				panic(fmt.Sprintf("Error en funcion de plugin %s::%s: %v", pluginCallable.PluginName, pluginCallable.Function, err))
			}
			return result
		}
	}

	if functionName, ok := callable.(string); ok {
		if result, ok := r.callBuiltin(functionName, arguments); ok {
			return result
		}
		if function, ok := r.Functions[functionName]; ok {
			return r.CallMethodEvaluated(function, nil, arguments)
		}
	}

	if closure, ok := callable.(*CapturedFunction); ok {
		return r.callCapturedFunction(closure, arguments)
	}

	if bound, ok := callable.(*BoundMethod); ok {
		if bound.Instance == nil && bound.StaticClass != "" {
			evaluatedArguments := append([]interface{}{}, arguments...)
			classStatement := r.Classes[bound.StaticClass]
			if classStatement == nil {
				classStatement = &parser.ClassStatement{
					Name: &parser.Identifier{Value: bound.StaticClass},
					Body: &parser.BlockStatement{Statements: []parser.Statement{}},
				}
			}
			dummyInstance := &Instance{Class: classStatement, Fields: make(map[string]interface{})}
			return r.executeNativeMethod(dummyInstance, bound.Method.Name.Value, evaluatedArguments)
		}

		if bound.Instance != nil && bound.Instance.Fields != nil && bound.Instance.Class != nil {
			if pluginName, isPlugin := bound.Instance.Fields["__plugin__"].(string); isPlugin && pluginName != "" && r.PluginRegistry != nil {
				result, err := r.PluginRegistry.CallMethod(pluginName, bound.Instance.Class.Name.Value, bound.Method.Name.Value, bound.Instance, arguments)
				if err == nil {
					return result
				}
			}
		}

		if bound.Instance != nil {
			bound.Instance.Mu.RLock()
			destroyed := bound.Instance.Destroyed
			bound.Instance.Mu.RUnlock()
			if destroyed {
				className := "objeto"
				if bound.Instance.Class != nil && bound.Instance.Class.Name != nil {
					className = bound.Instance.Class.Name.Value
				}
				panic(&JossError{Type: "SecurityError", Message: fmt.Sprintf("Acceso denegado: el objeto '%s' ya fue destruido por protección", className), File: r.CurrentFile})
			}
		}

		result := r.CallMethodEvaluated(bound.Method, bound.Instance, arguments)
		if bound.Instance != nil && (bound.Method.Name.Value == "destructor" || bound.Method.Name.Value == "destroy" || bound.Method.Name.Value == "__destruct") {
			bound.Instance.AutoDestroy(r, true)
		}
		return result
	}

	if method, ok := callable.(*parser.MethodStatement); ok {
		return r.CallMethodEvaluated(method, nil, arguments)
	}

	if literal, ok := callable.(*parser.FunctionLiteral); ok {
		method := &parser.MethodStatement{
			Token:      literal.Token,
			Name:       &parser.Identifier{Value: "anonymous"},
			Parameters: literal.Parameters,
			ReturnType: literal.ReturnType,
			Body:       literal.Body,
		}
		return r.callMethodEvaluatedWithPlan(method, nil, arguments, nil, r.planForFunction(literal))
	}

	if handler, ok := callable.(NativeHandler); ok {
		if hasVariableReference(arguments) {
			panic(&JossError{Type: "ReferenceEscape", Message: "Las referencias mutables no pueden pasarse a handlers nativos", File: r.CurrentFile})
		}
		return handler(r, nil, "", arguments)
	}

	if goFunction, ok := callable.(func([]interface{}) interface{}); ok {
		return goFunction(arguments)
	}

	value := reflect.ValueOf(callable)
	if value.IsValid() && value.Kind() == reflect.Func {
		functionType := value.Type()
		inputArguments := []reflect.Value{}
		for index := 0; index < functionType.NumIn() && index < len(arguments); index++ {
			expectedType := functionType.In(index)
			if arguments[index] == nil {
				inputArguments = append(inputArguments, reflect.Zero(expectedType))
			} else {
				argumentValue := reflect.ValueOf(arguments[index])
				if argumentValue.Type().AssignableTo(expectedType) {
					inputArguments = append(inputArguments, argumentValue)
				} else if argumentValue.Type().ConvertibleTo(expectedType) {
					inputArguments = append(inputArguments, argumentValue.Convert(expectedType))
				} else {
					inputArguments = append(inputArguments, reflect.Zero(expectedType))
				}
			}
		}
		results := value.Call(inputArguments)
		if len(results) > 0 {
			return results[0].Interface()
		}
		return nil
	}

	if callable == nil {
		panic(&JossError{Type: "NotCallable", Message: "Intento de invocar un valor nulo", File: r.CurrentFile})
	}

	panic(&JossError{Type: "NotCallable", Message: fmt.Sprintf("'%v' (tipo %T) no es una función invocable", callable, callable), File: r.CurrentFile})
}
