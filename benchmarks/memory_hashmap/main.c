#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <windows.h>

#define N 50000
#define BUCKETS 65536

typedef struct Node {
    char key[24];
    char val[16];
    struct Node *next;
} Node;

static inline unsigned int hash_str(const char *str) {
    unsigned int hash = 5381;
    int c;
    while ((c = *str++))
        hash = ((hash << 5) + hash) + c;
    return hash % BUCKETS;
}

int main() {
    LARGE_INTEGER freq, start, end;
    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&start);

    Node **table = (Node **)calloc(BUCKETS, sizeof(Node *));
    if (!table) return 1;

    char key_buf[24];
    char val_buf[16];

    // Insertion
    for (int i = 0; i < N; i++) {
        sprintf(key_buf, "user_key_%d", i);
        sprintf(val_buf, "val_%d", i);
        unsigned int h = hash_str(key_buf);
        Node *node = (Node *)malloc(sizeof(Node));
        strcpy(node->key, key_buf);
        strcpy(node->val, val_buf);
        node->next = table[h];
        table[h] = node;
    }

    // Lookup
    int hits = 0;
    for (int i = 0; i < N; i++) {
        sprintf(key_buf, "user_key_%d", i);
        sprintf(val_buf, "val_%d", i);
        unsigned int h = hash_str(key_buf);
        Node *curr = table[h];
        while (curr) {
            if (strcmp(curr->key, key_buf) == 0 && strcmp(curr->val, val_buf) == 0) {
                hits++;
                break;
            }
            curr = curr->next;
        }
    }

    QueryPerformanceCounter(&end);
    double elapsed_ms = (double)(end.QuadPart - start.QuadPart) * 1000.0 / (double)freq.QuadPart;

    if (hits != N) {
        fprintf(stderr, "Verification failed: hits = %d expected %d\n", hits, N);
        return 1;
    }

    printf("HashMap inserted and queried %d items (%d hits) in %.4f ms\n", N, hits, elapsed_ms);

    // Free memory
    for (int b = 0; b < BUCKETS; b++) {
        Node *curr = table[b];
        while (curr) {
            Node *tmp = curr;
            curr = curr->next;
            free(tmp);
        }
    }
    free(table);
    return 0;
}
