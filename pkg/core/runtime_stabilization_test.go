package core

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func runStabilizationProgram(t *testing.T, source string) *Runtime {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	if errors := p.Errors(); len(errors) > 0 {
		t.Fatalf("parse errors: %v", errors)
	}
	runtime := benchmarkPreparedRuntime(t, source)
	runtime.Execute(program)
	return runtime
}

// 1. Test Closures + Slots: Closure retains captured variables after outer function returns.
func TestClosureRetainsCapturedSlotsAfterReturn(t *testing.T) {
	source := `
public func makeAdder(int $inc) {
    return func(int $val): int {
        return $val + $inc
    }
}
$add10 = makeAdder(10)
$add20 = makeAdder(20)
$res1 = $add10(5)
$res2 = $add20(5)
`
	r := runStabilizationProgram(t, source)
	if got := r.Variables["res1"]; got != int64(15) {
		t.Fatalf("res1 = %v, want 15", got)
	}
	if got := r.Variables["res2"]; got != int64(25) {
		t.Fatalf("res2 = %v, want 25", got)
	}
}

func TestClosureCapturesAndMutatesOuterVariable(t *testing.T) {
	source := `
public func makeTracker() {
    $history = []
    return func(mixed $msg) {
        $count = 0
        foreach ($history as $item) {
            $count = $count + 1
        }
        $history = array_push($history, $msg)
        return $count
    }
}
$tracker = makeTracker()
$c0 = $tracker("first")
$c1 = $tracker("second")
$c2 = $tracker("third")
`
	r := runStabilizationProgram(t, source)
	if got := r.Variables["c0"]; got != int64(0) && got != 0 {
		t.Fatalf("c0 = %v, want 0", got)
	}
	if got := r.Variables["c1"]; got != int64(1) && got != 1 {
		t.Fatalf("c1 = %v, want 1", got)
	}
	if got := r.Variables["c2"]; got != int64(2) && got != 2 {
		t.Fatalf("c2 = %v, want 2", got)
	}
}

// 2. Test Ref + Slots: Passing references across multiple call levels modifies caller slots in-place.
func TestRefParametersAcrossMultipleLevels(t *testing.T) {
	source := `
public func level2(ref int $num) {
    $num = $num * 3
}

public func level1(ref int $num) {
    $num = $num + 5
    level2(ref $num)
}

public func testRef(): int {
    int $target = 10
    level1(ref $target)
    return $target
}
`
	r := benchmarkPreparedRuntime(t, source)
	fn := r.Functions["testRef"]
	result := r.CallMethodEvaluated(fn, nil, nil)
	// (10 + 5) * 3 = 45
	if result != int64(45) {
		t.Fatalf("testRef = %v, want 45", result)
	}
}

// 3. Test Async + Slots: Async tasks run with isolated runtime forks and do not mutate caller frame slots.
func TestAsyncExecutionWithSlotsIsolation(t *testing.T) {
	source := `
public func runAsyncTest(): int {
    int $x = 100
    $future = async {
        int $y = 50
        return $y * 2
    }
    int $asyncResult = await($future)
    return $x + $asyncResult
}
`
	r := benchmarkPreparedRuntime(t, source)
	fn := r.Functions["runAsyncTest"]
	result := r.CallMethodEvaluated(fn, nil, nil)
	if result != int64(200) {
		t.Fatalf("runAsyncTest = %v, want 200", result)
	}
}

// 4. Test Exception + Pooled Frame: Pooled frames correctly reset and handle exceptions cleanly.
func TestExceptionRecoveryAndFramePoolIntegrity(t *testing.T) {
	source := `
public func mayThrow(bool $shouldThrow): int {
    int $a = 42
    int $b = 100
    $shouldThrow ? {
        throw new Exception("intentional error")
    } : {
        return $a + $b
    }
}

public func testExceptionRecovery(): int {
    int $caught = 0
    try {
        mayThrow(true)
    } catch ($e) {
        $caught = 1
    }
    // Call again to verify recycled frame has clean slots
    int $val = mayThrow(false)
    return $caught + $val
}
`
	r := benchmarkPreparedRuntime(t, source)
	fn := r.Functions["testExceptionRecovery"]
	result := r.CallMethodEvaluated(fn, nil, nil)
	// 1 (caught) + 142 (42 + 100) = 143
	if result != int64(143) {
		t.Fatalf("testExceptionRecovery = %v, want 143", result)
	}
}

// 5. Test Recursion + Pooled Frame: Deep recursion maintains isolated slots per frame and unwinds cleanly.
func TestRecursionFramePoolDepth(t *testing.T) {
	source := `
public func fib(int $n): int {
    return $n <= 1 ? {
        return $n
    } : {
        return fib($n - 1) + fib($n - 2)
    }
}
`
	r := benchmarkPreparedRuntime(t, source)
	fn := r.Functions["fib"]
	result := r.CallMethodEvaluated(fn, nil, []interface{}{int64(10)})
	if result != int64(55) {
		t.Fatalf("fib(10) = %v, want 55", result)
	}
}

// 6. Test Inheritance + Cached Methods: O(1) ClassMetadata correctly dispatches overridden vs inherited methods.
func TestInheritanceCachedMethodDispatch(t *testing.T) {
	source := `
public class Base {
    public func identify(): string {
        return "Base"
    }
    public func baseOnly(): string {
        return "BaseOnly"
    }
}

public class Derived extends Base {
    public func identify(): string {
        return "Derived"
    }
}

public func testInheritance(): string {
    Derived $d = new Derived()
    return $d->identify() . "-" . $d->baseOnly()
}
`
	r := benchmarkPreparedRuntime(t, source)
	fn := r.Functions["testInheritance"]
	result := r.CallMethodEvaluated(fn, nil, nil)
	if result != "Derived-BaseOnly" {
		t.Fatalf("testInheritance = %v, want 'Derived-BaseOnly'", result)
	}
}

// 7. Test Shadowing + Closures: Inner parameters correctly shadow outer variables inside closures.
func TestShadowingInClosures(t *testing.T) {
	source := `
public func outer(): int {
    int $x = 100
    $f = func(int $x): int {
        return $x * 2
    }
    int $res = $f(5)
    return $x + $res
}
`
	r := benchmarkPreparedRuntime(t, source)
	fn := r.Functions["outer"]
	result := r.CallMethodEvaluated(fn, nil, nil)
	// $x is 100 in outer, $f(5) produces 10 -> total 110
	if result != int64(110) {
		t.Fatalf("outer = %v, want 110", result)
	}
}

// 8. Test Multiple Consecutive Calls With Different Argument Types On Recycled Frames.
func TestRecycledFrameConsecutiveCallsIntegrity(t *testing.T) {
	source := `
public func compute(int $a, int $b): int {
    int $sum = $a + $b
    return $sum
}
`
	r := benchmarkPreparedRuntime(t, source)
	fn := r.Functions["compute"]
	for i := 0; i < 1000; i++ {
		res := r.CallMethodEvaluated(fn, nil, []interface{}{int64(i), int64(i + 1)})
		expected := int64(2*i + 1)
		if res != expected {
			t.Fatalf("iteration %d: compute(%d, %d) = %v, want %d", i, i, i+1, res, expected)
		}
	}
}

// Helper benchmark test to ensure no compiler/runtime regression
func BenchmarkFrameRecycling(b *testing.B) {
	source := `
public func worker(int $a, int $b): int {
    int $c = $a + $b
    int $d = $c * 2
    return $d
}
`
	r := benchmarkPreparedRuntime(b, source)
	b.Cleanup(r.Free)
	fn := r.Functions["worker"]
	args := []interface{}{int64(10), int64(20)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.CallMethodEvaluated(fn, nil, args)
	}
}
