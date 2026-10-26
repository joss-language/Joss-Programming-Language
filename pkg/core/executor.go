package core

import (
	"fmt"
	"sort"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

// ExecutePrepared runs a fully validated PreparedProgram produced by the semantic analyzer.
// All declarations across all project source units are registered directly from the semantic model,
// avoiding duplicate file scans or re-parsing.
func (r *Runtime) ExecutePrepared(prep *semanticanalyzer.PreparedProgram) {
	if prep == nil {
		return
	}
	r.PreparedProgram = prep
	if prep.Facts != nil {
		r.AnalysisFacts = prep.Facts
	}

	defer func() {
		for i := len(r.topDefers) - 1; i >= 0; i-- {
			d := r.topDefers[i]
			if d != nil && d.Body != nil {
				r.executeStatement(d.Body)
			}
		}
		r.topDefers = nil
	}()

	if len(r.Env) == 0 {
		r.LoadEnv(nil)
	}

	// Register declarations across all units in the semantic model
	for _, unit := range prep.Units {
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
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
	}

	// Execute entrypoint
	entry := prep.Entrypoint()
	if entry == nil {
		return
	}

	hasClasses := false
	for _, stmt := range entry.Statements {
		if _, ok := stmt.(*parser.ClassStatement); ok {
			hasClasses = true
			break
		}
	}

	if hasClasses {
		hasMain := false
		for _, stmt := range entry.Statements {
			if s, ok := stmt.(*parser.ClassStatement); ok && s.Name.Value == "Main" {
				hasMain = true
				break
			}
		}
		if hasMain {
			r.executeMain(entry)
		} else {
			for _, stmt := range entry.Statements {
				if _, ok := stmt.(*parser.ClassStatement); !ok {
					if _, ok := stmt.(*parser.InterfaceStatement); !ok {
						r.executeStatement(stmt)
						if r.topHasReturn {
							break
						}
					}
				}
			}
		}
	} else {
		for _, stmt := range entry.Statements {
			r.executeStatement(stmt)
			if r.topHasReturn {
				break
			}
		}
	}
}

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
						if r.topHasReturn {
							break
						}
					}
				}
			}
		}
	} else {
		// Script programs execute their top-level statements directly.
		for _, stmt := range program.Statements {
			r.executeStatement(stmt)
			if r.topHasReturn {
				break
			}
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
	if r.executionContext != nil {
		r.checkExecutionCancelled()
	}
	var result interface{}
	frame := r.currentFrame
	if frame != nil {
		for _, stmt := range block.Statements {
			result = r.executeStatement(stmt)
			if frame.hasReturn {
				return frame.returnValue
			}
			if r.hasBreak || r.hasContinue {
				return result
			}
		}
		return result
	}
	for _, stmt := range block.Statements {
		result = r.executeStatement(stmt)
		if r.topHasReturn {
			return r.topReturnValue
		}
		if r.hasBreak || r.hasContinue {
			return result
		}
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
	if r.executionContext != nil {
		r.checkExecutionCancelled()
	}
	switch s := stmt.(type) {
	case *parser.LetStatement:
		if slot, resolved := r.slotForIdentifier(s.Name); resolved {
			if s.Value != nil {
				if n, ok := r.slotIntegerOperand(s.Value); ok && (slot.Type.Kind == typesystem.Int || slot.Inferred) {
					if !slot.Initialized {
						slot.Type = typesystem.Type{Kind: typesystem.Int}
						slot.TypeName = "int"
					}
					slot.SetInt(n)
					return nil
				}
				if f, isF, ok := r.slotFloatOperand(s.Value); ok && isF && (slot.Type.Kind == typesystem.Float || slot.Inferred) {
					if !slot.Initialized {
						slot.Type = typesystem.Type{Kind: typesystem.Float}
						slot.TypeName = "float"
					}
					slot.SetFloat(f)
					return nil
				}
			}
			var value interface{}
			if s.Value != nil {
				value = r.evaluateExpression(s.Value)
			} else {
				value = r.getZeroValue(s.Token.Literal)
			}
			r.assignSlot(slot, value, true)
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
	case *parser.DestructureStatement:
		val := r.evaluateExpression(s.Value)
		var items []interface{}
		if list, ok := val.([]interface{}); ok {
			items = list
		} else if listMap, ok := val.([]map[string]interface{}); ok {
			for _, item := range listMap {
				items = append(items, item)
			}
		} else {
			items = []interface{}{val}
		}
		for i, ident := range s.Names {
			var elemVal interface{}
			if i < len(items) {
				elemVal = items[i]
			}
			if ident != nil {
				r.bindDestructuredVariable(ident, elemVal)
			}
		}
	case *parser.ExpressionStatement:
		switch expr := s.Expression.(type) {
		case *parser.PostfixExpression:
			if r.executePostfixStatement(expr) {
				return nil
			}
			return r.evaluatePostfix(expr)
		case *parser.AssignExpression:
			return r.evaluateAssign(expr)
		case *parser.TernaryExpression:
			return r.evaluateTernary(expr)
		default:
			return r.evaluateExpression(expr)
		}
	case *parser.ForeachStatement:
		return r.executeForeach(s)
	case *parser.EchoStatement:
		val := r.evaluateExpression(s.Value)
		fmt.Fprintln(r.Output(), val)
	case *parser.WhileStatement:
		return r.executeWhile(s)
	case *parser.GuardStatement:
		return r.executeGuard(s)
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
	if r.currentFrame != nil {
		r.currentFrame.hasReturn = true
		r.currentFrame.returnValue = val
		return val
	}
	r.topHasReturn = true
	r.topReturnValue = val
	return val
}

func (r *Runtime) executeBreak(bs *parser.BreakStatement) interface{} {
	r.hasBreak = true
	return nil
}

func (r *Runtime) executeContinue(cs *parser.ContinueStatement) interface{} {
	r.hasContinue = true
	return nil
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
		r.executeBlock(fs.Body)
		if r.hasBreak {
			r.hasBreak = false
			return true
		}
		if r.hasContinue {
			r.hasContinue = false
		}
		if (r.currentFrame != nil && r.currentFrame.hasReturn) || r.topHasReturn {
			return true
		}
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
	frame := r.currentFrame
	if frame != nil {
		for {
			if !r.evaluateCondition(ws.Condition) {
				break
			}
			r.executeBlock(ws.Body)
			if r.hasBreak {
				r.hasBreak = false
				break
			}
			if r.hasContinue {
				r.hasContinue = false
			}
			if frame.hasReturn {
				break
			}
		}
		return nil
	}
	for {
		if !r.evaluateCondition(ws.Condition) {
			break
		}
		r.executeBlock(ws.Body)
		if r.hasBreak {
			r.hasBreak = false
			break
		}
		if r.hasContinue {
			r.hasContinue = false
		}
		if r.topHasReturn {
			break
		}
	}
	return nil
}

func (r *Runtime) executeGuard(gs *parser.GuardStatement) interface{} {
	if !r.evaluateCondition(gs.Condition) {
		return r.executeBlock(gs.Body)
	}
	return nil
}

func (r *Runtime) executeDoWhile(dws *parser.DoWhileStatement) interface{} {
	for {
		r.executeBlock(dws.Body)
		if r.hasBreak {
			r.hasBreak = false
			break
		}
		if r.hasContinue {
			r.hasContinue = false
		}
		if (r.currentFrame != nil && r.currentFrame.hasReturn) || r.topHasReturn {
			break
		}
		if !r.evaluateCondition(dws.Condition) {
			break
		}
	}
	return nil
}

func (r *Runtime) evaluateCondition(expression parser.Expression) bool {
	if infix, ok := expression.(*parser.InfixExpression); ok {
		if result, handled := r.evaluateSlotIntegerInfix(infix); handled {
			if b, ok := result.(bool); ok {
				return b
			}
		}
		if result, handled := r.evaluateSlotFloatInfix(infix); handled {
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
			// Preserve thrown instances and structured objects (P4).
			var errVal interface{}
			if inst, ok := err.(*Instance); ok {
				errVal = inst
			} else if m, ok := err.(map[string]interface{}); ok {
				errVal = m
			} else if je, ok := err.(*JossError); ok {
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
				errVal = err
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
