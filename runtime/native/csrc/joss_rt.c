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
