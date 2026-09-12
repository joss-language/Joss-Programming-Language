package analyzer

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

func (a *Analyzer) collectDeclarations(units []SourceUnit) {
	for _, unit := range units {
		if unit.Program == nil {
			continue
		}
		for _, statement := range unit.Program.Statements {
			switch node := statement.(type) {
			case *parser.MethodStatement:
				a.declareFunction(node, unit.Path)
			case *parser.ClassStatement:
				a.declareClass(node, unit.Path)
			case *parser.InterfaceStatement:
				a.declareInterface(node, unit.Path)
			case *parser.EnumStatement:
				a.declareEnum(node, unit.Path)
			}
		}
	}
}

func (a *Analyzer) declareFunction(method *parser.MethodStatement, file string) {
	if method == nil || method.Name == nil {
		return
	}
	name := method.Name.Value
	if previous, exists := a.functions[name]; exists {
		a.add("JOSS-DECL-001", diagnostics.SeverityError, file, method.Name.Token,
			fmt.Sprintf("Function `%s` is already declared at %s:%d.", name, previous.file, previous.token.Line),
			"Each project-level function must have a unique name.", "Rename or remove one declaration.")
		return
	}
	callable := callableFromMethod(method)
	callable.File = file
	a.functions[name] = functionDeclaration{callable: callable, token: method.Name.Token, file: file}
}

func (a *Analyzer) declareInterface(ifaceNode *parser.InterfaceStatement, file string) {
	if ifaceNode == nil || ifaceNode.Name == nil {
		return
	}
	name := ifaceNode.Name.Value
	if previous, exists := a.interfaces[name]; exists {
		where := "the runtime environment"
		if declaration, ok := a.interfaceTokens[name]; ok {
			where = fmt.Sprintf("%s:%d", declaration.file, declaration.token.Line)
		} else if previous.Name != "" {
			where = "a declared interface"
		}
		a.add("JOSS-DECL-004", diagnostics.SeverityError, file, ifaceNode.Name.Token,
			fmt.Sprintf("Interface `%s` conflicts with %s.", name, where),
			"Interface names share one project-wide namespace.", "Choose a unique interface name.")
		return
	}
	if classDecl, exists := a.classTokens[name]; exists {
		a.add("JOSS-DECL-004", diagnostics.SeverityError, file, ifaceNode.Name.Token,
			fmt.Sprintf("Interface `%s` conflicts with class at %s:%d.", name, classDecl.file, classDecl.token.Line),
			"Interfaces and classes share one project-wide type namespace.", "Choose a unique name.")
		return
	}
	iface := Interface{
		Name:       name,
		Extends:    make([]string, 0, len(ifaceNode.Extends)),
		Methods:    make(map[string]Callable),
		Visibility: ifaceNode.Visibility,
		File:       file,
	}
	for _, ext := range ifaceNode.Extends {
		if ext != nil {
			iface.Extends = append(iface.Extends, ext.Value)
		}
	}
	for _, method := range ifaceNode.Methods {
		if method == nil || method.Name == nil {
			continue
		}
		methodName := method.Name.Value
		callable := callableFromMethod(method)
		callable.Owner = name
		callable.File = file
		if _, exists := iface.Methods[methodName]; exists {
			a.add("JOSS-DECL-003", diagnostics.SeverityError, file, method.Name.Token,
				fmt.Sprintf("Method `%s::%s` is declared more than once.", name, methodName),
				"An interface cannot contain duplicate method names.", "Rename or remove one method.")
			continue
		}
		iface.Methods[methodName] = callable
	}
	a.interfaces[name] = iface
	a.interfaceTokens[name] = functionDeclaration{token: ifaceNode.Name.Token, file: file}
}

func (a *Analyzer) declareClass(classNode *parser.ClassStatement, file string) {
	if classNode == nil || classNode.Name == nil {
		return
	}
	name := classNode.Name.Value
	if previous, exists := a.classes[name]; exists {
		where := "the runtime environment"
		if declaration, ok := a.classTokens[name]; ok {
			where = fmt.Sprintf("%s:%d", declaration.file, declaration.token.Line)
		} else if previous.Name != "" {
			where = "a native or plugin class"
		}
		a.add("JOSS-DECL-002", diagnostics.SeverityError, file, classNode.Name.Token,
			fmt.Sprintf("Class `%s` conflicts with %s.", name, where),
			"Class names share one project-wide namespace.", "Choose a unique class name.")
		return
	}
	if ifaceDecl, exists := a.interfaceTokens[name]; exists {
		a.add("JOSS-DECL-004", diagnostics.SeverityError, file, classNode.Name.Token,
			fmt.Sprintf("Class `%s` conflicts with interface at %s:%d.", name, ifaceDecl.file, ifaceDecl.token.Line),
			"Interfaces and classes share one project-wide type namespace.", "Choose a unique name.")
		return
	}
	class := Class{Name: name, Methods: make(map[string]Callable), Fields: make(map[string]Field), Visibility: classNode.Visibility, File: file, IsAbstract: classNode.IsAbstract}
	if classNode.SuperClass != nil {
		class.SuperClass = classNode.SuperClass.Value
	}
	for _, iface := range classNode.Interfaces {
		if iface != nil {
			class.Interfaces = append(class.Interfaces, iface.Value)
		}
	}
	if classNode.Body != nil {
		for _, member := range classNode.Body.Statements {
			if declaration, ok := member.(*parser.LetStatement); ok && declaration.Name != nil {
				class.Fields[declaration.Name.Value] = Field{Type: typeFromToken(declaration.Token), Constant: declaration.IsConst, Visibility: declaration.Visibility, Owner: name}
				continue
			}
			if declarations, ok := member.(*parser.MultiLetStatement); ok {
				for _, declaration := range declarations.Declarations {
					if declaration.Name != nil {
						class.Fields[declaration.Name.Value] = Field{Type: typeFromToken(declarations.TypeToken), Visibility: declarations.Visibility, Owner: name}
					}
				}
				continue
			}
			var methodName string
			var callable Callable
			var token parser.Token
			switch method := member.(type) {
			case *parser.MethodStatement:
				if method.Name == nil {
					continue
				}
				methodName, callable, token = method.Name.Value, callableFromMethod(method), method.Name.Token
				callable.Owner = name
				callable.File = file
				if callable.IsAbstract && !classNode.IsAbstract {
					a.add("JOSS-DECL-005", diagnostics.SeverityError, file, token,
						fmt.Sprintf("Class `%s` must be declared abstract because it contains abstract method `%s`.", name, methodName),
						"A class containing abstract methods must be marked `abstract class`.", "Add `abstract` to the class declaration or provide a method body.")
				}
				if methodName == "constructor" || methodName == "Init" {
					for _, p := range method.Parameters {
						if p != nil && p.Name != nil && p.Visibility.Literal != "" {
							class.Fields[p.Name.Value] = Field{
								Type:       typeFromToken(p.Type),
								Constant:   p.IsConst,
								Visibility: p.Visibility.Literal,
								Owner:      name,
							}
						}
					}
				}
			case *parser.InitStatement:
				if method.Name == nil {
					continue
				}
				methodName = method.Name.Value
				callable = Callable{Name: methodName, Parameters: parametersFromAST(method.Parameters), ReturnType: typesystem.Type{Kind: typesystem.Unknown}}
				token = method.Name.Token
				if methodName == "constructor" || methodName == "Init" || methodName == "main" {
					for _, p := range method.Parameters {
						if p != nil && p.Name != nil && p.Visibility.Literal != "" {
							class.Fields[p.Name.Value] = Field{
								Type:       typeFromToken(p.Type),
								Constant:   p.IsConst,
								Visibility: p.Visibility.Literal,
								Owner:      name,
							}
						}
					}
				}
			default:
				continue
			}
			if _, exists := class.Methods[methodName]; exists {
				a.add("JOSS-DECL-003", diagnostics.SeverityError, file, token,
					fmt.Sprintf("Method `%s::%s` is declared more than once.", name, methodName),
					"A class cannot contain duplicate method names.", "Rename or remove one method.")
				continue
			}
			class.Methods[methodName] = callable
		}
	}
	a.classes[name] = class
	a.classTokens[name] = functionDeclaration{token: classNode.Name.Token, file: file}
}

func (a *Analyzer) declareEnum(enumNode *parser.EnumStatement, file string) {
	if enumNode == nil || enumNode.Name == nil {
		return
	}
	name := enumNode.Name.Value
	if previous, exists := a.enums[name]; exists {
		where := "a declared enum"
		if declaration, ok := a.enumTokens[name]; ok {
			where = fmt.Sprintf("%s:%d", declaration.file, declaration.token.Line)
		} else if previous.Name != "" {
			where = "a declared enum"
		}
		a.add("JOSS-DECL-004", diagnostics.SeverityError, file, enumNode.Name.Token,
			fmt.Sprintf("Enum `%s` conflicts with %s.", name, where),
			"Enum names share one project-wide type namespace.", "Choose a unique enum name.")
		return
	}
	if classDecl, exists := a.classTokens[name]; exists {
		a.add("JOSS-DECL-004", diagnostics.SeverityError, file, enumNode.Name.Token,
			fmt.Sprintf("Enum `%s` conflicts with class at %s:%d.", name, classDecl.file, classDecl.token.Line),
			"Enums and classes share one project-wide type namespace.", "Choose a unique name.")
		return
	}
	if ifaceDecl, exists := a.interfaceTokens[name]; exists {
		a.add("JOSS-DECL-004", diagnostics.SeverityError, file, enumNode.Name.Token,
			fmt.Sprintf("Enum `%s` conflicts with interface at %s:%d.", name, ifaceDecl.file, ifaceDecl.token.Line),
			"Enums and interfaces share one project-wide type namespace.", "Choose a unique name.")
		return
	}

	backingType := typesystem.Type{Kind: typesystem.Unknown}
	if enumNode.BackingType.Literal != "" {
		backingType = typeFromToken(enumNode.BackingType)
	}

	enumDef := Enum{
		Name:        name,
		BackingType: backingType,
		Cases:       make(map[string]typesystem.Type),
		Visibility:  enumNode.Visibility,
		File:        file,
	}

	for _, c := range enumNode.Cases {
		if c == nil || c.Name == nil {
			continue
		}
		caseName := c.Name.Value
		if _, exists := enumDef.Cases[caseName]; exists {
			a.add("JOSS-DECL-003", diagnostics.SeverityError, file, c.Name.Token,
				fmt.Sprintf("Case `%s` is declared more than once in enum `%s`.", caseName, name),
				"An enum cannot contain duplicate case names.", "Remove or rename the duplicate case.")
			continue
		}
		enumDef.Cases[caseName] = typesystem.Type{Kind: typesystem.Class, Name: name}
	}

	a.enums[name] = enumDef
	a.enumTokens[name] = functionDeclaration{token: enumNode.Name.Token, file: file}
}

func callableFromMethod(method *parser.MethodStatement) Callable {
	return Callable{
		Name:       method.Name.Value,
		Parameters: parametersFromAST(method.Parameters),
		ReturnType: typeFromToken(method.ReturnType),
		Visibility: method.Visibility,
		IsAbstract: method.IsAbstract,
	}
}

func typeFromToken(token parser.Token) typesystem.Type {
	if token.Literal == "" {
		return typesystem.Type{Kind: typesystem.Unknown}
	}
	return typesystem.Parse(token.Literal)
}

func parametersFromAST(parameters []*parser.Parameter) []Parameter {
	result := make([]Parameter, 0, len(parameters))
	for _, parameter := range parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		parameterType := typesystem.Type{Kind: typesystem.Mixed}
		if parameter.Type.Literal != "" && parameter.Type.Type != parser.VAR {
			parameterType = typesystem.Parse(parameter.Type.Literal)
		}
		result = append(result, Parameter{Name: parameter.Name.Value, Type: parameterType, HasDefault: parameter.DefaultValue != nil, ByReference: parameter.ByReference})
	}
	return result
}
