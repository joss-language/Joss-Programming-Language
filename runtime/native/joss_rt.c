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
 * Native Stack Trace (Zero-Cost Thread-Local Call Frame Stack)
 * ========================================================================= */
#define JOSS_MAX_CALL_DEPTH 256

typedef struct {
    const char* func;
    const char* file;
    int64_t     line;
} JossStackFrame;

#if defined(__STDC_VERSION__) && __STDC_VERSION__ >= 201112L
static _Thread_local JossStackFrame g_joss_trace_frames[JOSS_MAX_CALL_DEPTH];
static _Thread_local int            g_joss_trace_depth = 0;
#else
static __thread JossStackFrame g_joss_trace_frames[JOSS_MAX_CALL_DEPTH];
static __thread int            g_joss_trace_depth = 0;
#endif

void joss_trace_push(const char* func, const char* file, int64_t line) {
    if (g_joss_trace_depth < JOSS_MAX_CALL_DEPTH) {
        JossStackFrame* f = &g_joss_trace_frames[g_joss_trace_depth];
        f->func = func ? func : "<anonymous>";
        f->file = file ? file : "<native>";
        f->line = line;
    }
    g_joss_trace_depth++;
}

void joss_trace_pop(void) {
    if (g_joss_trace_depth > 0) {
        g_joss_trace_depth--;
    }
}

void joss_trace_set_line(int64_t line) {
    if (g_joss_trace_depth > 0 && g_joss_trace_depth <= JOSS_MAX_CALL_DEPTH) {
        g_joss_trace_frames[g_joss_trace_depth - 1].line = line;
    }
}

int64_t joss_trace_depth(void) {
    return (int64_t)g_joss_trace_depth;
}

void joss_trace_reset(void) {
    g_joss_trace_depth = 0;
}

void joss_print_stack_trace(void) {
    if (g_joss_trace_depth <= 0) return;
    fprintf(stderr, "Stack trace:\n");
    int top = g_joss_trace_depth < JOSS_MAX_CALL_DEPTH ? g_joss_trace_depth : JOSS_MAX_CALL_DEPTH;
    for (int i = top - 1; i >= 0; i--) {
        const JossStackFrame* f = &g_joss_trace_frames[i];
        if (f->line > 0) {
            fprintf(stderr, "  at %s (%s:%lld)\n",
                    f->func ? f->func : "<anonymous>",
                    f->file ? f->file : "<native>",
                    (long long)f->line);
        } else {
            fprintf(stderr, "  at %s (%s)\n",
                    f->func ? f->func : "<anonymous>",
                    f->file ? f->file : "<native>");
        }
    }
    if (g_joss_trace_depth > JOSS_MAX_CALL_DEPTH) {
        fprintf(stderr, "  ... (%d more frames omitted)\n", g_joss_trace_depth - JOSS_MAX_CALL_DEPTH);
    }
}

/* =========================================================================
 * Panic & Error Handling
 * ========================================================================= */
static void joss_unwind_if_trying(const char* msg);

void joss_panic(const char* msg) {
    joss_unwind_if_trying(msg);
    fprintf(stderr, "Panic: %s\n", msg ? msg : "(null)");
    joss_print_stack_trace();
    exit(1);
}

/* =========================================================================
 * Tagged Dynamic Values (JossBoxedValue)
 * ========================================================================= */
static _Atomic(uintptr_t) g_joss_box_min = UINTPTR_MAX;
static _Atomic(uintptr_t) g_joss_box_max = 0;

bool joss_is_boxed_ptr(int64_t val) {
    uintptr_t u = (uintptr_t)(uint64_t)val;
    if (u < JOSS_MIN_PTR_ADDR || (u & 7U) != 0) {
        return false;
    }
    uintptr_t mn = atomic_load_explicit(&g_joss_box_min, memory_order_relaxed);
    uintptr_t mx = atomic_load_explicit(&g_joss_box_max, memory_order_relaxed);
    if (u < mn || u > mx) {
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

static _Thread_local char*  g_tls_str_slab = NULL;
static _Thread_local size_t g_tls_str_used = 0;
static _Thread_local size_t g_tls_str_cap  = 0;

static bool joss_req_is_active(void);

static inline char* joss_tls_str_alloc(size_t n) {
    if (__builtin_expect(joss_req_is_active(), 0)) {
        return (char*)joss_req_alloc(n);
    }
    if (__builtin_expect(n > 4096, 0)) {
        return (char*)malloc(n);
    }
    if (__builtin_expect(g_tls_str_used + n > g_tls_str_cap, 0)) {
        size_t cap = 262144; /* 256 KB per slab */
        char* slab = (char*)malloc(cap);
        if (!slab) return (char*)malloc(n);
        g_tls_str_slab = slab;
        g_tls_str_used = 0;
        g_tls_str_cap = cap;
    }
    char* p = g_tls_str_slab + g_tls_str_used;
    g_tls_str_used += n;
    return p;
}

const char* joss_i64_to_str(int64_t v) {
    char tmp[32];
    int neg = 0;
    uint64_t uv;
    if (v < 0) {
        neg = 1;
        uv = (uint64_t)(-(v + 1)) + 1ULL;
    } else {
        uv = (uint64_t)v;
    }
    int pos = 31;
    tmp[pos] = '\0';
    do {
        tmp[--pos] = (char)('0' + (uv % 10ULL));
        uv /= 10ULL;
    } while (uv > 0ULL);
    if (neg) tmp[--pos] = '-';
    size_t len = (size_t)(32 - pos);
    char* out = joss_tls_str_alloc(len);
    if (!out) return "0";
    memcpy(out, tmp + pos, len);
    return out;
}

const char* joss_f64_to_string(double d) {
    char* out = joss_tls_str_alloc(64);
    if (!out) return "0";
    snprintf(out, 64, "%.15g", d);
    if (strcmp(out, "-0") == 0) {
        out[0] = '0';
        out[1] = '\0';
    }
    return out;
}

/* =========================================================================
 * Arena Allocator + Per-Request Heap Tracker
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

static bool joss_req_is_active(void) {
    return tls_req_active && tls_req_arena != NULL;
}

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

size_t joss_req_arena_bytes_used(void) {
    if (!tls_req_arena) return 0;
    size_t total = 0;
    for (JossArenaChunk* c = tls_req_arena->head; c != NULL; c = c->next) {
        total += c->used;
    }
    return total;
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
    if (!arr) joss_panic("null array");
    JossArray* a = (JossArray*)arr;
    if (__builtin_expect(a->magic != JOSS_ARR_MAGIC, 0)) {
        a = (JossArray*)unbox_ptr(arr);
        if (!a) joss_panic("null array");
    }
    if (__builtin_expect(a->len >= a->cap, 0)) arr_grow(a);
    a->items[a->len++] = val;
}

void joss_arr_push_str(void* arr, const char* val) {
    joss_arr_push_i64(arr, (int64_t)(uintptr_t)(val ? val : ""));
}

int64_t joss_arr_get_i64(void* arr, int64_t idx) {
    if (!arr) return 0;
    JossArray* a = (JossArray*)arr;
    if (__builtin_expect(a->magic == JOSS_ARR_MAGIC, 1)) {
        if (__builtin_expect((uint64_t)idx >= JOSS_MIN_PTR_ADDR, 0)) {
            idx = joss_unbox_value(idx);
        }
        if (__builtin_expect((uint64_t)idx < (uint64_t)a->len, 1)) {
            return a->items[idx];
        }
        return 0;
    }
    arr = unbox_ptr(arr);
    if (!arr) return 0;
    if (joss_is_map_ptr((int64_t)(uintptr_t)arr)) {
        const char* k = unbox_cstr((const char*)(uintptr_t)idx);
        return joss_map_get_i64(arr, k);
    }
    if (!joss_is_arr_ptr((int64_t)(uintptr_t)arr)) return 0;
    idx = joss_unbox_value(idx);
    a = (JossArray*)arr;
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
    if (!arr) return;
    JossArray* a = (JossArray*)arr;
    if (__builtin_expect(a->magic == JOSS_ARR_MAGIC, 1)) {
        if (__builtin_expect((uint64_t)idx >= JOSS_MIN_PTR_ADDR, 0)) {
            idx = joss_unbox_value(idx);
        }
        if (__builtin_expect(idx < 0, 0)) return;
        if (__builtin_expect(idx >= a->cap, 0)) {
            while (idx >= a->cap) arr_grow(a);
        }
        if (idx >= a->len) a->len = idx + 1;
        a->items[idx] = val;
        return;
    }
    arr = unbox_ptr(arr);
    if (!arr) return;
    if (joss_is_map_ptr((int64_t)(uintptr_t)arr)) {
        const char* k = unbox_cstr((const char*)(uintptr_t)idx);
        joss_map_set_i64(arr, k, val);
        return;
    }
    if (!joss_is_arr_ptr((int64_t)(uintptr_t)arr)) return;
    idx = joss_unbox_value(idx);
    a = (JossArray*)arr;
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
#define JOSS_MAP_INLINE_CAP 4

typedef struct {
    char*    key;
    uint32_t hash;
    uint8_t  deleted;
    uint8_t  key_owned;
    int64_t  val;
} JossMapEntry;

typedef struct {
    uint64_t      magic;
    const char*   class_name;
    int64_t       count;
    int32_t       entry_len;
    int32_t       entry_cap;
    int32_t       slot_cap;
    int32_t       slot_used;
    JossMapEntry* entries;
    int32_t*      slots; /* -1 = empty, -2 = tombstone, >= 0 = index in entries */
    JossMapEntry  inline_entries[JOSS_MAP_INLINE_CAP];
} JossMap;

static _Thread_local JossMap* g_tls_map_slab = NULL;
static _Thread_local size_t   g_tls_map_used = 0;
static _Thread_local size_t   g_tls_map_cap  = 0;

static inline JossMap* joss_map_alloc_raw(void) {
    if (__builtin_expect(g_tls_map_used >= g_tls_map_cap, 0)) {
        size_t cap = 2048; /* ~512 KB per slab -> 1 atomic bounds update per 2048 objects */
        JossMap* slab = (JossMap*)malloc(cap * sizeof(JossMap));
        if (!slab) joss_panic("out of memory allocating map slab");
        joss_atomic_update_bounds(&g_joss_map_min, &g_joss_map_max, (uintptr_t)&slab[0]);
        joss_atomic_update_bounds(&g_joss_map_min, &g_joss_map_max, (uintptr_t)&slab[cap - 1]);
        g_tls_map_slab = slab;
        g_tls_map_used = 0;
        g_tls_map_cap = cap;
    }
    return &g_tls_map_slab[g_tls_map_used++];
}

static inline uint32_t fnv1a_hash(const char* s) {
    uint32_t h = 2166136261U;
    if (!s) return h;
    while (*s) {
        h ^= (uint8_t)(*s++);
        h *= 16777619U;
    }
    return h;
}

static void joss_map_rehash(JossMap* m, int32_t new_slot_cap) {
    if (new_slot_cap < 16) new_slot_cap = 16;
    if (m->entries == m->inline_entries) {
        int32_t ncap = 16;
        JossMapEntry* heap_e = (JossMapEntry*)malloc((size_t)ncap * sizeof(JossMapEntry));
        if (!heap_e) joss_panic("out of memory promoting map entries");
        if (m->entry_len > 0) {
            memcpy(heap_e, m->inline_entries, (size_t)m->entry_len * sizeof(JossMapEntry));
        }
        m->entries = heap_e;
        m->entry_cap = ncap;
    }
    int32_t* new_slots = (int32_t*)malloc((size_t)new_slot_cap * sizeof(int32_t));
    if (!new_slots) joss_panic("out of memory rehashing map");
    for (int32_t i = 0; i < new_slot_cap; i++) new_slots[i] = -1;

    uint32_t mask = (uint32_t)(new_slot_cap - 1);
    int32_t live = 0;
    for (int32_t i = 0; i < m->entry_len; i++) {
        if (m->entries[i].deleted) continue;
        if (live != i) m->entries[live] = m->entries[i];
        if (m->entries[live].hash == 0 && m->entries[live].key) {
            m->entries[live].hash = fnv1a_hash(m->entries[live].key);
        }
        uint32_t pos = m->entries[live].hash & mask;
        while (new_slots[pos] >= 0) {
            pos = (pos + 1U) & mask;
        }
        new_slots[pos] = live;
        live++;
    }
    free(m->slots);
    m->slots = new_slots;
    m->slot_cap = new_slot_cap;
    m->entry_len = live;
    m->slot_used = live;
    m->count = live;
}

void* joss_map_new(void) {
    JossMap* m = joss_map_alloc_raw();
    m->magic = JOSS_MAP_MAGIC;
    m->class_name = NULL;
    m->count = 0;
    m->entry_len = 0;
    m->entry_cap = JOSS_MAP_INLINE_CAP;
    m->slot_cap = 0;
    m->slot_used = 0;
    m->entries = m->inline_entries;
    m->slots = NULL;
    return m;
}

static inline void joss_map_set_internal(JossMap* m, const char* key, int64_t val, uint8_t dup_key) {
    if (m->slot_cap == 0) {
        /* Inline small-map / small-object fast path (<= 4 entries, zero malloc/hash) */
        for (int32_t i = 0; i < m->entry_len; i++) {
            JossMapEntry* e = &m->entries[i];
            if (!e->deleted && (e->key == key || strcmp(e->key, key) == 0)) {
                e->val = val;
                return;
            }
        }
        if (m->entry_len < JOSS_MAP_INLINE_CAP) {
            int32_t idx = m->entry_len++;
            m->entries[idx].key = dup_key ? strdup(key) : (char*)key;
            m->entries[idx].hash = 0;
            m->entries[idx].deleted = 0;
            m->entries[idx].key_owned = dup_key;
            m->entries[idx].val = val;
            m->count++;
            return;
        }
        joss_map_rehash(m, 32);
    } else if ((m->slot_used + 1) * 10 >= m->slot_cap * 7) {
        int32_t target = (m->count * 2 >= m->slot_cap) ? (m->slot_cap * 2) : m->slot_cap;
        joss_map_rehash(m, target);
    }

    uint32_t h = fnv1a_hash(key);
    uint32_t mask = (uint32_t)(m->slot_cap - 1);
    uint32_t pos = h & mask;
    int32_t first_tomb = -1;

    for (;;) {
        int32_t idx = m->slots[pos];
        if (idx == -1) {
            uint32_t insert_pos = (first_tomb >= 0) ? (uint32_t)first_tomb : pos;
            if (m->entry_len >= m->entry_cap) {
                int32_t ncap = m->entry_cap * 2;
                JossMapEntry* ne = (JossMapEntry*)realloc(m->entries, (size_t)ncap * sizeof(JossMapEntry));
                if (!ne) joss_panic("out of memory growing map entries");
                m->entries = ne;
                m->entry_cap = ncap;
            }
            int32_t new_idx = m->entry_len++;
            m->entries[new_idx].key = dup_key ? strdup(key) : (char*)key;
            m->entries[new_idx].hash = h;
            m->entries[new_idx].deleted = 0;
            m->entries[new_idx].key_owned = dup_key;
            m->entries[new_idx].val = val;
            m->slots[insert_pos] = new_idx;
            if (first_tomb < 0) m->slot_used++;
            m->count++;
            return;
        }
        if (idx == -2) {
            if (first_tomb < 0) first_tomb = (int32_t)pos;
        } else {
            JossMapEntry* e = &m->entries[idx];
            if (e->key == key || (e->hash == h && strcmp(e->key, key) == 0)) {
                e->val = val;
                return;
            }
        }
        pos = (pos + 1U) & mask;
    }
}

static inline int64_t joss_map_get_internal(JossMap* m, const char* key) {
    if (m->count == 0) return 0;
    if (m->slot_cap == 0) {
        for (int32_t i = 0; i < m->entry_len; i++) {
            JossMapEntry* e = &m->entries[i];
            if (!e->deleted && (e->key == key || strcmp(e->key, key) == 0)) {
                return e->val;
            }
        }
        return 0;
    }
    uint32_t h = fnv1a_hash(key);
    uint32_t mask = (uint32_t)(m->slot_cap - 1);
    uint32_t pos = h & mask;
    for (;;) {
        int32_t idx = m->slots[pos];
        if (idx == -1) return 0;
        if (idx >= 0) {
            JossMapEntry* e = &m->entries[idx];
            if (e->key == key || (e->hash == h && strcmp(e->key, key) == 0)) {
                return e->val;
            }
        }
        pos = (pos + 1U) & mask;
    }
}

void joss_map_set_i64(void* mp, const char* key, int64_t val) {
    if (!mp || !key) return;
    JossMap* m = (JossMap*)mp;
    if (__builtin_expect(m->magic != JOSS_MAP_MAGIC, 0)) {
        mp = unbox_ptr(mp);
        if (!mp || !joss_is_map_ptr((int64_t)(uintptr_t)mp)) return;
        m = (JossMap*)mp;
    }
    if (__builtin_expect(joss_is_boxed_ptr((int64_t)(uintptr_t)key), 0)) {
        key = unbox_cstr(key);
        if (!key) return;
    }
    joss_map_set_internal(m, key, val, 1);
}

int64_t joss_map_get_i64(void* mp, const char* key) {
    if (!mp || !key) return 0;
    JossMap* m = (JossMap*)mp;
    if (__builtin_expect(m->magic == JOSS_MAP_MAGIC, 1)) {
        if (__builtin_expect(joss_is_boxed_ptr((int64_t)(uintptr_t)key), 0)) {
            key = unbox_cstr(key);
            if (!key) return 0;
        }
        return joss_map_get_internal(m, key);
    }
    mp = unbox_ptr(mp);
    if (!mp) return 0;
    if (joss_is_arr_ptr((int64_t)(uintptr_t)mp)) {
        return joss_arr_get_i64(mp, joss_intval((int64_t)(uintptr_t)key));
    }
    if (!joss_is_map_ptr((int64_t)(uintptr_t)mp)) return 0;
    key = unbox_cstr(key);
    if (!key) return 0;
    return joss_map_get_internal((JossMap*)mp, key);
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
    if (m->count == 0) return;
    if (m->slot_cap == 0) {
        for (int32_t i = 0; i < m->entry_len; i++) {
            JossMapEntry* e = &m->entries[i];
            if (!e->deleted && (e->key == key || strcmp(e->key, key) == 0)) {
                if (e->key_owned) free(e->key);
                e->key = NULL;
                e->deleted = 1;
                m->count--;
                return;
            }
        }
        return;
    }

    uint32_t h = fnv1a_hash(key);
    uint32_t mask = (uint32_t)(m->slot_cap - 1);
    uint32_t pos = h & mask;
    for (;;) {
        int32_t idx = m->slots[pos];
        if (idx == -1) return;
        if (idx >= 0) {
            JossMapEntry* e = &m->entries[idx];
            if (e->key == key || (e->hash == h && strcmp(e->key, key) == 0)) {
                if (e->key_owned) free(e->key);
                e->key = NULL;
                e->deleted = 1;
                m->slots[pos] = -2;
                m->count--;
                return;
            }
        }
        pos = (pos + 1U) & mask;
    }
}

/* =========================================================================
 * Nominal Flat-Slot Objects (Compile-Time Fixed Offset Layout)
 * ========================================================================= */
#define JOSS_NOMINAL_MAGIC 0x4A4F534E4F424A31ULL /* "JOSNOBJ1" */

typedef struct JossNominalObj {
    uint64_t           magic;        /* offset 0 */
    const char*        class_name;   /* offset 8 */
    int64_t            num_fields;   /* offset 16 */
    const char* const* field_names;  /* offset 24 */
    void*              overflow_map; /* offset 32 */
    int64_t            fields[];     /* offset 40 */
} JossNominalObj;

static _Atomic(uintptr_t) g_joss_nom_min = UINTPTR_MAX;
static _Atomic(uintptr_t) g_joss_nom_max = 0;

#if defined(__STDC_VERSION__) && __STDC_VERSION__ >= 201112L
static _Thread_local uint8_t* g_tls_nom_slab = NULL;
static _Thread_local size_t   g_tls_nom_used = 0;
static _Thread_local size_t   g_tls_nom_cap  = 0;
#else
static __thread uint8_t* g_tls_nom_slab = NULL;
static __thread size_t   g_tls_nom_used = 0;
static __thread size_t   g_tls_nom_cap  = 0;
#endif

static inline JossNominalObj* joss_nom_alloc_raw(size_t bytes) {
    size_t aligned = (bytes + 7U) & ~((size_t)7U);
    if (__builtin_expect(joss_req_is_active(), 0)) {
        JossNominalObj* o = (JossNominalObj*)joss_req_alloc(aligned);
        memset(o, 0, aligned);
        joss_atomic_update_bounds(&g_joss_nom_min, &g_joss_nom_max, (uintptr_t)o);
        return o;
    }
    if (__builtin_expect(aligned > 4096, 0)) {
        JossNominalObj* o = (JossNominalObj*)calloc(1, aligned);
        if (!o) joss_panic("out of memory allocating nominal object");
        joss_atomic_update_bounds(&g_joss_nom_min, &g_joss_nom_max, (uintptr_t)o);
        return o;
    }
    if (__builtin_expect(g_tls_nom_used + aligned > g_tls_nom_cap, 0)) {
        size_t cap = 262144; /* 256 KB per TLS slab */
        uint8_t* slab = (uint8_t*)calloc(1, cap);
        if (!slab) joss_panic("out of memory allocating nominal object slab");
        joss_atomic_update_bounds(&g_joss_nom_min, &g_joss_nom_max, (uintptr_t)slab);
        joss_atomic_update_bounds(&g_joss_nom_min, &g_joss_nom_max, (uintptr_t)(slab + cap - 8));
        g_tls_nom_slab = slab;
        g_tls_nom_used = 0;
        g_tls_nom_cap = cap;
    }
    JossNominalObj* o = (JossNominalObj*)(g_tls_nom_slab + g_tls_nom_used);
    g_tls_nom_used += aligned;
    return o;
}

void* joss_nominal_obj_new(const char* class_name, int64_t num_fields, const char* const* field_names) {
    if (num_fields < 0) num_fields = 0;
    size_t bytes = sizeof(JossNominalObj) + (size_t)num_fields * sizeof(int64_t);
    JossNominalObj* o = joss_nom_alloc_raw(bytes);
    o->magic = JOSS_NOMINAL_MAGIC;
    o->class_name = class_name;
    o->num_fields = num_fields;
    o->field_names = field_names;
    o->overflow_map = NULL;
    return o;
}

bool joss_is_nominal_ptr(int64_t v) {
    uintptr_t u = (uintptr_t)(uint64_t)v;
    if (u < JOSS_MIN_PTR_ADDR || (u & 7U) != 0) return false;
    uintptr_t mn = atomic_load_explicit(&g_joss_nom_min, memory_order_relaxed);
    uintptr_t mx = atomic_load_explicit(&g_joss_nom_max, memory_order_relaxed);
    if (u < mn || u > mx) return false;
    return ((const JossNominalObj*)u)->magic == JOSS_NOMINAL_MAGIC;
}

int64_t joss_nominal_get_slot(void* obj, int64_t slot_idx) {
    if (!obj || slot_idx < 0) return 0;
    JossNominalObj* o = (JossNominalObj*)obj;
    if (__builtin_expect(o->magic == JOSS_NOMINAL_MAGIC && slot_idx < o->num_fields, 1)) {
        return o->fields[slot_idx];
    }
    return 0;
}

void joss_nominal_set_slot(void* obj, int64_t slot_idx, int64_t val) {
    if (!obj || slot_idx < 0) return;
    JossNominalObj* o = (JossNominalObj*)obj;
    if (__builtin_expect(o->magic == JOSS_NOMINAL_MAGIC && slot_idx < o->num_fields, 1)) {
        o->fields[slot_idx] = val;
    }
}

static inline int64_t joss_nominal_find_field(const JossNominalObj* o, const char* field) {
    if (!o || !field || !o->field_names) return -1;
    for (int64_t i = 0; i < o->num_fields; i++) {
        const char* fn = o->field_names[i];
        if (fn == field || (fn && strcmp(fn, field) == 0)) {
            return i;
        }
    }
    return -1;
}

static void joss_nominal_set_field_by_name(JossNominalObj* o, const char* field, int64_t val) {
    int64_t idx = joss_nominal_find_field(o, field);
    if (idx >= 0) {
        o->fields[idx] = val;
        return;
    }
    if (!o->overflow_map) {
        o->overflow_map = joss_map_new();
    }
    joss_map_set_internal((JossMap*)o->overflow_map, field, val, 0);
}

static int64_t joss_nominal_get_field_by_name(const JossNominalObj* o, const char* field) {
    int64_t idx = joss_nominal_find_field(o, field);
    if (idx >= 0) {
        return o->fields[idx];
    }
    if (o->overflow_map) {
        return joss_map_get_internal((JossMap*)o->overflow_map, field);
    }
    return 0;
}

void* joss_obj_new(const char* class_name) {
    JossMap* m = (JossMap*)joss_map_new();
    m->class_name = class_name;
    return m;
}

void joss_obj_set_field_i64(void* obj, const char* field, int64_t val) {
    if (!obj || !field) return;
    uint64_t magic = *(const uint64_t*)obj;
    if (__builtin_expect(magic == JOSS_NOMINAL_MAGIC, 1)) {
        joss_nominal_set_field_by_name((JossNominalObj*)obj, field, val);
        return;
    }
    if (__builtin_expect(magic == JOSS_MAP_MAGIC, 1)) {
        joss_map_set_internal((JossMap*)obj, field, val, 0);
        return;
    }
    obj = unbox_ptr(obj);
    if (!obj) return;
    if (joss_is_nominal_ptr((int64_t)(uintptr_t)obj)) {
        joss_nominal_set_field_by_name((JossNominalObj*)obj, field, val);
        return;
    }
    joss_map_set_i64(obj, field, val);
}

int64_t joss_obj_get_field_i64(void* obj, const char* field) {
    if (!obj || !field) return 0;
    uint64_t magic = *(const uint64_t*)obj;
    if (__builtin_expect(magic == JOSS_NOMINAL_MAGIC, 1)) {
        return joss_nominal_get_field_by_name((const JossNominalObj*)obj, field);
    }
    if (__builtin_expect(magic == JOSS_MAP_MAGIC, 1)) {
        return joss_map_get_internal((JossMap*)obj, field);
    }
    obj = unbox_ptr(obj);
    if (!obj) return 0;
    if (joss_is_nominal_ptr((int64_t)(uintptr_t)obj)) {
        return joss_nominal_get_field_by_name((const JossNominalObj*)obj, field);
    }
    return joss_map_get_i64(obj, field);
}

void joss_obj_set_field_str(void* obj, const char* field, const char* val) {
    joss_obj_set_field_i64(obj, field, (int64_t)(uintptr_t)(val ? val : ""));
}

const char* joss_obj_get_field_str(void* obj, const char* field) {
    int64_t v = joss_obj_get_field_i64(obj, field);
    if (v == 0) return "";
    int64_t u = joss_unbox_value(v);
    if (joss_is_arr_ptr(u) || joss_is_map_ptr(u) || joss_is_nominal_ptr(u)) return (const char*)(uintptr_t)u;
    return joss_strval(v);
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
    if (joss_is_nominal_ptr((int64_t)(uintptr_t)m)) {
        const JossNominalObj* o = (const JossNominalObj*)m;
        int64_t extra = o->overflow_map ? joss_map_len(o->overflow_map) : 0;
        return o->num_fields + extra;
    }
    if (!joss_is_map_ptr((int64_t)(uintptr_t)m)) return 0;
    JossMap* jm = (JossMap*)m;
    return jm->count;
}

const char* joss_map_key_at(void* m, int64_t idx) {
    m = unbox_ptr(m);
    if (!m || idx < 0) return "";
    if (joss_is_nominal_ptr((int64_t)(uintptr_t)m)) {
        const JossNominalObj* o = (const JossNominalObj*)m;
        if (idx < o->num_fields) {
            return (o->field_names && o->field_names[idx]) ? o->field_names[idx] : "";
        }
        if (o->overflow_map) {
            return joss_map_key_at(o->overflow_map, idx - o->num_fields);
        }
        return "";
    }
    if (!joss_is_map_ptr((int64_t)(uintptr_t)m)) return "";
    JossMap* jm = (JossMap*)m;
    if (idx >= jm->count) return "";
    if (jm->count == (int64_t)jm->entry_len) {
        return jm->entries[idx].key ? jm->entries[idx].key : "";
    }
    int64_t seen = 0;
    for (int32_t i = 0; i < jm->entry_len; i++) {
        if (jm->entries[i].deleted) continue;
        if (seen == idx) return jm->entries[i].key ? jm->entries[i].key : "";
        seen++;
    }
    return "";
}

int64_t joss_map_val_at(void* m, int64_t idx) {
    m = unbox_ptr(m);
    if (!m || idx < 0) return 0;
    if (joss_is_arr_ptr((int64_t)(uintptr_t)m)) {
        return joss_arr_get_i64(m, idx);
    }
    if (joss_is_nominal_ptr((int64_t)(uintptr_t)m)) {
        const JossNominalObj* o = (const JossNominalObj*)m;
        if (idx < o->num_fields) {
            return o->fields[idx];
        }
        if (o->overflow_map) {
            return joss_map_val_at(o->overflow_map, idx - o->num_fields);
        }
        return 0;
    }
    if (!joss_is_map_ptr((int64_t)(uintptr_t)m)) return 0;
    JossMap* jm = (JossMap*)m;
    if (idx >= jm->count) return 0;
    if (jm->count == (int64_t)jm->entry_len) {
        return jm->entries[idx].val;
    }
    int64_t seen = 0;
    for (int32_t i = 0; i < jm->entry_len; i++) {
        if (jm->entries[i].deleted) continue;
        if (seen == idx) return jm->entries[i].val;
        seen++;
    }
    return 0;
}

int64_t joss_array_key_exists(const char* key, void* m) {
    m = unbox_ptr(m);
    key = unbox_cstr(key);
    if (!key || !m) return 0;
    if (joss_is_nominal_ptr((int64_t)(uintptr_t)m)) {
        const JossNominalObj* o = (const JossNominalObj*)m;
        if (joss_nominal_find_field(o, key) >= 0) return 1;
        return o->overflow_map ? joss_array_key_exists(key, o->overflow_map) : 0;
    }
    if (!joss_is_map_ptr((int64_t)(uintptr_t)m)) return 0;
    JossMap* jm = (JossMap*)m;
    if (jm->count == 0) return 0;
    if (jm->slot_cap == 0) {
        for (int32_t i = 0; i < jm->entry_len; i++) {
            JossMapEntry* e = &jm->entries[i];
            if (!e->deleted && e->key && (e->key == key || strcmp(e->key, key) == 0)) return 1;
        }
        return 0;
    }
    uint32_t h = fnv1a_hash(key);
    uint32_t mask = (uint32_t)(jm->slot_cap - 1);
    uint32_t pos = h & mask;
    for (;;) {
        int32_t idx = jm->slots[pos];
        if (idx == -1) return 0;
        if (idx >= 0) {
            JossMapEntry* e = &jm->entries[idx];
            if (e->key && (e->key == key || (e->hash == h && strcmp(e->key, key) == 0))) return 1;
        }
        pos = (pos + 1U) & mask;
    }
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
static _Thread_local int         g_joss_try_trace_depth[JOSS_MAX_TRY_DEPTH];
static _Thread_local int         g_joss_try_depth = 0;
static _Thread_local const char* g_joss_last_exc  = NULL;

void joss_try_reset(void) {
    g_joss_try_depth = 0;
    g_joss_last_exc = NULL;
    g_joss_trace_depth = 0;
}

void* joss_try_push(void) {
    if (g_joss_try_depth >= JOSS_MAX_TRY_DEPTH) {
        joss_panic("JOSS-EXC-001: try/catch nesting depth exceeded");
    }
    g_joss_try_trace_depth[g_joss_try_depth] = g_joss_trace_depth;
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
        g_joss_trace_depth = g_joss_try_trace_depth[g_joss_try_depth];
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

