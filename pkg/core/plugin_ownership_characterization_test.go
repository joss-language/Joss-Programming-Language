package core

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"
	"unsafe"

	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/pluginpkg"
)

func buildTestASTPluginPackage(t *testing.T, name, source string) []byte {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if errors := p.Errors(); len(errors) > 0 {
		t.Fatalf("parser errors in plugin %s: %v", name, errors)
	}

	encodedAST, err := bytecode.Encode(prog)
	if err != nil {
		t.Fatalf("error encoding AST for plugin %s: %v", name, err)
	}

	metadata := pluginpkg.Metadata{
		Name:     name,
		Version:  "1.0.0",
		Language: "joss",
		Bytecode: "bytecode/main.jbc",
		Symbols:  pluginpkg.SymbolsPath,
	}

	files := map[string][]byte{
		"bytecode/main.jbc":   encodedAST,
		pluginpkg.SymbolsPath: []byte(fmt.Sprintf(`{"schema":1,"package":%q}`, name)),
		"joss.yaml":           []byte(fmt.Sprintf("name: %s\nversion: 1.0.0\n", name)),
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}

	pkgBytes, err := pluginpkg.BuildSigned(metadata, files, priv)
	if err != nil {
		t.Fatalf("failed to build signed plugin package: %v", err)
	}
	return pkgBytes
}

func TestASTPluginExecutionIsolatedPerFork(t *testing.T) {
	pluginSource := `
public func get_tag(): string {
    return env("FORK_TAG");
}
public func add(int $a, int $b): int {
    return $a + $b;
}
`
	pkgBytes := buildTestASTPluginPackage(t, "isolation_plugin", pluginSource)

	parent := NewRuntime()
	defer parent.Free()

	if err := parent.LoadPluginBytes(pkgBytes); err != nil {
		t.Fatalf("LoadPluginBytes failed: %v", err)
	}

	var wg sync.WaitGroup
	results := make([]string, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			fork := parent.Fork()
			tag := fmt.Sprintf("TAG_%d", idx)
			fork.Env["FORK_TAG"] = tag

			res, err := fork.PluginRegistry.CallFunction("isolation_plugin", "get_tag", nil)
			if err != nil {
				t.Errorf("fork %d CallFunction failed: %v", idx, err)
				return
			}
			results[idx] = fmt.Sprint(res)

			addRes, err := fork.PluginRegistry.CallFunction("isolation_plugin", "add", []interface{}{int64(idx), int64(10)})
			if err != nil || addRes != int64(idx+10) {
				t.Errorf("fork %d add failed: got %v, err %v", idx, addRes, err)
			}
			fork.Free()
		}(i)
	}
	wg.Wait()

	for i, got := range results {
		expected := fmt.Sprintf("TAG_%d", i)
		if got != expected {
			t.Errorf("fork %d got tag %q, want %q", i, got, expected)
		}
	}
}

func TestTwoASTPluginsWithIdenticalFunctionNamesDoNotCollide(t *testing.T) {
	pkgA := buildTestASTPluginPackage(t, "service_alpha", `
public func calculate(int $x): int {
    return $x + 10;
}
`)
	pkgB := buildTestASTPluginPackage(t, "service_beta", `
public func calculate(int $x): int {
    return $x + 1000;
}
`)

	r := NewRuntime()
	defer r.Free()

	if err := r.LoadPluginBytes(pkgA); err != nil {
		t.Fatalf("failed loading service_alpha: %v", err)
	}
	if err := r.LoadPluginBytes(pkgB); err != nil {
		t.Fatalf("failed loading service_beta: %v", err)
	}

	resA, err := r.PluginRegistry.CallFunction("service_alpha", "calculate", []interface{}{int64(5)})
	if err != nil {
		t.Fatalf("service_alpha calculate failed: %v", err)
	}
	if resA != int64(15) {
		t.Fatalf("service_alpha calculate = %v, want 15", resA)
	}

	resB, err := r.PluginRegistry.CallFunction("service_beta", "calculate", []interface{}{int64(5)})
	if err != nil {
		t.Fatalf("service_beta calculate failed: %v", err)
	}
	if resB != int64(1005) {
		t.Fatalf("service_beta calculate = %v, want 1005", resB)
	}

	// Also verify in a Fork
	fork := r.Fork()
	defer fork.Free()

	forkA, err := fork.PluginRegistry.CallFunction("service_alpha", "calculate", []interface{}{int64(20)})
	if err != nil || forkA != int64(30) {
		t.Fatalf("fork service_alpha failed: got %v, err %v", forkA, err)
	}

	forkB, err := fork.PluginRegistry.CallFunction("service_beta", "calculate", []interface{}{int64(20)})
	if err != nil || forkB != int64(1020) {
		t.Fatalf("fork service_beta failed: got %v, err %v", forkB, err)
	}
}

func TestNativeDriverConcurrentCallsAndSafeUnload(t *testing.T) {
	mockResponse := []byte("{\"status\":\"ok\"}\x00")
	driver := &NativeDriverDefinition{
		Name:   "mock_driver",
		Path:   "/virtual/mock.dll",
		Handle: 0,
		Call: func(method, argsJSON string) *byte {
			return &mockResponse[0]
		},
		Free: func(p *byte) {},
	}

	parent := NewRuntime()
	defer parent.Free()
	parent.NativeDrivers["mock_driver"] = driver

	forkA := parent.Fork()
	defer forkA.Free()
	forkB := parent.Fork()
	defer forkB.Free()

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			res, err := callLoadedNativeDriver(parent.NativeDrivers["mock_driver"], "test", "{}")
			if err != nil || res != "{\"status\":\"ok\"}" {
				t.Errorf("parent call failed: res=%q err=%v", res, err)
			}
		}()
		go func() {
			defer wg.Done()
			res, err := callLoadedNativeDriver(forkA.NativeDrivers["mock_driver"], "test", "{}")
			if err != nil || res != "{\"status\":\"ok\"}" {
				t.Errorf("forkA call failed: res=%q err=%v", res, err)
			}
		}()
		go func() {
			defer wg.Done()
			res, err := callLoadedNativeDriver(forkB.NativeDrivers["mock_driver"], "test", "{}")
			if err != nil || res != "{\"status\":\"ok\"}" {
				t.Errorf("forkB call failed: res=%q err=%v", res, err)
			}
		}()
	}
	wg.Wait()

	// Safe Unload: unloading clears Handle, Call, Free under lock
	if err := driver.Unload(); err != nil {
		t.Fatalf("Unload failed: %v", err)
	}

	// Subsequent calls must return an error without panicking
	_, err := callLoadedNativeDriver(driver, "test", "{}")
	if err == nil {
		t.Fatal("expected error after driver unload, got nil")
	}
	_ = unsafe.Pointer(nil)
}
