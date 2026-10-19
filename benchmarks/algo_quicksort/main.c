#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <stdbool.h>
#include <windows.h>

#define N 50000

static inline void swap(int32_t *a, int32_t *b) {
    int32_t t = *a;
    *a = *b;
    *b = t;
}

static int partition(int32_t *arr, int low, int high) {
    int32_t pivot = arr[high];
    int i = low - 1;
    for (int j = low; j < high; j++) {
        if (arr[j] <= pivot) {
            i++;
            swap(&arr[i], &arr[j]);
        }
    }
    swap(&arr[i + 1], &arr[high]);
    return i + 1;
}

static void quicksort(int32_t *arr, int low, int high) {
    if (low < high) {
        int pi = partition(arr, low, high);
        quicksort(arr, low, pi - 1);
        quicksort(arr, pi + 1, high);
    }
}

int main() {
    int32_t *arr = (int32_t *)malloc(N * sizeof(int32_t));
    if (!arr) return 1;

    // LCG Generator seed = 123456789
    uint32_t state = 123456789;
    for (int i = 0; i < N; i++) {
        state = state * 1664525u + 1013904223u;
        arr[i] = (int32_t)(state & 0x7FFFFFFF);
    }

    LARGE_INTEGER freq, start, end;
    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&start);

    quicksort(arr, 0, N - 1);

    QueryPerformanceCounter(&end);
    double elapsed_ms = (double)(end.QuadPart - start.QuadPart) * 1000.0 / (double)freq.QuadPart;

    // Verification
    for (int i = 0; i < N - 1; i++) {
        if (arr[i] > arr[i + 1]) {
            printf("Error: Array is not sorted at index %d\n", i);
            free(arr);
            return 1;
        }
    }

    printf("Quicksort sorted %d elements in %.4f ms\n", N, elapsed_ms);
    free(arr);
    return 0;
}
