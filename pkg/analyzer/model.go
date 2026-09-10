// Package analyzer performs project-aware semantic analysis over the Joss AST.
// It deliberately depends only on language-layer packages; runtime symbols are
// supplied through Environment so the analyzer remains reusable and testable.
package analyzer

import (
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

type SourceUnit struct {
	Path    string
	Program *parser.Program
}

type Parameter struct {
	Name        string
	Type        typesystem.Type
	HasDefault  bool
	ByReference bool
}

type Callable struct {
	Name       string
	Parameters []Parameter
	ReturnType typesystem.Type
	Variadic   bool
	Visibility string
	Owner      string
	File       string
}

type Field struct {
	Type       typesystem.Type
	Constant   bool
	Visibility string
	Owner      string
}

type Interface struct {
	Name       string
	Extends    []string
	Methods    map[string]Callable
	Visibility string
	File       string
}

type Class struct {
	Name       string
	SuperClass string
	Interfaces []string
	Methods    map[string]Callable
	Fields     map[string]Field
	Visibility string
	File       string
}

type Environment struct {
	Builtins   map[string]Callable
	Classes    map[string]Class
	Interfaces map[string]Interface
	Globals    map[string]typesystem.Type
}

func NewEnvironment() Environment {
	return Environment{
		Builtins:   make(map[string]Callable),
		Classes:    make(map[string]Class),
		Interfaces: make(map[string]Interface),
		Globals:    make(map[string]typesystem.Type),
	}
}
