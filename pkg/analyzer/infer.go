package analyzer

import (
	"fmt"
	"strings"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

func (a *Analyzer) inferExpression(expression parser.Expression, current *scope) typesystem.Type {
	if expression == nil {
		return typesystem.Type{Kind: typesystem.Unknown}
	}
	switch node := expression.(type) {
	case *parser.StringLiteral:
		return typesystem.Type{Kind: typesystem.String}
	case *parser.IntegerLiteral:
		return typesystem.Type{Kind: typesystem.Int}
	case *parser.FloatLiteral:
		return typesystem.Type{Kind: typesystem.Float}
	case *parser.DecimalLiteral:
		return typesystem.Type{Kind: typesystem.Decimal}
	case *parser.Boolean:
		return typesystem.Type{Kind: typesystem.Bool}
	case *parser.NullLiteral:
		return typesystem.Type{Kind: typesystem.Null}
	case *parser.Identifier:
		return a.inferIdentifier(node, current)
	case *parser.ArrayLiteral:
		var elemType *typesystem.Type
		for _, element := range node.Elements {
			t := a.inferExpression(element, current)
			if t.IsKnown() {
				if elemType == nil {
					elemCopy := t
					elemType = &elemCopy
				} else if *elemType != t {
					elemType = nil
				}
			}
		}
		if elemType != nil {
			return typesystem.Type{Kind: typesystem.Array, Element: elemType}
		}
		return typesystem.Type{Kind: typesystem.Array}
	case *parser.MapLiteral:
		for key, value := range node.Pairs {
			if _, isSpread := key.(*parser.SpreadExpression); isSpread {
				a.inferExpression(key, current)
				continue
			}
			keyType := a.inferExpression(key, current)
			if keyType.IsKnown() && keyType.Kind != typesystem.String {
				a.add("JOSS-TYPE-005", diagnostics.SeverityError, a.file, tokenOfExpression(key),
					fmt.Sprintf("Map key has type `%s`; Joss maps require `string` keys.", keyType.String()),
					"The runtime indexes maps by string.", "Convert the key to string.")
			}
			a.inferExpression(value, current)
		}
		return typesystem.Type{Kind: typesystem.Map}
	case *parser.AssignExpression:
		return a.inferAssignment(node, current)
	case *parser.InfixExpression:
		return a.inferInfix(node, current)
	case *parser.PrefixExpression:
		valueType := a.inferExpression(node.Right, current)
		if node.Operator == "!" {
			return typesystem.Type{Kind: typesystem.Bool}
		}
		if node.Operator == "-" && valueType.IsKnown() && !valueType.IsNumeric() {
			a.invalidOperator(node.Token, node.Operator, valueType, typesystem.Type{})
		}
		return valueType
	case *parser.ReferenceExpression:
		valueType := a.inferExpression(node.Target, current)
		a.add("JOSS-REF-005", diagnostics.SeverityError, a.file, node.Token,
			"A reference cannot escape its call argument.",
			"Joss references are temporary aliases, not storable or returnable pointer values.", "Use `ref $variable` only in a call to a matching `ref` parameter.")
		return valueType
	case *parser.PostfixExpression:
		valueType := a.inferExpression(node.Left, current)
		if valueType.IsKnown() && !valueType.IsNumeric() {
			a.invalidOperator(node.Token, node.Operator, valueType, typesystem.Type{})
		}
		return valueType
	case *parser.TernaryExpression:
		a.inferExpression(node.Condition, current)
		trueScope, falseScope := a.narrowScopeFromCondition(node.Condition, current)
		trueType := a.inferExpression(node.True, trueScope)
		if node.True == nil {
			trueType = a.inferExpression(node.Condition, trueScope)
		}
		falseType := a.inferExpression(node.False, falseScope)
		return commonType(trueType, falseType)
	case *parser.IndexExpression:
		containerType := a.inferExpression(node.Left, current)
		indexType := a.inferExpression(node.Index, current)
		if containerType.IsKnown() {
			switch containerType.Kind {
			case typesystem.Array:
				if indexType.IsKnown() && indexType.Kind != typesystem.Int {
					a.add("JOSS-TYPE-006", diagnostics.SeverityError, a.file, node.Token,
						fmt.Sprintf("`%s` values require an `int` index, got `%s`.", containerType.String(), indexType.String()),
						"Index type is known before execution.", "Use an integer index.")
				}
				if containerType.Element != nil {
					return *containerType.Element
				}
				return typesystem.Type{Kind: typesystem.Unknown}
			case typesystem.String:
				if indexType.IsKnown() && indexType.Kind != typesystem.Int {
					a.add("JOSS-TYPE-006", diagnostics.SeverityError, a.file, node.Token,
						fmt.Sprintf("`%s` values require an `int` index, got `%s`.", containerType.String(), indexType.String()),
						"Index type is known before execution.", "Use an integer index.")
				}
				return typesystem.Type{Kind: typesystem.String}
			case typesystem.Map:
				if indexType.IsKnown() && indexType.Kind != typesystem.String {
					a.add("JOSS-TYPE-006", diagnostics.SeverityError, a.file, node.Token,
						fmt.Sprintf("`map` values require a `string` index, got `%s`.", indexType.String()),
						"Map keys are strings in the Joss runtime.", "Use a string key.")
				}
				if containerType.Element != nil {
					return *containerType.Element
				}
				return typesystem.Type{Kind: typesystem.Unknown}
			default:
				a.add("JOSS-TYPE-007", diagnostics.SeverityError, a.file, node.Token,
					fmt.Sprintf("Values of type `%s` cannot be indexed.", containerType.String()),
					"Only arrays, maps and strings support index access.", "Check the value or use member access.")
			}
		}
		return typesystem.Type{Kind: typesystem.Unknown}
	case *parser.NewExpression:
		return a.inferNew(node, current)
	case *parser.MemberExpression:
		return a.inferMember(node, current)
	case *parser.CallExpression:
		return a.inferCall(node, current)
	case *parser.FunctionLiteral:
		returnType := typeFromToken(node.ReturnType)
		a.validateDeclaredType(returnType, node.ReturnType, "return annotation")
		a.analyzeCallable(node.Parameters, node.Body, current, "", returnType)
		return typesystem.Type{Kind: typesystem.Object}
	case *parser.IssetExpression:
		a.suppressUndefined++
		for _, argument := range node.Arguments {
			a.inferExpression(argument, current)
		}
		a.suppressUndefined--
		return typesystem.Type{Kind: typesystem.Bool}
	case *parser.EmptyExpression:
		a.suppressUndefined++
		a.inferExpression(node.Argument, current)
		a.suppressUndefined--
		return typesystem.Type{Kind: typesystem.Bool}
	case *parser.BlockExpression:
		if node.Block != nil {
			a.analyzeBlock(node.Block, current)
		}
		return typesystem.Type{Kind: typesystem.Unknown}
	case *parser.MatchExpression:
		a.inferExpression(node.Subject, current)
		result := typesystem.Type{Kind: typesystem.Unknown}
		for _, arm := range node.Arms {
			if !arm.IsDefault {
				for _, key := range arm.Keys {
					if identifier, ok := key.(*parser.Identifier); ok && identifier.Value == "default" {
						continue
					}
					a.inferExpression(key, current)
				}
			}
			result = commonType(result, a.inferExpression(arm.Value, current))
		}
		return result
	case *parser.IsExpression:
		a.inferExpression(node.Left, current)
		return typesystem.Type{Kind: typesystem.Bool}
	case *parser.SpreadExpression:
		return a.inferExpression(node.Expression, current)
	case *parser.NamedArgument:
		return a.inferExpression(node.Value, current)
	case *parser.YieldExpression:
		if node.Key != nil {
			a.inferExpression(node.Key, current)
		}
		if node.Value != nil {
			a.inferExpression(node.Value, current)
		}
		return typesystem.Type{Kind: typesystem.Mixed}
	default:
		return typesystem.Type{Kind: typesystem.Unknown}
	}
}

func (a *Analyzer) inferIdentifier(identifier *parser.Identifier, current *scope) typesystem.Type {
	name := cleanName(identifier.Value)
	switch name {
	case "null", "nil":
		return typesystem.Type{Kind: typesystem.Null}
	case "true", "false":
		return typesystem.Type{Kind: typesystem.Bool}
	case "self", "this":
		if a.currentClass != "" {
			return typesystem.Type{Kind: typesystem.Class, Name: a.currentClass}
		}
	case "parent", "super":
		if class, exists := a.classes[a.currentClass]; exists && class.SuperClass != "" {
			return typesystem.Type{Kind: typesystem.Class, Name: class.SuperClass}
		}
	}
	if value, exists := current.resolve(name); exists {
		value.Used = true
		return value.Type
	}
	if _, exists := a.classes[name]; exists {
		return typesystem.Type{Kind: typesystem.Class, Name: name}
	}
	if _, exists := a.interfaces[name]; exists {
		return typesystem.Type{Kind: typesystem.Class, Name: name}
	}
	if _, exists := a.functions[name]; exists {
		return typesystem.Type{Kind: typesystem.Object}
	}
	if _, exists := a.environment.Builtins[name]; exists {
		return typesystem.Type{Kind: typesystem.Object}
	}
	// Uppercase names can still come from runtime/environment integrations. They
	// remain unknown unless an explicit const declaration resolved them above.
	if name != "" && strings.ToUpper(name) == name && len(name) > 1 {
		return typesystem.Type{Kind: typesystem.Unknown}
	}
	if a.suppressUndefined == 0 {
		a.add("JOSS-SYM-001", diagnostics.SeverityError, a.file, identifier.Token,
			fmt.Sprintf("Variable `$%s` is used before it is declared.", name),
			"The active lexical scope has no symbol with this name.", "Declare or assign the variable before this use.")
	}
	return typesystem.Type{Kind: typesystem.Unknown}
}

func (a *Analyzer) inferAssignment(assignment *parser.AssignExpression, current *scope) typesystem.Type {
	valueType := a.inferExpression(assignment.Value, current)
	if identifier, ok := assignment.Left.(*parser.Identifier); ok {
		name := cleanName(identifier.Value)
		if existing, exists := current.resolve(name); exists {
			if existing.Constant {
				a.add("JOSS-SYM-006", diagnostics.SeverityError, a.file, identifier.Token,
					fmt.Sprintf("Constant `$%s` cannot be reassigned.", name),
					"Constants are immutable after their declaration.", "Create a new variable instead of assigning to the constant.")
				return existing.Type
			}
			if existing.Inferred && !existing.Type.IsKnown() {
				existing.Type = typesystem.MergeInference(existing.Type, valueType)
			}
			if !existing.Dynamic && !a.assignableExpression(existing.Type, valueType, assignment.Value) {
				a.typeMismatch("JOSS-TYPE-001", name, existing.Type, valueType, identifier.Token, "assignment")
			}
			return existing.Type
		}
		inferredType := typesystem.MergeInference(typesystem.Type{Kind: typesystem.Unknown}, valueType)
		current.put(&symbol{Name: name, Type: inferredType, Kind: symbolVariable, Token: identifier.Token, File: a.file, Inferred: true})
		return inferredType
	}
	if arrLit, ok := assignment.Left.(*parser.ArrayLiteral); ok {
		for _, elem := range arrLit.Elements {
			var identifier *parser.Identifier
			elemType := typesystem.Type{Kind: typesystem.Mixed}
			if id, ok := elem.(*parser.Identifier); ok {
				identifier = id
				if valueType.Element != nil {
					elemType = *valueType.Element
				}
			} else if assign, ok := elem.(*parser.AssignExpression); ok {
				if id, ok := assign.Left.(*parser.Identifier); ok {
					identifier = id
					defType := a.inferExpression(assign.Value, current)
					if valueType.Element != nil {
						elemType = *valueType.Element
					} else {
						elemType = defType
					}
				}
			}
			if identifier != nil {
				name := cleanName(identifier.Value)
				if existing, exists := current.resolve(name); exists {
					if existing.Inferred && !existing.Type.IsKnown() {
						existing.Type = elemType
					}
				} else {
					current.put(&symbol{Name: name, Type: elemType, Kind: symbolVariable, Token: identifier.Token, File: a.file, Inferred: true})
				}
			}
		}
		return typesystem.Type{Kind: typesystem.Array}
	}
	if mapLit, ok := assignment.Left.(*parser.MapLiteral); ok {
		for _, valExpr := range mapLit.Pairs {
			var identifier *parser.Identifier
			targetType := typesystem.Type{Kind: typesystem.Mixed}
			if id, ok := valExpr.(*parser.Identifier); ok {
				identifier = id
				if valueType.Element != nil {
					targetType = *valueType.Element
				}
			} else if assign, ok := valExpr.(*parser.AssignExpression); ok {
				if id, ok := assign.Left.(*parser.Identifier); ok {
					identifier = id
					defaultType := a.inferExpression(assign.Value, current)
					if valueType.Element != nil {
						targetType = *valueType.Element
					} else {
						targetType = defaultType
					}
				}
			}
			if identifier != nil {
				name := cleanName(identifier.Value)
				if existing, exists := current.resolve(name); exists {
					if existing.Inferred && !existing.Type.IsKnown() {
						existing.Type = targetType
					}
				} else {
					current.put(&symbol{Name: name, Type: targetType, Kind: symbolVariable, Token: identifier.Token, File: a.file, Inferred: true})
				}
			}
		}
		return typesystem.Type{Kind: typesystem.Map}
	}
	if member, ok := assignment.Left.(*parser.MemberExpression); ok && member.Property != nil {
		receiver := a.receiverType(member.Left, current)
		if receiver.Kind == typesystem.Class {
			if field, exists := a.lookupField(receiver.Name, member.Property.Value); exists {
				if !a.canAccess(field.Visibility, field.Owner) {
					a.accessError(member.Property.Token, field.Visibility, field.Owner, member.Property.Value)
				}
				if field.Constant {
					a.add("JOSS-SYM-006", diagnostics.SeverityError, a.file, member.Property.Token,
						fmt.Sprintf("Constant property `%s::%s` cannot be reassigned.", receiver.Name, member.Property.Value),
						"Constant properties are immutable after instance initialization.", "Create a mutable property or assign a different variable.")
					return field.Type
				}
				if field.Type.IsKnown() && !a.assignableExpression(field.Type, valueType, assignment.Value) {
					a.add("JOSS-TYPE-001", diagnostics.SeverityError, a.file, member.Property.Token,
						fmt.Sprintf("Cannot assign `%s` to property `%s::%s` of type `%s`.", valueType.String(), receiver.Name, member.Property.Value, field.Type.String()),
						"Properties keep their declared type.", "Assign a compatible value or correct the property declaration.")
				}
				return field.Type
			}
		}
		return valueType
	}
	// Member/index assignment still needs the receiver and index checked.
	a.inferExpression(assignment.Left, current)
	return valueType
}

func (a *Analyzer) inferInfix(expression *parser.InfixExpression, current *scope) typesystem.Type {
	left := a.inferExpression(expression.Left, current)
	right := a.inferExpression(expression.Right, current)
	a.checkConstantIntegerOperation(expression)
	switch expression.Operator {
	case ".":
		return typesystem.Type{Kind: typesystem.String}
	case "==", "!=", "===", "!==", "<", ">", "<=", ">=", "&&", "||":
		return typesystem.Type{Kind: typesystem.Bool}
	case "<=>":
		return typesystem.Type{Kind: typesystem.Int}
	case "??":
		return commonType(left, right)
	case "+", "-", "*", "/", "%":
		if left.IsKnown() && !left.IsNumeric() {
			a.invalidOperator(expression.Token, expression.Operator, left, right)
			return typesystem.Type{Kind: typesystem.Unknown}
		}
		if right.IsKnown() && !right.IsNumeric() {
			a.invalidOperator(expression.Token, expression.Operator, left, right)
			return typesystem.Type{Kind: typesystem.Unknown}
		}
		if left.Kind == typesystem.Decimal || right.Kind == typesystem.Decimal {
			return typesystem.Type{Kind: typesystem.Decimal}
		}
		if expression.Operator == "/" || left.Kind == typesystem.Float || right.Kind == typesystem.Float {
			return typesystem.Type{Kind: typesystem.Float}
		}
		if left.Kind == typesystem.Int && right.Kind == typesystem.Int {
			return typesystem.Type{Kind: typesystem.Int}
		}
		return typesystem.Type{Kind: typesystem.Unknown}
	case "<<", ">>", "|>":
		return typesystem.Type{Kind: typesystem.Unknown}
	default:
		return typesystem.Type{Kind: typesystem.Unknown}
	}
}

func (a *Analyzer) checkConstantIntegerOperation(expression *parser.InfixExpression) {
	left, leftOK := constantInteger(expression.Left)
	right, rightOK := constantInteger(expression.Right)
	if !leftOK || !rightOK {
		return
	}
	var fault typesystem.ArithmeticFault
	if expression.Operator == "/" {
		if right == 0 {
			fault = typesystem.ArithmeticDivisionByZero
		}
	} else {
		_, fault = typesystem.CheckedIntBinary(expression.Operator, left, right)
	}
	switch fault {
	case typesystem.ArithmeticOverflow:
		a.add(diagnostics.CodeArithmeticOverflow, diagnostics.SeverityError, a.file, expression.Token,
			fmt.Sprintf("Integer expression `%d %s %d` overflows `int`.", left, expression.Operator, right),
			"Joss integer arithmetic never wraps silently.", "Reduce the value, use a float deliberately, or handle the boundary before this operation.")
	case typesystem.ArithmeticDivisionByZero:
		a.add(diagnostics.CodeDivisionByZero, diagnostics.SeverityError, a.file, expression.Token,
			fmt.Sprintf("Integer expression `%d %s %d` divides by zero.", left, expression.Operator, right),
			"Division and modulo by zero are arithmetic errors.", "Ensure the divisor is non-zero before evaluating this expression.")
	}
}

func constantInteger(expression parser.Expression) (int64, bool) {
	switch node := expression.(type) {
	case *parser.IntegerLiteral:
		return node.Value, true
	case *parser.PrefixExpression:
		if node.Operator != "-" {
			return 0, false
		}
		value, ok := constantInteger(node.Right)
		if !ok {
			return 0, false
		}
		negated, fault := typesystem.CheckedIntNegate(value)
		return negated, fault == typesystem.ArithmeticOK
	default:
		return 0, false
	}
}

func (a *Analyzer) invalidOperator(token parser.Token, operator string, left, right typesystem.Type) {
	message := fmt.Sprintf("Operator `%s` is not defined for `%s`.", operator, left.String())
	if right.Kind != "" {
		message = fmt.Sprintf("Operator `%s` is not defined for `%s` and `%s`.", operator, left.String(), right.String())
	}
	a.add("JOSS-TYPE-004", diagnostics.SeverityError, a.file, token, message,
		"The operand types are known and incompatible with this operator.", "Convert the operands or use an operator defined for these types.")
}

func commonType(left, right typesystem.Type) typesystem.Type {
	if left == right {
		return left
	}
	if left.Kind == typesystem.Mixed || right.Kind == typesystem.Mixed {
		return typesystem.Type{Kind: typesystem.Mixed}
	}
	if left.Kind == typesystem.Unknown || left.Kind == "" {
		return right
	}
	if right.Kind == typesystem.Unknown || right.Kind == "" {
		return left
	}
	if left.Kind == typesystem.Null || right.Kind == typesystem.Null {
		return typesystem.Parse(left.String() + "|" + right.String())
	}
	if !left.IsKnown() {
		return right
	}
	if !right.IsKnown() {
		return left
	}
	if left.IsNumeric() && right.IsNumeric() {
		if left.Kind == typesystem.Decimal || right.Kind == typesystem.Decimal {
			return typesystem.Type{Kind: typesystem.Decimal}
		}
		return typesystem.Type{Kind: typesystem.Float}
	}
	return typesystem.Parse(left.String() + "|" + right.String())
}

func tokenOfExpression(expression parser.Expression) parser.Token {
	switch node := expression.(type) {
	case *parser.Identifier:
		return node.Token
	case *parser.StringLiteral:
		return node.Token
	case *parser.IntegerLiteral:
		return node.Token
	case *parser.FloatLiteral:
		return node.Token
	case *parser.DecimalLiteral:
		return node.Token
	case *parser.Boolean:
		return node.Token
	case *parser.NullLiteral:
		return node.Token
	case *parser.ReferenceExpression:
		return node.Token
	case *parser.ArrayLiteral:
		return node.Token
	case *parser.MapLiteral:
		return node.Token
	case *parser.AssignExpression:
		return node.Token
	case *parser.InfixExpression:
		return node.Token
	case *parser.PrefixExpression:
		return node.Token
	case *parser.PostfixExpression:
		return node.Token
	case *parser.TernaryExpression:
		return node.Token
	case *parser.IndexExpression:
		return node.Token
	case *parser.NewExpression:
		return node.Token
	case *parser.MemberExpression:
		return node.Token
	case *parser.CallExpression:
		return node.Token
	case *parser.FunctionLiteral:
		return node.Token
	case *parser.IssetExpression:
		return node.Token
	case *parser.EmptyExpression:
		return node.Token
	case *parser.BlockExpression:
		return node.Token
	case *parser.MatchExpression:
		return node.Token
	case *parser.IsExpression:
		return node.Token
	case *parser.SpreadExpression:
		return node.Token
	case *parser.YieldExpression:
		return node.Token
	default:
		return parser.Token{}
	}
}
