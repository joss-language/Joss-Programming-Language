package native

import (
	"bytes"
	"encoding/binary"
)

const (
	macho64Magic   uint32 = 0xfeedfacf // MH_MAGIC_64
	machoCpuAmd64  uint32 = 0x01000007 // CPU_TYPE_X86_64
	machoCpuArm64  uint32 = 0x0100000c // CPU_TYPE_ARM64
	machoCpuSubAll uint32 = 0x80000003 // CPU_SUBTYPE_LIB64 | CPU_SUBTYPE_X86_ALL
	machoFileType  uint32 = 0x00000002 // MH_EXECUTE
	machoPageAlign uint32 = 0x1000
	machoBaseAddr  uint64 = 0x0000000100000000
)

// MachOImageLayout holds the computed RVAs for a 64-bit Mach-O binary (macOS amd64/arm64).
type MachOImageLayout struct {
	TextRVA   uint32
	RODataRVA uint32
	DataRVA   uint32
}

// PrepareMachOLayout computes section layouts for a 64-bit Mach-O executable.
func PrepareMachOLayout(textLen int, rodataLen int) *MachOImageLayout {
	rodataRVA := alignUpU32(uint32(textLen), 16)
	dataRVA := alignUpU32(rodataRVA+uint32(rodataLen)+16, machoPageAlign)
	return &MachOImageLayout{
		TextRVA:   0x1000,
		RODataRVA: rodataRVA + 0x1000,
		DataRVA:   dataRVA + 0x1000,
	}
}

// EmitMachO64Executable serializes a 64-bit Mach-O executable image for macOS.
func EmitMachO64Executable(layout *MachOImageLayout, textBytes []byte, rodataBytes []byte, dataSize uint32, arch string) []byte {
	var out bytes.Buffer

	cpuType := machoCpuAmd64
	if arch == "arm64" {
		cpuType = machoCpuArm64
	}

	// 1. Mach-O 64 Header (32 bytes)
	_ = binary.Write(&out, binary.LittleEndian, macho64Magic)
	_ = binary.Write(&out, binary.LittleEndian, cpuType)
	_ = binary.Write(&out, binary.LittleEndian, machoCpuSubAll)
	_ = binary.Write(&out, binary.LittleEndian, machoFileType)
	_ = binary.Write(&out, binary.LittleEndian, uint32(2))          // ncmds: 2 (LC_SEGMENT_64 __PAGEZERO, LC_SEGMENT_64 __TEXT)
	_ = binary.Write(&out, binary.LittleEndian, uint32(72+152))     // sizeofcmds
	_ = binary.Write(&out, binary.LittleEndian, uint32(0x00200085)) // flags: MH_NOUNDEFS | MH_DYLDLINK | MH_TWOLEVEL | MH_PIE
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))          // reserved

	// 2. Load Command 1: LC_SEGMENT_64 __PAGEZERO (72 bytes)
	_ = binary.Write(&out, binary.LittleEndian, uint32(0x19)) // LC_SEGMENT_64
	_ = binary.Write(&out, binary.LittleEndian, uint32(72))   // cmdsize
	var segZeroName [16]byte
	copy(segZeroName[:], "__PAGEZERO")
	out.Write(segZeroName[:])
	_ = binary.Write(&out, binary.LittleEndian, uint64(0))     // vmaddr
	_ = binary.Write(&out, binary.LittleEndian, machoBaseAddr) // vmsize (4 GB)
	_ = binary.Write(&out, binary.LittleEndian, uint64(0))     // fileoff
	_ = binary.Write(&out, binary.LittleEndian, uint64(0))     // filesize
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))     // maxprot (NONE)
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))     // initprot (NONE)
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))     // nsects
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))     // flags

	// 3. Load Command 2: LC_SEGMENT_64 __TEXT (152 bytes = 72 header + 80 section)
	_ = binary.Write(&out, binary.LittleEndian, uint32(0x19)) // LC_SEGMENT_64
	_ = binary.Write(&out, binary.LittleEndian, uint32(152))  // cmdsize (72 + 80)
	var segTextName [16]byte
	copy(segTextName[:], "__TEXT")
	out.Write(segTextName[:])
	_ = binary.Write(&out, binary.LittleEndian, machoBaseAddr)          // vmaddr
	_ = binary.Write(&out, binary.LittleEndian, uint64(0x1000))         // vmsize
	_ = binary.Write(&out, binary.LittleEndian, uint64(0))              // fileoff
	_ = binary.Write(&out, binary.LittleEndian, uint64(len(textBytes))) // filesize
	_ = binary.Write(&out, binary.LittleEndian, uint32(7))              // maxprot: rwx
	_ = binary.Write(&out, binary.LittleEndian, uint32(5))              // initprot: r-x
	_ = binary.Write(&out, binary.LittleEndian, uint32(1))              // nsects: 1 (__text)
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))              // flags

	// Section 1: __text (80 bytes)
	var sectTextName [16]byte
	copy(sectTextName[:], "__text")
	out.Write(sectTextName[:])
	out.Write(segTextName[:])                                                         // segname "__TEXT"
	_ = binary.Write(&out, binary.LittleEndian, machoBaseAddr+uint64(layout.TextRVA)) // addr
	_ = binary.Write(&out, binary.LittleEndian, uint64(len(textBytes)))               // size
	_ = binary.Write(&out, binary.LittleEndian, uint32(0x1000))                       // offset in file
	_ = binary.Write(&out, binary.LittleEndian, uint32(4))                            // align: 2^4 = 16
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))                            // reloff
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))                            // nreloc
	_ = binary.Write(&out, binary.LittleEndian, uint32(0x80000400))                   // flags: S_REGULAR | S_ATTR_SOME_INSTRUCTIONS | S_ATTR_PURE_INSTRUCTIONS
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))                            // reserved1
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))                            // reserved2
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))                            // reserved3

	// Pad header to 0x1000
	if out.Len() < int(machoPageAlign) {
		out.Write(make([]byte, int(machoPageAlign)-out.Len()))
	}

	// Write code
	out.Write(textBytes)
	out.Write(rodataBytes)

	return out.Bytes()
}
