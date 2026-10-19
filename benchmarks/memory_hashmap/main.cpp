#include <iostream>
#include <unordered_map>
#include <string>
#include <chrono>

constexpr int N = 50000;

int main() {
    auto start = std::chrono::high_resolution_clock::now();

    std::unordered_map<std::string, std::string> map;
    map.reserve(N);

    // Insertion
    for (int i = 0; i < N; i++) {
        std::string key = "user_key_" + std::to_string(i);
        std::string val = "val_" + std::to_string(i);
        map[key] = val;
    }

    // Lookup
    int hits = 0;
    for (int i = 0; i < N; i++) {
        std::string key = "user_key_" + std::to_string(i);
        auto it = map.find(key);
        if (it != map.end() && it->second == ("val_" + std::to_string(i))) {
            hits++;
        }
    }

    auto end = std::chrono::high_resolution_clock::now();
    double elapsed_ms = std::chrono::duration<double, std::milli>(end - start).count();

    if (hits != N) {
        std::cerr << "Verification failed: hits = " << hits << " expected " << N << std::endl;
        return 1;
    }

    std::cout << "HashMap inserted and queried " << N << " items (" << hits << " hits) in " << elapsed_ms << " ms" << std::endl;
    return 0;
}
