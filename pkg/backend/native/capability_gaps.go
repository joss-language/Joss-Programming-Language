package native

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/jit"
)

// ValidateAOTCapabilities performs the same capability check as the native
// emitter without producing an artifact. The CLI uses it before optional
// project preparation that can touch external services, so unsupported AOT
// features fail before those side effects occur.
func ValidateAOTCapabilities(program *ir.Program, target Target) error {
	if target.OS == "" {
		target = HostTarget()
	}
	return validateSharedBackendCapabilities(program, target, false)
}

// ValidateAOTCapabilitiesForBackend validates AOT capabilities for a specific
// backend kind. BackendLLVM links joss_web_rt.c with native SQLite3 support.
func ValidateAOTCapabilitiesForBackend(program *ir.Program, target Target, backend BackendKind) error {
	if target.OS == "" {
		target = HostTarget()
	}
	return validateSharedBackendCapabilitiesForBackend(program, target, false, backend)
}

func isDatabaseHostBridgeRuntime(name string) bool {
	switch name {
	case "__joss_db_is_connected", "__joss_db_query", "__joss_db_execute":
		return true
	default:
		return false
	}
}

func validateSharedBackendCapabilities(program *ir.Program, target Target, jitMode bool) error {
	return validateSharedBackendCapabilitiesForBackend(program, target, jitMode, BackendNative)
}

// validateSharedBackendCapabilitiesForBackend reports runtime capabilities that do not
// have a native implementation for this execution sink. It is shared by both
// ISA emitters so an unsupported feature fails before relocation or loading.
func validateSharedBackendCapabilitiesForBackend(program *ir.Program, target Target, jitMode bool, backend BackendKind) error {
	if program == nil {
		return fmt.Errorf("backend nativo: programa IR nulo")
	}
	var gaps []string
	seen := make(map[string]bool)
	report := func(name string) {
		if !seen[name] {
			seen[name] = true
			gaps = append(gaps, name)
		}
	}
	checkDatabaseCall := func(name string) {
		if !isDatabaseHostBridgeRuntime(name) {
			return
		}
		if backend == BackendLLVM {
			return
		}
		if jitMode && jit.SupportsInProcessHostBridge() && jit.HasHostBridgeHandler(name) {
			return
		}
		report("database runtime " + name + " (no native implementation or available JIT callback)")
	}
	isSupportedLLVMHostService := func(className, methodName string) bool {
		if backend != BackendLLVM {
			return false
		}
		switch className {
		case "Cron":
			switch methodName {
			case "schedule", "runDue":
				return true
			}
		case "Cache":
			switch methodName {
			case "get", "put", "has", "forget":
				return true
			}
		case "Markdown":
			switch methodName {
			case "readFile", "toHtml":
				return true
			}
		case "Blueprint":
			return true
		case "Process":
			switch methodName {
			case "constructor", "start", "wait", "kill", "pid":
				return true
			}
		case "Response", "WebResponse", "Request", "Session", "Cookie", "Auth", "MFA", "TwoFactor", "Sitemap", "IndexNow", "SmtpClient", "Mail", "Http":
			return true
		case "System":
			switch methodName {
			case "Run", "sleep", "env", "log", "now":
				return true
			}
		case "Schema":
			switch methodName {
			case "create", "table", "drop", "dropIfExists", "rename", "hasTable", "hasColumn":
				return true
			}
		case "WebSocket":
			switch methodName {
			case "subscribe", "unsubscribe", "publish", "subscriberCount":
				return true
			}
		case "AI":
			switch methodName {
			case "client", "chat":
				return true
			}
		case "Notify":
			switch methodName {
			case "app", "registerDevice", "unregisterDevice", "pendingMessages", "acknowledge":
				return true
			}
		case "Plugin":
			switch methodName {
			case "call", "load", "path", "platform", "stream":
				return true
			}
		case "Console":
			switch methodName {
			case "log", "error", "readLine":
				return true
			}
		case "FileStream":
			switch methodName {
			case "open", "read", "write", "seek", "size", "flush", "close":
				return true
			}
		case "StreamReader":
			switch methodName {
			case "open", "readLine", "readToEnd", "close":
				return true
			}
		case "StreamWriter":
			switch methodName {
			case "open", "write", "writeLine", "flush", "close":
				return true
			}
		case "DateTime":
			switch methodName {
			case "now", "create", "parse", "format", "timestamp", "iso", "year", "month", "day", "hour", "minute", "second", "add", "sub", "diff":
				return true
			}
		case "DateInterval":
			switch methodName {
			case "days", "hours", "minutes", "seconds", "totalSeconds":
				return true
			}
		case "Zip":
			switch methodName {
			case "extract":
				return true
			}
		case "Model":
			return true
		case "Window":
			return true
		case "Lang":
			switch methodName {
			case "locale", "setLocale", "get":
				return true
			}
		case "Mutex":
			switch methodName {
			case "new", "lock", "unlock":
				return true
			}
		case "RWMutex":
			switch methodName {
			case "new", "lock", "unlock", "rLock", "rUnlock":
				return true
			}
		case "WaitGroup":
			switch methodName {
			case "new", "add", "done", "wait":
				return true
			}
		case "Socket":
			switch methodName {
			case "listen", "accept", "read", "recv", "send", "write", "close", "connect", "bind":
				return true
			}
		}
		return false
	}
	checkHostService := func(className, methodName string) {
		if hasJossHostServiceImplementation(program, className, methodName) || isSupportedHostService(target, className, methodName) || isSupportedLLVMHostService(className, methodName) {
			return
		}
		service := className + "::" + methodName
		if jitMode && jit.SupportsInProcessHostBridge() && jit.HasHostBridgeHandler(service) {
			return
		}
		report("host service " + service + " (no Joss implementation or native platform ABI)")
	}
	checkLLVMOnlyRuntimeCall := func(name string) {
		if backend == BackendLLVM {
			return
		}
		clean := strings.TrimPrefix(name, "fn_")
		if hasProgramFunction(program, clean) {
			return
		}
		if jitMode && jit.SupportsInProcessHostBridge() && jit.HasHostBridgeHandler(strings.TrimPrefix(clean, "joss_")) {
			return
		}
		trimmed := strings.TrimPrefix(clean, "joss_")
		switch {
		case clean == "joss_register_plugin_dispatcher",
			clean == "joss_register_schema_db",
			strings.HasPrefix(clean, "joss___joss_http_"),
			strings.HasPrefix(clean, "joss___joss_socket_"),
			strings.HasPrefix(clean, "joss___joss_session_"),
			strings.HasPrefix(clean, "joss___joss_rate_"),
			strings.HasPrefix(clean, "joss___joss_jwt_"),
			strings.HasPrefix(clean, "joss___joss_ws_"),
			strings.HasPrefix(clean, "joss_ws_"),
			strings.HasPrefix(trimmed, "__joss_http_"),
			strings.HasPrefix(trimmed, "__joss_socket_"),
			strings.HasPrefix(trimmed, "__joss_session_"),
			strings.HasPrefix(trimmed, "__joss_rate_"),
			strings.HasPrefix(trimmed, "__joss_jwt_"),
			strings.HasPrefix(trimmed, "__joss_ws_"),
			strings.HasPrefix(clean, "joss_sb_"),
			strings.HasPrefix(clean, "joss_buf_"):
			report("llvm runtime " + name + " (requires BackendLLVM C11 runtime)")
		case strings.HasPrefix(clean, "joss___joss_rt_"),
			strings.HasPrefix(trimmed, "__joss_rt_"):
			switch trimmed {
			case "__joss_rt_strlen", "__joss_rt_strcmp", "__joss_rt_throw", "__joss_rt_route_matches", "__joss_rt_route_param":
			default:
				report("llvm runtime " + name + " (requires BackendLLVM C11 runtime)")
			}
		}
	}
	checkProductShim := func(name string) {
		if strings.HasPrefix(strings.TrimPrefix(name, "fn_"), "SEO_") {
			method := strings.TrimPrefix(strings.TrimPrefix(name, "fn_"), "SEO_")
			if program.Functions["__joss_host_SEO_"+method] != nil {
				return
			}
		}
		if isProductShimWithoutJossOwner(name) && !hasProgramFunction(program, name) {
			report("product service " + strings.TrimPrefix(name, "fn_") + " (missing Joss implementation; placeholder emission is disabled)")
		}
	}
	checkARM64Stub := func(name string) {
		if target.Arch != "arm64" || backend == BackendLLVM {
			return
		}
		clean := strings.TrimPrefix(name, "fn_")
		if hasProgramFunction(program, clean) {
			return
		}
		switch clean {
		case "file_exists", "Markdown_toHtml", "redirect", "json", "response", "__joss_rt_route_matches", "__joss_rt_route_param":
			report("arm64 intrinsic " + clean + " (not yet implemented for ARM64; silent stub disabled)")
		}
	}
	var scanBlocks func([]*ir.BasicBlock)
	scanBlocks = func(blocks []*ir.BasicBlock) {
		for _, block := range blocks {
			for _, instruction := range block.Instructions {
				switch call := instruction.(type) {
				case *ir.AllocBufferInst, *ir.FreeBufferInst, *ir.BufferLenInst, *ir.BufferGetInst, *ir.BufferSetInst:
					if backend != BackendLLVM {
						report("bytebuffer native runtime (requires BackendLLVM)")
					}
				case *ir.CallBuiltinInst:
					checkProductShim(call.BuiltinID)
					checkARM64Stub(call.BuiltinID)
					if call.BuiltinID == "json_decode" {
						if backend != BackendLLVM && (!jitMode || !jit.SupportsInProcessHostBridge() || !jit.HasHostBridgeHandler("json_decode")) {
							report("builtin json_decode (native implementation unavailable; JIT host callback unavailable)")
						}
					}
					checkDatabaseCall(call.BuiltinID)
					checkLLVMOnlyRuntimeCall(call.BuiltinID)
				case *ir.CallRuntimeInst:
					checkDatabaseCall(call.Func)
					checkProductShim(call.Func)
					checkARM64Stub(call.Func)
					checkLLVMOnlyRuntimeCall(call.Func)
				case *ir.CallInst:
					checkDatabaseCall(call.Callee)
					checkProductShim(call.Callee)
					checkARM64Stub(call.Callee)
					checkLLVMOnlyRuntimeCall(call.Callee)
					if program.Functions[call.Callee] == nil && isUnimplementedFrameworkTarget("fn_"+call.Callee) {
						report("framework method " + call.Callee + " (missing Joss implementation; placeholder emission is disabled)")
					}
				case *ir.CallHostServiceInst:
					if !isSupportedLLVMHostService(call.ClassName, call.MethodName) {
						checkProductShim(call.ClassName + "_" + call.MethodName)
					}
					checkHostService(call.ClassName, call.MethodName)
				case *ir.TryCatchInst:
					scanBlocks(call.TryBlocks)
					scanBlocks(call.CatchBlocks)
				}
			}
		}
	}
	for _, function := range program.Functions {
		scanBlocks(function.Blocks)
	}
	if len(gaps) == 0 {
		return nil
	}
	sort.Strings(gaps)
	return fmt.Errorf("%s\n\nConstructos sin implementación nativa: %v", analyzer.FormatCapabilityError(gaps[0]), gaps)
}

// validateWindowsARM64JITCapabilities prevents the Windows ARM64 memory sink
// from running Linux syscall-based runtime helpers. Until their Windows ABI is
// emitted, only arithmetic/control flow and direct Joss function calls are safe.
func validateWindowsARM64JITCapabilities(program *ir.Program) error {
	if program == nil {
		return fmt.Errorf("JIT Windows ARM64: programa IR nulo")
	}
	var gaps []string
	seen := make(map[string]bool)
	report := func(name string) {
		if !seen[name] {
			seen[name] = true
			gaps = append(gaps, name)
		}
	}
	var scan func([]*ir.BasicBlock)
	scan = func(blocks []*ir.BasicBlock) {
		for _, block := range blocks {
			for _, instruction := range block.Instructions {
				switch call := instruction.(type) {
				case *ir.CallBuiltinInst:
					report("builtin " + call.BuiltinID + " (Windows ARM64 JIT runtime primitive is not implemented)")
				case *ir.CallRuntimeInst:
					report("runtime call " + call.Func + " (Windows ARM64 JIT runtime primitive is not implemented)")
				case *ir.CallHostServiceInst:
					report("host service " + call.ClassName + "::" + call.MethodName + " (Windows ARM64 JIT host service is not implemented)")
				case *ir.CallPrimitiveMethodInst:
					report("primitive method " + call.TypeKind + "::" + call.MethodName + " (Windows ARM64 JIT primitive is not implemented)")
				case *ir.CallInst:
					if program.Functions[call.Callee] == nil {
						report("call " + call.Callee + " (no Windows ARM64 JIT implementation)")
					}
				case *ir.CallIndirectInst:
					report("indirect call (Windows ARM64 JIT runtime primitive is not implemented)")
				case *ir.TryCatchInst:
					report("try/catch (Windows ARM64 JIT exception runtime is not implemented)")
					scan(call.TryBlocks)
					scan(call.CatchBlocks)
				case *ir.AllocaInst, *ir.StoreInst, *ir.LoadInst,
					*ir.GlobalLoadInst, *ir.GlobalStoreInst, *ir.MoveInst,
					*ir.BinaryInst, *ir.UnaryInst, *ir.CompareInst:
					// These instructions operate only on stack slots, static data,
					// or registers and do not enter OS-specific runtime helpers.
				default:
					report(fmt.Sprintf("IR instruction %T (Windows ARM64 JIT support is not implemented)", instruction))
				}
			}
		}
	}
	for _, function := range program.Functions {
		scan(function.Blocks)
	}
	if len(gaps) == 0 {
		return nil
	}
	sort.Strings(gaps)
	return fmt.Errorf("%s\n\nConstructos sin implementación JIT para Windows ARM64: %s", analyzer.FormatCapabilityError(gaps[0]), strings.Join(gaps, ", "))
}
