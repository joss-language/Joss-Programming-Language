package core

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
	runtimeerrors "github.com/jossecurity/joss/pkg/runtime/errors"
	runtimeplan "github.com/jossecurity/joss/pkg/runtime/plan"
	"github.com/jossecurity/joss/pkg/typesystem"
)

type NamedValue struct {
	Name  string
	Value interface{}
}

type defaultValueMarker struct{}

func (r *Runtime) bindArguments(method *parser.MethodStatement, args []interface{}) []interface{} {
	hasNamed := false
	for _, a := range args {
		if _, ok := a.(*NamedValue); ok {
			hasNamed = true
			break
		}
	}
	if !hasNamed {
		return args
	}

	positional := make([]interface{}, 0, len(args))
	named := make(map[string]interface{})
	for _, a := range args {
		if nv, ok := a.(*NamedValue); ok {
			named[nv.Name] = nv.Value
		} else {
			positional = append(positional, a)
		}
	}

	resolved := make([]interface{}, len(method.Parameters))
	for i, param := range method.Parameters {
		cleanName := strings.TrimPrefix(param.Name.Value, "$")
		if i < len(positional) {
			resolved[i] = positional[i]
		} else if v, ok := named[cleanName]; ok {
			resolved[i] = v
			delete(named, cleanName)
		} else if param.DefaultValue != nil {
			resolved[i] = defaultValueMarker{}
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

func (r *Runtime) CallMethod(method *parser.MethodStatement, instance *Instance, args []parser.Expression) (res interface{}) {
	evaluated := r.evaluateCallArguments(args)
	return r.CallMethodEvaluated(method, instance, evaluated)
}

func (r *Runtime) CallMethodEvaluated(method *parser.MethodStatement, instance *Instance, args []interface{}) (res interface{}) {
	return r.callMethodEvaluated(method, instance, args, nil)
}

func (r *Runtime) callMethodEvaluated(method *parser.MethodStatement, instance *Instance, args []interface{}, writeBack *ClosureEnvironment) (res interface{}) {
	return r.callMethodEvaluatedWithPlan(method, instance, args, writeBack, r.planForMethod(method))
}

func (r *Runtime) callMethodEvaluatedWithPlan(method *parser.MethodStatement, instance *Instance, args []interface{}, writeBack *ClosureEnvironment, compiled *runtimeplan.Callable) (res interface{}) {
	// Native Method Support
	if method.Body == nil {
		return r.executeNativeMethod(instance, method.Name.Value, args)
	}
	args = r.bindArguments(method, args)
	if containsYield(method.Body) && r.currentGenerator == nil {
		gen := newGenerator()
		forked := r.Fork()
		forked.currentGenerator = gen
		forked.generatorIndex = 0

		go func() {
			defer func() {
				recover()
				close(gen.items)
			}()
			forked.callMethodEvaluatedWithPlan(method, instance, args, writeBack, compiled)
		}()

		return gen
	}
	if compiled == nil {
		compiled = runtimeplan.CompileMethod(method, instance != nil)
	}
	if len(args) < compiled.RequiredCount || len(args) > compiled.ParameterCount {
		panic(fmt.Sprintf("Arity Error: %s() espera entre %d y %d argumentos, se recibieron %d", method.Name.Value, compiled.RequiredCount, compiled.ParameterCount, len(args)))
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
	frame := runtimeerrors.Frame{Function: method.Name.Value, Class: r.currentClass, File: r.CurrentFile, Line: method.Token.Line, Column: method.Token.Column}
	r.callStack = append(r.callStack, frame)

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
				if val, ok := writeBack.Variables[cleanName]; ok {
					callFrame.slots[index].Set(val)
				} else if val, ok := writeBack.Variables["$"+cleanName]; ok {
					callFrame.slots[index].Set(val)
				}
			}
		}
	}

	previousCaptureEnvironment := r.captureEnvironment
	r.captureEnvironment = nil

	defer func() {
		for i := len(callFrame.defers) - 1; i >= 0; i-- {
			d := callFrame.defers[i]
			if d != nil && d.Body != nil {
				r.executeStatement(d.Body)
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
						val := slot.Value.Interface()
						writeBack.Variables[name] = val
						writeBack.Variables[cleanName] = val
						writeBack.Variables["$"+cleanName] = val
						writeBack.VarTypes[name] = slot.TypeName
						writeBack.Constants[name] = slot.Constant
					}
				}
			}
		}
		r.currentFrame = parentFrame
		releaseExecutionFrame(callFrame)
		r.captureEnvironment = previousCaptureEnvironment

		if p := recover(); p != nil {
			if rp, ok := p.(*ReturnPanic); ok {
				res = rp.Value
				r.callStack = r.callStack[:len(r.callStack)-1]
			} else {
				if runtimeError, ok := p.(*JossError); ok {
					runtimeError.AttachStack(r.callStack)
				}
				r.callStack = r.callStack[:len(r.callStack)-1]
				panic(p)
			}
		} else {
			r.callStack = r.callStack[:len(r.callStack)-1]
		}
		if compiled.ReturnTypeName != "" {
			res = r.coerceToParsedType(res, compiled.ReturnType)
			if !r.checkParsedType(res, compiled.ReturnType) {
				panic(&JossError{
					Type:    "ReturnTypeError",
					Message: fmt.Sprintf("La función '%s' debe retornar %s, recibió %T", method.Name.Value, compiled.ReturnTypeName, res),
					File:    r.CurrentFile,
					Line:    method.Token.Line,
				})
			}
		}
	}()

	for index, param := range method.Parameters {
		if param.Type.Literal == "" || param.Type.Type == parser.VAR {
			panic(&JossError{Type: "ImplicitMixedParameter", Message: fmt.Sprintf("El parámetro $%s requiere un tipo explícito; usa mixed si el dinamismo es intencional", param.Name.Value), File: r.CurrentFile, Line: param.Name.Token.Line})
		}
		if param.ByReference && param.DefaultValue != nil {
			panic(&JossError{Type: "ReferenceArgumentError", Message: fmt.Sprintf("El parámetro ref $%s no puede tener valor por defecto", param.Name.Value), File: r.CurrentFile, Line: param.Name.Token.Line})
		}
		slot := &callFrame.slots[index]
		var val interface{}
		if index < len(args) {
			val = args[index]
			if _, isDef := val.(defaultValueMarker); isDef {
				if param.DefaultValue != nil {
					val = r.evaluateExpression(param.DefaultValue)
				} else {
					val = nil
				}
			} else {
				if param.ByReference {
					reference, ok := val.(*VariableReference)
					if !ok {
						panic(&JossError{Type: "ReferenceArgumentError", Message: fmt.Sprintf("El argumento %d ($%s) debe pasarse con ref", index+1, param.Name.Value), File: r.CurrentFile, Line: param.Name.Token.Line})
					}
					if param.Type.Literal != "" && reference.Type() != param.Type.Literal {
						panic(&JossError{Type: "ReferenceTypeError", Message: fmt.Sprintf("La referencia $%s es %s; se requiere exactamente %s", param.Name.Value, reference.Type(), param.Type.Literal), File: r.CurrentFile, Line: param.Name.Token.Line})
					}
					slot.Set(reference)
					if instance != nil && param.Visibility.Literal != "" {
						instance.Fields[param.Name.Value] = reference.Get()
					}
					continue
				}
				if _, ok := val.(*VariableReference); ok {
					panic(&JossError{Type: "ReferenceArgumentError", Message: fmt.Sprintf("El parámetro $%s no está declarado con ref", param.Name.Value), File: r.CurrentFile, Line: param.Name.Token.Line})
				}
				if slot.Type.Kind != typesystem.Mixed {
					val = r.coerceToParsedType(val, slot.Type)
					if !r.checkParsedType(val, slot.Type) {
						panic(fmt.Sprintf("Type Error: El argumento %d ($%s) debe ser de tipo %s, se recibió %T", index+1, param.Name.Value, param.Type.Literal, val))
					}
				}
			}
		} else if param.DefaultValue != nil {
			val = r.evaluateExpression(param.DefaultValue)
			if slot.Type.Kind != typesystem.Mixed {
				val = r.coerceToParsedType(val, slot.Type)
				if !r.checkParsedType(val, slot.Type) {
					panic(fmt.Sprintf("Type Error: El valor por defecto de $%s debe ser de tipo %s, se recibió %T", param.Name.Value, param.Type.Literal, val))
				}
			}
		} else {
			val = nil
		}
		slot.Set(val)
		if instance != nil && param.Visibility.Literal != "" {
			instance.Fields[param.Name.Value] = val
			if param.IsConst {
				instance.Constants[param.Name.Value] = true
			}
		}
	}

	return r.executeBlock(method.Body)
}

func (r *Runtime) executeCall(call *parser.CallExpression) interface{} {
	// 1. Evaluate arguments first
	args := r.evaluateCallArguments(call.Arguments)

	// 2. Try Builtin (only if not shadowed by user function or local)
	if ident, ok := call.Function.(*parser.Identifier); ok {
		_, hasUserFunc := r.Functions[ident.Value]
		_, hasLocal, _ := r.localValue(ident)
		if !hasUserFunc && !hasLocal {
			if hasVariableReference(args) && IsBuiltin(ident.Value) {
				panic(&JossError{Type: "ReferenceEscape", Message: fmt.Sprintf("La función nativa '%s' no declara parámetros ref", ident.Value), File: r.CurrentFile, Line: ident.Token.Line})
			}
			if res, ok := r.callBuiltin(ident.Value, args); ok {
				return res
			}
		}
	}

	// 3. For plain identifiers: check r.Functions first, then r.Variables.
	//    This avoids a false "UndefinedVariable" panic for user-defined functions.
	var fn interface{}
	if ident, ok := call.Function.(*parser.Identifier); ok {
		if f, ok := r.Functions[ident.Value]; ok {
			fn = f
		} else if value, resolved, initialized := r.localValue(ident); resolved && initialized {
			fn = value
		} else if v, ok := r.Variables[ident.Value]; ok && r.sourceMapVisible(ident.Value) {
			fn = v
		}
	} else {
		// Non-identifier (e.g., member expression, closure variable): evaluate normally.
		fn = r.evaluateExpression(call.Function)
	}

	if fn == nil {
		if mem, ok := call.Function.(*parser.MemberExpression); ok && (mem.NullSafe || mem.Token.Type == parser.NULL_SAFE_ARROW) {
			return nil
		}
		if ident, ok := call.Function.(*parser.Identifier); ok {
			panic(&JossError{
				Type:    "UndefinedFunction",
				Message: fmt.Sprintf("Función '%s' no encontrada", ident.Value),
				File:    r.CurrentFile,
				Line:    ident.Token.Line,
			})
		}
		panic(&JossError{
			Type:    "NotCallable",
			Message: "Intento de invocar un objeto nulo o no invocable",
			File:    r.CurrentFile,
		})
	}

	return r.applyFunction(fn, args)
}

func (r *Runtime) evaluateCallArguments(rawArgs []parser.Expression) []interface{} {
	args := make([]interface{}, 0, len(rawArgs))
	for _, arg := range rawArgs {
		if spread, ok := arg.(*parser.SpreadExpression); ok {
			val := r.evaluateExpression(spread.Expression)
			if slice, ok := val.([]interface{}); ok {
				args = append(args, slice...)
				continue
			}
			rv := reflect.ValueOf(val)
			if rv.IsValid() && rv.Kind() == reflect.Slice {
				for i := 0; i < rv.Len(); i++ {
					args = append(args, rv.Index(i).Interface())
				}
				continue
			}
			if m, ok := val.(map[string]interface{}); ok {
				for k, v := range m {
					args = append(args, &NamedValue{Name: k, Value: v})
				}
				continue
			}
			if m, ok := val.(map[interface{}]interface{}); ok {
				for k, v := range m {
					args = append(args, &NamedValue{Name: fmt.Sprintf("%v", k), Value: v})
				}
				continue
			}
		}
		args = append(args, r.evaluateCallArgument(arg))
	}
	return args
}

func (r *Runtime) evaluateCallArgument(argument parser.Expression) interface{} {
	if named, ok := argument.(*parser.NamedArgument); ok {
		val := r.evaluateCallArgument(named.Value)
		return &NamedValue{Name: named.Name, Value: val}
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

func (r *Runtime) ApplyFunction(fn interface{}, args []interface{}) interface{} {
	return r.applyFunction(fn, args)
}

func (r *Runtime) CallCallable(fn interface{}, args []interface{}) interface{} {
	return r.applyFunction(fn, args)
}

func (r *Runtime) applyFunction(fn interface{}, args []interface{}) interface{} {
	if callable, ok := fn.(*PluginCallable); ok {
		if hasVariableReference(args) {
			panic(&JossError{Type: "ReferenceEscape", Message: "Las referencias mutables no pueden cruzar la frontera de plugins", File: r.CurrentFile})
		}
		if r.PluginRegistry != nil {
			if callable.ClassName != "" {
				res, err := r.PluginRegistry.CallMethod(callable.PluginName, callable.ClassName, callable.Function, nil, args)
				if err != nil {
					panic(fmt.Sprintf("Error en metodo de plugin %s::%s.%s: %v", callable.PluginName, callable.ClassName, callable.Function, err))
				}
				return res
			}
			res, err := r.PluginRegistry.CallFunction(callable.PluginName, callable.Function, args)
			if err != nil {
				panic(fmt.Sprintf("Error en funcion de plugin %s::%s: %v", callable.PluginName, callable.Function, err))
			}
			return res
		}
	}

	if fnStr, ok := fn.(string); ok {
		if res, ok := r.callBuiltin(fnStr, args); ok {
			return res
		}
		if f, ok := r.Functions[fnStr]; ok {
			return r.CallMethodEvaluated(f, nil, args)
		}
	}

	if closure, ok := fn.(*CapturedFunction); ok {
		return r.callCapturedFunction(closure, args)
	}

	if bound, ok := fn.(*BoundMethod); ok {
		if bound.Instance == nil && bound.StaticClass != "" {
			evalArgs := []interface{}{}
			for _, arg := range args {
				evalArgs = append(evalArgs, arg)
			}
			classStmt := r.Classes[bound.StaticClass]
			if classStmt == nil {
				classStmt = &parser.ClassStatement{
					Name: &parser.Identifier{Value: bound.StaticClass},
					Body: &parser.BlockStatement{Statements: []parser.Statement{}},
				}
			}
			dummyInstance := &Instance{
				Class:  classStmt,
				Fields: make(map[string]interface{}),
			}
			return r.executeNativeMethod(dummyInstance, bound.Method.Name.Value, evalArgs)
		}

		if bound.Instance != nil && bound.Instance.Fields != nil && bound.Instance.Class != nil {
			if pluginName, isPlug := bound.Instance.Fields["__plugin__"].(string); isPlug && pluginName != "" {
				if r.PluginRegistry != nil {
					res, err := r.PluginRegistry.CallMethod(pluginName, bound.Instance.Class.Name.Value, bound.Method.Name.Value, bound.Instance, args)
					if err == nil {
						return res
					}
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
				panic(&JossError{
					Type:    "SecurityError",
					Message: fmt.Sprintf("Acceso denegado: el objeto '%s' ya fue destruido por protección", className),
					File:    r.CurrentFile,
				})
			}
		}

		res := r.CallMethodEvaluated(bound.Method, bound.Instance, args)
		if bound.Instance != nil && (bound.Method.Name.Value == "destructor" || bound.Method.Name.Value == "destroy" || bound.Method.Name.Value == "__destruct") {
			bound.Instance.AutoDestroy(r, true)
		}
		return res
	}

	if method, ok := fn.(*parser.MethodStatement); ok {
		return r.CallMethodEvaluated(method, nil, args)
	}

	if lit, ok := fn.(*parser.FunctionLiteral); ok {
		method := &parser.MethodStatement{
			Token:      lit.Token,
			Name:       &parser.Identifier{Value: "anonymous"},
			Parameters: lit.Parameters,
			ReturnType: lit.ReturnType,
			Body:       lit.Body,
		}
		return r.callMethodEvaluatedWithPlan(method, nil, args, nil, r.planForFunction(lit))
	}

	if handler, ok := fn.(NativeHandler); ok {
		if hasVariableReference(args) {
			panic(&JossError{Type: "ReferenceEscape", Message: "Las referencias mutables no pueden pasarse a handlers nativos", File: r.CurrentFile})
		}
		return handler(r, nil, "", args)
	}

	if goFn, ok := fn.(func([]interface{}) interface{}); ok {
		return goFn(args)
	}

	val := reflect.ValueOf(fn)
	if val.Kind() == reflect.Func {
		fnType := val.Type()
		inArgs := []reflect.Value{}
		for i := 0; i < fnType.NumIn() && i < len(args); i++ {
			expectedType := fnType.In(i)
			if args[i] == nil {
				inArgs = append(inArgs, reflect.Zero(expectedType))
			} else {
				argVal := reflect.ValueOf(args[i])
				if argVal.Type().AssignableTo(expectedType) {
					inArgs = append(inArgs, argVal)
				} else if argVal.Type().ConvertibleTo(expectedType) {
					inArgs = append(inArgs, argVal.Convert(expectedType))
				} else {
					inArgs = append(inArgs, reflect.Zero(expectedType))
				}
			}
		}
		results := val.Call(inArgs)
		if len(results) > 0 {
			return results[0].Interface()
		}
		return nil
	}

	if fn == nil {
		panic(&JossError{
			Type:    "NotCallable",
			Message: "Intento de invocar un valor nulo",
			File:    r.CurrentFile,
		})
	}

	panic(&JossError{
		Type:    "NotCallable",
		Message: fmt.Sprintf("'%v' (tipo %T) no es una función invocable", fn, fn),
		File:    r.CurrentFile,
	})
}

func hasVariableReference(args []interface{}) bool {
	for _, argument := range args {
		if _, ok := argument.(*VariableReference); ok {
			return true
		}
	}
	return false
}

// CallFunction is the public API for executing functions
func (r *Runtime) CallFunction(fn interface{}, args []interface{}) interface{} {
	return r.applyFunction(fn, args)
}

// callBuiltin dispatches to domain-specific built-in handlers
func (r *Runtime) callBuiltin(name string, args []interface{}) (interface{}, bool) {
	if !IsBuiltin(name) {
		return nil, false
	}
	// 1. Date & Time Builtins
	if res, ok := r.callBuiltinDate(name, args); ok {
		return res, true
	}

	// 2. String & Formatting Builtins
	if res, ok := r.callBuiltinString(name, args); ok {
		return res, true
	}

	// 3. Array & Map Builtins
	if res, ok := r.callBuiltinArray(name, args); ok {
		return res, true
	}

	// 4. Async & Channels Builtins
	if res, ok := r.callBuiltinAsync(name, args); ok {
		return res, true
	}

	// 5. IO, Web & Formats Builtins
	if res, ok := r.callBuiltinIO(name, args); ok {
		return res, true
	}

	panic(fmt.Sprintf("internal error: builtin %q is catalogued without a runtime handler", name))
}
