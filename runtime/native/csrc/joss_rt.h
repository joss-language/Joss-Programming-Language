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

#ifdef __cplusplus
}
#endif

#endif // JOSS_RT_H
