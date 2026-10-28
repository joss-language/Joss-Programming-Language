package native

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

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
	arm64RelocBranch     arm64RelocKind = iota // B o BL (imm26)
	arm64RelocCondBranch                       // B.cond (imm19)
	arm64RelocDataOffset                       // Puntero absoluto o relativo a ROData
)

type arm64Reloc struct {
	kind        arm64RelocKind
	patchOffset int
	targetLabel string
}

// ARM64Builder produce código máquina nativo AArch64 (ARMv8-A) directamente desde Joss Native IR.
type ARM64Builder struct {
	target          Target
	guiMode         bool
	prog            *ir.Program
	text            bytes.Buffer
	rodata          bytes.Buffer
	labels          map[string]int
	relocs          []arm64Reloc
	stringPool      map[string]uint32
	asmListing      strings.Builder
	needsHostBridge bool
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

// -----------------------------------------------------------------------------
// Emisión de Instrucciones AArch64 (Little-Endian de 32 bits)
// -----------------------------------------------------------------------------

func (b *ARM64Builder) emitInst(inst uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], inst)
	b.text.Write(buf[:])
}

func (b *ARM64Builder) defineLabel(name string) {
	b.labels[name] = b.text.Len()
	b.asmListing.WriteString(fmt.Sprintf("%s: ; [offset 0x%04x]\n", name, b.text.Len()))
}

// RET: ret {xn} (por defecto X30)
func (b *ARM64Builder) emitRet(rn uint32) {
	b.emitInst(0xD65F0000 | (rn << 5))
	if rn == regX30 {
		b.asmListing.WriteString("    ret\n")
	} else {
		b.asmListing.WriteString(fmt.Sprintf("    ret x%d\n", rn))
	}
}

// B label (unconditional branch, imm26)
func (b *ARM64Builder) emitBranch(label string) {
	b.relocs = append(b.relocs, arm64Reloc{
		kind:        arm64RelocBranch,
		patchOffset: b.text.Len(),
		targetLabel: label,
	})
	b.emitInst(0x14000000)
	b.asmListing.WriteString(fmt.Sprintf("    b %s\n", label))
}

// BL label (branch with link, call, imm26)
func (b *ARM64Builder) emitCall(label string) {
	b.relocs = append(b.relocs, arm64Reloc{
		kind:        arm64RelocBranch,
		patchOffset: b.text.Len(),
		targetLabel: label,
	})
	b.emitInst(0x94000000)
	b.asmListing.WriteString(fmt.Sprintf("    bl %s\n", label))
}

// B.cond label (conditional branch, imm19)
func (b *ARM64Builder) emitBranchCond(cond uint32, label string) {
	b.relocs = append(b.relocs, arm64Reloc{
		kind:        arm64RelocCondBranch,
		patchOffset: b.text.Len(),
		targetLabel: label,
	})
	b.emitInst(0x54000000 | cond)
	condNames := []string{"eq", "ne", "cs", "cc", "mi", "pl", "vs", "vc", "hi", "ls", "ge", "lt", "gt", "le", "al"}
	cName := "cond"
	if int(cond) < len(condNames) {
		cName = condNames[cond]
	}
	b.asmListing.WriteString(fmt.Sprintf("    b.%s %s\n", cName, label))
}

// STP x29, x30, [sp, #-frameSize]!
func (b *ARM64Builder) emitProlog(frameSize int32) {
	if frameSize < 16 {
		frameSize = 16
	}
	frameSize = (frameSize + 15) & ^15
	imm7 := ((-frameSize) / 8) & 0x7F
	inst := uint32(0xA9800000) | uint32(0x00200000) | (uint32(imm7) << 15) | (regX30 << 10) | (regSP << 5) | regX29
	b.emitInst(inst)
	// mov x29, sp -> add x29, sp, #0
	b.emitInst(0x91000000 | (regSP << 5) | regX29)
	b.asmListing.WriteString(fmt.Sprintf("    stp x29, x30, [sp, #-%d]!\n    mov x29, sp\n", frameSize))
}

// LDP x29, x30, [sp], #frameSize; ret
func (b *ARM64Builder) emitEpilog(frameSize int32) {
	if frameSize < 16 {
		frameSize = 16
	}
	frameSize = (frameSize + 15) & ^15
	imm7 := (frameSize / 8) & 0x7F
	inst := uint32(0xA8800000) | uint32(0x00400000) | (uint32(imm7) << 15) | (regX30 << 10) | (regSP << 5) | regX29
	b.emitInst(inst)
	b.emitRet(regX30)
	b.asmListing.WriteString(fmt.Sprintf("    ldp x29, x30, [sp], #%d\n    ret\n", frameSize))
}

// MOV xd, xm (ORR xd, xzr, xm)
func (b *ARM64Builder) emitMovReg(xd, xm uint32) {
	if xd == xm {
		return
	}
	inst := uint32(0xAA0003E0) | (xm << 16) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    mov x%d, x%d\n", xd, xm))
}

// Cargar constante entera de 64 bits usando MOVZ / MOVK
func (b *ARM64Builder) emitLoadImm64(xd uint32, val uint64) {
	if val == 0 {
		b.emitInst(uint32(0xAA1F03E0) | xd)
		b.asmListing.WriteString(fmt.Sprintf("    mov x%d, #0\n", xd))
		return
	}

	chunks := [4]uint16{
		uint16(val & 0xFFFF),
		uint16((val >> 16) & 0xFFFF),
		uint16((val >> 32) & 0xFFFF),
		uint16((val >> 48) & 0xFFFF),
	}

	first := true
	for i, chunk := range chunks {
		if chunk != 0 || (first && val < 0x10000) {
			hw := uint32(i)
			if first {
				inst := uint32(0xD2800000) | (hw << 21) | (uint32(chunk) << 5) | xd
				b.emitInst(inst)
				b.asmListing.WriteString(fmt.Sprintf("    movz x%d, #0x%04x, lsl #%d\n", xd, chunk, i*16))
				first = false
			} else {
				inst := uint32(0xF2800000) | (hw << 21) | (uint32(chunk) << 5) | xd
				b.emitInst(inst)
				b.asmListing.WriteString(fmt.Sprintf("    movk x%d, #0x%04x, lsl #%d\n", xd, chunk, i*16))
			}
		}
	}
}

// ADD xd, xn, xm
func (b *ARM64Builder) emitAdd(xd, xn, xm uint32) {
	inst := uint32(0x8B000000) | (xm << 16) | (xn << 5) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    add x%d, x%d, x%d\n", xd, xn, xm))
}

// SUB xd, xn, xm
func (b *ARM64Builder) emitSub(xd, xn, xm uint32) {
	inst := uint32(0xCB000000) | (xm << 16) | (xn << 5) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    sub x%d, x%d, x%d\n", xd, xn, xm))
}

// MUL xd, xn, xm
func (b *ARM64Builder) emitMul(xd, xn, xm uint32) {
	inst := uint32(0x9B007C00) | (xm << 16) | (xn << 5) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    mul x%d, x%d, x%d\n", xd, xn, xm))
}

// SDIV xd, xn, xm
func (b *ARM64Builder) emitSDiv(xd, xn, xm uint32) {
	inst := uint32(0x9AC00C00) | (xm << 16) | (xn << 5) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    sdiv x%d, x%d, x%d\n", xd, xn, xm))
}

// CMP xn, xm
func (b *ARM64Builder) emitCmp(xn, xm uint32) {
	inst := uint32(0xEB00001F) | (xm << 16) | (xn << 5)
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    cmp x%d, x%d\n", xn, xm))
}

// LDR xd, [xn, #pimm]
func (b *ARM64Builder) emitLdr(xd, xn uint32, pimm uint32) {
	imm12 := (pimm / 8) & 0xFFF
	inst := uint32(0xF9400000) | (imm12 << 10) | (xn << 5) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    ldr x%d, [x%d, #%d]\n", xd, xn, pimm))
}

// STR xd, [xn, #pimm]
func (b *ARM64Builder) emitStr(xd, xn uint32, pimm uint32) {
	imm12 := (pimm / 8) & 0xFFF
	inst := uint32(0xF9000000) | (imm12 << 10) | (xn << 5) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    str x%d, [x%d, #%d]\n", xd, xn, pimm))
}

// LDRB wd, [xn, #pimm]
func (b *ARM64Builder) emitLdrb(wd, xn uint32, pimm uint32) {
	imm12 := pimm & 0xFFF
	inst := uint32(0x39400000) | (imm12 << 10) | (xn << 5) | wd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    ldrb w%d, [x%d, #%d]\n", wd, xn, pimm))
}

// STRB wd, [xn, #pimm]
func (b *ARM64Builder) emitStrb(wd, xn uint32, pimm uint32) {
	imm12 := pimm & 0xFFF
	inst := uint32(0x39000000) | (imm12 << 10) | (xn << 5) | wd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    strb w%d, [x%d, #%d]\n", wd, xn, pimm))
}

// Syscall en AArch64
func (b *ARM64Builder) emitSyscall() {
	if b.target.OS == "darwin" {
		b.emitInst(0xD4001001)
		b.asmListing.WriteString("    svc #0x80 ; Darwin syscall\n")
	} else {
		b.emitInst(0xD4000001)
		b.asmListing.WriteString("    svc #0 ; Linux AArch64 syscall\n")
	}
}

func (b *ARM64Builder) internString(s string) uint32 {
	if off, exists := b.stringPool[s]; exists {
		return off
	}
	off := uint32(b.rodata.Len())
	b.rodata.WriteString(s)
	b.rodata.WriteByte(0)
	b.stringPool[s] = off
	return off
}

func (b *ARM64Builder) emitLoadStringAddr(xd uint32, s string) {
	strOff := b.internString(s)
	strLabel := fmt.Sprintf("str_%d", strOff)
	b.relocs = append(b.relocs, arm64Reloc{
		kind:        arm64RelocDataOffset,
		patchOffset: b.text.Len(),
		targetLabel: strLabel,
	})
	b.emitInst(uint32(0xD2800000) | xd)
	b.asmListing.WriteString(fmt.Sprintf("    adr_rodata x%d, %s ; \"%s\"\n", xd, strLabel, strings.ReplaceAll(s, "\n", "\\n")))
}

// -----------------------------------------------------------------------------
// Emisión del Entrypoint y Runtime Builtins en AArch64
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
}

// -----------------------------------------------------------------------------
// Emisión de Funciones IR en AArch64
// -----------------------------------------------------------------------------

type arm64FrameLayout struct {
	frameSize    int32
	tempOffsets  map[int]int32
	paramOffsets map[string]int32
}

func (b *ARM64Builder) computeFrameLayout(fn *ir.Function) *arm64FrameLayout {
	layout := &arm64FrameLayout{
		tempOffsets:  make(map[int]int32),
		paramOffsets: make(map[string]int32),
	}

	offset := int32(16)
	for _, blk := range fn.Blocks {
		for _, inst := range blk.Instructions {
			if res := inst.Result(); res != nil {
				layout.tempOffsets[res.ID] = offset
				offset += 8
			}
		}
	}
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
			b.emitStr(uint32(i), regSP, uint32(disp))
			layout.paramOffsets[param.Name] = disp
		}
	}

	for _, blk := range fn.Blocks {
		b.defineLabel(fmt.Sprintf(".L_%s_%s", fn.Name, blk.Label))
		for _, inst := range blk.Instructions {
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

func (b *ARM64Builder) emitLoadValToReg(layout *arm64FrameLayout, reg uint32, val ir.Value) {
	switch v := val.(type) {
	case *ir.ConstInt:
		b.emitLoadImm64(reg, uint64(v.Val))
	case *ir.ConstBool:
		if v.Val {
			b.emitLoadImm64(reg, 1)
		} else {
			b.emitLoadImm64(reg, 0)
		}
	case *ir.ConstString:
		b.emitLoadStringAddr(reg, v.Val)
	case *ir.ConstNull:
		b.emitLoadImm64(reg, 0)
	case *ir.TempValue:
		disp := layout.tempOffsets[v.ID]
		b.emitLdr(reg, regSP, uint32(disp))
	case *ir.ParamValue:
		disp := layout.paramOffsets[v.Name]
		b.emitLdr(reg, regSP, uint32(disp))
	default:
		b.emitLoadImm64(reg, 0)
	}
}

func (b *ARM64Builder) emitInstruction(fn *ir.Function, layout *arm64FrameLayout, inst ir.Instruction) error {
	switch i := inst.(type) {
	case *ir.AllocaInst:
		b.emitLoadImm64(regX0, 0)
		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX0, regSP, uint32(disp))

	case *ir.StoreInst:
		if temp, ok := i.Ptr.(*ir.TempValue); ok {
			dstOff := layout.tempOffsets[temp.ID]
			b.emitLoadValToReg(layout, regX0, i.Val)
			b.emitStr(regX0, regSP, uint32(dstOff))
		}

	case *ir.LoadInst:
		if temp, ok := i.Ptr.(*ir.TempValue); ok {
			srcOff := layout.tempOffsets[temp.ID]
			dstOff := layout.tempOffsets[i.Dest.ID]
			b.emitLdr(regX0, regSP, uint32(srcOff))
			b.emitStr(regX0, regSP, uint32(dstOff))
		}

	case *ir.MoveInst:
		b.emitLoadValToReg(layout, regX0, i.Src)
		disp := layout.tempOffsets[i.Dest.ID]
		b.emitStr(regX0, regSP, uint32(disp))

	case *ir.BinaryInst:
		b.emitLoadValToReg(layout, regX0, i.Left)
		b.emitLoadValToReg(layout, regX1, i.Right)

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
		b.emitStr(regX2, regSP, uint32(disp))

	case *ir.UnaryInst:
		b.emitLoadValToReg(layout, regX0, i.Val)
		switch i.Op {
		case ir.OpNeg:
			b.emitSub(regX1, regXZR, regX0)
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
		b.emitStr(regX1, regSP, uint32(disp))

	case *ir.CompareInst:
		b.emitLoadValToReg(layout, regX0, i.Left)
		b.emitLoadValToReg(layout, regX1, i.Right)
		b.emitCmp(regX0, regX1)
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
		b.emitStr(regX2, regSP, uint32(disp))

	case *ir.CallInst:
		if i.Callee == "print" {
			if len(i.Args) > 0 {
				arg := i.Args[0]
				b.emitLoadValToReg(layout, regX0, arg)
				if arg.Type().Kind == ir.TypeKindString {
					b.emitCall("joss_print_string")
				} else if arg.Type().Kind == ir.TypeKindBool {
					b.emitCall("joss_print_bool")
				} else {
					b.emitCall("joss_print_i64")
				}
			}
			return nil
		}

		for idx, arg := range i.Args {
			if idx < 8 {
				b.emitLoadValToReg(layout, uint32(idx), arg)
			}
		}

		targetLabel := "fn_" + i.Callee
		b.emitCall(targetLabel)

		if i.Dest != nil {
			disp := layout.tempOffsets[i.Dest.ID]
			b.emitStr(regX0, regSP, uint32(disp))
		}

	case *ir.CallRuntimeInst:
		b.needsHostBridge = true
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

func (b *ARM64Builder) applyRelocations(textBytes []byte, textRVA, rodataRVA, dataRVA uint32) error {
	baseAddr := elfBaseAddr
	if b.target.OS == "darwin" {
		baseAddr = machoBaseAddr
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
			absAddr := baseAddr + uint64(targetAddr)
			chunk0 := uint32(absAddr & 0xFFFF)
			orig := binary.LittleEndian.Uint32(textBytes[rel.patchOffset : rel.patchOffset+4])
			reg := orig & 0x1F
			inst0 := uint32(0xD2800000) | (chunk0 << 5) | reg
			binary.LittleEndian.PutUint32(textBytes[rel.patchOffset:rel.patchOffset+4], inst0)
		}
	}
	return nil
}

// Compile traduce el ir.Program a un ejecutable nativo puro ARM64 (ELF64 o Mach-O)
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

	textBytes := append([]byte(nil), b.text.Bytes()...)
	rodataBytes := b.rodata.Bytes()

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
