package native

import (
	"bytes"
	"encoding/binary"
)

const (
	peImageBase        uint64 = 0x0000000140000000
	peSectionAlignment uint32 = 0x1000
	peFileAlignment    uint32 = 0x0200
	peTextRVA          uint32 = 0x1000
)

// WinImportSlot identifies the KERNEL32.dll and joss_host.dll IAT slots in the PE32+ binary.
type WinImportSlot int

const (
	IATGetStdHandle WinImportSlot = iota
	IATWriteFile
	IATGetProcessHeap
	IATHeapAlloc
	IATExitProcess
	IATLoadLibraryA
	IATGetProcAddress
	IATJossHostBridge // For _joss_host_bridge in joss_host.dll
	iatSlotCount
)

// PEImageLayout holds the computed RVAs and binary buffers for a Windows x86_64 PE32+ executable.
type PEImageLayout struct {
	TextRVA   uint32
	RDataRVA  uint32
	DataRVA   uint32
	IATRVAs   [iatSlotCount]uint32
	RODataRVA uint32
	RDataBuf  []byte
}

// PreparePERData constructs the .rdata section containing the KERNEL32.dll and joss_host.dll Import Table
// (IAT, Import Directory Table, ILT, Hint/Name table) followed by read-only program data (rodata).
func PreparePERData(textLen int, rodata []byte) *PEImageLayout {
	rdataRVA := alignUpU32(peTextRVA+uint32(textLen), peSectionAlignment)

	// Layout inside .rdata:
	// [0x00 .. 0x48): IAT for KERNEL32 (7 entries + 1 null = 64 bytes) + IAT for joss_host (1 entry + 1 null = 16 bytes) (Total 80 bytes)
	// [0x50 .. 0x8C): Import Directory Table (3 * 20 bytes = 60 bytes)
	// [0x8C .. 0xDC): ILT for KERNEL32 (64 bytes) + ILT for joss_host (16 bytes) (Total 80 bytes)
	// [0xDC ..     ): DLL Names ("KERNEL32.dll\0", "joss_host.dll\0") + Hint/Name entries
	const (
		iatOffset = 0x00
		idtOffset = 0x50
		iltOffset = 0x8C
		strOffset = 0xDC
	)

	var tail bytes.Buffer
	dll1NameRVA := rdataRVA + strOffset
	tail.WriteString("KERNEL32.dll\x00")
	if tail.Len()%2 != 0 {
		tail.WriteByte(0)
	}

	dll2NameRVA := rdataRVA + strOffset + uint32(tail.Len())
	tail.WriteString("joss_host.dll\x00")
	if tail.Len()%2 != 0 {
		tail.WriteByte(0)
	}

	funcNames := [iatSlotCount]string{
		"GetStdHandle",
		"WriteFile",
		"GetProcessHeap",
		"HeapAlloc",
		"ExitProcess",
		"LoadLibraryA",
		"GetProcAddress",
		"_joss_host_bridge", // slot 7
	}

	var hintNameRVAs [iatSlotCount]uint32
	for i, name := range funcNames {
		if tail.Len()%2 != 0 {
			tail.WriteByte(0)
		}
		hintNameRVAs[i] = rdataRVA + strOffset + uint32(tail.Len())
		_ = binary.Write(&tail, binary.LittleEndian, uint16(0)) // Hint = 0
		tail.WriteString(name)
		tail.WriteByte(0)
	}
	for tail.Len()%8 != 0 {
		tail.WriteByte(0)
	}

	rodataOffset := strOffset + uint32(tail.Len())
	rodataRVA := rdataRVA + rodataOffset

	var rdata bytes.Buffer
	// 1. IAT (80 bytes)
	var iatRVAs [iatSlotCount]uint32
	// KERNEL32.dll IAT
	for i := 0; i < 7; i++ {
		iatRVAs[i] = rdataRVA + iatOffset + uint32(i*8)
		_ = binary.Write(&rdata, binary.LittleEndian, uint64(hintNameRVAs[i]))
	}
	_ = binary.Write(&rdata, binary.LittleEndian, uint64(0)) // Null terminator for KERNEL32
	
	// joss_host.dll IAT
	iatRVAs[7] = rdataRVA + iatOffset + 64
	_ = binary.Write(&rdata, binary.LittleEndian, uint64(hintNameRVAs[7]))
	_ = binary.Write(&rdata, binary.LittleEndian, uint64(0)) // Null terminator for joss_host.dll

	// 2. Import Directory Table (60 bytes)
	// Entry 0: KERNEL32.dll
	_ = binary.Write(&rdata, binary.LittleEndian, rdataRVA+iltOffset) // OriginalFirstThunk (ILT RVA)
	_ = binary.Write(&rdata, binary.LittleEndian, uint32(0))          // TimeDateStamp
	_ = binary.Write(&rdata, binary.LittleEndian, uint32(0))          // ForwarderChain
	_ = binary.Write(&rdata, binary.LittleEndian, dll1NameRVA)         // Name RVA
	_ = binary.Write(&rdata, binary.LittleEndian, rdataRVA+iatOffset) // FirstThunk (IAT RVA)
	
	// Entry 1: joss_host.dll
	_ = binary.Write(&rdata, binary.LittleEndian, rdataRVA+iltOffset+64) // OriginalFirstThunk (ILT RVA)
	_ = binary.Write(&rdata, binary.LittleEndian, uint32(0))          // TimeDateStamp
	_ = binary.Write(&rdata, binary.LittleEndian, uint32(0))          // ForwarderChain
	_ = binary.Write(&rdata, binary.LittleEndian, dll2NameRVA)         // Name RVA
	_ = binary.Write(&rdata, binary.LittleEndian, rdataRVA+iatOffset+64) // FirstThunk (IAT RVA)
	
	// Entry 2: Null terminator (20 bytes)
	rdata.Write(make([]byte, 20))

	// 3. ILT (80 bytes)
	// KERNEL32.dll ILT
	for i := 0; i < 7; i++ {
		_ = binary.Write(&rdata, binary.LittleEndian, uint64(hintNameRVAs[i]))
	}
	_ = binary.Write(&rdata, binary.LittleEndian, uint64(0))
	// joss_host.dll ILT
	_ = binary.Write(&rdata, binary.LittleEndian, uint64(hintNameRVAs[7]))
	_ = binary.Write(&rdata, binary.LittleEndian, uint64(0))

	// 4. Strings + Hint/Name table
	rdata.Write(tail.Bytes())

	// 5. Read-only string literals pool (rodata)
	rdata.Write(rodata)
	if len(rodata) == 0 {
		rdata.WriteByte(0)
	}

	dataRVA := alignUpU32(rdataRVA+uint32(rdata.Len()), peSectionAlignment)

	return &PEImageLayout{
		TextRVA:   peTextRVA,
		RDataRVA:  rdataRVA,
		DataRVA:   dataRVA,
		IATRVAs:   iatRVAs,
		RODataRVA: rodataRVA,
		RDataBuf:  rdata.Bytes(),
	}
}

// EmitPE32PlusExecutable serializes a complete Windows x86_64 PE32+ executable (.exe)
// from the assembled .text, .rdata (via PEImageLayout), and .data sections.
func EmitPE32PlusExecutable(layout *PEImageLayout, textBytes []byte, dataSize uint32, guiSubsystem bool) []byte {
	const (
		headersFileSize = 0x0200 // 512 bytes (DOS 64 + PE 4 + COFF 20 + Opt 240 + 3*40 SecHdr = 448 <= 512)
		idtOffsetInRDat = 0x50
		idtSize         = 60
		iatOffsetInRDat = 0x00
		iatSize         = 80
	)

	textRawSize := alignUpU32(uint32(len(textBytes)), peFileAlignment)
	rdataRawSize := alignUpU32(uint32(len(layout.RDataBuf)), peFileAlignment)
	dataRawSize := alignUpU32(dataSize, peFileAlignment)

	textFileOffset := uint32(headersFileSize)
	rdataFileOffset := textFileOffset + textRawSize
	dataFileOffset := rdataFileOffset + rdataRawSize

	sizeOfImage := alignUpU32(layout.DataRVA+dataSize, peSectionAlignment)

	var out bytes.Buffer

	// 1. DOS Header (64 bytes)
	dosHeader := make([]byte, 64)
	binary.LittleEndian.PutUint16(dosHeader[0:2], 0x5A4D)   // "MZ"
	binary.LittleEndian.PutUint32(dosHeader[60:64], 0x0040) // e_lfanew = 64
	out.Write(dosHeader)

	// 2. PE Signature (4 bytes)
	out.WriteString("PE\x00\x00")

	// 3. COFF File Header (20 bytes)
	_ = binary.Write(&out, binary.LittleEndian, uint16(0x8664)) // Machine = IMAGE_FILE_MACHINE_AMD64
	_ = binary.Write(&out, binary.LittleEndian, uint16(3))      // NumberOfSections = 3 (.text, .rdata, .data)
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))      // TimeDateStamp
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))      // PointerToSymbolTable
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))      // NumberOfSymbols
	_ = binary.Write(&out, binary.LittleEndian, uint16(240))    // SizeOfOptionalHeader = 240 (PE32+)
	_ = binary.Write(&out, binary.LittleEndian, uint16(0x0022)) // Characteristics = EXECUTABLE_IMAGE | LARGE_ADDRESS_AWARE

	// 4. Optional Header PE32+ (240 bytes)
	subsystem := uint16(3) // IMAGE_SUBSYSTEM_WINDOWS_CUI
	if guiSubsystem {
		subsystem = 2 // IMAGE_SUBSYSTEM_WINDOWS_GUI
	}
	_ = binary.Write(&out, binary.LittleEndian, uint16(0x020B))           // Magic = PE32+
	_ = binary.Write(&out, binary.LittleEndian, uint8(1))                 // MajorLinkerVersion
	_ = binary.Write(&out, binary.LittleEndian, uint8(0))                 // MinorLinkerVersion
	_ = binary.Write(&out, binary.LittleEndian, textRawSize)              // SizeOfCode
	_ = binary.Write(&out, binary.LittleEndian, rdataRawSize+dataRawSize) // SizeOfInitializedData
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))                // SizeOfUninitializedData
	_ = binary.Write(&out, binary.LittleEndian, layout.TextRVA)           // AddressOfEntryPoint (_start at RVA 0x1000)
	_ = binary.Write(&out, binary.LittleEndian, layout.TextRVA)           // BaseOfCode
	_ = binary.Write(&out, binary.LittleEndian, peImageBase)              // ImageBase
	_ = binary.Write(&out, binary.LittleEndian, peSectionAlignment)       // SectionAlignment
	_ = binary.Write(&out, binary.LittleEndian, peFileAlignment)          // FileAlignment
	_ = binary.Write(&out, binary.LittleEndian, uint16(6))                // MajorOperatingSystemVersion
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))                // MinorOperatingSystemVersion
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))                // MajorImageVersion
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))                // MinorImageVersion
	_ = binary.Write(&out, binary.LittleEndian, uint16(6))                // MajorSubsystemVersion
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))                // MinorSubsystemVersion
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))                // Win32VersionValue
	_ = binary.Write(&out, binary.LittleEndian, sizeOfImage)              // SizeOfImage
	_ = binary.Write(&out, binary.LittleEndian, uint32(headersFileSize))  // SizeOfHeaders
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))                // CheckSum
	_ = binary.Write(&out, binary.LittleEndian, subsystem)                // Subsystem
	_ = binary.Write(&out, binary.LittleEndian, uint16(0x0100))           // DllCharacteristics (NX_COMPAT)
	_ = binary.Write(&out, binary.LittleEndian, uint64(0x00400000))       // SizeOfStackReserve (4 MB)
	_ = binary.Write(&out, binary.LittleEndian, uint64(0x00010000))       // SizeOfStackCommit (64 KB)
	_ = binary.Write(&out, binary.LittleEndian, uint64(0x01000000))       // SizeOfHeapReserve (16 MB)
	_ = binary.Write(&out, binary.LittleEndian, uint64(0x00010000))       // SizeOfHeapCommit (64 KB)
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))                // LoaderFlags
	_ = binary.Write(&out, binary.LittleEndian, uint32(16))               // NumberOfRvaAndSizes

	// Data Directories (16 * 8 = 128 bytes)
	var dataDirs [16][2]uint32
	dataDirs[1] = [2]uint32{layout.RDataRVA + idtOffsetInRDat, idtSize}  // Import Table
	dataDirs[12] = [2]uint32{layout.RDataRVA + iatOffsetInRDat, iatSize} // IAT
	for _, dd := range dataDirs {
		_ = binary.Write(&out, binary.LittleEndian, dd[0])
		_ = binary.Write(&out, binary.LittleEndian, dd[1])
	}

	// 5. Section Headers (3 * 40 = 120 bytes)
	writePESectionHeader(&out, ".text", uint32(len(textBytes)), layout.TextRVA, textRawSize, textFileOffset, 0x60000020)
	writePESectionHeader(&out, ".rdata", uint32(len(layout.RDataBuf)), layout.RDataRVA, rdataRawSize, rdataFileOffset, 0x40000040)
	writePESectionHeader(&out, ".data", dataSize, layout.DataRVA, dataRawSize, dataFileOffset, 0xC0000040)

	// Pad headers to headersFileSize (512 bytes)
	if out.Len() < headersFileSize {
		out.Write(make([]byte, headersFileSize-out.Len()))
	}

	// 6. Write .text section padded to FileAlignment
	out.Write(textBytes)
	if rem := int(textRawSize) - len(textBytes); rem > 0 {
		out.Write(make([]byte, rem))
	}

	// 7. Write .rdata section padded to FileAlignment
	out.Write(layout.RDataBuf)
	if rem := int(rdataRawSize) - len(layout.RDataBuf); rem > 0 {
		out.Write(make([]byte, rem))
	}

	// 8. Write .data section (zero-initialized globals) padded to FileAlignment
	out.Write(make([]byte, dataRawSize))

	return out.Bytes()
}

func writePESectionHeader(out *bytes.Buffer, name string, virtualSize, virtualAddr, rawSize, rawPtr, characteristics uint32) {
	var secName [8]byte
	copy(secName[:], name)
	out.Write(secName[:])
	_ = binary.Write(out, binary.LittleEndian, virtualSize)
	_ = binary.Write(out, binary.LittleEndian, virtualAddr)
	_ = binary.Write(out, binary.LittleEndian, rawSize)
	_ = binary.Write(out, binary.LittleEndian, rawPtr)
	_ = binary.Write(out, binary.LittleEndian, uint32(0)) // PointerToRelocations
	_ = binary.Write(out, binary.LittleEndian, uint32(0)) // PointerToLinenumbers
	_ = binary.Write(out, binary.LittleEndian, uint16(0)) // NumberOfRelocations
	_ = binary.Write(out, binary.LittleEndian, uint16(0)) // NumberOfLinenumbers
	_ = binary.Write(out, binary.LittleEndian, characteristics)
}

func alignUpU32(val, align uint32) uint32 {
	if align == 0 {
		return val
	}
	return (val + align - 1) & ^(align - 1)
}
