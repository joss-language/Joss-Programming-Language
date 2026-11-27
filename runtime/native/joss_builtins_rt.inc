/* =========================================================================
 * Phase 1: Native Global Builtins & Base Host Classes
 * ========================================================================= */

#if !defined(_WIN32)
#include <sys/stat.h>
#include <sys/time.h>
#endif

static char* joss_web_strdup(const char* s) {
    if (!s) s = "";
    size_t n = strlen(s);
    char* out = (char*)malloc(n + 1);
    if (!out) return "";
    memcpy(out, s, n + 1);
    return out;
}

static double joss_i64_to_double_bits(int64_t bits) {
    double d = 0.0;
    memcpy(&d, &bits, sizeof(double));
    return d;
}

static int64_t joss_double_to_i64_bits(double d) {
    int64_t bits = 0;
    memcpy(&bits, &d, sizeof(double));
    return bits;
}

int64_t joss_microtime(void) {
#if defined(_WIN32)
    FILETIME ft;
    GetSystemTimeAsFileTime(&ft);
    ULARGE_INTEGER uli;
    uli.LowPart = ft.dwLowDateTime;
    uli.HighPart = ft.dwHighDateTime;
    double sec = (double)(uli.QuadPart - 116444736000000000ULL) / 10000000.0;
    return joss_double_to_i64_bits(sec);
#else
    struct timeval tv;
    gettimeofday(&tv, NULL);
    double sec = (double)tv.tv_sec + (double)tv.tv_usec / 1000000.0;
    return joss_double_to_i64_bits(sec);
#endif
}

int64_t joss_sleep(int64_t seconds) {
    if (seconds <= 0) return 0;
#if defined(_WIN32)
    Sleep((DWORD)(seconds * 1000));
#else
    sleep((unsigned int)seconds);
#endif
    return 0;
}

int64_t fn_System_sleep(int64_t seconds) {
    joss_sleep(joss_unbox_value(seconds));
    return 1;
}

const char* fn_System_env(const char* key, int64_t defVal) {
    key = unbox_web_cstr(key);
    const char* def_str = "";
    if (joss_is_not_null(defVal)) {
        const char* s = joss_strval(defVal);
        if (s) def_str = s;
    }
    return joss_env(key, def_str);
}

void fn_System_log(int64_t msg) {
    const char* s = joss_strval(msg);
    printf("[System Log] %s\n", s ? s : "");
    fflush(stdout);
}

const char* fn_System_now(int64_t days_offset) {
    int64_t offset = joss_is_not_null(days_offset) ? joss_intval(days_offset) : 0;
    time_t now = time(NULL);
    struct tm tm_val;
#if defined(_WIN32)
    localtime_s(&tm_val, &now);
#else
    localtime_r(&now, &tm_val);
#endif
    if (offset != 0) {
        tm_val.tm_mday += (int)offset;
        tm_val.tm_isdst = -1;
        mktime(&tm_val);
    }
    char buf[80];
    snprintf(buf, sizeof(buf), "%04d-%02d-%02d %02d:%02d:%02d",
             tm_val.tm_year + 1900, tm_val.tm_mon + 1, tm_val.tm_mday,
             tm_val.tm_hour, tm_val.tm_min, tm_val.tm_sec);
    return joss_req_strdup(buf);
}

#if !defined(_WIN32)
static char* joss_proc_strdup_heap(const char* s) {
    if (!s) s = "";
    size_t n = strlen(s);
    char* out = (char*)malloc(n + 1);
    if (!out) return NULL;
    memcpy(out, s, n + 1);
    return out;
}
#else
static void joss_win_append_quoted_arg(JossStringBuilder* sb, const char* arg) {
    if (!arg || !*arg) {
        joss_sb_append(sb, "\"\"");
        return;
    }
    bool needs_quote = false;
    for (const char* p = arg; *p; p++) {
        if (*p == ' ' || *p == '\t' || *p == '\n' || *p == '\v' || *p == '"') {
            needs_quote = true;
            break;
        }
    }
    if (!needs_quote) {
        joss_sb_append(sb, arg);
        return;
    }
    joss_buf_append_byte(sb, '"');
    for (const char* p = arg; ; ) {
        size_t bs = 0;
        while (*p == '\\') {
            bs++;
            p++;
        }
        if (*p == '\0') {
            for (size_t k = 0; k < bs * 2U; k++) joss_buf_append_byte(sb, '\\');
            break;
        } else if (*p == '"') {
            for (size_t k = 0; k < bs * 2U + 1U; k++) joss_buf_append_byte(sb, '\\');
            joss_buf_append_byte(sb, '"');
            p++;
        } else {
            for (size_t k = 0; k < bs; k++) joss_buf_append_byte(sb, '\\');
            joss_buf_append_byte(sb, (uint8_t)*p);
            p++;
        }
    }
    joss_buf_append_byte(sb, '"');
}
#endif

void* fn_Process_constructor(void* proc, const char* cmd, void* args) {
    proc = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)proc);
    if (!proc || !joss_is_map_ptr((int64_t)(uintptr_t)proc)) {
        proc = joss_obj_new("Process");
    }
    cmd = unbox_web_cstr(cmd);
    void* args_arr = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)args);
    joss_obj_set_field_str(proc, "_cmd", cmd ? joss_web_strdup(cmd) : "");
    joss_obj_set_field_i64(proc, "_args", (args_arr && joss_is_arr_ptr((int64_t)(uintptr_t)args_arr)) ? (int64_t)(uintptr_t)args_arr : 0);
    joss_obj_set_field_i64(proc, "_started", 0);
    joss_obj_set_field_i64(proc, "_pid", 0);
    joss_obj_set_field_i64(proc, "_handle", 0);
    joss_obj_set_field_i64(proc, "_waited", 0);
    joss_obj_set_field_i64(proc, "_exit_code", -1);
    return proc;
}

int64_t fn_Process_start(void* proc) {
    proc = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)proc);
    if (!proc || !joss_is_map_ptr((int64_t)(uintptr_t)proc)) return 0;
    if (joss_obj_get_field_i64(proc, "_started") != 0) return 0;
    const char* cmd = joss_obj_get_field_str(proc, "_cmd");
    if (!cmd || !cmd[0]) return 0;
    void* args_arr = (void*)(uintptr_t)joss_obj_get_field_i64(proc, "_args");
    int64_t nargs = (args_arr && joss_is_arr_ptr((int64_t)(uintptr_t)args_arr)) ? joss_arr_len(args_arr) : 0;

#if defined(_WIN32)
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    joss_win_append_quoted_arg(sb, cmd);
    for (int64_t i = 0; i < nargs; i++) {
        joss_buf_append_byte(sb, ' ');
        const char* a = joss_strval(joss_arr_get_i64(args_arr, i));
        joss_win_append_quoted_arg(sb, a ? a : "");
    }
    char* cmdline = joss_sb_to_string(sb);
    if (!cmdline) return 0;

    STARTUPINFOA si;
    PROCESS_INFORMATION pi;
    memset(&si, 0, sizeof(si));
    si.cb = (DWORD)sizeof(si);
    memset(&pi, 0, sizeof(pi));
    BOOL ok = CreateProcessA(NULL, cmdline, NULL, NULL, FALSE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi);
    free(cmdline);
    if (!ok) return 0;
    CloseHandle(pi.hThread);
    joss_obj_set_field_i64(proc, "_started", 1);
    joss_obj_set_field_i64(proc, "_pid", (int64_t)pi.dwProcessId);
    joss_obj_set_field_i64(proc, "_handle", (int64_t)(uintptr_t)pi.hProcess);
    return 1;
#elif defined(__unix__) || defined(__APPLE__)
    char** argv = (char**)calloc((size_t)nargs + 2U, sizeof(char*));
    if (!argv) return 0;
    argv[0] = joss_proc_strdup_heap(cmd);
    if (!argv[0]) { free(argv); return 0; }
    for (int64_t i = 0; i < nargs; i++) {
        const char* a = joss_strval(joss_arr_get_i64(args_arr, i));
        argv[i + 1] = joss_proc_strdup_heap(a ? a : "");
        if (!argv[i + 1]) {
            for (int64_t j = 0; j <= i; j++) free(argv[j]);
            free(argv);
            return 0;
        }
    }
    argv[nargs + 1] = NULL;

    int err_pipe[2] = {-1, -1};
    if (pipe(err_pipe) == 0) {
        (void)fcntl(err_pipe[0], F_SETFD, FD_CLOEXEC);
        (void)fcntl(err_pipe[1], F_SETFD, FD_CLOEXEC);
    }
    pid_t pid = fork();
    if (pid < 0) {
        if (err_pipe[0] >= 0) { close(err_pipe[0]); close(err_pipe[1]); }
        for (int64_t i = 0; i <= nargs; i++) free(argv[i]);
        free(argv);
        return 0;
    }
    if (pid == 0) {
        if (err_pipe[0] >= 0) close(err_pipe[0]);
        execvp(cmd, argv);
        int err_val = errno;
        if (err_pipe[1] >= 0) {
            ssize_t wr = write(err_pipe[1], &err_val, sizeof(err_val));
            (void)wr;
            close(err_pipe[1]);
        }
        _exit(127);
    }
    for (int64_t i = 0; i <= nargs; i++) free(argv[i]);
    free(argv);
    if (err_pipe[1] >= 0) close(err_pipe[1]);
    if (err_pipe[0] >= 0) {
        int child_err = 0;
        ssize_t rd = 0;
        do {
            rd = read(err_pipe[0], &child_err, sizeof(child_err));
        } while (rd < 0 && errno == EINTR);
        close(err_pipe[0]);
        if (rd > 0) {
            int st = 0;
            while (waitpid(pid, &st, 0) < 0 && errno == EINTR) {}
            return 0;
        }
    }
    joss_obj_set_field_i64(proc, "_started", 1);
    joss_obj_set_field_i64(proc, "_pid", (int64_t)pid);
    return 1;
#else
    (void)nargs;
    return 0;
#endif
}

int64_t fn_Process_wait(void* proc) {
    proc = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)proc);
    if (!proc || !joss_is_map_ptr((int64_t)(uintptr_t)proc)) return -1;
    if (joss_obj_get_field_i64(proc, "_started") == 0) return -1;
    if (joss_obj_get_field_i64(proc, "_waited") != 0) {
        return joss_obj_get_field_i64(proc, "_exit_code");
    }
#if defined(_WIN32)
    HANDLE hProc = (HANDLE)(uintptr_t)joss_obj_get_field_i64(proc, "_handle");
    if (!hProc) return -1;
    WaitForSingleObject(hProc, INFINITE);
    DWORD ec = 0;
    int64_t exit_code = GetExitCodeProcess(hProc, &ec) ? (int64_t)(int32_t)ec : -1;
    CloseHandle(hProc);
    joss_obj_set_field_i64(proc, "_handle", 0);
    joss_obj_set_field_i64(proc, "_waited", 1);
    joss_obj_set_field_i64(proc, "_exit_code", exit_code);
    return exit_code;
#elif defined(__unix__) || defined(__APPLE__)
    pid_t pid = (pid_t)joss_obj_get_field_i64(proc, "_pid");
    if (pid <= 0) return -1;
    int status = 0;
    while (waitpid(pid, &status, 0) < 0) {
        if (errno != EINTR) return -1;
    }
    int64_t exit_code = -1;
    if (WIFEXITED(status)) {
        exit_code = (int64_t)WEXITSTATUS(status);
    }
    joss_obj_set_field_i64(proc, "_waited", 1);
    joss_obj_set_field_i64(proc, "_exit_code", exit_code);
    return exit_code;
#else
    return -1;
#endif
}

int64_t fn_Process_kill(void* proc) {
    proc = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)proc);
    if (!proc || !joss_is_map_ptr((int64_t)(uintptr_t)proc)) return 0;
    if (joss_obj_get_field_i64(proc, "_started") == 0) return 0;
#if defined(_WIN32)
    HANDLE hProc = (HANDLE)(uintptr_t)joss_obj_get_field_i64(proc, "_handle");
    if (!hProc) return 0;
    return TerminateProcess(hProc, 1) ? 1 : 0;
#elif defined(__unix__) || defined(__APPLE__)
    if (joss_obj_get_field_i64(proc, "_waited") != 0) return 0;
    pid_t pid = (pid_t)joss_obj_get_field_i64(proc, "_pid");
    if (pid <= 0) return 0;
    return (kill(pid, SIGKILL) == 0) ? 1 : 0;
#else
    return 0;
#endif
}

int64_t fn_Process_pid(void* proc) {
    proc = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)proc);
    if (!proc || !joss_is_map_ptr((int64_t)(uintptr_t)proc)) return 0;
    return joss_obj_get_field_i64(proc, "_pid");
}

int64_t joss_usleep(int64_t usec) {
    if (usec <= 0) return 0;
#if defined(_WIN32)
    Sleep((DWORD)((usec + 999) / 1000));
#else
    usleep((useconds_t)usec);
#endif
    return 0;
}

int64_t joss_is_dir(const char* path) {
    path = unbox_web_cstr(path);
    if (!path || !path[0]) return 0;
#if defined(_WIN32)
    DWORD attr = GetFileAttributesA(path);
    return (attr != INVALID_FILE_ATTRIBUTES && (attr & FILE_ATTRIBUTE_DIRECTORY)) ? 1 : 0;
#else
    struct stat st;
    if (stat(path, &st) != 0) return 0;
    return S_ISDIR(st.st_mode) ? 1 : 0;
#endif
}

int64_t joss_is_file(const char* path) {
    path = unbox_web_cstr(path);
    if (!path || !path[0]) return 0;
#if defined(_WIN32)
    DWORD attr = GetFileAttributesA(path);
    return (attr != INVALID_FILE_ATTRIBUTES && !(attr & FILE_ATTRIBUTE_DIRECTORY)) ? 1 : 0;
#else
    struct stat st;
    if (stat(path, &st) != 0) return 0;
    return S_ISREG(st.st_mode) ? 1 : 0;
#endif
}

static uint64_t g_joss_rand_state = 0;

int64_t joss_rand(int64_t min_val, int64_t max_val) {
    joss_global_lock(JOSS_LOCK_RAND);
    if (g_joss_rand_state == 0) {
        g_joss_rand_state = (uint64_t)time(NULL) ^ 0x9E3779B97F4A7C15ULL;
    }
    /* xorshift64* */
    uint64_t x = g_joss_rand_state;
    x ^= x >> 12;
    x ^= x << 25;
    x ^= x >> 27;
    g_joss_rand_state = x;
    joss_global_unlock(JOSS_LOCK_RAND);
    uint64_t r = x * 0x2545F4914F6CDD1DULL;
    if (max_val <= min_val) return min_val;
    uint64_t span = (uint64_t)(max_val - min_val) + 1ULL;
    return min_val + (int64_t)(r % span);
}

int64_t joss_ord(const char* s) {
    s = unbox_web_cstr(s);
    if (!s || !s[0]) return 0;
    return (int64_t)(unsigned char)s[0];
}

int64_t joss_floor(int64_t val_bits) {
    double d = joss_i64_to_double_bits(val_bits);
    int64_t i = (int64_t)d;
    if (d < (double)i) i--;
    return joss_double_to_i64_bits((double)i);
}

int64_t joss_ceil(int64_t val_bits) {
    double d = joss_i64_to_double_bits(val_bits);
    int64_t i = (int64_t)d;
    if (d > (double)i) i++;
    return joss_double_to_i64_bits((double)i);
}

int64_t joss_round(int64_t val_bits) {
    double d = joss_i64_to_double_bits(val_bits);
    double r = (d >= 0.0) ? (double)((int64_t)(d + 0.5)) : (double)((int64_t)(d - 0.5));
    return joss_double_to_i64_bits(r);
}

const char* joss_config(const char* key, const char* def) {
    key = unbox_web_cstr(key);
    def = unbox_web_cstr(def);
    if (!key) return def ? def : "";
    const char* v = joss_env(key, def ? def : "");
    return v ? v : (def ? def : "");
}

const char* joss_toon_encode(const char* data) {
    data = unbox_web_cstr(data);
    if (!data) return "";
    return joss_str_concat("TOON:", data);
}

int64_t joss_toon_verify(const char* token) {
    token = unbox_web_cstr(token);
    if (!token) return 0;
    return (strncmp(token, "TOON:", 5) == 0) ? 1 : 0;
}

/* --- Console Host Class --- */
void fn_Console_log(const char* msg) {
    msg = unbox_web_cstr(msg);
    printf("%s\n", msg ? msg : "");
    fflush(stdout);
}

void fn_Console_error(const char* msg) {
    msg = unbox_web_cstr(msg);
    fprintf(stderr, "%s\n", msg ? msg : "");
    fflush(stderr);
}

const char* fn_Console_readLine(void) {
    char buf[4096];
    if (!fgets(buf, sizeof(buf), stdin)) return "";
    size_t n = strlen(buf);
    while (n > 0 && (buf[n - 1] == '\n' || buf[n - 1] == '\r')) buf[--n] = '\0';
    return joss_req_strdup(buf);
}

static const char* joss_console_wrap_ansi(const char* code, int64_t text_raw) {
    if (!code) code = "";
    if (text_raw == 0) {
        return joss_req_strdup(code);
    }
    const char* text = joss_strval(text_raw);
    if (!text) text = "";
    size_t clen = strlen(code);
    size_t tlen = strlen(text);
    const char* reset = "\033[0m";
    size_t rlen = 4;
    char* out = (char*)joss_req_alloc(clen + tlen + rlen + 1);
    if (!out) return text;
    memcpy(out, code, clen);
    memcpy(out + clen, text, tlen);
    memcpy(out + clen + tlen, reset, rlen);
    out[clen + tlen + rlen] = '\0';
    return out;
}

const char* fn_Console_green(int64_t text)   { return joss_console_wrap_ansi("\033[32m", text); }
const char* fn_Console_red(int64_t text)     { return joss_console_wrap_ansi("\033[31m", text); }
const char* fn_Console_yellow(int64_t text)  { return joss_console_wrap_ansi("\033[33m", text); }
const char* fn_Console_blue(int64_t text)    { return joss_console_wrap_ansi("\033[34m", text); }
const char* fn_Console_cyan(int64_t text)    { return joss_console_wrap_ansi("\033[36m", text); }
const char* fn_Console_magenta(int64_t text) { return joss_console_wrap_ansi("\033[35m", text); }
const char* fn_Console_gray(int64_t text)    { return joss_console_wrap_ansi("\033[90m", text); }
const char* fn_Console_bold(int64_t text)    { return joss_console_wrap_ansi("\033[1m", text); }
const char* fn_Console_clear(void)           { return joss_req_strdup("\033[2J\033[H"); }

const char* fn_Console_color(int64_t text_raw, int64_t code_raw) {
    const char* text = text_raw ? joss_strval(text_raw) : "";
    const char* code = code_raw ? joss_strval(code_raw) : "";
    if (!code || !code[0]) return joss_req_strdup(text ? text : "");
    char prefix[64];
    snprintf(prefix, sizeof(prefix), "\033[%sm", code);
    return joss_console_wrap_ansi(prefix, text_raw);
}

/* --- FileStream, StreamReader & StreamWriter Host Classes --- */
#define JOSS_FILESTREAM_MAGIC 0x4A4653545631ULL

typedef struct {
    uint64_t magic;
    FILE*    fp;
    int      owned;
} JossFileStream;

static JossFileStream* joss_fs_unwrap(void* handle) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)handle);
    if (raw >= (int64_t)JOSS_MIN_PTR_ADDR && ((uint64_t)raw >> JOSS_PTR_VA_BITS) == 0) {
        JossFileStream* fs = (JossFileStream*)(uintptr_t)(uint64_t)raw;
        if (fs->magic == JOSS_FILESTREAM_MAGIC) {
            return fs;
        }
    }
    return NULL;
}

static void* joss_fs_wrap(FILE* fp, int owned) {
    if (!fp) return NULL;
    JossFileStream* fs = (JossFileStream*)malloc(sizeof(JossFileStream));
    if (!fs) {
        if (owned) fclose(fp);
        return NULL;
    }
    fs->magic = JOSS_FILESTREAM_MAGIC;
    fs->fp = fp;
    fs->owned = owned;
    return (void*)fs;
}

void* fn_FileStream_open(const char* path, const char* mode) {
    path = unbox_web_cstr(path);
    mode = unbox_web_cstr(mode);
    if (!path || !path[0]) return NULL;
    const char* cmode = "rb";
    if (mode && mode[0]) {
        if (strcmp(mode, "w") == 0) cmode = "wb";
        else if (strcmp(mode, "a") == 0) cmode = "ab";
        else if (strcmp(mode, "r+") == 0) cmode = "r+b";
        else if (strcmp(mode, "w+") == 0) cmode = "w+b";
        else if (strcmp(mode, "a+") == 0) cmode = "a+b";
        else if (strcmp(mode, "r") == 0) cmode = "rb";
        else cmode = mode;
    }
    return joss_fs_wrap(fopen(path, cmode), 1);
}

const char* fn_FileStream_read(void* handle, int64_t max_bytes) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (!fs || !fs->fp) return "";
    max_bytes = joss_unbox_value(max_bytes);
    if (max_bytes <= 0) {
        JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
        uint8_t chunk[4096];
        for (;;) {
            size_t n = fread(chunk, 1, sizeof(chunk), fs->fp);
            if (n > 0) joss_buf_append_bytes(sb, chunk, (int64_t)n);
            if (n < sizeof(chunk)) break;
        }
        char* out = joss_sb_to_req_string(sb);
        return out;
    }
    if (max_bytes > 16 * 1024 * 1024) max_bytes = 16 * 1024 * 1024;
    char* buf = (char*)joss_req_alloc((size_t)max_bytes + 1);
    if (!buf) return "";
    size_t n = fread(buf, 1, (size_t)max_bytes, fs->fp);
    buf[n] = '\0';
    return buf;
}

int64_t fn_FileStream_write(void* handle, const char* data) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    data = unbox_web_cstr(data);
    if (!fs || !fs->fp || !data) return 0;
    size_t n = strlen(data);
    size_t w = fwrite(data, 1, n, fs->fp);
    fflush(fs->fp);
    return (int64_t)w;
}

int64_t fn_FileStream_seek(void* handle, int64_t offset, int64_t whence) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (!fs || !fs->fp) return -1;
    int w = SEEK_SET;
    int64_t wv = joss_unbox_value(whence);
    if (wv == 1) w = SEEK_CUR;
    else if (wv == 2) w = SEEK_END;
    if (fseek(fs->fp, (long)joss_unbox_value(offset), w) != 0) return -1;
    return (int64_t)ftell(fs->fp);
}

int64_t fn_FileStream_size(void* handle) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (!fs || !fs->fp) return 0;
    long cur = ftell(fs->fp);
    if (fseek(fs->fp, 0, SEEK_END) != 0) return 0;
    long sz = ftell(fs->fp);
    if (cur >= 0) fseek(fs->fp, cur, SEEK_SET);
    return sz >= 0 ? (int64_t)sz : 0;
}

void fn_FileStream_flush(void* handle) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (fs && fs->fp) {
        fflush(fs->fp);
    }
}

void fn_FileStream_close(void* handle) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (fs && fs->fp) {
        if (fs->owned) fclose(fs->fp);
        fs->fp = NULL;
    }
}

void* fn_StreamReader_open(void* source) {
    JossFileStream* existing = joss_fs_unwrap(source);
    if (existing && existing->fp) {
        return joss_fs_wrap(existing->fp, 0);
    }
    const char* path = unbox_web_cstr((const char*)source);
    if (!path || !path[0]) return NULL;
    return joss_fs_wrap(fopen(path, "rb"), 1);
}

const char* fn_StreamReader_readLine(void* handle) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (!fs || !fs->fp) return NULL;
    char buf[4096];
    if (!fgets(buf, (int)sizeof(buf), fs->fp)) {
        return NULL;
    }
    size_t len = strlen(buf);
    if (len > 0 && buf[len - 1] == '\n') {
        len--;
        if (len > 0 && buf[len - 1] == '\r') len--;
        char* out = (char*)joss_req_alloc(len + 1);
        if (!out) return "";
        memcpy(out, buf, len);
        out[len] = '\0';
        return out;
    }
    if (feof(fs->fp)) {
        if (len > 0 && buf[len - 1] == '\r') len--;
        char* out = (char*)joss_req_alloc(len + 1);
        if (!out) return "";
        memcpy(out, buf, len);
        out[len] = '\0';
        return out;
    }
    /* Long line (> 4095 bytes): accumulate remaining chunks */
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, buf);
    while (fgets(buf, (int)sizeof(buf), fs->fp)) {
        size_t clen = strlen(buf);
        int done = (clen > 0 && buf[clen - 1] == '\n');
        if (done) buf[--clen] = '\0';
        joss_sb_append(sb, buf);
        if (done) break;
    }
    if (sb->len > 0 && sb->data[sb->len - 1] == '\r') {
        sb->len--;
        sb->data[sb->len] = '\0';
    }
    char* out = joss_sb_to_req_string(sb);
    return out;
}

const char* fn_StreamReader_readToEnd(void* handle) {
    return fn_FileStream_read(handle, 0);
}

void fn_StreamReader_close(void* handle) {
    fn_FileStream_close(handle);
}

void* fn_StreamWriter_open(void* dest, int64_t append) {
    JossFileStream* existing = joss_fs_unwrap(dest);
    if (existing && existing->fp) {
        return joss_fs_wrap(existing->fp, 0);
    }
    const char* path = unbox_web_cstr((const char*)dest);
    if (!path || !path[0]) return NULL;
    const char* cmode = (joss_unbox_value(append) != 0) ? "ab" : "wb";
    return joss_fs_wrap(fopen(path, cmode), 1);
}

int64_t fn_StreamWriter_write(void* handle, const char* text) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    text = unbox_web_cstr(text);
    if (!fs || !fs->fp || !text) return 0;
    size_t n = strlen(text);
    return (int64_t)fwrite(text, 1, n, fs->fp);
}

int64_t fn_StreamWriter_writeLine(void* handle, const char* text) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    text = unbox_web_cstr(text);
    if (!fs || !fs->fp) return 0;
    size_t n = text ? strlen(text) : 0;
    size_t w = n > 0 ? fwrite(text, 1, n, fs->fp) : 0;
    w += fwrite("\n", 1, 1, fs->fp);
    return (int64_t)w;
}

void fn_StreamWriter_flush(void* handle) {
    fn_FileStream_flush(handle);
}

void fn_StreamWriter_close(void* handle) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (fs && fs->fp) {
        fflush(fs->fp);
        if (fs->owned) fclose(fs->fp);
        fs->fp = NULL;
    }
}

/* --- DateTime & DateInterval Host Classes --- */
#define JOSS_DATETIME_MAGIC     0x4A44544D5631ULL
#define JOSS_DATEINTERVAL_MAGIC 0x4A44494E5631ULL

typedef struct {
    uint64_t magic;
    int64_t  unix_sec;
} JossDateTime;

typedef struct {
    uint64_t magic;
    int64_t  total_sec;
} JossDateInterval;

static void* joss_dt_new(int64_t unix_sec) {
    JossDateTime* dt = (JossDateTime*)joss_req_alloc(sizeof(JossDateTime));
    if (!dt) return NULL;
    dt->magic = JOSS_DATETIME_MAGIC;
    dt->unix_sec = unix_sec;
    return (void*)dt;
}

static void* joss_di_new(int64_t total_sec) {
    JossDateInterval* di = (JossDateInterval*)joss_req_alloc(sizeof(JossDateInterval));
    if (!di) return NULL;
    di->magic = JOSS_DATEINTERVAL_MAGIC;
    di->total_sec = total_sec;
    return (void*)di;
}

static int64_t joss_dt_extract_sec(void* self) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)self);
    if (raw >= (int64_t)JOSS_MIN_PTR_ADDR && ((uint64_t)raw >> JOSS_PTR_VA_BITS) == 0) {
        const JossDateTime* dt = (const JossDateTime*)(uintptr_t)(uint64_t)raw;
        if (dt->magic == JOSS_DATETIME_MAGIC) {
            return dt->unix_sec;
        }
    }
    return (int64_t)time(NULL);
}

static int64_t joss_di_extract_sec(int64_t val) {
    int64_t raw = joss_unbox_value(val);
    if (raw >= (int64_t)JOSS_MIN_PTR_ADDR && ((uint64_t)raw >> JOSS_PTR_VA_BITS) == 0) {
        const JossDateInterval* di = (const JossDateInterval*)(uintptr_t)(uint64_t)raw;
        if (di->magic == JOSS_DATEINTERVAL_MAGIC) {
            return di->total_sec;
        }
    }
    return raw;
}

static void joss_dt_to_local_tm(int64_t unix_sec, struct tm* out_tm) {
    time_t t = (time_t)unix_sec;
#if defined(_WIN32)
    localtime_s(out_tm, &t);
#else
    localtime_r(&t, out_tm);
#endif
}

void* fn_DateInterval_days(int64_t n) {
    return joss_di_new(joss_unbox_value(n) * 86400LL);
}

void* fn_DateInterval_hours(int64_t n) {
    return joss_di_new(joss_unbox_value(n) * 3600LL);
}

void* fn_DateInterval_minutes(int64_t n) {
    return joss_di_new(joss_unbox_value(n) * 60LL);
}

void* fn_DateInterval_seconds(int64_t n) {
    return joss_di_new(joss_unbox_value(n));
}

int64_t fn_DateInterval_totalSeconds(void* self) {
    return joss_di_extract_sec((int64_t)(uintptr_t)self);
}

void* fn_DateTime_now(void) {
    return joss_dt_new((int64_t)time(NULL));
}

void* fn_DateTime_create(int64_t year, int64_t month, int64_t day) {
    struct tm tm_val;
    memset(&tm_val, 0, sizeof(tm_val));
    tm_val.tm_year = (int)joss_unbox_value(year) - 1900;
    tm_val.tm_mon  = (int)joss_unbox_value(month) - 1;
    tm_val.tm_mday = (int)joss_unbox_value(day);
    tm_val.tm_isdst = -1;
    time_t t = mktime(&tm_val);
    return joss_dt_new((int64_t)t);
}

void* fn_DateTime_parse(const char* str) {
    str = unbox_web_cstr(str);
    if (!str || !str[0]) {
        return joss_dt_new((int64_t)time(NULL));
    }
    int y = 0, mo = 1, d = 1, h = 0, mi = 0, s = 0;
    int tzh = 0, tzm = 0;
    char tzsign = 0;
    if (sscanf(str, "%d-%d-%dT%d:%d:%d%c%d:%d", &y, &mo, &d, &h, &mi, &s, &tzsign, &tzh, &tzm) >= 7 &&
        (tzsign == 'Z' || tzsign == 'z' || tzsign == '+' || tzsign == '-')) {
        struct tm tm_val;
        memset(&tm_val, 0, sizeof(tm_val));
        tm_val.tm_year = y - 1900;
        tm_val.tm_mon  = mo - 1;
        tm_val.tm_mday = d;
        tm_val.tm_hour = h;
        tm_val.tm_min  = mi;
        tm_val.tm_sec  = s;
#if defined(_WIN32)
        int64_t utc_sec = (int64_t)_mkgmtime(&tm_val);
#else
        int64_t utc_sec = (int64_t)timegm(&tm_val);
#endif
        int64_t off = (int64_t)tzh * 3600LL + (int64_t)tzm * 60LL;
        if (tzsign == '+') utc_sec -= off;
        else if (tzsign == '-') utc_sec += off;
        return joss_dt_new(utc_sec);
    }
    if (sscanf(str, "%d-%d-%d %d:%d:%d", &y, &mo, &d, &h, &mi, &s) >= 3 ||
        sscanf(str, "%d-%d-%dT%d:%d:%d", &y, &mo, &d, &h, &mi, &s) >= 3) {
        struct tm tm_val;
        memset(&tm_val, 0, sizeof(tm_val));
        tm_val.tm_year = y - 1900;
        tm_val.tm_mon  = mo - 1;
        tm_val.tm_mday = d;
        tm_val.tm_hour = h;
        tm_val.tm_min  = mi;
        tm_val.tm_sec  = s;
        tm_val.tm_isdst = -1;
        return joss_dt_new((int64_t)mktime(&tm_val));
    }
    return joss_dt_new(joss_strtotime(str));
}

int64_t fn_DateTime_timestamp(void* self) {
    return joss_dt_extract_sec(self);
}

const char* fn_DateTime_format(void* self, const char* fmt) {
    int64_t ts = joss_dt_extract_sec(self);
    fmt = unbox_web_cstr(fmt);
    if (!fmt || !fmt[0]) fmt = "Y-m-d H:i:s";
    struct tm tm_val;
    joss_dt_to_local_tm(ts, &tm_val);
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    char tmp[32];
    for (const char* p = fmt; *p; p++) {
        switch (*p) {
            case 'Y':
                snprintf(tmp, sizeof(tmp), "%04d", tm_val.tm_year + 1900);
                joss_sb_append(sb, tmp);
                break;
            case 'y':
                snprintf(tmp, sizeof(tmp), "%02d", (tm_val.tm_year + 1900) % 100);
                joss_sb_append(sb, tmp);
                break;
            case 'm':
                snprintf(tmp, sizeof(tmp), "%02d", tm_val.tm_mon + 1);
                joss_sb_append(sb, tmp);
                break;
            case 'd':
                snprintf(tmp, sizeof(tmp), "%02d", tm_val.tm_mday);
                joss_sb_append(sb, tmp);
                break;
            case 'H':
                snprintf(tmp, sizeof(tmp), "%02d", tm_val.tm_hour);
                joss_sb_append(sb, tmp);
                break;
            case 'h': {
                int h12 = tm_val.tm_hour % 12;
                if (h12 == 0) h12 = 12;
                snprintf(tmp, sizeof(tmp), "%02d", h12);
                joss_sb_append(sb, tmp);
                break;
            }
            case 'i':
                snprintf(tmp, sizeof(tmp), "%02d", tm_val.tm_min);
                joss_sb_append(sb, tmp);
                break;
            case 's':
                snprintf(tmp, sizeof(tmp), "%02d", tm_val.tm_sec);
                joss_sb_append(sb, tmp);
                break;
            case 'a':
                joss_sb_append(sb, tm_val.tm_hour < 12 ? "am" : "pm");
                break;
            case 'A':
                joss_sb_append(sb, tm_val.tm_hour < 12 ? "AM" : "PM");
                break;
            default:
                joss_buf_append_byte(sb, (uint8_t)*p);
                break;
        }
    }
    char* out = joss_sb_to_req_string(sb);
    return out;
}

const char* fn_DateTime_iso(void* self) {
    int64_t ts = joss_dt_extract_sec(self);
    struct tm loc_tm, utc_tm;
    joss_dt_to_local_tm(ts, &loc_tm);
    time_t t = (time_t)ts;
#if defined(_WIN32)
    gmtime_s(&utc_tm, &t);
#else
    gmtime_r(&t, &utc_tm);
#endif
    utc_tm.tm_isdst = 0;
    int64_t loc_as_utc = 0, utc_as_utc = 0;
#if defined(_WIN32)
    loc_as_utc = (int64_t)_mkgmtime(&loc_tm);
    utc_as_utc = (int64_t)_mkgmtime(&utc_tm);
#else
    loc_as_utc = (int64_t)timegm(&loc_tm);
    utc_as_utc = (int64_t)timegm(&utc_tm);
#endif
    int64_t off_sec = loc_as_utc - utc_as_utc;
    char buf[64];
    if (off_sec == 0) {
        snprintf(buf, sizeof(buf), "%04d-%02d-%02dT%02d:%02d:%02dZ",
            loc_tm.tm_year + 1900, loc_tm.tm_mon + 1, loc_tm.tm_mday,
            loc_tm.tm_hour, loc_tm.tm_min, loc_tm.tm_sec);
    } else {
        char sign = off_sec >= 0 ? '+' : '-';
        int64_t abs_off = off_sec >= 0 ? off_sec : -off_sec;
        int oh = (int)(abs_off / 3600);
        int om = (int)((abs_off % 3600) / 60);
        snprintf(buf, sizeof(buf), "%04d-%02d-%02dT%02d:%02d:%02d%c%02d:%02d",
            loc_tm.tm_year + 1900, loc_tm.tm_mon + 1, loc_tm.tm_mday,
            loc_tm.tm_hour, loc_tm.tm_min, loc_tm.tm_sec, sign, oh, om);
    }
    return joss_req_strdup(buf);
}

int64_t fn_DateTime_year(void* self) {
    struct tm tm_val;
    joss_dt_to_local_tm(joss_dt_extract_sec(self), &tm_val);
    return (int64_t)(tm_val.tm_year + 1900);
}

int64_t fn_DateTime_month(void* self) {
    struct tm tm_val;
    joss_dt_to_local_tm(joss_dt_extract_sec(self), &tm_val);
    return (int64_t)(tm_val.tm_mon + 1);
}

int64_t fn_DateTime_day(void* self) {
    struct tm tm_val;
    joss_dt_to_local_tm(joss_dt_extract_sec(self), &tm_val);
    return (int64_t)tm_val.tm_mday;
}

int64_t fn_DateTime_hour(void* self) {
    struct tm tm_val;
    joss_dt_to_local_tm(joss_dt_extract_sec(self), &tm_val);
    return (int64_t)tm_val.tm_hour;
}

int64_t fn_DateTime_minute(void* self) {
    struct tm tm_val;
    joss_dt_to_local_tm(joss_dt_extract_sec(self), &tm_val);
    return (int64_t)tm_val.tm_min;
}

int64_t fn_DateTime_second(void* self) {
    struct tm tm_val;
    joss_dt_to_local_tm(joss_dt_extract_sec(self), &tm_val);
    return (int64_t)tm_val.tm_sec;
}

void* fn_DateTime_add(void* self, int64_t interval_val) {
    int64_t base = joss_dt_extract_sec(self);
    int64_t delta = joss_di_extract_sec(interval_val);
    return joss_dt_new(base + delta);
}

void* fn_DateTime_sub(void* self, int64_t interval_val) {
    int64_t base = joss_dt_extract_sec(self);
    int64_t delta = joss_di_extract_sec(interval_val);
    return joss_dt_new(base - delta);
}

void* fn_DateTime_diff(void* self, void* other) {
    int64_t t1 = joss_dt_extract_sec(self);
    int64_t t2 = joss_dt_extract_sec(other);
    int64_t d = t1 - t2;
    if (d < 0) d = -d;
    return joss_di_new(d);
}

/* --- Lang Host Class --- */
static char g_joss_locale[32] = "es";

const char* fn_Lang_locale(void) {
    joss_global_lock(JOSS_LOCK_GENERAL);
    char* copy = joss_req_strdup(g_joss_locale);
    joss_global_unlock(JOSS_LOCK_GENERAL);
    return copy;
}

int64_t fn_Lang_setLocale(const char* loc) {
    loc = unbox_web_cstr(loc);
    if (!loc || !loc[0]) return 0;
    joss_global_lock(JOSS_LOCK_GENERAL);
    snprintf(g_joss_locale, sizeof(g_joss_locale), "%s", loc);
    joss_global_unlock(JOSS_LOCK_GENERAL);
    return 1;
}

const char* fn_Lang_get(const char* key) {
    key = unbox_web_cstr(key);
    return key ? key : "";
}

/* --- WebSocket Connection Object ($ws) & Event Loop --- */
void* joss_ws_wrap(void* sock_val) {
    void* ws = joss_obj_new("WebSocket");
    int64_t fd = joss_unbox_value((int64_t)(uintptr_t)sock_val);
    joss_map_set_i64(ws, "_sock", (int64_t)(uintptr_t)joss_box_i64(fd));
    joss_map_set_i64(ws, "_closed", (int64_t)(uintptr_t)joss_box_i64(0));
    joss_map_set_i64(ws, "_on_message", 0);
    joss_map_set_i64(ws, "_on_close", 0);
    return ws;
}

static bool joss_ws_obj_get_socket(void* ws_ptr, joss_socket_t* out_sock) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)ws_ptr);
    if (raw && joss_is_map_ptr(raw)) {
        int64_t fd = joss_intval(joss_map_get_i64((void*)(uintptr_t)raw, "_sock"));
        return joss_ws_resolve_socket(fd, out_sock);
    }
    return joss_ws_resolve_socket(raw, out_sock);
}

int64_t joss_ws_obj_send(void* ws_ptr, const char* msg) {
    msg = unbox_web_cstr(msg);
    if (!msg) msg = "";
    joss_socket_t sock;
    if (!joss_ws_obj_get_socket(ws_ptr, &sock)) return 0;
    size_t plen = strlen(msg);
    uint8_t* frame = (uint8_t*)malloc(plen + 16U);
    if (!frame) return 0;
    size_t flen = joss_ws_encode_text_frame(msg, frame, plen + 16U);
    bool ok = (flen > 0) && joss_ws_send_all(sock, frame, flen);
    free(frame);
    return ok ? 1 : 0;
}

int64_t joss_ws_obj_on_message(void* ws_ptr, int64_t cb) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)ws_ptr);
    if (!raw || !joss_is_map_ptr(raw)) return 0;
    joss_map_set_i64((void*)(uintptr_t)raw, "_on_message", cb);
    return 1;
}

int64_t joss_ws_obj_on_close(void* ws_ptr, int64_t cb) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)ws_ptr);
    if (!raw || !joss_is_map_ptr(raw)) return 0;
    joss_map_set_i64((void*)(uintptr_t)raw, "_on_close", cb);
    return 1;
}

int64_t joss_ws_obj_subscribe(void* ws_ptr, const char* channel) {
    channel = unbox_web_cstr(channel);
    if (!channel || !channel[0]) return 0;
    joss_socket_t sock;
    if (!joss_ws_obj_get_socket(ws_ptr, &sock)) return 0;
    joss_ws_hub_subscribe_raw(channel, sock);
    return 1;
}

int64_t joss_ws_obj_close(void* ws_ptr) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)ws_ptr);
    if (raw && joss_is_map_ptr(raw)) {
        joss_map_set_i64((void*)(uintptr_t)raw, "_closed", (int64_t)(uintptr_t)joss_box_i64(1));
    }
    joss_socket_t sock;
    if (joss_ws_obj_get_socket(ws_ptr, &sock)) {
        (void)joss_ws_send_control_frame(sock, 0x88U, NULL, 0U);
        joss_ws_hub_unsubscribe_all(sock);
        joss_ws_close_socket(sock);
    }
    return 1;
}

const char* joss_ws_run_loop(void* ws_ptr) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)ws_ptr);
    void* ws_map = (raw && joss_is_map_ptr(raw)) ? (void*)(uintptr_t)raw : NULL;
    joss_socket_t sock;
    if (!ws_map || !joss_ws_obj_get_socket(ws_map, &sock)) {
        return "__JOSS_WEBSOCKET_HANDLED__";
    }
    int64_t fd = joss_intval(joss_map_get_i64(ws_map, "_sock"));
    while (joss_intval(joss_map_get_i64(ws_map, "_closed")) == 0) {
        int opcode = 0;
        char* payload = NULL;
        size_t plen = 0;
        int rc = joss_ws_read_frame(sock, &opcode, &payload, &plen);
        if (rc != 0) {
            free(payload);
            break;
        }
        if ((opcode == 0x1 || opcode == 0x2) && payload) {
            int64_t cb = joss_map_get_i64(ws_map, "_on_message");
            if (cb != 0) {
                void* fn = joss_closure_prepare_call((void*)(uintptr_t)cb);
                if (fn) {
                    (void)((JossUnaryCb)fn)((int64_t)(uintptr_t)payload);
                    joss_closure_env_tls_reset();
                }
            } else {
                (void)joss_ws_obj_send(ws_map, payload);
            }
        }
        free(payload);
    }
    int64_t close_cb = joss_map_get_i64(ws_map, "_on_close");
    if (close_cb != 0) {
        void* fn = joss_closure_prepare_call((void*)(uintptr_t)close_cb);
        if (fn) {
            ((JossCronCallbackFn)fn)();
            joss_closure_env_tls_reset();
        }
    }
    joss_ws_hub_unsubscribe_all(sock);
    joss_unregister_wrapped_socket((intptr_t)fd);
    if (joss_intval(joss_map_get_i64(ws_map, "_closed")) == 0) {
        joss_ws_close_socket(sock);
    }
    joss_req_state_reset();
    joss_req_arena_reset();
    return "__JOSS_WEBSOCKET_HANDLED__";
}

/* --- Http::json & Schema Host Services --- */
int64_t joss_json_decode(const char* s);

int64_t fn_Http_json(const char* method, const char* url, int64_t payload, void* headers) {
    method = unbox_web_cstr(method);
    url = unbox_web_cstr(url);
    if (!method || !method[0]) method = "POST";
    if (!url || !url[0]) return 0;

    const char* body_json = "";
    if (joss_is_not_null(payload)) {
        if (joss_is_map_ptr(payload) || joss_is_arr_ptr(payload)) {
            body_json = joss_json_encode(payload);
        } else {
            body_json = unbox_web_cstr((const char*)(uintptr_t)payload);
            if (!body_json) body_json = "";
        }
    }

    int64_t hdr_raw = joss_unbox_value((int64_t)(uintptr_t)headers);
    void* hdrs = (hdr_raw && joss_is_map_ptr(hdr_raw)) ? (void*)(uintptr_t)hdr_raw : joss_map_new();
    if (!joss_array_key_exists("Content-Type", hdrs) && !joss_array_key_exists("content-type", hdrs)) {
        joss_map_set_str(hdrs, "Content-Type", "application/json");
    }
    if (!joss_array_key_exists("Accept", hdrs) && !joss_array_key_exists("accept", hdrs)) {
        joss_map_set_str(hdrs, "Accept", "application/json");
    }

    void* resp = fn_Http_request(method, url, body_json, hdrs);
    if (!resp || !joss_is_map_ptr((int64_t)(uintptr_t)resp)) return 0;
    const char* resp_body = joss_map_get_str(resp, "body");
    if (!resp_body || !resp_body[0]) return 0;
    return joss_json_decode(resp_body);
}

static void* joss_schema_db_query(const char* sql, void* bindings) {
    return g_joss_schema_db_query ? g_joss_schema_db_query(sql, bindings) : joss_arr_new(0);
}

static int64_t joss_schema_db_exec(const char* sql, void* bindings) {
    return g_joss_schema_db_exec ? g_joss_schema_db_exec(sql, bindings) : -1;
}

static const char* joss_schema_dialect(void) {
    const char* drv = joss_env("DB", "");
    if (!drv || !*drv) drv = joss_env("DB_MOTOR", "");
    if (!drv || !*drv) drv = joss_env("DB_CONNECTION", "");
    if (!drv || !*drv) drv = joss_env("DB_DRIVER", "");
    if (drv && *drv) {
        if (strcasecmp(drv, "sqlite") == 0 || strcasecmp(drv, "sqlite3") == 0) return "sqlite";
        if (strcasecmp(drv, "pgsql") == 0 || strcasecmp(drv, "postgres") == 0 || strcasecmp(drv, "postgresql") == 0) return "postgres";
        if (strcasecmp(drv, "mysql") == 0 || strcasecmp(drv, "mariadb") == 0) return "mysql";
        if (strcasecmp(drv, "sqlsrv") == 0 || strcasecmp(drv, "mssql") == 0 || strcasecmp(drv, "odbc") == 0) return "sqlsrv";
    }
    const char* db_host = joss_env("DB_HOST", "");
    if (db_host && *db_host) return "mysql";
    const char* db_path = joss_env("DB_DATABASE", "");
    if (!db_path || !*db_path) db_path = joss_env("DB_PATH", "");
    if ((db_path && *db_path) || joss_file_exists("database.sqlite")) return "sqlite";
    return "mysql";
}

static void* joss_bp_add_col(void* bp, const char* name, const char* type_sql, bool is_pk) {
    bp = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)bp);
    name = unbox_web_cstr(name);
    if (!bp || !name || !name[0]) return bp;
    int64_t cols_v = joss_map_get_i64(bp, "_cols");
    void* cols = joss_is_arr_ptr(cols_v) ? (void*)(uintptr_t)cols_v : joss_arr_new(8);
    if (!joss_is_arr_ptr(cols_v)) joss_map_set_i64(bp, "_cols", (int64_t)(uintptr_t)cols);
    void* col = joss_map_new();
    joss_map_set_str(col, "name", joss_web_strdup(name));
    joss_map_set_str(col, "type", joss_web_strdup(type_sql ? type_sql : "TEXT"));
    joss_map_set_i64(col, "pk", is_pk ? 1 : 0);
    joss_map_set_i64(col, "nullable", is_pk ? 0 : 0);
    joss_map_set_i64(col, "unique", 0);
    joss_map_set_str(col, "default", "");
    joss_arr_push_i64(cols, (int64_t)(uintptr_t)col);
    joss_map_set_i64(bp, "_last_col", (int64_t)(uintptr_t)col);
    return bp;
}

void* fn_Blueprint_id(void* bp, const char* name) {
    name = unbox_web_cstr(name);
    if (!name || !name[0]) name = "id";
    const char* d = joss_schema_dialect();
    if (strcmp(d, "sqlite") == 0) return joss_bp_add_col(bp, name, "INTEGER PRIMARY KEY AUTOINCREMENT", true);
    if (strcmp(d, "postgres") == 0) return joss_bp_add_col(bp, name, "SERIAL PRIMARY KEY", true);
    return joss_bp_add_col(bp, name, "INT AUTO_INCREMENT PRIMARY KEY", true);
}
void* fn_Blueprint_increments(void* bp, const char* name) { return fn_Blueprint_id(bp, name); }
void* fn_Blueprint_char(void* bp, const char* name, int64_t len) {
    int64_t n = joss_intval(len);
    if (n <= 0) n = 255;
    char tb[48];
    snprintf(tb, sizeof(tb), "CHAR(%lld)", (long long)n);
    return joss_bp_add_col(bp, name, tb, false);
}
void* fn_Blueprint_string(void* bp, const char* name, int64_t len) {
    int64_t n = joss_intval(len);
    if (n <= 0) n = 255;
    char tb[48];
    snprintf(tb, sizeof(tb), "VARCHAR(%lld)", (long long)n);
    return joss_bp_add_col(bp, name, tb, false);
}
void* fn_Blueprint_integer(void* bp, const char* name) { return joss_bp_add_col(bp, name, "INTEGER", false); }
void* fn_Blueprint_tinyInteger(void* bp, const char* name) { return joss_bp_add_col(bp, name, "SMALLINT", false); }
void* fn_Blueprint_smallInteger(void* bp, const char* name) { return joss_bp_add_col(bp, name, "SMALLINT", false); }
void* fn_Blueprint_mediumInteger(void* bp, const char* name) { return joss_bp_add_col(bp, name, "INTEGER", false); }
void* fn_Blueprint_bigInteger(void* bp, const char* name) { return joss_bp_add_col(bp, name, "BIGINT", false); }
void* fn_Blueprint_unsignedInteger(void* bp, const char* name) { return joss_bp_add_col(bp, name, "INTEGER", false); }
void* fn_Blueprint_unsignedBigInteger(void* bp, const char* name) { return joss_bp_add_col(bp, name, "BIGINT", false); }
void* fn_Blueprint_text(void* bp, const char* name) { return joss_bp_add_col(bp, name, "TEXT", false); }
void* fn_Blueprint_mediumText(void* bp, const char* name) {
    const char* d = joss_schema_dialect();
    return joss_bp_add_col(bp, name, (strcmp(d, "sqlite") == 0 || strcmp(d, "postgres") == 0) ? "TEXT" : "MEDIUMTEXT", false);
}
void* fn_Blueprint_longText(void* bp, const char* name) {
    const char* d = joss_schema_dialect();
    return joss_bp_add_col(bp, name, (strcmp(d, "sqlite") == 0 || strcmp(d, "postgres") == 0) ? "TEXT" : "LONGTEXT", false);
}
void* fn_Blueprint_date(void* bp, const char* name) { return joss_bp_add_col(bp, name, "DATE", false); }
void* fn_Blueprint_dateTime(void* bp, const char* name) {
    const char* d = joss_schema_dialect();
    return joss_bp_add_col(bp, name, (strcmp(d, "postgres") == 0) ? "TIMESTAMP" : "DATETIME", false);
}
void* fn_Blueprint_time(void* bp, const char* name) { return joss_bp_add_col(bp, name, "TIME", false); }
void* fn_Blueprint_boolean(void* bp, const char* name) { return joss_bp_add_col(bp, name, "BOOLEAN", false); }
void* fn_Blueprint_json(void* bp, const char* name) {
    const char* d = joss_schema_dialect();
    if (strcmp(d, "postgres") == 0) return joss_bp_add_col(bp, name, "JSONB", false);
    if (strcmp(d, "sqlite") == 0) return joss_bp_add_col(bp, name, "TEXT", false);
    return joss_bp_add_col(bp, name, "JSON", false);
}
void* fn_Blueprint_enum(void* bp, const char* name, void* vals) { (void)vals; return joss_bp_add_col(bp, name, "TEXT", false); }
void* fn_Blueprint_float(void* bp, const char* name) { return joss_bp_add_col(bp, name, "DOUBLE PRECISION", false); }
void* fn_Blueprint_double(void* bp, const char* name) { return joss_bp_add_col(bp, name, "DOUBLE PRECISION", false); }
void* fn_Blueprint_decimal(void* bp, const char* name, int64_t p, int64_t s) {
    int64_t pp = joss_intval(p), ss = joss_intval(s);
    if (pp <= 0) pp = 10;
    if (ss < 0) ss = 2;
    char tb[64];
    snprintf(tb, sizeof(tb), "DECIMAL(%lld,%lld)", (long long)pp, (long long)ss);
    return joss_bp_add_col(bp, name, tb, false);
}
void* fn_Blueprint_timestamp(void* bp, const char* name) { return joss_bp_add_col(bp, name, "TIMESTAMP", false); }
void* fn_Blueprint_timestamps(void* bp) {
    bp = joss_bp_add_col(bp, "created_at", "TIMESTAMP", false);
    return joss_bp_add_col(bp, "updated_at", "TIMESTAMP", false);
}
void* fn_Blueprint_softDeletes(void* bp) {
    bp = joss_bp_add_col(bp, "deleted_at", "TIMESTAMP", false);
    int64_t lv = bp ? joss_map_get_i64(bp, "_last_col") : 0;
    if (joss_is_map_ptr(lv)) joss_map_set_i64((void*)(uintptr_t)lv, "nullable", 1);
    return bp;
}
void* fn_Blueprint_nullable(void* bp) {
    bp = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)bp);
    int64_t lv = bp ? joss_map_get_i64(bp, "_last_col") : 0;
    if (joss_is_map_ptr(lv)) joss_map_set_i64((void*)(uintptr_t)lv, "nullable", 1);
    return bp;
}
void* fn_Blueprint_unique(void* bp, int64_t cols) {
    bp = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)bp);
    if (cols == 0) {
        int64_t lv = bp ? joss_map_get_i64(bp, "_last_col") : 0;
        if (joss_is_map_ptr(lv)) joss_map_set_i64((void*)(uintptr_t)lv, "unique", 1);
    }
    return bp;
}
void* fn_Blueprint_default(void* bp, int64_t val) {
    bp = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)bp);
    int64_t lv = bp ? joss_map_get_i64(bp, "_last_col") : 0;
    if (joss_is_map_ptr(lv)) joss_map_set_str((void*)(uintptr_t)lv, "default", joss_web_strdup(joss_strval(val)));
    return bp;
}
void* fn_Blueprint_unsigned(void* bp) { return bp; }
void* fn_Blueprint_comment(void* bp, const char* text) { (void)text; return bp; }
void* fn_Blueprint_dropColumn(void* bp, int64_t cols) { (void)cols; return bp; }
void* fn_Blueprint_renameColumn(void* bp, const char* from_col, const char* to_col) { (void)from_col; (void)to_col; return bp; }
void* fn_Blueprint_index(void* bp, int64_t cols, const char* name) { (void)cols; (void)name; return bp; }
void* fn_Blueprint_uniqueIndex(void* bp, int64_t cols, const char* name) { (void)cols; (void)name; return bp; }
void* fn_Blueprint_dropIndex(void* bp, const char* name) { (void)name; return bp; }
void* fn_Blueprint_foreign(void* bp, int64_t cols, const char* name) { (void)cols; (void)name; return bp; }
void* fn_Blueprint_references(void* bp, int64_t cols) { (void)cols; return bp; }
void* fn_Blueprint_on(void* bp, const char* table) { (void)table; return bp; }
void* fn_Blueprint_onDelete(void* bp, const char* action) { (void)action; return bp; }
void* fn_Blueprint_onUpdate(void* bp, const char* action) { (void)action; return bp; }
void* fn_Blueprint_dropForeign(void* bp, const char* name) { (void)name; return bp; }

static const char* joss_schema_prefixed_table(const char* table, char* buf, size_t cap) {
    if (!table || !table[0]) return "";
    const char* prefix = joss_env("PREFIX", "");
    if (!prefix || !prefix[0]) prefix = joss_env("DB_PREFIX", "");
    if (prefix && prefix[0]) {
        size_t plen = strlen(prefix);
        if (strncmp(table, prefix, plen) != 0) {
            snprintf(buf, cap, "%s%s", prefix, table);
            return buf;
        }
    }
    snprintf(buf, cap, "%s", table);
    return buf;
}

static void joss_schema_append_col_def(JossStringBuilder* sb, void* col) {
    const char* cname = joss_map_get_str(col, "name");
    const char* ctype = joss_map_get_str(col, "type");
    bool is_pk = joss_map_get_i64(col, "pk") != 0;
    bool nullable = joss_map_get_i64(col, "nullable") != 0;
    bool uniq = joss_map_get_i64(col, "unique") != 0;
    const char* defv = joss_map_get_str(col, "default");
    joss_sb_append(sb, cname);
    joss_sb_append(sb, " ");
    joss_sb_append(sb, ctype);
    if (!is_pk) {
        if (!nullable) joss_sb_append(sb, " NOT NULL");
        if (uniq) joss_sb_append(sb, " UNIQUE");
        if (defv && defv[0]) {
            if (ctype && strcmp(ctype, "BOOLEAN") == 0) {
                if (joss_strcasecmp_ascii(defv, "true") == 0 || strcmp(defv, "1") == 0) {
                    joss_sb_append(sb, " DEFAULT 1");
                } else {
                    joss_sb_append(sb, " DEFAULT 0");
                }
            } else {
                joss_sb_append(sb, " DEFAULT '");
                joss_sb_append(sb, defv);
                joss_sb_append(sb, "'");
            }
        }
    }
}

int64_t fn_Schema_create(const char* table, void* callback) {
    table = unbox_web_cstr(table);
    void* cb = joss_closure_prepare_call(callback);
    if (!table || !table[0] || !cb) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    void* bp = joss_obj_new("Blueprint");
    joss_map_set_str(bp, "_table", joss_web_strdup(tbl));
    joss_map_set_i64(bp, "_cols", (int64_t)(uintptr_t)joss_arr_new(8));
    ((void (*)(void*))cb)(bp);
    void* cols = (void*)(uintptr_t)joss_map_get_i64(bp, "_cols");
    int64_t n = cols ? joss_arr_len(cols) : 0;
    if (n <= 0) return 0;
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "CREATE TABLE IF NOT EXISTS ");
    joss_sb_append(sb, tbl);
    joss_sb_append(sb, " (");
    for (int64_t i = 0; i < n; i++) {
        if (i > 0) joss_sb_append(sb, ", ");
        void* col = (void*)(uintptr_t)joss_arr_get_i64(cols, i);
        if (joss_is_map_ptr((int64_t)(uintptr_t)col)) joss_schema_append_col_def(sb, col);
    }
    joss_sb_append(sb, ")");
    char* sql = joss_sb_to_string(sb);
    int64_t rc = joss_schema_db_exec(sql, joss_arr_new(0));
    free(sql);
    return rc >= 0 ? 1 : 0;
}

int64_t fn_Schema_table(const char* table, void* callback) {
    table = unbox_web_cstr(table);
    void* cb = joss_closure_prepare_call(callback);
    if (!table || !table[0] || !cb) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    void* bp = joss_obj_new("Blueprint");
    joss_map_set_str(bp, "_table", joss_web_strdup(tbl));
    joss_map_set_i64(bp, "_cols", (int64_t)(uintptr_t)joss_arr_new(4));
    ((void (*)(void*))cb)(bp);
    void* cols = (void*)(uintptr_t)joss_map_get_i64(bp, "_cols");
    int64_t n = cols ? joss_arr_len(cols) : 0;
    for (int64_t i = 0; i < n; i++) {
        void* col = (void*)(uintptr_t)joss_arr_get_i64(cols, i);
        if (!joss_is_map_ptr((int64_t)(uintptr_t)col)) continue;
        JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
        joss_sb_append(sb, "ALTER TABLE ");
        joss_sb_append(sb, tbl);
        joss_sb_append(sb, " ADD COLUMN ");
        joss_schema_append_col_def(sb, col);
        char* sql = joss_sb_to_string(sb);
        joss_schema_db_exec(sql, joss_arr_new(0));
        free(sql);
    }
    return 1;
}

int64_t fn_Schema_drop(const char* table) {
    table = unbox_web_cstr(table);
    if (!table || !table[0]) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    char sql[256];
    snprintf(sql, sizeof(sql), "DROP TABLE %s", tbl);
    return joss_schema_db_exec(sql, joss_arr_new(0)) >= 0 ? 1 : 0;
}

int64_t fn_Schema_dropIfExists(const char* table) {
    table = unbox_web_cstr(table);
    if (!table || !table[0]) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    char sql[256];
    snprintf(sql, sizeof(sql), "DROP TABLE IF EXISTS %s", tbl);
    return joss_schema_db_exec(sql, joss_arr_new(0)) >= 0 ? 1 : 0;
}

int64_t fn_Schema_rename(const char* from, const char* to) {
    from = unbox_web_cstr(from);
    to = unbox_web_cstr(to);
    if (!from || !from[0] || !to || !to[0]) return 0;
    char full_from[192], full_to[192];
    const char* f = joss_schema_prefixed_table(from, full_from, sizeof(full_from));
    const char* t = joss_schema_prefixed_table(to, full_to, sizeof(full_to));
    char sql[448];
    snprintf(sql, sizeof(sql), "ALTER TABLE %s RENAME TO %s", f, t);
    return joss_schema_db_exec(sql, joss_arr_new(0)) >= 0 ? 1 : 0;
}

int64_t fn_Schema_hasTable(const char* table) {
    table = unbox_web_cstr(table);
    if (!table || !table[0]) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    const char* d = joss_schema_dialect();
    char sql[256];
    if (strcmp(d, "sqlite") == 0) {
        snprintf(sql, sizeof(sql), "SELECT name FROM sqlite_master WHERE type='table' AND name='%s'", tbl);
    } else if (strcmp(d, "postgres") == 0) {
        snprintf(sql, sizeof(sql), "SELECT table_name FROM information_schema.tables WHERE table_name='%s'", tbl);
    } else {
        snprintf(sql, sizeof(sql), "SHOW TABLES LIKE '%s'", tbl);
    }
    void* rows = joss_schema_db_query(sql, joss_arr_new(0));
    return (rows && joss_arr_len(rows) > 0) ? 1 : 0;
}

int64_t fn_Schema_hasColumn(const char* table, const char* column) {
    table = unbox_web_cstr(table);
    column = unbox_web_cstr(column);
    if (!table || !table[0] || !column || !column[0]) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    const char* d = joss_schema_dialect();
    char sql[256];
    if (strcmp(d, "sqlite") == 0) {
        snprintf(sql, sizeof(sql), "PRAGMA table_info(%s)", tbl);
        void* rows = joss_schema_db_query(sql, joss_arr_new(0));
        int64_t n = rows ? joss_arr_len(rows) : 0;
        for (int64_t i = 0; i < n; i++) {
            void* r = (void*)(uintptr_t)joss_arr_get_i64(rows, i);
            if (joss_is_map_ptr((int64_t)(uintptr_t)r) && strcmp(joss_map_get_str(r, "name"), column) == 0) return 1;
        }
        return 0;
    } else if (strcmp(d, "postgres") == 0) {
        snprintf(sql, sizeof(sql), "SELECT column_name FROM information_schema.columns WHERE table_name='%s' AND column_name='%s'", tbl, column);
    } else {
        snprintf(sql, sizeof(sql), "SHOW COLUMNS FROM %s LIKE '%s'", tbl, column);
    }
    void* rows = joss_schema_db_query(sql, joss_arr_new(0));
    return (rows && joss_arr_len(rows) > 0) ? 1 : 0;
}

int64_t fn_Schema_hasForeign(const char* table, const char* constraint_name) {
    table = unbox_web_cstr(table);
    constraint_name = unbox_web_cstr(constraint_name);
    if (!table || !table[0] || !constraint_name || !constraint_name[0]) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    const char* d = joss_schema_dialect();
    if (strcmp(d, "sqlite") == 0) {
        void* params = joss_arr_new(1);
        joss_arr_push_i64(params, (int64_t)(uintptr_t)tbl);
        void* rows = joss_schema_db_query("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", params);
        if (!rows || joss_arr_len(rows) == 0) return 0;
        void* r = (void*)(uintptr_t)joss_arr_get_i64(rows, 0);
        if (!joss_is_map_ptr((int64_t)(uintptr_t)r)) return 0;
        const char* def = joss_map_get_str(r, "sql");
        if (!def || !def[0]) return 0;
        char pat1[256], pat2[256], pat3[256];
        snprintf(pat1, sizeof(pat1), "CONSTRAINT `%s`", constraint_name);
        snprintf(pat2, sizeof(pat2), "CONSTRAINT \"%s\"", constraint_name);
        snprintf(pat3, sizeof(pat3), "CONSTRAINT %s ", constraint_name);
        return (strstr(def, pat1) || strstr(def, pat2) || strstr(def, pat3)) ? 1 : 0;
    }
    void* params = joss_arr_new(2);
    joss_arr_push_i64(params, (int64_t)(uintptr_t)tbl);
    joss_arr_push_i64(params, (int64_t)(uintptr_t)constraint_name);
    const char* sql = NULL;
    if (strcmp(d, "postgres") == 0) {
        sql = "SELECT constraint_name FROM information_schema.table_constraints WHERE table_schema = current_schema() AND table_name = ? AND constraint_name = ? AND constraint_type = 'FOREIGN KEY'";
    } else if (strcmp(d, "sqlserver") == 0) {
        sql = "SELECT fk.name FROM sys.foreign_keys fk JOIN sys.tables t ON t.object_id = fk.parent_object_id WHERE t.name = ? AND fk.name = ?";
    } else {
        sql = "SELECT constraint_name FROM information_schema.table_constraints WHERE table_schema = DATABASE() AND table_name = ? AND constraint_name = ? AND constraint_type = 'FOREIGN KEY'";
    }
    void* rows = joss_schema_db_query(sql, params);
    return (rows && joss_arr_len(rows) > 0) ? 1 : 0;
}

/* --- Additional C11 Web/Utility Builtins for Full App Linking --- */
void* joss_end(void* arr) {
    if (!arr) return NULL;
    int64_t n = joss_count(arr);
    if (n <= 0) return NULL;
    return (void*)(intptr_t)joss_arr_get_i64(arr, n - 1);
}

int64_t joss_is_numeric(const char* s) {
    s = unbox_web_cstr(s);
    if (!s || !s[0]) return 0;
    char* endp = NULL;
    (void)strtod(s, &endp);
    return (endp && *endp == '\0') ? 1 : 0;
}

const char* joss_html_escape(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    size_t len = strlen(s);
    size_t cap = len * 6 + 1;
    char* out = (char*)joss_req_alloc(cap);
    if (!out) return s;
    size_t j = 0;
    for (size_t i = 0; i < len; i++) {
        switch (s[i]) {
            case '&': memcpy(out + j, "&amp;", 5); j += 5; break;
            case '<': memcpy(out + j, "&lt;", 4); j += 4; break;
            case '>': memcpy(out + j, "&gt;", 4); j += 4; break;
            case '"': memcpy(out + j, "&quot;", 6); j += 6; break;
            case '\'': memcpy(out + j, "&#39;", 5); j += 5; break;
            default: out[j++] = s[i]; break;
        }
    }
    out[j] = '\0';
    return out;
}

void* joss_append(void* arr, void* val) {
    if (!arr) arr = joss_arr_new(4);
    joss_arr_push_i64(arr, (int64_t)(intptr_t)val);
    return arr;
}

int64_t joss_len(void* v) {
    if (!v) return 0;
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)v);
    if (!raw) return 0;
    if (joss_is_arr_ptr(raw) || joss_is_map_ptr(raw)) {
        return joss_count((void*)(uintptr_t)raw);
    }
    const char* s = unbox_web_cstr((const char*)v);
    return s ? (int64_t)strlen(s) : 0;
}

int64_t joss_unlink(const char* path) {
    path = unbox_web_cstr(path);
    if (!path || !path[0]) return 0;
    return (remove(path) == 0) ? 1 : 0;
}

int64_t joss_mkdir(const char* path, int64_t mode, int64_t recursive) {
    (void)mode;
    (void)recursive;
    path = unbox_web_cstr(path);
    if (!path || !path[0]) return 0;
#if defined(_WIN32)
    return CreateDirectoryA(path, NULL) ? 1 : 0;
#else
    return (mkdir(path, 0755) == 0) ? 1 : 0;
#endif
}

int64_t joss_in_array(void* needle, void* haystack) {
    if (!haystack) return 0;
    int64_t n = joss_count(haystack);
    const char* ns = unbox_web_cstr((const char*)needle);
    for (int64_t i = 0; i < n; i++) {
        void* item = (void*)(intptr_t)joss_arr_get_i64(haystack, i);
        if (item == needle) return 1;
        const char* is = unbox_web_cstr((const char*)item);
        if (ns && is && strcmp(ns, is) == 0) return 1;
    }
    return 0;
}

const char* joss_rawurlencode(const char* s);

const char* joss_urlencode(const char* s) {
    return joss_rawurlencode(s);
}

const char* joss_rawurlencode(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    size_t len = strlen(s);
    char* out = (char*)joss_req_alloc(len * 3 + 1);
    if (!out) return s;
    static const char hex[] = "0123456789ABCDEF";
    size_t j = 0;
    for (size_t i = 0; i < len; i++) {
        unsigned char c = (unsigned char)s[i];
        if ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
            c == '-' || c == '_' || c == '.' || c == '~') {
            out[j++] = (char)c;
        } else {
            out[j++] = '%';
            out[j++] = hex[c >> 4];
            out[j++] = hex[c & 0x0F];
        }
    }
    out[j] = '\0';
    return out;
}

const char* joss_gmdate(const char* fmt, int64_t ts) {
    fmt = unbox_web_cstr(fmt);
    time_t t = (ts > 946684800LL && ts < 4102444800LL) ? (time_t)ts : time(NULL);
    struct tm tm_val;
#if defined(_WIN32)
    gmtime_s(&tm_val, &t);
#else
    gmtime_r(&t, &tm_val);
#endif
    char buf[128];
    if (!fmt || !fmt[0] || strcmp(fmt, "Y-m-d H:i:s") == 0) {
        snprintf(buf, sizeof(buf), "%04d-%02d-%02d %02d:%02d:%02d",
            tm_val.tm_year + 1900, tm_val.tm_mon + 1, tm_val.tm_mday,
            tm_val.tm_hour, tm_val.tm_min, tm_val.tm_sec);
        return joss_req_strdup(buf);
    }
    if (strcmp(fmt, "D, d M Y H:i:s") == 0) {
        static const char* days[] = {"Sun","Mon","Tue","Wed","Thu","Fri","Sat"};
        static const char* mons[] = {"Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"};
        snprintf(buf, sizeof(buf), "%s, %02d %s %04d %02d:%02d:%02d",
            days[tm_val.tm_wday % 7], tm_val.tm_mday, mons[tm_val.tm_mon % 12],
            tm_val.tm_year + 1900, tm_val.tm_hour, tm_val.tm_min, tm_val.tm_sec);
        return joss_req_strdup(buf);
    }
    strftime(buf, sizeof(buf), fmt, &tm_val);
    return joss_req_strdup(buf);
}

const char* joss_md5(const char* s) {
    size_t len = 0;
    const uint8_t* ptr = joss_bytes_unwrap(s, &len);
    if (!ptr) {
        s = unbox_web_cstr(s);
        if (!s) s = "";
        ptr = (const uint8_t*)s;
        len = strlen(s);
    }
    uint8_t digest[16];
    joss_prim_md5(ptr, len, digest);
    static const char hex[] = "0123456789abcdef";
    char buf[33];
    for (int i = 0; i < 16; i++) {
        buf[i * 2]     = hex[(digest[i] >> 4) & 0x0F];
        buf[i * 2 + 1] = hex[digest[i] & 0x0F];
    }
    buf[32] = '\0';
    return joss_req_strdup(buf);
}

const char* joss_sha256_base64(const char* s) {
    size_t len = 0;
    const uint8_t* ptr = joss_bytes_unwrap(s, &len);
    if (!ptr) {
        s = unbox_web_cstr(s);
        if (!s) s = "";
        ptr = (const uint8_t*)s;
        len = strlen(s);
    }
    uint8_t digest[32];
    joss_prim_sha256(ptr, len, digest);
    char b64[64];
    joss_prim_base64_encode(digest, sizeof(digest), b64);
    return joss_req_strdup(b64);
}

#if defined(__has_include) && !defined(JOSS_DISABLE_OPENSSL_SIGN)
#if __has_include(<openssl/evp.h>) && __has_include(<openssl/pem.h>)
#define JOSS_HAS_OPENSSL_SIGN 1
#include <openssl/bio.h>
#include <openssl/evp.h>
#include <openssl/pem.h>
JOSS_DEFINE_AUTO_CLEANUP(BIO*, joss_auto_bio, BIO_free)
JOSS_DEFINE_AUTO_CLEANUP(EVP_PKEY*, joss_auto_evp_pkey, EVP_PKEY_free)
JOSS_DEFINE_AUTO_CLEANUP(EVP_MD_CTX*, joss_auto_evp_ctx, EVP_MD_CTX_free)
#define JOSS_AUTO_BIO      JOSS_AUTO(joss_auto_bio)
#define JOSS_AUTO_EVP_PKEY JOSS_AUTO(joss_auto_evp_pkey)
#define JOSS_AUTO_EVP_CTX  JOSS_AUTO(joss_auto_evp_ctx)
#endif
#endif

#if !defined(JOSS_HAS_OPENSSL_SIGN) && defined(_WIN32)
#include <wincrypt.h>
#include <bcrypt.h>
#endif

#define JOSS_RSA_MAX_LIMBS 132

static int joss_rsa_cmp_k(const uint32_t* a, const uint32_t* b, int k) {
    for (int i = k - 1; i >= 0; i--) {
        if (a[i] < b[i]) return -1;
        if (a[i] > b[i]) return 1;
    }
    return 0;
}

static void joss_rsa_sub_k(uint32_t* a, const uint32_t* b, int k) {
    uint64_t borrow = 0;
    for (int i = 0; i < k; i++) {
        uint64_t diff = (uint64_t)a[i] - (uint64_t)b[i] - borrow;
        a[i] = (uint32_t)diff;
        borrow = (diff >> 63) & 1ULL;
    }
}

static uint32_t joss_rsa_mont_n0inv(uint32_t n0) {
    uint32_t x = n0;
    x *= 2U - n0 * x;
    x *= 2U - n0 * x;
    x *= 2U - n0 * x;
    x *= 2U - n0 * x;
    return (uint32_t)(0U - x);
}

static void joss_rsa_mont_mul(uint32_t* out, const uint32_t* a, const uint32_t* b,
                              const uint32_t* n, int k, uint32_t n0inv) {
    uint32_t t[JOSS_RSA_MAX_LIMBS + 2];
    memset(t, 0, (size_t)(k + 2) * sizeof(uint32_t));
    for (int i = 0; i < k; i++) {
        uint64_t carry = 0;
        uint64_t ai = (uint64_t)a[i];
        for (int j = 0; j < k; j++) {
            uint64_t s = (uint64_t)t[j] + ai * (uint64_t)b[j] + carry;
            t[j] = (uint32_t)s;
            carry = s >> 32;
        }
        uint64_t s = (uint64_t)t[k] + carry;
        t[k] = (uint32_t)s;
        t[k + 1] = (uint32_t)(s >> 32);

        uint32_t m = t[0] * n0inv;
        carry = ((uint64_t)t[0] + (uint64_t)m * (uint64_t)n[0]) >> 32;
        for (int j = 1; j < k; j++) {
            uint64_t u = (uint64_t)t[j] + (uint64_t)m * (uint64_t)n[j] + carry;
            t[j - 1] = (uint32_t)u;
            carry = u >> 32;
        }
        uint64_t u = (uint64_t)t[k] + carry;
        t[k - 1] = (uint32_t)u;
        t[k] = t[k + 1] + (uint32_t)(u >> 32);
    }
    if (t[k] != 0 || joss_rsa_cmp_k(t, n, k) >= 0) {
        joss_rsa_sub_k(t, n, k);
    }
    memcpy(out, t, (size_t)k * sizeof(uint32_t));
}

static bool joss_rsa_bytes_to_limbs(const uint8_t* be, size_t len, uint32_t* limbs, int k) {
    memset(limbs, 0, (size_t)k * sizeof(uint32_t));
    while (len > 0 && *be == 0) {
        be++;
        len--;
    }
    if (len > (size_t)k * 4U) return false;
    for (size_t i = 0; i < len; i++) {
        size_t pos = len - 1 - i;
        limbs[pos / 4U] |= ((uint32_t)be[i]) << ((pos % 4U) * 8U);
    }
    return true;
}

static void joss_rsa_limbs_to_bytes(const uint32_t* limbs, int k, uint8_t* out, size_t out_len) {
    memset(out, 0, out_len);
    for (size_t i = 0; i < out_len; i++) {
        size_t pos = out_len - 1 - i;
        int limb_idx = (int)(pos / 4U);
        if (limb_idx < k) {
            out[i] = (uint8_t)((limbs[limb_idx] >> ((pos % 4U) * 8U)) & 0xFFU);
        }
    }
}

static bool joss_asn1_read_tlv(const uint8_t* buf, size_t len, size_t* off,
                               uint8_t* tag, const uint8_t** val, size_t* vlen) {
    if (*off + 2 > len) return false;
    *tag = buf[(*off)++];
    uint8_t lb = buf[(*off)++];
    size_t n = 0;
    if ((lb & 0x80U) == 0) {
        n = lb;
    } else {
        int nb = (int)(lb & 0x7FU);
        if (nb <= 0 || nb > 4 || *off + (size_t)nb > len) return false;
        for (int i = 0; i < nb; i++) {
            n = (n << 8) | buf[(*off)++];
        }
    }
    if (*off + n > len) return false;
    *val = buf + *off;
    *vlen = n;
    *off += n;
    return true;
}

static bool joss_rsa_parse_pem_key(const char* pem,
                                   const uint8_t** out_der_buf,
                                   const uint8_t** out_n, size_t* out_n_len,
                                   const uint8_t** out_d, size_t* out_d_len) {
    static const char B64_STD[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    const char* begin = strstr(pem, "-----BEGIN ");
    if (!begin) return false;
    const char* line_end = strstr(begin, "-----");
    if (!line_end) return false;
    line_end = strstr(line_end + 5, "-----");
    if (!line_end) return false;
    const char* body = line_end + 5;
    const char* end = strstr(body, "-----END ");
    if (!end || end <= body) return false;

    size_t body_len = (size_t)(end - body);
    char* clean = (char*)malloc(body_len + 1);
    if (!clean) return false;
    size_t clen = 0;
    for (size_t i = 0; i < body_len; i++) {
        unsigned char c = (unsigned char)body[i];
        if (c != '\r' && c != '\n' && c != ' ' && c != '\t') {
            clean[clen++] = (char)c;
        }
    }
    clean[clen] = '\0';

    size_t der_cap = (clen * 3U) / 4U + 8U;
    uint8_t* der = (uint8_t*)malloc(der_cap);
    if (!der) {
        free(clean);
        return false;
    }
    size_t der_len = joss_prim_base64_decode_alphabet(clean, clen, B64_STD, der, der_cap);
    free(clean);
    if (der_len < 16) {
        free(der);
        return false;
    }

    size_t off = 0;
    uint8_t tag = 0;
    const uint8_t* seq = NULL;
    size_t seq_len = 0;
    if (!joss_asn1_read_tlv(der, der_len, &off, &tag, &seq, &seq_len) || tag != 0x30U) {
        free(der);
        return false;
    }

    size_t soff = 0;
    const uint8_t* v0 = NULL; size_t v0_len = 0;
    const uint8_t* v1 = NULL; size_t v1_len = 0;
    uint8_t t0 = 0, t1 = 0;
    if (!joss_asn1_read_tlv(seq, seq_len, &soff, &t0, &v0, &v0_len) || t0 != 0x02U ||
        !joss_asn1_read_tlv(seq, seq_len, &soff, &t1, &v1, &v1_len)) {
        free(der);
        return false;
    }

    /* PKCS#8 PrivateKeyInfo: version(INT), algId(SEQ 0x30), privateKey(OCTET STRING 0x04) */
    if (t1 == 0x30U) {
        uint8_t t2 = 0;
        const uint8_t* oct = NULL; size_t oct_len = 0;
        if (!joss_asn1_read_tlv(seq, seq_len, &soff, &t2, &oct, &oct_len) || t2 != 0x04U) {
            free(der);
            return false;
        }
        size_t ioff = 0;
        if (!joss_asn1_read_tlv(oct, oct_len, &ioff, &tag, &seq, &seq_len) || tag != 0x30U) {
            free(der);
            return false;
        }
        soff = 0;
        if (!joss_asn1_read_tlv(seq, seq_len, &soff, &t0, &v0, &v0_len) || t0 != 0x02U ||
            !joss_asn1_read_tlv(seq, seq_len, &soff, &t1, &v1, &v1_len)) {
            free(der);
            return false;
        }
    }

    /* PKCS#1 RSAPrivateKey: version(INT), n(INT), e(INT), d(INT), ... */
    if (t1 != 0x02U) {
        free(der);
        return false;
    }
    uint8_t te = 0, td = 0;
    const uint8_t* ve = NULL; size_t ve_len = 0;
    const uint8_t* vd = NULL; size_t vd_len = 0;
    if (!joss_asn1_read_tlv(seq, seq_len, &soff, &te, &ve, &ve_len) || te != 0x02U ||
        !joss_asn1_read_tlv(seq, seq_len, &soff, &td, &vd, &vd_len) || td != 0x02U) {
        free(der);
        return false;
    }
    while (v1_len > 0 && *v1 == 0) { v1++; v1_len--; }
    while (vd_len > 0 && *vd == 0) { vd++; vd_len--; }
    if (v1_len < 64 || v1_len > 512 || vd_len == 0 || vd_len > v1_len) {
        free(der);
        return false;
    }

    *out_der_buf = der;
    *out_n = v1;
    *out_n_len = v1_len;
    *out_d = vd;
    *out_d_len = vd_len;
    return true;
}

const char* joss_rsa_sign_sha256_pure(const char* data, const char* priv_key) {
    if (!data || !priv_key || !*priv_key) return "";
    const uint8_t* der_buf = NULL;
    const uint8_t* n_be = NULL; size_t n_len = 0;
    const uint8_t* d_be = NULL; size_t d_len = 0;
    if (!joss_rsa_parse_pem_key(priv_key, &der_buf, &n_be, &n_len, &d_be, &d_len)) {
        return "";
    }

    int k = (int)((n_len + 3U) / 4U);
    if (k <= 0 || k > JOSS_RSA_MAX_LIMBS) {
        free((void*)der_buf);
        return "";
    }

    uint32_t mod_n[JOSS_RSA_MAX_LIMBS];
    uint32_t exp_d[JOSS_RSA_MAX_LIMBS];
    if (!joss_rsa_bytes_to_limbs(n_be, n_len, mod_n, k) ||
        !joss_rsa_bytes_to_limbs(d_be, d_len, exp_d, k) ||
        (mod_n[0] & 1U) == 0) {
        free((void*)der_buf);
        return "";
    }
    free((void*)der_buf);

    /* EMSA-PKCS1-v1_5 with SHA-256 DigestInfo (19-byte prefix + 32-byte digest) */
    static const uint8_t SHA256_DER_PREFIX[19] = {
        0x30, 0x31, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01,
        0x65, 0x03, 0x04, 0x02, 0x01, 0x05, 0x00, 0x04, 0x20
    };
    if (n_len < 11U + sizeof(SHA256_DER_PREFIX) + 32U) return "";

    uint8_t digest[32];
    joss_prim_sha256((const uint8_t*)data, strlen(data), digest);

    uint8_t em[512];
    size_t ps_len = n_len - 3U - sizeof(SHA256_DER_PREFIX) - 32U;
    em[0] = 0x00;
    em[1] = 0x01;
    memset(em + 2, 0xFF, ps_len);
    em[2 + ps_len] = 0x00;
    memcpy(em + 3 + ps_len, SHA256_DER_PREFIX, sizeof(SHA256_DER_PREFIX));
    memcpy(em + 3 + ps_len + sizeof(SHA256_DER_PREFIX), digest, 32);

    uint32_t base[JOSS_RSA_MAX_LIMBS];
    if (!joss_rsa_bytes_to_limbs(em, n_len, base, k)) return "";

    uint32_t n0inv = joss_rsa_mont_n0inv(mod_n[0]);

    /* Compute R^2 mod n via repeated doubling */
    uint32_t r2[JOSS_RSA_MAX_LIMBS];
    memset(r2, 0, (size_t)k * sizeof(uint32_t));
    r2[0] = 1U;
    int total_bits = 2 * k * 32;
    for (int bit = 0; bit < total_bits; bit++) {
        uint32_t carry = 0;
        for (int j = 0; j < k; j++) {
            uint32_t next_c = r2[j] >> 31;
            r2[j] = (r2[j] << 1) | carry;
            carry = next_c;
        }
        if (carry || joss_rsa_cmp_k(r2, mod_n, k) >= 0) {
            joss_rsa_sub_k(r2, mod_n, k);
        }
    }

    uint32_t one[JOSS_RSA_MAX_LIMBS];
    memset(one, 0, (size_t)k * sizeof(uint32_t));
    one[0] = 1U;

    uint32_t m_base[JOSS_RSA_MAX_LIMBS];
    uint32_t acc[JOSS_RSA_MAX_LIMBS];
    joss_rsa_mont_mul(m_base, base, r2, mod_n, k, n0inv);
    joss_rsa_mont_mul(acc, one, r2, mod_n, k, n0inv);

    int top_bit = k * 32 - 1;
    while (top_bit > 0 && ((exp_d[top_bit / 32] >> (top_bit % 32)) & 1U) == 0) {
        top_bit--;
    }
    for (int b = top_bit; b >= 0; b--) {
        joss_rsa_mont_mul(acc, acc, acc, mod_n, k, n0inv);
        if ((exp_d[b / 32] >> (b % 32)) & 1U) {
            joss_rsa_mont_mul(acc, acc, m_base, mod_n, k, n0inv);
        }
    }

    uint32_t sig_limbs[JOSS_RSA_MAX_LIMBS];
    joss_rsa_mont_mul(sig_limbs, acc, one, mod_n, k, n0inv);

    uint8_t sig_bytes[512];
    joss_rsa_limbs_to_bytes(sig_limbs, k, sig_bytes, n_len);

    size_t b64_len = ((n_len + 2U) / 3U) * 4U + 1U;
    char* b64_out = (char*)joss_req_alloc(b64_len);
    if (!b64_out) return "";
    joss_prim_base64_encode(sig_bytes, n_len, b64_out);
    return b64_out;
}

const char* joss_rsa_sign_sha256(const char* data, const char* priv_key) {
    data = unbox_web_cstr(data);
    priv_key = unbox_web_cstr(priv_key);
    if (!data || !priv_key || !*priv_key) return "";
#ifdef JOSS_HAS_OPENSSL_SIGN
    JOSS_AUTO_BIO BIO* bio = BIO_new_mem_buf(priv_key, -1);
    if (!bio) return "";
    JOSS_AUTO_EVP_PKEY EVP_PKEY* pkey = PEM_read_bio_PrivateKey(bio, NULL, NULL, NULL);
    if (!pkey) return "";
    JOSS_AUTO_EVP_CTX EVP_MD_CTX* ctx = EVP_MD_CTX_new();
    if (!ctx) return "";
    size_t sig_len = 0;
    char* b64_out = NULL;
    if (EVP_DigestSignInit(ctx, NULL, EVP_sha256(), NULL, pkey) == 1 &&
        EVP_DigestSignUpdate(ctx, data, strlen(data)) == 1 &&
        EVP_DigestSignFinal(ctx, NULL, &sig_len) == 1 && sig_len > 0) {
        JOSS_AUTO_PTR unsigned char* sig = (unsigned char*)malloc(sig_len);
        if (sig && EVP_DigestSignFinal(ctx, sig, &sig_len) == 1) {
            size_t b64_len = ((sig_len + 2) / 3) * 4 + 1;
            b64_out = (char*)joss_req_alloc(b64_len);
            if (b64_out) {
                int out_n = EVP_EncodeBlock((unsigned char*)b64_out, sig, (int)sig_len);
                b64_out[out_n >= 0 ? out_n : 0] = '\0';
            }
        }
    }
    return b64_out ? b64_out : "";
#elif defined(_WIN32)
    DWORD der_len = 0;
    if (!CryptStringToBinaryA(priv_key, (DWORD)strlen(priv_key), CRYPT_STRING_BASE64HEADER, NULL, &der_len, NULL, NULL)) {
        return "";
    }
    BYTE* der = (BYTE*)malloc(der_len);
    if (!der) return "";
    if (!CryptStringToBinaryA(priv_key, (DWORD)strlen(priv_key), CRYPT_STRING_BASE64HEADER, der, &der_len, NULL, NULL)) {
        free(der);
        return "";
    }
    BYTE* rsa_blob = NULL;
    DWORD rsa_blob_len = 0;
    CRYPT_PRIVATE_KEY_INFO* pki = NULL;
    DWORD pki_len = 0;
    if (CryptDecodeObjectEx(X509_ASN_ENCODING | PKCS_7_ASN_ENCODING, PKCS_PRIVATE_KEY_INFO,
                            der, der_len, CRYPT_DECODE_ALLOC_FLAG, NULL, &pki, &pki_len) && pki) {
        CryptDecodeObjectEx(X509_ASN_ENCODING | PKCS_7_ASN_ENCODING, PKCS_RSA_PRIVATE_KEY,
                            pki->PrivateKey.pbData, pki->PrivateKey.cbData, CRYPT_DECODE_ALLOC_FLAG, NULL, &rsa_blob, &rsa_blob_len);
        LocalFree(pki);
    } else {
        CryptDecodeObjectEx(X509_ASN_ENCODING | PKCS_7_ASN_ENCODING, PKCS_RSA_PRIVATE_KEY,
                            der, der_len, CRYPT_DECODE_ALLOC_FLAG, NULL, &rsa_blob, &rsa_blob_len);
    }
    free(der);
    if (!rsa_blob) return "";
    BCRYPT_ALG_HANDLE hAlg = NULL, hHashAlg = NULL;
    BCRYPT_KEY_HANDLE hKey = NULL;
    BCRYPT_HASH_HANDLE hHash = NULL;
    char* b64_out = NULL;
    if (BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_RSA_ALGORITHM, NULL, 0) == 0 &&
        BCryptOpenAlgorithmProvider(&hHashAlg, BCRYPT_SHA256_ALGORITHM, NULL, 0) == 0 &&
        BCryptImportKeyPair(hAlg, NULL, LEGACY_RSAPRIVATE_BLOB, &hKey, rsa_blob, rsa_blob_len, 0) == 0 &&
        BCryptCreateHash(hHashAlg, &hHash, NULL, 0, NULL, 0, 0) == 0) {
        BYTE hash[32];
        if (BCryptHashData(hHash, (PUCHAR)data, (ULONG)strlen(data), 0) == 0 &&
            BCryptFinishHash(hHash, hash, sizeof(hash), 0) == 0) {
            BCRYPT_PKCS1_PADDING_INFO pad = { BCRYPT_SHA256_ALGORITHM };
            ULONG sig_len = 0;
            if (BCryptSignHash(hKey, &pad, hash, sizeof(hash), NULL, 0, &sig_len, BCRYPT_PAD_PKCS1) == 0 && sig_len > 0) {
                BYTE* sig = (BYTE*)malloc(sig_len);
                if (sig && BCryptSignHash(hKey, &pad, hash, sizeof(hash), sig, sig_len, &sig_len, BCRYPT_PAD_PKCS1) == 0) {
                    DWORD b64_len = 0;
                    if (CryptBinaryToStringA(sig, sig_len, CRYPT_STRING_BASE64 | CRYPT_STRING_NOCRLF, NULL, &b64_len)) {
                        b64_out = (char*)joss_req_alloc((size_t)b64_len + 1U);
                        if (b64_out && !CryptBinaryToStringA(sig, sig_len, CRYPT_STRING_BASE64 | CRYPT_STRING_NOCRLF, b64_out, &b64_len)) {
                            b64_out = NULL;
                        }
                    }
                }
                free(sig);
            }
        }
    }
    if (hHash) BCryptDestroyHash(hHash);
    if (hKey) BCryptDestroyKey(hKey);
    if (hHashAlg) BCryptCloseAlgorithmProvider(hHashAlg, 0);
    if (hAlg) BCryptCloseAlgorithmProvider(hAlg, 0);
    if (rsa_blob) LocalFree(rsa_blob);
    return b64_out ? b64_out : "";
#else
    return joss_rsa_sign_sha256_pure(data, priv_key);
#endif
}

/* --- Native Sitemap Service (parity with pkg/core/native_seo.go) --- */
typedef struct {
    char* loc;
    char* changefreq;
    char* priority;
} JossSitemapEntry;

static JossSitemapEntry g_sitemap_entries[256];
static int g_sitemap_count = 0;

void fn_Sitemap_add(const char* loc, const char* changefreq, const char* priority) {
    loc = unbox_web_cstr(loc);
    changefreq = unbox_web_cstr(changefreq);
    priority = unbox_web_cstr(priority);
    if (!loc || !loc[0]) return;
    joss_global_lock(JOSS_LOCK_SITEMAP);
    if (g_sitemap_count < 256) {
        g_sitemap_entries[g_sitemap_count].loc = joss_web_strdup(loc);
        g_sitemap_entries[g_sitemap_count].changefreq = joss_web_strdup((changefreq && changefreq[0]) ? changefreq : "weekly");
        g_sitemap_entries[g_sitemap_count].priority = joss_web_strdup((priority && priority[0]) ? priority : "0.8");
        g_sitemap_count++;
    }
    joss_global_unlock(JOSS_LOCK_SITEMAP);
}

typedef void* (*JossSitemapProviderFn)(void);
static void* g_sitemap_providers[32];
static int g_sitemap_provider_count = 0;
static char* g_sitemap_excludes[64];
static int g_sitemap_exclude_count = 0;

static bool joss_sitemap_is_excluded(const char* loc) {
    if (!loc) return false;
    for (int i = 0; i < g_sitemap_exclude_count; i++) {
        const char* pat = g_sitemap_excludes[i];
        if (!pat || !pat[0]) continue;
        size_t plen = strlen(pat);
        if (pat[plen - 1] == '*') {
            if (strncmp(loc, pat, plen - 1) == 0) return true;
        } else if (strcmp(loc, pat) == 0) {
            return true;
        }
    }
    return false;
}

void* fn_Sitemap_provider(void* callback) {
    if (!callback || !joss_is_not_null((int64_t)(uintptr_t)callback)) return NULL;
    joss_global_lock(JOSS_LOCK_SITEMAP);
    if (g_sitemap_provider_count < 32) {
        g_sitemap_providers[g_sitemap_provider_count++] = callback;
    }
    joss_global_unlock(JOSS_LOCK_SITEMAP);
    return NULL;
}

void fn_Sitemap_exclude(void* paths) {
    int64_t pv = joss_unbox_value((int64_t)(uintptr_t)paths);
    joss_global_lock(JOSS_LOCK_SITEMAP);
    if (joss_is_arr_ptr(pv)) {
        int64_t n = joss_arr_len((void*)(uintptr_t)pv);
        for (int64_t i = 0; i < n && g_sitemap_exclude_count < 64; i++) {
            const char* s = joss_strval(joss_arr_get_i64((void*)(uintptr_t)pv, i));
            if (s && s[0]) g_sitemap_excludes[g_sitemap_exclude_count++] = joss_web_strdup(s);
        }
    } else {
        const char* s = unbox_web_cstr((const char*)(uintptr_t)pv);
        if (s && s[0] && g_sitemap_exclude_count < 64) {
            g_sitemap_excludes[g_sitemap_exclude_count++] = joss_web_strdup(s);
        }
    }
    joss_global_unlock(JOSS_LOCK_SITEMAP);
}

const char* fn_Sitemap_xsl(void) {
    return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"
           "<xsl:stylesheet version=\"2.0\" xmlns:xsl=\"http://www.w3.org/1999/XSL/Transform\" "
           "xmlns:sitemap=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n"
           "<xsl:output method=\"html\" version=\"1.0\" encoding=\"UTF-8\" indent=\"yes\"/>\n"
           "<xsl:template match=\"/\"><html lang=\"es\"><head><title>Sitemap</title></head>"
           "<body><h1>Sitemap XML</h1><ul><xsl:for-each select=\"sitemap:urlset/sitemap:url\">"
           "<li><a href=\"{sitemap:loc}\"><xsl:value-of select=\"sitemap:loc\"/></a></li>"
           "</xsl:for-each></ul></body></html></xsl:template></xsl:stylesheet>\n";
}

const char* fn_Sitemap_generate(const char* base_url) {
    base_url = unbox_web_cstr(base_url);
    if (!base_url || !base_url[0]) {
        base_url = joss_env("APP_URL", "http://localhost");
    }
    if (!base_url || !base_url[0]) base_url = "http://localhost";
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n");
    joss_sb_append(sb, "<?xml-stylesheet type=\"text/xsl\" href=\"/sitemap.xsl\"?>\n");
    joss_sb_append(sb, "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n");
    joss_global_lock(JOSS_LOCK_SITEMAP);
    if (g_sitemap_count == 0 && g_sitemap_provider_count == 0) {
        static const char* defaults[] = {"/", "/lenguaje", "/docs", "/blog", "/creador", "/catalogs", "/pub"};
        for (int i = 0; i < 7; i++) {
            if (joss_sitemap_is_excluded(defaults[i])) continue;
            joss_sb_append(sb, "  <url><loc>");
            joss_sb_append(sb, base_url);
            joss_sb_append(sb, defaults[i]);
            joss_sb_append(sb, "</loc><changefreq>weekly</changefreq><priority>0.8</priority></url>\n");
        }
    } else {
        for (int i = 0; i < g_sitemap_count; i++) {
            if (joss_sitemap_is_excluded(g_sitemap_entries[i].loc)) continue;
            joss_sb_append(sb, "  <url><loc>");
            if (strncmp(g_sitemap_entries[i].loc, "http", 4) != 0) joss_sb_append(sb, base_url);
            joss_sb_append(sb, g_sitemap_entries[i].loc);
            joss_sb_append(sb, "</loc><changefreq>");
            joss_sb_append(sb, g_sitemap_entries[i].changefreq);
            joss_sb_append(sb, "</changefreq><priority>");
            joss_sb_append(sb, g_sitemap_entries[i].priority);
            joss_sb_append(sb, "</priority></url>\n");
        }
        for (int p = 0; p < g_sitemap_provider_count; p++) {
            void* fn_raw = joss_closure_prepare_call(g_sitemap_providers[p]);
            if (!fn_raw) continue;
            void* items = ((JossSitemapProviderFn)fn_raw)();
            int64_t iv = joss_unbox_value((int64_t)(uintptr_t)items);
            if (!joss_is_arr_ptr(iv)) continue;
            int64_t n = joss_arr_len((void*)(uintptr_t)iv);
            for (int64_t i = 0; i < n; i++) {
                int64_t el = joss_arr_get_i64((void*)(uintptr_t)iv, i);
                const char* loc = NULL;
                const char* cf = "weekly";
                const char* prio = "0.8";
                if (joss_is_map_ptr(el)) {
                    loc = joss_map_get_str((void*)(uintptr_t)el, "loc");
                    const char* mcf = joss_map_get_str((void*)(uintptr_t)el, "changefreq");
                    const char* mpr = joss_map_get_str((void*)(uintptr_t)el, "priority");
                    if (mcf && mcf[0]) cf = mcf;
                    if (mpr && mpr[0]) prio = mpr;
                } else {
                    loc = joss_strval(el);
                }
                if (!loc || !loc[0] || joss_sitemap_is_excluded(loc)) continue;
                joss_sb_append(sb, "  <url><loc>");
                if (strncmp(loc, "http", 4) != 0) joss_sb_append(sb, base_url);
                joss_sb_append(sb, loc);
                joss_sb_append(sb, "</loc><changefreq>");
                joss_sb_append(sb, cf);
                joss_sb_append(sb, "</changefreq><priority>");
                joss_sb_append(sb, prio);
                joss_sb_append(sb, "</priority></url>\n");
            }
        }
    }
    joss_global_unlock(JOSS_LOCK_SITEMAP);
    joss_sb_append(sb, "</urlset>\n");
    char* out = joss_sb_to_req_string(sb);
    return out;
}

