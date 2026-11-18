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

func isDatabaseHostBridgeRuntime(name string) bool {
	switch name {
	case "__joss_db_is_connected", "__joss_db_query", "__joss_db_execute":
		return true
	default:
		return false
	}
}

// validateSharedBackendCapabilities reports runtime capabilities that do not
// have a native implementation for this execution sink. It is shared by both
// ISA emitters so an unsupported feature fails before relocation or loading.
func validateSharedBackendCapabilities(program *ir.Program, target Target, jitMode bool) error {
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
		if jitMode && jit.SupportsInProcessHostBridge() && jit.HasHostBridgeHandler(name) {
			return
		}
		report("database runtime " + name + " (no native implementation or available JIT callback)")
	}
	checkHostService := func(className, methodName string) {
		if hasJossHostServiceImplementation(program, className, methodName) || isSupportedHostService(target, className, methodName) {
			return
		}
		service := className + "::" + methodName
		if jitMode && jit.SupportsInProcessHostBridge() && jit.HasHostBridgeHandler(service) {
			return
		}
		report("host service " + service + " (no Joss implementation or native platform ABI)")
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
	var scanBlocks func([]*ir.BasicBlock)
	scanBlocks = func(blocks []*ir.BasicBlock) {
		for _, block := range blocks {
			for _, instruction := range block.Instructions {
				switch call := instruction.(type) {
				case *ir.CallBuiltinInst:
					checkProductShim(call.BuiltinID)
					if call.BuiltinID == "json_decode" {
						if !jitMode || !jit.SupportsInProcessHostBridge() || !jit.HasHostBridgeHandler("json_decode") {
							report("builtin json_decode (native implementation unavailable; JIT host callback unavailable)")
						}
					}
					checkDatabaseCall(call.BuiltinID)
				case *ir.CallRuntimeInst:
					checkDatabaseCall(call.Func)
					checkProductShim(call.Func)
				case *ir.CallInst:
					checkDatabaseCall(call.Callee)
					checkProductShim(call.Callee)
					if program.Functions[call.Callee] == nil && isUnimplementedFrameworkTarget("fn_"+call.Callee) {
						report("framework method " + call.Callee + " (missing Joss implementation; placeholder emission is disabled)")
					}
				case *ir.CallHostServiceInst:
					checkProductShim(call.ClassName + "_" + call.MethodName)
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
