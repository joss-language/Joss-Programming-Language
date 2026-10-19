#include <iostream>
#include <vector>
#include <thread>
#include <cstdint>
#include <chrono>

constexpr int NUM_WORKERS = 8;
constexpr int TASKS_PER_WORKER = 100;
constexpr int INNER_ITERS = 500;

void worker(int start_k, int end_k, int64_t &out_result) {
    int64_t total = 0;
    for (int k = start_k; k <= end_k; k++) {
        int64_t task_sum = 0;
        for (int j = 1; j <= INNER_ITERS; j++) {
            task_sum += static_cast<int64_t>((j * 17 + k * 31) % 1000);
        }
        total += task_sum;
    }
    out_result = total;
}

int main() {
    auto start = std::chrono::high_resolution_clock::now();

    std::vector<std::thread> threads;
    std::vector<int64_t> results(NUM_WORKERS, 0);

    for (int w = 0; w < NUM_WORKERS; w++) {
        int start_k = w * TASKS_PER_WORKER + 1;
        int end_k = (w + 1) * TASKS_PER_WORKER;
        threads.emplace_back(worker, start_k, end_k, std::ref(results[w]));
    }

    for (auto &t : threads) {
        t.join();
    }

    int64_t grand_total = 0;
    for (int64_t r : results) {
        grand_total += r;
    }

    auto end = std::chrono::high_resolution_clock::now();
    double elapsed_ms = std::chrono::duration<double, std::milli>(end - start).count();

    std::cout << "Concurrency 8 workers processed 800 tasks: checksum="
              << grand_total << " in " << elapsed_ms << " ms" << std::endl;
    return 0;
}
