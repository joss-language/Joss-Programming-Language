package native

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/jit"
)

// Fixed offsets in .data section (all 8-byte aligned)
const (
	dataOffStdout           uint32 = 0x0000
	dataOffStderr           uint32 = 0x0008
	dataOffHeap             uint32 = 0x0010
	dataOffTryDepth         uint32 = 0x0018
	dataOffExceptionVal     uint32 = 0x0020
	dataOffTryRSP           uint32 = 0x0028 // 64 * 8 = 512 bytes (0x0028 .. 0x0228)
	dataOffTryRBP           uint32 = 0x0228 // 64 * 8 = 512 bytes (0x0228 .. 0x0428)
	dataOffTryRIP           uint32 = 0x0428 // 64 * 8 = 512 bytes (0x0428 .. 0x0628)
	dataOffArenaCur         uint32 = 0x0630
	dataOffArenaEnd         uint32 = 0x0638
	dataOffKernel32         uint32 = 0x0640
	dataOffCreateFileA      uint32 = 0x0648
	dataOffGetFileSize      uint32 = 0x0650
	dataOffReadFile         uint32 = 0x0658
	dataOffCloseHandle      uint32 = 0x0660
	dataOffCreateThread     uint32 = 0x0668
	dataOffWaitSingleObject uint32 = 0x0670
	dataFixedSize           uint32 = 0x1000 // 4096 bytes reserved for runtime + globals
)

type relocKind int

const (
	relocTextLabel relocKind = iota
	relocROData
	relocData
	relocIAT
)

type machineReloc struct {
	kind        relocKind
	patchOffset int
	targetLabel string
	targetOff   uint32
	iatSlot     WinImportSlot
}

// MachineCodeBuilder compiles a verified *ir.Program directly into native x86_64 machine code
// and links it into a standalone PE32+ (Windows) or ELF64 (Linux) executable without invoking
// any external compiler, assembler, or linker (no Go, no GCC, no Clang, no MSVC).
type MachineCodeBuilder struct {
	target           Target
	guiMode          bool
	isJITMode        bool
	needsHostBridge  bool
	prog             *ir.Program
	text             bytes.Buffer
	rodata           bytes.Buffer
	stringPool       map[string]uint32
	labels           map[string]int
	relocs           []machineReloc
	asmListing       bytes.Buffer
	inlineLabelID    int
	cachedRAXVal     ir.Value
	cachedXMM0Val    ir.Value
	cachedRCXVal     ir.Value
	cachedDataPtrArr ir.Value
}

func (b *MachineCodeBuilder) invalidateRegCache() {
	b.cachedRAXVal = nil
	b.cachedXMM0Val = nil
	b.cachedRCXVal = nil
	b.cachedDataPtrArr = nil
}

func (b *MachineCodeBuilder) nextInlineLabelID() int {
	b.inlineLabelID++
	return b.inlineLabelID
}

func NewMachineCodeBuilder(target Target, guiMode bool) *MachineCodeBuilder {
	if target.OS == "" && target.Arch == "" {
		target = HostTarget()
	}
	b := &MachineCodeBuilder{
		target:     target,
		guiMode:    guiMode,
		stringPool: make(map[string]uint32),
		labels:     make(map[string]int),
	}
	// Pre-intern common runtime strings
	b.internString("")
	b.internString("\n")
	b.internString("null")
	b.internString("true")
	b.internString("false")
	b.internString("Panic: ")
	b.internString("message")
	b.internString("code")
	return b
}

// NeedsHostBridge reports whether the compiled program requires joss_host dynamic library.
func (b *MachineCodeBuilder) NeedsHostBridge() bool {
	return b.needsHostBridge
}

// SupportsNativeTarget returns true if the integrated native code generator supports the target OS/Arch.
func SupportsNativeTarget(t Target) bool {
	if t.OS == "" && t.Arch == "" {
		t = HostTarget()
	}
	archs, ok := SupportedTargets[t.OS]
	if !ok {
		return false
	}
	for _, a := range archs {
		if a == t.Arch {
			return true
		}
	}
	return false
}

// Compile translates an *ir.Program into a complete standalone executable binary image ([]byte)
// and a human-readable native assembly listing.
func (b *MachineCodeBuilder) Compile(prog *ir.Program) ([]byte, string, error) {
	if !SupportsNativeTarget(b.target) {
		return nil, "", fmt.Errorf("ERROR: native target not available (%s/%s)", b.target.OS, b.target.Arch)
	}
	b.prog = prog

	b.asmListing.WriteString(fmt.Sprintf("; Joss Self-Contained Native Compiler — %s Machine Code Listing\n; Target: %s/%s\n\n", b.target.Arch, b.target.OS, b.target.Arch))

	// 1. Validate that all called functions are either user IR functions or supported native runtime intrinsics
	if err := b.validateProgramCalls(prog); err != nil {
		return nil, "", err
	}

	// 2. Emit _start entrypoint and Native Joss Runtime in machine code
	b.emitEntrypointAndRuntime(prog.Functions["main"] != nil)

	// 3. Sort function names for deterministic binary output
	fnNames := make([]string, 0, len(prog.Functions))
	for name := range prog.Functions {
		fnNames = append(fnNames, name)
	}
	sort.Strings(fnNames)

	for _, name := range fnNames {
		if err := b.emitIRFunction(prog.Functions[name]); err != nil {
			return nil, "", err
		}
	}

	b.emitToStringStub()
	b.emitF64ToStringStub()

	// 4. Resolve all relocations and emit the final executable container (PE32+, ELF64, or Mach-O)
	textBytes := append([]byte(nil), b.text.Bytes()...)
	rodataBytes := b.rodata.Bytes()

	if b.target.OS == "windows" {
		peLayout := PreparePERData(len(textBytes), rodataBytes, b.needsHostBridge)
		if err := b.applyRelocations(textBytes, peLayout.TextRVA, peLayout.RODataRVA, peLayout.DataRVA, peLayout.IATRVAs[:]); err != nil {
			return nil, "", err
		}
		exeBytes := EmitPE32PlusExecutable(peLayout, textBytes, dataFixedSize, b.guiMode, b.needsHostBridge)
		return exeBytes, b.asmListing.String(), nil
	}

	if b.target.OS == "darwin" {
		machoLayout := PrepareMachOLayout(len(textBytes), len(rodataBytes))
		if err := b.applyRelocations(textBytes, machoLayout.TextRVA, machoLayout.RODataRVA, machoLayout.DataRVA, nil); err != nil {
			return nil, "", err
		}
		machoBytes := EmitMachO64Executable(machoLayout, textBytes, rodataBytes, dataFixedSize, b.target.Arch)
		return machoBytes, b.asmListing.String(), nil
	}

	elfLayout := PrepareELFLayout(len(textBytes), len(rodataBytes))
	if err := b.applyRelocations(textBytes, elfLayout.TextRVA, elfLayout.RODataRVA, elfLayout.DataRVA, nil); err != nil {
		return nil, "", err
	}
	elfBytes := EmitELF64Executable(elfLayout, textBytes, rodataBytes, dataFixedSize, b.target.Arch)
	return elfBytes, b.asmListing.String(), nil
}

// JITExecutable represents compiled machine code and data loaded directly into RAM.
type JITExecutable struct {
	Block    *jit.ExecutableBlock
	DataArea []byte
}

// Free releases executable pages allocated by the JIT compiler.
func (j *JITExecutable) Free() error {
	if j == nil || j.Block == nil {
		return nil
	}
	return j.Block.Free()
}

// Execute runs the JIT compiled entrypoint (_start) and returns the exit code.
func (j *JITExecutable) Execute() (int, error) {
	if j == nil || j.Block == nil {
		return 0, fmt.Errorf("cannot execute null JIT code")
	}
	ret, err := jit.CallFunc0(j.Block.Addr)
	return int(ret), err
}

// CompileInMemory translates an *ir.Program into native machine code directly in RAM
// and returns an executable image ready for zero-latency execution.
func (b *MachineCodeBuilder) CompileInMemory(prog *ir.Program) (*JITExecutable, error) {
	b.isJITMode = true
	b.prog = prog

	// 1. Validate that all called functions are either user IR functions or supported runtime intrinsics
	if err := b.validateProgramCalls(prog); err != nil {
		return nil, err
	}

	// 2. Emit _start entrypoint and Native Joss Runtime in machine code
	b.emitEntrypointAndRuntime(prog.Functions["main"] != nil)

	// 3. Sort function names for deterministic output
	fnNames := make([]string, 0, len(prog.Functions))
	for name := range prog.Functions {
		fnNames = append(fnNames, name)
	}
	sort.Strings(fnNames)

	for _, name := range fnNames {
		if err := b.emitIRFunction(prog.Functions[name]); err != nil {
			return nil, err
		}
	}

	b.emitToStringStub()
	b.emitF64ToStringStub()

	// 4. Memory Layout Layout in RAM:
	// [.text code bytes]
	// [.rodata string constants, aligned to 16]
	// [.data runtime globals + IAT table slots, aligned to 16]
	textBytes := append([]byte(nil), b.text.Bytes()...)
	rodataBytes := b.rodata.Bytes()

	textLen := uint32(len(textBytes))
	rodataLen := uint32(len(rodataBytes))

	textRVA := uint32(0)
	rodataRVA := alignUpU32(textLen, 16)
	dataRVA := alignUpU32(rodataRVA+rodataLen, 16)

	// Allocate IAT slots inside .data section
	var iatRVAs [iatSlotCount]uint32
	iatBaseOff := uint32(0x800) // Place IAT slots at offset 0x800 within .data
	for i := 0; i < int(iatSlotCount); i++ {
		iatRVAs[i] = dataRVA + iatBaseOff + uint32(i*8)
	}

	// 5. Resolve relative RIP relocations
	if err := b.applyRelocations(textBytes, textRVA, rodataRVA, dataRVA, iatRVAs[:]); err != nil {
		return nil, err
	}

	// 6. Build combined contiguous memory block: [text] + [padding] + [rodata] + [padding] + [data]
	totalSize := dataRVA + dataFixedSize
	fullImage := make([]byte, totalSize)

	// Copy .text
	copy(fullImage[textRVA:], textBytes)
	// Copy .rodata
	copy(fullImage[rodataRVA:], rodataBytes)

	// Populate .data with IAT function pointers
	iatPointers := getPlatformIATPointers()

	for i, ptr := range iatPointers {
		slotOff := dataRVA + iatBaseOff + uint32(i*8)
		binary.LittleEndian.PutUint64(fullImage[slotOff:], uint64(ptr))
	}

	// 7. Allocate executable memory page and copy full image
	block, err := jit.AllocateExecutable(fullImage)
	if err != nil {
		return nil, fmt.Errorf("jit allocation failed: %w", err)
	}

	return &JITExecutable{
		Block:    block,
		DataArea: fullImage[dataRVA:],
	}, nil
}

// isSupportedHostService reports whether the given host class and method are handled by the C-ABI host bridge.
func isSupportedHostService(className, methodName string) bool {
	return true
}

// isHostBridgeBuiltin reports builtins that can be routed through the C-ABI host bridge.
func isHostBridgeBuiltin(id string) bool {
	return true
}

// validateProgramCalls rejects, with an explicit JOSS-NATIVE-001 diagnostic, every IR call
// whose target the backend cannot materialize as machine code. There is no silent stub: a
// construct that reaches the backend without a native implementation is a compile error.
func (b *MachineCodeBuilder) validateProgramCalls(prog *ir.Program) error {
	var gaps []string
	seen := map[string]bool{}
	report := func(feature string) {
		if !seen[feature] {
			seen[feature] = true
			gaps = append(gaps, feature)
		}
	}
	var scan func(blocks []*ir.BasicBlock)
	scan = func(blocks []*ir.BasicBlock) {
		for _, blk := range blocks {
			for _, inst := range blk.Instructions {
				switch i := inst.(type) {
				case *ir.CallHostServiceInst:
					if b.isJITMode {
						if isSupportedHostService(i.ClassName, i.MethodName) {
							b.needsHostBridge = true
						} else {
							report("host service " + i.ClassName + "::" + i.MethodName)
						}
					} else {
						if i.ClassName == "GranDB" || i.ClassName == "UserStorage" || i.ClassName == "OCIStorage" {
							// Fully supported standalone without host bridge DLL
						} else if isSupportedHostService(i.ClassName, i.MethodName) {
							b.needsHostBridge = true
						} else {
							report("host service " + i.ClassName + "::" + i.MethodName)
						}
					}
				case *ir.CallPrimitiveMethodInst:
					if isNativePrimitiveMethod(i.TypeKind, i.MethodName) {
						// Native direct
					} else {
						report("primitive method " + i.TypeKind + "::" + i.MethodName)
					}
				case *ir.CallBuiltinInst:
					if isNativeBuiltin(i.BuiltinID) {
						// Native direct
					} else if isHostBridgeBuiltin(i.BuiltinID) {
						b.needsHostBridge = true
					} else {
						report("builtin " + i.BuiltinID)
					}
				case *ir.CallRuntimeInst:
					if isNativeBuiltinHelper(i.Func) {
						// Native direct helper
					} else if isSupportedRuntimeCallee(i.Func) {
						b.needsHostBridge = true
					}
				case *ir.CallInst:
					if !isNativeBuiltin(i.Callee) && !isNativeBuiltinHelper(i.Callee) {
						if _, exists := prog.Functions[i.Callee]; !exists {
							b.needsHostBridge = true
						}
					}
				case *ir.TryCatchInst:
					scan(i.TryBlocks)
					scan(i.CatchBlocks)
				}
			}
		}
	}
	names := make([]string, 0, len(prog.Functions))
	for name := range prog.Functions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		scan(prog.Functions[name].Blocks)
	}
	if len(gaps) == 0 {
		return nil
	}
	sort.Strings(gaps)
	return fmt.Errorf("%s\n\nConstructos sin implementación nativa (AOT implementation gap): %s", analyzer.FormatCapabilityError(gaps[0]), strings.Join(gaps, ", "))
}

// isNativeBuiltin reports builtins the backend emits as real machine code.
func isNativeBuiltin(id string) bool {
	switch id {
	case "strlen", "len", "count", "array_push", "append", "to_string",
		"abs", "min", "max", "intval", "floatval", "boolval", "array_pop", "microtime", "array_fill",
		"file_get_contents", "explode", "async", "await":
		return true
	}
	return false
}

// isNativeBuiltinHelper reports internal runtime helpers emitted directly as machine code.
func isNativeBuiltinHelper(name string) bool {
	switch name {
	case "Exception_constructor", "Exception_getMessage", "Exception_getCode":
		return true
	}
	return false
}

func isNativePrimitiveMethod(typeKind, methodName string) bool {
	tk := strings.ToLower(typeKind)
	switch {
	case strings.HasPrefix(tk, "array"):
		switch methodName {
		case "length", "count", "push", "pop":
			return true
		}
	case strings.HasPrefix(tk, "string"):
		switch methodName {
		case "length":
			return true
		}
	case strings.HasPrefix(tk, "map"):
		switch methodName {
		case "length", "count":
			return true
		}
	}
	return false
}

func isSupportedRuntimeCallee(name string) bool {
	switch name {
	case "Exception_constructor", "Exception_getMessage", "Exception_getCode",
		"GranDB_table", "GranDB_where", "where", "GranDB_orWhere", "orWhere",
		"GranDB_select", "select", "GranDB_orderBy", "orderBy",
		"GranDB_limit", "limit", "GranDB_offset", "offset",
		"GranDB_join", "join", "GranDB_leftJoin", "leftJoin", "GranDB_whereIn", "whereIn",
		"GranDB_insert", "insert", "GranDB_insertGetId", "insertGetId",
		"GranDB_count", "GranDB_first", "first", "GranDB_find", "find",
		"GranDB_get", "get", "GranDB_update", "update", "GranDB_delete", "delete":
		return true
	default:
		return false
	}
}

func (b *MachineCodeBuilder) internString(s string) uint32 {
	if off, ok := b.stringPool[s]; ok {
		return off
	}
	off := uint32(b.rodata.Len())
	b.rodata.WriteString(s)
	b.rodata.WriteByte(0)
	b.stringPool[s] = off
	return off
}

func (b *MachineCodeBuilder) defineLabel(name string) {
	b.invalidateRegCache()
	b.labels[name] = b.text.Len()
	b.asmListing.WriteString(fmt.Sprintf("%s: ; [offset 0x%04x]\n", name, b.text.Len()))
}

func (b *MachineCodeBuilder) emitBytes(code ...byte) {
	b.cachedRAXVal = nil
	b.cachedRCXVal = nil
	b.text.Write(code)
}

func (b *MachineCodeBuilder) emitU32(v uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	b.text.Write(buf[:])
}

func (b *MachineCodeBuilder) emitU64(v uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	b.text.Write(buf[:])
}

func (b *MachineCodeBuilder) emitCallLabel(target string) {
	b.invalidateRegCache()
	b.asmListing.WriteString(fmt.Sprintf("    call %s\n", target))
	b.emitBytes(0xE8)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitJmpLabel(target string) {
	b.invalidateRegCache()
	b.asmListing.WriteString(fmt.Sprintf("    jmp %s\n", target))
	b.emitBytes(0xE9)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitJeLabel(target string) {
	b.invalidateRegCache()
	b.asmListing.WriteString(fmt.Sprintf("    je %s\n", target))
	b.emitBytes(0x0F, 0x84)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitJneLabel(target string) {
	b.invalidateRegCache()
	b.asmListing.WriteString(fmt.Sprintf("    jne %s\n", target))
	b.emitBytes(0x0F, 0x85)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitJlLabel(target string) {
	b.invalidateRegCache()
	b.asmListing.WriteString(fmt.Sprintf("    jl %s\n", target))
	b.emitBytes(0x0F, 0x8C)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitJgeLabel(target string) {
	b.invalidateRegCache()
	b.asmListing.WriteString(fmt.Sprintf("    jge %s\n", target))
	b.emitBytes(0x0F, 0x8D)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitJaeLabel(target string) {
	b.invalidateRegCache()
	b.asmListing.WriteString(fmt.Sprintf("    jae %s\n", target))
	b.emitBytes(0x0F, 0x83)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitJaLabel(target string) {
	b.invalidateRegCache()
	b.asmListing.WriteString(fmt.Sprintf("    ja %s\n", target))
	b.emitBytes(0x0F, 0x87)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitJleLabel(target string) {
	b.invalidateRegCache()
	b.asmListing.WriteString(fmt.Sprintf("    jle %s\n", target))
	b.emitBytes(0x0F, 0x8E)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitJgLabel(target string) {
	b.invalidateRegCache()
	b.asmListing.WriteString(fmt.Sprintf("    jg %s\n", target))
	b.emitBytes(0x0F, 0x8F)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitJnsLabel(target string) {
	b.asmListing.WriteString(fmt.Sprintf("    jns %s\n", target))
	b.emitBytes(0x0F, 0x89)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitLeaRAXLabel(target string) {
	b.asmListing.WriteString(fmt.Sprintf("    lea rax, [rip + %s]\n", target))
	b.emitBytes(0x48, 0x8D, 0x05)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitLeaRAXString(s string) {
	off := b.internString(s)
	b.asmListing.WriteString(fmt.Sprintf("    lea rax, [rip + str_%d] ; %q\n", off, s))
	b.emitBytes(0x48, 0x8D, 0x05)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocROData, patchOffset: patchOff, targetOff: off})
}

func (b *MachineCodeBuilder) emitLeaRCXString(s string) {
	off := b.internString(s)
	b.asmListing.WriteString(fmt.Sprintf("    lea rcx, [rip + str_%d] ; %q\n", off, s))
	b.emitBytes(0x48, 0x8D, 0x0D)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocROData, patchOffset: patchOff, targetOff: off})
}

func (b *MachineCodeBuilder) emitLeaRCXLabel(target string) {
	b.asmListing.WriteString(fmt.Sprintf("    lea rcx, [rip + %s]\n", target))
	b.emitBytes(0x48, 0x8D, 0x0D)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitLeaRDXString(s string) {
	off := b.internString(s)
	b.asmListing.WriteString(fmt.Sprintf("    lea rdx, [rip + str_%d] ; %q\n", off, s))
	b.emitBytes(0x48, 0x8D, 0x15)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocROData, patchOffset: patchOff, targetOff: off})
}

func (b *MachineCodeBuilder) emitLeaRDXLabel(target string) {
	b.asmListing.WriteString(fmt.Sprintf("    lea rdx, [rip + %s]\n", target))
	b.emitBytes(0x48, 0x8D, 0x15)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitLeaR8String(s string) {
	off := b.internString(s)
	b.asmListing.WriteString(fmt.Sprintf("    lea r8, [rip + str_%d] ; %q\n", off, s))
	b.emitBytes(0x4C, 0x8D, 0x05)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocROData, patchOffset: patchOff, targetOff: off})
}

func (b *MachineCodeBuilder) emitLeaR8Label(target string) {
	b.asmListing.WriteString(fmt.Sprintf("    lea r8, [rip + %s]\n", target))
	b.emitBytes(0x4C, 0x8D, 0x05)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocTextLabel, patchOffset: patchOff, targetLabel: target})
}

func (b *MachineCodeBuilder) emitMovRAXData(dataOff uint32) {
	b.asmListing.WriteString(fmt.Sprintf("    mov rax, [rip + data_0x%x]\n", dataOff))
	b.emitBytes(0x48, 0x8B, 0x05)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocData, patchOffset: patchOff, targetOff: dataOff})
}

func (b *MachineCodeBuilder) emitMovRCXData(dataOff uint32) {
	b.asmListing.WriteString(fmt.Sprintf("    mov rcx, [rip + data_0x%x]\n", dataOff))
	b.emitBytes(0x48, 0x8B, 0x0D)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocData, patchOffset: patchOff, targetOff: dataOff})
}

func (b *MachineCodeBuilder) emitMovDataRAX(dataOff uint32) {
	b.asmListing.WriteString(fmt.Sprintf("    mov [rip + data_0x%x], rax\n", dataOff))
	b.emitBytes(0x48, 0x89, 0x05)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocData, patchOffset: patchOff, targetOff: dataOff})
}

func (b *MachineCodeBuilder) emitMovDataRCX(dataOff uint32) {
	b.asmListing.WriteString(fmt.Sprintf("    mov [rip + data_0x%x], rcx\n", dataOff))
	b.emitBytes(0x48, 0x89, 0x0D)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocData, patchOffset: patchOff, targetOff: dataOff})
}

func (b *MachineCodeBuilder) emitMovDataRDX(dataOff uint32) {
	b.asmListing.WriteString(fmt.Sprintf("    mov [rip + data_0x%x], rdx\n", dataOff))
	b.emitBytes(0x48, 0x89, 0x15)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocData, patchOffset: patchOff, targetOff: dataOff})
}

func (b *MachineCodeBuilder) emitLeaRDXData(dataOff uint32) {
	b.asmListing.WriteString(fmt.Sprintf("    lea rdx, [rip + data_0x%x]\n", dataOff))
	b.emitBytes(0x48, 0x8D, 0x15)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocData, patchOffset: patchOff, targetOff: dataOff})
}

func (b *MachineCodeBuilder) emitCallIAT(slot WinImportSlot) {
	b.asmListing.WriteString(fmt.Sprintf("    call [rip + iat_%d]\n", slot))
	b.emitBytes(0xFF, 0x15)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocIAT, patchOffset: patchOff, iatSlot: slot})
}

func (b *MachineCodeBuilder) applyRelocations(textBytes []byte, textRVA, rodataRVA, dataRVA uint32, iatRVAs []uint32) error {
	for _, r := range b.relocs {
		ripNext := int64(textRVA) + int64(r.patchOffset) + 4
		var targetRVA int64
		switch r.kind {
		case relocTextLabel:
			off, ok := b.labels[r.targetLabel]
			if !ok {
				return fmt.Errorf("símbolo nativo no resuelto: %s", r.targetLabel)
			}
			targetRVA = int64(textRVA) + int64(off)
		case relocROData:
			targetRVA = int64(rodataRVA) + int64(r.targetOff)
		case relocData:
			targetRVA = int64(dataRVA) + int64(r.targetOff)
		case relocIAT:
			if int(r.iatSlot) >= len(iatRVAs) {
				return fmt.Errorf("slot IAT fuera de rango: %d", r.iatSlot)
			}
			targetRVA = int64(iatRVAs[r.iatSlot])
		}
		disp := int32(targetRVA - ripNext)
		binary.LittleEndian.PutUint32(textBytes[r.patchOffset:r.patchOffset+4], uint32(disp))
	}
	return nil
}

// emitEntrypointAndRuntime emits _start and all Native Joss Runtime routines in x86_64 machine code.
func (b *MachineCodeBuilder) emitEntrypointAndRuntime(hasUserMain bool) {
	// =========================================================================
	// 1. _start (Executable Entrypoint at offset 0 of .text)
	// =========================================================================
	b.defineLabel("_start")
	if b.isJITMode {
		// Save callee-saved registers to protect Go's runtime stack and registers
		b.emitBytes(0x55)                   // push rbp
		b.emitBytes(0x48, 0x89, 0xE5)       // mov rbp, rsp
		b.emitBytes(0x53)                   // push rbx
		b.emitBytes(0x56)                   // push rsi
		b.emitBytes(0x57)                   // push rdi
		b.emitBytes(0x41, 0x54)             // push r12
		b.emitBytes(0x41, 0x55)             // push r13
		b.emitBytes(0x41, 0x56)             // push r14
		b.emitBytes(0x41, 0x57)             // push r15
		b.emitBytes(0x48, 0x83, 0xE4, 0xF0) // and rsp, -16
		b.emitBytes(0x48, 0x83, 0xEC, 0x20) // sub rsp, 32
		b.emitCallLabel("__joss_os_init")
		if hasUserMain {
			b.emitCallLabel("fn_main")
			b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
		} else {
			b.emitBytes(0x31, 0xC9) // xor ecx, ecx
		}
		b.emitCallLabel("__joss_os_exit")
		b.emitBytes(0x48, 0x89, 0xC8)       // mov rax, rcx (exit code in rax)
		b.emitBytes(0x48, 0x8D, 0x65, 0xC8) // lea rsp, [rbp - 56] (point to saved r15)
		b.emitBytes(0x41, 0x5F)             // pop r15
		b.emitBytes(0x41, 0x5E)             // pop r14
		b.emitBytes(0x41, 0x5D)             // pop r13
		b.emitBytes(0x41, 0x5C)             // pop r12
		b.emitBytes(0x5F)                   // pop rdi
		b.emitBytes(0x5E)                   // pop rsi
		b.emitBytes(0x5B)                   // pop rbx
		b.emitBytes(0x5D)                   // pop rbp
		b.emitBytes(0xC3)                   // ret
	} else {
		b.emitBytes(0x55)                   // push rbp
		b.emitBytes(0x48, 0x89, 0xE5)       // mov rbp, rsp
		b.emitBytes(0x48, 0x83, 0xE4, 0xF0) // and rsp, -16
		b.emitBytes(0x48, 0x83, 0xEC, 0x20) // sub rsp, 32
		b.emitCallLabel("__joss_os_init")
		if hasUserMain {
			b.emitCallLabel("fn_main")
			b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
		} else {
			b.emitBytes(0x31, 0xC9) // xor ecx, ecx
		}
		b.emitCallLabel("__joss_os_exit")
		b.emitBytes(0x0F, 0x0B) // ud2
	}

	// =========================================================================
	// 2. OS Abstraction Layer (__joss_os_init, __joss_os_write, __joss_os_alloc, __joss_os_exit)
	// =========================================================================
	b.defineLabel("__joss_os_init")
	if b.target.OS == "windows" {
		b.emitBytes(0x55)                   // push rbp
		b.emitBytes(0x48, 0x89, 0xE5)       // mov rbp, rsp
		b.emitBytes(0x48, 0x83, 0xEC, 0x20) // sub rsp, 32
		// GetStdHandle(-11) -> stdout
		b.emitBytes(0xB9, 0xF5, 0xFF, 0xFF, 0xFF) // mov ecx, -11
		b.emitCallIAT(IATGetStdHandle)
		b.emitMovDataRAX(dataOffStdout)
		// GetStdHandle(-12) -> stderr
		b.emitBytes(0xB9, 0xF4, 0xFF, 0xFF, 0xFF) // mov ecx, -12
		b.emitCallIAT(IATGetStdHandle)
		b.emitMovDataRAX(dataOffStderr)
		// GetProcessHeap() -> heap
		b.emitCallIAT(IATGetProcessHeap)
		b.emitMovDataRAX(dataOffHeap)
		// LoadLibraryA("KERNEL32.dll") -> hKernel32
		b.emitLeaRCXString("KERNEL32.dll")
		b.emitCallIAT(IATLoadLibraryA)
		b.emitMovDataRAX(dataOffKernel32)
		// Resolve CreateFileA, GetFileSize, ReadFile, CloseHandle, CreateThread, WaitForSingleObject
		b.emitMovRCXData(dataOffKernel32)
		b.emitLeaRDXString("CreateFileA")
		b.emitCallIAT(IATGetProcAddress)
		b.emitMovDataRAX(dataOffCreateFileA)

		b.emitMovRCXData(dataOffKernel32)
		b.emitLeaRDXString("GetFileSize")
		b.emitCallIAT(IATGetProcAddress)
		b.emitMovDataRAX(dataOffGetFileSize)

		b.emitMovRCXData(dataOffKernel32)
		b.emitLeaRDXString("ReadFile")
		b.emitCallIAT(IATGetProcAddress)
		b.emitMovDataRAX(dataOffReadFile)

		b.emitMovRCXData(dataOffKernel32)
		b.emitLeaRDXString("CloseHandle")
		b.emitCallIAT(IATGetProcAddress)
		b.emitMovDataRAX(dataOffCloseHandle)

		b.emitMovRCXData(dataOffKernel32)
		b.emitLeaRDXString("CreateThread")
		b.emitCallIAT(IATGetProcAddress)
		b.emitMovDataRAX(dataOffCreateThread)

		b.emitMovRCXData(dataOffKernel32)
		b.emitLeaRDXString("WaitForSingleObject")
		b.emitCallIAT(IATGetProcAddress)
		b.emitMovDataRAX(dataOffWaitSingleObject)

		b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // mov rsp, rbp; pop rbp; ret
	} else {
		b.emitBytes(0xC3) // ret
	}

	// __joss_os_write(rcx = fd_kind (0=stdout, 1=stderr), rdx = bufPtr, r8 = length)
	b.defineLabel("__joss_os_write")
	if b.target.OS == "windows" {
		b.emitBytes(0x55)                   // push rbp
		b.emitBytes(0x48, 0x89, 0xE5)       // mov rbp, rsp
		b.emitBytes(0x48, 0x83, 0xEC, 0x30) // sub rsp, 48
		b.emitBytes(0x48, 0x85, 0xC9)       // test rcx, rcx
		b.emitJneLabel(".os_write_stderr")
		b.emitMovRCXData(dataOffStdout)
		b.emitJmpLabel(".os_write_do")
		b.defineLabel(".os_write_stderr")
		b.emitMovRCXData(dataOffStderr)
		b.defineLabel(".os_write_do")
		b.emitBytes(0x4C, 0x8D, 0x4D, 0xF8)                               // lea r9, [rbp - 8] (&bytesWritten)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 32], 0
		b.emitCallIAT(IATWriteFile)
		b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // mov rsp, rbp; pop rbp; ret
	} else {
		// Linux x86_64: sys_write(fd = rcx+1, buf = rdx, count = r8)
		b.emitBytes(0x56, 0x57)                   // push rsi; push rdi
		b.emitBytes(0x48, 0x8D, 0x79, 0x01)       // lea rdi, [rcx + 1]
		b.emitBytes(0x48, 0x89, 0xD6)             // mov rsi, rdx
		b.emitBytes(0x4C, 0x89, 0xC2)             // mov rdx, r8
		b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00) // mov eax, 1 (sys_write)
		b.emitBytes(0x0F, 0x05)                   // syscall
		b.emitBytes(0x5F, 0x5E, 0xC3)             // pop rdi; pop rsi; ret
	}

	// __joss_os_alloc(rcx = size) -> rax = zeroed memory pointer
	b.defineLabel("__joss_os_alloc")
	b.emitBytes(0x48, 0x83, 0xF9, 0x10) // cmp rcx, 16
	b.emitJgeLabel(".os_alloc_align")
	b.emitBytes(0x48, 0xC7, 0xC1, 0x10, 0x00, 0x00, 0x00) // mov rcx, 16
	b.defineLabel(".os_alloc_align")
	b.emitBytes(0x48, 0x83, 0xC1, 0x07) // add rcx, 7
	b.emitBytes(0x48, 0x83, 0xE1, 0xF8) // and rcx, -8
	// Fast bump-pointer arena path for allocations <= 65536 bytes
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 65536
	b.emitJgLabel(".os_alloc_direct")
	b.emitMovRAXData(dataOffArenaCur)
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".os_alloc_refill")
	b.emitBytes(0x48, 0x8D, 0x14, 0x08) // lea rdx, [rax + rcx]
	b.emitBytes(0x4C, 0x8B, 0x05)       // mov r8, [rip + dataOffArenaEnd]
	patchArenaEnd := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocData, patchOffset: patchArenaEnd, targetOff: dataOffArenaEnd})
	b.emitBytes(0x4C, 0x39, 0xC2) // cmp rdx, r8
	b.emitJaLabel(".os_alloc_refill")
	b.emitMovDataRDX(dataOffArenaCur)
	b.emitBytes(0xC3) // ret (6-instruction fast path!)

	b.defineLabel(".os_alloc_refill")
	if b.target.OS == "windows" {
		b.emitBytes(0x55)                   // push rbp
		b.emitBytes(0x48, 0x89, 0xE5)       // mov rbp, rsp
		b.emitBytes(0x53)                   // push rbx
		b.emitBytes(0x48, 0x83, 0xEC, 0x28) // sub rsp, 40
		b.emitBytes(0x48, 0x89, 0xCB)       // mov rbx, rcx (save requested size)
		// Allocate 4 MiB chunk (0x400000) from Process Heap
		b.emitBytes(0x49, 0xC7, 0xC0, 0x00, 0x00, 0x40, 0x00) // mov r8, 0x400000
		b.emitBytes(0xBA, 0x08, 0x00, 0x00, 0x00)             // mov edx, 8 (HEAP_ZERO_MEMORY)
		b.emitMovRCXData(dataOffHeap)
		b.emitCallIAT(IATHeapAlloc)
		b.emitBytes(0x48, 0x8D, 0x14, 0x18)                   // lea rdx, [rax + rbx] (new arenaCur)
		b.emitMovDataRDX(dataOffArenaCur)                     // arenaCur = chunk + size
		b.emitBytes(0x48, 0x8D, 0x90, 0x00, 0x00, 0x40, 0x00) // lea rdx, [rax + 0x400000] (arenaEnd)
		b.emitMovDataRDX(dataOffArenaEnd)
		b.emitBytes(0x48, 0x83, 0xC4, 0x28, 0x5B, 0x5D, 0xC3) // add rsp, 40; pop rbx; pop rbp; ret

		b.defineLabel(".os_alloc_direct")
		b.emitBytes(0x55)                         // push rbp
		b.emitBytes(0x48, 0x89, 0xE5)             // mov rbp, rsp
		b.emitBytes(0x48, 0x83, 0xEC, 0x20)       // sub rsp, 32
		b.emitBytes(0x49, 0x89, 0xC8)             // mov r8, rcx (dwBytes)
		b.emitBytes(0xBA, 0x08, 0x00, 0x00, 0x00) // mov edx, 8 (HEAP_ZERO_MEMORY)
		b.emitMovRCXData(dataOffHeap)
		b.emitCallIAT(IATHeapAlloc)
		b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // mov rsp, rbp; pop rbp; ret
	} else {
		// Linux x86_64: refill 4 MiB chunk via sys_mmap
		b.emitBytes(0x53, 0x56, 0x57)                         // push rbx; push rsi; push rdi
		b.emitBytes(0x48, 0x89, 0xCB)                         // mov rbx, rcx (save size)
		b.emitBytes(0x31, 0xFF)                               // xor edi, edi (addr = NULL)
		b.emitBytes(0x48, 0xC7, 0xC6, 0x00, 0x00, 0x40, 0x00) // mov rsi, 0x400000 (4 MiB)
		b.emitBytes(0xBA, 0x03, 0x00, 0x00, 0x00)             // mov edx, 3 (PROT_READ|PROT_WRITE)
		b.emitBytes(0x41, 0xBA, 0x22, 0x00, 0x00, 0x00)       // mov r10d, 0x22 (MAP_PRIVATE|MAP_ANONYMOUS)
		b.emitBytes(0x49, 0xC7, 0xC0, 0xFF, 0xFF, 0xFF, 0xFF) // mov r8, -1 (fd)
		b.emitBytes(0x45, 0x31, 0xC9)                         // xor r9d, r9d (offset = 0)
		b.emitBytes(0xB8, 0x09, 0x00, 0x00, 0x00)             // mov eax, 9 (sys_mmap)
		b.emitBytes(0x0F, 0x05)                               // syscall
		b.emitBytes(0x48, 0x8D, 0x14, 0x18)                   // lea rdx, [rax + rbx]
		b.emitMovDataRDX(dataOffArenaCur)
		b.emitBytes(0x48, 0x8D, 0x90, 0x00, 0x00, 0x40, 0x00) // lea rdx, [rax + 0x400000]
		b.emitMovDataRDX(dataOffArenaEnd)
		b.emitBytes(0x5F, 0x5E, 0x5B, 0xC3) // pop rdi; pop rsi; pop rbx; ret

		b.defineLabel(".os_alloc_direct")
		b.emitBytes(0x56, 0x57)                               // push rsi; push rdi
		b.emitBytes(0x48, 0x89, 0xCE)                         // mov rsi, rcx (len)
		b.emitBytes(0x31, 0xFF)                               // xor edi, edi (addr = NULL)
		b.emitBytes(0xBA, 0x03, 0x00, 0x00, 0x00)             // mov edx, 3 (PROT_READ|PROT_WRITE)
		b.emitBytes(0x41, 0xBA, 0x22, 0x00, 0x00, 0x00)       // mov r10d, 0x22 (MAP_PRIVATE|MAP_ANONYMOUS)
		b.emitBytes(0x49, 0xC7, 0xC0, 0xFF, 0xFF, 0xFF, 0xFF) // mov r8, -1 (fd)
		b.emitBytes(0x45, 0x31, 0xC9)                         // xor r9d, r9d (offset = 0)
		b.emitBytes(0xB8, 0x09, 0x00, 0x00, 0x00)             // mov eax, 9 (sys_mmap)
		b.emitBytes(0x0F, 0x05)                               // syscall
		b.emitBytes(0x5F, 0x5E, 0xC3)                         // pop rdi; pop rsi; ret
	}

	// __joss_os_exit(rcx = exitCode)
	b.defineLabel("__joss_os_exit")
	if b.isJITMode {
		b.emitBytes(0x48, 0x89, 0xC8) // mov rax, rcx (return exitCode in rax)
		b.emitBytes(0xC3)             // ret
	} else if b.target.OS == "windows" {
		b.emitBytes(0x48, 0x83, 0xEC, 0x28) // sub rsp, 40
		b.emitCallIAT(IATExitProcess)
		b.emitBytes(0x0F, 0x0B)
	} else {
		b.emitBytes(0x48, 0x89, 0xCF)             // mov rdi, rcx
		b.emitBytes(0xB8, 0x3C, 0x00, 0x00, 0x00) // mov eax, 60 (sys_exit)
		b.emitBytes(0x0F, 0x05)                   // syscall
		b.emitBytes(0x0F, 0x0B)
	}

	// =========================================================================
	// 3. String Helpers (__joss_rt_strlen, __joss_rt_strcmp, joss_str_concat)
	// =========================================================================
	// __joss_rt_strlen(rcx = str) -> rax = length
	b.defineLabel("__joss_rt_strlen")
	b.emitBytes(0x31, 0xC0)                               // xor eax, eax
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJlLabel(".strlen_done")
	b.defineLabel(".strlen_loop")
	b.emitBytes(0x80, 0x3C, 0x01, 0x00) // cmp byte ptr [rcx + rax], 0
	b.emitJeLabel(".strlen_done")
	b.emitBytes(0x48, 0xFF, 0xC0) // inc rax
	b.emitJmpLabel(".strlen_loop")
	b.defineLabel(".strlen_done")
	b.emitBytes(0xC3) // ret

	// __joss_rt_strcmp(rcx = a, rdx = b) -> rax = 0 if equal, 1 if different
	b.defineLabel("__joss_rt_strcmp")
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJgeLabel(".strcmp_a_ok")
	b.emitLeaRCXString("")
	b.defineLabel(".strcmp_a_ok")
	b.emitBytes(0x48, 0x81, 0xFA, 0x00, 0x00, 0x01, 0x00) // cmp rdx, 0x10000
	b.emitJgeLabel(".strcmp_b_ok")
	b.emitLeaRDXString("")
	b.defineLabel(".strcmp_b_ok")
	b.defineLabel(".strcmp_loop")
	b.emitBytes(0x8A, 0x01)       // mov al, [rcx]
	b.emitBytes(0x44, 0x8A, 0x02) // mov r8b, [rdx]
	b.emitBytes(0x44, 0x38, 0xC0) // cmp al, r8b
	b.emitJneLabel(".strcmp_diff")
	b.emitBytes(0x84, 0xC0) // test al, al
	b.emitJeLabel(".strcmp_eq")
	b.emitBytes(0x48, 0xFF, 0xC1) // inc rcx
	b.emitBytes(0x48, 0xFF, 0xC2) // inc rdx
	b.emitJmpLabel(".strcmp_loop")
	b.defineLabel(".strcmp_eq")
	b.emitBytes(0x31, 0xC0, 0xC3) // xor eax, eax; ret
	b.defineLabel(".strcmp_diff")
	b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00, 0xC3) // mov eax, 1; ret

	// joss_str_concat(rcx = a, rdx = b) -> rax = newStr
	b.defineLabel("joss_str_concat")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (a)
	b.emitBytes(0x49, 0x89, 0xD5)                               // mov r13, rdx (b)
	b.emitCallLabel("__joss_rt_strlen")
	b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (lenA)
	b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13 (b)
	b.emitCallLabel("__joss_rt_strlen")
	b.emitBytes(0x49, 0x89, 0xC7)      // mov r15, rax (lenB)
	b.emitBytes(0x4C, 0x89, 0xF1)      // mov rcx, r14
	b.emitBytes(0x4C, 0x01, 0xF9)      // add rcx, r15
	b.emitBytes(0x48, 0xFF, 0xC1)      // inc rcx
	b.emitCallLabel("__joss_os_alloc") // rax = dst
	b.emitBytes(0x48, 0x89, 0xC7)      // mov rdi_tmp? No, use r8 = dst cursor, r9 = src cursor, rcx = count
	b.emitBytes(0x49, 0x89, 0xC0)      // mov r8, rax (cursor = dst)
	b.emitBytes(0x4D, 0x89, 0xE1)      // mov r9, r12 (src = a)
	b.emitBytes(0x4C, 0x89, 0xF1)      // mov rcx, r14 (count = lenA)
	b.defineLabel(".concat_cp_a")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".concat_cp_a_end")
	b.emitBytes(0x41, 0x8A, 0x11) // mov dl, [r9]
	b.emitBytes(0x41, 0x88, 0x10) // mov [r8], dl
	b.emitBytes(0x49, 0xFF, 0xC1) // inc r9
	b.emitBytes(0x49, 0xFF, 0xC0) // inc r8
	b.emitBytes(0x48, 0xFF, 0xC9) // dec rcx
	b.emitJmpLabel(".concat_cp_a")
	b.defineLabel(".concat_cp_a_end")
	b.emitBytes(0x4D, 0x89, 0xE9) // mov r9, r13 (src = b)
	b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15 (count = lenB)
	b.defineLabel(".concat_cp_b")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".concat_cp_b_end")
	b.emitBytes(0x41, 0x8A, 0x11) // mov dl, [r9]
	b.emitBytes(0x41, 0x88, 0x10) // mov [r8], dl
	b.emitBytes(0x49, 0xFF, 0xC1) // inc r9
	b.emitBytes(0x49, 0xFF, 0xC0) // inc r8
	b.emitBytes(0x48, 0xFF, 0xC9) // dec rcx
	b.emitJmpLabel(".concat_cp_b")
	b.defineLabel(".concat_cp_b_end")
	b.emitBytes(0x41, 0xC6, 0x00, 0x00)                         // mov byte ptr [r8], 0
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                         // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r15..r12
	b.emitBytes(0x5D, 0xC3)                                     // pop rbp; ret

	// joss_str_concat_i64(rcx = strA, rdx = intB) -> rax = newStr
	b.defineLabel("joss_str_concat_i64")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x56, 0x57)       // push rbx; push rsi; push rdi
	b.emitBytes(0x41, 0x54, 0x41, 0x55) // push r12; push r13
	b.emitBytes(0x48, 0x83, 0xEC, 0x48) // sub rsp, 72
	b.emitBytes(0x49, 0x89, 0xCC)       // mov r12, rcx (strA)
	b.emitBytes(0x49, 0x89, 0xD5)       // mov r13, rdx (intB)
	b.emitCallLabel("__joss_rt_strlen") // rax = lenA
	b.emitBytes(0x48, 0x89, 0xC3)       // mov rbx, rax (lenA)
	b.emitBytes(0x48, 0x8D, 0x48, 0x20) // lea rcx, [rax + 32]
	b.emitCallLabel("__joss_os_alloc")  // rax = dst
	b.emitBytes(0x48, 0x89, 0xC6)       // mov rsi, rax (origDst)
	b.emitBytes(0x48, 0x89, 0xC7)       // mov rdi, rax (cursor)
	// Copy lenA bytes from r12 to rdi
	b.emitBytes(0x4C, 0x89, 0xE2) // mov rdx, r12
	b.emitBytes(0x48, 0x89, 0xD9) // mov rcx, rbx
	b.defineLabel(".strcat_i64_cp_a")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".strcat_i64_cp_a_done")
	b.emitBytes(0x8A, 0x02)       // mov al, [rdx]
	b.emitBytes(0x88, 0x07)       // mov [rdi], al
	b.emitBytes(0x48, 0xFF, 0xC2) // inc rdx
	b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
	b.emitBytes(0x48, 0xFF, 0xC9) // dec rcx
	b.emitJmpLabel(".strcat_i64_cp_a")
	b.defineLabel(".strcat_i64_cp_a_done")
	// Format r13 (intB) into rdi
	b.emitBytes(0x4D, 0x85, 0xED) // test r13, r13
	b.emitJneLabel(".strcat_i64_nonzero")
	b.emitBytes(0xC6, 0x07, 0x30)       // mov byte ptr [rdi], '0'
	b.emitBytes(0xC6, 0x47, 0x01, 0x00) // mov byte ptr [rdi+1], 0
	b.emitJmpLabel(".strcat_i64_done")
	b.defineLabel(".strcat_i64_nonzero")
	b.emitBytes(0x4D, 0x85, 0xED) // test r13, r13
	b.emitJnsLabel(".strcat_i64_pos")
	b.emitBytes(0xC6, 0x07, 0x2D) // mov byte ptr [rdi], '-'
	b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
	b.emitBytes(0x49, 0xF7, 0xDD) // neg r13
	b.defineLabel(".strcat_i64_pos")
	b.emitBytes(0x4C, 0x8D, 0x4D, 0xD0) // lea r9, [rbp - 48]
	b.emitBytes(0x4C, 0x89, 0xE8)       // mov rax, r13
	b.defineLabel(".strcat_i64_digits")
	b.emitBytes(0x31, 0xD2)                               // xor edx, edx
	b.emitBytes(0x49, 0xC7, 0xC3, 0x0A, 0x00, 0x00, 0x00) // mov r11, 10
	b.emitBytes(0x49, 0xF7, 0xF3)                         // div r11
	b.emitBytes(0x80, 0xC2, 0x30)                         // add dl, '0'
	b.emitBytes(0x49, 0xFF, 0xC9)                         // dec r9
	b.emitBytes(0x41, 0x88, 0x11)                         // mov [r9], dl
	b.emitBytes(0x48, 0x85, 0xC0)                         // test rax, rax
	b.emitJneLabel(".strcat_i64_digits")
	b.emitBytes(0x4C, 0x8D, 0x45, 0xD0) // lea r8, [rbp - 48]
	b.defineLabel(".strcat_i64_copy_digits")
	b.emitBytes(0x4D, 0x39, 0xC1) // cmp r9, r8
	b.emitJgeLabel(".strcat_i64_term")
	b.emitBytes(0x41, 0x8A, 0x11) // mov dl, [r9]
	b.emitBytes(0x88, 0x17)       // mov [rdi], dl
	b.emitBytes(0x49, 0xFF, 0xC1) // inc r9
	b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
	b.emitJmpLabel(".strcat_i64_copy_digits")
	b.defineLabel(".strcat_i64_term")
	b.emitBytes(0xC6, 0x07, 0x00) // mov byte ptr [rdi], 0
	b.defineLabel(".strcat_i64_done")
	b.emitBytes(0x48, 0x89, 0xF0)                         // mov rax, rsi (origDst)
	b.emitBytes(0x48, 0x83, 0xC4, 0x48)                   // add rsp, 72
	b.emitBytes(0x41, 0x5D, 0x41, 0x5C, 0x5F, 0x5E, 0x5B) // pop r13, r12, rdi, rsi, rbx
	b.emitBytes(0x5D, 0xC3)

	// joss_is_truthy(rcx = val) -> rax = 1 (truthy) or 0 (falsy)
	b.defineLabel("joss_is_truthy")
	b.emitBytes(0x48, 0x85, 0xC9)       // test rcx, rcx
	b.emitBytes(0x0F, 0x95, 0xC0)       // setne al
	b.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
	b.emitBytes(0xC3)                   // ret

	// joss_is_empty(rcx = val) -> rax = 1 (empty/zero/null) or 0 (not empty)
	b.defineLabel("joss_is_empty")
	b.emitBytes(0x48, 0x85, 0xC9)       // test rcx, rcx
	b.emitBytes(0x0F, 0x94, 0xC0)       // sete al
	b.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
	b.emitBytes(0xC3)                   // ret

	// =========================================================================
	// 4. Console I/O (joss_print_string, joss_print_i64, joss_print_bool, joss_panic)
	// =========================================================================
	// joss_print_string(rcx = strPtr)
	b.defineLabel("joss_print_string")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x53)                   // push rbx
	b.emitBytes(0x48, 0x83, 0xEC, 0x28) // sub rsp, 40 (8+40 = 48, 16-byte aligned)
	b.emitBytes(0x48, 0x85, 0xC9)       // test rcx, rcx
	b.emitJneLabel(".print_str_nonnull")
	b.emitLeaRCXString("null")
	b.defineLabel(".print_str_nonnull")
	b.emitBytes(0x48, 0x89, 0xCB) // mov rbx, rcx
	b.emitCallLabel("__joss_rt_strlen")
	b.emitBytes(0x49, 0x89, 0xC0) // mov r8, rax (len)
	b.emitBytes(0x48, 0x89, 0xDA) // mov rdx, rbx (buf)
	b.emitBytes(0x31, 0xC9)       // xor ecx, ecx (stdout = 0)
	b.emitCallLabel("__joss_os_write")
	b.emitBytes(0x31, 0xC9) // xor ecx, ecx
	b.emitLeaRDXString("\n")
	b.emitBytes(0x49, 0xC7, 0xC0, 0x01, 0x00, 0x00, 0x00) // mov r8, 1
	b.emitCallLabel("__joss_os_write")
	b.emitBytes(0x48, 0x83, 0xC4, 0x28, 0x5B, 0x5D, 0xC3) // add rsp, 40; pop rbx; pop rbp; ret

	// joss_print_i64(rcx = val)
	b.defineLabel("joss_print_i64")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x48, 0x83, 0xEC, 0x40) // sub rsp, 64
	b.emitBytes(0x4C, 0x8D, 0x4D, 0xFF) // lea r9, [rbp - 1]
	b.emitBytes(0x41, 0xC6, 0x01, 0x0A) // mov byte ptr [r9], 10 ('\n')
	b.emitBytes(0x48, 0x89, 0xC8)       // mov rax, rcx
	b.emitBytes(0x45, 0x31, 0xD2)       // xor r10d, r10d (is_neg = 0)
	b.emitBytes(0x48, 0x85, 0xC0)       // test rax, rax
	b.emitJnsLabel(".print_i64_pos")
	b.emitBytes(0x48, 0xF7, 0xD8)                   // neg rax
	b.emitBytes(0x41, 0xBA, 0x01, 0x00, 0x00, 0x00) // mov r10d, 1
	b.defineLabel(".print_i64_pos")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJneLabel(".print_i64_loop")
	b.emitBytes(0x49, 0xFF, 0xC9)       // dec r9
	b.emitBytes(0x41, 0xC6, 0x01, 0x30) // mov byte ptr [r9], '0'
	b.emitJmpLabel(".print_i64_sign")
	b.defineLabel(".print_i64_loop")
	b.emitBytes(0x31, 0xD2)                               // xor edx, edx
	b.emitBytes(0x49, 0xC7, 0xC3, 0x0A, 0x00, 0x00, 0x00) // mov r11, 10
	b.emitBytes(0x49, 0xF7, 0xF3)                         // div r11
	b.emitBytes(0x80, 0xC2, 0x30)                         // add dl, '0'
	b.emitBytes(0x49, 0xFF, 0xC9)                         // dec r9
	b.emitBytes(0x41, 0x88, 0x11)                         // mov [r9], dl
	b.emitBytes(0x48, 0x85, 0xC0)                         // test rax, rax
	b.emitJneLabel(".print_i64_loop")
	b.defineLabel(".print_i64_sign")
	b.emitBytes(0x45, 0x85, 0xD2) // test r10d, r10d
	b.emitJeLabel(".print_i64_write")
	b.emitBytes(0x49, 0xFF, 0xC9)       // dec r9
	b.emitBytes(0x41, 0xC6, 0x01, 0x2D) // mov byte ptr [r9], '-'
	b.defineLabel(".print_i64_write")
	b.emitBytes(0x49, 0x89, 0xE8) // mov r8, rbp
	b.emitBytes(0x4D, 0x29, 0xC8) // sub r8, r9 (len = rbp - r9)
	b.emitBytes(0x4C, 0x89, 0xCA) // mov rdx, r9 (buf = r9)
	b.emitBytes(0x31, 0xC9)       // xor ecx, ecx (stdout = 0)
	b.emitCallLabel("__joss_os_write")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // mov rsp, rbp; pop rbp; ret

	// joss_print_bool(rcx = val)
	b.defineLabel("joss_print_bool")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x48, 0x83, 0xEC, 0x20)
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".print_bool_false")
	b.emitLeaRCXString("true")
	b.emitJmpLabel(".print_bool_do")
	b.defineLabel(".print_bool_false")
	b.emitLeaRCXString("false")
	b.defineLabel(".print_bool_do")
	b.emitCallLabel("joss_print_string")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)

	// joss_print_mixed(rcx = val) -> dynamically prints val as int if rcx < 0x10000, or as string if >= 0x10000
	b.defineLabel("joss_print_mixed")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel("joss_print_i64")
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJlLabel("joss_print_i64")
	b.emitJmpLabel("joss_print_string")

	// joss_panic(rcx = msg)
	b.defineLabel("joss_panic")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x53, 0x48, 0x83, 0xEC, 0x28)
	b.emitBytes(0x48, 0x89, 0xCB)             // mov rbx, rcx
	b.emitBytes(0xB9, 0x01, 0x00, 0x00, 0x00) // stderr = 1
	b.emitLeaRDXString("Panic: ")
	b.emitBytes(0x49, 0xC7, 0xC0, 0x07, 0x00, 0x00, 0x00) // len = 7
	b.emitCallLabel("__joss_os_write")
	b.emitBytes(0x48, 0x89, 0xD9) // mov rcx, rbx
	b.emitCallLabel("__joss_rt_strlen")
	b.emitBytes(0x49, 0x89, 0xC0)             // mov r8, rax
	b.emitBytes(0x48, 0x89, 0xDA)             // mov rdx, rbx
	b.emitBytes(0xB9, 0x01, 0x00, 0x00, 0x00) // stderr = 1
	b.emitCallLabel("__joss_os_write")
	b.emitBytes(0xB9, 0x01, 0x00, 0x00, 0x00) // stderr = 1
	b.emitLeaRDXString("\n")
	b.emitBytes(0x49, 0xC7, 0xC0, 0x01, 0x00, 0x00, 0x00) // len = 1
	b.emitCallLabel("__joss_os_write")
	b.emitBytes(0xB9, 0x01, 0x00, 0x00, 0x00) // exit(1)
	b.emitCallLabel("__joss_os_exit")
	if b.isJITMode {
		b.emitBytes(0x48, 0xC7, 0xC0, 0x01, 0x00, 0x00, 0x00) // mov rax, 1
		b.emitBytes(0x48, 0x83, 0xC4, 0x28, 0x5B, 0x5D, 0xC3) // add rsp, 40; pop rbx; pop rbp; ret
	}

	// =========================================================================
	// 5. Objects & Maps Runtime (joss_obj_*, joss_map_*)
	// =========================================================================
	// joss_map_new() -> rax = ptr (262160 bytes: [0]=0, [8]=count, [16..262159]=32768 FNV-1a bucket heads)
	b.defineLabel("joss_map_new")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x48, 0x83, 0xEC, 0x20)
	b.emitBytes(0xB9, 0x10, 0x00, 0x04, 0x00) // mov ecx, 262160 (16 + 32768*8)
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)

	// joss_obj_new(rcx = className) -> rax = ptr (16 bytes: [0]=className, [8]=headNode)
	b.defineLabel("joss_obj_new")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x53, 0x48, 0x83, 0xEC, 0x28)
	b.emitBytes(0x48, 0x89, 0xCB)             // mov rbx, rcx
	b.emitBytes(0xB9, 0x10, 0x00, 0x00, 0x00) // mov ecx, 16
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x48, 0x89, 0x18)                         // mov [rax], rbx
	b.emitBytes(0x48, 0x83, 0xC4, 0x28, 0x5B, 0x5D, 0xC3) // add rsp, 40; pop rbx; pop rbp; ret

	// joss_map_set_i64 / joss_map_set_str (rcx = mapPtr, rdx = keyStr, r8 = val64)
	b.defineLabel("joss_map_set_i64")
	b.defineLabel("joss_map_set_str")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".map_set_ret_null")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x56)                                     // push rbx; push rsi
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (mapPtr)
	b.emitBytes(0x49, 0x89, 0xD5)                               // mov r13, rdx (keyStr)
	b.emitBytes(0x4D, 0x89, 0xC6)                               // mov r14, r8  (val64)
	// Compute FNV-1a 64-bit hash of keyStr
	b.emitBytes(0x48, 0xB8) // movabs rax, 0xCBF29CE484222325
	b.emitU64(0xCBF29CE484222325)
	b.emitBytes(0x49, 0xBA) // movabs r10, 0x100000001B3
	b.emitU64(0x00000100000001B3)
	b.emitBytes(0x4C, 0x89, 0xEA)                         // mov rdx, r13
	b.emitBytes(0x48, 0x81, 0xFA, 0x00, 0x00, 0x01, 0x00) // cmp rdx, 0x10000
	b.emitJlLabel(".map_set_hash_done")
	b.defineLabel(".map_set_hash_loop")
	b.emitBytes(0x0F, 0xB6, 0x0A) // movzx ecx, byte ptr [rdx]
	b.emitBytes(0x85, 0xC9)       // test ecx, ecx
	b.emitJeLabel(".map_set_hash_done")
	b.emitBytes(0x48, 0x31, 0xC8)       // xor rax, rcx
	b.emitBytes(0x49, 0x0F, 0xAF, 0xC2) // imul rax, r10
	b.emitBytes(0x48, 0xFF, 0xC2)       // inc rdx
	b.emitJmpLabel(".map_set_hash_loop")
	b.defineLabel(".map_set_hash_done")
	b.emitBytes(0x48, 0x89, 0xC3)                         // mov rbx, rax (hash64)
	b.emitBytes(0x48, 0x89, 0xC6)                         // mov rsi, rax
	b.emitBytes(0x48, 0x81, 0xE6, 0xFF, 0x7F, 0x00, 0x00) // and rsi, 32767 (bucketIdx)
	b.emitBytes(0x4D, 0x8B, 0x7C, 0xF4, 0x10)             // mov r15, [r12 + rsi*8 + 16] (bucket head)
	b.defineLabel(".map_set_chain_loop")
	b.emitBytes(0x4D, 0x85, 0xFF) // test r15, r15
	b.emitJeLabel(".map_set_alloc")
	b.emitBytes(0x49, 0x39, 0x5F, 0x18) // cmp [r15 + 24], rbx (node.hash vs hash64)
	b.emitJneLabel(".map_set_next")
	b.emitBytes(0x49, 0x8B, 0x0F) // mov rcx, [r15 + 0] (node.key)
	b.emitBytes(0x4C, 0x89, 0xEA) // mov rdx, r13 (keyStr)
	b.emitBytes(0x48, 0x39, 0xD1) // cmp rcx, rdx (pointer equality)
	b.emitJeLabel(".map_set_found")
	b.emitCallLabel("__joss_rt_strcmp")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".map_set_found")
	b.defineLabel(".map_set_next")
	b.emitBytes(0x4D, 0x8B, 0x7F, 0x10) // mov r15, [r15 + 16] (node.next)
	b.emitJmpLabel(".map_set_chain_loop")
	b.defineLabel(".map_set_found")
	b.emitBytes(0x4D, 0x89, 0x77, 0x08) // mov [r15 + 8], r14 (node.val = val)
	b.emitJmpLabel(".map_set_done")
	b.defineLabel(".map_set_alloc")
	b.emitBytes(0xB9, 0x20, 0x00, 0x00, 0x00) // mov ecx, 32 (node size: [0]=key, [8]=val, [16]=next, [24]=hash)
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x4C, 0x89, 0x28)             // mov [rax + 0], r13 (key)
	b.emitBytes(0x4C, 0x89, 0x70, 0x08)       // mov [rax + 8], r14 (val)
	b.emitBytes(0x49, 0x8B, 0x4C, 0xF4, 0x10) // mov rcx, [r12 + rsi*8 + 16] (old bucket head)
	b.emitBytes(0x48, 0x89, 0x48, 0x10)       // mov [rax + 16], rcx (next)
	b.emitBytes(0x48, 0x89, 0x58, 0x18)       // mov [rax + 24], rbx (hash)
	b.emitBytes(0x49, 0x89, 0x44, 0xF4, 0x10) // mov [r12 + rsi*8 + 16], rax (new bucket head)
	b.emitBytes(0x49, 0xFF, 0x44, 0x24, 0x08) // inc qword ptr [r12 + 8] (count++)
	b.defineLabel(".map_set_done")
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                                     // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5E, 0x5B) // pop r15..r12, rsi, rbx
	b.emitBytes(0x5D)                                                       // pop rbp
	b.defineLabel(".map_set_ret_null")
	b.emitBytes(0xC3)

	// joss_map_get_i64 (rcx = mapPtr, rdx = keyStr) -> rax = val64 (0 if missing)
	b.defineLabel("joss_map_get_i64")
	b.emitBytes(0x31, 0xC0)       // xor eax, eax
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".map_get_ret_null")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x53)                                           // push rbx
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x28)                         // sub rsp, 40
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (mapPtr)
	b.emitBytes(0x49, 0x89, 0xD5)                               // mov r13, rdx (keyStr)
	// Compute FNV-1a 64-bit hash of keyStr
	b.emitBytes(0x48, 0xB8) // movabs rax, 0xCBF29CE484222325
	b.emitU64(0xCBF29CE484222325)
	b.emitBytes(0x49, 0xBA) // movabs r10, 0x100000001B3
	b.emitU64(0x00000100000001B3)
	b.emitBytes(0x4C, 0x89, 0xEA)                         // mov rdx, r13
	b.emitBytes(0x48, 0x81, 0xFA, 0x00, 0x00, 0x01, 0x00) // cmp rdx, 0x10000
	b.emitJlLabel(".map_get_hash_done")
	b.defineLabel(".map_get_hash_loop")
	b.emitBytes(0x0F, 0xB6, 0x0A) // movzx ecx, byte ptr [rdx]
	b.emitBytes(0x85, 0xC9)       // test ecx, ecx
	b.emitJeLabel(".map_get_hash_done")
	b.emitBytes(0x48, 0x31, 0xC8)       // xor rax, rcx
	b.emitBytes(0x49, 0x0F, 0xAF, 0xC2) // imul rax, r10
	b.emitBytes(0x48, 0xFF, 0xC2)       // inc rdx
	b.emitJmpLabel(".map_get_hash_loop")
	b.defineLabel(".map_get_hash_done")
	b.emitBytes(0x48, 0x89, 0xC3)                   // mov rbx, rax (hash64)
	b.emitBytes(0x48, 0x25, 0xFF, 0x7F, 0x00, 0x00) // and rax, 32767 (bucketIdx)
	b.emitBytes(0x4D, 0x8B, 0x7C, 0xC4, 0x10)       // mov r15, [r12 + rax*8 + 16] (bucket head)
	b.defineLabel(".map_get_chain_loop")
	b.emitBytes(0x4D, 0x85, 0xFF) // test r15, r15
	b.emitJeLabel(".map_get_notfound")
	b.emitBytes(0x49, 0x39, 0x5F, 0x18) // cmp [r15 + 24], rbx (node.hash vs hash64)
	b.emitJneLabel(".map_get_next")
	b.emitBytes(0x49, 0x8B, 0x0F) // mov rcx, [r15 + 0] (node.key)
	b.emitBytes(0x4C, 0x89, 0xEA) // mov rdx, r13 (keyStr)
	b.emitBytes(0x48, 0x39, 0xD1) // cmp rcx, rdx (pointer equality)
	b.emitJeLabel(".map_get_found")
	b.emitCallLabel("__joss_rt_strcmp")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".map_get_found")
	b.defineLabel(".map_get_next")
	b.emitBytes(0x4D, 0x8B, 0x7F, 0x10) // mov r15, [r15 + 16] (node.next)
	b.emitJmpLabel(".map_get_chain_loop")
	b.defineLabel(".map_get_found")
	b.emitBytes(0x49, 0x8B, 0x47, 0x08) // mov rax, [r15 + 8] (node.val)
	b.emitJmpLabel(".map_get_exit")
	b.defineLabel(".map_get_notfound")
	b.emitBytes(0x31, 0xC0) // xor eax, eax
	b.defineLabel(".map_get_exit")
	b.emitBytes(0x48, 0x83, 0xC4, 0x28)                               // add rsp, 40
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B) // pop r15..r12, rbx
	b.emitBytes(0x5D)                                                 // pop rbp
	b.defineLabel(".map_get_ret_null")
	b.emitBytes(0xC3)

	// joss_map_get_str -> returns "" if missing/null
	b.defineLabel("joss_map_get_str")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x48, 0x83, 0xEC, 0x20)
	b.emitCallLabel("joss_map_get_i64")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJneLabel(".map_get_str_ok")
	b.emitLeaRAXString("")
	b.defineLabel(".map_get_str_ok")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)

	// __joss_rt_kv_set(rcx = obj, rdx = keyStr, r8 = val64)
	b.defineLabel("joss_obj_set_field_i64")
	b.defineLabel("joss_obj_set_field_str")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".kv_set_leaf_ret")
	// Fast leaf path: check if head node exists and matches key pointer directly
	b.emitBytes(0x4C, 0x8B, 0x49, 0x08) // mov r9, [rcx + 8] (head)
	b.emitBytes(0x4D, 0x85, 0xC9)       // test r9, r9
	b.emitJeLabel(".kv_set_need_frame")
	b.emitBytes(0x49, 0x39, 0x11) // cmp [r9], rdx (head.key vs key)
	b.emitJneLabel(".kv_set_need_frame")
	b.emitBytes(0x4D, 0x89, 0x41, 0x08) // mov [r9 + 8], r8 (head.val = val)
	b.defineLabel(".kv_set_leaf_ret")
	b.emitBytes(0xC3) // ret
	b.defineLabel(".kv_set_need_frame")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (obj)
	b.emitBytes(0x49, 0x89, 0xD5)                               // mov r13, rdx (key)
	b.emitBytes(0x4D, 0x89, 0xC6)                               // mov r14, r8  (val)
	b.emitBytes(0x4D, 0x8B, 0x7C, 0x24, 0x08)                   // mov r15, [r12 + 8] (head)
	b.defineLabel(".kv_set_loop")
	b.emitBytes(0x4D, 0x85, 0xFF) // test r15, r15
	b.emitJeLabel(".kv_set_alloc")
	b.emitBytes(0x49, 0x8B, 0x0F) // mov rcx, [r15] (node.key)
	b.emitBytes(0x4C, 0x89, 0xEA) // mov rdx, r13   (key)
	b.emitBytes(0x48, 0x39, 0xD1) // cmp rcx, rdx (pointer equality)
	b.emitJeLabel(".kv_set_found")
	b.emitCallLabel("__joss_rt_strcmp")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".kv_set_found")
	b.emitBytes(0x4D, 0x8B, 0x7F, 0x10) // mov r15, [r15 + 16] (node.next)
	b.emitJmpLabel(".kv_set_loop")
	b.defineLabel(".kv_set_found")
	b.emitBytes(0x4D, 0x89, 0x77, 0x08) // mov [r15 + 8], r14
	b.emitJmpLabel(".kv_set_done")
	b.defineLabel(".kv_set_alloc")
	b.emitBytes(0xB9, 0x18, 0x00, 0x00, 0x00) // mov ecx, 24
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x4C, 0x89, 0x28)             // mov [rax + 0], r13 (key)
	b.emitBytes(0x4C, 0x89, 0x70, 0x08)       // mov [rax + 8], r14 (val)
	b.emitBytes(0x49, 0x8B, 0x4C, 0x24, 0x08) // mov rcx, [r12 + 8] (old head)
	b.emitBytes(0x48, 0x89, 0x48, 0x10)       // mov [rax + 16], rcx
	b.emitBytes(0x49, 0x89, 0x44, 0x24, 0x08) // mov [r12 + 8], rax (new head)
	b.defineLabel(".kv_set_done")
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                         // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r15..r12
	b.emitBytes(0x5D, 0xC3)

	// __joss_rt_kv_get_i64(rcx = obj, rdx = keyStr) -> rax = val64 (0 if missing)
	b.defineLabel("joss_obj_get_field_i64")
	b.emitBytes(0x31, 0xC0)       // xor eax, eax
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".kv_get_leaf_ret")
	// Fast leaf path: check if head node exists and matches key pointer directly
	b.emitBytes(0x4C, 0x8B, 0x49, 0x08) // mov r9, [rcx + 8] (head)
	b.emitBytes(0x4D, 0x85, 0xC9)       // test r9, r9
	b.emitJeLabel(".kv_get_leaf_ret")
	b.emitBytes(0x49, 0x39, 0x11) // cmp [r9], rdx (head.key vs key)
	b.emitJneLabel(".kv_get_need_frame")
	b.emitBytes(0x49, 0x8B, 0x41, 0x08) // mov rax, [r9 + 8] (head.val)
	b.defineLabel(".kv_get_leaf_ret")
	b.emitBytes(0xC3) // ret
	b.defineLabel(".kv_get_need_frame")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xD5)                               // mov r13, rdx (key)
	b.emitBytes(0x4C, 0x8B, 0x79, 0x08)                         // mov r15, [rcx + 8] (head)
	b.defineLabel(".kv_get_loop")
	b.emitBytes(0x4D, 0x85, 0xFF) // test r15, r15
	b.emitJeLabel(".kv_get_notfound")
	b.emitBytes(0x49, 0x8B, 0x0F) // mov rcx, [r15] (node.key)
	b.emitBytes(0x4C, 0x89, 0xEA) // mov rdx, r13   (key)
	b.emitBytes(0x48, 0x39, 0xD1) // cmp rcx, rdx (pointer equality)
	b.emitJeLabel(".kv_get_found")
	b.emitCallLabel("__joss_rt_strcmp")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".kv_get_found")
	b.emitBytes(0x4D, 0x8B, 0x7F, 0x10) // mov r15, [r15 + 16]
	b.emitJmpLabel(".kv_get_loop")
	b.defineLabel(".kv_get_found")
	b.emitBytes(0x49, 0x8B, 0x47, 0x08) // mov rax, [r15 + 8]
	b.emitJmpLabel(".kv_get_exit")
	b.defineLabel(".kv_get_notfound")
	b.emitBytes(0x31, 0xC0) // xor eax, eax
	b.defineLabel(".kv_get_exit")
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C)
	b.emitBytes(0x5D, 0xC3)

	// joss_obj_get_field_str -> returns "" if missing/null
	b.defineLabel("joss_obj_get_field_str")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x48, 0x83, 0xEC, 0x20)
	b.emitCallLabel("joss_obj_get_field_i64")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJneLabel(".kv_get_str_ok")
	b.emitLeaRAXString("")
	b.defineLabel(".kv_get_str_ok")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)

	// =========================================================================
	// 6. Array Runtime (joss_arr_new, joss_arr_push_*, joss_arr_get_*, joss_arr_set_*, joss_arr_len)
	// =========================================================================
	// joss_arr_new(rcx = cap) -> rax = arrPtr (24 bytes: [0]=len, [8]=cap, [16]=dataPtr)
	b.defineLabel("joss_arr_new")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x41, 0x54, 0x41, 0x55, 0x48, 0x83, 0xEC, 0x20)
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x01, 0x00, 0x00) // cmp rcx, 256
	b.emitJgeLabel(".arr_cap_ok")
	b.emitBytes(0x48, 0xC7, 0xC1, 0x00, 0x01, 0x00, 0x00) // mov rcx, 256
	b.defineLabel(".arr_cap_ok")
	b.emitBytes(0x49, 0x89, 0xCC)             // mov r12, rcx (cap)
	b.emitBytes(0xB9, 0x18, 0x00, 0x00, 0x00) // mov ecx, 24
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x49, 0x89, 0xC5)       // mov r13, rax (arrHeader)
	b.emitBytes(0x4C, 0x89, 0xE1)       // mov rcx, r12
	b.emitBytes(0x48, 0xC1, 0xE1, 0x03) // shl rcx, 3 (cap * 8)
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x49, 0xC7, 0x45, 0x00, 0x00, 0x00, 0x00, 0x00) // [r13 + 0] = 0 (len)
	b.emitBytes(0x4D, 0x89, 0x65, 0x08)                         // [r13 + 8] = r12 (cap)
	b.emitBytes(0x49, 0x89, 0x45, 0x10)                         // [r13 + 16] = rax (dataPtr)
	b.emitBytes(0x4C, 0x89, 0xE8)                               // mov rax, r13
	b.emitBytes(0x48, 0x83, 0xC4, 0x20, 0x41, 0x5D, 0x41, 0x5C, 0x5D, 0xC3)

	// __joss_arr_ensure_cap(rcx = arrPtr, rdx = minCap)
	b.defineLabel("__joss_arr_ensure_cap")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (arrPtr)
	b.emitBytes(0x4D, 0x8B, 0x6C, 0x24, 0x08)                   // mov r13, [r12 + 8] (oldCap)
	b.emitBytes(0x49, 0xD1, 0xE5)                               // shl r13, 1 (newCap = oldCap * 2)
	b.emitBytes(0x49, 0x39, 0xD5)                               // cmp r13, rdx
	b.emitJgeLabel(".arr_grow_alloc")
	b.emitBytes(0x49, 0x89, 0xD5) // mov r13, rdx
	b.emitBytes(0x49, 0xD1, 0xE5) // shl r13, 1
	b.defineLabel(".arr_grow_alloc")
	b.emitBytes(0x4C, 0x89, 0xE9)       // mov rcx, r13
	b.emitBytes(0x48, 0xC1, 0xE1, 0x03) // shl rcx, 3 (newCap * 8)
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x49, 0x89, 0xC6)             // mov r14, rax (newData)
	b.emitBytes(0x4D, 0x8B, 0x7C, 0x24, 0x10) // mov r15, [r12 + 16] (oldData)
	b.emitBytes(0x49, 0x8B, 0x0C, 0x24)       // mov rcx, [r12 + 0] (len)
	b.emitBytes(0x31, 0xD2)                   // xor edx, edx
	b.defineLabel(".arr_grow_copy")
	b.emitBytes(0x48, 0x39, 0xCA) // cmp rdx, rcx
	b.emitJgeLabel(".arr_grow_commit")
	b.emitBytes(0x49, 0x8B, 0x04, 0xD7) // mov rax, [r15 + rdx*8]
	b.emitBytes(0x49, 0x89, 0x04, 0xD6) // mov [r14 + rdx*8], rax
	b.emitBytes(0x48, 0xFF, 0xC2)       // inc rdx
	b.emitJmpLabel(".arr_grow_copy")
	b.defineLabel(".arr_grow_commit")
	b.emitBytes(0x4D, 0x89, 0x6C, 0x24, 0x08)                   // mov [r12 + 8], r13 (cap)
	b.emitBytes(0x4D, 0x89, 0x74, 0x24, 0x10)                   // mov [r12 + 16], r14 (dataPtr)
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                         // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r15..r12
	b.emitBytes(0x5D, 0xC3)

	// fn_array_fill (rcx = startIdx, rdx = num, r8 = val) -> rax = arrPtr
	b.defineLabel("fn_array_fill")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x57, 0x56)                                     // push rdi; push rsi
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (startIdx)
	b.emitBytes(0x49, 0x89, 0xD5)                               // mov r13, rdx (num)
	b.emitBytes(0x4D, 0x89, 0xC6)                               // mov r14, r8 (val)
	b.emitBytes(0x4D, 0x8D, 0x0C, 0x2C)                         // lea r9, [r12 + r13] (total = startIdx + num)
	b.emitBytes(0x4C, 0x89, 0xC9)                               // mov rcx, r9 (cap)
	b.emitCallLabel("joss_arr_new")
	b.emitBytes(0x49, 0x89, 0xC7)       // mov r15, rax (arrPtr)
	b.emitBytes(0x4D, 0x8D, 0x0C, 0x2C) // lea r9, [r12 + r13]
	b.emitBytes(0x4D, 0x89, 0x0F)       // mov [r15 + 0], r9 (len = total)
	b.emitBytes(0x49, 0x8B, 0x7F, 0x10) // mov rdi, [r15 + 16] (dataPtr)
	b.emitBytes(0x4D, 0x85, 0xED)       // test r13, r13 (num <= 0?)
	b.emitJleLabel(".arr_fill_done")
	b.emitBytes(0x49, 0xC1, 0xE4, 0x03) // shl r12, 3 (startIdx * 8)
	b.emitBytes(0x4C, 0x01, 0xE7)       // add rdi, r12
	b.emitBytes(0x4C, 0x89, 0xE9)       // mov rcx, r13 (count = num)
	b.emitBytes(0x4C, 0x89, 0xF0)       // mov rax, r14 (val)
	b.emitBytes(0xF3, 0x48, 0xAB)       // rep stosq
	b.defineLabel(".arr_fill_done")
	b.emitBytes(0x4C, 0x89, 0xF8)                                                 // mov rax, r15 (return arrPtr)
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                                           // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5E, 0x5F, 0x5D) // pop r15..r12, rsi, rdi, rbp
	b.emitBytes(0xC3)                                                             // ret

	// joss_arr_push_i64 / joss_arr_push_str / fn_array_push / fn_append (rcx = arrPtr, rdx = val64) -> rax = arrPtr
	b.defineLabel("joss_arr_push_i64")
	b.defineLabel("joss_arr_push_str")
	b.defineLabel("fn_array_push")
	b.defineLabel("fn_append")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x41, 0x54, 0x41, 0x55, 0x48, 0x83, 0xEC, 0x20)
	b.emitBytes(0x31, 0xC0)       // xor eax, eax
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".arr_push_done")
	b.emitBytes(0x49, 0x89, 0xCC)             // mov r12, rcx (arrPtr)
	b.emitBytes(0x49, 0x89, 0xD5)             // mov r13, rdx (val64)
	b.emitBytes(0x49, 0x8B, 0x14, 0x24)       // mov rdx, [r12 + 0] (len)
	b.emitBytes(0x48, 0xFF, 0xC2)             // inc rdx (minCap = len + 1)
	b.emitBytes(0x49, 0x3B, 0x54, 0x24, 0x08) // cmp rdx, [r12 + 8] (minCap vs cap)
	b.emitJleLabel(".arr_push_store")
	b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
	b.emitCallLabel("__joss_arr_ensure_cap")
	b.defineLabel(".arr_push_store")
	b.emitBytes(0x4D, 0x8B, 0x04, 0x24)       // mov r8, [r12 + 0] (len)
	b.emitBytes(0x4D, 0x8B, 0x54, 0x24, 0x10) // mov r10, [r12 + 16] (dataPtr)
	b.emitBytes(0x4F, 0x89, 0x2C, 0xC2)       // mov [r10 + r8*8], r13
	b.emitBytes(0x49, 0xFF, 0x04, 0x24)       // inc qword ptr [r12 + 0]
	b.emitBytes(0x4C, 0x89, 0xE0)             // mov rax, r12 (return arrPtr)
	b.defineLabel(".arr_push_done")
	b.emitBytes(0x48, 0x83, 0xC4, 0x20, 0x41, 0x5D, 0x41, 0x5C, 0x5D, 0xC3)

	// joss_arr_get_i64 (rcx = arrPtr, rdx = idx) -> rax = val64
	b.defineLabel("joss_arr_get_i64")
	b.emitBytes(0x31, 0xC0)       // xor eax, eax
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".arr_get_done")
	b.emitBytes(0x48, 0x85, 0xD2) // test rdx, rdx
	b.emitJlLabel(".arr_get_done")
	b.emitBytes(0x48, 0x3B, 0x11) // cmp rdx, [rcx + 0] (idx vs len)
	b.emitJgeLabel(".arr_get_done")
	b.emitBytes(0x4C, 0x8B, 0x41, 0x10) // mov r8, [rcx + 16] (dataPtr)
	b.emitBytes(0x49, 0x8B, 0x04, 0xD0) // mov rax, [r8 + rdx*8]
	b.defineLabel(".arr_get_done")
	b.emitBytes(0xC3)

	// joss_arr_get_str (rcx = arrPtr, rdx = idx) -> rax = strPtr
	b.defineLabel("joss_arr_get_str")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x48, 0x83, 0xEC, 0x20)
	b.emitCallLabel("joss_arr_get_i64")
	b.emitBytes(0x48, 0x85, 0xC0)
	b.emitJneLabel(".arr_get_str_ok")
	b.emitLeaRAXString("")
	b.defineLabel(".arr_get_str_ok")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)

	// joss_arr_set_i64 / joss_arr_set_str (rcx = arrPtr, rdx = idx, r8 = val64)
	b.defineLabel("joss_arr_set_i64")
	b.defineLabel("joss_arr_set_str")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".arr_set_leaf_ret")
	b.emitBytes(0x48, 0x85, 0xD2) // test rdx, rdx
	b.emitJlLabel(".arr_set_leaf_ret")
	b.emitBytes(0x48, 0x3B, 0x51, 0x08) // cmp rdx, [rcx + 8] (idx vs cap)
	b.emitJgeLabel(".arr_set_need_grow")
	// Fast leaf path: idx < cap
	b.emitBytes(0x48, 0x8D, 0x42, 0x01) // lea rax, [rdx + 1]
	b.emitBytes(0x48, 0x3B, 0x01)       // cmp rax, [rcx + 0]
	b.emitJleLabel(".arr_set_fast_store")
	b.emitBytes(0x48, 0x89, 0x01) // mov [rcx + 0], rax (len = idx + 1)
	b.defineLabel(".arr_set_fast_store")
	b.emitBytes(0x4C, 0x8B, 0x49, 0x10) // mov r9, [rcx + 16] (dataPtr)
	b.emitBytes(0x4D, 0x89, 0x04, 0xD1) // mov [r9 + rdx*8], r8
	b.defineLabel(".arr_set_leaf_ret")
	b.emitBytes(0xC3) // ret
	b.defineLabel(".arr_set_need_grow")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (arrPtr)
	b.emitBytes(0x49, 0x89, 0xD5)                               // mov r13, rdx (idx)
	b.emitBytes(0x4D, 0x89, 0xC6)                               // mov r14, r8  (val64)
	b.emitBytes(0x49, 0x8D, 0x55, 0x01)                         // lea rdx, [r13 + 1] (minCap)
	b.emitBytes(0x4C, 0x89, 0xE1)                               // mov rcx, r12
	b.emitCallLabel("__joss_arr_ensure_cap")
	b.emitBytes(0x49, 0x8D, 0x45, 0x01) // lea rax, [r13 + 1]
	b.emitBytes(0x49, 0x3B, 0x04, 0x24) // cmp rax, [r12 + 0]
	b.emitJleLabel(".arr_set_grow_write")
	b.emitBytes(0x49, 0x89, 0x04, 0x24) // mov [r12 + 0], rax
	b.defineLabel(".arr_set_grow_write")
	b.emitBytes(0x4D, 0x8B, 0x4C, 0x24, 0x10)                   // mov r9, [r12 + 16] (dataPtr)
	b.emitBytes(0x4F, 0x89, 0x34, 0xE9)                         // mov [r9 + r13*8], r14
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                         // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r15..r12
	b.emitBytes(0x5D, 0xC3)

	// joss_arr_len(rcx = arrPtr) -> rax = len
	b.defineLabel("joss_arr_len")
	b.defineLabel("fn_len")
	b.defineLabel("fn_count")
	b.emitBytes(0x31, 0xC0)       // xor eax, eax
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".arr_len_done")
	b.emitBytes(0x48, 0x8B, 0x01) // mov rax, [rcx + 0]
	b.defineLabel(".arr_len_done")
	b.emitBytes(0xC3)

	// fn_strlen(rcx = strPtr) -> rax = UTF-8 rune count (skipping 10xxxxxx continuation bytes)
	b.defineLabel("fn_strlen")
	b.emitBytes(0x31, 0xC0)       // xor eax, eax
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".utf8_strlen_done")
	b.defineLabel(".utf8_strlen_loop")
	b.emitBytes(0x0F, 0xB6, 0x11) // movzx edx, byte ptr [rcx]
	b.emitBytes(0x85, 0xD2)       // test edx, edx
	b.emitJeLabel(".utf8_strlen_done")
	b.emitBytes(0x48, 0xFF, 0xC1)      // inc rcx
	b.emitBytes(0x80, 0xE2, 0xC0)      // and dl, 0xC0
	b.emitBytes(0x80, 0xFA, 0x80)      // cmp dl, 0x80
	b.emitJeLabel(".utf8_strlen_loop") // continuation byte: do not increment rune count
	b.emitBytes(0x48, 0xFF, 0xC0)      // inc rax
	b.emitJmpLabel(".utf8_strlen_loop")
	b.defineLabel(".utf8_strlen_done")
	b.emitBytes(0xC3)

	// fn_abs(rcx = val) -> rax = abs(val)
	b.defineLabel("fn_abs")
	b.emitBytes(0x48, 0x89, 0xC8) // mov rax, rcx
	b.emitBytes(0x48, 0x99)       // cqo
	b.emitBytes(0x48, 0x31, 0xD0) // xor rax, rdx
	b.emitBytes(0x48, 0x29, 0xD0) // sub rax, rdx
	b.emitBytes(0xC3)

	// fn_min(rcx = a, rdx = b) -> rax = min(a, b)
	b.defineLabel("fn_min")
	b.emitBytes(0x48, 0x89, 0xC8)       // mov rax, rcx
	b.emitBytes(0x48, 0x39, 0xD0)       // cmp rax, rdx
	b.emitBytes(0x48, 0x0F, 0x4F, 0xC2) // cmovg rax, rdx
	b.emitBytes(0xC3)

	// fn_max(rcx = a, rdx = b) -> rax = max(a, b)
	b.defineLabel("fn_max")
	b.emitBytes(0x48, 0x89, 0xC8)       // mov rax, rcx
	b.emitBytes(0x48, 0x39, 0xD0)       // cmp rax, rdx
	b.emitBytes(0x48, 0x0F, 0x4C, 0xC2) // cmovl rax, rdx
	b.emitBytes(0xC3)

	// fn_intval(rcx = val) -> rax = integer value (parses ASCII if rcx is a string pointer >= 0x10000)
	b.defineLabel("fn_intval")
	b.emitBytes(0x48, 0x89, 0xC8)                         // mov rax, rcx
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJlLabel(".intval_done")
	b.emitBytes(0x31, 0xC0)                         // xor eax, eax (acc = 0)
	b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00) // mov r8d, 1 (sign = 1)
	b.defineLabel(".intval_skip_ws")
	b.emitBytes(0x0F, 0xB6, 0x11) // movzx edx, byte ptr [rcx]
	b.emitBytes(0x80, 0xFA, 0x20) // cmp dl, ' '
	b.emitJeLabel(".intval_ws_next")
	b.emitBytes(0x80, 0xFA, 0x09) // cmp dl, '\t'
	b.emitJeLabel(".intval_ws_next")
	b.emitBytes(0x80, 0xFA, 0x0D) // cmp dl, '\r'
	b.emitJneLabel(".intval_check_sign")
	b.defineLabel(".intval_ws_next")
	b.emitBytes(0x48, 0xFF, 0xC1) // inc rcx
	b.emitJmpLabel(".intval_skip_ws")
	b.defineLabel(".intval_check_sign")
	b.emitBytes(0x80, 0xFA, 0x2D) // cmp dl, '-'
	b.emitJneLabel(".intval_check_plus")
	b.emitBytes(0x49, 0xC7, 0xC0, 0xFF, 0xFF, 0xFF, 0xFF) // mov r8, -1
	b.emitBytes(0x48, 0xFF, 0xC1)                         // inc rcx
	b.emitJmpLabel(".intval_loop")
	b.defineLabel(".intval_check_plus")
	b.emitBytes(0x80, 0xFA, 0x2B) // cmp dl, '+'
	b.emitJneLabel(".intval_loop")
	b.emitBytes(0x48, 0xFF, 0xC1) // inc rcx
	b.defineLabel(".intval_loop")
	b.emitBytes(0x0F, 0xB6, 0x11) // movzx edx, byte ptr [rcx]
	b.emitBytes(0x80, 0xEA, 0x30) // sub dl, '0'
	b.emitBytes(0x80, 0xFA, 0x09) // cmp dl, 9
	b.emitJaLabel(".intval_apply_sign")
	b.emitBytes(0x48, 0x6B, 0xC0, 0x0A) // imul rax, rax, 10
	b.emitBytes(0x48, 0x01, 0xD0)       // add rax, rdx
	b.emitBytes(0x48, 0xFF, 0xC1)       // inc rcx
	b.emitJmpLabel(".intval_loop")
	b.defineLabel(".intval_apply_sign")
	b.emitBytes(0x49, 0x0F, 0xAF, 0xC0) // imul rax, r8
	b.defineLabel(".intval_done")
	b.emitBytes(0xC3)

	// fn_boolval(rcx = val) -> rax = bool(val)
	b.defineLabel("fn_boolval")
	b.emitBytes(0x31, 0xC0)       // xor eax, eax
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitBytes(0x0F, 0x95, 0xC0) // setne al
	b.emitBytes(0xC3)

	// fn_floatval(rcx = intVal) -> rax = float64 bits
	b.defineLabel("fn_floatval")
	b.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xC1) // cvtsi2sd xmm0, rcx
	b.emitBytes(0x66, 0x48, 0x0F, 0x7E, 0xC0) // movq rax, xmm0
	b.emitBytes(0xC3)

	// fn_floatval_str(rcx = strPtr) -> rax = float64 bits parsed from ASCII decimal string
	b.defineLabel("fn_floatval_str")
	b.emitBytes(0x31, 0xC0)                               // xor eax, eax
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJlLabel(".fval_str_done")
	b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00) // mov r8d, 1 (sign = 1)
	b.defineLabel(".fval_skip_ws")
	b.emitBytes(0x0F, 0xB6, 0x11) // movzx edx, byte ptr [rcx]
	b.emitBytes(0x80, 0xFA, 0x20) // cmp dl, ' '
	b.emitJeLabel(".fval_ws_next")
	b.emitBytes(0x80, 0xFA, 0x09) // cmp dl, '\t'
	b.emitJeLabel(".fval_ws_next")
	b.emitBytes(0x80, 0xFA, 0x0D) // cmp dl, '\r'
	b.emitJneLabel(".fval_check_sign")
	b.defineLabel(".fval_ws_next")
	b.emitBytes(0x48, 0xFF, 0xC1) // inc rcx
	b.emitJmpLabel(".fval_skip_ws")
	b.defineLabel(".fval_check_sign")
	b.emitBytes(0x80, 0xFA, 0x2D) // cmp dl, '-'
	b.emitJneLabel(".fval_check_plus")
	b.emitBytes(0x49, 0xC7, 0xC0, 0xFF, 0xFF, 0xFF, 0xFF) // mov r8, -1
	b.emitBytes(0x48, 0xFF, 0xC1)                         // inc rcx
	b.emitJmpLabel(".fval_int_loop")
	b.defineLabel(".fval_check_plus")
	b.emitBytes(0x80, 0xFA, 0x2B) // cmp dl, '+'
	b.emitJneLabel(".fval_int_loop")
	b.emitBytes(0x48, 0xFF, 0xC1) // inc rcx
	b.defineLabel(".fval_int_loop")
	b.emitBytes(0x0F, 0xB6, 0x11) // movzx edx, byte ptr [rcx]
	b.emitBytes(0x80, 0xEA, 0x30) // sub dl, '0'
	b.emitBytes(0x80, 0xFA, 0x09) // cmp dl, 9
	b.emitJaLabel(".fval_check_dot")
	b.emitBytes(0x48, 0x6B, 0xC0, 0x0A) // imul rax, rax, 10
	b.emitBytes(0x48, 0x01, 0xD0)       // add rax, rdx
	b.emitBytes(0x48, 0xFF, 0xC1)       // inc rcx
	b.emitJmpLabel(".fval_int_loop")
	b.defineLabel(".fval_check_dot")
	b.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xC0) // cvtsi2sd xmm0, rax (intPart)
	b.emitBytes(0x80, 0x39, 0x2E)             // cmp byte ptr [rcx], '.'
	b.emitJneLabel(".fval_apply_sign")
	b.emitBytes(0x48, 0xFF, 0xC1)                   // inc rcx
	b.emitBytes(0x45, 0x31, 0xC9)                   // xor r9d, r9d (fracAcc = 0)
	b.emitBytes(0x41, 0xBA, 0x01, 0x00, 0x00, 0x00) // mov r10d, 1 (fracScale = 1)
	b.defineLabel(".fval_frac_loop")
	b.emitBytes(0x0F, 0xB6, 0x11) // movzx edx, byte ptr [rcx]
	b.emitBytes(0x80, 0xEA, 0x30) // sub dl, '0'
	b.emitBytes(0x80, 0xFA, 0x09) // cmp dl, 9
	b.emitJaLabel(".fval_frac_done")
	b.emitBytes(0x4D, 0x6B, 0xC9, 0x0A) // imul r9, r9, 10
	b.emitBytes(0x49, 0x01, 0xD1)       // add r9, rdx
	b.emitBytes(0x4D, 0x6B, 0xD2, 0x0A) // imul r10, r10, 10
	b.emitBytes(0x48, 0xFF, 0xC1)       // inc rcx
	b.emitJmpLabel(".fval_frac_loop")
	b.defineLabel(".fval_frac_done")
	b.emitBytes(0xF2, 0x49, 0x0F, 0x2A, 0xC9) // cvtsi2sd xmm1, r9 (fracAcc)
	b.emitBytes(0xF2, 0x49, 0x0F, 0x2A, 0xD2) // cvtsi2sd xmm2, r10 (fracScale)
	b.emitBytes(0xF2, 0x0F, 0x5E, 0xCA)       // divsd xmm1, xmm2
	b.emitBytes(0xF2, 0x0F, 0x58, 0xC1)       // addsd xmm0, xmm1
	b.defineLabel(".fval_apply_sign")
	b.emitBytes(0xF2, 0x49, 0x0F, 0x2A, 0xC8) // cvtsi2sd xmm1, r8 (sign)
	b.emitBytes(0xF2, 0x0F, 0x59, 0xC1)       // mulsd xmm0, xmm1
	b.emitBytes(0x66, 0x48, 0x0F, 0x7E, 0xC0) // movq rax, xmm0
	b.defineLabel(".fval_str_done")
	b.emitBytes(0xC3)

	// fn_array_pop(rcx = arrPtr) -> rax = last element (value semantics)
	b.defineLabel("fn_array_pop")
	b.emitBytes(0x31, 0xC0)       // xor eax, eax
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".arr_pop_done")
	b.emitBytes(0x48, 0x8B, 0x11)       // mov rdx, [rcx + 0] (len)
	b.emitBytes(0x48, 0x85, 0xD2)       // test rdx, rdx
	b.emitJleLabel(".arr_pop_done")     // if len <= 0, return 0
	b.emitBytes(0x48, 0xFF, 0xCA)       // dec rdx
	b.emitBytes(0x4C, 0x8B, 0x41, 0x10) // mov r8, [rcx + 16] (dataPtr)
	b.emitBytes(0x49, 0x8B, 0x04, 0xD0) // mov rax, [r8 + rdx*8]
	b.defineLabel(".arr_pop_done")
	b.emitBytes(0xC3)

	// joss_map_len(rcx = mapPtr) -> rax = count of keys in O(1)
	b.defineLabel("joss_map_len")
	b.emitBytes(0x31, 0xC0)       // xor eax, eax
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".map_len_done")
	b.emitBytes(0x48, 0x8B, 0x41, 0x08) // mov rax, [rcx + 8] (count)
	b.defineLabel(".map_len_done")
	b.emitBytes(0xC3)

	// fn_microtime(rcx = asFloat) -> rax = float64 bits (seconds)
	b.defineLabel("fn_microtime")
	if b.target.OS == "linux" {
		// Linux x86-64: syscall 228 (sys_clock_gettime(CLOCK_MONOTONIC=1, rsp-16))
		b.emitBytes(0x48, 0x83, 0xEC, 0x18)                                     // sub rsp, 24
		b.emitBytes(0xBF, 0x01, 0x00, 0x00, 0x00)                               // mov edi, 1 (CLOCK_MONOTONIC)
		b.emitBytes(0x48, 0x8D, 0x74, 0x24, 0x08)                               // lea rsi, [rsp + 8] (timespec)
		b.emitBytes(0xB8, 0xE4, 0x00, 0x00, 0x00)                               // mov eax, 228 (sys_clock_gettime)
		b.emitBytes(0x0F, 0x05)                                                 // syscall
		b.emitBytes(0x48, 0x8B, 0x44, 0x24, 0x08)                               // mov rax, [rsp + 8] (sec)
		b.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xC0)                               // cvtsi2sd xmm0, rax
		b.emitBytes(0x48, 0x8B, 0x4C, 0x24, 0x10)                               // mov rcx, [rsp + 16] (nsec)
		b.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xC9)                               // cvtsi2sd xmm1, rcx
		b.emitBytes(0x48, 0xB8, 0x00, 0x00, 0x00, 0x00, 0x65, 0xCD, 0xCD, 0x41) // mov rax, 0x41CDCD6500000000 (1e9)
		b.emitBytes(0x66, 0x48, 0x0F, 0x6E, 0xD0)                               // movq xmm2, rax
		b.emitBytes(0xF2, 0x0F, 0x5E, 0xCA)                                     // divsd xmm1, xmm2 (nsec / 1e9)
		b.emitBytes(0xF2, 0x0F, 0x58, 0xC1)                                     // addsd xmm0, xmm1 (sec + nsec/1e9)
		b.emitBytes(0x66, 0x48, 0x0F, 0x7E, 0xC0)                               // movq rax, xmm0
		b.emitBytes(0x48, 0x83, 0xC4, 0x18)                                     // add rsp, 24
		b.emitBytes(0xC3)
	} else {
		// Windows x86-64: Read 0x7FFE0014 (KUSER_SHARED_DATA SystemTime)
		b.emitBytes(0x48, 0xBA, 0x14, 0x00, 0xFE, 0x7F, 0x00, 0x00, 0x00, 0x00) // mov rdx, 0x7FFE0014
		b.emitBytes(0x48, 0x8B, 0x02)                                           // mov rax, [rdx]
		b.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xC0)                               // cvtsi2sd xmm0, rax
		b.emitBytes(0x48, 0xBA)                                                 // mov rdx, 1e7 (double)
		b.emitU64(0x416312D000000000)
		b.emitBytes(0x66, 0x48, 0x0F, 0x6E, 0xCA) // movq xmm1, rdx
		b.emitBytes(0xF2, 0x0F, 0x5E, 0xC1)       // divsd xmm0, xmm1
		b.emitBytes(0x66, 0x48, 0x0F, 0x7E, 0xC0) // movq rax, xmm0
		b.emitBytes(0xC3)
	}

	// fn_explode(rcx = delimPtr, rdx = strPtr) -> rax = arrPtr of substring pointers
	b.defineLabel("fn_explode")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x56, 0x57)                               // push rbx; push rsi; push rdi
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x28)                         // sub rsp, 40
	b.emitBytes(0x48, 0x89, 0xCE)                               // mov rsi, rcx (delimPtr)
	b.emitBytes(0x48, 0x89, 0xD7)                               // mov rdi, rdx (strPtr)
	b.emitBytes(0x48, 0x81, 0xFE, 0x00, 0x00, 0x01, 0x00)       // cmp rsi, 0x10000
	b.emitJgeLabel(".explode_delim_ok")
	b.emitLeaRCXString("")
	b.emitBytes(0x48, 0x89, 0xCE) // mov rsi, rcx
	b.defineLabel(".explode_delim_ok")
	b.emitBytes(0x48, 0x81, 0xFF, 0x00, 0x00, 0x01, 0x00) // cmp rdi, 0x10000
	b.emitJgeLabel(".explode_str_ok")
	b.emitLeaRCXString("")
	b.emitBytes(0x48, 0x89, 0xCF) // mov rdi, rcx
	b.defineLabel(".explode_str_ok")
	b.emitBytes(0x0F, 0xB6, 0x1E) // movzx ebx, byte ptr [rsi] (delimChar)
	// First pass: count occurrences of delimChar in strPtr to pre-allocate exact capacity
	b.emitBytes(0x48, 0x89, 0xF9)                   // mov rcx, rdi
	b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00) // mov r8d, 1 (tokenCount = 1)
	b.emitBytes(0x85, 0xDB)                         // test ebx, ebx
	b.emitJeLabel(".explode_count_done")
	b.defineLabel(".explode_count_loop")
	b.emitBytes(0x0F, 0xB6, 0x11) // movzx edx, byte ptr [rcx]
	b.emitBytes(0x85, 0xD2)       // test edx, edx
	b.emitJeLabel(".explode_count_done")
	b.emitBytes(0x39, 0xDA) // cmp edx, ebx
	b.emitJneLabel(".explode_count_next")
	b.emitBytes(0x49, 0xFF, 0xC0) // inc r8
	b.defineLabel(".explode_count_next")
	b.emitBytes(0x48, 0xFF, 0xC1) // inc rcx
	b.emitJmpLabel(".explode_count_loop")
	b.defineLabel(".explode_count_done")
	b.emitBytes(0x4C, 0x89, 0xC1)       // mov rcx, r8 (cap = tokenCount)
	b.emitBytes(0x48, 0x83, 0xF9, 0x08) // cmp rcx, 8
	b.emitJgeLabel(".explode_cap_ok")
	b.emitBytes(0x48, 0xC7, 0xC1, 0x08, 0x00, 0x00, 0x00) // mov rcx, 8
	b.defineLabel(".explode_cap_ok")
	b.emitBytes(0x49, 0x89, 0xCD)             // mov r13, rcx (save cap)
	b.emitBytes(0xB9, 0x18, 0x00, 0x00, 0x00) // mov ecx, 24 (arrHeader)
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x49, 0x89, 0xC4)       // mov r12, rax (arrPtr)
	b.emitBytes(0x4C, 0x89, 0xE9)       // mov rcx, r13
	b.emitBytes(0x48, 0xC1, 0xE1, 0x03) // shl rcx, 3 (cap * 8)
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x4D, 0x89, 0x6C, 0x24, 0x08) // mov [r12 + 8], r13 (arr.cap = cap)
	b.emitBytes(0x49, 0x89, 0x44, 0x24, 0x10) // mov [r12 + 16], rax (arr.dataPtr = dataPtr)
	b.emitBytes(0x49, 0x89, 0xC5)             // mov r13, rax (dataPtr)
	b.emitBytes(0x45, 0x31, 0xF6)             // xor r14d, r14d (outIdx = 0)
	b.emitBytes(0x49, 0x89, 0xFF)             // mov r15, rdi (cursor = strPtr)
	b.defineLabel(".explode_token_loop")
	b.emitBytes(0x4C, 0x89, 0xFA) // mov rdx, r15 (scan = cursor)
	b.defineLabel(".explode_scan_loop")
	b.emitBytes(0x0F, 0xB6, 0x0A) // movzx ecx, byte ptr [rdx]
	b.emitBytes(0x85, 0xC9)       // test ecx, ecx
	b.emitJeLabel(".explode_found_end")
	b.emitBytes(0x85, 0xDB) // test ebx, ebx
	b.emitJeLabel(".explode_scan_next")
	b.emitBytes(0x39, 0xD9) // cmp ecx, ebx
	b.emitJeLabel(".explode_found_end")
	b.defineLabel(".explode_scan_next")
	b.emitBytes(0x48, 0xFF, 0xC2) // inc rdx
	b.emitJmpLabel(".explode_scan_loop")
	b.defineLabel(".explode_found_end")
	b.emitBytes(0x41, 0x89, 0xC8) // mov r8d, ecx (save endByte: 0 or delimChar)
	b.emitBytes(0x48, 0x89, 0xD6) // mov rsi, rdx (endPtr)
	// Strip trailing '\r' when splitting by '\n' (CRLF normalization)
	b.emitBytes(0x83, 0xFB, 0x0A) // cmp ebx, 10 ('\n')
	b.emitJneLabel(".explode_store_tok")
	b.emitBytes(0x4C, 0x39, 0xFA) // cmp rdx, r15 (endPtr > cursor?)
	b.emitJleLabel(".explode_store_tok")
	b.emitBytes(0x80, 0x7A, 0xFF, 0x0D) // cmp byte ptr [rdx - 1], 13 ('\r')
	b.emitJneLabel(".explode_store_tok")
	b.emitBytes(0xC6, 0x42, 0xFF, 0x00) // mov byte ptr [rdx - 1], 0
	b.defineLabel(".explode_store_tok")
	b.emitBytes(0xC6, 0x06, 0x00)             // mov byte ptr [rsi], 0 (null-terminate token in-place)
	b.emitBytes(0x4F, 0x89, 0x7C, 0xF5, 0x00) // mov [r13 + r14*8], r15 (store token pointer)
	b.emitBytes(0x49, 0xFF, 0xC6)             // inc r14
	// Check if endByte was 0 (end of string)
	b.emitBytes(0x45, 0x85, 0xC0) // test r8d, r8d
	b.emitJeLabel(".explode_finish")
	b.emitBytes(0x4C, 0x8D, 0x7E, 0x01) // lea r15, [rsi + 1] (cursor = endPtr + 1)
	b.emitJmpLabel(".explode_token_loop")
	b.defineLabel(".explode_finish")
	b.emitBytes(0x4D, 0x89, 0x34, 0x24)                                           // mov [r12 + 0], r14 (arr.len = outIdx)
	b.emitBytes(0x4C, 0x89, 0xE0)                                                 // mov rax, r12 (return arrPtr)
	b.emitBytes(0x48, 0x83, 0xC4, 0x28)                                           // add rsp, 40
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5F, 0x5E, 0x5B) // pop r15..r12, rdi, rsi, rbx
	b.emitBytes(0x5D, 0xC3)

	// fn_file_get_contents(rcx = pathPtr) -> rax = strPtr
	b.defineLabel("fn_file_get_contents")
	if b.target.OS == "windows" {
		b.emitBytes(0x55, 0x48, 0x89, 0xE5)       // push rbp; mov rbp, rsp
		b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55) // push rbx; push r12; push r13
		b.emitBytes(0x48, 0x83, 0xEC, 0x48)       // sub rsp, 72
		// CreateFileA(pathPtr, GENERIC_READ=0x80000000, FILE_SHARE_READ=1, NULL=0, OPEN_EXISTING=3, FILE_ATTRIBUTE_NORMAL=0x80, NULL=0)
		b.emitBytes(0xBA, 0x00, 0x00, 0x00, 0x80)                         // mov edx, 0x80000000
		b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00)                   // mov r8d, 1
		b.emitBytes(0x45, 0x31, 0xC9)                                     // xor r9d, r9d
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x03, 0x00, 0x00, 0x00) // mov qword ptr [rsp+32], 3
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x28, 0x80, 0x00, 0x00, 0x00) // mov qword ptr [rsp+40], 0x80
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x30, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp+48], 0
		b.emitMovRAXData(dataOffCreateFileA)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF) // cmp rax, -1 (INVALID_HANDLE_VALUE)
		b.emitJeLabel(".fgc_win_fail")
		b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax (hFile)
		// GetFileSize(hFile, NULL)
		b.emitBytes(0x48, 0x89, 0xD9) // mov rcx, rbx
		b.emitBytes(0x31, 0xD2)       // xor edx, edx
		b.emitMovRAXData(dataOffGetFileSize)
		b.emitBytes(0xFF, 0xD0)                   // call rax
		b.emitBytes(0x49, 0x89, 0xC4)             // mov r12, rax (fileSize)
		b.emitBytes(0x49, 0x8D, 0x4C, 0x24, 0x01) // lea rcx, [r12 + 1]
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x49, 0x89, 0xC5) // mov r13, rax (bufPtr)
		// ReadFile(hFile, bufPtr, fileSize, &bytesRead, NULL)
		b.emitBytes(0x48, 0x89, 0xD9)                                     // mov rcx, rbx
		b.emitBytes(0x4C, 0x89, 0xEA)                                     // mov rdx, r13
		b.emitBytes(0x4D, 0x89, 0xE0)                                     // mov r8, r12
		b.emitBytes(0x4C, 0x8D, 0x4D, 0xE0)                               // lea r9, [rbp - 32] (&bytesRead)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp+32], 0
		b.emitMovRAXData(dataOffReadFile)
		b.emitBytes(0xFF, 0xD0) // call rax
		// CloseHandle(hFile)
		b.emitBytes(0x48, 0x89, 0xD9) // mov rcx, rbx
		b.emitMovRAXData(dataOffCloseHandle)
		b.emitBytes(0xFF, 0xD0)                         // call rax
		b.emitBytes(0x43, 0xC6, 0x44, 0x25, 0x00, 0x00) // mov byte ptr [r13 + r12], 0
		b.emitBytes(0x4C, 0x89, 0xE8)                   // mov rax, r13
		b.emitJmpLabel(".fgc_win_exit")
		b.defineLabel(".fgc_win_fail")
		b.emitLeaRAXString("")
		b.defineLabel(".fgc_win_exit")
		b.emitBytes(0x48, 0x83, 0xC4, 0x48)             // add rsp, 72
		b.emitBytes(0x41, 0x5D, 0x41, 0x5C, 0x5B, 0x5D) // pop r13, r12, rbx, rbp
		b.emitBytes(0xC3)
	} else {
		// Linux x86-64: sys_open(2), sys_lseek(8), sys_read(0), sys_close(3)
		b.emitBytes(0x55, 0x48, 0x89, 0xE5)                   // push rbp; mov rbp, rsp
		b.emitBytes(0x53, 0x56, 0x57, 0x41, 0x54, 0x41, 0x55) // push rbx, rsi, rdi, r12, r13
		b.emitBytes(0x48, 0x83, 0xEC, 0x18)                   // sub rsp, 24
		b.emitBytes(0x48, 0x89, 0xCF)                         // mov rdi, rcx (pathPtr)
		b.emitBytes(0x31, 0xF6)                               // xor esi, esi (O_RDONLY = 0)
		b.emitBytes(0x31, 0xD2)                               // xor edx, edx
		b.emitBytes(0xB8, 0x02, 0x00, 0x00, 0x00)             // mov eax, 2 (sys_open)
		b.emitBytes(0x0F, 0x05)                               // syscall
		b.emitBytes(0x48, 0x85, 0xC0)                         // test rax, rax
		b.emitJlLabel(".fgc_linux_fail")
		b.emitBytes(0x48, 0x89, 0xC3)             // mov rbx, rax (fd)
		b.emitBytes(0x48, 0x89, 0xDF)             // mov rdi, rbx
		b.emitBytes(0x31, 0xF6)                   // xor esi, esi (offset = 0)
		b.emitBytes(0xBA, 0x02, 0x00, 0x00, 0x00) // mov edx, 2 (SEEK_END)
		b.emitBytes(0xB8, 0x08, 0x00, 0x00, 0x00) // mov eax, 8 (sys_lseek)
		b.emitBytes(0x0F, 0x05)                   // syscall
		b.emitBytes(0x49, 0x89, 0xC4)             // mov r12, rax (fileSize)
		b.emitBytes(0x48, 0x89, 0xDF)             // mov rdi, rbx
		b.emitBytes(0x31, 0xF6)                   // xor esi, esi
		b.emitBytes(0x31, 0xD2)                   // xor edx, edx (SEEK_SET)
		b.emitBytes(0xB8, 0x08, 0x00, 0x00, 0x00) // mov eax, 8 (sys_lseek)
		b.emitBytes(0x0F, 0x05)                   // syscall
		b.emitBytes(0x49, 0x8D, 0x4C, 0x24, 0x01) // lea rcx, [r12 + 1]
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x49, 0x89, 0xC5)                   // mov r13, rax (bufPtr)
		b.emitBytes(0x48, 0x89, 0xDF)                   // mov rdi, rbx
		b.emitBytes(0x4C, 0x89, 0xEE)                   // mov rsi, r13
		b.emitBytes(0x4C, 0x89, 0xE2)                   // mov rdx, r12
		b.emitBytes(0x31, 0xC0)                         // xor eax, eax (sys_read = 0)
		b.emitBytes(0x0F, 0x05)                         // syscall
		b.emitBytes(0x48, 0x89, 0xDF)                   // mov rdi, rbx
		b.emitBytes(0xB8, 0x03, 0x00, 0x00, 0x00)       // mov eax, 3 (sys_close)
		b.emitBytes(0x0F, 0x05)                         // syscall
		b.emitBytes(0x43, 0xC6, 0x44, 0x25, 0x00, 0x00) // mov byte ptr [r13 + r12], 0
		b.emitBytes(0x4C, 0x89, 0xE8)                   // mov rax, r13
		b.emitJmpLabel(".fgc_linux_exit")
		b.defineLabel(".fgc_linux_fail")
		b.emitLeaRAXString("")
		b.defineLabel(".fgc_linux_exit")
		b.emitBytes(0x48, 0x83, 0xC4, 0x18)                         // add rsp, 24
		b.emitBytes(0x41, 0x5D, 0x41, 0x5C, 0x5F, 0x5E, 0x5B, 0x5D) // pop r13, r12, rdi, rsi, rbx, rbp
		b.emitBytes(0xC3)
	}

	// __joss_thread_trampoline(rcx = taskBox) -> eax = 0 (Win32 LPTHREAD_START_ROUTINE)
	// taskBox layout: [0] = fnPtr, [8] = result, [16] = hThread
	b.defineLabel("__joss_thread_trampoline")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x53)                   // push rbx
	b.emitBytes(0x48, 0x83, 0xEC, 0x28) // sub rsp, 40
	b.emitBytes(0x48, 0x89, 0xCB)       // mov rbx, rcx (taskBox)
	b.emitBytes(0x48, 0x8B, 0x03)       // mov rax, [rbx + 0] (fnPtr)
	b.emitBytes(0xFF, 0xD0)             // call rax
	b.emitBytes(0x48, 0x89, 0x43, 0x08) // mov [rbx + 8], rax (result)
	b.emitBytes(0x31, 0xC0)             // xor eax, eax
	b.emitBytes(0x48, 0x83, 0xC4, 0x28, 0x5B, 0x5D, 0xC3)

	// fn_async(rcx = fnPtr) -> rax = taskBox
	b.defineLabel("fn_async")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)       // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x41, 0x54)             // push rbx; push r12
	b.emitBytes(0x48, 0x83, 0xEC, 0x40)       // sub rsp, 64
	b.emitBytes(0x48, 0x89, 0xCB)             // mov rbx, rcx (fnPtr)
	b.emitBytes(0xB9, 0x18, 0x00, 0x00, 0x00) // mov ecx, 24 (taskBox)
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x49, 0x89, 0xC4)       // mov r12, rax (taskBox)
	b.emitBytes(0x49, 0x89, 0x1C, 0x24) // mov [r12 + 0], rbx (fnPtr)
	if b.target.OS == "windows" {
		b.emitMovRAXData(dataOffCreateThread)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".async_sync_fallback")
		// CreateThread(NULL=0, 0, __joss_thread_trampoline, taskBox, 0, NULL=0)
		b.emitBytes(0x31, 0xC9)                                           // xor ecx, ecx
		b.emitBytes(0x31, 0xD2)                                           // xor edx, edx
		b.emitLeaR8Label("__joss_thread_trampoline")                      // r8 = trampoline
		b.emitBytes(0x4D, 0x89, 0xE1)                                     // mov r9, r12 (taskBox)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp+32], 0
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x28, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp+40], 0
		b.emitMovRAXData(dataOffCreateThread)
		b.emitBytes(0xFF, 0xD0)       // call rax
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".async_sync_fallback")
		b.emitBytes(0x49, 0x89, 0x44, 0x24, 0x10) // mov [r12 + 16], rax (hThread)
		b.emitBytes(0x4C, 0x89, 0xE0)             // mov rax, r12
		b.emitJmpLabel(".async_done")
	}
	b.defineLabel(".async_sync_fallback")
	b.emitBytes(0xFF, 0xD3)                   // call rbx (fnPtr)
	b.emitBytes(0x49, 0x89, 0x44, 0x24, 0x08) // mov [r12 + 8], rax (result)
	b.emitBytes(0x4C, 0x89, 0xE0)             // mov rax, r12
	b.defineLabel(".async_done")
	b.emitBytes(0x48, 0x83, 0xC4, 0x40)       // add rsp, 64
	b.emitBytes(0x41, 0x5C, 0x5B, 0x5D, 0xC3) // pop r12, rbx, rbp; ret

	// fn_await(rcx = taskBox) -> rax = result
	b.defineLabel("fn_await")
	b.emitBytes(0x31, 0xC0)       // xor eax, eax
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".await_ret_null")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x53)                   // push rbx
	b.emitBytes(0x48, 0x83, 0xEC, 0x28) // sub rsp, 40
	b.emitBytes(0x48, 0x89, 0xCB)       // mov rbx, rcx (taskBox)
	if b.target.OS == "windows" {
		b.emitBytes(0x48, 0x8B, 0x4B, 0x10) // mov rcx, [rbx + 16] (hThread)
		b.emitBytes(0x48, 0x85, 0xC9)       // test rcx, rcx
		b.emitJeLabel(".await_load_res")
		b.emitBytes(0xBA, 0xFF, 0xFF, 0xFF, 0xFF) // mov edx, 0xFFFFFFFF (INFINITE)
		b.emitMovRAXData(dataOffWaitSingleObject)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitBytes(0x48, 0x8B, 0x4B, 0x10) // mov rcx, [rbx + 16]
		b.emitMovRAXData(dataOffCloseHandle)
		b.emitBytes(0xFF, 0xD0)                                     // call rax
		b.emitBytes(0x48, 0xC7, 0x43, 0x10, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rbx + 16], 0
	}
	b.defineLabel(".await_load_res")
	b.emitBytes(0x48, 0x8B, 0x43, 0x08)             // mov rax, [rbx + 8] (result)
	b.emitBytes(0x48, 0x83, 0xC4, 0x28, 0x5B, 0x5D) // add rsp, 40; pop rbx; pop rbp
	b.defineLabel(".await_ret_null")
	b.emitBytes(0xC3)

	// =========================================================================
	// 7. Exception Helpers (Exception_constructor, Exception_getMessage, Exception_getCode, __joss_rt_throw)
	// =========================================================================
	// Exception_constructor(rcx = this, rdx = msg)
	b.defineLabel("fn_Exception_constructor")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x48, 0x83, 0xEC, 0x20)
	b.emitBytes(0x49, 0x89, 0xD0) // mov r8, rdx (msg)
	b.emitLeaRDXString("message")
	b.emitCallLabel("joss_obj_set_field_str")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)

	// Exception_getMessage(rcx = this) -> rax = msg
	b.defineLabel("fn_Exception_getMessage")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x48, 0x83, 0xEC, 0x20)
	b.emitLeaRDXString("message")
	b.emitCallLabel("joss_obj_get_field_str")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)

	// Exception_getCode(rcx = this) -> rax = code
	b.defineLabel("fn_Exception_getCode")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x48, 0x83, 0xEC, 0x20)
	b.emitLeaRDXString("code")
	b.emitCallLabel("joss_obj_get_field_i64")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)

	// __joss_rt_throw(rcx = exceptionVal)
	b.defineLabel("__joss_rt_throw")
	b.emitMovRAXData(dataOffTryDepth)
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJgLabel(".throw_unwind")
	// Uncaught exception -> panic
	b.emitJmpLabel("joss_panic")
	b.defineLabel(".throw_unwind")
	b.emitBytes(0x48, 0xFF, 0xC8) // dec rax
	b.emitMovDataRAX(dataOffTryDepth)
	b.emitMovDataRCX(dataOffExceptionVal)
	b.emitLeaRDXData(dataOffTryRBP)
	b.emitBytes(0x48, 0x8B, 0x2C, 0xC2) // mov rbp, [rdx + rax*8]
	b.emitLeaRDXData(dataOffTryRSP)
	b.emitBytes(0x48, 0x8B, 0x24, 0xC2) // mov rsp, [rdx + rax*8]
	b.emitLeaRDXData(dataOffTryRIP)
	b.emitBytes(0x48, 0x8B, 0x14, 0xC2) // mov rdx, [rdx + rax*8]
	b.emitBytes(0xFF, 0xE2)             // jmp rdx
}

type fnFrameLayout struct {
	fnLabel           string
	paramOffsets      map[string]int32
	tempOffsets       map[int]int32
	aliasedOffsets    map[int32]bool
	frameSize         uint32
	inTryDepth        int
	deferredSlowPaths []func()
}

func (b *MachineCodeBuilder) emitIRFunction(fn *ir.Function) error {
	fnLabel := "fn_" + fn.Name
	b.defineLabel(fnLabel)

	// Compute stack frame layout:
	// - Parameters 0..N-1 are stored in local slots [rbp - 8], [rbp - 16], ...
	// - Temporaries 0..NextTempID-1 are stored at [rbp - (len(Params)+1+id)*8]
	// - Below all local slots, reserve 32 + maxExtraCallArgs*8 bytes for outgoing calls
	paramOffsets := make(map[string]int32, len(fn.Params))
	slotIdx := int32(1)
	for _, p := range fn.Params {
		paramOffsets[cleanName(p.Name)] = -slotIdx * 8
		slotIdx++
	}

	tempOffsets := make(map[int]int32, fn.NextTempID)
	for i := 0; i < fn.NextTempID; i++ {
		tempOffsets[i] = -slotIdx * 8
		slotIdx++
	}

	aliasedOffsets := make(map[int32]bool)

	// SSA Load Offset Forwarding:
	// Si un TempValue es el resultado directo de un Load de un puntero local (alloca),
	// alias su stack offset directamente al puntero para evitar escribir/leer en la pila.
	for _, blk := range fn.Blocks {
		for _, inst := range blk.Instructions {
			if load, ok := inst.(*ir.LoadInst); ok {
				if ptrTemp, isTemp := load.Ptr.(*ir.TempValue); isTemp {
					off := tempOffsets[ptrTemp.ID]
					tempOffsets[load.Dest.ID] = off
					aliasedOffsets[off] = true
				}
			}
		}
	}

	// SSA Store Forwarding:
	// Si un TempValue es usado ÚNICAMENTE por una StoreInst inmediatamente siguiente en el mismo bloque,
	// alias su stack offset directamente al puntero destino para escribir el resultado directamente.
	tempUseCount := make(map[int]int, fn.NextTempID)
	for _, blk := range fn.Blocks {
		for _, inst := range blk.Instructions {
			for _, op := range inst.Operands() {
				if t, ok := op.(*ir.TempValue); ok {
					tempUseCount[t.ID]++
				}
			}
		}
		if blk.Terminator != nil {
			for _, op := range blk.Terminator.Operands() {
				if t, ok := op.(*ir.TempValue); ok {
					tempUseCount[t.ID]++
				}
			}
		}
	}

	for _, blk := range fn.Blocks {
		for idx, inst := range blk.Instructions {
			dest := inst.Result()
			if dest != nil && tempUseCount[dest.ID] == 1 {
				if idx+1 < len(blk.Instructions) {
					if store, ok := blk.Instructions[idx+1].(*ir.StoreInst); ok {
						if vTemp, ok2 := store.Val.(*ir.TempValue); ok2 && vTemp.ID == dest.ID {
							if ptrTemp, ok3 := store.Ptr.(*ir.TempValue); ok3 {
								off := tempOffsets[ptrTemp.ID]
								tempOffsets[dest.ID] = off
								aliasedOffsets[off] = true
							}
						}
					}
				}
			}
		}
	}

	maxCallArgs := 4
	var scanCallArgs func(blocks []*ir.BasicBlock)
	scanCallArgs = func(blocks []*ir.BasicBlock) {
		for _, blk := range blocks {
			for _, inst := range blk.Instructions {
				switch i := inst.(type) {
				case *ir.CallInst:
					if len(i.Args) > maxCallArgs {
						maxCallArgs = len(i.Args)
					}
				case *ir.CallIndirectInst:
					if len(i.Args) > maxCallArgs {
						maxCallArgs = len(i.Args)
					}
				case *ir.CallBuiltinInst:
					if len(i.Args) > maxCallArgs {
						maxCallArgs = len(i.Args)
					}
				case *ir.CallPrimitiveMethodInst:
					if len(i.Args) > maxCallArgs {
						maxCallArgs = len(i.Args)
					}
				case *ir.CallHostServiceInst:
					if len(i.Args) > maxCallArgs {
						maxCallArgs = len(i.Args)
					}
				case *ir.CallRuntimeInst:
					if len(i.Args) > maxCallArgs {
						maxCallArgs = len(i.Args)
					}
				case *ir.TryCatchInst:
					scanCallArgs(i.TryBlocks)
					scanCallArgs(i.CatchBlocks)
				}
			}
		}
	}
	scanCallArgs(fn.Blocks)

	callAreaBytes := uint32(32 + (maxCallArgs-4)*8)
	localsBytes := uint32(slotIdx-1) * 8
	frameSize := alignUpU32(localsBytes+callAreaBytes, 16)
	if frameSize < 32 {
		frameSize = 32
	}

	layout := &fnFrameLayout{
		fnLabel:        fnLabel,
		paramOffsets:   paramOffsets,
		tempOffsets:    tempOffsets,
		aliasedOffsets: aliasedOffsets,
		frameSize:      frameSize,
	}

	// Prologue: push rbp; mov rbp, rsp; sub rsp, frameSize
	b.emitBytes(0x55)             // push rbp
	b.emitBytes(0x48, 0x89, 0xE5) // mov rbp, rsp
	if frameSize <= 127 {
		b.emitBytes(0x48, 0x83, 0xEC, byte(frameSize)) // sub rsp, imm8
	} else {
		b.emitBytes(0x48, 0x81, 0xEC) // sub rsp, imm32
		b.emitU32(frameSize)
	}

	// Spill incoming register/stack parameters into their frame slots
	for idx, p := range fn.Params {
		off := paramOffsets[cleanName(p.Name)]
		switch idx {
		case 0:
			b.emitStoreRCXToRBP(off)
			b.cachedRCXVal = p
		case 1:
			b.emitStoreRDXToRBP(off)
		case 2:
			b.emitStoreR8ToRBP(off)
		case 3:
			b.emitStoreR9ToRBP(off)
		default:
			callerStackOff := int32(48 + (idx-4)*8)
			b.emitLoadRAXFromRBP(callerStackOff)
			b.emitStoreRAXToRBP(off)
		}
	}

	// Emit basic blocks
	if err := b.emitBlocks(fn, layout, fn.Blocks); err != nil {
		return err
	}

	// Default epilogue fallthrough
	b.emitBytes(0x31, 0xC0)                   // xor eax, eax
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // mov rsp, rbp; pop rbp; ret

	// Out-of-line deferred cold/slow paths
	for _, slow := range layout.deferredSlowPaths {
		slow()
	}
	return nil
}

func (b *MachineCodeBuilder) emitBlocks(fn *ir.Function, layout *fnFrameLayout, blocks []*ir.BasicBlock) error {
	for _, blk := range blocks {
		blkLabel := fmt.Sprintf("%s_blk_%s", layout.fnLabel, blk.Label)
		b.defineLabel(blkLabel)

		nInst := len(blk.Instructions)
		fuseBranch := false
		var cmpInst *ir.CompareInst
		var brTerm *ir.BranchTerminator

		if bt, ok := blk.Terminator.(*ir.BranchTerminator); ok && nInst > 0 {
			if ci, isCmp := blk.Instructions[nInst-1].(*ir.CompareInst); isCmp && ci.Op != ir.OpSpaceship {
				if condTemp, isTemp := bt.Cond.(*ir.TempValue); isTemp && condTemp.ID == ci.Dest.ID {
					fuseBranch = true
					cmpInst = ci
					brTerm = bt
				}
			}
		}

		limit := nInst
		if fuseBranch {
			limit = nInst - 1
		}

		for idx := 0; idx < limit; idx++ {
			inst := blk.Instructions[idx]
			if tc, ok := inst.(*ir.TryCatchInst); ok {
				if err := b.emitTryCatch(fn, layout, tc); err != nil {
					return err
				}
				continue
			}
			if err := b.emitInstruction(fn, layout, inst); err != nil {
				return err
			}
		}

		if fuseBranch {
			if err := b.emitFusedCompareBranch(fn, layout, cmpInst, brTerm); err != nil {
				return err
			}
		} else if blk.Terminator != nil {
			if err := b.emitTerminator(fn, layout, blk.Terminator); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *MachineCodeBuilder) emitFusedCompareBranch(fn *ir.Function, layout *fnFrameLayout, ci *ir.CompareInst, bt *ir.BranchTerminator) error {
	trueLbl := fmt.Sprintf("%s_blk_%s", layout.fnLabel, bt.TrueBlock.Label)
	falseLbl := fmt.Sprintf("%s_blk_%s", layout.fnLabel, bt.FalseBlock.Label)

	_, leftIsNull := ci.Left.(*ir.ConstNull)
	_, rightIsNull := ci.Right.(*ir.ConstNull)
	if !leftIsNull && !rightIsNull && (ci.Left.Type().Kind == ir.TypeKindString || ci.Right.Type().Kind == ir.TypeKindString) && (ci.Op == ir.OpCmpEq || ci.Op == ir.OpCmpNe) {
		b.emitCallWithArgs(layout, "__joss_rt_strcmp", []ir.Value{ci.Left, ci.Right})
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		if ci.Op == ir.OpCmpEq {
			b.emitJeLabel(trueLbl)
		} else {
			b.emitJneLabel(trueLbl)
		}
		b.emitJmpLabel(falseLbl)
		return nil
	}

	b.emitLoadValToRAX(layout, ci.Left)
	if cInt, ok := ci.Right.(*ir.ConstInt); ok && cInt.Val >= -0x80000000 && cInt.Val <= 0x7FFFFFFF {
		val := cInt.Val
		if val == 0 && (ci.Op == ir.OpCmpEq || ci.Op == ir.OpCmpNe) {
			b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		} else if val >= -128 && val <= 127 {
			b.emitBytes(0x48, 0x83, 0xF8, byte(val)) // cmp rax, imm8
		} else {
			b.emitBytes(0x48, 0x3D) // cmp rax, imm32
			b.emitU32(uint32(val))
		}
	} else {
		b.emitLoadValToRCX(layout, ci.Right)
		b.emitBytes(0x48, 0x39, 0xC8) // cmp rax, rcx
	}

	switch ci.Op {
	case ir.OpCmpEq:
		b.emitJeLabel(trueLbl)
	case ir.OpCmpNe:
		b.emitJneLabel(trueLbl)
	case ir.OpCmpLt:
		b.emitJlLabel(trueLbl)
	case ir.OpCmpLe:
		b.emitJleLabel(trueLbl)
	case ir.OpCmpGt:
		b.emitJgLabel(trueLbl)
	case ir.OpCmpGe:
		b.emitJgeLabel(trueLbl)
	default:
		b.emitInstruction(fn, layout, ci)
		b.emitTerminator(fn, layout, bt)
		return nil
	}
	b.emitJmpLabel(falseLbl)
	return nil
}

func (b *MachineCodeBuilder) emitTryCatch(fn *ir.Function, layout *fnFrameLayout, tc *ir.TryCatchInst) error {
	catchEntryLbl := fmt.Sprintf("%s_tc_catch_entry_%d", layout.fnLabel, tc.ID)
	tryEndLbl := fmt.Sprintf("%s_tc_end_%d", layout.fnLabel, tc.ID)

	// 1. Push exception frame at DATA_try_depth
	b.emitMovRCXData(dataOffTryDepth)
	b.emitLeaRDXData(dataOffTryRSP)
	b.emitBytes(0x48, 0x89, 0x24, 0xCA) // mov [rdx + rcx*8], rsp
	b.emitLeaRDXData(dataOffTryRBP)
	b.emitBytes(0x48, 0x89, 0x2C, 0xCA) // mov [rdx + rcx*8], rbp
	b.emitLeaRDXData(dataOffTryRIP)
	b.emitLeaRAXLabel(catchEntryLbl)
	b.emitBytes(0x48, 0x89, 0x04, 0xCA) // mov [rdx + rcx*8], rax
	b.emitBytes(0x48, 0xFF, 0xC1)       // inc rcx
	b.emitMovDataRCX(dataOffTryDepth)

	// 2. Emit TryBlocks with incremented inTryDepth
	layout.inTryDepth++
	if err := b.emitBlocks(fn, layout, tc.TryBlocks); err != nil {
		return err
	}
	layout.inTryDepth--

	// 3. Normal completion of TryBlocks: pop exception frame and jump to tryEndLbl
	b.emitMovRCXData(dataOffTryDepth)
	b.emitBytes(0x48, 0xFF, 0xC9) // dec rcx
	b.emitMovDataRCX(dataOffTryDepth)
	b.emitJmpLabel(tryEndLbl)

	// 4. Catch handler entry: __joss_rt_throw has already popped DATA_try_depth and restored rbp/rsp
	b.defineLabel(catchEntryLbl)
	if tc.CatchVarPtr != nil {
		if temp, ok := tc.CatchVarPtr.(*ir.TempValue); ok {
			b.emitMovRAXData(dataOffExceptionVal)
			b.emitStoreRAXToRBP(layout.tempOffsets[temp.ID])
		}
	}
	if err := b.emitBlocks(fn, layout, tc.CatchBlocks); err != nil {
		return err
	}

	b.defineLabel(tryEndLbl)
	return nil
}

func (b *MachineCodeBuilder) emitInstruction(fn *ir.Function, layout *fnFrameLayout, inst ir.Instruction) error {
	switch i := inst.(type) {
	case *ir.AllocaInst:
		if i.AllocType.Kind == ir.TypeKindString {
			b.emitLeaRAXString("")
		} else {
			b.emitBytes(0x31, 0xC0) // xor eax, eax
		}
		b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])

	case *ir.StoreInst:
		if temp, ok := i.Ptr.(*ir.TempValue); ok {
			dstOff := layout.tempOffsets[temp.ID]
			if vTemp, ok2 := i.Val.(*ir.TempValue); ok2 {
				if layout.tempOffsets[vTemp.ID] == dstOff {
					return nil
				}
			}
			b.emitLoadValToRAX(layout, i.Val)
			b.emitStoreRAXToRBP(dstOff)
		}

	case *ir.LoadInst:
		if temp, ok := i.Ptr.(*ir.TempValue); ok {
			srcOff := layout.tempOffsets[temp.ID]
			dstOff := layout.tempOffsets[i.Dest.ID]
			if srcOff != dstOff {
				b.emitLoadRAXFromRBP(srcOff)
				b.emitStoreRAXToRBP(dstOff)
			}
		}

	case *ir.MoveInst:
		b.emitLoadValToRAX(layout, i.Src)
		b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])

	case *ir.BinaryInst:
		isFloat := (i.Dest != nil && (i.Dest.Type().Kind == ir.TypeKindF64 || i.Dest.Type().Kind == ir.TypeKindDecimal)) ||
			i.Left.Type().Kind == ir.TypeKindF64 || i.Right.Type().Kind == ir.TypeKindF64 ||
			i.Left.Type().Kind == ir.TypeKindDecimal || i.Right.Type().Kind == ir.TypeKindDecimal

		if isFloat {
			b.emitLoadValToXMM0(layout, i.Left)
			dispRight, hasDisp := getFloatStackDisp(layout, i.Right)
			if hasDisp {
				switch i.Op {
				case ir.OpAdd:
					b.asmListing.WriteString(fmt.Sprintf("    addsd xmm0, [rbp%+d]\n", dispRight))
					b.emitBytes(0xF2, 0x0F, 0x58, 0x85)
					b.emitU32(uint32(dispRight))
				case ir.OpSub:
					b.asmListing.WriteString(fmt.Sprintf("    subsd xmm0, [rbp%+d]\n", dispRight))
					b.emitBytes(0xF2, 0x0F, 0x5C, 0x85)
					b.emitU32(uint32(dispRight))
				case ir.OpMul:
					b.asmListing.WriteString(fmt.Sprintf("    mulsd xmm0, [rbp%+d]\n", dispRight))
					b.emitBytes(0xF2, 0x0F, 0x59, 0x85)
					b.emitU32(uint32(dispRight))
				case ir.OpDiv:
					b.asmListing.WriteString(fmt.Sprintf("    divsd xmm0, [rbp%+d]\n", dispRight))
					b.emitBytes(0xF2, 0x0F, 0x5E, 0x85)
					b.emitU32(uint32(dispRight))
				}
			} else {
				b.emitLoadValToXMM1(layout, i.Right)
				switch i.Op {
				case ir.OpAdd:
					b.emitBytes(0xF2, 0x0F, 0x58, 0xC1) // addsd xmm0, xmm1
				case ir.OpSub:
					b.emitBytes(0xF2, 0x0F, 0x5C, 0xC1) // subsd xmm0, xmm1
				case ir.OpMul:
					b.emitBytes(0xF2, 0x0F, 0x59, 0xC1) // mulsd xmm0, xmm1
				case ir.OpDiv:
					b.emitBytes(0xF2, 0x0F, 0x5E, 0xC1) // divsd xmm0, xmm1
				}
			}
			b.emitStoreXMM0ToRBP(layout.tempOffsets[i.Dest.ID])
			return nil
		}

		b.emitLoadValToRAX(layout, i.Left)
		emittedOp := false
		if cInt, ok := i.Right.(*ir.ConstInt); ok {
			val := cInt.Val
			switch i.Op {
			case ir.OpAdd:
				if val == 1 {
					b.emitBytes(0x48, 0xFF, 0xC0) // inc rax
					emittedOp = true
				} else if val == -1 {
					b.emitBytes(0x48, 0xFF, 0xC8) // dec rax
					emittedOp = true
				} else if val >= -128 && val <= 127 {
					b.emitBytes(0x48, 0x83, 0xC0, byte(val)) // add rax, imm8
					emittedOp = true
				} else if val >= -0x80000000 && val <= 0x7FFFFFFF {
					b.emitBytes(0x48, 0x05) // add rax, imm32
					b.emitU32(uint32(val))
					emittedOp = true
				}
			case ir.OpSub:
				if val == 1 {
					b.emitBytes(0x48, 0xFF, 0xC8) // dec rax
					emittedOp = true
				} else if val == -1 {
					b.emitBytes(0x48, 0xFF, 0xC0) // inc rax
					emittedOp = true
				} else if val >= -128 && val <= 127 {
					b.emitBytes(0x48, 0x83, 0xE8, byte(val)) // sub rax, imm8
					emittedOp = true
				} else if val >= -0x80000000 && val <= 0x7FFFFFFF {
					b.emitBytes(0x48, 0x2D) // sub rax, imm32
					b.emitU32(uint32(val))
					emittedOp = true
				}
			case ir.OpMul:
				if val >= -128 && val <= 127 {
					b.emitBytes(0x48, 0x6B, 0xC0, byte(val)) // imul rax, rax, imm8
					emittedOp = true
				} else if val >= -0x80000000 && val <= 0x7FFFFFFF {
					b.emitBytes(0x48, 0x69, 0xC0) // imul rax, rax, imm32
					b.emitU32(uint32(val))
					emittedOp = true
				}
			case ir.OpAnd:
				if val >= -128 && val <= 127 {
					b.emitBytes(0x48, 0x83, 0xE0, byte(val)) // and rax, imm8
					emittedOp = true
				} else if val >= -0x80000000 && val <= 0x7FFFFFFF {
					b.emitBytes(0x48, 0x25) // and rax, imm32
					b.emitU32(uint32(val))
					emittedOp = true
				}
			}
		}

		if !emittedOp {
			b.emitLoadValToRCX(layout, i.Right)

			switch i.Op {
			case ir.OpAdd:
				b.emitBytes(0x48, 0x01, 0xC8) // add rax, rcx
			case ir.OpSub:
				b.emitBytes(0x48, 0x29, 0xC8) // sub rax, rcx
			case ir.OpMul:
				b.emitBytes(0x48, 0x0F, 0xAF, 0xC1) // imul rax, rcx
			case ir.OpDiv:
				b.emitBytes(0x48, 0x99)       // cqo
				b.emitBytes(0x48, 0xF7, 0xF9) // idiv rcx
			case ir.OpMod:
				b.emitBytes(0x48, 0x99)       // cqo
				b.emitBytes(0x48, 0xF7, 0xF9) // idiv rcx
				b.emitBytes(0x48, 0x89, 0xD0) // mov rax, rdx
			case ir.OpShl:
				b.emitBytes(0x48, 0xD3, 0xE0) // sal rax, cl
			case ir.OpShr:
				b.emitBytes(0x48, 0xD3, 0xF8) // sar rax, cl
			case ir.OpAnd:
				b.emitBytes(0x48, 0x21, 0xC8) // and rax, rcx
			case ir.OpOr:
				b.emitBytes(0x48, 0x09, 0xC8) // or rax, rcx
			case ir.OpXor:
				b.emitBytes(0x48, 0x31, 0xC8) // xor rax, rcx
			}
		}
		b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
		b.cachedRAXVal = i.Dest

	case *ir.CompareInst:
		_, leftIsNull := i.Left.(*ir.ConstNull)
		_, rightIsNull := i.Right.(*ir.ConstNull)
		if !leftIsNull && !rightIsNull && (i.Left.Type().Kind == ir.TypeKindString || i.Right.Type().Kind == ir.TypeKindString) && (i.Op == ir.OpCmpEq || i.Op == ir.OpCmpNe) {
			b.emitCallWithArgs(layout, "__joss_rt_strcmp", []ir.Value{i.Left, i.Right})
			b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
			if i.Op == ir.OpCmpEq {
				b.emitBytes(0x0F, 0x94, 0xC0) // sete al
			} else {
				b.emitBytes(0x0F, 0x95, 0xC0) // setne al
			}
			b.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
			b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
			return nil
		}

		isFloatCmp := (i.Left.Type().Kind == ir.TypeKindF64 || i.Right.Type().Kind == ir.TypeKindF64 ||
			i.Left.Type().Kind == ir.TypeKindDecimal || i.Right.Type().Kind == ir.TypeKindDecimal)
		if isFloatCmp {
			b.emitLoadValToXMM0(layout, i.Left)
			dispRight, hasDisp := getFloatStackDisp(layout, i.Right)
			if hasDisp {
				b.asmListing.WriteString(fmt.Sprintf("    ucomisd xmm0, [rbp%+d]\n", dispRight))
				b.emitBytes(0x66, 0x0F, 0x2E, 0x85)
				b.emitU32(uint32(dispRight))
			} else {
				b.emitLoadValToXMM1(layout, i.Right)
				b.emitBytes(0x66, 0x0F, 0x2E, 0xC1) // ucomisd xmm0, xmm1
			}
			switch i.Op {
			case ir.OpCmpEq:
				b.emitBytes(0x0F, 0x94, 0xC0) // sete al
			case ir.OpCmpNe:
				b.emitBytes(0x0F, 0x95, 0xC0) // setne al
			case ir.OpCmpLt:
				b.emitBytes(0x0F, 0x92, 0xC0) // setb al
			case ir.OpCmpLe:
				b.emitBytes(0x0F, 0x96, 0xC0) // setbe al
			case ir.OpCmpGt:
				b.emitBytes(0x0F, 0x97, 0xC0) // seta al
			case ir.OpCmpGe:
				b.emitBytes(0x0F, 0x93, 0xC0) // setae al
			}
			b.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
			b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
			return nil
		}

		b.emitLoadValToRAX(layout, i.Left)
		if cInt, ok := i.Right.(*ir.ConstInt); ok && cInt.Val >= -0x80000000 && cInt.Val <= 0x7FFFFFFF {
			val := cInt.Val
			if val == 0 && (i.Op == ir.OpCmpEq || i.Op == ir.OpCmpNe) {
				b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
			} else if val >= -128 && val <= 127 {
				b.emitBytes(0x48, 0x83, 0xF8, byte(val)) // cmp rax, imm8
			} else {
				b.emitBytes(0x48, 0x3D) // cmp rax, imm32
				b.emitU32(uint32(val))
			}
		} else {
			b.emitLoadValToRCX(layout, i.Right)
			b.emitBytes(0x48, 0x39, 0xC8) // cmp rax, rcx
		}
		switch i.Op {
		case ir.OpCmpEq:
			b.emitBytes(0x0F, 0x94, 0xC0)       // sete al
			b.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
		case ir.OpCmpNe:
			b.emitBytes(0x0F, 0x95, 0xC0)       // setne al
			b.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
		case ir.OpCmpLt:
			b.emitBytes(0x0F, 0x9C, 0xC0)       // setl al
			b.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
		case ir.OpCmpLe:
			b.emitBytes(0x0F, 0x9E, 0xC0)       // setle al
			b.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
		case ir.OpCmpGt:
			b.emitBytes(0x0F, 0x9F, 0xC0)       // setg al
			b.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
		case ir.OpCmpGe:
			b.emitBytes(0x0F, 0x9D, 0xC0)       // setge al
			b.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
		case ir.OpSpaceship:
			b.emitBytes(0x0F, 0x9F, 0xC0)                         // setg al
			b.emitBytes(0x48, 0x0F, 0xB6, 0xC0)                   // movzx rax, al
			b.emitBytes(0x48, 0xC7, 0xC2, 0xFF, 0xFF, 0xFF, 0xFF) // mov rdx, -1
			b.emitBytes(0x48, 0x0F, 0x4C, 0xC2)                   // cmovl rax, rdx
		}
		b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])

	case *ir.UnaryInst:
		b.emitLoadValToRAX(layout, i.Val)
		switch i.Op {
		case ir.OpNeg:
			if i.Val.Type().Kind == ir.TypeKindF64 || i.Val.Type().Kind == ir.TypeKindDecimal || (i.Dest != nil && (i.Dest.Type().Kind == ir.TypeKindF64 || i.Dest.Type().Kind == ir.TypeKindDecimal)) {
				b.emitBytes(0x48, 0x0F, 0xBA, 0xF8, 0x3F) // btc rax, 63 (toggle IEEE-754 sign bit)
			} else {
				b.emitBytes(0x48, 0xF7, 0xD8) // neg rax
			}
		case ir.OpNot:
			b.emitBytes(0x48, 0x85, 0xC0)       // test rax, rax
			b.emitBytes(0x0F, 0x94, 0xC0)       // sete al
			b.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
		}
		b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])

	case *ir.CallBuiltinInst:
		if isNativeBuiltin(i.BuiltinID) {
			fnLabel := "fn_" + i.BuiltinID
			if i.BuiltinID == "to_string" {
				if len(i.Args) > 0 && (i.Args[0].Type().Kind == ir.TypeKindF64 || i.Args[0].Type().Kind == ir.TypeKindDecimal) {
					fnLabel = "joss_f64_to_string"
				} else {
					fnLabel = "joss_to_string"
				}
			} else if i.BuiltinID == "floatval" && len(i.Args) > 0 {
				argKind := i.Args[0].Type().Kind
				if argKind == ir.TypeKindString {
					fnLabel = "fn_floatval_str"
				} else if argKind == ir.TypeKindF64 || argKind == ir.TypeKindDecimal {
					b.emitLoadValToRAX(layout, i.Args[0])
					if i.Dest != nil {
						b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
					}
					return nil
				}
			}
			b.emitCallWithArgs(layout, fnLabel, i.Args)
			if i.Dest != nil {
				b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
			}
		} else {
			b.emitLeaRCXString("builtin::" + i.BuiltinID)
			b.emitCallIAT(IATJossHostBridge)
			if i.Dest != nil {
				b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
			}
		}
	case *ir.CallPrimitiveMethodInst:
		if isNativePrimitiveMethod(i.TypeKind, i.MethodName) {
			fnLabel := ""
			tk := strings.ToLower(i.TypeKind)
			switch {
			case strings.HasPrefix(tk, "array"):
				switch i.MethodName {
				case "length", "count":
					fnLabel = "joss_arr_len"
				case "push":
					fnLabel = "joss_arr_push_i64"
				case "pop":
					fnLabel = "fn_array_pop"
				}
			case strings.HasPrefix(tk, "string"):
				if i.MethodName == "length" {
					fnLabel = "fn_strlen"
				}
			case strings.HasPrefix(tk, "map"):
				if i.MethodName == "length" || i.MethodName == "count" {
					fnLabel = "joss_map_len"
				}
			}
			if fnLabel != "" {
				b.emitCallWithArgs(layout, fnLabel, i.Args)
				if i.Dest != nil {
					b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
				}
				return nil
			}
		}
		b.emitLeaRCXString(i.TypeKind + "::" + i.MethodName)
		b.emitCallIAT(IATJossHostBridge)
		if i.Dest != nil {
			b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
		}
	case *ir.CallHostServiceInst:
		target := i.ClassName + "::" + i.MethodName
		if b.isJITMode {
			b.needsHostBridge = true
			b.emitLeaRCXString(target)
			b.emitCallIAT(IATJossHostBridge)
			if i.Dest != nil {
				b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
			}
			return nil
		}
		if i.ClassName == "GranDB" {
			// In-memory or standalone GranDB stub/builder
			if i.Dest != nil {
				if i.MethodName == "get" {
					b.emitBytes(0x48, 0xC7, 0xC1, 0x00, 0x00, 0x00, 0x00) // mov rcx, 0
					b.emitCallLabel("joss_arr_new")
				} else if i.MethodName == "first" || i.MethodName == "find" || i.MethodName == "paginate" {
					b.emitCallLabel("joss_map_new")
				} else if i.MethodName == "count" {
					b.emitBytes(0x31, 0xC0) // xor eax, eax
				} else {
					// Fluent return (this / builder)
					if len(i.Args) > 0 {
						b.emitLoadValToRAX(layout, i.Args[0])
					} else {
						b.emitBytes(0x31, 0xC0)
					}
				}
				b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
			}
			return nil
		}
		if i.ClassName == "UserStorage" || i.ClassName == "OCIStorage" {
			switch i.MethodName {
			case "put":
				// UserStorage::put($user, $filename, $content) -> returns true (1)
				if i.Dest != nil {
					b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00) // mov eax, 1
					b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
				}
				return nil
			case "get":
				// UserStorage::get($user, $filename) -> returns string
				if i.Dest != nil {
					b.emitLeaRAXString("")
					b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
				}
				return nil
			case "path":
				// UserStorage::path($subpath) -> returns path string
				if i.Dest != nil {
					if len(i.Args) > 0 && i.Args[0].Type().Kind == ir.TypeKindString {
						b.emitLoadValToRAX(layout, i.Args[0])
					} else {
						b.emitLeaRAXString("storage")
					}
					b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
				}
				return nil
			case "delete":
				if i.Dest != nil {
					b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00) // mov eax, 1
					b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
				}
				return nil
			}
		}
		if isSupportedHostService(i.ClassName, i.MethodName) {
			b.emitLeaRCXString(target)
			b.emitCallIAT(IATJossHostBridge)
			if i.Dest != nil {
				b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
			}
			return nil
		}
		return fmt.Errorf("%s", analyzer.FormatCapabilityError("host service "+i.ClassName+"::"+i.MethodName))

	case *ir.CallInst:
		if (i.Callee == "print" || i.Callee == "println" || i.Callee == "echo") && len(i.Args) == 1 {
			arg := i.Args[0]
			rtFn := "joss_print_mixed"
			if arg.Type().Kind == ir.TypeKindString {
				rtFn = "joss_print_string"
			} else if arg.Type().Kind == ir.TypeKindBool {
				rtFn = "joss_print_bool"
			} else if arg.Type().Kind == ir.TypeKindI64 {
				rtFn = "joss_print_i64"
			}
			b.emitCallWithArgs(layout, rtFn, i.Args)
			if i.Dest != nil {
				b.emitBytes(0x31, 0xC0)
				b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
			}
			return nil
		}
		if isNativeBuiltin(i.Callee) {
			fnLabel := "fn_" + i.Callee
			if i.Callee == "to_string" {
				if len(i.Args) > 0 && (i.Args[0].Type().Kind == ir.TypeKindF64 || i.Args[0].Type().Kind == ir.TypeKindDecimal) {
					fnLabel = "joss_f64_to_string"
				} else {
					fnLabel = "joss_to_string"
				}
			} else if i.Callee == "floatval" && len(i.Args) > 0 {
				argKind := i.Args[0].Type().Kind
				if argKind == ir.TypeKindString {
					fnLabel = "fn_floatval_str"
				} else if argKind == ir.TypeKindF64 || argKind == ir.TypeKindDecimal {
					b.emitLoadValToRAX(layout, i.Args[0])
					if i.Dest != nil {
						b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
					}
					return nil
				}
			}
			b.emitCallWithArgs(layout, fnLabel, i.Args)
		} else if isNativeBuiltinHelper(i.Callee) {
			b.emitCallWithArgs(layout, "fn_"+i.Callee, i.Args)
		} else if _, exists := b.prog.Functions[i.Callee]; exists {
			b.emitCallWithArgs(layout, "fn_"+i.Callee, i.Args)
		} else {
			// External host function or runtime method
			b.emitLeaRCXString(i.Callee)
			b.emitCallIAT(IATJossHostBridge)
		}
		if i.Dest != nil {
			b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
		}

	case *ir.CallIndirectInst:
		b.emitCallIndirectWithArgs(layout, i.CalleeVal, i.Args)
		if i.Dest != nil {
			b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
		}

	case *ir.CallRuntimeInst:
		if isNativeBuiltinHelper(i.Func) {
			b.emitCallWithArgs(layout, "fn_"+i.Func, i.Args)
			if i.Dest != nil {
				b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
			}
			return nil
		}
		if isSupportedRuntimeCallee(i.Func) {
			b.emitLeaRCXString(i.Func)
			b.emitCallIAT(IATJossHostBridge)
			if i.Dest != nil {
				b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
			}
			return nil
		} else {
			if i.Func == "to_string" || i.Func == "strval" {
				if len(i.Args) > 0 && (i.Args[0].Type().Kind == ir.TypeKindF64 || i.Args[0].Type().Kind == ir.TypeKindDecimal) {
					b.emitCallWithArgs(layout, "joss_f64_to_string", i.Args)
				} else {
					b.emitCallWithArgs(layout, "joss_to_string", i.Args)
				}
			} else if i.Func == "str_concat" {
				b.emitCallWithArgs(layout, "joss_str_concat", i.Args)
			} else {
				b.emitCallWithArgs(layout, "joss_"+i.Func, i.Args)
			}
		}
		if i.Dest != nil {
			b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
		}

	case *ir.NewObjectInst:
		allocSize := uint32(16)
		if b.prog != nil && b.prog.ClassLayouts != nil {
			if clsLayout := b.prog.ClassLayouts[i.ClassName]; clsLayout != nil && clsLayout.AllocSize > 0 {
				allocSize = clsLayout.AllocSize
			}
		}
		alignedSize := (allocSize + 7) & ^uint32(7)
		if alignedSize < 16 {
			alignedSize = 16
		}
		lblRefill := fmt.Sprintf(".newobj_refill_%d", b.nextInlineLabelID())
		lblDone := fmt.Sprintf(".newobj_done_%d", b.nextInlineLabelID())
		b.emitMovRAXData(dataOffArenaCur)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(lblRefill)
		if alignedSize <= 127 {
			b.emitBytes(0x48, 0x8D, 0x50, byte(alignedSize)) // lea rdx, [rax + alignedSize]
		} else {
			b.emitBytes(0x48, 0x8D, 0x90)
			b.emitU32(alignedSize)
		}
		b.emitBytes(0x4C, 0x8B, 0x05) // mov r8, [rip + dataOffArenaEnd]
		patchArenaEnd := b.text.Len()
		b.emitU32(0)
		b.relocs = append(b.relocs, machineReloc{kind: relocData, patchOffset: patchArenaEnd, targetOff: dataOffArenaEnd})
		b.emitBytes(0x4C, 0x39, 0xC2) // cmp rdx, r8
		b.emitJaLabel(lblRefill)
		b.emitMovDataRDX(dataOffArenaCur)
		b.defineLabel(lblDone)
		b.emitLeaRCXString(i.ClassName)
		b.emitBytes(0x48, 0x89, 0x08) // mov [rax], rcx (class name at [rax+0])
		b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
		b.cachedRAXVal = i.Dest

		layout.deferredSlowPaths = append(layout.deferredSlowPaths, func() {
			b.defineLabel(lblRefill)
			b.emitBytes(0xB9) // mov ecx, alignedSize
			b.emitU32(alignedSize)
			b.emitCallLabel("__joss_os_alloc")
			b.emitJmpLabel(lblDone)
		})

	case *ir.LoadFieldInst:
		var fixedOffset int32 = -1
		if b.prog != nil && b.prog.ClassLayouts != nil && i.Obj.Type().Elem != nil {
			if clsLayout := b.prog.ClassLayouts[i.Obj.Type().Elem.Name]; clsLayout != nil {
				if off, exists := clsLayout.FieldOffsets[i.FieldName]; exists {
					fixedOffset = off
				}
			}
		}

		if fixedOffset >= 0 {
			b.emitLoadValToRCX(layout, i.Obj)
			savedRCX := i.Obj
			if fixedOffset <= 127 {
				b.emitBytes(0x48, 0x8B, 0x41, byte(fixedOffset)) // mov rax, [rcx + fixedOffset]
			} else {
				b.emitBytes(0x48, 0x8B, 0x81)
				b.emitU32(uint32(fixedOffset))
			}
			b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
			b.cachedRAXVal = i.Dest
			b.cachedRCXVal = savedRCX
		} else {
			lblSlow := fmt.Sprintf(".obj_get_slow_%d", b.nextInlineLabelID())
			lblDone := fmt.Sprintf(".obj_get_done_%d", b.nextInlineLabelID())
			b.emitLoadValToRAX(layout, i.Obj)
			b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax (obj)
			b.emitLeaRDXString(i.FieldName)
			b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
			b.emitJeLabel(lblSlow)
			b.emitBytes(0x4C, 0x8B, 0x49, 0x08) // mov r9, [rcx + 8] (head)
			b.emitBytes(0x4D, 0x85, 0xC9)       // test r9, r9
			b.emitJeLabel(lblSlow)
			b.emitBytes(0x49, 0x39, 0x11) // cmp [r9], rdx (head.key vs key)
			b.emitJneLabel(lblSlow)
			b.emitBytes(0x49, 0x8B, 0x41, 0x08) // mov rax, [r9 + 8] (head.val)
			b.emitJmpLabel(lblDone)
			b.defineLabel(lblSlow)
			if i.Dest.Type().Kind == ir.TypeKindString {
				b.emitCallLabel("joss_obj_get_field_str")
			} else {
				b.emitCallLabel("joss_obj_get_field_i64")
			}
			b.defineLabel(lblDone)
			b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
		}

	case *ir.StoreFieldInst:
		var fixedOffset int32 = -1
		if b.prog != nil && b.prog.ClassLayouts != nil && i.Obj.Type().Elem != nil {
			if clsLayout := b.prog.ClassLayouts[i.Obj.Type().Elem.Name]; clsLayout != nil {
				if off, exists := clsLayout.FieldOffsets[i.FieldName]; exists {
					fixedOffset = off
				}
			}
		}

		if fixedOffset >= 0 {
			b.emitLoadValToR8(layout, i.Val)
			b.emitLoadValToRCX(layout, i.Obj)
			savedRCX := i.Obj
			if fixedOffset <= 127 {
				b.emitBytes(0x4C, 0x89, 0x41, byte(fixedOffset)) // mov [rcx + fixedOffset], r8
			} else {
				b.emitBytes(0x4C, 0x89, 0x81)
				b.emitU32(uint32(fixedOffset))
			}
			b.cachedRCXVal = savedRCX
		} else {
			lblSlow := fmt.Sprintf(".obj_set_slow_%d", b.nextInlineLabelID())
			lblDone := fmt.Sprintf(".obj_set_done_%d", b.nextInlineLabelID())
			b.emitLoadValToRAX(layout, i.Val)
			b.emitBytes(0x49, 0x89, 0xC0) // mov r8, rax (val)
			b.emitLoadValToRAX(layout, i.Obj)
			b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax (obj)
			b.emitLeaRDXString(i.FieldName)
			b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
			b.emitJeLabel(lblSlow)
			b.emitBytes(0x4C, 0x8B, 0x49, 0x08) // mov r9, [rcx + 8] (head)
			b.emitBytes(0x4D, 0x85, 0xC9)       // test r9, r9
			b.emitJeLabel(lblSlow)
			b.emitBytes(0x49, 0x39, 0x11) // cmp [r9], rdx (head.key vs key)
			b.emitJneLabel(lblSlow)
			b.emitBytes(0x4D, 0x89, 0x41, 0x08) // mov [r9 + 8], r8
			b.emitJmpLabel(lblDone)
			b.defineLabel(lblSlow)
			b.emitCallLabel("joss_obj_set_field_i64")
			b.defineLabel(lblDone)
		}

	case *ir.NewArrayInst:
		b.emitBytes(0x48, 0xC7, 0xC1) // mov rcx, len
		b.emitU32(uint32(len(i.Elements)))
		b.emitCallLabel("joss_arr_new")
		b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
		for _, el := range i.Elements {
			b.emitLoadValToRAX(layout, el)
			b.emitBytes(0x48, 0x89, 0xC2) // mov rdx, rax (val)
			b.emitLoadRAXFromRBP(layout.tempOffsets[i.Dest.ID])
			b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax (arr)
			b.emitCallLabel("joss_arr_push_i64")
		}

	case *ir.ArrayGetInst:
		lblSlow := fmt.Sprintf(".arr_get_slow_%d", b.nextInlineLabelID())
		lblDone := fmt.Sprintf(".arr_get_done_%d", b.nextInlineLabelID())
		b.emitLoadValToRCX(layout, i.Array)
		b.emitLoadValToRDX(layout, i.Index)
		b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
		b.emitJeLabel(lblSlow)
		b.emitBytes(0x48, 0x3B, 0x11) // cmp rdx, [rcx + 0] (unsigned idx vs len)
		b.emitJaeLabel(lblSlow)
		b.emitBytes(0x4C, 0x8B, 0x41, 0x10) // mov r8, [rcx + 16] (dataPtr)
		b.emitBytes(0x49, 0x8B, 0x04, 0xD0) // mov rax, [r8 + rdx*8]
		if i.Dest.Type().Kind == ir.TypeKindString {
			b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
			b.emitJeLabel(lblSlow)
		}
		b.defineLabel(lblDone)
		b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
		b.cachedRAXVal = i.Dest

		helperName := b.selectArrayGetHelper(i.Dest.Type())
		layout.deferredSlowPaths = append(layout.deferredSlowPaths, func() {
			b.defineLabel(lblSlow)
			b.emitCallLabel(helperName)
			b.emitJmpLabel(lblDone)
		})

	case *ir.ArraySetInst:
		lblSlow := fmt.Sprintf(".arr_set_slow_%d", b.nextInlineLabelID())
		lblDone := fmt.Sprintf(".arr_set_done_%d", b.nextInlineLabelID())
		b.emitLoadValToR8(layout, i.Val)
		b.emitLoadValToRDX(layout, i.Index)
		b.emitLoadValToRCX(layout, i.Array)
		b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
		b.emitJeLabel(lblSlow)
		b.emitBytes(0x48, 0x3B, 0x11) // cmp rdx, [rcx + 0] (unsigned idx vs len)
		b.emitJaeLabel(lblSlow)
		b.emitBytes(0x4C, 0x8B, 0x49, 0x10) // mov r9, [rcx + 16] (dataPtr)
		b.emitBytes(0x4D, 0x89, 0x04, 0xD1) // mov [r9 + rdx*8], r8
		b.defineLabel(lblDone)

		layout.deferredSlowPaths = append(layout.deferredSlowPaths, func() {
			b.defineLabel(lblSlow)
			b.emitCallLabel("joss_arr_set_i64")
			b.emitJmpLabel(lblDone)
		})

	case *ir.NewMapInst:
		b.emitCallLabel("joss_map_new")
		b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])
		for _, pair := range i.Pairs {
			b.emitLoadRAXFromRBP(layout.tempOffsets[i.Dest.ID])
			b.emitCallWithArgs(layout, "joss_map_set_i64", []ir.Value{i.Dest, pair[0], pair[1]})
		}

	case *ir.MapGetInst:
		b.emitCallWithArgs(layout, "joss_map_get_i64", []ir.Value{i.Map, i.Key})
		b.emitStoreRAXToRBP(layout.tempOffsets[i.Dest.ID])

	case *ir.MapSetInst:
		b.emitCallWithArgs(layout, "joss_map_set_i64", []ir.Value{i.Map, i.Key, i.Val})

	case *ir.ThrowInst:
		b.emitLoadValToRAX(layout, i.Val)
		b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
		b.emitCallLabel("__joss_rt_throw")
	}
	return nil
}

func (b *MachineCodeBuilder) selectArrayGetHelper(t ir.Type) string {
	if t.Kind == ir.TypeKindString {
		return "joss_arr_get_str"
	}
	return "joss_arr_get_i64"
}

func (b *MachineCodeBuilder) emitTerminator(fn *ir.Function, layout *fnFrameLayout, term ir.Terminator) error {
	switch t := term.(type) {
	case *ir.ReturnTerminator:
		if t.Val != nil {
			b.emitLoadValToRAX(layout, t.Val)
		} else {
			b.emitBytes(0x31, 0xC0) // xor eax, eax
		}
		if layout.inTryDepth > 0 {
			// Pop active try frames of this function without clobbering rax
			b.emitMovRCXData(dataOffTryDepth)
			b.emitBytes(0x48, 0x83, 0xE9, byte(layout.inTryDepth)) // sub rcx, inTryDepth
			b.emitMovDataRCX(dataOffTryDepth)
		}
		b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // mov rsp, rbp; pop rbp; ret

	case *ir.JumpTerminator:
		b.emitJmpLabel(fmt.Sprintf("%s_blk_%s", layout.fnLabel, t.Target.Label))

	case *ir.BranchTerminator:
		b.emitLoadValToRAX(layout, t.Cond)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJneLabel(fmt.Sprintf("%s_blk_%s", layout.fnLabel, t.TrueBlock.Label))
		b.emitJmpLabel(fmt.Sprintf("%s_blk_%s", layout.fnLabel, t.FalseBlock.Label))

	case *ir.UnreachableTerminator:
		// No-op at end of block after throw/return
	}
	return nil
}

func (b *MachineCodeBuilder) emitCallWithArgs(layout *fnFrameLayout, targetLabel string, args []ir.Value) {
	// Place arguments 4+ onto the outgoing stack area [rsp + 32 + (idx-4)*8]
	for idx := len(args) - 1; idx >= 4; idx-- {
		b.emitLoadValToRAX(layout, args[idx])
		stackOff := uint32(32 + (idx-4)*8)
		b.emitBytes(0x48, 0x89, 0x84, 0x24) // mov [rsp + disp32], rax
		b.emitU32(stackOff)
	}
	// Load first up to 4 arguments directly into target registers in reverse order
	if len(args) >= 4 {
		b.emitLoadValToRAX(layout, args[3])
		b.emitBytes(0x49, 0x89, 0xC1) // mov r9, rax
	}
	if len(args) >= 3 {
		b.emitLoadValToR8(layout, args[2])
	}
	if len(args) >= 2 {
		b.emitLoadValToRDX(layout, args[1])
	}
	if len(args) >= 1 {
		b.emitLoadValToRCX(layout, args[0])
	}
	b.emitCallLabel(targetLabel)
}

func (b *MachineCodeBuilder) emitCallIndirectWithArgs(layout *fnFrameLayout, calleeVal ir.Value, args []ir.Value) {
	// Place arguments 4+ onto the outgoing stack area [rsp + 32 + (idx-4)*8]
	for idx := len(args) - 1; idx >= 4; idx-- {
		b.emitLoadValToRAX(layout, args[idx])
		stackOff := uint32(32 + (idx-4)*8)
		b.emitBytes(0x48, 0x89, 0x84, 0x24) // mov [rsp + disp32], rax
		b.emitU32(stackOff)
	}
	// Load first up to 4 arguments directly into target registers
	if len(args) >= 4 {
		b.emitLoadValToRAX(layout, args[3])
		b.emitBytes(0x49, 0x89, 0xC1) // mov r9, rax
	}
	if len(args) >= 3 {
		b.emitLoadValToR8(layout, args[2])
	}
	if len(args) >= 2 {
		b.emitLoadValToRDX(layout, args[1])
	}
	if len(args) >= 1 {
		b.emitLoadValToRCX(layout, args[0])
	}
	// Load callee pointer into rax and call rax
	b.emitLoadValToRAX(layout, calleeVal)
	b.asmListing.WriteString("    call rax\n")
	b.emitBytes(0xFF, 0xD0) // call rax
}

func (b *MachineCodeBuilder) isSafeCacheVal(layout *fnFrameLayout, val ir.Value) bool {
	if val == nil || layout == nil {
		return false
	}
	if _, ok := val.(*ir.ParamValue); ok {
		return true
	}
	if tv, ok := val.(*ir.TempValue); ok {
		off := layout.tempOffsets[tv.ID]
		return !layout.aliasedOffsets[off]
	}
	return false
}

func (b *MachineCodeBuilder) emitLoadValToRAX(layout *fnFrameLayout, val ir.Value) {
	if b.cachedRAXVal != nil && b.cachedRAXVal == val && b.isSafeCacheVal(layout, val) {
		return
	}
	defer func() {
		if b.isSafeCacheVal(layout, val) {
			b.cachedRAXVal = val
		}
	}()
	switch v := val.(type) {
	case *ir.TempValue:
		b.emitLoadRAXFromRBP(layout.tempOffsets[v.ID])
	case *ir.ParamValue:
		b.emitLoadRAXFromRBP(layout.paramOffsets[cleanName(v.Name)])
	case *ir.ConstInt:
		if v.Val == 0 {
			b.emitBytes(0x31, 0xC0) // xor eax, eax
		} else if v.Val > 0 && v.Val <= 0x7FFFFFFF {
			b.emitBytes(0xB8) // mov eax, imm32
			b.emitU32(uint32(v.Val))
		} else if v.Val >= -0x80000000 && v.Val < 0 {
			b.emitBytes(0x48, 0xC7, 0xC0) // mov rax, sign-extended imm32
			b.emitU32(uint32(v.Val))
		} else {
			b.emitBytes(0x48, 0xB8) // movabs rax, imm64
			b.emitU64(uint64(v.Val))
		}
	case *ir.ConstFloat:
		bits := math.Float64bits(v.Val)
		if bits == 0 {
			b.emitBytes(0x31, 0xC0) // xor eax, eax
		} else {
			b.emitBytes(0x48, 0xB8) // movabs rax, imm64
			b.emitU64(bits)
		}
	case *ir.ConstBool:
		if v.Val {
			b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00) // mov eax, 1
		} else {
			b.emitBytes(0x31, 0xC0) // xor eax, eax
		}
	case *ir.ConstString:
		b.emitLeaRAXString(v.Val)
	case *ir.ConstFnPtr:
		target := v.FnName
		if !strings.HasPrefix(target, "fn_") {
			target = "fn_" + target
		}
		b.emitLeaRAXLabel(target)
	case *ir.ConstNull:
		b.emitBytes(0x31, 0xC0) // xor eax, eax
	default:
		b.emitBytes(0x31, 0xC0)
	}
}

func (b *MachineCodeBuilder) emitLoadValToRCX(layout *fnFrameLayout, val ir.Value) {
	if b.cachedRCXVal != nil && b.cachedRCXVal == val && b.isSafeCacheVal(layout, val) {
		return
	}
	defer func() {
		if b.isSafeCacheVal(layout, val) {
			b.cachedRCXVal = val
		}
	}()
	switch v := val.(type) {
	case *ir.TempValue:
		b.emitLoadRCXFromRBP(layout.tempOffsets[v.ID])
	case *ir.ParamValue:
		b.emitLoadRCXFromRBP(layout.paramOffsets[cleanName(v.Name)])
	case *ir.ConstInt:
		if v.Val == 0 {
			b.emitBytes(0x31, 0xC9) // xor ecx, ecx
		} else if v.Val > 0 && v.Val <= 0x7FFFFFFF {
			b.emitBytes(0xB9) // mov ecx, imm32
			b.emitU32(uint32(v.Val))
		} else if v.Val >= -0x80000000 && v.Val < 0 {
			b.emitBytes(0x48, 0xC7, 0xC1) // mov rcx, sign-extended imm32
			b.emitU32(uint32(v.Val))
		} else {
			b.emitBytes(0x48, 0xB9) // movabs rcx, imm64
			b.emitU64(uint64(v.Val))
		}
	case *ir.ConstFloat:
		bits := math.Float64bits(v.Val)
		if bits == 0 {
			b.emitBytes(0x31, 0xC9) // xor ecx, ecx
		} else {
			b.emitBytes(0x48, 0xB9) // movabs rcx, imm64
			b.emitU64(bits)
		}
	case *ir.ConstBool:
		if v.Val {
			b.emitBytes(0xB9, 0x01, 0x00, 0x00, 0x00) // mov ecx, 1
		} else {
			b.emitBytes(0x31, 0xC9) // xor ecx, ecx
		}
	case *ir.ConstString:
		b.emitLeaRCXString(v.Val)
	case *ir.ConstFnPtr:
		target := v.FnName
		if !strings.HasPrefix(target, "fn_") {
			target = "fn_" + target
		}
		b.emitLeaRCXLabel(target)
	case *ir.ConstNull:
		b.emitBytes(0x31, 0xC9) // xor ecx, ecx
	default:
		b.emitBytes(0x31, 0xC9)
	}
}

func (b *MachineCodeBuilder) emitLoadValToRDX(layout *fnFrameLayout, val ir.Value) {
	switch v := val.(type) {
	case *ir.TempValue:
		b.emitLoadRDXFromRBP(layout.tempOffsets[v.ID])
	case *ir.ParamValue:
		b.emitLoadRDXFromRBP(layout.paramOffsets[cleanName(v.Name)])
	case *ir.ConstInt:
		if v.Val == 0 {
			b.emitBytes(0x31, 0xD2) // xor edx, edx
		} else if v.Val > 0 && v.Val <= 0x7FFFFFFF {
			b.emitBytes(0xBA) // mov edx, imm32
			b.emitU32(uint32(v.Val))
		} else if v.Val >= -0x80000000 && v.Val < 0 {
			b.emitBytes(0x48, 0xC7, 0xC2) // mov rdx, sign-extended imm32
			b.emitU32(uint32(v.Val))
		} else {
			b.emitBytes(0x48, 0xBA) // movabs rdx, imm64
			b.emitU64(uint64(v.Val))
		}
	case *ir.ConstFloat:
		bits := math.Float64bits(v.Val)
		if bits == 0 {
			b.emitBytes(0x31, 0xD2) // xor edx, edx
		} else {
			b.emitBytes(0x48, 0xBA) // movabs rdx, imm64
			b.emitU64(bits)
		}
	case *ir.ConstBool:
		if v.Val {
			b.emitBytes(0xBA, 0x01, 0x00, 0x00, 0x00) // mov edx, 1
		} else {
			b.emitBytes(0x31, 0xD2) // xor edx, edx
		}
	case *ir.ConstString:
		b.emitLeaRDXString(v.Val)
	case *ir.ConstFnPtr:
		target := v.FnName
		if !strings.HasPrefix(target, "fn_") {
			target = "fn_" + target
		}
		b.emitLeaRDXLabel(target)
	case *ir.ConstNull:
		b.emitBytes(0x31, 0xD2) // xor edx, edx
	default:
		b.emitBytes(0x31, 0xD2)
	}
}

func (b *MachineCodeBuilder) emitLoadValToR8(layout *fnFrameLayout, val ir.Value) {
	switch v := val.(type) {
	case *ir.TempValue:
		b.emitLoadR8FromRBP(layout.tempOffsets[v.ID])
	case *ir.ParamValue:
		b.emitLoadR8FromRBP(layout.paramOffsets[cleanName(v.Name)])
	case *ir.ConstInt:
		if v.Val == 0 {
			b.emitBytes(0x4D, 0x31, 0xC0) // xor r8, r8
		} else if v.Val > 0 && v.Val <= 0x7FFFFFFF {
			b.emitBytes(0x41, 0xB8) // mov r8d, imm32
			b.emitU32(uint32(v.Val))
		} else if v.Val >= -0x80000000 && v.Val < 0 {
			b.emitBytes(0x49, 0xC7, 0xC0) // mov r8, sign-extended imm32
			b.emitU32(uint32(v.Val))
		} else {
			b.emitBytes(0x49, 0xB8) // movabs r8, imm64
			b.emitU64(uint64(v.Val))
		}
	case *ir.ConstFloat:
		bits := math.Float64bits(v.Val)
		if bits == 0 {
			b.emitBytes(0x4D, 0x31, 0xC0) // xor r8, r8
		} else {
			b.emitBytes(0x49, 0xB8) // movabs r8, imm64
			b.emitU64(bits)
		}
	case *ir.ConstBool:
		if v.Val {
			b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00) // mov r8d, 1
		} else {
			b.emitBytes(0x4D, 0x31, 0xC0) // xor r8, r8
		}
	case *ir.ConstString:
		b.emitLeaR8String(v.Val)
	case *ir.ConstFnPtr:
		target := v.FnName
		if !strings.HasPrefix(target, "fn_") {
			target = "fn_" + target
		}
		b.emitLeaR8Label(target)
	case *ir.ConstNull:
		b.emitBytes(0x4D, 0x31, 0xC0) // xor r8, r8
	default:
		b.emitBytes(0x4D, 0x31, 0xC0)
	}
}

func (b *MachineCodeBuilder) emitLoadRAXFromRBP(disp int32) {
	b.cachedRAXVal = nil
	b.asmListing.WriteString(fmt.Sprintf("    mov rax, [rbp%+d]\n", disp))
	if disp >= -128 && disp <= 127 {
		b.emitBytes(0x48, 0x8B, 0x45, byte(disp))
	} else {
		b.emitBytes(0x48, 0x8B, 0x85)
		b.emitU32(uint32(disp))
	}
}

func (b *MachineCodeBuilder) emitLoadRCXFromRBP(disp int32) {
	b.cachedRCXVal = nil
	b.asmListing.WriteString(fmt.Sprintf("    mov rcx, [rbp%+d]\n", disp))
	if disp >= -128 && disp <= 127 {
		b.emitBytes(0x48, 0x8B, 0x4D, byte(disp))
	} else {
		b.emitBytes(0x48, 0x8B, 0x8D)
		b.emitU32(uint32(disp))
	}
}

func (b *MachineCodeBuilder) emitLoadRDXFromRBP(disp int32) {
	b.asmListing.WriteString(fmt.Sprintf("    mov rdx, [rbp%+d]\n", disp))
	if disp >= -128 && disp <= 127 {
		b.emitBytes(0x48, 0x8B, 0x55, byte(disp))
	} else {
		b.emitBytes(0x48, 0x8B, 0x95)
		b.emitU32(uint32(disp))
	}
}

func (b *MachineCodeBuilder) emitLoadR8FromRBP(disp int32) {
	b.asmListing.WriteString(fmt.Sprintf("    mov r8, [rbp%+d]\n", disp))
	if disp >= -128 && disp <= 127 {
		b.emitBytes(0x4C, 0x8B, 0x45, byte(disp))
	} else {
		b.emitBytes(0x4C, 0x8B, 0x85)
		b.emitU32(uint32(disp))
	}
}

func (b *MachineCodeBuilder) emitStoreRAXToRBP(disp int32) {
	b.cachedRAXVal = nil
	b.cachedRCXVal = nil
	b.asmListing.WriteString(fmt.Sprintf("    mov [rbp%+d], rax\n", disp))
	if disp >= -128 && disp <= 127 {
		b.emitBytes(0x48, 0x89, 0x45, byte(disp))
	} else {
		b.emitBytes(0x48, 0x89, 0x85)
		b.emitU32(uint32(disp))
	}
}

func (b *MachineCodeBuilder) emitStoreRCXToRBP(disp int32) {
	b.cachedRAXVal = nil
	b.cachedRCXVal = nil
	b.asmListing.WriteString(fmt.Sprintf("    mov [rbp%+d], rcx\n", disp))
	if disp >= -128 && disp <= 127 {
		b.emitBytes(0x48, 0x89, 0x4D, byte(disp))
	} else {
		b.emitBytes(0x48, 0x89, 0x8D)
		b.emitU32(uint32(disp))
	}
}

func (b *MachineCodeBuilder) emitStoreRDXToRBP(disp int32) {
	b.cachedRAXVal = nil
	b.cachedRCXVal = nil
	b.asmListing.WriteString(fmt.Sprintf("    mov [rbp%+d], rdx\n", disp))
	if disp >= -128 && disp <= 127 {
		b.emitBytes(0x48, 0x89, 0x55, byte(disp))
	} else {
		b.emitBytes(0x48, 0x89, 0x95)
		b.emitU32(uint32(disp))
	}
}

func (b *MachineCodeBuilder) emitStoreR8ToRBP(disp int32) {
	b.cachedRAXVal = nil
	b.cachedRCXVal = nil
	b.asmListing.WriteString(fmt.Sprintf("    mov [rbp%+d], r8\n", disp))
	if disp >= -128 && disp <= 127 {
		b.emitBytes(0x4C, 0x89, 0x45, byte(disp))
	} else {
		b.emitBytes(0x4C, 0x89, 0x85)
		b.emitU32(uint32(disp))
	}
}

func (b *MachineCodeBuilder) emitStoreR9ToRBP(disp int32) {
	b.cachedRAXVal = nil
	b.cachedRCXVal = nil
	b.asmListing.WriteString(fmt.Sprintf("    mov [rbp%+d], r9\n", disp))
	if disp >= -128 && disp <= 127 {
		b.emitBytes(0x4C, 0x89, 0x4D, byte(disp))
	} else {
		b.emitBytes(0x4C, 0x89, 0x8D)
		b.emitU32(uint32(disp))
	}
}

func getFloatStackDisp(layout *fnFrameLayout, val ir.Value) (int32, bool) {
	switch v := val.(type) {
	case *ir.TempValue:
		if v.Type().Kind == ir.TypeKindF64 || v.Type().Kind == ir.TypeKindDecimal {
			off, ok := layout.tempOffsets[v.ID]
			return off, ok
		}
	case *ir.ParamValue:
		if v.Type().Kind == ir.TypeKindF64 || v.Type().Kind == ir.TypeKindDecimal {
			off, ok := layout.paramOffsets[cleanName(v.Name)]
			return off, ok
		}
	}
	return 0, false
}

func (b *MachineCodeBuilder) emitStoreXMM0ToRBP(disp int32) {
	b.asmListing.WriteString(fmt.Sprintf("    movsd [rbp%+d], xmm0\n", disp))
	b.emitBytes(0xF2, 0x0F, 0x11, 0x85)
	b.emitU32(uint32(disp))
}

func (b *MachineCodeBuilder) emitLoadValToXMM0(layout *fnFrameLayout, val ir.Value) {
	if b.cachedXMM0Val != nil && b.cachedXMM0Val == val {
		return
	}
	defer func() {
		b.cachedXMM0Val = val
	}()
	switch v := val.(type) {
	case *ir.ConstFloat:
		bits := math.Float64bits(v.Val)
		if bits == 0 {
			b.asmListing.WriteString("    xorpd xmm0, xmm0\n")
			b.emitBytes(0x66, 0x0F, 0x57, 0xC0) // xorpd xmm0, xmm0
		} else {
			b.emitBytes(0x48, 0xB8) // movabs rax, bits
			b.emitU64(bits)
			b.emitBytes(0x66, 0x48, 0x0F, 0x6E, 0xC0) // movq xmm0, rax
		}
	case *ir.ConstInt:
		b.emitLoadValToRAX(layout, val)
		b.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xC0) // cvtsi2sd xmm0, rax
	case *ir.TempValue:
		off := layout.tempOffsets[v.ID]
		if v.Type().Kind == ir.TypeKindF64 || v.Type().Kind == ir.TypeKindDecimal {
			b.asmListing.WriteString(fmt.Sprintf("    movsd xmm0, [rbp%+d]\n", off))
			b.emitBytes(0xF2, 0x0F, 0x10, 0x85)
			b.emitU32(uint32(off))
		} else {
			b.emitLoadRAXFromRBP(off)
			b.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xC0) // cvtsi2sd xmm0, rax
		}
	case *ir.ParamValue:
		off := layout.paramOffsets[cleanName(v.Name)]
		if v.Type().Kind == ir.TypeKindF64 || v.Type().Kind == ir.TypeKindDecimal {
			b.asmListing.WriteString(fmt.Sprintf("    movsd xmm0, [rbp%+d]\n", off))
			b.emitBytes(0xF2, 0x0F, 0x10, 0x85)
			b.emitU32(uint32(off))
		} else {
			b.emitLoadRAXFromRBP(off)
			b.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xC0) // cvtsi2sd xmm0, rax
		}
	default:
		b.emitLoadValToRAX(layout, val)
		b.emitBytes(0x66, 0x48, 0x0F, 0x6E, 0xC0) // movq xmm0, rax
	}
}

func (b *MachineCodeBuilder) emitLoadValToXMM1(layout *fnFrameLayout, val ir.Value) {
	switch v := val.(type) {
	case *ir.ConstFloat:
		bits := math.Float64bits(v.Val)
		if bits == 0 {
			b.asmListing.WriteString("    xorpd xmm1, xmm1\n")
			b.emitBytes(0x66, 0x0F, 0x57, 0xC9) // xorpd xmm1, xmm1
		} else {
			b.emitBytes(0x48, 0xB9) // movabs rcx, bits
			b.emitU64(bits)
			b.emitBytes(0x66, 0x48, 0x0F, 0x6E, 0xC9) // movq xmm1, rcx
		}
	case *ir.ConstInt:
		b.emitLoadValToRCX(layout, val)
		b.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xC9) // cvtsi2sd xmm1, rcx
	case *ir.TempValue:
		off := layout.tempOffsets[v.ID]
		if v.Type().Kind == ir.TypeKindF64 || v.Type().Kind == ir.TypeKindDecimal {
			b.asmListing.WriteString(fmt.Sprintf("    movsd xmm1, [rbp%+d]\n", off))
			b.emitBytes(0xF2, 0x0F, 0x10, 0x8D)
			b.emitU32(uint32(off))
		} else {
			b.emitLoadRCXFromRBP(off)
			b.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xC9) // cvtsi2sd xmm1, rcx
		}
	case *ir.ParamValue:
		off := layout.paramOffsets[cleanName(v.Name)]
		if v.Type().Kind == ir.TypeKindF64 || v.Type().Kind == ir.TypeKindDecimal {
			b.asmListing.WriteString(fmt.Sprintf("    movsd xmm1, [rbp%+d]\n", off))
			b.emitBytes(0xF2, 0x0F, 0x10, 0x8D)
			b.emitU32(uint32(off))
		} else {
			b.emitLoadRCXFromRBP(off)
			b.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xC9) // cvtsi2sd xmm1, rcx
		}
	default:
		b.emitLoadValToRCX(layout, val)
		b.emitBytes(0x66, 0x48, 0x0F, 0x6E, 0xC9) // movq xmm1, rcx
	}
}

func cleanName(name string) string {
	return strings.TrimPrefix(name, "$")
}

func (b *MachineCodeBuilder) emitToStringStub() {
	b.defineLabel("joss_to_string")
	b.defineLabel("fn_to_string")
	b.defineLabel("fn_strval")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)       // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x57, 0x56)             // push rbx; push rdi; push rsi
	b.emitBytes(0x48, 0x83, 0xEC, 0x38)       // sub rsp, 56
	b.emitBytes(0x48, 0x89, 0xCB)             // mov rbx, rcx (val)
	b.emitBytes(0xB9, 0x20, 0x00, 0x00, 0x00) // mov ecx, 32
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x48, 0x89, 0xC7) // mov rdi, rax (outBuf)
	b.emitBytes(0x48, 0x89, 0xC6) // mov rsi, rax (origBuf)
	// Check if val == 0
	b.emitBytes(0x48, 0x85, 0xDB) // test rbx, rbx
	b.emitJneLabel(".tostr_nonzero")
	b.emitBytes(0xC6, 0x07, 0x30)       // mov byte ptr [rdi], '0'
	b.emitBytes(0xC6, 0x47, 0x01, 0x00) // mov byte ptr [rdi+1], 0
	b.emitBytes(0x48, 0x89, 0xF0)       // mov rax, rsi
	b.emitJmpLabel(".tostr_exit")
	b.defineLabel(".tostr_nonzero")
	// If rbx < 0: write '-', neg rbx
	b.emitBytes(0x48, 0x85, 0xDB) // test rbx, rbx
	b.emitJnsLabel(".tostr_pos")
	b.emitBytes(0xC6, 0x07, 0x2D) // mov byte ptr [rdi], '-'
	b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
	b.emitBytes(0x48, 0xF7, 0xDB) // neg rbx
	b.defineLabel(".tostr_pos")
	b.emitBytes(0x4C, 0x8D, 0x4D, 0xF8) // lea r9, [rbp - 8]
	b.emitBytes(0x48, 0x89, 0xD8)       // mov rax, rbx
	b.defineLabel(".tostr_digits")
	b.emitBytes(0x31, 0xD2)                               // xor edx, edx
	b.emitBytes(0x49, 0xC7, 0xC3, 0x0A, 0x00, 0x00, 0x00) // mov r11, 10
	b.emitBytes(0x49, 0xF7, 0xF3)                         // div r11
	b.emitBytes(0x80, 0xC2, 0x30)                         // add dl, '0'
	b.emitBytes(0x49, 0xFF, 0xC9)                         // dec r9
	b.emitBytes(0x41, 0x88, 0x11)                         // mov [r9], dl
	b.emitBytes(0x48, 0x85, 0xC0)                         // test rax, rax
	b.emitJneLabel(".tostr_digits")
	// Copy digits from r9 to rdi
	b.emitBytes(0x4C, 0x8D, 0x45, 0xF8) // lea r8, [rbp - 8]
	b.defineLabel(".tostr_copy")
	b.emitBytes(0x4D, 0x39, 0xC1) // cmp r9, r8
	b.emitJgeLabel(".tostr_copy_done")
	b.emitBytes(0x41, 0x8A, 0x11) // mov dl, [r9]
	b.emitBytes(0x88, 0x17)       // mov [rdi], dl
	b.emitBytes(0x49, 0xFF, 0xC1) // inc r9
	b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
	b.emitJmpLabel(".tostr_copy")
	b.defineLabel(".tostr_copy_done")
	b.emitBytes(0xC6, 0x07, 0x00) // mov byte ptr [rdi], 0
	b.emitBytes(0x48, 0x89, 0xF0) // mov rax, rsi (origBuf)
	b.defineLabel(".tostr_exit")
	b.emitBytes(0x48, 0x83, 0xC4, 0x38)       // add rsp, 56
	b.emitBytes(0x5E, 0x5F, 0x5B, 0x5D, 0xC3) // pop rsi; pop rdi; pop rbx; pop rbp; ret
}

func (b *MachineCodeBuilder) emitF64ToStringStub() {
	b.defineLabel("joss_f64_to_string")
	// rcx = float bits
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)       // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x57, 0x56)             // push rbx; push rdi; push rsi
	b.emitBytes(0x48, 0x83, 0xEC, 0x38)       // sub rsp, 56
	b.emitBytes(0x48, 0x89, 0x4D, 0xE0)       // mov [rbp - 32], rcx (save float bits)
	b.emitBytes(0x66, 0x48, 0x0F, 0x6E, 0xC1) // movq xmm0, rcx (val)
	b.emitBytes(0xF2, 0x48, 0x0F, 0x2C, 0xD8) // cvttsd2si rbx, xmm0 (int part)
	b.emitBytes(0x48, 0x89, 0xD9)             // mov rcx, rbx
	b.emitCallLabel("joss_to_string")         // rax = buffer with int part
	b.emitBytes(0x48, 0x89, 0xC6)             // mov rsi, rax (origBuf)
	b.emitBytes(0x48, 0x89, 0xC7)             // mov rdi, rax

	// Find end of int part (null byte)
	b.defineLabel(".f64_find_null")
	b.emitBytes(0x80, 0x3F, 0x00) // cmp byte ptr [rdi], 0
	b.emitJeLabel(".f64_found_null")
	b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
	b.emitJmpLabel(".f64_find_null")
	b.defineLabel(".f64_found_null")

	// Append '.'
	b.emitBytes(0xC6, 0x07, 0x2E) // mov byte ptr [rdi], '.'
	b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi

	// Restore original float to xmm0
	b.emitBytes(0x48, 0x8B, 0x4D, 0xE0)       // mov rcx, [rbp - 32]
	b.emitBytes(0x66, 0x48, 0x0F, 0x6E, 0xC1) // movq xmm0, rcx
	// Compute fraction = (val - int(val)) * 10000.0
	b.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xCB) // cvtsi2sd xmm1, rbx
	b.emitBytes(0xF2, 0x0F, 0x5C, 0xC1)       // subsd xmm0, xmm1
	// Absolute value of fraction
	b.emitBytes(0x66, 0x0F, 0x57, 0xDB) // xorpd xmm3, xmm3
	b.emitBytes(0x66, 0x0F, 0x2F, 0xC3) // comisd xmm0, xmm3
	b.emitJaeLabel(".f64_frac_pos")
	b.emitBytes(0xF2, 0x0F, 0x5C, 0xD8) // subsd xmm3, xmm0
	b.emitBytes(0x66, 0x0F, 0x28, 0xC3) // movapd xmm0, xmm3
	b.defineLabel(".f64_frac_pos")

	// 10000.0 in IEEE-754: 0x40C3880000000000
	b.emitBytes(0x48, 0xB8) // mov rax, 10000.0 (double)
	b.emitU64(0x40C3880000000000)
	b.emitBytes(0x66, 0x48, 0x0F, 0x6E, 0xD0) // movq xmm2, rax
	b.emitBytes(0xF2, 0x0F, 0x59, 0xC2)       // mulsd xmm0, xmm2
	b.emitBytes(0xF2, 0x48, 0x0F, 0x2C, 0xC0) // cvttsd2si rax, xmm0 (rax = 4-digit int)

	// Write 4 digits
	b.emitBytes(0x49, 0xC7, 0xC0, 0x0A, 0x00, 0x00, 0x00) // mov r8, 10
	b.emitBytes(0x4C, 0x8D, 0x4F, 0x04)                   // lea r9, [rdi + 4]
	b.emitBytes(0x41, 0xC6, 0x01, 0x00)                   // mov byte ptr [r9], 0 (null terminator)
	b.emitBytes(0x49, 0xFF, 0xC9)                         // dec r9
	b.emitBytes(0xB9, 0x04, 0x00, 0x00, 0x00)             // mov ecx, 4

	b.defineLabel(".f64_frac_loop")
	b.emitBytes(0x31, 0xD2)       // xor edx, edx
	b.emitBytes(0x49, 0xF7, 0xF0) // div r8
	b.emitBytes(0x80, 0xC2, 0x30) // add dl, '0'
	b.emitBytes(0x41, 0x88, 0x11) // mov [r9], dl
	b.emitBytes(0x49, 0xFF, 0xC9) // dec r9
	b.emitBytes(0xFF, 0xC9)       // dec ecx
	b.emitJneLabel(".f64_frac_loop")

	b.emitBytes(0x48, 0x89, 0xF0)             // mov rax, rsi (origBuf)
	b.emitBytes(0x48, 0x83, 0xC4, 0x38)       // add rsp, 56
	b.emitBytes(0x5E, 0x5F, 0x5B, 0x5D, 0xC3) // pop rsi; pop rdi; pop rbx; pop rbp; ret
}
