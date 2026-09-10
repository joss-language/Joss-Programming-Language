package parser

import (
	"testing"
)

func TestParseInterfaceStatement(t *testing.T) {
	input := `
public interface Imprimible {
    public func imprimir(): string;
    public func obtenerFormato(): string;
}

public interface Serializable extends Imprimible {
    public func toJson(): string;
}

public class Factura extends Documento implements Imprimible, Serializable {
    public func imprimir(): string {
        return "factura";
    }
}
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

	// 1. Check first interface
	iface1, ok := prog.Statements[0].(*InterfaceStatement)
	if !ok {
		t.Fatalf("stmt[0] expected *InterfaceStatement, got %T", prog.Statements[0])
	}
	if iface1.Name.Value != "Imprimible" || iface1.Visibility != "public" {
		t.Errorf("expected public interface Imprimible, got %s interface %s", iface1.Visibility, iface1.Name.Value)
	}
	if len(iface1.Methods) != 2 {
		t.Fatalf("expected 2 methods in Imprimible, got %d", len(iface1.Methods))
	}
	if iface1.Methods[0].Name.Value != "imprimir" || iface1.Methods[0].ReturnType.Literal != "string" {
		t.Errorf("expected func imprimir(): string, got %s", iface1.Methods[0].Name.Value)
	}

	// 2. Check second interface with extends
	iface2, ok := prog.Statements[1].(*InterfaceStatement)
	if !ok {
		t.Fatalf("stmt[1] expected *InterfaceStatement, got %T", prog.Statements[1])
	}
	if len(iface2.Extends) != 1 || iface2.Extends[0].Value != "Imprimible" {
		t.Errorf("expected Serializable to extend Imprimible")
	}

	// 3. Check class with implements
	classStmt, ok := prog.Statements[2].(*ClassStatement)
	if !ok {
		t.Fatalf("stmt[2] expected *ClassStatement, got %T", prog.Statements[2])
	}
	if classStmt.SuperClass == nil || classStmt.SuperClass.Value != "Documento" {
		t.Errorf("expected superclass Documento")
	}
	if len(classStmt.Interfaces) != 2 {
		t.Fatalf("expected 2 interfaces implemented, got %d", len(classStmt.Interfaces))
	}
	if classStmt.Interfaces[0].Value != "Imprimible" || classStmt.Interfaces[1].Value != "Serializable" {
		t.Errorf("expected Imprimible and Serializable, got %v", classStmt.Interfaces)
	}
}

func TestParseInterfaceNegative(t *testing.T) {
	// Interface without visibility
	input1 := `interface SinVisibilidad {}`
	p1 := NewParser(NewLexer(input1))
	p1.ParseProgram()
	if len(p1.Errors()) == 0 {
		t.Errorf("expected error for interface without visibility")
	}

	// Interface method with body
	input2 := `
public interface ConCuerpo {
    public func metodo(): void {
        return;
    }
}
`
	p2 := NewParser(NewLexer(input2))
	p2.ParseProgram()
	if len(p2.Errors()) == 0 {
		t.Errorf("expected error for interface method with body")
	}
}
