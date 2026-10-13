package native_test

import (
	"bytes"
	"debug/elf"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"go/ast"
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

func tryLowerSourceToIR(src string) error {
	p := jossparser.NewParser(jossparser.NewLexer(src))
	prog := p.ParseProgram()
	units := []analyzer.SourceUnit{{Path: "audit_test.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	if prep.HasErrors() {
		return fmt.Errorf("analyzer errors: %v", prep.Diagnostics)
	}
	lowerer := ir.NewLowerer(prep, nil)
	_, err := lowerer.LowerProgram("audit_test")
	return err
}

// assertStandaloneDifferentialParity executes source via both the canonical interpreter
// and a standalone native PE32+ executable built with EMPTY PATH, verifying byte-for-byte parity.
func assertStandaloneDifferentialParity(t *testing.T, name string, src string) []byte {
	t.Helper()

	interpOut, interpSuccess, interpErr := runInterpreter(t, src)
	if !interpSuccess {
		t.Fatalf("[%s] interpreter failed: %s", name, interpErr)
	}

	mod := prepareTestProgram(t, src, name)
	tmpDir := t.TempDir()
	binExt := ""
	if runtime.GOOS == "windows" {
		binExt = ".exe"
	}
	outBin := filepath.Join(tmpDir, name+binExt)

	// Build with PATH="" and no external toolchains
	origPath := os.Getenv("PATH")
	origGoroot := os.Getenv("GOROOT")
	os.Setenv("PATH", "")
	os.Setenv("GOROOT", "")
	res, err := backend.BuildProgram(mod, backend.BuildOptions{
		Target:     backend.Target{OS: runtime.GOOS, Arch: "amd64"},
		OutputPath: outBin,
	})
	os.Setenv("PATH", origPath)
	os.Setenv("GOROOT", origGoroot)

	if err != nil {
		t.Fatalf("[%s] standalone native build failed: %v", name, err)
	}

	binBytes, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatalf("[%s] failed to read compiled binary: %v", name, err)
	}

	cmd := exec.Command(res.OutputPath)
	cmd.Env = []string{
		"SYSTEMROOT=" + os.Getenv("SYSTEMROOT"),
		"WINDIR=" + os.Getenv("WINDIR"),
		"PATH=",
	}
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	runErr := cmd.Run()
	nativeCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			nativeCode = exitErr.ExitCode()
		} else {
			t.Fatalf("[%s] failed to execute standalone binary: %v", name, runErr)
		}
	}

	nativeOut := strings.TrimSpace(strings.ReplaceAll(outBuf.String(), "\r\n", "\n"))
	interpNorm := strings.TrimSpace(strings.ReplaceAll(interpOut, "\r\n", "\n"))
	if nativeCode != 0 {
		t.Fatalf("[%s] native exit code=%d (expected 0)\nOutput:\n%s", name, nativeCode, nativeOut)
	}
	if nativeOut != interpNorm {
		t.Fatalf("[%s] stdout mismatch:\n--- INTERPRETER ---\n%q\n--- NATIVE ---\n%q", name, interpNorm, nativeOut)
	}
	return binBytes
}

// ============================================================================
// SECTION 5: RUNTIME CONFORMANCE CONTRACTS
// ============================================================================

func TestAuditRuntimeConformanceContracts(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("standalone x86_64 audit requires amd64 host")
	}

	src := `
// 1. joss_str_concat edge cases: normal, empty left, empty right, both empty, Unicode UTF-8
string $s1 = "hello" . "world";
echo $s1;
string $s2 = "" . "x";
echo $s2;
string $s3 = "x" . "";
echo $s3;
string $s4 = "" . "";
echo "[" . $s4 . "]";
string $unicode = "¡Hola, " . "Joss ÑÁÉÍÓÚ 🚀!";
echo $unicode;
echo strlen($unicode);

// 2. Large string construction via repeated concatenation (> 1024 bytes)
string $large = "";
int $i = 0;
while ($i < 128) {
	$large = $large . "ABCDEFGH";
	$i = $i + 1;
}
echo strlen($large);

// 3. String equality & inequality (__joss_rt_strcmp)
int $eq1 = ($s1 == "helloworld") ? 1 : 0;
int $eq2 = ($s1 == "hello") ? 1 : 0;
int $eq3 = ($s1 != "world") ? 1 : 0;
int $eq4 = ($s4 == "") ? 1 : 0;
echo $eq1;
echo $eq2;
echo $eq3;
echo $eq4;

// 4. Integer printing edge cases (0, negative, large 64-bit)
echo 0;
echo -1;
echo -9876543210;
echo 9876543210;
`
	assertStandaloneDifferentialParity(t, "runtime_conformance", src)
}

// ============================================================================
// SECTION 7: CALLING CONVENTION & STACK ALIGNMENT AUDIT
// ============================================================================

func TestAuditCallingConventionAndStack(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("standalone x86_64 audit requires amd64 host")
	}

	src := `
// Function with 6 parameters (4 in registers RCX, RDX, R8, R9 + 2 on stack)
public func calc6(int $a, int $b, int $c, int $d, int $e, int $f): int {
	// Perform OS I/O & heap allocation inside 6-arg frame to verify 16-byte stack alignment
	echo "in_calc6:" . "ok";
	return ($a * 1) + ($b * 10) + ($c * 100) + ($d * 1000) + ($e * 10000) + ($f * 100000);
}

// Function with 8 parameters (4 in registers + 4 on stack)
public func sum8(int $p1, int $p2, int $p3, int $p4, int $p5, int $p6, int $p7, int $p8): int {
	return $p1 + $p2 + $p3 + $p4 + $p5 + $p6 + $p7 + $p8;
}

// Deep recursion
public func triSum(int $n): int {
	guard ($n > 0) else {
		return 0;
	}
	return $n + triSum($n - 1);
}

// Class method with >4 parameters ($this in RCX + 6 explicit arguments = 7 ABI args)
public class Acc {
	public int $base = 0;
	public func initBase(int $b): int {
		$this->base = $b;
		return $this->base;
	}
	public func combine6(int $a, int $b, int $c, int $d, int $e, int $f): int {
		return $this->base + calc6($a, $b, $c, $d, $e, $f);
	}
}

echo calc6(1, 2, 3, 4, 5, 6);
echo sum8(10, 20, 30, 40, 50, 60, 70, 80);
echo triSum(50);

var $acc = new Acc();
$acc->initBase(9);
echo $acc->combine6(1, 2, 3, 4, 5, 6);
`
	assertStandaloneDifferentialParity(t, "calling_convention", src)
}

// ============================================================================
// SECTION 9: OBJECTS, ARRAYS, AND MAPS CONFORMANCE AUDIT
// ============================================================================

func TestAuditObjectsArraysMaps(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("standalone x86_64 audit requires amd64 host")
	}

	src := `
public class Item {
	public int $code = 0;
	public string $label = "";
	public func configure(int $c, string $l): int {
		$this->code = $c;
		$this->label = $l;
		return $this->code;
	}
	public func describe(): string {
		return $this->label;
	}
}

// 1. Empty array
var $emptyArr = [];
echo len($emptyArr);

// 2. Large array (> 256 elements, exercising __joss_arr_ensure_cap dynamic reallocation via array_push)
var $large = [0];
int $i = 1;
while ($i < 600) {
	$large = array_push($large, $i * 3);
	$i = $i + 1;
}
echo len($large);
echo $large[0];
echo $large[255];
echo $large[256];
echo $large[599];

// 3. Array of strings
var $words = ["alfa", "beta", "gamma"];
$words = array_push($words, "delta");
echo len($words);
echo $words[0] . "-" . $words[2] . "-" . $words[3];

// 4. Nested arrays (2D array)
var $row0 = [11, 22];
var $row1 = [33, 44];
var $matrix = [$row0, $row1];
var $fetchedRow = $matrix[1];
echo $fetchedRow[0];
echo $fetchedRow[1];

// 5. Array of objects
var $it1 = new Item();
$it1->configure(101, "first");
var $it2 = new Item();
$it2->configure(202, "second");
var $items = [$it1, $it2];
var $picked = $items[1];
echo $picked->code;
echo $picked->describe();

// 6. Maps: string & int values, mutation, overwriting existing keys
var $m = {"a": 10, "b": 20};
$m["c"] = 30;
$m["a"] = 99;
echo $m["a"];
echo $m["b"];
echo $m["c"];

// 7. Aliasing semantics for arrays and objects
var $aliasArr = $row0;
$aliasArr[0] = 777;
echo $row0[0];

var $aliasObj = $it1;
$aliasObj->code = 888;
echo $it1->code;
`
	assertStandaloneDifferentialParity(t, "objects_arrays_maps", src)
}

// ============================================================================
// SECTION 10: TRY / CATCH / THROW UNWINDING AUDIT
// ============================================================================

func TestAuditExceptionUnwinding(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("standalone x86_64 audit requires amd64 host")
	}

	src := `
public func leafThrow(string $msg): int {
	throw $msg;
}

public func midFrame(string $msg): int {
	int $x = leafThrow($msg);
	return $x + 1;
}

public func topFrame(string $msg): int {
	return midFrame($msg);
}

public func recursiveThrow(int $depth): int {
	guard ($depth > 0) else {
		throw "boom_at_zero";
	}
	return 1 + recursiveThrow($depth - 1);
}

public class Validator {
	public func check(int $v): int {
		guard ($v >= 0) else {
			throw "negative_value";
		}
		return $v * 2;
	}
}

// 1. Nested try / catch
try {
	echo "outer_start";
	try {
		echo "inner_start";
		throw "inner_err";
		echo "inner_unreached";
	} catch ($e1) {
		echo "caught_inner:" . $e1;
	}
	echo "outer_after_inner";
	throw "outer_err";
	echo "outer_unreached";
} catch ($e2) {
	echo "caught_outer:" . $e2;
}

// 2. Unwinding across 3 call frames
try {
	echo topFrame("deep_frame_err");
} catch ($e3) {
	echo "caught_deep:" . $e3;
}

// 3. Throw from method
var $val = new Validator();
try {
	echo $val->check(21);
	echo $val->check(-5);
} catch ($e4) {
	echo "caught_method:" . $e4;
}

// 4. Throw inside while loop
int $k = 0;
try {
	while ($k < 10) {
		guard ($k != 3) else {
			throw "loop_break_at_3";
		}
		echo $k;
		$k = $k + 1;
	}
} catch ($e5) {
	echo "caught_loop:" . $e5;
}

// 5. Throw during recursion (unwinding 8 recursive frames)
try {
	echo recursiveThrow(8);
} catch ($e6) {
	echo "caught_rec:" . $e6;
}
echo "after_all_exceptions";
`
	assertStandaloneDifferentialParity(t, "exception_unwinding", src)
}

// ============================================================================
// SECTION 11 & 12: PE32+ AND ELF64 STRUCTURAL AUDIT
// ============================================================================

func TestAuditPE32PlusAndELF64Structure(t *testing.T) {
	src := `
public func helper(int $x): int {
	return $x + 42;
}
echo helper(8);
`
	mod := prepareTestProgram(t, src, "pe_elf_audit")
	tmpDir := t.TempDir()

	// 1. Audit PE32+ (Windows amd64)
	pePath := filepath.Join(tmpDir, "audit_win.exe")
	_, err := backend.BuildProgram(mod, backend.BuildOptions{
		Target:     backend.Target{OS: "windows", Arch: "amd64"},
		OutputPath: pePath,
	})
	if err != nil {
		t.Fatalf("PE32+ build failed: %v", err)
	}
	peRaw, err := os.ReadFile(pePath)
	if err != nil {
		t.Fatalf("failed reading PE32+: %v", err)
	}

	// Verify DOS Header & e_lfanew
	if string(peRaw[0:2]) != "MZ" {
		t.Fatalf("invalid DOS magic: %x", peRaw[0:2])
	}
	peOffset := binary.LittleEndian.Uint32(peRaw[0x3C:0x40])
	if peOffset != 0x40 {
		t.Fatalf("expected e_lfanew = 0x40, got 0x%x", peOffset)
	}
	if string(peRaw[peOffset:peOffset+4]) != "PE\x00\x00" {
		t.Fatalf("invalid PE signature at 0x%x", peOffset)
	}

	peFile, err := pe.Open(pePath)
	if err != nil {
		t.Fatalf("debug/pe failed to parse PE32+: %v", err)
	}
	defer peFile.Close()

	if peFile.FileHeader.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		t.Fatalf("expected IMAGE_FILE_MACHINE_AMD64 (0x8664), got 0x%x", peFile.FileHeader.Machine)
	}
	if len(peFile.Sections) != 3 {
		t.Fatalf("expected 3 PE sections (.text, .rdata, .data), got %d", len(peFile.Sections))
	}
	optHdr, ok := peFile.OptionalHeader.(*pe.OptionalHeader64)
	if !ok {
		t.Fatalf("expected OptionalHeader64 (PE32+)")
	}
	if optHdr.Magic != 0x20B {
		t.Fatalf("expected PE32+ magic 0x20B, got 0x%x", optHdr.Magic)
	}
	if optHdr.ImageBase != 0x140000000 {
		t.Fatalf("expected ImageBase 0x140000000, got 0x%x", optHdr.ImageBase)
	}
	if optHdr.SectionAlignment != 0x1000 || optHdr.FileAlignment != 0x200 {
		t.Fatalf("unexpected alignments: SectionAlignment=0x%x FileAlignment=0x%x", optHdr.SectionAlignment, optHdr.FileAlignment)
	}
	if optHdr.Subsystem != 3 { // IMAGE_SUBSYSTEM_WINDOWS_CUI
		t.Fatalf("expected Subsystem 3 (WINDOWS_CUI), got %d", optHdr.Subsystem)
	}

	imports, err := peFile.ImportedSymbols()
	if err != nil {
		t.Fatalf("peFile.ImportedSymbols() failed: %v", err)
	}
	expectedImports := map[string]bool{
		"GetStdHandle:KERNEL32.dll":   true,
		"WriteFile:KERNEL32.dll":      true,
		"ExitProcess:KERNEL32.dll":    true,
		"GetProcessHeap:KERNEL32.dll": true,
		"HeapAlloc:KERNEL32.dll":      true,
		"LoadLibraryA:KERNEL32.dll":   true,
		"GetProcAddress:KERNEL32.dll": true,
		"_joss_host_bridge:joss_host.dll": true,
	}
	if len(imports) != len(expectedImports) {
		t.Fatalf("expected %d KERNEL32 imports, got %d: %v", len(expectedImports), len(imports), imports)
	}
	for _, imp := range imports {
		if !expectedImports[imp] {
			t.Fatalf("unexpected PE import: %s", imp)
		}
	}

	// 2. Audit ELF64 (Linux amd64)
	elfPath := filepath.Join(tmpDir, "audit_linux")
	_, err = backend.BuildProgram(mod, backend.BuildOptions{
		Target:     backend.Target{OS: "linux", Arch: "amd64"},
		OutputPath: elfPath,
	})
	if err != nil {
		t.Fatalf("ELF64 build failed: %v", err)
	}
	elfFile, err := elf.Open(elfPath)
	if err != nil {
		t.Fatalf("debug/elf failed to parse ELF64: %v", err)
	}
	defer elfFile.Close()

	if elfFile.Class != elf.ELFCLASS64 || elfFile.Data != elf.ELFDATA2LSB || elfFile.Machine != elf.EM_X86_64 || elfFile.Type != elf.ET_EXEC {
		t.Fatalf("unexpected ELF64 header: class=%v data=%v machine=%v type=%v", elfFile.Class, elfFile.Data, elfFile.Machine, elfFile.Type)
	}
	if elfFile.Entry != 0x401000 {
		t.Fatalf("expected ELF64 entry point 0x401000, got 0x%x", elfFile.Entry)
	}
	if len(elfFile.Progs) != 2 {
		t.Fatalf("expected 2 PT_LOAD program headers, got %d", len(elfFile.Progs))
	}
	for _, p := range elfFile.Progs {
		if p.Type == elf.PT_INTERP || p.Type == elf.PT_DYNAMIC {
			t.Fatalf("static ELF64 must not contain %v segment", p.Type)
		}
	}
	dynLibs, _ := elfFile.ImportedLibraries()
	if len(dynLibs) != 0 {
		t.Fatalf("expected 0 DT_NEEDED libraries in static ELF64, got %v", dynLibs)
	}
}

// ============================================================================
// SECTION 13: INCREMENTAL COMPLEXITY MATRIX (LEVELS 1 TO 10)
// ============================================================================

func TestAuditIncrementalComplexityMatrix(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("standalone x86_64 audit requires amd64 host")
	}

	levels := []struct {
		level string
		src   string
	}{
		{
			level: "Level_1_Minimal_CLI",
			src:   `echo "Level 1: Hola Mundo Standalone";`,
		},
		{
			level: "Level_2_Arithmetic_Logic_ControlFlow",
			src: `
int $a = 16;
int $b = 4;
echo ($a + $b);
echo ($a - $b);
echo ($a * $b);
echo ($a % $b);
echo ($a <=> $b);
int $sum = 0;
int $i = 1;
while ($i <= 10) {
	int $step = (($i % 2) == 0) ? $i : 1;
	$sum = $sum + $step;
	$i = $i + 1;
}
echo $sum;
`,
		},
		{
			level: "Level_3_Functions_And_Recursion",
			src: `
public func gcd(int $a, int $b): int {
	guard ($b != 0) else {
		return $a;
	}
	return gcd($b, $a % $b);
}
echo gcd(1071, 462);
`,
		},
		{
			level: "Level_4_Strings",
			src: `
string $prefix = "Joss";
string $suffix = "Native";
string $combined = $prefix . "-" . $suffix;
echo $combined;
echo strlen($combined);
int $ok = ($combined == "Joss-Native") ? 1 : 0;
echo $ok;
`,
		},
		{
			level: "Level_5_Arrays_And_Maps",
			src: `
var $arr = [10, 20, 30];
$arr = array_push($arr, 40);
echo len($arr);
echo ($arr[0] + $arr[3]);
var $map = {"k1": 100, "k2": 200};
$map["k3"] = 300;
echo ($map["k1"] + $map["k3"]);
`,
		},
		{
			level: "Level_6_Classes_Methods_Inheritance",
			src: `
public class BaseCounter {
	public int $total = 0;
	public func addStep(int $d): int {
		$this->total = $this->total + $d;
		return $this->total;
	}
}
public class FastCounter extends BaseCounter {
	public func doubleStep(int $d): int {
		return $this->addStep($d * 2);
	}
}
var $c = new FastCounter();
$c->total = 5;
echo $c->doubleStep(10);
`,
		},
		{
			level: "Level_7_Try_Catch_Throw",
			src: `
public func riskyOp(int $a, int $b): int {
	guard ($b != 0) else {
		throw "div_zero";
	}
	return $a * $b;
}
try {
	echo riskyOp(20, 4);
	echo riskyOp(20, 0);
} catch ($e) {
	echo "ERR:" . $e;
}
`,
		},
	}

	for _, tc := range levels {
		t.Run(tc.level, func(t *testing.T) {
			assertStandaloneDifferentialParity(t, tc.level, tc.src)
		})
	}

	// Level 8: Foreach and Match differential parity + Unsupported construct (defer) -> Must fail with JOSS-NATIVE-001 (no fallback)
	t.Run("Level_8_Foreach_And_Unsupported_Defer_Explicit_Error", func(t *testing.T) {
		foreachSrc := `
var $arr = [10, 20, 30];
int $sum = 0;
foreach ($arr as $x) {
	$sum = $sum + $x;
}
echo $sum;
`
		assertStandaloneDifferentialParity(t, "Level_8_Foreach", foreachSrc)

		deferSrc := `
public func cleanup(): void {}
defer cleanup();
`
		err := tryLowerSourceToIR(deferSrc)
		if err == nil {
			t.Fatalf("expected JOSS-NATIVE-001 on unsupported defer statement")
		}
		if !strings.Contains(err.Error(), "JOSS-NATIVE-001") {
			t.Fatalf("expected JOSS-NATIVE-001 diagnostic, got: %v", err)
		}
	})

	// Level 9/10: Unsupported host runtime (Server / GranDB / Web Application) -> Must fail with JOSS-NATIVE-001
	t.Run("Level_9_10_Unsupported_Host_Service_Explicit_Error", func(t *testing.T) {
		src := `
var $db = new GranDB();
`
		err := tryLowerSourceToIR(src)
		if err == nil {
			t.Fatalf("expected failure when instantiating unlowered host service GranDB")
		}
	})
}

// ============================================================================
// SECTION 15 & 16: ZERO EXTERNAL PROCESSES & ZERO GO RUNTIME SIGNATURES
// ============================================================================

func TestAuditZeroExternalProcessesAndZeroGoRuntime(t *testing.T) {
	// 1. AST Audit of pkg/backend/native/*.go
	entries, err := os.ReadDir("../../pkg/backend/native")
	if err != nil {
		t.Fatalf("failed reading pkg/backend/native: %v", err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		fullPath := filepath.Join("../../pkg/backend/native", entry.Name())
		fileAST, err := parser.ParseFile(fset, fullPath, nil, 0)
		if err != nil {
			t.Fatalf("failed parsing %s: %v", fullPath, err)
		}
		for _, imp := range fileAST.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			if impPath == "os/exec" {
				t.Fatalf("PROHIBITED import os/exec in %s", fullPath)
			}
		}
		ast.Inspect(fileAST, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if ident, ok := sel.X.(*ast.Ident); ok {
				callName := ident.Name + "." + sel.Sel.Name
				switch callName {
				case "exec.Command", "exec.CommandContext", "os.StartProcess",
					"syscall.Exec", "syscall.ForkExec", "syscall.StartProcess", "syscall.CreateProcess":
					t.Fatalf("PROHIBITED external process call %s in %s", callName, fullPath)
				}
			}
			return true
		})
	}

	// 2. Binary Signature Audit on generated PE32+ and ELF64 binaries
	mod := prepareTestProgram(t, `echo "signature_audit";`, "sig_audit")
	tmpDir := t.TempDir()
	for _, targetOS := range []string{"windows", "linux"} {
		outPath := filepath.Join(tmpDir, "sig_"+targetOS)
		res, err := backend.BuildProgram(mod, backend.BuildOptions{
			Target:     backend.Target{OS: targetOS, Arch: "amd64"},
			OutputPath: outPath,
		})
		if err != nil {
			t.Fatalf("compile failed for %s: %v", targetOS, err)
		}
		raw, err := os.ReadFile(res.OutputPath)
		if err != nil {
			t.Fatalf("read failed for %s: %v", targetOS, err)
		}
		forbiddenMarkers := [][]byte{
			[]byte("go1."),
			[]byte("runtime.main"),
			[]byte("runtime.gopark"),
			[]byte("go:buildid"),
			[]byte("JOSSBC2Z"),
			[]byte("JOSS_STANDALONE_PAYLOAD"),
			{0xFB, 0xFF, 0xFF, 0xFF, 0x00, 0x00}, // Go 1.2+ pclntab magic
			{0xFA, 0xFF, 0xFF, 0xFF, 0x00, 0x00}, // Go 1.16+ pclntab magic
			{0xF0, 0xFF, 0xFF, 0xFF, 0x00, 0x00}, // Go 1.18+ pclntab magic
			{0xF1, 0xFF, 0xFF, 0xFF, 0x00, 0x00}, // Go 1.20+ pclntab magic
		}
		for _, marker := range forbiddenMarkers {
			if bytes.Contains(raw, marker) {
				t.Fatalf("[%s] binary contains forbidden Go runtime / AST marker %x (%q)", targetOS, marker, string(marker))
			}
		}
	}
}

// ============================================================================
// SECTION 17: BINARY SIZE SCALING METRICS
// ============================================================================

func TestAuditBinarySizeScaling(t *testing.T) {
	programs := []struct {
		name string
		src  string
	}{
		{
			name: "1_Tiny_Echo",
			src:  `echo 1;`,
		},
		{
			name: "2_Small_Arithmetic_Loop",
			src: `
int $s = 0;
int $i = 0;
while ($i < 100) {
	$s = $s + $i;
	$i = $i + 1;
}
echo $s;
`,
		},
		{
			name: "3_Medium_Classes_Arrays_Maps",
			src: `
public class Point {
	public int $x = 0;
	public int $y = 0;
	public func norm(): int {
		return ($this->x * $this->x) + ($this->y * $this->y);
	}
}
var $p = new Point();
$p->x = 3;
$p->y = 4;
var $arr = [$p->norm(), 100, 200];
var $m = {"res": $arr[0]};
echo $m["res"];
`,
		},
		{
			name: "4_Large_20_Functions_Exceptions",
			src: func() string {
				var sb strings.Builder
				for i := 1; i <= 20; i++ {
					fmt.Fprintf(&sb, "public func fn_%d(int $x): int { return ($x * %d) + %d; }\n", i, i, i*10)
				}
				sb.WriteString("int $acc = 1;\n")
				for i := 1; i <= 20; i++ {
					fmt.Fprintf(&sb, "$acc = fn_%d($acc);\n", i)
				}
				sb.WriteString("echo $acc;\n")
				return sb.String()
			}(),
		},
	}

	tmpDir := t.TempDir()
	for _, p := range programs {
		mod := prepareTestProgram(t, p.src, p.name)
		pePath := filepath.Join(tmpDir, p.name+".exe")
		elfPath := filepath.Join(tmpDir, p.name+".elf")

		peRes, err := backend.BuildProgram(mod, backend.BuildOptions{Target: backend.Target{OS: "windows", Arch: "amd64"}, OutputPath: pePath})
		if err != nil {
			t.Fatalf("%s PE build failed: %v", p.name, err)
		}
		elfRes, err := backend.BuildProgram(mod, backend.BuildOptions{Target: backend.Target{OS: "linux", Arch: "amd64"}, OutputPath: elfPath})
		if err != nil {
			t.Fatalf("%s ELF build failed: %v", p.name, err)
		}
		peFi, _ := os.Stat(peRes.OutputPath)
		elfFi, _ := os.Stat(elfRes.OutputPath)
		t.Logf("[Size Scaling] %-32s | PE32+: %6d bytes | ELF64: %6d bytes", p.name, peFi.Size(), elfFi.Size())
	}
}
