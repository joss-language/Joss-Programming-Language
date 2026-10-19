// Benchmark A: Prime Sieve (C implementation)
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <time.h>

long long count_primes(int limit) {
    if (limit < 2) return 0;
    uint8_t *sieve = (uint8_t *)calloc(limit + 1, sizeof(uint8_t));
    if (!sieve) return 0;
    sieve[0] = 1;
    sieve[1] = 1;
    for (int p = 2; (long long)p * p <= limit; p++) {
        if (!sieve[p]) {
            for (int i = p * p; i <= limit; i += p) {
                sieve[i] = 1;
            }
        }
    }
    long long count = 0;
    for (int p = 2; p <= limit; p++) {
        if (!sieve[p]) count++;
    }
    free(sieve);
    return count;
}

int main(int argc, char **argv) {
    int limit = 1000000;
    if (argc > 1) limit = atoi(argv[1]);
    
    clock_t start = clock();
    long long count = count_primes(limit);
    clock_t end = clock();
    double ms = ((double)(end - start)) / CLOCKS_PER_SEC * 1000.0;
    
    printf("LIMIT=%d COUNT=%lld TIME_MS=%.3f\n", limit, count, ms);
    return 0;
}
