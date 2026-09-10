package core

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

func (r *Runtime) evaluateExpression(exp parser.Expression) interface{} {
	switch e := exp.(type) {
	case *parser.StringLiteral:
		return e.Value
	case *parser.IntegerLiteral:
		return e.Value
	case *parser.FloatLiteral:
		return e.Value
	case *parser.DecimalLiteral:
		return e.Value
	case *parser.Boolean:
		return e.Value
	case *parser.NullLiteral:
		return nil
	case *parser.CallExpression:
		return r.executeCall(e)
	case *parser.Identifier:
		if e.Value == "null" || e.Value == "nil" {
			return nil
		}
		if e.Value == "true" {
			return true
		}
		if e.Value == "false" {
			return false
		}
		if value, resolved, initialized := r.localValue(e); resolved {
			if !initialized {
				panic(&JossError{Type: "UndefinedVariable", Message: fmt.Sprintf("Variable '%s' usada antes de inicializar", e.Value), File: r.CurrentFile, Line: e.Token.Line, Column: e.Token.Column})
			}
			return value
		}
		if val, ok := r.Variables[e.Value]; ok && r.sourceMapVisible(e.Value) {
			if reference, ok := val.(*VariableReference); ok {
				return reference.Get()
			}
			return val
		}
		if classStmt, ok := r.Classes[e.Value]; ok {
			return classStmt
		}
		if r.isNativeClass(e.Value) || IsNativeClass(e.Value) {
			return e.Value
		}
		panic(&JossError{
			Type:    "UndefinedVariable",
			Message: fmt.Sprintf("Variable '%s' no definida", e.Value),
			File:    r.CurrentFile,
			Line:    e.Token.Line,
		})
	case *parser.TernaryExpression:
		return r.evaluateTernary(e)
	case *parser.InfixExpression:
		return r.evaluateInfix(e)
	case *parser.ArrayLiteral:
		return r.evaluateArray(e)
	case *parser.MapLiteral:
		return r.evaluateMap(e)
	case *parser.IndexExpression:
		return r.evaluateIndex(e)
	case *parser.NewExpression:
		return r.evaluateNew(e)
	case *parser.MemberExpression:
		return r.evaluateMember(e)
	case *parser.AssignExpression:
		return r.evaluateAssign(e)
	case *parser.IssetExpression:
		return r.evaluateIsset(e)
	case *parser.EmptyExpression:
		return r.evaluateEmpty(e)
	case *parser.BlockExpression:
		// Return the block itself (or a closure wrapper if we had one)
		// For now, just return the BlockStatement so Task can execute it.
		return e.Block
	case *parser.FunctionLiteral:
		return r.captureFunction(e)
	case *parser.PrefixExpression:
		return r.evaluatePrefix(e)
	case *parser.PostfixExpression:
		return r.evaluatePostfix(e)
	case *parser.MatchExpression:
		return r.evaluateMatch(e)
	case *parser.ReferenceExpression:
		panic(&JossError{Type: "InvalidReference", Message: "`ref` solo puede usarse como argumento de una llamada", File: r.CurrentFile, Line: e.Token.Line})
	case *parser.IsExpression:
		return r.evaluateIs(e)
	case *parser.SpreadExpression:
		return r.evaluateExpression(e.Expression)
	case *parser.YieldExpression:
		return r.evaluateYield(e)
	}
	return nil
}

func (r *Runtime) evaluateIs(ie *parser.IsExpression) interface{} {
	val := r.evaluateExpression(ie.Left)
	target := ie.TargetType.Literal
	parsed := typesystem.Parse(target)
	if parsed.Kind == typesystem.Class {
		if ev, ok := val.(*EnumValue); ok {
			return ev.EnumName == parsed.Name
		}
	}
	return r.checkParsedType(val, parsed)
}
