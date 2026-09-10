package core

import (
	"fmt"
	"sort"

	"github.com/jossecurity/joss/pkg/parser"
	runtimeplan "github.com/jossecurity/joss/pkg/runtime/plan"
)

// Execute runs the parsed program
func (r *Runtime) Execute(program *parser.Program) {
	defer func() {
		for i := len(r.topDefers) - 1; i >= 0; i-- {
			d := r.topDefers[i]
			if d != nil && d.Body != nil {
				r.executeStatement(d.Body)
			}
		}
		r.topDefers = nil
	}()

	// Ensure env is loaded
	if len(r.Env) == 0 {
		r.LoadEnv(nil)
	}

	// First pass: Register classes, interfaces and functions
	for _, stmt := range program.Statements {
		if classStmt, ok := stmt.(*parser.ClassStatement); ok {
			r.registerClass(classStmt)
		}
		if ifaceStmt, ok := stmt.(*parser.InterfaceStatement); ok {
			r.RegisterInterface(ifaceStmt)
		}
		if enumStmt, ok := stmt.(*parser.EnumStatement); ok {
			r.RegisterEnum(enumStmt)
		}
		if methodStmt, ok := stmt.(*parser.MethodStatement); ok {
			r.Functions[methodStmt.Name.Value] = methodStmt
			r.planForMethod(methodStmt)
		}
	}

	// Find and execute Main class Init main
	hasClasses := false
	for _, stmt := range program.Statements {
		if _, ok := stmt.(*parser.ClassStatement); ok {
			hasClasses = true
			break
		}
	}

	if hasClasses {
		hasMain := false
		for _, stmt := range program.Statements {
			if s, ok := stmt.(*parser.ClassStatement); ok && s.Name.Value == "Main" {
				hasMain = true
				break
			}
		}
		if hasMain {
			r.executeMain(program)
		} else {
			for _, stmt := range program.Statements {
				if _, ok := stmt.(*parser.ClassStatement); !ok {
					if _, ok := stmt.(*parser.InterfaceStatement); !ok {
						r.executeStatement(stmt)
					}
				}
			}
		}
	} else {
		// Script programs execute their top-level statements directly.
		for _, stmt := range program.Statements {
			r.executeStatement(stmt)
		}
	}
}

func (r *Runtime) executeMain(program *parser.Program) {
	// Find Class Main
	var mainClass *parser.ClassStatement
	for _, stmt := range program.Statements {
		if s, ok := stmt.(*parser.ClassStatement); ok {
			if s.Name.Value == "Main" {
				mainClass = s
				break
			}
		}
	}

	if mainClass == nil {
		// fmt.Println("Error: No se encontró la clase Main")
		return
	}

	// Find Init main inside Main
	var initMain *parser.InitStatement
	for _, stmt := range mainClass.Body.Statements {
		if s, ok := stmt.(*parser.InitStatement); ok {
			if s.Name.Value == "main" {
				initMain = s
				break
			}
		}
	}

	if initMain == nil {
		fmt.Println("Error: No se encontró Init main() en la clase Main")
		return
	}

	// Execute Init main body
	r.executeBlock(initMain.Body)
}

func (r *Runtime) executeBlock(block *parser.BlockStatement) interface{} {
	var result interface{}
	for _, stmt := range block.Statements {
		result = r.executeStatement(stmt)
	}
	return result
}

func (r *Runtime) registerClass(stmt *parser.ClassStatement) {
	r.Classes[stmt.Name.Value] = stmt
	if stmt.Body != nil {
		for _, member := range stmt.Body.Statements {
			if method, ok := member.(*parser.MethodStatement); ok && method.Body != nil {
				r.planForMethod(method)
			}
		}
	}
}

func (r *Runtime) executeStatement(stmt parser.Statement) interface{} {
	switch s := stmt.(type) {
	case *parser.LetStatement:
		if _, resolved := r.slotForIdentifier(s.Name); resolved {
			var value interface{}
			if s.Value != nil {
				value = r.evaluateExpression(s.Value)
			} else {
				value = r.getZeroValue(s.Token.Literal)
			}
			r.assignLocal(s.Name, value, true)
			return nil
		}
		if r.Constants == nil {
			r.Constants = make(map[string]bool)
		}
		if r.Constants[s.Name.Value] {
			panic(&JossError{Type: "ConstantAssignment", Message: fmt.Sprintf("La constante '%s' no puede redeclararse", s.Name.Value), File: r.CurrentFile, Line: s.Name.Token.Line})
		}
		var val interface{}
		if s.Value != nil {
			val = r.evaluateExpression(s.Value)
			if s.Token.Literal != "var" {
				val = r.coerceToTypedValue(val, s.Token.Literal)
			}
		} else {
			val = r.getZeroValue(s.Token.Literal)
		}

		declaredType := s.Token.Literal
		if declaredType == "var" {
			declaredType = runtimeTypeName(val)
		}
		if declaredType != "" {
			r.VarTypes[s.Name.Value] = declaredType
		}
		if declaredType != "" && !r.checkType(val, declaredType) {
			panic(fmt.Sprintf("Error de Tipado: Variable '%s' definida como '%s' pero asignada valor incompatible", s.Name.Value, s.Token.Literal))
		}
		r.Variables[s.Name.Value] = val
		if s.IsConst {
			r.Constants[s.Name.Value] = true
		}
	case *parser.MultiLetStatement:
		// int $a,$b  or  int $a=1,$b=2
		for _, decl := range s.Declarations {
			var val interface{}
			if decl.Value != nil {
				val = r.evaluateExpression(decl.Value)
				if s.TypeToken.Literal != "var" {
					val = r.coerceToTypedValue(val, s.TypeToken.Literal)
				}
			} else {
				val = r.getZeroValue(s.TypeToken.Literal)
			}
			if _, resolved := r.slotForIdentifier(decl.Name); resolved {
				r.assignLocal(decl.Name, val, true)
				continue
			}
			declaredType := s.TypeToken.Literal
			if declaredType == "var" {
				declaredType = runtimeTypeName(val)
			}
			if declaredType != "" {
				r.VarTypes[decl.Name.Value] = declaredType
			}
			if declaredType != "" && !r.checkType(val, declaredType) {
				panic(fmt.Sprintf("Error de Tipado: Variable '%s' definida como '%s' pero asignada valor incompatible", decl.Name.Value, s.TypeToken.Literal))
			}
			r.Variables[decl.Name.Value] = val
		}
	case *parser.ExpressionStatement:
		if postfix, ok := s.Expression.(*parser.PostfixExpression); ok && r.executePostfixStatement(postfix) {
			return nil
		}
		return r.evaluateExpression(s.Expression)
	case *parser.ForeachStatement:
		return r.executeForeach(s)
	case *parser.EchoStatement:
		val := r.evaluateExpression(s.Value)
		fmt.Println(val)
	case *parser.WhileStatement:
		return r.executeWhile(s)
	case *parser.DoWhileStatement:
		return r.executeDoWhile(s)
	case *parser.TryCatchStatement:
		return r.executeTryCatch(s)
	case *parser.ThrowStatement:
		return r.executeThrow(s)
	case *parser.ReturnStatement:
		return r.executeReturn(s)
	case *parser.BreakStatement:
		return r.executeBreak(s)
	case *parser.ContinueStatement:
		return r.executeContinue(s)
	case *parser.DeferStatement:
		return r.executeDefer(s)
	case *parser.BlockStatement:
		return r.executeBlock(s)
	case *parser.MethodStatement:
		r.Functions[s.Name.Value] = s
		r.planForMethod(s)
	case *parser.ClassStatement:
		r.registerClass(s)
	case *parser.InterfaceStatement:
		r.RegisterInterface(s)
	case *parser.EnumStatement:
		r.RegisterEnum(s)
	case *parser.SelectStatement:
		return r.executeSelect(s)
	}
	return nil
}

func (r *Runtime) executeDefer(ds *parser.DeferStatement) interface{} {
	if r.currentFrame != nil {
		r.currentFrame.defers = append(r.currentFrame.defers, ds)
	} else {
		r.topDefers = append(r.topDefers, ds)
	}
	return nil
}

func (r *Runtime) executeReturn(rs *parser.ReturnStatement) interface{} {
	var val interface{}
	if rs.ReturnValue != nil {
		val = r.evaluateExpression(rs.ReturnValue)
	}
	panic(&ReturnPanic{Value: val})
}

func (r *Runtime) executeBreak(bs *parser.BreakStatement) interface{} {
	panic(&BreakPanic{})
}

func (r *Runtime) executeContinue(cs *parser.ContinueStatement) interface{} {
	panic(&ContinuePanic{})
}

func (r *Runtime) executeForeach(fs *parser.ForeachStatement) interface{} {
	iterable := r.evaluateExpression(fs.Iterable)

	valSlot := -1
	keySlot := -1
	if r.currentFrame != nil && r.currentFrame.plan != nil {
		if s, exists := r.currentFrame.plan.ForeachSlots[fs]; exists {
			valSlot = s
		}
		if fs.Key != "" {
			if s, exists := r.currentFrame.plan.ForeachKeySlots[fs]; exists {
				keySlot = s
			}
		}
	}
	hasControl := r.blockHasControl(fs.Body)

	executeIter := func(key interface{}, item interface{}) (shouldBreak bool) {
		if fs.Key != "" {
			if keySlot >= 0 {
				r.bindSlot(keySlot, key)
			} else {
				r.Variables[fs.Key] = key
			}
		}
		if valSlot >= 0 {
			r.bindSlot(valSlot, item)
		} else {
			r.Variables[fs.Value] = item
		}
		if hasControl {
			return r.executeLoopBodyProtected(fs.Body)
		}
		r.executeBlock(fs.Body)
		return false
	}

	if list, ok := iterable.([]interface{}); ok {
		for i, item := range list {
			if executeIter(int64(i), item) {
				break
			}
		}
	} else if m, ok := iterable.(map[string]interface{}); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if executeIter(k, m[k]) {
				break
			}
		}
	} else if list, ok := iterable.([]map[string]interface{}); ok {
		for i, item := range list {
			if executeIter(int64(i), item) {
				break
			}
		}
	} else if ch, ok := iterable.(*Channel); ok {
		var i int64
		for item := range ch.Ch {
			if executeIter(i, item) {
				break
			}
			i++
		}
	} else if gen, ok := iterable.(*Generator); ok {
		defer gen.Close()
		for {
			select {
			case item, ok := <-gen.items:
				if !ok {
					return nil
				}
				if executeIter(item.Key, item.Value) {
					return nil
				}
			case <-gen.done:
				return nil
			}
		}
	} else {
		fmt.Printf("Error: Foreach espera un array, mapa o canal, se obtuvo: %T\n", iterable)
	}
	return nil
}

func (r *Runtime) executeWhile(ws *parser.WhileStatement) interface{} {
	hasControl := r.blockHasControl(ws.Body)
	for {
		if !r.evaluateCondition(ws.Condition) {
			break
		}
		if hasControl {
			if r.executeLoopBodyProtected(ws.Body) {
				break
			}
		} else {
			r.executeBlock(ws.Body)
		}
	}
	return nil
}

func (r *Runtime) executeDoWhile(dws *parser.DoWhileStatement) interface{} {
	hasControl := r.blockHasControl(dws.Body)
	for {
		if hasControl {
			if r.executeLoopBodyProtected(dws.Body) {
				break
			}
		} else {
			r.executeBlock(dws.Body)
		}
		if !r.evaluateCondition(dws.Condition) {
			break
		}
	}
	return nil
}

func (r *Runtime) executeLoopBodyProtected(body *parser.BlockStatement) (shouldBreak bool) {
	defer func() {
		if err := recover(); err != nil {
			switch err.(type) {
			case *BreakPanic:
				shouldBreak = true
			case *ContinuePanic:
				// Skip
			default:
				panic(err)
			}
		}
	}()
	r.executeBlock(body)
	return false
}

func (r *Runtime) blockHasControl(body *parser.BlockStatement) bool {
	if r.currentFrame != nil && r.currentFrame.plan != nil && r.currentFrame.plan.LoopControl != nil {
		if has, ok := r.currentFrame.plan.LoopControl[body]; ok {
			return has
		}
	}
	return runtimeplan.BlockHasBreakOrContinue(body)
}

func (r *Runtime) evaluateCondition(expression parser.Expression) bool {
	if infix, ok := expression.(*parser.InfixExpression); ok {
		if result, handled := r.evaluateSlotIntegerInfix(infix); handled {
			if b, ok := result.(bool); ok {
				return b
			}
		}
	}
	return isTruthy(r.evaluateExpression(expression))
}

func (r *Runtime) executeTryCatch(tcs *parser.TryCatchStatement) (result interface{}) {
	defer func() {
		if err := recover(); err != nil {
			// Do NOT catch internal control flow panics
			switch err.(type) {
			case *ReturnPanic, *BreakPanic, *ContinuePanic:
				panic(err) // Let it bubble up
			}

			// Build the value exposed to the catch variable.
			// If it is a JossError, expose a map so Joss code can inspect fields.
			var errVal interface{}
			if je, ok := err.(*JossError); ok {
				errVal = map[string]interface{}{
					"message": je.Message,
					"type":    je.Type,
					"file":    je.File,
					"line":    int64(je.Line),
					"error":   je.Error(),
				}
			} else if e, ok := err.(error); ok {
				errVal = e.Error()
			} else {
				errVal = fmt.Sprintf("%v", err)
			}

			// Bind error variable
			if r.currentFrame != nil {
				if slot, exists := r.currentFrame.plan.CatchSlots[tcs]; exists {
					r.bindSlot(slot, errVal)
				} else {
					r.Variables[tcs.CatchVar] = errVal
				}
			} else {
				r.Variables[tcs.CatchVar] = errVal
			}

			// Execute catch block
			result = r.executeBlock(tcs.CatchBlock)
		}
	}()

	return r.executeBlock(tcs.TryBlock)
}

func (r *Runtime) executeThrow(ts *parser.ThrowStatement) interface{} {
	val := r.evaluateExpression(ts.Value)
	panic(val)
}

// EvaluateExpression evaluates an expression AST node in the runtime.
func (r *Runtime) EvaluateExpression(exp parser.Expression) interface{} {
	return r.evaluateExpression(exp)
}

// ExecuteStatement executes a statement AST node in the runtime.
func (r *Runtime) ExecuteStatement(stmt parser.Statement) interface{} {
	return r.executeStatement(stmt)
}
