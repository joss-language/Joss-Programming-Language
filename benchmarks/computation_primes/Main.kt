// Benchmark A: Prime Sieve (Kotlin implementation)
fun countPrimes(limit: Int): Long {
    if (limit < 2) return 0L
    val isPrime = BooleanArray(limit + 1) { true }
    isPrime[0] = false
    isPrime[1] = false

    var p = 2L
    while (p * p <= limit) {
        val pi = p.toInt()
        if (isPrime[pi]) {
            var i = p * p
            while (i <= limit) {
                isPrime[i.toInt()] = false
                i += p
            }
        }
        p++
    }

    var count = 0L
    for (i in 2..limit) {
        if (isPrime[i]) count++
    }
    return count
}

fun main(args: Array<String>) {
    var limit = 1000000
    if (args.isNotEmpty()) {
        args[0].toIntOrNull()?.let { limit = it }
    }

    val start = System.nanoTime()
    val count = countPrimes(limit)
    val end = System.nanoTime()
    val ms = (end - start) / 1_000_000.0

    println("LIMIT=$limit COUNT=$count TIME_MS=%.3f".format(ms))
}
