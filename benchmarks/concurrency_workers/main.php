<?php

const NUM_WORKERS = 8;
const TASKS_PER_WORKER = 100;
const INNER_ITERS = 500;

function worker_task($start_k, $end_k) {
    $total = 0;
    for ($k = $start_k; $k <= $end_k; $k++) {
        $task_sum = 0;
        for ($j = 1; $j <= INNER_ITERS; $j++) {
            $task_sum += ($j * 17 + $k * 31) % 1000;
        }
        $total += $task_sum;
    }
    return $total;
}

function main() {
    $start = microtime(true);

    $grand_total = 0;
    for ($w = 0; $w < NUM_WORKERS; $w++) {
        $start_k = $w * TASKS_PER_WORKER + 1;
        $end_k = ($w + 1) * TASKS_PER_WORKER;
        $grand_total += worker_task($start_k, $end_k);
    }

    $end = microtime(true);
    $elapsed_ms = ($end - $start) * 1000.0;

    printf("Concurrency 8 workers processed 800 tasks: checksum=%d in %.4f ms\n", $grand_total, $elapsed_ms);
}

main();
