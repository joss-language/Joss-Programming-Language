package native_test

import (
	"os"
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

func TestIR_DeadFunctionElimination_PrunesUnreachableFunctions(t *testing.T) {
	src := `
public func unused_heavy_helper(int $a, int $b): int {
    return $a * $b + 42
}

public func live_helper(int $x): int {
    return $x + 1
}

print(live_helper(41))
`
	p := parser.NewParser(parser.NewLexer(src))
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: "main.joss", Program: astProg}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyze errors: %v", prep.Diagnostics)
	}
	irProg, err := ir.NewLowerer(prep, nil).LowerProgram("dfe_test")
	if err != nil {
		t.Fatalf("IR lowering failed: %v", err)
	}

	beforeCount := len(irProg.Functions)
	ir.OptimizeProgram(irProg)
	afterCount := len(irProg.Functions)

	if _, exists := irProg.Functions["fn_unused_heavy_helper"]; exists {
		t.Fatalf("expected fn_unused_heavy_helper to be pruned by DFE, still present in IR")
	}
	if _, exists := irProg.Functions["unused_heavy_helper"]; exists {
		t.Fatalf("expected unused_heavy_helper to be pruned by DFE, still present in IR")
	}
	if _, exists := irProg.Functions["main"]; !exists {
		t.Fatalf("expected main to remain after DFE")
	}
	// Con toda la stdlib cargada (>100 funciones), DFE debe retener solo main, live_helper y __joss_init_static_globals
	if afterCount > 3 {
		t.Fatalf("expected DFE to prune dead stdlib/user functions down to <=3, got %d", afterCount)
	}
	t.Logf("IR DFE pruned functions (with full stdlib loaded): %d -> %d", beforeCount, afterCount)
}

func TestNative_AggressivePruning_CLIAndSQLiteDependencies(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang no disponible")
	}

	target := backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
	tmpDir := t.TempDir()

	// 1. Programa CLI puro (sin web, sin DB, sin crypto, sin gorutinas)
	cliSrc := `
public func fib(int $n): int {
	($n <= 1) ? {
		return $n;
	}
	return fib($n - 1) + fib($n - 2);
}

public func unused_dead_fn(int $x): int {
	return $x * 999;
}

print(fib(28));
`
	p := parser.NewParser(parser.NewLexer(cliSrc))
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	prep := analyzer.PrepareProgram([]analyzer.SourceUnit{{Path: "main.joss", Program: astProg}}, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyze errors: %v", prep.Diagnostics)
	}
	irProg, err := ir.NewLowerer(prep, nil).LowerProgram("cli_pruned")
	if err != nil {
		t.Fatalf("lower error: %v", err)
	}

	cliBin := filepath.Join(tmpDir, "cli_pruned")
	if runtime.GOOS == "windows" {
		cliBin += ".exe"
	}
	if _, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     target,
		Backend:    backend.BackendLLVM,
		OutputPath: cliBin,
	}); err != nil {
		t.Fatalf("compile CLI failed: %v", err)
	}

	cliOut, err := exec.Command(cliBin).CombinedOutput()
	if err != nil {
		t.Fatalf("run CLI failed: %v\n%s", err, string(cliOut))
	}
	if strings.TrimSpace(string(cliOut)) != "317811" {
		t.Fatalf("expected 317811, got %q", strings.TrimSpace(string(cliOut)))
	}
	cliInfo, _ := os.Stat(cliBin)

	if runtime.GOOS == "linux" {
		if lddOut, lddErr := exec.Command("ldd", cliBin).CombinedOutput(); lddErr == nil {
			lddStr := string(lddOut)
			for _, forbidden := range []string{"libsqlite3", "libpq", "libmariadb", "libmysqlclient", "libodbc", "libcrypto"} {
				if strings.Contains(lddStr, forbidden) {
					t.Errorf("CLI binary unexpectedly linked %s:\n%s", forbidden, lddStr)
				}
			}
			t.Logf("CLI pruned binary: %d bytes\nldd:\n%s", cliInfo.Size(), strings.TrimSpace(lddStr))
		}
	} else {
		t.Logf("CLI pruned binary: %d bytes", cliInfo.Size())
	}

	// 2. Programa GranDB SQLite selectivo: solo debe enlazar libsqlite3, nunca libpq/libmariadb/libodbc/libcrypto
	t.Setenv("DB_CONNECTION", "sqlite")
	dbPath := filepath.Join(tmpDir, "pruned.sqlite")
	t.Setenv("DB_DATABASE", dbPath)

	dbSrc := `
__joss_db_execute("CREATE TABLE items (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)", []);
int $ins = GranDB::table("items")->insert({"name": "pruned-ok"});
print($ins);
var $row = GranDB::table("items")->where("name", "pruned-ok")->first();
print($row["name"]);
`
	p2 := parser.NewParser(parser.NewLexer(dbSrc))
	astProg2 := p2.ParseProgram()
	units2 := append([]analyzer.SourceUnit{{Path: "db_pruned.joss", Program: astProg2}}, std.GetStdlibUnits()...)
	prep2 := analyzer.PrepareProgram(units2, core.BuildAnalysisEnvironment())
	if prep2.HasErrors() {
		t.Fatalf("analyze DB errors: %v", prep2.Diagnostics)
	}
	irProg2, err := ir.NewLowerer(prep2, nil).LowerProgram("db_sqlite_pruned")
	if err != nil {
		t.Fatalf("lower DB error: %v", err)
	}
	beforeDB := len(irProg2.Functions)
	ir.OptimizeProgram(irProg2)
	afterDB := len(irProg2.Functions)

	dbBin := filepath.Join(tmpDir, "db_sqlite_pruned")
	if runtime.GOOS == "windows" {
		dbBin += ".exe"
	}
	if _, err := backend.BuildProgram(irProg2, backend.BuildOptions{
		Target:     target,
		Backend:    backend.BackendLLVM,
		OutputPath: dbBin,
	}); err != nil {
		t.Fatalf("compile DB failed: %v", err)
	}

	dbOut, err := exec.Command(dbBin).CombinedOutput()
	if err != nil {
		t.Fatalf("run DB failed: %v\n%s", err, string(dbOut))
	}
	if !strings.Contains(string(dbOut), "pruned-ok") {
		t.Fatalf("expected pruned-ok, got %q", string(dbOut))
	}
	dbInfo, _ := os.Stat(dbBin)

	if runtime.GOOS == "linux" {
		if lddOut, lddErr := exec.Command("ldd", dbBin).CombinedOutput(); lddErr == nil {
			lddStr := string(lddOut)
			for _, forbidden := range []string{"libpq", "libmariadb", "libmysqlclient", "libodbc", "libcrypto"} {
				if strings.Contains(lddStr, forbidden) {
					t.Errorf("SQLite-only binary unexpectedly linked %s:\n%s", forbidden, lddStr)
				}
			}
			t.Logf("SQLite-only pruned binary: %d bytes (IR funcs: %d -> %d)\nldd:\n%s", dbInfo.Size(), beforeDB, afterDB, strings.TrimSpace(lddStr))
		}
	} else {
		t.Logf("SQLite-only pruned binary: %d bytes (IR funcs: %d -> %d)", dbInfo.Size(), beforeDB, afterDB)
	}
}
