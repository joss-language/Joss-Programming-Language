// Benchmark A: Prime Sieve (Java implementation)
import java.util.Arrays;

public class Main {
    public static long countPrimes(int limit) {
        if (limit < 2) return 0;
        boolean[] isPrime = new boolean[limit + 1];
        Arrays.fill(isPrime, true);
        isPrime[0] = false;
        isPrime[1] = false;

        for (int p = 2; (long) p * p <= limit; p++) {
            if (isPrime[p]) {
                for (int i = p * p; i <= limit; i += p) {
                    isPrime[i] = false;
                }
            }
        }

        long count = 0;
        for (int p = 2; p <= limit; p++) {
            if (isPrime[p]) count++;
        }
        return count;
    }

    public static void main(String[] args) {
        int limit = 1000000;
        if (args.length > 0) {
            try {
                limit = Integer.parseInt(args[0]);
            } catch (NumberFormatException ignored) {}
        }

        long start = System.nanoTime();
        long count = countPrimes(limit);
        long end = System.nanoTime();
        double ms = (end - start) / 1_000_000.0;

        System.out.printf("LIMIT=%d COUNT=%d TIME_MS=%.3f\n", limit, count, ms);
    }
}
