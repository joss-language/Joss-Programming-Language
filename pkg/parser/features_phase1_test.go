package parser

import (
	"testing"
)

func TestParseIsAndInstanceof(t *testing.T) {
	input := `
$res1 = $x is int;
$res2 = $obj instanceof Persona;
$res3 = $val is string|null;
`
	l := NewLexer(input)
	p := NewParser(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("unexpected parser errors: %v", p.Errors())
	}
	if len(prog.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(prog.Statements))
	}

	// 1. $x is int
	exprStmt1, ok := prog.Statements[0].(*ExpressionStatement)
	if !ok {
		t.Fatalf("stmt[0] expected *ExpressionStatement, got %T", prog.Statements[0])
	}
	assign1, ok := exprStmt1.Expression.(*AssignExpression)
	if !ok {
		t.Fatalf("expected assign, got %T", exprStmt1.Expression)
	}
	isExpr, ok := assign1.Value.(*IsExpression)
	if !ok {
		t.Fatalf("expected *IsExpression, got %T", assign1.Value)
	}
	if isExpr.Token.Literal != "is" || isExpr.TargetType.Literal != "int" {
		t.Errorf("unexpected IsExpression: %+v", isExpr)
	}

	// 2. $obj instanceof Persona
	exprStmt2 := prog.Statements[1].(*ExpressionStatement)
	assign2 := exprStmt2.Expression.(*AssignExpression)
	instExpr, ok := assign2.Value.(*IsExpression)
	if !ok {
		t.Fatalf("expected *IsExpression for instanceof, got %T", assign2.Value)
	}
	if instExpr.Token.Literal != "instanceof" || instExpr.TargetType.Literal != "Persona" {
		t.Errorf("unexpected instanceof expr: %+v", instExpr)
	}

	// 3. $val is string|null
	exprStmt3 := prog.Statements[2].(*ExpressionStatement)
	assign3 := exprStmt3.Expression.(*AssignExpression)
	unionIsExpr := assign3.Value.(*IsExpression)
	if unionIsExpr.TargetType.Literal != "string|null" {
		t.Errorf("expected string|null, got %s", unionIsExpr.TargetType.Literal)
	}
}

func TestParseAbstractClassAndMethods(t *testing.T) {
	input := `
public abstract class Figura {
    public abstract func calcularArea(): float;
    public func descripcion(): string {
        return "figura";
    }
}
`
	l := NewLexer(input)
	p := NewParser(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("unexpected parser errors: %v", p.Errors())
	}
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 class statement, got %d", len(prog.Statements))
	}

	classStmt, ok := prog.Statements[0].(*ClassStatement)
	if !ok {
		t.Fatalf("expected *ClassStatement, got %T", prog.Statements[0])
	}
	if !classStmt.IsAbstract || classStmt.Visibility != "public" || classStmt.Name.Value != "Figura" {
		t.Errorf("expected public abstract class Figura, got %+v", classStmt)
	}
	if len(classStmt.Body.Statements) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(classStmt.Body.Statements))
	}

	m1 := classStmt.Body.Statements[0].(*MethodStatement)
	if !m1.IsAbstract || m1.Name.Value != "calcularArea" || m1.ReturnType.Literal != "float" || m1.Body != nil {
		t.Errorf("expected abstract method without body, got %+v", m1)
	}

	m2 := classStmt.Body.Statements[1].(*MethodStatement)
	if m2.IsAbstract || m2.Name.Value != "descripcion" || m2.Body == nil {
		t.Errorf("expected concrete method with body, got %+v", m2)
	}
}

func TestParseEnumStatement(t *testing.T) {
	input := `
public enum Status {
    case Pending;
    case Approved;
    case Rejected;
}

public enum HttpStatus: int {
    case OK = 200;
    case NotFound = 404;
}
`
	l := NewLexer(input)
	p := NewParser(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("unexpected parser errors: %v", p.Errors())
	}
	if len(prog.Statements) != 2 {
		t.Fatalf("expected 2 enum statements, got %d", len(prog.Statements))
	}

	// 1. Status pure enum
	enum1, ok := prog.Statements[0].(*EnumStatement)
	if !ok {
		t.Fatalf("expected *EnumStatement, got %T", prog.Statements[0])
	}
	if enum1.Name.Value != "Status" || enum1.Visibility != "public" || enum1.BackingType.Literal != "" {
		t.Errorf("unexpected enum1: %+v", enum1)
	}
	if len(enum1.Cases) != 3 {
		t.Fatalf("expected 3 cases, got %d", len(enum1.Cases))
	}
	if enum1.Cases[0].Name.Value != "Pending" || enum1.Cases[0].Value != nil {
		t.Errorf("unexpected case 0: %+v", enum1.Cases[0])
	}

	// 2. HttpStatus backed enum
	enum2, ok := prog.Statements[1].(*EnumStatement)
	if !ok {
		t.Fatalf("expected *EnumStatement, got %T", prog.Statements[1])
	}
	if enum2.Name.Value != "HttpStatus" || enum2.BackingType.Literal != "int" {
		t.Errorf("unexpected enum2: %+v", enum2)
	}
	if len(enum2.Cases) != 2 {
		t.Fatalf("expected 2 cases in HttpStatus, got %d", len(enum2.Cases))
	}
	if enum2.Cases[0].Name.Value != "OK" || enum2.Cases[0].Value == nil {
		t.Errorf("expected OK case with value, got %+v", enum2.Cases[0])
	}
}
