#include <stdio.h>
#include <windows.h>

static int mandelbrot(int N, int maxIter) {
    int total = 0;
    double fn = (double)N;
    for (int y = 0; y < N; y++) {
        double ci = -1.25 + (2.5 * (double)y / fn);
        for (int x = 0; x < N; x++) {
            double cr = -2.0 + (2.5 * (double)x / fn);
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

int main(void) {
    LARGE_INTEGER freq, t1, t2;
    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&t1);

    int total = mandelbrot(500, 50);

    QueryPerformanceCounter(&t2);
    double elapsedMs = (double)(t2.QuadPart - t1.QuadPart) * 1000.0 / (double)freq.QuadPart;

    printf("total=%d time=%.2fms\n", total, elapsedMs);
    return 0;
}
