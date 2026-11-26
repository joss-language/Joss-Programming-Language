package tester

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeTestRunner(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "calculator_test.joss")

	testCode := `public func add(int $a, int $b): int {
    return $a + $b
}

test("suma basica", func() {
    assertEqual(add(2, 3), 5)
})

test("verificar aserciones booleanas", func() {
    assertTrue(true)
    assertFalse(false)
    assertNull(null)
    assertNotNull("hello")
})

test("prueba con pipeline y null safe", func() {
    int $doubled = 10 |> func(int $x): int { return $x * 2 }
    assertEqual($doubled, 20)
})
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	runner := NewRunner()
	report, err := runner.Run(tempDir)
	if err != nil {
		t.Fatalf("runner failed: %v", err)
	}

	if report.TotalPassed != 3 {
		report.PrintSummary()
		t.Fatalf("expected 3 passed tests, got %d", report.TotalPassed)
	}
	if report.TotalFailed != 0 {
		t.Fatalf("expected 0 failed tests, got %d", report.TotalFailed)
	}
}

func TestNativeTestRunnerFailureDetection(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "failing_test.joss")

	testCode := `test("failing assertion", func() {
    assertEqual(1, 2)
})
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	runner := NewRunner()
	report, err := runner.Run(tempDir)
	if err != nil {
		t.Fatalf("runner failed: %v", err)
	}

	if report.TotalFailed != 1 {
		t.Fatalf("expected 1 failed test, got %d", report.TotalFailed)
	}
}
