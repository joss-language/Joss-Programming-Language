// Benchmark A: Prime Sieve (C++ implementation)
#include <iostream>
#include <vector>
#include <chrono>
#include <cstdlib>

long long count_primes(int limit) {
    if (limit < 2) return 0;
    std::vector<bool> is_prime(limit + 1, true);
    is_prime[0] = false;
    is_prime[1] = false;
    for (long long p = 2; p * p <= limit; p++) {
        if (is_prime[p]) {
            for (long long i = p * p; i <= limit; i += p) {
                is_prime[i] = false;
            }
        }
    }
    long long count = 0;
    for (int p = 2; p <= limit; p++) {
        if (is_prime[p]) count++;
    }
    return count;
}

int main(int argc, char** argv) {
    int limit = 1000000;
    if (argc > 1) limit = std::atoi(argv[1]);

    auto start = std::chrono::high_resolution_clock::now();
    long long count = count_primes(limit);
    auto end = std::chrono::high_resolution_clock::now();
    double ms = std::chrono::duration<double, std::milli>(end - start).count();

    std::cout << "LIMIT=" << limit << " COUNT=" << count << " TIME_MS=" << ms << std::endl;
    return 0;
}
