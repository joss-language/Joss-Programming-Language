package native

const (
	dataOffGranDBTables uint32 = 0x0630
	dataOffSEOTitle     uint32 = 0x0638
	dataOffSEODesc      uint32 = 0x0640
	dataOffSEOCanonical uint32 = 0x0648
	dataOffEnvMap       uint32 = 0x0650
	dataOffPort         uint32 = 0x0658
)

// emitFundamentalRuntimePrimitives emits core standalone x86_64 primitives.
// GranDB, HTTP, and heavy Stdlib logic are purposefully omitted to respect the Single Source of Truth architecture.
func (b *MachineCodeBuilder) emitFundamentalRuntimePrimitives() {
	b.internString("0")

	// =========================================================================
	// 1. Truthiness, Escaping & String Conversion
	// =========================================================================
	// joss_is_truthy(rcx = val) -> rax = 1 if truthy, 0 if falsy (null, false, 0, "", empty array)
	b.defineLabel("joss_is_truthy")
	b.defineLabel("fn_is_truthy")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".truthy_false")
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJlLabel(".truthy_true")
	b.emitBytes(0x80, 0x39, 0x00) // cmp byte ptr [rcx], 0
	b.emitJeLabel(".truthy_false")
	b.defineLabel(".truthy_true")
	b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00, 0xC3) // mov eax, 1; ret
	b.defineLabel(".truthy_false")
	b.emitBytes(0x31, 0xC0, 0xC3) // xor eax, eax; ret

	// joss_is_empty(rcx = val) -> rax = 1 if empty, 0 if non-empty
	b.defineLabel("joss_is_empty")
	b.defineLabel("fn_is_empty")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".empty_true")
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJlLabel(".empty_false")
	b.emitBytes(0x80, 0x39, 0x00) // cmp byte ptr [rcx], 0
	b.emitJeLabel(".empty_true")
	b.defineLabel(".empty_false")
	b.emitBytes(0x31, 0xC0, 0xC3) // xor eax, eax; ret
	b.defineLabel(".empty_true")
	b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00, 0xC3) // mov eax, 1; ret

	// fn_html_escape(rcx = val) -> rax = strPtr ("" if 0/null, else joss_to_string)
	b.defineLabel("fn_html_escape")
	b.defineLabel("joss_html_escape")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJneLabel("joss_to_string")
	b.emitLeaRAXString("")
	b.emitBytes(0xC3)

	// joss_to_string(rcx = val) -> rax = strPtr
	b.defineLabel("joss_to_string")
	b.defineLabel("fn_strval")
	b.defineLabel("fn_to_string")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55) // push r12; push r13
	b.emitBytes(0x48, 0x83, 0xEC, 0x20) // sub rsp, 32
	b.emitBytes(0x48, 0x85, 0xC9)       // test rcx, rcx
	b.emitJeLabel(".tostr_zero")
	// If rcx >= 0x10000 and rcx > 0 (valid user-space pointer in .rodata or heap), return it as-is
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJlLabel(".tostr_int")
	b.emitBytes(0x48, 0x89, 0xC8) // mov rax, rcx
	b.emitJmpLabel(".tostr_exit")
	b.defineLabel(".tostr_zero")
	b.emitLeaRAXString("0")
	b.emitJmpLabel(".tostr_exit")
	b.defineLabel(".tostr_int")
	b.emitBytes(0x49, 0x89, 0xCC)             // mov r12, rcx (val)
	b.emitBytes(0xB9, 0x20, 0x00, 0x00, 0x00) // mov ecx, 32
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x49, 0x89, 0xC5)       // mov r13, rax (buf)
	b.emitBytes(0x4D, 0x8D, 0x4D, 0x1E) // lea r9, [r13 + 30]
	b.emitBytes(0x41, 0xC6, 0x01, 0x00) // mov byte ptr [r9], 0
	b.emitBytes(0x4C, 0x89, 0xE0)       // mov rax, r12
	b.emitBytes(0x45, 0x31, 0xD2)       // xor r10d, r10d (is_neg)
	b.emitBytes(0x48, 0x85, 0xC0)       // test rax, rax
	b.emitJnsLabel(".tostr_loop")
	b.emitBytes(0x48, 0xF7, 0xD8)                   // neg rax
	b.emitBytes(0x41, 0xBA, 0x01, 0x00, 0x00, 0x00) // mov r10d, 1
	b.defineLabel(".tostr_loop")
	b.emitBytes(0x31, 0xD2)                               // xor edx, edx
	b.emitBytes(0x49, 0xC7, 0xC3, 0x0A, 0x00, 0x00, 0x00) // mov r11, 10
	b.emitBytes(0x49, 0xF7, 0xF3)                         // div r11
	b.emitBytes(0x80, 0xC2, 0x30)                         // add dl, '0'
	b.emitBytes(0x49, 0xFF, 0xC9)                         // dec r9
	b.emitBytes(0x41, 0x88, 0x11)                         // mov [r9], dl
	b.emitBytes(0x48, 0x85, 0xC0)                         // test rax, rax
	b.emitJneLabel(".tostr_loop")
	b.emitBytes(0x45, 0x85, 0xD2) // test r10d, r10d
	b.emitJeLabel(".tostr_done_ptr")
	b.emitBytes(0x49, 0xFF, 0xC9)       // dec r9
	b.emitBytes(0x41, 0xC6, 0x01, 0x2D) // mov byte ptr [r9], '-'
	b.defineLabel(".tostr_done_ptr")
	b.emitBytes(0x4C, 0x89, 0xC8) // mov rax, r9
	b.defineLabel(".tostr_exit")
	b.emitBytes(0x48, 0x83, 0xC4, 0x20, 0x41, 0x5D, 0x41, 0x5C, 0x5D, 0xC3)

}
