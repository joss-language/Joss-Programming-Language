#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <windows.h>

int main() {
    LARGE_INTEGER freq, start, end;
    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&start);

    FILE *f = fopen("benchmarks/io_csv_stats/data.csv", "r");
    if (!f) {
        fprintf(stderr, "Error opening file\n");
        return 1;
    }

    char line[256];
    // Skip header
    if (!fgets(line, sizeof(line), f)) {
        fclose(f);
        return 1;
    }

    int matching_count = 0;
    double total_raw_sales = 0.0;
    double total_taxed_value = 0.0;

    while (fgets(line, sizeof(line), f)) {
        int id;
        char region[16];
        char category[16];
        double sales_amount;
        int quantity;
        double tax_rate;

        // Parse CSV fields
        if (sscanf(line, "%d,%15[^,],%15[^,],%lf,%d,%lf",
                   &id, region, category, &sales_amount, &quantity, &tax_rate) == 6) {
            if (strcmp(region, "North") == 0 && quantity >= 5) {
                matching_count++;
                total_raw_sales += sales_amount;
                total_taxed_value += sales_amount * (double)quantity * (1.0 + tax_rate);
            }
        }
    }

    fclose(f);

    QueryPerformanceCounter(&end);
    double elapsed_ms = (double)(end.QuadPart - start.QuadPart) * 1000.0 / (double)freq.QuadPart;

    printf("CSV Stats: matches=%d, raw=%.2f, taxed=%.2f in %.4f ms\n",
           matching_count, total_raw_sales, total_taxed_value, elapsed_ms);
    return 0;
}
