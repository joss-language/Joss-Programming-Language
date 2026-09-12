package analyzer

import (
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

func (a *Analyzer) narrowScopeFromCondition(condition parser.Expression, current *scope) (*scope, *scope) {
	trueScope := newScope(current)
	falseScope := newScope(current)
	if isExpression, ok := condition.(*parser.IsExpression); ok {
		if identifier, ok := isExpression.Left.(*parser.Identifier); ok {
			if existing, ok := current.resolve(identifier.Value); ok {
				trueScope.put(narrowedSymbol(existing, typeFromToken(isExpression.TargetType)))
			}
		}
	}
	if infix, ok := condition.(*parser.InfixExpression); ok {
		identifier := nullComparedIdentifier(infix)
		if identifier != nil {
			if existing, ok := current.resolve(identifier.Value); ok && existing.Type.Kind == typesystem.Union {
				nonNull := existing.Type.Without(typesystem.Null)
				nullType := typesystem.Type{Kind: typesystem.Null}
				switch infix.Operator {
				case "!=", "!==":
					trueScope.put(narrowedSymbol(existing, nonNull))
					falseScope.put(narrowedSymbol(existing, nullType))
				case "==", "===":
					trueScope.put(narrowedSymbol(existing, nullType))
					falseScope.put(narrowedSymbol(existing, nonNull))
				}
			}
		}
	}
	return trueScope, falseScope
}

func narrowedSymbol(existing *symbol, narrowedType typesystem.Type) *symbol {
	return &symbol{
		Name: existing.Name, Type: narrowedType, Kind: existing.Kind, Token: existing.Token,
		File: existing.File, Dynamic: existing.Dynamic, Inferred: existing.Inferred,
		Constant: existing.Constant, Synthetic: true,
	}
}

func nullComparedIdentifier(expression *parser.InfixExpression) *parser.Identifier {
	if identifier, ok := expression.Left.(*parser.Identifier); ok && isNullLiteral(expression.Right) {
		return identifier
	}
	if identifier, ok := expression.Right.(*parser.Identifier); ok && isNullLiteral(expression.Left) {
		return identifier
	}
	return nil
}

func isNullLiteral(expression parser.Expression) bool {
	switch node := expression.(type) {
	case *parser.NullLiteral:
		return true
	case *parser.Identifier:
		return node.Value == "null" || node.Value == "nil"
	default:
		return false
	}
}
