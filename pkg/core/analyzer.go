package core

import (
	"fmt"
	"sort"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

// AnalysisReport exposes the canonical structured diagnostics produced by the
// semantic analyzer. Consumers must not maintain parallel string lists.
type AnalysisReport struct {
	Diagnostics []diagnostics.Diagnostic
}

func (ar *AnalysisReport) HasIssues() bool { return len(ar.Diagnostics) > 0 }
func (ar *AnalysisReport) HasErrors() bool {
	return ar.Count(diagnostics.SeverityError) > 0
}

func (ar *AnalysisReport) Count(severity diagnostics.Severity) int {
	count := 0
	for _, item := range ar.Diagnostics {
		if item.Severity == severity {
			count++
		}
	}
	return count
}

func (ar *AnalysisReport) PrintReport() {
	if !ar.HasIssues() {
		fmt.Println("Static analysis completed: no problems found.")
		return
	}
	fmt.Println("\nJoss static analysis")
	fmt.Println("------------------------------------------------------------")
	for _, item := range ar.Diagnostics {
		fmt.Println(item.String())
		if item.Suggestion != "" {
			fmt.Printf("  suggestion: %s\n", item.Suggestion)
		}
	}
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("%d error(s), %d warning(s)\n", ar.Count(diagnostics.SeverityError), ar.Count(diagnostics.SeverityWarning))
}

// AnalyzeProgram analyzes an in-memory program. Project tooling should prefer
// AnalyzeSourceUnits so diagnostics retain their source file.
func AnalyzeProgram(program *parser.Program) *AnalysisReport {
	return AnalyzeSourceUnits([]semanticanalyzer.SourceUnit{{Path: "<memory>", Program: program}})
}

// AnalyzeSourceUnits performs project-aware, cross-file semantic analysis.
func AnalyzeSourceUnits(units []semanticanalyzer.SourceUnit) *AnalysisReport {
	environment := buildAnalysisEnvironment()
	items := semanticanalyzer.Analyze(units, environment)
	return AnalysisReportFromDiagnostics(items)
}

func AnalysisReportFromDiagnostics(items []diagnostics.Diagnostic) *AnalysisReport {
	return &AnalysisReport{Diagnostics: append([]diagnostics.Diagnostic(nil), items...)}
}

// buildAnalysisEnvironment adapts the runtime's actual registries. It avoids a
// second hand-maintained class/function table: core natives and loaded plugin
// symbol indexes are the source of truth for the analyzer as well as execution.
func buildAnalysisEnvironment() semanticanalyzer.Environment {
	environment := semanticanalyzer.NewEnvironment()
	for _, name := range GetBuiltinFunctionNames() {
		environment.Builtins[name] = semanticanalyzer.Callable{
			Name: name, ReturnType: builtinReturnType(name), Variadic: true,
		}
	}

	runtime := NewRuntime()
	defer runtime.Free()
	classNames := make([]string, 0, len(runtime.Classes))
	for name := range runtime.Classes {
		classNames = append(classNames, name)
	}
	sort.Strings(classNames)
	for _, name := range classNames {
		classNode := runtime.Classes[name]
		class := semanticanalyzer.Class{Name: name, Methods: make(map[string]semanticanalyzer.Callable), Fields: make(map[string]semanticanalyzer.Field)}
		if classNode != nil && classNode.SuperClass != nil {
			class.SuperClass = classNode.SuperClass.Value
		}
		if classNode != nil && classNode.Body != nil {
			for _, statement := range classNode.Body.Statements {
				switch method := statement.(type) {
				case *parser.MethodStatement:
					if method.Name != nil {
						class.Methods[method.Name.Value] = analysisCallable(method.Name.Value, method.Parameters, method.ReturnType, method.Body == nil)
					}
				case *parser.InitStatement:
					if method.Name != nil {
						class.Methods[method.Name.Value] = analysisCallable(method.Name.Value, method.Parameters, parser.Token{}, method.Body == nil)
					}
				case *parser.LetStatement:
					if method.Name != nil {
						class.Fields[method.Name.Value] = semanticanalyzer.Field{Type: typesystem.Parse(method.Token.Literal), Constant: method.IsConst}
					}
				}
			}
		}
		if classNode != nil {
			for _, iface := range classNode.Interfaces {
				if iface != nil {
					class.Interfaces = append(class.Interfaces, iface.Value)
				}
			}
		}
		environment.Classes[name] = class
	}

	ifaceNames := make([]string, 0, len(runtime.Interfaces))
	for name := range runtime.Interfaces {
		ifaceNames = append(ifaceNames, name)
	}
	sort.Strings(ifaceNames)
	for _, name := range ifaceNames {
		ifaceNode := runtime.Interfaces[name]
		if ifaceNode == nil {
			continue
		}
		iface := semanticanalyzer.Interface{
			Name:       name,
			Methods:    make(map[string]semanticanalyzer.Callable),
			Extends:    make([]string, 0, len(ifaceNode.Extends)),
			Visibility: ifaceNode.Visibility,
		}
		for _, ext := range ifaceNode.Extends {
			if ext != nil {
				iface.Extends = append(iface.Extends, ext.Value)
			}
		}
		for _, method := range ifaceNode.Methods {
			if method != nil && method.Name != nil {
				iface.Methods[method.Name.Value] = analysisCallable(method.Name.Value, method.Parameters, method.ReturnType, false)
			}
		}
		environment.Interfaces[name] = iface
	}

	// Plugin symbol indexes carry signatures even for non-AST plugin formats.
	if runtime.PluginRegistry != nil {
		for _, plugin := range runtime.PluginRegistry.List() {
			for _, symbolClass := range plugin.Symbols.Classes {
				class := environment.Classes[symbolClass.Name]
				if class.Name == "" {
					class = semanticanalyzer.Class{Name: symbolClass.Name, Methods: make(map[string]semanticanalyzer.Callable), Fields: make(map[string]semanticanalyzer.Field)}
				}
				if class.Methods == nil {
					class.Methods = make(map[string]semanticanalyzer.Callable)
				}
				class.SuperClass = symbolClass.SuperClass
				for _, method := range symbolClass.Methods {
					parameters := make([]semanticanalyzer.Parameter, 0, len(method.Parameters))
					for _, parameter := range method.Parameters {
						parameterType := typesystem.Type{Kind: typesystem.Mixed}
						if parameter.Type != "" {
							parameterType = typesystem.Parse(parameter.Type)
						}
						parameters = append(parameters, semanticanalyzer.Parameter{Name: parameter.Name, Type: parameterType})
					}
					returnType := typesystem.Type{Kind: typesystem.Unknown}
					if method.ReturnType != "" {
						returnType = typesystem.Parse(method.ReturnType)
					}
					class.Methods[method.Name] = semanticanalyzer.Callable{Name: method.Name, Parameters: parameters, ReturnType: returnType}
				}
				environment.Classes[class.Name] = class
			}
			for _, function := range plugin.Symbols.Functions {
				returnType := typesystem.Type{Kind: typesystem.Unknown}
				if function.ReturnType != "" {
					returnType = typesystem.Parse(function.ReturnType)
				}
				environment.Builtins[function.Name] = semanticanalyzer.Callable{Name: function.Name, ReturnType: returnType, Variadic: true}
			}
		}
	}
	for name := range runtime.Variables {
		environment.Globals[name] = typesystem.Type{Kind: typesystem.Unknown}
	}
	return environment
}

func analysisCallable(name string, parameters []*parser.Parameter, returnToken parser.Token, signatureUnknown bool) semanticanalyzer.Callable {
	returnType := typesystem.Type{Kind: typesystem.Unknown}
	if returnToken.Literal != "" {
		returnType = typesystem.Parse(returnToken.Literal)
	}
	result := semanticanalyzer.Callable{Name: name, ReturnType: returnType, Variadic: signatureUnknown}
	for _, parameter := range parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		parameterType := typesystem.Type{Kind: typesystem.Mixed}
		if parameter.Type.Literal != "" && parameter.Type.Type != parser.VAR {
			parameterType = typesystem.Parse(parameter.Type.Literal)
		}
		result.Parameters = append(result.Parameters, semanticanalyzer.Parameter{Name: parameter.Name.Value, Type: parameterType, HasDefault: parameter.DefaultValue != nil, ByReference: parameter.ByReference})
	}
	return result
}
