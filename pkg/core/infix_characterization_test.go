package core

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestInfixDomainsCharacterization(t *testing.T) {
	runtime := executeCode(t, `
public func twice(int $value): int { return $value * 2 }
public func plus(int $value, int $amount): int { return $value + $amount }

$pipeline = 5 |> twice |> plus(3)
$elvis = "" ?: "fallback"
$ternary = true ? "yes" : "no"
$matchResult = match (2) { 1 => "one", 2 => "two", default => "other" }
$strictDifferent = (1 !== 1.0)
$spaceship = (3 <=> 2)
$decimalValue = 0.10m + 0.20m
$range = 3..1
int $counter = 4
$oldCounter = $counter++
`)

	assertRuntimeValue(t, runtime, "pipeline", int64(13))
	assertRuntimeValue(t, runtime, "elvis", "fallback")
	assertRuntimeValue(t, runtime, "ternary", "yes")
	assertRuntimeValue(t, runtime, "matchResult", "two")
	assertRuntimeValue(t, runtime, "strictDifferent", true)
	assertRuntimeValue(t, runtime, "spaceship", int64(1))
	assertRuntimeValue(t, runtime, "oldCounter", int64(4))
	assertRuntimeValue(t, runtime, "counter", int64(5))

	decimalValue, ok := runtime.Variables["decimalValue"].(decimal.Decimal)
	if !ok || !decimalValue.Equal(decimal.RequireFromString("0.30")) {
		t.Fatalf("decimalValue = %#v, want exact 0.30", runtime.Variables["decimalValue"])
	}
	rangeValue, ok := runtime.Variables["range"].([]interface{})
	if !ok || len(rangeValue) != 3 || rangeValue[0] != int64(3) || rangeValue[1] != int64(2) || rangeValue[2] != int64(1) {
		t.Fatalf("range = %#v, want [3 2 1]", runtime.Variables["range"])
	}
}

func assertRuntimeValue(t *testing.T, runtime *Runtime, name string, want interface{}) {
	t.Helper()
	if got := runtime.Variables[name]; got != want {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}
