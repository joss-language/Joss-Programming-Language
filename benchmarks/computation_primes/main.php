<?php
// Benchmark A: Prime Sieve (PHP implementation)

function count_primes(int $limit): int {
    if ($limit < 2) return 0;
    // Utilizar SplFixedArray para un arreglo de tamaño fijo con tipado eficiente
    $sieve = new SplFixedArray($limit + 1);
    for ($i = 2; $i <= $limit; $i++) {
        $sieve[$i] = 1;
    }

    for ($p = 2; $p * $p <= $limit; $p++) {
        if ($sieve[$p] === 1) {
            for ($i = $p * $p; $i <= $limit; $i += $p) {
                $sieve[$i] = 0;
            }
        }
    }

    $count = 0;
    for ($p = 2; $p <= $limit; $p++) {
        if ($sieve[$p] === 1) {
            $count++;
        }
    }
    return $count;
}

$limit = 1000000;
if ($argc > 1) {
    $limit = (int)$argv[1];
}

$start = hrtime(true);
$count = count_primes($limit);
$end = hrtime(true);
$ms = ($end - $start) / 1e6;

printf("LIMIT=%d COUNT=%d TIME_MS=%.3f\n", $limit, $count, $ms);
