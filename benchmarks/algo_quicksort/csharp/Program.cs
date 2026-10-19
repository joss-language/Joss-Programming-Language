using System;
using System.Diagnostics;

class Program {
    const int N = 50000;

    static int Partition(int[] arr, int low, int high) {
        int pivot = arr[high];
        int i = low - 1;
        for (int j = low; j < high; j++) {
            if (arr[j] <= pivot) {
                i++;
                int tmp = arr[i];
                arr[i] = arr[j];
                arr[j] = tmp;
            }
        }
        int tmp2 = arr[i + 1];
        arr[i + 1] = arr[high];
        arr[high] = tmp2;
        return i + 1;
    }

    static void QuickSort(int[] arr, int low, int high) {
        if (low < high) {
            int pi = Partition(arr, low, high);
            QuickSort(arr, low, pi - 1);
            QuickSort(arr, pi + 1, high);
        }
    }

    static void Main(string[] args) {
        long state = 123456789;
        int[] arr = new int[N];
        for (int i = 0; i < N; i++) {
            state = (state * 1664525 + 1013904223) & 0xFFFFFFFF;
            arr[i] = (int)(state & 0x7FFFFFFF);
        }

        var sw = Stopwatch.StartNew();
        QuickSort(arr, 0, N - 1);
        sw.Stop();

        for (int i = 0; i < N - 1; i++) {
            if (arr[i] > arr[i + 1]) {
                Console.Error.WriteLine($"Error: Array not sorted at {i}");
                Environment.Exit(1);
            }
        }

        double elapsedMs = sw.Elapsed.TotalMilliseconds;
        Console.WriteLine($"Quicksort sorted {N} elements in {elapsedMs:F4} ms");
    }
}
