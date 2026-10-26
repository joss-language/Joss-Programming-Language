package core

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	runtimeframe "github.com/jossecurity/joss/pkg/runtime/frame"
	"github.com/jossecurity/joss/pkg/typesystem"
	"github.com/shopspring/decimal"
)

func (r *Runtime) evaluateNumericInfix(expression *parser.InfixExpression, left, right interface{}) (interface{}, bool) {
	leftInteger, leftIsInteger := runtimeInteger(left)
	rightInteger, rightIsInteger := runtimeInteger(right)
	if leftIsInteger && rightIsInteger {
		switch expression.Operator {
		case "+", "-", "*", "%":
			result, fault := typesystem.CheckedIntBinary(expression.Operator, leftInteger, rightInteger)
			if fault != typesystem.ArithmeticOK {
				panic(r.integerArithmeticError(expression, fault, leftInteger, rightInteger))
			}
			return result, true
		case "/":
			if rightInteger == 0 {
				panic(r.integerArithmeticError(expression, typesystem.ArithmeticDivisionByZero, leftInteger, rightInteger))
			}
			if !typesystem.IntExactlyRepresentableAsFloat64(leftInteger) || !typesystem.IntExactlyRepresentableAsFloat64(rightInteger) {
				panic(r.precisionLossError(expression, "integer division requires an inexact float conversion"))
			}
			return float64(leftInteger) / float64(rightInteger), true
		case "<":
			return leftInteger < rightInteger, true
		case ">":
			return leftInteger > rightInteger, true
		case ">=":
			return leftInteger >= rightInteger, true
		case "<=":
			return leftInteger <= rightInteger, true
		case "==":
			return leftInteger == rightInteger, true
		case "!=":
			return leftInteger != rightInteger, true
		}
	}

	_, leftIsDecimal := left.(decimal.Decimal)
	_, rightIsDecimal := right.(decimal.Decimal)
	if leftIsDecimal || rightIsDecimal {
		if _, isFloat := left.(float64); isFloat {
			panic(r.precisionLossError(expression, "float cannot be promoted implicitly to decimal"))
		}
		if _, isFloat := right.(float64); isFloat {
			panic(r.precisionLossError(expression, "float cannot be promoted implicitly to decimal"))
		}
		leftDecimal, leftOK := runtimeDecimal(left)
		rightDecimal, rightOK := runtimeDecimal(right)
		if leftOK && rightOK {
			switch expression.Operator {
			case "+":
				return leftDecimal.Add(rightDecimal), true
			case "-":
				return leftDecimal.Sub(rightDecimal), true
			case "*":
				return leftDecimal.Mul(rightDecimal), true
			case "/":
				if rightDecimal.IsZero() {
					panic(r.divisionByZeroError(expression))
				}
				return leftDecimal.Div(rightDecimal), true
			case "%":
				if rightDecimal.IsZero() {
					panic(r.divisionByZeroError(expression))
				}
				return leftDecimal.Mod(rightDecimal), true
			case "<":
				return leftDecimal.LessThan(rightDecimal), true
			case ">":
				return leftDecimal.GreaterThan(rightDecimal), true
			case "<=":
				return leftDecimal.LessThanOrEqual(rightDecimal), true
			case ">=":
				return leftDecimal.GreaterThanOrEqual(rightDecimal), true
			case "==":
				return leftDecimal.Equal(rightDecimal), true
			case "!=":
				return !leftDecimal.Equal(rightDecimal), true
			case "&&":
				return !leftDecimal.IsZero() && !rightDecimal.IsZero(), true
			case "||":
				return !leftDecimal.IsZero() || !rightDecimal.IsZero(), true
			}
		}
	}

	leftFloat, leftIsNumber := runtimeFloat(left)
	rightFloat, rightIsNumber := runtimeFloat(right)
	if !leftIsNumber || !rightIsNumber {
		return nil, false
	}
	if expression.Operator == "/" {
		if rightFloat == 0 {
			panic(r.divisionByZeroError(expression))
		}
		return leftFloat / rightFloat, true
	}
	if expression.Operator == "%" {
		leftModulo, rightModulo := int64(leftFloat), int64(rightFloat)
		result, fault := typesystem.CheckedIntBinary("%", leftModulo, rightModulo)
		if fault != typesystem.ArithmeticOK {
			panic(r.integerArithmeticError(expression, fault, leftModulo, rightModulo))
		}
		return result, true
	}

	_, leftWasFloat := left.(float64)
	_, rightWasFloat := right.(float64)
	if leftWasFloat || rightWasFloat {
		if integer, ok := runtimeInteger(left); ok && !typesystem.IntExactlyRepresentableAsFloat64(integer) {
			panic(r.precisionLossError(expression, "integer cannot be promoted exactly to float"))
		}
		if integer, ok := runtimeInteger(right); ok && !typesystem.IntExactlyRepresentableAsFloat64(integer) {
			panic(r.precisionLossError(expression, "integer cannot be promoted exactly to float"))
		}
		switch expression.Operator {
		case "+":
			return leftFloat + rightFloat, true
		case "-":
			return leftFloat - rightFloat, true
		case "*":
			return leftFloat * rightFloat, true
		case "<":
			return leftFloat < rightFloat, true
		case ">":
			return leftFloat > rightFloat, true
		case ">=":
			return leftFloat >= rightFloat, true
		case "<=":
			return leftFloat <= rightFloat, true
		case "==":
			return leftFloat == rightFloat, true
		case "!=":
			return leftFloat != rightFloat, true
		case "&&":
			return leftFloat != 0 && rightFloat != 0, true
		case "||":
			return leftFloat != 0 || rightFloat != 0, true
		}
	}

	leftInteger = int64(leftFloat)
	rightInteger = int64(rightFloat)
	switch expression.Operator {
	case "+":
		return leftInteger + rightInteger, true
	case "-":
		return leftInteger - rightInteger, true
	case "*":
		return leftInteger * rightInteger, true
	case "<":
		return leftInteger < rightInteger, true
	case ">":
		return leftInteger > rightInteger, true
	case ">=":
		return leftInteger >= rightInteger, true
	case "<=":
		return leftInteger <= rightInteger, true
	case "==":
		return leftInteger == rightInteger, true
	case "!=":
		return leftInteger != rightInteger, true
	case "&&":
		return leftInteger != 0 && rightInteger != 0, true
	case "||":
		return leftInteger != 0 || rightInteger != 0, true
	default:
		return nil, false
	}
}

func runtimeDecimal(value interface{}) (decimal.Decimal, bool) {
	switch number := value.(type) {
	case decimal.Decimal:
		return number, true
	case int64:
		return decimal.NewFromInt(number), true
	case int:
		return decimal.NewFromInt(int64(number)), true
	case float64:
		return decimal.NewFromFloat(number), true
	case float32:
		return decimal.NewFromFloat(float64(number)), true
	default:
		return decimal.Zero, false
	}
}

func runtimeFloat(value interface{}) (float64, bool) {
	switch number := value.(type) {
	case int64:
		return float64(number), true
	case int:
		return float64(number), true
	case float64:
		return number, true
	default:
		return 0, false
	}
}

func (r *Runtime) evaluateSlotIntegerInfix(expression *parser.InfixExpression) (interface{}, bool) {
	left, leftOK := r.slotIntegerOperand(expression.Left)
	right, rightOK := r.slotIntegerOperand(expression.Right)
	if !leftOK || !rightOK {
		return nil, false
	}
	switch expression.Operator {
	case "+", "-", "*", "%":
		result, fault := typesystem.CheckedIntBinary(expression.Operator, left, right)
		if fault != typesystem.ArithmeticOK {
			panic(r.integerArithmeticError(expression, fault, left, right))
		}
		return result, true
	case "/":
		if right == 0 {
			panic(r.integerArithmeticError(expression, typesystem.ArithmeticDivisionByZero, left, right))
		}
		if !typesystem.IntExactlyRepresentableAsFloat64(left) || !typesystem.IntExactlyRepresentableAsFloat64(right) {
			panic(r.precisionLossError(expression, "integer division requires an inexact float conversion"))
		}
		return float64(left) / float64(right), true
	case "<":
		return left < right, true
	case ">":
		return left > right, true
	case ">=":
		return left >= right, true
	case "<=":
		return left <= right, true
	case "==", "===":
		return left == right, true
	case "!=", "!==":
		return left != right, true
	default:
		return nil, false
	}
}

func (r *Runtime) slotIntegerOperand(expression parser.Expression) (int64, bool) {
	switch node := expression.(type) {
	case *parser.IntegerLiteral:
		return node.Value, true
	case *parser.Identifier:
		if r.currentFrame != nil {
			slot, resolved := r.slotForIdentifier(node)
			if !resolved || !slot.Initialized || slot.Value.Kind != runtimeframe.Int {
				return 0, false
			}
			return slot.Value.Integer, true
		}
		if val, ok := r.Variables[node.Value]; ok {
			return runtimeInteger(val)
		}
		return 0, false
	case *parser.InfixExpression:
		if res, handled := r.evaluateSlotIntegerInfix(node); handled {
			if n, ok := res.(int64); ok {
				return n, true
			}
		}
		return 0, false
	case *parser.IndexExpression:
		var list []interface{}
		if r.currentFrame != nil {
			if idLeft, ok := node.Left.(*parser.Identifier); ok {
				if leftSlot, resolved := r.slotForIdentifier(idLeft); resolved && leftSlot.Initialized {
					list, _ = leftSlot.Value.Dynamic.([]interface{})
				}
			}
		}
		if list == nil {
			left := r.evaluateExpression(node.Left)
			var ok bool
			list, ok = left.([]interface{})
			if !ok {
				return 0, false
			}
		}
		var idx int64
		var idxOK bool
		if intLit, ok := node.Index.(*parser.IntegerLiteral); ok {
			idx = intLit.Value
			idxOK = true
		} else {
			idx, idxOK = r.slotIntegerOperand(node.Index)
			if !idxOK {
				rawIdx := r.evaluateExpression(node.Index)
				idx, idxOK = runtimeInteger(rawIdx)
			}
		}
		if idxOK && idx >= 0 && idx < int64(len(list)) {
			if n, ok := list[idx].(int64); ok {
				return n, true
			}
		}
		return 0, false
	default:
		return 0, false
	}
}

func (r *Runtime) evaluateSlotFloatInfix(expression *parser.InfixExpression) (interface{}, bool) {
	left, leftIsFloat, leftOK := r.slotFloatOperand(expression.Left)
	right, rightIsFloat, rightOK := r.slotFloatOperand(expression.Right)
	if !leftOK || !rightOK || (!leftIsFloat && !rightIsFloat) {
		return nil, false
	}
	switch expression.Operator {
	case "+":
		return left + right, true
	case "-":
		return left - right, true
	case "*":
		return left * right, true
	case "/":
		if right == 0 {
			panic(r.divisionByZeroError(expression))
		}
		return left / right, true
	case "<":
		return left < right, true
	case ">":
		return left > right, true
	case ">=":
		return left >= right, true
	case "<=":
		return left <= right, true
	case "==", "===":
		return left == right && leftIsFloat == rightIsFloat, true
	case "!=", "!==":
		return left != right || leftIsFloat != rightIsFloat, true
	default:
		return nil, false
	}
}

func (r *Runtime) slotFloatOperand(expression parser.Expression) (float64, bool, bool) {
	switch node := expression.(type) {
	case *parser.FloatLiteral:
		return node.Value, true, true
	case *parser.IntegerLiteral:
		if !typesystem.IntExactlyRepresentableAsFloat64(node.Value) {
			return 0, false, false
		}
		return float64(node.Value), false, true
	case *parser.Identifier:
		if r.currentFrame != nil {
			slot, resolved := r.slotForIdentifier(node)
			if !resolved || !slot.Initialized {
				return 0, false, false
			}
			if slot.Value.Kind == runtimeframe.Float {
				return slot.Value.Float, true, true
			}
			if slot.Value.Kind == runtimeframe.Int {
				if !typesystem.IntExactlyRepresentableAsFloat64(slot.Value.Integer) {
					return 0, false, false
				}
				return float64(slot.Value.Integer), false, true
			}
			return 0, false, false
		}
		if val, ok := r.Variables[node.Value]; ok {
			if f, isF := val.(float64); isF {
				return f, true, true
			}
			if n, isI := runtimeInteger(val); isI && typesystem.IntExactlyRepresentableAsFloat64(n) {
				return float64(n), false, true
			}
		}
		return 0, false, false
	case *parser.InfixExpression:
		if res, handled := r.evaluateSlotFloatInfix(node); handled {
			if f, ok := res.(float64); ok {
				return f, true, true
			}
		}
		if res, handled := r.evaluateSlotIntegerInfix(node); handled {
			if f, ok := res.(float64); ok {
				return f, true, true
			}
			if n, ok := res.(int64); ok && typesystem.IntExactlyRepresentableAsFloat64(n) {
				return float64(n), false, true
			}
		}
		return 0, false, false
	default:
		return 0, false, false
	}
}

func runtimeInteger(value interface{}) (int64, bool) {
	switch number := value.(type) {
	case int64:
		return number, true
	case int:
		return int64(number), true
	default:
		return 0, false
	}
}

func (r *Runtime) integerArithmeticError(expression *parser.InfixExpression, fault typesystem.ArithmeticFault, left, right int64) *JossError {
	code := diagnostics.CodeArithmeticOverflow
	message := fmt.Sprintf("Overflow entero en %d %s %d", left, expression.Operator, right)
	if fault == typesystem.ArithmeticDivisionByZero {
		code = diagnostics.CodeDivisionByZero
		message = fmt.Sprintf("División entre cero en %d %s %d", left, expression.Operator, right)
	}
	return &JossError{Code: code, Type: "ArithmeticError", Message: message, File: r.CurrentFile, Line: expression.Token.Line, Column: expression.Token.Column}
}

func (r *Runtime) divisionByZeroError(expression *parser.InfixExpression) *JossError {
	return &JossError{Code: diagnostics.CodeDivisionByZero, Type: "ArithmeticError", Message: "División entre cero", File: r.CurrentFile, Line: expression.Token.Line, Column: expression.Token.Column}
}

func (r *Runtime) precisionLossError(expression *parser.InfixExpression, detail string) *JossError {
	return &JossError{Code: diagnostics.CodePrecisionLoss, Type: "ArithmeticError", Message: "Conversión numérica insegura: " + detail, File: r.CurrentFile, Line: expression.Token.Line, Column: expression.Token.Column}
}
