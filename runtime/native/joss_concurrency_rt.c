//go:build ignore
#if defined(__APPLE__) && !defined(_DARWIN_C_SOURCE)
#define _DARWIN_C_SOURCE
#elif defined(__unix__)
#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif
#ifndef _POSIX_C_SOURCE
#define _POSIX_C_SOURCE 200809L
#endif
#endif

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
    bool               detached;
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
            JossTaskFn real_fn = (JossTaskFn)(uintptr_t)joss_closure_prepare_call((void*)(uintptr_t)task->fn);
            if (real_fn) {
                res = real_fn();
            }
            joss_closure_env_tls_reset();
        }

        pthread_mutex_lock(&task->mu);
        bool detached = task->detached;
        task->result = res;
        task->done = true;
        if (!detached) {
            pthread_cond_signal(&task->cv);
        }
        pthread_mutex_unlock(&task->mu);
        if (detached) {
            pthread_cond_destroy(&task->cv);
            pthread_mutex_destroy(&task->mu);
            free(task);
        }
    }
    return NULL;
}

static void joss_pool_init(void) {
    joss_gc_set_multithreaded(true);
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

static int64_t joss_async_start(int64_t fn_ptr, bool detached) {
    pthread_once(&g_pool_once, joss_pool_init);
    JossFuture* fut = (JossFuture*)calloc(1, sizeof(JossFuture));
    if (!fut) joss_panic("out of memory allocating async future");
    pthread_mutex_init(&fut->mu, NULL);
    pthread_cond_init(&fut->cv, NULL);
    fut->fn = (JossTaskFn)(uintptr_t)fn_ptr;
    fut->done = false;
    fut->detached = detached;

    pthread_mutex_lock(&g_pool_mu);
    if (g_q_tail) {
        g_q_tail->next = fut;
        g_q_tail = fut;
    } else {
        g_q_head = g_q_tail = fut;
    }
    pthread_cond_signal(&g_pool_cv);
    pthread_mutex_unlock(&g_pool_mu);

    return detached ? 0 : (int64_t)(uintptr_t)fut;
}

int64_t joss_async(int64_t fn_ptr) {
    return joss_async_start(fn_ptr, false);
}

void joss_async_detached(int64_t fn_ptr) {
    (void)joss_async_start(fn_ptr, true);
}

int64_t joss_await(int64_t fut_handle) {
    fut_handle = joss_unbox_value(fut_handle);
    uint64_t u = (uint64_t)fut_handle;
    if (u < JOSS_MIN_PTR_ADDR || (u >> JOSS_PTR_VA_BITS) != 0) return fut_handle;
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
    if (ch->closed && ch->count == 0 && ch->buf) {
        free(ch->buf);
        ch->buf = NULL;
    }
    pthread_cond_signal(&ch->not_full);
    pthread_mutex_unlock(&ch->mu);
    return val;
}

void joss_close(int64_t ch_handle) {
    JossChannel* ch = (JossChannel*)(uintptr_t)ch_handle;
    if (!ch) return;
    pthread_mutex_lock(&ch->mu);
    ch->closed = true;
    if (ch->count == 0 && ch->buf) {
        free(ch->buf);
        ch->buf = NULL;
    }
    pthread_cond_broadcast(&ch->not_empty);
    pthread_cond_broadcast(&ch->not_full);
    pthread_mutex_unlock(&ch->mu);
}

/* =========================================================================
 * 4. Native Mutex, RWMutex, and WaitGroup (POSIX)
 * ========================================================================= */
int64_t fn_Mutex_new(void) {
    pthread_mutex_t* m = (pthread_mutex_t*)calloc(1, sizeof(pthread_mutex_t));
    if (!m) joss_panic("JOSS-OOM: Mutex allocation failed");
    pthread_mutex_init(m, NULL);
    return (int64_t)(uintptr_t)m;
}

void fn_Mutex_lock(int64_t handle) {
    pthread_mutex_t* m = (pthread_mutex_t*)(uintptr_t)handle;
    if (m) pthread_mutex_lock(m);
}

void fn_Mutex_unlock(int64_t handle) {
    pthread_mutex_t* m = (pthread_mutex_t*)(uintptr_t)handle;
    if (m) pthread_mutex_unlock(m);
}

int64_t fn_RWMutex_new(void) {
    pthread_rwlock_t* rw = (pthread_rwlock_t*)calloc(1, sizeof(pthread_rwlock_t));
    if (!rw) joss_panic("JOSS-OOM: RWMutex allocation failed");
    pthread_rwlock_init(rw, NULL);
    return (int64_t)(uintptr_t)rw;
}

void fn_RWMutex_lock(int64_t handle) {
    pthread_rwlock_t* rw = (pthread_rwlock_t*)(uintptr_t)handle;
    if (rw) pthread_rwlock_wrlock(rw);
}

void fn_RWMutex_unlock(int64_t handle) {
    pthread_rwlock_t* rw = (pthread_rwlock_t*)(uintptr_t)handle;
    if (rw) pthread_rwlock_unlock(rw);
}

void fn_RWMutex_rLock(int64_t handle) {
    pthread_rwlock_t* rw = (pthread_rwlock_t*)(uintptr_t)handle;
    if (rw) pthread_rwlock_rdlock(rw);
}

void fn_RWMutex_rUnlock(int64_t handle) {
    pthread_rwlock_t* rw = (pthread_rwlock_t*)(uintptr_t)handle;
    if (rw) pthread_rwlock_unlock(rw);
}

typedef struct {
    pthread_mutex_t mu;
    pthread_cond_t  cv;
    int64_t         count;
} JossWaitGroup;

int64_t fn_WaitGroup_new(void) {
    JossWaitGroup* wg = (JossWaitGroup*)calloc(1, sizeof(JossWaitGroup));
    if (!wg) joss_panic("JOSS-OOM: WaitGroup allocation failed");
    pthread_mutex_init(&wg->mu, NULL);
    pthread_cond_init(&wg->cv, NULL);
    return (int64_t)(uintptr_t)wg;
}

void fn_WaitGroup_add(int64_t handle, int64_t delta) {
    JossWaitGroup* wg = (JossWaitGroup*)(uintptr_t)handle;
    if (!wg) return;
    pthread_mutex_lock(&wg->mu);
    wg->count += delta;
    if (wg->count <= 0) {
        wg->count = 0;
        pthread_cond_broadcast(&wg->cv);
    }
    pthread_mutex_unlock(&wg->mu);
}

void fn_WaitGroup_done(int64_t handle) {
    fn_WaitGroup_add(handle, -1);
}

void fn_WaitGroup_wait(int64_t handle) {
    JossWaitGroup* wg = (JossWaitGroup*)(uintptr_t)handle;
    if (!wg) return;
    pthread_mutex_lock(&wg->mu);
    while (wg->count > 0) {
        pthread_cond_wait(&wg->cv, &wg->mu);
    }
    pthread_mutex_unlock(&wg->mu);
}

#elif defined(_WIN32)

#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <windows.h>

typedef struct JossFuture {
    CRITICAL_SECTION   cs;
    CONDITION_VARIABLE cv;
    bool               done;
    bool               detached;
    int64_t            result;
} JossFuture;

typedef struct JossWorkItem {
    JossTaskFn           fn;
    JossFuture*          fut;
    struct JossWorkItem* next;
} JossWorkItem;

static CRITICAL_SECTION   g_win_pool_cs;
static CONDITION_VARIABLE g_win_pool_cv;
static JossWorkItem*      g_win_q_head = NULL;
static JossWorkItem*      g_win_q_tail = NULL;
static LONG               g_win_pool_init_state = 0;

static DWORD WINAPI joss_win_worker_loop(LPVOID arg) {
    (void)arg;
    for (;;) {
        EnterCriticalSection(&g_win_pool_cs);
        while (!g_win_q_head) {
            SleepConditionVariableCS(&g_win_pool_cv, &g_win_pool_cs, INFINITE);
        }
        JossWorkItem* item = g_win_q_head;
        g_win_q_head = item->next;
        if (!g_win_q_head) g_win_q_tail = NULL;
        LeaveCriticalSection(&g_win_pool_cs);

        int64_t res = 0;
        if (item->fn) {
            JossTaskFn real_fn = (JossTaskFn)(uintptr_t)joss_closure_prepare_call((void*)(uintptr_t)item->fn);
            if (real_fn) {
                res = real_fn();
            }
            joss_closure_env_tls_reset();
        }

        JossFuture* fut = item->fut;
        EnterCriticalSection(&fut->cs);
        bool detached = fut->detached;
        fut->result = res;
        fut->done = true;
        if (!detached) {
            WakeAllConditionVariable(&fut->cv);
        }
        LeaveCriticalSection(&fut->cs);
        if (detached) {
            DeleteCriticalSection(&fut->cs);
            free(fut);
        }
        free(item);
    }
    return 0;
}

static void joss_win_pool_ensure(void) {
    if (InterlockedCompareExchange(&g_win_pool_init_state, 1, 0) == 0) {
        InitializeCriticalSection(&g_win_pool_cs);
        joss_gc_set_multithreaded(true);
        InitializeConditionVariable(&g_win_pool_cv);
        SYSTEM_INFO si;
        GetSystemInfo(&si);
        DWORD nworkers = si.dwNumberOfProcessors;
        if (nworkers < 2) nworkers = 2;
        if (nworkers > 16) nworkers = 16;
        for (DWORD i = 0; i < nworkers; i++) {
            HANDLE h = CreateThread(NULL, 0, joss_win_worker_loop, NULL, 0, NULL);
            if (h) CloseHandle(h);
        }
        InterlockedExchange(&g_win_pool_init_state, 2);
    } else {
        while (InterlockedCompareExchange(&g_win_pool_init_state, 2, 2) != 2) {
            Sleep(0);
        }
    }
}

static int64_t joss_win_async_start(int64_t fn_ptr, bool detached) {
    joss_win_pool_ensure();
    JossFuture* fut = (JossFuture*)calloc(1, sizeof(JossFuture));
    if (!fut) joss_panic("JOSS-OOM: failed to allocate Future");
    InitializeCriticalSection(&fut->cs);
    InitializeConditionVariable(&fut->cv);
    fut->detached = detached;

    JossWorkItem* item = (JossWorkItem*)malloc(sizeof(JossWorkItem));
    if (!item) joss_panic("JOSS-OOM: failed to allocate WorkItem");
    item->fn = (JossTaskFn)(uintptr_t)fn_ptr;
    item->fut = fut;
    item->next = NULL;

    EnterCriticalSection(&g_win_pool_cs);
    if (g_win_q_tail) {
        g_win_q_tail->next = item;
        g_win_q_tail = item;
    } else {
        g_win_q_head = g_win_q_tail = item;
    }
    WakeConditionVariable(&g_win_pool_cv);
    LeaveCriticalSection(&g_win_pool_cs);
    return detached ? 0 : (int64_t)(uintptr_t)fut;
}

int64_t joss_async(int64_t fn_ptr) {
    return joss_win_async_start(fn_ptr, false);
}

void joss_async_detached(int64_t fn_ptr) {
    (void)joss_win_async_start(fn_ptr, true);
}

int64_t joss_await(int64_t fut_handle) {
    fut_handle = joss_unbox_value(fut_handle);
    uint64_t u = (uint64_t)fut_handle;
    if (u < JOSS_MIN_PTR_ADDR || (u >> JOSS_PTR_VA_BITS) != 0) return fut_handle;
    JossFuture* fut = (JossFuture*)(uintptr_t)u;
    EnterCriticalSection(&fut->cs);
    while (!fut->done) {
        SleepConditionVariableCS(&fut->cv, &fut->cs, INFINITE);
    }
    int64_t res = fut->result;
    LeaveCriticalSection(&fut->cs);
    DeleteCriticalSection(&fut->cs);
    free(fut);
    return res;
}

typedef struct JossWinChannel {
    CRITICAL_SECTION   cs;
    CONDITION_VARIABLE not_empty;
    CONDITION_VARIABLE not_full;
    int64_t*           buf;
    int64_t            cap;
    int64_t            head;
    int64_t            tail;
    int64_t            count;
    bool               closed;
} JossWinChannel;

int64_t joss_make_chan(int64_t capacity) {
    if (capacity < 0) joss_panic("JOSS-CHANNEL-002: invalid negative channel capacity");
    if (capacity == 0) capacity = 1;
    JossWinChannel* ch = (JossWinChannel*)calloc(1, sizeof(JossWinChannel));
    if (!ch) joss_panic("JOSS-OOM: channel allocation failed");
    ch->buf = (int64_t*)calloc((size_t)capacity, sizeof(int64_t));
    ch->cap = capacity;
    InitializeCriticalSection(&ch->cs);
    InitializeConditionVariable(&ch->not_empty);
    InitializeConditionVariable(&ch->not_full);
    return (int64_t)(uintptr_t)ch;
}

void joss_send(int64_t ch_handle, int64_t val) {
    JossWinChannel* ch = (JossWinChannel*)(uintptr_t)ch_handle;
    if (!ch) return;
    EnterCriticalSection(&ch->cs);
    while (ch->count == ch->cap && !ch->closed) {
        SleepConditionVariableCS(&ch->not_full, &ch->cs, INFINITE);
    }
    if (ch->closed) {
        LeaveCriticalSection(&ch->cs);
        joss_panic("JOSS-CHANNEL-001: send on closed channel");
    }
    ch->buf[ch->tail] = val;
    ch->tail = (ch->tail + 1) % ch->cap;
    ch->count++;
    WakeConditionVariable(&ch->not_empty);
    LeaveCriticalSection(&ch->cs);
}

int64_t joss_recv(int64_t ch_handle) {
    JossWinChannel* ch = (JossWinChannel*)(uintptr_t)ch_handle;
    if (!ch) return 0;
    EnterCriticalSection(&ch->cs);
    while (ch->count == 0 && !ch->closed) {
        SleepConditionVariableCS(&ch->not_empty, &ch->cs, INFINITE);
    }
    if (ch->count == 0 && ch->closed) {
        LeaveCriticalSection(&ch->cs);
        return 0;
    }
    int64_t val = ch->buf[ch->head];
    ch->head = (ch->head + 1) % ch->cap;
    ch->count--;
    if (ch->closed && ch->count == 0 && ch->buf) {
        free(ch->buf);
        ch->buf = NULL;
    }
    WakeConditionVariable(&ch->not_full);
    LeaveCriticalSection(&ch->cs);
    return val;
}

void joss_close(int64_t ch_handle) {
    JossWinChannel* ch = (JossWinChannel*)(uintptr_t)ch_handle;
    if (!ch) return;
    EnterCriticalSection(&ch->cs);
    ch->closed = true;
    if (ch->count == 0 && ch->buf) {
        free(ch->buf);
        ch->buf = NULL;
    }
    WakeAllConditionVariable(&ch->not_empty);
    WakeAllConditionVariable(&ch->not_full);
    LeaveCriticalSection(&ch->cs);
}

int64_t fn_Mutex_new(void) {
    CRITICAL_SECTION* cs = (CRITICAL_SECTION*)calloc(1, sizeof(CRITICAL_SECTION));
    if (!cs) joss_panic("JOSS-OOM: Mutex allocation failed");
    InitializeCriticalSection(cs);
    return (int64_t)(uintptr_t)cs;
}
void fn_Mutex_lock(int64_t handle) {
    CRITICAL_SECTION* cs = (CRITICAL_SECTION*)(uintptr_t)handle;
    if (cs) EnterCriticalSection(cs);
}
void fn_Mutex_unlock(int64_t handle) {
    CRITICAL_SECTION* cs = (CRITICAL_SECTION*)(uintptr_t)handle;
    if (cs) LeaveCriticalSection(cs);
}

int64_t fn_RWMutex_new(void) {
    SRWLOCK* rw = (SRWLOCK*)calloc(1, sizeof(SRWLOCK));
    if (!rw) joss_panic("JOSS-OOM: RWMutex allocation failed");
    InitializeSRWLock(rw);
    return (int64_t)(uintptr_t)rw;
}
void fn_RWMutex_lock(int64_t handle) {
    SRWLOCK* rw = (SRWLOCK*)(uintptr_t)handle;
    if (rw) AcquireSRWLockExclusive(rw);
}
void fn_RWMutex_unlock(int64_t handle) {
    SRWLOCK* rw = (SRWLOCK*)(uintptr_t)handle;
    if (rw) ReleaseSRWLockExclusive(rw);
}
void fn_RWMutex_rLock(int64_t handle) {
    SRWLOCK* rw = (SRWLOCK*)(uintptr_t)handle;
    if (rw) AcquireSRWLockShared(rw);
}
void fn_RWMutex_rUnlock(int64_t handle) {
    SRWLOCK* rw = (SRWLOCK*)(uintptr_t)handle;
    if (rw) ReleaseSRWLockShared(rw);
}

typedef struct {
    CRITICAL_SECTION   cs;
    CONDITION_VARIABLE cv;
    int64_t            count;
} JossWinWaitGroup;

int64_t fn_WaitGroup_new(void) {
    JossWinWaitGroup* wg = (JossWinWaitGroup*)calloc(1, sizeof(JossWinWaitGroup));
    if (!wg) joss_panic("JOSS-OOM: WaitGroup allocation failed");
    InitializeCriticalSection(&wg->cs);
    InitializeConditionVariable(&wg->cv);
    return (int64_t)(uintptr_t)wg;
}
void fn_WaitGroup_add(int64_t handle, int64_t delta) {
    JossWinWaitGroup* wg = (JossWinWaitGroup*)(uintptr_t)handle;
    if (!wg) return;
    EnterCriticalSection(&wg->cs);
    wg->count += delta;
    if (wg->count <= 0) {
        wg->count = 0;
        WakeAllConditionVariable(&wg->cv);
    }
    LeaveCriticalSection(&wg->cs);
}
void fn_WaitGroup_done(int64_t handle) { fn_WaitGroup_add(handle, -1); }
void fn_WaitGroup_wait(int64_t handle) {
    JossWinWaitGroup* wg = (JossWinWaitGroup*)(uintptr_t)handle;
    if (!wg) return;
    EnterCriticalSection(&wg->cs);
    while (wg->count > 0) {
        SleepConditionVariableCS(&wg->cv, &wg->cs, INFINITE);
    }
    LeaveCriticalSection(&wg->cs);
}

#else

int64_t joss_async(int64_t fn_ptr) {
    JossTaskFn fn = (JossTaskFn)(uintptr_t)joss_closure_prepare_call((void*)(uintptr_t)fn_ptr);
    int64_t res = fn ? fn() : 0;
    joss_closure_env_tls_reset();
    return res;
}
void    joss_async_detached(int64_t fn_ptr) { (void)joss_async(fn_ptr); }
int64_t joss_await(int64_t val) { return val; }
int64_t joss_make_chan(int64_t capacity) { (void)capacity; return 0; }
void    joss_send(int64_t ch, int64_t val) { (void)ch; (void)val; }
int64_t joss_recv(int64_t ch) { (void)ch; return 0; }
void    joss_close(int64_t ch) { (void)ch; }
int64_t fn_Mutex_new(void) { return 0; }
void    fn_Mutex_lock(int64_t h) { (void)h; }
void    fn_Mutex_unlock(int64_t h) { (void)h; }
int64_t fn_RWMutex_new(void) { return 0; }
void    fn_RWMutex_lock(int64_t h) { (void)h; }
void    fn_RWMutex_unlock(int64_t h) { (void)h; }
void    fn_RWMutex_rLock(int64_t h) { (void)h; }
void    fn_RWMutex_rUnlock(int64_t h) { (void)h; }
int64_t fn_WaitGroup_new(void) { return 0; }
void    fn_WaitGroup_add(int64_t h, int64_t d) { (void)h; (void)d; }
void    fn_WaitGroup_done(int64_t h) { (void)h; }
void    fn_WaitGroup_wait(int64_t h) { (void)h; }

#endif
