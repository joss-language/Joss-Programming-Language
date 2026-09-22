package analyzer

import (
	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

// AnalysisFacts stores sidecar semantic information indexed by AST node or callable name,
// without mutating the AST shared across execution frames or runtime forks.
type AnalysisFacts struct {
	InferredTypes map[parser.Node]typesystem.Type
	Symbols       map[string]*symbol
	CallableTypes map[string]typesystem.Type
	Effects       map[string][]string
}

// NewAnalysisFacts constructs an initialized facts container.
func NewAnalysisFacts() *AnalysisFacts {
	return &AnalysisFacts{
		InferredTypes: make(map[parser.Node]typesystem.Type),
		Symbols:       make(map[string]*symbol),
		CallableTypes: make(map[string]typesystem.Type),
		Effects:       make(map[string][]string),
	}
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
	diags := Analyze(units, env)
	facts := NewAnalysisFacts()

	for _, unit := range units {
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			if method, ok := stmt.(*parser.MethodStatement); ok {
				returnType := typesystem.Parse(method.ReturnType.Literal)
				facts.CallableTypes[method.Name.Value] = returnType
				if isEffectfulMethod(method.Name.Value) {
					facts.Effects[method.Name.Value] = []string{"io", "blocking"}
				}
			}
		}
	}

	return &PreparedProgram{
		Units:       units,
		Environment: env,
		Diagnostics: diags,
		Facts:       facts,
	}
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

func isEffectfulMethod(name string) bool {
	switch name {
	case "run", "start", "read", "write", "send", "recv", "connect":
		return true
	default:
		return false
	}
}
