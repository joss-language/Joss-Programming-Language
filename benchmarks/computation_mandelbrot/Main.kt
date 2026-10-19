fun mandelbrot(n: Int, maxIter: Int): Int {
    var total = 0
    val fn = n.toDouble()
    for (y in 0 until n) {
        val ci = -1.25 + (2.5 * y.toDouble() / fn)
        for (x in 0 until n) {
            val cr = -2.0 + (2.5 * x.toDouble() / fn)
            var zr = 0.0
            var zi = 0.0
            var it = 0
            while (it < maxIter && (zr * zr + zi * zi) <= 4.0) {
                val zr2 = zr * zr
                val zi2 = zi * zi
                zi = 2.0 * zr * zi + ci
                zr = zr2 - zi2 + cr
                it++
            }
            total += it
        }
    }
    return total
}

fun main() {
    val t1 = System.nanoTime()
    val total = mandelbrot(500, 50)
    val t2 = System.nanoTime()
    val elapsedMs = (t2 - t1) / 1_000_000.0
    println("total=$total time=${"%.2f".format(elapsedMs)}ms")
}
