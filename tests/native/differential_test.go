package native_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	core "github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/mobile"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

// runInterpreter ejecuta código Joss mediante el intérprete oficial (joss run).
func runInterpreter(t *testing.T, source string) (string, bool, string) {
	t.Helper()
	res := mobile.RunDirect(source, 5000)
	out := strings.TrimSpace(res.Stdout)
	errInfo := res.Error
	if len(res.Diagnostics) > 0 {
		var diags []string
		for _, d := range res.Diagnostics {
			diags = append(diags, d.Code+": "+d.Message)
		}
		errInfo += " | Diags: " + strings.Join(diags, "; ")
	}
	return out, res.Success, errInfo
}

// compileAndRunNative compila código Joss mediante la ruta oficial de compilador nativo (joss build)
// y ejecuta el binario resultante.
func compileAndRunNative(t *testing.T, source string, name string) (string, int, error) {
	return compileAndRunNativeWithUnits(t, source, name, nil)
}

func compileAndRunNativeWithUnits(t *testing.T, source string, name string, additionalUnits []analyzer.SourceUnit) (string, int, error) {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors in %s: %v", name, p.Errors())
	}

	units := append([]analyzer.SourceUnit{{Path: name + ".joss", Program: prog}}, additionalUnits...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors in %s: %v", name, prep.Diagnostics)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram(name)
	if err != nil {
		return "", -1, err
	}
	verifier := ir.NewVerifier()
	if err := verifier.Verify(irProg); err != nil {
		t.Fatalf("IR verification failed in %s: %v", name, err)
	}

	tempDir := t.TempDir()
	outExe := filepath.Join(tempDir, name+".exe")

	res, err := backend.BuildProgram(irProg, backend.BuildOptions{
		OutputPath: outExe,
		Backend:    backend.BackendAuto,
	})
	if err != nil {
		return "", -1, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, res.OutputPath)
	outBytes, runErr := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(outBytes), -1, ctx.Err()
	}
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", -1, runErr
		}
	}

	return strings.TrimSpace(string(outBytes)), exitCode, nil
}

func TestDifferential_PureJossAuthUnauthenticatedPolicies(t *testing.T) {
	source := `echo Auth::check(); echo Auth::guest(); echo Auth::hasRole("admin");`
	var authUnit []analyzer.SourceUnit
	for _, unit := range std.GetStdlibUnits() {
		if unit.Path == "<std/auth/auth.joss>" || strings.HasPrefix(unit.Path, "<std/date/") {
			authUnit = append(authUnit, unit)
		}
	}
	if len(authUnit) != 4 {
		t.Fatalf("Auth and date Standard Library modules embedded = %d, want 4", len(authUnit))
	}
	interpOut, interpSuccess, interpErr := runInterpreter(t, source)
	if !interpSuccess {
		t.Fatalf("interpreter failed: %s", interpErr)
	}
	nativeOut, nativeExit, err := compileAndRunNativeWithUnits(t, source, "pure_joss_auth_check_guest", authUnit)
	if err != nil {
		t.Fatalf("native compiler failed: %v", err)
	}
	if nativeExit != 0 || strings.ReplaceAll(nativeOut, "\r\n", "\n") != strings.ReplaceAll(interpOut, "\r\n", "\n") {
		t.Fatalf("Auth differential mismatch: interpreted=%q native=%q exit=%d", interpOut, nativeOut, nativeExit)
	}
}

func TestDifferential_TimeBuiltinUsesUnixWallClock(t *testing.T) {
	assertDifferential(t, "time_builtin_wall_clock", `echo time() > 0;`)
}

func TestDifferential_PureJossNowFormatsLocalClock(t *testing.T) {
	dateUnits := make([]analyzer.SourceUnit, 0, 3)
	for _, unit := range std.GetStdlibUnits() {
		if strings.HasPrefix(unit.Path, "<std/date/") {
			dateUnits = append(dateUnits, unit)
		}
	}
	if len(dateUnits) != 3 {
		t.Fatalf("date Standard Library modules embedded = %d, want 3", len(dateUnits))
	}
	source := `echo now("Y-m-d H:i:s O P");`
	interpOut, interpSuccess, interpErr := runInterpreter(t, source)
	if !interpSuccess {
		t.Fatalf("interpreter failed: %s", interpErr)
	}
	nativeOut, nativeExit, err := compileAndRunNativeWithUnits(t, source, "pure_joss_now", dateUnits)
	if err != nil {
		t.Fatalf("native compiler failed: %v", err)
	}
	pattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} [+-]\d{4} [+-]\d{2}:\d{2}$`)
	if nativeExit != 0 || !pattern.MatchString(nativeOut) {
		t.Fatalf("now() native output %q, exit %d; interpreted output was %q", nativeOut, nativeExit, interpOut)
	}
	if got, want := nativeOut[len(nativeOut)-12:], time.Now().Format("-0700 -07:00"); got != want {
		t.Fatalf("native local offset = %q, want process local offset %q", got, want)
	}
	clock, err := time.ParseInLocation("2006-01-02 15:04:05", nativeOut[:19], time.Local)
	if err != nil {
		t.Fatalf("parse native local clock %q: %v", nativeOut[:19], err)
	}
	if delta := time.Since(clock); delta < -3*time.Second || delta > 3*time.Second {
		t.Fatalf("native local clock differs from system clock by %s", delta)
	}
}

func TestDifferential_PureJossDateFormatterNative(t *testing.T) {
	var dateUnits []analyzer.SourceUnit
	for _, unit := range std.GetStdlibUnits() {
		if unit.Path == "<std/date/format.joss>" {
			dateUnits = append(dateUnits, unit)
		}
	}
	got, exitCode, err := compileAndRunNativeWithUnits(t, `echo DateFormatter::formatEpoch("Y-m-d H:i:s D l F M c O P T", 0, 3600, "STD"); echo DateFormatter::formatEpoch("\\Y", 0, 0);`, "pure_joss_formatter", dateUnits)
	if err != nil {
		t.Fatalf("native compiler failed: %v", err)
	}
	if exitCode != 0 || got != "1970-01-01 01:00:00 Thu Thursday January Jan 1970-01-01T01:00:00+01:00 +0100 +01:00 STD\nY" {
		t.Fatalf("native formatter output %q, exit %d", got, exitCode)
	}
}

func TestDifferential_PureJossHTTPFormParser(t *testing.T) {
	source := `
string $raw = "POST /password/reset HTTP/1.1\r\nContent-Type: application/x-www-form-urlencoded\r\n\r\ntoken=smoke&password=strong-password-123&password_confirmation=different-password";
var $request = HttpRequestParser::parse($raw);
echo $request["password_confirmation"];
`
	var requestUnit []analyzer.SourceUnit
	for _, unit := range std.GetStdlibUnits() {
		if unit.Path == "<std/http/request.joss>" {
			requestUnit = append(requestUnit, unit)
			break
		}
	}
	if len(requestUnit) != 1 {
		t.Fatal("canonical std/http/request.joss unit was not embedded")
	}
	interpOut, interpSuccess, interpErr := runInterpreter(t, source)
	if !interpSuccess {
		t.Fatalf("interpreter failed: %s", interpErr)
	}
	nativeOut, nativeExit, err := compileAndRunNativeWithUnits(t, source, "pure_joss_http_form_parser", requestUnit)
	if err != nil {
		t.Fatalf("native compiler failed: %v", err)
	}
	if nativeExit != 0 || strings.ReplaceAll(nativeOut, "\r\n", "\n") != strings.ReplaceAll(interpOut, "\r\n", "\n") {
		t.Fatalf("HTTP form parser differential mismatch: interpreted=%q native=%q exit=%d", interpOut, nativeOut, nativeExit)
	}
}

func TestDifferential_HTTPParserStringPrimitives(t *testing.T) {
	cases := map[string]string{
		"strpos":                 `echo strpos("abc", "b");`,
		"strpos_unicode":         `echo strpos("café", "é");`,
		"substr":                 `echo substr("abc", 1, 1);`,
		"substr_unicode":         `echo substr("aé😊z", 1, 2);`,
		"substr_negative":        `echo substr("aé😊z", -2, 1);`,
		"substr_negative_length": `echo substr("aé😊z", 1, -1);`,
		"substr_omitted_length":  `echo substr("aé😊z", 2);`,
		"substr_zero_length":     `echo substr("abc", 1, 0);`,
		"explode":                `var $source = "a=1&b=2"; var $parts = explode("&", $source); echo $parts[1]; echo $source;`,
		"urldecode":              `echo urldecode("a%20b");`,
		"urldecode_plus":         `echo urldecode("a+b");`,
		"urldecode_utf8":         `echo urldecode("%C3%A9");`,
		"urldecode_invalid":      `echo urldecode("bad%2Xescape");`,
		"strtoupper_ascii":       `echo strtoupper("get");`,
		"strtolower_ascii":       `echo strtolower("POST");`,
		"map_assign":             `var $data = {}; $data["email"] = "ada@example.test"; echo $data["email"];`,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) { assertDifferential(t, "http_parser_primitive_"+name, source) })
	}
}

// assertDifferential verifica que el intérprete y el binario compilado producen la misma salida.
func assertDifferential(t *testing.T, name string, source string) {
	t.Helper()

	// 1. Ejecutar con intérprete (joss run)
	interpOut, interpSuccess, interpErr := runInterpreter(t, source)
	if !interpSuccess {
		t.Fatalf("[%s] intérprete falló al ejecutar el código: %s", name, interpErr)
	}

	// 2. Compilar y ejecutar con backend nativo (joss build)
	nativeOut, nativeExit, err := compileAndRunNative(t, source, name)
	if err != nil {
		t.Fatalf("[%s] compilador nativo falló: %v", name, err)
	}
	if nativeExit != 0 {
		t.Logf("[%s] native stdout/stderr: %q", name, nativeOut)
		t.Fatalf("[%s] binario nativo terminó con código de salida %d (esperado 0)", name, nativeExit)
	}

	// 3. Comparación diferencial exacta (normalizando saltos de línea para portabilidad multiplataforma)
	normInterp := strings.ReplaceAll(strings.TrimSpace(interpOut), "\r\n", "\n")
	normNative := strings.ReplaceAll(strings.TrimSpace(nativeOut), "\r\n", "\n")
	if normInterp != normNative {
		t.Errorf("[%s] Discrepancia diferencial detectada:\n--- Intérprete (joss run) ---\n%s\n--- Nativo (joss build) ---\n%s", name, normInterp, normNative)
	}
}

func TestDifferential_ArithmeticAndVariables(t *testing.T) {
	source := `
		int $a = 10;
		int $b = 25;
		int $sum = $a + $b;
		int $sub = $b - $a;
		int $mul = $a * 3;
		echo $sum;
		echo $sub;
		echo $mul;
	`
	assertDifferential(t, "arithmetic_vars", source)
}

func TestDifferential_SourceStaticMethodOverridesNativeClass(t *testing.T) {
	source := `
public class SEO {
    public static func pureMarker(): int { return 42; }
}
echo(SEO::pureMarker());
`
	assertDifferential(t, "source_static_method_native_precedence", source)
}

func TestDifferential_StaticMapPropertyReadWrite(t *testing.T) {
	source := `
public class RequestCache {
    public static map<string, string> $items = {}
    public static func put(string $key, string $value): void {
        RequestCache::$items[$key] = $value;
    }
    public static func get(string $key): string {
        return RequestCache::$items[$key];
    }
}
RequestCache::put("request", "joss-state");
echo RequestCache::get("request");
`
	assertDifferential(t, "static_map_property_read_write", source)
}

func TestDifferential_ControlFlowBranching(t *testing.T) {
	source := `
		int $x = 42;
		string $res1 = ($x > 50) ? "mayor" : "menor o igual";
		string $res2 = ($x == 42) ? "exacto" : "otro";
		echo $res1;
		echo $res2;
	`
	assertDifferential(t, "control_flow_branching", source)
}

func TestDifferential_Loops(t *testing.T) {
	source := `
		int $sum = 0;
		int $i = 1;
		while ($i <= 5) {
			$sum = $sum + $i;
			$i = $i + 1;
		}
		echo $sum;

		int $c = 3;
		while ($c > 0) {
			echo $c;
			$c = $c - 1;
		}
	`
	assertDifferential(t, "loops", source)
}

func TestDifferential_FunctionsAndCalls(t *testing.T) {
	source := `
		public func square(int $n): int {
			return $n * $n;
		}

		public func addThree(int $a, int $b, int $c): int {
			return $a + $b + $c;
		}

		echo square(7);
		echo addThree(10, 20, 30);
	`
	assertDifferential(t, "functions", source)
}

func TestDifferential_FibonacciRecursion(t *testing.T) {
	source := `
		public func fib(int $n): int {
			guard ($n > 1) else {
				return $n;
			}
			return fib($n - 1) + fib($n - 2);
		}

		echo fib(0);
		echo fib(1);
		echo fib(6);
		echo fib(8);
	`
	assertDifferential(t, "fibonacci", source)
}

func TestDifferential_StringsAndMultiplePrints(t *testing.T) {
	source := `
		string $saludo = "Hola";
		string $mundo = "Mundo";
		echo $saludo;
		echo $mundo;
		echo "Fin de prueba";
	`
	assertDifferential(t, "strings_echo", source)
}

func TestDifferential_ClassesAndFields(t *testing.T) {
	source := `
		public class Persona {
			public string $nombre = "Joss";
			public int $edad = 30;
			public func celebrar(): int {
				$this->edad = $this->edad + 1;
				return $this->edad;
			}
		}

		var $p = new Persona();
		echo $p->nombre;
		echo $p->edad;
		int $nuevaEdad = $p->celebrar();
		echo $nuevaEdad;
	`
	assertDifferential(t, "class_properties_methods", source)
}

func TestDifferential_LogicalAndRelational(t *testing.T) {
	source := `
		int $x = 12;
		int $y = 3;
		int $c1 = ($x > 10 && $y < 5) ? 1 : 0;
		int $c2 = ($x < 5 || $y == 3) ? 1 : 0;
		int $c3 = (!($x == $y)) ? 1 : 0;
		int $c4 = ($x >= 12 && $y <= 3) ? 1 : 0;
		echo $c1;
		echo $c2;
		echo $c3;
		echo $c4;
	`
	assertDifferential(t, "logical_relational", source)
}

func TestDifferential_NestedFunctionCalls(t *testing.T) {
	source := `
		public func doubleVal(int $n): int {
			return $n * 2;
		}

		public func quadVal(int $n): int {
			return doubleVal(doubleVal($n));
		}

		echo quadVal(5);
	`
	assertDifferential(t, "nested_calls", source)
}

func TestDifferential_ArraysAndMaps(t *testing.T) {
	source := `
		var $arr = [10, 20, 30];
		echo $arr[0];
		echo $arr[1];
		echo $arr[2];

		$arr[1] = 99;
		echo $arr[1];

		var $map = {"clave": "valor", "otro": "mundo"};
		echo $map["clave"];
		echo $map["otro"];

		$map["clave"] = "nuevo";
		echo $map["clave"];
	`
	assertDifferential(t, "arrays_and_maps", source)
}

func TestDifferential_ClassMethodsAndSemanticBridge(t *testing.T) {
	source := `
		public class MathUtil {
			public static func add(int $a, int $b): int {
				return $a + $b;
			}
			public static func multiply(int $x, int $y): int {
				return $x * $y;
			}
		}

		int $s = MathUtil::add(15, 27);
		int $p = MathUtil::multiply(6, 7);
		echo $s;
		echo $p;
	`
	assertDifferential(t, "class_methods", source)
}

func TestDifferential_FeatureProbe_Spaceship(t *testing.T) {
	source := `
		public func compareNumbers(int $a, int $b): int {
			return $a <=> $b;
		}

		int $res1 = compareNumbers(15, 30);
		int $res2 = compareNumbers(42, 42);
		int $res3 = compareNumbers(100, 50);

		echo $res1;
		echo $res2;
		echo $res3;
	`
	assertDifferential(t, "feature_probe_spaceship", source)
}

func TestDifferential_CapabilityContract_DiagnosticFormatting(t *testing.T) {
	formatted := analyzer.FormatCapabilityError("DynamicClasses")
	if !strings.Contains(formatted, "JOSS-NATIVE-001") {
		t.Fatalf("expected JOSS-NATIVE-001 in formatted error, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "joss run") {
		t.Fatalf("expected alternative suggestion 'joss run', got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "DynamicClasses") {
		t.Fatalf("expected feature name 'DynamicClasses', got:\n%s", formatted)
	}
}

func TestDifferential_FeatureProbe_Deep(t *testing.T) {
	source := `
		public class MathEngine {
			public static func power(int $base, int $exp): int {
				int $result = 1;
				int $i = 0;
				while ($i < $exp) {
					$result = $result * $base;
					$i = $i + 1;
				}
				return $result;
			}
		}

		public func evaluateComparison(int $val, int $target): int {
			int $sq = MathEngine::power($val, 2);
			int $cmp = $sq <=> $target;
			return $cmp;
		}

		int $t1 = evaluateComparison(3, 10);
		int $t2 = evaluateComparison(4, 16);
		int $t3 = evaluateComparison(5, 20);

		echo $t1;
		echo $t2;
		echo $t3;
	`
	assertDifferential(t, "feature_probe_deep", source)
}

func TestDifferential_AcceptanceFullParity(t *testing.T) {
	source := `
		public class Item {
			public string $title = "Default";
			public int $quantity = 1;

			public func totalCost(int $price): int {
				return $this->quantity * $price;
			}
		}

		public class Order {
			public string $customer = "Cliente";
			public int $status = 0;

			public func process(): string {
				$this->status = 1;
				return "Procesada";
			}
		}

		var $item = new Item();
		$item->title = "Laptop";
		$item->quantity = 3;

		int $cost = $item->totalCost(500);
		echo $item->title;
		echo $cost;

		var $order = new Order();
		string $res = $order->process();
		echo $res;
		echo $order->status;

		var $items = ["Laptop", "Mouse", "Teclado"];
		echo $items[0];
		echo $items[1];
		echo $items[2];

		$items[1] = "Monitor";
		echo $items[1];

		var $metadata = {"categoria": "Tech", "region": "LATAM"};
		echo $metadata["categoria"];
		echo $metadata["region"];

		$metadata["region"] = "Global";
		echo $metadata["region"];
	`
	assertDifferential(t, "acceptance_full_parity", source)
}

func TestDifferential_Interfaces(t *testing.T) {
	source := `
		public interface Greeter {
			public func greet(string $target): string;
		}

		public class SpanishGreeter implements Greeter {
			public func greet(string $target): string {
				return "Hola, " . $target;
			}
		}

		public class EnglishGreeter implements Greeter {
			public func greet(string $target): string {
				return "Hello, " . $target;
			}
		}

		var $g1 = new SpanishGreeter();
		var $g2 = new EnglishGreeter();
		echo $g1->greet("Mundo");
		echo $g2->greet("World");
	`
	assertDifferential(t, "interfaces_parity", source)
}

func TestDifferential_Exceptions_TryCatch(t *testing.T) {
	source := `
		public func fail(): string {
			throw "boom";
		}

		public func computeSafe(int $val): string {
			try {
				guard ($val != 0) else {
					throw "cero no permitido";
				}
				return "safe";
			} catch ($e) {
				return "caught: " . $e;
			}
		}

		string $r1 = computeSafe(10);
		string $r2 = computeSafe(0);
		echo $r1;
		echo $r2;

		try {
			echo "intentando directo";
			throw "error directo";
			echo "nunca";
		} catch ($err) {
			echo "recuperado: " . $err;
		}
		echo "continuando flujo";
	`
	assertDifferential(t, "exceptions_try_catch_parity", source)
}

func TestDifferential_ClosuresAndIndirectCalls(t *testing.T) {
	source := `
		public func applyTwice(mixed $fn, int $val): int {
			return $fn($fn($val));
		}

		var $double = func(int $x): int {
			return $x * 2;
		};

		int $r = applyTwice($double, 5);
		echo $r;
	`
	assertDifferential(t, "closures_indirect_parity", source)
}

func TestNativeAOT_StandaloneBinaryDoesNotContainASTInterpreter(t *testing.T) {
	source := `
		int $x = 10;
		int $y = 20;
		int $z = $x + $y;
		echo $z;
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := []analyzer.SourceUnit{{Path: "pure_aot_check.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("pure_aot_check")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	verifier := ir.NewVerifier()
	if err := verifier.Verify(irProg); err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	tempDir := t.TempDir()
	outExe := filepath.Join(tempDir, "pure_aot_check.exe")
	res, err := backend.BuildProgram(irProg, backend.BuildOptions{
		OutputPath: outExe,
		Backend:    backend.BackendStandalone,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Verify generated standalone Go source code does NOT import AST/core/parser packages
	standalone := backend.NewStandaloneBuilder()
	src := standalone.GenerateSource(irProg)
	forbidden := []string{
		"github.com/jossecurity/joss/pkg/core",
		"github.com/jossecurity/joss/pkg/parser",
		"github.com/jossecurity/joss/pkg/analyzer",
		"ExecutePrepared",
		"evaluator",
	}
	for _, f := range forbidden {
		if strings.Contains(src, f) {
			t.Errorf("AOT standalone source contains forbidden interpreter dependency: %q", f)
		}
	}

	// Verify the executable runs independently and outputs 30
	cmd := exec.Command(res.OutputPath)
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	outStr := strings.TrimSpace(string(outBytes))
	if outStr != "30" {
		t.Errorf("expected output 30, got %q", outStr)
	}
}

func TestDifferential_NativeBuiltins_MathAndCollections(t *testing.T) {
	source := `
		var $val1 = abs(-42);
		var $val2 = abs(17);
		int $m1 = min(15, 30);
		int $m2 = max(15, 30);
		int $iv = intval(123);
		int $b1 = boolval(1) ? 1 : 0;
		int $b2 = boolval(0) ? 1 : 0;

		var $arr = [10, 20, 30, 40];
		int $popped = array_pop($arr);

		echo $val1;
		echo $val2;
		echo $m1;
		echo $m2;
		echo $iv;
		echo $b1;
		echo $b2;
		echo $popped;
		echo count($arr);
	`
	assertDifferential(t, "native_builtins_math_colls", source)
}

func TestDifferential_PrimitiveMethods_ArrayStringMap(t *testing.T) {
	source := `
		var $nums = [5, 10, 15];
		echo $nums->length();
		echo $nums->count();

		var $nums2 = $nums->push(20);
		echo $nums2->length();
		echo $nums2[3];

		string $txt = "Joss Lang";
		echo $txt->length();

		var $dict = {"alpha": 1, "beta": 2, "gamma": 3};
		echo $dict->length();
		echo $dict->count();
	`
	assertDifferential(t, "primitive_methods_asm", source)
}

func TestDifferential_Algorithms_SieveOfEratosthenes(t *testing.T) {
	source := `
		public func countPrimes(int $limit): int {
			var $isPrime = [];
			int $i = 0;
			while ($i <= $limit) {
				$isPrime = $isPrime->push(1);
				$i = $i + 1;
			}
			$isPrime[0] = 0;
			$isPrime[1] = 0;

			int $p = 2;
			while ($p * $p <= $limit) {
				int $v = $isPrime[$p];
				int $multiple = ($v == 1) ? ($p * $p) : ($limit + 1);
				while ($multiple <= $limit) {
					$isPrime[$multiple] = 0;
					$multiple = $multiple + $p;
				}
				$p = $p + 1;
			}

			int $count = 0;
			int $idx = 2;
			while ($idx <= $limit) {
				int $isP = $isPrime[$idx];
				$count = ($isP == 1) ? ($count + 1) : $count;
				$idx = $idx + 1;
			}
			return $count;
		}

		echo countPrimes(100);
		echo countPrimes(1000);
	`
	assertDifferential(t, "sieve_algorithm", source)
}

func TestDifferential_Algorithms_Quicksort(t *testing.T) {
	source := `
		public func partition(var $arr, int $low, int $high): int {
			int $pivot = $arr[$high];
			int $i = $low;
			int $j = $low;
			while ($j < $high) {
				int $val = $arr[$j];
				int $less = ($val <= $pivot) ? 1 : 0;
				int $tmp = $arr[$j];
				$arr[$j] = ($less == 1) ? $arr[$i] : $arr[$j];
				$arr[$i] = ($less == 1) ? $tmp : $arr[$i];
				$i = ($less == 1) ? ($i + 1) : $i;
				$j = $j + 1;
			}
			int $tmp2 = $arr[$i];
			$arr[$i] = $arr[$high];
			$arr[$high] = $tmp2;
			return $i;
		}

		public func quicksort(var $arr, int $low, int $high): int {
			guard ($low < $high) else {
				return 0;
			}
			int $pi = partition($arr, $low, $high);
			quicksort($arr, $low, $pi - 1);
			quicksort($arr, $pi + 1, $high);
			return 1;
		}

		var $items = [64, 25, 12, 22, 11, 90, 42, 1, 88, 55];
		int $len = $items->length();
		quicksort($items, 0, $len - 1);

		int $k = 0;
		while ($k < $len) {
			echo $items[$k];
			$k = $k + 1;
		}
	`
	assertDifferential(t, "quicksort_algorithm", source)
}
