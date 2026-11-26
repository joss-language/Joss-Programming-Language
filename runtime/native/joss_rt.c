//go:build ignore
#if defined(__APPLE__) && !defined(_DARWIN_C_SOURCE)
#define _DARWIN_C_SOURCE
#elif defined(__unix__)
#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif
#ifndef _POSIX_C_SOURCE
#define _POSIX_C_SOURCE 200809L
#endif
#endif

#include "joss_rt.h"

#include <inttypes.h>
#include <setjmp.h>
#include <stdatomic.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#if defined(_WIN32)
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <windows.h>
#elif defined(__unix__) || defined(__APPLE__)
#include <pthread.h>
#endif

static inline void joss_atomic_update_bounds(_Atomic(uintptr_t)* min_p, _Atomic(uintptr_t)* max_p, uintptr_t addr) {
    uintptr_t cur_min = atomic_load_explicit(min_p, memory_order_relaxed);
    while (addr < cur_min &&
           !atomic_compare_exchange_weak_explicit(min_p, &cur_min, addr, memory_order_release, memory_order_relaxed)) {
    }
    uintptr_t cur_max = atomic_load_explicit(max_p, memory_order_relaxed);
    while (addr > cur_max &&
           !atomic_compare_exchange_weak_explicit(max_p, &cur_max, addr, memory_order_release, memory_order_relaxed)) {
    }
}

#if defined(_WIN32)
static SRWLOCK g_joss_global_locks[JOSS_LOCK_COUNT] = { {0} };
void joss_global_lock(uint32_t slot) {
    if (slot < JOSS_LOCK_COUNT) {
        AcquireSRWLockExclusive(&g_joss_global_locks[slot]);
    }
}
void joss_global_unlock(uint32_t slot) {
    if (slot < JOSS_LOCK_COUNT) {
        ReleaseSRWLockExclusive(&g_joss_global_locks[slot]);
    }
}
#elif defined(__unix__) || defined(__APPLE__)
static pthread_mutex_t g_joss_global_locks[JOSS_LOCK_COUNT] = {
    PTHREAD_MUTEX_INITIALIZER, PTHREAD_MUTEX_INITIALIZER, PTHREAD_MUTEX_INITIALIZER, PTHREAD_MUTEX_INITIALIZER,
    PTHREAD_MUTEX_INITIALIZER, PTHREAD_MUTEX_INITIALIZER, PTHREAD_MUTEX_INITIALIZER, PTHREAD_MUTEX_INITIALIZER,
    PTHREAD_MUTEX_INITIALIZER, PTHREAD_MUTEX_INITIALIZER, PTHREAD_MUTEX_INITIALIZER
};
void joss_global_lock(uint32_t slot) {
    if (slot < JOSS_LOCK_COUNT) {
        pthread_mutex_lock(&g_joss_global_locks[slot]);
    }
}
void joss_global_unlock(uint32_t slot) {
    if (slot < JOSS_LOCK_COUNT) {
        pthread_mutex_unlock(&g_joss_global_locks[slot]);
    }
}
#else
void joss_global_lock(uint32_t slot) { (void)slot; }
void joss_global_unlock(uint32_t slot) { (void)slot; }
#endif

/* =========================================================================
 * Panic & Error Handling
 * ========================================================================= */
static void joss_unwind_if_trying(const char* msg);

void joss_panic(const char* msg) {
    joss_unwind_if_trying(msg);
    fprintf(stderr, "Panic: %s\n", msg ? msg : "(null)");
    exit(1);
}

/* =========================================================================
 * Tagged Dynamic Values (JossBoxedValue)
 * ========================================================================= */
static _Atomic(uintptr_t) g_joss_box_min = UINTPTR_MAX;
static _Atomic(uintptr_t) g_joss_box_max = 0;

bool joss_is_boxed_ptr(int64_t val) {
    uintptr_t u = (uintptr_t)(uint64_t)val;
    uintptr_t mn = atomic_load_explicit(&g_joss_box_min, memory_order_acquire);
    uintptr_t mx = atomic_load_explicit(&g_joss_box_max, memory_order_acquire);
    if (u == 0 || (u & 7U) != 0 || u < mn || u > mx) {
        return false;
    }
    const JossBoxedValue* b = (const JossBoxedValue*)u;
    return (b->header >> 32) == JOSS_BOX_MAGIC;
}

JossBoxedValue* joss_box_value(uint64_t tag, uint64_t payload) {
    JossBoxedValue* box = (JossBoxedValue*)calloc(1, sizeof(JossBoxedValue));
    if (!box) {
        joss_panic("out of memory boxing value");
    }
    box->header = (JOSS_BOX_MAGIC << 32) | (tag & 0xFFFFFFFFULL);
    box->payload = payload;
    joss_atomic_update_bounds(&g_joss_box_min, &g_joss_box_max, (uintptr_t)box);
    return box;
}

int64_t joss_unbox_value(int64_t val) {
    if (!joss_is_boxed_ptr(val)) {
        return val;
    }
    const JossBoxedValue* b = (const JossBoxedValue*)(uintptr_t)(uint64_t)val;
    return (int64_t)b->payload;
}

JossBoxedValue* joss_box_i64(int64_t v) {
    if (joss_is_boxed_ptr(v)) {
        return (JossBoxedValue*)(uintptr_t)(uint64_t)v;
    }
    return joss_box_value(JOSS_TAG_INT, (uint64_t)v);
}

JossBoxedValue* joss_box_f64(double v) {
    uint64_t bits = 0;
    memcpy(&bits, &v, sizeof(bits));
    return joss_box_value(JOSS_TAG_FLOAT, bits);
}

JossBoxedValue* joss_box_bool(bool v) {
    return joss_box_value(JOSS_TAG_BOOL, v ? 1ULL : 0ULL);
}

JossBoxedValue* joss_box_str(const char* s) {
    return joss_box_value(JOSS_TAG_STRING, (uint64_t)(uintptr_t)(s ? s : ""));
}

void* joss_bytes_new(const void* data, size_t len) {
    JossBytesBuf* buf = (JossBytesBuf*)malloc(sizeof(JossBytesBuf) + len + 1);
    if (!buf) return NULL;
    buf->len = (uint64_t)len;
    if (data && len > 0) memcpy(buf->data, data, len);
    buf->data[len] = '\0';
    return (void*)(uintptr_t)joss_box_value(JOSS_TAG_BYTES, (uint64_t)(uintptr_t)buf);
}

const uint8_t* joss_bytes_unwrap(const void* val, size_t* out_len) {
    if (!val) return NULL;
    int64_t v = (int64_t)(uintptr_t)val;
    if (!joss_is_boxed_ptr(v)) return NULL;
    const JossBoxedValue* box = (const JossBoxedValue*)(uintptr_t)(uint64_t)v;
    if ((uint32_t)(box->header & 0xFFFFFFFFULL) != JOSS_TAG_BYTES) return NULL;
    const JossBytesBuf* buf = (const JossBytesBuf*)(uintptr_t)box->payload;
    if (!buf) return NULL;
    if (out_len) *out_len = (size_t)buf->len;
    return buf->data;
}

void* joss_closure_alloc(void* fn_ptr, void* env_map) {
    fn_ptr = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)fn_ptr);
    env_map = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)env_map);
    JossClosureBox* box = (JossClosureBox*)calloc(1, sizeof(JossClosureBox));
    if (!box) {
        joss_panic("out of memory allocating closure box");
    }
    box->header = (JOSS_BOX_MAGIC << 32) | (uint64_t)JOSS_TAG_CLOSURE;
    box->fn_ptr = fn_ptr;
    box->env_map = env_map;
    joss_atomic_update_bounds(&g_joss_box_min, &g_joss_box_max, (uintptr_t)box);
    if (fn_ptr && env_map) {
        joss_closure_env_set(fn_ptr, env_map);
    }
    return box;
}

void* joss_closure_prepare_call(void* callee) {
    uintptr_t u = (uintptr_t)callee;
    if ((uint64_t)u >= JOSS_MIN_PTR_ADDR && ((uint64_t)u >> JOSS_PTR_VA_BITS) == 0 && (u & 7U) == 0) {
        uint64_t header = 0;
        memcpy(&header, (const void*)u, sizeof(header));
        if ((header >> 32) == JOSS_BOX_MAGIC) {
            uint32_t tag = (uint32_t)(header & 0xFFFFFFFFULL);
            if (tag == JOSS_TAG_CLOSURE) {
                const JossClosureBox* box = (const JossClosureBox*)u;
                if (box->fn_ptr && box->env_map) {
                    joss_closure_env_set(box->fn_ptr, box->env_map);
                }
                return box->fn_ptr;
            }
            void* inner = (void*)(uintptr_t)joss_unbox_value((int64_t)u);
            if (inner && inner != callee) {
                return joss_closure_prepare_call(inner);
            }
            return inner;
        }
    }
    return (void*)(uintptr_t)joss_unbox_value((int64_t)u);
}

bool joss_is_not_null(int64_t v) {
    if (v == 0) {
        return false;
    }
    if (joss_is_boxed_ptr(v)) {
        const JossBoxedValue* box = (const JossBoxedValue*)(uintptr_t)(uint64_t)v;
        uint32_t tag = (uint32_t)(box->header & 0xFFFFFFFFULL);
        if (tag == JOSS_TAG_NULL) {
            return false;
        }
    }
    return true;
}

const char* joss_i64_to_str(int64_t v) {
    char* out = (char*)malloc(32);
    if (!out) return "0";
    snprintf(out, 32, "%" PRId64, v);
    return out;
}

const char* joss_f64_to_string(double d) {
    char* out = (char*)malloc(64);
    if (!out) return "0";
    snprintf(out, 64, "%.15g", d);
    if (strcmp(out, "-0") == 0) {
        out[0] = '0';
        out[1] = '\0';
    }
    return out;
}

/* =========================================================================
 * Arena Allocator
 * ========================================================================= */
static JossArenaChunk* arena_new_chunk(size_t cap) {
    JossArenaChunk* c = (JossArenaChunk*)calloc(1, sizeof(JossArenaChunk) + cap);
    if (!c) {
        joss_panic("out of memory allocating arena chunk");
    }
    c->cap = cap;
    c->used = 0;
    c->next = NULL;
    return c;
}

JossArena* joss_arena_create(size_t initial_cap) {
    if (initial_cap < 4096) {
        initial_cap = 4096;
    }
    JossArena* a = (JossArena*)calloc(1, sizeof(JossArena));
    if (!a) {
        joss_panic("out of memory creating arena");
    }
    a->default_chunk_cap = initial_cap;
    a->head = arena_new_chunk(initial_cap);
    return a;
}

void* joss_arena_alloc(JossArena* arena, size_t size) {
    if (!arena) {
        joss_panic("null arena");
    }
    if (size == 0) {
        size = 8;
    }
    size_t aligned = (size + 7U) & ~((size_t)7U);
    JossArenaChunk* cur = arena->head;
    if (!cur || cur->used + aligned > cur->cap) {
        size_t next_cap = arena->default_chunk_cap;
        if (aligned > next_cap) {
            next_cap = aligned;
        }
        JossArenaChunk* next = arena_new_chunk(next_cap);
        next->next = cur;
        arena->head = next;
        cur = next;
    }
    void* ptr = cur->data + cur->used;
    cur->used += aligned;
    return ptr;
}

void joss_arena_reset(JossArena* arena) {
    if (!arena || !arena->head) {
        return;
    }
    JossArenaChunk* cur = arena->head->next;
    while (cur) {
        JossArenaChunk* nxt = cur->next;
        free(cur);
        cur = nxt;
    }
    arena->head->next = NULL;
    arena->head->used = 0;
    memset(arena->head->data, 0, arena->head->cap);
}

void joss_arena_destroy(JossArena* arena) {
    if (!arena) {
        return;
    }
    JossArenaChunk* cur = arena->head;
    while (cur) {
        JossArenaChunk* nxt = cur->next;
        free(cur);
        cur = nxt;
    }
    free(arena);
}

#if defined(__STDC_VERSION__) && __STDC_VERSION__ >= 201112L
static _Thread_local JossArena* tls_req_arena = NULL;
static _Thread_local bool       tls_req_active = false;
#else
static __thread JossArena* tls_req_arena = NULL;
static __thread bool       tls_req_active = false;
#endif

void joss_req_arena_begin(void) {
    if (!tls_req_arena) {
        tls_req_arena = joss_arena_create(65536);
    }
    tls_req_active = true;
}

void joss_req_arena_reset(void) {
    if (tls_req_arena) {
        joss_arena_reset(tls_req_arena);
    }
    tls_req_active = false;
}

void* joss_req_alloc(size_t size) {
    if (tls_req_active && tls_req_arena) {
        return joss_arena_alloc(tls_req_arena, size);
    }
    return malloc(size ? size : 1);
}

static const char* unbox_cstr_rt(const char* s);

char* joss_req_strdup(const char* s) {
    s = unbox_cstr_rt(s);
    if (!s) s = "";
    size_t n = strlen(s);
    char* out = (char*)joss_req_alloc(n + 1);
    if (!out) joss_panic("out of memory duplicating string");
    memcpy(out, s, n + 1);
    return out;
}

/* =========================================================================
 * Bounds-Checked ByteBuffer & StringBuilder
 * ========================================================================= */
JossByteBuffer* joss_buf_new(int64_t initial_cap) {
    if (initial_cap < 16) {
        initial_cap = 16;
    }
    JossByteBuffer* buf = (JossByteBuffer*)calloc(1, sizeof(JossByteBuffer));
    if (!buf) {
        joss_panic("out of memory creating ByteBuffer");
    }
    buf->data = (uint8_t*)calloc((size_t)initial_cap, 1);
    if (!buf->data) {
        free(buf);
        joss_panic("out of memory allocating ByteBuffer data");
    }
    buf->cap = initial_cap;
    buf->len = 0;
    return buf;
}

void joss_buf_free(JossByteBuffer* buf) {
    if (!buf) {
        return;
    }
    free(buf->data);
    buf->data = NULL;
    buf->len = 0;
    buf->cap = 0;
    free(buf);
}

int64_t joss_buf_len(const JossByteBuffer* buf) {
    return buf ? buf->len : 0;
}

uint8_t joss_buf_get(const JossByteBuffer* buf, int64_t idx) {
    if (!buf || idx < 0 || idx >= buf->len) {
        joss_panic("ByteBuffer index out of bounds");
    }
    return buf->data[idx];
}

void joss_buf_set(JossByteBuffer* buf, int64_t idx, uint8_t val) {
    if (!buf || idx < 0 || idx >= buf->len) {
        joss_panic("ByteBuffer index out of bounds");
    }
    buf->data[idx] = val;
}

static void joss_buf_ensure(JossByteBuffer* buf, int64_t extra) {
    if (!buf) {
        joss_panic("null ByteBuffer");
    }
    if (extra <= 0) {
        return;
    }
    if (buf->len + extra <= buf->cap) {
        return;
    }
    int64_t new_cap = buf->cap > 0 ? buf->cap * 2 : 16;
    while (new_cap < buf->len + extra) {
        new_cap *= 2;
    }
    uint8_t* next = (uint8_t*)realloc(buf->data, (size_t)new_cap);
    if (!next) {
        joss_panic("out of memory growing ByteBuffer");
    }
    buf->data = next;
    buf->cap = new_cap;
}

void joss_buf_append_byte(JossByteBuffer* buf, uint8_t val) {
    joss_buf_ensure(buf, 1);
    buf->data[buf->len++] = val;
}

void joss_buf_append_bytes(JossByteBuffer* buf, const uint8_t* data, int64_t len) {
    if (len <= 0 || !data) {
        return;
    }
    joss_buf_ensure(buf, len);
    memcpy(buf->data + buf->len, data, (size_t)len);
    buf->len += len;
}

static const char* unbox_cstr_rt(const char* s);

void joss_buf_append_cstr(JossByteBuffer* buf, const char* s) {
    s = unbox_cstr_rt(s);
    if (!s) return;
    joss_buf_append_bytes(buf, (const uint8_t*)s, (int64_t)strlen(s));
}

char* joss_buf_to_cstr(const JossByteBuffer* buf) {
    int64_t n = buf ? buf->len : 0;
    char* out = (char*)malloc((size_t)n + 1);
    if (!out) {
        joss_panic("out of memory converting ByteBuffer to string");
    }
    if (n > 0 && buf && buf->data) {
        memcpy(out, buf->data, (size_t)n);
    }
    out[n] = '\0';
    return out;
}

JossByteSlice joss_buf_as_slice(const JossByteBuffer* buf) {
    JossByteSlice s;
    s.data = buf ? buf->data : NULL;
    s.len = buf ? buf->len : 0;
    return s;
}

JossStringBuilder* joss_sb_new(void) {
    return joss_buf_new(64);
}

void joss_sb_append(JossStringBuilder* sb, const char* s) {
    size_t blen = 0;
    const uint8_t* bdata = joss_bytes_unwrap(s, &blen);
    if (bdata) {
        if (blen > 0) joss_buf_append_bytes(sb, bdata, (int64_t)blen);
        return;
    }
    s = unbox_cstr_rt(s);
    if (!s) {
        return;
    }
    joss_buf_append_bytes(sb, (const uint8_t*)s, (int64_t)strlen(s));
}

void joss_sb_append_i64(JossStringBuilder* sb, int64_t v) {
    char tmp[32];
    int n = snprintf(tmp, sizeof(tmp), "%" PRId64, v);
    if (n > 0) {
        joss_buf_append_bytes(sb, (const uint8_t*)tmp, (int64_t)n);
    }
}

char* joss_sb_to_string(const JossStringBuilder* sb) {
    return joss_buf_to_cstr(sb);
}

void joss_sb_free(JossStringBuilder* sb) {
    joss_buf_free(sb);
}

/* =========================================================================
 * Core Printing & Strings
 * ========================================================================= */
void joss_print_i64(int64_t v) {
    printf("%" PRId64 "\n", v);
    fflush(stdout);
}

void joss_print_f64(double v) {
    printf("%g\n", v);
    fflush(stdout);
}

void joss_print_bool(bool v) {
    printf("%s\n", v ? "true" : "false");
    fflush(stdout);
}

const char* joss_strval(int64_t v) {
    if (joss_is_arr_ptr(v)) return "[Array]";
    if (joss_is_map_ptr(v)) return "[Map]";
    if (joss_is_boxed_ptr(v)) {
        const JossBoxedValue* box = (const JossBoxedValue*)(uintptr_t)(uint64_t)v;
        uint32_t tag = (uint32_t)(box->header & 0xFFFFFFFFULL);
        if (tag == JOSS_TAG_BYTES) {
            return (const char*)(uintptr_t)v;
        }
        if (tag == JOSS_TAG_STRING) {
            uint64_t p = box->payload;
            if (p < 0x10000ULL || (int64_t)p < 0) return "";
            return (const char*)(uintptr_t)p;
        }
        if (tag == JOSS_TAG_BOOL) return box->payload ? "true" : "false";
        if (tag == JOSS_TAG_NULL) return "";
        if (tag == JOSS_TAG_INT) {
            return joss_i64_to_str((int64_t)box->payload);
        }
        if (tag == JOSS_TAG_FLOAT) {
            double d = 0.0;
            memcpy(&d, &box->payload, sizeof(d));
            return joss_f64_to_string(d);
        }
        if (tag == JOSS_TAG_CLOSURE) {
            return "[Closure]";
        }
    }
    uint64_t u = (uint64_t)v;
    if (u >= JOSS_MIN_PTR_ADDR && (u >> JOSS_PTR_VA_BITS) == 0) {
        return (const char*)(uintptr_t)u;
    }
    return joss_i64_to_str(v);
}

const char* joss_to_string(int64_t v) {
    return joss_strval(v);
}

static const char* unbox_cstr_rt(const char* s) {
    if (!s) return NULL;
    size_t blen = 0;
    const uint8_t* bdata = joss_bytes_unwrap(s, &blen);
    if (bdata) return (const char*)bdata;
    uintptr_t u = (uintptr_t)s;
    int64_t v = (int64_t)u;
    if (joss_is_boxed_ptr(v)) {
        return joss_strval(v);
    }
    if (u < 0x10000ULL || (u >> JOSS_PTR_VA_BITS) != 0) {
        return joss_strval(v);
    }
    return s;
}

static void joss_ensure_utf8_console(void) {
#if defined(_WIN32)
    static volatile LONG g_utf8_inited = 0;
    if (InterlockedCompareExchange(&g_utf8_inited, 1, 0) == 0) {
        SetConsoleOutputCP(CP_UTF8);
        SetConsoleCP(CP_UTF8);
        HANDLE hOut = GetStdHandle(STD_OUTPUT_HANDLE);
        if (hOut != INVALID_HANDLE_VALUE && hOut != NULL) {
            DWORD mode = 0;
            if (GetConsoleMode(hOut, &mode)) {
                SetConsoleMode(hOut, mode | 0x0004);
            }
        }
    }
#endif
}

#if defined(_WIN32) && (defined(__GNUC__) || defined(__clang__))
__attribute__((constructor))
static void joss_win32_utf8_ctor(void) {
    joss_ensure_utf8_console();
}
#endif

void joss_print_string(const char* s) {
    joss_ensure_utf8_console();
    s = unbox_cstr_rt(s);
    if (!s) return;
    printf("%s\n", s);
    fflush(stdout);
}

void joss_print_mixed(int64_t val) {
    if (joss_is_boxed_ptr(val)) {
        const JossBoxedValue* box = (const JossBoxedValue*)(uintptr_t)(uint64_t)val;
        uint32_t tag = (uint32_t)(box->header & 0xFFFFFFFFULL);
        switch (tag) {
            case JOSS_TAG_NULL:
                return;
            case JOSS_TAG_INT:
                joss_print_i64((int64_t)box->payload);
                return;
            case JOSS_TAG_FLOAT: {
                double d = 0.0;
                memcpy(&d, &box->payload, sizeof(d));
                joss_print_f64(d);
                return;
            }
            case JOSS_TAG_BOOL:
                joss_print_bool(box->payload != 0);
                return;
            case JOSS_TAG_STRING:
                joss_print_string((const char*)(uintptr_t)box->payload);
                return;
            case JOSS_TAG_BYTES:
                joss_print_string((const char*)(uintptr_t)val);
                return;
            default:
                joss_print_i64((int64_t)box->payload);
                return;
        }
    }
    uint64_t u = (uint64_t)val;
    if (u == 0 || u < JOSS_MIN_PTR_ADDR || (u >> JOSS_PTR_VA_BITS) != 0) {
        joss_print_i64(val);
        return;
    }
    joss_print_string((const char*)(uintptr_t)u);
}

char* joss_str_concat(const char* a, const char* b) {
    size_t bla = 0, blb = 0;
    const uint8_t* bda = joss_bytes_unwrap(a, &bla);
    const uint8_t* bdb = joss_bytes_unwrap(b, &blb);
    if (bda || bdb) {
        if (!bda) {
            a = unbox_cstr_rt(a);
            bda = (const uint8_t*)a;
            bla = a ? strlen(a) : 0;
        }
        if (!bdb) {
            b = unbox_cstr_rt(b);
            bdb = (const uint8_t*)b;
            blb = b ? strlen(b) : 0;
        }
        char* tmp = (char*)malloc(bla + blb + 1);
        if (!tmp) joss_panic("out of memory in binary string concat");
        if (bla > 0 && bda) memcpy(tmp, bda, bla);
        if (blb > 0 && bdb) memcpy(tmp + bla, bdb, blb);
        tmp[bla + blb] = '\0';
        void* boxed = joss_bytes_new(tmp, bla + blb);
        free(tmp);
        return (char*)boxed;
    }
    a = unbox_cstr_rt(a);
    b = unbox_cstr_rt(b);
    size_t la = a ? strlen(a) : 0;
    size_t lb = b ? strlen(b) : 0;
    char* out = (char*)joss_req_alloc(la + lb + 1);
    if (!out) {
        joss_panic("out of memory in string concat");
    }
    if (la > 0) memcpy(out, a, la);
    if (lb > 0) memcpy(out + la, b, lb);
    out[la + lb] = '\0';
    return out;
}

/* =========================================================================
 * Dynamic Arrays
 * ========================================================================= */
#define JOSS_ARR_MAGIC 0xA11A4001A11A4001ULL
#define JOSS_MAP_MAGIC 0x4A0550024A055002ULL

static _Atomic(uintptr_t) g_joss_arr_min = UINTPTR_MAX;
static _Atomic(uintptr_t) g_joss_arr_max = 0;
static _Atomic(uintptr_t) g_joss_map_min = UINTPTR_MAX;
static _Atomic(uintptr_t) g_joss_map_max = 0;

typedef struct {
    uint64_t magic;
    int64_t  len;
    int64_t  cap;
    int64_t* items;
} JossArray;

void* joss_arr_new(int64_t initial_cap) {
    if (initial_cap < 8) initial_cap = 8;
    JossArray* a = (JossArray*)calloc(1, sizeof(JossArray));
    if (!a) joss_panic("out of memory allocating array");
    a->magic = JOSS_ARR_MAGIC;
    a->cap = initial_cap;
    a->items = (int64_t*)calloc((size_t)initial_cap, sizeof(int64_t));
    if (!a->items) joss_panic("out of memory allocating array items");
    joss_atomic_update_bounds(&g_joss_arr_min, &g_joss_arr_max, (uintptr_t)a);
    return a;
}

static void arr_grow(JossArray* a) {
    int64_t new_cap = a->cap > 0 ? a->cap * 2 : 8;
    int64_t* next = (int64_t*)realloc(a->items, (size_t)new_cap * sizeof(int64_t));
    if (!next) joss_panic("out of memory growing array");
    a->items = next;
    a->cap = new_cap;
}

static void* unbox_ptr(void* p) {
    if (!p) return NULL;
    return (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)p);
}

static const char* unbox_cstr(const char* s) {
    return unbox_cstr_rt(s);
}

void joss_arr_push_i64(void* arr, int64_t val) {
    arr = unbox_ptr(arr);
    JossArray* a = (JossArray*)arr;
    if (!a) joss_panic("null array");
    if (a->len >= a->cap) arr_grow(a);
    a->items[a->len++] = val;
}

void joss_arr_push_str(void* arr, const char* val) {
    joss_arr_push_i64(arr, (int64_t)(uintptr_t)(val ? val : ""));
}

int64_t joss_arr_get_i64(void* arr, int64_t idx) {
    arr = unbox_ptr(arr);
    if (!arr) return 0;
    if (joss_is_map_ptr((int64_t)(uintptr_t)arr)) {
        const char* k = unbox_cstr((const char*)(uintptr_t)idx);
        return joss_map_get_i64(arr, k);
    }
    if (!joss_is_arr_ptr((int64_t)(uintptr_t)arr)) return 0;
    idx = joss_unbox_value(idx);
    JossArray* a = (JossArray*)arr;
    if (idx < 0 || idx >= a->len) return 0;
    return a->items[idx];
}

const char* joss_arr_get_str(void* arr, int64_t idx) {
    int64_t v = joss_arr_get_i64(arr, idx);
    if (v == 0) return "";
    int64_t u = joss_unbox_value(v);
    if (joss_is_arr_ptr(u) || joss_is_map_ptr(u)) return (const char*)(uintptr_t)u;
    return joss_strval(v);
}

void joss_arr_set_i64(void* arr, int64_t idx, int64_t val) {
    arr = unbox_ptr(arr);
    if (!arr) return;
    if (joss_is_map_ptr((int64_t)(uintptr_t)arr)) {
        const char* k = unbox_cstr((const char*)(uintptr_t)idx);
        joss_map_set_i64(arr, k, val);
        return;
    }
    if (!joss_is_arr_ptr((int64_t)(uintptr_t)arr)) return;
    idx = joss_unbox_value(idx);
    JossArray* a = (JossArray*)arr;
    if (idx < 0) return;
    while (idx >= a->cap) arr_grow(a);
    if (idx >= a->len) a->len = idx + 1;
    a->items[idx] = val;
}

void joss_arr_set_str(void* arr, int64_t idx, const char* val) {
    joss_arr_set_i64(arr, idx, (int64_t)(uintptr_t)(val ? val : ""));
}

int64_t joss_arr_len(void* arr) {
    arr = unbox_ptr(arr);
    if (!arr) return 0;
    if (joss_is_map_ptr((int64_t)(uintptr_t)arr)) {
        return joss_map_len(arr);
    }
    if (!joss_is_arr_ptr((int64_t)(uintptr_t)arr)) return 0;
    JossArray* a = (JossArray*)arr;
    return a->len;
}

void* joss_arr_push_ret(void* arr, int64_t val) {
    arr = unbox_ptr(arr);
    if (!arr) {
        arr = joss_arr_new(4);
    }
    joss_arr_push_i64(arr, val);
    return arr;
}

int64_t joss_array_pop(void* arr) {
    arr = unbox_ptr(arr);
    JossArray* a = (JossArray*)arr;
    if (!a || a->len <= 0) return 0;
    return a->items[a->len - 1];
}

/* =========================================================================
 * Associative Maps & Dynamic Objects
 * ========================================================================= */
typedef struct JossMapEntry {
    char*                key;
    int64_t              val;
    struct JossMapEntry* next;
} JossMapEntry;

#define JOSS_MAP_BUCKETS 256

typedef struct {
    uint64_t      magic;
    char*         class_name;
    int64_t       count;
    JossMapEntry* buckets[JOSS_MAP_BUCKETS];
} JossMap;

static uint32_t fnv1a_hash(const char* s) {
    uint32_t h = 2166136261U;
    if (!s) return h;
    while (*s) {
        h ^= (uint8_t)(*s++);
        h *= 16777619U;
    }
    return h;
}

void* joss_map_new(void) {
    JossMap* m = (JossMap*)calloc(1, sizeof(JossMap));
    if (!m) joss_panic("out of memory allocating map");
    m->magic = JOSS_MAP_MAGIC;
    joss_atomic_update_bounds(&g_joss_map_min, &g_joss_map_max, (uintptr_t)m);
    return m;
}

void joss_map_set_i64(void* mp, const char* key, int64_t val) {
    mp = unbox_ptr(mp);
    key = unbox_cstr(key);
    if (!mp || !key || !joss_is_map_ptr((int64_t)(uintptr_t)mp)) return;
    JossMap* m = (JossMap*)mp;
    uint32_t b = fnv1a_hash(key) % JOSS_MAP_BUCKETS;
    JossMapEntry* e = m->buckets[b];
    while (e) {
        if (strcmp(e->key, key) == 0) {
            e->val = val;
            return;
        }
        e = e->next;
    }
    e = (JossMapEntry*)calloc(1, sizeof(JossMapEntry));
    if (!e) joss_panic("out of memory allocating map entry");
    e->key = strdup(key);
    e->val = val;
    e->next = m->buckets[b];
    m->buckets[b] = e;
    m->count++;
}

int64_t joss_map_get_i64(void* mp, const char* key) {
    mp = unbox_ptr(mp);
    if (!mp) return 0;
    if (joss_is_arr_ptr((int64_t)(uintptr_t)mp)) {
        return joss_arr_get_i64(mp, joss_intval((int64_t)(uintptr_t)key));
    }
    if (!joss_is_map_ptr((int64_t)(uintptr_t)mp)) return 0;
    key = unbox_cstr(key);
    if (!key) return 0;
    JossMap* m = (JossMap*)mp;
    uint32_t b = fnv1a_hash(key) % JOSS_MAP_BUCKETS;
    JossMapEntry* e = m->buckets[b];
    while (e) {
        if (strcmp(e->key, key) == 0) {
            return e->val;
        }
        e = e->next;
    }
    return 0;
}

void joss_map_set_str(void* mp, const char* key, const char* val) {
    joss_map_set_i64(mp, key, (int64_t)(uintptr_t)(val ? val : ""));
}

const char* joss_map_get_str(void* mp, const char* key) {
    int64_t v = joss_map_get_i64(mp, key);
    if (v == 0) return "";
    int64_t u = joss_unbox_value(v);
    if (joss_is_arr_ptr(u) || joss_is_map_ptr(u)) return (const char*)(uintptr_t)u;
    return joss_strval(v);
}

void joss_map_delete_i64(void* mp, const char* key) {
    mp = unbox_ptr(mp);
    key = unbox_cstr(key);
    if (!mp || !key || !joss_is_map_ptr((int64_t)(uintptr_t)mp)) return;
    JossMap* m = (JossMap*)mp;
    uint32_t b = fnv1a_hash(key) % JOSS_MAP_BUCKETS;
    JossMapEntry** pp = &m->buckets[b];
    while (*pp) {
        JossMapEntry* cur = *pp;
        if (strcmp(cur->key, key) == 0) {
            *pp = cur->next;
            free(cur->key);
            free(cur);
            m->count--;
            return;
        }
        pp = &cur->next;
    }
}

void* joss_obj_new(const char* class_name) {
    JossMap* m = (JossMap*)joss_map_new();
    m->class_name = class_name ? strdup(class_name) : NULL;
    return m;
}

void joss_obj_set_field_i64(void* obj, const char* field, int64_t val) {
    joss_map_set_i64(obj, field, val);
}

int64_t joss_obj_get_field_i64(void* obj, const char* field) {
    return joss_map_get_i64(obj, field);
}

void joss_obj_set_field_str(void* obj, const char* field, const char* val) {
    joss_map_set_str(obj, field, val);
}

const char* joss_obj_get_field_str(void* obj, const char* field) {
    return joss_map_get_str(obj, field);
}

int64_t joss_intval(int64_t v) {
    if (joss_is_boxed_ptr(v)) {
        const JossBoxedValue* b = (const JossBoxedValue*)(uintptr_t)(uint64_t)v;
        uint32_t tag = (uint32_t)(b->header & 0xFFFFFFFFULL);
        if (tag == JOSS_TAG_STRING) {
            const char* s = (const char*)(uintptr_t)b->payload;
            return s ? (int64_t)strtoll(s, NULL, 10) : 0;
        }
        return (int64_t)b->payload;
    }
    uint64_t u = (uint64_t)v;
    if (u >= JOSS_MIN_PTR_ADDR && (u >> JOSS_PTR_VA_BITS) == 0) {
        const char* s = (const char*)(uintptr_t)u;
        return (int64_t)strtoll(s, NULL, 10);
    }
    return v;
}

int64_t joss_abs(int64_t v) {
    return v < 0 ? -v : v;
}

int64_t joss_min(int64_t a, int64_t b) {
    return a < b ? a : b;
}

int64_t joss_max(int64_t a, int64_t b) {
    return a > b ? a : b;
}

int64_t joss_strlen(const char* s) {
    size_t blen = 0;
    if (joss_bytes_unwrap(s, &blen)) {
        return (int64_t)blen;
    }
    s = unbox_cstr(s);
    if (!s) return 0;
    int64_t runes = 0;
    for (const char* p = s; *p; p++) {
        if (((unsigned char)*p & 0xC0) != 0x80) {
            runes++;
        }
    }
    return runes;
}

int joss_str_contains(const char* s, const char* sub) {
    s = unbox_cstr(s);
    sub = unbox_cstr(sub);
    if (!s || !sub) return 0;
    return strstr(s, sub) != NULL ? 1 : 0;
}


int64_t joss_map_len(void* m) {
    m = unbox_ptr(m);
    if (!m) return 0;
    if (joss_is_arr_ptr((int64_t)(uintptr_t)m)) {
        return ((JossArray*)m)->len;
    }
    if (!joss_is_map_ptr((int64_t)(uintptr_t)m)) return 0;
    JossMap* jm = (JossMap*)m;
    return jm->count;
}

const char* joss_map_key_at(void* m, int64_t idx) {
    m = unbox_ptr(m);
    if (!m || idx < 0 || !joss_is_map_ptr((int64_t)(uintptr_t)m)) return "";
    JossMap* jm = (JossMap*)m;
    int64_t i = 0;
    for (int b = 0; b < JOSS_MAP_BUCKETS; b++) {
        JossMapEntry* cur = jm->buckets[b];
        while (cur) {
            if (i == idx) return cur->key ? cur->key : "";
            i++;
            cur = cur->next;
        }
    }
    return "";
}

int64_t joss_map_val_at(void* m, int64_t idx) {
    m = unbox_ptr(m);
    if (!m || idx < 0) return 0;
    if (joss_is_arr_ptr((int64_t)(uintptr_t)m)) {
        return joss_arr_get_i64(m, idx);
    }
    if (!joss_is_map_ptr((int64_t)(uintptr_t)m)) return 0;
    JossMap* jm = (JossMap*)m;
    int64_t i = 0;
    for (int b = 0; b < JOSS_MAP_BUCKETS; b++) {
        JossMapEntry* cur = jm->buckets[b];
        while (cur) {
            if (i == idx) return cur->val;
            i++;
            cur = cur->next;
        }
    }
    return 0;
}

int64_t joss_array_key_exists(const char* key, void* m) {
    m = unbox_ptr(m);
    key = unbox_cstr(key);
    if (!key || !m || !joss_is_map_ptr((int64_t)(uintptr_t)m)) return 0;
    JossMap* jm = (JossMap*)m;
    uint32_t b = fnv1a_hash(key) % JOSS_MAP_BUCKETS;
    JossMapEntry* cur = jm->buckets[b];
    while (cur) {
        if (cur->key && strcmp(cur->key, key) == 0) return 1;
        cur = cur->next;
    }
    return 0;
}

bool joss_str_eq(const char* a, const char* b) {
    size_t bla = 0, blb = 0;
    const uint8_t* bda = joss_bytes_unwrap(a, &bla);
    const uint8_t* bdb = joss_bytes_unwrap(b, &blb);
    if (bda || bdb) {
        if (!bda) {
            a = unbox_cstr(a);
            bda = (const uint8_t*)a;
            bla = a ? strlen(a) : 0;
        }
        if (!bdb) {
            b = unbox_cstr(b);
            bdb = (const uint8_t*)b;
            blb = b ? strlen(b) : 0;
        }
        if (bla != blb) return false;
        if (bla == 0) return true;
        return memcmp(bda, bdb, bla) == 0;
    }
    a = unbox_cstr(a);
    b = unbox_cstr(b);
    if (a == b) return true;
    if (!a || !b) return false;
    return strcmp(a, b) == 0;
}

bool joss_is_arr_ptr(int64_t v) {
    uintptr_t u = (uintptr_t)(uint64_t)v;
    uintptr_t mn = atomic_load_explicit(&g_joss_arr_min, memory_order_acquire);
    uintptr_t mx = atomic_load_explicit(&g_joss_arr_max, memory_order_acquire);
    if (u == 0 || (u & 7U) != 0 || u < mn || u > mx) return false;
    return ((const JossArray*)u)->magic == JOSS_ARR_MAGIC;
}

bool joss_is_map_ptr(int64_t v) {
    uintptr_t u = (uintptr_t)(uint64_t)v;
    uintptr_t mn = atomic_load_explicit(&g_joss_map_min, memory_order_acquire);
    uintptr_t mx = atomic_load_explicit(&g_joss_map_max, memory_order_acquire);
    if (u == 0 || (u & 7U) != 0 || u < mn || u > mx) return false;
    return ((const JossMap*)u)->magic == JOSS_MAP_MAGIC;
}

#define JOSS_MAX_TRY_DEPTH 64
static _Thread_local jmp_buf     g_joss_try_stack[JOSS_MAX_TRY_DEPTH];
static _Thread_local int         g_joss_try_depth = 0;
static _Thread_local const char* g_joss_last_exc  = NULL;

void joss_try_reset(void) {
    g_joss_try_depth = 0;
    g_joss_last_exc = NULL;
}

void* joss_try_push(void) {
    if (g_joss_try_depth >= JOSS_MAX_TRY_DEPTH) {
        joss_panic("JOSS-EXC-001: try/catch nesting depth exceeded");
    }
    return (void*)&g_joss_try_stack[g_joss_try_depth++];
}

void joss_try_pop(void) {
    if (g_joss_try_depth > 0) {
        g_joss_try_depth--;
    }
}

const char* joss_catch_msg(void) {
    return g_joss_last_exc ? g_joss_last_exc : "";
}

static void joss_unwind_if_trying(const char* msg) {
    if (g_joss_try_depth > 0) {
        g_joss_last_exc = msg ? msg : "Joss Runtime Panic";
        g_joss_try_depth--;
        longjmp(g_joss_try_stack[g_joss_try_depth], 1);
    }
}

void joss_throw(const char* msg) {
    joss_unwind_if_trying(msg ? msg : "Joss Exception");
    joss_panic(msg ? msg : "Joss Exception");
}

/* =========================================================================
 * Lexical Closure Environment Table (TLS + Global Cross-Thread Fallback)
 * ========================================================================= */
#define JOSS_TLS_CLOSURE_SLOTS    64
#define JOSS_GLOBAL_CLOSURE_SLOTS 256

typedef struct {
    void* fn_ptr;
    void* env_map;
} JossClosureEnvSlot;

static _Thread_local JossClosureEnvSlot g_tls_closure_envs[JOSS_TLS_CLOSURE_SLOTS];
static _Thread_local int                g_tls_closure_env_count = 0;

static JossClosureEnvSlot g_global_closure_envs[JOSS_GLOBAL_CLOSURE_SLOTS];
static int                g_global_closure_env_count = 0;

void joss_closure_env_set(void* fn_ptr, void* env_map) {
    fn_ptr = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)fn_ptr);
    env_map = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)env_map);
    if (!fn_ptr || !env_map) return;

    int found_tls = -1;
    for (int i = g_tls_closure_env_count - 1; i >= 0; i--) {
        if (g_tls_closure_envs[i].fn_ptr == fn_ptr) {
            found_tls = i;
            break;
        }
    }
    if (found_tls >= 0) {
        g_tls_closure_envs[found_tls].env_map = env_map;
    } else if (g_tls_closure_env_count < JOSS_TLS_CLOSURE_SLOTS) {
        g_tls_closure_envs[g_tls_closure_env_count].fn_ptr = fn_ptr;
        g_tls_closure_envs[g_tls_closure_env_count].env_map = env_map;
        g_tls_closure_env_count++;
    } else {
        g_tls_closure_envs[JOSS_TLS_CLOSURE_SLOTS - 1].fn_ptr = fn_ptr;
        g_tls_closure_envs[JOSS_TLS_CLOSURE_SLOTS - 1].env_map = env_map;
    }

    joss_global_lock(JOSS_LOCK_GENERAL);
    int found_glob = -1;
    for (int i = g_global_closure_env_count - 1; i >= 0; i--) {
        if (g_global_closure_envs[i].fn_ptr == fn_ptr) {
            found_glob = i;
            break;
        }
    }
    if (found_glob >= 0) {
        g_global_closure_envs[found_glob].env_map = env_map;
    } else if (g_global_closure_env_count < JOSS_GLOBAL_CLOSURE_SLOTS) {
        g_global_closure_envs[g_global_closure_env_count].fn_ptr = fn_ptr;
        g_global_closure_envs[g_global_closure_env_count].env_map = env_map;
        g_global_closure_env_count++;
    } else {
        g_global_closure_envs[JOSS_GLOBAL_CLOSURE_SLOTS - 1].fn_ptr = fn_ptr;
        g_global_closure_envs[JOSS_GLOBAL_CLOSURE_SLOTS - 1].env_map = env_map;
    }
    joss_global_unlock(JOSS_LOCK_GENERAL);
}

void* joss_closure_env_get(void* fn_ptr) {
    fn_ptr = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)fn_ptr);
    if (fn_ptr) {
        for (int i = g_tls_closure_env_count - 1; i >= 0; i--) {
            if (g_tls_closure_envs[i].fn_ptr == fn_ptr && g_tls_closure_envs[i].env_map) {
                return g_tls_closure_envs[i].env_map;
            }
        }
        joss_global_lock(JOSS_LOCK_GENERAL);
        void* found = NULL;
        for (int i = g_global_closure_env_count - 1; i >= 0; i--) {
            if (g_global_closure_envs[i].fn_ptr == fn_ptr && g_global_closure_envs[i].env_map) {
                found = g_global_closure_envs[i].env_map;
                break;
            }
        }
        joss_global_unlock(JOSS_LOCK_GENERAL);
        if (found) return found;
    }
    return joss_map_new();
}

void joss_closure_env_tls_reset(void) {
    g_tls_closure_env_count = 0;
}

/* =========================================================================
 * Schema DB Bridge Hooks (shared between joss_db_rt.c and joss_web_rt.c)
 * ========================================================================= */
JossSchemaDbQueryFn g_joss_schema_db_query = NULL;
JossSchemaDbExecFn  g_joss_schema_db_exec  = NULL;

void joss_register_schema_db(JossSchemaDbQueryFn q, JossSchemaDbExecFn e) {
    g_joss_schema_db_query = q;
    g_joss_schema_db_exec  = e;
}

