use std::sync::mpsc;
use std::thread;
use std::time::Instant;

const NUM_WORKERS: usize = 8;
const TASKS_PER_WORKER: usize = 100;
const INNER_ITERS: usize = 500;

fn main() {
    let start = Instant::now();

    let (tx, rx) = mpsc::channel();

    for w in 0..NUM_WORKERS {
        let tx_clone = tx.clone();
        thread::spawn(move || {
            let start_k = w * TASKS_PER_WORKER + 1;
            let end_k = (w + 1) * TASKS_PER_WORKER;
            let mut total = 0i64;
            for k in start_k..=end_k {
                let mut task_sum = 0i64;
                for j in 1..=INNER_ITERS {
                    task_sum += ((j * 17 + k * 31) % 1000) as i64;
                }
                total += task_sum;
            }
            tx_clone.send(total).unwrap();
        });
    }

    drop(tx);

    let mut grand_total = 0i64;
    for received in rx {
        grand_total += received;
    }

    let elapsed = start.elapsed();
    let elapsed_ms = elapsed.as_secs_f64() * 1000.0;

    println!("Concurrency 8 workers processed 800 tasks: checksum={} in {:.4} ms", grand_total, elapsed_ms);
}
