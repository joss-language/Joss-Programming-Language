package core

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestTruthinessUsesSemanticCategories(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{"null", nil, false},
		{"false", false, false},
		{"true", true, true},
		{"integer zero", int64(0), false},
		{"float zero", float64(0), false},
		{"decimal zero", decimal.Zero, false},
		{"integer nonzero", int64(-1), true},
		{"float nonzero", float64(0.5), true},
		{"decimal nonzero", decimal.RequireFromString("0.01"), true},
		{"empty string", "", false},
		{"nonempty zero string", "0", true},
		{"empty array", []interface{}{}, false},
		{"nonempty array", []interface{}{nil}, true},
		{"empty map", map[string]interface{}{}, false},
		{"nonempty map", map[string]interface{}{"x": nil}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isTruthy(test.value); got != test.want {
				t.Fatalf("isTruthy(%#v) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}
