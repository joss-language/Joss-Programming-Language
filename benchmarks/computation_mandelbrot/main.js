function mandelbrot(N, maxIter) {
    let total = 0;
    const fn = N;
    for (let y = 0; y < N; y++) {
        const ci = -1.25 + (2.5 * y / fn);
        for (let x = 0; x < N; x++) {
            const cr = -2.0 + (2.5 * x / fn);
            let zr = 0.0;
            let zi = 0.0;
            let it = 0;
            while (it < maxIter && (zr * zr + zi * zi) <= 4.0) {
                const zr2 = zr * zr;
                const zi2 = zi * zi;
                zi = 2.0 * zr * zi + ci;
                zr = zr2 - zi2 + cr;
                it++;
            }
            total += it;
        }
    }
    return total;
}

const t1 = performance.now();
const total = mandelbrot(500, 50);
const t2 = performance.now();
const ms = t2 - t1;
console.log(`total=${total} time=${ms.toFixed(2)}ms`);
