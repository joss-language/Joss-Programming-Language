public class Main {
    public static int mandelbrot(int N, int maxIter) {
        int total = 0;
        double fn = (double) N;
        for (int y = 0; y < N; y++) {
            double ci = -1.25 + (2.5 * (double) y / fn);
            for (int x = 0; x < N; x++) {
                double cr = -2.0 + (2.5 * (double) x / fn);
                double zr = 0.0;
                double zi = 0.0;
                int it = 0;
                while (it < maxIter && (zr * zr + zi * zi) <= 4.0) {
                    double zr2 = zr * zr;
                    double zi2 = zi * zi;
                    zi = 2.0 * zr * zi + ci;
                    zr = zr2 - zi2 + cr;
                    it++;
                }
                total += it;
            }
        }
        return total;
    }

    public static void main(String[] args) {
        long t1 = System.nanoTime();
        int total = mandelbrot(500, 50);
        long t2 = System.nanoTime();
        double elapsedMs = (t2 - t1) / 1_000_000.0;
        System.out.printf("total=%d time=%.2fms\n", total, elapsedMs);
    }
}
