package analyzer

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

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
	_, returnType, exists := typesystem.PrimitiveMethod(receiver, method)
	return returnType, exists
}

func (a *Analyzer) checkCall(callable Callable, arguments []parser.Expression, current *scope, token parser.Token) {
	bindings, spread := a.normalizeCallArguments(callable, arguments, current, token)
	if spread {
		return
	}
	for parameterIndex, argument := range bindings {
		if argument == nil {
			if parameterIndex < len(callable.Parameters) && !callable.Parameters[parameterIndex].HasDefault {
				a.callArityError(callable, len(arguments), token)
				return
			}
			continue
		}
		a.validateCallArgument(callable, parameterIndex, argument, current)
	}
}

func (a *Analyzer) normalizeCallArguments(callable Callable, arguments []parser.Expression, current *scope, token parser.Token) ([]parser.Expression, bool) {
	bindings := make([]parser.Expression, len(callable.Parameters))
	parameterByName := make(map[string]int, len(callable.Parameters))
	for index, parameter := range callable.Parameters {
		parameterByName[cleanName(parameter.Name)] = index
	}
	positional := 0
	spread := false
	for _, sourceArgument := range arguments {
		if expanded, ok := sourceArgument.(*parser.SpreadExpression); ok {
			spread = true
			a.inferExpression(expanded.Expression, current)
			continue
		}
		argument := sourceArgument
		parameterIndex := positional
		if named, ok := sourceArgument.(*parser.NamedArgument); ok {
			argument = named.Value
			resolved, exists := parameterByName[named.Name]
			if !exists {
				a.inferCallArgument(argument, current)
				a.add("JOSS-CALL-001", diagnostics.SeverityError, a.file, named.Token,
					fmt.Sprintf("Unknown named parameter `%s` in call to `%s`.", named.Name, callable.Name),
					"Named arguments must match parameter names.", "Check parameter name.")
				continue
			}
			parameterIndex = resolved
		} else {
			positional++
		}
		if parameterIndex >= len(bindings) {
			a.inferCallArgument(argument, current)
			if reference, ok := argument.(*parser.ReferenceExpression); ok {
				a.add("JOSS-REF-001", diagnostics.SeverityError, a.file, tokenOfExpression(reference),
					fmt.Sprintf("Argument %d to `%s` is `ref`, but no reference parameter is declared.", parameterIndex+1, callable.Name),
					"Variadic or unpublished native arguments never imply mutable reference semantics.", "Remove `ref` or call a Joss function with an explicit matching ref parameter.")
			}
			continue
		}
		if bindings[parameterIndex] != nil {
			a.inferCallArgument(argument, current)
			a.add("JOSS-CALL-001", diagnostics.SeverityError, a.file, tokenOfExpression(argument),
				fmt.Sprintf("Parameter `$%s` is provided more than once in call to `%s`.", cleanName(callable.Parameters[parameterIndex].Name), callable.Name),
				"A positional and named argument cannot bind the same parameter.", "Remove the duplicate argument.")
			continue
		}
		bindings[parameterIndex] = argument
	}
	if !spread && !callable.Variadic && positional > len(callable.Parameters) {
		a.callArityError(callable, len(arguments), token)
	}
	return bindings, spread
}

func (a *Analyzer) inferCallArgument(argument parser.Expression, current *scope) typesystem.Type {
	if reference, ok := argument.(*parser.ReferenceExpression); ok {
		return a.inferExpression(reference.Target, current)
	}
	return a.inferExpression(argument, current)
}

func (a *Analyzer) validateCallArgument(callable Callable, index int, argument parser.Expression, current *scope) {
	parameter := callable.Parameters[index]
	argumentType := a.inferCallArgument(argument, current)
	reference, argumentIsReference := argument.(*parser.ReferenceExpression)
	if parameter.ByReference != argumentIsReference {
		message := fmt.Sprintf("Argument %d to `%s` is marked `ref`, but the parameter is passed by value.", index+1, callable.Name)
		suggestion := "Remove `ref` or update the Joss function signature."
		if parameter.ByReference {
			message = fmt.Sprintf("Argument %d to `%s` must be passed with `ref`.", index+1, callable.Name)
			suggestion = "Pass a mutable variable as `ref $variable`."
		}
		a.add("JOSS-REF-001", diagnostics.SeverityError, a.file, tokenOfExpression(argument), message,
			"Mutable references require an explicit matching contract at both call and declaration.", suggestion)
		return
	}
	if parameter.ByReference {
		identifier, valid := reference.Target.(*parser.Identifier)
		if !valid {
			a.add("JOSS-REF-002", diagnostics.SeverityError, a.file, tokenOfExpression(reference.Target), "A mutable reference must target a variable.", "Literals, temporaries, function results, fields and indexes do not have a stable call-scoped binding yet.", "Assign the value to a local variable and pass `ref $variable`.")
			return
		}
		name := cleanName(identifier.Value)
		symbol, exists := current.resolve(name)
		if !exists {
			return
		}
		if symbol.Constant {
			a.add("JOSS-REF-003", diagnostics.SeverityError, a.file, identifier.Token, fmt.Sprintf("Constant `$%s` cannot be passed as a mutable reference.", name), "A ref parameter can assign through the caller binding.", "Pass a mutable variable instead.")
			return
		}
		if parameter.Type != symbol.Type {
			a.add("JOSS-REF-004", diagnostics.SeverityError, a.file, identifier.Token, fmt.Sprintf("Cannot pass `$%s` of type `%s` as `ref %s`.", name, symbol.Type.String(), parameter.Type.String()), "Mutable references require an exact invariant type match.", "Use a variable with exactly the declared parameter type.")
		}
		return
	}
	if !a.assignableExpression(parameter.Type, argumentType, argument) {
		a.add("JOSS-TYPE-003", diagnostics.SeverityError, a.file, tokenOfExpression(argument), fmt.Sprintf("Argument %d to `%s` has type `%s`; parameter `$%s` requires `%s`.", index+1, callable.Name, argumentType.String(), parameter.Name, parameter.Type.String()), "Function arguments follow the same assignment compatibility rules as variables.", "Convert the argument or correct the parameter type.")
	}
}

func (a *Analyzer) callArityError(callable Callable, got int, token parser.Token) {
	minimum := 0
	for _, parameter := range callable.Parameters {
		if !parameter.HasDefault {
			minimum++
		}
	}
	expected := fmt.Sprintf("%d", len(callable.Parameters))
	if minimum != len(callable.Parameters) {
		expected = fmt.Sprintf("%d..%d", minimum, len(callable.Parameters))
	}
	if callable.Variadic {
		expected = fmt.Sprintf("at least %d", minimum)
	}
	a.add("JOSS-CALL-001", diagnostics.SeverityError, a.file, token, fmt.Sprintf("Call to `%s` expects %s argument(s), got %d.", callable.Name, expected, got), "The callable signature is known at analysis time.", "Pass the required arguments or update the function signature.")
}
