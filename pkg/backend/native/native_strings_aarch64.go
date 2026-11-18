package native

// emitNativeStringSearchSliceARM64 emits the generic UTF-8 string primitives
// used by std/strings. Indices count Unicode code points; emitted code only
// operates on UTF-8 bytes and contains no product-level policy.
func (b *ARM64Builder) emitNativeStringSearchSliceARM64() {
	b.emitNativeStringCaseARM64("fn_strtolower", false)
	b.emitNativeStringCaseARM64("fn_strtoupper", true)
	b.emitNativeStrposARM64()
	b.emitNativeSubstrARM64()
}

// emitNativeStringCaseARM64 implements the same byte-oriented ASCII case
// conversion contract as the AMD64 emitter. Unicode case mapping stays outside
// this ABI primitive, matching the current native string intrinsic contract.
func (b *ARM64Builder) emitNativeStringCaseARM64(label string, upper bool) {
	b.defineLabel(label)
	b.emitProlog(64)
	for i, reg := range []uint32{regX19, regX20, regX21, regX22} {
		b.emitStr(reg, regX29, uint32(16+i*8))
	}
	b.emitMovReg(regX19, regX0) // input string
	b.emitLoadImm64(regX20, 0)  // byte length
	b.defineLabel("." + label + "_length")
	b.emitAdd(regX2, regX19, regX20)
	b.emitLdrb(regX3, regX2, 0)
	b.emitCmp(regX3, regXZR)
	b.emitBranchCond(condEQ, "."+label+"_allocate")
	b.emitAddImm(regX20, regX20, 1)
	b.emitBranch("." + label + "_length")

	b.defineLabel("." + label + "_allocate")
	b.emitAddImm(regX0, regX20, 1)
	b.emitCall("__joss_os_alloc")
	b.emitMovReg(regX21, regX0) // output string
	b.emitLoadImm64(regX22, 0)
	b.defineLabel("." + label + "_loop")
	b.emitCmp(regX22, regX20)
	b.emitBranchCond(condGE, "."+label+"_done")
	b.emitAdd(regX2, regX19, regX22)
	b.emitLdrb(regX3, regX2, 0)
	if upper {
		b.emitCmpImm(regX3, 'a')
		b.emitBranchCond(condLT, "."+label+"_store")
		b.emitCmpImm(regX3, 'z')
		b.emitBranchCond(condGT, "."+label+"_store")
		b.emitSubImm(regX3, regX3, 32)
	} else {
		b.emitCmpImm(regX3, 'A')
		b.emitBranchCond(condLT, "."+label+"_store")
		b.emitCmpImm(regX3, 'Z')
		b.emitBranchCond(condGT, "."+label+"_store")
		b.emitAddImm(regX3, regX3, 32)
	}
	b.defineLabel("." + label + "_store")
	b.emitAdd(regX4, regX21, regX22)
	b.emitStrb(regX3, regX4, 0)
	b.emitAddImm(regX22, regX22, 1)
	b.emitBranch("." + label + "_loop")
	b.defineLabel("." + label + "_done")
	b.emitAdd(regX4, regX21, regX20)
	b.emitStrb(regXZR, regX4, 0)
	b.emitMovReg(regX0, regX21)
	for i, reg := range []uint32{regX19, regX20, regX21, regX22} {
		b.emitLdr(reg, regX29, uint32(16+i*8))
	}
	b.emitEpilog(64)
}

func (b *ARM64Builder) emitNativeStrposARM64() {
	b.defineLabel("fn_strpos")
	b.emitProlog(64)
	for i, reg := range []uint32{regX19, regX20, regX21, regX22, regX23} {
		b.emitStr(reg, regX29, uint32(16+i*8))
	}
	b.emitMovReg(regX19, regX0) // haystack
	b.emitMovReg(regX20, regX1) // needle
	b.emitLoadImm64(regX21, 0)  // candidate byte offset
	b.emitLdrb(regX2, regX20, 0)
	b.emitCmp(regX2, regXZR)
	b.emitBranchCond(condEQ, ".arm64_strpos_found")
	b.defineLabel(".arm64_strpos_outer")
	b.emitAdd(regX2, regX19, regX21)
	b.emitLdrb(regX3, regX2, 0)
	b.emitCmp(regX3, regXZR)
	b.emitBranchCond(condEQ, ".arm64_strpos_missing")
	b.emitMovReg(regX22, regX21) // haystack cursor
	b.emitMovReg(regX23, regX20) // needle cursor
	b.defineLabel(".arm64_strpos_inner")
	b.emitLdrb(regX4, regX23, 0)
	b.emitCmp(regX4, regXZR)
	b.emitBranchCond(condEQ, ".arm64_strpos_found")
	b.emitAdd(regX5, regX19, regX22)
	b.emitLdrb(regX6, regX5, 0)
	b.emitCmp(regX4, regX6)
	b.emitBranchCond(condNE, ".arm64_strpos_next")
	b.emitAddImm(regX22, regX22, 1)
	b.emitAddImm(regX23, regX23, 1)
	b.emitBranch(".arm64_strpos_inner")
	b.defineLabel(".arm64_strpos_next")
	b.emitAddImm(regX21, regX21, 1)
	b.emitBranch(".arm64_strpos_outer")
	b.defineLabel(".arm64_strpos_found")
	b.emitLoadImm64(regX0, 0) // character index
	b.emitLoadImm64(regX2, 0) // byte cursor
	b.defineLabel(".arm64_strpos_count")
	b.emitCmp(regX2, regX21)
	b.emitBranchCond(condGE, ".arm64_strpos_done")
	b.emitAdd(regX3, regX19, regX2)
	b.emitLdrb(regX4, regX3, 0)
	b.emitLoadImm64(regX5, 0xC0)
	b.emitLoadImm64(regX6, 0x80)
	b.emitAndW(regX7, regX4, regX5)
	b.emitCmp(regX7, regX6)
	b.emitBranchCond(condEQ, ".arm64_strpos_continuation")
	b.emitAddImm(regX0, regX0, 1)
	b.defineLabel(".arm64_strpos_continuation")
	b.emitAddImm(regX2, regX2, 1)
	b.emitBranch(".arm64_strpos_count")
	b.defineLabel(".arm64_strpos_missing")
	b.emitLoadImm64(regX0, 0) // false is represented as zero in native IR
	b.defineLabel(".arm64_strpos_done")
	for i, reg := range []uint32{regX19, regX20, regX21, regX22, regX23} {
		b.emitLdr(reg, regX29, uint32(16+i*8))
	}
	b.emitEpilog(64)
}

func (b *ARM64Builder) emitNativeSubstrARM64() {
	b.defineLabel("fn_substr")
	b.emitProlog(112)
	for i, reg := range []uint32{regX19, regX20, regX21, regX22, regX23, regX24, regX25, regX26, regX27, regX28} {
		b.emitStr(reg, regX29, uint32(16+i*8))
	}
	b.emitMovReg(regX19, regX0) // source
	b.emitMovReg(regX20, regX1) // requested start rune
	b.emitMovReg(regX21, regX2) // requested rune length; MaxInt64 means omitted
	b.emitMovReg(regX0, regX19)
	b.emitCall("fn_strlen")
	b.emitMovReg(regX22, regX0) // total runes

	b.emitCmp(regX20, regXZR)
	b.emitBranchCond(condGE, ".arm64_substr_start_nonnegative")
	b.emitAdd(regX20, regX22, regX20)
	b.emitCmp(regX20, regXZR)
	b.emitBranchCond(condGE, ".arm64_substr_start_nonnegative")
	b.emitLoadImm64(regX20, 0)
	b.defineLabel(".arm64_substr_start_nonnegative")
	b.emitCmp(regX20, regX22)
	b.emitBranchCond(condLE, ".arm64_substr_start_clamped")
	b.emitMovReg(regX20, regX22)
	b.emitLoadImm64(regX21, 0)
	b.emitBranch(".arm64_substr_length_ready")
	b.defineLabel(".arm64_substr_start_clamped")
	b.emitLoadImm64(regX0, ^uint64(0))
	b.emitCmp(regX21, regX0)
	b.emitBranchCond(condEQ, ".arm64_substr_length_omitted")
	b.emitCmp(regX21, regXZR)
	b.emitBranchCond(condGE, ".arm64_substr_length_positive")
	b.emitAdd(regX21, regX22, regX21) // negative length means end = total + length
	b.emitCmp(regX21, regX20)
	b.emitBranchCond(condGT, ".arm64_substr_negative_length_ready")
	b.emitLoadImm64(regX21, 0)
	b.emitBranch(".arm64_substr_length_ready")
	b.defineLabel(".arm64_substr_negative_length_ready")
	b.emitSub(regX21, regX21, regX20)
	b.emitBranch(".arm64_substr_length_ready")
	b.defineLabel(".arm64_substr_length_positive")
	b.emitSub(regX0, regX22, regX20)
	b.emitCmp(regX21, regX0)
	b.emitBranchCond(condLE, ".arm64_substr_length_ready")
	b.emitMovReg(regX21, regX0)
	b.emitBranch(".arm64_substr_length_ready")
	b.defineLabel(".arm64_substr_length_omitted")
	b.emitSub(regX21, regX22, regX20)
	b.defineLabel(".arm64_substr_length_ready")
	b.emitAdd(regX25, regX20, regX21) // exclusive end rune

	// Find the byte offset of the requested starting rune.
	b.emitLoadImm64(regX23, 0) // byte cursor
	b.emitLoadImm64(regX24, 0) // rune cursor
	b.defineLabel(".arm64_substr_find_start")
	b.emitCmp(regX24, regX20)
	b.emitBranchCond(condGE, ".arm64_substr_start_found")
	b.emitAdd(regX0, regX19, regX23)
	b.emitLdrb(regX1, regX0, 0)
	b.emitCmp(regX1, regXZR)
	b.emitBranchCond(condEQ, ".arm64_substr_start_found")
	b.emitLoadImm64(regX2, 0xC0)
	b.emitLoadImm64(regX3, 0x80)
	b.emitAndW(regX4, regX1, regX2)
	b.emitCmp(regX4, regX3)
	b.emitBranchCond(condEQ, ".arm64_substr_start_continuation")
	b.emitAddImm(regX24, regX24, 1)
	b.defineLabel(".arm64_substr_start_continuation")
	b.emitAddImm(regX23, regX23, 1)
	b.emitBranch(".arm64_substr_find_start")
	b.defineLabel(".arm64_substr_start_found")
	b.emitMovReg(regX26, regX23) // start byte offset

	// Continue to the exclusive end rune and retain its byte offset.
	b.defineLabel(".arm64_substr_find_end")
	b.emitCmp(regX24, regX25)
	b.emitBranchCond(condGE, ".arm64_substr_end_found")
	b.emitAdd(regX0, regX19, regX23)
	b.emitLdrb(regX1, regX0, 0)
	b.emitCmp(regX1, regXZR)
	b.emitBranchCond(condEQ, ".arm64_substr_end_found")
	b.emitLoadImm64(regX2, 0xC0)
	b.emitLoadImm64(regX3, 0x80)
	b.emitAndW(regX4, regX1, regX2)
	b.emitCmp(regX4, regX3)
	b.emitBranchCond(condEQ, ".arm64_substr_end_continuation")
	b.emitAddImm(regX24, regX24, 1)
	b.defineLabel(".arm64_substr_end_continuation")
	b.emitAddImm(regX23, regX23, 1)
	b.emitBranch(".arm64_substr_find_end")
	b.defineLabel(".arm64_substr_end_found")
	b.emitMovReg(regX28, regX23) // end byte offset
	b.emitSub(regX0, regX28, regX26)
	b.emitAddImm(regX0, regX0, 1) // include NUL terminator
	b.emitCall("__joss_os_alloc")
	b.emitMovReg(regX27, regX0) // result
	b.emitMovReg(regX23, regX26)
	b.emitLoadImm64(regX24, 0)
	b.defineLabel(".arm64_substr_copy")
	b.emitSub(regX0, regX28, regX26)
	b.emitCmp(regX24, regX0)
	b.emitBranchCond(condGE, ".arm64_substr_copy_done")
	b.emitAdd(regX1, regX19, regX23)
	b.emitLdrb(regX2, regX1, 0)
	b.emitAdd(regX1, regX27, regX24)
	b.emitStrb(regX2, regX1, 0)
	b.emitAddImm(regX23, regX23, 1)
	b.emitAddImm(regX24, regX24, 1)
	b.emitBranch(".arm64_substr_copy")
	b.defineLabel(".arm64_substr_copy_done")
	b.emitAdd(regX1, regX27, regX24)
	b.emitStrb(regXZR, regX1, 0)
	b.emitMovReg(regX0, regX27)
	for i, reg := range []uint32{regX19, regX20, regX21, regX22, regX23, regX24, regX25, regX26, regX27, regX28} {
		b.emitLdr(reg, regX29, uint32(16+i*8))
	}
	b.emitEpilog(112)
}
