package analyzer

import (
	"reflect"
	"testing"
)

func TestSemanticPipelineIsIndependentOfSourceUnitOrder(t *testing.T) {
	contract := parseUnit(t, "a_contract.joss", `
public interface FormatterContract {
    public func format(string $value): string;
}
`)
	implementation := parseUnit(t, "b_formatter.joss", `
public class Formatter implements FormatterContract {
    public func format(string $value): string { return $value; }
}

public func render(FormatterContract $formatter, string $value): string {
    return $formatter->format($value);
}
`)

	forward := Analyze([]SourceUnit{contract, implementation}, NewEnvironment())
	reverse := Analyze([]SourceUnit{implementation, contract}, NewEnvironment())
	if !reflect.DeepEqual(forward, reverse) {
		t.Fatalf("diagnostics depend on source order:\nforward=%#v\nreverse=%#v", forward, reverse)
	}
	if len(forward) != 0 {
		t.Fatalf("valid cross-file contract produced diagnostics: %#v", forward)
	}
}

func TestCallableCursorDoesNotLeakBetweenBodies(t *testing.T) {
	issues := analyzeSource(t, `
public class First {
    private int $secret = 1;
    public func value(): int { return $this->secret; }
}

public class Second {
    public func value(): string { return "ok"; }
}

public func standalone(): bool { return true; }
`, NewEnvironment())
	if len(issues) != 0 {
		t.Fatalf("class/return cursor leaked between callables: %#v", issues)
	}
}

func TestCallResolutionUsesOneSignatureValidationPath(t *testing.T) {
	issues := analyzeSource(t, `
public func update(ref int $value, string $label = "value"): int { return $value; }
public func run(): int {
    int $value = 1;
    update(ref $value, label: "count");
    update($value);
    update(ref $value, unknown: "count");
    return $value;
}
`, NewEnvironment())

	if countCode(issues, "JOSS-REF-001") != 1 {
		t.Fatalf("expected one ref-contract diagnostic, got %#v", issues)
	}
	if countCode(issues, "JOSS-CALL-001") != 1 {
		t.Fatalf("expected one named-argument diagnostic, got %#v", issues)
	}
}

func TestArgumentNormalizationMatrix(t *testing.T) {
	prefix := `
public func update(ref int $value, string $label = "value"): int { return $value; }
public func run(): int {
    int $value = 1;
`
	suffix := `
    return $value;
}
`
	tests := []struct{ name, call, code string }{
		{name: "named ref", call: `update(value: ref $value, label: "ok");`},
		{name: "missing required", call: `update(label: "missing");`, code: "JOSS-CALL-001"},
		{name: "unknown named", call: `update(value: ref $value, unknown: "x");`, code: "JOSS-CALL-001"},
		{name: "positional named duplicate", call: `update(ref $value, value: ref $value);`, code: "JOSS-CALL-001"},
		{name: "wrong ref marker", call: `update(value: $value);`, code: "JOSS-REF-001"},
		{name: "too many", call: `update(ref $value, "x", "extra");`, code: "JOSS-CALL-001"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issues := analyzeSource(t, prefix+test.call+suffix, NewEnvironment())
			if test.code == "" {
				if len(issues) != 0 {
					t.Fatalf("valid call produced diagnostics: %#v", issues)
				}
				return
			}
			if countCode(issues, test.code) == 0 {
				t.Fatalf("expected %s, got %#v", test.code, issues)
			}
		})
	}
}
