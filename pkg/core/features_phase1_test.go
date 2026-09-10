package core

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func executeCode(t *testing.T, code string) *Runtime {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	r := NewRuntime()
	r.Execute(prog)
	return r
}

func TestRuntimeIsAndInstanceOf(t *testing.T) {
	code := `
public interface IIdentificable {}

public class Entidad implements IIdentificable {}

public enum Estado {
    case Activo;
    case Inactivo;
}

$num = 42;
$txt = "hola";
$e = new Entidad();
$st = Estado::Activo;

$t1 = ($num is int);
$t2 = ($num is string);
$t3 = ($txt is string);
$t4 = ($e is Entidad);
$t5 = ($e is IIdentificable);
$t6 = ($e instanceof Entidad);
$t7 = ($st is Estado);
`
	r := executeCode(t, code)

	tests := []struct {
		field string
		want  bool
	}{
		{"t1", true},
		{"t2", false},
		{"t3", true},
		{"t4", true},
		{"t5", true},
		{"t6", true},
		{"t7", true},
	}

	for _, tt := range tests {
		val := r.Variables[tt.field]
		if val != tt.want {
			t.Errorf("expected $%s = %v, got %v", tt.field, tt.want, val)
		}
	}
}

func TestRuntimeAbstractClass(t *testing.T) {
	t.Run("instantiating abstract class panics with InstantiationError", func(t *testing.T) {
		code := `
public abstract class Base {
    public abstract func saludar(): string;
}

$b = new Base();
`
		defer func() {
			rec := recover()
			if rec == nil {
				t.Fatalf("expected panic when instantiating abstract class")
			}
			jerr, ok := rec.(*JossError)
			if !ok || jerr.Type != "InstantiationError" {
				t.Fatalf("expected InstantiationError, got %v", rec)
			}
		}()

		executeCode(t, code)
	})

	t.Run("subclass implements abstract method and executes successfully", func(t *testing.T) {
		code := `
public abstract class Vehiculo {
    public abstract func tipo(): string;

    public func claxon(): string {
        return "beep beep";
    }
}

public class Auto extends Vehiculo {
    public func tipo(): string {
        return "auto";
    }
}

$a = new Auto();
$resultadoTipo = $a->tipo();
$resultadoClaxon = $a->claxon();
`
		r := executeCode(t, code)
		if r.Variables["resultadoTipo"] != "auto" {
			t.Errorf("expected 'auto', got %v", r.Variables["resultadoTipo"])
		}
		if r.Variables["resultadoClaxon"] != "beep beep" {
			t.Errorf("expected 'beep beep', got %v", r.Variables["resultadoClaxon"])
		}
	})
}

func TestRuntimeEnums(t *testing.T) {
	code := `
public enum Prioridad {
    case Baja;
    case Media;
    case Alta;
}

public enum CodigoHTTP: int {
    case OK = 200;
    case NotFound = 404;
}

$p = Prioridad::Alta;
$casoNombre = $p->name;
$casoValor = $p->value;

$casos = Prioridad::cases();
$totalCasos = count($casos);

$http = CodigoHTTP::from(200);
$fromCaso = $http->name;

$inexistente = CodigoHTTP::tryFrom(999);
$tryFromVal = $inexistente;

$comparacionIgual = (Prioridad::Alta == Prioridad::Alta);
$comparacionDistinto = (Prioridad::Alta == Prioridad::Baja);
`
	r := executeCode(t, code)

	if r.Variables["casoNombre"] != "Alta" {
		t.Errorf("expected Alta, got %v", r.Variables["casoNombre"])
	}
	if r.Variables["casoValor"] != "Alta" {
		t.Errorf("expected Alta value, got %v", r.Variables["casoValor"])
	}
	if n, ok := r.Variables["totalCasos"].(int64); !ok || n != 3 {
		t.Errorf("expected 3 cases, got %v (%T)", r.Variables["totalCasos"], r.Variables["totalCasos"])
	}
	if r.Variables["fromCaso"] != "OK" {
		t.Errorf("expected OK, got %v", r.Variables["fromCaso"])
	}
	if r.Variables["tryFromVal"] != nil {
		t.Errorf("expected nil for tryFrom(999), got %v", r.Variables["tryFromVal"])
	}
	if r.Variables["comparacionIgual"] != true {
		t.Errorf("expected true for Prioridad::Alta == Prioridad::Alta, got %v", r.Variables["comparacionIgual"])
	}
	if r.Variables["comparacionDistinto"] != false {
		t.Errorf("expected false for Prioridad::Alta == Prioridad::Baja, got %v", r.Variables["comparacionDistinto"])
	}
}
