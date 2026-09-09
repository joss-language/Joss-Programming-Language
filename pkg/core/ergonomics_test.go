package core

import (
	"bufio"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestPipelineOperator(t *testing.T) {
	src := `public func doubleIt(int $x): int {
    return $x * 2
}
public func addN(int $x, int $n): int {
    return $x + $n
}
public func testPipeline(): int {
    int $res = 5 |> doubleIt |> addN(10)
    return $res
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn := runtime.Functions["testPipeline"]
	result := runtime.CallMethodEvaluated(fn, nil, nil)

	if val, ok := result.(int64); !ok || val != 20 {
		t.Fatalf("expected 20, got %v (%T)", result, result)
	}
}

func TestNullSafeNavigation(t *testing.T) {
	src := `public class Profile {
    public string $email = "test@example.com"
    public func getEmail(): string {
        return $this->email
    }
}
public class User {
    public Profile|null $profile = null
    public func getProfile(): Profile|null {
        return $this->profile
    }
}
public func testNullSafe(): string {
    User|null $user = null
    // Should safely evaluate to null without throwing NullReference panic
    Profile|null $p = $user?->profile
    Profile|null $p2 = $user?->getProfile()
    return $p == null && $p2 == null ? {
        return "safely_null"
    } : {
        return "failed"
    }
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn := runtime.Functions["testNullSafe"]
	result := runtime.CallMethodEvaluated(fn, nil, nil)

	if str, ok := result.(string); !ok || str != "safely_null" {
		t.Fatalf("expected 'safely_null', got %v", result)
	}
}

func TestTrailingCommasEverywhere(t *testing.T) {
	src := `public func compute(
    int $a,
    int $b,
): int {
    array<int> $items = [
        10,
        20,
        30,
    ]
    map $data = {
        "key": 40,
    }
    return $a + $b + $items[0] + $data["key"]
}
public func testTrailing(): int {
    return compute(
        1,
        2,
    )
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn := runtime.Functions["testTrailing"]
	result := runtime.CallMethodEvaluated(fn, nil, nil)

	// 1 + 2 + 10 + 40 = 53
	if val, ok := result.(int64); !ok || val != 53 {
		t.Fatalf("expected 53, got %v", result)
	}
}

func TestMatchWithBlocks(t *testing.T) {
	src := `public func runMatch(int $option): string {
    string $result = ""
    match ($option) {
        1 => {
            $result = "first"
        },
        2 => {
            $result = "second"
        },
        default => {
            $result = "other"
        }
    }
    return $result
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn := runtime.Functions["runMatch"]

	r1 := runtime.CallMethodEvaluated(fn, nil, []interface{}{int64(1)})
	if r1 != "first" {
		t.Fatalf("expected 'first', got %v", r1)
	}

	r2 := runtime.CallMethodEvaluated(fn, nil, []interface{}{int64(2)})
	if r2 != "second" {
		t.Fatalf("expected 'second', got %v", r2)
	}

	r3 := runtime.CallMethodEvaluated(fn, nil, []interface{}{int64(99)})
	if r3 != "other" {
		t.Fatalf("expected 'other', got %v", r3)
	}
}

func TestSingleBranchConditional(t *testing.T) {
	src := `public func testSingleBranch(int $x): string {
    string $status = "initial"
    ($x > 10) ? {
        $status = "greater"
    }
    return $status
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn := runtime.Functions["testSingleBranch"]

	r1 := runtime.CallMethodEvaluated(fn, nil, []interface{}{int64(20)})
	if r1 != "greater" {
		t.Fatalf("expected 'greater', got %v", r1)
	}

	r2 := runtime.CallMethodEvaluated(fn, nil, []interface{}{int64(5)})
	if r2 != "initial" {
		t.Fatalf("expected 'initial', got %v", r2)
	}
}

func TestStringInterpolationFlutterStyle(t *testing.T) {
	src := `public func testInterp(int $indice, string $res): string {
    return "-> Resultado de la suma del número ${indice}: ${res}"
}
public func testInterpExpr(int $a, int $b): string {
    return "Total: ${$a + $b}"
}
public func testInterpEscaped(): string {
    return "Literal: \${indice}"
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn1 := runtime.Functions["testInterp"]
	r1 := runtime.CallMethodEvaluated(fn1, nil, []interface{}{int64(1), "15"})
	expected1 := "-> Resultado de la suma del número 1: 15"
	if r1 != expected1 {
		t.Fatalf("expected %q, got %q", expected1, r1)
	}

	fn2 := runtime.Functions["testInterpExpr"]
	r2 := runtime.CallMethodEvaluated(fn2, nil, []interface{}{int64(10), int64(25)})
	expected2 := "Total: 35"
	if r2 != expected2 {
		t.Fatalf("expected %q, got %q", expected2, r2)
	}

	fn3 := runtime.Functions["testInterpEscaped"]
	r3 := runtime.CallMethodEvaluated(fn3, nil, nil)
	expected3 := "Literal: ${indice}"
	if r3 != expected3 {
		t.Fatalf("expected %q, got %q", expected3, r3)
	}
}

func TestRangeOperatorAndForeach(t *testing.T) {
	src := `public func sumRange(int $max): int {
    int $total = 0
    foreach (1..$max as $i) {
        $total += $i
    }
    return $total
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn := runtime.Functions["sumRange"]
	r := runtime.CallMethodEvaluated(fn, nil, []interface{}{int64(5)})
	// 1 + 2 + 3 + 4 + 5 = 15
	if r != int64(15) {
		t.Fatalf("expected 15, got %v (%T)", r, r)
	}
}

func TestDecrementOperator(t *testing.T) {
	src := `public func testDec(): int {
    int $x = 10
    $x--
    $x--
    return $x
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn := runtime.Functions["testDec"]
	r := runtime.CallMethodEvaluated(fn, nil, nil)
	if r != int64(8) {
		t.Fatalf("expected 8, got %v (%T)", r, r)
	}
}

func TestNullCoalescingAssignOperator(t *testing.T) {
	src := `public func testNullCoalesce(): string {
    string|null $x = null
    $x ??= "default_value"
    $x ??= "second_value"
    return $x
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn := runtime.Functions["testNullCoalesce"]
	r := runtime.CallMethodEvaluated(fn, nil, nil)
	if r != "default_value" {
		t.Fatalf("expected 'default_value', got %v (%T)", r, r)
	}
}

func TestAdaptiveCin(t *testing.T) {
	runtime := NewRuntime()
	runtime.cinReader = bufio.NewReader(strings.NewReader("42\nJuan Perez\n10 20\n"))

	identNum := &parser.Identifier{Value: "num"}
	res1 := runtime.readCinInputForIdentifier(identNum)
	if res1 != int64(42) {
		t.Fatalf("expected int64(42), got %v (%T)", res1, res1)
	}

	identStr := &parser.Identifier{Value: "name"}
	res2 := runtime.readCinInputForIdentifier(identStr)
	if res2 != "Juan Perez" {
		t.Fatalf("expected 'Juan Perez', got %v (%T)", res2, res2)
	}

	identA := &parser.Identifier{Value: "a"}
	runtime.VarTypes["a"] = "int"
	resA := runtime.readCinInputForIdentifier(identA)
	if resA != int64(10) {
		t.Fatalf("expected int64(10), got %v (%T)", resA, resA)
	}

	identB := &parser.Identifier{Value: "b"}
	runtime.VarTypes["b"] = "int"
	resB := runtime.readCinInputForIdentifier(identB)
	if resB != int64(20) {
		t.Fatalf("expected int64(20), got %v (%T)", resB, resB)
	}
}
