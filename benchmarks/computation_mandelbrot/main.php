<?php
function mandelbrot(int $N, int $maxIter): int {
    $total = 0;
    $fn = (float)$N;
    for ($y = 0; $y < $N; $y++) {
        $ci = -1.25 + (2.5 * (float)$y / $fn);
        for ($x = 0; $x < $N; $x++) {
            $cr = -2.0 + (2.5 * (float)$x / $fn);
            $zr = 0.0;
            $zi = 0.0;
            $it = 0;
            while ($it < $maxIter && ($zr * $zr + $zi * $zi) <= 4.0) {
                $zr2 = $zr * $zr;
                $zi2 = $zi * $zi;
                $zi = 2.0 * $zr * $zi + $ci;
                $zr = $zr2 - $zi2 + $cr;
                $it++;
            }
            $total += $it;
        }
    }
    return $total;
}

$t1 = hrtime(true);
$total = mandelbrot(500, 50);
$t2 = hrtime(true);
$elapsedMs = ($t2 - $t1) / 1e6;

printf("total=%d time=%.2fms\n", $total, $elapsedMs);
