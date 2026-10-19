#include <iostream>
#include <chrono>

static int mandelbrot(int N, int maxIter) {
    int total = 0;
    double fn = static_cast<double>(N);
    for (int y = 0; y < N; y++) {
        double ci = -1.25 + (2.5 * static_cast<double>(y) / fn);
        for (int x = 0; x < N; x++) {
            double cr = -2.0 + (2.5 * static_cast<double>(x) / fn);
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

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();

    int total = mandelbrot(500, 50);

    auto t2 = std::chrono::high_resolution_clock::now();
    double elapsedMs = std::chrono::duration<double, std::milli>(t2 - t1).count();

    std::cout << "total=" << total << " time=" << elapsedMs << "ms\n";
    return 0;
}
