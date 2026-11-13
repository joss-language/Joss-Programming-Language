package native

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Helper methods for emitting primitive AArch64 machine code instructions (32-bit little-endian),
// branches, memory addresses, and system calls.

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

// AND wd, wn, wm
func (b *ARM64Builder) emitAndW(wd, wn, wm uint32) {
	inst := uint32(0x0A000000) | (wm << 16) | (wn << 5) | wd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    and w%d, w%d, w%d\n", wd, wn, wm))
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

// UDIV xd, xn, xm
func (b *ARM64Builder) emitUDiv(xd, xn, xm uint32) {
	inst := uint32(0x9AC00800) | (xm << 16) | (xn << 5) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    udiv x%d, x%d, x%d\n", xd, xn, xm))
}

// CMP xn, xm
func (b *ARM64Builder) emitCmp(xn, xm uint32) {
	inst := uint32(0xEB00001F) | (xm << 16) | (xn << 5)
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    cmp x%d, x%d\n", xn, xm))
}

// ADD xd, xn, #imm12
func (b *ARM64Builder) emitAddImm(xd, xn uint32, imm uint32) {
	inst := uint32(0x91000000) | ((imm & 0xFFF) << 10) | (xn << 5) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    add x%d, x%d, #%d\n", xd, xn, imm))
}

// SUB xd, xn, #imm12
func (b *ARM64Builder) emitSubImm(xd, xn uint32, imm uint32) {
	inst := uint32(0xD1000000) | ((imm & 0xFFF) << 10) | (xn << 5) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    sub x%d, x%d, #%d\n", xd, xn, imm))
}

// CMP xn, #imm12 (SUBS xzr, xn, #imm12)
func (b *ARM64Builder) emitCmpImm(xn uint32, imm uint32) {
	inst := uint32(0xF1000000) | ((imm & 0xFFF) << 10) | (xn << 5) | regXZR
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    cmp x%d, #%d\n", xn, imm))
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

// LSLV xd, xn, xm (logical shift left variable)
func (b *ARM64Builder) emitLslv(xd, xn, xm uint32) {
	inst := uint32(0x9AC02000) | (xm << 16) | (xn << 5) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    lsl x%d, x%d, x%d\n", xd, xn, xm))
}

// ASRV xd, xn, xm (arithmetic shift right variable)
func (b *ARM64Builder) emitAsrv(xd, xn, xm uint32) {
	inst := uint32(0x9AC02800) | (xm << 16) | (xn << 5) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    asr x%d, x%d, x%d\n", xd, xn, xm))
}

// BLR xn (branch with link to register)
func (b *ARM64Builder) emitBlr(rn uint32) {
	inst := uint32(0xD63F0000) | (rn << 5)
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    blr x%d\n", rn))
}

// BR xn (unconditional branch to register)
func (b *ARM64Builder) emitBr(rn uint32) {
	inst := uint32(0xD61F0000) | (rn << 5)
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    br x%d\n", rn))
}

// FMOV dd, xn (general-purpose 64-bit register to FP double register)
func (b *ARM64Builder) emitFmovFromGen(dd, xn uint32) {
	inst := uint32(0x9E670000) | (xn << 5) | dd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    fmov d%d, x%d\n", dd, xn))
}

// FMOV xd, dn (FP double register to general-purpose 64-bit register)
func (b *ARM64Builder) emitFmovToGen(xd, dn uint32) {
	inst := uint32(0x9E660000) | (dn << 5) | xd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    fmov x%d, d%d\n", xd, dn))
}

// FADD dd, dn, dm
func (b *ARM64Builder) emitFadd(dd, dn, dm uint32) {
	inst := uint32(0x1E602800) | (dm << 16) | (dn << 5) | dd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    fadd d%d, d%d, d%d\n", dd, dn, dm))
}

// FSUB dd, dn, dm
func (b *ARM64Builder) emitFsub(dd, dn, dm uint32) {
	inst := uint32(0x1E603800) | (dm << 16) | (dn << 5) | dd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    fsub d%d, d%d, d%d\n", dd, dn, dm))
}

// FMUL dd, dn, dm
func (b *ARM64Builder) emitFmul(dd, dn, dm uint32) {
	inst := uint32(0x1E600800) | (dm << 16) | (dn << 5) | dd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    fmul d%d, d%d, d%d\n", dd, dn, dm))
}

// FDIV dd, dn, dm
func (b *ARM64Builder) emitFdiv(dd, dn, dm uint32) {
	inst := uint32(0x1E601800) | (dm << 16) | (dn << 5) | dd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    fdiv d%d, d%d, d%d\n", dd, dn, dm))
}

// FNEG dd, dn
func (b *ARM64Builder) emitFneg(dd, dn uint32) {
	inst := uint32(0x1E614000) | (dn << 5) | dd
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    fneg d%d, d%d\n", dd, dn))
}

// FCMP dn, dm
func (b *ARM64Builder) emitFcmp(dn, dm uint32) {
	inst := uint32(0x1E602000) | (dm << 16) | (dn << 5)
	b.emitInst(inst)
	b.asmListing.WriteString(fmt.Sprintf("    fcmp d%d, d%d\n", dn, dm))
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
		targetOff:   strOff,
	})
	b.emitInst(uint32(0xD2800000) | xd) // movz xd, #0
	b.emitInst(uint32(0xF2A00000) | xd) // movk xd, #0, lsl #16
	b.emitInst(uint32(0xF2C00000) | xd) // movk xd, #0, lsl #32
	b.asmListing.WriteString(fmt.Sprintf("    adr_rodata x%d, %s ; \"%s\"\n", xd, strLabel, strings.ReplaceAll(s, "\n", "\\n")))
}

func (b *ARM64Builder) emitLoadDataAddr(xd uint32, dataOff uint32) {
	b.relocs = append(b.relocs, arm64Reloc{
		kind:        arm64RelocDataSection,
		patchOffset: b.text.Len(),
		targetOff:   dataOff,
	})
	b.emitInst(uint32(0xD2800000) | xd) // movz xd, #0
	b.emitInst(uint32(0xF2A00000) | xd) // movk xd, #0, lsl #16
	b.emitInst(uint32(0xF2C00000) | xd) // movk xd, #0, lsl #32
	b.asmListing.WriteString(fmt.Sprintf("    adr_data x%d, data_0x%x\n", xd, dataOff))
}

func (b *ARM64Builder) emitLoadCodeAddr(xd uint32, label string) {
	b.relocs = append(b.relocs, arm64Reloc{
		kind:        arm64RelocCodeAddr,
		patchOffset: b.text.Len(),
		targetLabel: label,
	})
	b.emitInst(uint32(0xD2800000) | xd) // movz xd, #0
	b.emitInst(uint32(0xF2A00000) | xd) // movk xd, #0, lsl #16
	b.emitInst(uint32(0xF2C00000) | xd) // movk xd, #0, lsl #32
	b.asmListing.WriteString(fmt.Sprintf("    adr_code x%d, %s\n", xd, label))
}
