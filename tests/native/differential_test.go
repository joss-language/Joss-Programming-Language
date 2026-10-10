package native_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/mobile"
	"github.com/jossecurity/joss/pkg/parser"
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
	t.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors in %s: %v", name, p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: name + ".joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
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

	cmd := exec.Command(res.OutputPath)
	outBytes, runErr := cmd.CombinedOutput()
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
