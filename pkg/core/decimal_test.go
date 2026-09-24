package core

import (
	"encoding/json"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/shopspring/decimal"
)

func evalDecimalSource(source string) *Runtime {
	l := parser.NewLexer(source)
	p := parser.NewParser(l)
	program := p.ParseProgram()
	rt := NewRuntime()
	rt.Execute(program)
	return rt
}

func TestDecimalRepeatedSumsTaxesAndDivisionRemainExact(t *testing.T) {
	source := `
decimal $repeated = 0.10m + 0.10m + 0.10m + 0.10m + 0.10m + 0.10m + 0.10m + 0.10m + 0.10m + 0.10m
decimal $tax = 19.99m * 0.16m
decimal $share = 57.50m / 2m
decimal $aggregated = sum([0.10m, 0.20m, 1, 2.70m])
`
	runtime := evalDecimalSource(source)
	tests := map[string]string{
		"repeated":   "1.00",
		"tax":        "3.1984",
		"share":      "28.75",
		"aggregated": "4.00",
	}
	for name, expected := range tests {
		value, ok := runtime.Variables[name].(decimal.Decimal)
		if !ok || !value.Equal(decimal.RequireFromString(expected)) {
			t.Errorf("%s = %#v, want exact decimal %s", name, runtime.Variables[name], expected)
		}
	}
}

func TestDecimalJSONSerializationPreservesDecimalDigits(t *testing.T) {
	runtime := benchmarkRuntimeInstance()
	encoded, handled := runtime.callBuiltin("json_encode", []interface{}{map[string]interface{}{
		"amount": decimal.RequireFromString("9007199254740993.01"),
	}})
	if !handled {
		t.Fatal("json_encode was not handled")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(encoded.(string)), &raw); err != nil {
		t.Fatal(err)
	}
	if got := string(raw["amount"]); got != `"9007199254740993.01"` {
		t.Fatalf("serialized decimal = %s, want quoted exact digits", got)
	}
}

func TestSumRejectsOverflowAndMixedFloatDecimal(t *testing.T) {
	runtime := benchmarkRuntimeInstance()
	for _, values := range [][]interface{}{
		{int64(9223372036854775807), int64(1)},
		{decimal.RequireFromString("0.10"), float64(0.2)},
		{int64(9007199254740993), float64(1)},
	} {
		func() {
			defer func() {
				if recovered := recover(); recovered == nil {
					t.Fatalf("sum(%#v) should fail safely", values)
				}
			}()
			runtime.callBuiltin("sum", []interface{}{values})
		}()
	}
}

func TestDecimalExactArithmetic(t *testing.T) {
	source := `
decimal $a = 0.10m
decimal $b = 0.20m
decimal $c = $a + $b
$exactMatch = ($c == 0.30m)
`
	p := evalDecimalSource(source)
	match, ok := p.Variables["exactMatch"].(bool)
	if !ok || !match {
		t.Fatalf("expected exact match 0.10m + 0.20m == 0.30m, got %v", p.Variables["c"])
	}
}

func TestDecimalBankingCalculation(t *testing.T) {
	source := `
decimal $saldo = 0.60m
decimal $precio = 0.10m + 0.20m
$puedeComprar = ($saldo >= $precio)
decimal $resto = $saldo - $precio
$esTreinta = ($resto == 0.30m)
`
	p := evalDecimalSource(source)
	puedeComprar, ok := p.Variables["puedeComprar"].(bool)
	if !ok || !puedeComprar {
		t.Fatalf("expected puedeComprar == true")
	}
	esTreinta, ok := p.Variables["esTreinta"].(bool)
	if !ok || !esTreinta {
		t.Fatalf("expected $resto == 0.30m, got %v", p.Variables["resto"])
	}
}

func TestDecimalBuiltinsAndCoercion(t *testing.T) {
	source := `
$val = decimal("45.67")
$isDec = is_decimal($val)
$isNum = is_numeric($val)
decimal $fromInt = 100
$exact = ($fromInt == 100.0m)
$str = "Total: " . $val
`
	p := evalDecimalSource(source)
	if isDec, ok := p.Variables["isDec"].(bool); !ok || !isDec {
		t.Fatalf("expected is_decimal to be true")
	}
	if isNum, ok := p.Variables["isNum"].(bool); !ok || !isNum {
		t.Fatalf("expected is_numeric to be true")
	}
	if exact, ok := p.Variables["exact"].(bool); !ok || !exact {
		t.Fatalf("expected decimal from int coercion equality")
	}
	if str, ok := p.Variables["str"].(string); !ok || str != "Total: 45.67" {
		t.Fatalf("expected string concatenation 'Total: 45.67', got %q", str)
	}
}

func TestDecimalOperationsAndComparisons(t *testing.T) {
	source := `
decimal $x = 10.50m
decimal $y = 2.0m
$mul = $x * $y
$div = $x / $y
$neg = -$x
$cmpSpaceship = ($x <=> $y)
$cmpLess = ($y < $x)
$cmpGreater = ($x > $y)
`
	p := evalDecimalSource(source)
	mulVal, ok := p.Variables["mul"].(decimal.Decimal)
	if !ok || !mulVal.Equal(decimal.NewFromFloat(21.0)) {
		t.Fatalf("expected 21.0, got %v", p.Variables["mul"])
	}
	divVal, ok := p.Variables["div"].(decimal.Decimal)
	if !ok || !divVal.Equal(decimal.NewFromFloat(5.25)) {
		t.Fatalf("expected 5.25, got %v", p.Variables["div"])
	}
	negVal, ok := p.Variables["neg"].(decimal.Decimal)
	if !ok || !negVal.Equal(decimal.NewFromFloat(-10.50)) {
		t.Fatalf("expected -10.50, got %v", p.Variables["neg"])
	}
	cmp, ok := p.Variables["cmpSpaceship"].(int64)
	if !ok || cmp != 1 {
		t.Fatalf("expected spaceship 1, got %v", p.Variables["cmpSpaceship"])
	}
	if less, ok := p.Variables["cmpLess"].(bool); !ok || !less {
		t.Fatalf("expected cmpLess true")
	}
	if greater, ok := p.Variables["cmpGreater"].(bool); !ok || !greater {
		t.Fatalf("expected cmpGreater true")
	}
}
