public class Main {
    private static final int N = 50000;

    private static int partition(int[] arr, int low, int high) {
        int pivot = arr[high];
        int i = low - 1;
        for (int j = low; j < high; j++) {
            if (arr[j] <= pivot) {
                i++;
                int t = arr[i];
                arr[i] = arr[j];
                arr[j] = t;
            }
        }
        int t = arr[i + 1];
        arr[i + 1] = arr[high];
        arr[high] = t;
        return i + 1;
    }

    private static void quicksort(int[] arr, int low, int high) {
        if (low < high) {
            int pi = partition(arr, low, high);
            quicksort(arr, low, pi - 1);
            quicksort(arr, pi + 1, high);
        }
    }

    public static void main(String[] args) {
        int[] arr = new int[N];
        long state = 123456789L;
        for (int i = 0; i < N; i++) {
            state = (state * 1664525L + 1013904223L) & 0xFFFFFFFFL;
            arr[i] = (int)(state & 0x7FFFFFFFL);
        }

        long start = System.nanoTime();
        quicksort(arr, 0, N - 1);
        long end = System.nanoTime();

        for (int i = 0; i < N - 1; i++) {
            if (arr[i] > arr[i + 1]) {
                System.err.println("Error: Array not sorted at " + i);
                System.exit(1);
            }
        }

        double elapsedMs = (end - start) / 1_000_000.0;
        System.out.printf("Quicksort sorted %d elements in %.4f ms\n", N, elapsedMs);
    }
}
