package native

import (
	"encoding/binary"
	"fmt"
)

// Helper methods for emitting primitive x86_64 machine code bytes,
// RIP-relative branches, calls, and relocations.

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

func (b *MachineCodeBuilder) emitLeaRDIString(s string) {
	off := b.internString(s)
	b.asmListing.WriteString(fmt.Sprintf("    lea rdi, [rip + str_%d] ; %q\n", off, s))
	b.emitBytes(0x48, 0x8D, 0x3D)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocROData, patchOffset: patchOff, targetOff: off})
}

func (b *MachineCodeBuilder) emitLeaRSIString(s string) {
	off := b.internString(s)
	b.asmListing.WriteString(fmt.Sprintf("    lea rsi, [rip + str_%d] ; %q\n", off, s))
	b.emitBytes(0x48, 0x8D, 0x35)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocROData, patchOffset: patchOff, targetOff: off})
}
