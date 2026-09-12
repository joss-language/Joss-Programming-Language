package analyzer

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

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
