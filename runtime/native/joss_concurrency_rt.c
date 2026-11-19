//go:build ignore
#include "joss_rt.h"

#include <stdlib.h>
#include <string.h>

#if defined(__unix__) || defined(__APPLE__)
#include <pthread.h>
#include <unistd.h>
#define JOSS_HAS_PTHREADS 1
#endif

/* =========================================================================
 * 1. Automatic Dynamic Programming (Auto-DP) Lock-Free Thread-Local Cache
 * ========================================================================= */
#define JOSS_DP_SLOTS 8192U
#define JOSS_DP_MASK  (JOSS_DP_SLOTS - 1U)

typedef struct {
    uint64_t key_hash;
    int64_t  fn_id;
    int64_t  a0;
    int64_t  a1;
    int64_t  val;
    uint8_t  occupied;
} JossDPEntry;

#if defined(__STDC_VERSION__) && __STDC_VERSION__ >= 201112L
static _Thread_local JossDPEntry* tls_dp_table = NULL;
#else
static __thread JossDPEntry* tls_dp_table = NULL;
#endif

static inline uint64_t dp_hash(int64_t fn_id, int64_t a0, int64_t a1) {
    uint64_t h = (uint64_t)fn_id * 0x9E3779B97F4A7C15ULL;
    h ^= (uint64_t)a0 * 0xBF58476D1CE4E5B9ULL;
    h ^= (h >> 30);
    h ^= (uint64_t)a1 * 0x94D049BB133111EBULL;
    h ^= (h >> 27);
    return h ? h : 1ULL;
}

bool joss_dp_get(int64_t fn_id, int64_t a0, int64_t a1, int64_t* out_val) {
    if (!tls_dp_table) return false;
    uint64_t h = dp_hash(fn_id, a0, a1);
    uint32_t idx = (uint32_t)(h & JOSS_DP_MASK);
    JossDPEntry* e = &tls_dp_table[idx];
    if (e->occupied && e->key_hash == h && e->fn_id == fn_id && e->a0 == a0 && e->a1 == a1) {
        if (out_val) *out_val = e->val;
        return true;
    }
    return false;
}

void joss_dp_put(int64_t fn_id, int64_t a0, int64_t a1, int64_t val) {
    if (!tls_dp_table) {
        tls_dp_table = (JossDPEntry*)calloc(JOSS_DP_SLOTS, sizeof(JossDPEntry));
        if (!tls_dp_table) return;
    }
    uint64_t h = dp_hash(fn_id, a0, a1);
    uint32_t idx = (uint32_t)(h & JOSS_DP_MASK);
    JossDPEntry* e = &tls_dp_table[idx];
    e->key_hash = h;
    e->fn_id = fn_id;
    e->a0 = a0;
    e->a1 = a1;
    e->val = val;
    e->occupied = 1;
}

/* =========================================================================
 * 2. Parallel Multi-Core Goroutine-Style Task Scheduler (pthreads)
 * ========================================================================= */
typedef int64_t (*JossTaskFn)(void);

#if defined(JOSS_HAS_PTHREADS)

typedef struct JossFuture {
    pthread_mutex_t    mu;
    pthread_cond_t     cv;
    JossTaskFn         fn;
    int64_t            result;
    bool               done;
    struct JossFuture* next;
} JossFuture;

static pthread_once_t  g_pool_once = PTHREAD_ONCE_INIT;
static pthread_mutex_t g_pool_mu   = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t  g_pool_cv   = PTHREAD_COND_INITIALIZER;
static JossFuture*     g_q_head    = NULL;
static JossFuture*     g_q_tail    = NULL;

static void* joss_worker_loop(void* arg) {
    (void)arg;
    for (;;) {
        pthread_mutex_lock(&g_pool_mu);
        while (!g_q_head) {
            pthread_cond_wait(&g_pool_cv, &g_pool_mu);
        }
        JossFuture* task = g_q_head;
        g_q_head = task->next;
        if (!g_q_head) g_q_tail = NULL;
        pthread_mutex_unlock(&g_pool_mu);

        int64_t res = 0;
        if (task->fn) {
            res = task->fn();
        }

        pthread_mutex_lock(&task->mu);
        task->result = res;
        task->done = true;
        pthread_cond_signal(&task->cv);
        pthread_mutex_unlock(&task->mu);
    }
    return NULL;
}

static void joss_pool_init(void) {
    long ncpu = sysconf(_SC_NPROCESSORS_ONLN);
    if (ncpu < 2) ncpu = 2;
    if (ncpu > 32) ncpu = 32;
    for (long i = 0; i < ncpu; i++) {
        pthread_t tid;
        if (pthread_create(&tid, NULL, joss_worker_loop, NULL) == 0) {
            pthread_detach(tid);
        }
    }
}

int64_t joss_async(int64_t fn_ptr) {
    pthread_once(&g_pool_once, joss_pool_init);
    JossFuture* fut = (JossFuture*)calloc(1, sizeof(JossFuture));
    if (!fut) joss_panic("out of memory allocating async future");
    pthread_mutex_init(&fut->mu, NULL);
    pthread_cond_init(&fut->cv, NULL);
    fut->fn = (JossTaskFn)(uintptr_t)fn_ptr;
    fut->done = false;

    pthread_mutex_lock(&g_pool_mu);
    if (g_q_tail) {
        g_q_tail->next = fut;
        g_q_tail = fut;
    } else {
        g_q_head = g_q_tail = fut;
    }
    pthread_cond_signal(&g_pool_cv);
    pthread_mutex_unlock(&g_pool_mu);

    return (int64_t)(uintptr_t)fut;
}

int64_t joss_await(int64_t fut_handle) {
    uint64_t u = (uint64_t)fut_handle;
    if (u < JOSS_MIN_PTR_ADDR || (u >> 47) != 0) return fut_handle;
    JossFuture* fut = (JossFuture*)(uintptr_t)u;
    pthread_mutex_lock(&fut->mu);
    while (!fut->done) {
        pthread_cond_wait(&fut->cv, &fut->mu);
    }
    int64_t res = fut->result;
    pthread_mutex_unlock(&fut->mu);
    pthread_cond_destroy(&fut->cv);
    pthread_mutex_destroy(&fut->mu);
    free(fut);
    return res;
}

/* =========================================================================
 * 3. Native Bounded Channels (make_chan / send / recv / close)
 * ========================================================================= */
typedef struct {
    pthread_mutex_t mu;
    pthread_cond_t  not_empty;
    pthread_cond_t  not_full;
    int64_t*        buf;
    int64_t         cap;
    int64_t         head;
    int64_t         tail;
    int64_t         count;
    bool            closed;
} JossChannel;

int64_t joss_make_chan(int64_t capacity) {
    if (capacity <= 0) capacity = 16;
    JossChannel* ch = (JossChannel*)calloc(1, sizeof(JossChannel));
    if (!ch) joss_panic("out of memory allocating channel");
    ch->buf = (int64_t*)calloc((size_t)capacity, sizeof(int64_t));
    if (!ch->buf) joss_panic("out of memory allocating channel buffer");
    ch->cap = capacity;
    pthread_mutex_init(&ch->mu, NULL);
    pthread_cond_init(&ch->not_empty, NULL);
    pthread_cond_init(&ch->not_full, NULL);
    return (int64_t)(uintptr_t)ch;
}

void joss_send(int64_t ch_handle, int64_t val) {
    JossChannel* ch = (JossChannel*)(uintptr_t)ch_handle;
    if (!ch) joss_panic("JOSS-CHANNEL-001: send on null channel");
    pthread_mutex_lock(&ch->mu);
    while (ch->count == ch->cap && !ch->closed) {
        pthread_cond_wait(&ch->not_full, &ch->mu);
    }
    if (ch->closed) {
        pthread_mutex_unlock(&ch->mu);
        joss_panic("JOSS-CHANNEL-001: send on closed channel");
    }
    ch->buf[ch->tail] = val;
    ch->tail = (ch->tail + 1) % ch->cap;
    ch->count++;
    pthread_cond_signal(&ch->not_empty);
    pthread_mutex_unlock(&ch->mu);
}

int64_t joss_recv(int64_t ch_handle) {
    JossChannel* ch = (JossChannel*)(uintptr_t)ch_handle;
    if (!ch) return 0;
    pthread_mutex_lock(&ch->mu);
    while (ch->count == 0 && !ch->closed) {
        pthread_cond_wait(&ch->not_empty, &ch->mu);
    }
    if (ch->count == 0 && ch->closed) {
        pthread_mutex_unlock(&ch->mu);
        return 0;
    }
    int64_t val = ch->buf[ch->head];
    ch->head = (ch->head + 1) % ch->cap;
    ch->count--;
    pthread_cond_signal(&ch->not_full);
    pthread_mutex_unlock(&ch->mu);
    return val;
}

void joss_close(int64_t ch_handle) {
    JossChannel* ch = (JossChannel*)(uintptr_t)ch_handle;
    if (!ch) return;
    pthread_mutex_lock(&ch->mu);
    ch->closed = true;
    pthread_cond_broadcast(&ch->not_empty);
    pthread_cond_broadcast(&ch->not_full);
    pthread_mutex_unlock(&ch->mu);
}

#else

int64_t joss_async(int64_t fn_ptr) {
    JossTaskFn fn = (JossTaskFn)(uintptr_t)fn_ptr;
    return fn ? fn() : 0;
}
int64_t joss_await(int64_t val) { return val; }
int64_t joss_make_chan(int64_t capacity) { (void)capacity; return 0; }
void    joss_send(int64_t ch, int64_t val) { (void)ch; (void)val; }
int64_t joss_recv(int64_t ch) { (void)ch; return 0; }
void    joss_close(int64_t ch) { (void)ch; }

#endif
