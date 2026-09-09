package core

import (
	"testing"
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

