import java.io.BufferedReader;
import java.io.FileReader;

public class Main {
    public static void main(String[] args) throws Exception {
        long start = System.nanoTime();

        int matchingCount = 0;
        double totalRawSales = 0.0;
        double totalTaxedValue = 0.0;

        try (BufferedReader br = new BufferedReader(new FileReader("benchmarks/io_csv_stats/data.csv"))) {
            // Skip header
            String line = br.readLine();
            while ((line = br.readLine()) != null) {
                String[] parts = line.split(",");
                if (parts.length >= 6) {
                    String region = parts[1];
                    int quantity = Integer.parseInt(parts[4]);
                    if (region.equals("North") && quantity >= 5) {
                        double sales = Double.parseDouble(parts[3]);
                        double tax = Double.parseDouble(parts[5]);

                        matchingCount++;
                        totalRawSales += sales;
                        totalTaxedValue += sales * quantity * (1.0 + tax);
                    }
                }
            }
        }

        long end = System.nanoTime();
        double elapsedMs = (end - start) / 1_000_000.0;

        System.out.printf("CSV Stats: matches=%d, raw=%.2f, taxed=%.2f in %.4f ms\n",
                matchingCount, totalRawSales, totalTaxedValue, elapsedMs);
    }
}
