package core

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	runtimeframe "github.com/jossecurity/joss/pkg/runtime/frame"
	"github.com/jossecurity/joss/pkg/typesystem"
	"github.com/shopspring/decimal"
)

func (r *Runtime) evaluatePrefix(expression *parser.PrefixExpression) interface{} {
	right := r.evaluateExpression(expression.Right)
	if expression.Operator == "!" {
		return !isTruthy(right)
	}
	if expression.Operator != "-" {
		return nil
	}
	switch value := right.(type) {
	case int64:
		result, fault := typesystem.CheckedIntNegate(value)
		if fault != typesystem.ArithmeticOK {
			panic(&JossError{Code: diagnostics.CodeArithmeticOverflow, Type: "ArithmeticError", Message: fmt.Sprintf("Overflow entero al negar %d", value), File: r.CurrentFile, Line: expression.Token.Line, Column: expression.Token.Column})
		}
		return result
	case float64:
		return -value
	case decimal.Decimal:
		return value.Neg()
	default:
		return nil
	}
}

func (r *Runtime) evaluatePostfix(expression *parser.PostfixExpression) interface{} {
	if expression.Operator != "++" && expression.Operator != "--" {
		return nil
	}
	if value, handled := r.updatePostfixSlot(expression, true); handled {
		return value
	}
	oldValue := r.evaluateExpression(expression.Left)

	var newValue interface{}
	operator := "+"
	if expression.Operator == "--" {
		operator = "-"
	}
	switch value := oldValue.(type) {
	case int64:
		result, fault := typesystem.CheckedIntBinary(operator, value, 1)
		if fault != typesystem.ArithmeticOK {
			panic(&JossError{Code: diagnostics.CodeArithmeticOverflow, Type: "ArithmeticError", Message: fmt.Sprintf("Overflow entero al operar %d", value), File: r.CurrentFile, Line: expression.Token.Line, Column: expression.Token.Column})
		}
		newValue = result
	case float64:
		if expression.Operator == "--" {
			newValue = value - 1
		} else {
			newValue = value + 1
		}
	case decimal.Decimal:
		if expression.Operator == "--" {
			newValue = value.Sub(decimal.NewFromInt(1))
		} else {
			newValue = value.Add(decimal.NewFromInt(1))
		}
	default:
		fmt.Printf("Error: Operador %s solo aplicable a números\n", expression.Operator)
		return nil
	}

	r.updateVariable(expression.Left, newValue)
	return oldValue
}

func (r *Runtime) executePostfixStatement(expression *parser.PostfixExpression) bool {
	_, handled := r.updatePostfixSlot(expression, false)
	return handled
}

func (r *Runtime) updatePostfixSlot(expression *parser.PostfixExpression, returnOld bool) (interface{}, bool) {
	identifier, ok := expression.Left.(*parser.Identifier)
	if !ok || (expression.Operator != "++" && expression.Operator != "--") {
		return nil, false
	}
	slot, resolved := r.slotForIdentifier(identifier)
	if !resolved || !slot.Initialized || slot.ByReference {
		return nil, false
	}
	if slot.Constant {
		panic(&JossError{Type: "ConstantAssignment", Message: fmt.Sprintf("La constante '%s' no puede modificarse", slot.Name), File: r.CurrentFile, Line: expression.Token.Line, Column: expression.Token.Column})
	}
	operator := "+"
	if expression.Operator == "--" {
		operator = "-"
	}
	switch slot.Value.Kind {
	case runtimeframe.Int:
		oldValue := slot.Value.Integer
		updated, fault := typesystem.CheckedIntBinary(operator, oldValue, 1)
		if fault != typesystem.ArithmeticOK {
			panic(&JossError{Code: diagnostics.CodeArithmeticOverflow, Type: "ArithmeticError", Message: fmt.Sprintf("Overflow entero al operar %d", oldValue), File: r.CurrentFile, Line: expression.Token.Line, Column: expression.Token.Column})
		}
		slot.Value.Integer = updated
		if returnOld {
			return oldValue, true
		}
		return nil, true
	case runtimeframe.Float:
		oldValue := slot.Value.Float
		if expression.Operator == "--" {
			slot.Value.Float = oldValue - 1
		} else {
			slot.Value.Float = oldValue + 1
		}
		if returnOld {
			return oldValue, true
		}
		return nil, true
	default:
		return nil, false
	}
}
