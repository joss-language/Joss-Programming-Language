package analyzer

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestCapabilityMatrixLookup(t *testing.T) {
	functionsCap, found := LookupCapability("Functions")
	if !found {
		t.Fatal("expected 'Functions' capability to be found")
	}
	if functionsCap.Language != Supported || functionsCap.Native != Supported {
		t.Fatalf("expected Functions to be supported in Language and Native, got %+v", functionsCap)
	}

	mapsCap, found := LookupCapability("DynamicMaps")
	if !found {
		t.Fatal("expected 'DynamicMaps' capability to be found")
	}
	if mapsCap.Language != Supported || mapsCap.Native != Supported {
		t.Fatalf("expected DynamicMaps to be supported in language and native, got %+v", mapsCap)
	}

	exceptionsCap, found := LookupCapability("Exceptions")
	if !found {
		t.Fatal("expected 'Exceptions' capability to be found")
	}
	if exceptionsCap.Language != Supported || exceptionsCap.Native != Supported {
		t.Fatalf("expected Exceptions to be supported in language and native, got %+v", exceptionsCap)
	}

	channelsCap, found := LookupCapability("Channels")
	if !found {
		t.Fatal("expected 'Channels' capability to be found")
	}
	if channelsCap.Language != Supported || channelsCap.Native != Unsupported {
		t.Fatalf("expected Channels to be supported in language but unsupported in native, got %+v", channelsCap)
	}

	errStr := FormatCapabilityError("Channels")
	if errStr == "" {
		t.Fatal("expected formatted capability error to be non-empty")
	}

	// Matrix 2.0 Ownership & State Checks
	ownership, ok := GetFeatureOwnership("Functions")
	if !ok || ownership.SemanticOwner != "pkg/analyzer" {
		t.Fatalf("expected semantic owner 'pkg/analyzer' for Functions, got %+v", ownership)
	}
	if functionsCap.State != StateFullyComplete {
		t.Fatalf("expected Functions to be StateFullyComplete, got %s", functionsCap.State)
	}
	if mapsCap.State != StateFullyComplete {
		t.Fatalf("expected DynamicMaps to be StateFullyComplete, got %s", mapsCap.State)
	}
	if exceptionsCap.State != StateFullyComplete {
		t.Fatalf("expected Exceptions to be StateFullyComplete, got %s", exceptionsCap.State)
	}
	if channelsCap.State != StateInterpreterComplete {
		t.Fatalf("expected Channels to be StateInterpreterComplete, got %s", channelsCap.State)
	}
	if !IsNativeSupported("SpaceshipOperator") {
		t.Fatal("expected SpaceshipOperator to be native supported")
	}
	if !IsNativeSupported("DynamicClasses") {
		t.Fatal("expected DynamicClasses to be native supported")
	}
	if !IsNativeSupported("Exceptions") {
		t.Fatal("expected Exceptions to be native supported")
	}
	if IsNativeSupported("Channels") {
		t.Fatal("expected Channels to be NOT native supported")
	}
}

func TestPreparedProgramSemanticModelQueries(t *testing.T) {
	source := `
public class MathService {
    public static func add(int $a, int $b): int {
        return $a + $b;
    }
}
public func compute(int $x): int {
    return MathService::add($x, 42);
}
`
	l := parser.NewLexer(source)
	p := parser.NewParser(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	prep := PrepareProgram([]SourceUnit{{Path: "math.joss", Program: prog}}, NewEnvironment())
	if prep.HasErrors() {
		t.Fatalf("unexpected diagnostics: %v", prep.Diagnostics)
	}

	// Test semantic model methods
	classes := prep.Classes()
	if _, ok := classes["MathService"]; !ok {
		t.Fatal("expected class 'MathService' in semantic model classes")
	}

	functions := prep.Functions()
	if _, ok := functions["compute"]; !ok {
		t.Fatal("expected function 'compute' in semantic model functions")
	}

	modules := prep.Modules()
	if len(modules) != 1 || modules[0].Path != "math.joss" {
		t.Fatalf("expected 1 module with path 'math.joss', got %v", modules)
	}

	if sym, ok := prep.Symbol("compute"); !ok || sym.Kind != "function" {
		t.Fatalf("expected symbol fact for compute function, got %+v", sym)
	}
}
