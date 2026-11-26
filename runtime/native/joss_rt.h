#ifndef JOSS_RT_H
#define JOSS_RT_H

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

/* =========================================================================
 * 1. Tagged Dynamic Value Representation (Phase 1)
 * ========================================================================= */
#define JOSS_BOX_MAGIC 0x8192A3B4ULL
#if UINTPTR_MAX > 0xFFFFFFFFULL
#define JOSS_MIN_PTR_ADDR 0x100000ULL
/* Linux AArch64 user-space virtual addresses span 48 bits (up to 0x0000FFFFFFFFFFFF). */
#define JOSS_PTR_VA_BITS  48
#else
#define JOSS_MIN_PTR_ADDR 0x10000ULL
#define JOSS_PTR_VA_BITS  32
#endif

typedef enum {
    JOSS_TAG_NULL    = 0,
    JOSS_TAG_INT     = 1,
    JOSS_TAG_FLOAT   = 2,
    JOSS_TAG_BOOL    = 3,
    JOSS_TAG_STRING  = 4,
    JOSS_TAG_OBJECT  = 5,
    JOSS_TAG_ARRAY   = 6,
    JOSS_TAG_MAP     = 7,
    JOSS_TAG_BYTES   = 8,
    JOSS_TAG_CLOSURE = 9
} JossTag;

typedef struct {
    uint64_t header;  /* (JOSS_BOX_MAGIC << 32) | (uint32_t)tag */
    uint64_t payload; /* i64, bitcast f64, bool (0/1), or pointer */
} JossBoxedValue;

typedef struct {
    uint64_t header;
    void*    fn_ptr;
    void*    env_map;
} JossClosureBox;

typedef struct {
    uint64_t len;
    uint8_t  data[];
} JossBytesBuf;

JossBoxedValue* joss_box_value(uint64_t tag, uint64_t payload);
int64_t         joss_unbox_value(int64_t val);
bool            joss_is_boxed_ptr(int64_t val);
JossBoxedValue* joss_box_i64(int64_t v);
JossBoxedValue* joss_box_f64(double v);
JossBoxedValue* joss_box_bool(bool v);
JossBoxedValue* joss_box_str(const char* s);
void*           joss_bytes_new(const void* data, size_t len);
const uint8_t*  joss_bytes_unwrap(const void* val, size_t* out_len);
void*           joss_closure_alloc(void* fn_ptr, void* env_map);
void*           joss_closure_prepare_call(void* callee);
bool            joss_is_not_null(int64_t v);
const char*     joss_i64_to_str(int64_t v);
const char*     joss_f64_to_string(double d);

/* =========================================================================
 * 2. Deterministic Arena Allocator
 * ========================================================================= */
typedef struct JossArenaChunk {
    struct JossArenaChunk* next;
    size_t used;
    size_t cap;
    uint8_t data[];
} JossArenaChunk;

typedef struct {
    JossArenaChunk* head;
    size_t default_chunk_cap;
} JossArena;

JossArena* joss_arena_create(size_t initial_cap);
void*      joss_arena_alloc(JossArena* arena, size_t size);
void       joss_arena_reset(JossArena* arena);
void       joss_arena_destroy(JossArena* arena);

void       joss_req_arena_begin(void);
void       joss_req_arena_reset(void);
bool       joss_req_arena_pause(void);
void       joss_req_arena_resume(bool prev_active);
size_t     joss_req_arena_bytes_used(void);
void*      joss_req_alloc(size_t size);
char*      joss_req_strdup(const char* s);

/* =========================================================================
 * 2b. Mini Garbage Collector (Conservative Mark-and-Sweep + Root Registry)
 * ========================================================================= */
typedef struct {
    size_t objects_live;
    size_t bytes_live;
    size_t total_allocs;
    size_t total_freed;
    size_t collections;
    size_t threshold_bytes;
} JossGCStats;

void   joss_gc_init(void* stack_bottom);
void   joss_gc_add_root(void** root_slot);
void   joss_gc_set_threshold(size_t bytes);
void   joss_gc_set_multithreaded(bool enabled);
size_t joss_gc_collect(void);
void   joss_gc_stats(JossGCStats* out);

/* =========================================================================
 * 3. Bounds-Checked ByteSlice / ByteBuffer / StringBuilder
 * ========================================================================= */
typedef struct {
    const uint8_t* data;
    int64_t        len;
} JossByteSlice;

typedef struct {
    uint8_t* data;
    int64_t  len;
    int64_t  cap;
} JossByteBuffer;

JossByteBuffer* joss_buf_new(int64_t initial_cap);
void            joss_buf_free(JossByteBuffer* buf);
int64_t         joss_buf_len(const JossByteBuffer* buf);
uint8_t         joss_buf_get(const JossByteBuffer* buf, int64_t idx);
void            joss_buf_set(JossByteBuffer* buf, int64_t idx, uint8_t val);
void            joss_buf_append_byte(JossByteBuffer* buf, uint8_t val);
void            joss_buf_append_bytes(JossByteBuffer* buf, const uint8_t* data, int64_t len);
void            joss_buf_append_cstr(JossByteBuffer* buf, const char* s);
char*           joss_buf_to_cstr(const JossByteBuffer* buf);
JossByteSlice   joss_buf_as_slice(const JossByteBuffer* buf);

typedef JossByteBuffer JossStringBuilder;
JossStringBuilder* joss_sb_new(void);
void               joss_sb_append(JossStringBuilder* sb, const char* s);
void               joss_sb_append_i64(JossStringBuilder* sb, int64_t v);
char*              joss_sb_to_string(const JossStringBuilder* sb);
char*              joss_sb_to_req_string(const JossStringBuilder* sb);
void               joss_sb_free(JossStringBuilder* sb);

/* =========================================================================
 * 4. Core Runtime I/O, Strings, Arrays & Maps (LLVM & Native Backend ABI)
 * ========================================================================= */
void        joss_panic(const char* msg);
void        joss_trace_push(const char* func, const char* file, int64_t line);
void        joss_trace_pop(void);
void        joss_trace_set_line(int64_t line);
int64_t     joss_trace_depth(void);
void        joss_trace_reset(void);
void        joss_print_stack_trace(void);

void        joss_print_i64(int64_t v);
void        joss_print_f64(double v);
void        joss_print_bool(bool v);
void        joss_print_string(const char* s);
void        joss_print_mixed(int64_t val);
char*       joss_str_concat(const char* a, const char* b);
const char* joss_strval(int64_t v);
int64_t     joss_strlen(const char* s);

void*       joss_arr_new(int64_t initial_cap);
void        joss_arr_push_i64(void* arr, int64_t val);
void        joss_arr_push_str(void* arr, const char* val);
int64_t     joss_arr_get_i64(void* arr, int64_t idx);
const char* joss_arr_get_str(void* arr, int64_t idx);
void        joss_arr_set_i64(void* arr, int64_t idx, int64_t val);
void        joss_arr_set_str(void* arr, int64_t idx, const char* val);
int64_t     joss_arr_len(void* arr);
void*       joss_arr_push_ret(void* arr, int64_t val);
int64_t     joss_array_pop(void* arr);

void*       joss_map_new(void);
void        joss_map_set_i64(void* m, const char* key, int64_t val);
int64_t     joss_map_get_i64(void* m, const char* key);
void        joss_map_set_str(void* m, const char* key, const char* val);
const char* joss_map_get_str(void* m, const char* key);
void        joss_map_delete_i64(void* m, const char* key);
int64_t     joss_map_len(void* m);
const char* joss_map_key_at(void* m, int64_t idx);
int64_t     joss_map_val_at(void* m, int64_t idx);
int64_t     joss_array_key_exists(const char* key, void* m);
bool        joss_is_arr_ptr(int64_t v);
bool        joss_is_map_ptr(int64_t v);
bool        joss_is_nominal_ptr(int64_t v);

void*       joss_obj_new(const char* class_name);
void*       joss_nominal_obj_new(const char* class_name, int64_t num_fields, const char* const* field_names);
int64_t     joss_nominal_get_slot(void* obj, int64_t slot_idx);
void        joss_nominal_set_slot(void* obj, int64_t slot_idx, int64_t val);
void        joss_obj_set_field_i64(void* obj, const char* field, int64_t val);
int64_t     joss_obj_get_field_i64(void* obj, const char* field);
void        joss_obj_set_field_str(void* obj, const char* field, const char* val);
const char* joss_obj_get_field_str(void* obj, const char* field);
int64_t     joss_intval(int64_t v);
const char* joss_strval(int64_t v);

void        joss_try_reset(void);

void        joss_closure_env_set(void* fn_ptr, void* env_map);
void*       joss_closure_env_get(void* fn_ptr);
void        joss_closure_env_tls_reset(void);

typedef enum {
    JOSS_LOCK_ENV = 0,
    JOSS_LOCK_CACHE,
    JOSS_LOCK_RATELIMIT,
    JOSS_LOCK_SESSION,
    JOSS_LOCK_WS,
    JOSS_LOCK_DB,
    JOSS_LOCK_SITEMAP,
    JOSS_LOCK_CRON,
    JOSS_LOCK_RAND,
    JOSS_LOCK_NOTIFY,
    JOSS_LOCK_GENERAL,
    JOSS_LOCK_COUNT
} JossLockSlot;

void        joss_global_lock(uint32_t slot);
void        joss_global_unlock(uint32_t slot);

/* =========================================================================
 * 5. Schema DB Bridge & WebSocket / Cron Native Contracts
 * ========================================================================= */
typedef void*   (*JossSchemaDbQueryFn)(const char*, void*);
typedef int64_t (*JossSchemaDbExecFn)(const char*, void*);
extern JossSchemaDbQueryFn g_joss_schema_db_query;
extern JossSchemaDbExecFn  g_joss_schema_db_exec;
void        joss_register_schema_db(JossSchemaDbQueryFn q, JossSchemaDbExecFn e);
int64_t     joss_db_ensure_connected(void);

#if defined(_WIN32)
typedef uintptr_t joss_socket_t;
#else
typedef int joss_socket_t;
#endif

const char* joss_ws_accept_key(const char* client_key);
size_t      joss_ws_encode_text_frame(const char* payload, uint8_t* out_buf, size_t out_cap);
int         joss_ws_read_frame(joss_socket_t sock, int* out_opcode, char** out_payload, size_t* out_len);
const char* joss___joss_ws_read_text(int64_t sock_fd);
int64_t     joss___joss_ws_send_text(int64_t sock_fd, const char* text);
int64_t     fn_WebSocket_subscribe(const char* channel, int64_t sock_fd);
int64_t     fn_WebSocket_unsubscribe(const char* channel, int64_t sock_fd);
int64_t     fn_WebSocket_publish(const char* channel, const char* message);
int64_t     fn_WebSocket_subscriberCount(const char* channel);

void*       fn_Cron_schedule(const char* expr, void* callback);
int64_t     fn_Cron_runDue(void);

#ifdef __cplusplus
}
#endif

#endif /* JOSS_RT_H */
