package analyzer

import "github.com/jossecurity/joss/pkg/parser"

// expressionTerminatesCallable is a side-effect-free control-flow query. It
// complements semantic inference without emitting duplicate diagnostics.
func expressionTerminatesCallable(expression parser.Expression) bool {
	switch node := expression.(type) {
	case *parser.BlockExpression:
		return blockTerminatesCallable(node.Block)
	case *parser.TernaryExpression:
		return node.True != nil && node.False != nil &&
			expressionTerminatesCallable(node.True) && expressionTerminatesCallable(node.False)
	case *parser.MatchExpression:
		if len(node.Arms) == 0 {
			return false
		}
		hasDefault := false
		for _, arm := range node.Arms {
			hasDefault = hasDefault || arm.IsDefault
			if !expressionTerminatesCallable(arm.Value) {
				return false
			}
		}
		return hasDefault
	default:
		return false
	}
}

func statementTerminatesCallable(statement parser.Statement) bool {
	switch node := statement.(type) {
	case *parser.ReturnStatement, *parser.ThrowStatement:
		return true
	case *parser.ExpressionStatement:
		return expressionTerminatesCallable(node.Expression)
	case *parser.TryCatchStatement:
		return node.TryBlock != nil && node.CatchBlock != nil &&
			blockTerminatesCallable(node.TryBlock) && blockTerminatesCallable(node.CatchBlock)
	default:
		return false
	}
}

func hasYield(node interface{}) bool {
	if node == nil {
		return false
	}
	switch n := node.(type) {
	case *parser.BlockStatement:
		for _, stmt := range n.Statements {
			if hasYield(stmt) {
				return true
			}
		}
	case *parser.ExpressionStatement:
		return hasYield(n.Expression)
	case *parser.YieldExpression:
		return true
	case *parser.WhileStatement:
		return hasYield(n.Body)
	case *parser.GuardStatement:
		return hasYield(n.Body)
	case *parser.DoWhileStatement:
		return hasYield(n.Body)
	case *parser.ForeachStatement:
		return hasYield(n.Body)
	case *parser.TryCatchStatement:
		return hasYield(n.TryBlock) || hasYield(n.CatchBlock)
	case *parser.TernaryExpression:
		return hasYield(n.True) || hasYield(n.False)
	case *parser.BlockExpression:
		return hasYield(n.Block)
	}
	return false
}

func blockTerminatesCallable(block *parser.BlockStatement) bool {
	if block == nil {
		return false
	}
	if hasYield(block) {
		return true
	}
	for _, statement := range block.Statements {
		if statementTerminatesCallable(statement) {
			return true
		}
	}
	return false
}
