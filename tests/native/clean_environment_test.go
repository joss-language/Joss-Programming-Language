package native_test

import (
	"bytes"
	"debug/elf"
	"debug/pe"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/ir"
	jossparser "github.com/jossecurity/joss/pkg/parser"
)

// TestCleanEnvironment001_JossBuildWithoutGoGCCClangMSVC (JOSS-CLEAN-ENVIRONMENT-001)
// prepares an isolated environment where Go, GCC, Clang, MSVC, MinGW, CMake, and Ninja
// are NOT installed in PATH — only the `joss` binary exists — and verifies that
// `joss build hello.joss` produces a working native `hello.exe` that runs on a clean machine.
func TestCleanEnvironment001_JossBuildWithoutGoGCCClangMSVC(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}

	// 1. Build the standalone `joss` compiler binary once into an isolated directory
	jossInstallDir := t.TempDir()
	jossBinName := "joss"
	if runtime.GOOS == "windows" {
		jossBinName = "joss.exe"
	}
	jossBinPath := filepath.Join(jossInstallDir, jossBinName)

	buildJossCmd := exec.Command("go", "build", "-o", jossBinPath, "./cmd/joss")
	buildJossCmd.Dir = repoRoot
	if out, err := buildJossCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build joss CLI for clean environment test: %v\n%s", err, string(out))
	}

	// 2. Verify that in an environment with PATH=jossInstallDir, NO external compilers exist
	forbiddenTools := []string{"go", "gcc", "clang", "cl", "mingw32-make", "cmake", "ninja", "lld", "ld"}
	for _, tool := range forbiddenTools {
		toolExe := tool
		if runtime.GOOS == "windows" {
			toolExe += ".exe"
		}
		if _, err := os.Stat(filepath.Join(jossInstallDir, toolExe)); err == nil {
			t.Fatalf("clean environment directory unexpectedly contains %s", toolExe)
		}
	}

	// 3. Create hello.joss in a separate user workspace directory
	userWorkDir := t.TempDir()
	helloSource := `
public func sumar(int $x, int $y): int {
	return $x + $y;
}

int $a = 10;
int $b = 20;
echo sumar($a, $b);
echo "Hola Joss Standalone";
`
	helloFile := filepath.Join(userWorkDir, "hello.joss")
	if err := os.WriteFile(helloFile, []byte(helloSource), 0644); err != nil {
		t.Fatalf("failed to write hello.joss: %v", err)
	}

	// 4. Execute `joss build hello.joss --trace` with PATH containing ONLY jossInstallDir
	// (Go = NOT INSTALLED, GCC = NOT INSTALLED, Clang = NOT INSTALLED, MSVC = NOT INSTALLED)
	cleanBuildEnv := []string{
		"PATH=" + jossInstallDir,
		"GOROOT=",
		"GOPATH=",
		"CC=",
		"CXX=",
		"SYSTEMROOT=" + os.Getenv("SYSTEMROOT"),
		"WINDIR=" + os.Getenv("WINDIR"),
		"TMP=" + userWorkDir,
		"TEMP=" + userWorkDir,
	}

	jossBuildCmd := exec.Command(jossBinPath, "build", "hello.joss", "--trace")
	jossBuildCmd.Dir = userWorkDir
	jossBuildCmd.Env = cleanBuildEnv
	buildOut, err := jossBuildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("[JOSS-CLEAN-ENVIRONMENT-001] `joss build hello.joss` failed in clean environment without Go/GCC/Clang: %v\nOutput:\n%s", err, string(buildOut))
	}

	expectedExe := filepath.Join(userWorkDir, "hello")
	if runtime.GOOS == "windows" {
		expectedExe += ".exe"
	}
	fi, err := os.Stat(expectedExe)
	if err != nil {
		t.Fatalf("[JOSS-CLEAN-ENVIRONMENT-001] expected output executable %s was not created: %v", expectedExe, err)
	}
	if fi.Size() == 0 {
		t.Fatalf("[JOSS-CLEAN-ENVIRONMENT-001] generated executable is empty")
	}

	// 5. Copy `hello.exe` to a completely empty target machine directory (no Joss, no Go, no source files)
	cleanRunDir := t.TempDir()
	cleanRunExe := filepath.Join(cleanRunDir, filepath.Base(expectedExe))
	exeBytes, err := os.ReadFile(expectedExe)
	if err != nil {
		t.Fatalf("failed to read hello executable: %v", err)
	}
	if err := os.WriteFile(cleanRunExe, exeBytes, 0755); err != nil {
		t.Fatalf("failed to copy hello executable to clean run dir: %v", err)
	}

	runCmd := exec.Command(cleanRunExe)
	runCmd.Dir = cleanRunDir
	runCmd.Env = []string{
		"PATH=",
		"SYSTEMROOT=" + os.Getenv("SYSTEMROOT"),
		"WINDIR=" + os.Getenv("WINDIR"),
	}
	runOut, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("[JOSS-CLEAN-ENVIRONMENT-001] execution of standalone binary failed on clean machine: %v\nOutput:\n%s", err, string(runOut))
	}

	got := strings.ReplaceAll(strings.TrimSpace(string(runOut)), "\r\n", "\n")
	want := "30\nHola Joss Standalone"
	if got != want {
		t.Fatalf("[JOSS-CLEAN-ENVIRONMENT-001] unexpected output from standalone binary:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestCleanEnvironment002_PEAndELFBinaryInspection_ZeroGoZeroInterpreter inspects the
// generated PE32+ (Windows x86_64) and ELF64 (Linux x86_64) executables to prove they
// contain pure native machine code, only import OS APIs (KERNEL32.dll on Windows, 0 libs on Linux),
// and contain zero Go runtime symbols, zero Go build IDs, and zero Joss AST interpreter artifacts.
func TestCleanEnvironment002_PEAndELFBinaryInspection_ZeroGoZeroInterpreter(t *testing.T) {
	src := `
		int $a = 10;
		int $b = 20;
		echo $a + $b;
	`
	irProg := prepareTestProgram(t, src, "binary_audit")
	tempDir := t.TempDir()

	// 1. Build and inspect Windows x86_64 PE32+ executable
	winExe := filepath.Join(tempDir, "audit_win.exe")
	_, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     backend.Target{OS: "windows", Arch: "amd64"},
		OutputPath: winExe,
	})
	if err != nil {
		t.Fatalf("failed to build windows-amd64 executable: %v", err)
	}

	winBytes, err := os.ReadFile(winExe)
	if err != nil {
		t.Fatalf("failed to read windows executable: %v", err)
	}

	// Verify compact native size (< 32 KB; Go binaries are > 1.5 MB)
	if len(winBytes) > 32*1024 {
		t.Errorf("expected compact native binary (< 32 KB), got %d bytes", len(winBytes))
	}

	peFile, err := pe.Open(winExe)
	if err != nil {
		t.Fatalf("generated Windows binary is not a valid PE file: %v", err)
	}
	defer peFile.Close()

	if peFile.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		t.Errorf("expected IMAGE_FILE_MACHINE_AMD64 (0x8664), got 0x%x", peFile.Machine)
	}

	libs, err := peFile.ImportedLibraries()
	if err != nil {
		t.Fatalf("failed to inspect PE imported libraries: %v", err)
	}
	for _, lib := range libs {
		if !strings.EqualFold(lib, "KERNEL32.dll") {
			t.Errorf("unexpected imported library %q; standalone binary must only depend on OS APIs (KERNEL32.dll)", lib)
		}
	}

	forbiddenMarkers := []string{
		"go.buildid",
		"Go build ID:",
		"runtime.main",
		"runtime.gopanic",
		"runtime/cgo",
		"libgo",
		"JOSSBC2Z",
		"JOSS_RUNNER_DATA",
		"ExecutePrepared",
		"evaluateExpression",
		"github.com/jossecurity/joss/pkg/core",
		"github.com/jossecurity/joss/pkg/parser",
	}
	for _, marker := range forbiddenMarkers {
		if bytes.Contains(winBytes, []byte(marker)) {
			t.Errorf("VIOLATION: Windows native binary contains forbidden Go/Interpreter marker %q", marker)
		}
	}

	// 2. Build and inspect Linux x86_64 ELF64 executable
	linuxExe := filepath.Join(tempDir, "audit_linux")
	_, err = backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     backend.Target{OS: "linux", Arch: "amd64"},
		OutputPath: linuxExe,
	})
	if err != nil {
		t.Fatalf("failed to build linux-amd64 executable: %v", err)
	}

	elfFile, err := elf.Open(linuxExe)
	if err != nil {
		t.Fatalf("generated Linux binary is not a valid ELF64 file: %v", err)
	}
	defer elfFile.Close()

	if elfFile.Machine != elf.EM_X86_64 {
		t.Errorf("expected EM_X86_64, got %v", elfFile.Machine)
	}
	elfLibs, _ := elfFile.ImportedLibraries()
	if len(elfLibs) != 0 {
		t.Errorf("expected 0 dynamic library dependencies in static Linux ELF64 executable, got %v", elfLibs)
	}
}

// TestCleanEnvironment003_NoExternalToolchainInvocationRegression verifies that no file in
// pkg/backend/native imports "os/exec" or invokes external compilers (go, gcc, clang, cl).
func TestCleanEnvironment003_NoExternalToolchainInvocationRegression(t *testing.T) {
	backendDir, err := filepath.Abs(filepath.Join("..", "..", "pkg", "backend", "native"))
	if err != nil {
		t.Fatalf("failed to locate pkg/backend/native: %v", err)
	}

	entries, err := os.ReadDir(backendDir)
	if err != nil {
		t.Fatalf("failed to read pkg/backend/native: %v", err)
	}

	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fullPath := filepath.Join(backendDir, name)
		srcBytes, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", fullPath, err)
		}

		f, err := parser.ParseFile(fset, fullPath, srcBytes, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("failed to parse %s: %v", fullPath, err)
		}
		for _, imp := range f.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			if impPath == "os/exec" {
				t.Errorf("ARCHITECTURAL REGRESSION: %s imports \"os/exec\"; native backend must NEVER invoke external compilers", name)
			}
		}

		srcText := string(srcBytes)
		if strings.Contains(srcText, "exec.Command(") || strings.Contains(srcText, "exec.LookPath(") {
			t.Errorf("ARCHITECTURAL REGRESSION: %s contains exec.Command/LookPath invocation", name)
		}
	}
}

// TestCleanEnvironment004_NoGoFallbackAndExplicitJOSSNative001 verifies that unsupported
// features or unavailable targets fail explicitly with JOSS-NATIVE-001 or target error
// and never fall back to Go or the interpreter.
func TestCleanEnvironment004_NoGoFallbackAndExplicitJOSSNative001(t *testing.T) {
	// 1. Unsupported language feature -> JOSS-NATIVE-001
	unsupportedSrc := `
		var $items = [1, 2, 3];
		foreach ($items as $x) {
			echo $x;
		}
	`
	p := jossparser.NewParser(jossparser.NewLexer(unsupportedSrc))
	prog := p.ParseProgram()
	units := []analyzer.SourceUnit{{Path: "unsupported_foreach.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	lowerer := ir.NewLowerer(prep, nil)
	_, err := lowerer.LowerProgram("unsupported_foreach")
	if err == nil {
		t.Fatal("expected unsupported feature 'foreach' to fail lowering with JOSS-NATIVE-001")
	}
	if !strings.Contains(err.Error(), "JOSS-NATIVE-001") {
		t.Errorf("expected JOSS-NATIVE-001 diagnostic, got: %v", err)
	}

	// 2. Unavailable target architecture -> ERROR: native target not available
	validSrc := `
		int $x = 1;
		echo $x;
	`
	irProg := prepareTestProgram(t, validSrc, "target_check")
	_, targetErr := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     backend.Target{OS: "darwin", Arch: "arm64"},
		OutputPath: filepath.Join(t.TempDir(), "out_darwin"),
	})
	if targetErr == nil {
		t.Fatal("expected unavailable target darwin/arm64 to fail explicitly")
	}
	if !strings.Contains(targetErr.Error(), "ERROR: native target not available") {
		t.Errorf("expected 'ERROR: native target not available', got: %v", targetErr)
	}
}
