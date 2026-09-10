package core

import (
	"github.com/jossecurity/joss/pkg/parser"
)

type GeneratorItem struct {
	Key   interface{}
	Value interface{}
}

type Generator struct {
	items chan GeneratorItem
	done  chan struct{}
	curr  GeneratorItem
	valid bool
}

func newGenerator() *Generator {
	return &Generator{
		items: make(chan GeneratorItem),
		done:  make(chan struct{}),
	}
}

func (g *Generator) Next() {
	item, ok := <-g.items
	if !ok {
		g.valid = false
		g.curr = GeneratorItem{}
		return
	}
	g.curr = item
	g.valid = true
}

func (g *Generator) Current() interface{} {
	return g.curr.Value
}

func (g *Generator) Key() interface{} {
	return g.curr.Key
}

func (g *Generator) Valid() bool {
	return g.valid
}

func (g *Generator) Close() {
	select {
	case <-g.done:
	default:
		close(g.done)
	}
}

func containsYield(node parser.Node) bool {
	if node == nil {
		return false
	}
	found := false
	var inspect func(n parser.Node)
	inspect = func(n parser.Node) {
		if n == nil || found {
			return
		}
		if _, ok := n.(*parser.YieldExpression); ok {
			found = true
			return
		}
		switch x := n.(type) {
		case *parser.BlockStatement:
			for _, s := range x.Statements {
				inspect(s)
			}
		case *parser.ExpressionStatement:
			inspect(x.Expression)
		case *parser.AssignExpression:
			inspect(x.Left)
			inspect(x.Value)
		case *parser.TernaryExpression:
			inspect(x.Condition)
			inspect(x.True)
			inspect(x.False)
		case *parser.WhileStatement:
			inspect(x.Condition)
			inspect(x.Body)
		case *parser.DoWhileStatement:
			inspect(x.Condition)
			inspect(x.Body)
		case *parser.ForeachStatement:
			inspect(x.Iterable)
			inspect(x.Body)
		case *parser.CallExpression:
			inspect(x.Function)
			for _, arg := range x.Arguments {
				inspect(arg)
			}
		case *parser.InfixExpression:
			inspect(x.Left)
			inspect(x.Right)
		case *parser.PrefixExpression:
			inspect(x.Right)
		case *parser.ReturnStatement:
			if x.ReturnValue != nil {
				inspect(x.ReturnValue)
			}
		case *parser.YieldExpression:
			found = true
		}
	}
	inspect(node)
	return found
}

func (r *Runtime) evaluateYield(ye *parser.YieldExpression) interface{} {
	if r.currentGenerator == nil {
		panic(&JossError{Type: "YieldError", Message: "yield solo puede ser utilizado dentro de una funcion generadora", File: r.CurrentFile, Line: ye.Token.Line})
	}
	var key interface{}
	var val interface{}
	if ye.Key != nil {
		key = r.evaluateExpression(ye.Key)
	} else {
		key = r.generatorIndex
		r.generatorIndex++
	}
	if ye.Value != nil {
		val = r.evaluateExpression(ye.Value)
	}
	item := GeneratorItem{Key: key, Value: val}

	select {
	case r.currentGenerator.items <- item:
		return val
	case <-r.currentGenerator.done:
		panic(&BreakPanic{})
	}
}
