package architecture_test

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	core "github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

// TestArchitecture_GoResourceAndSafetyGate parses every non-test Go file under
// pkg/ and cmd/ with Go's AST parser and blocks CI if it detects:
//   - HTTP calls (http.Get/Post/Head) without resp.Body.Close() in the same function
//   - time.NewTicker(...) without .Stop() in the same function
//   - context.WithCancel / WithTimeout / WithDeadline without invoking or storing cancel
//   - Unfinished panic("TODO") / panic("not implemented") stubs in production code
func TestArchitecture_GoResourceAndSafetyGate(t *testing.T) {
	roots := []string{
		filepath.Join("..", "..", "pkg"),
		filepath.Join("..", "..", "cmd"),
	}
	fset := token.NewFileSet()
	var violations []string
	scannedFiles := 0

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if d.Name() == "vendor" || d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			scannedFiles++
			fileAST, parseErr := goparser.ParseFile(fset, path, nil, goparser.ParseComments)
			if parseErr != nil {
				violations = append(violations, fmt.Sprintf("BLOQUEADO [JOSS-GO-GATE] %s: error de sintaxis Go: %v", path, parseErr))
				return nil
			}

			relPath := filepath.ToSlash(path)
			for _, decl := range fileAST.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				auditGoFunc(fset, relPath, fn.Name.Name, fn.Body, &violations)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s failed: %v", root, err)
		}
	}

	if scannedFiles < 50 {
		t.Fatalf("expected at least 50 Go files in pkg/ and cmd/, scanned %d", scannedFiles)
	}
	if len(violations) > 0 {
		t.Fatalf("Auditoría estática Go BLOQUEADA (%d violaciones):\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

func auditGoFunc(fset *token.FileSet, file, fnName string, body *ast.BlockStmt, violations *[]string) {
	var (
		hasHTTPGet     bool
		httpGetLine    int
		hasBodyClose   bool
		hasNewTicker   bool
		tickerLine     int
		hasTickerStop  bool
		hasFileOpen    bool
		fileOpenLine   int
		hasClose       bool
		returnsFile    bool
		hasZipOrGzip   bool
		zipOrGzipLine  int
		zipOrGzipKind  string
		hasSQLQuery    bool
		sqlQueryLine   int
		returnsRows    bool
		cancelVars     = map[string]int{}
		usedCancelVars = map[string]bool{}
	)

	// Check for `defer` inside loops (defers accumulate until function exit -> FD/RAM leak)
	// and `regexp.Compile` / `regexp.MustCompile` inside loops.
	var walkLoop func(node ast.Node, inLoop bool)
	walkLoop = func(node ast.Node, inLoop bool) {
		if node == nil {
			return
		}
		switch n := node.(type) {
		case *ast.ForStmt:
			ast.Inspect(n.Body, func(c ast.Node) bool {
				if _, ok := c.(*ast.FuncLit); ok {
					return false
				}
				if d, ok := c.(*ast.DeferStmt); ok {
					pos := fset.Position(d.Pos())
					*violations = append(*violations, fmt.Sprintf(
						"BLOQUEADO [JOSS-GO-GATE] %s:%d (%s): `defer` dentro de un bucle `for` (acumula recursos/memoria hasta salir de la función).",
						file, pos.Line, fnName))
				}
				if call, ok := c.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						if pkgID, ok := sel.X.(*ast.Ident); ok && pkgID.Name == "regexp" &&
							(sel.Sel.Name == "MustCompile" || sel.Sel.Name == "Compile") {
							pos := fset.Position(call.Pos())
							*violations = append(*violations, fmt.Sprintf(
								"BLOQUEADO [JOSS-GO-GATE] %s:%d (%s): regexp.%s() dentro de un bucle `for` (compílalo fuera del bucle).",
								file, pos.Line, fnName, sel.Sel.Name))
						}
					}
				}
				return true
			})
			return
		case *ast.RangeStmt:
			ast.Inspect(n.Body, func(c ast.Node) bool {
				if _, ok := c.(*ast.FuncLit); ok {
					return false
				}
				if d, ok := c.(*ast.DeferStmt); ok {
					pos := fset.Position(d.Pos())
					*violations = append(*violations, fmt.Sprintf(
						"BLOQUEADO [JOSS-GO-GATE] %s:%d (%s): `defer` dentro de un bucle `for range` (acumula recursos/memoria hasta salir de la función).",
						file, pos.Line, fnName))
				}
				if call, ok := c.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						if pkgID, ok := sel.X.(*ast.Ident); ok && pkgID.Name == "regexp" &&
							(sel.Sel.Name == "MustCompile" || sel.Sel.Name == "Compile") {
							pos := fset.Position(call.Pos())
							*violations = append(*violations, fmt.Sprintf(
								"BLOQUEADO [JOSS-GO-GATE] %s:%d (%s): regexp.%s() dentro de un bucle `for range` (compílalo fuera del bucle).",
								file, pos.Line, fnName, sel.Sel.Name))
						}
					}
				}
				return true
			})
			return
		}
		ast.Inspect(node, func(c ast.Node) bool {
			if c == node {
				return true
			}
			switch c.(type) {
			case *ast.ForStmt, *ast.RangeStmt:
				walkLoop(c, true)
				return false
			}
			return true
		})
	}
	walkLoop(body, false)

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Track `ctx, cancel := context.WithTimeout/WithCancel/WithDeadline(...)`
			if len(node.Rhs) == 1 && len(node.Lhs) == 2 {
				if call, ok := node.Rhs[0].(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						if pkgID, ok := sel.X.(*ast.Ident); ok && pkgID.Name == "context" {
							switch sel.Sel.Name {
							case "WithCancel", "WithTimeout", "WithDeadline":
								if cid, ok := node.Lhs[1].(*ast.Ident); ok {
									if cid.Name == "_" {
										pos := fset.Position(call.Pos())
										*violations = append(*violations, fmt.Sprintf(
											"BLOQUEADO [JOSS-GO-GATE] %s:%d (%s): context.%s() descarta cancel con `_` (fuga de timer/contexto).",
											file, pos.Line, fnName, sel.Sel.Name))
									} else {
										cancelVars[cid.Name] = fset.Position(call.Pos()).Line
									}
								}
							}
						}
					}
				}
			}
			// If cancel var is assigned to a struct field or returned, mark used
			for _, rhs := range node.Rhs {
				if id, ok := rhs.(*ast.Ident); ok {
					usedCancelVars[id.Name] = true
				}
			}
		case *ast.KeyValueExpr:
			if id, ok := node.Value.(*ast.Ident); ok {
				usedCancelVars[id.Name] = true
			}
		case *ast.ReturnStmt:
			for _, res := range node.Results {
				if id, ok := res.(*ast.Ident); ok {
					usedCancelVars[id.Name] = true
				}
			}
		case *ast.CallExpr:
			// Direct call to cancel() or passing cancel as arg
			if id, ok := node.Fun.(*ast.Ident); ok {
				usedCancelVars[id.Name] = true
				if id.Name == "panic" && len(node.Args) == 1 {
					if lit, ok := node.Args[0].(*ast.BasicLit); ok {
						low := strings.ToLower(lit.Value)
						if strings.Contains(low, "todo:") || strings.Contains(low, "\"todo\"") || strings.Contains(low, "not implemented") {
							pos := fset.Position(node.Pos())
							*violations = append(*violations, fmt.Sprintf(
								"BLOQUEADO [JOSS-GO-GATE] %s:%d (%s): stub sin implementar panic(%s) en código de producción.",
								file, pos.Line, fnName, lit.Value))
						}
					}
				}
			}
			for _, arg := range node.Args {
				if id, ok := arg.(*ast.Ident); ok {
					usedCancelVars[id.Name] = true
				}
			}
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if pkgID, ok := sel.X.(*ast.Ident); ok {
					if pkgID.Name == "http" && (sel.Sel.Name == "Get" || sel.Sel.Name == "Post" || sel.Sel.Name == "Head") {
						hasHTTPGet = true
						httpGetLine = fset.Position(node.Pos()).Line
					}
					if pkgID.Name == "time" && sel.Sel.Name == "NewTicker" {
						hasNewTicker = true
						tickerLine = fset.Position(node.Pos()).Line
					}
					if pkgID.Name == "os" && (sel.Sel.Name == "Open" || sel.Sel.Name == "Create" || sel.Sel.Name == "OpenFile" || sel.Sel.Name == "CreateTemp") {
						hasFileOpen = true
						fileOpenLine = fset.Position(node.Pos()).Line
					}
					if pkgID.Name == "zip" && sel.Sel.Name == "OpenReader" {
						hasZipOrGzip = true
						zipOrGzipLine = fset.Position(node.Pos()).Line
						zipOrGzipKind = "zip.OpenReader"
					}
					if pkgID.Name == "gzip" && (sel.Sel.Name == "NewReader" || sel.Sel.Name == "NewWriter") {
						hasZipOrGzip = true
						zipOrGzipLine = fset.Position(node.Pos()).Line
						zipOrGzipKind = "gzip." + sel.Sel.Name
					}
				}
				if sel.Sel.Name == "QueryContext" {
					hasSQLQuery = true
					sqlQueryLine = fset.Position(node.Pos()).Line
				}
				if sel.Sel.Name == "Close" {
					hasClose = true
					if inner, ok := sel.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "Body" {
						hasBodyClose = true
					}
				}
				if sel.Sel.Name == "Stop" {
					hasTickerStop = true
				}
			}
		}
		return true
	})

	// Allow helper functions that intentionally return an opened *os.File or *sql.Rows
	ast.Inspect(body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok && len(ret.Results) > 0 {
			for _, r := range ret.Results {
				if id, ok := r.(*ast.Ident); ok {
					if id.Name == "f" || id.Name == "file" || id.Name == "out" {
						returnsFile = true
					}
					if id.Name == "rows" {
						returnsRows = true
					}
				}
				if call, ok := r.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "QueryContext" {
						returnsRows = true
					}
				}
			}
		}
		return true
	})

	if hasHTTPGet && !hasBodyClose {
		*violations = append(*violations, fmt.Sprintf(
			"BLOQUEADO [JOSS-GO-GATE] %s:%d (%s): llamada http.Get/Post/Head sin `defer resp.Body.Close()` (fuga de sockets/memoria).",
			file, httpGetLine, fnName))
	}
	if hasNewTicker && !hasTickerStop {
		*violations = append(*violations, fmt.Sprintf(
			"BLOQUEADO [JOSS-GO-GATE] %s:%d (%s): `time.NewTicker` sin `ticker.Stop()` en la misma función (fuga de goroutine/timer).",
			file, tickerLine, fnName))
	}
	if hasFileOpen && !hasClose && !returnsFile && fnName != "InitLogger" {
		*violations = append(*violations, fmt.Sprintf(
			"BLOQUEADO [JOSS-GO-GATE] %s:%d (%s): `os.Open/Create/OpenFile/CreateTemp` sin `.Close()` en la misma función (fuga de descriptor de archivo).",
			file, fileOpenLine, fnName))
	}
	if hasZipOrGzip && !hasClose {
		*violations = append(*violations, fmt.Sprintf(
			"BLOQUEADO [JOSS-GO-GATE] %s:%d (%s): `%s` sin `.Close()` en la misma función (fuga de buffers/descriptores).",
			file, zipOrGzipLine, fnName, zipOrGzipKind))
	}
	if hasSQLQuery && !hasClose && !returnsRows {
		*violations = append(*violations, fmt.Sprintf(
			"BLOQUEADO [JOSS-GO-GATE] %s:%d (%s): `QueryContext` sin `rows.Close()` en la misma función (fuga de conexiones SQL).",
			file, sqlQueryLine, fnName))
	}
	for cname, line := range cancelVars {
		if !usedCancelVars[cname] {
			*violations = append(*violations, fmt.Sprintf(
				"BLOQUEADO [JOSS-GO-GATE] %s:%d (%s): variable cancel `%s` nunca se invoca ni almacena (fuga de contexto).",
				file, line, fnName, cname))
		}
	}
}

// TestArchitecture_StdlibJossIntegrityGate parses and semantically analyzes
// EVERY .joss file in std/ to guarantee zero syntax errors, zero type errors,
// zero broken symbols, and required HTTP security invariants.
func TestArchitecture_StdlibJossIntegrityGate(t *testing.T) {
	stdRoot := filepath.Join("..", "..", "std")
	var jossFiles []string
	err := filepath.WalkDir(stdRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".joss") {
			jossFiles = append(jossFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk std/ failed: %v", err)
	}
	if len(jossFiles) < 5 {
		t.Fatalf("expected at least 5 stdlib .joss files, found %d", len(jossFiles))
	}

	// 1. Every individual stdlib .joss file must parse cleanly with 0 errors
	for _, f := range jossFiles {
		src, readErr := os.ReadFile(f)
		if readErr != nil {
			t.Fatalf("read %s: %v", f, readErr)
		}
		p := parser.NewParser(parser.NewLexer(string(src)))
		_ = p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatalf("BLOQUEADO [JOSS-STDLIB-GATE] %s tiene errores de sintaxis: %v", f, p.Errors())
		}
	}

	// 2. Full semantic analysis of all stdlib units together must have 0 errors
	units := std.GetStdlibUnits()
	if len(units) == 0 {
		t.Fatalf("BLOQUEADO [JOSS-STDLIB-GATE] std.GetStdlibUnits() devolvió 0 unidades")
	}
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("BLOQUEADO [JOSS-STDLIB-GATE] errores semánticos en la biblioteca estándar std/: %v", prep.Diagnostics)
	}

	// 3. Security invariants in std/http/server.joss
	srvBytes, err := os.ReadFile(filepath.Join(stdRoot, "http", "server.joss"))
	if err != nil {
		t.Fatalf("read std/http/server.joss: %v", err)
	}
	srv := string(srvBytes)
	requiredInvariants := []struct {
		needle string
		desc   string
	}{
		{"X-Content-Type-Options: nosniff", "header anti-MIME-sniffing"},
		{"X-Frame-Options: DENY", "header anti-clickjacking"},
		{"Referrer-Policy: strict-origin-when-cross-origin", "header Referrer-Policy"},
		{"TRUST_PROXY", "control TRUST_PROXY para X-Forwarded-For"},
		{"__joss_rate_limit_check", "protección Rate Limiter DDoS"},
		{`str_contains($lower, "..")`, "guarda anti-Path Traversal"},
	}
	for _, inv := range requiredInvariants {
		if !strings.Contains(srv, inv.needle) {
			t.Fatalf("BLOQUEADO [JOSS-STDLIB-GATE] std/http/server.joss perdió invariante de seguridad: %s (%q)", inv.desc, inv.needle)
		}
	}
}

// TestArchitecture_LLVMExternalSymbolsExistInC11Runtime verifies that critical
// C11 runtime symbols referenced by the LLVM IR emitter actually have real
// implementations in runtime/native/*.c or *.inc (prevents broken/missing link symbols).
func TestArchitecture_LLVMExternalSymbolsExistInC11Runtime(t *testing.T) {
	rtDir := filepath.Join("..", "..", "runtime", "native")
	entries, err := os.ReadDir(rtDir)
	if err != nil {
		t.Fatalf("read runtime/native: %v", err)
	}
	var allC strings.Builder
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), "_test.c") {
			continue
		}
		if strings.HasSuffix(e.Name(), ".c") || strings.HasSuffix(e.Name(), ".inc") {
			b, rErr := os.ReadFile(filepath.Join(rtDir, e.Name()))
			if rErr != nil {
				t.Fatalf("read %s: %v", e.Name(), rErr)
			}
			allC.Write(b)
			allC.WriteByte('\n')
		}
	}
	cCorpus := allC.String()

	requiredSymbols := []string{
		"fn_Memory_rss", "fn_Memory_arenaUsed", "fn_Memory_gcStats", "fn_Memory_gcCollect",
		"fn_Memory_trim", "fn_Memory_sizeOf", "fn_Memory_snapshot", "fn_Memory_diff",
		"fn_Memory_report", "fn_Memory_checkGrowth",
		"fn_Cache_put", "fn_Cache_get", "fn_Cache_has", "fn_Cache_forget",
		"joss_req_arena_begin", "joss_req_arena_reset", "joss_req_alloc", "joss_req_alloc_zeroed",
		"joss_req_strdup", "joss_sb_to_req_string",
		"joss_owned_scope_begin", "joss_owned_scope_end", "joss_owned_scope_destroy",
		"joss_async", "joss_async_detached", "joss_await",
		"joss_gc_init", "joss_gc_collect", "joss_gc_add_root",
	}
	for _, sym := range requiredSymbols {
		if !strings.Contains(cCorpus, sym+"(") {
			t.Fatalf("BLOQUEADO [JOSS-ABI-GATE] símbolo C11 requerido `%s` no existe en runtime/native/*.c|*.inc", sym)
		}
	}
}
