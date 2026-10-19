use std::fs::File;
use std::io::{BufRead, BufReader};
use std::time::Instant;

fn main() {
    let start = Instant::now();

    let file = File::open("benchmarks/io_csv_stats/data.csv").expect("Failed to open data.csv");
    let mut reader = BufReader::new(file);

    let mut line = String::new();
    // Skip header
    reader.read_line(&mut line).unwrap();

    let mut matching_count = 0;
    let mut total_raw_sales = 0.0f64;
    let mut total_taxed_value = 0.0f64;

    line.clear();
    while reader.read_line(&mut line).unwrap() > 0 {
        let parts: Vec<&str> = line.trim_end().split(',').collect();
        if parts.len() >= 6 {
            let region = parts[1];
            let quantity: i32 = parts[4].parse().unwrap_or(0);

            if region == "North" && quantity >= 5 {
                let sales: f64 = parts[3].parse().unwrap_or(0.0);
                let tax: f64 = parts[5].parse().unwrap_or(0.0);

                matching_count += 1;
                total_raw_sales += sales;
                total_taxed_value += sales * (quantity as f64) * (1.0 + tax);
            }
        }
        line.clear();
    }

    let elapsed = start.elapsed();
    let elapsed_ms = elapsed.as_secs_f64() * 1000.0;

    println!(
        "CSV Stats: matches={}, raw={:.2}, taxed={:.2} in {:.4} ms",
        matching_count, total_raw_sales, total_taxed_value, elapsed_ms
    );
}
