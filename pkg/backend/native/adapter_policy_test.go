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

func TestNativeSocketHostServicesMatchEmittedTargetABIs(t *testing.T) {
	cases := []struct {
		target Target
		want   bool
	}{
		{target: Target{OS: "windows", Arch: "amd64"}, want: true},
		{target: Target{OS: "linux", Arch: "amd64"}, want: true},
		{target: Target{OS: "linux", Arch: "arm64"}, want: true},
		{target: Target{OS: "darwin", Arch: "arm64"}, want: true},
		{target: Target{OS: "darwin", Arch: "amd64"}, want: false},
		{target: Target{OS: "windows", Arch: "arm64"}, want: false},
	}
	for _, tc := range cases {
		for _, method := range []string{"listen", "accept", "receive", "send", "close"} {
			if got := isSupportedHostService(tc.target, "Socket", method); got != tc.want {
				t.Errorf("Socket::%s support for %s = %v, want %v", method, tc.target, got, tc.want)
			}
		}
	}
	if isSupportedHostService(Target{OS: "linux", Arch: "amd64"}, "Socket", "bind") {
		t.Fatal("unemitted Socket::bind must not be reported as supported")
	}
}

func TestJITCompilesNativeSocketCallsOnWindowsAMD64(t *testing.T) {
	if !jit.SupportsInProcessHostBridge() {
		t.Skip("the in-memory socket server currently requires Windows AMD64")
	}
	program := ir.NewProgram("jit_native_socket")
	main := ir.NewFunction("main", ir.TypeI64)
	main.EntryBlock.AddInstruction(&ir.CallHostServiceInst{ClassName: "Socket", MethodName: "listen"})
	main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	program.AddFunction(main)

	builder := NewMachineCodeBuilder(HostTarget(), false)
	if _, err := builder.CompileInMemory(program); err != nil {
		t.Fatalf("JIT must compile Socket::listen through the native socket intrinsic: %v", err)
	}
}

func TestJITRejectsCrossTargetCompilation(t *testing.T) {
	host := HostTarget()
	otherOS := "linux"
	if host.OS == "linux" {
		otherOS = "windows"
	}
	program := ir.NewProgram("jit_cross_target")
	main := ir.NewFunction("main", ir.TypeI64)
	main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	program.AddFunction(main)

	builder := NewMachineCodeBuilder(Target{OS: otherOS, Arch: "amd64"}, false)
	if _, err := builder.CompileInMemory(program); err == nil || !strings.Contains(err.Error(), "requires the current AMD64 host target") {
		t.Fatalf("JIT should reject cross-target execution, got %v", err)
	}
}

func TestJITBridgeBuiltinsHaveExplicitHandlerNames(t *testing.T) {
	for _, id := range []string{"json", "str_contains", "rawurldecode", "__joss_db_query"} {
		if handler, ok := jitBuiltinBridgeHandler(id); !ok || handler != id {
			t.Errorf("JIT bridge builtin %q maps to (%q, %v), want (%q, true)", id, handler, ok, id)
		}
	}
	if _, ok := jitBuiltinBridgeHandler("strpos"); ok {
		t.Fatal("native JIT strpos must not be routed through a Go callback")
	}
}

func TestJITRejectsBridgeCallsWhenPlatformHasNoCallbackABI(t *testing.T) {
	if jit.SupportsInProcessHostBridge() {
		t.Skip("the current platform provides the Windows AMD64 callback ABI")
	}
	const service = "UnixBridgeProbe::call"
	jit.RegisterHostBridgeHandler(service, func(...uintptr) uintptr { return 7 })

	builder := NewMachineCodeBuilder(HostTarget(), false)
	builder.isJITMode = true
	builder.prog = ir.NewProgram("jit_unix_bridge_probe")
	if builder.hostServiceAvailable("UnixBridgeProbe", "call") {
		t.Fatal("JIT must reject registered callbacks when the platform has no callback ABI")
	}
	if builder.hostServiceAvailable("Http", "json") {
		t.Fatal("JIT must reject built-in callbacks when the platform has no callback ABI")
	}

	program := ir.NewProgram("jit_unix_bridge_rejection")
	main := ir.NewFunction("main", ir.TypeI64)
	main.EntryBlock.AddInstruction(&ir.CallHostServiceInst{ClassName: "UnixBridgeProbe", MethodName: "call"})
	main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	program.AddFunction(main)
	builder = NewMachineCodeBuilder(HostTarget(), false)
	if _, err := builder.CompileInMemory(program); err == nil || !strings.Contains(err.Error(), "UnixBridgeProbe::call") {
		t.Fatalf("JIT should reject a bridge call before allocating code, got %v", err)
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
