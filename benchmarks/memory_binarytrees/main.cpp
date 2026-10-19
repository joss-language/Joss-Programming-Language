#include <iostream>
#include <chrono>
#include <memory>

struct Node {
    std::unique_ptr<Node> left;
    std::unique_ptr<Node> right;
    int item;

    Node(int item, std::unique_ptr<Node> l, std::unique_ptr<Node> r)
        : left(std::move(l)), right(std::move(r)), item(item) {}

    int check() const {
        if (!left) return item;
        return item + left->check() - right->check();
    }
};

std::unique_ptr<Node> bottomUpTree(int item, int depth) {
    if (depth > 0) {
        return std::make_unique<Node>(
            item,
            bottomUpTree(2 * item - 1, depth - 1),
            bottomUpTree(2 * item, depth - 1)
        );
    }
    return std::make_unique<Node>(item, nullptr, nullptr);
}

int main() {
    const int maxDepth = 14;
    const int minDepth = 4;
    const int stretchDepth = maxDepth + 1;

    auto start = std::chrono::high_resolution_clock::now();

    auto stretchTree = bottomUpTree(0, stretchDepth);
    int stretchCheck = stretchTree->check();
    stretchTree.reset();

    auto longLivedTree = bottomUpTree(0, maxDepth);

    int totalChecks = 0;
    for (int depth = minDepth; depth <= maxDepth; depth += 2) {
        int iterations = 1 << (maxDepth - depth + minDepth);
        int check = 0;
        for (int i = 1; i <= iterations; ++i) {
            auto t1 = bottomUpTree(i, depth);
            auto t2 = bottomUpTree(-i, depth);
            check += t1->check() + t2->check();
        }
        totalChecks += check;
    }

    int longLivedCheck = longLivedTree->check();
    longLivedTree.reset();

    auto end = std::chrono::high_resolution_clock::now();
    double elapsed_ms = std::chrono::duration<double, std::milli>(end - start).count();

    std::cout << "stretch tree depth " << stretchDepth << " check: " << stretchCheck << "\n";
    std::cout << "long lived tree depth " << maxDepth << " check: " << longLivedCheck << "\n";
    std::cout << "total checks: " << totalChecks << "\n";
    std::cout << "Binary trees (depth " << maxDepth << ") completed in " << elapsed_ms << " ms\n";

    return 0;
}
