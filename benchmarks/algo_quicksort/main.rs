use std::time::Instant;

const N: usize = 50000;

fn partition(arr: &mut [i32], low: usize, high: usize) -> usize {
    let pivot = arr[high];
    let mut i = low;
    for j in low..high {
        if arr[j] <= pivot {
            arr.swap(i, j);
            i += 1;
        }
    }
    arr.swap(i, high);
    i
}

fn quicksort(arr: &mut [i32], low: usize, high: usize) {
    if low < high {
        let pi = partition(arr, low, high);
        if pi > 0 {
            quicksort(arr, low, pi - 1);
        }
        quicksort(arr, pi + 1, high);
    }
}

fn main() {
    let mut arr = vec![0i32; N];
    let mut state: u32 = 123456789;
    for i in 0..N {
        state = state.wrapping_mul(1664525).wrapping_add(1013904223);
        arr[i] = (state & 0x7FFFFFFF) as i32;
    }

    let start = Instant::now();
    quicksort(&mut arr, 0, N - 1);
    let duration = start.elapsed();

    for i in 0..N - 1 {
        if arr[i] > arr[i + 1] {
            eprintln!("Error: Array not sorted at {}", i);
            std::process::exit(1);
        }
    }

    let elapsed_ms = duration.as_secs_f64() * 1000.0;
    println!("Quicksort sorted {} elements in {:.4} ms", N, elapsed_ms);
}
