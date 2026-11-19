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
#define JOSS_MIN_PTR_ADDR 0x100000000ULL
#else
#define JOSS_MIN_PTR_ADDR 0x10000ULL
#endif

typedef enum {
    JOSS_TAG_NULL   = 0,
    JOSS_TAG_INT    = 1,
    JOSS_TAG_FLOAT  = 2,
    JOSS_TAG_BOOL   = 3,
    JOSS_TAG_STRING = 4,
    JOSS_TAG_OBJECT = 5,
    JOSS_TAG_ARRAY  = 6,
    JOSS_TAG_MAP    = 7,
    JOSS_TAG_BYTES  = 8
} JossTag;

typedef struct {
    uint64_t header;  /* (JOSS_BOX_MAGIC << 32) | (uint32_t)tag */
    uint64_t payload; /* i64, bitcast f64, bool (0/1), or pointer */
} JossBoxedValue;

JossBoxedValue* joss_box_value(uint64_t tag, uint64_t payload);
int64_t         joss_unbox_value(int64_t val);
JossBoxedValue* joss_box_i64(int64_t v);
JossBoxedValue* joss_box_f64(double v);
JossBoxedValue* joss_box_bool(bool v);
JossBoxedValue* joss_box_str(const char* s);

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
void*      joss_req_alloc(size_t size);
char*      joss_req_strdup(const char* s);

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
void               joss_sb_free(JossStringBuilder* sb);

/* =========================================================================
 * 4. Core Runtime I/O, Strings, Arrays & Maps (LLVM & Native Backend ABI)
 * ========================================================================= */
void        joss_panic(const char* msg);
void        joss_print_i64(int64_t v);
void        joss_print_f64(double v);
void        joss_print_bool(bool v);
void        joss_print_string(const char* s);
void        joss_print_mixed(int64_t val);
char*       joss_str_concat(const char* a, const char* b);

void*       joss_arr_new(int64_t initial_cap);
void        joss_arr_push_i64(void* arr, int64_t val);
void        joss_arr_push_str(void* arr, const char* val);
int64_t     joss_arr_get_i64(void* arr, int64_t idx);
const char* joss_arr_get_str(void* arr, int64_t idx);
void        joss_arr_set_i64(void* arr, int64_t idx, int64_t val);
void        joss_arr_set_str(void* arr, int64_t idx, const char* val);
int64_t     joss_arr_len(void* arr);
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

void*       joss_obj_new(const char* class_name);
void        joss_obj_set_field_i64(void* obj, const char* field, int64_t val);
int64_t     joss_obj_get_field_i64(void* obj, const char* field);
void        joss_obj_set_field_str(void* obj, const char* field, const char* val);
const char* joss_obj_get_field_str(void* obj, const char* field);

#ifdef __cplusplus
}
#endif

#endif /* JOSS_RT_H */
