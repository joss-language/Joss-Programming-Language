use std::collections::HashMap;
use std::time::Instant;

const N: usize = 50000;

fn main() {
    let start = Instant::now();

    let mut map = HashMap::with_capacity(N);

    // Insertion
    for i in 0..N {
        let key = format!("user_key_{}", i);
        let val = format!("val_{}", i);
        map.insert(key, val);
    }

    // Lookup
    let mut hits = 0;
    for i in 0..N {
        let key = format!("user_key_{}", i);
        let expected_val = format!("val_{}", i);
        if let Some(val) = map.get(&key) {
            if val == &expected_val {
                hits += 1;
            }
        }
    }

    let duration = start.elapsed();
    let elapsed_ms = duration.as_secs_f64() * 1000.0;

    if hits != N {
        eprintln!("Verification failed: hits = {} expected {}", hits, N);
        std::process::exit(1);
    }

    println!("HashMap inserted and queried {} items ({} hits) in {:.4} ms", N, hits, elapsed_ms);
}
