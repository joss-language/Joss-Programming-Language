package analyzer

import (
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

// assignableExpression augments canonical structural assignability with the
// project-specific nominal graph and literal coercion. Runtime execution does
// not depend on this analyzer-owned graph.
func (a *Analyzer) assignableExpression(destination, source typesystem.Type, expression parser.Expression) bool {
	if typesystem.Assignable(destination, source) {
		return true
	}
	if destination.Kind == typesystem.Class && source.Kind == typesystem.Class {
		if a.classImplements(source.Name, destination.Name) || a.isSubclass(source.Name, destination.Name) {
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

func (a *Analyzer) classImplements(className, interfaceName string) bool {
	visited := map[string]bool{}
	for className != "" && !visited[className] {
		visited[className] = true
		class, exists := a.classes[className]
		if !exists {
			return false
		}
		for _, interfaceNameImplemented := range class.Interfaces {
			if a.interfaceInherits(interfaceNameImplemented, interfaceName, map[string]bool{}) {
				return true
			}
		}
		className = class.SuperClass
	}
	return false
}

func (a *Analyzer) interfaceInherits(currentInterface, targetInterface string, visited map[string]bool) bool {
	if currentInterface == targetInterface {
		return true
	}
	if visited[currentInterface] {
		return false
	}
	visited[currentInterface] = true
	interfaceDefinition, exists := a.interfaces[currentInterface]
	if !exists {
		return false
	}
	for _, parent := range interfaceDefinition.Extends {
		if a.interfaceInherits(parent, targetInterface, visited) {
			return true
		}
	}
	return false
}

func (a *Analyzer) isSubclass(subclass, baseClass string) bool {
	visited := map[string]bool{}
	for subclass != "" && !visited[subclass] {
		visited[subclass] = true
		class, exists := a.classes[subclass]
		if !exists {
			return false
		}
		if class.SuperClass == baseClass {
			return true
		}
		subclass = class.SuperClass
	}
	return false
}
