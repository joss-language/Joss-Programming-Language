package core

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	runtimeframe "github.com/jossecurity/joss/pkg/runtime/frame"
	"github.com/jossecurity/joss/pkg/typesystem"
	"github.com/shopspring/decimal"
)

func (r *Runtime) evaluateTernary(te *parser.TernaryExpression) interface{} {
	cond := r.evaluateExpression(te.Condition)
	isTrue := isTruthy(cond)

	var result interface{}

	if te.True == nil {
		// Elvis Operator
		if isTrue {
			result = cond
		} else {
			result = r.evaluateExpression(te.False)
		}
	} else if te.False == nil {
		// Single-branch conditional
		if isTrue {
			result = r.evaluateExpression(te.True)
		} else {
			return nil
		}
	} else {
		// Standard Ternary
		if isTrue {
			result = r.evaluateExpression(te.True)
		} else {
			result = r.evaluateExpression(te.False)
		}
	}

	if blk, ok := result.(*parser.BlockStatement); ok {
		return r.executeBlock(blk)
	}

	return result
}

func (r *Runtime) evaluateInfix(ie *parser.InfixExpression) interface{} {
	if ie.Operator == "??" {
		var left interface{}
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					left = nil
				}
			}()
			left = r.evaluateExpression(ie.Left)
		}()
		if left != nil {
			return left
		}
		return r.evaluateExpression(ie.Right)
	}
	if ie.Operator == "|>" {
		leftVal := r.evaluateExpression(ie.Left)
		switch right := ie.Right.(type) {
		case *parser.CallExpression:
			// Prepend leftVal as first argument
			args := make([]interface{}, 0, len(right.Arguments)+1)
			args = append(args, leftVal)
			for _, arg := range right.Arguments {
				args = append(args, r.evaluateCallArgument(arg))
			}
			var fn interface{}
			if ident, ok := right.Function.(*parser.Identifier); ok {
				if res, ok := r.callBuiltin(ident.Value, args); ok {
					return res
				}
				if f, ok := r.Functions[ident.Value]; ok {
					fn = f
				} else if val, resolved, initialized := r.localValue(ident); resolved && initialized {
					fn = val
				} else if v, ok := r.Variables[ident.Value]; ok && r.sourceMapVisible(ident.Value) {
					fn = v
				}
			} else {
				fn = r.evaluateExpression(right.Function)
			}
			if fn == nil {
				panic(&JossError{Type: "NotCallable", Message: "Función de pipeline no encontrada o nula", File: r.CurrentFile})
			}
			return r.applyFunction(fn, args)
		case *parser.Identifier:
			if res, ok := r.callBuiltin(right.Value, []interface{}{leftVal}); ok {
				return res
			}
			var fn interface{}
			if f, ok := r.Functions[right.Value]; ok {
				fn = f
			} else if val, resolved, initialized := r.localValue(right); resolved && initialized {
				fn = val
			} else if v, ok := r.Variables[right.Value]; ok && r.sourceMapVisible(right.Value) {
				fn = v
			}
			if fn == nil {
				panic(&JossError{Type: "UndefinedFunction", Message: fmt.Sprintf("Función '%s' no encontrada", right.Value), File: r.CurrentFile, Line: right.Token.Line})
			}
			return r.applyFunction(fn, []interface{}{leftVal})
		case *parser.FunctionLiteral:
			captured := r.captureFunction(right)
			return r.callCapturedFunction(captured, []interface{}{leftVal})
		default:
			rightVal := r.evaluateExpression(ie.Right)
			return r.applyFunction(rightVal, []interface{}{leftVal})
		}
	}
	if result, handled := r.evaluateSlotIntegerInfix(ie); handled {
		return result
	}

	left := r.evaluateExpression(ie.Left)

	// Short-Circuit Logic for && and ||
	if ie.Operator == "&&" {
		if !isTruthy(left) {
			return false
		}
		right := r.evaluateExpression(ie.Right)
		return isTruthy(right)
	}

	if ie.Operator == "||" {
		if isTruthy(left) {
			return true
		}
		right := r.evaluateExpression(ie.Right)
		return isTruthy(right)
	}

	// Handle cin >> $var
	if ie.Operator == ">>" {
		if _, ok := left.(*Cin); ok {
			if noInteract, ok := r.Env["NON_INTERACTIVE"]; ok && (noInteract == "true" || noInteract == "1") {
				fmt.Println("[Cin] Input skipped (NON_INTERACTIVE mode)")
				return nil
			}

			if ident, ok := ie.Right.(*parser.Identifier); ok {
				val := r.readCinInputForIdentifier(ident)

				if _, resolved := r.assignLocal(ident, val, false); resolved {
					return left
				}
				if reference, exists := r.Variables[ident.Value].(*VariableReference); exists {
					reference.Set(r, val)
					return left
				}
				if expectedType, exists := r.VarTypes[ident.Value]; exists {
					val = r.coerceToTypedValue(val, expectedType)
					if !r.checkType(val, expectedType) {
						fmt.Printf("Error de Tipado: No se puede asignar valor a '%s' (se espera %s)\n", ident.Value, expectedType)
						return nil
					}
				}

				r.Variables[ident.Value] = val
				return left
			}
			fmt.Println("Error: cin >> requiere una variable")
			return nil
		}
	}

	right := r.evaluateExpression(ie.Right)

	if ie.Operator == "===" {
		return strictCompare(left, right)
	}
	if ie.Operator == "!==" {
		return !strictCompare(left, right)
	}
	if ie.Operator == "<=>" {
		return spaceshipCompare(left, right)
	}

	// Handle cout << val, cerr << val or channel << val
	if ie.Operator == "<<" {
		if _, ok := left.(*Cout); ok {
			fmt.Print(right)
			return left
		}
		if _, ok := left.(*Cerr); ok {
			fmt.Fprint(os.Stderr, right)
			return left
		}
		if ch, ok := left.(*Channel); ok {
			ch.Ch <- right
			return ch
		}
	}

	// Handle Pipe Operator |>
	if ie.Operator == "|>" {
		switch rightNode := ie.Right.(type) {
		case *parser.Identifier:
			fnName := rightNode.Value
			if fn, ok := r.Functions[fnName]; ok {
				return r.applyFunction(fn, []interface{}{left})
			}
			if res, ok := r.callBuiltin(fnName, []interface{}{left}); ok {
				return res
			}
			fmt.Printf("Error: Función '%s' no encontrada para pipe\n", fnName)
			return nil

		case *parser.CallExpression:
			var fn interface{}
			if ident, ok := rightNode.Function.(*parser.Identifier); ok {
				if f, ok := r.Functions[ident.Value]; ok {
					fn = f
				} else {
					args := []interface{}{left}
					for _, argExp := range rightNode.Arguments {
						args = append(args, r.evaluateExpression(argExp))
					}

					if res, ok := r.callBuiltin(ident.Value, args); ok {
						return res
					}
					fmt.Printf("Error: Función '%s' no encontrada en pipe call\n", ident.Value)
					return nil
				}
			} else {
				fn = r.evaluateExpression(rightNode.Function)
			}

			args := []interface{}{left}
			for _, argExp := range rightNode.Arguments {
				args = append(args, r.evaluateExpression(argExp))
			}

			return r.applyFunction(fn, args)

		case *parser.FunctionLiteral:
			return r.applyFunction(rightNode, []interface{}{left})

		default:
			fmt.Printf("Error: El lado derecho del pipe debe ser una función o llamada, se obtuvo %T\n", ie.Right)
			return nil
		}
	}

	// Preserve exact integer semantics. Float promotion is used only when at
	// least one operand is actually a float.
	leftInt, leftIsInt := runtimeInteger(left)
	rightInt, rightIsInt := runtimeInteger(right)
	if leftIsInt && rightIsInt {
		switch ie.Operator {
		case "+", "-", "*", "%":
			result, fault := typesystem.CheckedIntBinary(ie.Operator, leftInt, rightInt)
			if fault != typesystem.ArithmeticOK {
				panic(r.integerArithmeticError(ie, fault, leftInt, rightInt))
			}
			return result
		case "/":
			if rightInt == 0 {
				panic(r.integerArithmeticError(ie, typesystem.ArithmeticDivisionByZero, leftInt, rightInt))
			}
			return float64(leftInt) / float64(rightInt)
		case "<":
			return leftInt < rightInt
		case ">":
			return leftInt > rightInt
		case ">=":
			return leftInt >= rightInt
		case "<=":
			return leftInt <= rightInt
		case "==":
			return leftInt == rightInt
		case "!=":
			return leftInt != rightInt
		}
	}

	// Decimal operations preserve exact precision.
	toDecimal := func(val interface{}) (decimal.Decimal, bool) {
		switch v := val.(type) {
		case decimal.Decimal:
			return v, true
		case int64:
			return decimal.NewFromInt(v), true
		case int:
			return decimal.NewFromInt(int64(v)), true
		case float64:
			return decimal.NewFromFloat(v), true
		case float32:
			return decimal.NewFromFloat(float64(v)), true
		default:
			return decimal.Zero, false
		}
	}

	_, leftIsDec := left.(decimal.Decimal)
	_, rightIsDec := right.(decimal.Decimal)
	if leftIsDec || rightIsDec {
		lDec, lOk := toDecimal(left)
		rDec, rOk := toDecimal(right)
		if lOk && rOk {
			switch ie.Operator {
			case "+":
				return lDec.Add(rDec)
			case "-":
				return lDec.Sub(rDec)
			case "*":
				return lDec.Mul(rDec)
			case "/":
				if rDec.IsZero() {
					panic(&JossError{Code: diagnostics.CodeDivisionByZero, Type: "ArithmeticError", Message: "División entre cero", File: r.CurrentFile, Line: ie.Token.Line, Column: ie.Token.Column})
				}
				return lDec.Div(rDec)
			case "%":
				if rDec.IsZero() {
					panic(&JossError{Code: diagnostics.CodeDivisionByZero, Type: "ArithmeticError", Message: "División entre cero", File: r.CurrentFile, Line: ie.Token.Line, Column: ie.Token.Column})
				}
				return lDec.Mod(rDec)
			case "<":
				return lDec.LessThan(rDec)
			case ">":
				return lDec.GreaterThan(rDec)
			case "<=":
				return lDec.LessThanOrEqual(rDec)
			case ">=":
				return lDec.GreaterThanOrEqual(rDec)
			case "==":
				return lDec.Equal(rDec)
			case "!=":
				return !lDec.Equal(rDec)
			case "&&":
				return !lDec.IsZero() && !rDec.IsZero()
			case "||":
				return !lDec.IsZero() || !rDec.IsZero()
			}
		}
	}

	// Mixed int/float operations promote to float.
	toFloat := func(val interface{}) (float64, bool) {
		if i, ok := val.(int64); ok {
			return float64(i), true
		}
		if i, ok := val.(int); ok {
			return float64(i), true
		}
		if f, ok := val.(float64); ok {
			return f, true
		}
		return 0, false
	}

	lFloat, lIsNum := toFloat(left)
	rFloat, rIsNum := toFloat(right)

	if lIsNum && rIsNum {
		if ie.Operator == "/" {
			if rFloat == 0 {
				panic(&JossError{Code: diagnostics.CodeDivisionByZero, Type: "ArithmeticError", Message: "División entre cero", File: r.CurrentFile, Line: ie.Token.Line, Column: ie.Token.Column})
			}
			return lFloat / rFloat
		}
		if ie.Operator == "%" {
			leftModulo, rightModulo := int64(lFloat), int64(rFloat)
			result, fault := typesystem.CheckedIntBinary("%", leftModulo, rightModulo)
			if fault != typesystem.ArithmeticOK {
				panic(r.integerArithmeticError(ie, fault, leftModulo, rightModulo))
			}
			return result
		}

		isFloatOp := false
		if _, ok := left.(float64); ok {
			isFloatOp = true
		}
		if _, ok := right.(float64); ok {
			isFloatOp = true
		}

		if isFloatOp {
			switch ie.Operator {
			case "+":
				return lFloat + rFloat
			case "-":
				return lFloat - rFloat
			case "*":
				return lFloat * rFloat
			case "<":
				return lFloat < rFloat
			case ">":
				return lFloat > rFloat
			case ">=":
				return lFloat >= rFloat
			case "<=":
				return lFloat <= rFloat
			case "==":
				return lFloat == rFloat
			case "!=":
				return lFloat != rFloat
			case "&&":
				return (lFloat != 0) && (rFloat != 0)
			case "||":
				return (lFloat != 0) || (rFloat != 0)
			}
		} else {
			lInt := int64(lFloat)
			rInt := int64(rFloat)
			switch ie.Operator {
			case "+":
				return lInt + rInt
			case "-":
				return lInt - rInt
			case "*":
				return lInt * rInt
			case "<":
				return lInt < rInt
			case ">":
				return lInt > rInt
			case ">=":
				return lInt >= rInt
			case "<=":
				return lInt <= rInt
			case "==":
				return lInt == rInt
			case "!=":
				return lInt != rInt
			case "%":
				return lInt % rInt
			case "&&":
				return (lInt != 0) && (rInt != 0)
			case "||":
				return (lInt != 0) || (rInt != 0)
			}
		}
	}

	lStr := ""
	rStr := ""
	if left != nil {
		lStr = fmt.Sprintf("%v", left)
	}
	if right != nil {
		rStr = fmt.Sprintf("%v", right)
	}

	if ie.Operator == ".." {
		lInt, lOk := toInt64Safe(left)
		rInt, rOk := toInt64Safe(right)
		if lOk && rOk {
			if lInt <= rInt {
				count := rInt - lInt + 1
				if count > 1000000 {
					count = 1000000
				}
				res := make([]interface{}, 0, count)
				for i := lInt; i <= rInt; i++ {
					res = append(res, i)
				}
				return res
			} else {
				count := lInt - rInt + 1
				if count > 1000000 {
					count = 1000000
				}
				res := make([]interface{}, 0, count)
				for i := lInt; i >= rInt; i-- {
					res = append(res, i)
				}
				return res
			}
		}
		return []interface{}{}
	}

	if ie.Operator == "." {
		return lStr + rStr
	}
	if ie.Operator == "+" {
		fmt.Println("Error: El operador '+' es solo para números. Use '.' para concatenar cadenas.")
		return nil
	}
	if ie.Operator == "==" {
		return lStr == rStr
	}
	if ie.Operator == "!=" {
		return lStr != rStr
	}

	if bLeft, ok := left.(bool); ok {
		if bRight, ok := right.(bool); ok {
			if ie.Operator == "&&" {
				return bLeft && bRight
			}
			if ie.Operator == "||" {
				return bLeft || bRight
			}
		}
	}

	if ie.Operator == "??" {
		if left != nil {
			return left
		}
		return right
	}

	return nil
}

func (r *Runtime) evaluateSlotIntegerInfix(expression *parser.InfixExpression) (interface{}, bool) {
	left, leftOK := r.slotIntegerOperand(expression.Left)
	right, rightOK := r.slotIntegerOperand(expression.Right)
	if !leftOK || !rightOK {
		return nil, false
	}
	switch expression.Operator {
	case "+", "-", "*", "%":
		result, fault := typesystem.CheckedIntBinary(expression.Operator, left, right)
		if fault != typesystem.ArithmeticOK {
			panic(r.integerArithmeticError(expression, fault, left, right))
		}
		return result, true
	case "/":
		if right == 0 {
			panic(r.integerArithmeticError(expression, typesystem.ArithmeticDivisionByZero, left, right))
		}
		return float64(left) / float64(right), true
	case "<":
		return left < right, true
	case ">":
		return left > right, true
	case ">=":
		return left >= right, true
	case "<=":
		return left <= right, true
	case "==", "===":
		return left == right, true
	case "!=", "!==":
		return left != right, true
	default:
		return nil, false
	}
}

func (r *Runtime) slotIntegerOperand(expression parser.Expression) (int64, bool) {
	switch node := expression.(type) {
	case *parser.IntegerLiteral:
		return node.Value, true
	case *parser.Identifier:
		slot, resolved := r.slotForIdentifier(node)
		if !resolved || !slot.Initialized || slot.Value.Kind != runtimeframe.Int {
			return 0, false
		}
		return slot.Value.Integer, true
	default:
		return 0, false
	}
}

func runtimeInteger(value interface{}) (int64, bool) {
	switch number := value.(type) {
	case int64:
		return number, true
	case int:
		return int64(number), true
	default:
		return 0, false
	}
}

func (r *Runtime) integerArithmeticError(expression *parser.InfixExpression, fault typesystem.ArithmeticFault, left, right int64) *JossError {
	code := diagnostics.CodeArithmeticOverflow
	message := fmt.Sprintf("Overflow entero en %d %s %d", left, expression.Operator, right)
	if fault == typesystem.ArithmeticDivisionByZero {
		code = diagnostics.CodeDivisionByZero
		message = fmt.Sprintf("División entre cero en %d %s %d", left, expression.Operator, right)
	}
	return &JossError{Code: code, Type: "ArithmeticError", Message: message, File: r.CurrentFile, Line: expression.Token.Line, Column: expression.Token.Column}
}

func (r *Runtime) evaluatePrefix(pe *parser.PrefixExpression) interface{} {
	right := r.evaluateExpression(pe.Right)

	if pe.Operator == "!" {
		return !isTruthy(right)
	}

	if pe.Operator == "-" {
		if i, ok := right.(int64); ok {
			value, fault := typesystem.CheckedIntNegate(i)
			if fault != typesystem.ArithmeticOK {
				panic(&JossError{Code: diagnostics.CodeArithmeticOverflow, Type: "ArithmeticError", Message: fmt.Sprintf("Overflow entero al negar %d", i), File: r.CurrentFile, Line: pe.Token.Line, Column: pe.Token.Column})
			}
			return value
		}
		if f, ok := right.(float64); ok {
			return -f
		}
		if d, ok := right.(decimal.Decimal); ok {
			return d.Neg()
		}
	}

	return nil
}

func (r *Runtime) evaluatePostfix(pe *parser.PostfixExpression) interface{} {
	if pe.Operator == "++" || pe.Operator == "--" {
		if value, handled := r.updatePostfixSlot(pe, true); handled {
			return value
		}
		val := r.evaluateExpression(pe.Left)

		var newVal interface{}
		op := "+"
		if pe.Operator == "--" {
			op = "-"
		}
		if i, ok := val.(int64); ok {
			value, fault := typesystem.CheckedIntBinary(op, i, 1)
			if fault != typesystem.ArithmeticOK {
				panic(&JossError{Code: diagnostics.CodeArithmeticOverflow, Type: "ArithmeticError", Message: fmt.Sprintf("Overflow entero al operar %d", i), File: r.CurrentFile, Line: pe.Token.Line, Column: pe.Token.Column})
			}
			newVal = value
		} else if f, ok := val.(float64); ok {
			if pe.Operator == "--" {
				newVal = f - 1.0
			} else {
				newVal = f + 1.0
			}
		} else if d, ok := val.(decimal.Decimal); ok {
			if pe.Operator == "--" {
				newVal = d.Sub(decimal.NewFromInt(1))
			} else {
				newVal = d.Add(decimal.NewFromInt(1))
			}
		} else {
			fmt.Printf("Error: Operador %s solo aplicable a números\n", pe.Operator)
			return nil
		}

		r.updateVariable(pe.Left, newVal)
		return val
	}
	return nil
}

func (r *Runtime) executePostfixStatement(expression *parser.PostfixExpression) bool {
	_, handled := r.updatePostfixSlot(expression, false)
	return handled
}

func (r *Runtime) updatePostfixSlot(expression *parser.PostfixExpression, returnOld bool) (interface{}, bool) {
	identifier, ok := expression.Left.(*parser.Identifier)
	if !ok || (expression.Operator != "++" && expression.Operator != "--") {
		return nil, false
	}
	slot, resolved := r.slotForIdentifier(identifier)
	if !resolved || !slot.Initialized || slot.ByReference {
		return nil, false
	}
	if slot.Constant {
		panic(&JossError{Type: "ConstantAssignment", Message: fmt.Sprintf("La constante '%s' no puede modificarse", slot.Name), File: r.CurrentFile, Line: expression.Token.Line, Column: expression.Token.Column})
	}
	op := "+"
	if expression.Operator == "--" {
		op = "-"
	}
	switch slot.Value.Kind {
	case runtimeframe.Int:
		old := slot.Value.Integer
		updated, fault := typesystem.CheckedIntBinary(op, old, 1)
		if fault != typesystem.ArithmeticOK {
			panic(&JossError{Code: diagnostics.CodeArithmeticOverflow, Type: "ArithmeticError", Message: fmt.Sprintf("Overflow entero al operar %d", old), File: r.CurrentFile, Line: expression.Token.Line, Column: expression.Token.Column})
		}
		slot.Value.Integer = updated
		if returnOld {
			return old, true
		}
		return nil, true
	case runtimeframe.Float:
		old := slot.Value.Float
		if expression.Operator == "--" {
			slot.Value.Float = old - 1
		} else {
			slot.Value.Float = old + 1
		}
		if returnOld {
			return old, true
		}
		return nil, true
	default:
		return nil, false
	}
}

func (r *Runtime) evaluateMatch(me *parser.MatchExpression) interface{} {
	subject := r.evaluateExpression(me.Subject)

	var defaultArm *parser.MatchArm
	for _, arm := range me.Arms {
		if arm.IsDefault {
			defaultArm = &arm
			continue
		}

		for _, keyExpr := range arm.Keys {
			keyVal := r.evaluateExpression(keyExpr)
			if strictCompare(subject, keyVal) {
				result := r.evaluateExpression(arm.Value)
				if blk, ok := result.(*parser.BlockStatement); ok {
					return r.executeBlock(blk)
				}
				return result
			}
		}
	}

	if defaultArm != nil {
		result := r.evaluateExpression(defaultArm.Value)
		if blk, ok := result.(*parser.BlockStatement); ok {
			return r.executeBlock(blk)
		}
		return result
	}

	return nil
}

func (r *Runtime) readCinInputForIdentifier(ident *parser.Identifier) interface{} {
	targetType := ""
	if slot, resolved := r.slotForIdentifier(ident); resolved {
		if slot.TypeName != "" {
			targetType = slot.TypeName
		} else if slot.Value.Kind == runtimeframe.Int {
			targetType = "int"
		} else if slot.Value.Kind == runtimeframe.Float {
			targetType = "float"
		} else if slot.Value.Kind == runtimeframe.String {
			targetType = "string"
		}
	} else if t, ok := r.VarTypes[ident.Value]; ok {
		targetType = t
	} else if existing, ok := r.Variables[ident.Value]; ok {
		switch existing.(type) {
		case int, int64, int32:
			targetType = "int"
		case float64, float32:
			targetType = "float"
		case string:
			targetType = "string"
		}
	}

	var raw string
	if len(r.cinTokens) > 0 {
		raw = r.cinTokens[0]
		r.cinTokens = r.cinTokens[1:]
	} else {
		if r.cinReader == nil {
			r.cinReader = bufio.NewReader(os.Stdin)
		}
		line, err := r.cinReader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return ""
		}
		line = strings.TrimRight(line, "\r\n")

		if targetType == "int" || targetType == "float" || targetType == "decimal" {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				raw = fields[0]
				r.cinTokens = fields[1:]
			} else if len(fields) == 1 {
				raw = fields[0]
			} else {
				raw = ""
			}
		} else {
			raw = line
		}
	}

	trimmed := strings.TrimSpace(raw)
	if targetType == "int" {
		if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return n
		}
	} else if targetType == "float" {
		if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return f
		}
	} else if targetType == "decimal" {
		clean := strings.TrimRight(trimmed, "mMdD")
		if d, err := decimal.NewFromString(clean); err == nil {
			return d
		}
	} else if targetType == "string" {
		return raw
	} else {
		if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return n
		}
		if f, err := strconv.ParseFloat(trimmed, 64); err == nil && strings.Contains(trimmed, ".") {
			return f
		}
	}

	return raw
}
