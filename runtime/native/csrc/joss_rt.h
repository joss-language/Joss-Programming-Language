#ifndef JOSS_RT_H
#define JOSS_RT_H

#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

void joss_print_i64(int64_t val);
void joss_print_f64(double val);
void joss_print_bool(bool val);
void joss_print_string(const char* str);
void joss_panic(const char* msg);

// String helpers
const char* joss_str_concat(const char* a, const char* b);

// Object runtime ABI
void* joss_obj_new(const char* className);
void joss_obj_set_field_i64(void* obj, const char* field, int64_t val);
int64_t joss_obj_get_field_i64(void* obj, const char* field);
void joss_obj_set_field_str(void* obj, const char* field, const char* val);
const char* joss_obj_get_field_str(void* obj, const char* field);

// Array runtime ABI
void* joss_arr_new(int64_t initialCap);
void joss_arr_push_i64(void* arr, int64_t val);
void joss_arr_push_str(void* arr, const char* val);
int64_t joss_arr_get_i64(void* arr, int64_t idx);
const char* joss_arr_get_str(void* arr, int64_t idx);
void joss_arr_set_i64(void* arr, int64_t idx, int64_t val);
void joss_arr_set_str(void* arr, int64_t idx, const char* val);
int64_t joss_arr_len(void* arr);

// Map runtime ABI
void* joss_map_new(void);
void joss_map_set_i64(void* m, const char* key, int64_t val);
int64_t joss_map_get_i64(void* m, const char* key);
void joss_map_set_str(void* m, const char* key, const char* val);
const char* joss_map_get_str(void* m, const char* key);

#ifdef __cplusplus
}
#endif

#endif // JOSS_RT_H
