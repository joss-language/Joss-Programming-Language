package mobile

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCaptureBufferSnapshotIgnoresLateConcurrentWrites(t *testing.T) {
	buffer := &captureBuffer{}
	if _, err := buffer.Write([]byte("before")); err != nil {
		t.Fatal(err)
	}
	var writers sync.WaitGroup
	for i := 0; i < 20; i++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			_, _ = buffer.Write([]byte("later"))
		}()
	}
	snapshot := buffer.closeAndSnapshot()
	writers.Wait()
	if !strings.HasPrefix(snapshot, "before") || buffer.closeAndSnapshot() != snapshot {
		t.Fatalf("capture changed after snapshot: %q", snapshot)
	}
}

func TestRunTimeoutInterruptsSocketAccept(t *testing.T) {
	started := time.Now()
	result := RunDirect(`
$server = new Socket()
$server->listen("127.0.0.1", "0")
$server->accept()
`, 30)
	if !result.TimedOut || result.Success {
		t.Fatalf("socket accept did not time out: %+v", result)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("socket accept outlived timeout: %v", elapsed)
	}
}

func TestVersion(t *testing.T) {
	v := Version()
	if v == "" {
		t.Fatal("Version() returned empty string")
	}
}

func TestRunSimpleScript(t *testing.T) {
	code := `print("Hola desde Joss Mobile");`
	jsonOutput := Run(code, 3000)

	var res ExecutionResult
	if err := json.Unmarshal([]byte(jsonOutput), &res); err != nil {
		t.Fatalf("Failed to parse JSON result: %v", err)
	}

	if !res.Success {
		t.Fatalf("Expected success=true, got false: %s", res.Error)
	}

	if !strings.Contains(res.Stdout, "Hola desde Joss Mobile") {
		t.Fatalf("Expected stdout to contain 'Hola desde Joss Mobile', got: %q", res.Stdout)
	}
}

func TestRunCoutStream(t *testing.T) {
	code := `cout << "Prueba Cout" << endl;`
	res := RunDirect(code, 3000)

	if !res.Success {
		t.Fatalf("Expected success, got error: %s", res.Error)
	}

	if !strings.Contains(res.Stdout, "Prueba Cout") {
		t.Fatalf("Expected stdout to contain 'Prueba Cout', got: %q", res.Stdout)
	}
}

func TestRunSyntaxError(t *testing.T) {
	code := `public func ( {`
	res := RunDirect(code, 3000)

	if res.Success {
		t.Fatal("Expected syntax error, but run reported success")
	}

	if len(res.Diagnostics) == 0 {
		t.Fatal("Expected diagnostics for syntax error")
	}
}

func TestRunSemanticError(t *testing.T) {
	code := `
public func test(): void {
	int $a = "no es un entero";
}
test();
`
	res := RunDirect(code, 3000)

	if res.Success {
		t.Fatal("Expected semantic type error, but run reported success")
	}

	if len(res.Diagnostics) == 0 {
		t.Fatal("Expected diagnostics for semantic error")
	}
}

func TestAnalyze(t *testing.T) {
	validCode := `
public func suma(int $a, int $b): int {
	return $a + $b;
}
`
	analysisJSON := Analyze(validCode)
	var res AnalysisResult
	if err := json.Unmarshal([]byte(analysisJSON), &res); err != nil {
		t.Fatalf("Failed to parse analysis JSON: %v", err)
	}

	if !res.Valid {
		t.Fatalf("Expected code to be valid, got diagnostics: %+v", res.Diagnostics)
	}

	invalidCode := `
public func roto(): int {
	return "invalido";
}
`
	resInvalid := AnalyzeDirect(invalidCode)
	if resInvalid.Valid {
		t.Fatal("Expected invalid code to fail analysis")
	}
	if len(resInvalid.Diagnostics) == 0 {
		t.Fatal("Expected diagnostics for invalid return type")
	}
}

func TestRunTimeout(t *testing.T) {
	infiniteLoopCode := `
while (true) {
	// loop
}
`
	// 150ms timeout
	res := RunDirect(infiniteLoopCode, 150)

	if res.Success {
		t.Fatal("Expected timeout failure, got success")
	}

	if !res.TimedOut {
		t.Fatalf("Expected TimedOut=true, got: %+v", res)
	}
}
