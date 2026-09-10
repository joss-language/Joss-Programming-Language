package core

import (
	"testing"
)

func TestRuntimeSelectNonBlocking(t *testing.T) {
	code := `
$ch = make_chan(1);
$ejecutoDefault = false;

select {
    case $msg = recv($ch):
        $ejecutoDefault = false;
    default:
        $ejecutoDefault = true;
}
`
	r := executeCode(t, code)
	if r.Variables["ejecutoDefault"] != true {
		t.Errorf("expected ejecutoDefault = true, got %v", r.Variables["ejecutoDefault"])
	}
}

func TestRuntimeSelectSendAndRecv(t *testing.T) {
	code := `
$ch1 = make_chan(1);
$ch2 = make_chan(1);

send($ch1, "mensaje1");

$recibido = "";
select {
    case $msg = recv($ch1):
        $recibido = $msg;
    case $msg = recv($ch2):
        $recibido = "ch2";
    default:
        $recibido = "ninguno";
}
`
	r := executeCode(t, code)
	if r.Variables["recibido"] != "mensaje1" {
		t.Errorf("expected 'mensaje1', got %v", r.Variables["recibido"])
	}
}

func TestRuntimeGeneratorWithYield(t *testing.T) {
	code := `
public func contador(int $max): mixed {
    $i = 1;
    while ($i <= $max) {
        yield $i;
        $i++;
    }
}

$resultados = [];
foreach (contador(4) as $val) {
    $resultados[] = $val;
}
`
	r := executeCode(t, code)
	res, ok := r.Variables["resultados"].([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", r.Variables["resultados"])
	}
	if len(res) != 4 {
		t.Fatalf("expected length 4, got %d", len(res))
	}
	for i := 0; i < 4; i++ {
		if res[i] != int64(i+1) {
			t.Errorf("at index %d: expected %d, got %v", i, i+1, res[i])
		}
	}
}

func TestRuntimeGeneratorWithKeyValueYield(t *testing.T) {
	code := `
public func diccionario(): mixed {
    yield "a" => 10;
    yield "b" => 20;
    yield "c" => 30;
}

$claves = [];
$valores = [];
foreach (diccionario() as $k => $v) {
    $claves[] = $k;
    $valores[] = $v;
}
`
	r := executeCode(t, code)

	claves, ok := r.Variables["claves"].([]interface{})
	if !ok || len(claves) != 3 {
		t.Fatalf("expected 3 claves, got %v", r.Variables["claves"])
	}
	if claves[0] != "a" || claves[1] != "b" || claves[2] != "c" {
		t.Errorf("unexpected claves: %v", claves)
	}

	valores, ok := r.Variables["valores"].([]interface{})
	if !ok || len(valores) != 3 {
		t.Fatalf("expected 3 valores, got %v", r.Variables["valores"])
	}
	if valores[0] != int64(10) || valores[1] != int64(20) || valores[2] != int64(30) {
		t.Errorf("unexpected valores: %v", valores)
	}
}

func TestRuntimeGeneratorMethods(t *testing.T) {
	code := `
public func generadorSimple(): mixed {
    yield "primero";
    yield "segundo";
}

$gen = generadorSimple();
$gen->next();
$v1 = $gen->current();
$ok1 = $gen->valid();

$gen->next();
$v2 = $gen->current();
$ok2 = $gen->valid();

$gen->next();
$ok3 = $gen->valid();
`
	r := executeCode(t, code)

	if r.Variables["v1"] != "primero" {
		t.Errorf("expected 'primero', got %v", r.Variables["v1"])
	}
	if r.Variables["ok1"] != true {
		t.Errorf("expected ok1 = true, got %v", r.Variables["ok1"])
	}
	if r.Variables["v2"] != "segundo" {
		t.Errorf("expected 'segundo', got %v", r.Variables["v2"])
	}
	if r.Variables["ok2"] != true {
		t.Errorf("expected ok2 = true, got %v", r.Variables["ok2"])
	}
	if r.Variables["ok3"] != false {
		t.Errorf("expected ok3 = false, got %v", r.Variables["ok3"])
	}
}
