package linter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jossecurity/joss/pkg/diagnostics"
)

func TestLinterDetectsUntypedParams(t *testing.T) {
	src := `public func compute($a, int $b): int {
    return $b;
}
`
	linter := NewLinter()
	issues, err := linter.LintSource("test.joss", src)
	if err != nil {
		t.Fatalf("lint error: %v", err)
	}

	found := false
	for _, issue := range issues {
		if issue.RuleID == "JOSS-LINT-002" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected JOSS-LINT-002 for untyped param, got: %v", issues)
	}
}

func TestLinterReportsFiniteMigrationWarnings(t *testing.T) {
	issues, err := NewLinter().LintSource("legacy.joss", "let $value = nil\n$other = 1\npublic func old() { return; }")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"JOSS-DECL-006": false, "JOSS-DECL-007": false, "JOSS-DECL-010": false, "JOSS-TYPE-014": false}
	for _, issue := range issues {
		if _, ok := want[issue.RuleID]; ok {
			if issue.Severity != diagnostics.SeverityWarning {
				t.Fatalf("%s severity = %s", issue.RuleID, issue.Severity)
			}
			want[issue.RuleID] = true
		}
	}
	for code, found := range want {
		if !found {
			t.Fatalf("missing %s in %#v", code, issues)
		}
	}
}

func TestLintPathAnalyzesDirectoryAsOneProject(t *testing.T) {
	project := t.TempDir()
	files := map[string]string{
		"Service.joss": `public class Service {
    public static func value(): string {
        return "ok"
    }
}`,
		"Controller.joss": `public class Controller {
    public func index(): string {
        return Service::value()
    }
}`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(project, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	issues, err := NewLinter().LintPath(project)
	if err != nil {
		t.Fatalf("lint directory: %v", err)
	}
	for _, issue := range issues {
		if issue.RuleID == "JOSS-SYM-004" || issue.RuleID == "JOSS-SYM-001" {
			t.Fatalf("cross-file symbol produced a false positive: %s", issue.String())
		}
	}
}

func TestLinterDetectsHardcodedSecret(t *testing.T) {
	src := `public func connect() {
    $apiKey = "secret_key_12345"
}
`
	linter := NewLinter()
	issues, err := linter.LintSource("test.joss", src)
	if err != nil {
		t.Fatalf("lint error: %v", err)
	}

	found := false
	for _, issue := range issues {
		if issue.RuleID == "JOSS-SEC-001" {
			found = true
			if issue.Severity != diagnostics.SeverityWarning {
				t.Fatalf("expected warning severity, got %v", issue.Severity)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected JOSS-SEC-001 for hardcoded secret, got: %v", issues)
	}
}

func TestLinterValidatesViewTemplates(t *testing.T) {
	l := NewLinter()

	// 1. Valid view template
	validTemplate := `<div class="container">
		{{ (isset($success) && $success) ? {
			<div class="alert">{{ $success }}</div>
		} : {} }}
		<h1>{{ $title }}</h1>
	</div>`
	issues := l.LintViewSource("test.joss.html", validTemplate)
	for _, issue := range issues {
		if issue.Severity == diagnostics.SeverityError {
			t.Fatalf("unexpected error in valid template: %s", issue.Message)
		}
	}

	// 2. Syntax error in view expression
	syntaxErrTemplate := `<div>{{ $foo + }}</div>`
	issues = l.LintViewSource("syntax_err.joss.html", syntaxErrTemplate)
	foundSyntaxErr := false
	for _, issue := range issues {
		if issue.RuleID == "JOSS-VIEW-SYNTAX" {
			foundSyntaxErr = true
			break
		}
	}
	if !foundSyntaxErr {
		t.Fatalf("expected JOSS-VIEW-SYNTAX error, got: %v", issues)
	}

	// 3. Raw undeclared variable warning in ternary
	rawVarTemplate := `<div>
		{{ ($customVar) ? {
			<p>{{ $customVar }}</p>
		} : {} }}
	</div>`
	issues = l.LintViewSource("raw_var.joss.html", rawVarTemplate)
	foundRawVarWarn := false
	for _, issue := range issues {
		if issue.RuleID == "JOSS-VIEW-UNDEF" {
			foundRawVarWarn = true
			break
		}
	}
	if !foundRawVarWarn {
		t.Fatalf("expected JOSS-VIEW-UNDEF warning, got: %v", issues)
	}
}
