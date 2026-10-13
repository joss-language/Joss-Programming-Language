package native_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
)

// TestNativeGranDBInsertionAndQuery compiles and executes a standalone native binary
// that exercises GranDB::table, insert, insertGetId, where, count, first, get (with foreach),
// update, and delete, together with newly lowered AST constructs (foreach, match, ??, ++, isset, empty, '.' concat).
func TestNativeGranDBInsertionAndQuery(t *testing.T) {
	t.Skip("GranDB native execution requires static Host Bridge linking (libjoss_base.a)")
	src := `
		// 1. Insert rows into 'users' table using GranDB::table()->insert() and insertGetId()
		int $id1 = GranDB::table("users")->insert({
			"username": "joss_admin",
			"role": "admin",
			"score": 95
		});
		int $id2 = GranDB::table("users")->insertGetId({
			"username": "ada_sec",
			"role": "admin",
			"score": 88
		});
		int $id3 = GranDB::table("users")->insert({
			"username": "temp_guest",
			"role": "guest",
			"score": 10
		});

		echo "IDs:" . $id1 . "-" . $id2 . "-" . $id3;

		// 2. Count total and filtered rows
		int $total = GranDB::table("users")->count();
		int $adminCount = GranDB::table("users")->where("role", "admin")->count();
		echo "Counts:" . $total . "/" . $adminCount;

		// 3. Query single row with where()->first()
		var $firstAdmin = GranDB::table("users")->where("username", "joss_admin")->first();
		echo "First:" . $firstAdmin["username"] . ":" . $firstAdmin["role"] . ":" . $firstAdmin["score"];

		// 4. Update a row and verify updated field
		GranDB::table("users")->where("username", "ada_sec")->update({
			"score": 999
		});
		var $ada = GranDB::table("users")->where("username", "ada_sec")->first();
		echo "UpdatedAda:" . $ada["score"];

		// 5. Retrieve multiple rows with where()->get() and iterate with foreach + postfix ++ + match
		var $admins = GranDB::table("users")->where("role", "admin")->get();
		int $sumScores = 0;
		int $iterCount = 0;
		foreach ($admins as $idx => $row) {
			$iterCount++;
			int $sc = $row["score"];
			$sumScores = $sumScores + $sc;
		}
		string $tier = match ($iterCount) {
			1 => "single",
			2 => "pair",
			default => "many"
		};
		echo "Admins:" . $iterCount . ":" . $tier . ":" . $sumScores;

		// 6. Delete guest row and verify count after deletion
		int $deleted = GranDB::table("users")->where("username", "temp_guest")->delete();
		int $remaining = GranDB::table("users")->count();
		echo "AfterDelete:" . $deleted . ":" . $remaining;
	`

	irProg := prepareTestProgram(t, src, "test_grandb_insert_query")
	tempDir := t.TempDir()
	outExe := filepath.Join(tempDir, "grandb_insert_query.exe")

	_, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Backend:    backend.BackendStandalone,
		OutputPath: outExe,
	})
	if err != nil {
		t.Fatalf("BuildProgram failed for GranDB insertion test: %v", err)
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command(outExe)
		cmd.Dir = tempDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("GranDB native executable failed: %v\nOutput:\n%s", err, string(out))
		}
		got := strings.ReplaceAll(strings.TrimSpace(string(out)), "\r\n", "\n")
		want := strings.Join([]string{
			"IDs:1-2-3",
			"Counts:3/2",
			"First:joss_admin:admin:95",
			"UpdatedAda:999",
			"Admins:2:pair:1094",
			"AfterDelete:1:2",
		}, "\n")
		if got != want {
			t.Fatalf("GranDB native output mismatch:\ngot:\n%s\nwant:\n%s", got, want)
		}
	}
}

// TestNativeJosSecurityFullProjectCompilationAndExecution compiles the entire
// ejemplos/Joss-Red-JosSecurity application into standalone Windows PE32+ and Linux ELF64
// native binaries, verifies zero Go runtime / zero AST interpreter embedding, and executes it.
func TestNativeJosSecurityFullProjectCompilationAndExecution(t *testing.T) {
	t.Skip("Full project native compilation requires static Host Bridge linking (libjoss_base.a)")
	josSecurityDir, err := filepath.Abs(filepath.Join("..", "..", "ejemplos", "Joss-Red-JosSecurity"))
	if err != nil {
		t.Fatalf("filepath.Abs failed: %v", err)
	}
	mainFile := filepath.Join(josSecurityDir, "main.joss")
	if _, err := os.Stat(mainFile); err != nil {
		t.Skipf("Joss-Red-JosSecurity not found at %s: %v", mainFile, err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	if err := os.Chdir(josSecurityDir); err != nil {
		t.Fatalf("os.Chdir(%s) failed: %v", josSecurityDir, err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	units, loadDiags := analyzer.LoadProject("main.joss", "app")
	if len(loadDiags) > 0 {
		t.Fatalf("LoadProject diagnostics on Joss-Red-JosSecurity: %v", loadDiags)
	}

	report := core.AnalyzeSourceUnits(units)
	if report.Prepared == nil {
		t.Fatalf("AnalyzeSourceUnits returned nil PreparedProgram")
	}
	if report.Prepared.HasErrors() {
		t.Fatalf("Joss-Red-JosSecurity semantic errors: %v", report.Prepared.Diagnostics)
	}

	lowerer := ir.NewLowerer(report.Prepared, nil)
	irProg, err := lowerer.LowerProgram("JosSecurity")
	if err != nil {
		t.Fatalf("Lowering Joss-Red-JosSecurity failed: %v", err)
	}

	verifier := ir.NewVerifier()
	if err := verifier.Verify(irProg); err != nil {
		t.Fatalf("IR Verification of Joss-Red-JosSecurity failed: %v", err)
	}

	// Verify that controllers, models, and main were lowered into IR functions
	if len(irProg.Functions) < 50 {
		t.Fatalf("expected at least 50 lowered IR functions in Joss-Red-JosSecurity, got %d", len(irProg.Functions))
	}

	tempDir := t.TempDir()

	// 1. Build Windows PE32+ standalone executable
	winExe := filepath.Join(tempDir, "JosSecurity.exe")
	_, err = backend.BuildProgram(irProg, backend.BuildOptions{
		Backend:    backend.BackendStandalone,
		Target:     backend.Target{OS: "windows", Arch: "amd64"},
		OutputPath: winExe,
	})
	if err != nil {
		t.Fatalf("Windows PE32+ compilation of Joss-Red-JosSecurity failed: %v", err)
	}

	winBytes, err := os.ReadFile(winExe)
	if err != nil {
		t.Fatalf("failed to read JosSecurity.exe: %v", err)
	}
	if len(winBytes) < 4096 || winBytes[0] != 'M' || winBytes[1] != 'Z' {
		t.Fatalf("invalid PE32+ header in JosSecurity.exe (size=%d)", len(winBytes))
	}
	for _, forbidden := range []string{"JOSS_RUNNER_DATA", "go.buildid", "runtime.main", "github.com/jossecurity/joss/pkg/core"} {
		if bytes.Contains(winBytes, []byte(forbidden)) {
			t.Fatalf("JosSecurity.exe contains forbidden non-standalone marker %q", forbidden)
		}
	}

	// 2. Build Linux ELF64 standalone executable
	linuxElf := filepath.Join(tempDir, "JosSecurity-linux")
	_, err = backend.BuildProgram(irProg, backend.BuildOptions{
		Backend:    backend.BackendStandalone,
		Target:     backend.Target{OS: "linux", Arch: "amd64"},
		OutputPath: linuxElf,
	})
	if err != nil {
		t.Fatalf("Linux ELF64 compilation of Joss-Red-JosSecurity failed: %v", err)
	}

	elfBytes, err := os.ReadFile(linuxElf)
	if err != nil {
		t.Fatalf("failed to read JosSecurity-linux: %v", err)
	}
	if len(elfBytes) < 4096 || !bytes.Equal(elfBytes[:4], []byte{0x7F, 'E', 'L', 'F'}) {
		t.Fatalf("invalid ELF64 header in JosSecurity-linux (size=%d)", len(elfBytes))
	}

	// 3. Execute JosSecurity.exe on Windows host:
	//    (a) Verify check-only startup output
	//    (b) Launch live native HTTP server on port 8000 and verify dynamic MVC routes & static assets
	if runtime.GOOS == "windows" {
		checkCmd := exec.Command(winExe)
		checkCmd.Dir = josSecurityDir
		checkCmd.Env = append(os.Environ(), "JOSS_NATIVE_CHECK_ONLY=1")
		out, err := checkCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("JosSecurity.exe check-only execution failed: %v\nOutput:\n%s", err, string(out))
		}
		got := strings.ReplaceAll(strings.TrimSpace(string(out)), "\r\n", "\n")
		if !strings.Contains(got, "Iniciando Sistema JosSecurity...") {
			t.Fatalf("expected 'Iniciando Sistema JosSecurity...' in output, got:\n%s", got)
		}
		if !strings.Contains(got, "[Joss Native Server] Escuchando en http://localhost:8000") {
			t.Fatalf("expected '[Joss Native Server] Escuchando en http://localhost:8000' in output, got:\n%s", got)
		}

		// Launch live HTTP server without JOSS_NATIVE_CHECK_ONLY
		srvCmd := exec.Command(winExe)
		srvCmd.Dir = josSecurityDir
		if err := srvCmd.Start(); err != nil {
			t.Fatalf("failed to start live JosSecurity.exe HTTP server: %v", err)
		}
		defer func() {
			if srvCmd.Process != nil {
				_ = srvCmd.Process.Kill()
				_, _ = srvCmd.Process.Wait()
			}
		}()

		client := &http.Client{Timeout: 1500 * time.Millisecond}
		var resp *http.Response
		var reqErr error
		for attempt := 0; attempt < 25; attempt++ {
			time.Sleep(40 * time.Millisecond)
			resp, reqErr = client.Get("http://127.0.0.1:8000/")
			if reqErr == nil {
				break
			}
		}
		if reqErr != nil {
			t.Fatalf("live HTTP GET to http://127.0.0.1:8000/ failed: %v", reqErr)
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("failed reading HTTP response body: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected HTTP 200 OK from native JosSecurity server, got %d", resp.StatusCode)
		}
		homeHTML := string(bodyBytes)
		for _, expectedFragment := range []string{
			"Astriom | Productos, software e infraestructura",
			"Software, productos e infraestructura para sistemas que deben perdurar.",
			"Joss Language",
			"/public/css/landing.css",
		} {
			if !strings.Contains(homeHTML, expectedFragment) {
				t.Fatalf("expected %q in GET / response body, got (len=%d):\n%.600s", expectedFragment, len(homeHTML), homeHTML)
			}
		}

		// Verify /creador route dynamically dispatches CreatorController::index and renders creator.joss.html
		creatorResp, err := client.Get("http://127.0.0.1:8000/creador")
		if err != nil {
			t.Fatalf("live HTTP GET to /creador failed: %v", err)
		}
		creatorBytes, _ := io.ReadAll(creatorResp.Body)
		creatorResp.Body.Close()
		if !strings.Contains(string(creatorBytes), "José Luis Melchor Estrada") {
			t.Fatalf("expected creator view content on GET /creador, got (len=%d):\n%.500s", len(creatorBytes), string(creatorBytes))
		}

		// Verify /docs route dynamically dispatches DocsController::index and reads Markdown from disk
		docsResp, err := client.Get("http://127.0.0.1:8000/docs")
		if err != nil {
			t.Fatalf("live HTTP GET to /docs failed: %v", err)
		}
		docsBytes, _ := io.ReadAll(docsResp.Body)
		docsResp.Body.Close()
		docsHTML := string(docsBytes)
		if !strings.Contains(docsHTML, "Documentación Joss | Guía completa") || !strings.Contains(docsHTML, "Documentación de Joss") {
			t.Fatalf("expected Markdown documentation content on GET /docs, got (len=%d):\n%.500s", len(docsHTML), docsHTML)
		}

		// Verify /login route dynamically dispatches AuthController::showLogin and renders auth/login.joss.html
		loginResp, err := client.Get("http://127.0.0.1:8000/login")
		if err != nil {
			t.Fatalf("live HTTP GET to /login failed: %v", err)
		}
		loginBytes, _ := io.ReadAll(loginResp.Body)
		loginResp.Body.Close()
		if !strings.Contains(string(loginBytes), "Iniciar Sesión") {
			t.Fatalf("expected login view content on GET /login, got (len=%d):\n%.500s", len(loginBytes), string(loginBytes))
		}

		// Verify static CSS file serving from disk
		cssResp, err := client.Get("http://127.0.0.1:8000/public/css/landing.css")
		if err != nil {
			t.Fatalf("live HTTP GET to /public/css/landing.css failed: %v", err)
		}
		cssBytes, _ := io.ReadAll(cssResp.Body)
		cssResp.Body.Close()
		if cssResp.StatusCode != http.StatusOK || len(cssBytes) < 100 {
			t.Fatalf("expected valid CSS response on GET /public/css/landing.css, status=%d len=%d", cssResp.StatusCode, len(cssBytes))
		}
	}
}
