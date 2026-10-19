#include <stdio.h>
#include <stdlib.h>
#include <windows.h>

typedef struct Node {
    struct Node *left;
    struct Node *right;
    int item;
} Node;

Node *bottomUpTree(int item, int depth) {
    Node *n = (Node *)malloc(sizeof(Node));
    n->item = item;
    if (depth > 0) {
        n->left = bottomUpTree(2 * item - 1, depth - 1);
        n->right = bottomUpTree(2 * item, depth - 1);
    } else {
        n->left = NULL;
        n->right = NULL;
    }
    return n;
}

int itemCheck(Node *tree) {
    if (tree->left == NULL) {
        return tree->item;
    }
    return tree->item + itemCheck(tree->left) - itemCheck(tree->right);
}

void deleteTree(Node *tree) {
    if (tree->left != NULL) {
        deleteTree(tree->left);
        deleteTree(tree->right);
    }
    free(tree);
}

int main() {
    int maxDepth = 14;
    int minDepth = 4;
    int stretchDepth = maxDepth + 1;

    LARGE_INTEGER freq, start, end;
    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&start);

    Node *stretchTree = bottomUpTree(0, stretchDepth);
    int stretchCheck = itemCheck(stretchTree);
    deleteTree(stretchTree);

    Node *longLivedTree = bottomUpTree(0, maxDepth);

    int totalChecks = 0;
    for (int depth = minDepth; depth <= maxDepth; depth += 2) {
        int iterations = 1 << (maxDepth - depth + minDepth);
        int check = 0;
        for (int i = 1; i <= iterations; i++) {
            Node *t1 = bottomUpTree(i, depth);
            Node *t2 = bottomUpTree(-i, depth);
            check += itemCheck(t1) + itemCheck(t2);
            deleteTree(t1);
            deleteTree(t2);
        }
        totalChecks += check;
    }

    int longLivedCheck = itemCheck(longLivedTree);
    deleteTree(longLivedTree);

    QueryPerformanceCounter(&end);
    double elapsed_ms = (double)(end.QuadPart - start.QuadPart) * 1000.0 / (double)freq.QuadPart;

    printf("stretch tree depth %d check: %d\n", stretchDepth, stretchCheck);
    printf("long lived tree depth %d check: %d\n", maxDepth, longLivedCheck);
    printf("total checks: %d\n", totalChecks);
    printf("Binary trees (depth %d) completed in %.4f ms\n", maxDepth, elapsed_ms);

    return 0;
}
