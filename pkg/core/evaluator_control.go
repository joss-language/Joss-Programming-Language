package core

import (
	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
)

func (r *Runtime) evaluateTernary(expression *parser.TernaryExpression) interface{} {
	condition := r.evaluateExpression(expression.Condition)
	conditionIsTrue := isTruthy(condition)

	var result interface{}
	switch {
	case expression.True == nil:
		if conditionIsTrue {
			result = condition
		} else {
			result = r.evaluateExpression(expression.False)
		}
	case expression.False == nil:
		if !conditionIsTrue {
			return nil
		}
		result = r.evaluateExpression(expression.True)
	default:
		if conditionIsTrue {
			result = r.evaluateExpression(expression.True)
		} else {
			result = r.evaluateExpression(expression.False)
		}
	}

	if block, ok := result.(*parser.BlockStatement); ok {
		return r.executeBlock(block)
	}
	return result
}

// evaluateNullCoalescing preserves Joss's recovery contract: lookup/type errors
// on the left mean null, while control-flow and arithmetic failures propagate.
func (r *Runtime) evaluateNullCoalescing(expression *parser.InfixExpression) interface{} {
	var left interface{}
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				switch runtimeValue := recovered.(type) {
				case *ReturnPanic, *BreakPanic, *ContinuePanic:
					panic(recovered)
				case *JossError:
					if runtimeValue.Type == "ArithmeticError" || runtimeValue.Code == diagnostics.CodeDivisionByZero || runtimeValue.Code == diagnostics.CodeArithmeticOverflow {
						panic(recovered)
					}
					left = nil
				case *Instance:
					panic(recovered)
				default:
					left = nil
				}
			}
		}()
		left = r.evaluateExpression(expression.Left)
	}()
	if left != nil {
		return left
	}
	return r.evaluateExpression(expression.Right)
}

func (r *Runtime) evaluateMatch(expression *parser.MatchExpression) interface{} {
	subject := r.evaluateExpression(expression.Subject)

	var defaultArm *parser.MatchArm
	for _, arm := range expression.Arms {
		if arm.IsDefault {
			armCopy := arm
			defaultArm = &armCopy
			continue
		}

		for _, keyExpression := range arm.Keys {
			if strictCompare(subject, r.evaluateExpression(keyExpression)) {
				return r.evaluateMatchArm(arm.Value)
			}
		}
	}

	if defaultArm != nil {
		return r.evaluateMatchArm(defaultArm.Value)
	}
	return nil
}

func (r *Runtime) evaluateMatchArm(expression parser.Expression) interface{} {
	result := r.evaluateExpression(expression)
	if block, ok := result.(*parser.BlockStatement); ok {
		return r.executeBlock(block)
	}
	return result
}
