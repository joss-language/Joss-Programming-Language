package core

import (
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

// evaluateNullCoalescing handles absence only. Errors raised while evaluating
// the left operand remain errors and must never be converted into null.
func (r *Runtime) evaluateNullCoalescing(expression *parser.InfixExpression) interface{} {
	left := r.evaluateExpression(expression.Left)
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
