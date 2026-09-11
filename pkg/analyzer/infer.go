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
		if node.Operator == "-" && valueType.IsKnown() && !isNumeric(valueType) {
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
		if valueType.IsKnown() && !isNumeric(valueType) {
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
		if left.IsKnown() && !isNumeric(left) {
			a.invalidOperator(expression.Token, expression.Operator, left, right)
			return typesystem.Type{Kind: typesystem.Unknown}
		}
		if right.IsKnown() && !isNumeric(right) {
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

func (a *Analyzer) inferNew(expression *parser.NewExpression, current *scope) typesystem.Type {
	for _, argument := range expression.Arguments {
		a.inferExpression(argument, current)
	}
	if expression.Class == nil {
		return typesystem.Type{Kind: typesystem.Unknown}
	}
	className := expression.Class.Value
	class, exists := a.classes[className]
	if !exists {
		a.add("JOSS-SYM-004", diagnostics.SeverityError, a.file, expression.Class.Token,
			fmt.Sprintf("Class `%s` does not exist.", className),
			"The class was not found in project sources, native classes or loaded plugin symbols.", "Check the class name and plugin configuration.")
		return typesystem.Type{Kind: typesystem.Unknown}
	}
	if class.Visibility == "private" && class.File != "" && class.File != a.file {
		a.add("JOSS-ACCESS-001", diagnostics.SeverityError, a.file, expression.Class.Token,
			fmt.Sprintf("Class `%s` is private to `%s`.", className, class.File),
			"Private project declarations are visible only in their source file.", "Use a public class or instantiate it from its declaring file.")
	}
	if class.IsAbstract {
		a.add("JOSS-DECL-006", diagnostics.SeverityError, a.file, expression.Class.Token,
			fmt.Sprintf("Cannot instantiate abstract class `%s`.", className),
			"Abstract classes cannot be directly instantiated.", "Instantiate a concrete subclass that implements all abstract methods.")
		return typesystem.Type{Kind: typesystem.Unknown}
	}
	if constructor, ok := class.Methods["constructor"]; ok {
		a.checkCall(constructor, expression.Arguments, current, expression.Class.Token)
	}
	return typesystem.Type{Kind: typesystem.Class, Name: className}
}

func (a *Analyzer) inferMember(expression *parser.MemberExpression, current *scope) typesystem.Type {
	if expression == nil {
		return typesystem.Type{Kind: typesystem.Unknown}
	}
	receiver := a.receiverType(expression.Left, current)
	if receiver.Kind == typesystem.Class && expression.Property != nil {
		if enumDef, isEnum := a.enums[receiver.Name]; isEnum {
			if caseType, ok := enumDef.Cases[expression.Property.Value]; ok {
				return caseType
			}
		}
		if field, exists := a.lookupField(receiver.Name, expression.Property.Value); exists {
			if !a.canAccess(field.Visibility, field.Owner) {
				a.accessError(expression.Property.Token, field.Visibility, field.Owner, expression.Property.Value)
			}
			return field.Type
		}
	}
	return typesystem.Type{Kind: typesystem.Unknown}
}

func (a *Analyzer) inferCall(call *parser.CallExpression, current *scope) typesystem.Type {
	if identifier, ok := call.Function.(*parser.Identifier); ok {
		name := identifier.Value
		if builtin, exists := a.environment.Builtins[name]; exists {
			a.checkCall(builtin, call.Arguments, current, identifier.Token)
			return builtin.ReturnType
		}
		if function, exists := a.functions[name]; exists {
			if function.callable.Visibility == "private" && function.file != a.file {
				a.add("JOSS-ACCESS-001", diagnostics.SeverityError, a.file, identifier.Token,
					fmt.Sprintf("Function `%s` is private to `%s`.", name, function.file),
					"Private project declarations are visible only in their source file.", "Make the function public or call it from its declaring file.")
			}
			a.checkCall(function.callable, call.Arguments, current, identifier.Token)
			return function.callable.ReturnType
		}
		if variable, exists := current.resolve(name); exists {
			variable.Used = true
			for _, argument := range call.Arguments {
				a.inferExpression(argument, current)
			}
			return typesystem.Type{Kind: typesystem.Unknown}
		}
		for _, argument := range call.Arguments {
			a.inferExpression(argument, current)
		}
		a.add("JOSS-SYM-003", diagnostics.SeverityError, a.file, identifier.Token,
			fmt.Sprintf("Function `%s` does not exist.", name),
			"No project function, runtime builtin or callable variable has this name.", "Declare the function or check its spelling.")
		return typesystem.Type{Kind: typesystem.Unknown}
	}
	if member, ok := call.Function.(*parser.MemberExpression); ok {
		receiver := a.receiverType(member.Left, current)
		if receiver.Kind == typesystem.Class && member.Property != nil {
			if _, isEnum := a.enums[receiver.Name]; isEnum {
				switch member.Property.Value {
				case "cases":
					for _, argument := range call.Arguments {
						a.inferExpression(argument, current)
					}
					elem := typesystem.Type{Kind: typesystem.Class, Name: receiver.Name}
					return typesystem.Type{Kind: typesystem.Array, Element: &elem}
				case "from", "tryFrom":
					for _, argument := range call.Arguments {
						a.inferExpression(argument, current)
					}
					return typesystem.Type{Kind: typesystem.Class, Name: receiver.Name}
				}
			}
			if callable, exists := a.lookupMethod(receiver.Name, member.Property.Value); exists {
				if !a.canAccess(callable.Visibility, callable.Owner) {
					a.accessError(member.Property.Token, callable.Visibility, callable.Owner, member.Property.Value)
				}
				a.checkCall(callable, call.Arguments, current, member.Property.Token)
				return callable.ReturnType
			}
			a.add("JOSS-MEMBER-001", diagnostics.SeverityError, a.file, member.Property.Token,
				fmt.Sprintf("Class `%s` has no method `%s`.", receiver.Name, member.Property.Value),
				"The receiver class is known and its method table has been resolved.", "Check the method name or the class API.")
		}
		if member.Property != nil {
			if primType, isPrim := a.inferPrimitiveMethod(receiver, member.Property.Value, call, current); isPrim {
				return primType
			}
		}
		for _, argument := range call.Arguments {
			a.inferExpression(argument, current)
		}
		return typesystem.Type{Kind: typesystem.Unknown}
	}
	functionType := a.inferExpression(call.Function, current)
	for _, argument := range call.Arguments {
		a.inferExpression(argument, current)
	}
	return functionType
}

func (a *Analyzer) inferPrimitiveMethod(receiver typesystem.Type, method string, call *parser.CallExpression, current *scope) (typesystem.Type, bool) {
	for _, argument := range call.Arguments {
		a.inferExpression(argument, current)
	}
	switch receiver.Kind {
	case typesystem.String:
		switch method {
		case "trim", "lower", "upper", "replace", "substring", "substr", "repeat":
			return typesystem.Type{Kind: typesystem.String}, true
		case "length", "indexOf":
			return typesystem.Type{Kind: typesystem.Int}, true
		case "contains", "startsWith", "endsWith":
			return typesystem.Type{Kind: typesystem.Bool}, true
		case "split", "lines":
			strType := typesystem.Type{Kind: typesystem.String}
			return typesystem.Type{Kind: typesystem.Array, Element: &strType}, true
		}
	case typesystem.Array:
		switch method {
		case "length", "count", "indexOf":
			return typesystem.Type{Kind: typesystem.Int}, true
		case "join":
			return typesystem.Type{Kind: typesystem.String}, true
		case "contains", "has":
			return typesystem.Type{Kind: typesystem.Bool}, true
		case "slice", "reverse", "push", "filter":
			return receiver, true
		case "map":
			return typesystem.Type{Kind: typesystem.Array}, true
		case "first", "last", "pop":
			if receiver.Element != nil {
				return *receiver.Element, true
			}
			return typesystem.Type{Kind: typesystem.Unknown}, true
		case "reduce":
			return typesystem.Type{Kind: typesystem.Unknown}, true
		}
	case typesystem.Map:
		switch method {
		case "keys":
			strType := typesystem.Type{Kind: typesystem.String}
			return typesystem.Type{Kind: typesystem.Array, Element: &strType}, true
		case "values":
			return typesystem.Type{Kind: typesystem.Array}, true
		case "has", "contains":
			return typesystem.Type{Kind: typesystem.Bool}, true
		case "length", "count":
			return typesystem.Type{Kind: typesystem.Int}, true
		case "get":
			if receiver.Element != nil {
				return *receiver.Element, true
			}
			return typesystem.Type{Kind: typesystem.Unknown}, true
		case "set", "remove", "merge":
			return receiver, true
		}
	}
	return typesystem.Type{}, false
}

func (a *Analyzer) canAccess(visibility, owner string) bool {
	if visibility == "" || visibility == "public" || owner == "" {
		return true
	}
	if a.currentClass == owner {
		return true
	}
	if visibility == "private" || a.currentClass == "" {
		return false
	}
	for className := a.currentClass; className != ""; {
		class, exists := a.classes[className]
		if !exists || class.SuperClass == "" {
			return false
		}
		if class.SuperClass == owner {
			return true
		}
		className = class.SuperClass
	}
	return false
}

func (a *Analyzer) accessError(token parser.Token, visibility, owner, member string) {
	a.add("JOSS-ACCESS-002", diagnostics.SeverityError, a.file, token,
		fmt.Sprintf("Member `%s::%s` is `%s` and is not accessible here.", owner, member, visibility),
		"Private members are limited to their class; protected members also allow subclasses.", "Expose a public method or move the access into an allowed class.")
}

func (a *Analyzer) receiverType(expression parser.Expression, current *scope) typesystem.Type {
	if identifier, ok := expression.(*parser.Identifier); ok {
		// A local may intentionally have the same spelling as its class (a common
		// model pattern). Lexical symbols shadow class names for instance access.
		if value, exists := current.resolve(identifier.Value); exists {
			value.Used = true
			return value.Type
		}
		if _, exists := a.classes[identifier.Value]; exists {
			return typesystem.Type{Kind: typesystem.Class, Name: identifier.Value}
		}
		if _, exists := a.interfaces[identifier.Value]; exists {
			return typesystem.Type{Kind: typesystem.Class, Name: identifier.Value}
		}
		if _, exists := a.enums[identifier.Value]; exists {
			return typesystem.Type{Kind: typesystem.Class, Name: identifier.Value}
		}
	}
	return a.inferExpression(expression, current)
}

func (a *Analyzer) lookupMethod(className, methodName string) (Callable, bool) {
	if iface, ok := a.interfaces[className]; ok {
		return a.lookupInterfaceMethod(iface, methodName, map[string]bool{})
	}
	visited := map[string]bool{}
	for className != "" && !visited[className] {
		visited[className] = true
		class, exists := a.classes[className]
		if !exists {
			return Callable{}, false
		}
		if method, exists := class.Methods[methodName]; exists {
			return method, true
		}
		className = class.SuperClass
	}
	return Callable{}, false
}

func (a *Analyzer) lookupInterfaceMethod(iface Interface, methodName string, visited map[string]bool) (Callable, bool) {
	if visited[iface.Name] {
		return Callable{}, false
	}
	visited[iface.Name] = true
	if method, exists := iface.Methods[methodName]; exists {
		return method, true
	}
	for _, extName := range iface.Extends {
		if parentIface, exists := a.interfaces[extName]; exists {
			if m, ok := a.lookupInterfaceMethod(parentIface, methodName, visited); ok {
				return m, true
			}
		}
	}
	return Callable{}, false
}

func (a *Analyzer) lookupField(className, fieldName string) (Field, bool) {
	visited := map[string]bool{}
	for className != "" && !visited[className] {
		visited[className] = true
		class, exists := a.classes[className]
		if !exists {
			return Field{}, false
		}
		if field, exists := class.Fields[fieldName]; exists {
			return field, true
		}
		className = class.SuperClass
	}
	return Field{}, false
}

func (a *Analyzer) checkCall(callable Callable, arguments []parser.Expression, current *scope, token parser.Token) {
	argumentTypes := make([]typesystem.Type, len(arguments))
	for index, argument := range arguments {
		if reference, ok := argument.(*parser.ReferenceExpression); ok {
			argumentTypes[index] = a.inferExpression(reference.Target, current)
		} else {
			argumentTypes[index] = a.inferExpression(argument, current)
		}
	}
	minimum := 0
	for _, parameter := range callable.Parameters {
		if !parameter.HasDefault {
			minimum++
		}
	}
	hasSpread := false
	hasNamed := false
	namedMap := make(map[string]*parser.NamedArgument)
	for _, argument := range arguments {
		if _, ok := argument.(*parser.SpreadExpression); ok {
			hasSpread = true
		} else if named, ok := argument.(*parser.NamedArgument); ok {
			hasNamed = true
			namedMap[named.Name] = named
		}
	}
	if !hasSpread && !hasNamed {
		if len(arguments) < minimum || (!callable.Variadic && len(arguments) > len(callable.Parameters)) {
			expected := fmt.Sprintf("%d", len(callable.Parameters))
			if minimum != len(callable.Parameters) {
				expected = fmt.Sprintf("%d..%d", minimum, len(callable.Parameters))
			}
			if callable.Variadic {
				expected = fmt.Sprintf("at least %d", minimum)
			}
			a.add("JOSS-CALL-001", diagnostics.SeverityError, a.file, token,
				fmt.Sprintf("Call to `%s` expects %s argument(s), got %d.", callable.Name, expected, len(arguments)),
				"The callable signature is known at analysis time.", "Pass the required arguments or update the function signature.")
		}
	}
	if hasNamed {
		for name, namedArg := range namedMap {
			matched := false
			for _, param := range callable.Parameters {
				if cleanName(param.Name) == name {
					matched = true
					argType := a.inferExpression(namedArg.Value, current)
					if !a.assignableExpression(param.Type, argType, namedArg.Value) {
						a.add("JOSS-TYPE-003", diagnostics.SeverityError, a.file, namedArg.Token,
							fmt.Sprintf("Argument `%s` to `%s` has type `%s`; parameter `$%s` requires `%s`.", name, callable.Name, argType.String(), param.Name, param.Type.String()),
							"Function arguments follow the same assignment compatibility rules as variables.", "Convert the argument or correct the parameter type.")
					}
					break
				}
			}
			if !matched && len(callable.Parameters) > 0 {
				a.add("JOSS-CALL-001", diagnostics.SeverityError, a.file, namedArg.Token,
					fmt.Sprintf("Unknown named parameter `%s` in call to `%s`.", name, callable.Name),
					"Named arguments must match parameter names.", "Check parameter name.")
			}
		}
	}
	if !hasNamed && !hasSpread {
		for index := 0; index < len(arguments) && index < len(callable.Parameters); index++ {
			parameter := callable.Parameters[index]
			reference, argumentIsReference := arguments[index].(*parser.ReferenceExpression)
			if parameter.ByReference != argumentIsReference {
				if parameter.ByReference {
					a.add("JOSS-REF-001", diagnostics.SeverityError, a.file, tokenOfExpression(arguments[index]),
						fmt.Sprintf("Argument %d to `%s` must be passed with `ref`.", index+1, callable.Name),
						"Mutable reference parameters require explicit mutation at the call site.", "Pass a mutable variable as `ref $variable`.")
				} else {
					a.add("JOSS-REF-001", diagnostics.SeverityError, a.file, tokenOfExpression(arguments[index]),
						fmt.Sprintf("Argument %d to `%s` is marked `ref`, but the parameter is passed by value.", index+1, callable.Name),
						"References are accepted only by parameters declared with `ref`.", "Remove `ref` or update the Joss function signature.")
				}
				continue
			}
			argumentExpression := arguments[index]
			if parameter.ByReference {
				identifier, valid := reference.Target.(*parser.Identifier)
				if !valid {
					a.add("JOSS-REF-002", diagnostics.SeverityError, a.file, tokenOfExpression(reference.Target),
						"A mutable reference must target a variable.",
						"Literals, temporaries, function results, fields and indexes do not have a stable call-scoped binding yet.", "Assign the value to a local variable and pass `ref $variable`.")
					continue
				}
				name := cleanName(identifier.Value)
				symbol, exists := current.resolve(name)
				if !exists {
					// inferExpression already emitted the undefined-symbol diagnostic.
					continue
				}
				if symbol.Constant {
					a.add("JOSS-REF-003", diagnostics.SeverityError, a.file, identifier.Token,
						fmt.Sprintf("Constant `$%s` cannot be passed as a mutable reference.", name),
						"A ref parameter can assign through the caller binding.", "Pass a mutable variable instead.")
					continue
				}
				if parameter.Type != symbol.Type {
					a.add("JOSS-REF-004", diagnostics.SeverityError, a.file, identifier.Token,
						fmt.Sprintf("Cannot pass `$%s` of type `%s` as `ref %s`.", name, symbol.Type.String(), parameter.Type.String()),
						"Mutable references require an exact invariant type match.", "Use a variable with exactly the declared parameter type.")
				}
				continue
			}
			if !a.assignableExpression(parameter.Type, argumentTypes[index], argumentExpression) {
				a.add("JOSS-TYPE-003", diagnostics.SeverityError, a.file, tokenOfExpression(arguments[index]),
					fmt.Sprintf("Argument %d to `%s` has type `%s`; parameter `$%s` requires `%s`.", index+1, callable.Name, argumentTypes[index].String(), parameter.Name, parameter.Type.String()),
					"Function arguments follow the same assignment compatibility rules as variables.", "Convert the argument or correct the parameter type.")
			}
		}
		for index := len(callable.Parameters); index < len(arguments); index++ {
			if _, isReference := arguments[index].(*parser.ReferenceExpression); isReference {
				a.add("JOSS-REF-001", diagnostics.SeverityError, a.file, tokenOfExpression(arguments[index]),
					fmt.Sprintf("Argument %d to `%s` is `ref`, but no reference parameter is declared.", index+1, callable.Name),
					"Variadic or unpublished native arguments never imply mutable reference semantics.", "Remove `ref` or call a Joss function with an explicit matching ref parameter.")
			}
		}
	}
}

func isNumeric(valueType typesystem.Type) bool {
	return valueType.Kind == typesystem.Int || valueType.Kind == typesystem.Float || valueType.Kind == typesystem.Decimal
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
	if isNumeric(left) && isNumeric(right) {
		if left.Kind == typesystem.Decimal || right.Kind == typesystem.Decimal {
			return typesystem.Type{Kind: typesystem.Decimal}
		}
		return typesystem.Type{Kind: typesystem.Float}
	}
	return typesystem.Parse(left.String() + "|" + right.String())
}

func (a *Analyzer) classImplements(className, interfaceName string) bool {
	visited := map[string]bool{}
	for className != "" && !visited[className] {
		visited[className] = true
		class, exists := a.classes[className]
		if !exists {
			return false
		}
		for _, iface := range class.Interfaces {
			if a.interfaceInherits(iface, interfaceName, map[string]bool{}) {
				return true
			}
		}
		className = class.SuperClass
	}
	return false
}

func (a *Analyzer) interfaceInherits(currentIface, targetIface string, visited map[string]bool) bool {
	if currentIface == targetIface {
		return true
	}
	if visited[currentIface] {
		return false
	}
	visited[currentIface] = true
	iface, exists := a.interfaces[currentIface]
	if !exists {
		return false
	}
	for _, ext := range iface.Extends {
		if a.interfaceInherits(ext, targetIface, visited) {
			return true
		}
	}
	return false
}

func (a *Analyzer) isSubclass(sub, base string) bool {
	visited := map[string]bool{}
	for sub != "" && !visited[sub] {
		visited[sub] = true
		c, ok := a.classes[sub]
		if !ok {
			return false
		}
		if c.SuperClass == base {
			return true
		}
		sub = c.SuperClass
	}
	return false
}

func (a *Analyzer) assignableExpression(destination, source typesystem.Type, expression parser.Expression) bool {
	if typesystem.Assignable(destination, source) {
		return true
	}
	if destination.Kind == typesystem.Class && source.Kind == typesystem.Class {
		if a.classImplements(source.Name, destination.Name) {
			return true
		}
		if a.isSubclass(source.Name, destination.Name) {
			return true
		}
	}
	if destination.Kind == typesystem.Union && source.Kind == typesystem.Class {
		for _, member := range destination.Members() {
			if member.Kind == typesystem.Class && (a.classImplements(source.Name, member.Name) || a.isSubclass(source.Name, member.Name)) {
				return true
			}
		}
	}
	if literal, ok := expression.(*parser.StringLiteral); ok {
		_, coerced := typesystem.CoerceString(destination, literal.Value)
		return coerced
	}
	return false
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

func (a *Analyzer) narrowScopeFromCondition(condition parser.Expression, current *scope) (*scope, *scope) {
	trueScope := newScope(current)
	falseScope := newScope(current)
	if isExpr, ok := condition.(*parser.IsExpression); ok {
		if ident, ok := isExpr.Left.(*parser.Identifier); ok {
			if sym, ok := current.resolve(ident.Value); ok {
				targetType := typeFromToken(isExpr.TargetType)
				trueScope.put(&symbol{
					Name: sym.Name, Type: targetType, Kind: sym.Kind, Token: sym.Token, File: sym.File, Dynamic: sym.Dynamic, Inferred: sym.Inferred, Constant: sym.Constant, Synthetic: true,
				})
			}
		}
	}
	if infix, ok := condition.(*parser.InfixExpression); ok {
		var ident *parser.Identifier
		var isNullCheck bool
		if id, ok := infix.Left.(*parser.Identifier); ok && isNullLiteral(infix.Right) {
			ident = id
			isNullCheck = true
		} else if id, ok := infix.Right.(*parser.Identifier); ok && isNullLiteral(infix.Left) {
			ident = id
			isNullCheck = true
		}
		if isNullCheck && ident != nil {
			if sym, ok := current.resolve(ident.Value); ok && sym.Type.Kind == typesystem.Union {
				if infix.Operator == "!=" || infix.Operator == "!==" {
					narrowed := sym.Type.Without(typesystem.Null)
					trueScope.put(&symbol{
						Name: sym.Name, Type: narrowed, Kind: sym.Kind, Token: sym.Token, File: sym.File, Dynamic: sym.Dynamic, Inferred: sym.Inferred, Constant: sym.Constant, Synthetic: true,
					})
					falseScope.put(&symbol{
						Name: sym.Name, Type: typesystem.Type{Kind: typesystem.Null}, Kind: sym.Kind, Token: sym.Token, File: sym.File, Dynamic: sym.Dynamic, Inferred: sym.Inferred, Constant: sym.Constant, Synthetic: true,
					})
				} else if infix.Operator == "==" || infix.Operator == "===" {
					narrowed := sym.Type.Without(typesystem.Null)
					falseScope.put(&symbol{
						Name: sym.Name, Type: narrowed, Kind: sym.Kind, Token: sym.Token, File: sym.File, Dynamic: sym.Dynamic, Inferred: sym.Inferred, Constant: sym.Constant, Synthetic: true,
					})
					trueScope.put(&symbol{
						Name: sym.Name, Type: typesystem.Type{Kind: typesystem.Null}, Kind: sym.Kind, Token: sym.Token, File: sym.File, Dynamic: sym.Dynamic, Inferred: sym.Inferred, Constant: sym.Constant, Synthetic: true,
					})
				}
			}
		}
	}
	return trueScope, falseScope
}

func isNullLiteral(expr parser.Expression) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *parser.NullLiteral:
		return true
	case *parser.Identifier:
		return e.Value == "null" || e.Value == "nil"
	}
	return false
}
