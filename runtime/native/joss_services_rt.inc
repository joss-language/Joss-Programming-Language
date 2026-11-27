/* =========================================================================
 * Native C11 Host Services: Cache, Markdown, Response, System::Run,
 * Http, WebSocket, AI, Notify & Plugin
 * ========================================================================= */

static void* g_joss_cache_values = NULL;
static void* g_joss_cache_expiry = NULL;
static void* g_joss_cache_owners = NULL;

static void joss_cache_init(void) {
    bool _prev_arena = joss_req_arena_pause();
    if (!g_joss_cache_values) {
        g_joss_cache_values = joss_map_new();
        joss_gc_add_root(&g_joss_cache_values);
    }
    if (!g_joss_cache_expiry) {
        g_joss_cache_expiry = joss_map_new();
        joss_gc_add_root(&g_joss_cache_expiry);
    }
    if (!g_joss_cache_owners) {
        g_joss_cache_owners = joss_map_new();
        joss_gc_add_root(&g_joss_cache_owners);
    }
    joss_req_arena_resume(_prev_arena);
}

static void joss_cache_remove_locked(const char* key) {
    if (!key) return;
    if (g_joss_cache_owners && joss_array_key_exists(key, g_joss_cache_owners)) {
        JossOwnedScope* old_owner = (JossOwnedScope*)(uintptr_t)joss_map_get_i64(g_joss_cache_owners, key);
        joss_map_delete_i64(g_joss_cache_owners, key);
        if (old_owner) joss_owned_scope_destroy(old_owner);
    }
    if (g_joss_cache_values) joss_map_delete_i64(g_joss_cache_values, key);
    if (g_joss_cache_expiry) joss_map_delete_i64(g_joss_cache_expiry, key);
}

static int64_t joss_cache_has_locked(const char* key) {
    if (!key) return 0;
    joss_cache_init();
    if (!joss_array_key_exists(key, g_joss_cache_values)) return 0;
    int64_t exp = joss_map_get_i64(g_joss_cache_expiry, key);
    if (exp > 0 && (int64_t)time(NULL) > exp) {
        joss_cache_remove_locked(key);
        return 0;
    }
    return 1;
}

static int64_t joss_clone_session_value(int64_t v);

int64_t fn_Cache_put(const char* key, int64_t val, int64_t ttl_seconds) {
    key = unbox_web_cstr(key);
    if (!key) return 0;
    joss_global_lock(JOSS_LOCK_CACHE);
    joss_cache_init();
    JossOwnedScope* owner = joss_owned_scope_begin();
    int64_t persisted_val = joss_clone_session_value(val);
    joss_owned_scope_end(owner);
    bool _prev_arena = joss_req_arena_pause();
    if (joss_array_key_exists(key, g_joss_cache_owners)) {
        JossOwnedScope* old_owner = (JossOwnedScope*)(uintptr_t)joss_map_get_i64(g_joss_cache_owners, key);
        if (old_owner) joss_owned_scope_destroy(old_owner);
    }
    joss_map_set_i64(g_joss_cache_values, key, persisted_val);
    joss_map_set_i64(g_joss_cache_owners, key, (int64_t)(uintptr_t)owner);
    int64_t exp = (ttl_seconds > 0) ? ((int64_t)time(NULL) + ttl_seconds) : 0;
    joss_map_set_i64(g_joss_cache_expiry, key, exp);
    joss_req_arena_resume(_prev_arena);
    joss_global_unlock(JOSS_LOCK_CACHE);
    return 1;
}

int64_t fn_Cache_has(const char* key) {
    key = unbox_web_cstr(key);
    if (!key) return 0;
    joss_global_lock(JOSS_LOCK_CACHE);
    int64_t ok = joss_cache_has_locked(key);
    joss_global_unlock(JOSS_LOCK_CACHE);
    return ok;
}

int64_t fn_Cache_get(const char* key, int64_t defVal) {
    key = unbox_web_cstr(key);
    if (!key) return defVal;
    joss_global_lock(JOSS_LOCK_CACHE);
    int64_t val = defVal;
    if (joss_cache_has_locked(key)) {
        /* Clone into the caller's active request arena (or Mini-GC) so the cache's
         * private JossOwnedScope can always be safely destroyed on overwrite/expiry/forget. */
        val = joss_clone_session_value(joss_map_get_i64(g_joss_cache_values, key));
    }
    joss_global_unlock(JOSS_LOCK_CACHE);
    return val;
}

int64_t fn_Cache_forget(const char* key) {
    key = unbox_web_cstr(key);
    if (!key) return 0;
    joss_global_lock(JOSS_LOCK_CACHE);
    joss_cache_init();
    joss_cache_remove_locked(key);
    joss_global_unlock(JOSS_LOCK_CACHE);
    return 1;
}

/* Markdown::readFile and Markdown::toHtml now live in Pure Joss (std/markdown/markdown.joss). */

const char* fn_Response_json(int64_t data, int64_t status) {
    return joss_json(data, status);
}

const char* fn_Response_raw(const char* body, int64_t status, const char* content_type) {
    size_t bin_len = 0;
    const uint8_t* bin_data = joss_bytes_unwrap(body, &bin_len);
    if (!bin_data) {
        body = unbox_web_cstr(body);
        if (!body) body = "";
    }
    status = joss_intval(status);
    if (status < 100 || status > 599) status = 200;
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 ");
    joss_sb_append_i64(sb, status);
    joss_sb_append(sb, " OK\r\n");
    int has_ct = 0;
    int64_t hv = (int64_t)(uintptr_t)content_type;
    if (joss_is_map_ptr(hv)) {
        /* Headers map: emit every header (parity with Response::raw in pkg/core). */
        int64_t n = joss_map_len((void*)content_type);
        for (int64_t i = 0; i < n; i++) {
            const char* k = unbox_web_cstr(joss_map_key_at((void*)content_type, i));
            const char* v = unbox_web_cstr((const char*)(uintptr_t)joss_map_val_at((void*)content_type, i));
            if (!k || !*k || !v) continue;
            if (strcasecmp(k, "Content-Type") == 0) has_ct = 1;
            joss_sb_append(sb, k);
            joss_sb_append(sb, ": ");
            joss_sb_append(sb, v);
            joss_sb_append(sb, "\r\n");
        }
    } else {
        content_type = unbox_web_cstr(content_type);
        if (content_type && *content_type) {
            joss_sb_append(sb, "Content-Type: ");
            joss_sb_append(sb, content_type);
            joss_sb_append(sb, "\r\n");
            has_ct = 1;
        }
    }
    if (!has_ct) joss_sb_append(sb, "Content-Type: text/plain; charset=utf-8\r\n");
    if (bin_data) {
        joss_sb_append(sb, "Content-Length: ");
        joss_sb_append_i64(sb, (int64_t)bin_len);
        joss_sb_append(sb, "\r\nConnection: close\r\n\r\n");
        joss_buf_append_bytes(sb, bin_data, (int64_t)bin_len);
        void* out = joss_bytes_new(sb->data, (size_t)sb->len);
        return (const char*)out;
    }
    joss_sb_append(sb, "Connection: close\r\n\r\n");
    joss_sb_append(sb, body);
    char* out = joss_sb_to_req_string(sb);
    return out;
}

const char* fn_Response_error(const char* msg, int64_t status) {
    msg = unbox_web_cstr(msg);
    status = joss_intval(status);
    if (status < 100 || status > 599) status = 500;
    return fn_Response_raw(msg ? msg : "Internal Server Error", status, "text/plain; charset=utf-8");
}

const char* fn_Response_download(const char* path, const char* filename) {
    path = unbox_web_cstr(path);
    filename = unbox_web_cstr(filename);
    const char* content = joss_file_get_contents(path);
    if (!filename || !*filename) filename = "download.bin";
    size_t bin_len = 0;
    const uint8_t* bin_data = joss_bytes_unwrap(content, &bin_len);
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=\"");
    joss_sb_append(sb, filename);
    joss_sb_append(sb, "\"\r\nConnection: close\r\n\r\n");
    if (bin_data) {
        joss_buf_append_bytes(sb, bin_data, (int64_t)bin_len);
        void* out = joss_bytes_new(sb->data, (size_t)sb->len);
        return (const char*)out;
    }
    joss_sb_append(sb, content ? content : "");
    char* out = joss_sb_to_req_string(sb);
    return out;
}

const char* fn_Response_stream(const char* body, const char* content_type) {
    return fn_Response_raw(body, 200, content_type);
}

const char* fn_Response_redirect(const char* url, int64_t status) {
    return joss_redirect(url, status);
}

const char* fn_Response_back(void) {
    return joss_back();
}

void* fn_WebResponse_with(void* resp, const char* key, int64_t value) {
    return with(resp, key, value);
}

void* fn_WebResponse_withCookie(void* resp, const char* name, const char* value) {
    return withCookie(resp, unbox_web_cstr(name), unbox_web_cstr(value));
}

#if defined(_WIN32)
static int joss_win_run_hidden_cmd(const char* cmd) {
    if (!cmd || !*cmd) return -1;
    const char* comspec = getenv("COMSPEC");
    if (!comspec || !*comspec) comspec = "cmd.exe";
    size_t clen = strlen(comspec) + strlen(cmd) + 16;
    char* cmdline = (char*)malloc(clen);
    if (!cmdline) return -1;
    snprintf(cmdline, clen, "\"%s\" /c %s", comspec, cmd);

    STARTUPINFOA si;
    PROCESS_INFORMATION pi;
    memset(&si, 0, sizeof(si));
    memset(&pi, 0, sizeof(pi));
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;

    BOOL ok = CreateProcessA(NULL, cmdline, NULL, NULL, FALSE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi);
    free(cmdline);
    if (!ok) return -1;
    WaitForSingleObject(pi.hProcess, INFINITE);
    DWORD exit_code = 1;
    GetExitCodeProcess(pi.hProcess, &exit_code);
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);
    return (int)exit_code;
}
#endif

#if defined(JOSS_HAS_RUST_RT)
extern uint8_t* joss_rs_system_run_alloc(const char* cmd, size_t* out_len);
extern void     joss_rs_bytes_free(uint8_t* ptr, size_t len);
#endif

static char* joss_system_run_bytes(const char* cmd, size_t* out_len) {
    if (out_len) *out_len = 0;
    cmd = unbox_web_cstr(cmd);
    if (!cmd || !*cmd) return NULL;
#if defined(JOSS_HAS_RUST_RT)
    size_t n = 0;
    uint8_t* raw = joss_rs_system_run_alloc(cmd, &n);
    if (!raw) return NULL;
    char* out = (char*)joss_req_alloc(n + 1U);
    if (!out) {
        joss_rs_bytes_free(raw, n);
        return NULL;
    }
    if (n > 0) memcpy(out, raw, n);
    out[n] = '\0';
    joss_rs_bytes_free(raw, n);
    if (out_len) *out_len = n;
    return out;
#elif defined(_WIN32)
    const char* comspec = getenv("COMSPEC");
    if (!comspec || !*comspec) comspec = "cmd.exe";
    size_t clen = strlen(comspec) + strlen(cmd) + 16;
    char* cmdline = (char*)malloc(clen);
    if (!cmdline) return NULL;
    snprintf(cmdline, clen, "\"%s\" /c %s", comspec, cmd);

    SECURITY_ATTRIBUTES sa;
    memset(&sa, 0, sizeof(sa));
    sa.nLength = sizeof(sa);
    sa.bInheritHandle = TRUE;
    HANDLE hRead = NULL, hWrite = NULL;
    if (!CreatePipe(&hRead, &hWrite, &sa, 0)) {
        free(cmdline);
        return NULL;
    }
    SetHandleInformation(hRead, HANDLE_FLAG_INHERIT, 0);

    STARTUPINFOA si;
    PROCESS_INFORMATION pi;
    memset(&si, 0, sizeof(si));
    memset(&pi, 0, sizeof(pi));
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESTDHANDLES | STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;
    si.hStdOutput = hWrite;
    si.hStdError = GetStdHandle(STD_ERROR_HANDLE);
    si.hStdInput = GetStdHandle(STD_INPUT_HANDLE);

    BOOL ok = CreateProcessA(NULL, cmdline, NULL, NULL, TRUE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi);
    free(cmdline);
    CloseHandle(hWrite);
    if (!ok) {
        CloseHandle(hRead);
        return NULL;
    }

    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    char buf[4096];
    DWORD nread = 0;
    while (ReadFile(hRead, buf, sizeof(buf), &nread, NULL) && nread > 0) {
        joss_buf_append_bytes(sb, (const uint8_t*)buf, (int64_t)nread);
    }
    CloseHandle(hRead);
    WaitForSingleObject(pi.hProcess, INFINITE);
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);
    if (out_len) *out_len = (size_t)sb->len;
    return joss_sb_to_req_string(sb);
#else
    FILE* pipe = popen(cmd, "r");
    if (!pipe) return NULL;
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    char buf[4096];
    size_t n;
    while ((n = fread(buf, 1, sizeof(buf), pipe)) > 0) {
        joss_buf_append_bytes(sb, (const uint8_t*)buf, (int64_t)n);
    }
    pclose(pipe);
    if (out_len) *out_len = (size_t)sb->len;
    return joss_sb_to_req_string(sb);
#endif
}

static bool joss_http_write_temp_file(const uint8_t* data, size_t len, char* out_path, size_t out_cap);
static char* joss_web_strdup(const char* s);

static const char* joss_maybe_materialize_embedded_arg(const char* arg, char* tmp_out, size_t tmp_cap) {
    if (!arg || !arg[0] || !g_joss_embedded_asset_resolver) return arg;
    size_t alen = strlen(arg);
    if (alen == 0 || alen >= 400 || strchr(arg, '\n') != NULL || strchr(arg, '\r') != NULL) return arg;
    /* If it already exists on disk, use it directly. */
    JOSS_AUTO_FILE FILE* f = fopen(arg, "rb");
    if (f) {
        return arg;
    }
    const char* emb = joss_lookup_embedded_priv_or_asset(arg, true);
    if (!emb || !emb[0]) return arg;
    if (joss_http_write_temp_file((const uint8_t*)emb, strlen(emb), tmp_out, tmp_cap)) {
        return tmp_out;
    }
    return arg;
}

#if defined(_WIN32)
static void joss_win_append_quoted_arg(JossStringBuilder* sb, const char* arg);

static char* joss_system_run_argv(const char* prog, void* args_arr) {
    if (!prog || !*prog) return NULL;
    JOSS_AUTO_SB JossStringBuilder* cmd_sb = joss_sb_new();
    joss_win_append_quoted_arg(cmd_sb, prog);
    int64_t argc = (args_arr && joss_is_arr_ptr((int64_t)(uintptr_t)args_arr)) ? joss_arr_len(args_arr) : 0;
    char** temp_files = (argc > 0) ? (char**)calloc((size_t)argc, sizeof(char*)) : NULL;
    for (int64_t i = 0; i < argc; i++) {
        joss_buf_append_byte(cmd_sb, ' ');
        const char* a = joss_strval(joss_arr_get_i64(args_arr, i));
        char tmp_path[512];
        tmp_path[0] = '\0';
        const char* resolved = joss_maybe_materialize_embedded_arg(a ? a : "", tmp_path, sizeof(tmp_path));
        if (resolved == tmp_path && temp_files) {
            temp_files[i] = joss_web_strdup(tmp_path);
        }
        joss_win_append_quoted_arg(cmd_sb, resolved ? resolved : "");
    }
    char* cmdline = joss_sb_to_string(cmd_sb);
    if (!cmdline) {
        if (temp_files) {
            for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
            free(temp_files);
        }
        return NULL;
    }

    SECURITY_ATTRIBUTES sa;
    memset(&sa, 0, sizeof(sa));
    sa.nLength = sizeof(sa);
    sa.bInheritHandle = TRUE;
    HANDLE hRead = NULL, hWrite = NULL;
    if (!CreatePipe(&hRead, &hWrite, &sa, 0)) {
        free(cmdline);
        if (temp_files) {
            for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
            free(temp_files);
        }
        return NULL;
    }
    SetHandleInformation(hRead, HANDLE_FLAG_INHERIT, 0);

    STARTUPINFOA si;
    PROCESS_INFORMATION pi;
    memset(&si, 0, sizeof(si));
    memset(&pi, 0, sizeof(pi));
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESTDHANDLES;
    si.hStdOutput = hWrite;
    si.hStdError = hWrite;
    si.hStdInput = GetStdHandle(STD_INPUT_HANDLE);

    BOOL ok = CreateProcessA(NULL, cmdline, NULL, NULL, TRUE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi);
    free(cmdline);
    CloseHandle(hWrite);
    if (!ok) {
        CloseHandle(hRead);
        if (temp_files) {
            for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
            free(temp_files);
        }
        return NULL;
    }

    JOSS_AUTO_SB JossStringBuilder* out_sb = joss_sb_new();
    char buf[4096];
    DWORD nread = 0;
    while (ReadFile(hRead, buf, sizeof(buf), &nread, NULL) && nread > 0) {
        joss_buf_append_bytes(out_sb, (const uint8_t*)buf, (int64_t)nread);
    }
    CloseHandle(hRead);
    WaitForSingleObject(pi.hProcess, 30000);
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);
    if (temp_files) {
        for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
        free(temp_files);
    }
    char* out = joss_sb_to_req_string(out_sb);
    return out;
}
#else
#include <sys/wait.h>
static char* joss_system_run_argv(const char* prog, void* args_arr) {
    if (!prog || !*prog) return NULL;
    int64_t argc = (args_arr && joss_is_arr_ptr((int64_t)(uintptr_t)args_arr)) ? joss_arr_len(args_arr) : 0;
    char** argv = (char**)calloc((size_t)(argc + 2), sizeof(char*));
    char** temp_files = (argc > 0) ? (char**)calloc((size_t)argc, sizeof(char*)) : NULL;
    if (!argv) { free(temp_files); return NULL; }
    argv[0] = (char*)prog;
    for (int64_t i = 0; i < argc; i++) {
        const char* a = joss_strval(joss_arr_get_i64(args_arr, i));
        char tmp_path[512];
        tmp_path[0] = '\0';
        const char* resolved = joss_maybe_materialize_embedded_arg(a ? a : "", tmp_path, sizeof(tmp_path));
        if (resolved == tmp_path && temp_files) {
            temp_files[i] = joss_web_strdup(tmp_path);
            argv[i + 1] = temp_files[i];
        } else {
            argv[i + 1] = (char*)(resolved ? resolved : "");
        }
    }
    argv[argc + 1] = NULL;

    int pipefd[2];
    if (pipe(pipefd) != 0) {
        free(argv);
        if (temp_files) {
            for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
            free(temp_files);
        }
        return NULL;
    }
    pid_t pid = fork();
    if (pid == 0) {
        close(pipefd[0]);
        dup2(pipefd[1], STDOUT_FILENO);
        dup2(pipefd[1], STDERR_FILENO);
        close(pipefd[1]);
        execvp(prog, argv);
        _exit(127);
    }
    free(argv);
    close(pipefd[1]);
    if (pid < 0) {
        close(pipefd[0]);
        if (temp_files) {
            for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
            free(temp_files);
        }
        return NULL;
    }
    JOSS_AUTO_SB JossStringBuilder* out_sb = joss_sb_new();
    char buf[4096];
    ssize_t nread = 0;
    while ((nread = read(pipefd[0], buf, sizeof(buf))) > 0) {
        joss_buf_append_bytes(out_sb, (const uint8_t*)buf, (int64_t)nread);
    }
    close(pipefd[0]);
    int status = 0;
    waitpid(pid, &status, 0);
    if (temp_files) {
        for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
        free(temp_files);
    }
    char* out = joss_sb_to_req_string(out_sb);
    return out;
}
#endif

const char* fn_System_Run(const char* cmd, void* args_arr) {
    cmd = unbox_web_cstr(cmd);
    if (!cmd || !*cmd) return "";
    int64_t raw_args = joss_unbox_value((int64_t)(uintptr_t)args_arr);
    if (raw_args && joss_is_arr_ptr(raw_args)) {
        char* out = joss_system_run_argv(cmd, (void*)(uintptr_t)raw_args);
        return out ? out : "";
    }
    char* out = joss_system_run_bytes(cmd, NULL);
    return out ? out : "";
}

static int append_curl_headers_checked(JossStringBuilder* cmd, void* hdr_map) {
    int has_ct = 0;
    if (!hdr_map || !joss_is_map_ptr((int64_t)(uintptr_t)hdr_map)) return 0;
    int64_t n = joss_map_len(hdr_map);
    for (int64_t i = 0; i < n; i++) {
        const char* k = unbox_web_cstr(joss_map_key_at(hdr_map, i));
        const char* v = unbox_web_cstr((const char*)(uintptr_t)joss_map_val_at(hdr_map, i));
        if (!k || !*k || !v) continue;
        if (strcasecmp(k, "Content-Type") == 0) has_ct = 1;
        joss_sb_append(cmd, " -H \"");
        joss_sb_append(cmd, k);
        joss_sb_append(cmd, ": ");
        for (const char* p = v; *p; p++) {
            if (*p == '"' || *p == '\\') joss_buf_append_byte(cmd, '\\');
            joss_buf_append_byte(cmd, (uint8_t)*p);
        }
        joss_buf_append_byte(cmd, '"');
    }
    return has_ct;
}

static bool joss_http_write_temp_file(const uint8_t* data, size_t len, char* out_path, size_t out_cap) {
    if (!out_path || out_cap < 64) return false;
    static volatile int64_t g_tmp_seq = 0;
#if defined(_WIN32)
    char tmp_dir[MAX_PATH];
    DWORD dlen = GetTempPathA((DWORD)sizeof(tmp_dir), tmp_dir);
    if (dlen == 0 || dlen >= sizeof(tmp_dir)) snprintf(tmp_dir, sizeof(tmp_dir), ".\\");
    DWORD pid = GetCurrentProcessId();
    for (int attempt = 0; attempt < 32; attempt++) {
        LONG seq = (LONG)InterlockedIncrement64(&g_tmp_seq);
        snprintf(out_path, out_cap, "%sjoss_http_%lu_%ld.bin", tmp_dir, (unsigned long)pid, (long)seq);
        HANDLE h = CreateFileA(out_path, GENERIC_WRITE, 0, NULL, CREATE_NEW, FILE_ATTRIBUTE_TEMPORARY, NULL);
        if (h == INVALID_HANDLE_VALUE) continue;
        DWORD written = 0;
        BOOL ok = (len == 0) ? TRUE : WriteFile(h, data, (DWORD)len, &written, NULL);
        CloseHandle(h);
        if (ok && (size_t)written == len) return true;
        remove(out_path);
        return false;
    }
    return false;
#else
    const char* tmp_dir = getenv("TMPDIR");
    if (!tmp_dir || !*tmp_dir) tmp_dir = "/tmp";
    unsigned long pid = (unsigned long)getpid();
    for (int attempt = 0; attempt < 32; attempt++) {
        long long seq = (long long)__sync_add_and_fetch(&g_tmp_seq, 1);
        snprintf(out_path, out_cap, "%s/joss_http_%lu_%lld.bin", tmp_dir, pid, seq);
        int fd = open(out_path, O_WRONLY | O_CREAT | O_EXCL, 0600);
        if (fd < 0) continue;
        size_t off = 0;
        bool ok = true;
        while (off < len) {
            ssize_t w = write(fd, data + off, len - off);
            if (w <= 0) { ok = false; break; }
            off += (size_t)w;
        }
        close(fd);
        if (ok) return true;
        remove(out_path);
        return false;
    }
    return false;
#endif
}

static void append_url_encoded(JossStringBuilder* sb, const char* s) {
    if (!sb || !s) return;
    static const char hex[] = "0123456789ABCDEF";
    for (const unsigned char* p = (const unsigned char*)s; *p; p++) {
        unsigned char c = *p;
        if ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
            c == '-' || c == '_' || c == '.' || c == '~') {
            joss_buf_append_byte(sb, c);
        } else {
            joss_buf_append_byte(sb, '%');
            joss_buf_append_byte(sb, (uint8_t)hex[c >> 4]);
            joss_buf_append_byte(sb, (uint8_t)hex[c & 0x0F]);
        }
    }
}

void* fn_Http_request(const char* method, const char* url, const char* body, void* headers) {
    method = unbox_web_cstr(method);
    url = unbox_web_cstr(url);
    int64_t hdr_raw = joss_unbox_value((int64_t)(uintptr_t)headers);
    headers = (hdr_raw && joss_is_map_ptr(hdr_raw)) ? (void*)(uintptr_t)hdr_raw : NULL;
    void* res_map = joss_map_new();
    joss_map_set_i64(res_map, "status", 0);
    joss_map_set_i64(res_map, "success", (int64_t)(uintptr_t)joss_box_bool(false));
    joss_map_set_str(res_map, "body", "");
    if (!url || !*url) return res_map;
    if (!method || !*method) method = "GET";

    void* opt_map = NULL;
    void* form_map = NULL;
    const uint8_t* bin_body = NULL;
    size_t bin_body_len = 0;
    const char* body_str = NULL;
    int64_t timeout_sec = 30;
    int64_t body_raw = joss_unbox_value((int64_t)(uintptr_t)body);
    if (body_raw && joss_is_map_ptr(body_raw)) {
        opt_map = (void*)(uintptr_t)body_raw;
        if (joss_array_key_exists("body", opt_map)) {
            int64_t bv = joss_map_get_i64(opt_map, "body");
            bin_body = joss_bytes_unwrap((const void*)(uintptr_t)bv, &bin_body_len);
            if (!bin_body) {
                body_str = unbox_web_cstr((const char*)(uintptr_t)bv);
            }
        }
        if (joss_array_key_exists("form", opt_map)) {
            int64_t fm = joss_unbox_value(joss_map_get_i64(opt_map, "form"));
            if (fm && joss_is_map_ptr(fm)) form_map = (void*)(uintptr_t)fm;
        }
        if (!headers && joss_array_key_exists("headers", opt_map)) {
            int64_t hm = joss_unbox_value(joss_map_get_i64(opt_map, "headers"));
            if (hm && joss_is_map_ptr(hm)) headers = (void*)(uintptr_t)hm;
        }
        if (joss_array_key_exists("timeout", opt_map)) {
            int64_t tv = joss_intval(joss_map_get_i64(opt_map, "timeout"));
            if (tv > 0) timeout_sec = tv;
        }
    } else {
        bin_body = joss_bytes_unwrap(body, &bin_body_len);
        if (!bin_body) {
            body_str = unbox_web_cstr(body);
        }
    }

    JOSS_AUTO_SB JossStringBuilder* cmd = joss_sb_new();
    joss_sb_append(cmd, "curl -sS --compressed --connect-timeout 8 --max-time ");
    joss_sb_append_i64(cmd, timeout_sec);
    joss_sb_append(cmd, " -w \"\\n%{http_code}\" -X \"");
    for (const char* p = method; *p; p++) {
        if (*p == '"' || *p == '\\') joss_buf_append_byte(cmd, '\\');
        joss_buf_append_byte(cmd, (uint8_t)*p);
    }
    joss_buf_append_byte(cmd, '"');
    int has_ct = append_curl_headers_checked(cmd, headers);

    char tmp_body_path[512];
    tmp_body_path[0] = '\0';

    if (form_map) {
        JOSS_AUTO_SB JossStringBuilder* fb = joss_sb_new();
        int64_t fn = joss_map_len(form_map);
        for (int64_t i = 0; i < fn; i++) {
            const char* fk = unbox_web_cstr(joss_map_key_at(form_map, i));
            const char* fv = unbox_web_cstr(joss_strval(joss_map_val_at(form_map, i)));
            if (!fk || !*fk) continue;
            if (fb->len > 0) joss_buf_append_byte(fb, '&');
            append_url_encoded(fb, fk);
            joss_buf_append_byte(fb, '=');
            if (fv) append_url_encoded(fb, fv);
        }
        char* form_str = joss_sb_to_string(fb);
        size_t flen = (size_t)fb->len;
        if (form_str) {
            if (!has_ct) {
                joss_sb_append(cmd, " -H \"Content-Type: application/x-www-form-urlencoded\"");
            }
            if (joss_http_write_temp_file((const uint8_t*)form_str, flen, tmp_body_path, sizeof(tmp_body_path))) {
                joss_sb_append(cmd, " --data-binary \"@");
                joss_sb_append(cmd, tmp_body_path);
                joss_buf_append_byte(cmd, '"');
            }
            free(form_str);
        }
    } else if (bin_body || (body_str && *body_str)) {
        const uint8_t* ptr = bin_body ? bin_body : (const uint8_t*)body_str;
        size_t plen = bin_body ? bin_body_len : strlen(body_str);
        if (!has_ct) {
            joss_sb_append(cmd, bin_body ? " -H \"Content-Type: application/octet-stream\"" : " -H \"Content-Type: application/json\"");
        }
        if (joss_http_write_temp_file(ptr, plen, tmp_body_path, sizeof(tmp_body_path))) {
            joss_sb_append(cmd, " --data-binary \"@");
            joss_sb_append(cmd, tmp_body_path);
            joss_buf_append_byte(cmd, '"');
        }
    }

    joss_sb_append(cmd, " \"");
    for (const char* p = url; *p; p++) {
        if (*p == '"' || *p == '\\') joss_buf_append_byte(cmd, '\\');
        joss_buf_append_byte(cmd, (uint8_t)*p);
    }
    joss_buf_append_byte(cmd, '"');
    char* cmd_str = joss_sb_to_string(cmd);

    size_t raw_len = 0;
    char* raw = joss_system_run_bytes(cmd_str, &raw_len);
    free(cmd_str);
    if (tmp_body_path[0] != '\0') {
        remove(tmp_body_path);
    }
    if (!raw || raw_len == 0) {
        return res_map;
    }

    size_t nl_pos = raw_len;
    while (nl_pos > 0 && raw[nl_pos - 1] != '\n') {
        nl_pos--;
    }
    if (nl_pos > 0) {
        int64_t code = atoll(raw + nl_pos);
        joss_map_set_i64(res_map, "status", code);
        joss_map_set_i64(res_map, "success", (int64_t)(uintptr_t)joss_box_bool(code >= 200 && code < 300));
        size_t blen = nl_pos - 1;
        if (memchr(raw, '\0', blen) != NULL) {
            void* boxed = joss_bytes_new(raw, blen);
            joss_map_set_i64(res_map, "body", (int64_t)(uintptr_t)boxed);
        } else {
            char* bcopy = (char*)joss_req_alloc(blen + 1);
            if (bcopy) {
                memcpy(bcopy, raw, blen);
                bcopy[blen] = '\0';
                joss_map_set_str(res_map, "body", bcopy);
            }
        }
    } else {
        joss_map_set_i64(res_map, "status", 200);
        joss_map_set_str(res_map, "body", raw);
    }
    return res_map;
}

const char* fn_Http_get(const char* url, void* headers) {
    void* res = fn_Http_request("GET", url, "", headers);
    return joss_map_get_str(res, "body");
}

const char* fn_Http_post(const char* url, const char* body, void* headers) {
    void* res = fn_Http_request("POST", url, body, headers);
    return joss_map_get_str(res, "body");
}

