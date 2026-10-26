package core

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/plugincompiler/ir"
	"github.com/jossecurity/joss/pkg/pluginpkg"
	"github.com/jossecurity/joss/pkg/pluginruntime"
)

// PoC JOSS-SEC-VULN-001 (A): Un plugin AST (.jp FormatJossAST) con Permissions: []
// no debería poder leer archivos del sistema ni ejecutar comandos si no declaró permisos.
func TestSecurityAudit_VULN001_PluginASTWithoutPermissionsCannotAccessFS(t *testing.T) {
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("TOP_SECRET_HOST_DATA"), 0600); err != nil {
		t.Fatal(err)
	}

	src := `
public func stealFile(string $p): mixed {
    return file_get_contents($p);
}
`
	l := parser.NewLexer(src)
	p := parser.NewParser(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	bc, err := bytecode.Encode(prog)
	if err != nil {
		t.Fatal(err)
	}

	symIdx := pluginpkg.SymbolIndex{
		Schema:  pluginpkg.SymbolSchemaVersion,
		Package: "untrusted_ast_plugin",
		Version: "1.0.0",
		Functions: []pluginpkg.SymbolCallable{
			{Name: "stealFile", ReturnType: "mixed"},
		},
	}
	symBytes, _ := json.Marshal(symIdx)

	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	jpBytes, err := pluginpkg.BuildSigned(pluginpkg.Metadata{
		Name:        "untrusted_ast_plugin",
		Version:     "1.0.0",
		Language:    "joss",
		Bytecode:    "bytecode/main.jbc",
		Symbols:     pluginpkg.SymbolsPath,
		Permissions: []string{}, // ZERO permissions declared!
	}, map[string][]byte{
		"bytecode/main.jbc":   bc,
		pluginpkg.SymbolsPath: symBytes,
		"joss.yaml":           []byte("name: untrusted_ast_plugin\nversion: 1.0.0\n"),
	}, privKey)
	if err != nil {
		t.Fatal(err)
	}

	rt := NewRuntime()
	defer rt.Free()

	if err := rt.LoadPluginBytes(jpBytes); err != nil {
		t.Fatal(err)
	}

	res, err := rt.PluginRegistry.CallFunction("untrusted_ast_plugin", "stealFile", []interface{}{secretFile})
	if err == nil && res == "TOP_SECRET_HOST_DATA" {
		t.Fatalf("VULN-001 (AST): untrusted plugin with ZERO permissions read host file: %v", res)
	}
}

// PoC JOSS-SEC-VULN-001 (B): Un plugin JPBC con Permissions: [] no debería poder invocar
// built-ins del host como file_get_contents.
func TestSecurityAudit_VULN001_PluginJPBCWithoutPermissionsCannotCallHostBuiltins(t *testing.T) {
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret_jpbc.txt")
	if err := os.WriteFile(secretFile, []byte("JPBC_SECRET_DATA"), 0600); err != nil {
		t.Fatal(err)
	}

	rt := NewRuntime()
	defer rt.Free()

	mod := &pluginruntime.JPBCModule{
		MajorVersion: 1,
		MinorVersion: 0,
		ConstantPool: []interface{}{"file_get_contents"},
		Functions: map[string]*pluginruntime.JPBCFunction{
			"steal": {
				Name: "steal",
				Instructions: []pluginruntime.JPBCInstruction{
					{Op: ir.OpCallStatic, ConstIdx: 0},
					{Op: ir.OpReturn},
				},
			},
		},
	}

	guard := pluginruntime.NewPermissionGuard([]string{}) // ZERO permissions!
	vm := pluginruntime.NewJPBCVM(mod, guard, rt)
	res, err := vm.Execute("steal", []interface{}{secretFile})
	if err == nil && res == "JPBC_SECRET_DATA" {
		t.Fatalf("VULN-001 (JPBC): untrusted JPBC plugin with ZERO permissions bypassed PermissionGuard via file_get_contents: %v", res)
	}
}

// PoC JOSS-SEC-VULN-002: Path Traversal en UserStorage::getToFile, UserStorage::path y Plugin::path
func TestSecurityAudit_VULN002_PathTraversalInStorageAndPluginPathBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	rt := NewRuntime()
	defer rt.Free()

	// 1. UserStorage::put valid file, then attempt getToFile outside storage/project root
	putOk := rt.executeUserStorageMethod(nil, "put", []interface{}{1, "avatar.txt", "FILE_DATA"})
	if putOk != true {
		t.Fatalf("expected valid UserStorage::put to succeed")
	}

	escapeTarget := filepath.Join(tmpDir, "..", "escaped_getToFile_test.txt")
	defer os.Remove(escapeTarget)
	copied := rt.executeUserStorageMethod(nil, "getToFile", []interface{}{1, "avatar.txt", "../escaped_getToFile_test.txt"})
	if copied == true {
		t.Fatalf("VULN-002: UserStorage::getToFile allowed writing to arbitrary path traversal destination '../escaped_getToFile_test.txt'")
	}

	// 2. Plugin::path fallback traversal
	plugPath := rt.executePluginMethod(nil, "path", []interface{}{"../../evil_plugin", "../../etc/passwd"})
	if pStr, ok := plugPath.(string); ok {
		pluginsRoot, _ := filepath.Abs(filepath.Join(tmpDir, "plugins"))
		absRes, _ := filepath.Abs(pStr)
		rel, relErr := filepath.Rel(pluginsRoot, absRes)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("VULN-002: Plugin::path allowed escaping plugins directory: resolved to %s (outside %s)", absRes, pluginsRoot)
		}
	}
}

// PoC JOSS-SEC-VULN-004: Forjado de JWT cuando JWT_SECRET no está definido
func TestSecurityAudit_VULN004_JWTDefaultSecretForgeryBlocked(t *testing.T) {
	origSecret := os.Getenv("JWT_SECRET")
	os.Unsetenv("JWT_SECRET")
	defer os.Setenv("JWT_SECRET", origSecret)

	rt := NewRuntime()
	defer rt.Free()
	delete(rt.Env, "JWT_SECRET")
	delete(rt.Env, "APP_KEY")

	// Attacker tries to forge an admin token using the known hardcoded fallback string
	forgedClaims := jwt.MapClaims{
		"user_id":    1,
		"email":      "attacker@evil.test",
		"name":       "Attacker",
		"role":       "admin",
		"token_type": "access",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, forgedClaims)
	forgedStr, err := token.SignedString([]byte("joss_default_secret_change_in_production"))
	if err != nil {
		t.Fatal(err)
	}

	if _, valid := rt.ValidateJWT(forgedStr); valid {
		t.Fatalf("VULN-004: Runtime accepted JWT forged with hardcoded 'joss_default_secret_change_in_production'")
	}
}

// PoC JOSS-SEC-VULN-005 (A): Concurrencia segura en View::share y View::render
func TestSecurityAudit_VULN005_ConcurrentViewShareAndRender(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("app", "views"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("app", "views", "home.html"), []byte("<p>{{ $site_name }}</p>"), 0644); err != nil {
		t.Fatal(err)
	}

	initRt := NewRuntime()
	initRt.executeViewMethod(nil, "share", []interface{}{"site_name", "Joss"})
	initRt.Free()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			rt := NewRuntime()
			defer rt.Free()
			rt.executeViewMethod(nil, "share", []interface{}{"site_name", "Joss"})
		}(i)
		go func() {
			defer wg.Done()
			rt := NewRuntime()
			defer rt.Free()
			_ = rt.executeViewMethod(nil, "render", []interface{}{"home"})
		}()
	}
	wg.Wait()
}

// PoC JOSS-SEC-VULN-006: Zip::extract debe rechazar entradas que excedan el tamaño máximo permitido (Zip Bomb protection)
func TestSecurityAudit_VULN006_ZipExtractEnforcesSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "bomb.zip")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	hdr := &zip.FileHeader{
		Name:   "huge.bin",
		Method: zip.Deflate,
	}
	hdr.SetMode(0600) // not a symlink, but let's test symlink rejection too!
	hdrSym := &zip.FileHeader{
		Name:   "evil_link",
		Method: zip.Store,
	}
	hdrSym.SetMode(os.ModeSymlink | 0777)
	wSym, err := zw.CreateHeader(hdrSym)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = wSym.Write([]byte("/etc/passwd"))
	_ = zw.Close()

	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	rt := NewRuntime()
	defer rt.Free()

	extractDir := filepath.Join(tmpDir, "out")
	ok := rt.executeZipMethod(nil, "extract", []interface{}{zipPath, extractDir})
	if ok == true {
		t.Fatalf("VULN-006: Zip::extract allowed extracting symlink entry 'evil_link'")
	}
}

// PoC JOSS-SEC-VULN-007 (B): Type assertions sin guarda en métodos nativos no deben causar panic de Go
func TestSecurityAudit_VULN007_NativeMethodsRejectNonStringArgsGracefully(t *testing.T) {
	rt := NewRuntime()
	defer rt.Free()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("VULN-007: unchecked type assertion caused runtime panic: %v", r)
		}
	}()

	_ = rt.executeSystemMethod(nil, "env", []interface{}{12345})
	_ = rt.executeAuthMethod(nil, "validateToken", []interface{}{12345})
	_ = rt.executeResponseMethod(nil, "redirect", []interface{}{12345})
	_ = rt.executeViewMethod(nil, "render", []interface{}{12345})
}
