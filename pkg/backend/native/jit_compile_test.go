package native_test

import (
	"runtime"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/jit"
	"github.com/jossecurity/joss/pkg/parser"
)

func TestMachineCodeBuilder_CompileInMemory_Simple(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("JIT test is x86_64 amd64 only")
	}

	source := `
		public func main(): int {
			int $x = 15;
			int $y = 27;
			return $x + $y;
		}
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: "jit_test.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	if prep.HasErrors() {
		t.Fatalf("semantic errors: %v", prep.Diagnostics)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("jit_test")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	builder := backend.NewMachineCodeBuilder(backend.HostTarget(), false)
	jitExe, err := builder.CompileInMemory(irProg)
	if err != nil {
		t.Fatalf("CompileInMemory failed: %v", err)
	}
	defer jitExe.Free()

	exitCode, err := jitExe.Execute()
	if err != nil && err.Error() != "The operation completed successfully." {
		t.Logf("Execute notice: %v", err)
	}

	if exitCode != 42 {
		t.Fatalf("expected return value 42 (15 + 27), got %d", exitCode)
	}
}

func TestMachineCodeBuilder_CompileInMemory_LoopAndWrite(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("JIT test is x86_64 amd64 only")
	}

	source := `
		public func main(): int {
			int $sum = 0;
			int $i = 1;
			while ($i <= 10) {
				$sum = $sum + $i;
				$i = $i + 1;
			}
			return $sum;
		}
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: "jit_loop.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	if prep.HasErrors() {
		t.Fatalf("semantic errors: %v", prep.Diagnostics)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("jit_loop")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	builder := backend.NewMachineCodeBuilder(backend.HostTarget(), false)
	jitExe, err := builder.CompileInMemory(irProg)
	if err != nil {
		t.Fatalf("CompileInMemory failed: %v", err)
	}
	defer jitExe.Free()

	exitCode, err := jitExe.Execute()
	if err != nil && err.Error() != "The operation completed successfully." {
		t.Logf("Execute notice: %v", err)
	}

	// 1 + 2 + ... + 10 = 55
	if exitCode != 55 {
		t.Fatalf("expected return value 55, got %d", exitCode)
	}
}

func TestMachineCodeBuilder_CompileInMemory_HostBridgeService(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("In-memory HostBridge callback via IAT slot is only supported on Windows amd64")
	}

	var hostCalled bool
	jit.RegisterHostBridgeHandler("TestService::ping", func(args ...uintptr) uintptr {
		hostCalled = true
		return 123
	})

	source := `
		public func main(): int {
			return 1;
		}
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: "jit_host.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	if prep.HasErrors() {
		t.Fatalf("semantic errors: %v", prep.Diagnostics)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("jit_host")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	// Inject a CallHostService instruction to verify JIT C-ABI invocation
	entryBlk := irProg.Functions["main"].Blocks[0]
	entryBlk.Instructions = append([]ir.Instruction{
		&ir.CallHostServiceInst{
			ClassName:  "TestService",
			MethodName: "ping",
		},
	}, entryBlk.Instructions...)

	builder := backend.NewMachineCodeBuilder(backend.HostTarget(), false)
	jitExe, err := builder.CompileInMemory(irProg)
	if err != nil {
		t.Fatalf("CompileInMemory failed: %v", err)
	}
	defer jitExe.Free()

	exitCode, _ := jitExe.Execute()
	if exitCode != 1 {
		t.Fatalf("expected exitCode 1, got %d", exitCode)
	}
	if !hostCalled {
		t.Fatal("expected host bridge handler to be called via JIT dispatch table")
	}
}
