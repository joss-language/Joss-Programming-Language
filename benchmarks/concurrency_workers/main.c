#include <stdio.h>
#include <stdlib.h>
#include <windows.h>

#define NUM_WORKERS 8
#define TASKS_PER_WORKER 100
#define INNER_ITERS 500

typedef struct {
    int start_k;
    int end_k;
    long long result;
} WorkerData;

DWORD WINAPI worker_thread(LPVOID param) {
    WorkerData *data = (WorkerData *)param;
    long long total = 0;
    for (int k = data->start_k; k <= data->end_k; k++) {
        long long task_sum = 0;
        for (int j = 1; j <= INNER_ITERS; j++) {
            task_sum += (long long)((j * 17 + k * 31) % 1000);
        }
        total += task_sum;
    }
    data->result = total;
    return 0;
}

int main() {
    LARGE_INTEGER freq, start, end;
    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&start);

    HANDLE threads[NUM_WORKERS];
    WorkerData data[NUM_WORKERS];

    for (int w = 0; w < NUM_WORKERS; w++) {
        data[w].start_k = w * TASKS_PER_WORKER + 1;
        data[w].end_k = (w + 1) * TASKS_PER_WORKER;
        data[w].result = 0;
        threads[w] = CreateThread(NULL, 0, worker_thread, &data[w], 0, NULL);
    }

    WaitForMultipleObjects(NUM_WORKERS, threads, TRUE, INFINITE);

    long long grand_total = 0;
    for (int w = 0; w < NUM_WORKERS; w++) {
        grand_total += data[w].result;
        CloseHandle(threads[w]);
    }

    QueryPerformanceCounter(&end);
    double elapsed_ms = (double)(end.QuadPart - start.QuadPart) * 1000.0 / (double)freq.QuadPart;

    printf("Concurrency 8 workers processed 800 tasks: checksum=%lld in %.4f ms\n", grand_total, elapsed_ms);
    return 0;
}
