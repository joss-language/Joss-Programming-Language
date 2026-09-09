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

func TestPHPStyleAssociativeArrays(t *testing.T) {
	src := `public func testMap(): string {
    let $data = [
        "name" => "Joss",
        "nested" => [
            "framework" => "JosSecurity"
        ]
    ]
    return $data["name"] . "_" . $data["nested"]["framework"]
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn := runtime.Functions["testMap"]
	res := runtime.CallMethodEvaluated(fn, nil, nil)
	if res != "Joss_JosSecurity" {
		t.Fatalf("expected 'Joss_JosSecurity', got %v", res)
	}
}

func TestForeachKeyValue(t *testing.T) {
	src := `public func testForeachKV(): string {
    let $m = ["a" => "uno", "b" => "dos"]
    string $acc = ""
    foreach ($m as $k => $v) {
        $acc = $acc . $k . "=" . $v . ";"
    }
    return $acc
}
public func testForeachArrayKV(): string {
    let $arr = ["x", "y", "z"]
    string $acc = ""
    foreach ($arr as $idx => $val) {
        $acc = $acc . $idx . ":" . $val . ","
    }
    return $acc
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn1 := runtime.Functions["testForeachKV"]
	res1 := runtime.CallMethodEvaluated(fn1, nil, nil).(string)
	if !strings.Contains(res1, "a=uno;") || !strings.Contains(res1, "b=dos;") {
		t.Fatalf("expected key=value pairs, got %v", res1)
	}

	fn2 := runtime.Functions["testForeachArrayKV"]
	res2 := runtime.CallMethodEvaluated(fn2, nil, nil)
	if res2 != "0:x,1:y,2:z," {
		t.Fatalf("expected '0:x,1:y,2:z,', got %v", res2)
	}
}

func TestArrayDestructuring(t *testing.T) {
	src := `public func testDestructuring(): int {
    [$a, $b] = [15, 25]
    return $a + $b
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn := runtime.Functions["testDestructuring"]
	res := runtime.CallMethodEvaluated(fn, nil, nil)
	if res != int64(40) {
		t.Fatalf("expected 40, got %v (%T)", res, res)
	}
}

func TestDeferStatement(t *testing.T) {
	src := `public func testDefer(): string {
    string $log = "start;"
    defer {
        $log = $log . "defer1;"
    }
    defer {
        $log = $log . "defer2;"
    }
    $log = $log . "end;"
    return $log
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn := runtime.Functions["testDefer"]
	res := runtime.CallMethodEvaluated(fn, nil, nil)
	// Defers run in LIFO order upon exiting: defer2 runs before defer1
	// Wait, return evaluates expression $log first ("start;end;"), then defers execute!
	if res != "start;end;" {
		t.Fatalf("expected 'start;end;', got %v", res)
	}
}

func TestHigherOrderFunctions(t *testing.T) {
	src := `public func testHigherOrder(): int {
    let $nums = [1, 2, 3, 4, 5]
    // Filter even numbers: [2, 4]
    let $evens = $nums |> filter(func(int $x): bool { return $x % 2 == 0; })
    // Map multiply by 10: [20, 40]
    let $scaled = $evens |> map(func(int $x): int { return $x * 10; })
    // Sum: 60
    return sum($scaled)
}
public func testFindAndAnyAll(): bool {
    let $list = [10, 20, 30]
    let $found = $list |> find(func(int $x): bool { return $x > 15; })
    let $hasTwenty = $list |> any(func(int $x): bool { return $x == 20; })
    let $allPositive = $list |> all(func(int $x): bool { return $x > 0; })
    return $found == 20 && $hasTwenty && $allPositive
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	fn1 := runtime.Functions["testHigherOrder"]
	res1 := runtime.CallMethodEvaluated(fn1, nil, nil)
	if res1 != int64(60) {
		t.Fatalf("expected 60, got %v (%T)", res1, res1)
	}

	fn2 := runtime.Functions["testFindAndAnyAll"]
	res2 := runtime.CallMethodEvaluated(fn2, nil, nil)
	if res2 != true {
		t.Fatalf("expected true, got %v (%T)", res2, res2)
	}
}

func TestConsoleColors(t *testing.T) {
	src := `public func testConsole(): string {
    return Console::green("success")
}
`
	runtime := NewRuntime()
	runtime.Execute(benchmarkParse(t, src))
	fn := runtime.Functions["testConsole"]
	res := runtime.CallMethodEvaluated(fn, nil, nil)
	expected := "\033[32msuccess\033[0m"
	if res != expected {
		t.Fatalf("expected %q, got %q", expected, res)
	}
}

func TestJSONEncodeAndDecodeArrays(t *testing.T) {
	src := `public func testJSONRoundtrip(): bool {
    // 1. Classical array encode/decode
    let $clasico = [1, 2, "tres", true]
    let $json1 = json_encode($clasico)
    let $dec1 = json_decode($json1)
    (!is_array($dec1) || count($dec1) != 4 || $dec1[0] != 1 || $dec1[2] != "tres") ? {
        return false
    } : {}

    // 2. PHP-style associative array encode/decode
    let $assoc = [
        "nombre" => "Carlos",
        "edad" => 30,
        "detalles" => [
            "ciudad" => "Lima",
            "piso" => 4
        ]
    ]
    let $json2 = json_encode($assoc)
    let $dec2 = json_decode($json2)
    (!is_array($dec2) || $dec2["nombre"] != "Carlos" || $dec2["edad"] != 30) ? {
        return false
    } : {}
    ($dec2["detalles"]["ciudad"] != "Lima" || $dec2["detalles"]["piso"] != 4) ? {
        return false
    } : {}

    // 3. Nested array of associative arrays
    let $items = [
        ["id" => 1, "producto" => "A"],
        ["id" => 2, "producto" => "B"]
    ]
    let $json3 = JSON::encode($items)
    let $dec3 = JSON::decode($json3)
    (!is_array($dec3) || count($dec3) != 2 || $dec3[1]["producto"] != "B") ? {
        return false
    } : {}

    // 4. Numeric keys in associative array
    let $numKeys = [1 => "uno", 2 => "dos"]
    let $json4 = json_encode($numKeys)
    let $dec4 = json_decode($json4)
    ($dec4[1] != "uno" || $dec4["2"] != "dos") ? {
        return false
    } : {}

    return true
}
`
	runtime := benchmarkPreparedRuntime(t, src)
	runtime.RegisterNativeClasses()
	fn := runtime.Functions["testJSONRoundtrip"]
	res := runtime.CallMethodEvaluated(fn, nil, nil)
	if res != true {
		t.Fatalf("expected true, got %v (%T)", res, res)
	}
}
