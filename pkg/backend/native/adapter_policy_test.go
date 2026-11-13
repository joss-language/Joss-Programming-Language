package native

import (
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/jit"
)

func TestFrameworkAdapterPolicyOnlyAllowsFrameworkMethodFamilies(t *testing.T) {
	for _, label := range []string{
		"fn_Router_get",
		"fn_Schema_foreign",
		"fn_Plugin_register",
		"fn_GranDB_table",
		"fn_HttpServer_start",
	} {
		if !isFrameworkAdapterTarget(label) {
			t.Errorf("expected framework adapter target %q to be allowed", label)
		}
	}
	for _, label := range []string{
		"fn_strpos",
		"fn_Auth_check",
		"fn_Session_get",
		"fn___joss_db_query",
		"fn_User_find",
	} {
		if isFrameworkAdapterTarget(label) {
			t.Errorf("runtime or application symbol %q must not get a pass-through adapter", label)
		}
	}
}

func TestAOTRejectsUnimplementedHostServiceInsteadOfEmittingNoop(t *testing.T) {
	program := ir.NewProgram("unsupported_host_service")
	main := ir.NewFunction("main", ir.TypeI64)
	main.EntryBlock.AddInstruction(&ir.CallHostServiceInst{ClassName: "TestService", MethodName: "ping"})
	main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	program.AddFunction(main)

	builder := NewMachineCodeBuilder(Target{OS: "linux", Arch: "amd64"}, false)
	_, _, err := builder.Compile(program)
	if err == nil || !strings.Contains(err.Error(), "TestService::ping") {
		t.Fatalf("AOT compile should reject unimplemented TestService::ping, got %v", err)
	}
}

func TestHostServiceAvailabilityMatchesExecutionMode(t *testing.T) {
	const registeredService = "ParityProbe::invoke"
	jit.RegisterHostBridgeHandler(registeredService, func(...uintptr) uintptr { return 1 })

	for _, mode := range []struct {
		name string
		jit  bool
	}{
		{name: "AOT"},
		{name: "JIT", jit: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			builder := NewMachineCodeBuilder(Target{OS: "windows", Arch: "amd64"}, false)
			builder.isJITMode = mode.jit
			if !builder.hostServiceAvailable("Server", "start") {
				t.Fatal("Server::start must use its shared native implementation")
			}
			if !builder.hostServiceAvailable("Session", "get") {
				t.Fatal("AMD64 Session::get must use its emitted native implementation")
			}
			if !builder.hostServiceAvailable("Str", "contains") {
				t.Fatal("Str::contains must use its emitted native implementation")
			}
			program := ir.NewProgram("joss_grandb_service")
			program.AddFunction(ir.NewFunction("GranDB_table", ir.TypeI64))
			builder.prog = program
			if !builder.hostServiceAvailable("GranDB", "table") {
				t.Fatal("a service with a Joss Standard Library implementation should be accepted")
			}
			if mode.jit && !builder.hostServiceAvailable("ParityProbe", "invoke") {
				t.Fatal("registered JIT callback should be accepted")
			}
			if builder.hostServiceAvailable("TestService", "missing") {
				t.Fatal("unimplemented service must be rejected")
			}
		})
	}
}

func TestAArch64RejectsUnimplementedHostService(t *testing.T) {
	program := ir.NewProgram("unsupported_host_service_arm64")
	main := ir.NewFunction("main", ir.TypeI64)
	main.EntryBlock.AddInstruction(&ir.CallHostServiceInst{ClassName: "TestService", MethodName: "ping"})
	main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	program.AddFunction(main)

	builder := NewARM64Builder(Target{OS: "linux", Arch: "arm64"}, false)
	_, _, err := builder.Compile(program)
	if err == nil || !strings.Contains(err.Error(), "TestService::ping") {
		t.Fatalf("AArch64 AOT compile should reject unimplemented TestService::ping, got %v", err)
	}
}

func TestUnresolvedRuntimeSymbolsFailForBothNativeBackends(t *testing.T) {
	t.Run("amd64", func(t *testing.T) {
		b := NewMachineCodeBuilder(Target{OS: "linux", Arch: "amd64"}, false)
		b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, targetLabel: "fn_strpos"})
		err := b.applyRelocations(make([]byte, 4), 0, 0, 0, nil)
		if err == nil || !strings.Contains(err.Error(), "fn_strpos") {
			t.Fatalf("expected unresolved fn_strpos error, got %v", err)
		}
	})

	t.Run("arm64", func(t *testing.T) {
		b := NewARM64Builder(Target{OS: "linux", Arch: "arm64"}, false)
		b.relocs = append(b.relocs, arm64Reloc{kind: arm64RelocBranch, targetLabel: "fn_strpos"})
		err := b.applyRelocations(make([]byte, 4), 0, 0, 0)
		if err == nil || !strings.Contains(err.Error(), "fn_strpos") {
			t.Fatalf("expected unresolved fn_strpos error, got %v", err)
		}
	})
}
