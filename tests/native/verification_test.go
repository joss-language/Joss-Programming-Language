package native_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
)

func prepareTestProgram(t *testing.T, source string, name string) *ir.Program {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: name + ".joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram(name)
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	verifier := ir.NewVerifier()
	if err := verifier.Verify(irProg); err != nil {
		t.Fatalf("IR verification failed: %v", err)
	}

	return irProg
}

func TestVerification_TraceArtifactsAndCleanExecution(t *testing.T) {
	src := `
		int $a = 15;
		int $b = 27;
		int $c = $a + $b;
		echo "Sum:";
		echo $c;
	`
	irProg := prepareTestProgram(t, src, "test_trace_clean")

	tempDir := t.TempDir()
	outExe := filepath.Join(tempDir, "test_trace_clean.exe")

	res, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Backend:    backend.BackendAuto,
		OutputPath: outExe,
		Trace:      true,
		TraceDir:   tempDir,
	})
	if err != nil {
		t.Fatalf("BuildProgram failed: %v", err)
	}

	// 1. Verificar artefactos de Trace
	if res.IRDumpPath == "" {
		t.Errorf("expected IRDumpPath to be set in trace mode")
	} else {
		irData, err := os.ReadFile(res.IRDumpPath)
		if err != nil || !strings.Contains(string(irData), "func @main") {
			t.Errorf("IR dump file invalid or missing: %v", err)
		}
	}

	if res.LLVMPath == "" {
		t.Errorf("expected LLVMPath to be set in trace mode")
	} else {
		llvmData, err := os.ReadFile(res.LLVMPath)
		if err != nil || !strings.Contains(string(llvmData), "@main") {
			t.Errorf("LLVM file invalid or missing: %v", err)
		}
	}

	if res.BackendUsed == backend.BackendStandalone {
		if res.BootstrapPath == "" {
			t.Errorf("expected BootstrapPath to be set in trace mode when standalone backend is used")
		} else {
			goData, err := os.ReadFile(res.BootstrapPath)
			if err != nil || !strings.Contains(string(goData), "func main()") {
				t.Errorf("Bootstrap file invalid or missing: %v", err)
			}
		}
	}

	// 2. Ejecución completamente aislada en directorio temporal limpio
	isolatedDir := t.TempDir()
	isolatedExe := filepath.Join(isolatedDir, "isolated_app.exe")
	exeBytes, err := os.ReadFile(outExe)
	if err != nil {
		t.Fatalf("failed to read generated binary: %v", err)
	}
	if err := os.WriteFile(isolatedExe, exeBytes, 0755); err != nil {
		t.Fatalf("failed to copy binary to isolated dir: %v", err)
	}

	cmd := exec.Command(isolatedExe)
	cmd.Dir = isolatedDir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "SYSTEMROOT=" + os.Getenv("SYSTEMROOT")}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated execution failed: %v, output: %s", err, string(out))
	}

	expectedOutput := "Sum:\n42"
	actualOutput := strings.ReplaceAll(strings.TrimSpace(string(out)), "\r\n", "\n")
	if actualOutput != expectedOutput {
		t.Errorf("expected output %q, got %q", expectedOutput, actualOutput)
	}

	// 3. Inspección estricta de cadenas en el binario compilado
	// El binario NO debe contener el intérprete de Joss, AST, parsers ni drivers de base de datos.
	prohibitedPatterns := []string{
		"JOSSBC2Z",
		"JOSS_RUNNER_DATA",
		"github.com/jossecurity/joss/pkg/core",
		"github.com/jossecurity/joss/pkg/parser",
		"github.com/jossecurity/joss/pkg/analyzer",
		"modernc.org/sqlite",
		"github.com/jackc/pgx",
		"github.com/gorilla/websocket",
	}

	for _, pattern := range prohibitedPatterns {
		if bytes.Contains(exeBytes, []byte(pattern)) {
			t.Errorf("VIOLACIÓN: el binario nativo contiene referencia prohibida a %q", pattern)
		}
	}
}

func TestVerification_BackendSelection(t *testing.T) {
	src := `
		int $x = 100;
		return $x;
	`
	irProg := prepareTestProgram(t, src, "test_backend_sel")
	tempDir := t.TempDir()

	// 1. Standalone explícito debe compilar con éxito
	outStandalone := filepath.Join(tempDir, "standalone.exe")
	resStandalone, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Backend:    backend.BackendStandalone,
		OutputPath: outStandalone,
	})
	if err != nil {
		t.Fatalf("expected standalone backend to succeed, got error: %v", err)
	}
	if resStandalone.BackendUsed != backend.BackendStandalone {
		t.Errorf("expected BackendStandalone, got %s", resStandalone.BackendUsed)
	}

	// 2. LLVM explícito: si no hay clang/gcc en PATH, debe fallar con mensaje claro
	compiler := backend.NewNativeCompiler()
	outLLVM := filepath.Join(tempDir, "llvm.exe")
	_, errLLVM := backend.BuildProgram(irProg, backend.BuildOptions{
		Backend:    backend.BackendLLVM,
		OutputPath: outLLVM,
	})

	if !compiler.HasNativeToolchain() {
		if errLLVM == nil {
			t.Errorf("expected LLVM backend to fail when clang is not in PATH")
		} else if !strings.Contains(errLLVM.Error(), "clang") {
			t.Errorf("expected error message to mention clang/gcc, got: %v", errLLVM)
		}
	} else {
		if errLLVM != nil {
			t.Errorf("expected LLVM backend to succeed with toolchain, got: %v", errLLVM)
		}
	}
}
