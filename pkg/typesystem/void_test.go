package typesystem

import "testing"

func TestVoidIsCanonicalKnownType(t *testing.T) {
	parsed := Parse("void")
	if parsed.Kind != Void || parsed.String() != "void" || !parsed.IsKnown() {
		t.Fatalf("Parse(void) = %#v", parsed)
	}
	if Assignable(parsed, Type{Kind: Null}) {
		t.Fatal("null must not be assignable to void")
	}
}
