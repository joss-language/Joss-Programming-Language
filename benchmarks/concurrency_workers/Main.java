import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.ArrayList;
import java.util.List;

public class Main {
    private static final int NUM_WORKERS = 8;
    private static final int TASKS_PER_WORKER = 100;
    private static final int INNER_ITERS = 500;

    public static void main(String[] args) throws Exception {
        long start = System.nanoTime();

        ExecutorService executor = Executors.newFixedThreadPool(NUM_WORKERS);
        List<Future<Long>> futures = new ArrayList<>();

        for (int w = 0; w < NUM_WORKERS; w++) {
            final int startK = w * TASKS_PER_WORKER + 1;
            final int endK = (w + 1) * TASKS_PER_WORKER;
            futures.add(executor.submit(() -> {
                long total = 0;
                for (int k = startK; k <= endK; k++) {
                    long taskSum = 0;
                    for (int j = 1; j <= INNER_ITERS; j++) {
                        taskSum += (j * 17 + k * 31) % 1000;
                    }
                    total += taskSum;
                }
                return total;
            }));
        }

        long grandTotal = 0;
        for (Future<Long> f : futures) {
            grandTotal += f.get();
        }
        executor.shutdown();

        long end = System.nanoTime();
        double elapsedMs = (end - start) / 1_000_000.0;

        System.out.printf("Concurrency 8 workers processed 800 tasks: checksum=%d in %.4f ms\n", grandTotal, elapsedMs);
    }
}
