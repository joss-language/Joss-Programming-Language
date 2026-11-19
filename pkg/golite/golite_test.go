package golite_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/jossecurity/joss/pkg/backend/llvm"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/golite"
	"github.com/jossecurity/joss/pkg/ir"
)

func TestGoLite_CompileAndExecuteNativeWithoutGoRuntime(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang not available on PATH")
	}

	goSrc := `package main

func fib(n int64) int64 {
	if n <= 1 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func sumSquares(limit int64) int64 {
	var acc int64 = 0
	for i := int64(1); i <= limit; i++ {
		acc += i * i
	}
	return acc
}

func main() int64 {
	println(fib(10))
	println(sumSquares(5))
	return 0
}
`
	prog, err := golite.CompileToIR("golite_algo", goSrc)
	if err != nil {
		t.Fatalf("CompileToIR failed: %v", err)
	}
	if err := ir.NewVerifier().Verify(prog); err != nil {
		t.Fatalf("IR verify failed: %v", err)
	}
	ir.OptimizeProgram(prog)

	outExe := filepath.Join(t.TempDir(), "golite_bin")
	if runtime.GOOS == "windows" {
		outExe += ".exe"
	}
	res, err := backend.BuildProgram(prog, backend.BuildOptions{
		Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    backend.BackendLLVM,
		OutputPath: outExe,
	})
	if err != nil {
		t.Fatalf("BuildProgram failed: %v", err)
	}

	out, err := exec.Command(outExe).CombinedOutput()
	if err != nil {
		t.Fatalf("running golite_bin failed: %v\n%s", err, string(out))
	}
	got := strings.TrimSpace(strings.ReplaceAll(string(out), "\r\n", "\n"))
	want := "55\n55"
	if got != want {
		t.Fatalf("unexpected output:\ngot:\n%s\nwant:\n%s", got, want)
	}
	t.Logf("Go-Lite standalone binary size (zero Go runtime): %d bytes", res.ExecutableSize)
}
