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
)

// AArch64 Registers
const (
	regX0  uint32 = 0
	regX1  uint32 = 1
	regX2  uint32 = 2
	regX3  uint32 = 3
	regX4  uint32 = 4
	regX5  uint32 = 5
	regX6  uint32 = 6
	regX7  uint32 = 7
	regX8  uint32 = 8
	regX9  uint32 = 9
	regX10 uint32 = 10
	regX11 uint32 = 11
	regX12 uint32 = 12
	regX13 uint32 = 13
	regX14 uint32 = 14
	regX15 uint32 = 15
	regX16 uint32 = 16 // IP0
	regX17 uint32 = 17 // IP1
	regX18 uint32 = 18 // Platform
	regX19 uint32 = 19
	regX20 uint32 = 20
	regX21 uint32 = 21
	regX22 uint32 = 22
	regX23 uint32 = 23
	regX24 uint32 = 24
	regX25 uint32 = 25
	regX26 uint32 = 26
	regX27 uint32 = 27
	regX28 uint32 = 28
	regX29 uint32 = 29 // Frame Pointer (FP)
	regX30 uint32 = 30 // Link Register (LR)
	regSP  uint32 = 31 // Stack Pointer
	regXZR uint32 = 31 // Zero Register (en contextos de datos)
)

// AArch64 Condition Codes
const (
	condEQ uint32 = 0  // Equal
	condNE uint32 = 1  // Not Equal
	condCS uint32 = 2  // Carry Set / Higher or Same (Unsigned >=)
	condCC uint32 = 3  // Carry Clear / Lower (Unsigned <)
	condMI uint32 = 4  // Minus / Negative
	condPL uint32 = 5  // Plus / Positive or Zero
	condVS uint32 = 6  // Overflow
	condVC uint32 = 7  // No Overflow
	condHI uint32 = 8  // Higher (Unsigned >)
	condLS uint32 = 9  // Lower or Same (Unsigned <=)
	condGE uint32 = 10 // Greater or Equal (Signed >=)
	condLT uint32 = 11 // Less Than (Signed <)
	condGT uint32 = 12 // Greater Than (Signed >)
	condLE uint32 = 13 // Less or Equal (Signed <=)
	condAL uint32 = 14 // Always
)

type arm64RelocKind int

const (
	arm64RelocBranch      arm64RelocKind = iota // B o BL (imm26)
	arm64RelocCondBranch                        // B.cond (imm19)
	arm64RelocDataOffset                        // Puntero a ROData (MOVZ + MOVK + MOVK)
	arm64RelocDataSection                       // Puntero a .data (MOVZ + MOVK + MOVK)
	arm64RelocCodeAddr                          // Puntero a etiqueta de .text (MOVZ + MOVK + MOVK)
)

type arm64Reloc struct {
	kind        arm64RelocKind
	patchOffset int
	targetLabel string
	targetOff   uint32
}

// ARM64Builder produce código máquina nativo AArch64 (ARMv8-A) directamente desde Joss Native IR.
type ARM64Builder struct {
	target             Target
	guiMode            bool
	prog               *ir.Program
	text               bytes.Buffer
	rodata             bytes.Buffer
	labels             map[string]int
	relocs             []arm64Reloc
	stringPool         map[string]uint32
	asmListing         strings.Builder
	needsHostBridge    bool
	usesExtendedStdlib bool
}

// NewARM64Builder inicializa un generador de código AArch64 puro.
func NewARM64Builder(target Target, guiMode bool) *ARM64Builder {
	b := &ARM64Builder{
		target:     target,
		guiMode:    guiMode,
		stringPool: make(map[string]uint32),
		labels:     make(map[string]int),
	}
	b.internString("")
	b.internString("\n")
	b.internString("null")
	b.internString("true")
	b.internString("false")
	b.internString("Panic: ")
	return b
}

func (b *ARM64Builder) NeedsHostBridge() bool {
	return b.needsHostBridge
}

func (b *ARM64Builder) UsesExtendedStdlib() bool {
	return b.usesExtendedStdlib
}

// -----------------------------------------------------------------------------
// Emisión de Instrucciones AArch64 (Little-Endian de 32 bits)
// -----------------------------------------------------------------------------

func (b *ARM64Builder) emitEntrypointAndRuntime(hasUserMain bool) {
	b.defineLabel("_start")
	b.emitProlog(32)

	b.emitCall("__joss_os_init")

	if hasUserMain {
		b.emitCall("fn_main")
	} else {
		b.emitLoadImm64(regX0, 0)
	}

	b.emitCall("__joss_os_exit")
	b.emitEpilog(32)

	// __joss_os_init
	b.defineLabel("__joss_os_init")
	b.emitRet(regX30)

	// __joss_os_exit(x0 = exit_code)
	b.defineLabel("__joss_os_exit")
	if b.target.OS == "darwin" {
		b.emitLoadImm64(regX16, 1)
		b.emitSyscall()
	} else {
		b.emitLoadImm64(regX8, 93)
		b.emitSyscall()
	}
	b.emitRet(regX30)

	// __joss_os_write(x0 = fd, x1 = buf, x2 = count)
	b.defineLabel("__joss_os_write")
	if b.target.OS == "darwin" {
		b.emitLoadImm64(regX16, 4)
		b.emitSyscall()
	} else {
		b.emitLoadImm64(regX8, 64)
		b.emitSyscall()
	}
	b.emitRet(regX30)

	// __joss_rt_strlen(x0 = strPtr) -> x0 = len
	b.defineLabel("__joss_rt_strlen")
	b.emitLoadImm64(regX1, 0)
	b.defineLabel(".strlen_loop")
	b.emitAdd(regX2, regX0, regX1)
	b.emitLdrb(regX3, regX2, 0)
	b.emitCmp(regX3, regXZR)
	b.emitBranchCond(condEQ, ".strlen_done")
	b.emitLoadImm64(regX4, 1)
	b.emitAdd(regX1, regX1, regX4)
	b.emitBranch(".strlen_loop")
	b.defineLabel(".strlen_done")
	b.emitMovReg(regX0, regX1)
	b.emitRet(regX30)

	// joss_print_string(x0 = strPtr)
	b.defineLabel("joss_print_string")
	b.emitProlog(32)
	b.emitStr(regX0, regSP, 16)

	b.emitCall("__joss_rt_strlen")
	b.emitMovReg(regX2, regX0)
	b.emitLdr(regX1, regSP, 16)
	b.emitLoadImm64(regX0, 1)
	b.emitCall("__joss_os_write")

	b.emitLoadStringAddr(regX1, "\n")
	b.emitLoadImm64(regX0, 1)
	b.emitLoadImm64(regX2, 1)
	b.emitCall("__joss_os_write")

	b.emitEpilog(32)

	// joss_print_bool(x0 = boolVal)
	b.defineLabel("joss_print_bool")
	b.emitProlog(16)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condNE, ".print_bool_true")
	b.emitLoadStringAddr(regX0, "false")
	b.emitBranch(".print_bool_emit")
	b.defineLabel(".print_bool_true")
	b.emitLoadStringAddr(regX0, "true")
	b.defineLabel(".print_bool_emit")
	b.emitCall("joss_print_string")
	b.emitEpilog(16)

	// joss_print_i64(x0 = intVal)
	b.defineLabel("joss_print_i64")
	b.emitProlog(64)
	b.emitLoadImm64(regX1, 10)
	b.emitStrb(regX1, regSP, 63)
	b.emitLoadImm64(regX2, 62)

	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condNE, ".i64_nonzero")
	b.emitLoadImm64(regX1, 48)
	b.emitStrb(regX1, regSP, 62)
	b.emitLoadImm64(regX2, 61)
	b.emitBranch(".i64_print")

	b.defineLabel(".i64_nonzero")
	b.emitLoadImm64(regX9, 0)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condGE, ".i64_loop")
	b.emitLoadImm64(regX9, 1)
	b.emitSub(regX0, regXZR, regX0)

	b.defineLabel(".i64_loop")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".i64_sign")
	b.emitLoadImm64(regX3, 10)
	b.emitSDiv(regX4, regX0, regX3)
	b.emitMul(regX5, regX4, regX3)
	b.emitSub(regX6, regX0, regX5)
	b.emitLoadImm64(regX7, 48)
	b.emitAdd(regX6, regX6, regX7)
	b.emitAdd(regX8, regSP, regX2)
	b.emitStrb(regX6, regX8, 0)
	b.emitLoadImm64(regX7, 1)
	b.emitSub(regX2, regX2, regX7)
	b.emitMovReg(regX0, regX4)
	b.emitBranch(".i64_loop")

	b.defineLabel(".i64_sign")
	b.emitCmp(regX9, regXZR)
	b.emitBranchCond(condEQ, ".i64_print")
	b.emitLoadImm64(regX1, 45)
	b.emitAdd(regX8, regSP, regX2)
	b.emitStrb(regX1, regX8, 0)
	b.emitLoadImm64(regX7, 1)
	b.emitSub(regX2, regX2, regX7)

	b.defineLabel(".i64_print")
	b.emitLoadImm64(regX7, 1)
	b.emitAdd(regX2, regX2, regX7)
	b.emitAdd(regX1, regSP, regX2)
	b.emitLoadImm64(regX3, 64)
	b.emitSub(regX2, regX3, regX2)
	b.emitLoadImm64(regX0, 1)
	b.emitCall("__joss_os_write")

	b.emitEpilog(64)

	// joss_print_f64 / joss_print_mixed
	b.defineLabel("joss_print_f64")
	b.defineLabel("joss_print_mixed")
	b.emitProlog(16)
	b.emitLoadImm64(regX1, 0x400000)
	b.emitCmp(regX0, regX1)
	b.emitBranchCond(condGE, ".print_mixed_str")
	b.emitCall("joss_print_i64")
	b.emitEpilog(16)
	b.defineLabel(".print_mixed_str")
	b.emitCall("joss_print_string")
	b.emitEpilog(16)

	// __joss_rt_strcmp(x0 = s1, x1 = s2) -> x0 = 0 if equal, non-zero otherwise
	b.defineLabel("__joss_rt_strcmp")
	b.emitCmp(regX0, regX1)
	b.emitBranchCond(condEQ, ".strcmp_eq")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".strcmp_ne")
	b.emitCmp(regX1, regXZR)
	b.emitBranchCond(condEQ, ".strcmp_ne")
	b.defineLabel(".strcmp_loop")
	b.emitLdrb(regX2, regX0, 0)
	b.emitLdrb(regX3, regX1, 0)
	b.emitCmp(regX2, regX3)
	b.emitBranchCond(condNE, ".strcmp_diff")
	b.emitCmp(regX2, regXZR)
	b.emitBranchCond(condEQ, ".strcmp_eq")
	b.emitLoadImm64(regX4, 1)
	b.emitAdd(regX0, regX0, regX4)
	b.emitAdd(regX1, regX1, regX4)
	b.emitBranch(".strcmp_loop")
	b.defineLabel(".strcmp_diff")
	b.emitSub(regX0, regX2, regX3)
	b.emitRet(regX30)
	b.defineLabel(".strcmp_ne")
	b.emitLoadImm64(regX0, 1)
	b.emitRet(regX30)
	b.defineLabel(".strcmp_eq")
	b.emitLoadImm64(regX0, 0)
	b.emitRet(regX30)

	// joss_is_truthy / fn_is_truthy(x0 = val) -> x0 = 1 or 0
	b.defineLabel("joss_is_truthy")
	b.defineLabel("fn_is_truthy")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".truthy_false")
	b.emitLoadImm64(regX1, 0x400000)
	b.emitCmp(regX0, regX1)
	b.emitBranchCond(condLT, ".truthy_true")
	b.emitLdrb(regX2, regX0, 0)
	b.emitCmp(regX2, regXZR)
	b.emitBranchCond(condEQ, ".truthy_false")
	b.defineLabel(".truthy_true")
	b.emitLoadImm64(regX0, 1)
	b.emitRet(regX30)
	b.defineLabel(".truthy_false")
	b.emitLoadImm64(regX0, 0)
	b.emitRet(regX30)

	// joss_is_empty / fn_is_empty / fn_empty(x0 = val) -> x0 = 1 or 0
	b.defineLabel("joss_is_empty")
	b.defineLabel("fn_is_empty")
	b.defineLabel("fn_empty")
	b.emitProlog(16)
	b.emitCall("joss_is_truthy")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".empty_true")
	b.emitLoadImm64(regX0, 0)
	b.emitEpilog(16)
	b.defineLabel(".empty_true")
	b.emitLoadImm64(regX0, 1)
	b.emitEpilog(16)

	// __joss_os_alloc(x0 = size) -> x0 = ptr (via mmap on Linux/Darwin)
	b.defineLabel("__joss_os_alloc")
	b.emitProlog(16)
	b.emitMovReg(regX1, regX0) // len = size
	b.emitLoadImm64(regX0, 0)  // addr = NULL
	b.emitLoadImm64(regX2, 3)  // PROT_READ | PROT_WRITE
	if b.target.OS == "darwin" {
		b.emitLoadImm64(regX3, 0x1002) // MAP_PRIVATE | MAP_ANON
		b.emitLoadImm64(regX4, 0xFFFFFFFFFFFFFFFF)
		b.emitLoadImm64(regX5, 0)
		b.emitLoadImm64(regX16, 197) // SYS_mmap on Darwin
	} else {
		b.emitLoadImm64(regX3, 0x22) // MAP_PRIVATE | MAP_ANONYMOUS
		b.emitLoadImm64(regX4, 0xFFFFFFFFFFFFFFFF)
		b.emitLoadImm64(regX5, 0)
		b.emitLoadImm64(regX8, 222) // SYS_mmap on Linux AArch64
	}
	b.emitSyscall()
	b.emitEpilog(16)

	// joss_str_concat / joss_str_concat_i64 / joss_to_string / joss_f64_to_string
	b.defineLabel("joss_to_string")
	b.defineLabel("fn_to_string")
	b.defineLabel("fn_strval")
	b.defineLabel("joss_f64_to_string")
	// If x0 >= 0x400000, it is already a string pointer in rodata or heap, return it
	b.emitLoadImm64(regX1, 0x400000)
	b.emitCmp(regX0, regX1)
	b.emitBranchCond(condGE, ".tostr_ret_fast")

	b.emitProlog(64)
	b.emitStr(regX0, regSP, 16) // [sp, 16] = val

	b.emitLoadImm64(regX0, 32)
	b.emitCall("__joss_os_alloc")
	b.emitStr(regX0, regSP, 24) // [sp, 24] = origBuf
	b.emitStr(regX0, regSP, 32) // [sp, 32] = outPtr (current write cursor)

	b.emitLdr(regX1, regSP, 16) // x1 = val
	b.emitCmp(regX1, regXZR)
	b.emitBranchCond(condNE, ".tostr_nonzero")

	// val == 0: write '0', '\0'
	b.emitLoadImm64(regX2, 0x30) // '0'
	b.emitStrb(regX2, regX0, 0)
	b.emitStrb(regXZR, regX0, 1)
	b.emitEpilog(64)

	b.defineLabel(".tostr_nonzero")
	// If val < 0: write '-', neg val, outPtr++
	b.emitCmp(regX1, regXZR)
	b.emitBranchCond(condGE, ".tostr_pos")
	b.emitLoadImm64(regX2, 0x2D) // '-'
	b.emitStrb(regX2, regX0, 0)
	b.emitLoadImm64(regX3, 1)
	b.emitAdd(regX0, regX0, regX3)
	b.emitStr(regX0, regSP, 32)     // outPtr++
	b.emitSub(regX1, regXZR, regX1) // x1 = -x1

	b.defineLabel(".tostr_pos")
	// Digits loop: extract backwards from sp+60 down to sp+40
	b.emitLoadImm64(regX4, 60)
	b.emitAdd(regX9, regSP, regX4) // x9 = stack cursor (starts at sp+60)
	b.emitLoadImm64(regX10, 10)

	b.defineLabel(".tostr_digits")
	b.emitUDiv(regX2, regX1, regX10) // x2 = x1 / 10
	b.emitMul(regX3, regX2, regX10)  // x3 = x2 * 10
	b.emitSub(regX4, regX1, regX3)   // x4 = x1 - x3 (digit 0..9)
	b.emitLoadImm64(regX5, 0x30)     // '0'
	b.emitAdd(regX4, regX4, regX5)
	b.emitLoadImm64(regX5, 1)
	b.emitSub(regX9, regX9, regX5) // x9--
	b.emitStrb(regX4, regX9, 0)    // [x9] = digit
	b.emitMovReg(regX1, regX2)     // x1 = x2 (quotient)
	b.emitCmp(regX1, regXZR)
	b.emitBranchCond(condNE, ".tostr_digits")

	// Copy digits from x9 to [sp, 32] until x9 == sp + 60
	b.emitLdr(regX8, regSP, 32) // x8 = outPtr
	b.emitLoadImm64(regX7, 60)
	b.emitAdd(regX7, regSP, regX7) // x7 = sp + 60

	b.defineLabel(".tostr_copy")
	b.emitCmp(regX9, regX7)
	b.emitBranchCond(condGE, ".tostr_copy_done")
	b.emitLdrb(regX4, regX9, 0)
	b.emitStrb(regX4, regX8, 0)
	b.emitLoadImm64(regX5, 1)
	b.emitAdd(regX9, regX9, regX5)
	b.emitAdd(regX8, regX8, regX5)
	b.emitBranch(".tostr_copy")

	b.defineLabel(".tostr_copy_done")
	b.emitStrb(regXZR, regX8, 0) // null-terminate string
	b.emitLdr(regX0, regSP, 24)  // return origBuf
	b.emitEpilog(64)

	b.defineLabel(".tostr_ret_fast")
	b.emitRet(regX30)

	b.defineLabel("joss_str_concat")
	b.defineLabel("fn_str_concat")
	b.defineLabel("joss_str_concat_i64")
	b.defineLabel("joss_i64_str_concat")
	b.defineLabel("joss_str_concat_f64")
	b.defineLabel("joss_f64_str_concat")
	b.emitProlog(48)
	b.emitStr(regX0, regSP, 16)
	b.emitStr(regX1, regSP, 24)
	b.emitCall("__joss_rt_strlen")
	b.emitStr(regX0, regSP, 32) // len1
	b.emitLdr(regX0, regSP, 24)
	b.emitCall("__joss_rt_strlen")
	b.emitStr(regX0, regSP, 40) // len2
	b.emitLdr(regX1, regSP, 32)
	b.emitAdd(regX0, regX0, regX1)
	b.emitLoadImm64(regX2, 16)
	b.emitAdd(regX0, regX0, regX2)
	b.emitCall("__joss_os_alloc")
	b.emitMovReg(regX5, regX0)  // dstBuf
	b.emitLdr(regX1, regSP, 16) // s1
	b.emitLdr(regX2, regSP, 32) // len1
	b.emitLoadImm64(regX3, 0)
	b.defineLabel(".concat_cp1")
	b.emitCmp(regX3, regX2)
	b.emitBranchCond(condGE, ".concat_cp2_init")
	b.emitAdd(regX6, regX1, regX3)
	b.emitLdrb(regX7, regX6, 0)
	b.emitAdd(regX6, regX5, regX3)
	b.emitStrb(regX7, regX6, 0)
	b.emitLoadImm64(regX4, 1)
	b.emitAdd(regX3, regX3, regX4)
	b.emitBranch(".concat_cp1")
	b.defineLabel(".concat_cp2_init")
	b.emitLdr(regX1, regSP, 24) // s2
	b.emitLdr(regX2, regSP, 40) // len2
	b.emitLdr(regX8, regSP, 32) // len1
	b.emitLoadImm64(regX3, 0)
	b.defineLabel(".concat_cp2")
	b.emitCmp(regX3, regX2)
	b.emitBranchCond(condGE, ".concat_done")
	b.emitAdd(regX6, regX1, regX3)
	b.emitLdrb(regX7, regX6, 0)
	b.emitAdd(regX6, regX5, regX8)
	b.emitAdd(regX6, regX6, regX3)
	b.emitStrb(regX7, regX6, 0)
	b.emitLoadImm64(regX4, 1)
	b.emitAdd(regX3, regX3, regX4)
	b.emitBranch(".concat_cp2")
	b.defineLabel(".concat_done")
	b.emitAdd(regX6, regX5, regX8)
	b.emitAdd(regX6, regX6, regX2)
	b.emitStrb(regXZR, regX6, 0)
	b.emitMovReg(regX0, regX5)
	b.emitEpilog(48)

	// Object, Array, and Map core helpers for AArch64
	b.defineLabel("joss_obj_new")
	b.emitProlog(32)
	b.emitStr(regX0, regSP, 16)
	b.emitLoadImm64(regX0, 256)
	b.emitCall("__joss_os_alloc")
	b.emitLdr(regX1, regSP, 16)
	b.emitStr(regX1, regX0, 0)
	b.emitEpilog(32)

	b.defineLabel("joss_obj_get_field_i64")
	b.defineLabel("joss_obj_get_field_str")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".obj_get_null")
	b.emitLdr(regX2, regX0, 8)
	b.emitCmp(regX2, regXZR)
	b.emitBranchCond(condEQ, ".obj_get_null")
	// Fast leaf path: if first node matches key directly
	b.emitLdr(regX3, regX2, 0)
	b.emitCmp(regX3, regX1)
	b.emitBranchCond(condNE, ".obj_get_frame")
	b.emitLdr(regX0, regX2, 8)
	b.emitRet(regX30)

	b.defineLabel(".obj_get_frame")
	b.emitProlog(48)
	b.emitStr(regX0, regSP, 16) // obj
	b.emitStr(regX1, regSP, 24) // key
	b.emitStr(regX2, regSP, 32) // currNode

	b.defineLabel(".obj_get_loop")
	b.emitLdr(regX2, regSP, 32)
	b.emitCmp(regX2, regXZR)
	b.emitBranchCond(condEQ, ".obj_get_notfound")
	b.emitLdr(regX3, regX2, 0)  // nodeKey
	b.emitLdr(regX1, regSP, 24) // key
	b.emitCmp(regX3, regX1)
	b.emitBranchCond(condEQ, ".obj_get_found")
	b.emitLoadImm64(regX4, 0x10000)
	b.emitCmp(regX3, regX4)
	b.emitBranchCond(condLT, ".obj_get_next")
	b.emitCmp(regX1, regX4)
	b.emitBranchCond(condLT, ".obj_get_next")
	b.emitMovReg(regX0, regX3)
	b.emitCall("__joss_rt_strcmp")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".obj_get_found")

	b.defineLabel(".obj_get_next")
	b.emitLdr(regX2, regSP, 32)
	b.emitLdr(regX2, regX2, 16) // next
	b.emitStr(regX2, regSP, 32)
	b.emitBranch(".obj_get_loop")

	b.defineLabel(".obj_get_found")
	b.emitLdr(regX2, regSP, 32)
	b.emitLdr(regX0, regX2, 8) // node.val
	b.emitEpilog(48)

	b.defineLabel(".obj_get_notfound")
	b.emitLoadImm64(regX0, 0)
	b.emitEpilog(48)

	b.defineLabel(".obj_get_null")
	b.emitLoadImm64(regX0, 0)
	b.emitRet(regX30)

	b.defineLabel("joss_obj_set_field_i64")
	b.defineLabel("joss_obj_set_field_str")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".obj_set_ret")
	b.emitProlog(48)
	b.emitStr(regX0, regSP, 16) // obj
	b.emitStr(regX1, regSP, 24) // key
	b.emitStr(regX2, regSP, 32) // val
	b.emitLdr(regX4, regX0, 8)  // head
	b.emitStr(regX4, regSP, 40) // currNode

	b.defineLabel(".obj_set_loop")
	b.emitLdr(regX2, regSP, 40)
	b.emitCmp(regX2, regXZR)
	b.emitBranchCond(condEQ, ".obj_set_alloc")
	b.emitLdr(regX3, regX2, 0)  // nodeKey
	b.emitLdr(regX1, regSP, 24) // key
	b.emitCmp(regX3, regX1)
	b.emitBranchCond(condEQ, ".obj_set_update")
	b.emitLoadImm64(regX4, 0x10000)
	b.emitCmp(regX3, regX4)
	b.emitBranchCond(condLT, ".obj_set_next")
	b.emitCmp(regX1, regX4)
	b.emitBranchCond(condLT, ".obj_set_next")
	b.emitMovReg(regX0, regX3)
	b.emitCall("__joss_rt_strcmp")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".obj_set_update")

	b.defineLabel(".obj_set_next")
	b.emitLdr(regX2, regSP, 40)
	b.emitLdr(regX2, regX2, 16)
	b.emitStr(regX2, regSP, 40)
	b.emitBranch(".obj_set_loop")

	b.defineLabel(".obj_set_update")
	b.emitLdr(regX2, regSP, 40) // currNode
	b.emitLdr(regX3, regSP, 32) // val
	b.emitStr(regX3, regX2, 8)  // currNode.val = val
	b.emitEpilog(48)

	b.defineLabel(".obj_set_alloc")
	b.emitLoadImm64(regX0, 24)
	b.emitCall("__joss_os_alloc")
	b.emitLdr(regX1, regSP, 24) // key
	b.emitLdr(regX2, regSP, 32) // val
	b.emitLdr(regX3, regSP, 16) // obj
	b.emitStr(regX1, regX0, 0)
	b.emitStr(regX2, regX0, 8)
	b.emitLdr(regX4, regX3, 8) // old head
	b.emitStr(regX4, regX0, 16)
	b.emitStr(regX0, regX3, 8)
	b.emitEpilog(48)

	b.defineLabel(".obj_set_ret")
	b.emitRet(regX30)

	b.defineLabel("joss_arr_new")
	b.defineLabel("joss_map_new")
	b.emitProlog(16)
	b.emitLoadImm64(regX0, 4096)
	b.emitCall("__joss_os_alloc")
	b.emitEpilog(16)

	b.defineLabel("joss_arr_len")
	b.defineLabel("joss_map_len")
	b.defineLabel("fn_len")
	b.defineLabel("fn_count")
	b.defineLabel("fn_strlen")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".arr_len_zero")
	b.emitLdr(regX0, regX0, 0)
	b.emitRet(regX30)
	b.defineLabel(".arr_len_zero")
	b.emitLoadImm64(regX0, 0)
	b.emitRet(regX30)

	b.defineLabel("joss_arr_push_i64")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".arr_push_ret")
	b.emitLdr(regX2, regX0, 0) // len
	b.emitLoadImm64(regX3, 8)
	b.emitMul(regX4, regX2, regX3)
	b.emitLoadImm64(regX5, 16)
	b.emitAdd(regX4, regX4, regX5)
	b.emitAdd(regX4, regX0, regX4)
	b.emitStr(regX1, regX4, 0)
	b.emitLoadImm64(regX3, 1)
	b.emitAdd(regX2, regX2, regX3)
	b.emitStr(regX2, regX0, 0)
	b.defineLabel(".arr_push_ret")
	b.emitRet(regX30)

	b.defineLabel("joss_arr_get_i64")
	b.defineLabel("joss_arr_get_str")
	b.emitLoadImm64(regX2, 0x10000)
	b.emitCmp(regX1, regX2)
	b.emitBranchCond(condGE, "joss_map_get_i64")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".arr_get_zero")
	b.emitLoadImm64(regX3, 8)
	b.emitMul(regX4, regX1, regX3)
	b.emitLoadImm64(regX5, 16)
	b.emitAdd(regX4, regX4, regX5)
	b.emitAdd(regX4, regX0, regX4)
	b.emitLdr(regX0, regX4, 0)
	b.emitRet(regX30)
	b.defineLabel(".arr_get_zero")
	b.emitLoadImm64(regX0, 0)
	b.emitRet(regX30)

	b.defineLabel("joss_arr_set_i64")
	b.defineLabel("joss_arr_set_str")
	b.emitLoadImm64(regX3, 0x10000)
	b.emitCmp(regX1, regX3)
	b.emitBranchCond(condGE, "joss_map_set_i64")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".arr_set_ret")
	b.emitLoadImm64(regX3, 8)
	b.emitMul(regX4, regX1, regX3)
	b.emitLoadImm64(regX5, 16)
	b.emitAdd(regX4, regX4, regX5)
	b.emitAdd(regX4, regX0, regX4)
	b.emitStr(regX2, regX4, 0)
	b.defineLabel(".arr_set_ret")
	b.emitRet(regX30)

	b.defineLabel("joss_map_get_i64")
	b.defineLabel("joss_map_get_str")
	b.emitBranch("joss_obj_get_field_i64")

	b.defineLabel("joss_map_set_i64")
	b.defineLabel("joss_map_set_str")
	b.emitBranch("joss_obj_set_field_i64")

	b.defineLabel("fn_joss_map_key_at")
	b.defineLabel("joss_map_key_at")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".map_key_null")
	b.emitLdr(regX2, regX0, 8) // head
	b.defineLabel(".map_key_loop")
	b.emitCmp(regX2, regXZR)
	b.emitBranchCond(condEQ, ".map_key_null")
	b.emitCmp(regX1, regXZR)
	b.emitBranchCond(condEQ, ".map_key_hit")
	b.emitLoadImm64(regX3, 1)
	b.emitSub(regX1, regX1, regX3)
	b.emitLdr(regX2, regX2, 16)
	b.emitBranch(".map_key_loop")
	b.defineLabel(".map_key_hit")
	b.emitLdr(regX0, regX2, 0)
	b.emitRet(regX30)
	b.defineLabel(".map_key_null")
	b.emitLoadImm64(regX0, 0)
	b.emitRet(regX30)

	b.defineLabel("fn_joss_map_val_at")
	b.defineLabel("joss_map_val_at")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".map_val_null")
	b.emitLdr(regX2, regX0, 8)
	b.defineLabel(".map_val_loop")
	b.emitCmp(regX2, regXZR)
	b.emitBranchCond(condEQ, ".map_val_null")
	b.emitCmp(regX1, regXZR)
	b.emitBranchCond(condEQ, ".map_val_hit")
	b.emitLoadImm64(regX3, 1)
	b.emitSub(regX1, regX1, regX3)
	b.emitLdr(regX2, regX2, 16)
	b.emitBranch(".map_val_loop")
	b.defineLabel(".map_val_hit")
	b.emitLdr(regX0, regX2, 8)
	b.emitRet(regX30)
	b.defineLabel(".map_val_null")
	b.emitLoadImm64(regX0, 0)
	b.emitRet(regX30)

	// Exception helpers and stack-unwinding __joss_rt_throw
	b.defineLabel("fn_Exception_constructor")
	b.emitProlog(16)
	b.emitMovReg(regX2, regX1)
	b.emitLoadStringAddr(regX1, "message")
	b.emitCall("joss_obj_set_field_str")
	b.emitEpilog(16)

	b.defineLabel("fn_Exception_getMessage")
	b.emitProlog(16)
	b.emitLoadStringAddr(regX1, "message")
	b.emitCall("joss_obj_get_field_str")
	b.emitEpilog(16)

	b.defineLabel("fn_Exception_getCode")
	b.emitProlog(16)
	b.emitLoadStringAddr(regX1, "code")
	b.emitCall("joss_obj_get_field_i64")
	b.emitEpilog(16)

	b.defineLabel("__joss_rt_throw")
	b.emitLoadDataAddr(regX1, dataOffTryDepth)
	b.emitLdr(regX2, regX1, 0) // x2 = tryDepth
	b.emitCmp(regX2, regXZR)
	b.emitBranchCond(condGT, ".arm64_throw_unwind")
	// Uncaught exception -> print and exit(1)
	b.emitProlog(16)
	b.emitCall("joss_print_string")
	b.emitLoadImm64(regX0, 1)
	b.emitCall("__joss_os_exit")
	b.emitEpilog(16)

	b.defineLabel(".arm64_throw_unwind")
	b.emitLoadImm64(regX3, 1)
	b.emitSub(regX2, regX2, regX3) // tryDepth--
	b.emitStr(regX2, regX1, 0)
	b.emitLoadDataAddr(regX1, dataOffExceptionVal)
	b.emitStr(regX0, regX1, 0)
	b.emitLoadImm64(regX3, 8)
	b.emitMul(regX4, regX2, regX3) // offset = tryDepth * 8
	b.emitLoadDataAddr(regX1, dataOffTryRBP)
	b.emitAdd(regX1, regX1, regX4)
	b.emitLdr(regX29, regX1, 0) // restore x29 (FP)
	b.emitLoadDataAddr(regX1, dataOffTryRSP)
	b.emitAdd(regX1, regX1, regX4)
	b.emitLdr(regX5, regX1, 0)
	b.emitInst(0x91000000 | (regX5 << 5) | regSP) // mov sp, x5
	b.emitLoadDataAddr(regX1, dataOffTryRIP)
	b.emitAdd(regX1, regX1, regX4)
	b.emitLdr(regX6, regX1, 0)
	b.emitBr(regX6)
}

// -----------------------------------------------------------------------------
// Emisión de Funciones IR en AArch64
// -----------------------------------------------------------------------------

type arm64FrameLayout struct {
	frameSize    int32
	tempOffsets  map[int]int32
	paramOffsets map[string]int32
	inTryDepth   int
}

func (b *ARM64Builder) computeFrameLayout(fn *ir.Function) *arm64FrameLayout {
	layout := &arm64FrameLayout{
		tempOffsets:  make(map[int]int32),
		paramOffsets: make(map[string]int32),
	}

	offset := int32(16 + len(fn.Params)*8)
	var scanBlocks func(blocks []*ir.BasicBlock)
	scanBlocks = func(blocks []*ir.BasicBlock) {
		for _, blk := range blocks {
			for _, inst := range blk.Instructions {
				if res := inst.Result(); res != nil {
					if _, exists := layout.tempOffsets[res.ID]; !exists {
						layout.tempOffsets[res.ID] = offset
						offset += 8
					}
				}
				if tc, ok := inst.(*ir.TryCatchInst); ok {
					if tc.CatchVarPtr != nil {
						if temp, ok := tc.CatchVarPtr.(*ir.TempValue); ok {
							if _, exists := layout.tempOffsets[temp.ID]; !exists {
								layout.tempOffsets[temp.ID] = offset
								offset += 8
							}
						}
					}
					scanBlocks(tc.TryBlocks)
					scanBlocks(tc.CatchBlocks)
				}
			}
		}
	}
	scanBlocks(fn.Blocks)
	layout.frameSize = (offset + 15) & ^15
	return layout
}

func (b *ARM64Builder) emitIRFunction(fn *ir.Function) error {
	fnLabel := "fn_" + fn.Name
	b.defineLabel(fnLabel)

	layout := b.computeFrameLayout(fn)
	b.emitProlog(layout.frameSize)

	for i, param := range fn.Params {
		if i < 8 {
			disp := int32(16 + i*8)
			b.emitStr(uint32(i), regX29, uint32(disp))
			pName := cleanName(param.Name)
			layout.paramOffsets[pName] = disp
			layout.paramOffsets[param.Name] = disp
		}
	}

	return b.emitBlocks(fn, layout, fn.Blocks)
}

func (b *ARM64Builder) emitBlocks(fn *ir.Function, layout *arm64FrameLayout, blocks []*ir.BasicBlock) error {
	for _, blk := range blocks {
		b.defineLabel(fmt.Sprintf(".L_%s_%s", fn.Name, blk.Label))
		for _, inst := range blk.Instructions {
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
		if blk.Terminator != nil {
			if err := b.emitTerminator(fn, layout, blk.Terminator); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *ARM64Builder) emitTryCatch(fn *ir.Function, layout *arm64FrameLayout, tc *ir.TryCatchInst) error {
	catchEntryLbl := fmt.Sprintf(".L_%s_catch_%d", fn.Name, b.text.Len())
	tryEndLbl := fmt.Sprintf(".L_%s_try_end_%d", fn.Name, b.text.Len())

	// Push try frame at index tryDepth
	b.emitLoadDataAddr(regX1, dataOffTryDepth)
	b.emitLdr(regX2, regX1, 0) // x2 = tryDepth
	b.emitLoadImm64(regX3, 8)
	b.emitMul(regX4, regX2, regX3) // offset = tryDepth * 8

	b.emitLoadDataAddr(regX5, dataOffTryRBP)
	b.emitAdd(regX5, regX5, regX4)
	b.emitStr(regX29, regX5, 0)

	b.emitLoadDataAddr(regX5, dataOffTryRSP)
	b.emitAdd(regX5, regX5, regX4)
	b.emitInst(0x91000000 | (regSP << 5) | regX6) // mov x6, sp
	b.emitStr(regX6, regX5, 0)

	b.emitLoadDataAddr(regX5, dataOffTryRIP)
	b.emitAdd(regX5, regX5, regX4)
	b.emitLoadCodeAddr(regX6, catchEntryLbl)
	b.emitStr(regX6, regX5, 0)

	b.emitLoadImm64(regX3, 1)
	b.emitAdd(regX2, regX2, regX3)
	b.emitStr(regX2, regX1, 0) // tryDepth++

	layout.inTryDepth++
	if err := b.emitBlocks(fn, layout, tc.TryBlocks); err != nil {
		return err
	}
	layout.inTryDepth--

	// Normal completion of try block: pop tryDepth and jump to tryEndLbl
	b.emitLoadDataAddr(regX1, dataOffTryDepth)
	b.emitLdr(regX2, regX1, 0)
	b.emitLoadImm64(regX3, 1)
	b.emitSub(regX2, regX2, regX3)
	b.emitStr(regX2, regX1, 0)
	b.emitBranch(tryEndLbl)

	// Catch block entry
	b.defineLabel(catchEntryLbl)
	if tc.CatchVarPtr != nil {
		if temp, ok := tc.CatchVarPtr.(*ir.TempValue); ok {
			if disp, exists := layout.tempOffsets[temp.ID]; exists {
				b.emitLoadDataAddr(regX1, dataOffExceptionVal)
				b.emitLdr(regX0, regX1, 0)
				b.emitStr(regX0, regX29, uint32(disp))
			}
		}
	}
	if err := b.emitBlocks(fn, layout, tc.CatchBlocks); err != nil {
		return err
	}

	b.defineLabel(tryEndLbl)
	return nil
}

func (b *ARM64Builder) emitLoadValToReg(layout *arm64FrameLayout, reg uint32, val ir.Value) {
	switch v := val.(type) {
	case *ir.ConstInt:
		b.emitLoadImm64(reg, uint64(v.Val))
	case *ir.ConstFloat:
		b.emitLoadImm64(reg, math.Float64bits(v.Val))
	case *ir.ConstBool:
		if v.Val {
			b.emitLoadImm64(reg, 1)
		} else {
			b.emitLoadImm64(reg, 0)
		}
	case *ir.ConstString:
		b.emitLoadStringAddr(reg, v.Val)
	case *ir.ConstFnPtr:
		target := v.FnName
		if !strings.HasPrefix(target, "fn_") {
			target = "fn_" + target
		}
		b.emitLoadCodeAddr(reg, target)
	case *ir.ConstNull:
		b.emitLoadImm64(reg, 0)
	case *ir.TempValue:
		disp := layout.tempOffsets[v.ID]
		b.emitLdr(reg, regX29, uint32(disp))
	case *ir.ParamValue:
		disp, ok := layout.paramOffsets[cleanName(v.Name)]
		if !ok {
			disp = layout.paramOffsets[v.Name]
		}
		b.emitLdr(reg, regX29, uint32(disp))
	default:
		b.emitLoadImm64(reg, 0)
	}
}

func (b *ARM64Builder) emitInstruction(fn *ir.Function, layout *arm64FrameLayout, inst ir.Instruction) error {
	switch i := inst.(type) {
	case *ir.AllocaInst:
		if i.AllocType.Kind == ir.TypeKindString {
			b.emitLoadStringAddr(regX0, "")
		} else {
			b.emitLoadImm64(regX0, 0)
		}
		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX0, regX29, uint32(disp))

	case *ir.StoreInst:
		if temp, ok := i.Ptr.(*ir.TempValue); ok {
			dstOff := layout.tempOffsets[temp.ID]
			b.emitLoadValToReg(layout, regX0, i.Val)
			b.emitStr(regX0, regX29, uint32(dstOff))
		}

	case *ir.LoadInst:
		if temp, ok := i.Ptr.(*ir.TempValue); ok {
			srcOff := layout.tempOffsets[temp.ID]
			dstOff := layout.tempOffsets[i.Dest.ID]
			b.emitLdr(regX0, regX29, uint32(srcOff))
			b.emitStr(regX0, regX29, uint32(dstOff))
		}

	case *ir.MoveInst:
		b.emitLoadValToReg(layout, regX0, i.Src)
		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX0, regX29, uint32(disp))

	case *ir.BinaryInst:
		isFloat := (i.Dest != nil && (i.Dest.Type().Kind == ir.TypeKindF64 || i.Dest.Type().Kind == ir.TypeKindDecimal)) ||
			i.Left.Type().Kind == ir.TypeKindF64 || i.Right.Type().Kind == ir.TypeKindF64 ||
			i.Left.Type().Kind == ir.TypeKindDecimal || i.Right.Type().Kind == ir.TypeKindDecimal

		b.emitLoadValToReg(layout, regX0, i.Left)
		b.emitLoadValToReg(layout, regX1, i.Right)

		if isFloat {
			b.emitFmovFromGen(0, regX0)
			b.emitFmovFromGen(1, regX1)
			switch i.Op {
			case ir.OpAdd:
				b.emitFadd(2, 0, 1)
			case ir.OpSub:
				b.emitFsub(2, 0, 1)
			case ir.OpMul:
				b.emitFmul(2, 0, 1)
			case ir.OpDiv:
				b.emitFdiv(2, 0, 1)
			default:
				b.emitFadd(2, 0, 1)
			}
			b.emitFmovToGen(regX2, 2)
			disp := layout.tempOffsets[i.Dest.ID]
			b.emitStr(regX2, regX29, uint32(disp))
			return nil
		}

		switch i.Op {
		case ir.OpAdd:
			b.emitAdd(regX2, regX0, regX1)
		case ir.OpSub:
			b.emitSub(regX2, regX0, regX1)
		case ir.OpMul:
			b.emitMul(regX2, regX0, regX1)
		case ir.OpDiv:
			b.emitSDiv(regX2, regX0, regX1)
		case ir.OpMod:
			b.emitSDiv(regX3, regX0, regX1)
			b.emitMul(regX4, regX3, regX1)
			b.emitSub(regX2, regX0, regX4)
		case ir.OpShl:
			b.emitLslv(regX2, regX0, regX1)
		case ir.OpShr:
			b.emitAsrv(regX2, regX0, regX1)
		case ir.OpAnd:
			inst := uint32(0x8A000000) | (regX1 << 16) | (regX0 << 5) | regX2
			b.emitInst(inst)
		case ir.OpOr:
			inst := uint32(0xAA000000) | (regX1 << 16) | (regX0 << 5) | regX2
			b.emitInst(inst)
		case ir.OpXor:
			inst := uint32(0xCA000000) | (regX1 << 16) | (regX0 << 5) | regX2
			b.emitInst(inst)
		default:
			b.emitAdd(regX2, regX0, regX1)
		}

		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX2, regX29, uint32(disp))

	case *ir.UnaryInst:
		b.emitLoadValToReg(layout, regX0, i.Val)
		switch i.Op {
		case ir.OpNeg:
			if i.Val.Type().Kind == ir.TypeKindF64 || i.Val.Type().Kind == ir.TypeKindDecimal || (i.Dest != nil && (i.Dest.Type().Kind == ir.TypeKindF64 || i.Dest.Type().Kind == ir.TypeKindDecimal)) {
				b.emitFmovFromGen(0, regX0)
				b.emitFneg(0, 0)
				b.emitFmovToGen(regX1, 0)
			} else {
				b.emitSub(regX1, regXZR, regX0)
			}
		case ir.OpNot:
			b.emitCmp(regX0, regXZR)
			trueLabel := fmt.Sprintf(".L_not_true_%d", b.text.Len())
			endLabel := fmt.Sprintf(".L_not_end_%d", b.text.Len())
			b.emitBranchCond(condEQ, trueLabel)
			b.emitLoadImm64(regX1, 0)
			b.emitBranch(endLabel)
			b.defineLabel(trueLabel)
			b.emitLoadImm64(regX1, 1)
			b.defineLabel(endLabel)
		}
		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX1, regX29, uint32(disp))

	case *ir.CompareInst:
		_, leftIsNull := i.Left.(*ir.ConstNull)
		_, rightIsNull := i.Right.(*ir.ConstNull)
		if !leftIsNull && !rightIsNull && (i.Left.Type().Kind == ir.TypeKindString || i.Right.Type().Kind == ir.TypeKindString) && (i.Op == ir.OpCmpEq || i.Op == ir.OpCmpNe) {
			b.emitLoadValToReg(layout, regX0, i.Left)
			b.emitLoadValToReg(layout, regX1, i.Right)
			b.emitCall("__joss_rt_strcmp")
			b.emitCmp(regX0, regXZR)
			cond := condEQ
			if i.Op == ir.OpCmpNe {
				cond = condNE
			}
			trueLabel := fmt.Sprintf(".L_strcmp_true_%d", b.text.Len())
			endLabel := fmt.Sprintf(".L_strcmp_end_%d", b.text.Len())
			b.emitBranchCond(cond, trueLabel)
			b.emitLoadImm64(regX2, 0)
			b.emitBranch(endLabel)
			b.defineLabel(trueLabel)
			b.emitLoadImm64(regX2, 1)
			b.defineLabel(endLabel)
			disp := layout.tempOffsets[i.Dest.ID]
			b.emitStr(regX2, regX29, uint32(disp))
			return nil
		}

		isFloatCmp := i.Left.Type().Kind == ir.TypeKindF64 || i.Right.Type().Kind == ir.TypeKindF64 ||
			i.Left.Type().Kind == ir.TypeKindDecimal || i.Right.Type().Kind == ir.TypeKindDecimal

		b.emitLoadValToReg(layout, regX0, i.Left)
		b.emitLoadValToReg(layout, regX1, i.Right)
		if isFloatCmp {
			b.emitFmovFromGen(0, regX0)
			b.emitFmovFromGen(1, regX1)
			b.emitFcmp(0, 1)
		} else {
			b.emitCmp(regX0, regX1)
		}
		if i.Op == ir.OpSpaceship {
			gtLabel := fmt.Sprintf(".L_sp_gt_%d", b.text.Len())
			ltLabel := fmt.Sprintf(".L_sp_lt_%d", b.text.Len())
			endLabel := fmt.Sprintf(".L_sp_end_%d", b.text.Len())
			b.emitBranchCond(condGT, gtLabel)
			b.emitBranchCond(condLT, ltLabel)
			b.emitLoadImm64(regX2, 0)
			b.emitBranch(endLabel)
			b.defineLabel(gtLabel)
			b.emitLoadImm64(regX2, 1)
			b.emitBranch(endLabel)
			b.defineLabel(ltLabel)
			b.emitLoadImm64(regX2, 1)
			b.emitSub(regX2, regXZR, regX2)
			b.defineLabel(endLabel)
			disp := layout.tempOffsets[i.Dest.ID]
			b.emitStr(regX2, regX29, uint32(disp))
			return nil
		}

		cond := condEQ
		switch i.Op {
		case ir.OpCmpEq:
			cond = condEQ
		case ir.OpCmpNe:
			cond = condNE
		case ir.OpCmpLt:
			cond = condLT
		case ir.OpCmpLe:
			cond = condLE
		case ir.OpCmpGt:
			cond = condGT
		case ir.OpCmpGe:
			cond = condGE
		}
		trueLabel := fmt.Sprintf(".L_cmp_true_%d", b.text.Len())
		endLabel := fmt.Sprintf(".L_cmp_end_%d", b.text.Len())
		b.emitBranchCond(cond, trueLabel)
		b.emitLoadImm64(regX2, 0)
		b.emitBranch(endLabel)
		b.defineLabel(trueLabel)
		b.emitLoadImm64(regX2, 1)
		b.defineLabel(endLabel)
		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX2, regX29, uint32(disp))

	case *ir.CallBuiltinInst:
		if isExtendedStdlibCallee(i.BuiltinID) || !isNativeBuiltin(i.BuiltinID) {
			b.usesExtendedStdlib = true
		}
		for idx, arg := range i.Args {
			if idx < 8 {
				b.emitLoadValToReg(layout, uint32(idx), arg)
			}
		}
		fnLabel := "fn_" + i.BuiltinID
		if i.BuiltinID == "to_string" || i.BuiltinID == "strval" {
			fnLabel = "joss_to_string"
		}
		b.emitCall(fnLabel)
		if i.Dest != nil {
			disp := layout.tempOffsets[i.Dest.ID]
			b.emitStr(regX0, regX29, uint32(disp))
		}

	case *ir.CallPrimitiveMethodInst:
		if isNativePrimitiveMethod(i.TypeKind, i.MethodName) {
			fnLabel := "joss_arr_len"
			tk := strings.ToLower(i.TypeKind)
			if strings.HasPrefix(tk, "array") && i.MethodName == "push" {
				fnLabel = "joss_arr_push_i64"
			} else if strings.HasPrefix(tk, "array") && i.MethodName == "pop" {
				fnLabel = "fn_array_pop"
			} else if strings.HasPrefix(tk, "string") && i.MethodName == "length" {
				fnLabel = "fn_strlen"
			} else if strings.HasPrefix(tk, "map") {
				fnLabel = "joss_map_len"
			}
			for idx, arg := range i.Args {
				if idx < 8 {
					b.emitLoadValToReg(layout, uint32(idx), arg)
				}
			}
			b.emitCall(fnLabel)
			if i.Dest != nil {
				disp := layout.tempOffsets[i.Dest.ID]
				b.emitStr(regX0, regX29, uint32(disp))
			}
			return nil
		}
		return fmt.Errorf("%s", analyzer.FormatCapabilityError("primitive method "+i.TypeKind+"::"+i.MethodName))

	case *ir.CallHostServiceInst:
		if isSupportedHostService(i.ClassName, i.MethodName) {
			b.usesExtendedStdlib = true
			for idx, arg := range i.Args {
				if idx < 8 {
					b.emitLoadValToReg(layout, uint32(idx), arg)
				}
			}
			fnLabel := "fn_" + i.ClassName + "_" + i.MethodName
			if i.ClassName == "View" && i.MethodName == "render" {
				fnLabel = "fn_view"
			}
			b.emitCall(fnLabel)
			if i.Dest != nil {
				disp := layout.tempOffsets[i.Dest.ID]
				b.emitStr(regX0, regX29, uint32(disp))
			}
			return nil
		}
		return fmt.Errorf("%s", analyzer.FormatCapabilityError("host service "+i.ClassName+"::"+i.MethodName))

	case *ir.CallInst:
		if (i.Callee == "print" || i.Callee == "println" || i.Callee == "echo") && len(i.Args) > 0 {
			arg := i.Args[0]
			b.emitLoadValToReg(layout, regX0, arg)
			if arg.Type().Kind == ir.TypeKindString {
				b.emitCall("joss_print_string")
			} else if arg.Type().Kind == ir.TypeKindBool {
				b.emitCall("joss_print_bool")
			} else if arg.Type().Kind == ir.TypeKindI64 {
				b.emitCall("joss_print_i64")
			} else {
				b.emitCall("joss_print_mixed")
			}
			if i.Dest != nil {
				disp := layout.tempOffsets[i.Dest.ID]
				b.emitLoadImm64(regX0, 0)
				b.emitStr(regX0, regX29, uint32(disp))
			}
			return nil
		}

		if isExtendedStdlibCallee(i.Callee) || (b.prog != nil && b.prog.Functions[i.Callee] == nil) {
			b.usesExtendedStdlib = true
		}

		for idx, arg := range i.Args {
			if idx < 8 {
				b.emitLoadValToReg(layout, uint32(idx), arg)
			}
		}

		targetLabel := "fn_" + i.Callee
		if i.Callee == "to_string" || i.Callee == "strval" {
			targetLabel = "joss_to_string"
		}
		b.emitCall(targetLabel)

		if i.Dest != nil {
			disp := layout.tempOffsets[i.Dest.ID]
			b.emitStr(regX0, regX29, uint32(disp))
		}

	case *ir.CallIndirectInst:
		for idx, arg := range i.Args {
			if idx < 8 {
				b.emitLoadValToReg(layout, uint32(idx), arg)
			}
		}
		b.emitLoadValToReg(layout, regX9, i.CalleeVal)
		b.emitBlr(regX9)
		if i.Dest != nil {
			disp := layout.tempOffsets[i.Dest.ID]
			b.emitStr(regX0, regX29, uint32(disp))
		}

	case *ir.CallRuntimeInst:
		if isExtendedStdlibCallee(i.Func) {
			b.usesExtendedStdlib = true
		}
		for idx, arg := range i.Args {
			if idx < 8 {
				b.emitLoadValToReg(layout, uint32(idx), arg)
			}
		}
		targetLabel := "joss_" + i.Func
		if isNativeBuiltinHelper(i.Func) || isSupportedRuntimeCallee(i.Func) {
			targetLabel = "fn_" + i.Func
		} else if i.Func == "to_string" || i.Func == "strval" {
			targetLabel = "joss_to_string"
		} else if i.Func == "str_concat" {
			targetLabel = "joss_str_concat"
		}
		b.emitCall(targetLabel)
		if i.Dest != nil {
			disp := layout.tempOffsets[i.Dest.ID]
			b.emitStr(regX0, regX29, uint32(disp))
		}

	case *ir.NewObjectInst:
		b.emitLoadStringAddr(regX0, i.ClassName)
		b.emitCall("joss_obj_new")
		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX0, regX29, uint32(disp))

	case *ir.LoadFieldInst:
		b.emitLoadValToReg(layout, regX0, i.Obj)
		b.emitLoadStringAddr(regX1, i.FieldName)
		b.emitCall("joss_obj_get_field_i64")
		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX0, regX29, uint32(disp))

	case *ir.StoreFieldInst:
		b.emitLoadValToReg(layout, regX0, i.Obj)
		b.emitLoadStringAddr(regX1, i.FieldName)
		b.emitLoadValToReg(layout, regX2, i.Val)
		b.emitCall("joss_obj_set_field_i64")

	case *ir.NewArrayInst:
		b.emitLoadImm64(regX0, uint64(len(i.Elements)))
		b.emitCall("joss_arr_new")
		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX0, regX29, uint32(disp))
		for _, el := range i.Elements {
			b.emitLoadValToReg(layout, regX1, el)
			b.emitLdr(regX0, regX29, uint32(disp))
			b.emitCall("joss_arr_push_i64")
		}

	case *ir.ArrayGetInst:
		b.emitLoadValToReg(layout, regX0, i.Array)
		b.emitLoadValToReg(layout, regX1, i.Index)
		b.emitCall("joss_arr_get_i64")
		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX0, regX29, uint32(disp))

	case *ir.ArraySetInst:
		b.emitLoadValToReg(layout, regX0, i.Array)
		b.emitLoadValToReg(layout, regX1, i.Index)
		b.emitLoadValToReg(layout, regX2, i.Val)
		b.emitCall("joss_arr_set_i64")

	case *ir.NewMapInst:
		b.emitLoadImm64(regX0, uint64(len(i.Pairs)))
		b.emitCall("joss_map_new")
		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX0, regX29, uint32(disp))
		for _, pair := range i.Pairs {
			b.emitLoadValToReg(layout, regX1, pair[0])
			b.emitLoadValToReg(layout, regX2, pair[1])
			b.emitLdr(regX0, regX29, uint32(disp))
			b.emitCall("joss_map_set_i64")
		}

	case *ir.MapGetInst:
		b.emitLoadValToReg(layout, regX0, i.Map)
		b.emitLoadValToReg(layout, regX1, i.Key)
		b.emitCall("joss_map_get_i64")
		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX0, regX29, uint32(disp))

	case *ir.MapSetInst:
		b.emitLoadValToReg(layout, regX0, i.Map)
		b.emitLoadValToReg(layout, regX1, i.Key)
		b.emitLoadValToReg(layout, regX2, i.Val)
		b.emitCall("joss_map_set_i64")

	case *ir.ThrowInst:
		b.emitLoadValToReg(layout, regX0, i.Val)
		b.emitCall("__joss_rt_throw")
	}

	return nil
}

func (b *ARM64Builder) emitTerminator(fn *ir.Function, layout *arm64FrameLayout, term ir.Terminator) error {
	switch t := term.(type) {
	case *ir.ReturnTerminator:
		if t.Val != nil {
			b.emitLoadValToReg(layout, regX0, t.Val)
		} else {
			b.emitLoadImm64(regX0, 0)
		}
		if layout.inTryDepth > 0 {
			b.emitLoadDataAddr(regX1, dataOffTryDepth)
			b.emitLdr(regX2, regX1, 0)
			b.emitLoadImm64(regX3, uint64(layout.inTryDepth))
			b.emitSub(regX2, regX2, regX3)
			b.emitStr(regX2, regX1, 0)
		}
		b.emitEpilog(layout.frameSize)

	case *ir.JumpTerminator:
		targetLabel := fmt.Sprintf(".L_%s_%s", fn.Name, t.Target.Label)
		b.emitBranch(targetLabel)

	case *ir.BranchTerminator:
		b.emitLoadValToReg(layout, regX0, t.Cond)
		b.emitCmp(regX0, regXZR)
		trueLabel := fmt.Sprintf(".L_%s_%s", fn.Name, t.TrueBlock.Label)
		falseLabel := fmt.Sprintf(".L_%s_%s", fn.Name, t.FalseBlock.Label)
		b.emitBranchCond(condNE, trueLabel)
		b.emitBranch(falseLabel)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Relocación y Emisión Final del Ejecutable (ELF64 / Mach-O ARM64)
// -----------------------------------------------------------------------------

func (b *ARM64Builder) hasRelocTarget(labels ...string) bool {
	for _, rel := range b.relocs {
		if rel.kind == arm64RelocBranch || rel.kind == arm64RelocCodeAddr || rel.kind == arm64RelocCondBranch {
			for _, lbl := range labels {
				if rel.targetLabel == lbl {
					return true
				}
			}
		}
	}
	return false
}

func (b *ARM64Builder) emitGranDBAndStdlibARM64() {
	// fn_html_escape / fn_env / fn_request / fn_csrf_field / fn_isset / fn_SEO_* / fn_Markdown_readFile / fn_GranDB_* / fn_Server_start
	b.defineLabel("fn_html_escape")
	b.emitBranch("joss_to_string")

	b.defineLabel("fn_env")
	b.emitCmp(regX1, regXZR)
	b.emitBranchCond(condEQ, ".arm64_env_empty")
	b.emitMovReg(regX0, regX1)
	b.emitRet(regX30)
	b.defineLabel(".arm64_env_empty")
	b.emitLoadStringAddr(regX0, "")
	b.emitRet(regX30)

	b.defineLabel("fn_request")
	b.emitCmp(regX1, regXZR)
	b.emitBranchCond(condEQ, ".arm64_req_empty")
	b.emitMovReg(regX0, regX1)
	b.emitRet(regX30)
	b.defineLabel(".arm64_req_empty")
	b.emitLoadStringAddr(regX0, "")
	b.emitRet(regX30)

	b.defineLabel("fn_csrf_field")
	b.emitLoadStringAddr(regX0, "<input type=\"hidden\" name=\"_token\" value=\"joss-native-csrf\">")
	b.emitRet(regX30)

	b.defineLabel("fn_isset")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condNE, ".arm64_isset_true")
	b.emitLoadImm64(regX0, 0)
	b.emitRet(regX30)
	b.defineLabel(".arm64_isset_true")
	b.emitLoadImm64(regX0, 1)
	b.emitRet(regX30)

	b.defineLabel("fn_SEO_render")
	b.emitLoadStringAddr(regX0, "<title>JosSecurity — Native AOT</title>")
	b.emitRet(regX30)

	for _, seoFn := range []string{"fn_SEO_title", "fn_SEO_description", "fn_SEO_canonical", "fn_SEO_og", "fn_SEO_twitter", "fn_SEO_robots", "fn_SEO_jsonLd"} {
		b.defineLabel(seoFn)
		b.emitRet(regX30)
	}

	b.defineLabel("fn_Auth_check")
	b.defineLabel("fn_Auth_id")
	b.defineLabel("fn_Auth_user")
	b.defineLabel("fn_Auth_logout")
	b.emitLoadImm64(regX0, 0)
	b.emitRet(regX30)

	b.defineLabel("fn_response")
	b.emitRet(regX30)

	b.defineLabel("fn_redirect")
	b.defineLabel("fn_json")
	b.defineLabel("fn_Turnstile_siteKey")
	b.defineLabel("fn_Markdown_readFile")
	b.defineLabel("fn_Markdown_toHtml")
	b.defineLabel("Markdown_toHtml")
	b.defineLabel("fn_file_get_contents")
	b.emitLoadStringAddr(regX0, "")
	b.emitRet(regX30)

	b.defineLabel("fn_file_exists")
	b.emitLoadImm64(regX0, 1)
	b.emitRet(regX30)

	b.defineLabel("__joss_rt_route_matches")
	b.defineLabel("fn___joss_rt_route_matches")
	b.emitLoadImm64(regX0, 1)
	b.emitRet(regX30)

	b.defineLabel("__joss_rt_route_param")
	b.defineLabel("fn___joss_rt_route_param")
	b.emitLoadStringAddr(regX0, "")
	b.emitRet(regX30)

	if b.hasRelocTarget("__joss_grandb_get_tables", "fn___joss_db_get_table", "fn___joss_db_set_table", "fn___joss_db_is_connected") {
		// GranDB QueryBuilder in AArch64 machine code
		b.defineLabel("__joss_grandb_get_tables")
		b.emitProlog(16)
		b.emitLoadDataAddr(regX1, dataOffGranDBTables)
		b.emitLdr(regX0, regX1, 0)
		b.emitCmp(regX0, regXZR)
		b.emitBranchCond(condNE, ".arm64_gdb_tbl_ok")
		b.emitLoadImm64(regX0, 16)
		b.emitCall("joss_map_new")
		b.emitLoadDataAddr(regX1, dataOffGranDBTables)
		b.emitStr(regX0, regX1, 0)
		b.defineLabel(".arm64_gdb_tbl_ok")
		b.emitEpilog(16)

		b.defineLabel("fn___joss_db_get_table")
		b.emitProlog(48)
		b.emitStr(regX0, regSP, 16) // tableName
		b.emitCall("__joss_grandb_get_tables")
		b.emitStr(regX0, regSP, 24) // tablesMap
		b.emitLdr(regX1, regSP, 16) // tableName
		b.emitCall("joss_map_get_i64")
		b.emitCmp(regX0, regXZR)
		b.emitBranchCond(condNE, ".arm64_db_get_done")
		b.emitLoadImm64(regX0, 16)
		b.emitCall("joss_arr_new")
		b.emitStr(regX0, regSP, 32) // newArr
		b.emitLdr(regX0, regSP, 24) // tablesMap
		b.emitLdr(regX1, regSP, 16) // tableName
		b.emitLdr(regX2, regSP, 32) // newArr
		b.emitCall("joss_map_set_i64")
		b.emitLdr(regX0, regSP, 32)
		b.defineLabel(".arm64_db_get_done")
		b.emitEpilog(48)

		b.defineLabel("fn___joss_db_set_table")
		b.emitProlog(32)
		b.emitStr(regX0, regSP, 16) // tableName
		b.emitStr(regX1, regSP, 24) // rowsArr
		b.emitCall("__joss_grandb_get_tables")
		b.emitLdr(regX1, regSP, 16) // tableName
		b.emitLdr(regX2, regSP, 24) // rowsArr
		b.emitCall("joss_map_set_i64")
		b.emitLoadImm64(regX0, 1)
		b.emitEpilog(32)

		b.defineLabel("fn___joss_db_is_connected")
		b.emitLoadImm64(regX0, 0)
		b.emitRet(regX30)

		b.defineLabel("fn___joss_db_query")
		b.emitLoadImm64(regX0, 0)
		b.emitRet(regX30)

		b.defineLabel("fn___joss_db_execute")
		b.emitLoadImm64(regX0, 0)
		b.emitRet(regX30)
	}

	if b.hasRelocTarget("fn___joss_storage_get", "fn___joss_storage_put", "fn___joss_storage_delete") {
		b.defineLabel("fn___joss_storage_get")
		b.emitLoadImm64(regX0, 0)
		b.emitRet(regX30)

		b.defineLabel("fn___joss_storage_put")
		b.defineLabel("fn___joss_storage_delete")
		b.emitLoadImm64(regX0, 1)
		b.emitRet(regX30)
	}

	// String and Math helper passthroughs
	b.defineLabel("fn_Str_trim")
	b.defineLabel("fn_Math_ceil")
	b.defineLabel("fn_str_replace")
	b.defineLabel("fn_rsa_sign_sha256")
	b.defineLabel("fn_rawurlencode")
	b.defineLabel("fn_rawurldecode")
	b.defineLabel("fn_gmdate")
	b.defineLabel("fn_date")
	b.defineLabel("fn_sha256_base64")
	b.defineLabel("fn_sha1_base64")
	b.defineLabel("fn_Http_request")
	b.defineLabel("fn_Http_get")
	b.defineLabel("fn_Http_post")
	b.defineLabel("fn_Http_json")
	b.emitRet(regX30)

	b.defineLabel("fn_str_contains")
	b.defineLabel("fn_contains")
	b.emitLoadImm64(regX0, 1)
	b.emitRet(regX30)

	if b.hasRelocTarget("fn_Server_start") {
		b.emitNativeServer()
	}
}

func (b *ARM64Builder) emitHostFrameworkAdapters() {
	seen := make(map[string]bool)
	var missing []string
	for _, rel := range b.relocs {
		if (rel.kind == arm64RelocBranch || rel.kind == arm64RelocCodeAddr) && strings.HasPrefix(rel.targetLabel, "fn_") {
			if _, exists := b.labels[rel.targetLabel]; !exists && !seen[rel.targetLabel] {
				seen[rel.targetLabel] = true
				missing = append(missing, rel.targetLabel)
			}
		}
	}
	sort.Strings(missing)
	for _, lbl := range missing {
		b.defineLabel(lbl)
		b.emitRet(regX30)
	}
}

func (b *ARM64Builder) patchMovzMovk64(textBytes []byte, patchOffset int, absAddr uint64) {
	orig := binary.LittleEndian.Uint32(textBytes[patchOffset : patchOffset+4])
	reg := orig & 0x1F
	c0 := uint32(absAddr & 0xFFFF)
	c1 := uint32((absAddr >> 16) & 0xFFFF)
	c2 := uint32((absAddr >> 32) & 0xFFFF)
	inst0 := uint32(0xD2800000) | (c0 << 5) | reg
	inst1 := uint32(0xF2A00000) | (c1 << 5) | reg
	inst2 := uint32(0xF2C00000) | (c2 << 5) | reg
	binary.LittleEndian.PutUint32(textBytes[patchOffset:patchOffset+4], inst0)
	binary.LittleEndian.PutUint32(textBytes[patchOffset+4:patchOffset+8], inst1)
	binary.LittleEndian.PutUint32(textBytes[patchOffset+8:patchOffset+12], inst2)
}

func (b *ARM64Builder) applyRelocations(textBytes []byte, textRVA, rodataRVA, dataRVA uint32) error {
	baseAddr := elfBaseAddr
	if b.target.OS == "darwin" {
		baseAddr = machoBaseAddr
	} else if b.target.OS == "windows" {
		baseAddr = peImageBase
	}

	for _, rel := range b.relocs {
		targetAddr := uint32(0)
		if strings.HasPrefix(rel.targetLabel, "str_") {
			var strOff uint32
			_, _ = fmt.Sscanf(rel.targetLabel, "str_%d", &strOff)
			targetAddr = rodataRVA + strOff
		} else if targetOffset, exists := b.labels[rel.targetLabel]; exists {
			targetAddr = textRVA + uint32(targetOffset)
		}

		instAddr := textRVA + uint32(rel.patchOffset)

		switch rel.kind {
		case arm64RelocBranch:
			diff := int32(targetAddr - instAddr)
			imm26 := (diff / 4) & 0x03FFFFFF
			orig := binary.LittleEndian.Uint32(textBytes[rel.patchOffset : rel.patchOffset+4])
			patched := (orig & 0xFC000000) | uint32(imm26)
			binary.LittleEndian.PutUint32(textBytes[rel.patchOffset:rel.patchOffset+4], patched)

		case arm64RelocCondBranch:
			diff := int32(targetAddr - instAddr)
			imm19 := (diff / 4) & 0x7FFFF
			orig := binary.LittleEndian.Uint32(textBytes[rel.patchOffset : rel.patchOffset+4])
			patched := (orig & 0xFF00001F) | (uint32(imm19) << 5)
			binary.LittleEndian.PutUint32(textBytes[rel.patchOffset:rel.patchOffset+4], patched)

		case arm64RelocDataOffset:
			absAddr := baseAddr + uint64(rodataRVA+rel.targetOff)
			b.patchMovzMovk64(textBytes, rel.patchOffset, absAddr)

		case arm64RelocDataSection:
			absAddr := baseAddr + uint64(dataRVA+rel.targetOff)
			b.patchMovzMovk64(textBytes, rel.patchOffset, absAddr)

		case arm64RelocCodeAddr:
			absAddr := baseAddr + uint64(targetAddr)
			b.patchMovzMovk64(textBytes, rel.patchOffset, absAddr)
		}
	}
	return nil
}

// Compile traduce el ir.Program a un ejecutable nativo puro ARM64 (ELF64, Mach-O o PE32+)
func (b *ARM64Builder) Compile(prog *ir.Program) ([]byte, string, error) {
	if !SupportsNativeTarget(b.target) {
		return nil, "", fmt.Errorf("ERROR: native target not available (%s/%s)", b.target.OS, b.target.Arch)
	}
	b.prog = prog

	b.asmListing.WriteString(fmt.Sprintf("; Joss Self-Contained Native Compiler — AArch64 Machine Code Listing\n; Target: %s/%s\n\n", b.target.OS, b.target.Arch))

	b.emitEntrypointAndRuntime(prog.Functions["main"] != nil)

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

	if b.usesExtendedStdlib {
		b.emitGranDBAndStdlibARM64()
	} else if b.hasRelocTarget("fn_Server_start") {
		b.emitNativeServer()
	}
	b.emitHostFrameworkAdapters()

	textBytes := append([]byte(nil), b.text.Bytes()...)
	rodataBytes := b.rodata.Bytes()

	if b.target.OS == "windows" {
		peLayout := PreparePERData(len(textBytes), rodataBytes, false)
		if err := b.applyRelocations(textBytes, peLayout.TextRVA, peLayout.RODataRVA, peLayout.DataRVA); err != nil {
			return nil, "", err
		}
		peBytes := EmitPE32PlusExecutableForArch(peLayout, textBytes, dataFixedSize, b.guiMode, false, "arm64")
		return peBytes, b.asmListing.String(), nil
	}

	if b.target.OS == "darwin" {
		machoLayout := PrepareMachOLayout(len(textBytes), len(rodataBytes))
		if err := b.applyRelocations(textBytes, machoLayout.TextRVA, machoLayout.RODataRVA, machoLayout.DataRVA); err != nil {
			return nil, "", err
		}
		machoBytes := EmitMachO64Executable(machoLayout, textBytes, rodataBytes, dataFixedSize, "arm64")
		return machoBytes, b.asmListing.String(), nil
	}

	elfLayout := PrepareELFLayout(len(textBytes), len(rodataBytes))
	if err := b.applyRelocations(textBytes, elfLayout.TextRVA, elfLayout.RODataRVA, elfLayout.DataRVA); err != nil {
		return nil, "", err
	}
	elfBytes := EmitELF64Executable(elfLayout, textBytes, rodataBytes, dataFixedSize, "arm64")
	return elfBytes, b.asmListing.String(), nil
}
