import java.util.HashMap;

public class Main {
    private static final int N = 50000;

    public static void main(String[] args) {
        long start = System.nanoTime();

        HashMap<String, String> map = new HashMap<>(N);

        // Insertion
        for (int i = 0; i < N; i++) {
            String key = "user_key_" + i;
            String val = "val_" + i;
            map.put(key, val);
        }

        // Lookup
        int hits = 0;
        for (int i = 0; i < N; i++) {
            String key = "user_key_" + i;
            String val = map.get(key);
            if (val != null && val.equals("val_" + i)) {
                hits++;
            }
        }

        long end = System.nanoTime();
        double elapsedMs = (end - start) / 1_000_000.0;

        if (hits != N) {
            System.err.println("Verification failed: hits = " + hits + " expected " + N);
            System.exit(1);
        }

        System.out.printf("HashMap inserted and queried %d items (%d hits) in %.4f ms\n", N, hits, elapsedMs);
    }
}
