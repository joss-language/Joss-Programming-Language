package native_test

import (
	"fmt"
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

func TestBenchmark_JossNativeVsJavaGoCNodePython(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang no disponible en PATH")
	}

	tmpDir := t.TempDir()
	target := backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}

	// 1. Joss Native (LLVM + Auto-DP) — fib(35)
	jossSrc := `
public func fib(int $n): int {
	($n <= 1) ? {
		return $n;
	}
	return fib($n - 1) + fib($n - 2);
}

print(fib(35));
`
	p := parser.NewParser(parser.NewLexer(jossSrc))
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	prep := analyzer.PrepareProgram([]analyzer.SourceUnit{{Path: "fib35.joss", Program: astProg}}, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyze errors: %v", prep.Diagnostics)
	}
	irProg, err := ir.NewLowerer(prep, nil).LowerProgram("fib35")
	if err != nil {
		t.Fatalf("lower error: %v", err)
	}
	jossLLVMBin := filepath.Join(tmpDir, "joss_fib35")
	if runtime.GOOS == "windows" {
		jossLLVMBin += ".exe"
	}
	if _, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     target,
		Backend:    backend.BackendLLVM,
		OutputPath: jossLLVMBin,
	}); err != nil {
		t.Fatalf("BuildProgram(LLVM) failed: %v", err)
	}
	jossLLVMInfo, _ := os.Stat(jossLLVMBin)
	t0 := time.Now()
	out, err := exec.Command(jossLLVMBin).CombinedOutput()
	jossLLVMDur := time.Since(t0)
	if err != nil || strings.TrimSpace(string(out)) != "9227465" {
		t.Fatalf("Joss LLVM fib(35) failed: %v out=%q", err, string(out))
	}

	type benchRow struct {
		lang   string
		dur    time.Duration
		size   int64
		status string
	}
	rows := []benchRow{
		{lang: "Joss Native (LLVM + Auto-DP)", dur: jossLLVMDur, size: jossLLVMInfo.Size(), status: "OK"},
	}

	// 2. C (clang -O2) — fib(35)
	cFile := filepath.Join(tmpDir, "fib35.c")
	cBin := filepath.Join(tmpDir, "c_fib35")
	if runtime.GOOS == "windows" {
		cBin += ".exe"
	}
	_ = os.WriteFile(cFile, []byte(`#include <stdio.h>
long long fib(long long n){ if(n<=1) return n; return fib(n-1)+fib(n-2); }
int main(void){ printf("%lld\n", fib(35)); return 0; }
`), 0644)
	if err := exec.Command("clang", "-O2", cFile, "-o", cBin).Run(); err == nil {
		cInfo, _ := os.Stat(cBin)
		t0 = time.Now()
		cOut, cErr := exec.Command(cBin).CombinedOutput()
		cDur := time.Since(t0)
		if cErr == nil && strings.TrimSpace(string(cOut)) == "9227465" {
			rows = append(rows, benchRow{lang: "C (clang -O2)", dur: cDur, size: cInfo.Size(), status: "OK"})
		}
	}

	// 3. Go compiled — fib(35)
	if goPath, err := exec.LookPath("go"); err == nil {
		goFile := filepath.Join(tmpDir, "fib35.go")
		goBin := filepath.Join(tmpDir, "go_fib35")
		if runtime.GOOS == "windows" {
			goBin += ".exe"
		}
		_ = os.WriteFile(goFile, []byte(`package main
import "fmt"
func fib(n int64) int64 { if n <= 1 { return n }; return fib(n-1) + fib(n-2) }
func main() { fmt.Println(fib(35)) }
`), 0644)
		if err := exec.Command(goPath, "build", "-ldflags=-s -w", "-o", goBin, goFile).Run(); err == nil {
			goInfo, _ := os.Stat(goBin)
			t0 = time.Now()
			goOut, goErr := exec.Command(goBin).CombinedOutput()
			goDur := time.Since(t0)
			if goErr == nil && strings.TrimSpace(string(goOut)) == "9227465" {
				rows = append(rows, benchRow{lang: "Go (compiled)", dur: goDur, size: goInfo.Size(), status: "OK"})
			}
		}
	}

	// 4. Java (OpenJDK javac + java) — fib(35)
	if javacPath, err := exec.LookPath("javac"); err == nil {
		if javaPath, err2 := exec.LookPath("java"); err2 == nil {
			javaFile := filepath.Join(tmpDir, "Fib35.java")
			_ = os.WriteFile(javaFile, []byte(`public class Fib35 {
    static long fib(long n) { return n <= 1 ? n : fib(n - 1) + fib(n - 2); }
    public static void main(String[] args) { System.out.println(fib(35)); }
}
`), 0644)
			if err := exec.Command(javacPath, javaFile).Run(); err == nil {
				classInfo, _ := os.Stat(filepath.Join(tmpDir, "Fib35.class"))
				t0 = time.Now()
				jOut, jErr := exec.Command(javaPath, "-cp", tmpDir, "Fib35").CombinedOutput()
				jDur := time.Since(t0)
				if jErr == nil && strings.TrimSpace(string(jOut)) == "9227465" {
					rows = append(rows, benchRow{lang: "Java 17 (OpenJDK)", dur: jDur, size: classInfo.Size(), status: "OK"})
				}
			}
		}
	}

	// 5. Node.js — fib(35)
	if nodePath, err := exec.LookPath("node"); err == nil {
		jsFile := filepath.Join(tmpDir, "fib35.js")
		_ = os.WriteFile(jsFile, []byte(`function fib(n){ return n <= 1 ? n : fib(n-1) + fib(n-2); } console.log(fib(35));`), 0644)
		t0 = time.Now()
		nOut, nErr := exec.Command(nodePath, jsFile).CombinedOutput()
		nDur := time.Since(t0)
		if nErr == nil && strings.TrimSpace(string(nOut)) == "9227465" {
			rows = append(rows, benchRow{lang: "Node.js", dur: nDur, size: 0, status: "OK"})
		}
	}

	// 6. Python 3 — fib(35)
	if pyPath, err := exec.LookPath("python3"); err == nil {
		pyFile := filepath.Join(tmpDir, "fib35.py")
		_ = os.WriteFile(pyFile, []byte(`def fib(n): return n if n <= 1 else fib(n-1) + fib(n-2)
print(fib(35))`), 0644)
		t0 = time.Now()
		pOut, pErr := exec.Command(pyPath, pyFile).CombinedOutput()
		pDur := time.Since(t0)
		if pErr == nil && strings.TrimSpace(string(pOut)) == "9227465" {
			rows = append(rows, benchRow{lang: "Python 3", dur: pDur, size: 0, status: "OK"})
		}
	}

	var report strings.Builder
	report.WriteString("\n=== BENCHMARK fib(35) = 9227465 (v3.6.8.3) ===\n")
	for _, r := range rows {
		sizeStr := "interpreted"
		if r.size > 0 {
			sizeStr = fmt.Sprintf("%d B", r.size)
		}
		report.WriteString(fmt.Sprintf("%-30s | %12v | %14s | %s\n", r.lang, r.dur.Round(time.Microsecond), sizeStr, r.status))
	}
	t.Log(report.String())
}
