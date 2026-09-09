package core

import (
	"fmt"
	"strconv"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	runtimevalue "github.com/jossecurity/joss/pkg/runtime/value"
)

func (r *Runtime) evaluateAssign(ae *parser.AssignExpression) interface{} {
	if ident, ok := ae.Left.(*parser.Identifier); ok {
		if slot, resolved := r.slotForIdentifier(ident); resolved && slot.Constant && slot.Initialized {
			panic(&JossError{Type: "ConstantAssignment", Message: fmt.Sprintf("La constante '%s' no puede reasignarse", ident.Value), File: r.CurrentFile, Line: ident.Token.Line})
		}
		if r.Constants[ident.Value] && r.sourceMapVisible(ident.Value) {
			panic(&JossError{Type: "ConstantAssignment", Message: fmt.Sprintf("La constante '%s' no puede reasignarse", ident.Value), File: r.CurrentFile, Line: ident.Token.Line})
		}
	}
	val := r.evaluateExpression(ae.Value)

	if ident, ok := ae.Left.(*parser.Identifier); ok {
		if assigned, resolved := r.assignLocal(ident, val, false); resolved {
			return assigned
		}
		if reference, exists := r.Variables[ident.Value].(*VariableReference); exists {
			return reference.Set(r, val)
		}
		if expectedType, exists := r.VarTypes[ident.Value]; exists {
			if expectedType != "mixed" {
				val = r.coerceToTypedValue(val, expectedType)
				if !r.checkType(val, expectedType) {
					panic(fmt.Sprintf("Error de Tipado: No se puede asignar valor a '%s' (se espera %s)", ident.Value, expectedType))
				}
			}
		} else if _, alreadyExists := r.Variables[ident.Value]; !alreadyExists || r.Variables[ident.Value] == nil {
			if inferredType := runtimeTypeName(val); inferredType != "" {
				r.VarTypes[ident.Value] = inferredType
			}
		}
		r.Variables[ident.Value] = val
		return val
	}

	if arrLit, ok := ae.Left.(*parser.ArrayLiteral); ok {
		var items []interface{}
		if list, ok := val.([]interface{}); ok {
			items = list
		} else if listMap, ok := val.([]map[string]interface{}); ok {
			for _, item := range listMap {
				items = append(items, item)
			}
		} else {
			items = []interface{}{val}
		}

		for i, elem := range arrLit.Elements {
			var elemVal interface{}
			if i < len(items) {
				elemVal = items[i]
			}
			if ident, ok := elem.(*parser.Identifier); ok {
				if assigned, resolved := r.assignLocal(ident, elemVal, false); resolved {
					_ = assigned
					continue
				}
				if reference, exists := r.Variables[ident.Value].(*VariableReference); exists {
					reference.Set(r, elemVal)
					continue
				}
				if expectedType, exists := r.VarTypes[ident.Value]; exists {
					if expectedType != "mixed" {
						elemVal = r.coerceToTypedValue(elemVal, expectedType)
						if !r.checkType(elemVal, expectedType) {
							panic(fmt.Sprintf("Error de Tipado: No se puede asignar valor a '%s' (se espera %s)", ident.Value, expectedType))
						}
					}
				} else if _, alreadyExists := r.Variables[ident.Value]; !alreadyExists || r.Variables[ident.Value] == nil {
					if inferredType := runtimeTypeName(elemVal); inferredType != "" {
						r.VarTypes[ident.Value] = inferredType
					}
				}
				r.Variables[ident.Value] = elemVal
			}
		}
		return val
	}

	if member, ok := ae.Left.(*parser.MemberExpression); ok {
		left := r.evaluateExpression(member.Left)
		if instance, ok := left.(*Instance); ok {
			return r.setInstanceField(instance, member.Property.Value, val, member.Property.Token.Line)
		}
		fmt.Printf("Error: Asignación a miembro de no-instancia: %v\n", left)
		return nil
	}

	if indexExp, ok := ae.Left.(*parser.IndexExpression); ok {
		left := r.evaluateExpression(indexExp.Left)

		if indexExp.Index == nil {
			if list, ok := left.([]interface{}); ok {
				newList := append(list, val)
				return r.updateVariable(indexExp.Left, newList)
			}
			fmt.Println("Error: Append [] solo permitido en arrays")
			return nil
		}

		index := r.evaluateExpression(indexExp.Index)

		if m, ok := left.(map[string]interface{}); ok {
			var key string
			if k, ok := index.(string); ok {
				key = k
			} else if n, ok := toInt64Safe(index); ok {
				key = strconv.FormatInt(n, 10)
			}
			if key != "" {
				m[key] = val
				return val
			}
		}

		if list, ok := left.([]interface{}); ok {
			if idx, ok := index.(int64); ok {
				if idx >= 0 && idx < int64(len(list)) {
					list[idx] = val
					return val
				}
			}
		}
	}

	fmt.Printf("Error: Asignación inválida a %T\n", ae.Left)
	return nil
}

func (r *Runtime) evaluateArray(al *parser.ArrayLiteral) []interface{} {
	elements := []interface{}{}
	for _, el := range al.Elements {
		elements = append(elements, r.evaluateExpression(el))
	}
	return elements
}

func (r *Runtime) evaluateMap(ml *parser.MapLiteral) map[string]interface{} {
	m := make(map[string]interface{})
	for k, v := range ml.Pairs {
		key := r.evaluateExpression(k)
		val := r.evaluateExpression(v)
		if keyStr, ok := key.(string); ok {
			m[keyStr] = val
		} else if num, ok := toInt64Safe(key); ok {
			m[strconv.FormatInt(num, 10)] = val
		} else {
			m[fmt.Sprintf("%v", key)] = val
		}
	}
	return m
}

func (r *Runtime) evaluateIndex(ie *parser.IndexExpression) interface{} {
	left := r.evaluateExpression(ie.Left)
	index := r.evaluateExpression(ie.Index)

	if list, ok := left.([]interface{}); ok {
		if idx, ok := index.(int64); ok {
			if idx >= 0 && idx < int64(len(list)) {
				return list[idx]
			}
			panic(&JossError{Code: diagnostics.CodeIndexOutOfRange, Type: "IndexError", Message: fmt.Sprintf("Índice %d fuera de rango para array de longitud %d", idx, len(list)), File: r.CurrentFile, Line: ie.Token.Line, Column: ie.Token.Column})
		} else {
			panic(&JossError{Code: diagnostics.CodeInvalidIndexType, Type: "TypeError", Message: fmt.Sprintf("El índice de array debe ser int, se recibió %T", index), File: r.CurrentFile, Line: ie.Token.Line, Column: ie.Token.Column})
		}
	}

	if m, ok := left.(map[string]interface{}); ok {
		var key string
		if k, ok := index.(string); ok {
			key = k
		} else if n, ok := toInt64Safe(index); ok {
			key = strconv.FormatInt(n, 10)
		} else {
			panic(&JossError{Code: diagnostics.CodeInvalidIndexType, Type: "TypeError", Message: fmt.Sprintf("El índice de map debe ser string o int, se recibió %T", index), File: r.CurrentFile, Line: ie.Token.Line, Column: ie.Token.Column})
		}
		if val, exists := m[key]; exists {
			return val
		}
		return nil
	}

	if str, ok := left.(string); ok {
		if idx, ok := index.(int64); ok {
			if character, exists := runtimevalue.StringIndex(str, idx); exists {
				return character
			}
			panic(&JossError{Code: diagnostics.CodeIndexOutOfRange, Type: "IndexError", Message: fmt.Sprintf("Índice %d fuera de rango para string", idx), File: r.CurrentFile, Line: ie.Token.Line, Column: ie.Token.Column})
		}
		panic(&JossError{Code: diagnostics.CodeInvalidIndexType, Type: "TypeError", Message: fmt.Sprintf("El índice de string debe ser int, se recibió %T", index), File: r.CurrentFile, Line: ie.Token.Line, Column: ie.Token.Column})
	}

	panic(&JossError{Code: diagnostics.CodeInvalidIndexType, Type: "TypeError", Message: fmt.Sprintf("No se puede indexar un valor de tipo %T", left), File: r.CurrentFile, Line: ie.Token.Line, Column: ie.Token.Column})
}

func (r *Runtime) evaluateIsset(ie *parser.IssetExpression) bool {
	for _, arg := range ie.Arguments {
		if !r.checkExistence(arg) {
			return false
		}
	}
	return true
}

func (r *Runtime) evaluateEmpty(ee *parser.EmptyExpression) bool {
	if !r.checkExistence(ee.Argument) {
		return true
	}

	val := r.evaluateExpression(ee.Argument)
	return isFalsy(val)
}

func (r *Runtime) updateVariable(exp parser.Expression, newVal interface{}) interface{} {
	if ident, ok := exp.(*parser.Identifier); ok {
		if assigned, resolved := r.assignLocal(ident, newVal, false); resolved {
			return assigned
		}
		r.Variables[ident.Value] = newVal
		return newVal
	}
	if member, ok := exp.(*parser.MemberExpression); ok {
		left := r.evaluateExpression(member.Left)
		if instance, ok := left.(*Instance); ok {
			return r.setInstanceField(instance, member.Property.Value, newVal, member.Property.Token.Line)
		}
	}
	fmt.Println("Error: No se puede actualizar la variable (expresión no soportada)")
	return nil
}

func (r *Runtime) setInstanceField(instance *Instance, name string, value interface{}, line int) interface{} {
	if instance.Constants != nil && instance.Constants[name] {
		panic(&JossError{
			Type:    "ConstantAssignment",
			Message: fmt.Sprintf("La propiedad constante '%s' no puede reasignarse", name),
			File:    r.CurrentFile,
			Line:    line,
		})
	}
	if declaration, owner := r.lookupInstanceFieldOwner(instance, name); declaration != nil {
		r.requireMemberAccess(declaration.Visibility, owner, name, line)
		declaredType := declaration.Token.Literal
		if declaredType != "" && declaredType != "var" && declaredType != "mixed" {
			value = r.coerceToTypedValue(value, declaredType)
			if !r.checkType(value, declaredType) {
				panic(&JossError{
					Type:    "PropertyTypeError",
					Message: fmt.Sprintf("La propiedad '%s' requiere %s", name, declaredType),
					File:    r.CurrentFile,
					Line:    line,
				})
			}
		}
	}
	instance.Fields[name] = value
	return value
}

func (r *Runtime) lookupInstanceField(instance *Instance, name string) *parser.LetStatement {
	declaration, _ := r.lookupInstanceFieldOwner(instance, name)
	return declaration
}

func (r *Runtime) lookupInstanceFieldOwner(instance *Instance, name string) (*parser.LetStatement, string) {
	if instance == nil || instance.Class == nil || instance.Class.Name == nil {
		return nil, ""
	}
	meta := r.lookupClassMetadata(instance.Class.Name.Value)
	if meta != nil {
		if field, ok := meta.Fields[name]; ok {
			return field.Declaration, field.OwnerClass
		}
	}
	return nil, ""
}
