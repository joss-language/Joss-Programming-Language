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

func TestNative_GranDB_RealSQLiteCRUD(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang not available on PATH")
	}
	if runtime.GOOS == "windows" {
		t.Skip("SQLite3 C development library not preinstalled on Windows CI runner")
	}

	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "app.sqlite")

	source := `
__joss_db_execute("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, email TEXT)", []);
int $ins = GranDB::table("users")->insert({"name": "Alice", "email": "alice@joss.dev"});
print($ins);

var $row = GranDB::table("users")->where("email", "alice@joss.dev")->first();
print($row["name"]);

int $upd = GranDB::table("users")->where("email", "alice@joss.dev")->update({"name": "Alice Updated"});
print($upd);

var $row2 = GranDB::table("users")->where("email", "alice@joss.dev")->first();
print($row2["name"]);

int $del = GranDB::table("users")->where("email", "alice@joss.dev")->delete();
print($del);
`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: "grandb_sqlite.joss", Program: prog}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	irProg, err := ir.NewLowerer(prep, nil).LowerProgram("grandb_sqlite")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	if err := ir.NewVerifier().Verify(irProg); err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	outExe := filepath.Join(tmpDir, "grandb_sqlite_bin")
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

	cmd := exec.Command(outExe)
	cmd.Env = append(os.Environ(), "DB_CONNECTION=sqlite", "DB_DATABASE="+dbFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running grandb_sqlite_bin failed: %v\n%s", err, string(out))
	}
	got := strings.TrimSpace(string(out))
	want := "1\nAlice\n1\nAlice Updated\n1"
	if got != want {
		t.Fatalf("GranDB SQLite CRUD mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
	t.Logf("[GranDB Native SQLite3] Real CRUD verified on disk (%s), binary=%d bytes", dbFile, res.ExecutableSize)
}

func TestNative_OpenSSL_PBKDF2AndSHA256(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang not available on PATH")
	}

	source := `
extern "C" library "" {
	public func joss_password_hash(string $password): string;
	public func joss_password_verify(string $hash, string $password): int;
}

string $digest = sha256("joss");
print($digest);

string $h = joss_password_hash("Sup3rS3cret!");
print(strlen($h));
print(joss_password_verify($h, "Sup3rS3cret!"));
print(joss_password_verify($h, "WrongPass"));
`
	llvmOut, llvmErr, exitCode, runErr := compileAndRunLLVM(t, source, "openssl_crypto", nil)
	if runErr != nil || exitCode != 0 {
		t.Fatalf("LLVM openssl_crypto failed (exit=%d, err=%v, stderr=%s)", exitCode, runErr, llvmErr)
	}
	lines := strings.Split(llvmOut, "\n")
	if len(lines) != 4 || len(lines[0]) != 64 || lines[1] != "119" || lines[2] != "1" || lines[3] != "0" {
		t.Fatalf("unexpected OpenSSL crypto output:\ngot:\n%s", llvmOut)
	}
	t.Logf("[OpenSSL Native Crypto] SHA-256=%s, PBKDF2-HMAC-SHA256(100k) len=%s verify OK", lines[0], lines[1])
}
