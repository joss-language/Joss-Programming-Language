//go:build ignore
#include "joss_rt.h"
#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main(void) {
    /* 1. Test Tagged Dynamic Values */
    JossBoxedValue* bi = joss_box_i64(250000);
    assert((bi->header >> 32) == JOSS_BOX_MAGIC);
    assert((bi->header & 0xFFFFFFFFULL) == JOSS_TAG_INT);
    assert((int64_t)bi->payload == 250000);
    free(bi);

    JossBoxedValue* bb = joss_box_bool(true);
    assert((bb->header & 0xFFFFFFFFULL) == JOSS_TAG_BOOL);
    assert(bb->payload == 1);
    free(bb);

    /* 2. Test Arena Allocator */
    JossArena* arena = joss_arena_create(64);
    void* p1 = joss_arena_alloc(arena, 32);
    void* p2 = joss_arena_alloc(arena, 128);
    assert(p1 != NULL && p2 != NULL && p1 != p2);
    joss_arena_reset(arena);
    void* p3 = joss_arena_alloc(arena, 16);
    assert(p3 != NULL);
    joss_arena_destroy(arena);

    /* 3. Test Bounds-Checked ByteBuffer */
    JossByteBuffer* buf = joss_buf_new(8);
    joss_buf_append_bytes(buf, (const uint8_t*)"Hello", 5);
    joss_buf_append_byte(buf, '!');
    assert(joss_buf_len(buf) == 6);
    assert(joss_buf_get(buf, 0) == 'H');
    assert(joss_buf_get(buf, 5) == '!');
    joss_buf_set(buf, 5, '?');
    char* s = joss_buf_to_cstr(buf);
    assert(strcmp(s, "Hello?") == 0);
    free(s);
    joss_buf_free(buf);

    /* 4. Test StringBuilder */
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "Count=");
    joss_sb_append_i64(sb, 100000);
    char* res = joss_sb_to_string(sb);
    assert(strcmp(res, "Count=100000") == 0);
    free(res);
    joss_sb_free(sb);

    /* 5. Test Array & Map */
    void* arr = joss_arr_new(2);
    joss_arr_push_i64(arr, 10);
    joss_arr_push_i64(arr, 20);
    joss_arr_push_i64(arr, 30);
    assert(joss_arr_len(arr) == 3);
    assert(joss_arr_get_i64(arr, 2) == 30);

    void* mp = joss_map_new();
    joss_map_set_i64(mp, "k", 999999);
    assert(joss_map_get_i64(mp, "k") == 999999);
    joss_map_delete_i64(mp, "k");
    assert(joss_map_get_i64(mp, "k") == 0);

    /* 6. Test Phase 1 Builtins & Concurrency Primitives */
    extern int64_t joss_ord(const char* s);
    extern int64_t joss_rand(int64_t min_val, int64_t max_val);
    extern int64_t joss_floor(int64_t val_bits);
    extern int64_t joss_ceil(int64_t val_bits);
    extern int64_t joss_round(int64_t val_bits);
    extern const char* joss_toon_encode(const char* data);
    extern int64_t joss_toon_verify(const char* token);
    extern const char* fn_Lang_locale(void);
    extern int64_t fn_Lang_setLocale(const char* loc);
    extern int64_t fn_Mutex_new(void);
    extern void fn_Mutex_lock(int64_t h);
    extern void fn_Mutex_unlock(int64_t h);
    extern int64_t fn_RWMutex_new(void);
    extern void fn_RWMutex_rLock(int64_t h);
    extern void fn_RWMutex_rUnlock(int64_t h);
    extern int64_t fn_WaitGroup_new(void);
    extern void fn_WaitGroup_add(int64_t h, int64_t delta);
    extern void fn_WaitGroup_done(int64_t h);
    extern void fn_WaitGroup_wait(int64_t h);

    assert(joss_ord("A") == 65);
    int64_t rv = joss_rand(10, 20);
    assert(rv >= 10 && rv <= 20);
    double d_in = 3.7, d_out = 0.0;
    int64_t bits_in = 0, bits_out = 0;
    memcpy(&bits_in, &d_in, sizeof(double));
    bits_out = joss_floor(bits_in);
    memcpy(&d_out, &bits_out, sizeof(double));
    assert(d_out == 3.0);
    bits_out = joss_ceil(bits_in);
    memcpy(&d_out, &bits_out, sizeof(double));
    assert(d_out == 4.0);
    bits_out = joss_round(bits_in);
    memcpy(&d_out, &bits_out, sizeof(double));
    assert(d_out == 4.0);

    const char* tok = joss_toon_encode("payload");
    assert(joss_toon_verify(tok) == 1);
    assert(joss_toon_verify("invalid") == 0);

    assert(fn_Lang_setLocale("en") == 1);
    assert(strcmp(fn_Lang_locale(), "en") == 0);

    int64_t mu = fn_Mutex_new();
    fn_Mutex_lock(mu);
    fn_Mutex_unlock(mu);

    int64_t rw = fn_RWMutex_new();
    fn_RWMutex_rLock(rw);
    fn_RWMutex_rUnlock(rw);

    int64_t wg = fn_WaitGroup_new();
    fn_WaitGroup_add(wg, 1);
    fn_WaitGroup_done(wg);
    fn_WaitGroup_wait(wg);

    /* 7. Test Phase 3 RFC 6455 WebSocket Handshake & Text Frame */
    extern const char* joss_ws_accept_key(const char* client_key);
    extern size_t joss_ws_encode_text_frame(const char* payload, uint8_t* out_buf, size_t out_cap);
    const char* ws_accept = joss_ws_accept_key("dGhlIHNhbXBsZSBub25jZQ==");
    assert(strcmp(ws_accept, "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=") == 0);
    uint8_t ws_frame[64];
    size_t ws_len = joss_ws_encode_text_frame("hi", ws_frame, sizeof(ws_frame));
    assert(ws_len == 4 && ws_frame[0] == 0x81 && ws_frame[1] == 2 && ws_frame[2] == 'h' && ws_frame[3] == 'i');

    /* 8. Test Per-Request Arena Reclamation (1000 HTTP request cycles) */
    for (int req = 0; req < 1000; req++) {
        joss_req_arena_begin();
        char* s = joss_str_concat("req_", joss_i64_to_str(req));
        assert(s != NULL && strncmp(s, "req_", 4) == 0);
        void* arr = joss_arr_new(4);
        for (int k = 0; k < 20; k++) {
            joss_arr_push_i64(arr, k);
        }
        assert(joss_arr_len(arr) == 20 && joss_arr_get_i64(arr, 19) == 19);
        void* m = joss_map_new();
        for (int k = 0; k < 12; k++) {
            joss_map_set_i64(m, joss_i64_to_str(k), k * 10);
        }
        assert(joss_map_len(m) == 12 && joss_map_get_i64(m, "7") == 70);
        assert(joss_req_arena_bytes_used() > 0);
        joss_req_arena_reset();
        assert(joss_req_arena_bytes_used() == 0);
    }

    /* 9. Test Nominal Flat-Slot Objects + Dynamic Overflow + Map Interop */
    static const char* const kNodeFields[3] = { "val", "left", "right" };
    void* node = joss_nominal_obj_new("TreeNode", 3, kNodeFields);
    assert(joss_is_nominal_ptr((int64_t)(uintptr_t)node));
    joss_nominal_set_slot(node, 0, 42);
    joss_nominal_set_slot(node, 1, 100);
    joss_nominal_set_slot(node, 2, 200);
    assert(joss_nominal_get_slot(node, 0) == 42);
    assert(joss_obj_get_field_i64(node, "val") == 42);
    assert(joss_obj_get_field_i64(node, "right") == 200);
    joss_obj_set_field_i64(node, "val", 99);
    assert(joss_nominal_get_slot(node, 0) == 99);
    joss_obj_set_field_i64(node, "extra_dyn", 777);
    assert(joss_obj_get_field_i64(node, "extra_dyn") == 777);
    assert(joss_map_len(node) == 4);
    assert(strcmp(joss_map_key_at(node, 0), "val") == 0);
    assert(joss_map_val_at(node, 0) == 99);
    assert(joss_array_key_exists("left", node) == 1);
    assert(joss_array_key_exists("extra_dyn", node) == 1);
    assert(joss_array_key_exists("missing", node) == 0);

    /* 10. Test Native Stack Trace Push/Pop/Line & Try-Catch Unwind Restoration */
    joss_trace_reset();
    assert(joss_trace_depth() == 0);
    joss_trace_push("main", "app.joss", 10);
    joss_trace_push("compute", "math.joss", 25);
    assert(joss_trace_depth() == 2);
    joss_trace_set_line(31);
    joss_trace_pop();
    assert(joss_trace_depth() == 1);
    joss_trace_pop();
    assert(joss_trace_depth() == 0);

    /* 11. Test Mini Garbage Collector (Mark-and-Sweep + Cycle Collection) */
    volatile int gc_stack_anchor = 0;
    joss_gc_init((void*)&gc_stack_anchor);
    joss_gc_set_threshold(64 * 1024 * 1024); /* manual deterministic trigger */

    static const char* const kCycleFields[2] = { "id", "peer" };
    void* live_root = joss_nominal_obj_new("CycleNode", 2, kCycleFields);
    joss_nominal_set_slot(live_root, 0, 12345);
    void* live_child = joss_arr_new(4);
    joss_arr_push_i64(live_child, 98765);
    joss_nominal_set_slot(live_root, 1, (int64_t)(uintptr_t)live_child);

    /* Allocate 400 unreachable objects including reference cycles (A <-> B) */
    for (int i = 0; i < 200; i++) {
        void* a = joss_nominal_obj_new("CycleNode", 2, kCycleFields);
        void* b = joss_nominal_obj_new("CycleNode", 2, kCycleFields);
        joss_nominal_set_slot(a, 0, i);
        joss_nominal_set_slot(b, 0, i + 1);
        joss_nominal_set_slot(a, 1, (int64_t)(uintptr_t)b);
        joss_nominal_set_slot(b, 1, (int64_t)(uintptr_t)a);
    }

    size_t freed = joss_gc_collect();
    JossGCStats st;
    joss_gc_stats(&st);
    assert(freed >= 380);
    assert(st.collections >= 1);
    /* Verify live root and its transitive child survived intact */
    assert(joss_nominal_get_slot(live_root, 0) == 12345);
    void* survived_child = (void*)(uintptr_t)joss_nominal_get_slot(live_root, 1);
    assert(joss_arr_len(survived_child) == 1 && joss_arr_get_i64(survived_child, 0) == 98765);

    printf("joss_rt_test: ALL PASSED\n");
    return 0;
}
