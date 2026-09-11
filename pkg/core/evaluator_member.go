package core

import (
	"fmt"
	"runtime"

	"github.com/jossecurity/joss/pkg/parser"
)

func (r *Runtime) evaluateNew(ne *parser.NewExpression) interface{} {
	className := ne.Class.Value

	evalArgs := r.evaluateCallArguments(ne.Arguments)

	// Check PluginRegistry for exported class instantiation first
	if r.PluginRegistry != nil {
		for _, p := range r.PluginRegistry.List() {
			for _, cls := range p.Symbols.Classes {
				if cls.Name == className {
					res, err := r.PluginRegistry.Instantiate(p.Name, cls.Name, evalArgs)
					if err == nil && res != nil {
						return res
					}
					if err != nil {
						panic(fmt.Sprintf("Error instanciando clase de plugin %s::%s: %v", p.Name, cls.Name, err))
					}
				}
			}
		}
	}

	classStmt, ok := r.Classes[className]
	if !ok {
		panic(&JossError{
			Type:    "UndefinedClass",
			Message: fmt.Sprintf("Clase '%s' no encontrada", className),
			File:    r.CurrentFile,
			Line:    ne.Class.Token.Line,
		})
	}

	if classStmt.IsAbstract {
		panic(&JossError{
			Type:    "InstantiationError",
			Message: fmt.Sprintf("No se puede instanciar la clase abstracta '%s'", className),
			File:    r.CurrentFile,
			Line:    ne.Class.Token.Line,
		})
	}

	instance := &Instance{
		Class:     classStmt,
		Fields:    make(map[string]interface{}),
		Constants: make(map[string]bool),
	}

	meta := r.lookupClassMetadata(className)
	if meta != nil {
		for _, field := range meta.FieldOrder {
			var value interface{}
			if field.Declaration.Value != nil {
				value = r.evaluateExpression(field.Declaration.Value)
			} else {
				value = r.getZeroValue(field.DeclaredType)
			}
			if field.DeclaredType != "" && field.DeclaredType != "var" && field.DeclaredType != "mixed" {
				value = r.coerceToParsedType(value, field.ParsedType)
				if !r.checkParsedType(value, field.ParsedType) {
					panic(&JossError{
						Type:    "PropertyTypeError",
						Message: fmt.Sprintf("La propiedad '%s' requiere %s", field.Declaration.Name.Value, field.DeclaredType),
						File:    r.CurrentFile,
						Line:    field.Declaration.Name.Token.Line,
					})
				}
			}
			instance.Fields[field.Declaration.Name.Value] = value
			if field.IsConst {
				instance.Constants[field.Declaration.Name.Value] = true
			}
		}

		if meta.Constructor != nil {
			r.requireMemberAccess(meta.Constructor.Visibility, className, meta.Constructor.Name.Value, meta.Constructor.Name.Token.Line)
			r.CallMethod(meta.Constructor, instance, ne.Arguments)
		}
	}

	capturedRuntime := r.Fork()
	runtime.SetFinalizer(instance, func(inst *Instance) {
		inst.AutoDestroy(capturedRuntime)
	})

	return instance
}

func (r *Runtime) evaluateMember(me *parser.MemberExpression) interface{} {
	// 1. Support Static Class Access (e.g. Turnstile::siteKey, GranDB::table, Session::get, etc.)
	if ident, ok := me.Left.(*parser.Identifier); ok {
		className := ident.Value

		// Check Enum
		if enumDef, ok := r.Enums[className]; ok {
			caseName := me.Property.Value
			if c, ok := enumDef.Cases[caseName]; ok {
				return c
			}
			if caseName == "cases" {
				return func(args []interface{}) interface{} {
					res := make([]interface{}, len(enumDef.CaseOrder))
					for i, v := range enumDef.CaseOrder {
						res[i] = v
					}
					return res
				}
			}
			if caseName == "from" {
				return func(args []interface{}) interface{} {
					if len(args) == 0 {
						panic(&JossError{Type: "ValueError", Message: "from() requiere un valor", File: r.CurrentFile})
					}
					target := args[0]
					for _, c := range enumDef.CaseOrder {
						if fmt.Sprintf("%v", c.Value) == fmt.Sprintf("%v", target) {
							return c
						}
					}
					panic(&JossError{Type: "ValueError", Message: fmt.Sprintf("'%v' no es un valor válido para el enum %s", target, className), File: r.CurrentFile})
				}
			}
			if caseName == "tryFrom" {
				return func(args []interface{}) interface{} {
					if len(args) == 0 {
						return nil
					}
					target := args[0]
					for _, c := range enumDef.CaseOrder {
						if fmt.Sprintf("%v", c.Value) == fmt.Sprintf("%v", target) {
							return c
						}
					}
					return nil
				}
			}
			panic(&JossError{
				Type:    "UndefinedEnumCase",
				Message: fmt.Sprintf("El caso '%s' no existe en el enum '%s'", caseName, className),
				File:    r.CurrentFile,
			})
		}

		// Check user-defined class in r.Classes
		if classStmt, ok := r.Classes[className]; ok {
			if _, native := r.NativeHandlers[className]; native {
				return &BoundMethod{
					Method:      &parser.MethodStatement{Name: &parser.Identifier{Value: me.Property.Value}},
					StaticClass: className,
				}
			}
			meta := r.lookupClassMetadata(className)
			if meta != nil {
				if methodInfo, ok := meta.Methods[me.Property.Value]; ok {
					r.requireMemberAccess(methodInfo.Method.Visibility, className, methodInfo.Method.Name.Value, methodInfo.Method.Name.Token.Line)
					return &BoundMethod{Method: methodInfo.Method, Instance: &Instance{Class: classStmt, Fields: make(map[string]interface{})}}
				}
			}
		}

		// Check native class
		if r.isNativeClass(className) || IsNativeClass(className) {
			return &BoundMethod{
				Method: &parser.MethodStatement{
					Name: &parser.Identifier{Value: me.Property.Value},
					Body: nil,
				},
				Instance:    nil,
				StaticClass: className,
			}
		}

		// Check loaded plugin in PluginRegistry
		if r.PluginRegistry != nil && r.PluginRegistry.Get(className) != nil {
			return &PluginCallable{
				PluginName: className,
				Function:   me.Property.Value,
			}
		}

		// Check if className is an exported class or function across any loaded plugin
		if r.PluginRegistry != nil {
			for _, p := range r.PluginRegistry.List() {
				for _, cls := range p.Symbols.Classes {
					if cls.Name == className {
						return &PluginCallable{
							PluginName: p.Name,
							ClassName:  cls.Name,
							Function:   me.Property.Value,
						}
					}
				}
				for _, fn := range p.Symbols.Functions {
					if fn.Name == me.Property.Value && p.Name == className {
						return &PluginCallable{
							PluginName: p.Name,
							Function:   fn.Name,
						}
					}
				}
			}
		}
	}

	left := r.evaluateExpression(me.Left)
	if left == nil && (me.NullSafe || me.Token.Type == parser.NULL_SAFE_ARROW) {
		return nil
	}

	// Support Plugin Namespace access (e.g. plugin_name::function or plugin_name.function)
	if ns, ok := left.(*PluginNamespace); ok {
		return &PluginCallable{
			PluginName: ns.Name,
			Function:   me.Property.Value,
		}
	}

	// Support Map access via dot notation (e.g. $item.id where $item is a map) or fluent methods
	if m, ok := left.(map[string]interface{}); ok {
		if val, exists := m[me.Property.Value]; exists {
			return val
		}
		if fn, ok := r.resolveMapMethod(m, me.Property); ok {
			return fn
		}
		return nil
	}

	// Support fluent methods on string ($str->trim(), $str->lower(), etc.)
	if s, ok := left.(string); ok {
		if fn, ok := r.resolveStringMethod(s, me.Property); ok {
			return fn
		}
	}

	// Support fluent methods on array ($arr->map(...), $arr->join(...), etc.)
	if arr, ok := left.([]interface{}); ok {
		if fn, ok := r.resolveArrayMethod(arr, me.Property); ok {
			return fn
		}
	}

	// Support EnumValue properties (e.g. $status.name, $status.value)
	if ev, ok := left.(*EnumValue); ok {
		if me.Property.Value == "name" {
			return ev.Name
		}
		if me.Property.Value == "value" {
			return ev.Value
		}
		return nil
	}

	// Support Generator methods ($gen->next(), $gen->current(), $gen->key(), $gen->valid())
	if gen, ok := left.(*Generator); ok {
		switch me.Property.Value {
		case "current":
			return func(args []interface{}) interface{} {
				return gen.Current()
			}
		case "key":
			return func(args []interface{}) interface{} {
				return gen.Key()
			}
		case "next":
			return func(args []interface{}) interface{} {
				gen.Next()
				return nil
			}
		case "valid":
			return func(args []interface{}) interface{} {
				return gen.Valid()
			}
		}
	}

	instance, ok := left.(*Instance)
	if !ok || instance == nil {
		if ident, ok := me.Left.(*parser.Identifier); ok {
			className := ident.Value

			// Check if ident is a loaded plugin name in PluginRegistry
			if r.PluginRegistry != nil && r.PluginRegistry.Get(className) != nil {
				return &PluginCallable{
					PluginName: className,
					Function:   me.Property.Value,
				}
			}

			if classStmt, ok := r.Classes[className]; ok {
				if _, native := r.NativeHandlers[className]; native {
					return &BoundMethod{
						Method:      &parser.MethodStatement{Name: &parser.Identifier{Value: me.Property.Value}},
						StaticClass: className,
					}
				}
				for _, stmt := range classStmt.Body.Statements {
					if method, methodOK := stmt.(*parser.MethodStatement); methodOK && method.Name.Value == me.Property.Value {
						return &BoundMethod{Method: method, Instance: &Instance{Class: classStmt, Fields: make(map[string]interface{})}}
					}
				}
			} else if r.isNativeClass(className) {
				return &BoundMethod{
					Method: &parser.MethodStatement{
						Name: &parser.Identifier{Value: me.Property.Value},
						Body: nil,
					},
					Instance:    nil,
					StaticClass: className,
				}
			}

			// Fallback: check if className is a function/class exported by any loaded plugin
			if r.PluginRegistry != nil {
				for _, p := range r.PluginRegistry.List() {
					for _, fn := range p.Symbols.Functions {
						if fn.Name == me.Property.Value && p.Name == className {
							return &PluginCallable{
								PluginName: p.Name,
								Function:   fn.Name,
							}
						}
					}
					for _, cls := range p.Symbols.Classes {
						if cls.Name == className {
							return &PluginCallable{
								PluginName: p.Name,
								ClassName:  cls.Name,
								Function:   me.Property.Value,
							}
						}
					}
				}
			}

			if left == nil {
				panic(&JossError{
					Type:    "NullReference",
					Message: fmt.Sprintf("Clase o namespace '%s' no encontrado o es nulo", className),
					File:    r.CurrentFile,
					Line:    me.Property.Token.Line,
				})
			}

			panic(&JossError{
				Type:    "UndefinedClass",
				Message: fmt.Sprintf("Clase o plugin '%s' no registrado. Si pertenece a un plugin, verifique joss.yaml y la instalación del paquete", className),
				File:    r.CurrentFile,
				Line:    me.Property.Token.Line,
			})
		}

		if left == nil {
			if me.NullSafe || me.Token.Type == parser.NULL_SAFE_ARROW {
				return nil
			}
			panic(&JossError{
				Type:    "NullReference",
				Message: fmt.Sprintf("Intento de acceder a la propiedad '%s' sobre un valor nulo o no instanciado", me.Property.Value),
				File:    r.CurrentFile,
				Line:    me.Property.Token.Line,
			})
		}

		panic(&JossError{
			Type:    "NotAnInstance",
			Message: fmt.Sprintf("'%v' (tipo %T) no es una instancia. Intentando acceder a: '%s'", left, left, me.Property.Value),
			File:    r.CurrentFile,
			Line:    me.Property.Token.Line,
		})
	}

	instance.Mu.RLock()
	destroyed := instance.Destroyed
	instance.Mu.RUnlock()
	if destroyed {
		className := "objeto"
		if instance.Class != nil && instance.Class.Name != nil {
			className = instance.Class.Name.Value
		}
		panic(&JossError{
			Type:    "SecurityError",
			Message: fmt.Sprintf("Acceso denegado: el objeto '%s' ya fue destruido por protección", className),
			File:    r.CurrentFile,
			Line:    me.Property.Token.Line,
		})
	}

	propName := me.Property.Value

	if instance.Fields == nil {
		instance.Fields = make(map[string]interface{})
	}

	if val, ok := instance.Fields[propName]; ok {
		if declaration, owner := r.lookupInstanceFieldOwner(instance, propName); declaration != nil {
			r.requireMemberAccess(declaration.Visibility, owner, propName, me.Property.Token.Line)
		}
		return val
	}

	if instance.Class != nil && instance.Class.Name != nil {
		meta := r.lookupClassMetadata(instance.Class.Name.Value)
		if meta != nil {
			if methodInfo, ok := meta.Methods[propName]; ok {
				r.requireMemberAccess(methodInfo.Method.Visibility, methodInfo.OwnerClass, propName, me.Property.Token.Line)
				return &BoundMethod{Method: methodInfo.Method, Instance: instance}
			}
		}
	}

	checkClass := instance.Class
	isNative := false
	for checkClass != nil {
		className := checkClass.Name.Value
		if r.isNativeClass(className) {
			isNative = true
			break
		}
		if checkClass.SuperClass != nil {
			if parent, ok := r.Classes[checkClass.SuperClass.Value]; ok {
				checkClass = parent
			} else {
				break
			}
		} else {
			break
		}
	}

	if isNative {
		return &BoundMethod{
			Method: &parser.MethodStatement{
				Name: &parser.Identifier{Value: propName},
				Body: nil,
			},
			Instance: instance,
		}
	}

	if r.PluginRegistry != nil && instance.Class != nil {
		className := instance.Class.Name.Value
		for _, p := range r.PluginRegistry.List() {
			for _, cls := range p.Symbols.Classes {
				if cls.Name == className {
					for _, m := range cls.Methods {
						if m.Name == propName {
							return &BoundMethod{
								Method: &parser.MethodStatement{
									Name: &parser.Identifier{Value: propName},
									Body: nil,
								},
								Instance: instance,
							}
						}
					}
				}
			}
		}
	}

	className := "Anonymous"
	if instance.Class != nil {
		className = instance.Class.Name.Value
	}
	panic(&JossError{
		Type:    "UndefinedProperty",
		Message: fmt.Sprintf("Propiedad o método '%s' no encontrado en clase '%s'", propName, className),
		File:    r.CurrentFile,
		Line:    me.Property.Token.Line,
	})
}
