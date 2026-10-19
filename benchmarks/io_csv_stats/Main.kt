import java.io.BufferedReader
import java.io.FileReader

fun main() {
    val start = System.nanoTime()

    var matchingCount = 0
    var totalRawSales = 0.0
    var totalTaxedValue = 0.0

    BufferedReader(FileReader("benchmarks/io_csv_stats/data.csv")).use { br ->
        // Skip header
        br.readLine()
        var line: String? = br.readLine()
        while (line != null) {
            val parts = line.split(",")
            if (parts.size >= 6) {
                val region = parts[1]
                val quantity = parts[4].toInt()
                if (region == "North" && quantity >= 5) {
                    val sales = parts[3].toDouble()
                    val tax = parts[5].toDouble()

                    matchingCount++
                    totalRawSales += sales
                    totalTaxedValue += sales * quantity * (1.0 + tax)
                }
            }
            line = br.readLine()
        }
    }

    val end = System.nanoTime()
    val elapsedMs = (end - start) / 1_000_000.0

    System.out.printf("CSV Stats: matches=%d, raw=%.2f, taxed=%.2f in %.4f ms\n",
        matchingCount, totalRawSales, totalTaxedValue, elapsedMs)
}
