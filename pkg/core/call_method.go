package core

import (
	"fmt"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
	runtimeerrors "github.com/jossecurity/joss/pkg/runtime/errors"
	runtimeplan "github.com/jossecurity/joss/pkg/runtime/plan"
	"github.com/jossecurity/joss/pkg/typesystem"
)

// CallMethod evaluates source expressions and enters the single evaluated
// invocation path. Keeping this adapter thin prevents binding from diverging.
func (r *Runtime) CallMethod(method *parser.MethodStatement, instance *Instance, arguments []parser.Expression) interface{} {
	return r.CallMethodEvaluated(method, instance, r.evaluateCallArguments(arguments))
}

func (r *Runtime) CallMethodEvaluated(method *parser.MethodStatement, instance *Instance, arguments []interface{}) interface{} {
	return r.callMethodEvaluated(method, instance, arguments, nil)
}

func (r *Runtime) callMethodEvaluated(method *parser.MethodStatement, instance *Instance, arguments []interface{}, writeBack *ClosureEnvironment) interface{} {
	return r.callMethodEvaluatedWithPlan(method, instance, arguments, writeBack, r.planForMethod(method))
}

func (r *Runtime) callMethodEvaluatedWithPlan(method *parser.MethodStatement, instance *Instance, arguments []interface{}, writeBack *ClosureEnvironment, compiled *runtimeplan.Callable) (result interface{}) {
	if method.Body == nil {
		return r.executeNativeMethod(instance, method.Name.Value, arguments)
	}
	arguments = r.bindArguments(method, arguments)
	if containsYield(method.Body) && r.currentGenerator == nil {
		generator := newGenerator()
		forked := r.Fork()
		forked.currentGenerator = generator
		forked.generatorIndex = 0

		go func() {
			defer func() {
				recover()
				close(generator.items)
			}()
			forked.callMethodEvaluatedWithPlan(method, instance, arguments, writeBack, compiled)
		}()

		return generator
	}
	if compiled == nil {
		compiled = runtimeplan.CompileMethod(method, instance != nil)
	}
	if len(arguments) < compiled.RequiredCount || len(arguments) > compiled.ParameterCount {
		panic(fmt.Sprintf("Arity Error: %s() espera entre %d y %d argumentos, se recibieron %d", method.Name.Value, compiled.RequiredCount, compiled.ParameterCount, len(arguments)))
	}
	if r.MaxCallDepth <= 0 {
		r.MaxCallDepth = DefaultMaxCallDepth
	}
	if r.callDepth >= r.MaxCallDepth {
		panic(&JossError{
			Type:    "RecursionLimit",
			Message: fmt.Sprintf("La llamada a '%s' excedió el límite de recursión de %d frames", method.Name.Value, r.MaxCallDepth),
			File:    r.CurrentFile,
			Line:    method.Token.Line,
		})
	}
	r.callDepth++
	previousClass := r.currentClass
	if compiled.Owner != "" {
		r.currentClass = compiled.Owner
	}
	stackFrame := runtimeerrors.Frame{Function: method.Name.Value, Class: r.currentClass, File: r.CurrentFile, Line: method.Token.Line, Column: method.Token.Column}
	r.callStack = append(r.callStack, stackFrame)

	parentFrame := r.currentFrame
	callFrame := acquireExecutionFrame(compiled, writeBack != nil)
	r.currentFrame = callFrame
	if instance != nil && compiled.ThisSlot >= 0 {
		callFrame.slots[compiled.ThisSlot].Set(instance)
	}
	if writeBack != nil {
		for index, info := range compiled.Slots {
			if index >= compiled.ParameterCount {
				cleanName := strings.TrimPrefix(info.Name, "$")
				if value, ok := writeBack.Variables[cleanName]; ok {
					callFrame.slots[index].Set(value)
				} else if value, ok := writeBack.Variables["$"+cleanName]; ok {
					callFrame.slots[index].Set(value)
				}
			}
		}
	}

	previousCaptureEnvironment := r.captureEnvironment
	r.captureEnvironment = nil

	defer func() {
		for index := len(callFrame.defers) - 1; index >= 0; index-- {
			deferred := callFrame.defers[index]
			if deferred != nil && deferred.Body != nil {
				r.executeStatement(deferred.Body)
			}
		}
		callFrame.defers = nil

		r.callDepth--
		r.currentClass = previousClass
		if writeBack != nil {
			for name := range writeBack.Variables {
				cleanName := strings.TrimPrefix(name, "$")
				if index, exists := compiled.NameSlots[cleanName]; exists && index >= compiled.ParameterCount {
					slot := &callFrame.slots[index]
					if slot.Initialized {
						value := slot.Value.Interface()
						writeBack.Variables[name] = value
						writeBack.Variables[cleanName] = value
						writeBack.Variables["$"+cleanName] = value
						writeBack.VarTypes[name] = slot.TypeName
						writeBack.Constants[name] = slot.Constant
					}
				}
			}
		}
		r.currentFrame = parentFrame
		releaseExecutionFrame(callFrame)
		r.captureEnvironment = previousCaptureEnvironment

		if panicValue := recover(); panicValue != nil {
			if returnValue, ok := panicValue.(*ReturnPanic); ok {
				result = returnValue.Value
				r.callStack = r.callStack[:len(r.callStack)-1]
			} else {
				if runtimeError, ok := panicValue.(*JossError); ok {
					runtimeError.AttachStack(r.callStack)
				}
				r.callStack = r.callStack[:len(r.callStack)-1]
				panic(panicValue)
			}
		} else {
			r.callStack = r.callStack[:len(r.callStack)-1]
		}
		if compiled.ReturnTypeName != "" {
			if compiled.ReturnTypeName == "void" {
				if result != nil {
					panic(&JossError{
						Type:    "ReturnTypeError",
						Message: fmt.Sprintf("La función '%s' debe retornar void, recibió %T", method.Name.Value, result),
						File:    r.CurrentFile,
						Line:    method.Token.Line,
					})
				}
			} else {
				result = r.coerceToParsedType(result, compiled.ReturnType)
				if !r.checkParsedType(result, compiled.ReturnType) {
					panic(&JossError{
						Type:    "ReturnTypeError",
						Message: fmt.Sprintf("La función '%s' debe retornar %s, recibió %T", method.Name.Value, compiled.ReturnTypeName, result),
						File:    r.CurrentFile,
						Line:    method.Token.Line,
					})
				}
			}
		}
	}()

	for index, parameter := range method.Parameters {
		if parameter.Type.Literal == "" || parameter.Type.Type == parser.VAR {
			panic(&JossError{Type: "ImplicitMixedParameter", Message: fmt.Sprintf("El parámetro $%s requiere un tipo explícito; usa mixed si el dinamismo es intencional", parameter.Name.Value), File: r.CurrentFile, Line: parameter.Name.Token.Line})
		}
		if parameter.ByReference && parameter.DefaultValue != nil {
			panic(&JossError{Type: "ReferenceArgumentError", Message: fmt.Sprintf("El parámetro ref $%s no puede tener valor por defecto", parameter.Name.Value), File: r.CurrentFile, Line: parameter.Name.Token.Line})
		}
		slot := &callFrame.slots[index]
		var value interface{}
		if index < len(arguments) {
			value = arguments[index]
			if _, isDefault := value.(defaultValueMarker); isDefault {
				if parameter.DefaultValue != nil {
					value = r.evaluateExpression(parameter.DefaultValue)
				} else {
					value = nil
				}
			} else {
				if parameter.ByReference {
					reference, ok := value.(*VariableReference)
					if !ok {
						panic(&JossError{Type: "ReferenceArgumentError", Message: fmt.Sprintf("El argumento %d ($%s) debe pasarse con ref", index+1, parameter.Name.Value), File: r.CurrentFile, Line: parameter.Name.Token.Line})
					}
					if parameter.Type.Literal != "" && reference.Type() != parameter.Type.Literal {
						panic(&JossError{Type: "ReferenceTypeError", Message: fmt.Sprintf("La referencia $%s es %s; se requiere exactamente %s", parameter.Name.Value, reference.Type(), parameter.Type.Literal), File: r.CurrentFile, Line: parameter.Name.Token.Line})
					}
					slot.Set(reference)
					if instance != nil && parameter.Visibility.Literal != "" {
						instance.Fields[parameter.Name.Value] = reference.Get()
					}
					continue
				}
				if _, ok := value.(*VariableReference); ok {
					panic(&JossError{Type: "ReferenceArgumentError", Message: fmt.Sprintf("El parámetro $%s no está declarado con ref", parameter.Name.Value), File: r.CurrentFile, Line: parameter.Name.Token.Line})
				}
				if slot.Type.Kind != typesystem.Mixed {
					value = r.coerceToParsedType(value, slot.Type)
					if !r.checkParsedType(value, slot.Type) {
						panic(fmt.Sprintf("Type Error: El argumento %d ($%s) debe ser de tipo %s, se recibió %T", index+1, parameter.Name.Value, parameter.Type.Literal, value))
					}
				}
			}
		} else if parameter.DefaultValue != nil {
			value = r.evaluateExpression(parameter.DefaultValue)
			if slot.Type.Kind != typesystem.Mixed {
				value = r.coerceToParsedType(value, slot.Type)
				if !r.checkParsedType(value, slot.Type) {
					panic(fmt.Sprintf("Type Error: El valor por defecto de $%s debe ser de tipo %s, se recibió %T", parameter.Name.Value, parameter.Type.Literal, value))
				}
			}
		} else {
			value = nil
		}
		slot.Set(value)
		if instance != nil && parameter.Visibility.Literal != "" {
			instance.Fields[parameter.Name.Value] = value
			if parameter.IsConst {
				instance.Constants[parameter.Name.Value] = true
			}
		}
	}

	return r.executeBlock(method.Body)
}
