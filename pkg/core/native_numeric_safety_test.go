package core

import (
	"math"
	"testing"
)

func TestMathRandomSupportsFullInt64RangeWithoutHostPanic(t *testing.T) {
	r := NewRuntime()
	defer r.Free()
	for index := 0; index < 32; index++ {
		if _, ok := r.executeMathMethod(nil, "random", []interface{}{int64(math.MinInt64), int64(math.MaxInt64)}).(int64); !ok {
			t.Fatal("Math.random did not return int64")
		}
	}
}

func TestNativeStringAndRandomRangesRejectNegativeLengths(t *testing.T) {
	r := NewRuntime()
	defer r.Free()
	checks := []func(){
		func() { r.executeMathMethod(nil, "random", []interface{}{int64(2), int64(1)}) },
	}
	for index, check := range checks {
		func() {
			defer func() {
				raw := recover()
				recovered, ok := raw.(*JossError)
				if !ok || recovered.Type != "ValueError" {
					t.Fatalf("case %d: expected ValueError, got %#v", index, raw)
				}
			}()
			check()
		}()
	}
}
