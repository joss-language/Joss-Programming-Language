package analyzer

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func parseUnit(t *testing.T, path, code string) SourceUnit {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(code))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors in %s: %v", path, p.Errors())
	}
	return SourceUnit{Path: path, Program: program}
}

func TestAbstractClassAnalyzer(t *testing.T) {
	t.Run("instantiating abstract class produces JOSS-DECL-006", func(t *testing.T) {
		unit := parseUnit(t, "main.joss", `
public abstract class Animal {
    public abstract func hablar(): string;
}

public class Main {
    Init main() {
        $a = new Animal();
    }
}
`)
		issues := Analyze([]SourceUnit{unit}, NewEnvironment())
		found := false
		for _, diag := range issues {
			if diag.Code == "JOSS-DECL-006" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected JOSS-DECL-006 when instantiating abstract class, got %v", issues)
		}
	})

	t.Run("missing abstract method implementation in subclass produces JOSS-DECL-005", func(t *testing.T) {
		unit := parseUnit(t, "main.joss", `
public abstract class Animal {
    public abstract func hablar(): string;
}

public class Perro extends Animal {
}
`)
		issues := Analyze([]SourceUnit{unit}, NewEnvironment())
		found := false
		for _, diag := range issues {
			if diag.Code == "JOSS-DECL-005" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected JOSS-DECL-005 when subclass does not implement abstract method, got %v", issues)
		}
	})

	t.Run("concrete subclass correctly implementing abstract method produces no issues", func(t *testing.T) {
		unit := parseUnit(t, "main.joss", `
public abstract class Animal {
    public abstract func hablar(): string;
}

public class Perro extends Animal {
    public func hablar(): string {
        return "guau";
    }
}

public class Main {
    Init main() {
        $p = new Perro();
        $str = $p->hablar();
    }
}
`)
		issues := Analyze([]SourceUnit{unit}, NewEnvironment())
		for _, diag := range issues {
			if diag.Severity == "error" {
				t.Fatalf("unexpected error: %v", diag)
			}
		}
	})
}

func TestEnumAnalyzer(t *testing.T) {
	t.Run("valid enum and case member access", func(t *testing.T) {
		unit := parseUnit(t, "main.joss", `
public enum Status {
    case Pending;
    case Done;
}

public class Main {
    Init main() {
        $s = Status::Pending;
        $arr = Status::cases();
    }
}
`)
		issues := Analyze([]SourceUnit{unit}, NewEnvironment())
		for _, diag := range issues {
			if diag.Severity == "error" {
				t.Fatalf("unexpected error in enum analyzer: %v", diag)
			}
		}
	})

	t.Run("duplicate case produces JOSS-DECL-003", func(t *testing.T) {
		unit := parseUnit(t, "main.joss", `
public enum Role {
    case Admin;
    case Admin;
}
`)
		issues := Analyze([]SourceUnit{unit}, NewEnvironment())
		found := false
		for _, diag := range issues {
			if diag.Code == "JOSS-DECL-003" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected JOSS-DECL-003 for duplicate case, got %v", issues)
		}
	})
}
