package native_test

import (
	"bytes"
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

func runGoInterpreter(t *testing.T, source string) string {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("go parse errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: "main.joss", Program: astProg}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("go analyze errors: %v", prep.Diagnostics)
	}
	rt := core.NewRuntime()
	defer rt.Free()
	var buf bytes.Buffer
	rt.Out = &buf
	rt.ExecutePrepared(prep)
	return strings.TrimSpace(buf.String())
}

func runNativeLLVM(t *testing.T, name, source string, env []string) string {
	t.Helper()
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang no disponible")
	}
	p := parser.NewParser(parser.NewLexer(source))
	astProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("native parse errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: name + ".joss", Program: astProg}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("native analyze errors: %v", prep.Diagnostics)
	}
	irProg, err := ir.NewLowerer(prep, nil).LowerProgram(name)
	if err != nil {
		t.Fatalf("lower failed: %v", err)
	}
	if err := ir.NewVerifier().Verify(irProg); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	outExe := filepath.Join(t.TempDir(), name)
	if runtime.GOOS == "windows" {
		outExe += ".exe"
	}
	if _, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    backend.BackendLLVM,
		OutputPath: outExe,
	}); err != nil {
		t.Fatalf("BuildProgram failed: %v", err)
	}
	cmd := exec.Command(outExe)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native exec failed: %v\n%s", err, string(out))
	}
	return strings.TrimSpace(strings.ReplaceAll(string(out), "\r\n", "\n"))
}

func TestParity_GoVsNative_JWTAndRFC6238TOTP(t *testing.T) {
	source := `
string $jwt = jwt_sign("{\"sub\":42,\"role\":\"admin\"}", "super-secret-key-2026");
print($jwt);
print(jwt_verify($jwt, "super-secret-key-2026"));
print(jwt_verify($jwt, "wrong-key"));
print(jwt_verify($jwt . "x", "super-secret-key-2026"));

string $code = totp_generate("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", 59);
print($code);
print(totp_verify("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "287082", 59));
print(totp_verify("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "000000", 59));

string $h = Auth::hash("JossSecret#99");
print(Auth::verifyHash($h, "JossSecret#99"));
print(Auth::verifyHash($h, "WrongPassword"));

string $tok = Auth::issueToken(7, "dev@joss.dev", "dev", "admin");
print(Auth::verify($tok));
print(Auth::verify($tok . "tampered"));
`
	goOut := runGoInterpreter(t, source)
	nativeOut := runNativeLLVM(t, "auth_totp_parity", source, []string{"JWT_SECRET=parity-test-secret"})

	if goOut != nativeOut {
		// Auth::issueToken embeds time() which may cross a 1-second boundary between runs;
		// check deterministic lines (JWT, TOTP RFC 6238, PBKDF2 verify, Auth::verify).
		goLines := strings.Split(goOut, "\n")
		natLines := strings.Split(nativeOut, "\n")
		if len(goLines) != len(natLines) {
			t.Fatalf("line count mismatch:\nGo:\n%s\nNative:\n%s", goOut, nativeOut)
		}
		for i := range goLines {
			if goLines[i] != natLines[i] {
				t.Fatalf("line %d mismatch:\nGo:     %q\nNative: %q", i, goLines[i], natLines[i])
			}
		}
	}
	if !strings.Contains(nativeOut, "287082") {
		t.Fatalf("expected RFC 6238 test vector 287082, got:\n%s", nativeOut)
	}
	t.Logf("Go & Native 100%% byte-for-byte identical on HS256 JWT, RFC 6238 TOTP (287082), PBKDF2-HMAC-SHA256 Auth:\n%s", nativeOut)
}

func TestNative_GranDB_ExtendedQueryBuilderAndRollback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real SQLite3 C driver and POSIX setjmp test runs on Linux/macOS")
	}
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "grandb_ext.sqlite")

	source := `
__joss_db_execute("CREATE TABLE products (id INTEGER PRIMARY KEY AUTOINCREMENT, category TEXT, name TEXT, price INTEGER)", []);
GranDB::table("products")->insert({"category": "books", "name": "Compiler", "price": 50});
GranDB::table("products")->insert({"category": "books", "name": "Systems", "price": 70});
GranDB::table("products")->insert({"category": "tools", "name": "Linker", "price": 30});

print(GranDB::table("products")->where("name", "Compiler")->value("price"));
print(count(GranDB::table("products")->whereNot("category", "tools")->get()));
print(GranDB::table("products")->where("name", "Linker")->sole()["name"]);
print(count(GranDB::table("products")->findMany([1, 3])));

GranDB::table("products")->updateOrInsert({"name": "Linker"}, {"price": 35});
print(GranDB::table("products")->where("name", "Linker")->value("price"));
GranDB::table("products")->updateOrInsert({"name": "Assembler"}, {"category": "tools", "price": 45});
print(GranDB::table("products")->where("name", "Assembler")->value("price"));

var $pg = GranDB::table("products")->orderBy("id", "asc")->simplePaginate(2, 1);
print(count($pg["data"]));
($pg["has_more"]) ? { print("has_more_yes"); } : { print("has_more_no"); }

var $groups = GranDB::table("products")->select("category")->groupBy("category")->having("category", "!=", "none")->get();
print(count($groups));

try {
	GranDB::transaction(func(mixed $db): mixed {
		GranDB::table("products")->insert({"category": "temp", "name": "ShouldRollback", "price": 1});
		throw "abort_tx";
	});
} catch ($e) {
	print("rolled_back");
}
print(GranDB::table("products")->where("name", "ShouldRollback")->exists());

GranDB::table("products")->deleteAll();
print(GranDB::table("products")->count());
`
	t.Setenv("DB_CONNECTION", "sqlite")
	t.Setenv("DB_DATABASE", dbFile)
	nativeOut := runNativeLLVM(t, "grandb_extended", source, []string{"DB_CONNECTION=sqlite", "DB_DATABASE=" + dbFile})
	expected := strings.Join([]string{
		"50",
		"2",
		"Linker",
		"2",
		"35",
		"45",
		"2",
		"has_more_yes",
		"2",
		"rolled_back",
		"false",
		"0",
	}, "\n")
	if nativeOut != expected {
		t.Fatalf("GranDB extended mismatch:\ngot:\n%s\nwant:\n%s", nativeOut, expected)
	}
	t.Logf("GranDB native extended Query Builder + ROLLBACK verified:\n%s", nativeOut)
}

func TestNative_EjemploTestNativoSuite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real SQLite3 C driver and POSIX setjmp suite runs on Linux/macOS")
	}
	suitePath := filepath.Join("..", "..", "ejemplos", "test-nativo-suite", "main.joss")
	data, err := os.ReadFile(suitePath)
	if err != nil {
		t.Fatalf("read %s: %v", suitePath, err)
	}
	dbFile := filepath.Join(t.TempDir(), "suite.sqlite")
	t.Setenv("DB_CONNECTION", "sqlite")
	t.Setenv("DB_DATABASE", dbFile)

	out := runNativeLLVM(t, "test_nativo_suite", string(data), []string{"DB_CONNECTION=sqlite", "DB_DATABASE=" + dbFile})
	for _, want := range []string{"9227465", "500500", "2001000", "287082", "10", "rollback_ok", "=== SUITE COMPLETADA ==="} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, out)
		}
	}
	t.Logf("ejemplos/test-nativo-suite/main.joss native execution PASSED:\n%s", out)
}
