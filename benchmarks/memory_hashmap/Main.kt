const val N = 50000

fun main() {
    val start = System.nanoTime()

    val map = HashMap<String, String>(N)

    // Insertion
    for (i in 0 until N) {
        val key = "user_key_$i"
        val valStr = "val_$i"
        map[key] = valStr
    }

    // Lookup
    var hits = 0
    for (i in 0 until N) {
        val key = "user_key_$i"
        val expectedVal = "val_$i"
        val v = map[key]
        if (v == expectedVal) {
            hits++
        }
    }

    val end = System.nanoTime()
    val elapsedMs = (end - start) / 1_000_000.0

    if (hits != N) {
        System.err.println("Verification failed: hits = $hits expected $N")
        System.exit(1)
    }

    System.out.printf("HashMap inserted and queried %d items (%d hits) in %.4f ms\n", N, hits, elapsedMs)
}
