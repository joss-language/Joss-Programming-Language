package native_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/analyzer"
	_ "github.com/jossecurity/joss/pkg/backend/llvm"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	core "github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
)

func TestNative_AutoDynamicProgramming_Fib50AndGridPaths(t *testing.T) {
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

public func grid_paths(int $r, int $c): int {
	($r == 0 || $c == 0) ? {
		return 1;
	}
	return grid_paths($r - 1, $c) + grid_paths($r, $c - 1);
}

print(fib(50));
print(grid_paths(15, 15));
`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := []analyzer.SourceUnit{{Path: "auto_dp.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	irProg, err := ir.NewLowerer(prep, nil).LowerProgram("auto_dp")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	if err := ir.NewVerifier().Verify(irProg); err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	outExe := filepath.Join(t.TempDir(), "auto_dp_bin")
	if runtime.GOOS == "windows" {
		outExe += ".exe"
	}
	res, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    backend.BackendLLVM,
		OutputPath: outExe,
	})
	if err != nil {
		t.Fatalf("BuildProgram failed: %v", err)
	}

	t0 := time.Now()
	out, err := exec.Command(outExe).CombinedOutput()
	dur := time.Since(t0)
	if err != nil {
		t.Fatalf("execution failed: %v\n%s", err, string(out))
	}
	got := strings.TrimSpace(string(out))
	want := "12586269025\n155117520"
	if got != want {
		t.Fatalf("Auto-DP output mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
	t.Logf("[Auto-DP] fib(50) + grid_paths(15,15) executed in %v (binary=%d bytes)", dur, res.ExecutableSize)
}

func TestNative_ParallelGoroutines_AsyncAwaitAndChannels(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang not available on PATH")
	}

	source := `
public func work_sum(int $n): int {
	int $acc = 0;
	int $i = 1;
	while ($i <= $n) {
		$acc = $acc + $i;
		$i = $i + 1;
	}
	return $acc;
}

var $f1 = async { return work_sum(10000); };
var $f2 = async { return work_sum(20000); };
var $f3 = async { return work_sum(30000); };
var $f4 = async { return work_sum(40000); };

int $r1 = intval(await $f1);
int $r2 = intval(await $f2);
int $r3 = intval(await $f3);
int $r4 = intval(await $f4);

print($r1);
print($r2);
print($r3);
print($r4);
print($r1 + $r2 + $r3 + $r4);
`
	interpOut, ok, errInfo := runInterpreter(t, source)
	if !ok {
		t.Fatalf("interpreter failed: %s", errInfo)
	}

	llvmOut, llvmErr, exitCode, runErr := compileAndRunLLVM(t, source, "parallel_async", nil)
	if runErr != nil || exitCode != 0 {
		t.Fatalf("LLVM parallel_async failed (exit=%d, err=%v, stderr=%s)", exitCode, runErr, llvmErr)
	}
	if interpOut != llvmOut {
		t.Fatalf("parallel async differential mismatch:\ninterpreter:\n%s\nllvm:\n%s", interpOut, llvmOut)
	}
	t.Logf("[Parallel Goroutines] 4 parallel async/await tasks matched Go interpreter 100%%:\n%s", llvmOut)
}
