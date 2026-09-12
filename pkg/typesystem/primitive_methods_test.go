package typesystem

import "testing"

func TestPrimitiveMethodReturnProjection(t *testing.T) {
	element := Type{Kind: String}
	receiver := Type{Kind: Array, Element: &element}
	_, result, ok := PrimitiveMethod(receiver, "first")
	if !ok || result != element {
		t.Fatalf("array<string>.first = (%v, %v), want string", result, ok)
	}
	if _, _, ok := PrimitiveMethod(receiver, "pop"); ok {
		t.Fatal("array.pop must not be advertised until the primitive runtime defines its semantics")
	}
}
