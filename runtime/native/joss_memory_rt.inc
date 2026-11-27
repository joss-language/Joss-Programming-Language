/* =========================================================================
 * Rust-Inspired Memory Diagnostics & Watchdog API (Memory::*)
 * ========================================================================= */
#if defined(__APPLE__)
#include <mach/mach.h>
#include <malloc/malloc.h>
#elif defined(__unix__)
#include <unistd.h>
#endif

#if defined(_WIN32)
#include <malloc.h>
typedef struct {
    DWORD  cb;
    DWORD  PageFaultCount;
    SIZE_T PeakWorkingSetSize;
    SIZE_T WorkingSetSize;
    SIZE_T QuotaPeakPagedPoolUsage;
    SIZE_T QuotaPagedPoolUsage;
    SIZE_T QuotaPeakNonPagedPoolUsage;
    SIZE_T QuotaNonPagedPoolUsage;
    SIZE_T PagefileUsage;
    SIZE_T PeakPagefileUsage;
} JossWinProcMemCounters;
typedef BOOL (WINAPI *JossK32GetProcMemInfoFn)(HANDLE, JossWinProcMemCounters*, DWORD);
#endif

int64_t fn_Memory_rss(void) {
#if defined(JOSS_HAS_RUST_RT)
    return joss_rs_memory_rss();
#elif defined(_WIN32)
    static JossK32GetProcMemInfoFn s_fn = NULL;
    static int s_tried = 0;
    if (!s_tried) {
        s_tried = 1;
        HMODULE k32 = GetModuleHandleA("kernel32.dll");
        if (k32) {
            FARPROC p = GetProcAddress(k32, "K32GetProcessMemoryInfo");
            if (p) memcpy(&s_fn, &p, sizeof(s_fn));
        }
    }
    if (s_fn) {
        JossWinProcMemCounters pmc;
        memset(&pmc, 0, sizeof(pmc));
        pmc.cb = sizeof(pmc);
        if (s_fn(GetCurrentProcess(), &pmc, sizeof(pmc))) {
            return (int64_t)pmc.WorkingSetSize;
        }
    }
    return 0;
#elif defined(__APPLE__)
    struct mach_task_basic_info info;
    mach_msg_type_number_t count = MACH_TASK_BASIC_INFO_COUNT;
    if (task_info(mach_task_self(), MACH_TASK_BASIC_INFO, (task_info_t)&info, &count) == KERN_SUCCESS) {
        return (int64_t)info.resident_size;
    }
    return 0;
#elif defined(__unix__)
    JOSS_AUTO_FILE FILE* f = fopen("/proc/self/statm", "r");
    if (!f) return 0;
    unsigned long long size_pages = 0, rss_pages = 0;
    int matched = fscanf(f, "%llu %llu", &size_pages, &rss_pages);
    if (matched == 2) {
        long page_sz = sysconf(_SC_PAGESIZE);
        if (page_sz <= 0) page_sz = 4096;
        return (int64_t)(rss_pages * (unsigned long long)page_sz);
    }
    return 0;
#else
    return 0;
#endif
}

int64_t fn_Memory_arenaUsed(void) {
    return (int64_t)joss_req_arena_bytes_used();
}

void* fn_Memory_gcStats(void) {
    JossGCStats st;
    joss_gc_stats(&st);
    void* m = joss_map_new();
    joss_map_set_i64(m, "objects_live",    (int64_t)st.objects_live);
    joss_map_set_i64(m, "bytes_live",      (int64_t)st.bytes_live);
    joss_map_set_i64(m, "total_allocs",    (int64_t)st.total_allocs);
    joss_map_set_i64(m, "total_freed",     (int64_t)st.total_freed);
    joss_map_set_i64(m, "collections",     (int64_t)st.collections);
    joss_map_set_i64(m, "threshold_bytes", (int64_t)st.threshold_bytes);
    return m;
}

int64_t fn_Memory_gcCollect(void) {
    return (int64_t)joss_gc_collect();
}

int64_t fn_Memory_trim(void) {
#if defined(JOSS_HAS_RUST_RT)
    return joss_rs_memory_trim();
#elif defined(__GLIBC__)
    return malloc_trim(0) ? 1 : 0;
#elif defined(__APPLE__)
    malloc_zone_pressure_relief(NULL, 0);
    return 1;
#elif defined(_WIN32)
    int r = _heapmin();
    SIZE_T c = HeapCompact(GetProcessHeap(), 0);
    return (r == 0 || c > 0) ? 1 : 0;
#else
    return 0;
#endif
}

int64_t fn_Memory_sizeOf(int64_t val) {
    if (val == 0) return 0;
    size_t blen = 0;
    if (joss_bytes_unwrap((const void*)(uintptr_t)val, &blen)) {
        return (int64_t)(sizeof(JossBoxedValue) + sizeof(JossBytesBuf) + blen + 1);
    }
    if (joss_is_arr_ptr(val)) {
        const JossArray* a = (const JossArray*)(uintptr_t)(uint64_t)val;
        return (int64_t)(sizeof(JossArray) + (size_t)a->cap * sizeof(int64_t));
    }
    if (joss_is_nominal_ptr(val)) {
        const JossNominalObj* o = (const JossNominalObj*)(uintptr_t)(uint64_t)val;
        int64_t base = (int64_t)(sizeof(JossNominalObj) + (size_t)o->num_fields * sizeof(int64_t));
        if (o->overflow_map) base += fn_Memory_sizeOf((int64_t)(uintptr_t)o->overflow_map);
        return base;
    }
    if (joss_is_map_ptr(val)) {
        const JossMap* m = (const JossMap*)(uintptr_t)(uint64_t)val;
        size_t sz = sizeof(JossMap);
        if (m->entries && m->entries != m->inline_entries) {
            sz += (size_t)m->entry_cap * sizeof(JossMapEntry);
        }
        if (m->slots) {
            sz += (size_t)m->slot_cap * sizeof(int32_t);
        }
        return (int64_t)sz;
    }
    if (joss_is_boxed_ptr(val)) {
        const JossBoxedValue* b = (const JossBoxedValue*)(uintptr_t)(uint64_t)val;
        uint32_t tag = (uint32_t)(b->header & 0xFFFFFFFFULL);
        if (tag == JOSS_TAG_STRING && b->payload >= JOSS_MIN_PTR_ADDR) {
            const char* s = (const char*)(uintptr_t)b->payload;
            return (int64_t)(sizeof(JossBoxedValue) + strlen(s) + 1);
        }
        return (int64_t)sizeof(JossBoxedValue);
    }
    uint64_t u = (uint64_t)val;
    if (u >= JOSS_MIN_PTR_ADDR && (u >> JOSS_PTR_VA_BITS) == 0) {
        const char* s = (const char*)(uintptr_t)u;
        return (int64_t)(strlen(s) + 1);
    }
    return 8;
}

void* fn_Memory_snapshot(void) {
    JossGCStats st;
    joss_gc_stats(&st);
    void* m = joss_map_new();
    joss_map_set_i64(m, "rss_bytes",       fn_Memory_rss());
    joss_map_set_i64(m, "arena_bytes",     (int64_t)joss_req_arena_bytes_used());
    joss_map_set_i64(m, "gc_bytes_live",   (int64_t)st.bytes_live);
    joss_map_set_i64(m, "gc_objects_live", (int64_t)st.objects_live);
    joss_map_set_i64(m, "gc_collections",  (int64_t)st.collections);
    return m;
}

void* fn_Memory_diff(void* before_map) {
    JossGCStats st;
    joss_gc_stats(&st);
    int64_t cur_rss   = fn_Memory_rss();
    int64_t cur_arena = (int64_t)joss_req_arena_bytes_used();
    int64_t cur_gcb   = (int64_t)st.bytes_live;
    int64_t cur_gco   = (int64_t)st.objects_live;

    int64_t b_rss = 0, b_arena = 0, b_gcb = 0, b_gco = 0;
    before_map = unbox_ptr(before_map);
    if (before_map && joss_is_map_ptr((int64_t)(uintptr_t)before_map)) {
        b_rss   = joss_unbox_value(joss_map_get_i64(before_map, "rss_bytes"));
        b_arena = joss_unbox_value(joss_map_get_i64(before_map, "arena_bytes"));
        b_gcb   = joss_unbox_value(joss_map_get_i64(before_map, "gc_bytes_live"));
        b_gco   = joss_unbox_value(joss_map_get_i64(before_map, "gc_objects_live"));
    }

    void* m = joss_map_new();
    joss_map_set_i64(m, "rss_delta",        cur_rss - b_rss);
    joss_map_set_i64(m, "arena_delta",      cur_arena - b_arena);
    joss_map_set_i64(m, "gc_bytes_delta",   cur_gcb - b_gcb);
    joss_map_set_i64(m, "gc_objects_delta", cur_gco - b_gco);
    return m;
}

const char* fn_Memory_report(void) {
    JossGCStats st;
    joss_gc_stats(&st);
    int64_t rss = fn_Memory_rss();
    size_t arena = joss_req_arena_bytes_used();
    char buf[256];
    snprintf(buf, sizeof(buf),
             "Memory[RSS: %.2f MB | Arena: %.2f KB | GC live: %zu objs (%.2f KB) | GC runs: %zu]",
             (double)rss / (1024.0 * 1024.0),
             (double)arena / 1024.0,
             st.objects_live,
             (double)st.bytes_live / 1024.0,
             st.collections);
    return joss_req_strdup(buf);
}

int64_t fn_Memory_checkGrowth(void* before_map, int64_t max_delta_bytes, const char* label) {
    max_delta_bytes = joss_unbox_value(max_delta_bytes);
    label = unbox_cstr_rt(label);
    if (!label || !label[0]) label = "memory_check";

    void* d = fn_Memory_diff(before_map);
    int64_t rss_delta   = joss_unbox_value(joss_map_get_i64(d, "rss_delta"));
    int64_t gc_delta    = joss_unbox_value(joss_map_get_i64(d, "gc_bytes_delta"));
    int64_t arena_delta = joss_unbox_value(joss_map_get_i64(d, "arena_delta"));

    if (rss_delta > max_delta_bytes || gc_delta > max_delta_bytes) {
        fprintf(stderr,
                "Advertencia \xE2\x9A\xA0\xEF\xB8\x8F se ha detectado falla en la memoria (%s) en "
                "[RSS: %+lld KB | GC live: %+lld KB | Arena: %+lld KB | Limite: %lld KB], "
                "se est\xC3\xA1 detectando un crecimiento.\n",
                label,
                (long long)(rss_delta / 1024),
                (long long)(gc_delta / 1024),
                (long long)(arena_delta / 1024),
                (long long)(max_delta_bytes / 1024));
        fflush(stderr);
        return 0;
    }
    return 1;
}

/* Built-in Automatic Server Memory Watchdog (ALWAYS active with safe defaults).
 * Developers may override via env:
 *   MEMORY_AUTO_TRIM_MB (default: 128 MB; 0 disables auto-trim)
 *   MEMORY_WARN_MB      (default: 256 MB; 0 disables warning log)
 */
static int64_t joss_parse_mem_env_mb(const char* name, int64_t default_mb) {
    const char* raw = getenv(name);
    if (!raw || !raw[0]) return default_mb;
    char* endp = NULL;
    long long v = strtoll(raw, &endp, 10);
    if (endp == raw || v < 0) return default_mb;
    return (int64_t)v;
}

static void joss_memory_watchdog_tick(void) {
    static _Atomic(uint64_t) s_wd_ticks = 0;
    static _Atomic(int64_t)  s_wd_last_warn_sec = 0;
    uint64_t tick = atomic_fetch_add_explicit(&s_wd_ticks, 1ULL, memory_order_relaxed) + 1ULL;
    if ((tick & 255ULL) != 0ULL) return;

    int64_t trim_mb = joss_parse_mem_env_mb("MEMORY_AUTO_TRIM_MB", 128);
    int64_t warn_mb = joss_parse_mem_env_mb("MEMORY_WARN_MB", 256);
    if (trim_mb <= 0 && warn_mb <= 0) return;

    int64_t rss = fn_Memory_rss();
    if (rss <= 0) return;

    if (trim_mb > 0 && rss >= trim_mb * 1024LL * 1024LL) {
        (void)fn_Memory_trim();
        rss = fn_Memory_rss();
    }
    if (warn_mb > 0 && rss >= warn_mb * 1024LL * 1024LL) {
        int64_t now_s = (int64_t)time(NULL);
        int64_t prev_s = atomic_load_explicit(&s_wd_last_warn_sec, memory_order_relaxed);
        if (now_s - prev_s >= 30 &&
            atomic_compare_exchange_strong_explicit(&s_wd_last_warn_sec, &prev_s, now_s,
                                                    memory_order_acq_rel, memory_order_relaxed)) {
            fprintf(stderr,
                    "Advertencia \xE2\x9A\xA0\xEF\xB8\x8F se ha detectado falla en la memoria (Server Watchdog) en "
                    "[RSS: %.2f MB | Umbral: %lld MB | Arena: %.2f KB], "
                    "se est\xC3\xA1 detectando un crecimiento.\n",
                    (double)rss / (1024.0 * 1024.0),
                    (long long)warn_mb,
                    (double)joss_req_arena_bytes_used() / 1024.0);
            fflush(stderr);
        }
    }
}

