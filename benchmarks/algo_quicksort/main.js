const N = 50000;

function partition(arr, low, high) {
    const pivot = arr[high];
    let i = low - 1;
    for (let j = low; j < high; j++) {
        if (arr[j] <= pivot) {
            i++;
            const tmp = arr[i];
            arr[i] = arr[j];
            arr[j] = tmp;
        }
    }
    const tmp = arr[i + 1];
    arr[i + 1] = arr[high];
    arr[high] = tmp;
    return i + 1;
}

function quicksort(arr, low, high) {
    if (low < high) {
        const pi = partition(arr, low, high);
        quicksort(arr, low, pi - 1);
        quicksort(arr, pi + 1, high);
    }
}

function main() {
    let state = 123456789;
    const arr = new Int32Array(N);
    for (let i = 0; i < N; i++) {
        state = Math.imul(state, 1664525) + 1013904223 | 0;
        arr[i] = state & 0x7FFFFFFF;
    }

    const start = performance.now();
    quicksort(arr, 0, N - 1);
    const end = performance.now();

    for (let i = 0; i < N - 1; i++) {
        if (arr[i] > arr[i + 1]) {
            console.error(`Error: Array not sorted at ${i}`);
            process.exit(1);
        }
    }

    const elapsedMs = end - start;
    console.log(`Quicksort sorted ${N} elements in ${elapsedMs.toFixed(4)} ms`);
}

main();
