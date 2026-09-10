package analyzer

import (
	"testing"
)

func TestAnalyzerConstructorPropertyPromotion(t *testing.T) {
	code := `
public class Producto {
    Init(public string $titulo, private int $precio = 100) {
    }

    public func getTitulo(): string {
        return $this->titulo;
    }

    public func getPrecio(): int {
        return $this->precio;
    }
}
`
	unit := parseUnit(t, "producto.joss", code)
	issues := Analyze([]SourceUnit{unit}, NewEnvironment())
	for _, issue := range issues {
		t.Errorf("unexpected diagnostic: %s [%s] line %d", issue.Message, issue.Code, issue.Range.Start.Line)
	}
}

func TestAnalyzerNamedArguments(t *testing.T) {
	code := `
public func configurar(string $host, int $puerto = 8080, bool $ssl = false): string {
    return $host;
}

public class Main {
    Init main() {
        $c1 = configurar(host: "localhost");
        $c2 = configurar(host: "127.0.0.1", ssl: true, puerto: 443);
        echo($c1);
        echo($c2);
    }
}
`
	unit := parseUnit(t, "main.joss", code)
	issues := Analyze([]SourceUnit{unit}, NewEnvironment())
	for _, issue := range issues {
		t.Errorf("unexpected diagnostic: %s [%s] line %d", issue.Message, issue.Code, issue.Range.Start.Line)
	}
}

func TestAnalyzerNamedArgumentUnknownThrowsDiagnostic(t *testing.T) {
	code := `
public func sumar(int $a, int $b): int {
    return $a + $b;
}

public class Main {
    Init main() {
        $r = sumar(a: 10, desconocido: 20);
        echo($r);
    }
}
`
	unit := parseUnit(t, "main.joss", code)
	issues := Analyze([]SourceUnit{unit}, NewEnvironment())
	found := false
	for _, issue := range issues {
		if issue.Code == "JOSS-CALL-001" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected JOSS-CALL-001 diagnostic for unknown named parameter, got %v", issues)
	}
}
