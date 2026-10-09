package native_test

import (
	"errors"
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

	// 3. Comparación diferencial exacta
	if interpOut != nativeOut {
		t.Errorf("[%s] Discrepancia diferencial detectada:\n--- Intérprete (joss run) ---\n%s\n--- Nativo (joss build) ---\n%s", name, interpOut, nativeOut)
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

func TestDifferential_CapabilityContract_ClassesRejectedByCompiler(t *testing.T) {
	// Las clases están soportadas en el intérprete pero no en el backend nativo mínimo.
	// El compilador debe rechazar la compilación con UnsupportedCapabilityError y sugerir joss run.
	source := `
		public class Persona {
			public string $nombre = "Joss";
		}
		var $p = new Persona();
		echo $p->nombre;
	`

	// 1. El intérprete debe ser capaz de ejecutarlo
	interpOut, interpOk, interpErr := runInterpreter(t, source)
	if !interpOk || interpOut != "Joss" {
		t.Fatalf("el intérprete debería ejecutar la clase: out=%q, ok=%v, err=%s", interpOut, interpOk, interpErr)
	}

	// 2. El compilador nativo debe rechazarlo explícitamente sin fallbacks silenciosos
	_, _, err := compileAndRunNative(t, source, "class_rejection")
	if err == nil {
		t.Fatalf("se esperaba que joss build rechazara la clase con UnsupportedCapabilityError, pero no falló")
	}

	var capErr *ir.UnsupportedCapabilityError
	if !errors.As(err, &capErr) {
		t.Fatalf("se esperaba *ir.UnsupportedCapabilityError, se obtuvo: %T (%v)", err, err)
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "[JOSS-NATIVE-CAPABILITY]") {
		t.Errorf("el mensaje de error no contiene el tag [JOSS-NATIVE-CAPABILITY]: %s", errStr)
	}
	if !strings.Contains(errStr, "Language:\n    supported") {
		t.Errorf("el mensaje no contiene Language: supported: %s", errStr)
	}
	if !strings.Contains(errStr, "Interpreter:\n    supported") {
		t.Errorf("el mensaje no contiene Interpreter: supported: %s", errStr)
	}
	if !strings.Contains(errStr, "Native compiler:\n    not currently supported") {
		t.Errorf("el mensaje no contiene Native compiler: not currently supported: %s", errStr)
	}
	if !strings.Contains(errStr, "joss run") {
		t.Errorf("el mensaje de error no sugiere usar 'joss run': %s", errStr)
	}
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

func TestDifferential_CapabilityContract_DynamicArrayRejected(t *testing.T) {
	// Arrays dinámicos literales funcionan en el intérprete pero no en el backend nativo mínimo.
	source := `
		var $arr = [1, 2, 3];
		echo $arr[0];
	`

	// 1. Intérprete lo ejecuta
	interpOut, interpOk, _ := runInterpreter(t, source)
	if !interpOk || interpOut != "1" {
		t.Fatalf("intérprete falló ejecutando array literal: out=%q", interpOut)
	}

	// 2. Compilador nativo lo rechaza estrictamente
	_, _, err := compileAndRunNative(t, source, "array_rejection")
	if err == nil {
		t.Fatalf("se esperaba que joss build rechazara array literal, pero compiló")
	}

	var capErr *ir.UnsupportedCapabilityError
	if !errors.As(err, &capErr) {
		t.Fatalf("se esperaba *ir.UnsupportedCapabilityError, se obtuvo %T (%v)", err, err)
	}
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
