package native

import (
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/jit"
)

var (
	_ NativeCodeEmitter = (*MachineCodeBuilder)(nil)
	_ NativeCodeEmitter = (*ARM64Builder)(nil)
)

func TestUnimplementedFrameworkSymbolPolicyIsNarrow(t *testing.T) {
	for _, label := range []string{
		"fn_Router_get",
		"fn_Schema_foreign",
		"fn_Plugin_register",
		"fn_GranDB_table",
		"fn_HttpServer_start",
	} {
		if !isUnimplementedFrameworkTarget(label) {
			t.Errorf("expected framework placeholder target %q to be identified", label)
		}
	}
	for _, label := range []string{
		"fn_strpos",
		"fn_Auth_check",
		"fn_Session_get",
		"fn___joss_db_query",
		"fn_User_find",
	} {
		if isUnimplementedFrameworkTarget(label) {
			t.Errorf("runtime or application symbol %q must not be classified as a framework placeholder", label)
		}
	}
}

func TestAOTRejectsMissingFrameworkMethodInsteadOfEmittingPlaceholder(t *testing.T) {
	program := ir.NewProgram("missing_framework_method")
	main := ir.NewFunction("main", ir.TypeI64)
	main.EntryBlock.AddInstruction(&ir.CallInst{Callee: "Router_get"})
	main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	program.AddFunction(main)

	for _, emitter := range []NativeCodeEmitter{
		NewMachineCodeBuilder(Target{OS: "linux", Arch: "amd64"}, false),
		NewARM64Builder(Target{OS: "linux", Arch: "arm64"}, false),
	} {
		if _, _, err := emitter.Compile(program); err == nil || !strings.Contains(err.Error(), "JOSS-NATIVE-001") || !strings.Contains(err.Error(), "Router_get") {
			t.Errorf("missing framework implementation must fail with JOSS-NATIVE-001, got %v", err)
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

func TestAOTDoesNotEmitHostBridgeStub(t *testing.T) {
	program := ir.NewProgram("standalone_without_host_bridge")
	main := ir.NewFunction("main", ir.TypeI64)
	main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	program.AddFunction(main)

	builder := NewMachineCodeBuilder(Target{OS: "linux", Arch: "amd64"}, false)
	if _, listing, err := builder.Compile(program); err != nil {
		t.Fatalf("compile failed: %v", err)
	} else if strings.Contains(listing, "_joss_host_bridge") {
		t.Fatal("standalone AOT output must not contain a no-op host bridge symbol")
	}
	if builder.NeedsHostBridge() {
		t.Fatal("standalone AOT output must never report a host bridge dependency")
	}
}

func TestNativeEmittersRejectUnverifiedIR(t *testing.T) {
	program := ir.NewProgram("invalid_without_terminator")
	program.AddFunction(ir.NewFunction("main", ir.TypeI64))

	for _, target := range []Target{{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}} {
		t.Run(target.DashString(), func(t *testing.T) {
			_, _, err := SelectCodeEmitter(target, false).Compile(program)
			if err == nil || !strings.Contains(err.Error(), "IR no verificado") {
				t.Fatalf("native emitter must reject invalid IR before code generation, got %v", err)
			}
		})
	}
}

func TestNativeEmittersReportUnsupportedRuntimeCapabilities(t *testing.T) {
	targets := []Target{
		{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"}, {OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"}, {OS: "darwin", Arch: "arm64"},
		{OS: "android", Arch: "arm64"},
	}
	for _, target := range targets {
		t.Run(target.DashString()+"/json_decode", func(t *testing.T) {
			program := ir.NewProgram("unsupported_json_decode")
			main := ir.NewFunction("main", ir.TypeI64)
			result := main.NewTemp(ir.TypeI64, "decoded")
			main.EntryBlock.AddInstruction(&ir.CallBuiltinInst{
				Dest:      result,
				BuiltinID: "json_decode",
				Args:      []ir.Value{ir.NewConstString(`{"ok":true}`)},
			})
			main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
			program.AddFunction(main)

			_, _, err := SelectCodeEmitter(target, false).Compile(program)
			if err == nil || !strings.Contains(err.Error(), "JOSS-NATIVE-001") || !strings.Contains(err.Error(), "json_decode") {
				t.Fatalf("native backend must report the json_decode gap explicitly, got %v", err)
			}
		})

		for _, runtimeCall := range []string{"__joss_db_is_connected", "__joss_db_query", "__joss_db_execute"} {
			t.Run(target.DashString()+"/"+runtimeCall, func(t *testing.T) {
				program := ir.NewProgram("unsupported_database_runtime")
				main := ir.NewFunction("main", ir.TypeI64)
				main.EntryBlock.AddInstruction(&ir.CallRuntimeInst{Func: runtimeCall})
				main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
				program.AddFunction(main)

				_, _, err := SelectCodeEmitter(target, false).Compile(program)
				if err == nil || !strings.Contains(err.Error(), "JOSS-NATIVE-001") || !strings.Contains(err.Error(), runtimeCall) {
					t.Fatalf("native backend must report standalone database gap %s explicitly, got %v", runtimeCall, err)
				}
			})
		}

		t.Run(target.DashString()+"/session", func(t *testing.T) {
			program := ir.NewProgram("unsupported_session_state")
			main := ir.NewFunction("main", ir.TypeI64)
			main.EntryBlock.AddInstruction(&ir.CallHostServiceInst{ClassName: "Session", MethodName: "get", Args: []ir.Value{ir.NewConstString("user_id")}})
			main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
			program.AddFunction(main)

			_, _, err := SelectCodeEmitter(target, false).Compile(program)
			if err == nil || !strings.Contains(err.Error(), "JOSS-NATIVE-001") || !strings.Contains(err.Error(), "Session::get") {
				t.Fatalf("native backend must report missing session state explicitly, got %v", err)
			}
		})
	}
}

func TestJITJSONDecodeBridgeIsAvailableWhenTheHostSupportsCallbacks(t *testing.T) {
	if !jit.SupportsInProcessHostBridge() {
		t.Skip("host callback bridge is only available on supported hosts")
	}

	program := ir.NewProgram("jit_json_decode")
	main := ir.NewFunction("main", ir.TypeI64)
	decoded := main.NewTemp(ir.TypeI64, "decoded")
	main.EntryBlock.AddInstruction(&ir.CallBuiltinInst{
		Dest:      decoded,
		BuiltinID: "json_decode",
		Args:      []ir.Value{ir.NewConstString(`{"ok":true}`)},
	})
	main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	program.AddFunction(main)

	emitter := SelectCodeEmitter(HostTarget(), false)
	executable, err := emitter.CompileInMemory(program)
	if err != nil {
		t.Fatalf("JIT should compile json_decode when its host callback is available: %v", err)
	}
	defer executable.Free()
	if exitCode, err := executable.Execute(); err != nil || exitCode != 0 {
		t.Fatalf("JIT json_decode execution = (%d, %v), want (0, nil)", exitCode, err)
	}
}

func TestProductServiceCallsRequireAndPreferJossImplementations(t *testing.T) {
	for _, target := range []Target{{OS: "windows", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}} {
		t.Run(target.DashString()+"/missing", func(t *testing.T) {
			program := ir.NewProgram("missing_auth_owner")
			main := ir.NewFunction("main", ir.TypeI64)
			main.EntryBlock.AddInstruction(&ir.CallHostServiceInst{ClassName: "Auth", MethodName: "check"})
			main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
			program.AddFunction(main)

			_, _, err := SelectCodeEmitter(target, false).Compile(program)
			if err == nil || !strings.Contains(err.Error(), "JOSS-NATIVE-001") || !strings.Contains(err.Error(), "Auth_check") {
				t.Fatalf("missing Joss Auth implementation must fail explicitly, got %v", err)
			}
		})

		t.Run(target.DashString()+"/joss_owner", func(t *testing.T) {
			program := ir.NewProgram("joss_auth_owner")
			authCheck := ir.NewFunction("Auth_check", ir.TypeBool)
			authCheck.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstBool(true)})
			program.AddFunction(authCheck)
			main := ir.NewFunction("main", ir.TypeI64)
			result := main.NewTemp(ir.TypeBool, "check")
			main.EntryBlock.AddInstruction(&ir.CallHostServiceInst{Dest: result, ClassName: "Auth", MethodName: "check"})
			main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
			program.AddFunction(main)

			_, listing, err := SelectCodeEmitter(target, false).Compile(program)
			if err != nil {
				t.Fatalf("Joss-owned Auth implementation should compile: %v", err)
			}
			if count := strings.Count(listing, "fn_Auth_check:"); count != 1 {
				t.Fatalf("Joss Auth implementation was shadowed by a native placeholder: label count=%d", count)
			}
		})
	}
}

func TestSessionCapabilityFailureMatchesJITAndAOT(t *testing.T) {
	program := ir.NewProgram("session_capability_parity")
	main := ir.NewFunction("main", ir.TypeI64)
	main.EntryBlock.AddInstruction(&ir.CallHostServiceInst{
		ClassName:  "Session",
		MethodName: "get",
		Args:       []ir.Value{ir.NewConstString("user_id")},
	})
	main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	program.AddFunction(main)

	emitter := SelectCodeEmitter(HostTarget(), false)
	_, _, aotErr := emitter.Compile(program)
	if aotErr == nil {
		t.Fatal("AOT unexpectedly compiled Session::get without request-scoped session state")
	}
	_, jitErr := emitter.CompileInMemory(program)
	if jitErr == nil {
		t.Fatal("JIT unexpectedly compiled Session::get without request-scoped session state")
	}
	for mode, err := range map[string]error{"AOT": aotErr, "JIT": jitErr} {
		if !strings.Contains(err.Error(), "JOSS-NATIVE-001") || !strings.Contains(err.Error(), "Session::get") {
			t.Errorf("%s capability diagnostic = %v, want JOSS-NATIVE-001 for Session::get", mode, err)
		}
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

func TestNativeRequestInputSupportMatchesEmittedIntrinsic(t *testing.T) {
	for _, target := range []Target{
		{OS: "windows", Arch: "amd64"},
		{OS: "linux", Arch: "amd64"},
		{OS: "darwin", Arch: "amd64"},
		{OS: "windows", Arch: "arm64"},
		{OS: "linux", Arch: "arm64"},
		{OS: "darwin", Arch: "arm64"},
		{OS: "android", Arch: "arm64"},
	} {
		want := SupportsNativeTarget(target)
		if got := isSupportedHostService(target, "Request", "input"); got != want {
			t.Errorf("Request::input support for %s = %v, want %v", target, got, want)
		}
	}
	if isSupportedHostService(Target{OS: "linux", Arch: "amd64"}, "Request", "all") {
		t.Fatal("Request::all must remain unsupported until a matching target intrinsic exists")
	}
}

func TestSelectCodeEmitterRejectsUnsupportedArchitecture(t *testing.T) {
	program := ir.NewProgram("unsupported_arm32")
	main := ir.NewFunction("main", ir.TypeI64)
	main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	program.AddFunction(main)

	for _, target := range []Target{{OS: "linux", Arch: "arm"}, {OS: "plan9", Arch: "amd64"}} {
		_, _, err := SelectCodeEmitter(target, false).Compile(program)
		if err == nil || !strings.Contains(err.Error(), "target no soportado") || !strings.Contains(err.Error(), target.String()) {
			t.Errorf("unsupported target must fail instead of falling through to an emitter, got %v", err)
		}
	}

	for _, target := range []Target{{OS: "linux", Arch: "x86_64"}, {OS: "linux", Arch: "aarch64"}} {
		if _, _, err := SelectCodeEmitter(target, false).Compile(program); err != nil {
			t.Errorf("architecture alias %s should select its canonical emitter: %v", target, err)
		}
	}
}

func TestARM64JITRejectsCrossTargetExecution(t *testing.T) {
	target := Target{OS: "linux", Arch: "arm64"}
	if target == HostTarget() {
		target.OS = "darwin"
	}
	emitter := NewARM64Builder(target, false)
	if _, err := emitter.CompileInMemory(ir.NewProgram("arm64_cross_target_jit")); err == nil || !strings.Contains(err.Error(), "requires the current ARM64 target") {
		t.Fatalf("ARM64 JIT must reject cross-target execution, got %v", err)
	}
}

func TestWindowsARM64JITRejectsUnimplementedOSRuntimeCalls(t *testing.T) {
	program := ir.NewProgram("windows_arm64_jit_capabilities")
	main := ir.NewFunction("main", ir.TypeI64)
	main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(42, ir.TypeI64)})
	program.AddFunction(main)
	if err := validateWindowsARM64JITCapabilities(program); err != nil {
		t.Fatalf("pure arithmetic/control-flow program should be allowed: %v", err)
	}

	unsupported := ir.NewProgram("windows_arm64_jit_print")
	unsupportedMain := ir.NewFunction("main", ir.TypeI64)
	unsupportedMain.EntryBlock.AddInstruction(&ir.CallInst{Callee: "print", Args: []ir.Value{ir.NewConstString("hello")}})
	unsupportedMain.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	unsupported.AddFunction(unsupportedMain)
	if err := validateWindowsARM64JITCapabilities(unsupported); err == nil || !strings.Contains(err.Error(), "JOSS-NATIVE-001") || !strings.Contains(err.Error(), "print") {
		t.Fatalf("Windows ARM64 JIT must reject syscalls without a Windows ABI: %v", err)
	}

	indirect := ir.NewProgram("windows_arm64_jit_indirect_call")
	indirectMain := ir.NewFunction("main", ir.TypeI64)
	indirectMain.EntryBlock.AddInstruction(&ir.CallIndirectInst{CalleeVal: ir.NewConstInt(0, ir.TypeI64)})
	indirectMain.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	indirect.AddFunction(indirectMain)
	if err := validateWindowsARM64JITCapabilities(indirect); err == nil || !strings.Contains(err.Error(), "indirect call") {
		t.Fatalf("Windows ARM64 JIT must reject indirect calls without a verified runtime ABI: %v", err)
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
	for _, id := range []string{"json", "rawurldecode", "__joss_db_query"} {
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
			if builder.hostServiceAvailable("Server", "start") {
				t.Fatal("Server::start requires the Pure Joss HttpServer implementation")
			}
			if builder.hostServiceAvailable("Session", "get") {
				t.Fatal("Session::get must not be reported as native without request-scoped session state")
			}
			program := ir.NewProgram("joss_grandb_service")
			program.AddFunction(ir.NewFunction("Session_get", ir.TypeI64))
			program.AddFunction(ir.NewFunction("GranDB_table", ir.TypeI64))
			program.AddFunction(ir.NewFunction("HttpServer_start", ir.TypeI64))
			builder.prog = program
			if !builder.hostServiceAvailable("Session", "get") {
				t.Fatal("a Joss-owned Session_get implementation must be accepted")
			}
			if !builder.hostServiceAvailable("GranDB", "table") {
				t.Fatal("a service with a Joss Standard Library implementation should be accepted")
			}
			if !builder.hostServiceAvailable("Server", "start") {
				t.Fatal("Server::start should delegate to the Joss-owned HttpServer::start")
			}
			if mode.jit {
				if jit.SupportsInProcessHostBridge() {
					if !builder.hostServiceAvailable("ParityProbe", "invoke") {
						t.Fatal("registered JIT callback should be accepted")
					}
				} else {
					if builder.hostServiceAvailable("ParityProbe", "invoke") {
						t.Fatal("registered JIT callback should be rejected when platform has no callback ABI")
					}
				}
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

func TestArrayKeyExistsBuiltinEmitsForAMD64AndARM64(t *testing.T) {
	program := ir.NewProgram("array_key_exists")
	main := ir.NewFunction("main", ir.TypeI64)
	result := main.NewTemp(ir.TypeBool, "exists")
	main.EntryBlock.AddInstruction(&ir.CallBuiltinInst{
		Dest:      result,
		BuiltinID: "array_key_exists",
		Args: []ir.Value{
			ir.NewConstString("present"),
			ir.NewConstNull(ir.PtrType(ir.TypeI64)),
		},
	})
	main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	program.AddFunction(main)
	if err := ir.NewVerifier().Verify(program); err != nil {
		t.Fatalf("test IR is invalid: %v", err)
	}

	for _, target := range []Target{{OS: "windows", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}} {
		t.Run(target.DashString(), func(t *testing.T) {
			emitter := SelectCodeEmitter(target, false)
			binary, _, err := emitter.Compile(program)
			if err != nil {
				t.Fatalf("array_key_exists emission failed for %s: %v", target, err)
			}
			if len(binary) == 0 {
				t.Fatalf("array_key_exists emitted an empty binary for %s", target)
			}
		})
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
