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
