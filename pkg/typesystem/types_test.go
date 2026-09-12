package typesystem

import "testing"

func TestCanonicalNamesAndCompatibility(t *testing.T) {
	for _, removedAlias := range []string{"integer", "double", "boolean", "dynamic", "any", "list"} {
		if got := Parse(removedAlias); got.Kind != Class {
			t.Fatalf("removed alias %q resolved as %s, want unresolved class type", removedAlias, got.String())
		}
	}
	if !Assignable(Type{Kind: Float}, Type{Kind: Int}) {
		t.Fatal("int should be assignable to float")
	}
	if !Assignable(Type{Kind: Decimal}, Type{Kind: Int}) {
		t.Fatal("int should be assignable to decimal")
	}
	if !Assignable(Type{Kind: Decimal}, Type{Kind: Float}) {
		t.Fatal("float should be assignable to decimal")
	}
	if Assignable(Type{Kind: Int}, Type{Kind: Decimal}) {
		t.Fatal("decimal must not be assignable to int")
	}
	if Assignable(Type{Kind: Int}, Type{Kind: String}) {
		t.Fatal("string must not be assignable to int")
	}
	if !Assignable(Type{Kind: Mixed}, Type{Kind: String}) {
		t.Fatal("mixed is the explicit dynamic escape hatch")
	}
}

func TestInferenceWaitsForConcreteValue(t *testing.T) {
	unknown := Type{Kind: Unknown}
	if got := MergeInference(unknown, Type{Kind: Null}); got.Kind != Unknown {
		t.Fatalf("null should postpone inference, got %s", got.String())
	}
	if got := MergeInference(unknown, Type{Kind: Int}); got.Kind != Int {
		t.Fatalf("int should establish inference, got %s", got.String())
	}
}

func TestTypedStringCoercionIsLossless(t *testing.T) {
	if value, ok := CoerceString(Type{Kind: Int}, "20"); !ok || value != int64(20) {
		t.Fatalf("integer coercion = %v, %v", value, ok)
	}
	if _, ok := CoerceString(Type{Kind: Int}, "20.5"); ok {
		t.Fatal("fractional input must not be truncated into int")
	}
	if _, ok := CoerceString(Type{Kind: Int}, "twenty"); ok {
		t.Fatal("non-numeric input must not coerce to int")
	}
	if value, ok := CoerceString(Type{Kind: Int}, "9223372036854775808"); ok {
		t.Fatalf("overflowing integer was accepted as %v", value)
	}
	if value, ok := CoerceString(Type{Kind: Decimal}, "123.45m"); !ok || value != "123.45" {
		t.Fatalf("decimal coercion = %v, %v", value, ok)
	}
}

func TestUnionAndNullableCompatibility(t *testing.T) {
	nullable := Parse("int?")
	if nullable.Kind != Union || nullable.String() != "int|null" {
		t.Fatalf("nullable type = %#v", nullable)
	}
	if !Assignable(nullable, Type{Kind: Int}) || !Assignable(nullable, Type{Kind: Null}) {
		t.Fatal("nullable int must accept int and null")
	}
	if Assignable(nullable, Type{Kind: String}) {
		t.Fatal("nullable int must reject string")
	}
	if !Assignable(Parse("int|string"), Parse("int|string")) || Assignable(Parse("int"), Parse("int|string")) {
		t.Fatal("union source compatibility is not sound")
	}
}

func TestCheckedIntegerArithmeticRejectsOverflow(t *testing.T) {
	if _, fault := CheckedIntBinary("+", 9223372036854775807, 1); fault != ArithmeticOverflow {
		t.Fatalf("MAX_INT + 1 fault = %q", fault)
	}
	if _, fault := CheckedIntBinary("-", -9223372036854775807, 2); fault != ArithmeticOverflow {
		t.Fatalf("integer subtraction fault = %q", fault)
	}
	if _, fault := CheckedIntBinary("*", 9223372036854775807, 2); fault != ArithmeticOverflow {
		t.Fatalf("integer multiplication fault = %q", fault)
	}
	if _, fault := CheckedIntBinary("%", 1, 0); fault != ArithmeticDivisionByZero {
		t.Fatalf("modulo by zero fault = %q", fault)
	}
}

func TestCheckedIntegerArithmeticPreservesLargeIntegerPrecision(t *testing.T) {
	value, fault := CheckedIntBinary("+", 9007199254740993, 1)
	if fault != ArithmeticOK || value != 9007199254740994 {
		t.Fatalf("large integer result = %d, %q", value, fault)
	}
}

func TestTypedCollectionsAndNarrowing(t *testing.T) {
	arrInt := Parse("array<int>")
	if arrInt.Kind != Array || arrInt.Element == nil || arrInt.Element.Kind != Int || arrInt.String() != "array<int>" {
		t.Fatalf("unexpected arrInt: %#v (%s)", arrInt, arrInt.String())
	}

	mapStrUser := Parse("map<string, User>")
	if mapStrUser.Kind != Map || mapStrUser.Key == nil || mapStrUser.Element == nil || mapStrUser.Element.Kind != Class || mapStrUser.Element.Name != "User" {
		t.Fatalf("unexpected mapStrUser: %#v (%s)", mapStrUser, mapStrUser.String())
	}

	arrNullable := Parse("array<string>?")
	if arrNullable.Kind != Union || arrNullable.String() != "array<string>|null" {
		t.Fatalf("unexpected arrNullable: %#v (%s)", arrNullable, arrNullable.String())
	}

	if !Assignable(Parse("array<int>"), Parse("array<int>")) {
		t.Fatal("array<int> must be assignable to array<int>")
	}
	if Assignable(Parse("array<int>"), Parse("array<string>")) {
		t.Fatal("array<string> must NOT be assignable to array<int>")
	}
	if !Assignable(Parse("array"), Parse("array<int>")) {
		t.Fatal("array<int> must be assignable to untyped array")
	}

	unionType := Parse("User|null")
	narrowed := unionType.Without(Null)
	if narrowed.Kind != Class || narrowed.Name != "User" {
		t.Fatalf("narrowed User|null without null = %s, want User", narrowed.String())
	}
}

func TestNumericClassificationIsCanonical(t *testing.T) {
	for _, name := range []string{"int", "float", "decimal", "int|float"} {
		if !Parse(name).IsNumeric() {
			t.Errorf("%s should be numeric", name)
		}
	}
	for _, name := range []string{"string", "mixed", "int|null", "array<int>"} {
		if Parse(name).IsNumeric() {
			t.Errorf("%s should not be numeric", name)
		}
	}
}
