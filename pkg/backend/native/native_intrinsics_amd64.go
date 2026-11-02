package native

import "fmt"

const (
	dataOffGranDBTables uint32 = 0x0680
	dataOffSEOTitle     uint32 = 0x0688
	dataOffSEODesc      uint32 = 0x0690
	dataOffSEOCanonical uint32 = 0x0698
	dataOffEnvMap       uint32 = 0x06A0
	dataOffPort         uint32 = 0x06A8
)

func (b *MachineCodeBuilder) emitMovRDXData(dataOff uint32) {
	b.asmListing.WriteString(fmt.Sprintf("    mov rdx, [rip + data_%x]\n", dataOff))
	b.emitBytes(0x48, 0x8B, 0x15)
	patchOff := b.text.Len()
	b.emitU32(0)
	b.relocs = append(b.relocs, machineReloc{kind: relocData, patchOffset: patchOff, targetOff: dataOff})
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
	// fn_html_escape(rcx = val) -> rax = strPtr ("" if 0/null, else joss_to_string)
	b.defineLabel("fn_html_escape")
	b.defineLabel("joss_html_escape")
	b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
	b.emitJneLabel("joss_to_string")
	b.emitLeaRAXString("")
	b.emitBytes(0xC3)

	// String and Math helper passthroughs
	b.defineLabel("fn_Str_trim")
	b.defineLabel("fn_Math_ceil")
	b.emitBytes(0x48, 0x89, 0xC8, 0xC3) // mov rax, rcx; ret

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

	// __joss_db_is_connected() -> rax = 0 (standalone CLI has no DB connection)
	b.defineLabel("fn___joss_db_is_connected")
	b.emitBytes(0x31, 0xC0) // xor eax, eax
	b.emitBytes(0xC3)       // ret

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
	b.emitJeLabel(".env_load_done")
	b.emitBytes(0x80, 0x38, 0x00)
	b.emitJeLabel(".env_load_done")
	b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax
	b.emitLeaRCXString("[Security] Cargando entorno...\n[Security] Entorno cargado desde '.env'")
	b.emitCallLabel("joss_print_string")
	b.emitBytes(0x48, 0x89, 0xD8) // mov rax, rbx

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

	// fn_env(rcx = key, rdx = defaultVal) -> rax = val from loaded environment map, or defaultVal
	b.defineLabel("fn_env")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x48, 0x83, 0xEC, 0x20) // sub rsp, 32
	b.emitBytes(0x48, 0x89, 0x55, 0xF8) // mov [rbp - 8], rdx (save defaultVal)
	b.emitBytes(0x48, 0x89, 0x4D, 0xF0) // mov [rbp - 16], rcx (save key)
	b.emitMovRCXData(dataOffEnvMap)     // mov rcx, [dataOffEnvMap]
	b.emitBytes(0x48, 0x85, 0xC9)       // test rcx, rcx
	b.emitJneLabel(".env_map_ready")
	b.emitCallLabel("__joss_load_env_file")
	b.defineLabel(".env_map_ready")
	b.emitMovRCXData(dataOffEnvMap)
	b.emitBytes(0x48, 0x85, 0xC9)
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
	b.defineLabel("fn_Markdown_readFile")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x41, 0x54, 0x41, 0x55) // push r12; push r13
	b.emitBytes(0x48, 0x83, 0xEC, 0x20) // sub rsp, 32
	b.emitBytes(0x49, 0x89, 0xCC)       // mov r12, rcx (pathPtr)
	b.emitCallLabel("fn_file_get_contents")
	b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
	b.emitJeLabel(".md_try_fallback")
	b.emitBytes(0x80, 0x38, 0x00) // cmp byte ptr [rax], 0
	b.emitJneLabel(".md_read_done")
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
	b.emitJmpLabel(".md_read_done")
	b.defineLabel(".md_read_empty")
	b.emitLeaRAXString("")
	b.defineLabel(".md_read_done")
	b.emitBytes(0x48, 0x83, 0xC4, 0x20) // add rsp, 32
	b.emitBytes(0x41, 0x5D, 0x41, 0x5C, 0x5D, 0xC3)
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
