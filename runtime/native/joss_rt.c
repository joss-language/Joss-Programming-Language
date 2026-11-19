//go:build ignore
#include "joss_rt.h"

#include <inttypes.h>
#include <setjmp.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* =========================================================================
 * Panic & Error Handling
 * ========================================================================= */
void joss_panic(const char* msg) {
    fprintf(stderr, "Panic: %s\n", msg ? msg : "(null)");
    exit(1);
}

/* =========================================================================
 * Tagged Dynamic Values (JossBoxedValue)
 * ========================================================================= */
static uintptr_t g_joss_box_min = UINTPTR_MAX;
static uintptr_t g_joss_box_max = 0;

bool joss_is_boxed_ptr(int64_t val) {
    uintptr_t u = (uintptr_t)(uint64_t)val;
    if (u == 0 || (u & 7U) != 0 || u < g_joss_box_min || u > g_joss_box_max) {
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
    uintptr_t addr = (uintptr_t)box;
    if (addr < g_joss_box_min) g_joss_box_min = addr;
    if (addr > g_joss_box_max) g_joss_box_max = addr;
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

char* joss_req_strdup(const char* s) {
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

void joss_buf_append_cstr(JossByteBuffer* buf, const char* s) {
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
        if (tag == JOSS_TAG_STRING) return (const char*)(uintptr_t)box->payload;
        if (tag == JOSS_TAG_BOOL) return box->payload ? "true" : "false";
        if (tag == JOSS_TAG_NULL) return "";
        if (tag == JOSS_TAG_INT) {
            char* out = (char*)malloc(32);
            snprintf(out, 32, "%lld", (long long)(int64_t)box->payload);
            return out;
        }
    }
    uint64_t u = (uint64_t)v;
    if (u >= JOSS_MIN_PTR_ADDR && (u >> 47) == 0) {
        return (const char*)(uintptr_t)u;
    }
    char* out = (char*)malloc(32);
    snprintf(out, 32, "%lld", (long long)v);
    return out;
}

const char* joss_to_string(int64_t v) {
    return joss_strval(v);
}

static const char* unbox_cstr_rt(const char* s) {
    if (!s) return NULL;
    int64_t v = (int64_t)(uintptr_t)s;
    if (joss_is_boxed_ptr(v)) {
        return joss_strval(v);
    }
    return s;
}

void joss_print_string(const char* s) {
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
            default:
                joss_print_i64((int64_t)box->payload);
                return;
        }
    }
    uint64_t u = (uint64_t)val;
    if (u == 0 || u < JOSS_MIN_PTR_ADDR || (u >> 47) != 0) {
        joss_print_i64(val);
        return;
    }
    joss_print_string((const char*)(uintptr_t)u);
}

char* joss_str_concat(const char* a, const char* b) {
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

static uintptr_t g_joss_arr_min = UINTPTR_MAX;
static uintptr_t g_joss_arr_max = 0;
static uintptr_t g_joss_map_min = UINTPTR_MAX;
static uintptr_t g_joss_map_max = 0;

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
    uintptr_t addr = (uintptr_t)a;
    if (addr < g_joss_arr_min) g_joss_arr_min = addr;
    if (addr > g_joss_arr_max) g_joss_arr_max = addr;
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
    if (!s) return NULL;
    int64_t v = (int64_t)(uintptr_t)s;
    if (joss_is_boxed_ptr(v)) {
        return joss_strval(v);
    }
    return s;
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
    idx = joss_unbox_value(idx);
    JossArray* a = (JossArray*)arr;
    if (idx < 0 || idx >= a->len) return 0;
    return a->items[idx];
}

const char* joss_arr_get_str(void* arr, int64_t idx) {
    return (const char*)(uintptr_t)joss_arr_get_i64(arr, idx);
}

void joss_arr_set_i64(void* arr, int64_t idx, int64_t val) {
    arr = unbox_ptr(arr);
    if (!arr) return;
    if (joss_is_map_ptr((int64_t)(uintptr_t)arr)) {
        const char* k = unbox_cstr((const char*)(uintptr_t)idx);
        joss_map_set_i64(arr, k, val);
        return;
    }
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
    uintptr_t addr = (uintptr_t)m;
    if (addr < g_joss_map_min) g_joss_map_min = addr;
    if (addr > g_joss_map_max) g_joss_map_max = addr;
    return m;
}

void joss_map_set_i64(void* mp, const char* key, int64_t val) {
    mp = unbox_ptr(mp);
    key = unbox_cstr(key);
    JossMap* m = (JossMap*)mp;
    if (!m || !key) return;
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
    key = unbox_cstr(key);
    JossMap* m = (JossMap*)mp;
    if (!m || !key) return 0;
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
    return (const char*)(uintptr_t)joss_map_get_i64(mp, key);
}

void joss_map_delete_i64(void* mp, const char* key) {
    mp = unbox_ptr(mp);
    key = unbox_cstr(key);
    JossMap* m = (JossMap*)mp;
    if (!m || !key) return;
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
    if (u >= JOSS_MIN_PTR_ADDR && (u >> 47) == 0) {
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
    JossMap* jm = (JossMap*)m;
    return jm->count;
}

const char* joss_map_key_at(void* m, int64_t idx) {
    m = unbox_ptr(m);
    if (!m || idx < 0) return "";
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
    if (!key || !m) return 0;
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
    a = unbox_cstr(a);
    b = unbox_cstr(b);
    if (a == b) return true;
    if (!a || !b) return false;
    return strcmp(a, b) == 0;
}

bool joss_is_arr_ptr(int64_t v) {
    uintptr_t u = (uintptr_t)(uint64_t)v;
    if (u == 0 || (u & 7U) != 0 || u < g_joss_arr_min || u > g_joss_arr_max) return false;
    return ((const JossArray*)u)->magic == JOSS_ARR_MAGIC;
}

bool joss_is_map_ptr(int64_t v) {
    uintptr_t u = (uintptr_t)(uint64_t)v;
    if (u == 0 || (u & 7U) != 0 || u < g_joss_map_min || u > g_joss_map_max) return false;
    return ((const JossMap*)u)->magic == JOSS_MAP_MAGIC;
}

#define JOSS_MAX_TRY_DEPTH 64
static _Thread_local jmp_buf     g_joss_try_stack[JOSS_MAX_TRY_DEPTH];
static _Thread_local int         g_joss_try_depth = 0;
static _Thread_local const char* g_joss_last_exc  = NULL;

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

void joss_throw(const char* msg) {
    if (g_joss_try_depth > 0) {
        g_joss_last_exc = msg ? msg : "Joss Exception";
        g_joss_try_depth--;
        longjmp(g_joss_try_stack[g_joss_try_depth], 1);
    }
    joss_panic(msg ? msg : "Joss Exception");
}

