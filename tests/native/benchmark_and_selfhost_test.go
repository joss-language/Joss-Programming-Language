package native_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	core "github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
)

func TestPhase8_SizeAndPerformanceComparison(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang not available on PATH")
	}

	source := `
public func fib(int $n): int {
	($n <= 1) ? {
		return $n;
	}
	return fib($n - 1) + fib($n - 2);
}

print(fib(25));
`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := []analyzer.SourceUnit{{Path: "bench_fib.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("bench_fib")
	if err != nil {
		t.Fatalf("IR lowering failed: %v", err)
	}
	if err := ir.NewVerifier().Verify(irProg); err != nil {
		t.Fatalf("IR verify failed: %v", err)
	}

	tmpDir := t.TempDir()
	nativeExe := filepath.Join(tmpDir, "fib_integrated")
	llvmExe := filepath.Join(tmpDir, "fib_llvm")

	t0 := time.Now()
	resNative, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    backend.BackendNative,
		OutputPath: nativeExe,
	})
	nativeCompileDur := time.Since(t0)
	if err != nil {
		t.Fatalf("BackendNative failed: %v", err)
	}

	t1 := time.Now()
	resLLVM, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    backend.BackendLLVM,
		OutputPath: llvmExe,
	})
	llvmCompileDur := time.Since(t1)
	if err != nil {
		t.Fatalf("BackendLLVM failed: %v", err)
	}

	runT0 := time.Now()
	outN, err := exec.Command(nativeExe).CombinedOutput()
	nativeRunDur := time.Since(runT0)
	if err != nil {
		t.Fatalf("running nativeExe failed: %v", err)
	}

	runT1 := time.Now()
	outL, err := exec.Command(llvmExe).CombinedOutput()
	llvmRunDur := time.Since(runT1)
	if err != nil {
		t.Fatalf("running llvmExe failed: %v", err)
	}

	if strings.TrimSpace(string(outN)) != "75025" || strings.TrimSpace(string(outL)) != "75025" {
		t.Fatalf("unexpected fib(25) output: native=%q llvm=%q", string(outN), string(outL))
	}

	t.Logf("BackendNative: size=%d bytes, compile=%v, exec=%v", resNative.ExecutableSize, nativeCompileDur, nativeRunDur)
	t.Logf("BackendLLVM:   size=%d bytes, compile=%v, exec=%v", resLLVM.ExecutableSize, llvmCompileDur, llvmRunDur)
}

func TestPhase9_SelfHostingLexerSubset_PureJoss(t *testing.T) {
	source := `
public func is_digit(int $b): bool {
	return $b >= 48 && $b <= 57;
}

public func is_alpha(int $b): bool {
	return ($b >= 65 && $b <= 90) || ($b >= 97 && $b <= 122) || $b == 95 || $b == 36;
}

public func tokenize_and_count(string $code): int {
	var $buf = new ByteBuffer(64);
	$buf->appendString($code);
	int $n = $buf->len();
	int $i = 0;
	int $tokens = 0;

	while ($i < $n) {
		int $ch = $buf->get($i);
		(($ch == 32) || ($ch == 9) || ($ch == 10)) ? {
			$i = $i + 1;
		} : {
			(is_alpha($ch)) ? {
				while ($i < $n && (is_alpha($buf->get($i)) || is_digit($buf->get($i)))) {
					$i = $i + 1;
				}
				$tokens = $tokens + 1;
			} : {
				(is_digit($ch)) ? {
					while ($i < $n && is_digit($buf->get($i))) {
						$i = $i + 1;
					}
					$tokens = $tokens + 1;
				} : {
					$i = $i + 1;
					$tokens = $tokens + 1;
				}
			}
		}
	}
	$buf->free();
	return $tokens;
}

print(tokenize_and_count("var $x = 10 + 20;"));
`
	interpOut, ok, errInfo := runInterpreter(t, source)
	if !ok {
		t.Fatalf("interpreter failed: %s", errInfo)
	}
	llvmOut, llvmErr, exitCode, runErr := compileAndRunLLVM(t, source, "selfhost_lexer", nil)
	if runErr != nil || exitCode != 0 {
		t.Fatalf("LLVM selfhost_lexer failed (exit %d, err=%v, stderr=%s)", exitCode, runErr, llvmErr)
	}
	if interpOut != llvmOut || interpOut != "7" {
		t.Fatalf("selfhost lexer mismatch: interp=%q llvm=%q want=\"7\"", interpOut, llvmOut)
	}
}
