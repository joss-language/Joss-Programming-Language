<?php

function main() {
    $start = microtime(true);

    $handle = fopen("benchmarks/io_csv_stats/data.csv", "r");
    if (!$handle) {
        fwrite(STDERR, "Error opening file\n");
        exit(1);
    }

    // Skip header
    fgetcsv($handle);

    $matchingCount = 0;
    $totalRawSales = 0.0;
    $totalTaxedValue = 0.0;

    while (($row = fgetcsv($handle)) !== false) {
        if (count($row) >= 6) {
            $region = $row[1];
            $quantity = (int)$row[4];
            if ($region === "North" && $quantity >= 5) {
                $sales = (float)$row[3];
                $tax = (float)$row[5];

                $matchingCount++;
                $totalRawSales += $sales;
                $totalTaxedValue += $sales * $quantity * (1.0 + $tax);
            }
        }
    }

    fclose($handle);

    $end = microtime(true);
    $elapsed_ms = ($end - $start) * 1000.0;

    printf("CSV Stats: matches=%d, raw=%.2f, taxed=%.2f in %.4f ms\n",
        $matchingCount, $totalRawSales, $totalTaxedValue, $elapsed_ms);
}

main();
