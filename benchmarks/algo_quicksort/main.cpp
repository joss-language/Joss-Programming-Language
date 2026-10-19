#include <iostream>
#include <vector>
#include <chrono>
#include <cstdint>

constexpr int N = 50000;

static inline void swap_val(int32_t &a, int32_t &b) {
    int32_t t = a;
    a = b;
    b = t;
}

static int partition(std::vector<int32_t> &arr, int low, int high) {
    int32_t pivot = arr[high];
    int i = low - 1;
    for (int j = low; j < high; j++) {
        if (arr[j] <= pivot) {
            i++;
            swap_val(arr[i], arr[j]);
        }
    }
    swap_val(arr[i + 1], arr[high]);
    return i + 1;
}

static void quicksort(std::vector<int32_t> &arr, int low, int high) {
    if (low < high) {
        int pi = partition(arr, low, high);
        quicksort(arr, low, pi - 1);
        quicksort(arr, pi + 1, high);
    }
}

int main() {
    std::vector<int32_t> arr(N);
    uint32_t state = 123456789;
    for (int i = 0; i < N; i++) {
        state = state * 1664525u + 1013904223u;
        arr[i] = static_cast<int32_t>(state & 0x7FFFFFFF);
    }

    auto start = std::chrono::high_resolution_clock::now();

    quicksort(arr, 0, N - 1);

    auto end = std::chrono::high_resolution_clock::now();
    double elapsed_ms = std::chrono::duration<double, std::milli>(end - start).count();

    for (int i = 0; i < N - 1; i++) {
        if (arr[i] > arr[i + 1]) {
            std::cerr << "Error: Array not sorted at " << i << std::endl;
            return 1;
        }
    }

    std::cout << "Quicksort sorted " << N << " elements in " << elapsed_ms << " ms" << std::endl;
    return 0;
}
