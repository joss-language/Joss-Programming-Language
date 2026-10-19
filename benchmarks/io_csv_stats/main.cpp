#include <iostream>
#include <fstream>
#include <string>
#include <sstream>
#include <chrono>

int main() {
    auto start = std::chrono::high_resolution_clock::now();

    std::ifstream file("benchmarks/io_csv_stats/data.csv");
    if (!file.is_open()) {
        std::cerr << "Error opening data.csv" << std::endl;
        return 1;
    }

    std::string line;
    // Skip header
    std::getline(file, line);

    int matching_count = 0;
    double total_raw_sales = 0.0;
    double total_taxed_value = 0.0;

    while (std::getline(file, line)) {
        std::stringstream ss(line);
        std::string item;

        std::string id_str, region, category, sales_str, qty_str, tax_str;
        if (std::getline(ss, id_str, ',') &&
            std::getline(ss, region, ',') &&
            std::getline(ss, category, ',') &&
            std::getline(ss, sales_str, ',') &&
            std::getline(ss, qty_str, ',') &&
            std::getline(ss, tax_str, ',')) {

            int quantity = std::stoi(qty_str);
            if (region == "North" && quantity >= 5) {
                double sales = std::stod(sales_str);
                double tax = std::stod(tax_str);

                matching_count++;
                total_raw_sales += sales;
                total_taxed_value += sales * static_cast<double>(quantity) * (1.0 + tax);
            }
        }
    }

    auto end = std::chrono::high_resolution_clock::now();
    double elapsed_ms = std::chrono::duration<double, std::milli>(end - start).count();

    std::cout << "CSV Stats: matches=" << matching_count
              << ", raw=" << total_raw_sales
              << ", taxed=" << total_taxed_value
              << " in " << elapsed_ms << " ms" << std::endl;
    return 0;
}
