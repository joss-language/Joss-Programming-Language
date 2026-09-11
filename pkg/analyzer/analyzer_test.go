package analyzer

import (
	"testing"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

func analyzeSource(t *testing.T, source string, environment Environment) []diagnostics.Diagnostic {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	if errors := p.Errors(); len(errors) > 0 {
		t.Fatalf("parse errors: %v", errors)
	}
	return Analyze([]SourceUnit{{Path: "test.joss", Program: program}}, environment)
}

func hasCode(items []diagnostics.Diagnostic, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}

func countCode(items []diagnostics.Diagnostic, code string) int {
	count := 0
	for _, item := range items {
		if item.Code == code {
			count++
		}
	}
	return count
}

func TestInferredVariableKeepsFirstConcreteType(t *testing.T) {
	items := analyzeSource(t, `$age = 20
$age = 30
$age = "twenty"`, NewEnvironment())
	if !hasCode(items, "JOSS-TYPE-001") {
		t.Fatalf("expected inferred assignment error, got %#v", items)
	}
}

func TestExplicitDynamicVariableMayChangeType(t *testing.T) {
	items := analyzeSource(t, `let $value = 20
$value = "twenty"
echo $value`, NewEnvironment())
	if hasCode(items, "JOSS-TYPE-001") || hasCode(items, "JOSS-TYPE-002") {
		t.Fatalf("mixed variable should stay dynamic, got %#v", items)
	}
}

func TestVarDeclarationInfersAndTypedDeclarationChecks(t *testing.T) {
	items := analyzeSource(t, `var $age = 20
$age = "twenty"
string $name = 42`, NewEnvironment())
	if !hasCode(items, "JOSS-TYPE-001") || !hasCode(items, "JOSS-TYPE-002") {
		t.Fatalf("expected assignment and initializer diagnostics, got %#v", items)
	}
}

func TestTypedNumericStringLiteralUsesRuntimeCoercionPolicy(t *testing.T) {
	items := analyzeSource(t, `int $age = "20"
echo $age`, NewEnvironment())
	if hasCode(items, "JOSS-TYPE-002") {
		t.Fatalf("lossless numeric string should be accepted consistently: %#v", items)
	}
}

func TestCallableScopesDoNotLeak(t *testing.T) {
	items := analyzeSource(t, `public class Example {
  public func first(mixed $value) { $local = $value echo $local }
  public func second(mixed $value) { $local = $value echo $local }
}`, NewEnvironment())
	if hasCode(items, "JOSS-SYM-002") || hasCode(items, "JOSS-SYM-001") {
		t.Fatalf("method-local symbols leaked across scopes: %#v", items)
	}
}

func TestTypedFunctionArgumentsAndArity(t *testing.T) {
	items := analyzeSource(t, `public func add(int $a, int $b) { return $a + $b }
add(1, "two")
add(1)`, NewEnvironment())
	if !hasCode(items, "JOSS-TYPE-003") || !hasCode(items, "JOSS-CALL-001") {
		t.Fatalf("expected argument type and arity diagnostics, got %#v", items)
	}
}

func TestParametersRequireExplicitTypeOrMixed(t *testing.T) {
	items := analyzeSource(t, `public func invalid($value) { return $value }
public func dynamic(mixed $value) { return $value }`, NewEnvironment())
	if countCode(items, "JOSS-TYPE-011") != 1 {
		t.Fatalf("diagnostics = %#v, want one implicit parameter type error", items)
	}
}

func TestUnknownNativeSignatureDoesNotCreateArityFalsePositive(t *testing.T) {
	environment := NewEnvironment()
	environment.Classes["Native"] = Class{Name: "Native", Methods: map[string]Callable{
		"call": {Name: "call", Variadic: true, ReturnType: typesystem.Type{Kind: typesystem.Unknown}},
	}}
	items := analyzeSource(t, `Native::call(1, 2, 3)`, environment)
	if hasCode(items, "JOSS-CALL-001") {
		t.Fatalf("unknown native signature must not report arity: %#v", items)
	}
}

func TestIssetDoesNotTreatUnknownAsInvalid(t *testing.T) {
	items := analyzeSource(t, `isset($optional)`, NewEnvironment())
	if hasCode(items, "JOSS-SYM-001") {
		t.Fatalf("isset is an existence probe, got %#v", items)
	}
}

func TestForeachBindingCanBeReused(t *testing.T) {
	items := analyzeSource(t, `$one = []
$two = []
foreach ($one as $item) { echo $item }
foreach ($two as $item) { echo $item }`, NewEnvironment())
	if hasCode(items, "JOSS-SYM-002") {
		t.Fatalf("foreach reuse should be assignment-like, got %#v", items)
	}
}

func TestLocalReceiverShadowsSameNamedClassAndCountsAsUse(t *testing.T) {
	items := analyzeSource(t, `public class repository {
  public func first() { return 1 }
}
public func load() {
  $repository = new repository()
  return $repository->first()
}`, NewEnvironment())
	if hasCode(items, "JOSS-LINT-001") || hasCode(items, "JOSS-MEMBER-001") {
		t.Fatalf("instance receiver was mistaken for its same-named class: %#v", items)
	}
}

func TestDiagnosticsRetainFileAndPosition(t *testing.T) {
	items := analyzeSource(t, "\n echo $missing", NewEnvironment())
	if len(items) == 0 || items[0].File != "test.joss" || items[0].Range.Start.Line != 2 || items[0].Range.Start.Column == 0 {
		t.Fatalf("missing source location: %#v", items)
	}
}

func TestUnreachableStatementIsWarning(t *testing.T) {
	items := analyzeSource(t, `public func stop() { return 1 echo "never" }`, NewEnvironment())
	if !hasCode(items, "JOSS-FLOW-001") {
		t.Fatalf("expected unreachable warning, got %#v", items)
	}
}

func TestRecursiveFunctionUsesPredeclaredReturnType(t *testing.T) {
	items := analyzeSource(t, `public func factorial(int $n): int {
  ($n <= 1) ? { return 1 } : {}
  return $n * factorial($n - 1)
}
$result = factorial(5)
echo $result`, NewEnvironment())
	if hasCode(items, "JOSS-SYM-003") || hasCode(items, "JOSS-TYPE-004") || hasCode(items, "JOSS-TYPE-008") {
		t.Fatalf("valid recursive signature was not resolved: %#v", items)
	}
}

func TestRecursiveMethodUsesPredeclaredReturnType(t *testing.T) {
	items := analyzeSource(t, `public class Calculator {
    public func factorial(int $value): int {
        ($value <= 1) ? { return 1 } : { return $value * $this->factorial($value - 1) }
    }
}`, NewEnvironment())
	for _, code := range []string{"JOSS-MEMBER-001", "JOSS-TYPE-004", "JOSS-TYPE-008"} {
		if hasCode(items, code) {
			t.Fatalf("valid recursive method produced %s: %#v", code, items)
		}
	}
}

func TestDeclaredReturnTypeIsChecked(t *testing.T) {
	items := analyzeSource(t, `public func invalid(): int { return "wrong" }`, NewEnvironment())
	if !hasCode(items, "JOSS-TYPE-008") {
		t.Fatalf("expected return type diagnostic, got %#v", items)
	}
}

func TestUnannotatedReturnDoesNotInventATypeContract(t *testing.T) {
	items := analyzeSource(t, `public func flexible() { return "value" }`, NewEnvironment())
	if hasCode(items, "JOSS-TYPE-008") {
		t.Fatalf("unannotated return produced a type error: %#v", items)
	}
}

func TestConstantReassignmentIsRejected(t *testing.T) {
	items := analyzeSource(t, `const $limit = 10
$limit = 20
echo $limit`, NewEnvironment())
	if !hasCode(items, "JOSS-SYM-006") {
		t.Fatalf("expected immutable constant diagnostic, got %#v", items)
	}
}

func TestConstantAndTypedPropertiesAreChecked(t *testing.T) {
	items := analyzeSource(t, `public class Limits {
    public const int $maximum = 10
    private string $label = "safe"
    public func invalid() {
        $this->maximum = 20
        $this->label = false
    }
}`, NewEnvironment())
	if !hasCode(items, "JOSS-SYM-006") || !hasCode(items, "JOSS-TYPE-001") {
		t.Fatalf("property diagnostics = %#v", items)
	}
}

func TestRemovedTypeAliasesAreReportedAsUnknownTypes(t *testing.T) {
	items := analyzeSource(t, `integer $age = 20
public func enabled(boolean $value): boolean { return $value }`, NewEnvironment())
	if countCode(items, "JOSS-TYPE-009") != 3 {
		t.Fatalf("diagnostics = %#v, want three JOSS-TYPE-009 errors", items)
	}
}

func TestDeclaredClassNamesRemainValidTypes(t *testing.T) {
	items := analyzeSource(t, `public class User {}
public func identity(User $user): User { return $user }`, NewEnvironment())
	if countCode(items, "JOSS-TYPE-009") != 0 {
		t.Fatalf("declared class type diagnostics = %#v", items)
	}
}

func TestNullableAndUnionAssignments(t *testing.T) {
	items := analyzeSource(t, `int? $count = null
$count = 2
$count = "wrong"
public func normalize(int|string $value): string|null {
    ($value == 0) ? { return null } : { return "ok" }
}`, NewEnvironment())
	if countCode(items, "JOSS-TYPE-001") != 1 || hasCode(items, "JOSS-TYPE-008") || hasCode(items, "JOSS-TYPE-010") {
		t.Fatalf("union diagnostics = %#v", items)
	}
}

func TestDeclaredReturnRequiresEveryPath(t *testing.T) {
	items := analyzeSource(t, `public func incomplete(bool $ok): int {
    $ok ? { return 1 } : {}
}
public func complete(bool $ok): int {
    $ok ? { return 1 } : { return 2 }
}`, NewEnvironment())
	if countCode(items, "JOSS-TYPE-010") != 1 {
		t.Fatalf("return-path diagnostics = %#v", items)
	}
}

func TestGuardRequiresElseExitDiagnostic(t *testing.T) {
	itemsInvalid := analyzeSource(t, `public func test(int $x): int {
    guard ($x > 0) : {
        $a = 1
    }
    return $x
}`, NewEnvironment())
	if !hasCode(itemsInvalid, "JOSS-FLOW-005") {
		t.Fatalf("expected JOSS-FLOW-005 when guard else does not exit, got %#v", itemsInvalid)
	}

	itemsValid := analyzeSource(t, `public func test(int $x): int {
    guard ($x > 0) : {
        return 0
    }
    return $x
}`, NewEnvironment())
	if hasCode(itemsValid, "JOSS-FLOW-005") {
		t.Fatalf("did not expect JOSS-FLOW-005 for valid guard, got %#v", itemsValid)
	}
}

func TestSmartCastNarrowing(t *testing.T) {
	itemsGuard := analyzeSource(t, `public func test(int|null $x): int {
    guard ($x != null) : {
        return 0
    }
    return $x
}`, NewEnvironment())
	if hasCode(itemsGuard, "JOSS-TYPE-008") {
		t.Fatalf("guard should narrow $x to int, got %#v", itemsGuard)
	}

	itemsTernary := analyzeSource(t, `public func test(int|null $x): int {
    ($x == null) ? { return 0 }
    return $x
}`, NewEnvironment())
	if hasCode(itemsTernary, "JOSS-TYPE-008") {
		t.Fatalf("terminating null-check should narrow $x to int, got %#v", itemsTernary)
	}
}
