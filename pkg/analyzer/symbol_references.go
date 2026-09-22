package analyzer

import (
	"fmt"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
)

// SymbolOccurrence records an exact span where a symbol identifier is defined or referenced.
type SymbolOccurrence struct {
	Name         string
	File         string
	Line         int
	Column       int
	IsDefinition bool
	Kind         string // "variable", "parameter", "function", "class"
}

// TextEdit represents a text replacement in a source file for refactoring and rename operations.
type TextEdit struct {
	Line    int
	Column  int
	Length  int
	NewText string
}

// CollectSymbolOccurrences scans the AST of all source units and extracts all identifier positions.
func CollectSymbolOccurrences(units []SourceUnit) []SymbolOccurrence {
	var occurrences []SymbolOccurrence

	for _, unit := range units {
		if unit.Program == nil {
			continue
		}
		file := unit.Path
		for _, stmt := range unit.Program.Statements {
			occurrences = append(occurrences, collectFromStatement(stmt, file)...)
		}
	}

	return occurrences
}

func collectFromStatement(stmt parser.Statement, file string) []SymbolOccurrence {
	if stmt == nil {
		return nil
	}
	var res []SymbolOccurrence

	switch s := stmt.(type) {
	case *parser.LetStatement:
		if s.Name != nil {
			res = append(res, SymbolOccurrence{
				Name:         cleanName(s.Name.Value),
				File:         file,
				Line:         s.Name.Token.Line,
				Column:       s.Name.Token.Column,
				IsDefinition: true,
				Kind:         "variable",
			})
		}
		if s.Value != nil {
			res = append(res, collectFromExpression(s.Value, file)...)
		}

	case *parser.MultiLetStatement:
		for _, decl := range s.Declarations {
			if decl.Name != nil {
				res = append(res, SymbolOccurrence{
					Name:         cleanName(decl.Name.Value),
					File:         file,
					Line:         decl.Name.Token.Line,
					Column:       decl.Name.Token.Column,
					IsDefinition: true,
					Kind:         "variable",
				})
			}
			if decl.Value != nil {
				res = append(res, collectFromExpression(decl.Value, file)...)
			}
		}

	case *parser.ExpressionStatement:
		res = append(res, collectFromExpression(s.Expression, file)...)

	case *parser.EchoStatement:
		res = append(res, collectFromExpression(s.Value, file)...)

	case *parser.ReturnStatement:
		if s.ReturnValue != nil {
			res = append(res, collectFromExpression(s.ReturnValue, file)...)
		}

	case *parser.BlockStatement:
		for _, child := range s.Statements {
			res = append(res, collectFromStatement(child, file)...)
		}

	case *parser.MethodStatement:
		if s.Name != nil {
			res = append(res, SymbolOccurrence{
				Name:         s.Name.Value,
				File:         file,
				Line:         s.Name.Token.Line,
				Column:       s.Name.Token.Column,
				IsDefinition: true,
				Kind:         "function",
			})
		}
		for _, param := range s.Parameters {
			if param.Name != nil {
				res = append(res, SymbolOccurrence{
					Name:         cleanName(param.Name.Value),
					File:         file,
					Line:         param.Name.Token.Line,
					Column:       param.Name.Token.Column,
					IsDefinition: true,
					Kind:         "parameter",
				})
			}
		}
		if s.Body != nil {
			res = append(res, collectFromStatement(s.Body, file)...)
		}

	case *parser.ClassStatement:
		if s.Name != nil {
			res = append(res, SymbolOccurrence{
				Name:         s.Name.Value,
				File:         file,
				Line:         s.Name.Token.Line,
				Column:       s.Name.Token.Column,
				IsDefinition: true,
				Kind:         "class",
			})
		}
		if s.Body != nil {
			res = append(res, collectFromStatement(s.Body, file)...)
		}

	case *parser.ForeachStatement:
		if s.Key != "" {
			res = append(res, SymbolOccurrence{
				Name:         cleanName(s.Key),
				File:         file,
				Line:         s.Token.Line,
				Column:       s.Token.Column,
				IsDefinition: true,
				Kind:         "variable",
			})
		}
		if s.Value != "" {
			res = append(res, SymbolOccurrence{
				Name:         cleanName(s.Value),
				File:         file,
				Line:         s.Token.Line,
				Column:       s.Token.Column,
				IsDefinition: true,
				Kind:         "variable",
			})
		}
		if s.Body != nil {
			res = append(res, collectFromStatement(s.Body, file)...)
		}

	case *parser.TryCatchStatement:
		if s.TryBlock != nil {
			res = append(res, collectFromStatement(s.TryBlock, file)...)
		}
		if s.CatchVar != "" {
			res = append(res, SymbolOccurrence{
				Name:         cleanName(s.CatchVar),
				File:         file,
				Line:         s.CatchToken.Line,
				Column:       s.CatchToken.Column,
				IsDefinition: true,
				Kind:         "variable",
			})
		}
		if s.CatchBlock != nil {
			res = append(res, collectFromStatement(s.CatchBlock, file)...)
		}
	}

	return res
}

func collectFromExpression(expr parser.Expression, file string) []SymbolOccurrence {
	if expr == nil {
		return nil
	}
	var res []SymbolOccurrence

	switch e := expr.(type) {
	case *parser.Identifier:
		name := cleanName(e.Value)
		if !isKeywordOrLiteral(name) {
			res = append(res, SymbolOccurrence{
				Name:         name,
				File:         file,
				Line:         e.Token.Line,
				Column:       e.Token.Column,
				IsDefinition: false,
				Kind:         "variable",
			})
		}

	case *parser.AssignExpression:
		res = append(res, collectFromExpression(e.Left, file)...)
		res = append(res, collectFromExpression(e.Value, file)...)

	case *parser.InfixExpression:
		res = append(res, collectFromExpression(e.Left, file)...)
		res = append(res, collectFromExpression(e.Right, file)...)

	case *parser.PrefixExpression:
		res = append(res, collectFromExpression(e.Right, file)...)

	case *parser.PostfixExpression:
		res = append(res, collectFromExpression(e.Left, file)...)

	case *parser.TernaryExpression:
		res = append(res, collectFromExpression(e.Condition, file)...)
		if e.True != nil {
			res = append(res, collectFromExpression(e.True, file)...)
		}
		if e.False != nil {
			res = append(res, collectFromExpression(e.False, file)...)
		}

	case *parser.BlockExpression:
		if e.Block != nil {
			res = append(res, collectFromStatement(e.Block, file)...)
		}

	case *parser.CallExpression:
		res = append(res, collectFromExpression(e.Function, file)...)
		for _, arg := range e.Arguments {
			res = append(res, collectFromExpression(arg, file)...)
		}

	case *parser.MemberExpression:
		res = append(res, collectFromExpression(e.Left, file)...)

	case *parser.IndexExpression:
		res = append(res, collectFromExpression(e.Left, file)...)
		res = append(res, collectFromExpression(e.Index, file)...)

	case *parser.ArrayLiteral:
		for _, el := range e.Elements {
			res = append(res, collectFromExpression(el, file)...)
		}

	case *parser.MapLiteral:
		for k, v := range e.Pairs {
			res = append(res, collectFromExpression(k, file)...)
			res = append(res, collectFromExpression(v, file)...)
		}
	}

	return res
}

func isKeywordOrLiteral(name string) bool {
	switch name {
	case "null", "nil", "true", "false", "this", "self", "super", "parent":
		return true
	default:
		return false
	}
}

// FindOccurrencesAt finds all occurrences matching the symbol at the given file and position.
func FindOccurrencesAt(units []SourceUnit, file string, line, column int) []SymbolOccurrence {
	all := CollectSymbolOccurrences(units)
	var targetName string
	for _, occ := range all {
		if occ.File == file && occ.Line == line && (occ.Column == column || (column >= occ.Column && column <= occ.Column+len(occ.Name)+1)) {
			targetName = occ.Name
			break
		}
	}
	if targetName == "" {
		return nil
	}
	var matched []SymbolOccurrence
	for _, occ := range all {
		if occ.Name == targetName {
			matched = append(matched, occ)
		}
	}
	return matched
}

// PrepareRename produces workspace text edits to safely rename a symbol across all source units.
func PrepareRename(units []SourceUnit, file string, line, column int, newName string) (map[string][]TextEdit, error) {
	cleanNew := strings.TrimPrefix(newName, "$")
	if cleanNew == "" {
		return nil, fmt.Errorf("new name cannot be empty")
	}
	occurrences := FindOccurrencesAt(units, file, line, column)
	if len(occurrences) == 0 {
		return nil, fmt.Errorf("no symbol found at %s:%d:%d", file, line, column)
	}

	edits := make(map[string][]TextEdit)
	for _, occ := range occurrences {
		edit := TextEdit{
			Line:    occ.Line,
			Column:  occ.Column,
			Length:  len(occ.Name),
			NewText: cleanNew,
		}
		edits[occ.File] = append(edits[occ.File], edit)
	}

	return edits, nil
}
