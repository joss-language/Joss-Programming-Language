package native_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	_ "github.com/jossecurity/joss/pkg/backend/llvm"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	core "github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

func compileAndRunNativeMemoryProbe(t *testing.T, name, src string) (string, string) {
	t.Helper()
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang no disponible")
	}
	p := parser.NewParser(parser.NewLexer(src))
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: name + ".joss", Program: astProg}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyze errors: %v", prep.Diagnostics)
	}
	irProg, err := ir.NewLowerer(prep, nil).LowerProgram(name)
	if err != nil {
		t.Fatalf("lower error: %v", err)
	}
	if err := ir.NewVerifier().Verify(irProg); err != nil {
		t.Fatalf("verify error: %v", err)
	}
	outPath := filepath.Join(t.TempDir(), name)
	if runtime.GOOS == "windows" {
		outPath += ".exe"
	}
	if _, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    backend.BackendLLVM,
		OutputPath: outPath,
	}); err != nil {
		t.Fatalf("native build failed: %v", err)
	}
	cmd := exec.Command(outPath)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("native execution failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

// TestNative_MemoryDiagnosticsAndLeakGuard verifies Rust-inspired Memory::*
// debugging primitives and acts as an automated CI growth guard that prints a
// clear Spanish diagnostic warning if unbounded memory growth is detected.
func TestNative_MemoryDiagnosticsAndLeakGuard(t *testing.T) {
	src := `
public func main(): void {
	var $s = "hola mundo joss"
	var $szStr = Memory::sizeOf($s)
	($szStr < 16) ? {
		print("FAIL_SIZEOF_STR")
	}

	var $arr = [1, 2, 3, 4]
	var $szArr = Memory::sizeOf($arr)
	($szArr < 32) ? {
		print("FAIL_SIZEOF_ARR")
	}

	var $rss = Memory::rss()
	($rss <= 0) ? {
		print("FAIL_RSS")
	}

	// Warmup + baseline snapshot
	Memory::gcCollect()
	Memory::trim()
	var $snap1 = Memory::snapshot()

	// 1) Cache put + get + forget 2000 cycles (must NOT grow unboundedly)
	var $payload = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	$payload = $payload . $payload . $payload . $payload
	$payload = $payload . $payload . $payload . $payload
	var $i = 0
	while ($i < 2000) {
		Cache::put("leak_guard_key", $payload, 60)
		var $v = Cache::get("leak_guard_key", "")
		(strlen(strval($v)) < 100) ? {
			print("FAIL_CACHE_GET")
		}
		Cache::forget("leak_guard_key")
		$i = $i + 1
	}

	Memory::gcCollect()
	Memory::trim()
	var $okCache = Memory::checkGrowth($snap1, 16 * 1024 * 1024, "Cache put/get/forget 2000 ciclos")
	(!$okCache) ? {
		print("FAIL_CACHE_GROWTH")
	}

	// 2) Intentional tight threshold check to verify warning log format
	var $snap2 = Memory::snapshot()
	var $keep = []
	var $j = 0
	while ($j < 200) {
		array_push($keep, $payload . strval($j))
		$j = $j + 1
	}
	var $warnTriggered = Memory::checkGrowth($snap2, 1024, "prueba_alerta_crecimiento")
	($warnTriggered) ? {
		print("FAIL_WARN_NOT_TRIGGERED")
	}

	print(Memory::report())
	print("MEMORY_GUARD_OK")
}
main()
`
	stdout, stderr := compileAndRunNativeMemoryProbe(t, "memory_guard", src)
	if strings.Contains(stdout, "FAIL_") {
		t.Fatalf("Advertencia ⚠️ se ha detectado falla en la memoria (MemoryDiagnosticsAndLeakGuard) en [stdout=%s | stderr=%s], se está detectando un crecimiento.", strings.TrimSpace(stdout), strings.TrimSpace(stderr))
	}
	if !strings.Contains(stdout, "MEMORY_GUARD_OK") {
		t.Fatalf("expected MEMORY_GUARD_OK, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "Memory[RSS:") {
		t.Fatalf("expected Memory::report() output, got stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "Advertencia ⚠️ se ha detectado falla en la memoria (prueba_alerta_crecimiento)") ||
		!strings.Contains(stderr, "se está detectando un crecimiento.") {
		t.Fatalf("expected Spanish memory growth warning on stderr, got: %q", stderr)
	}
	fmt.Printf("Memory guard verified:\n%s\nExpected alert sample:\n%s\n", strings.TrimSpace(stdout), strings.TrimSpace(stderr))
}
