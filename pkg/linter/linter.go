package linter

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/viewtemplate"
)

type RuleCategory string

const (
	CategoryCorrectness RuleCategory = "correctness"
	CategoryStyle       RuleCategory = "style"
	CategorySecurity    RuleCategory = "security"
	CategoryPerformance RuleCategory = "performance"
)

type LintIssue struct {
	RuleID      string               `json:"rule_id"`
	Category    RuleCategory         `json:"category"`
	Severity    diagnostics.Severity `json:"severity"`
	File        string               `json:"file"`
	Line        int                  `json:"line"`
	Column      int                  `json:"column"`
	Message     string               `json:"message"`
	Explanation string               `json:"explanation,omitempty"`
	Suggestion  string               `json:"suggestion,omitempty"`
	AutoFixable bool                 `json:"auto_fixable"`
}

func (i LintIssue) String() string {
	loc := i.File
	if i.Line > 0 {
		loc = fmt.Sprintf("%s:%d:%d", i.File, i.Line, i.Column)
	}
	return fmt.Sprintf("[%s] %s %s: %s", i.RuleID, strings.ToUpper(string(i.Severity)), loc, i.Message)
}

type Linter struct {
	files []string
}

func NewLinter(paths ...string) *Linter {
	return &Linter{files: paths}
}

func (l *Linter) LintSource(filename, src string) ([]LintIssue, error) {
	p := parser.NewParser(parser.NewLexer(src))
	prog := p.ParseProgram()

	var issues []LintIssue

	// 1. Parser Errors
	if len(p.Errors()) > 0 {
		for _, errStr := range p.Errors() {
			issues = append(issues, LintIssue{
				RuleID:   "JOSS-SYNTAX-001",
				Category: CategoryCorrectness,
				Severity: diagnostics.SeverityError,
				File:     filename,
				Line:     1,
				Message:  errStr,
			})
		}
		return issues, nil
	}

	// 2. Semantic Analysis integration
	report := core.AnalyzeSourceUnits([]semanticanalyzer.SourceUnit{{Path: filename, Program: prog}})
	issues = append(issues, lintIssuesFromDiagnostics(report.Diagnostics, filename)...)

	// 3. Static AST Lint Rules
	astIssues := checkASTRules(filename, prog, src)
	issues = append(issues, astIssues...)

	return issues, nil
}

func (l *Linter) LintPath(targetPath string) ([]LintIssue, error) {
	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		data, err := os.ReadFile(targetPath)
		if err != nil {
			return nil, err
		}
		if isViewTemplateFile(targetPath) {
			return l.LintViewSource(targetPath, string(data)), nil
		}
		return l.LintSource(targetPath, string(data))
	}

	var allIssues []LintIssue
	var units []semanticanalyzer.SourceUnit
	err = filepath.WalkDir(targetPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if parser.IsIgnoredDirectory(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isViewTemplateFile(path) {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			allIssues = append(allIssues, l.LintViewSource(path, string(data))...)
			return nil
		}
		if parser.IsJossSourceFile(path) {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			p := parser.NewParser(parser.NewLexer(string(data)))
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				for _, errStr := range p.Errors() {
					allIssues = append(allIssues, LintIssue{
						RuleID: "JOSS-SYNTAX-001", Category: CategoryCorrectness,
						Severity: diagnostics.SeverityError, File: path, Line: 1,
						Message: errStr,
					})
				}
				return nil
			}
			units = append(units, semanticanalyzer.SourceUnit{Path: path, Program: program})
			allIssues = append(allIssues, checkASTRules(path, program, string(data))...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	report := core.AnalyzeSourceUnits(units)
	allIssues = append(lintIssuesFromDiagnostics(report.Diagnostics, targetPath), allIssues...)

	return allIssues, nil
}

func lintIssuesFromDiagnostics(items []diagnostics.Diagnostic, fallbackFile string) []LintIssue {
	issues := make([]LintIssue, 0, len(items))
	for _, diag := range items {
		filename := diag.File
		if filename == "" || filename == "<memory>" {
			filename = fallbackFile
		}
		issues = append(issues, LintIssue{
			RuleID:      diag.Code,
			Category:    CategoryCorrectness,
			Severity:    diag.Severity,
			File:        filename,
			Line:        diag.Range.Start.Line,
			Column:      diag.Range.Start.Column,
			Message:     diag.Message,
			Explanation: diag.Explanation,
			Suggestion:  diag.Suggestion,
			AutoFixable: false,
		})
	}
	return issues
}

func checkASTRules(filename string, prog *parser.Program, src string) []LintIssue {
	var issues []LintIssue
	if prog == nil {
		return issues
	}

	for _, stmt := range prog.Statements {
		switch s := stmt.(type) {
		case *parser.MethodStatement:
			// Rule: Check function naming (camelCase)
			if s.Name != nil && len(s.Name.Value) > 0 {
				first := rune(s.Name.Value[0])
				if first >= 'A' && first <= 'Z' && s.Name.Value != "Init" {
					issues = append(issues, LintIssue{
						RuleID:      "JOSS-LINT-007",
						Category:    CategoryStyle,
						Severity:    diagnostics.SeverityWarning,
						File:        filename,
						Line:        s.Name.Token.Line,
						Column:      s.Name.Token.Column,
						Message:     fmt.Sprintf("La función '%s' debería usar convención camelCase.", s.Name.Value),
						Suggestion:  fmt.Sprintf("Renombra la función a '%s%s'.", strings.ToLower(string(first)), s.Name.Value[1:]),
						AutoFixable: false,
					})
				}
			}
			// Rule: Explicit parameter types
			for _, param := range s.Parameters {
				if param.Type.Literal == "" || param.Type.Type == parser.VAR {
					issues = append(issues, LintIssue{
						RuleID:      "JOSS-LINT-002",
						Category:    CategoryCorrectness,
						Severity:    diagnostics.SeverityError,
						File:        filename,
						Line:        param.Name.Token.Line,
						Column:      param.Name.Token.Column,
						Message:     fmt.Sprintf("El parámetro '$%s' requiere un tipo explícito (o mixed).", param.Name.Value),
						Suggestion:  fmt.Sprintf("Declara el tipo explícito, por ejemplo: 'mixed $%s'.", param.Name.Value),
						AutoFixable: true,
					})
				}
			}
		case *parser.ClassStatement:
			// Rule: Class names must be PascalCase
			if s.Name != nil && len(s.Name.Value) > 0 {
				first := rune(s.Name.Value[0])
				if first >= 'a' && first <= 'z' {
					issues = append(issues, LintIssue{
						RuleID:      "JOSS-LINT-007",
						Category:    CategoryStyle,
						Severity:    diagnostics.SeverityWarning,
						File:        filename,
						Line:        s.Name.Token.Line,
						Column:      s.Name.Token.Column,
						Message:     fmt.Sprintf("La clase '%s' debería usar convención PascalCase.", s.Name.Value),
						Suggestion:  fmt.Sprintf("Renombra la clase a '%s%s'.", strings.ToUpper(string(first)), s.Name.Value[1:]),
						AutoFixable: false,
					})
				}
			}
		}
	}

	// Security: Check for hardcoded API keys or high-entropy credentials
	lines := strings.Split(src, "\n")
	for idx, line := range lines {
		lower := strings.ToLower(line)
		if (strings.Contains(lower, "password = \"") || strings.Contains(lower, "secret = \"") || strings.Contains(lower, "apikey = \"")) && !strings.Contains(line, "System::env") && !strings.Contains(line, "Env::") {
			issues = append(issues, LintIssue{
				RuleID:      "JOSS-SEC-001",
				Category:    CategorySecurity,
				Severity:    diagnostics.SeverityWarning,
				File:        filename,
				Line:        idx + 1,
				Column:      1,
				Message:     "Posible credencial o secreto sensible codificado en duro.",
				Explanation: "Las credenciales no deben almacenarse en el código fuente.",
				Suggestion:  "Usa System::env(\"VARIABLE\") para cargar credenciales desde el entorno.",
				AutoFixable: false,
			})
		}
	}

	return issues
}

func isViewTemplateFile(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	if strings.HasSuffix(lower, ".joss.html") {
		return true
	}
	if strings.HasSuffix(lower, ".html") && (strings.Contains(lower, "/views/") || strings.Contains(lower, "/app/views/")) {
		return true
	}
	return false
}

func (l *Linter) LintViewSource(filename, content string) []LintIssue {
	var issues []LintIssue

	// Pre-process Blade comments
	reBladeComments := regexp.MustCompile(`\{\{--[\s\S]*?--\}\}`)
	cleanHtml := reBladeComments.ReplaceAllString(content, "")

	// Pre-process directives through the syntax shared with runtime rendering.
	cleanHtml, directiveErr := viewtemplate.RewriteJSON(cleanHtml)
	if directiveErr != nil {
		return []LintIssue{{RuleID: "JOSS-VIEW-001", Category: CategoryCorrectness, Severity: diagnostics.SeverityError, File: filename, Line: 1, Message: fmt.Sprintf("Directiva de vista invalida: %v", directiveErr), Explanation: "La directiva no cierra correctamente sus argumentos.", Suggestion: "Cierra los parentesis y comillas de la directiva."}}
	}

	// Pre-process csrf_field() to be raw output
	reCsrfPre := regexp.MustCompile(`\{\{\s*csrf_field\(\)\s*\}\}`)
	cleanHtml = reCsrfPre.ReplaceAllString(cleanHtml, `{{! csrf_field() }}`)

	// 1. Check template compilation
	jossScript, errCompile := core.CompileViewToJOSS(cleanHtml)
	if errCompile != nil {
		issues = append(issues, LintIssue{
			RuleID:      "JOSS-VIEW-001",
			Category:    CategoryCorrectness,
			Severity:    diagnostics.SeverityError,
			File:        filename,
			Line:        1,
			Message:     fmt.Sprintf("Error compilando plantilla de vista: %v", errCompile),
			Explanation: "La plantilla contiene etiquetas o directivas de plantilla mal formadas.",
			Suggestion:  "Verifica que @foreach, ternarios de bloque {{ (...) ? { ... } : { ... } }} y llaves cierren correctamente.",
		})
		return issues
	}

	// 2. Check script syntax
	p := parser.NewParser(parser.NewLexer(jossScript))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		for _, errStr := range p.Errors() {
			issues = append(issues, LintIssue{
				RuleID:      "JOSS-VIEW-SYNTAX",
				Category:    CategoryCorrectness,
				Severity:    diagnostics.SeverityError,
				File:        filename,
				Line:        1,
				Message:     fmt.Sprintf("Error de sintaxis en expresión de vista: %s", errStr),
				Explanation: "Una expresión incrustada en la plantilla no es código Joss válido.",
				Suggestion:  "Verifica la sintaxis de las expresiones dentro de {{ ... }} o directivas de la plantilla.",
			})
		}
		return issues
	}

	// 3. Inspect AST for raw variable evaluation warnings
	guaranteedGlobals := map[string]bool{
		"auth_check": true, "auth_guest": true, "auth_user": true, "auth_role": true, "auth_email": true,
		"csrf_token": true, "success": true, "error": true, "__output": true, "__session": true, "__request": true,
		"true": true, "false": true, "nil": true, "null": true,
	}

	walkAST(prog, func(node parser.Node) {
		if tern, ok := node.(*parser.TernaryExpression); ok {
			if ident, ok := tern.Condition.(*parser.Identifier); ok {
				varName := strings.TrimPrefix(ident.Value, "$")
				if !guaranteedGlobals[varName] && !core.IsNativeClass(ident.Value) && !core.IsNativeClass(varName) {
					issues = append(issues, LintIssue{
						RuleID:      "JOSS-VIEW-UNDEF",
						Category:    CategoryCorrectness,
						Severity:    diagnostics.SeverityWarning,
						File:        filename,
						Line:        ident.Token.Line,
						Column:      ident.Token.Column,
						Message:     fmt.Sprintf("La variable '$%s' se evalúa directamente como condición en la vista.", varName),
						Explanation: fmt.Sprintf("Si el controlador no pasa '$%s' en view(), causará un error 500 en tiempo de ejecución.", varName),
						Suggestion:  fmt.Sprintf("Usa 'isset($%s)' o '(isset($%s) && $%s)' para verificar su existencia de forma segura.", varName, varName, varName),
					})
				}
			}
		}
	})

	return issues
}

func walkAST(node parser.Node, fn func(parser.Node)) {
	if node == nil {
		return
	}
	fn(node)
	switch n := node.(type) {
	case *parser.Program:
		for _, stmt := range n.Statements {
			walkAST(stmt, fn)
		}
	case *parser.ExpressionStatement:
		walkAST(n.Expression, fn)
	case *parser.BlockStatement:
		for _, stmt := range n.Statements {
			walkAST(stmt, fn)
		}
	case *parser.TernaryExpression:
		walkAST(n.Condition, fn)
		walkAST(n.True, fn)
		walkAST(n.False, fn)
	case *parser.ForeachStatement:
		walkAST(n.Iterable, fn)
		walkAST(n.Body, fn)
	case *parser.InfixExpression:
		walkAST(n.Left, fn)
		walkAST(n.Right, fn)
	case *parser.PrefixExpression:
		walkAST(n.Right, fn)
	case *parser.CallExpression:
		walkAST(n.Function, fn)
		for _, arg := range n.Arguments {
			walkAST(arg, fn)
		}
	case *parser.AssignExpression:
		walkAST(n.Left, fn)
		walkAST(n.Value, fn)
	case *parser.IndexExpression:
		walkAST(n.Left, fn)
		walkAST(n.Index, fn)
	case *parser.MemberExpression:
		walkAST(n.Left, fn)
		walkAST(n.Property, fn)
	case *parser.ReturnStatement:
		walkAST(n.ReturnValue, fn)
	case *parser.LetStatement:
		walkAST(n.Value, fn)
	}
}
