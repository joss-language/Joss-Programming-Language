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
		slot, resolved := r.slotForIdentifier(node)
		if !resolved || !slot.Initialized || slot.Value.Kind != runtimeframe.Int {
			return 0, false
		}
		return slot.Value.Integer, true
	default:
		return 0, false
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
