#include <stdio.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

#ifdef _WIN32
#define JOSS_EXPORT __declspec(dllexport)
#else
#define JOSS_EXPORT
#endif

JOSS_EXPORT void joss_print_i64(int64_t val) {
    printf("%lld\n", (long long)val);
    fflush(stdout);
}

JOSS_EXPORT void joss_print_f64(double val) {
    printf("%g\n", val);
    fflush(stdout);
}

JOSS_EXPORT void joss_print_bool(bool val) {
    if (val) {
        printf("true\n");
    } else {
        printf("false\n");
    }
    fflush(stdout);
}

JOSS_EXPORT void joss_print_string(const char* str) {
    if (str != NULL) {
        printf("%s\n", str);
    } else {
        printf("null\n");
    }
    fflush(stdout);
}

JOSS_EXPORT void joss_panic(const char* msg) {
    fprintf(stderr, "Joss Runtime Panic: %s\n", msg != NULL ? msg : "unspecified error");
    exit(1);
}

// -------------------------------------------------------------
// Strings
// -------------------------------------------------------------
#include <string.h>

JOSS_EXPORT const char* joss_str_concat(const char* a, const char* b) {
    if (a == NULL) a = "";
    if (b == NULL) b = "";
    size_t la = strlen(a);
    size_t lb = strlen(b);
    char* res = (char*)malloc(la + lb + 1);
    if (!res) {
        joss_panic("Out of memory in string concatenation");
    }
    memcpy(res, a, la);
    memcpy(res + la, b, lb);
    res[la + lb] = '\0';
    return res;
}

// -------------------------------------------------------------
// Dynamic Object Runtime
// -------------------------------------------------------------
typedef struct JossFieldEntry {
    char* name;
    int is_str;
    int64_t val_i64;
    char* val_str;
    struct JossFieldEntry* next;
} JossFieldEntry;

typedef struct JossObject {
    char* class_name;
    JossFieldEntry* fields;
} JossObject;

JOSS_EXPORT void* joss_obj_new(const char* className) {
    JossObject* obj = (JossObject*)malloc(sizeof(JossObject));
    if (!obj) joss_panic("Out of memory allocating object");
    obj->class_name = className ? strdup(className) : NULL;
    obj->fields = NULL;
    return (void*)obj;
}

static JossFieldEntry* joss_obj_find_field(JossObject* obj, const char* field) {
    JossFieldEntry* cur = obj->fields;
    while (cur) {
        if (strcmp(cur->name, field) == 0) return cur;
        cur = cur->next;
    }
    return NULL;
}

JOSS_EXPORT void joss_obj_set_field_i64(void* objPtr, const char* field, int64_t val) {
    if (!objPtr || !field) return;
    JossObject* obj = (JossObject*)objPtr;
    JossFieldEntry* entry = joss_obj_find_field(obj, field);
    if (!entry) {
        entry = (JossFieldEntry*)malloc(sizeof(JossFieldEntry));
        entry->name = strdup(field);
        entry->next = obj->fields;
        obj->fields = entry;
    }
    entry->is_str = 0;
    entry->val_i64 = val;
    entry->val_str = NULL;
}

JOSS_EXPORT int64_t joss_obj_get_field_i64(void* objPtr, const char* field) {
    if (!objPtr || !field) return 0;
    JossObject* obj = (JossObject*)objPtr;
    JossFieldEntry* entry = joss_obj_find_field(obj, field);
    if (!entry) return 0;
    return entry->val_i64;
}

JOSS_EXPORT void joss_obj_set_field_str(void* objPtr, const char* field, const char* val) {
    if (!objPtr || !field) return;
    JossObject* obj = (JossObject*)objPtr;
    JossFieldEntry* entry = joss_obj_find_field(obj, field);
    if (!entry) {
        entry = (JossFieldEntry*)malloc(sizeof(JossFieldEntry));
        entry->name = strdup(field);
        entry->next = obj->fields;
        obj->fields = entry;
    }
    entry->is_str = 1;
    entry->val_str = val ? strdup(val) : NULL;
    entry->val_i64 = 0;
}

JOSS_EXPORT const char* joss_obj_get_field_str(void* objPtr, const char* field) {
    if (!objPtr || !field) return "";
    JossObject* obj = (JossObject*)objPtr;
    JossFieldEntry* entry = joss_obj_find_field(obj, field);
    if (!entry || !entry->val_str) return "";
    return entry->val_str;
}

// -------------------------------------------------------------
// Array Runtime
// -------------------------------------------------------------
typedef struct JossArray {
    int64_t len;
    int64_t cap;
    int is_str;
    void* data; // int64_t* or char**
} JossArray;

JOSS_EXPORT void* joss_arr_new(int64_t initialCap) {
    if (initialCap < 4) initialCap = 4;
    JossArray* arr = (JossArray*)malloc(sizeof(JossArray));
    arr->len = 0;
    arr->cap = initialCap;
    arr->is_str = 0;
    arr->data = malloc(initialCap * sizeof(int64_t));
    return (void*)arr;
}

JOSS_EXPORT void joss_arr_push_i64(void* arrPtr, int64_t val) {
    JossArray* arr = (JossArray*)arrPtr;
    if (!arr) return;
    if (arr->len >= arr->cap) {
        arr->cap *= 2;
        arr->data = realloc(arr->data, arr->cap * sizeof(int64_t));
    }
    ((int64_t*)arr->data)[arr->len++] = val;
}

JOSS_EXPORT void joss_arr_push_str(void* arrPtr, const char* val) {
    JossArray* arr = (JossArray*)arrPtr;
    if (!arr) return;
    arr->is_str = 1;
    if (arr->len >= arr->cap) {
        arr->cap *= 2;
        arr->data = realloc(arr->data, arr->cap * sizeof(char*));
    }
    ((char**)arr->data)[arr->len++] = val ? strdup(val) : NULL;
}

JOSS_EXPORT int64_t joss_arr_get_i64(void* arrPtr, int64_t idx) {
    JossArray* arr = (JossArray*)arrPtr;
    if (!arr || idx < 0 || idx >= arr->len) return 0;
    return ((int64_t*)arr->data)[idx];
}

JOSS_EXPORT const char* joss_arr_get_str(void* arrPtr, int64_t idx) {
    JossArray* arr = (JossArray*)arrPtr;
    if (!arr || idx < 0 || idx >= arr->len) return "";
    char* s = ((char**)arr->data)[idx];
    return s ? s : "";
}

JOSS_EXPORT void joss_arr_set_i64(void* arrPtr, int64_t idx, int64_t val) {
    JossArray* arr = (JossArray*)arrPtr;
    if (!arr || idx < 0) return;
    if (idx >= arr->cap) {
        int64_t newCap = (idx + 1) * 2;
        arr->data = realloc(arr->data, newCap * sizeof(int64_t));
        arr->cap = newCap;
    }
    if (idx >= arr->len) arr->len = idx + 1;
    ((int64_t*)arr->data)[idx] = val;
}

JOSS_EXPORT void joss_arr_set_str(void* arrPtr, int64_t idx, const char* val) {
    JossArray* arr = (JossArray*)arrPtr;
    if (!arr || idx < 0) return;
    arr->is_str = 1;
    if (idx >= arr->cap) {
        int64_t newCap = (idx + 1) * 2;
        arr->data = realloc(arr->data, newCap * sizeof(char*));
        arr->cap = newCap;
    }
    if (idx >= arr->len) arr->len = idx + 1;
    ((char**)arr->data)[idx] = val ? strdup(val) : NULL;
}

JOSS_EXPORT int64_t joss_arr_len(void* arrPtr) {
    JossArray* arr = (JossArray*)arrPtr;
    if (!arr) return 0;
    return arr->len;
}

// -------------------------------------------------------------
// Map Runtime
// -------------------------------------------------------------
typedef struct JossMapEntry {
    char* key;
    int is_str;
    int64_t val_i64;
    char* val_str;
    struct JossMapEntry* next;
} JossMapEntry;

typedef struct JossMap {
    JossMapEntry* entries;
} JossMap;

JOSS_EXPORT void* joss_map_new(void) {
    JossMap* m = (JossMap*)malloc(sizeof(JossMap));
    m->entries = NULL;
    return (void*)m;
}

static JossMapEntry* joss_map_find(JossMap* m, const char* key) {
    JossMapEntry* cur = m->entries;
    while (cur) {
        if (strcmp(cur->key, key) == 0) return cur;
        cur = cur->next;
    }
    return NULL;
}

JOSS_EXPORT void joss_map_set_i64(void* mPtr, const char* key, int64_t val) {
    if (!mPtr || !key) return;
    JossMap* m = (JossMap*)mPtr;
    JossMapEntry* entry = joss_map_find(m, key);
    if (!entry) {
        entry = (JossMapEntry*)malloc(sizeof(JossMapEntry));
        entry->key = strdup(key);
        entry->next = m->entries;
        m->entries = entry;
    }
    entry->is_str = 0;
    entry->val_i64 = val;
    entry->val_str = NULL;
}

JOSS_EXPORT int64_t joss_map_get_i64(void* mPtr, const char* key) {
    if (!mPtr || !key) return 0;
    JossMap* m = (JossMap*)mPtr;
    JossMapEntry* entry = joss_map_find(m, key);
    if (!entry) return 0;
    return entry->val_i64;
}

JOSS_EXPORT void joss_map_set_str(void* mPtr, const char* key, const char* val) {
    if (!mPtr || !key) return;
    JossMap* m = (JossMap*)mPtr;
    JossMapEntry* entry = joss_map_find(m, key);
    if (!entry) {
        entry = (JossMapEntry*)malloc(sizeof(JossMapEntry));
        entry->key = strdup(key);
        entry->next = m->entries;
        m->entries = entry;
    }
    entry->is_str = 1;
    entry->val_str = val ? strdup(val) : NULL;
    entry->val_i64 = 0;
}

JOSS_EXPORT const char* joss_map_get_str(void* mPtr, const char* key) {
    if (!mPtr || !key) return "";
    JossMap* m = (JossMap*)mPtr;
    JossMapEntry* entry = joss_map_find(m, key);
    if (!entry || !entry->val_str) return "";
    return entry->val_str;
}
