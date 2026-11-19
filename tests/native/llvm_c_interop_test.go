package native_test

import (
	"bytes"
	"context"
	"os"
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

func compileAndRunLLVM(t *testing.T, source string, name string, staticLibs []string) (string, string, int, error) {
	t.Helper()
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang not available on PATH")
	}

	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors in %s: %v", name, p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: name + ".joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors in %s: %v", name, prep.Diagnostics)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram(name)
	if err != nil {
		t.Fatalf("IR lowering failed in %s: %v", name, err)
	}
	if err := ir.NewVerifier().Verify(irProg); err != nil {
		t.Fatalf("IR verification failed in %s: %v", name, err)
	}

	tmpDir := t.TempDir()
	outExe := filepath.Join(tmpDir, name)
	if runtime.GOOS == "windows" {
		outExe += ".exe"
	}

	opts := backend.BuildOptions{
		Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    backend.BackendLLVM,
		OutputPath: outExe,
		StaticLibs: staticLibs,
	}
	if _, err := backend.BuildProgram(irProg, opts); err != nil {
		t.Fatalf("LLVM BuildProgram failed in %s: %v", name, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, outExe)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), exitCode, runErr
}

func TestLLVMBackend_Differential_BuffersAndMixed(t *testing.T) {
	source := `
public func sum_mixed(mixed $a, mixed $b): int {
	return intval($a) + intval($b);
}

var $buf = new ByteBuffer(4);
$buf->appendByte(65);
$buf->appendByte(66);
$buf->set(0, 67);
print($buf->len());
print($buf->get(0));
print($buf->get(1));
$buf->free();

var $sb = new StringBuilder();
$sb->append("Joss");
$sb->append("-");
$sb->appendInt(2026);
print($sb->toString());
print($sb->len());
$sb->free();

print(sum_mixed(10, 32));
`
	interpOut, ok, errInfo := runInterpreter(t, source)
	if !ok {
		t.Fatalf("interpreter failed: %s", errInfo)
	}
	llvmOut, llvmErr, exitCode, runErr := compileAndRunLLVM(t, source, "llvm_buf_mixed", nil)
	if runErr != nil || exitCode != 0 {
		t.Fatalf("LLVM binary failed (exit %d, err=%v, stderr=%s)", exitCode, runErr, llvmErr)
	}
	if interpOut != llvmOut {
		t.Fatalf("differential mismatch:\ninterpreter:\n%s\nllvm:\n%s", interpOut, llvmOut)
	}
}

func TestLLVMBackend_ByteBufferBoundsCheck(t *testing.T) {
	source := `
var $buf = new ByteBuffer(2);
$buf->appendByte(10);
print($buf->get(5));
`
	_, ok, _ := runInterpreter(t, source)
	if ok {
		t.Fatalf("expected interpreter to fail on out-of-bounds ByteBuffer.get(5)")
	}
	_, _, exitCode, runErr := compileAndRunLLVM(t, source, "llvm_buf_oob", nil)
	if runErr == nil && exitCode == 0 {
		t.Fatalf("expected LLVM binary to abort on out-of-bounds ByteBuffer.get(5)")
	}
}

func TestPhase2_ExternCStaticLibraryLinking(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang not available on PATH")
	}
	arPath, err := exec.LookPath("ar")
	if err != nil {
		t.Skip("ar not available on PATH")
	}

	tmpDir := t.TempDir()
	cSrc := filepath.Join(tmpDir, "mymath.c")
	cObj := filepath.Join(tmpDir, "mymath.o")
	libA := filepath.Join(tmpDir, "libmymath.a")

	cCode := `#include <stdint.h>
int64_t c_mix_add(int64_t a, int64_t b) {
    return (a * 10) + b;
}
const char* c_greet(void) {
    return "Hello from C static lib!";
}
`
	if err := os.WriteFile(cSrc, []byte(cCode), 0644); err != nil {
		t.Fatal(err)
	}
	clangPath, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang not available on PATH")
	}
	if out, err := exec.Command(clangPath, "-c", "-Os", cSrc, "-o", cObj).CombinedOutput(); err != nil {
		t.Fatalf("clang -c failed: %v\n%s", err, string(out))
	}
	if out, err := exec.Command(arPath, "rcs", libA, cObj).CombinedOutput(); err != nil {
		t.Fatalf("ar rcs failed: %v\n%s", err, string(out))
	}

	jossSrc := `
extern "C" library "` + filepath.ToSlash(libA) + `" {
    public func c_mix_add(int $a, int $b): int;
    public func c_greet(): string;
}

print(c_mix_add(4, 2));
print(c_greet());
`
	p := parser.NewParser(parser.NewLexer(jossSrc))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := []analyzer.SourceUnit{{Path: "c_interop.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("c_interop")
	if err != nil {
		t.Fatalf("IR lowering failed: %v", err)
	}
	if err := ir.NewVerifier().Verify(irProg); err != nil {
		t.Fatalf("IR verification failed: %v", err)
	}

	outExe := filepath.Join(tmpDir, "c_interop_bin")
	if runtime.GOOS == "windows" {
		outExe += ".exe"
	}
	opts := backend.BuildOptions{
		Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    backend.BackendLLVM,
		OutputPath: outExe,
	}
	res, err := backend.BuildProgram(irProg, opts)
	if err != nil {
		t.Fatalf("BuildProgram failed: %v", err)
	}

	out, err := exec.Command(outExe).CombinedOutput()
	if err != nil {
		t.Fatalf("executing c_interop_bin failed: %v\n%s", err, string(out))
	}
	got := strings.TrimSpace(string(out))
	want := "42\nHello from C static lib!"
	if got != want {
		t.Fatalf("unexpected C interop output:\ngot:\n%s\nwant:\n%s", got, want)
	}

	// Verify standalone binary has zero Go runtime and zero Joss DLLs
	if runtime.GOOS == "linux" {
		if lddOut, err := exec.Command("ldd", outExe).CombinedOutput(); err == nil {
			lddStr := string(lddOut)
			if strings.Contains(lddStr, "joss") || strings.Contains(lddStr, "libgo") {
				t.Fatalf("unexpected Joss/Go dynamic dependency in ldd:\n%s", lddStr)
			}
		}
		if nmPath, err := exec.LookPath("nm"); err == nil {
			if nmOut, err := exec.Command(nmPath, outExe).CombinedOutput(); err == nil {
				if strings.Contains(string(nmOut), "runtime.gopanic") {
					t.Fatalf("Go runtime detected in native binary")
				}
			}
		}
	}
	t.Logf("Standalone C-linked ELF binary size: %d bytes", res.ExecutableSize)
}
