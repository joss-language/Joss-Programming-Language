/* =========================================================================
 * Cron Scheduler & Background Ticker (5-field RFC/Vixie Cron)
 * ========================================================================= */
#define JOSS_MAX_CRON_JOBS 64

typedef void (*JossCronCallbackFn)(void);

typedef struct {
    bool    active;
    char*   expr;
    void*   callback_raw;
    int64_t last_run_minute;
} JossCronJob;

static JossCronJob g_joss_cron_jobs[JOSS_MAX_CRON_JOBS];
static int         g_joss_cron_count = 0;
static atomic_flag g_joss_cron_ticker_started = ATOMIC_FLAG_INIT;

#if defined(JOSS_HAS_RUST_RT)
extern int64_t joss_rs_cron_matches(const char* expr, int32_t minute, int32_t hour, int32_t day, int32_t month, int32_t weekday);

static bool joss_cron_eval(const char* expr, const struct tm* tm_val, bool* out_valid) {
    if (out_valid) *out_valid = false;
    if (!expr) return false;
    while (*expr && isspace((unsigned char)*expr)) expr++;
    if (!*expr) return false;
    int min_v = tm_val ? tm_val->tm_min : 0;
    int hour_v = tm_val ? tm_val->tm_hour : 0;
    int dom_v = tm_val ? tm_val->tm_mday : 1;
    int mon_v = tm_val ? (tm_val->tm_mon + 1) : 1;
    int dow_v = tm_val ? tm_val->tm_wday : 0;
    int64_t res = joss_rs_cron_matches(expr, min_v, hour_v, dom_v, mon_v, dow_v);
    if (res < 0) return false;
    if (out_valid) *out_valid = true;
    return tm_val ? (res == 1) : true;
}
#else
static bool joss_cron_parse_int(const char* s, int min_v, int max_v, bool is_dow, int* out) {
    if (!s || !*s) return false;
    char* endp = NULL;
    long v = strtol(s, &endp, 10);
    if (endp == s || *endp != '\0') return false;
    if (is_dow && v == 7) v = 0;
    if (v < min_v || v > max_v) return false;
    *out = (int)v;
    return true;
}

static bool joss_cron_field_eval(const char* field, int val, int min_v, int max_v, bool is_dow, bool* out_valid) {
    if (!field || !*field) {
        *out_valid = false;
        return false;
    }
    char buf[128];
    size_t flen = strlen(field);
    if (flen >= sizeof(buf)) {
        *out_valid = false;
        return false;
    }
    memcpy(buf, field, flen + 1U);

    bool matched = false;
    char* p = buf;
    while (p && *p) {
        char* comma = strchr(p, ',');
        if (comma) *comma = '\0';
        char* tok = p;
        p = comma ? (comma + 1) : NULL;
        if (!*tok) {
            *out_valid = false;
            return false;
        }
        char* slash = strchr(tok, '/');
        int step = 1;
        if (slash) {
            *slash = '\0';
            char* step_str = slash + 1;
            char* endp = NULL;
            long sv = strtol(step_str, &endp, 10);
            if (*step_str == '\0' || *endp != '\0' || sv <= 0 || sv > 1000) {
                *out_valid = false;
                return false;
            }
            step = (int)sv;
        }
        if (strcmp(tok, "*") == 0) {
            if (val >= min_v && val <= max_v && ((val - min_v) % step) == 0) {
                matched = true;
            }
        } else {
            char* dash = strchr(tok, '-');
            if (dash) {
                *dash = '\0';
                const char* left = tok;
                const char* right = dash + 1;
                char* end1 = NULL;
                char* end2 = NULL;
                long a = strtol(left, &end1, 10);
                long b = strtol(right, &end2, 10);
                if (*left == '\0' || *end1 != '\0' || *right == '\0' || *end2 != '\0') {
                    *out_valid = false;
                    return false;
                }
                if (is_dow && a == 7 && b == 7) {
                    a = 0;
                    b = 0;
                }
                int max_allowed = is_dow ? 7 : max_v;
                if (a < min_v || b > max_allowed || a > b) {
                    *out_valid = false;
                    return false;
                }
                if (is_dow && b == 7 && val == 0) {
                    if (((7L - a) % step) == 0) matched = true;
                }
                if (val >= (int)a && val <= (int)b && ((val - (int)a) % step) == 0) {
                    matched = true;
                }
            } else {
                int num = 0;
                if (!joss_cron_parse_int(tok, min_v, max_v, is_dow, &num)) {
                    *out_valid = false;
                    return false;
                }
                if (slash) {
                    if (val >= num && val <= max_v && ((val - num) % step) == 0) {
                        matched = true;
                    }
                } else if (val == num) {
                    matched = true;
                }
            }
        }
    }
    *out_valid = true;
    return matched;
}

static bool joss_cron_eval(const char* expr, const struct tm* tm_val, bool* out_valid) {
    if (out_valid) *out_valid = false;
    if (!expr) return false;
    while (*expr && isspace((unsigned char)*expr)) expr++;
    if (!*expr) return false;

    const char* canonical = expr;
    if (strcasecmp(expr, "hourly") == 0 || strcasecmp(expr, "@hourly") == 0) canonical = "0 * * * *";
    else if (strcasecmp(expr, "daily") == 0 || strcasecmp(expr, "@daily") == 0) canonical = "0 0 * * *";
    else if (strcasecmp(expr, "weekly") == 0 || strcasecmp(expr, "@weekly") == 0) canonical = "0 0 * * 0";
    else if (strcasecmp(expr, "monthly") == 0 || strcasecmp(expr, "@monthly") == 0) canonical = "0 0 1 * *";
    else if (strcasecmp(expr, "yearly") == 0 || strcasecmp(expr, "@yearly") == 0 || strcasecmp(expr, "@annually") == 0) canonical = "0 0 1 1 *";

    char copy[256];
    size_t len = strlen(canonical);
    if (len >= sizeof(copy)) return false;
    memcpy(copy, canonical, len + 1U);

    char* fields[5] = {NULL, NULL, NULL, NULL, NULL};
    int nf = 0;
    char* p = copy;
    while (*p) {
        while (*p && isspace((unsigned char)*p)) p++;
        if (!*p) break;
        if (nf >= 5) return false;
        fields[nf++] = p;
        while (*p && !isspace((unsigned char)*p)) p++;
        if (*p) *p++ = '\0';
    }
    if (nf != 5) return false;

    int min_v = tm_val ? tm_val->tm_min : 0;
    int hour_v = tm_val ? tm_val->tm_hour : 0;
    int dom_v = tm_val ? tm_val->tm_mday : 1;
    int mon_v = tm_val ? (tm_val->tm_mon + 1) : 1;
    int dow_v = tm_val ? tm_val->tm_wday : 0;

    bool v0 = false, v1 = false, v2 = false, v3 = false, v4 = false;
    bool m0 = joss_cron_field_eval(fields[0], min_v,  0, 59, false, &v0);
    bool m1 = joss_cron_field_eval(fields[1], hour_v, 0, 23, false, &v1);
    bool m2 = joss_cron_field_eval(fields[2], dom_v,  1, 31, false, &v2);
    bool m3 = joss_cron_field_eval(fields[3], mon_v,  1, 12, false, &v3);
    bool m4 = joss_cron_field_eval(fields[4], dow_v,  0, 6,  true,  &v4);

    if (!v0 || !v1 || !v2 || !v3 || !v4) return false;
    if (out_valid) *out_valid = true;
    return tm_val ? (m0 && m1 && m2 && m3 && m4) : true;
}
#endif /* JOSS_HAS_RUST_RT */

static int64_t joss_cron_tick_once(bool force_due) {
    time_t now = time(NULL);
    struct tm tm_now;
#if defined(_WIN32)
    localtime_s(&tm_now, &now);
#else
    localtime_r(&now, &tm_now);
#endif
    int64_t cur_min = (int64_t)(now / 60);

    void* due[JOSS_MAX_CRON_JOBS];
    int due_count = 0;

    joss_global_lock(JOSS_LOCK_CRON);
    for (int i = 0; i < g_joss_cron_count; i++) {
        JossCronJob* job = &g_joss_cron_jobs[i];
        if (!job->active || !job->callback_raw || !job->expr) continue;
        bool valid = false;
        if (joss_cron_eval(job->expr, &tm_now, &valid) && valid) {
            if (force_due || job->last_run_minute != cur_min) {
                job->last_run_minute = cur_min;
                if (due_count < JOSS_MAX_CRON_JOBS) {
                    due[due_count++] = job->callback_raw;
                }
            }
        }
    }
    joss_global_unlock(JOSS_LOCK_CRON);

    for (int i = 0; i < due_count; i++) {
        joss_req_arena_begin();
        joss_try_reset();
        void* fn_raw = joss_closure_prepare_call(due[i]);
        if (fn_raw) {
            ((JossCronCallbackFn)fn_raw)();
        }
        joss_req_state_reset();
        joss_req_arena_reset();
    }
    return (int64_t)due_count;
}

#if defined(_WIN32)
static unsigned __stdcall joss_cron_ticker_main(void* arg) {
    (void)arg;
    for (;;) {
        Sleep(1000);
        (void)joss_cron_tick_once(false);
    }
    return 0;
}
#elif defined(__unix__) || defined(__APPLE__)
static void* joss_cron_ticker_main(void* arg) {
    (void)arg;
    for (;;) {
        sleep(1);
        (void)joss_cron_tick_once(false);
    }
    return NULL;
}
#endif

static void joss_cron_ensure_ticker_started(void) {
    if (!atomic_flag_test_and_set(&g_joss_cron_ticker_started)) {
#if defined(_WIN32)
        uintptr_t th = _beginthreadex(NULL, 0, joss_cron_ticker_main, NULL, 0, NULL);
        if (th != 0) CloseHandle((HANDLE)th);
#elif defined(__unix__) || defined(__APPLE__)
        pthread_t tid;
        if (pthread_create(&tid, NULL, joss_cron_ticker_main, NULL) == 0) {
            pthread_detach(tid);
        }
#endif
    }
}

void* fn_Cron_schedule(const char* expr, void* callback) {
    expr = unbox_web_cstr(expr);
    if (!expr || !expr[0] || !joss_is_not_null((int64_t)(uintptr_t)callback)) return NULL;
    bool valid = false;
    (void)joss_cron_eval(expr, NULL, &valid);
    if (!valid) return NULL;

    joss_global_lock(JOSS_LOCK_CRON);
    void* result = NULL;
    if (g_joss_cron_count < JOSS_MAX_CRON_JOBS) {
        JossCronJob* job = &g_joss_cron_jobs[g_joss_cron_count++];
        job->active = true;
        job->expr = joss_ws_strdup_heap(expr);
        job->callback_raw = callback;
        job->last_run_minute = -1;
        result = (void*)(intptr_t)1;
    }
    joss_global_unlock(JOSS_LOCK_CRON);
    if (result) {
        joss_cron_ensure_ticker_started();
    }
    return result;
}

int64_t fn_Cron_runDue(void) {
    return joss_cron_tick_once(true);
}

void* fn_AI_client(const char* provider, const char* api_key) {
    void* m = joss_map_new();
    joss_map_set_str(m, "provider", unbox_web_cstr(provider) ? unbox_web_cstr(provider) : "openai");
    joss_map_set_str(m, "api_key", unbox_web_cstr(api_key) ? unbox_web_cstr(api_key) : "");
    return m;
}

const char* fn_AI_chat(const char* prompt, void* opts) {
    prompt = unbox_web_cstr(prompt);
    (void)opts;
    return prompt ? prompt : "";
}

static void* g_joss_notify_devices = NULL;
static void* g_joss_notify_pending = NULL;

void* fn_Notify_app(const char* app_id) {
    void* m = joss_map_new();
    joss_map_set_str(m, "app_id", unbox_web_cstr(app_id) ? unbox_web_cstr(app_id) : "default");
    return m;
}

int64_t fn_Notify_registerDevice(const char* user_id, const char* token) {
    user_id = unbox_web_cstr(user_id);
    token = unbox_web_cstr(token);
    if (!user_id || !token) return 0;
    joss_global_lock(JOSS_LOCK_NOTIFY);
    bool _prev_arena = joss_req_arena_pause();
    if (!g_joss_notify_devices) g_joss_notify_devices = joss_map_new();
    joss_map_set_str(g_joss_notify_devices, user_id, joss_web_strdup(token));
    joss_req_arena_resume(_prev_arena);
    joss_global_unlock(JOSS_LOCK_NOTIFY);
    return 1;
}

int64_t fn_Notify_unregisterDevice(const char* user_id) {
    user_id = unbox_web_cstr(user_id);
    if (!user_id) return 0;
    joss_global_lock(JOSS_LOCK_NOTIFY);
    int64_t ok = 0;
    if (g_joss_notify_devices) {
        joss_map_delete_i64(g_joss_notify_devices, user_id);
        ok = 1;
    }
    joss_global_unlock(JOSS_LOCK_NOTIFY);
    return ok;
}

void* fn_Notify_pendingMessages(const char* user_id) {
    user_id = unbox_web_cstr(user_id);
    if (!user_id) return joss_arr_new(0);
    joss_global_lock(JOSS_LOCK_NOTIFY);
    int64_t v = g_joss_notify_pending ? joss_map_get_i64(g_joss_notify_pending, user_id) : 0;
    joss_global_unlock(JOSS_LOCK_NOTIFY);
    if (joss_is_arr_ptr(v)) return (void*)(uintptr_t)v;
    return joss_arr_new(0);
}

int64_t fn_Notify_acknowledge(const char* user_id, const char* msg_id) {
    (void)msg_id;
    user_id = unbox_web_cstr(user_id);
    if (!user_id) return 1;
    joss_global_lock(JOSS_LOCK_NOTIFY);
    if (g_joss_notify_pending) joss_map_delete_i64(g_joss_notify_pending, user_id);
    joss_global_unlock(JOSS_LOCK_NOTIFY);
    return 1;
}

typedef int64_t (*JossPluginDispatchFn)(const char*, const char*, void*);
static JossPluginDispatchFn g_joss_plugin_dispatcher = NULL;

void joss_register_plugin_dispatcher(void* fn) {
    g_joss_plugin_dispatcher = (JossPluginDispatchFn)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)fn);
}

int64_t fn_Plugin_call(const char* plugin_name, const char* func_name, void* args) {
    plugin_name = unbox_web_cstr(plugin_name);
    func_name = unbox_web_cstr(func_name);
    if (g_joss_plugin_dispatcher && plugin_name && func_name) {
        return g_joss_plugin_dispatcher(plugin_name, func_name, args);
    }
    return 0;
}

int64_t fn_Plugin_stream(const char* name, const char* func_name, void* args) {
    return fn_Plugin_call(name, func_name, args);
}

const char* fn_Plugin_platform(void) {
#if defined(_WIN32)
    #if defined(__aarch64__) || defined(_M_ARM64)
        return "windows-arm64";
    #elif defined(__x86_64__) || defined(_M_X64)
        return "windows-amd64";
    #elif defined(__i386__) || defined(_M_IX86)
        return "windows-386";
    #else
        return "windows-unknown";
    #endif
#elif defined(__APPLE__)
    #if defined(__aarch64__) || defined(__arm64__)
        return "darwin-arm64";
    #elif defined(__x86_64__)
        return "darwin-amd64";
    #else
        return "darwin-unknown";
    #endif
#elif defined(__ANDROID__)
    #if defined(__aarch64__)
        return "android-arm64";
    #elif defined(__x86_64__)
        return "android-amd64";
    #elif defined(__arm__)
        return "android-arm";
    #else
        return "android-unknown";
    #endif
#elif defined(__linux__)
    #if defined(__aarch64__)
        return "linux-arm64";
    #elif defined(__x86_64__)
        return "linux-amd64";
    #elif defined(__i386__)
        return "linux-386";
    #elif defined(__arm__)
        return "linux-arm";
    #else
        return "linux-unknown";
    #endif
#else
    return "unknown-unknown";
#endif
}

static bool joss_plugin_clean_rel(const char* val, char* out, size_t out_cap, bool allow_subdirs) {
    if (!val || !val[0] || !out || out_cap < 2) return false;
    if (val[0] == '/' || val[0] == '\\') return false;
    if (isalpha((unsigned char)val[0]) && val[1] == ':') return false;

    const char* seg_start[64];
    size_t seg_len[64];
    int seg_count = 0;

    const char* p = val;
    while (*p) {
        if (*p == '/' || *p == '\\') {
            if (!allow_subdirs) return false;
            p++;
            continue;
        }
        const char* start = p;
        while (*p && *p != '/' && *p != '\\') p++;
        size_t len = (size_t)(p - start);
        if (len == 1 && start[0] == '.') {
            continue;
        }
        if (len == 2 && start[0] == '.' && start[1] == '.') {
            return false;
        }
        if (seg_count >= 64) return false;
        seg_start[seg_count] = start;
        seg_len[seg_count] = len;
        seg_count++;
    }
    if (seg_count == 0) return false;

#if defined(_WIN32)
    const char sep = '\\';
#else
    const char sep = '/';
#endif
    size_t pos = 0;
    for (int i = 0; i < seg_count; i++) {
        if (i > 0) {
            if (pos + 1 >= out_cap) return false;
            out[pos++] = sep;
        }
        if (pos + seg_len[i] >= out_cap) return false;
        memcpy(out + pos, seg_start[i], seg_len[i]);
        pos += seg_len[i];
    }
    out[pos] = '\0';
    return true;
}

const char* fn_Plugin_path(const char* name, const char* rel) {
    name = unbox_web_cstr(name);
    rel = unbox_web_cstr(rel);

    char cwd[1024];
    cwd[0] = '\0';
#if defined(_WIN32)
    if (!_getcwd(cwd, (int)sizeof(cwd))) snprintf(cwd, sizeof(cwd), ".");
    const char sep = '\\';
#else
    if (!getcwd(cwd, sizeof(cwd))) snprintf(cwd, sizeof(cwd), ".");
    const char sep = '/';
#endif
    size_t cwd_len = strlen(cwd);
    while (cwd_len > 1 && (cwd[cwd_len - 1] == '/' || cwd[cwd_len - 1] == '\\')) {
        cwd[--cwd_len] = '\0';
    }

    char clean_name[256];
    char clean_rel[1024];
    bool ok_name = joss_plugin_clean_rel(name, clean_name, sizeof(clean_name), false);
    bool ok_rel = joss_plugin_clean_rel(rel, clean_rel, sizeof(clean_rel), true);

    char out[2400];
    if (!ok_name || !ok_rel) {
        snprintf(out, sizeof(out), "%s%cplugins", cwd, sep);
    } else {
        snprintf(out, sizeof(out), "%s%cplugins%c%s%c%s", cwd, sep, sep, clean_name, sep, clean_rel);
    }
    size_t n = strlen(out);
    char* copy = (char*)joss_req_alloc(n + 1);
    if (!copy) return "plugins";
    memcpy(copy, out, n + 1);
    return copy;
}

