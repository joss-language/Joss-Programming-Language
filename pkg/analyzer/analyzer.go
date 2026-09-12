package analyzer

import (
	"sort"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

type functionDeclaration struct {
	callable Callable
	token    parser.Token
	file     string
}

type Analyzer struct {
	environment       Environment
	diagnostics       diagnostics.Bag
	functions         map[string]functionDeclaration
	classes           map[string]Class
	classTokens       map[string]functionDeclaration
	interfaces        map[string]Interface
	interfaceTokens   map[string]functionDeclaration
	enums             map[string]Enum
	enumTokens        map[string]functionDeclaration
	file              string
	currentClass      string
	currentReturnType typesystem.Type
	suppressUndefined int
}

func Analyze(units []SourceUnit, environment Environment) []diagnostics.Diagnostic {
	a := &Analyzer{
		environment:       environment,
		functions:         make(map[string]functionDeclaration),
		classes:           make(map[string]Class),
		classTokens:       make(map[string]functionDeclaration),
		interfaces:        make(map[string]Interface),
		interfaceTokens:   make(map[string]functionDeclaration),
		enums:             make(map[string]Enum),
		enumTokens:        make(map[string]functionDeclaration),
		currentReturnType: typesystem.Type{Kind: typesystem.Unknown},
	}
	for name, class := range environment.Classes {
		a.classes[name] = class
	}
	for name, iface := range environment.Interfaces {
		a.interfaces[name] = iface
	}
	for name, enumVal := range environment.Enums {
		a.enums[name] = enumVal
	}
	a.collectDeclarations(units)
	global := a.projectScope()
	a.validateNominalContracts(units, global)
	a.analyzeSourceBodies(units, global)
	items := a.diagnostics.Items()
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].File != items[j].File {
			return items[i].File < items[j].File
		}
		if items[i].Range.Start.Line != items[j].Range.Start.Line {
			return items[i].Range.Start.Line < items[j].Range.Start.Line
		}
		if items[i].Range.Start.Column != items[j].Range.Start.Column {
			return items[i].Range.Start.Column < items[j].Range.Start.Column
		}
		return items[i].Code < items[j].Code
	})
	return items
}

func (a *Analyzer) withSourceFile(file string, analyze func()) {
	previousFile := a.file
	a.file = file
	defer func() { a.file = previousFile }()
	analyze()
}
