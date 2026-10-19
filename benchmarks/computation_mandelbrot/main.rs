use std::time::Instant;

fn mandelbrot(n: usize, max_iter: usize) -> usize {
    let mut total = 0;
    let fn_n = n as f64;
    for y in 0..n {
        let ci = -1.25 + (2.5 * (y as f64) / fn_n);
        for x in 0..n {
            let cr = -2.0 + (2.5 * (x as f64) / fn_n);
            let mut zr = 0.0f64;
            let mut zi = 0.0f64;
            let mut it = 0;
            while it < max_iter && (zr * zr + zi * zi) <= 4.0 {
                let zr2 = zr * zr;
                let zi2 = zi * zi;
                zi = 2.0 * zr * zi + ci;
                zr = zr2 - zi2 + cr;
                it += 1;
            }
            total += it;
        }
    }
    total
}

fn main() {
    let start = Instant::now();
    let total = mandelbrot(500, 50);
    let elapsed = start.elapsed();
    let elapsed_ms = (elapsed.as_nanos() as f64) / 1_000_000.0;
    println!("total={} time={:.2}ms", total, elapsed_ms);
}
