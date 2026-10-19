<?php

$N = 50000;

function main() {
    global $N;

    $start = microtime(true);

    $map = [];

    // Insertion
    for ($i = 0; $i < $N; $i++) {
        $map["user_key_" . $i] = "val_" . $i;
    }

    // Lookup
    $hits = 0;
    for ($i = 0; $i < $N; $i++) {
        $key = "user_key_" . $i;
        $expected = "val_" . $i;
        if (isset($map[$key]) && $map[$key] === $expected) {
            $hits++;
        }
    }

    $end = microtime(true);
    $elapsed_ms = ($end - $start) * 1000.0;

    if ($hits !== $N) {
        fwrite(STDERR, "Verification failed: hits = $hits expected $N\n");
        exit(1);
    }

    printf("HashMap inserted and queried %d items (%d hits) in %.4f ms\n", $N, $hits, $elapsed_ms);
}

main();
