package native

import (
	"bytes"
	"encoding/binary"
)

const (
	elfBaseAddr  uint64 = 0x00400000
	elfPageAlign uint32 = 0x1000
	elfTextRVA   uint32 = 0x1000
)

// ELFImageLayout holds the computed RVAs for a Linux x86_64 static ELF64 executable.
type ELFImageLayout struct {
	TextRVA   uint32
	RODataRVA uint32
	DataRVA   uint32
}

// PrepareELFLayout computes the section virtual offsets for a Linux x86_64 ELF64 binary.
func PrepareELFLayout(textLen int, rodataLen int) *ELFImageLayout {
	rodataRVA := alignUpU32(elfTextRVA+uint32(textLen), 16)
	dataRVA := alignUpU32(rodataRVA+uint32(rodataLen)+16, elfPageAlign)
	return &ELFImageLayout{
		TextRVA:   elfTextRVA,
		RODataRVA: rodataRVA,
		DataRVA:   dataRVA,
	}
}

// EmitELF64Executable serializes a Linux x86_64 ELF64 executable.
func EmitELF64Executable(layout *ELFImageLayout, textBytes []byte, rodataBytes []byte, dataSize uint32) []byte {
	var out bytes.Buffer

	// ELF64 Header (64 bytes) + 2 Program Headers (2 * 56 = 112 bytes) = 176 bytes, padded to 0x1000
	codeSegmentEnd := layout.RODataRVA + uint32(len(rodataBytes))
	if len(rodataBytes) == 0 {
		codeSegmentEnd = layout.RODataRVA + 1
	}
	codeSegmentFileSize := codeSegmentEnd

	dataFileOffset := layout.DataRVA

	// 1. ELF64 Header (64 bytes)
	out.Write([]byte{0x7F, 'E', 'L', 'F', 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0})      // e_ident (ELF64, Little-Endian, SYSV)
	_ = binary.Write(&out, binary.LittleEndian, uint16(2))                          // e_type = ET_EXEC
	_ = binary.Write(&out, binary.LittleEndian, uint16(0x3E))                       // e_machine = EM_X86_64
	_ = binary.Write(&out, binary.LittleEndian, uint32(1))                          // e_version = EV_CURRENT
	_ = binary.Write(&out, binary.LittleEndian, elfBaseAddr+uint64(layout.TextRVA)) // e_entry
	_ = binary.Write(&out, binary.LittleEndian, uint64(64))                         // e_phoff = 64
	_ = binary.Write(&out, binary.LittleEndian, uint64(0))                          // e_shoff = 0
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))                          // e_flags = 0
	_ = binary.Write(&out, binary.LittleEndian, uint16(64))                         // e_ehsize = 64
	_ = binary.Write(&out, binary.LittleEndian, uint16(56))                         // e_phentsize = 56
	_ = binary.Write(&out, binary.LittleEndian, uint16(2))                          // e_phnum = 2 (Code+ROData, Data)
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))                          // e_shentsize = 0
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))                          // e_shnum = 0
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))                          // e_shstrndx = 0

	// 2. Program Header 0: PT_LOAD (PF_R | PF_X = 5) covering headers + .text + .rodata
	_ = binary.Write(&out, binary.LittleEndian, uint32(1))                   // p_type = PT_LOAD
	_ = binary.Write(&out, binary.LittleEndian, uint32(5))                   // p_flags = PF_R | PF_X
	_ = binary.Write(&out, binary.LittleEndian, uint64(0))                   // p_offset = 0
	_ = binary.Write(&out, binary.LittleEndian, elfBaseAddr)                 // p_vaddr
	_ = binary.Write(&out, binary.LittleEndian, elfBaseAddr)                 // p_paddr
	_ = binary.Write(&out, binary.LittleEndian, uint64(codeSegmentFileSize)) // p_filesz
	_ = binary.Write(&out, binary.LittleEndian, uint64(codeSegmentFileSize)) // p_memsz
	_ = binary.Write(&out, binary.LittleEndian, uint64(elfPageAlign))        // p_align

	// 3. Program Header 1: PT_LOAD (PF_R | PF_W = 6) covering .data
	_ = binary.Write(&out, binary.LittleEndian, uint32(1))                          // p_type = PT_LOAD
	_ = binary.Write(&out, binary.LittleEndian, uint32(6))                          // p_flags = PF_R | PF_W
	_ = binary.Write(&out, binary.LittleEndian, uint64(dataFileOffset))             // p_offset
	_ = binary.Write(&out, binary.LittleEndian, elfBaseAddr+uint64(layout.DataRVA)) // p_vaddr
	_ = binary.Write(&out, binary.LittleEndian, elfBaseAddr+uint64(layout.DataRVA)) // p_paddr
	_ = binary.Write(&out, binary.LittleEndian, uint64(dataSize))                   // p_filesz
	_ = binary.Write(&out, binary.LittleEndian, uint64(dataSize))                   // p_memsz
	_ = binary.Write(&out, binary.LittleEndian, uint64(elfPageAlign))               // p_align

	// Pad up to .text offset (0x1000)
	if uint32(out.Len()) < layout.TextRVA {
		out.Write(make([]byte, int(layout.TextRVA)-out.Len()))
	}

	// Write .text
	out.Write(textBytes)

	// Pad up to .rodata offset
	if uint32(out.Len()) < layout.RODataRVA {
		out.Write(make([]byte, int(layout.RODataRVA)-out.Len()))
	}

	// Write .rodata
	out.Write(rodataBytes)
	if len(rodataBytes) == 0 {
		out.WriteByte(0)
	}

	// Pad up to .data offset
	if uint32(out.Len()) < dataFileOffset {
		out.Write(make([]byte, int(dataFileOffset)-out.Len()))
	}

	// Write .data (zero-initialized)
	out.Write(make([]byte, dataSize))

	return out.Bytes()
}
