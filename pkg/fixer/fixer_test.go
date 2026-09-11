package fixer

import (
	"strings"
	"testing"
)

func TestFixerAddsVisibilityAndFormats(t *testing.T) {
	input := `func compute(int $x): int {
return $x+1;
}`
	fixer := NewFixer(true)
	fixed, count := fixer.FixSource(input)

	if count == 0 {
		t.Fatalf("expected fixes to be applied")
	}
	if !strings.Contains(fixed, "public func compute(int $x): int {") {
		t.Fatalf("expected visibility to be added, got:\n%s", fixed)
	}
	if !strings.Contains(fixed, "    return $x + 1;") {
		t.Fatalf("expected indentation and spacing to be formatted, got:\n%s", fixed)
	}
}

func TestFixerRemovesEmptyTernaryElse(t *testing.T) {
	input := `public func test(): void {
    ($cond) ? {
        $x = 1;
    } : {}
}`
	fixer := NewFixer(true)
	fixed, count := fixer.FixSource(input)

	if count == 0 {
		t.Fatalf("expected fixes to be applied")
	}
	if strings.Contains(fixed, ": {}") {
		t.Fatalf("expected empty ternary else to be removed, got:\n%s", fixed)
	}
}

func TestFixerNormalizesDeprecatedTypesAndProceduralHelpers(t *testing.T) {
	input := `public func process(list $items, dynamic $opt): bool {
    return str_contains($items[0], "test");
}`
	fixer := NewFixer(true)
	fixed, count := fixer.FixSource(input)

	if count == 0 {
		t.Fatalf("expected fixes to be applied")
	}
	if !strings.Contains(fixed, "array $items") {
		t.Fatalf("expected 'list' to be fixed to 'array', got:\n%s", fixed)
	}
	if !strings.Contains(fixed, "mixed $opt") {
		t.Fatalf("expected 'dynamic' to be fixed to 'mixed', got:\n%s", fixed)
	}
	if !strings.Contains(fixed, "Str::contains(") {
		t.Fatalf("expected 'str_contains' to be fixed to 'Str::contains', got:\n%s", fixed)
	}
}
