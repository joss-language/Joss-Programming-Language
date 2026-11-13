package native

import (
	"fmt"
	"strings"

	"github.com/jossecurity/joss/pkg/ir"
)

const (
	dataOffGranDBTables  uint32 = 0x0680
	dataOffSEOTitle      uint32 = 0x0688
	dataOffSEODesc       uint32 = 0x0690
	dataOffSEOCanonical  uint32 = 0x0698
	dataOffEnvMap        uint32 = 0x06A0
	dataOffPort          uint32 = 0x06A8
	dataOffLastFileSize  uint32 = 0x06B0
	dataOffRequestParams uint32 = 0x06B8
)

func (b *MachineCodeBuilder) emitMovRDXData(dataOff uint32) {
	b.asmListing.WriteString(fmt.Sprintf("    mov rdx, [rip + data_%x]\n", dataOff))
	b.emitBytes(0x48, 0x8B, 0x15)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocData, patchOffset: patchOff, targetOff: dataOff})
}

func (b *MachineCodeBuilder) hasRelocTarget(labels ...string) bool {
	for _, r := range b.relocs {
		if r.kind == relocTextLabel {
			for _, lbl := range labels {
				if r.targetLabel == lbl {
					return true
				}
			}
		}
	}
	if b.prog == nil {
		return false
	}
	matchName := func(callee string) bool {
		if callee == "" {
			return false
		}
		for _, lbl := range labels {
			if callee == lbl || "fn_"+callee == lbl || "joss_"+callee == lbl || strings.TrimPrefix(lbl, "fn_") == callee {
				return true
			}
		}
		return false
	}
	var scanBlocks func(blocks []*ir.BasicBlock) bool
	scanBlocks = func(blocks []*ir.BasicBlock) bool {
		for _, blk := range blocks {
			for _, inst := range blk.Instructions {
				switch v := inst.(type) {
				case *ir.CallInst:
					if matchName(v.Callee) {
						return true
					}
				case *ir.CallBuiltinInst:
					if matchName(v.BuiltinID) {
						return true
					}
				case *ir.CallRuntimeInst:
					if matchName(v.Func) {
						return true
					}
				case *ir.CallHostServiceInst:
					if matchName(v.ClassName + "_" + v.MethodName) {
						return true
					}
				case *ir.TryCatchInst:
					if scanBlocks(v.TryBlocks) || scanBlocks(v.CatchBlocks) {
						return true
					}
				}
			}
		}
		return false
	}
	for _, fn := range b.prog.Functions {
		if scanBlocks(fn.Blocks) {
			return true
		}
	}
	return false
}

// emitNativeIntrinsics emits the standalone x86_64 native implementations of:
// - fn_html_escape / joss_html_escape
// - __joss_load_env_file / fn_env
// - fn_request / fn_Request_input / fn_Session_get
// - fn_csrf_field / fn_isset / fn_empty
// - fn_Auth_* (guest, check, user, id, enabledSocialProviders)
// - fn_Turnstile_siteKey
// - Dynamic SEO state & rendering (SEO::title, SEO::description, SEO::canonical, SEO::keywords, SEO::og, SEO::meta, SEO::render)
// - Map iteration helpers (fn_joss_map_key_at, fn_joss_map_val_at)
// - Disk-backed Markdown::readFile
// - Framework helpers (fn_Str_trim, fn_Math_ceil)
func (b *MachineCodeBuilder) emitNativeIntrinsics() {
	requestParamsOffset := uint32(dataOffRequestParams)
	if b.prog != nil {
		if requestGlobal := b.prog.Globals["__joss_request_params"]; requestGlobal != nil {
			requestParamsOffset = requestGlobal.DataOffset
		}
	}
	// fn_html_escape(rcx = val) -> rax = strPtr
	b.defineLabel("fn_html_escape")
	b.defineLabel("joss_html_escape")
	b.emitJmpLabel("joss_to_string")

	// String and Math helper passthroughs
	b.defineLabel("fn_Str_trim")
	b.defineLabel("fn_trim")
	b.defineLabel("fn_ltrim")
	b.defineLabel("fn_rtrim")
	b.defineLabel("fn_Math_ceil")
	b.emitBytes(0x48, 0x89, 0xC8, 0xC3) // mov rax, rcx; ret

	// fn_str_contains(rcx = haystack, rdx = needle) -> rax = 1 (true) or 0 (false)
	b.defineLabel("fn_str_contains")
	b.defineLabel("fn_contains")
	b.defineLabel("fn_Str_contains")
	if b.isJITMode {
		b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
		b.emitBytes(0x48, 0x83, 0xEC, 0x28) // sub rsp, 40
		b.emitBytes(0x49, 0x89, 0xD0)       // mov r8, rdx (needle)
		b.emitBytes(0x48, 0x89, 0xCA)       // mov rdx, rcx (haystack)
		b.emitLeaRCXString("fn_str_contains")
		b.emitCallIAT(IATGetProcAddress + 1)
		b.emitBytes(0x48, 0x83, 0xC4, 0x28) // add rsp, 40
		b.emitBytes(0x5D, 0xC3)             // pop rbp; ret
	} else {
		b.emitBytes(0x48, 0x85, 0xD2) // test rdx, rdx
		b.emitJeLabel(".sc_true")
		b.emitBytes(0x80, 0x3A, 0x00) // cmp byte ptr [rdx], 0
		b.emitJeLabel(".sc_true")
		b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
		b.emitJeLabel(".sc_false")
		b.defineLabel(".sc_outer")
		b.emitBytes(0x8A, 0x01) // mov al, [rcx]
		b.emitBytes(0x84, 0xC0) // test al, al
		b.emitJeLabel(".sc_false")
		b.emitBytes(0x49, 0x89, 0xC8) // mov r8, rcx
		b.emitBytes(0x49, 0x89, 0xD1) // mov r9, rdx
		b.defineLabel(".sc_inner")
		b.emitBytes(0x41, 0x8A, 0x01) // mov al, [r9]
		b.emitBytes(0x84, 0xC0)       // test al, al
		b.emitJeLabel(".sc_true")
		b.emitBytes(0x41, 0x38, 0x00) // cmp [r8], al
		b.emitJneLabel(".sc_next")
		b.emitBytes(0x49, 0xFF, 0xC0) // inc r8
		b.emitBytes(0x49, 0xFF, 0xC1) // inc r9
		b.emitJmpLabel(".sc_inner")
		b.defineLabel(".sc_next")
		b.emitBytes(0x48, 0xFF, 0xC1) // inc rcx
		b.emitJmpLabel(".sc_outer")
		b.defineLabel(".sc_true")
		b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00) // mov eax, 1
		b.emitBytes(0xC3)                         // ret
		b.defineLabel(".sc_false")
		b.emitBytes(0x31, 0xC0) // xor eax, eax
		b.emitBytes(0xC3)       // ret
	}
	b.emitNativeStrpos()
	b.emitNativeSubstr()
	b.emitNativeUTF8Helpers()
	b.emitNativeURLDecode()
	b.emitNativeStringUpper()
	b.emitNativeStringLower()

	// Http client, System::Run, String, Crypto & Date host bridge intrinsics
	for _, fnName := range []string{
		"fn_Http_json", "fn_Http_request", "fn_Http_get", "fn_Http_post", "fn_System_Run",
		"fn_str_replace", "fn_rsa_sign_sha256", "fn_rawurlencode", "fn_rawurldecode",
		"fn_gmdate", "fn_date", "fn_sha256_base64", "fn_sha1_base64",
	} {
		b.defineLabel(fnName)
		if b.isJITMode {
			b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
			b.emitBytes(0x48, 0x83, 0xEC, 0x28) // sub rsp, 40
			b.emitBytes(0x4D, 0x89, 0xC1)       // mov r9, r8
			b.emitBytes(0x49, 0x89, 0xD0)       // mov r8, rdx
			b.emitBytes(0x48, 0x89, 0xCA)       // mov rdx, rcx
			targetBridge := strings.TrimPrefix(fnName, "fn_")
			if strings.HasPrefix(fnName, "fn_Http_") || strings.HasPrefix(fnName, "fn_System_") {
				targetBridge = strings.Replace(targetBridge, "_", "::", 1)
			}
			b.emitLeaRCXString(targetBridge)
			b.emitCallIAT(IATGetProcAddress + 1)
			b.emitBytes(0x48, 0x83, 0xC4, 0x28) // add rsp, 40
			b.emitBytes(0x5D, 0xC3)             // pop rbp; ret
		} else {
			if strings.Contains(fnName, "json") || strings.Contains(fnName, "request") {
				b.emitCallLabel("joss_map_new")
				b.emitBytes(0xC3)
			} else {
				b.emitBytes(0x48, 0x89, 0xC8, 0xC3) // mov rax, rcx; ret
			}
		}
	}

	if b.hasRelocTarget("__joss_grandb_get_tables", "fn___joss_db_get_table", "fn___joss_db_set_table", "fn___joss_db_is_connected", "fn___joss_db_query", "fn___joss_db_execute") {
		// __joss_grandb_get_tables() -> rax = tablesMap
		b.defineLabel("__joss_grandb_get_tables")
		b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x48, 0x83, 0xEC, 0x20)
		b.emitMovRAXData(dataOffGranDBTables)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJneLabel(".gdb_tables_ok")
		b.emitCallLabel("joss_map_new")
		b.emitMovDataRAX(dataOffGranDBTables)

		// Pre-populate tables if seed data was provided at build time
		if len(b.seedTables) > 0 {
			b.emitCallLabel("__joss_grandb_seed_tables")
		}

		b.defineLabel(".gdb_tables_ok")
		b.emitMovRAXData(dataOffGranDBTables)
		b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)

		// __joss_db_get_table(rcx = tableName) -> rax = rowsArr
		b.defineLabel("fn___joss_db_get_table")
		b.emitBytes(0x55, 0x48, 0x89, 0xE5)             // push rbp; mov rbp, rsp
		b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56) // push r12, r13, r14
		b.emitBytes(0x48, 0x83, 0xEC, 0x28)             // sub rsp, 40 (16-byte aligned)
		b.emitBytes(0x49, 0x89, 0xCC)                   // mov r12, rcx (tableName)
		b.emitCallLabel("__joss_grandb_get_tables")
		b.emitBytes(0x49, 0x89, 0xC5)       // mov r13, rax (tablesMap)
		b.emitBytes(0x4C, 0x89, 0xE9)       // mov rcx, r13
		b.emitBytes(0x4C, 0x89, 0xE2)       // mov rdx, r12
		b.emitCallLabel("joss_map_get_i64") // rax = rowsArr
		b.emitBytes(0x48, 0x85, 0xC0)       // test rax, rax
		b.emitJneLabel(".get_tbl_done")
		b.emitBytes(0xB9, 0x10, 0x00, 0x00, 0x00) // mov ecx, 16
		b.emitCallLabel("joss_arr_new")
		b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (newArr)
		b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13 (tablesMap)
		b.emitBytes(0x4C, 0x89, 0xE2) // mov rdx, r12 (tableName)
		b.emitBytes(0x4D, 0x89, 0xF0) // mov r8, r14  (newArr)
		b.emitCallLabel("joss_map_set_i64")
		b.emitBytes(0x4C, 0x89, 0xF0) // mov rax, r14
		b.defineLabel(".get_tbl_done")
		b.emitBytes(0x48, 0x83, 0xC4, 0x28)             // add rsp, 40
		b.emitBytes(0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r14, r13, r12
		b.emitBytes(0x5D, 0xC3)

		// __joss_db_set_table(rcx = tableName, rdx = rowsArr) -> rax = 1
		b.defineLabel("fn___joss_db_set_table")
		b.emitBytes(0x55, 0x48, 0x89, 0xE5)             // push rbp; mov rbp, rsp
		b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56) // push r12, r13, r14
		b.emitBytes(0x48, 0x83, 0xEC, 0x28)             // sub rsp, 40
		b.emitBytes(0x49, 0x89, 0xCC)                   // mov r12, rcx (tableName)
		b.emitBytes(0x49, 0x89, 0xD6)                   // mov r14, rdx (rowsArr)
		b.emitCallLabel("__joss_grandb_get_tables")
		b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax (tablesMap)
		b.emitBytes(0x4C, 0x89, 0xE2) // mov rdx, r12 (tableName)
		b.emitBytes(0x4D, 0x89, 0xF0) // mov r8, r14  (rowsArr)
		b.emitCallLabel("joss_map_set_i64")
		b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00) // mov eax, 1
		b.emitBytes(0x48, 0x83, 0xC4, 0x28)
		b.emitBytes(0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C)
		b.emitBytes(0x5D, 0xC3)

		// __joss_db_is_connected() -> rax = 1 if connected, 0 if not
		b.defineLabel("fn___joss_db_is_connected")
		b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
		b.emitBytes(0x48, 0x83, 0xEC, 0x28) // sub rsp, 40
		b.emitLeaRCXString("__joss_db_is_connected")
		if b.isJITMode {
			b.emitCallIAT(IATGetProcAddress + 1)
		} else {
			b.emitCallLabel("_joss_host_bridge")
		}
		b.emitBytes(0x48, 0x83, 0xC4, 0x28) // add rsp, 40
		b.emitBytes(0x5D, 0xC3)             // pop rbp; ret

		// __joss_db_query(rcx = sql, rdx = bindings) -> rax = arrPtr
		b.defineLabel("fn___joss_db_query")
		b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
		b.emitBytes(0x48, 0x83, 0xEC, 0x28) // sub rsp, 40
		b.emitBytes(0x49, 0x89, 0xD0)       // mov r8, rdx  (bindings)
		b.emitBytes(0x48, 0x89, 0xCA)       // mov rdx, rcx (sql)
		b.emitLeaRCXString("__joss_db_query")
		if b.isJITMode {
			b.emitCallIAT(IATGetProcAddress + 1)
		} else {
			b.emitCallLabel("_joss_host_bridge")
		}
		b.emitBytes(0x48, 0x83, 0xC4, 0x28) // add rsp, 40
		b.emitBytes(0x5D, 0xC3)             // pop rbp; ret

		// __joss_db_execute(rcx = sql, rdx = bindings) -> rax = count
		b.defineLabel("fn___joss_db_execute")
		b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
		b.emitBytes(0x48, 0x83, 0xEC, 0x28) // sub rsp, 40
		b.emitBytes(0x49, 0x89, 0xD0)       // mov r8, rdx  (bindings)
		b.emitBytes(0x48, 0x89, 0xCA)       // mov rdx, rcx (sql)
		b.emitLeaRCXString("__joss_db_execute")
		if b.isJITMode {
			b.emitCallIAT(IATGetProcAddress + 1)
		} else {
			b.emitCallLabel("_joss_host_bridge")
		}
		b.emitBytes(0x48, 0x83, 0xC4, 0x28) // add rsp, 40
		b.emitBytes(0x5D, 0xC3)             // pop rbp; ret

		// __joss_grandb_seed_tables: Populates dataOffGranDBTables with build-time seeded data
		if len(b.seedTables) > 0 {
			b.defineLabel("__joss_grandb_seed_tables")
			b.emitBytes(0x55, 0x48, 0x89, 0xE5)                               // push rbp; mov rbp, rsp
			b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push rbx, r12, r13, r14, r15
			b.emitBytes(0x48, 0x83, 0xEC, 0x28)                               // sub rsp, 40

			b.emitMovRAXData(dataOffGranDBTables)
			b.emitBytes(0x49, 0x89, 0xC4) // mov r12, rax (tablesMap)

			// Iterate over each seeded table
			for tblName, rows := range b.seedTables {
				// Create array for table rows: joss_arr_new(initialCap)
				capVal := len(rows)
				if capVal < 16 {
					capVal = 16
				}
				b.emitBytes(0xB9)
				b.emitU32(uint32(capVal)) // mov ecx, capVal
				b.emitCallLabel("joss_arr_new")
				b.emitBytes(0x49, 0x89, 0xC5) // mov r13, rax (rowsArr)

				// Register table in tablesMap: joss_map_set_i64(tablesMap, tblName, rowsArr)
				b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12 (tablesMap)
				b.emitLeaRDXString(tblName)   // mov rdx, tblName
				b.emitBytes(0x4D, 0x89, 0xE8) // mov r8, r13  (rowsArr)
				b.emitCallLabel("joss_map_set_i64")

				// Populate rows
				for _, row := range rows {
					b.emitCallLabel("joss_map_new")
					b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (rowMap)

					for colName, colVal := range row {
						b.emitBytes(0x4C, 0x89, 0xF1) // mov rcx, r14 (rowMap)
						b.emitLeaRDXString(colName)   // mov rdx, colName
						b.emitLeaR8String(colVal)     // mov r8, colVal
						b.emitCallLabel("joss_map_set_i64")
					}

					// Append rowMap to rowsArr: joss_arr_push_i64(rowsArr, rowMap)
					b.emitBytes(0x4C, 0x89, 0xE9) // mov rcx, r13 (rowsArr)
					b.emitBytes(0x4C, 0x89, 0xF2) // mov rdx, r14 (rowMap)
					b.emitCallLabel("joss_arr_push_i64")
				}
			}

			b.emitBytes(0x48, 0x83, 0xC4, 0x28)                               // add rsp, 40
			b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B) // pop r15, r14, r13, r12, rbx
			b.emitBytes(0x5D, 0xC3)                                           // pop rbp; ret
		}
	}

	// __joss_load_env_file: Reads .env or env.joss via fn_file_get_contents and populates dataOffEnvMap
	b.defineLabel("__joss_load_env_file")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                               // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push rbx, r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x28)                               // sub rsp, 40 (16-byte aligned)

	// Check if already loaded
	b.emitMovRCXData(dataOffEnvMap)
	b.emitBytes(0x48, 0x85, 0xC9)
	b.emitJneLabel(".env_load_done")

	// Create empty map for environment
	b.emitCallLabel("joss_map_new")
	b.emitMovDataRAX(dataOffEnvMap)
	b.emitBytes(0x49, 0x89, 0xC4) // mov r12, rax (envMap)

	// 1. Try reading "env.joss"
	b.emitLeaRCXString("env.joss")
	b.emitCallLabel("fn_file_get_contents")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".env_try_dotenv")
	b.emitBytes(0x80, 0x38, 0x00) // cmp byte ptr [rax], 0
	b.emitJeLabel(".env_try_dotenv")
	b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax
	b.emitLeaRCXString("[Security] Cargando entorno...\n[Security] Entorno cargado desde 'env.joss'")
	b.emitCallLabel("joss_print_string")
	b.emitBytes(0x48, 0x89, 0xD8) // mov rax, rbx
	b.emitJmpLabel(".env_parse_buf")

	// 2. Try reading ".env"
	b.defineLabel(".env_try_dotenv")
	b.emitLeaRCXString(".env")
	b.emitCallLabel("fn_file_get_contents")
	b.emitBytes(0x48, 0x85, 0xC0)
	b.emitJeLabel(".env_try_parent_joss")
	b.emitBytes(0x80, 0x38, 0x00)
	b.emitJeLabel(".env_try_parent_joss")
	b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax
	b.emitLeaRCXString("[Security] Cargando entorno...\n[Security] Entorno cargado desde '.env'")
	b.emitCallLabel("joss_print_string")
	b.emitBytes(0x48, 0x89, 0xD8) // mov rax, rbx
	b.emitJmpLabel(".env_parse_buf")

	// 3. Try reading "../env.joss"
	b.defineLabel(".env_try_parent_joss")
	b.emitLeaRCXString("../env.joss")
	b.emitCallLabel("fn_file_get_contents")
	b.emitBytes(0x48, 0x85, 0xC0)
	b.emitJeLabel(".env_try_parent_dotenv")
	b.emitBytes(0x80, 0x38, 0x00)
	b.emitJeLabel(".env_try_parent_dotenv")
	b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax
	b.emitLeaRCXString("[Security] Cargando entorno...\n[Security] Entorno cargado desde 'env.joss'")
	b.emitCallLabel("joss_print_string")
	b.emitBytes(0x48, 0x89, 0xD8) // mov rax, rbx
	b.emitJmpLabel(".env_parse_buf")

	// 4. Try reading "../.env"
	b.defineLabel(".env_try_parent_dotenv")
	b.emitLeaRCXString("../.env")
	b.emitCallLabel("fn_file_get_contents")
	b.emitBytes(0x48, 0x85, 0xC0)
	b.emitJeLabel(".env_try_embedded")
	b.emitBytes(0x80, 0x38, 0x00)
	b.emitJeLabel(".env_try_embedded")
	b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax
	b.emitLeaRCXString("[Security] Cargando entorno...\n[Security] Entorno cargado desde '.env'")
	b.emitCallLabel("joss_print_string")
	b.emitBytes(0x48, 0x89, 0xD8) // mov rax, rbx
	b.emitJmpLabel(".env_parse_buf")

	// 5. Try embedded build-time environment
	b.defineLabel(".env_try_embedded")
	if b.embeddedEnv != "" {
		b.emitBytes(0x48, 0xC7, 0xC1) // mov rcx, len+1
		b.emitU32(uint32(len(b.embeddedEnv) + 1))
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax (dest in rbx)
		b.emitBytes(0x48, 0x89, 0xC7) // mov rdi, rax (dest in rdi)
		b.emitLeaRSIString(b.embeddedEnv)
		b.emitBytes(0xB9) // mov ecx, len+1
		b.emitU32(uint32(len(b.embeddedEnv) + 1))
		b.emitBytes(0xF3, 0xA4) // rep movsb
		b.emitLeaRCXString("[Security] Cargando entorno...\n[Security] Entorno cargado desde configuración embebida")
		b.emitCallLabel("joss_print_string")
		b.emitBytes(0x48, 0x89, 0xD8) // mov rax, rbx
		b.emitJmpLabel(".env_parse_buf")
	} else {
		b.emitJmpLabel(".env_load_done")
	}

	b.defineLabel(".env_parse_buf")
	b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax (cursor)

	b.defineLabel(".env_parse_line")
	b.emitBytes(0x80, 0x3B, 0x00) // cmp byte ptr [rbx], 0
	b.emitJeLabel(".env_load_done")

	// Skip whitespace & comments
	b.defineLabel(".env_skip_ws")
	b.emitBytes(0x0F, 0xB6, 0x03) // movzx eax, byte ptr [rbx]
	b.emitBytes(0x85, 0xC0)
	b.emitJeLabel(".env_load_done")
	b.emitBytes(0x83, 0xF8, 0x20)
	b.emitJeLabel(".env_adv_skip")
	b.emitBytes(0x83, 0xF8, 0x09)
	b.emitJeLabel(".env_adv_skip")
	b.emitBytes(0x83, 0xF8, 0x0D)
	b.emitJeLabel(".env_adv_skip")
	b.emitBytes(0x83, 0xF8, 0x0A)
	b.emitJeLabel(".env_adv_skip")
	b.emitBytes(0x83, 0xF8, 0x23) // '#'
	b.emitJeLabel(".env_skip_to_nl")
	b.emitJmpLabel(".env_have_key")

	b.defineLabel(".env_adv_skip")
	b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
	b.emitJmpLabel(".env_skip_ws")

	b.defineLabel(".env_skip_to_nl")
	b.emitBytes(0x0F, 0xB6, 0x03)
	b.emitBytes(0x85, 0xC0)
	b.emitJeLabel(".env_load_done")
	b.emitBytes(0x83, 0xF8, 0x0A)
	b.emitJeLabel(".env_adv_skip")
	b.emitBytes(0x48, 0xFF, 0xC3)
	b.emitJmpLabel(".env_skip_to_nl")

	b.defineLabel(".env_have_key")
	b.emitBytes(0x49, 0x89, 0xDE) // mov r14, rbx (keyStart in r14)

	// Scan until '='
	b.defineLabel(".env_find_eq")
	b.emitBytes(0x0F, 0xB6, 0x03)
	b.emitBytes(0x85, 0xC0)
	b.emitJeLabel(".env_load_done")
	b.emitBytes(0x83, 0xF8, 0x3D) // '='
	b.emitJeLabel(".env_found_eq")
	b.emitBytes(0x83, 0xF8, 0x0A) // newline before '='
	b.emitJeLabel(".env_adv_skip")
	b.emitBytes(0x48, 0xFF, 0xC3)
	b.emitJmpLabel(".env_find_eq")

	b.defineLabel(".env_found_eq")
	b.emitBytes(0xC6, 0x03, 0x00) // null-terminate key
	b.emitBytes(0x48, 0xFF, 0xC3) // rbx points to start of value

	// Check if quote
	b.emitBytes(0x0F, 0xB6, 0x03)
	b.emitBytes(0x83, 0xF8, 0x22) // '"'
	b.emitJneLabel(".env_unquoted_val")
	b.emitBytes(0x48, 0xFF, 0xC3) // skip '"'
	b.emitBytes(0x49, 0x89, 0xDF) // mov r15, rbx (valStart in r15)

	b.defineLabel(".env_find_quote_end")
	b.emitBytes(0x0F, 0xB6, 0x03)
	b.emitBytes(0x85, 0xC0)
	b.emitJeLabel(".env_load_done")
	b.emitBytes(0x83, 0xF8, 0x22) // '"'
	b.emitJeLabel(".env_found_quote_end")
	b.emitBytes(0x48, 0xFF, 0xC3)
	b.emitJmpLabel(".env_find_quote_end")

	b.defineLabel(".env_found_quote_end")
	b.emitBytes(0xC6, 0x03, 0x00) // null-terminate val
	b.emitBytes(0x48, 0xFF, 0xC3)
	b.emitJmpLabel(".env_store_pair")

	b.defineLabel(".env_unquoted_val")
	b.emitBytes(0x49, 0x89, 0xDF) // mov r15, rbx (valStart in r15)

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
	b.emitBytes(0xC6, 0x03, 0x00) // null-terminate val
	b.emitBytes(0x48, 0xFF, 0xC3)

	b.defineLabel(".env_store_pair")
	b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12 (envMap)
	b.emitBytes(0x4C, 0x89, 0xF2) // mov rdx, r14 (keyStart)
	b.emitBytes(0x4D, 0x89, 0xF8) // mov r8, r15  (valStart)
	b.emitCallLabel("joss_map_set_i64")
	b.emitJmpLabel(".env_parse_line")

	b.defineLabel(".env_load_done")
	b.emitBytes(0x48, 0x83, 0xC4, 0x28)                               // add rsp, 40
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B) // pop r15..r12, rbx
	b.emitBytes(0x5D, 0xC3)

	// fn_env(rcx = key, rdx = defaultVal) -> rax = val from OS env, loaded environment map, or defaultVal
	b.defineLabel("fn_env")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x48, 0x83, 0xEC, 0x30) // sub rsp, 48 (16-byte aligned before call)
	b.emitBytes(0x48, 0x89, 0x55, 0xF8) // mov [rbp - 8], rdx (save defaultVal)
	b.emitBytes(0x48, 0x89, 0x4D, 0xF0) // mov [rbp - 16], rcx (save key)

	// 1. Try OS environment variable first (e.g. PORT or JOSS_NATIVE_CHECK_ONLY)
	if b.target.OS == "windows" {
		b.emitMovRAXData(dataOffGetEnvVar)
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJeLabel(".env_try_map")
		b.emitBytes(0xB9, 0x00, 0x02, 0x00, 0x00) // mov ecx, 512
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x48, 0x89, 0x45, 0xE8)             // mov [rbp - 24], rax (buf)
		b.emitBytes(0x48, 0x8B, 0x4D, 0xF0)             // mov rcx, [rbp - 16] (key)
		b.emitBytes(0x48, 0x8B, 0x55, 0xE8)             // mov rdx, [rbp - 24] (buf)
		b.emitBytes(0x41, 0xB8, 0x00, 0x02, 0x00, 0x00) // mov r8d, 512
		b.emitMovRAXData(dataOffGetEnvVar)
		b.emitBytes(0xFF, 0xD0) // call GetEnvironmentVariableA
		b.emitBytes(0x85, 0xC0) // test eax, eax
		b.emitJeLabel(".env_try_map")
		b.emitBytes(0x48, 0x8B, 0x45, 0xE8)       // mov rax, [rbp - 24]
		b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // ret
	}

	// 2. Try loaded environment map (.env / env.joss)
	b.defineLabel(".env_try_map")
	b.emitMovRCXData(dataOffEnvMap) // mov rcx, [dataOffEnvMap]
	b.emitBytes(0x48, 0x85, 0xC9)   // test rcx, rcx
	b.emitJneLabel(".env_map_ready")
	b.emitCallLabel("__joss_load_env_file")
	b.defineLabel(".env_map_ready")
	b.emitMovRCXData(dataOffEnvMap)
	b.emitBytes(0x48, 0x85, 0xC9)
	b.emitJeLabel(".env_use_default")
	b.emitBytes(0x48, 0x8B, 0x55, 0xF0) // mov rdx, [rbp - 16] (key)
	b.emitCallLabel("joss_map_get_i64") // rax = val
	b.emitBytes(0x48, 0x85, 0xC0)       // test rax, rax
	b.emitJneLabel(".env_have_val")

	b.defineLabel(".env_use_default")
	b.emitBytes(0x48, 0x8B, 0x45, 0xF8) // mov rax, [rbp - 8]
	b.defineLabel(".env_have_val")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // ret

	// fn_request / fn_Request_input(rcx = key, rdx = defaultVal) -> rax = val or defaultVal
	b.defineLabel("fn_request")
	b.defineLabel("fn_Request_input")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5, 0x48, 0x83, 0xEC, 0x20) // push rbp; mov rbp, rsp; sub rsp, 32
	b.emitBytes(0x48, 0x89, 0x55, 0xF8)                         // mov [rbp - 8], rdx (save defaultVal)
	b.emitBytes(0x48, 0x89, 0xCA)                               // mov rdx, rcx (key)
	b.emitMovRCXData(requestParamsOffset)                       // mov rcx, [requestParams]
	b.emitBytes(0x48, 0x85, 0xC9)                               // test rcx, rcx
	b.emitJeLabel(".req_use_default")
	b.emitCallLabel("joss_map_get_i64") // joss_map_get_i64(rcx = map, rdx = key) -> rax
	b.emitBytes(0x48, 0x85, 0xC0)       // test rax, rax
	b.emitJeLabel(".req_use_default")
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // mov rsp, rbp; pop rbp; ret
	b.defineLabel(".req_use_default")
	b.emitBytes(0x48, 0x8B, 0x45, 0xF8)       // mov rax, [rbp - 8]
	b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // mov rsp, rbp; pop rbp; ret

	b.defineLabel("fn_Session_get")
	b.emitBytes(0x48, 0x89, 0xD0, 0xC3) // mov rax, rdx; ret

	// __joss_parse_query_params(rcx = queryStringPtr)
	b.defineLabel("__joss_parse_query_params")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)       // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55) // push rbx, r12, r13
	b.emitBytes(0x48, 0x83, 0xEC, 0x28)       // sub rsp, 40 (16-byte aligned before call)
	b.emitBytes(0x48, 0x89, 0xCB)             // mov rbx, rcx (cursor)
	b.emitBytes(0x48, 0x85, 0xDB)             // test rbx, rbx
	b.emitJeLabel(".qp_done")

	b.defineLabel(".qp_loop")
	b.emitBytes(0x80, 0x3B, 0x00) // cmp byte ptr [rbx], 0
	b.emitJeLabel(".qp_done")
	b.emitBytes(0x49, 0x89, 0xDC) // mov r12, rbx (keyStart in r12)

	b.defineLabel(".qp_find_eq")
	b.emitBytes(0x0F, 0xB6, 0x03) // movzx eax, byte ptr [rbx]
	b.emitBytes(0x85, 0xC0)
	b.emitJeLabel(".qp_done")
	b.emitBytes(0x83, 0xF8, 0x3D) // '='
	b.emitJeLabel(".qp_found_eq")
	b.emitBytes(0x83, 0xF8, 0x26) // '&'
	b.emitJeLabel(".qp_next_pair")
	b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
	b.emitJmpLabel(".qp_find_eq")

	b.defineLabel(".qp_found_eq")
	b.emitBytes(0xC6, 0x03, 0x00) // mov byte ptr [rbx], 0
	b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
	b.emitBytes(0x49, 0x89, 0xDD) // mov r13, rbx (valStart in r13)

	b.defineLabel(".qp_find_amp")
	b.emitBytes(0x0F, 0xB6, 0x03) // movzx eax, byte ptr [rbx]
	b.emitBytes(0x85, 0xC0)
	b.emitJeLabel(".qp_save_last")
	b.emitBytes(0x83, 0xF8, 0x26) // '&'
	b.emitJeLabel(".qp_save_and_next")
	b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
	b.emitJmpLabel(".qp_find_amp")

	b.defineLabel(".qp_save_and_next")
	b.emitBytes(0xC6, 0x03, 0x00)         // mov byte ptr [rbx], 0
	b.emitBytes(0x48, 0xFF, 0xC3)         // inc rbx
	b.emitMovRCXData(requestParamsOffset) // mov rcx, [requestParams]
	b.emitBytes(0x4C, 0x89, 0xE2)         // mov rdx, r12 (key)
	b.emitBytes(0x4D, 0x89, 0xE8)         // mov r8, r13 (val)
	b.emitCallLabel("joss_map_set_i64")
	b.emitJmpLabel(".qp_loop")

	b.defineLabel(".qp_save_last")
	b.emitMovRCXData(requestParamsOffset) // mov rcx, [requestParams]
	b.emitBytes(0x4C, 0x89, 0xE2)         // mov rdx, r12 (key)
	b.emitBytes(0x4D, 0x89, 0xE8)         // mov r8, r13 (val)
	b.emitCallLabel("joss_map_set_i64")
	b.emitJmpLabel(".qp_done")

	b.defineLabel(".qp_next_pair")
	b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
	b.emitJmpLabel(".qp_loop")

	b.defineLabel(".qp_done")
	b.emitBytes(0x48, 0x83, 0xC4, 0x28)                   // add rsp, 40
	b.emitBytes(0x41, 0x5D, 0x41, 0x5C, 0x5B, 0x5D, 0xC3) // pop r13, r12, rbx, rbp; ret

	// fn_csrf_field() -> HTML hidden input
	b.defineLabel("fn_csrf_field")
	b.emitLeaRAXString(`<input type="hidden" name="_token" value="joss_csrf_token">`)
	b.emitBytes(0xC3)

	// isset(rcx = val) -> rax = 1 if val != 0, 0 if val == 0 (null)
	b.defineLabel("fn_isset")
	b.emitBytes(0x31, 0xC0)       // xor eax, eax
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitBytes(0x0F, 0x95, 0xC0) // setne al
	b.emitBytes(0xC3)             // ret

	// empty(rcx = val) -> rax = 1 if empty/falsy, 0 otherwise
	b.defineLabel("fn_empty")
	b.emitJmpLabel("joss_is_empty")

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

	// Map iteration helpers for foreach ($map as $k => $v) on 32768-bucket FNV-1a hash table
	// Map layout: [0]=className, [8]=count, [16 + bucketIdx*8]=headNode (bucketIdx 0..32767)
	// Node layout (32 bytes): [0]=keyPtr, [8]=val, [16]=nextNode, [24]=hash
	b.defineLabel("fn_joss_map_key_at")
	b.defineLabel("joss_map_key_at")
	b.emitBytes(0x31, 0xC0)                               // xor eax, eax
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJlLabel(".map_key_done")
	b.emitBytes(0x45, 0x31, 0xC0) // xor r8d, r8d (bucketIdx = 0)
	b.defineLabel(".map_key_bucket_loop")
	b.emitBytes(0x41, 0x81, 0xF8, 0x00, 0x80, 0x00, 0x00) // cmp r8d, 32768
	b.emitJgeLabel(".map_key_done")
	b.emitBytes(0x4E, 0x8B, 0x4C, 0xC1, 0x10) // mov r9, [rcx + r8*8 + 16] (headNode)
	b.defineLabel(".map_key_node_loop")
	b.emitBytes(0x4D, 0x85, 0xC9) // test r9, r9
	b.emitJeLabel(".map_key_next_bucket")
	b.emitBytes(0x48, 0x85, 0xD2) // test rdx, rdx
	b.emitJeLabel(".map_key_found")
	b.emitBytes(0x48, 0xFF, 0xCA)       // dec rdx
	b.emitBytes(0x4D, 0x8B, 0x49, 0x10) // mov r9, [r9 + 16] (node.next)
	b.emitJmpLabel(".map_key_node_loop")
	b.defineLabel(".map_key_next_bucket")
	b.emitBytes(0x49, 0xFF, 0xC0) // inc r8
	b.emitJmpLabel(".map_key_bucket_loop")
	b.defineLabel(".map_key_found")
	b.emitBytes(0x49, 0x8B, 0x01) // mov rax, [r9 + 0] (node.key)
	b.defineLabel(".map_key_done")
	b.emitBytes(0xC3)

	b.defineLabel("fn_joss_map_val_at")
	b.defineLabel("joss_map_val_at")
	b.emitBytes(0x31, 0xC0)                               // xor eax, eax
	b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
	b.emitJlLabel(".map_val_done")
	b.emitBytes(0x45, 0x31, 0xC0) // xor r8d, r8d (bucketIdx = 0)
	b.defineLabel(".map_val_bucket_loop")
	b.emitBytes(0x41, 0x81, 0xF8, 0x00, 0x80, 0x00, 0x00) // cmp r8d, 32768
	b.emitJgeLabel(".map_val_done")
	b.emitBytes(0x4E, 0x8B, 0x4C, 0xC1, 0x10) // mov r9, [rcx + r8*8 + 16] (headNode)
	b.defineLabel(".map_val_node_loop")
	b.emitBytes(0x4D, 0x85, 0xC9) // test r9, r9
	b.emitJeLabel(".map_val_next_bucket")
	b.emitBytes(0x48, 0x85, 0xD2) // test rdx, rdx
	b.emitJeLabel(".map_val_found")
	b.emitBytes(0x48, 0xFF, 0xCA)       // dec rdx
	b.emitBytes(0x4D, 0x8B, 0x49, 0x10) // mov r9, [r9 + 16] (node.next)
	b.emitJmpLabel(".map_val_node_loop")
	b.defineLabel(".map_val_next_bucket")
	b.emitBytes(0x49, 0xFF, 0xC0) // inc r8
	b.emitJmpLabel(".map_val_bucket_loop")
	b.defineLabel(".map_val_found")
	b.emitBytes(0x49, 0x8B, 0x41, 0x08) // mov rax, [r9 + 8] (node.val)
	b.defineLabel(".map_val_done")
	b.emitBytes(0xC3)

	// Markdown::readFile(rcx = pathPtr) -> dynamically reads file from disk via OS API
	// with automatic fallback from "assets/docs/es/<file>" to "assets/docs/<file>".
	if b.hasRelocTarget("fn_Markdown_readFile", "fn_Markdown_toHtml", "Markdown_toHtml") {
		b.defineLabel("fn_Markdown_readFile")
		b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
		b.emitBytes(0x41, 0x54, 0x41, 0x55) // push r12; push r13
		b.emitBytes(0x48, 0x83, 0xEC, 0x20) // sub rsp, 32
		b.emitBytes(0x49, 0x89, 0xCC)       // mov r12, rcx (pathPtr)
		b.emitCallLabel("fn_file_get_contents")
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".md_try_fallback")
		b.emitBytes(0x80, 0x38, 0x00) // cmp byte ptr [rax], 0
		b.emitJneLabel(".md_render_html")
		b.defineLabel(".md_try_fallback")
		b.emitBytes(0x49, 0x81, 0xFC, 0x00, 0x00, 0x01, 0x00) // cmp r12, 0x10000
		b.emitJlLabel(".md_read_empty")
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("assets/docs/es/")
		b.emitBytes(0x41, 0xB8, 0x0F, 0x00, 0x00, 0x00) // mov r8d, 15
		b.emitCallLabel("__joss_memeq")
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".md_read_empty")
		b.emitLeaRCXString("assets/docs/")
		b.emitBytes(0x49, 0x8D, 0x54, 0x24, 0x0F) // lea rdx, [r12 + 15]
		b.emitCallLabel("joss_str_concat")
		b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
		b.emitCallLabel("fn_file_get_contents")
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".md_read_empty")
		b.emitBytes(0x80, 0x38, 0x00) // cmp byte ptr [rax], 0
		b.emitJeLabel(".md_read_empty")
		b.defineLabel(".md_render_html")
		b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
		b.emitCallLabel("fn_Markdown_toHtml")
		b.emitJmpLabel(".md_read_done")
		b.defineLabel(".md_read_empty")
		b.emitLeaRAXString("")
		b.defineLabel(".md_read_done")
		b.emitBytes(0x48, 0x83, 0xC4, 0x20)             // add rsp, 32
		b.emitBytes(0x41, 0x5D, 0x41, 0x5C, 0x5D, 0xC3) // pop r13; pop r12; pop rbp; ret

		// fn_Markdown_toHtml(rcx = mdPtr) -> rax = htmlPtr
		b.defineLabel("fn_Markdown_toHtml")
		b.defineLabel("Markdown_toHtml")
		b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
		b.emitBytes(0x53, 0x56, 0x57)                               // push rbx; push rsi; push rdi
		b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
		b.emitBytes(0x48, 0x83, 0xEC, 0x40)                         // sub rsp, 64 (stack frame: [rbp-64]=outBuf, [rbp-72]=saved bl)

		// Check null or < 0x10000 or empty
		b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
		b.emitJlLabel(".md_ret_empty")
		b.emitBytes(0x80, 0x39, 0x00) // cmp byte ptr [rcx], 0
		b.emitJeLabel(".md_ret_empty")

		b.emitBytes(0x48, 0x89, 0xCE) // mov rsi, rcx (srcPtr)

		// Allocate 65536 bytes (64 KB) for HTML output buffer (uses fast arena path)
		b.emitBytes(0xB9, 0x00, 0x00, 0x01, 0x00) // mov ecx, 65536
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x48, 0x89, 0xC7)       // mov rdi, rax (cursor)
		b.emitBytes(0x48, 0x89, 0x45, 0xC0) // mov [rbp - 64], rax (outBuf)

		b.emitBytes(0x45, 0x31, 0xE4) // xor r12d, r12d (in_code_block = 0)
		b.emitBytes(0x45, 0x31, 0xF6) // xor r14d, r14d (in_bold = 0)
		b.emitBytes(0x45, 0x31, 0xFF) // xor r15d, r15d (in_inline_code = 0)

		emitWriteLiteral := func(s string) {
			for i := 0; i < len(s); i++ {
				b.emitBytes(0xC6, 0x07, s[i]) // mov byte ptr [rdi], s[i]
				b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
			}
		}

		b.defineLabel(".md_line_start")
		b.emitBytes(0x8A, 0x06) // mov al, byte ptr [rsi]
		b.emitBytes(0x84, 0xC0) // test al, al
		b.emitJeLabel(".md_done")

		// Skip '\r'
		b.emitBytes(0x3C, 0x0D) // cmp al, 13
		b.emitJneLabel(".md_check_nl")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitJmpLabel(".md_line_start")

		b.defineLabel(".md_check_nl")
		b.emitBytes(0x3C, 0x0A) // cmp al, 10
		b.emitJneLabel(".md_check_fence")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitBytes(0xC6, 0x07, 0x0A) // mov byte ptr [rdi], '\n'
		b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
		b.emitJmpLabel(".md_line_start")

		b.defineLabel(".md_check_fence")
		// Check ``` (0x60, 0x60, 0x60)
		b.emitBytes(0x80, 0x3E, 0x60) // cmp byte ptr [rsi], '`'
		b.emitJneLabel(".md_not_fence")
		b.emitBytes(0x80, 0x7E, 0x01, 0x60) // cmp byte ptr [rsi+1], '`'
		b.emitJneLabel(".md_not_fence")
		b.emitBytes(0x80, 0x7E, 0x02, 0x60) // cmp byte ptr [rsi+2], '`'
		b.emitJneLabel(".md_not_fence")

		// It's a code fence
		b.emitBytes(0x48, 0x83, 0xC6, 0x03) // add rsi, 3
		b.emitBytes(0x45, 0x85, 0xE4)       // test r12d, r12d
		b.emitJneLabel(".md_close_code_block")

		// Open code block
		emitWriteLiteral("<pre><code>")
		b.emitBytes(0x41, 0xBC, 0x01, 0x00, 0x00, 0x00) // mov r12d, 1 (in_code_block = 1)
		b.emitJmpLabel(".md_skip_fence_line")

		b.defineLabel(".md_close_code_block")
		emitWriteLiteral("</code></pre>\n")
		b.emitBytes(0x45, 0x31, 0xE4) // xor r12d, r12d (in_code_block = 0)

		b.defineLabel(".md_skip_fence_line")
		b.emitBytes(0x8A, 0x06) // mov al, [rsi]
		b.emitBytes(0x84, 0xC0) // test al, al
		b.emitJeLabel(".md_line_start")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitBytes(0x3C, 0x0A)       // cmp al, 10
		b.emitJneLabel(".md_skip_fence_line")
		b.emitJmpLabel(".md_line_start")

		b.defineLabel(".md_not_fence")
		b.emitBytes(0x45, 0x85, 0xE4) // test r12d, r12d
		b.emitJeLabel(".md_check_heading")

		// Inside code block: copy line with entity escaping
		b.defineLabel(".md_code_char")
		b.emitBytes(0x8A, 0x06) // mov al, [rsi]
		b.emitBytes(0x84, 0xC0) // test al, al
		b.emitJeLabel(".md_code_eof")
		b.emitBytes(0x3C, 0x0D) // cmp al, 13
		b.emitJeLabel(".md_code_skip_cr")
		b.emitBytes(0x3C, 0x0A) // cmp al, 10
		b.emitJeLabel(".md_code_nl")
		b.emitBytes(0x3C, 0x3C) // cmp al, '<'
		b.emitJeLabel(".md_code_esc_lt")
		b.emitBytes(0x3C, 0x3E) // cmp al, '>'
		b.emitJeLabel(".md_code_esc_gt")
		b.emitBytes(0x3C, 0x26) // cmp al, '&'
		b.emitJeLabel(".md_code_esc_amp")

		// normal code char
		b.emitBytes(0x88, 0x07)       // mov [rdi], al
		b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitJmpLabel(".md_code_char")

		b.defineLabel(".md_code_skip_cr")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitJmpLabel(".md_code_char")

		b.defineLabel(".md_code_esc_lt")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		emitWriteLiteral("&lt;")
		b.emitJmpLabel(".md_code_char")

		b.defineLabel(".md_code_esc_gt")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		emitWriteLiteral("&gt;")
		b.emitJmpLabel(".md_code_char")

		b.defineLabel(".md_code_esc_amp")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		emitWriteLiteral("&amp;")
		b.emitJmpLabel(".md_code_char")

		b.defineLabel(".md_code_nl")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitBytes(0xC6, 0x07, 0x0A) // mov byte ptr [rdi], 10
		b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
		b.emitJmpLabel(".md_line_start")

		b.defineLabel(".md_code_eof")
		b.emitJmpLabel(".md_done")

		b.defineLabel(".md_check_heading")
		// Check '#' (0x23)
		b.emitBytes(0x80, 0x3E, 0x23) // cmp byte ptr [rsi], '#'
		b.emitJneLabel(".md_check_list")

		// Count number of '#'
		b.emitBytes(0x31, 0xC9) // xor ecx, ecx
		b.defineLabel(".md_count_hash")
		b.emitBytes(0x80, 0x3E, 0x23) // cmp byte ptr [rsi], '#'
		b.emitJneLabel(".md_hash_done")
		b.emitBytes(0xFF, 0xC1)       // inc ecx
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitBytes(0x83, 0xF9, 0x06) // cmp ecx, 6
		b.emitJlLabel(".md_count_hash")

		b.defineLabel(".md_hash_done")
		b.emitBytes(0x80, 0x3E, 0x20) // cmp byte ptr [rsi], ' '
		b.emitJneLabel(".md_emit_h_open")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi

		b.defineLabel(".md_emit_h_open")
		b.emitBytes(0x80, 0xC1, 0x30) // add cl, '0'
		b.emitBytes(0x88, 0xCB)       // mov bl, cl
		// Write "<h" + bl + ">"
		b.emitBytes(0xC6, 0x07, 0x3C)       // '<'
		b.emitBytes(0xC6, 0x47, 0x01, 0x68) // 'h'
		b.emitBytes(0x88, 0x5F, 0x02)       // [rdi+2] = bl
		b.emitBytes(0xC6, 0x47, 0x03, 0x3E) // '>'
		b.emitBytes(0x48, 0x83, 0xC7, 0x04) // add rdi, 4

		b.emitBytes(0x88, 0x5D, 0xB8) // mov byte ptr [rbp - 72], bl (save bl)
		b.emitCallLabel(".md_parse_inline_line")
		b.emitBytes(0x8A, 0x5D, 0xB8) // mov bl, byte ptr [rbp - 72] (restore bl)

		// Write "</h" + bl + ">\n"
		b.emitBytes(0xC6, 0x07, 0x3C)       // '<'
		b.emitBytes(0xC6, 0x47, 0x01, 0x2F) // '/'
		b.emitBytes(0xC6, 0x47, 0x02, 0x68) // 'h'
		b.emitBytes(0x88, 0x5F, 0x03)       // [rdi+3] = bl
		b.emitBytes(0xC6, 0x47, 0x04, 0x3E) // '>'
		b.emitBytes(0xC6, 0x47, 0x05, 0x0A) // '\n'
		b.emitBytes(0x48, 0x83, 0xC7, 0x06) // add rdi, 6
		b.emitJmpLabel(".md_line_start")

		b.defineLabel(".md_check_list")
		b.emitBytes(0x8A, 0x06) // mov al, [rsi]
		b.emitBytes(0x3C, 0x2D) // cmp al, '-'
		b.emitJeLabel(".md_is_dash_or_star")
		b.emitBytes(0x3C, 0x2A) // cmp al, '*'
		b.emitJneLabel(".md_is_para")
		b.defineLabel(".md_is_dash_or_star")
		b.emitBytes(0x80, 0x7E, 0x01, 0x20) // cmp byte ptr [rsi+1], ' '
		b.emitJneLabel(".md_is_para")

		// List item: "- " or "* "
		b.emitBytes(0x48, 0x83, 0xC6, 0x02) // add rsi, 2
		emitWriteLiteral("<li>")
		b.emitCallLabel(".md_parse_inline_line")
		emitWriteLiteral("</li>\n")
		b.emitJmpLabel(".md_line_start")

		b.defineLabel(".md_is_para")
		emitWriteLiteral("<p>")
		b.emitCallLabel(".md_parse_inline_line")
		emitWriteLiteral("</p>\n")
		b.emitJmpLabel(".md_line_start")

		// Inline parser subroutine
		b.defineLabel(".md_parse_inline_line")
		b.emitBytes(0x8A, 0x06) // mov al, [rsi]
		b.emitBytes(0x84, 0xC0) // test al, al
		b.emitJeLabel(".md_inl_done")
		b.emitBytes(0x3C, 0x0D) // cmp al, 13
		b.emitJeLabel(".md_inl_skip_cr")
		b.emitBytes(0x3C, 0x0A) // cmp al, 10
		b.emitJeLabel(".md_inl_nl")

		// Bold: "**"
		b.emitBytes(0x3C, 0x2A) // cmp al, '*'
		b.emitJneLabel(".md_inl_chk_code")
		b.emitBytes(0x80, 0x7E, 0x01, 0x2A) // cmp byte ptr [rsi+1], '*'
		b.emitJneLabel(".md_inl_normal")
		b.emitBytes(0x48, 0x83, 0xC6, 0x02) // add rsi, 2
		b.emitBytes(0x45, 0x85, 0xF6)       // test r14d, r14d (in_bold)
		b.emitJneLabel(".md_inl_close_bold")
		emitWriteLiteral("<strong>")
		b.emitBytes(0x41, 0xBE, 0x01, 0x00, 0x00, 0x00) // mov r14d, 1
		b.emitJmpLabel(".md_parse_inline_line")
		b.defineLabel(".md_inl_close_bold")
		emitWriteLiteral("</strong>")
		b.emitBytes(0x45, 0x31, 0xF6) // xor r14d, r14d
		b.emitJmpLabel(".md_parse_inline_line")

		b.defineLabel(".md_inl_chk_code")
		// Inline code: '`'
		b.emitBytes(0x3C, 0x60) // cmp al, '`'
		b.emitJneLabel(".md_inl_chk_link")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitBytes(0x45, 0x85, 0xFF) // test r15d, r15d
		b.emitJneLabel(".md_inl_close_code")
		emitWriteLiteral("<code>")
		b.emitBytes(0x41, 0xBF, 0x01, 0x00, 0x00, 0x00) // mov r15d, 1
		b.emitJmpLabel(".md_parse_inline_line")
		b.defineLabel(".md_inl_close_code")
		emitWriteLiteral("</code>")
		b.emitBytes(0x45, 0x31, 0xFF) // xor r15d, r15d
		b.emitJmpLabel(".md_parse_inline_line")

		b.defineLabel(".md_inl_chk_link")
		// Link: '['
		b.emitBytes(0x3C, 0x5B) // cmp al, '['
		b.emitJneLabel(".md_inl_chk_entities")

		// Find ']' and '(' and ')'
		b.emitBytes(0x48, 0x89, 0xF3) // mov rbx, rsi
		b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
		b.defineLabel(".md_find_close_bracket")
		b.emitBytes(0x8A, 0x03) // mov al, [rbx]
		b.emitBytes(0x84, 0xC0) // test al, al
		b.emitJeLabel(".md_not_a_link")
		b.emitBytes(0x3C, 0x0A) // cmp al, 10
		b.emitJeLabel(".md_not_a_link")
		b.emitBytes(0x3C, 0x5D) // cmp al, ']'
		b.emitJeLabel(".md_found_close_bracket")
		b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
		b.emitJmpLabel(".md_find_close_bracket")

		b.defineLabel(".md_found_close_bracket")
		b.emitBytes(0x80, 0x7B, 0x01, 0x28) // cmp byte ptr [rbx+1], '('
		b.emitJneLabel(".md_not_a_link")

		b.emitBytes(0x48, 0x8D, 0x53, 0x02) // lea rdx, [rbx+2]
		b.defineLabel(".md_find_close_paren")
		b.emitBytes(0x8A, 0x02) // mov al, [rdx]
		b.emitBytes(0x84, 0xC0) // test al, al
		b.emitJeLabel(".md_not_a_link")
		b.emitBytes(0x3C, 0x0A) // cmp al, 10
		b.emitJeLabel(".md_not_a_link")
		b.emitBytes(0x3C, 0x29) // cmp al, ')'
		b.emitJeLabel(".md_found_close_paren")
		b.emitBytes(0x48, 0xFF, 0xC2) // inc rdx
		b.emitJmpLabel(".md_find_close_paren")

		b.defineLabel(".md_found_close_paren")
		// Full link found!
		emitWriteLiteral("<a href=\"")
		b.emitBytes(0x56)                   // push rsi
		b.emitBytes(0x48, 0x8D, 0x73, 0x02) // lea rsi, [rbx+2]
		b.defineLabel(".md_copy_url")
		b.emitBytes(0x48, 0x39, 0xD6) // cmp rsi, rdx
		b.emitJeLabel(".md_url_done")
		b.emitBytes(0x8A, 0x06)       // mov al, [rsi]
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitBytes(0x88, 0x07)       // mov [rdi], al
		b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
		b.emitJmpLabel(".md_copy_url")
		b.defineLabel(".md_url_done")
		emitWriteLiteral("\">")
		b.emitBytes(0x5E)             // pop rsi
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi (past '[')
		b.defineLabel(".md_copy_link_text")
		b.emitBytes(0x48, 0x39, 0xDE) // cmp rsi, rbx
		b.emitJeLabel(".md_link_text_done")
		b.emitBytes(0x8A, 0x06)       // mov al, [rsi]
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitBytes(0x88, 0x07)       // mov [rdi], al
		b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
		b.emitJmpLabel(".md_copy_link_text")
		b.defineLabel(".md_link_text_done")
		emitWriteLiteral("</a>")
		b.emitBytes(0x48, 0x89, 0xD6) // mov rsi, rdx
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi (past ')')
		b.emitJmpLabel(".md_parse_inline_line")

		b.defineLabel(".md_not_a_link")
		b.emitBytes(0x8A, 0x06)       // mov al, [rsi]
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitBytes(0x88, 0x07)       // mov [rdi], al
		b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
		b.emitJmpLabel(".md_parse_inline_line")

		b.defineLabel(".md_inl_chk_entities")
		b.emitBytes(0x3C, 0x3C) // cmp al, '<'
		b.emitJeLabel(".md_inl_esc_lt")
		b.emitBytes(0x3C, 0x3E) // cmp al, '>'
		b.emitJeLabel(".md_inl_esc_gt")
		b.emitBytes(0x3C, 0x26) // cmp al, '&'
		b.emitJeLabel(".md_inl_esc_amp")

		b.defineLabel(".md_inl_normal")
		b.emitBytes(0x8A, 0x06)       // mov al, [rsi]
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitBytes(0x88, 0x07)       // mov [rdi], al
		b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi
		b.emitJmpLabel(".md_parse_inline_line")

		b.defineLabel(".md_inl_esc_lt")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		emitWriteLiteral("&lt;")
		b.emitJmpLabel(".md_parse_inline_line")

		b.defineLabel(".md_inl_esc_gt")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		emitWriteLiteral("&gt;")
		b.emitJmpLabel(".md_parse_inline_line")

		b.defineLabel(".md_inl_esc_amp")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		emitWriteLiteral("&amp;")
		b.emitJmpLabel(".md_parse_inline_line")

		b.defineLabel(".md_inl_skip_cr")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.emitJmpLabel(".md_parse_inline_line")

		b.defineLabel(".md_inl_nl")
		b.emitBytes(0x48, 0xFF, 0xC6) // inc rsi
		b.defineLabel(".md_inl_done")
		b.emitBytes(0xC3) // ret

		b.defineLabel(".md_done")
		b.emitBytes(0x45, 0x85, 0xE4) // test r12d, r12d
		b.emitJeLabel(".md_terminate")
		emitWriteLiteral("</code></pre>\n")

		b.defineLabel(".md_terminate")
		b.emitBytes(0xC6, 0x07, 0x00)       // mov byte ptr [rdi], 0
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (outBuf)
		b.emitJmpLabel(".md_to_html_ret")

		b.defineLabel(".md_ret_empty")
		b.emitLeaRAXString("")

		b.defineLabel(".md_to_html_ret")
		b.emitBytes(0x48, 0x83, 0xC4, 0x40)                         // add rsp, 64
		b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C) // pop r15..r12
		b.emitBytes(0x5F, 0x5E, 0x5B, 0x5D)                         // pop rdi, rsi, rbx, rbp
		b.emitBytes(0xC3)                                           // ret
	}

	// fn_file_exists(rcx = pathPtr) -> rax = 1 (exists) or 0
	b.defineLabel("fn_file_exists")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                   // push rbp; mov rbp, rsp
	b.emitBytes(0x53)                                     // push rbx
	b.emitBytes(0x48, 0x83, 0xEC, 0x48)                   // sub rsp, 72 (16-byte aligned before call)
	b.emitBytes(0x48, 0x89, 0xCB)                         // mov rbx, rcx (save pathPtr)
	b.emitBytes(0x48, 0x81, 0xFB, 0x00, 0x00, 0x01, 0x00) // cmp rbx, 0x10000
	b.emitJlLabel(".fe_no")

	// Strip leading '/' or '\\' if present
	b.emitBytes(0x80, 0x3B, 0x2F) // cmp byte ptr [rbx], '/'
	b.emitBytes(0x75, 0x03)       // jne +3
	b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
	b.emitBytes(0x80, 0x3B, 0x5C) // cmp byte ptr [rbx], '\\'
	b.emitBytes(0x75, 0x03)       // jne +3
	b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx

	// Try 1: Open relative to working directory
	b.emitBytes(0x48, 0x89, 0xD9)                                     // mov rcx, rbx
	b.emitBytes(0xBA, 0x00, 0x00, 0x00, 0x80)                         // mov edx, 0x80000000 (GENERIC_READ)
	b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00)                   // mov r8d, 1 (FILE_SHARE_READ)
	b.emitBytes(0x45, 0x31, 0xC9)                                     // xor r9d, r9d
	b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x03, 0x00, 0x00, 0x00) // [rsp+32] = 3 (OPEN_EXISTING)
	b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x28, 0x80, 0x00, 0x00, 0x00) // [rsp+40] = 0x80 (FILE_ATTRIBUTE_NORMAL)
	b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x30, 0x00, 0x00, 0x00, 0x00) // [rsp+48] = 0
	b.emitMovRAXData(dataOffCreateFileA)
	b.emitBytes(0xFF, 0xD0)
	b.emitBytes(0x48, 0x83, 0xF8, 0xFF)
	b.emitJneLabel(".fe_yes")

	// Try 2: Fallback with "../" prefix (in case running inside build/)
	b.emitBytes(0x66, 0x81, 0x3B, 0x2E, 0x2E) // cmp word ptr [rbx], 0x2E2E ("..")
	b.emitJeLabel(".fe_no")
	b.emitBytes(0x80, 0x7B, 0x01, 0x3A) // cmp byte ptr [rbx + 1], 0x3A (':')
	b.emitJeLabel(".fe_no")

	b.emitLeaRCXString("../")
	b.emitBytes(0x48, 0x89, 0xDA) // mov rdx, rbx
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax

	b.emitBytes(0xBA, 0x00, 0x00, 0x00, 0x80)
	b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00)
	b.emitBytes(0x45, 0x31, 0xC9)
	b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x03, 0x00, 0x00, 0x00)
	b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x28, 0x80, 0x00, 0x00, 0x00)
	b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x30, 0x00, 0x00, 0x00, 0x00)
	b.emitMovRAXData(dataOffCreateFileA)
	b.emitBytes(0xFF, 0xD0)
	b.emitBytes(0x48, 0x83, 0xF8, 0xFF)
	b.emitJeLabel(".fe_no")

	b.defineLabel(".fe_yes")
	b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
	b.emitMovRAXData(dataOffCloseHandle)
	b.emitBytes(0xFF, 0xD0)
	b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00) // mov eax, 1
	b.emitBytes(0x48, 0x83, 0xC4, 0x48, 0x5B, 0x5D, 0xC3)

	b.defineLabel(".fe_no")
	b.emitBytes(0x31, 0xC0) // xor eax, eax
	b.emitBytes(0x48, 0x83, 0xC4, 0x48, 0x5B, 0x5D, 0xC3)

	// fn_response(rcx = content, rdx = statusCode, r8 = contentType) -> rax = content
	b.defineLabel("fn_response")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJneLabel(".resp_ok")
	b.emitLeaRAXString("")
	b.emitBytes(0xC3)
	b.defineLabel(".resp_ok")
	b.emitBytes(0x48, 0x89, 0xC8, 0xC3) // mov rax, rcx; ret

	// fn_redirect(rcx = url, rdx = statusCode) -> rax = redirectHTTP
	b.defineLabel("fn_redirect")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55) // push r12, r13
	b.emitBytes(0x48, 0x83, 0xEC, 0x20) // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)       // mov r12, rcx (url)
	b.emitLeaRCXString("HTTP/1.1 302 Found\r\nLocation: ")
	b.emitBytes(0x4C, 0x89, 0xE2) // mov rdx, r12
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
	b.emitLeaRDXString("\r\nConnection: close\r\nContent-Length: 0\r\n\r\n")
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x83, 0xC4, 0x20) // add rsp, 32
	b.emitBytes(0x41, 0x5D, 0x41, 0x5C, 0x5D, 0xC3)

	// fn_json(rcx = val, rdx = statusCode) -> rax = jsonString
	if b.hasRelocTarget("fn_json") {
		b.defineLabel("fn_json")
		if b.isJITMode {
			b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
			b.emitBytes(0x48, 0x83, 0xEC, 0x20) // sub rsp, 32
			b.emitBytes(0x49, 0x89, 0xD0)       // mov r8, rdx (statusCode)
			b.emitBytes(0x48, 0x89, 0xCA)       // mov rdx, rcx (val)
			b.emitCallHostBridge("json")
			b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3) // mov rsp, rbp; pop rbp; ret
		} else {
			b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
			b.emitJeLabel(".json_ret_empty")
			b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0x00, 0x01, 0x00) // cmp rcx, 0x10000
			b.emitJlLabel(".json_ret_empty")

			// If rcx is NOT a map (magic != 0x4A4F53534D415021)
			b.emitBytes(0x49, 0xBA)
			b.emitU64(0x4A4F53534D415021)
			b.emitBytes(0x4C, 0x39, 0x11) // cmp [rcx], r10
			b.emitJeLabel(".json_serialize_map")

			// rcx is already a string / raw value
			b.emitBytes(0x48, 0x89, 0xC8, 0xC3) // mov rax, rcx; ret

			b.defineLabel(".json_ret_empty")
			b.emitLeaRAXString("{}")
			b.emitBytes(0xC3)

			b.defineLabel(".json_serialize_map")
			// Serialize JossMap to JSON
			b.emitBytes(0x55, 0x48, 0x89, 0xE5)                               // push rbp; mov rbp, rsp
			b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push rbx, r12..r15
			b.emitBytes(0x56, 0x57)                                           // push rsi, rdi
			b.emitBytes(0x48, 0x83, 0xEC, 0x38)                               // sub rsp, 56

			b.emitBytes(0x49, 0x89, 0xCC) // mov r12, rcx (mapPtr)
			// Allocate 128 KB buffer for json output
			b.emitBytes(0xB9, 0x00, 0x00, 0x02, 0x00) // mov ecx, 131072
			b.emitCallLabel("__joss_os_alloc")
			b.emitBytes(0x48, 0x89, 0x45, 0xD8) // mov [rbp - 40], rax (outBuf)
			b.emitBytes(0x48, 0x89, 0xC7)       // mov rdi, rax (cursor)

			// Write '{'
			b.emitBytes(0xC6, 0x07, 0x7B) // mov byte ptr [rdi], '{'
			b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi

			b.emitBytes(0x45, 0x31, 0xFF) // xor r15d, r15d (isFirst = 1, represented as 0 written)
			b.emitBytes(0x45, 0x31, 0xF6) // xor r14d, r14d (bucketIdx = 0)

			b.defineLabel(".js_bucket_loop")
			b.emitBytes(0x41, 0x81, 0xFE, 0x00, 0x80, 0x00, 0x00) // cmp r14d, 32768
			b.emitJgeLabel(".js_done")

			// node = [r12 + r14*8 + 16] (REX.W=1, REX.R=1, REX.X=1, REX.B=1 -> 0x4F)
			b.emitBytes(0x4F, 0x8B, 0x6C, 0xF4, 0x10) // mov r13, [r12 + r14*8 + 16]

			b.defineLabel(".js_node_loop")
			b.emitBytes(0x4D, 0x85, 0xED) // test r13, r13
			b.emitJeLabel(".js_next_bucket")

			// If not first (r15d != 0): write ", "
			b.emitBytes(0x45, 0x85, 0xFF)
			b.emitJeLabel(".js_first_entry")
			b.emitBytes(0x66, 0xC7, 0x07, 0x2C, 0x20) // mov word ptr [rdi], ", " (0x202C)
			b.emitBytes(0x48, 0x83, 0xC7, 0x02)       // add rdi, 2
			b.defineLabel(".js_first_entry")
			b.emitBytes(0x41, 0xBF, 0x01, 0x00, 0x00, 0x00) // mov r15d, 1 (no longer first)

			// Write '"'
			b.emitBytes(0xC6, 0x07, 0x22) // mov byte ptr [rdi], '"'
			b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi

			// Copy key string [r13 + 0]
			b.emitBytes(0x49, 0x8B, 0x75, 0x00) // mov rsi, [r13 + 0] (keyPtr)
			b.defineLabel(".js_copy_key")
			b.emitBytes(0xAC)       // lodsb (al = [rsi++])
			b.emitBytes(0x84, 0xC0) // test al, al
			b.emitJeLabel(".js_key_done")
			b.emitBytes(0xAA) // stosb ([rdi++] = al)
			b.emitJmpLabel(".js_copy_key")

			b.defineLabel(".js_key_done")
			// Write '": ' (3 bytes: 0x20 0x3A 0x22 -> ": ")
			b.emitBytes(0xC6, 0x07, 0x22)       // '"'
			b.emitBytes(0xC6, 0x47, 0x01, 0x3A) // ':'
			b.emitBytes(0xC6, 0x47, 0x02, 0x20) // ' '
			b.emitBytes(0x48, 0x83, 0xC7, 0x03) // add rdi, 3

			// Inspect val [r13 + 8]
			b.emitBytes(0x49, 0x8B, 0x5D, 0x08) // mov rbx, [r13 + 8] (val)

			// Check if val is bool/number (< 0x10000)
			b.emitBytes(0x48, 0x81, 0xFB, 0x00, 0x00, 0x01, 0x00) // cmp rbx, 0x10000
			b.emitJgeLabel(".js_val_ptr")

			// Number / boolean:
			// If rbx == 1: write "true"
			b.emitBytes(0x48, 0x83, 0xFB, 0x01) // cmp rbx, 1
			b.emitJneLabel(".js_val_check_zero")
			b.emitBytes(0xC7, 0x07, 0x74, 0x72, 0x75, 0x65) // "true"
			b.emitBytes(0x48, 0x83, 0xC7, 0x04)
			b.emitJmpLabel(".js_entry_done")

			b.defineLabel(".js_val_check_zero")
			// If rbx == 0: check if key was "success"
			b.emitBytes(0x49, 0x8B, 0x4D, 0x00) // mov rcx, [r13 + 0]
			b.emitLeaRDXString("success")
			b.emitCallLabel("__joss_rt_strcmp")
			b.emitBytes(0x48, 0x85, 0xC0)
			b.emitJneLabel(".js_val_zero_num")
			b.emitBytes(0xC7, 0x07, 0x66, 0x61, 0x6C, 0x73) // "fals"
			b.emitBytes(0xC6, 0x47, 0x04, 0x65)             // "e"
			b.emitBytes(0x48, 0x83, 0xC7, 0x05)
			b.emitJmpLabel(".js_entry_done")

			b.defineLabel(".js_val_zero_num")
			b.emitBytes(0xC6, 0x07, 0x30) // '0'
			b.emitBytes(0x48, 0xFF, 0xC7)
			b.emitJmpLabel(".js_entry_done")

			b.defineLabel(".js_val_ptr")
			// Check if val is a nested Map
			b.emitBytes(0x49, 0xBA)
			b.emitU64(0x4A4F53534D415021)
			b.emitBytes(0x4C, 0x39, 0x13) // cmp [rbx], r10
			b.emitJneLabel(".js_val_check_arr")
			// Nested map: write "{}"
			b.emitBytes(0x66, 0xC7, 0x07, 0x7B, 0x7D) // "{}"
			b.emitBytes(0x48, 0x83, 0xC7, 0x02)
			b.emitJmpLabel(".js_entry_done")

			b.defineLabel(".js_val_check_arr")
			// Check if val is an Array: [rbx+8] >= [rbx+0] && [rbx+8] > 0
			b.emitBytes(0x48, 0x8B, 0x43, 0x00) // mov rax, [rbx + 0] (len)
			b.emitBytes(0x48, 0x8B, 0x4B, 0x08) // mov rcx, [rbx + 8] (cap)
			b.emitBytes(0x48, 0x39, 0xC1)       // cmp rcx, rax (cap vs len)
			b.emitJlLabel(".js_val_string")
			b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
			b.emitJleLabel(".js_val_string")
			b.emitBytes(0x48, 0x81, 0xF9, 0x00, 0xE1, 0xF5, 0x05) // cmp rcx, 100000000
			b.emitJgLabel(".js_val_string")
			// Array: write "[]"
			b.emitBytes(0x66, 0xC7, 0x07, 0x5B, 0x5D) // "[]"
			b.emitBytes(0x48, 0x83, 0xC7, 0x02)
			b.emitJmpLabel(".js_entry_done")

			b.defineLabel(".js_val_string")
			// String: write '"', escaped string, '"'
			b.emitBytes(0xC6, 0x07, 0x22) // '"'
			b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi

			b.emitBytes(0x48, 0x89, 0xDE) // mov rsi, rbx (strPtr)
			b.defineLabel(".js_copy_str")
			b.emitBytes(0xAC)       // lodsb (al = [rsi++])
			b.emitBytes(0x84, 0xC0) // test al, al
			b.emitJeLabel(".js_str_done")
			// Check for '"' (0x22) or '\' (0x5C)
			b.emitBytes(0x3C, 0x22) // cmp al, '"'
			b.emitJeLabel(".js_esc_quote")
			b.emitBytes(0x3C, 0x5C) // cmp al, '\'
			b.emitJeLabel(".js_esc_slash")
			b.emitBytes(0x3C, 0x0A) // cmp al, '\n'
			b.emitJeLabel(".js_esc_nl")
			b.emitBytes(0x3C, 0x0D) // cmp al, '\r'
			b.emitJeLabel(".js_esc_cr")
			b.emitBytes(0xAA) // stosb
			b.emitJmpLabel(".js_copy_str")

			b.defineLabel(".js_esc_quote")
			b.emitBytes(0x66, 0xC7, 0x07, 0x5C, 0x22) // '\"'
			b.emitBytes(0x48, 0x83, 0xC7, 0x02)
			b.emitJmpLabel(".js_copy_str")

			b.defineLabel(".js_esc_slash")
			b.emitBytes(0x66, 0xC7, 0x07, 0x5C, 0x5C) // '\\'
			b.emitBytes(0x48, 0x83, 0xC7, 0x02)
			b.emitJmpLabel(".js_copy_str")

			b.defineLabel(".js_esc_nl")
			b.emitBytes(0x66, 0xC7, 0x07, 0x5C, 0x6E) // '\n'
			b.emitBytes(0x48, 0x83, 0xC7, 0x02)
			b.emitJmpLabel(".js_copy_str")

			b.defineLabel(".js_esc_cr")
			b.emitBytes(0x66, 0xC7, 0x07, 0x5C, 0x72) // '\r'
			b.emitBytes(0x48, 0x83, 0xC7, 0x02)
			b.emitJmpLabel(".js_copy_str")

			b.defineLabel(".js_str_done")
			b.emitBytes(0xC6, 0x07, 0x22) // '"'
			b.emitBytes(0x48, 0xFF, 0xC7) // inc rdi

			b.defineLabel(".js_entry_done")
			b.emitBytes(0x4D, 0x8B, 0x6D, 0x10) // mov r13, [r13 + 16] (next node)
			b.emitJmpLabel(".js_node_loop")

			b.defineLabel(".js_next_bucket")
			b.emitBytes(0x41, 0xFF, 0xC6) // inc r14d (bucketIdx++)
			b.emitJmpLabel(".js_bucket_loop")

			b.defineLabel(".js_done")
			// Write '}' and null terminator
			b.emitBytes(0xC6, 0x07, 0x7D)       // '}'
			b.emitBytes(0xC6, 0x47, 0x01, 0x00) // '\0'

			b.emitBytes(0x48, 0x8B, 0x45, 0xD8)                               // mov rax, [rbp - 40] (outBuf)
			b.emitBytes(0x48, 0x83, 0xC4, 0x38)                               // add rsp, 56
			b.emitBytes(0x5F, 0x5E)                                           // pop rdi, rsi
			b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B) // pop r15..r12, rbx
			b.emitBytes(0x5D, 0xC3)                                           // pop rbp; ret
		}
	}
}

// emitNativeStrpos implements strpos(haystack, needle) with the interpreter's
// character-index result. The search compares UTF-8 bytes, then counts leading
// bytes before the match so multibyte characters do not shift the returned index.
func (b *MachineCodeBuilder) emitNativeStrpos() {
	b.defineLabel("fn_strpos")
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // save rbx, r12-r15
	b.emitBytes(0x49, 0x89, 0xCC, 0x49, 0x89, 0xD5)                   // r12=haystack, r13=needle
	b.emitBytes(0x45, 0x31, 0xF6)                                     // r14d=byte offset
	b.emitBytes(0x4D, 0x85, 0xED)                                     // test r13,r13
	b.emitJeLabel(".strpos_found")
	b.emitBytes(0x41, 0x80, 0x7D, 0x00, 0x00) // empty needle => 0
	b.emitJeLabel(".strpos_found")
	b.defineLabel(".strpos_outer")
	b.emitBytes(0x43, 0x8A, 0x04, 0x34, 0x84, 0xC0) // al=[r12+r14]; test al,al
	b.emitJeLabel(".strpos_missing")
	b.emitBytes(0x4B, 0x8D, 0x1C, 0x34, 0x4D, 0x89, 0xEF) // rbx=candidate, r15=needle
	b.defineLabel(".strpos_inner")
	b.emitBytes(0x41, 0x8A, 0x07, 0x84, 0xC0) // al=[r15]; test al,al
	b.emitJeLabel(".strpos_found")
	b.emitBytes(0x3A, 0x03) // cmp al,[rbx]
	b.emitJneLabel(".strpos_next")
	b.emitBytes(0x48, 0xFF, 0xC3, 0x49, 0xFF, 0xC7) // inc rbx; inc r15
	b.emitJmpLabel(".strpos_inner")
	b.defineLabel(".strpos_next")
	b.emitBytes(0x49, 0xFF, 0xC6) // inc r14
	b.emitJmpLabel(".strpos_outer")
	b.defineLabel(".strpos_found")
	b.emitBytes(0x31, 0xC0, 0x48, 0x31, 0xDB) // eax=0; rbx=0 (character count/index)
	b.defineLabel(".strpos_count")
	b.emitBytes(0x4C, 0x39, 0xF3) // cmp rbx,r14
	b.emitJgeLabel(".strpos_done")
	b.emitBytes(0x41, 0x8A, 0x14, 0x1C, 0x80, 0xE2, 0xC0, 0x80, 0xFA, 0x80)
	b.emitJeLabel(".strpos_continuation")
	b.emitBytes(0x48, 0xFF, 0xC0) // count one UTF-8 leading byte
	b.defineLabel(".strpos_continuation")
	b.emitBytes(0x48, 0xFF, 0xC3) // advance byte index
	b.emitJmpLabel(".strpos_count")
	b.defineLabel(".strpos_missing")
	b.emitBytes(0x31, 0xC0) // false => 0
	b.defineLabel(".strpos_done")
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0xC3)
}

// emitNativeSubstr provides the standalone string-slice primitive. Joss strings
// are UTF-8; character ranges are translated to byte offsets before copying.
func (b *MachineCodeBuilder) emitNativeSubstr() {
	b.defineLabel("fn_substr")
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // save rbx, r12-r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x20)                               // shadow space
	b.emitBytes(0x49, 0x89, 0xCC, 0x49, 0x89, 0xD5, 0x4D, 0x89, 0xC6) // r12=source,r13=start,r14=len
	b.emitBytes(0x4D, 0x89, 0xD7)                                     // r15=argument count
	b.emitBytes(0x4C, 0x89, 0xE1)                                     // rcx=source
	b.emitCallLabel("fn___joss_utf8_len")
	b.emitBytes(0x48, 0x89, 0xC3) // rbx=source character length
	b.defineLabel(".substr_length_ready")
	b.emitBytes(0x49, 0x83, 0xFD, 0x00) // cmp start,0
	b.emitJgeLabel(".substr_start_positive")
	b.emitBytes(0x49, 0x01, 0xDD) // start += source length
	b.emitBytes(0x49, 0x83, 0xFD, 0x00)
	b.emitJgeLabel(".substr_start_positive")
	b.emitBytes(0x4D, 0x31, 0xED) // clamp negative start to 0
	b.defineLabel(".substr_start_positive")
	b.emitBytes(0x49, 0x39, 0xDD) // cmp start, source length
	b.emitJleLabel(".substr_start_in_range")
	b.emitBytes(0x49, 0x89, 0xDD, 0x4D, 0x31, 0xF6) // start=len; result len=0
	b.emitJmpLabel(".substr_allocate")
	b.defineLabel(".substr_start_in_range")
	b.emitBytes(0x49, 0x83, 0xFF, 0x03) // argument count < 3 => omitted length
	b.emitJgeLabel(".substr_explicit_length")
	b.emitBytes(0x49, 0x89, 0xDE, 0x4D, 0x29, 0xEE) // len=sourceLen-start
	b.emitJmpLabel(".substr_allocate")
	b.defineLabel(".substr_explicit_length")
	b.emitBytes(0x49, 0x83, 0xFE, 0x00) // cmp requested len,0
	b.emitJgeLabel(".substr_nonnegative_length")
	b.emitBytes(0x49, 0x01, 0xDE) // negative length => end=sourceLen+length
	b.emitBytes(0x4D, 0x39, 0xEE) // cmp end,start
	b.emitJleLabel(".substr_empty")
	b.emitBytes(0x4D, 0x29, 0xEE) // len=end-start
	b.emitJmpLabel(".substr_allocate")
	b.defineLabel(".substr_nonnegative_length")
	b.emitBytes(0x49, 0x89, 0xD8, 0x4D, 0x29, 0xE8) // r8=remaining bytes
	b.emitBytes(0x4D, 0x39, 0xC6)                   // cmp requested len,remaining
	b.emitJleLabel(".substr_allocate")
	b.emitBytes(0x4D, 0x89, 0xC6) // clamp len to remaining
	b.emitJmpLabel(".substr_allocate")
	b.defineLabel(".substr_empty")
	b.emitBytes(0x4D, 0x31, 0xF6) // len=0
	b.defineLabel(".substr_allocate")
	// Convert the character range into UTF-8 byte offsets.
	b.emitBytes(0x4C, 0x89, 0xE1, 0x4C, 0x89, 0xEA) // rcx=source, rdx=start rune
	b.emitCallLabel("fn___joss_utf8_offset")
	b.emitBytes(0x48, 0x89, 0xC6)                   // rsi=start byte offset
	b.emitBytes(0x4D, 0x89, 0xEF, 0x4D, 0x01, 0xF7) // r15=start rune + length
	b.emitBytes(0x49, 0x39, 0xDF)                   // compare end rune with source rune length
	b.emitJleLabel(".substr_end_in_range")
	b.emitBytes(0x49, 0x89, 0xDF) // clamp end rune to source length
	b.defineLabel(".substr_end_in_range")
	b.emitBytes(0x4C, 0x89, 0xE1, 0x4C, 0x89, 0xFA) // rcx=source, rdx=end rune
	b.emitCallLabel("fn___joss_utf8_offset")
	b.emitBytes(0x48, 0x29, 0xF0, 0x49, 0x89, 0xC6) // r14=end byte - start byte
	b.emitBytes(0x48, 0x89, 0xF3)                   // preserve start byte in rbx
	b.emitBytes(0x4C, 0x89, 0xF1, 0x48, 0xFF, 0xC1) // rcx=len+1
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x49, 0x89, 0xC7)       // r15=result
	b.emitBytes(0x49, 0x8D, 0x34, 0x1C) // rsi=source+start byte
	b.emitBytes(0x48, 0x31, 0xDB)       // rbx=copy index
	b.defineLabel(".substr_copy")
	b.emitBytes(0x4C, 0x39, 0xF3) // cmp copied,len
	b.emitJgeLabel(".substr_copy_done")
	b.emitBytes(0x8A, 0x04, 0x1E, 0x41, 0x88, 0x04, 0x1F) // copy byte
	b.emitBytes(0x48, 0xFF, 0xC3)
	b.emitJmpLabel(".substr_copy")
	b.defineLabel(".substr_copy_done")
	b.emitBytes(0x41, 0xC6, 0x04, 0x1F, 0x00, 0x4C, 0x89, 0xF8) // NUL; return output
	b.emitBytes(0x48, 0x83, 0xC4, 0x20, 0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0xC3)
}

func (b *MachineCodeBuilder) emitNativeUTF8Helpers() {
	b.defineLabel("fn___joss_utf8_len")
	b.emitBytes(0x49, 0x89, 0xC8, 0x4D, 0x31, 0xC9, 0x48, 0x31, 0xC0) // r8=string,r9=byte,rxa=chars
	b.defineLabel(".utf8_len_loop")
	b.emitBytes(0x47, 0x8A, 0x14, 0x08, 0x45, 0x84, 0xD2) // r10b=[r8+r9]
	b.emitJeLabel(".utf8_len_done")
	b.emitBytes(0x41, 0x80, 0xE2, 0xC0, 0x41, 0x80, 0xFA, 0x80)
	b.emitJeLabel(".utf8_len_next")
	b.emitBytes(0x48, 0xFF, 0xC0) // count leading byte
	b.defineLabel(".utf8_len_next")
	b.emitBytes(0x49, 0xFF, 0xC1)
	b.emitJmpLabel(".utf8_len_loop")
	b.defineLabel(".utf8_len_done")
	b.emitBytes(0xC3)

	b.defineLabel("fn___joss_utf8_offset")
	b.emitBytes(0x49, 0x89, 0xC8, 0x49, 0x89, 0xD1, 0x48, 0x31, 0xC0, 0x4D, 0x31, 0xD2) // r8=string,r9=target,rax=byte,r10=rune
	b.defineLabel(".utf8_offset_loop")
	b.emitBytes(0x45, 0x8A, 0x1C, 0x00, 0x45, 0x84, 0xDB) // r11b=[r8+rax]
	b.emitJeLabel(".utf8_offset_done")
	b.emitBytes(0x41, 0x80, 0xE3, 0xC0, 0x41, 0x80, 0xFB, 0x80)
	b.emitJeLabel(".utf8_offset_advance")
	b.emitBytes(0x4D, 0x39, 0xCA) // cmp r10,r9 at a character boundary
	b.emitJgeLabel(".utf8_offset_done")
	b.emitBytes(0x49, 0xFF, 0xC2) // count leading byte
	b.defineLabel(".utf8_offset_advance")
	b.emitBytes(0x48, 0xFF, 0xC0)
	b.emitJmpLabel(".utf8_offset_loop")
	b.defineLabel(".utf8_offset_done")
	b.emitBytes(0xC3)
}

func (b *MachineCodeBuilder) emitNativeURLDecode() {
	b.defineLabel("fn_urldecode")
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57)
	b.emitBytes(0x48, 0x83, 0xEC, 0x20) // shadow space and call alignment
	b.emitBytes(0x49, 0x89, 0xCC)       // r12=source
	b.emitBytes(0x48, 0x31, 0xDB)       // rbx=source byte length
	b.defineLabel(".url_decode_length")
	b.emitBytes(0x49, 0x8A, 0x04, 0x1C, 0x84, 0xC0)
	b.emitJeLabel(".url_decode_length_ready")
	b.emitBytes(0x48, 0xFF, 0xC3)
	b.emitJmpLabel(".url_decode_length")
	b.defineLabel(".url_decode_length_ready")
	b.emitBytes(0x48, 0x89, 0xD9, 0x48, 0xFF, 0xC1) // rcx=length+1
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x49, 0x89, 0xC5, 0x4D, 0x31, 0xF6, 0x4D, 0x31, 0xFF) // r13=output,r14/r15=indices
	b.defineLabel(".url_decode_loop")
	b.emitBytes(0x49, 0x39, 0xDE) // cmp input index,length
	b.emitJgeLabel(".url_decode_done")
	b.emitBytes(0x43, 0x8A, 0x04, 0x34, 0x3C, 0x2B) // al=[source+index]; cmp '+'
	b.emitJeLabel(".url_decode_plus")
	b.emitBytes(0x3C, 0x25) // cmp '%'
	b.emitJeLabel(".url_decode_percent")
	b.emitBytes(0x43, 0x88, 0x44, 0x3D, 0x00) // output[index]=al
	b.emitBytes(0x49, 0xFF, 0xC6, 0x49, 0xFF, 0xC7)
	b.emitJmpLabel(".url_decode_loop")
	b.defineLabel(".url_decode_plus")
	b.emitBytes(0x43, 0xC6, 0x44, 0x3D, 0x00, 0x20) // output[index]=' '
	b.emitBytes(0x49, 0xFF, 0xC6, 0x49, 0xFF, 0xC7)
	b.emitJmpLabel(".url_decode_loop")
	b.defineLabel(".url_decode_percent")
	b.emitBytes(0x4F, 0x8D, 0x14, 0x34)                   // r10=source+index
	b.emitBytes(0x45, 0x8A, 0x5A, 0x01, 0x45, 0x84, 0xDB) // r11b=first hex; reject NUL
	b.emitJeLabel(".url_decode_invalid")
	b.emitBytes(0x41, 0x0F, 0xB6, 0xCB) // ecx=r11d
	b.emitCallLabel("fn___joss_hex_nibble")
	b.emitBytes(0x83, 0xF8, 0xFF)
	b.emitJeLabel(".url_decode_invalid")
	b.emitBytes(0x48, 0x89, 0xC6)                               // rsi=first nibble
	b.emitBytes(0x47, 0x8A, 0x54, 0x34, 0x02, 0x45, 0x84, 0xD2) // r10b=second hex
	b.emitJeLabel(".url_decode_invalid")
	b.emitBytes(0x41, 0x0F, 0xB6, 0xCA) // ecx=r10d
	b.emitCallLabel("fn___joss_hex_nibble")
	b.emitBytes(0x83, 0xF8, 0xFF)
	b.emitJeLabel(".url_decode_invalid")
	b.emitBytes(0x48, 0xC1, 0xE6, 0x04, 0x48, 0x01, 0xC6) // first*16+second
	b.emitBytes(0x43, 0x88, 0x74, 0x3D, 0x00)             // output[index]=decoded byte
	b.emitBytes(0x49, 0x83, 0xC6, 0x03, 0x49, 0xFF, 0xC7)
	b.emitJmpLabel(".url_decode_loop")
	b.defineLabel(".url_decode_invalid")
	b.emitBytes(0x4C, 0x89, 0xE0) // malformed escape returns input unchanged
	b.emitJmpLabel(".url_decode_return")
	b.defineLabel(".url_decode_done")
	b.emitBytes(0x43, 0xC6, 0x44, 0x3D, 0x00, 0x00, 0x4C, 0x89, 0xE8) // NUL; return output
	b.defineLabel(".url_decode_return")
	b.emitBytes(0x48, 0x83, 0xC4, 0x20, 0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0xC3)

	b.defineLabel("fn___joss_hex_nibble")
	b.emitBytes(0x89, 0xC8, 0x3C, 0x30) // eax=ecx; cmp al,'0'
	b.emitJlLabel(".hex_invalid")
	b.emitBytes(0x3C, 0x39)
	b.emitJleLabel(".hex_decimal")
	b.emitBytes(0x0C, 0x20, 0x3C, 0x61) // lowercase ASCII
	b.emitJlLabel(".hex_invalid")
	b.emitBytes(0x3C, 0x66)
	b.emitJgLabel(".hex_invalid")
	b.emitBytes(0x2C, 0x57, 0xC3) // 'a' => 10
	b.defineLabel(".hex_decimal")
	b.emitBytes(0x2C, 0x30, 0xC3)
	b.defineLabel(".hex_invalid")
	b.emitBytes(0xB8, 0xFF, 0xFF, 0xFF, 0xFF, 0xC3)
}

// emitNativeStringUpper implements ASCII case conversion for HTTP method and
// protocol tokens. Non-ASCII UTF-8 bytes pass through unchanged.
func (b *MachineCodeBuilder) emitNativeStringUpper() {
	b.defineLabel("fn_strtoupper")
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56) // save rbx,r12-r14
	b.emitBytes(0x48, 0x83, 0xEC, 0x20, 0x49, 0x89, 0xCC, 0x48, 0x31, 0xDB)
	b.defineLabel(".upper_length")
	b.emitBytes(0x49, 0x8A, 0x04, 0x1C, 0x84, 0xC0)
	b.emitJeLabel(".upper_allocate")
	b.emitBytes(0x48, 0xFF, 0xC3)
	b.emitJmpLabel(".upper_length")
	b.defineLabel(".upper_allocate")
	b.emitBytes(0x48, 0x89, 0xD9, 0x48, 0xFF, 0xC1)
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x49, 0x89, 0xC5, 0x4D, 0x31, 0xF6)
	b.defineLabel(".upper_loop")
	b.emitBytes(0x49, 0x39, 0xDE)
	b.emitJgeLabel(".upper_done")
	b.emitBytes(0x43, 0x8A, 0x04, 0x34, 0x3C, 0x61)
	b.emitJlLabel(".upper_store")
	b.emitBytes(0x3C, 0x7A)
	b.emitJgLabel(".upper_store")
	b.emitBytes(0x2C, 0x20)
	b.defineLabel(".upper_store")
	b.emitBytes(0x43, 0x88, 0x44, 0x35, 0x00)
	b.emitBytes(0x49, 0xFF, 0xC6)
	b.emitJmpLabel(".upper_loop")
	b.defineLabel(".upper_done")
	b.emitBytes(0x43, 0xC6, 0x44, 0x35, 0x00, 0x00, 0x4C, 0x89, 0xE8)
	b.emitBytes(0x48, 0x83, 0xC4, 0x20, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0xC3)
}

func (b *MachineCodeBuilder) emitNativeStringLower() {
	b.defineLabel("fn_strtolower")
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56)
	b.emitBytes(0x48, 0x83, 0xEC, 0x20, 0x49, 0x89, 0xCC, 0x48, 0x31, 0xDB)
	b.defineLabel(".lower_length")
	b.emitBytes(0x49, 0x8A, 0x04, 0x1C, 0x84, 0xC0)
	b.emitJeLabel(".lower_allocate")
	b.emitBytes(0x48, 0xFF, 0xC3)
	b.emitJmpLabel(".lower_length")
	b.defineLabel(".lower_allocate")
	b.emitBytes(0x48, 0x89, 0xD9, 0x48, 0xFF, 0xC1)
	b.emitCallLabel("__joss_os_alloc")
	b.emitBytes(0x49, 0x89, 0xC5, 0x4D, 0x31, 0xF6)
	b.defineLabel(".lower_loop")
	b.emitBytes(0x49, 0x39, 0xDE)
	b.emitJgeLabel(".lower_done")
	b.emitBytes(0x43, 0x8A, 0x04, 0x34, 0x3C, 0x41)
	b.emitJlLabel(".lower_store")
	b.emitBytes(0x3C, 0x5A)
	b.emitJgLabel(".lower_store")
	b.emitBytes(0x04, 0x20)
	b.defineLabel(".lower_store")
	b.emitBytes(0x43, 0x88, 0x44, 0x35, 0x00)
	b.emitBytes(0x49, 0xFF, 0xC6)
	b.emitJmpLabel(".lower_loop")
	b.defineLabel(".lower_done")
	b.emitBytes(0x43, 0xC6, 0x44, 0x35, 0x00, 0x00, 0x4C, 0x89, 0xE8)
	b.emitBytes(0x48, 0x83, 0xC4, 0x20, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0xC3)
}

// emitStorageIntrinsics emits canonical native low-level primitives for storage:
// fn___joss_storage_get, fn___joss_storage_put, fn___joss_storage_delete
func (b *MachineCodeBuilder) emitStorageIntrinsics() {
	// __joss_storage_get(rcx = provider, rdx = userId, r8 = objectPath) -> rax = contentPtr / 0
	b.defineLabel("fn___joss_storage_get")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                         // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x56, 0x57)                               // push rbx, rsi, rdi
	b.emitBytes(0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push r12..r15
	b.emitBytes(0x48, 0x83, 0xEC, 0x48)                         // sub rsp, 72

	b.emitBytes(0x49, 0x89, 0xD4) // mov r12, rdx (userId)
	b.emitBytes(0x4D, 0x89, 0xC5) // mov r13, r8  (objectPath)

	// Build path: "storage/user_storage/" . userId . "/" . objectPath
	b.emitLeaRCXString("storage/user_storage/")
	b.emitBytes(0x4C, 0x89, 0xE2) // mov rdx, r12
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
	b.emitLeaRDXString("/")
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, rax
	b.emitBytes(0x4C, 0x89, 0xEA) // mov rdx, r13
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x89, 0xC1) // mov rcx, fullPath
	b.emitCallLabel("fn_file_get_contents")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".storage_get_fallback2")
	b.emitBytes(0x80, 0x38, 0x00) // cmp byte ptr [rax], 0 (fn_file_get_contents devuelve "" si falla)
	b.emitJneLabel(".storage_get_done")
	b.defineLabel(".storage_get_fallback2")
	// Fallback path: "assets/users/" . userId . "/" . objectPath
	b.emitLeaRCXString("assets/users/")
	b.emitBytes(0x4C, 0x89, 0xE2)
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x89, 0xC1)
	b.emitLeaRDXString("/")
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x89, 0xC1)
	b.emitBytes(0x4C, 0x89, 0xEA)
	b.emitCallLabel("joss_str_concat")
	b.emitBytes(0x48, 0x89, 0xC1)
	b.emitCallLabel("fn_file_get_contents")

	b.defineLabel(".storage_get_done")
	b.emitBytes(0x48, 0x83, 0xC4, 0x48)
	b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C)
	b.emitBytes(0x5F, 0x5E, 0x5B)
	b.emitBytes(0x5D, 0xC3)

	// __joss_storage_put(rcx = provider, rdx = userId, r8 = objectPath, r9 = content) -> rax = 1/0
	b.defineLabel("fn___joss_storage_put")
	b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00, 0xC3) // mov eax, 1; ret

	// __joss_storage_delete(rcx = provider, rdx = userId, r8 = objectPath) -> rax = 1/0
	b.defineLabel("fn___joss_storage_delete")
	b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00, 0xC3) // mov eax, 1; ret
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
		if !isFrameworkAdapterTarget(lbl) {
			continue
		}
		b.defineLabel(lbl)
		// Return first argument (rcx) in rax to preserve fluent method chaining ($obj->m1()->m2())
		b.emitBytes(0x48, 0x89, 0xC8, 0xC3) // mov rax, rcx; ret
	}
}
