//! Cross-platform concurrency runtime in pure Rust:
//! - Lock-free thread-local Auto-DP memoization (`joss_rs_dp_get`, `joss_rs_dp_put`)
//! - Bounded blocking channels (`joss_rs_make_chan`, `joss_rs_send`, `joss_rs_recv`, `joss_rs_close`)
//! - Multi-core async task pool (`joss_rs_async_spawn`, `joss_rs_await`)
//! - Native `Mutex` and `RWMutex` (`joss_rs_mutex_*`, `joss_rs_rwmutex_*`)
//!
//! Replaces POSIX/Windows `#ifdef` concurrency boilerplate in `joss_concurrency_rt.c`.

use std::cell::RefCell;
use std::collections::VecDeque;
use std::sync::{Arc, Condvar, Mutex, OnceLock};

const DP_SLOTS: usize = 8192;
const DP_MASK: usize = DP_SLOTS - 1;

#[derive(Clone, Copy, Default)]
struct DpEntry {
    key_hash: u64,
    fn_id: i64,
    a0: i64,
    a1: i64,
    val: i64,
    occupied: bool,
}

thread_local! {
    static TLS_DP: RefCell<Option<Box<[DpEntry]>>> = const { RefCell::new(None) };
}

#[inline]
fn dp_hash(fn_id: i64, a0: i64, a1: i64) -> u64 {
    let mut h = (fn_id as u64).wrapping_mul(0x9E37_79B9_7F4A_7C15);
    h ^= (a0 as u64).wrapping_mul(0xBF58_476D_1CE4_E5B9);
    h ^= h >> 30;
    h ^= (a1 as u64).wrapping_mul(0x94D0_49BB_1331_11EB);
    h ^= h >> 27;
    if h == 0 { 1 } else { h }
}

/// # Safety
/// `out_val` must either be null or point to a valid writable `i64`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_dp_get(
    fn_id: i64,
    a0: i64,
    a1: i64,
    out_val: *mut i64,
) -> bool {
    let h = dp_hash(fn_id, a0, a1);
    let idx = (h as usize) & DP_MASK;
    TLS_DP.with(|cell| {
        let borrow = cell.borrow();
        let Some(table) = borrow.as_ref() else {
            return false;
        };
        let e = &table[idx];
        if e.occupied && e.key_hash == h && e.fn_id == fn_id && e.a0 == a0 && e.a1 == a1 {
            if !out_val.is_null() {
                *out_val = e.val;
            }
            return true;
        }
        false
    })
}

#[no_mangle]
pub extern "C" fn joss_rs_dp_put(fn_id: i64, a0: i64, a1: i64, val: i64) {
    let h = dp_hash(fn_id, a0, a1);
    let idx = (h as usize) & DP_MASK;
    TLS_DP.with(|cell| {
        let mut borrow = cell.borrow_mut();
        let table = borrow.get_or_insert_with(|| vec![DpEntry::default(); DP_SLOTS].into_boxed_slice());
        table[idx] = DpEntry {
            key_hash: h,
            fn_id,
            a0,
            a1,
            val,
            occupied: true,
        };
    });
}

/* =========================================================================
 * Bounded Blocking Channel
 * ========================================================================= */

struct ChanInner {
    buf: VecDeque<i64>,
    cap: usize,
    closed: bool,
}

pub struct RustChannel {
    state: Mutex<ChanInner>,
    not_empty: Condvar,
    not_full: Condvar,
}

impl RustChannel {
    pub fn new(capacity: usize) -> Self {
        let cap = if capacity == 0 { 16 } else { capacity };
        Self {
            state: Mutex::new(ChanInner {
                buf: VecDeque::with_capacity(cap.min(1024)),
                cap,
                closed: false,
            }),
            not_empty: Condvar::new(),
            not_full: Condvar::new(),
        }
    }

    /// Returns `true` on success, `false` if the channel is closed.
    pub fn send(&self, val: i64) -> bool {
        let mut guard = self.state.lock().unwrap_or_else(|e| e.into_inner());
        while guard.buf.len() >= guard.cap && !guard.closed {
            guard = self.not_full.wait(guard).unwrap_or_else(|e| e.into_inner());
        }
        if guard.closed {
            return false;
        }
        guard.buf.push_back(val);
        self.not_empty.notify_one();
        true
    }

    pub fn recv(&self) -> i64 {
        let mut guard = self.state.lock().unwrap_or_else(|e| e.into_inner());
        while guard.buf.is_empty() && !guard.closed {
            guard = self.not_empty.wait(guard).unwrap_or_else(|e| e.into_inner());
        }
        let val = guard.buf.pop_front().unwrap_or(0);
        if guard.closed && guard.buf.is_empty() {
            guard.buf.shrink_to_fit();
        }
        self.not_full.notify_one();
        val
    }

    pub fn close(&self) {
        let mut guard = self.state.lock().unwrap_or_else(|e| e.into_inner());
        guard.closed = true;
        if guard.buf.is_empty() {
            guard.buf.shrink_to_fit();
        }
        self.not_empty.notify_all();
        self.not_full.notify_all();
    }
}

#[no_mangle]
pub extern "C" fn joss_rs_make_chan(capacity: i64) -> i64 {
    let cap = if capacity > 0 { capacity as usize } else { 16 };
    Box::into_raw(Box::new(RustChannel::new(cap))) as usize as i64
}

/// Returns 1 on success, 0 if channel is null or closed.
/// # Safety
/// `ch_handle` must be a valid pointer returned by `joss_rs_make_chan`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_send(ch_handle: i64, val: i64) -> i64 {
    let ptr = ch_handle as usize as *const RustChannel;
    match ptr.as_ref() {
        Some(ch) if ch.send(val) => 1,
        _ => 0,
    }
}

/// # Safety
/// `ch_handle` must be a valid pointer returned by `joss_rs_make_chan`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_recv(ch_handle: i64) -> i64 {
    let ptr = ch_handle as usize as *const RustChannel;
    match ptr.as_ref() {
        Some(ch) => ch.recv(),
        None => 0,
    }
}

/// # Safety
/// `ch_handle` must be a valid pointer returned by `joss_rs_make_chan`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_close(ch_handle: i64) {
    let ptr = ch_handle as usize as *const RustChannel;
    if let Some(ch) = ptr.as_ref() {
        ch.close();
    }
}

/* =========================================================================
 * Native Mutex & RWMutex (Reentrant-safe handle wrappers)
 * ========================================================================= */

struct RawMutex {
    state: Mutex<bool>,
    cv: Condvar,
}

impl RawMutex {
    fn new() -> Self {
        Self {
            state: Mutex::new(false),
            cv: Condvar::new(),
        }
    }
    fn lock(&self) {
        let mut locked = self.state.lock().unwrap_or_else(|e| e.into_inner());
        while *locked {
            locked = self.cv.wait(locked).unwrap_or_else(|e| e.into_inner());
        }
        *locked = true;
    }
    fn unlock(&self) {
        let mut locked = self.state.lock().unwrap_or_else(|e| e.into_inner());
        *locked = false;
        self.cv.notify_one();
    }
}

struct RawRWMutexState {
    readers: usize,
    writer: bool,
}

struct RawRWMutex {
    state: Mutex<RawRWMutexState>,
    cv: Condvar,
}

impl RawRWMutex {
    fn new() -> Self {
        Self {
            state: Mutex::new(RawRWMutexState {
                readers: 0,
                writer: false,
            }),
            cv: Condvar::new(),
        }
    }
    fn lock(&self) {
        let mut s = self.state.lock().unwrap_or_else(|e| e.into_inner());
        while s.writer || s.readers > 0 {
            s = self.cv.wait(s).unwrap_or_else(|e| e.into_inner());
        }
        s.writer = true;
    }
    fn unlock(&self) {
        let mut s = self.state.lock().unwrap_or_else(|e| e.into_inner());
        s.writer = false;
        self.cv.notify_all();
    }
    fn rlock(&self) {
        let mut s = self.state.lock().unwrap_or_else(|e| e.into_inner());
        while s.writer {
            s = self.cv.wait(s).unwrap_or_else(|e| e.into_inner());
        }
        s.readers += 1;
    }
    fn runlock(&self) {
        let mut s = self.state.lock().unwrap_or_else(|e| e.into_inner());
        if s.readers > 0 {
            s.readers -= 1;
        }
        if s.readers == 0 {
            self.cv.notify_all();
        }
    }
}

#[no_mangle]
pub extern "C" fn joss_rs_mutex_new() -> i64 {
    Box::into_raw(Box::new(RawMutex::new())) as usize as i64
}

/// # Safety
/// `handle` must be a valid pointer returned by `joss_rs_mutex_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_mutex_lock(handle: i64) {
    if let Some(m) = (handle as usize as *const RawMutex).as_ref() {
        m.lock();
    }
}

/// # Safety
/// `handle` must be a valid pointer returned by `joss_rs_mutex_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_mutex_unlock(handle: i64) {
    if let Some(m) = (handle as usize as *const RawMutex).as_ref() {
        m.unlock();
    }
}

#[no_mangle]
pub extern "C" fn joss_rs_rwmutex_new() -> i64 {
    Box::into_raw(Box::new(RawRWMutex::new())) as usize as i64
}

/// # Safety
/// `handle` must be a valid pointer returned by `joss_rs_rwmutex_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_rwmutex_lock(handle: i64) {
    if let Some(rw) = (handle as usize as *const RawRWMutex).as_ref() {
        rw.lock();
    }
}

/// # Safety
/// `handle` must be a valid pointer returned by `joss_rs_rwmutex_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_rwmutex_unlock(handle: i64) {
    if let Some(rw) = (handle as usize as *const RawRWMutex).as_ref() {
        rw.unlock();
    }
}

/// # Safety
/// `handle` must be a valid pointer returned by `joss_rs_rwmutex_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_rwmutex_rlock(handle: i64) {
    if let Some(rw) = (handle as usize as *const RawRWMutex).as_ref() {
        rw.rlock();
    }
}

/// # Safety
/// `handle` must be a valid pointer returned by `joss_rs_rwmutex_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_rwmutex_runlock(handle: i64) {
    if let Some(rw) = (handle as usize as *const RawRWMutex).as_ref() {
        rw.runlock();
    }
}

/* =========================================================================
 * Multi-Core Async Task Worker Pool
 * ========================================================================= */

type TaskRunnerFn = unsafe extern "C" fn(i64) -> i64;

struct FutureState {
    runner: TaskRunnerFn,
    fn_handle: i64,
    detached: bool,
    done: Mutex<(bool, i64)>,
    cv: Condvar,
}

struct TaskPool {
    queue: Mutex<VecDeque<Arc<FutureState>>>,
    cv: Condvar,
}

static TASK_POOL: OnceLock<Arc<TaskPool>> = OnceLock::new();

fn pool() -> &'static Arc<TaskPool> {
    TASK_POOL.get_or_init(|| {
        let p = Arc::new(TaskPool {
            queue: Mutex::new(VecDeque::new()),
            cv: Condvar::new(),
        });
        let ncpu = std::thread::available_parallelism()
            .map(|n| n.get())
            .unwrap_or(4)
            .clamp(2, 32);
        for _ in 0..ncpu {
            let wp = Arc::clone(&p);
            let _ = std::thread::Builder::new()
                .name("joss-async-worker".to_owned())
                .spawn(move || loop {
                    let task = {
                        let mut q = wp.queue.lock().unwrap_or_else(|e| e.into_inner());
                        while q.is_empty() {
                            q = wp.cv.wait(q).unwrap_or_else(|e| e.into_inner());
                        }
                        q.pop_front()
                    };
                    if let Some(fut) = task {
                        let res = unsafe { (fut.runner)(fut.fn_handle) };
                        if !fut.detached {
                            let mut st = fut.done.lock().unwrap_or_else(|e| e.into_inner());
                            *st = (true, res);
                            fut.cv.notify_one();
                        }
                    }
                });
        }
        p
    })
}

/// Spawns a task on the Rust worker pool using the caller-provided runner trampoline.
///
/// # Safety
/// `runner` must be a valid C function pointer safe to invoke on a worker thread.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_async_spawn(
    runner: TaskRunnerFn,
    fn_handle: i64,
    detached: bool,
) -> i64 {
    let fut = Arc::new(FutureState {
        runner,
        fn_handle,
        detached,
        done: Mutex::new((false, 0)),
        cv: Condvar::new(),
    });
    let ret = if detached {
        0
    } else {
        Arc::into_raw(Arc::clone(&fut)) as usize as i64
    };
    let p = pool();
    {
        let mut q = p.queue.lock().unwrap_or_else(|e| e.into_inner());
        q.push_back(fut);
    }
    p.cv.notify_one();
    ret
}

/// Blocks until the future completes, frees it, and returns its result.
///
/// # Safety
/// `fut_handle` must be a valid pointer returned by `joss_rs_async_spawn(..., false)`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_async_await(fut_handle: i64) -> i64 {
    if fut_handle == 0 {
        return 0;
    }
    let fut = Arc::from_raw(fut_handle as usize as *const FutureState);
    let mut st = fut.done.lock().unwrap_or_else(|e| e.into_inner());
    while !st.0 {
        st = fut.cv.wait(st).unwrap_or_else(|e| e.into_inner());
    }
    st.1
}

#[cfg(test)]
mod tests {
    use super::*;

    unsafe extern "C" fn sample_task(x: i64) -> i64 {
        x * 3 + 1
    }

    #[test]
    fn auto_dp_caches_and_returns_values() {
        let mut out = 0i64;
        assert!(!unsafe { joss_rs_dp_get(1, 10, 20, &mut out) });
        joss_rs_dp_put(1, 10, 20, 42);
        assert!(unsafe { joss_rs_dp_get(1, 10, 20, &mut out) });
        assert_eq!(out, 42);
    }

    #[test]
    fn channel_send_recv_close_works() {
        let ch = joss_rs_make_chan(4);
        assert_eq!(unsafe { joss_rs_send(ch, 100) }, 1);
        assert_eq!(unsafe { joss_rs_send(ch, 200) }, 1);
        assert_eq!(unsafe { joss_rs_recv(ch) }, 100);
        assert_eq!(unsafe { joss_rs_recv(ch) }, 200);
        unsafe { joss_rs_close(ch) };
        assert_eq!(unsafe { joss_rs_send(ch, 300) }, 0);
        assert_eq!(unsafe { joss_rs_recv(ch) }, 0);
    }

    #[test]
    fn async_pool_executes_and_awaits() {
        let fut = unsafe { joss_rs_async_spawn(sample_task, 14, false) };
        assert_ne!(fut, 0);
        let res = unsafe { joss_rs_async_await(fut) };
        assert_eq!(res, 43);
    }

    #[test]
    fn mutex_and_rwmutex_lock_unlock() {
        let m = joss_rs_mutex_new();
        unsafe {
            joss_rs_mutex_lock(m);
            joss_rs_mutex_unlock(m);
        }
        let rw = joss_rs_rwmutex_new();
        unsafe {
            joss_rs_rwmutex_rlock(rw);
            joss_rs_rwmutex_runlock(rw);
            joss_rs_rwmutex_lock(rw);
            joss_rs_rwmutex_unlock(rw);
        }
    }
}
