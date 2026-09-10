package core

import (
	"testing"
)

func TestRuntimeSpreadOperator(t *testing.T) {
	code := `
$parte = [2, 3];
$combinado = [1, ...$parte, 4];

public func sumarTres(int $a, int $b, int $c): int {
    return $a + $b + $c;
}

$args = [10, 20, 30];
$suma = sumarTres(...$args);
`
	r := executeCode(t, code)

	comb, ok := r.Variables["combinado"].([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", r.Variables["combinado"])
	}
	if len(comb) != 4 {
		t.Fatalf("expected length 4, got %d", len(comb))
	}
	expected := []int64{1, 2, 3, 4}
	for i, v := range expected {
		if comb[i] != v {
			t.Errorf("at index %d: expected %d, got %v", i, v, comb[i])
		}
	}

	if r.Variables["suma"] != int64(60) {
		t.Errorf("expected suma = 60, got %v", r.Variables["suma"])
	}
}

func TestRuntimeNamedArguments(t *testing.T) {
	code := `
public func construirUrl(string $protocolo, string $dominio, int $puerto = 80, string $ruta = "/"): string {
    return $protocolo . "://" . $dominio . ":" . $puerto . $ruta;
}

$url1 = construirUrl(dominio: "google.com", protocolo: "https");
$url2 = construirUrl("http", "localhost", ruta: "/api", puerto: 8080);
`
	r := executeCode(t, code)

	if r.Variables["url1"] != "https://google.com:80/" {
		t.Errorf("expected 'https://google.com:80/', got %v", r.Variables["url1"])
	}
	if r.Variables["url2"] != "http://localhost:8080/api" {
		t.Errorf("expected 'http://localhost:8080/api', got %v", r.Variables["url2"])
	}
}

func TestRuntimeConstructorPropertyPromotion(t *testing.T) {
	code := `
public class Usuario {
    Init(public string $nombre, private int $edad = 18) {
    }

    public func getEdad(): int {
        return $this->edad;
    }
}

$u1 = new Usuario("Ada", 25);
$nombre1 = $u1->nombre;
$edad1 = $u1->getEdad();

$u2 = new Usuario(nombre: "Grace");
$nombre2 = $u2->nombre;
$edad2 = $u2->getEdad();
`
	r := executeCode(t, code)

	if r.Variables["nombre1"] != "Ada" {
		t.Errorf("expected 'Ada', got %v", r.Variables["nombre1"])
	}
	if r.Variables["edad1"] != int64(25) {
		t.Errorf("expected 25, got %v", r.Variables["edad1"])
	}
	if r.Variables["nombre2"] != "Grace" {
		t.Errorf("expected 'Grace', got %v", r.Variables["nombre2"])
	}
	if r.Variables["edad2"] != int64(18) {
		t.Errorf("expected 18, got %v", r.Variables["edad2"])
	}
}
