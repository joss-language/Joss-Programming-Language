package core

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/parser"
)

// evaluateInfix owns evaluation order and delegates domain semantics. Keeping
// short-circuit, input and output ordering here makes the observable behavior
// explicit without mixing every operator implementation into one function.
func (r *Runtime) evaluateInfix(expression *parser.InfixExpression) interface{} {
	switch expression.Operator {
	case "??":
		return r.evaluateNullCoalescing(expression)
	case "|>":
		return r.evaluatePipeline(expression)
	}

	if result, handled := r.evaluateSlotIntegerInfix(expression); handled {
		return result
	}

	left := r.evaluateExpression(expression.Left)
	if expression.Operator == "&&" {
		if !isTruthy(left) {
			return false
		}
		return isTruthy(r.evaluateExpression(expression.Right))
	}
	if expression.Operator == "||" {
		if isTruthy(left) {
			return true
		}
		return isTruthy(r.evaluateExpression(expression.Right))
	}
	if result, handled := r.evaluateInputInfix(left, expression); handled {
		return result
	}

	right := r.evaluateExpression(expression.Right)
	switch expression.Operator {
	case "===":
		return strictCompare(left, right)
	case "!==":
		return !strictCompare(left, right)
	case "<=>":
		return spaceshipCompare(left, right)
	}
	if result, handled := r.evaluateOutputInfix(left, right, expression.Operator); handled {
		return result
	}
	if result, handled := r.evaluateNumericInfix(expression, left, right); handled {
		return result
	}
	if expression.Operator == ".." {
		return evaluateRange(left, right)
	}

	leftString := ""
	rightString := ""
	if left != nil {
		leftString = fmt.Sprintf("%v", left)
	}
	if right != nil {
		rightString = fmt.Sprintf("%v", right)
	}
	switch expression.Operator {
	case ".":
		return leftString + rightString
	case "+":
		fmt.Println("Error: El operador '+' es solo para números. Use '.' para concatenar cadenas.")
		return nil
	case "==":
		return leftString == rightString
	case "!=":
		return leftString != rightString
	}

	if leftBoolean, ok := left.(bool); ok {
		if rightBoolean, ok := right.(bool); ok {
			switch expression.Operator {
			case "&&":
				return leftBoolean && rightBoolean
			case "||":
				return leftBoolean || rightBoolean
			}
		}
	}
	return nil
}

func evaluateRange(left, right interface{}) []interface{} {
	leftInteger, leftOK := toInt64Safe(left)
	rightInteger, rightOK := toInt64Safe(right)
	if !leftOK || !rightOK {
		return []interface{}{}
	}

	if leftInteger <= rightInteger {
		count := rightInteger - leftInteger + 1
		if count > 1000000 {
			count = 1000000
		}
		result := make([]interface{}, 0, int(count))
		for value := leftInteger; value <= rightInteger; value++ {
			result = append(result, value)
		}
		return result
	}
	count := leftInteger - rightInteger + 1
	if count > 1000000 {
		count = 1000000
	}
	result := make([]interface{}, 0, int(count))
	for value := leftInteger; value >= rightInteger; value-- {
		result = append(result, value)
	}
	return result
}
