const val N = 50000

fun partition(arr: IntArray, low: Int, high: Int): Int {
    val pivot = arr[high]
    var i = low - 1
    for (j in low until high) {
        if (arr[j] <= pivot) {
            i++
            val t = arr[i]
            arr[i] = arr[j]
            arr[j] = t
        }
    }
    val t = arr[i + 1]
    arr[i + 1] = arr[high]
    arr[high] = t
    return i + 1
}

fun quicksort(arr: IntArray, low: Int, high: Int) {
    if (low < high) {
        val pi = partition(arr, low, high)
        quicksort(arr, low, pi - 1)
        quicksort(arr, pi + 1, high)
    }
}

fun main() {
    val arr = IntArray(N)
    var state = 123456789L
    for (i in 0 until N) {
        state = (state * 1664525L + 1013904223L) and 0xFFFFFFFFL
        arr[i] = (state and 0x7FFFFFFFL).toInt()
    }

    val start = System.nanoTime()
    quicksort(arr, 0, N - 1)
    val end = System.nanoTime()

    for (i in 0 until N - 1) {
        if (arr[i] > arr[i + 1]) {
            System.err.println("Error: Array not sorted at $i")
            System.exit(1)
        }
    }

    val elapsedMs = (end - start) / 1_000_000.0
    System.out.printf("Quicksort sorted %d elements in %.4f ms\n", N, elapsedMs)
}
