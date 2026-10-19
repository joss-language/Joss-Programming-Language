// Benchmark A: Prime Sieve (Rust implementation)
use std::env;
use std::time::Instant;

fn count_primes(limit: usize) -> usize {
    if limit < 2 {
        return 0;
    }
    let mut sieve = vec![true; limit + 1];
    sieve[0] = false;
    sieve[1] = false;
    let mut p = 2;
    while p * p <= limit {
        if sieve[p] {
            let mut i = p * p;
            while i <= limit {
                sieve[i] = false;
                i += p;
            }
        }
        p += 1;
    }
    sieve.iter().filter(|&&is_p| is_p).count()
}

fn main() {
    let args: Vec<String> = env::args().collect();
    let limit: usize = if args.len() > 1 {
        args[1].parse().unwrap_or(1_000_000)
    } else {
        1_000_000
    };

    let start = Instant::now();
    let count = count_primes(limit);
    let duration = start.elapsed();
    let ms = duration.as_secs_f64() * 1000.0;

    println!("LIMIT={} COUNT={} TIME_MS={:.3}", limit, count, ms);
}
