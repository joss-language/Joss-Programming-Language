package analyzer

import (
	"path"
	"strings"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

// AnalysisFacts stores sidecar semantic information indexed by AST node or callable name,
// without mutating the AST shared across execution frames or runtime forks.
type AnalysisFacts struct {
	InferredTypes map[parser.Node]typesystem.Type
	Symbols       map[string]SymbolFact
	CallableTypes map[string]typesystem.Type
	Effects       map[string][]string
	ResolvedCalls map[*parser.CallExpression]ResolvedCallFact
}

// ResolvedCallFact identifies calls whose target is stable at analysis time.
// Dynamic variables and mixed receivers deliberately have no entry.
type ResolvedCallFact struct {
	TargetID   string
	Kind       string
	ReturnType typesystem.Type
}

// SymbolFact is the immutable public projection of a declaration resolved by
// the semantic analyzer. Analyzer-owned scope state is never exposed.
type SymbolFact struct {
	ID   string
	Name string
	Kind string
	Type typesystem.Type
	File string
}

func symbolFact(kind, name string, valueType typesystem.Type, file string) SymbolFact {
	return SymbolFact{ID: kind + ":" + canonicalSourcePath(file) + ":" + name, Name: name, Kind: kind, Type: valueType, File: file}
}

func canonicalSourcePath(sourcePath string) string {
	return path.Clean(strings.ReplaceAll(sourcePath, `\`, "/"))
}

// NewAnalysisFacts constructs an initialized facts container.
func NewAnalysisFacts() *AnalysisFacts {
	return &AnalysisFacts{
		InferredTypes: make(map[parser.Node]typesystem.Type),
		Symbols:       make(map[string]SymbolFact),
		CallableTypes: make(map[string]typesystem.Type),
		Effects:       make(map[string][]string),
		ResolvedCalls: make(map[*parser.CallExpression]ResolvedCallFact),
	}
}

func (a *Analyzer) recordResolvedCall(call *parser.CallExpression, targetID, kind string, returnType typesystem.Type) {
	if a.facts == nil || call == nil {
		return
	}
	a.facts.ResolvedCalls[call] = ResolvedCallFact{TargetID: targetID, Kind: kind, ReturnType: returnType}
}

// PreparedProgram encapsulates a validated program with immutable source units,
// analyzer diagnostics, and sidecar semantic facts ready for execution or tooling.
type PreparedProgram struct {
	Units       []SourceUnit
	Environment Environment
	Diagnostics []diagnostics.Diagnostic
	Facts       *AnalysisFacts
}

// PrepareProgram analyzes the provided source units and packages them into an immutable PreparedProgram.
func PrepareProgram(units []SourceUnit, env Environment) *PreparedProgram {
	diags, facts := analyzeWithFacts(units, env)

	return &PreparedProgram{
		Units:       append([]SourceUnit(nil), units...),
		Environment: cloneEnvironment(env),
		Diagnostics: diags,
		Facts:       facts,
	}
}

func cloneEnvironment(source Environment) Environment {
	clone := NewEnvironment()
	for name, callable := range source.Builtins {
		clone.Builtins[name] = cloneCallable(callable)
	}
	for name, class := range source.Classes {
		class.Interfaces = append([]string(nil), class.Interfaces...)
		class.Methods = cloneCallables(class.Methods)
		class.Fields = make(map[string]Field, len(class.Fields))
		for fieldName, field := range source.Classes[name].Fields {
			class.Fields[fieldName] = field
		}
		clone.Classes[name] = class
	}
	for name, iface := range source.Interfaces {
		iface.Extends = append([]string(nil), iface.Extends...)
		iface.Methods = cloneCallables(iface.Methods)
		clone.Interfaces[name] = iface
	}
	for name, enum := range source.Enums {
		enum.Cases = make(map[string]typesystem.Type, len(enum.Cases))
		for caseName, caseType := range source.Enums[name].Cases {
			enum.Cases[caseName] = caseType
		}
		clone.Enums[name] = enum
	}
	for name, valueType := range source.Globals {
		clone.Globals[name] = valueType
	}
	return clone
}

func cloneCallables(source map[string]Callable) map[string]Callable {
	clone := make(map[string]Callable, len(source))
	for name, callable := range source {
		clone[name] = cloneCallable(callable)
	}
	return clone
}

func cloneCallable(callable Callable) Callable {
	callable.Parameters = append([]Parameter(nil), callable.Parameters...)
	callable.Effects = append([]string(nil), callable.Effects...)
	return callable
}

// HasErrors reports whether any diagnostic has Error severity.
func (p *PreparedProgram) HasErrors() bool {
	for _, d := range p.Diagnostics {
		if d.Severity == diagnostics.SeverityError {
			return true
		}
	}
	return false
}

// Entrypoint returns the first parsed source unit. LoadProject guarantees that
// the requested entrypoint occupies this position before routes and app files.
func (p *PreparedProgram) Entrypoint() *parser.Program {
	if p == nil || len(p.Units) == 0 {
		return nil
	}
	return p.Units[0].Program
}

// Program returns the validated AST associated with path. It accepts either
// slash style so VFS and native filesystem callers share the same lookup.
func (p *PreparedProgram) Program(path string) *parser.Program {
	if p == nil {
		return nil
	}
	wanted := canonicalSourcePath(path)
	for _, unit := range p.Units {
		candidate := canonicalSourcePath(unit.Path)
		if strings.EqualFold(candidate, wanted) {
			return unit.Program
		}
	}
	return nil
}

func (a *Analyzer) collectAnalysisFacts(global *scope) {
	if a.facts == nil {
		return
	}
	for name, declaration := range a.functions {
		a.facts.CallableTypes[name] = declaration.callable.ReturnType
		a.facts.Symbols[name] = symbolFact("function", name, declaration.callable.ReturnType, declaration.file)
	}
	for name, callable := range a.environment.Builtins {
		if len(callable.Effects) > 0 {
			a.facts.Effects[name] = append([]string(nil), callable.Effects...)
		}
	}
	for name, class := range a.classes {
		classType := typesystem.Type{Kind: typesystem.Class, Name: name}
		a.facts.Symbols[name] = symbolFact("class", name, classType, class.File)
		for methodName, callable := range class.Methods {
			qualified := name + "::" + methodName
			a.facts.CallableTypes[qualified] = callable.ReturnType
			a.facts.Symbols[qualified] = symbolFact("method", qualified, callable.ReturnType, class.File)
			if len(callable.Effects) > 0 {
				a.facts.Effects[name+"::"+methodName] = append([]string(nil), callable.Effects...)
			}
		}
	}
	for name, iface := range a.interfaces {
		a.facts.Symbols[name] = symbolFact("interface", name, typesystem.Type{Kind: typesystem.Class, Name: name}, iface.File)
	}
	for name, enum := range a.enums {
		a.facts.Symbols[name] = symbolFact("enum", name, typesystem.Type{Kind: typesystem.Class, Name: name}, enum.File)
	}
	for name, resolved := range global.symbols {
		if _, exists := a.facts.Symbols[name]; !exists {
			a.facts.Symbols[name] = symbolFact("global", name, resolved.Type, resolved.File)
		}
	}
}
