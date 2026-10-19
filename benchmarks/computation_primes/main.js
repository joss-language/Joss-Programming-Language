// Benchmark A: Prime Sieve (JavaScript / Node.js V8 implementation)
function countPrimes(limit) {
    if (limit < 2) return 0;
    const sieve = new Uint8Array(limit + 1);
    sieve.fill(1);
    sieve[0] = 0;
    sieve[1] = 0;
    for (let p = 2; p * p <= limit; p++) {
        if (sieve[p] === 1) {
            for (let i = p * p; i <= limit; i += p) {
                sieve[i] = 0;
            }
        }
    }
    let count = 0;
    for (let i = 2; i <= limit; i++) {
        if (sieve[i] === 1) count++;
    }
    return count;
}

const limit = process.argv[2] ? parseInt(process.argv[2], 10) : 1000000;
const start = performance.now();
const count = countPrimes(limit);
const end = performance.now();
const ms = end - start;
console.log(`LIMIT=${limit} COUNT=${count} TIME_MS=${ms.toFixed(3)}`);
