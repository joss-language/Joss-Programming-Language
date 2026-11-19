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

    printf("joss_rt_test: ALL PASSED\n");
    return 0;
}
