package core

import (
	"github.com/jossecurity/joss/pkg/parser"
)

func (r *Runtime) evaluateTernary(expression *parser.TernaryExpression) interface{} {
	if expression.True == nil {
		condition := r.evaluateExpression(expression.Condition)
		if isTruthy(condition) {
			return condition
		}
		if expression.False == nil {
			return nil
		}
		return r.evaluateExpression(expression.False)
	}

	if r.evaluateCondition(expression.Condition) {
		if blockExp, ok := expression.True.(*parser.BlockExpression); ok {
			return r.executeBlock(blockExp.Block)
		}
		res := r.evaluateExpression(expression.True)
		if block, ok := res.(*parser.BlockStatement); ok {
			return r.executeBlock(block)
		}
		return res
	}

	if expression.False == nil {
		return nil
	}
	if blockExp, ok := expression.False.(*parser.BlockExpression); ok {
		return r.executeBlock(blockExp.Block)
	}
	res := r.evaluateExpression(expression.False)
	if block, ok := res.(*parser.BlockStatement); ok {
		return r.executeBlock(block)
	}
	return res
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
