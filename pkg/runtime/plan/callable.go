// Package plan precomputes stable execution metadata from analyzed AST shapes.
// It deliberately contains no evaluator or framework behavior.
package plan

import (
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

type Slot struct {
	Name        string
	TypeName    string
	Type        typesystem.Type
	Constant    bool
	Inferred    bool
	Parameter   bool
	ByReference bool
}

type Callable struct {
	Name            string
	Owner           string
	Slots           []Slot
	NameSlots       map[string]int
	IdentifierSlots map[*parser.Identifier]int
	ForeachSlots    map[*parser.ForeachStatement]int
	ForeachKeySlots map[*parser.ForeachStatement]int
	CatchSlots      map[*parser.TryCatchStatement]int
	LoopControl     map[*parser.BlockStatement]bool
	ParameterCount  int
	RequiredCount   int
	ThisSlot        int
	ReturnTypeName  string
	ReturnType      typesystem.Type
}

func CompileMethod(method *parser.MethodStatement, hasThis bool) *Callable {
	if method == nil {
		return nil
	}
	name := "anonymous"
	if method.Name != nil {
		name = method.Name.Value
	}
	return compile(name, method.Parameters, method.ReturnType, method.Body, hasThis)
}

func CompileFunction(function *parser.FunctionLiteral) *Callable {
	if function == nil {
		return nil
	}
	return compile("anonymous", function.Parameters, function.ReturnType, function.Body, false)
}

func compile(name string, parameters []*parser.Parameter, returnToken parser.Token, body *parser.BlockStatement, hasThis bool) *Callable {
	result := &Callable{
		Name:            name,
		NameSlots:       make(map[string]int),
		IdentifierSlots: make(map[*parser.Identifier]int),
		ForeachSlots:    make(map[*parser.ForeachStatement]int),
		ForeachKeySlots: make(map[*parser.ForeachStatement]int),
		CatchSlots:      make(map[*parser.TryCatchStatement]int),
		LoopControl:     make(map[*parser.BlockStatement]bool),
		ThisSlot:        -1,
		ReturnTypeName:  returnToken.Literal,
		ReturnType:      typesystem.Parse(returnToken.Literal),
	}
	for _, parameter := range parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		typeName := parameter.Type.Literal
		result.addSlot(Slot{Name: clean(parameter.Name.Value), TypeName: typeName, Type: typesystem.Parse(typeName), Parameter: true, ByReference: parameter.ByReference})
		if parameter.DefaultValue == nil {
			result.RequiredCount++
		}
	}
	result.ParameterCount = len(result.Slots)
	if hasThis {
		result.ThisSlot = result.addSlot(Slot{Name: "this", Type: typesystem.Type{Kind: typesystem.Object}})
	}
	result.collectBlock(body)
	result.annotateBlock(body)
	result.analyzeLoopControl(body)
	for index, parameter := range parameters {
		if parameter != nil && parameter.Name != nil && index < result.ParameterCount {
			result.IdentifierSlots[parameter.Name] = index
		}
	}
	return result
}

func clean(name string) string { return strings.TrimPrefix(name, "$") }

func (callable *Callable) addSlot(slot Slot) int {
	slot.Name = clean(slot.Name)
	if existing, ok := callable.NameSlots[slot.Name]; ok {
		return existing
	}
	index := len(callable.Slots)
	callable.NameSlots[slot.Name] = index
	callable.Slots = append(callable.Slots, slot)
	return index
}

func (callable *Callable) collectBlock(block *parser.BlockStatement) {
	if block == nil {
		return
	}
	for _, statement := range block.Statements {
		callable.collectStatement(statement)
	}
}

func (callable *Callable) collectStatement(statement parser.Statement) {
	switch node := statement.(type) {
	case *parser.LetStatement:
		if node.Name != nil {
			typeName := node.Token.Literal
			callable.addSlot(Slot{Name: node.Name.Value, TypeName: typeName, Type: typesystem.Parse(typeName), Constant: node.IsConst, Inferred: typeName == "var"})
		}
		callable.collectExpression(node.Value)
	case *parser.MultiLetStatement:
		for _, declaration := range node.Declarations {
			if declaration.Name != nil {
				typeName := node.TypeToken.Literal
				callable.addSlot(Slot{Name: declaration.Name.Value, TypeName: typeName, Type: typesystem.Parse(typeName), Inferred: typeName == "var"})
			}
			callable.collectExpression(declaration.Value)
		}
	case *parser.ExpressionStatement:
		callable.collectExpression(node.Expression)
	case *parser.EchoStatement:
		callable.collectExpression(node.Value)
	case *parser.ReturnStatement:
		callable.collectExpression(node.ReturnValue)
	case *parser.ThrowStatement:
		callable.collectExpression(node.Value)
	case *parser.WhileStatement:
		callable.collectExpression(node.Condition)
		callable.collectBlock(node.Body)
	case *parser.GuardStatement:
		callable.collectExpression(node.Condition)
		callable.collectBlock(node.Body)
	case *parser.DoWhileStatement:
		callable.collectBlock(node.Body)
		callable.collectExpression(node.Condition)
	case *parser.ForeachStatement:
		callable.collectExpression(node.Iterable)
		if node.Key != "" {
			callable.ForeachKeySlots[node] = callable.addSlot(Slot{Name: node.Key, Inferred: true})
		}
		callable.ForeachSlots[node] = callable.addSlot(Slot{Name: node.Value, Inferred: true})
		callable.collectBlock(node.Body)
	case *parser.BlockStatement:
		callable.collectBlock(node)
	case *parser.DeferStatement:
		if node.Body != nil {
			callable.collectStatement(node.Body)
		}
	case *parser.TryCatchStatement:
		callable.collectBlock(node.TryBlock)
		if node.CatchVar != "" {
			callable.CatchSlots[node] = callable.addSlot(Slot{Name: node.CatchVar, TypeName: "object", Type: typesystem.Type{Kind: typesystem.Object}})
		}
		callable.collectBlock(node.CatchBlock)
	}
}

func (callable *Callable) collectExpression(expression parser.Expression) {
	switch node := expression.(type) {
	case *parser.AssignExpression:
		if identifier, ok := node.Left.(*parser.Identifier); ok {
			callable.addSlot(Slot{Name: identifier.Value, Inferred: true})
		} else if arrLit, ok := node.Left.(*parser.ArrayLiteral); ok {
			for _, elem := range arrLit.Elements {
				if id, ok := elem.(*parser.Identifier); ok {
					callable.addSlot(Slot{Name: id.Value, Inferred: true})
				} else if assign, ok := elem.(*parser.AssignExpression); ok {
					if id, ok := assign.Left.(*parser.Identifier); ok {
						callable.addSlot(Slot{Name: id.Value, Inferred: true})
					}
					callable.collectExpression(assign.Value)
				} else {
					callable.collectExpression(elem)
				}
			}
		} else if mapLit, ok := node.Left.(*parser.MapLiteral); ok {
			for _, valExpr := range mapLit.Pairs {
				if id, ok := valExpr.(*parser.Identifier); ok {
					callable.addSlot(Slot{Name: id.Value, Inferred: true})
				} else if assign, ok := valExpr.(*parser.AssignExpression); ok {
					if id, ok := assign.Left.(*parser.Identifier); ok {
						callable.addSlot(Slot{Name: id.Value, Inferred: true})
					}
					callable.collectExpression(assign.Value)
				} else {
					callable.collectExpression(valExpr)
				}
			}
		} else {
			callable.collectExpression(node.Left)
		}
		callable.collectExpression(node.Value)
	case *parser.InfixExpression:
		callable.collectExpression(node.Left)
		callable.collectExpression(node.Right)
	case *parser.PrefixExpression:
		callable.collectExpression(node.Right)
	case *parser.PostfixExpression:
		callable.collectExpression(node.Left)
	case *parser.TernaryExpression:
		callable.collectExpression(node.Condition)
		callable.collectExpression(node.True)
		callable.collectExpression(node.False)
	case *parser.ArrayLiteral:
		for _, element := range node.Elements {
			callable.collectExpression(element)
		}
	case *parser.MapLiteral:
		for key, value := range node.Pairs {
			callable.collectExpression(key)
			callable.collectExpression(value)
		}
	case *parser.IndexExpression:
		callable.collectExpression(node.Left)
		callable.collectExpression(node.Index)
	case *parser.NewExpression:
		for _, argument := range node.Arguments {
			callable.collectExpression(argument)
		}
	case *parser.MemberExpression:
		callable.collectExpression(node.Left)
	case *parser.CallExpression:
		callable.collectExpression(node.Function)
		for _, argument := range node.Arguments {
			callable.collectExpression(argument)
		}
	case *parser.ReferenceExpression:
		callable.collectExpression(node.Target)
	case *parser.IssetExpression:
		for _, argument := range node.Arguments {
			callable.collectExpression(argument)
		}
	case *parser.EmptyExpression:
		callable.collectExpression(node.Argument)
	case *parser.BlockExpression:
		callable.collectBlock(node.Block)
	case *parser.MatchExpression:
		callable.collectExpression(node.Subject)
		for _, arm := range node.Arms {
			for _, key := range arm.Keys {
				callable.collectExpression(key)
			}
			callable.collectExpression(arm.Value)
		}
	case *parser.IsExpression:
		callable.collectExpression(node.Left)
	case *parser.SpreadExpression:
		callable.collectExpression(node.Expression)
	case *parser.NamedArgument:
		callable.collectExpression(node.Value)
	case *parser.YieldExpression:
		if node.Key != nil {
			callable.collectExpression(node.Key)
		}
		if node.Value != nil {
			callable.collectExpression(node.Value)
		}
	case *parser.FunctionLiteral:
		// A nested closure owns an independent callable plan.
	}
}

func (callable *Callable) annotateBlock(block *parser.BlockStatement) {
	if block == nil {
		return
	}
	for _, statement := range block.Statements {
		callable.annotateStatement(statement)
	}
}

func (callable *Callable) annotateStatement(statement parser.Statement) {
	switch node := statement.(type) {
	case *parser.LetStatement:
		callable.annotateIdentifier(node.Name)
		callable.annotateExpression(node.Value)
	case *parser.MultiLetStatement:
		for _, declaration := range node.Declarations {
			callable.annotateIdentifier(declaration.Name)
			callable.annotateExpression(declaration.Value)
		}
	case *parser.ExpressionStatement:
		callable.annotateExpression(node.Expression)
	case *parser.EchoStatement:
		callable.annotateExpression(node.Value)
	case *parser.ReturnStatement:
		callable.annotateExpression(node.ReturnValue)
	case *parser.ThrowStatement:
		callable.annotateExpression(node.Value)
	case *parser.WhileStatement:
		callable.annotateExpression(node.Condition)
		callable.annotateBlock(node.Body)
	case *parser.GuardStatement:
		callable.annotateExpression(node.Condition)
		callable.annotateBlock(node.Body)
	case *parser.DoWhileStatement:
		callable.annotateBlock(node.Body)
		callable.annotateExpression(node.Condition)
	case *parser.ForeachStatement:
		callable.annotateExpression(node.Iterable)
		callable.annotateBlock(node.Body)
	case *parser.TryCatchStatement:
		callable.annotateBlock(node.TryBlock)
		callable.annotateBlock(node.CatchBlock)
	case *parser.BlockStatement:
		callable.annotateBlock(node)
	case *parser.DeferStatement:
		if node.Body != nil {
			callable.annotateStatement(node.Body)
		}
	}
}

func (callable *Callable) annotateIdentifier(identifier *parser.Identifier) {
	if identifier == nil {
		return
	}
	if slot, exists := callable.NameSlots[clean(identifier.Value)]; exists {
		callable.IdentifierSlots[identifier] = slot
	}
}

func (callable *Callable) annotateExpression(expression parser.Expression) {
	switch node := expression.(type) {
	case *parser.Identifier:
		callable.annotateIdentifier(node)
	case *parser.AssignExpression:
		callable.annotateExpression(node.Left)
		callable.annotateExpression(node.Value)
	case *parser.InfixExpression:
		callable.annotateExpression(node.Left)
		callable.annotateExpression(node.Right)
	case *parser.PrefixExpression:
		callable.annotateExpression(node.Right)
	case *parser.PostfixExpression:
		callable.annotateExpression(node.Left)
	case *parser.TernaryExpression:
		callable.annotateExpression(node.Condition)
		callable.annotateExpression(node.True)
		callable.annotateExpression(node.False)
	case *parser.ArrayLiteral:
		for _, element := range node.Elements {
			callable.annotateExpression(element)
		}
	case *parser.MapLiteral:
		for key, value := range node.Pairs {
			callable.annotateExpression(key)
			callable.annotateExpression(value)
		}
	case *parser.IndexExpression:
		callable.annotateExpression(node.Left)
		callable.annotateExpression(node.Index)
	case *parser.NewExpression:
		for _, argument := range node.Arguments {
			callable.annotateExpression(argument)
		}
	case *parser.MemberExpression:
		callable.annotateExpression(node.Left)
	case *parser.CallExpression:
		callable.annotateExpression(node.Function)
		for _, argument := range node.Arguments {
			callable.annotateExpression(argument)
		}
	case *parser.ReferenceExpression:
		callable.annotateExpression(node.Target)
	case *parser.IssetExpression:
		for _, argument := range node.Arguments {
			callable.annotateExpression(argument)
		}
	case *parser.EmptyExpression:
		callable.annotateExpression(node.Argument)
	case *parser.BlockExpression:
		callable.annotateBlock(node.Block)
	case *parser.MatchExpression:
		callable.annotateExpression(node.Subject)
		for _, arm := range node.Arms {
			for _, key := range arm.Keys {
				callable.annotateExpression(key)
			}
			callable.annotateExpression(arm.Value)
		}
	case *parser.IsExpression:
		callable.annotateExpression(node.Left)
	case *parser.SpreadExpression:
		callable.annotateExpression(node.Expression)
	case *parser.NamedArgument:
		callable.annotateExpression(node.Value)
	case *parser.YieldExpression:
		if node.Key != nil {
			callable.annotateExpression(node.Key)
		}
		if node.Value != nil {
			callable.annotateExpression(node.Value)
		}
	}
}

func (callable *Callable) analyzeLoopControl(block *parser.BlockStatement) {
	if block == nil {
		return
	}
	for _, statement := range block.Statements {
		switch node := statement.(type) {
		case *parser.WhileStatement:
			callable.LoopControl[node.Body] = BlockHasBreakOrContinue(node.Body)
			callable.analyzeLoopControl(node.Body)
		case *parser.DoWhileStatement:
			callable.LoopControl[node.Body] = BlockHasBreakOrContinue(node.Body)
			callable.analyzeLoopControl(node.Body)
		case *parser.ForeachStatement:
			callable.LoopControl[node.Body] = BlockHasBreakOrContinue(node.Body)
			callable.analyzeLoopControl(node.Body)
		case *parser.GuardStatement:
			callable.analyzeLoopControl(node.Body)
		case *parser.TryCatchStatement:
			callable.analyzeLoopControl(node.TryBlock)
			callable.analyzeLoopControl(node.CatchBlock)
		}
	}
}

// BlockHasBreakOrContinue reports whether the given block immediately contains
// break or continue statements that target the surrounding loop.
func BlockHasBreakOrContinue(block *parser.BlockStatement) bool {
	if block == nil {
		return false
	}
	for _, statement := range block.Statements {
		if statementHasBreakOrContinue(statement) {
			return true
		}
	}
	return false
}

func statementHasBreakOrContinue(statement parser.Statement) bool {
	switch node := statement.(type) {
	case *parser.BreakStatement, *parser.ContinueStatement:
		return true
	case *parser.TryCatchStatement:
		return BlockHasBreakOrContinue(node.TryBlock) || BlockHasBreakOrContinue(node.CatchBlock)
	case *parser.ExpressionStatement:
		if blockExp, ok := node.Expression.(*parser.BlockExpression); ok {
			return BlockHasBreakOrContinue(blockExp.Block)
		}
	}
	return false
}
