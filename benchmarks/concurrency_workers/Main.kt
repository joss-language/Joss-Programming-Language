import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.ArrayList

const val NUM_WORKERS = 8
const val TASKS_PER_WORKER = 100
const val INNER_ITERS = 500

fun main() {
    val start = System.nanoTime()

    val executor = Executors.newFixedThreadPool(NUM_WORKERS)
    val futures = ArrayList<Future<Long>>()

    for (w in 0 until NUM_WORKERS) {
        val startK = w * TASKS_PER_WORKER + 1
        val endK = (w + 1) * TASKS_PER_WORKER
        futures.add(executor.submit<Long> {
            var total = 0L
            for (k in startK..endK) {
                var taskSum = 0L
                for (j in 1..INNER_ITERS) {
                    taskSum += (j * 17 + k * 31) % 1000
                }
                total += taskSum
            }
            total
        })
    }

    var grandTotal = 0L
    for (f in futures) {
        grandTotal += f.get()
    }
    executor.shutdown()

    val end = System.nanoTime()
    val elapsedMs = (end - start) / 1_000_000.0

    System.out.printf("Concurrency 8 workers processed 800 tasks: checksum=%d in %.4f ms\n", grandTotal, elapsedMs)
}
