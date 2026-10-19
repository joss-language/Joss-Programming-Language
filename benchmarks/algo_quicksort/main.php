<?php

$N = 50000;

function partition(&$arr, $low, $high) {
    $pivot = $arr[$high];
    $i = $low - 1;
    for ($j = $low; $j < $high; $j++) {
        if ($arr[$j] <= $pivot) {
            $i++;
            $t = $arr[$i];
            $arr[$i] = $arr[$j];
            $arr[$j] = $t;
        }
    }
    $t = $arr[$i + 1];
    $arr[$i + 1] = $arr[$high];
    $arr[$high] = $t;
    return $i + 1;
}

function quicksort(&$arr, $low, $high) {
    if ($low < $high) {
        $pi = partition($arr, $low, $high);
        quicksort($arr, $low, $pi - 1);
        quicksort($arr, $pi + 1, $high);
    }
}

function main() {
    global $N;
    $arr = new SplFixedArray($N);
    $state = 123456789;
    for ($i = 0; $i < $N; $i++) {
        $state = ($state * 1664525 + 1013904223) & 0xFFFFFFFF;
        $arr[$i] = $state & 0x7FFFFFFF;
    }

    $start = microtime(true);
    quicksort($arr, 0, $N - 1);
    $end = microtime(true);

    for ($i = 0; $i < $N - 1; $i++) {
        if ($arr[$i] > $arr[$i + 1]) {
            fwrite(STDERR, "Error: Array not sorted at $i\n");
            exit(1);
        }
    }

    $elapsed_ms = ($end - $start) * 1000.0;
    printf("Quicksort sorted %d elements in %.4f ms\n", $N, $elapsed_ms);
}

main();
