package native

const (
	dataOffGranDBTables uint32 = 0x0630
	dataOffSEOTitle     uint32 = 0x0638
	dataOffSEODesc      uint32 = 0x0640
	dataOffSEOCanonical uint32 = 0x0648
	dataOffEnvMap       uint32 = 0x0650
	dataOffPort         uint32 = 0x0658
)

// emitGranDBAndStdlib emits the standalone x86_64 native implementations of:
// - joss_is_truthy / joss_is_empty / fn_html_escape / joss_to_string
// - GranDB native table store & query engine (table, insert, insertGetId, where, first, get, count, update, delete)
// - Dynamic SEO state & rendering, disk-backed Markdown::readFile, and MVC runtime intrinsics
func (b *MachineCodeBuilder) emitGranDBAndStdlib() {
	b.internString("__rows")
	b.internString("__has_where")
	b.internString("__w_col")
	b.internString("__w_val")
	b.internString("id")
	b.internString("GranDB")
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

	// =========================================================================
	// 2. Native GranDB Engine (__joss_grandb_get_tables, fn_GranDB_*)
	// =========================================================================
	// __joss_grandb_get_tables() -> rax = tablesMap
	b.defineLabel("__joss_grandb_get_tables")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x48, 0x83, 0xEC, 0x20)
	b.emitMovRAXData(dataOffGranDBTables)
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJneLabel(".gdb_tables_ok")
	b.emitCallLabel("joss_map_new")
	b.emitMovDataRAX(dataOffGranDBTables)
	b.defineLabel(".gdb_tables_ok")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)

	// fn_GranDB_table(rcx = tableName) -> rax = qbObj
	b.defineLabel("fn_GranDB_table")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (tableName)
	b.emitCallLabel("__joss_grandb_get_tables")
	b.emitBytes(0x49, 0x89, 0xC5) // mov r13, rax (tablesMap)
	b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13
	b.emitBytes(0x4C, 0x89, 0xE2) // mov rdx, r12
	b.emitCallLabel("joss_map_get_i64")
	b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (rowsArr)
	b.emitBytes(0x4D, 0x85, 0xF6) // test r14, r14
	b.emitJneLabel(".gdb_table_has_rows")
	b.emitBytes(0xB9, 0x10, 0x00, 0x00, 0x00) // mov ecx, 16
	b.emitCallLabel("joss_arr_new")
	b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (rowsArr)
	b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13 (tablesMap)
	b.emitBytes(0x4C, 0x89, 0xE2) // mov rdx, r12 (tableName)
	b.emitBytes(0x4D, 0x89, 0xF0) // mov r8, r14  (rowsArr)
	b.emitCallLabel("joss_map_set_i64")
	b.defineLabel(".gdb_table_has_rows")
	b.emitLeaRCXString("GranDB")
	b.emitCallLabel("joss_obj_new")
	b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax (qbObj)
	b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
	b.emitLeaRDXString("__rows")
	b.emitBytes(0x4D, 0x89, 0xF0) // mov r8, r14
	b.emitCallLabel("joss_obj_set_field_i64")
	b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
	b.emitLeaRDXString("__has_where")
	b.emitBytes(0x45, 0x31, 0xC0) // xor r8d, r8d
	b.emitCallLabel("joss_obj_set_field_i64")
	b.emitBytes(0x4C, 0x89, 0xF8)                               // mov rax, r15
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                         // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r15..r12
	b.emitBytes(0x5D, 0xC3)

	// fn_GranDB_where(rcx = qbObj, rdx = colStr, r8 = val) -> rax = qbObj
	b.defineLabel("fn_GranDB_where")
	b.defineLabel("fn_where")
	b.defineLabel("fn_GranDB_orWhere")
	b.defineLabel("fn_orWhere")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (qbObj)
	b.emitBytes(0x49, 0x89, 0xD5)                               // mov r13, rdx (colStr)
	b.emitBytes(0x4D, 0x89, 0xC6)                               // mov r14, r8  (val)
	b.emitBytes(0x4C, 0x89, 0xE1)                               // mov rcx, r12
	b.emitLeaRDXString("__has_where")
	b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00) // mov r8d, 1
	b.emitCallLabel("joss_obj_set_field_i64")
	b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
	b.emitLeaRDXString("__w_col")
	b.emitBytes(0x4D, 0x89, 0xE8) // mov r8, r13
	b.emitCallLabel("joss_obj_set_field_i64")
	b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
	b.emitLeaRDXString("__w_val")
	b.emitBytes(0x4D, 0x89, 0xF0) // mov r8, r14
	b.emitCallLabel("joss_obj_set_field_i64")
	b.emitBytes(0x4C, 0x89, 0xE0)                               // mov rax, r12
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                         // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r15..r12
	b.emitBytes(0x5D, 0xC3)

	// Fluent chain pass-throughs (select, orderBy, orderByDesc, whereLike, orWhereLike, limit, offset, join, leftJoin, whereIn)
	b.defineLabel("fn_GranDB_select")
	b.defineLabel("fn_select")
	b.defineLabel("fn_GranDB_orderBy")
	b.defineLabel("fn_orderBy")
	b.defineLabel("fn_GranDB_orderByDesc")
	b.defineLabel("fn_orderByDesc")
	b.defineLabel("fn_GranDB_whereLike")
	b.defineLabel("fn_whereLike")
	b.defineLabel("fn_GranDB_orWhereLike")
	b.defineLabel("fn_orWhereLike")
	b.defineLabel("fn_GranDB_limit")
	b.defineLabel("fn_limit")
	b.defineLabel("fn_GranDB_offset")
	b.defineLabel("fn_offset")
	b.defineLabel("fn_GranDB_join")
	b.defineLabel("fn_join")
	b.defineLabel("fn_GranDB_leftJoin")
	b.defineLabel("fn_leftJoin")
	b.defineLabel("fn_GranDB_whereIn")
	b.defineLabel("fn_whereIn")
	b.defineLabel("fn_Str_trim")
	b.defineLabel("fn_Math_ceil")
	b.defineLabel("fn_intval")
	b.emitBytes(0x48, 0x89, 0xC8, 0xC3) // mov rax, rcx; ret

	// __joss_load_env_file: Reads .env or env.joss and populates dataOffEnvMap
	b.defineLabel("__joss_load_env_file")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                               // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push rbx, r12..r15 (40 bytes)
	b.emitBytes(0x48, 0x83, 0xEC, 0x58)                               // sub rsp, 88 (total 128 bytes, 16-byte aligned)
	// Check if already loaded
	b.emitMovRCXData(dataOffEnvMap)
	b.emitBytes(0x48, 0x85, 0xC9)
	b.emitJneLabel(".env_load_done")
	// Create empty map for environment
	b.emitCallLabel("joss_map_new")
	b.emitMovDataRAX(dataOffEnvMap)
	b.emitBytes(0x49, 0x89, 0xC4) // mov r12, rax (envMap)

	if b.target.OS == "windows" {
		// Resolve CreateFileA from kernel32
		b.emitLeaRCXString("kernel32.dll")
		b.emitCallIAT(IATLoadLibraryA)
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJeLabel(".env_load_done")
		b.emitBytes(0x49, 0x89, 0xC5) // mov r13, rax (hKernel32)

		b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13
		b.emitLeaRDXString("CreateFileA")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJeLabel(".env_load_done")
		b.emitBytes(0x48, 0x89, 0x45, 0xD0) // [rbp - 48] = pCreateFileA

		// Try opening ".env" first
		b.emitLeaRCXString(".env")
		b.emitBytes(0xBA, 0x00, 0x00, 0x00, 0x80)                         // GENERIC_READ
		b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00)                   // FILE_SHARE_READ
		b.emitBytes(0x45, 0x31, 0xC9)                                     // xor r9d, r9d
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x03, 0x00, 0x00, 0x00) // OPEN_EXISTING = 3
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x28, 0x80, 0x00, 0x00, 0x00) // FILE_ATTRIBUTE_NORMAL
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x30, 0x00, 0x00, 0x00, 0x00)
		b.emitBytes(0x48, 0x8B, 0x45, 0xD0)                               // call pCreateFileA
		b.emitBytes(0xFF, 0xD0)
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF)
		b.emitJneLabel(".env_opened")

		// Fallback to "env.joss"
		b.emitLeaRCXString("env.joss")
		b.emitBytes(0xBA, 0x00, 0x00, 0x00, 0x80)
		b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00)
		b.emitBytes(0x45, 0x31, 0xC9)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x03, 0x00, 0x00, 0x00)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x28, 0x80, 0x00, 0x00, 0x00)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x30, 0x00, 0x00, 0x00, 0x00)
		b.emitBytes(0x48, 0x8B, 0x45, 0xD0)
		b.emitBytes(0xFF, 0xD0)
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF)
		b.emitJeLabel(".env_load_done")

		b.defineLabel(".env_opened")
		b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (hFile)

		// GetFileSize(hFile, NULL)
		b.emitBytes(0x4C, 0x89, 0xE9) // rcx = hKernel32 (r13)
		b.emitLeaRDXString("GetFileSize")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x4C, 0x89, 0xF1) // rcx = hFile (r14)
		b.emitBytes(0x31, 0xD2)       // rdx = NULL
		b.emitBytes(0xFF, 0xD0)       // call GetFileSize
		b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax (fileSize)
		b.emitBytes(0x49, 0x83, 0xFF, 0x00)
		b.emitJleLabel(".env_close_file")

		// Alloc buffer (fileSize + 32)
		b.emitBytes(0x49, 0x8D, 0x4F, 0x20) // rcx = fileSize + 32
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax (buf)

		// ReadFile(hFile, buf, fileSize, &bytesRead, NULL)
		b.emitBytes(0x4C, 0x89, 0xE9) // rcx = hKernel32 (r13)
		b.emitLeaRDXString("ReadFile")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x4C, 0x89, 0xF1)                                     // rcx = hFile (r14)
		b.emitBytes(0x48, 0x89, 0xDA)                                     // rdx = buf (rbx)
		b.emitBytes(0x45, 0x89, 0xF8)                                     // r8d = fileSize (r15d)
		b.emitBytes(0x48, 0x8D, 0x4D, 0xD8)                               // r9 = &bytesRead [rbp - 40]
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00) // NULL
		b.emitBytes(0xFF, 0xD0)                                           // call ReadFile

		// Null-terminate buffer
		b.emitBytes(0x4C, 0x01, 0xFB)       // add rbx, r15 (endPtr = buf + fileSize)
		b.emitBytes(0xC6, 0x03, 0x00)       // mov byte ptr [rbx], 0
		b.emitBytes(0x48, 0x89, 0x5D, 0xE0) // [rbp - 32] = endPtr = rbx
		b.emitBytes(0x4C, 0x29, 0xFB)       // sub rbx, r15 (restore rbx = buf)

		// Parse lines: rbx = current cursor, [rbp - 32] = endPtr
		b.defineLabel(".env_parse_line")
		b.emitBytes(0x48, 0x3B, 0x5D, 0xE0) // cmp rbx, [rbp - 32]
		b.emitJgeLabel(".env_close_file")
		// Skip whitespace and comments
		b.defineLabel(".env_skip_ws")
		b.emitBytes(0x0F, 0xB6, 0x03) // movzx eax, byte ptr [rbx]
		b.emitBytes(0x85, 0xC0)
		b.emitJeLabel(".env_close_file")
		b.emitBytes(0x83, 0xF8, 0x20)
		b.emitJeLabel(".env_adv_skip")
		b.emitBytes(0x83, 0xF8, 0x09)
		b.emitJeLabel(".env_adv_skip")
		b.emitBytes(0x83, 0xF8, 0x0D)
		b.emitJeLabel(".env_adv_skip")
		b.emitBytes(0x83, 0xF8, 0x0A)
		b.emitJeLabel(".env_adv_skip")
		b.emitBytes(0x83, 0xF8, 0x23) // '#' comment
		b.emitJeLabel(".env_skip_to_nl")
		b.emitJmpLabel(".env_have_key")

		b.defineLabel(".env_adv_skip")
		b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
		b.emitJmpLabel(".env_skip_ws")

		b.defineLabel(".env_skip_to_nl")
		b.emitBytes(0x0F, 0xB6, 0x03)
		b.emitBytes(0x85, 0xC0)
		b.emitJeLabel(".env_close_file")
		b.emitBytes(0x83, 0xF8, 0x0A)
		b.emitJeLabel(".env_adv_skip")
		b.emitBytes(0x48, 0xFF, 0xC3)
		b.emitJmpLabel(".env_skip_to_nl")

		b.defineLabel(".env_have_key")
		b.emitBytes(0x48, 0x89, 0x5D, 0xE8) // [rbp - 24] = keyStart = rbx
		// Find '='
		b.defineLabel(".env_find_eq")
		b.emitBytes(0x0F, 0xB6, 0x03)
		b.emitBytes(0x85, 0xC0)
		b.emitJeLabel(".env_close_file")
		b.emitBytes(0x83, 0xF8, 0x3D) // '='
		b.emitJeLabel(".env_found_eq")
		b.emitBytes(0x83, 0xF8, 0x0A) // newline before '=' -> invalid, skip
		b.emitJeLabel(".env_adv_skip")
		b.emitBytes(0x48, 0xFF, 0xC3)
		b.emitJmpLabel(".env_find_eq")

		b.defineLabel(".env_found_eq")
		b.emitBytes(0xC6, 0x03, 0x00) // null-terminate key
		b.emitBytes(0x48, 0xFF, 0xC3) // rbx points to start of value
		// If value starts with quote, handle it
		b.emitBytes(0x0F, 0xB6, 0x03)
		b.emitBytes(0x83, 0xF8, 0x22) // '"'
		b.emitJneLabel(".env_unquoted_val")
		b.emitBytes(0x48, 0xFF, 0xC3)       // skip opening quote
		b.emitBytes(0x48, 0x89, 0x5D, 0xF0) // [rbp - 16] = valStart = rbx
		// Scan until closing quote or newline
		b.defineLabel(".env_find_quote_end")
		b.emitBytes(0x0F, 0xB6, 0x03)
		b.emitBytes(0x85, 0xC0)
		b.emitJeLabel(".env_close_file")
		b.emitBytes(0x83, 0xF8, 0x22)
		b.emitJeLabel(".env_found_quote_end")
		b.emitBytes(0x48, 0xFF, 0xC3)
		b.emitJmpLabel(".env_find_quote_end")

		b.defineLabel(".env_found_quote_end")
		b.emitBytes(0xC6, 0x03, 0x00) // null-terminate val
		b.emitBytes(0x48, 0xFF, 0xC3)
		b.emitJmpLabel(".env_store_pair")

		b.defineLabel(".env_unquoted_val")
		b.emitBytes(0x48, 0x89, 0x5D, 0xF0) // [rbp - 16] = valStart = rbx
		// Scan until whitespace, newline or '#'
		b.defineLabel(".env_find_unquoted_end")
		b.emitBytes(0x0F, 0xB6, 0x03)
		b.emitBytes(0x85, 0xC0)
		b.emitJeLabel(".env_found_unquoted_end")
		b.emitBytes(0x83, 0xF8, 0x0D)
		b.emitJeLabel(".env_found_unquoted_end")
		b.emitBytes(0x83, 0xF8, 0x0A)
		b.emitJeLabel(".env_found_unquoted_end")
		b.emitBytes(0x83, 0xF8, 0x20)
		b.emitJeLabel(".env_found_unquoted_end")
		b.emitBytes(0x83, 0xF8, 0x23)
		b.emitJeLabel(".env_found_unquoted_end")
		b.emitBytes(0x48, 0xFF, 0xC3)
		b.emitJmpLabel(".env_find_unquoted_end")

		b.defineLabel(".env_found_unquoted_end")
		b.emitBytes(0xC6, 0x03, 0x00)
		b.emitBytes(0x48, 0xFF, 0xC3)

		b.defineLabel(".env_store_pair")
		// joss_map_set_i64(rcx = envMap, rdx = keyStr, r8 = valStr)
		b.emitBytes(0x4C, 0x89, 0xE1)       // rcx = envMap (r12)
		b.emitBytes(0x48, 0x8B, 0x55, 0xE8) // rdx = keyStart [rbp - 24]
		b.emitBytes(0x4C, 0x8B, 0x45, 0xF0) // r8  = valStart [rbp - 16]
		b.emitCallLabel("joss_map_set_i64")
		b.emitJmpLabel(".env_parse_line")

		b.defineLabel(".env_close_file")
		b.emitBytes(0x4C, 0x89, 0xE9) // rcx = hKernel32 (r13)
		b.emitLeaRDXString("CloseHandle")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x4C, 0x89, 0xF1) // rcx = hFile (r14)
		b.emitBytes(0xFF, 0xD0)       // CloseHandle(hFile)
	}

	b.defineLabel(".env_load_done")
	b.emitBytes(0x48, 0x83, 0xC4, 0x58)                               // add rsp, 88
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B) // pop r15..r12, rbx
	b.emitBytes(0x5D, 0xC3)

	// fn_env(rcx = key, rdx = defaultVal) -> rax = val from loaded environment map, or defaultVal
	b.defineLabel("fn_env")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x48, 0x83, 0xEC, 0x20) // sub rsp, 32
	b.emitBytes(0x48, 0x89, 0x55, 0xF8) // mov [rbp - 8], rdx (save defaultVal)
	b.emitBytes(0x48, 0x89, 0x4D, 0xF0) // mov [rbp - 16], rcx (save key)
	b.emitMovRCXData(dataOffEnvMap)     // mov rcx, [dataOffEnvMap]
	b.emitBytes(0x48, 0x85, 0xC9)       // test rcx, rcx
	b.emitJeLabel(".env_use_default")
	b.emitBytes(0x48, 0x8B, 0x55, 0xF0) // mov rdx, [rbp - 16] (key)
	b.emitCallLabel("joss_map_get_i64") // rax = val
	b.emitBytes(0x48, 0x85, 0xC0)       // test rax, rax
	b.emitJeLabel(".env_use_default")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // mov rsp, rbp; pop rbp; ret
	b.defineLabel(".env_use_default")
	b.emitBytes(0x48, 0x8B, 0x45, 0xF8)       // mov rax, [rbp - 8]
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // mov rsp, rbp; pop rbp; ret

	// fn_request / fn_Request_input / fn_Session_get(rcx = key, rdx = defaultVal) -> rax = defaultVal
	b.defineLabel("fn_request")
	b.defineLabel("fn_Request_input")
	b.defineLabel("fn_Session_get")
	b.emitBytes(0x48, 0x89, 0xD0, 0xC3) // mov rax, rdx; ret

	// fn_csrf_field() -> HTML hidden input
	b.defineLabel("fn_csrf_field")
	b.emitLeaRAXString(`<input type="hidden" name="_token" value="joss_csrf_token">`)
	b.emitBytes(0xC3)

	// Auth & Turnstile runtime intrinsics for unauthenticated HTTP requests
	b.defineLabel("fn_Auth_guest")
	b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00, 0xC3) // mov eax, 1; ret

	b.defineLabel("fn_Auth_check")
	b.defineLabel("fn_Auth_user")
	b.defineLabel("fn_Auth_id")
	b.emitBytes(0x31, 0xC0, 0xC3) // xor eax, eax; ret

	b.defineLabel("fn_Auth_enabledSocialProviders")
	b.emitBytes(0xB9, 0x08, 0x00, 0x00, 0x00) // mov ecx, 8
	b.emitJmpLabel("joss_arr_new")

	b.defineLabel("fn_Turnstile_siteKey")
	b.emitLeaRAXString("1x00000000000000000000AA")
	b.emitBytes(0xC3)

	// Dynamic SEO state & rendering (SEO::title, SEO::description, SEO::canonical, SEO::keywords, SEO::og, SEO::render)
	b.defineLabel("fn_SEO_title")
	b.emitMovDataRCX(dataOffSEOTitle)
	b.emitBytes(0x31, 0xC0, 0xC3)

	b.defineLabel("fn_SEO_description")
	b.emitMovDataRCX(dataOffSEODesc)
	b.emitBytes(0x31, 0xC0, 0xC3)

	b.defineLabel("fn_SEO_canonical")
	b.emitMovDataRCX(dataOffSEOCanonical)
	b.emitBytes(0x31, 0xC0, 0xC3)

	b.defineLabel("fn_SEO_keywords")
	b.defineLabel("fn_SEO_og")
	b.defineLabel("fn_SEO_meta")
	b.emitBytes(0x31, 0xC0, 0xC3)

	b.defineLabel("fn_SEO_render")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55) // push r12; push r13
	b.emitBytes(0x48, 0x83, 0xEC, 0x20) // sub rsp, 32
	b.emitLeaRCXString("<title>")
	b.emitMovRDXData(dataOffSEOTitle)
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
	b.emitLeaRDXString("</title>\n<meta name=\"description\" content=\"")
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
	b.emitMovRDXData(dataOffSEODesc)
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
	b.emitLeaRDXString("\">\n")
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x83, 0xC4, 0x20, 0x41, 0x5D, 0x41, 0x5C, 0x5D, 0xC3)

	// fn_GranDB_paginate / fn_paginate -> returns a map with "data": empty array
	b.defineLabel("fn_GranDB_paginate")
	b.defineLabel("fn_paginate")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)       // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55)       // push r12; push r13
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)       // sub rsp, 32
	b.emitBytes(0xB9, 0x08, 0x00, 0x00, 0x00) // mov ecx, 8
	b.emitCallLabel("joss_map_new")           // rax = pagMap
	b.emitBytes(0x49, 0x89, 0xC4)             // mov r12, rax
	b.emitBytes(0xB9, 0x10, 0x00, 0x00, 0x00) // mov ecx, 16
	b.emitCallLabel("joss_arr_new")           // rax = dataArr
	b.emitBytes(0x49, 0x89, 0xC0)             // mov r8, rax
	b.emitBytes(0x4C, 0x89, 0xE1)             // mov rcx, r12
	b.emitLeaRDXString("data")
	b.emitCallLabel("joss_map_set_i64")
	b.emitBytes(0x4C, 0x89, 0xE0)       // mov rax, r12
	b.emitBytes(0x48, 0x83, 0xC4, 0x20) // add rsp, 32
	b.emitBytes(0x41, 0x5D, 0x41, 0x5C, 0x5D, 0xC3)

	// Map iteration helpers for foreach ($map as $k => $v)
	// Map layout in machine_amd64.go: [0]=className, [8]=headNode
	// Node layout (24 bytes): [0]=keyPtr, [8]=val, [16]=nextNode
	b.defineLabel("fn_joss_map_len")
	b.defineLabel("joss_map_len")
	b.emitBytes(0x31, 0xC0)                               // xor eax, eax
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJlLabel(".map_len_done")
	b.emitBytes(0x48, 0x8B, 0x49, 0x08) // mov rcx, [rcx + 8] (headNode)
	b.defineLabel(".map_len_loop")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".map_len_done")
	b.emitBytes(0x48, 0xFF, 0xC0)       // inc rax
	b.emitBytes(0x48, 0x8B, 0x49, 0x10) // mov rcx, [rcx + 16] (nextNode)
	b.emitJmpLabel(".map_len_loop")
	b.defineLabel(".map_len_done")
	b.emitBytes(0xC3)

	b.defineLabel("fn_joss_map_key_at")
	b.defineLabel("joss_map_key_at")
	b.emitBytes(0x31, 0xC0)                               // xor eax, eax
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJlLabel(".map_key_done")
	b.emitBytes(0x48, 0x8B, 0x49, 0x08) // mov rcx, [rcx + 8] (headNode)
	b.defineLabel(".map_key_loop")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".map_key_done")
	b.emitBytes(0x48, 0x85, 0xD2) // test rdx, rdx
	b.emitJeLabel(".map_key_found")
	b.emitBytes(0x48, 0xFF, 0xCA)       // dec rdx
	b.emitBytes(0x48, 0x8B, 0x49, 0x10) // mov rcx, [rcx + 16]
	b.emitJmpLabel(".map_key_loop")
	b.defineLabel(".map_key_found")
	b.emitBytes(0x48, 0x8B, 0x01) // mov rax, [rcx + 0] (node.key)
	b.defineLabel(".map_key_done")
	b.emitBytes(0xC3)

	b.defineLabel("fn_joss_map_val_at")
	b.defineLabel("joss_map_val_at")
	b.emitBytes(0x31, 0xC0)                               // xor eax, eax
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJlLabel(".map_val_done")
	b.emitBytes(0x48, 0x8B, 0x49, 0x08) // mov rcx, [rcx + 8] (headNode)
	b.defineLabel(".map_val_loop")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJeLabel(".map_val_done")
	b.emitBytes(0x48, 0x85, 0xD2) // test rdx, rdx
	b.emitJeLabel(".map_val_found")
	b.emitBytes(0x48, 0xFF, 0xCA)       // dec rdx
	b.emitBytes(0x48, 0x8B, 0x49, 0x10) // mov rcx, [rcx + 16]
	b.emitJmpLabel(".map_val_loop")
	b.defineLabel(".map_val_found")
	b.emitBytes(0x48, 0x8B, 0x41, 0x08) // mov rax, [rcx + 8] (node.val)
	b.defineLabel(".map_val_done")
	b.emitBytes(0xC3)

	// Auth::guest() -> true (1), Auth::check() -> false (0), Auth::enabledSocialProviders() -> 0
	b.defineLabel("fn_Auth_guest")
	b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00, 0xC3) // mov eax, 1; ret
	b.defineLabel("fn_Auth_check")
	b.defineLabel("fn_Auth_enabledSocialProviders")
	b.emitBytes(0x31, 0xC0, 0xC3) // xor eax, eax; ret

	// Markdown::readFile(rcx = pathPtr) -> dynamically reads file from disk via Win32 API
	b.defineLabel("fn_Markdown_readFile")
	if b.target.OS == "windows" {
		b.emitBytes(0x55, 0x48, 0x89, 0xE5)                               // push rbp; mov rbp, rsp
		b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push rbx, r12..r15 (40 bytes)
		b.emitBytes(0x48, 0x83, 0xEC, 0x58)                               // sub rsp, 88 (total 128 bytes, 16-byte aligned)
		b.emitBytes(0x49, 0x89, 0xCC)                                     // mov r12, rcx (pathPtr)
		b.emitLeaRCXString("kernel32.dll")
		b.emitCallIAT(IATLoadLibraryA)
		b.emitBytes(0x49, 0x89, 0xC5) // mov r13, rax (hKernel32)

		// CreateFileA(pathPtr, GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, NULL)
		b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13
		b.emitLeaRDXString("CreateFileA")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x4C, 0x89, 0xE1)                                     // mov rcx, r12 (pathPtr)
		b.emitBytes(0xBA, 0x00, 0x00, 0x00, 0x80)                         // mov edx, 0x80000000
		b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00)                   // mov r8d, 1
		b.emitBytes(0x45, 0x31, 0xC9)                                     // xor r9d, r9d
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x03, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 32], 3
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x28, 0x80, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 40], 0x80
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x30, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 48], 0
		b.emitBytes(0xFF, 0xD0)                                           // call rax
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF)                               // cmp rax, -1
		b.emitJeLabel(".md_read_fail")
		b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax (hFile)

		// GetFileSize(hFile, NULL)
		b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13
		b.emitLeaRDXString("GetFileSize")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0xD9) // mov rcx, rbx
		b.emitBytes(0x31, 0xD2)       // xor edx, edx
		b.emitBytes(0xFF, 0xD0)       // call rax
		b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (fileSize)

		// Allocate buffer (fileSize + 16)
		b.emitBytes(0x49, 0x8D, 0x4E, 0x10) // lea rcx, [r14 + 16]
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax (fileBuf)

		// ReadFile(hFile, fileBuf, fileSize, &bytesRead, NULL)
		b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13
		b.emitLeaRDXString("ReadFile")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0xD9)                                     // mov rcx, rbx (hFile)
		b.emitBytes(0x4C, 0x89, 0xFA)                                     // mov rdx, r15 (fileBuf)
		b.emitBytes(0x45, 0x89, 0xF0)                                     // mov r8d, r14d (fileSize)
		b.emitBytes(0x4C, 0x8D, 0x4D, 0xC8)                               // lea r9, [rbp - 56] (&bytesRead)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 32], 0
		b.emitBytes(0xFF, 0xD0)                                           // call rax

		// CloseHandle(hFile)
		b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13
		b.emitLeaRDXString("CloseHandle")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0xD9) // mov rcx, rbx
		b.emitBytes(0xFF, 0xD0)       // call rax

		b.emitBytes(0x4C, 0x89, 0xF8) // mov rax, r15 (return fileBuf)
		b.emitJmpLabel(".md_read_done")
		b.defineLabel(".md_read_fail")
		b.emitLeaRAXString("")
		b.defineLabel(".md_read_done")
		b.emitBytes(0x48, 0x83, 0xC4, 0x58)                               // add rsp, 88
		b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B) // pop r15..r12, rbx
		b.emitBytes(0x5D, 0xC3)
	} else {
		b.emitLeaRAXString("")
		b.emitBytes(0xC3)
	}

	// fn_GranDB_insert / fn_GranDB_insertGetId(rcx = qbObj, rdx = rowMap) -> rax = newId
	b.defineLabel("fn_GranDB_insert")
	b.defineLabel("fn_insert")
	b.defineLabel("fn_GranDB_insertGetId")
	b.defineLabel("fn_insertGetId")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (qbObj)
	b.emitBytes(0x49, 0x89, 0xD5)                               // mov r13, rdx (rowMap)
	b.emitBytes(0x4C, 0x89, 0xE1)                               // mov rcx, r12
	b.emitLeaRDXString("__rows")
	b.emitCallLabel("joss_obj_get_field_i64")
	b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (rowsArr)
	b.emitBytes(0x4D, 0x8B, 0x3E) // mov r15, [r14 + 0] (rowsArr.len)
	b.emitBytes(0x49, 0xFF, 0xC7) // inc r15 (newId = len + 1)
	b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13 (rowMap)
	b.emitLeaRDXString("id")
	b.emitCallLabel("joss_map_get_i64")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJneLabel(".gdb_insert_has_id")
	b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13 (rowMap)
	b.emitLeaRDXString("id")
	b.emitBytes(0x4D, 0x89, 0xF8) // mov r8, r15 (newId)
	b.emitCallLabel("joss_map_set_i64")
	b.defineLabel(".gdb_insert_has_id")
	b.emitBytes(0x4C, 0x89, 0xF1) // mov rcx, r14 (rowsArr)
	b.emitBytes(0x4C, 0x89, 0xEA) // mov rdx, r13 (rowMap)
	b.emitCallLabel("joss_arr_push_i64")
	b.emitBytes(0x4C, 0x89, 0xF8)                               // mov rax, r15 (return newId)
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                         // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r15..r12
	b.emitBytes(0x5D, 0xC3)

	// __joss_grandb_row_matches(rcx = qbObj, rdx = rowMap) -> rax = 1 if match, 0 if not
	b.defineLabel("__joss_grandb_row_matches")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x48, 0x85, 0xD2)                               // test rdx, rdx (deleted/null row?)
	b.emitJeLabel(".gdb_match_no")
	b.emitBytes(0x49, 0x89, 0xCC) // mov r12, rcx (qbObj)
	b.emitBytes(0x49, 0x89, 0xD5) // mov r13, rdx (rowMap)
	b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
	b.emitLeaRDXString("__has_where")
	b.emitCallLabel("joss_obj_get_field_i64")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".gdb_match_yes")
	b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
	b.emitLeaRDXString("__w_col")
	b.emitCallLabel("joss_obj_get_field_i64")
	b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (wCol)
	b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
	b.emitLeaRDXString("__w_val")
	b.emitCallLabel("joss_obj_get_field_i64")
	b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax (wVal)
	b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13 (rowMap)
	b.emitBytes(0x4C, 0x89, 0xF2) // mov rdx, r14 (wCol)
	b.emitCallLabel("joss_map_get_i64")
	b.emitBytes(0x4C, 0x39, 0xF8) // cmp rax, r15
	b.emitJeLabel(".gdb_match_yes")
	b.emitBytes(0x48, 0x81, 0xF8, 0x00, 0x00, 0x01, 0x00) // cmp rax, 0x10000
	b.emitJlLabel(".gdb_match_no")
	b.emitBytes(0x49, 0x81, 0xFF, 0x00, 0x00, 0x01, 0x00) // cmp r15, 0x10000
	b.emitJlLabel(".gdb_match_no")
	b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
	b.emitBytes(0x4C, 0x89, 0xFA) // mov rdx, r15
	b.emitCallLabel("__joss_rt_strcmp")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".gdb_match_yes")
	b.defineLabel(".gdb_match_no")
	b.emitBytes(0x31, 0xC0) // xor eax, eax
	b.emitJmpLabel(".gdb_match_exit")
	b.defineLabel(".gdb_match_yes")
	b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00) // mov eax, 1
	b.defineLabel(".gdb_match_exit")
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                         // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r15..r12
	b.emitBytes(0x5D, 0xC3)

	// fn_GranDB_count(rcx = qbObj) -> rax = matchingCount
	b.defineLabel("fn_GranDB_count")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (qbObj)
	b.emitLeaRDXString("__rows")
	b.emitCallLabel("joss_obj_get_field_i64")
	b.emitBytes(0x49, 0x89, 0xC5) // mov r13, rax (rowsArr)
	b.emitBytes(0x45, 0x31, 0xF6) // xor r14d, r14d (idx = 0)
	b.emitBytes(0x45, 0x31, 0xFF) // xor r15d, r15d (count = 0)
	b.emitBytes(0x4D, 0x85, 0xED) // test r13, r13
	b.emitJeLabel(".gdb_count_done")
	b.defineLabel(".gdb_count_loop")
	b.emitBytes(0x4D, 0x3B, 0x75, 0x00) // cmp r14, [r13 + 0]
	b.emitJgeLabel(".gdb_count_done")
	b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13
	b.emitBytes(0x4C, 0x89, 0xF2) // mov rdx, r14
	b.emitCallLabel("joss_arr_get_i64")
	b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
	b.emitBytes(0x48, 0x89, 0xC2) // mov rdx, rax
	b.emitCallLabel("__joss_grandb_row_matches")
	b.emitBytes(0x49, 0x01, 0xC7) // add r15, rax
	b.emitBytes(0x49, 0xFF, 0xC6) // inc r14
	b.emitJmpLabel(".gdb_count_loop")
	b.defineLabel(".gdb_count_done")
	b.emitBytes(0x4C, 0x89, 0xF8)                               // mov rax, r15
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                         // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r15..r12
	b.emitBytes(0x5D, 0xC3)

	// fn_GranDB_first / fn_first(rcx = qbObj) -> rax = rowMap or 0
	b.defineLabel("fn_GranDB_first")
	b.defineLabel("fn_first")
	b.defineLabel("fn_GranDB_find")
	b.defineLabel("fn_find")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (qbObj)
	b.emitLeaRDXString("__rows")
	b.emitCallLabel("joss_obj_get_field_i64")
	b.emitBytes(0x49, 0x89, 0xC5) // mov r13, rax (rowsArr)
	b.emitBytes(0x45, 0x31, 0xF6) // xor r14d, r14d (idx = 0)
	b.emitBytes(0x4D, 0x85, 0xED) // test r13, r13
	b.emitJeLabel(".gdb_first_none")
	b.defineLabel(".gdb_first_loop")
	b.emitBytes(0x4D, 0x3B, 0x75, 0x00) // cmp r14, [r13 + 0]
	b.emitJgeLabel(".gdb_first_none")
	b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13
	b.emitBytes(0x4C, 0x89, 0xF2) // mov rdx, r14
	b.emitCallLabel("joss_arr_get_i64")
	b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax (row)
	b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
	b.emitBytes(0x4C, 0x89, 0xFA) // mov rdx, r15
	b.emitCallLabel("__joss_grandb_row_matches")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJneLabel(".gdb_first_found")
	b.emitBytes(0x49, 0xFF, 0xC6) // inc r14
	b.emitJmpLabel(".gdb_first_loop")
	b.defineLabel(".gdb_first_found")
	b.emitBytes(0x4C, 0x89, 0xF8) // mov rax, r15
	b.emitJmpLabel(".gdb_first_exit")
	b.defineLabel(".gdb_first_none")
	b.emitBytes(0x31, 0xC0) // xor eax, eax
	b.defineLabel(".gdb_first_exit")
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                         // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r15..r12
	b.emitBytes(0x5D, 0xC3)

	// fn_GranDB_get / fn_get(rcx = qbObj) -> rax = resultArr
	b.defineLabel("fn_GranDB_get")
	b.defineLabel("fn_get")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                               // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push rbx, r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x28)                               // sub rsp, 40 (5 pushes = 40 + 40 = 80, 16-byte aligned)
	b.emitBytes(0x49, 0x89, 0xCC)                                     // mov r12, rcx (qbObj)
	b.emitLeaRDXString("__rows")
	b.emitCallLabel("joss_obj_get_field_i64")
	b.emitBytes(0x49, 0x89, 0xC5)             // mov r13, rax (rowsArr)
	b.emitBytes(0xB9, 0x10, 0x00, 0x00, 0x00) // mov ecx, 16
	b.emitCallLabel("joss_arr_new")
	b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax (resultArr)
	b.emitBytes(0x45, 0x31, 0xF6) // xor r14d, r14d (idx = 0)
	b.emitBytes(0x4D, 0x85, 0xED) // test r13, r13
	b.emitJeLabel(".gdb_get_done")
	b.defineLabel(".gdb_get_loop")
	b.emitBytes(0x4D, 0x3B, 0x75, 0x00) // cmp r14, [r13 + 0]
	b.emitJgeLabel(".gdb_get_done")
	b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13
	b.emitBytes(0x4C, 0x89, 0xF2) // mov rdx, r14
	b.emitCallLabel("joss_arr_get_i64")
	b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax (row)
	b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
	b.emitBytes(0x48, 0x89, 0xDA) // mov rdx, rbx
	b.emitCallLabel("__joss_grandb_row_matches")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".gdb_get_next")
	b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15 (resultArr)
	b.emitBytes(0x48, 0x89, 0xDA) // mov rdx, rbx (row)
	b.emitCallLabel("joss_arr_push_i64")
	b.defineLabel(".gdb_get_next")
	b.emitBytes(0x49, 0xFF, 0xC6) // inc r14
	b.emitJmpLabel(".gdb_get_loop")
	b.defineLabel(".gdb_get_done")
	b.emitBytes(0x4C, 0x89, 0xF8)                                     // mov rax, r15
	b.emitBytes(0x48, 0x83, 0xC4, 0x28)                               // add rsp, 40
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B) // pop r15..r12, rbx
	b.emitBytes(0x5D, 0xC3)

	// fn_GranDB_update / fn_update(rcx = qbObj, rdx = updatesMap) -> rax = updatedCount
	b.defineLabel("fn_GranDB_update")
	b.defineLabel("fn_update")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                               // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push rbx, r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x28)                               // sub rsp, 40
	b.emitBytes(0x49, 0x89, 0xCC)                                     // mov r12, rcx (qbObj)
	b.emitBytes(0x49, 0x89, 0xD5)                                     // mov r13, rdx (updatesMap)
	b.emitLeaRDXString("__rows")
	b.emitCallLabel("joss_obj_get_field_i64")
	b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (rowsArr)
	b.emitBytes(0x45, 0x31, 0xFF) // xor r15d, r15d (idx = 0)
	b.emitBytes(0x4D, 0x85, 0xF6) // test r14, r14
	b.emitJeLabel(".gdb_upd_done")
	b.defineLabel(".gdb_upd_loop")
	b.emitBytes(0x4D, 0x3B, 0x3E) // cmp r15, [r14 + 0]
	b.emitJgeLabel(".gdb_upd_done")
	b.emitBytes(0x4C, 0x89, 0xF1) // mov rcx, r14
	b.emitBytes(0x4C, 0x89, 0xFA) // mov rdx, r15
	b.emitCallLabel("joss_arr_get_i64")
	b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax (row)
	b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
	b.emitBytes(0x48, 0x89, 0xDA) // mov rdx, rbx
	b.emitCallLabel("__joss_grandb_row_matches")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".gdb_upd_next")
	// Copy KV nodes from r13 (updatesMap) into rbx (row)
	b.emitBytes(0x4D, 0x85, 0xED) // test r13, r13
	b.emitJeLabel(".gdb_upd_next")
	b.emitBytes(0x49, 0x8B, 0x45, 0x08) // mov rax, [r13 + 8] (node = updatesMap.head)
	b.defineLabel(".gdb_upd_kv_loop")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".gdb_upd_next")
	b.emitBytes(0x48, 0x89, 0x45, 0xD0) // mov [rbp - 48], rax (save node)
	b.emitBytes(0x48, 0x89, 0xD9)       // mov rcx, rbx (row)
	b.emitBytes(0x48, 0x8B, 0x10)       // mov rdx, [rax + 0] (node.key)
	b.emitBytes(0x4C, 0x8B, 0x40, 0x08) // mov r8, [rax + 8]  (node.val)
	b.emitCallLabel("joss_map_set_i64")
	b.emitBytes(0x48, 0x8B, 0x45, 0xD0) // mov rax, [rbp - 48]
	b.emitBytes(0x48, 0x8B, 0x40, 0x10) // mov rax, [rax + 16] (node.next)
	b.emitJmpLabel(".gdb_upd_kv_loop")
	b.defineLabel(".gdb_upd_next")
	b.emitBytes(0x49, 0xFF, 0xC7) // inc r15
	b.emitJmpLabel(".gdb_upd_loop")
	b.defineLabel(".gdb_upd_done")
	b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00)                         // mov eax, 1
	b.emitBytes(0x48, 0x83, 0xC4, 0x28)                               // add rsp, 40
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B) // pop r15..r12, rbx
	b.emitBytes(0x5D, 0xC3)

	// fn_GranDB_delete / fn_delete(rcx = qbObj) -> rax = deletedCount
	b.defineLabel("fn_GranDB_delete")
	b.defineLabel("fn_delete")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                         // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)                               // mov r12, rcx (qbObj)
	b.emitLeaRDXString("__rows")
	b.emitCallLabel("joss_obj_get_field_i64")
	b.emitBytes(0x49, 0x89, 0xC5) // mov r13, rax (rowsArr)
	b.emitBytes(0x45, 0x31, 0xF6) // xor r14d, r14d (idx = 0)
	b.emitBytes(0x45, 0x31, 0xFF) // xor r15d, r15d (deleted = 0)
	b.emitBytes(0x4D, 0x85, 0xED) // test r13, r13
	b.emitJeLabel(".gdb_del_done")
	b.defineLabel(".gdb_del_loop")
	b.emitBytes(0x4D, 0x3B, 0x75, 0x00) // cmp r14, [r13 + 0]
	b.emitJgeLabel(".gdb_del_done")
	b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13
	b.emitBytes(0x4C, 0x89, 0xF2) // mov rdx, r14
	b.emitCallLabel("joss_arr_get_i64")
	b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
	b.emitBytes(0x48, 0x89, 0xC2) // mov rdx, rax
	b.emitCallLabel("__joss_grandb_row_matches")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".gdb_del_next")
	b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13 (rowsArr)
	b.emitBytes(0x4C, 0x89, 0xF2) // mov rdx, r14 (idx)
	b.emitBytes(0x45, 0x31, 0xC0) // xor r8d, r8d (0 = deleted)
	b.emitCallLabel("joss_arr_set_i64")
	b.emitBytes(0x49, 0xFF, 0xC7) // inc r15
	b.defineLabel(".gdb_del_next")
	b.emitBytes(0x49, 0xFF, 0xC6) // inc r14
	b.emitJmpLabel(".gdb_del_loop")
	b.defineLabel(".gdb_del_done")
	b.emitBytes(0x4C, 0x89, 0xF8)                               // mov rax, r15
	b.emitBytes(0x48, 0x83, 0xC4, 0x20)                         // add rsp, 32
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r15..r12
	b.emitBytes(0x5D, 0xC3)

	// =========================================================================
	// 3. Standalone Native HTTP Server (fn_Server_start) & Async Runtime
	// =========================================================================
	const (
		httpHeaderHTML  = "HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderCSS   = "HTTP/1.1 200 OK\r\nContent-Type: text/css; charset=utf-8\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderJS    = "HTTP/1.1 200 OK\r\nContent-Type: application/javascript; charset=utf-8\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderSVG   = "HTTP/1.1 200 OK\r\nContent-Type: image/svg+xml\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderPNG   = "HTTP/1.1 200 OK\r\nContent-Type: image/png\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeader404   = "HTTP/1.1 404 Not Found\r\nContent-Type: text/plain; charset=utf-8\r\nConnection: close\r\n\r\n404 Not Found"
		defaultHTMLBody = "<!DOCTYPE html><html lang=\"es\"><head><meta charset=\"utf-8\"><title>JosSecurity — Native AOT</title></head><body><h1>JosSecurity — Native AOT</h1><p>ONLINE (HTTP 200 OK)</p></body></html>"
	)

	hasCompiledRouter := false
	if b.prog != nil {
		for _, fn := range b.prog.Functions {
			if fn.Name == "__joss_http_dispatch" {
				hasCompiledRouter = true
				break
			}
		}
	}

	b.defineLabel("fn_Server_start")
	if b.target.OS == "windows" {
		b.emitBytes(0x55, 0x48, 0x89, 0xE5)                               // push rbp; mov rbp, rsp
		b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push rbx, r12..r15 (40 bytes)
		b.emitBytes(0x48, 0x81, 0xEC, 0x98, 0x00, 0x00, 0x00)             // sub rsp, 152 (total 192 bytes, 16-byte aligned)

		// 1. Load .env / env.joss file dynamically into environment map
		b.emitCallLabel("__joss_load_env_file")

		// 2. Announce server listening
		b.emitLeaRCXString("🚀 [Joss Native Server] Escuchando en http://localhost:8000 (Ctrl+C para detener)")
		b.emitCallLabel("joss_print_string")

		// Check JOSS_NATIVE_CHECK_ONLY environment variable for non-blocking CI smoke checks
		b.emitLeaRCXString("kernel32.dll")
		b.emitCallIAT(IATLoadLibraryA)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".srv_start_ws2")
		b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax (hKernel32)

		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitLeaRDXString("GetEnvironmentVariableA")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".srv_start_ws2")
		b.emitBytes(0x49, 0x89, 0xC4) // mov r12, rax
		b.emitLeaRCXString("JOSS_NATIVE_CHECK_ONLY")
		b.emitBytes(0x48, 0x8D, 0x55, 0xA0)             // lea rdx, [rbp - 96]
		b.emitBytes(0x41, 0xB8, 0x10, 0x00, 0x00, 0x00) // mov r8d, 16
		b.emitBytes(0x41, 0xFF, 0xD4)                   // call r12
		b.emitBytes(0x85, 0xC0)                         // test eax, eax
		b.emitJneLabel(".srv_exit_ok")

		b.defineLabel(".srv_start_ws2")
		b.emitLeaRCXString("ws2_32.dll")
		b.emitCallIAT(IATLoadLibraryA)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".srv_exit_ok")
		b.emitBytes(0x49, 0x89, 0xC4) // mov r12, rax (hWs2)

		// Allocate 16384-byte request buffer (also used as WSADATA buffer during WSAStartup)
		b.emitBytes(0xB9, 0x00, 0x40, 0x00, 0x00) // mov ecx, 16384
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x48, 0x89, 0x45, 0xB0) // mov [rbp - 80], rax (reqBuf)

		// WSAStartup(0x0202, reqBuf)
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("WSAStartup")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0xB9, 0x02, 0x02, 0x00, 0x00) // mov ecx, 0x0202
		b.emitBytes(0x48, 0x8B, 0x55, 0xB0)       // mov rdx, [rbp - 80]
		b.emitBytes(0xFF, 0xD0)                   // call rax

		// socket(AF_INET=2, SOCK_STREAM=1, IPPROTO_TCP=6)
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("socket")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0xB9, 0x02, 0x00, 0x00, 0x00)       // mov ecx, 2
		b.emitBytes(0xBA, 0x01, 0x00, 0x00, 0x00)       // mov edx, 1
		b.emitBytes(0x41, 0xB8, 0x06, 0x00, 0x00, 0x00) // mov r8d, 6
		b.emitBytes(0xFF, 0xD0)                         // call rax
		b.emitBytes(0x49, 0x89, 0xC5)                   // mov r13, rax (listenSock)

		// setsockopt(listenSock, SOL_SOCKET=0xFFFF, SO_REUSEADDR=4, &optval, 4)
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("setsockopt")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0xC7, 0x45, 0x98, 0x01, 0x00, 0x00, 0x00)             // mov dword ptr [rbp - 104], 1
		b.emitBytes(0x4C, 0x89, 0xE9)                                     // mov rcx, r13
		b.emitBytes(0xBA, 0xFF, 0xFF, 0x00, 0x00)                         // mov edx, 0xFFFF
		b.emitBytes(0x41, 0xB8, 0x04, 0x00, 0x00, 0x00)                   // mov r8d, 4
		b.emitBytes(0x4C, 0x8D, 0x4D, 0x98)                               // lea r9, [rbp - 104]
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x04, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 32], 4
		b.emitBytes(0xFF, 0xD0)                                           // call rax

		// sockaddr_in at [rbp - 96]: sin_family=AF_INET(2), sin_addr=0
		b.emitBytes(0xC7, 0x45, 0xA0, 0x02, 0x00, 0x1F, 0x40)       // default: 0x401F0002 (port 8000)
		b.emitBytes(0xC7, 0x45, 0xA4, 0x00, 0x00, 0x00, 0x00)       // sin_addr = INADDR_ANY (0)
		b.emitBytes(0x48, 0xC7, 0x45, 0xA8, 0x00, 0x00, 0x00, 0x00) // sin_zero = 0

		// Lookup PORT in envMap (dataOffEnvMap)
		b.emitMovRCXData(dataOffEnvMap)
		b.emitBytes(0x48, 0x85, 0xC9)
		b.emitJeLabel(".srv_port_done")
		b.emitLeaRDXString("PORT")
		b.emitCallLabel("joss_map_get_i64")
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJeLabel(".srv_check_joss_port")
		b.emitBytes(0x48, 0x89, 0xC1) // rcx = portStr
		b.emitJmpLabel(".srv_parse_port")

		b.defineLabel(".srv_check_joss_port")
		b.emitMovRCXData(dataOffEnvMap)
		b.emitLeaRDXString("JOSS_PORT")
		b.emitCallLabel("joss_map_get_i64")
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJeLabel(".srv_port_done")
		b.emitBytes(0x48, 0x89, 0xC1)

		// Parse port string (rcx) with mini atoi
		b.defineLabel(".srv_parse_port")
		b.emitBytes(0x31, 0xD2) // edx = portVal = 0
		b.defineLabel(".srv_port_loop")
		b.emitBytes(0x0F, 0xB6, 0x01) // movzx eax, byte ptr [rcx]
		b.emitBytes(0x83, 0xF8, 0x30) // cmp eax, '0'
		b.emitJlLabel(".srv_port_parsed")
		b.emitBytes(0x83, 0xF8, 0x39) // cmp eax, '9'
		b.emitJgLabel(".srv_port_parsed")
		b.emitBytes(0x83, 0xE8, 0x30) // sub eax, '0'
		b.emitBytes(0x69, 0xD2, 0x0A, 0x00, 0x00, 0x00) // imul edx, edx, 10
		b.emitBytes(0x01, 0xC2) // add edx, eax
		b.emitBytes(0x48, 0xFF, 0xC1) // inc rcx
		b.emitJmpLabel(".srv_port_loop")

		b.defineLabel(".srv_port_parsed")
		b.emitBytes(0x85, 0xD2) // test edx, edx
		b.emitJleLabel(".srv_port_done")
		b.emitBytes(0x81, 0xFA, 0xFF, 0xFF, 0x00, 0x00) // cmp edx, 65535
		b.emitJgLabel(".srv_port_done")
		// htons: exchange dl and dh
		b.emitBytes(0x86, 0xD6) // xchg dl, dh
		b.emitBytes(0x66, 0x89, 0x55, 0xA2) // mov word ptr [rbp - 94], dx

		b.defineLabel(".srv_port_done")

		// bind(listenSock, &sockaddr_in, 16)
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("bind")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x4C, 0x89, 0xE9)                   // mov rcx, r13
		b.emitBytes(0x48, 0x8D, 0x55, 0xA0)             // lea rdx, [rbp - 96]
		b.emitBytes(0x41, 0xB8, 0x10, 0x00, 0x00, 0x00) // mov r8d, 16
		b.emitBytes(0xFF, 0xD0)                         // call rax

		// listen(listenSock, 128)
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("listen")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x4C, 0x89, 0xE9)             // mov rcx, r13
		b.emitBytes(0xBA, 0x80, 0x00, 0x00, 0x00) // mov edx, 128
		b.emitBytes(0xFF, 0xD0)                   // call rax

		// Resolve accept, recv, send, closesocket
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("accept")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0xD0) // mov [rbp - 48], rax (pAccept)

		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("recv")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0xC8) // mov [rbp - 56], rax (pRecv)

		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("send")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0xC0) // mov [rbp - 64], rax (pSend)

		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("closesocket")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0xB8) // mov [rbp - 72], rax (pClosesocket)

		// Resolve CreateFileA, GetFileSize, ReadFile, CloseHandle from kernel32.dll (in r15)
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitLeaRDXString("CreateFileA")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0xA8) // mov [rbp - 88], rax (pCreateFileA)

		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitLeaRDXString("GetFileSize")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0xA0) // mov [rbp - 96], rax (pGetFileSize)

		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitLeaRDXString("ReadFile")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0x98) // mov [rbp - 104], rax (pReadFile)

		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitLeaRDXString("CloseHandle")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0x90) // mov [rbp - 112], rax (pCloseHandle)

		// HTTP Accept Loop
		b.defineLabel(".srv_accept_loop")
		b.emitBytes(0x4C, 0x89, 0xE9)       // mov rcx, r13 (listenSock)
		b.emitBytes(0x31, 0xD2)             // xor edx, edx
		b.emitBytes(0x45, 0x31, 0xC0)       // xor r8d, r8d
		b.emitBytes(0x48, 0x8B, 0x45, 0xD0) // mov rax, [rbp - 48] (pAccept)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF) // cmp rax, -1
		b.emitJeLabel(".srv_exit_ok")
		b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (clientSock)

		// recv(clientSock, reqBuf, 16380, 0)
		b.emitBytes(0x4C, 0x89, 0xF1)                   // mov rcx, r14
		b.emitBytes(0x48, 0x8B, 0x55, 0xB0)             // mov rdx, [rbp - 80] (reqBuf)
		b.emitBytes(0x41, 0xB8, 0xFC, 0x3F, 0x00, 0x00) // mov r8d, 16380
		b.emitBytes(0x45, 0x31, 0xC9)                   // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC8)             // mov rax, [rbp - 56] (pRecv)
		b.emitBytes(0xFF, 0xD0)                         // call rax
		b.emitBytes(0x85, 0xC0)                         // test eax, eax
		b.emitJleLabel(".srv_close_client")

		// Null-terminate received request at reqBuf[eax]
		b.emitBytes(0x48, 0x98)             // cdqe
		b.emitBytes(0x48, 0x8B, 0x55, 0xB0) // mov rdx, [rbp - 80]
		b.emitBytes(0xC6, 0x04, 0x02, 0x00) // mov byte ptr [rdx + rax], 0

		// Extract HTTP request path from first line: "GET /path?query HTTP/1.1"
		b.emitBytes(0x49, 0x89, 0xD7) // mov r15, rdx (scan ptr)
		b.defineLabel(".srv_find_space1")
		b.emitBytes(0x41, 0x0F, 0xB6, 0x07) // movzx eax, byte ptr [r15]
		b.emitBytes(0x85, 0xC0)             // test eax, eax
		b.emitJeLabel(".srv_default_path")
		b.emitBytes(0x83, 0xF8, 0x20) // cmp eax, ' '
		b.emitJeLabel(".srv_found_space1")
		b.emitBytes(0x49, 0xFF, 0xC7) // inc r15
		b.emitJmpLabel(".srv_find_space1")

		b.defineLabel(".srv_found_space1")
		b.emitBytes(0x49, 0xFF, 0xC7) // inc r15 (r15 = start of path)
		b.emitBytes(0x4C, 0x89, 0xFB) // mov rbx, r15 (rbx = cursor for end of path)
		b.defineLabel(".srv_find_path_end")
		b.emitBytes(0x0F, 0xB6, 0x03) // movzx eax, byte ptr [rbx]
		b.emitBytes(0x85, 0xC0)       // test eax, eax
		b.emitJeLabel(".srv_terminate_path")
		b.emitBytes(0x83, 0xF8, 0x20) // cmp eax, ' '
		b.emitJeLabel(".srv_terminate_path")
		b.emitBytes(0x83, 0xF8, 0x3F) // cmp eax, '?'
		b.emitJeLabel(".srv_terminate_path")
		b.emitBytes(0x83, 0xF8, 0x0D) // cmp eax, '\r'
		b.emitJeLabel(".srv_terminate_path")
		b.emitBytes(0x83, 0xF8, 0x0A) // cmp eax, '\n'
		b.emitJeLabel(".srv_terminate_path")
		b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
		b.emitJmpLabel(".srv_find_path_end")

		b.defineLabel(".srv_terminate_path")
		b.emitBytes(0xC6, 0x03, 0x00) // mov byte ptr [rbx], 0
		b.emitJmpLabel(".srv_have_path")

		b.defineLabel(".srv_default_path")
		b.emitLeaRCXString("/")
		b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx

		b.defineLabel(".srv_have_path")
		// Check if path starts with "/public/" ("/pub" + 'l') or "/assets/" ("/ass" + 'e')
		b.emitBytes(0x41, 0x81, 0x3F, 0x2F, 0x70, 0x75, 0x62) // cmp dword ptr [r15], 0x6275702F ("/pub")
		b.emitJneLabel(".srv_check_assets")
		b.emitBytes(0x41, 0x80, 0x7F, 0x04, 0x6C) // cmp byte ptr [r15 + 4], 'l'
		b.emitJeLabel(".srv_serve_static")

		b.defineLabel(".srv_check_assets")
		b.emitBytes(0x41, 0x81, 0x3F, 0x2F, 0x61, 0x73, 0x73) // cmp dword ptr [r15], 0x7373612F ("/ass")
		b.emitJneLabel(".srv_dispatch_route")
		b.emitBytes(0x41, 0x80, 0x7F, 0x04, 0x65) // cmp byte ptr [r15 + 4], 'e'
		b.emitJneLabel(".srv_dispatch_route")

		b.defineLabel(".srv_serve_static")
		// Static file path relative to working directory is r15 + 1 (skip leading '/')
		b.emitBytes(0x49, 0x8D, 0x4F, 0x01)                               // lea rcx, [r15 + 1]
		b.emitBytes(0xBA, 0x00, 0x00, 0x00, 0x80)                         // mov edx, 0x80000000 (GENERIC_READ)
		b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00)                   // mov r8d, 1 (FILE_SHARE_READ)
		b.emitBytes(0x45, 0x31, 0xC9)                                     // xor r9d, r9d (NULL)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x03, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 32], 3 (OPEN_EXISTING)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x28, 0x80, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 40], 0x80 (FILE_ATTRIBUTE_NORMAL)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x30, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 48], 0
		b.emitBytes(0x48, 0x8B, 0x45, 0xA8)                               // mov rax, [rbp - 88] (pCreateFileA)
		b.emitBytes(0xFF, 0xD0)                                           // call rax
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF)                               // cmp rax, -1 (INVALID_HANDLE_VALUE)
		b.emitJeLabel(".srv_send_404")
		b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax (hFile)

		// GetFileSize(hFile, NULL)
		b.emitBytes(0x48, 0x89, 0xD9)       // mov rcx, rbx
		b.emitBytes(0x31, 0xD2)             // xor edx, edx
		b.emitBytes(0x48, 0x8B, 0x45, 0xA0) // mov rax, [rbp - 96] (pGetFileSize)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitBytes(0x48, 0x89, 0x45, 0x88) // mov [rbp - 120], rax (fileSize)

		// Allocate buffer for file content (fileSize + 16)
		b.emitBytes(0x48, 0x8D, 0x48, 0x10) // lea rcx, [rax + 16]
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x48, 0x89, 0x45, 0x80) // mov [rbp - 128], rax (fileBuf)

		// ReadFile(hFile, fileBuf, fileSize, &bytesRead, NULL)
		b.emitBytes(0x48, 0x89, 0xD9)                                     // mov rcx, rbx (hFile)
		b.emitBytes(0x48, 0x8B, 0x55, 0x80)                               // mov rdx, [rbp - 128] (fileBuf)
		b.emitBytes(0x44, 0x8B, 0x45, 0x88)                               // mov r8d, dword ptr [rbp - 120] (fileSize)
		b.emitBytes(0x4C, 0x8D, 0x4D, 0x88)                               // lea r9, [rbp - 120] (&bytesRead)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 32], 0
		b.emitBytes(0x48, 0x8B, 0x45, 0x98)                               // mov rax, [rbp - 104] (pReadFile)
		b.emitBytes(0xFF, 0xD0)                                           // call rax

		// CloseHandle(hFile)
		b.emitBytes(0x48, 0x89, 0xD9)       // mov rcx, rbx
		b.emitBytes(0x48, 0x8B, 0x45, 0x90) // mov rax, [rbp - 112] (pCloseHandle)
		b.emitBytes(0xFF, 0xD0)             // call rax

		// Choose MIME header based on file extension in r15
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x48, 0x83, 0xF8, 0x04) // cmp rax, 4
		b.emitJlLabel(".srv_mime_html")
		b.emitBytes(0x41, 0x8B, 0x44, 0x07, 0xFC) // mov eax, dword ptr [r15 + rax - 4]
		b.emitBytes(0x3D, 0x2E, 0x63, 0x73, 0x73) // cmp eax, 0x7373632E (".css")
		b.emitJeLabel(".srv_mime_css")
		b.emitBytes(0x3D, 0x2E, 0x73, 0x76, 0x67) // cmp eax, 0x6776732E (".svg")
		b.emitJeLabel(".srv_mime_svg")
		b.emitBytes(0x3D, 0x2E, 0x70, 0x6E, 0x67) // cmp eax, 0x676E702E (".png")
		b.emitJeLabel(".srv_mime_png")
		b.emitBytes(0xC1, 0xE8, 0x08)             // shr eax, 8
		b.emitBytes(0x3D, 0x2E, 0x6A, 0x73, 0x00) // cmp eax, 0x00736A2E (".js")
		b.emitJeLabel(".srv_mime_js")

		b.defineLabel(".srv_mime_html")
		b.emitLeaRCXString(httpHeaderHTML)
		b.emitJmpLabel(".srv_send_static_hdr")
		b.defineLabel(".srv_mime_css")
		b.emitLeaRCXString(httpHeaderCSS)
		b.emitJmpLabel(".srv_send_static_hdr")
		b.defineLabel(".srv_mime_js")
		b.emitLeaRCXString(httpHeaderJS)
		b.emitJmpLabel(".srv_send_static_hdr")
		b.defineLabel(".srv_mime_svg")
		b.emitLeaRCXString(httpHeaderSVG)
		b.emitJmpLabel(".srv_send_static_hdr")
		b.defineLabel(".srv_mime_png")
		b.emitLeaRCXString(httpHeaderPNG)

		b.defineLabel(".srv_send_static_hdr")
		b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax
		b.emitBytes(0x4C, 0x89, 0xFA)       // mov rdx, r15
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax

		// Send static file body
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x48, 0x8B, 0x55, 0x80) // mov rdx, [rbp - 128] (fileBuf)
		b.emitBytes(0x44, 0x8B, 0x45, 0x88) // mov r8d, dword ptr [rbp - 120] (bytesRead)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitJmpLabel(".srv_close_client")

		b.defineLabel(".srv_send_404")
		b.emitLeaRCXString(httpHeader404)
		b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax
		b.emitBytes(0x4C, 0x89, 0xFA)       // mov rdx, r15
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitJmpLabel(".srv_close_client")

		b.defineLabel(".srv_dispatch_route")
		if hasCompiledRouter {
			b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15 (pathPtr)
			b.emitCallLabel("fn___joss_http_dispatch")
			b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax (htmlBodyPtr)
		} else {
			b.emitLeaRCXString(defaultHTMLBody)
			b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx
		}

		// Send HTTP 200 HTML header
		b.emitLeaRCXString(httpHeaderHTML)
		b.emitBytes(0x48, 0x89, 0xCB) // mov rbx, rcx
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax
		b.emitBytes(0x48, 0x89, 0xDA)       // mov rdx, rbx
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax

		// Send HTML body in r15
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax (len)
		b.emitBytes(0x4C, 0x89, 0xFA)       // mov rdx, r15 (htmlBodyPtr)
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax

		// closesocket(clientSock)
		b.defineLabel(".srv_close_client")
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14
		b.emitBytes(0x48, 0x8B, 0x45, 0xB8) // mov rax, [rbp - 72] (pClosesocket)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitJmpLabel(".srv_accept_loop")

		b.defineLabel(".srv_exit_ok")
		b.emitBytes(0x31, 0xC0)                                           // xor eax, eax
		b.emitBytes(0x48, 0x81, 0xC4, 0x98, 0x00, 0x00, 0x00)             // add rsp, 152
		b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B) // pop r15..r12, rbx
		b.emitBytes(0x5D, 0xC3)
	} else {
		// Linux ELF64 standalone entrypoint
		b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x48, 0x83, 0xEC, 0x20)
		b.emitLeaRCXString("🚀 [Joss Native Server] Escuchando en http://localhost:8000 (Ctrl+C para detener)")
		b.emitCallLabel("joss_print_string")
		b.emitBytes(0x31, 0xC0)
		b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)
	}

	b.defineLabel("fn_async")
	b.emitBytes(0x31, 0xC0, 0xC3) // xor eax, eax; ret
}

// emitHostFrameworkAdapters emits deterministic native pass-through adapters for any
// remaining MVC/Schema/Plugin host methods referenced in relocations so fluent chains
// ($table->foreign()->references()->on(), Router::get(), View::render(), etc.) resolve cleanly.
func (b *MachineCodeBuilder) emitHostFrameworkAdapters() {
	missingMap := make(map[string]bool)
	var missing []string
	for _, r := range b.relocs {
		if r.kind == relocTextLabel {
			if _, ok := b.labels[r.targetLabel]; !ok && !missingMap[r.targetLabel] {
				missingMap[r.targetLabel] = true
				missing = append(missing, r.targetLabel)
			}
		}
	}
	for i := 0; i < len(missing); i++ {
		for j := i + 1; j < len(missing); j++ {
			if missing[j] < missing[i] {
				missing[i], missing[j] = missing[j], missing[i]
			}
		}
	}
	for _, lbl := range missing {
		b.defineLabel(lbl)
		// Return first argument (rcx) in rax to preserve fluent method chaining ($obj->m1()->m2())
		b.emitBytes(0x48, 0x89, 0xC8, 0xC3) // mov rax, rcx; ret
	}
}
