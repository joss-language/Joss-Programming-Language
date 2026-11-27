package architecture_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestArchitecture_RuntimeCMemorySafetyGate analyzes every C11 runtime source
// file in runtime/native/*.c and *.inc and hard-fails the test suite (blocking
// CI and builds) if any banned unsafe C function, leaky per-request allocation
// pattern, unbalanced StringBuilder/FILE resource, or unapproved raw heap
// allocation is introduced.
func TestArchitecture_RuntimeCMemorySafetyGate(t *testing.T) {
	rtDir := filepath.Join("..", "..", "runtime", "native")
	entries, err := os.ReadDir(rtDir)
	if err != nil {
		t.Fatalf("failed reading runtime/native: %v", err)
	}

	// 1. Banned unsafe C functions (zero tolerance)
	bannedUnsafeRe := regexp.MustCompile(`\b(gets|strcpy|strcat|sprintf|vsprintf|alloca|valloc|strdup|strndup)\s*\(`)
	// 2. Banned return of persistent joss_web_strdup directly to Joss callers
	//    (must use joss_req_strdup unless annotated /* joss-mem: persistent-ok */)
	returnWebStrdupRe := regexp.MustCompile(`\breturn\b[^;]*\bjoss_web_strdup\s*\(`)
	// 3. Raw heap allocation ratchet per file (exact audited ceiling — prevents
	//    introducing new unreviewed malloc/calloc/realloc calls in runtime/native/)
	rawAllocRe := regexp.MustCompile(`\b(malloc|calloc|realloc)\s*\(`)
	// 4. Function header matcher for per-function resource balance checks
	funcHeaderRe := regexp.MustCompile(`^(?:static\s+)?(?:inline\s+)?[A-Za-z_][A-Za-z0-9_\s\*]+\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^;]*\)\s*\{`)
	sbNewRe := regexp.MustCompile(`\bjoss_sb_new\s*\(`)
	sbFreeRe := regexp.MustCompile(`\bjoss_sb_free\s*\(`)
	sbToStringRetRe := regexp.MustCompile(`char\*\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*joss_sb_to_string\s*\(`)
	fopenRe := regexp.MustCompile(`\bfopen\s*\(`)
	fcloseRe := regexp.MustCompile(`\bfclose\s*\(`)
	lockRe := regexp.MustCompile(`\bjoss_global_lock\s*\(`)
	unlockRe := regexp.MustCompile(`\bjoss_global_unlock\s*\(`)
	arenaPauseRe := regexp.MustCompile(`\bjoss_req_arena_pause\s*\(`)
	arenaResumeRe := regexp.MustCompile(`\bjoss_req_arena_resume\s*\(`)
	scopeBeginRe := regexp.MustCompile(`\bjoss_owned_scope_begin\s*\(`)
	scopeEndRe := regexp.MustCompile(`\bjoss_owned_scope_end\s*\(`)
	bufNewRe := regexp.MustCompile(`\bjoss_buf_new\s*\(`)
	bufFreeRe := regexp.MustCompile(`\bjoss_buf_free\s*\(`)
	sqlitePrepRe := regexp.MustCompile(`\bsqlite3_prepare_v2\s*\(`)
	sqliteFinRe := regexp.MustCompile(`\bsqlite3_finalize\s*\(`)
	evpNewRe := regexp.MustCompile(`\bEVP_MD_CTX_new\s*\(`)
	evpFreeRe := regexp.MustCompile(`\bEVP_MD_CTX_free\s*\(`)
	bioNewRe := regexp.MustCompile(`\bBIO_new_mem_buf\s*\(`)
	bioFreeRe := regexp.MustCompile(`\bBIO_free\s*\(`)
	bcryptAlgOpenRe := regexp.MustCompile(`\bBCryptOpenAlgorithmProvider\s*\(`)
	bcryptAlgCloseRe := regexp.MustCompile(`\bBCryptCloseAlgorithmProvider\s*\(`)
	bcryptHashCreateRe := regexp.MustCompile(`\bBCryptCreateHash\s*\(`)
	bcryptHashDestroyRe := regexp.MustCompile(`\bBCryptDestroyHash\s*\(`)
	pthreadLockRe := regexp.MustCompile(`\bpthread_mutex_lock\s*\(`)
	pthreadUnlockRe := regexp.MustCompile(`\bpthread_mutex_unlock\s*\(`)
	winLockRe := regexp.MustCompile(`\bEnterCriticalSection\s*\(`)
	winUnlockRe := regexp.MustCompile(`\bLeaveCriticalSection\s*\(`)
	mmapRe := regexp.MustCompile(`\bmmap\s*\(`)
	munmapRe := regexp.MustCompile(`\bmunmap\s*\(`)

	// Audited raw malloc/calloc/realloc ceiling per file.
	// To add new allocations in runtime/native/, prefer joss_req_alloc /
	// joss_req_alloc_zeroed / joss_sb_to_req_string / JossOwnedScope instead.
	maxRawAllocs := map[string]int{
		"joss_rt.c":                  35,
		"joss_memory_rt.inc":         0,
		"joss_web_rt.c":              86,
		"joss_redis_rt.inc":          0,
		"joss_zip_rt.inc":            8,
		"joss_concurrency_rt.c":      14,
		"joss_db_rt.c":               54,
		"joss_db_wire.inc":           20,
		"joss_crypto_rt.c":           1,
		"joss_crypto_primitives.inc": 2,
		"joss_gui_rt.c":              16,
	}

	var violations []string
	scannedFiles := 0

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasSuffix(name, "_test.c") {
			continue
		}
		if !strings.HasSuffix(name, ".c") && !strings.HasSuffix(name, ".inc") {
			continue
		}
		scannedFiles++
		fullPath := filepath.Join(rtDir, name)
		rawBytes, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			t.Fatalf("failed reading %s: %v", fullPath, readErr)
		}
		content := string(rawBytes)
		lines := strings.Split(content, "\n")

		rawAllocCount := 0
		inBlockComment := false

		// Per-function tracking for StringBuilder leak & joss_sb_to_string return checks
		curFunc := ""
		braceDepth := 0
		fnHasSbNew := false
		fnHasSbFree := false
		fnSbToStrVars := map[string]int{}
		fnBodyLines := []string{}

		flushFunc := func() {
			if curFunc == "" {
				return
			}
			body := strings.Join(fnBodyLines, "\n")
			if fnHasSbNew && !fnHasSbFree && curFunc != "joss_sb_new" {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a joss_sb_new() sin joss_sb_free() en la misma función — libera el StringBuilder antes de retornar.",
					name, curFunc))
			}
			if fopenRe.MatchString(body) && !fcloseRe.MatchString(body) &&
				curFunc != "fn_FileStream_open" && curFunc != "fn_StreamReader_open" && curFunc != "fn_StreamWriter_open" {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a fopen() sin fclose() en la misma función (fuga de descriptor de archivo).",
					name, curFunc))
			}
			if lockRe.MatchString(body) && !unlockRe.MatchString(body) && curFunc != "joss_global_lock" {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a joss_global_lock() sin joss_global_unlock() (riesgo de deadlock).",
					name, curFunc))
			}
			if arenaPauseRe.MatchString(body) && !arenaResumeRe.MatchString(body) && curFunc != "joss_req_arena_pause" {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a joss_req_arena_pause() sin joss_req_arena_resume() (fuga permanente en worker HTTP).",
					name, curFunc))
			}
			if scopeBeginRe.MatchString(body) && !scopeEndRe.MatchString(body) && curFunc != "joss_owned_scope_begin" {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a joss_owned_scope_begin() sin joss_owned_scope_end() (arena TLS corrupto).",
					name, curFunc))
			}
			if bufNewRe.MatchString(body) && !bufFreeRe.MatchString(body) && curFunc != "joss_sb_new" && curFunc != "joss_buf_new" {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a joss_buf_new() sin joss_buf_free() (fuga de ByteBuffer).",
					name, curFunc))
			}
			if sqlitePrepRe.MatchString(body) && !sqliteFinRe.MatchString(body) {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a sqlite3_prepare_v2() sin sqlite3_finalize() (fuga de statement SQLite).",
					name, curFunc))
			}
			if evpNewRe.MatchString(body) && !evpFreeRe.MatchString(body) {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a EVP_MD_CTX_new() sin EVP_MD_CTX_free() (fuga de contexto OpenSSL).",
					name, curFunc))
			}
			if bioNewRe.MatchString(body) && !bioFreeRe.MatchString(body) {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a BIO_new_mem_buf() sin BIO_free() (fuga de BIO OpenSSL).",
					name, curFunc))
			}
			if bcryptAlgOpenRe.MatchString(body) && !bcryptAlgCloseRe.MatchString(body) {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a BCryptOpenAlgorithmProvider() sin BCryptCloseAlgorithmProvider() (fuga de handle Win32).",
					name, curFunc))
			}
			if bcryptHashCreateRe.MatchString(body) && !bcryptHashDestroyRe.MatchString(body) {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a BCryptCreateHash() sin BCryptDestroyHash() (fuga de handle Win32).",
					name, curFunc))
			}
			if pthreadLockRe.MatchString(body) && !pthreadUnlockRe.MatchString(body) && curFunc != "joss_global_lock" && curFunc != "fn_Mutex_lock" {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a pthread_mutex_lock() sin pthread_mutex_unlock() (riesgo de deadlock).",
					name, curFunc))
			}
			if winLockRe.MatchString(body) && !winUnlockRe.MatchString(body) && curFunc != "fn_Mutex_lock" {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a EnterCriticalSection() sin LeaveCriticalSection() (riesgo de deadlock).",
					name, curFunc))
			}
			if mmapRe.MatchString(body) && !munmapRe.MatchString(body) {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s (%s): llama a mmap() sin munmap() en C11 (fuga de páginas virtuales).",
					name, curFunc))
			}
			for varName, lineNum := range fnSbToStrVars {
				retRe := regexp.MustCompile(`\breturn\s+(?:\([^)]*\)\s*)?` + regexp.QuoteMeta(varName) + `\b`)
				freeRe := regexp.MustCompile(`\bfree\s*\(\s*` + regexp.QuoteMeta(varName) + `\s*\)`)
				if retRe.MatchString(body) && !freeRe.MatchString(body) &&
					curFunc != "joss_sb_to_string" && curFunc != "joss_buf_to_cstr" &&
					!strings.Contains(body, "joss-mem: persistent-ok") &&
					curFunc != "joss_json_parse_string_val" && curFunc != "joss_run_cmd_capture" &&
					curFunc != "rewrite_pg_placeholders" && curFunc != "joss_wire_bind_sql" {
					violations = append(violations, fmt.Sprintf(
						"BLOQUEADO [JOSS-MEM-GATE] %s:%d (%s): retorna `%s` creado con joss_sb_to_string() (malloc crudo) sin free() — usa joss_sb_to_req_string(sb).",
						name, lineNum, curFunc, varName))
				}
			}
			curFunc = ""
			fnHasSbNew = false
			fnHasSbFree = false
			fnSbToStrVars = map[string]int{}
			fnBodyLines = nil
		}

		for idx, rawLine := range lines {
			lineNo := idx + 1
			line := strings.TrimSpace(rawLine)

			// Strip single-line comments & track block comments for pattern scanning
			scanLine := rawLine
			if inBlockComment {
				if endIdx := strings.Index(scanLine, "*/"); endIdx >= 0 {
					scanLine = scanLine[endIdx+2:]
					inBlockComment = false
				} else {
					continue
				}
			}
			for {
				startIdx := strings.Index(scanLine, "/*")
				if startIdx < 0 {
					break
				}
				endIdx := strings.Index(scanLine[startIdx+2:], "*/")
				if endIdx >= 0 {
					scanLine = scanLine[:startIdx] + scanLine[startIdx+2+endIdx+2:]
				} else {
					scanLine = scanLine[:startIdx]
					inBlockComment = true
					break
				}
			}
			if slIdx := strings.Index(scanLine, "//"); slIdx >= 0 {
				scanLine = scanLine[:slIdx]
			}
			trimmedScan := strings.TrimSpace(scanLine)
			if trimmedScan == "" {
				continue
			}

			// Rule 1: Zero banned unsafe C functions
			if m := bannedUnsafeRe.FindStringSubmatch(trimmedScan); m != nil {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s:%d: función insegura prohibida `%s()` — usa snprintf / joss_req_strdup / joss_req_alloc.",
					name, lineNo, m[1]))
			}

			// Rule 2: Zero direct `return joss_web_strdup(...)` unless explicitly approved
			if returnWebStrdupRe.MatchString(trimmedScan) &&
				!strings.Contains(rawLine, "joss-mem: persistent-ok") &&
				curFunc != "joss_clone_session_string" && curFunc != "joss_web_strdup" {
				violations = append(violations, fmt.Sprintf(
					"BLOQUEADO [JOSS-MEM-GATE] %s:%d (%s): retorno directo de `joss_web_strdup()` (fuga en heap) — usa `joss_req_strdup()`.",
					name, lineNo, curFunc))
			}

			// Rule 3: Count raw malloc/calloc/realloc calls
			if matches := rawAllocRe.FindAllString(trimmedScan, -1); len(matches) > 0 {
				if !strings.Contains(rawLine, "joss-mem: persistent-ok") {
					rawAllocCount += len(matches)
				}
			}

			// Track function scope for Rule 4 (StringBuilder & sb_to_string balance)
			if braceDepth == 0 {
				if fm := funcHeaderRe.FindStringSubmatch(trimmedScan); fm != nil {
					curFunc = fm[1]
					fnHasSbNew = false
					fnHasSbFree = false
					fnSbToStrVars = map[string]int{}
					fnBodyLines = []string{rawLine}
				}
			} else if curFunc != "" {
				fnBodyLines = append(fnBodyLines, rawLine)
			}

			if curFunc != "" {
				if sbNewRe.MatchString(trimmedScan) {
					fnHasSbNew = true
				}
				if sbFreeRe.MatchString(trimmedScan) {
					fnHasSbFree = true
				}
				if sm := sbToStringRetRe.FindStringSubmatch(trimmedScan); sm != nil {
					fnSbToStrVars[sm[1]] = lineNo
				}
			}

			braceDepth += strings.Count(trimmedScan, "{") - strings.Count(trimmedScan, "}")
			if braceDepth < 0 {
				braceDepth = 0
			}
			if braceDepth == 0 && curFunc != "" && strings.Contains(trimmedScan, "}") {
				flushFunc()
			}
			_ = line
		}
		flushFunc()

		// Enforce strict raw allocation ratchet ceiling per file
		ceiling, known := maxRawAllocs[name]
		if !known {
			violations = append(violations, fmt.Sprintf(
				"BLOQUEADO [JOSS-MEM-GATE] %s: nuevo archivo C en runtime/native/ no registrado en maxRawAllocs (raw allocs=%d) — audítalo y regístralo.",
				name, rawAllocCount))
		} else if rawAllocCount > ceiling {
			violations = append(violations, fmt.Sprintf(
				"BLOQUEADO [JOSS-MEM-GATE] %s: se detectaron %d llamadas crudas a malloc/calloc/realloc (límite auditado: %d) — usa joss_req_alloc / joss_req_alloc_zeroed / JossOwnedScope.",
				name, rawAllocCount, ceiling))
		}
	}

	if scannedFiles < 8 {
		t.Fatalf("expected at least 8 C11 runtime files in %s, scanned %d", rtDir, scannedFiles)
	}
	if len(violations) > 0 {
		t.Fatalf("Auditoría estática de seguridad de memoria C11 BLOQUEADA (%d violaciones):\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}
