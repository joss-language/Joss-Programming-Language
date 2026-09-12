package analyzer

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

type ifaceMethodRequirement struct {
	method    Callable
	ifaceName string
}

func (a *Analyzer) collectInterfaceMethods(interfaceName string, visited map[string]bool) []ifaceMethodRequirement {
	if visited[interfaceName] {
		return nil
	}
	visited[interfaceName] = true
	iface, exists := a.interfaces[interfaceName]
	if !exists {
		return nil
	}
	var reqs []ifaceMethodRequirement
	for _, method := range iface.Methods {
		reqs = append(reqs, ifaceMethodRequirement{method: method, ifaceName: interfaceName})
	}
	for _, ext := range iface.Extends {
		reqs = append(reqs, a.collectInterfaceMethods(ext, visited)...)
	}
	return reqs
}

type abstractMethodRequirement struct {
	method    Callable
	className string
}

func (a *Analyzer) collectAbstractMethods(className string, visited map[string]bool) []abstractMethodRequirement {
	if visited[className] {
		return nil
	}
	visited[className] = true
	class, exists := a.classes[className]
	if !exists {
		return nil
	}
	var reqs []abstractMethodRequirement
	for _, method := range class.Methods {
		if method.IsAbstract {
			reqs = append(reqs, abstractMethodRequirement{method: method, className: className})
		}
	}
	if class.SuperClass != "" {
		reqs = append(reqs, a.collectAbstractMethods(class.SuperClass, visited)...)
	}
	return reqs
}

func (a *Analyzer) analyzeInterface(ifaceNode *parser.InterfaceStatement) {
	if ifaceNode == nil || ifaceNode.Name == nil {
		return
	}
	name := ifaceNode.Name.Value
	visited := map[string]bool{name: true}
	for _, ext := range ifaceNode.Extends {
		if ext == nil {
			continue
		}
		if _, exists := a.interfaces[ext.Value]; !exists {
			a.add("JOSS-SYM-007", diagnostics.SeverityError, a.file, ext.Token,
				fmt.Sprintf("Extended interface `%s` does not exist.", ext.Value),
				"An interface can only extend existing interfaces.",
				"Check the interface name or declare it.")
			continue
		}
		if a.hasInterfaceCycle(ext.Value, visited) {
			a.add("JOSS-SYM-008", diagnostics.SeverityError, a.file, ext.Token,
				fmt.Sprintf("Cyclic inheritance detected in interface `%s` via `%s`.", name, ext.Value),
				"Interface inheritance hierarchy cannot form cycles.",
				"Remove the circular inheritance reference.")
		}
	}
	for _, method := range ifaceNode.Methods {
		if method == nil {
			continue
		}
		returnType := typeFromToken(method.ReturnType)
		a.validateDeclaredType(returnType, method.ReturnType, "return annotation")
		for _, param := range method.Parameters {
			if param == nil || param.Name == nil {
				continue
			}
			if param.Type.Literal == "" || param.Type.Type == parser.VAR {
				a.add("JOSS-TYPE-011", diagnostics.SeverityError, a.file, param.Name.Token,
					fmt.Sprintf("Parameter `$%s` requires an explicit type.", param.Name.Value),
					"Joss does not create implicit `mixed` parameters.",
					"Declare a concrete/class/union type, or write `mixed` explicitly.")
			} else {
				pType := typesystem.Parse(param.Type.Literal)
				a.validateDeclaredType(pType, param.Type, "parameter type")
			}
		}
	}
}

func (a *Analyzer) hasInterfaceCycle(current string, visited map[string]bool) bool {
	if visited[current] {
		return true
	}
	visited[current] = true
	defer func() { delete(visited, current) }()
	iface, exists := a.interfaces[current]
	if !exists {
		return false
	}
	for _, ext := range iface.Extends {
		if a.hasInterfaceCycle(ext, visited) {
			return true
		}
	}
	return false
}

func (a *Analyzer) validateClassContract(classNode *parser.ClassStatement, global *scope) {
	if classNode == nil || classNode.Name == nil {
		return
	}
	previousClass := a.currentClass
	a.currentClass = classNode.Name.Value
	defer func() { a.currentClass = previousClass }()
	if classNode.SuperClass != nil {
		if _, exists := a.classes[classNode.SuperClass.Value]; !exists {
			a.add("JOSS-SYM-005", diagnostics.SeverityError, a.file, classNode.SuperClass.Token,
				fmt.Sprintf("Base class `%s` does not exist.", classNode.SuperClass.Value),
				"Inheritance requires a class known to the project, runtime or loaded plugins.", "Check the class name or plugin configuration.")
		}
	}
	for _, ifaceNode := range classNode.Interfaces {
		if ifaceNode == nil {
			continue
		}
		if _, exists := a.interfaces[ifaceNode.Value]; !exists {
			a.add("JOSS-SYM-007", diagnostics.SeverityError, a.file, ifaceNode.Token,
				fmt.Sprintf("Interface `%s` does not exist.", ifaceNode.Value),
				"Classes can only implement declared interfaces.",
				"Check the interface name or declare the interface.")
		}
	}
	checkedMethods := map[string]bool{}
	for _, ifaceNode := range classNode.Interfaces {
		if ifaceNode == nil {
			continue
		}
		if _, exists := a.interfaces[ifaceNode.Value]; !exists {
			continue
		}
		reqs := a.collectInterfaceMethods(ifaceNode.Value, map[string]bool{})
		for _, req := range reqs {
			if checkedMethods[req.method.Name] {
				continue
			}
			checkedMethods[req.method.Name] = true
			classMethod, exists := a.lookupMethod(classNode.Name.Value, req.method.Name)
			if !exists {
				a.add("JOSS-DECL-005", diagnostics.SeverityError, a.file, classNode.Name.Token,
					fmt.Sprintf("Class `%s` does not implement method `%s` required by interface `%s`.",
						classNode.Name.Value, req.method.Name, req.ifaceName),
					"A class implementing an interface must implement all of its methods.",
					fmt.Sprintf("Implement `public func %s(...)` in class `%s`.", req.method.Name, classNode.Name.Value))
				continue
			}
			if classMethod.Visibility != "" && classMethod.Visibility != "public" {
				a.add("JOSS-DECL-005", diagnostics.SeverityError, a.file, classNode.Name.Token,
					fmt.Sprintf("Class `%s` implements interface method `%s` with visibility `%s`; must be `public`.",
						classNode.Name.Value, req.method.Name, classMethod.Visibility),
					"Interface methods define public contracts and must be implemented as public.",
					"Change method visibility to `public`.")
			}
			if len(classMethod.Parameters) != len(req.method.Parameters) {
				a.add("JOSS-DECL-005", diagnostics.SeverityError, a.file, classNode.Name.Token,
					fmt.Sprintf("Class `%s` method `%s` has %d parameter(s), but interface `%s` declares %d.",
						classNode.Name.Value, req.method.Name, len(classMethod.Parameters), req.ifaceName, len(req.method.Parameters)),
					"Method implementation must match the interface signature.",
					"Adjust the method parameters to match the interface contract.")
			} else {
				for i, ifaceParam := range req.method.Parameters {
					clsParam := classMethod.Parameters[i]
					if clsParam.Type != ifaceParam.Type {
						a.add("JOSS-DECL-005", diagnostics.SeverityError, a.file, classNode.Name.Token,
							fmt.Sprintf("Class `%s` method `%s` parameter `$%s` has type `%s`, but interface `%s` declares type `%s`.",
								classNode.Name.Value, req.method.Name, clsParam.Name, clsParam.Type.String(), req.ifaceName, ifaceParam.Type.String()),
							"Method parameter types must match the interface definition.",
							"Change parameter type to match the interface.")
						break
					}
				}
			}
			if req.method.ReturnType.Kind != typesystem.Unknown {
				if classMethod.ReturnType != req.method.ReturnType && !typesystem.Assignable(req.method.ReturnType, classMethod.ReturnType) {
					a.add("JOSS-DECL-005", diagnostics.SeverityError, a.file, classNode.Name.Token,
						fmt.Sprintf("Class `%s` method `%s` returns `%s`, but interface `%s` requires `%s`.",
							classNode.Name.Value, req.method.Name, classMethod.ReturnType.String(), req.ifaceName, req.method.ReturnType.String()),
						"Method return type must be compatible with the interface contract.",
						"Adjust the return type to match the interface.")
				}
			}
		}
	}
	if !classNode.IsAbstract && classNode.SuperClass != nil {
		reqs := a.collectAbstractMethods(classNode.SuperClass.Value, map[string]bool{})
		for _, req := range reqs {
			classMethod, exists := a.lookupMethod(classNode.Name.Value, req.method.Name)
			if !exists || classMethod.IsAbstract {
				a.add("JOSS-DECL-005", diagnostics.SeverityError, a.file, classNode.Name.Token,
					fmt.Sprintf("Class `%s` does not implement abstract method `%s` from class `%s`.",
						classNode.Name.Value, req.method.Name, req.className),
					"A concrete class extending an abstract class must implement all abstract methods.",
					fmt.Sprintf("Implement `public func %s(...)` in class `%s`.", req.method.Name, classNode.Name.Value))
				continue
			}
			if len(classMethod.Parameters) != len(req.method.Parameters) {
				a.add("JOSS-DECL-005", diagnostics.SeverityError, a.file, classNode.Name.Token,
					fmt.Sprintf("Class `%s` method `%s` has %d parameter(s), but abstract method in `%s` declares %d.",
						classNode.Name.Value, req.method.Name, len(classMethod.Parameters), req.className, len(req.method.Parameters)),
					"Method implementation must match the abstract method signature.",
					"Adjust the method parameters to match the contract.")
			}
		}
	}
}

func (a *Analyzer) analyzeClassBody(classNode *parser.ClassStatement, global *scope) {
	if classNode == nil || classNode.Name == nil {
		return
	}
	previousClass := a.currentClass
	a.currentClass = classNode.Name.Value
	defer func() { a.currentClass = previousClass }()
	classScope := newScope(global)
	classScope.put(&symbol{Name: "this", Type: typesystem.Type{Kind: typesystem.Class, Name: classNode.Name.Value}, Kind: symbolImplicit, Used: true, Synthetic: true})
	if classNode.Body == nil {
		return
	}
	for _, member := range classNode.Body.Statements {
		switch node := member.(type) {
		case *parser.MethodStatement:
			returnType := typeFromToken(node.ReturnType)
			a.validateDeclaredType(returnType, node.ReturnType, "return annotation")
			a.analyzeCallable(node.Parameters, node.Body, classScope, classNode.Name.Value, returnType)
		case *parser.InitStatement:
			a.analyzeCallable(node.Parameters, node.Body, classScope, classNode.Name.Value, typesystem.Type{Kind: typesystem.Unknown})
		case *parser.LetStatement:
			a.analyzeDeclaration(node, classScope, false)
		case *parser.MultiLetStatement:
			a.analyzeMultiDeclaration(node, classScope, false)
		}
	}
}
