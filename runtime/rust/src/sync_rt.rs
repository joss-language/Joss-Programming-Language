//! Thread-safe WaitGroup primitive in pure Rust (`std::sync::{Mutex, Condvar}`).
//!
//! Zero platform `#ifdef`s and compile-time data-race prevention.

use std::sync::{Condvar, Mutex};

pub struct RustWaitGroup {
    state: Mutex<i64>,
    cvar: Condvar,
}

impl RustWaitGroup {
    pub fn new() -> Self {
        Self {
            state: Mutex::new(0),
            cvar: Condvar::new(),
        }
    }

    pub fn add(&self, delta: i64) {
        let mut guard = self.state.lock().unwrap_or_else(|e| e.into_inner());
        *guard += delta;
        if *guard <= 0 {
            *guard = 0;
            self.cvar.notify_all();
        }
    }

    pub fn wait(&self) {
        let mut guard = self.state.lock().unwrap_or_else(|e| e.into_inner());
        while *guard > 0 {
            guard = self.cvar.wait(guard).unwrap_or_else(|e| e.into_inner());
        }
    }
}

impl Default for RustWaitGroup {
    fn default() -> Self {
        Self::new()
    }
}

#[no_mangle]
pub extern "C" fn joss_rs_wg_new() -> *mut RustWaitGroup {
    Box::into_raw(Box::new(RustWaitGroup::new()))
}

/// # Safety
/// `wg` must be a valid pointer returned by `joss_rs_wg_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_wg_add(wg: *mut RustWaitGroup, delta: i64) {
    if let Some(w) = wg.as_ref() {
        w.add(delta);
    }
}

/// # Safety
/// `wg` must be a valid pointer returned by `joss_rs_wg_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_wg_wait(wg: *mut RustWaitGroup) {
    if let Some(w) = wg.as_ref() {
        w.wait();
    }
}

/// # Safety
/// `wg` must be a valid pointer returned by `joss_rs_wg_new` and not freed previously.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_wg_free(wg: *mut RustWaitGroup) {
    if !wg.is_null() {
        drop(Box::from_raw(wg));
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;

    #[test]
    fn waitgroup_synchronizes_threads() {
        let wg = Arc::new(RustWaitGroup::new());
        wg.add(2);
        let w1 = Arc::clone(&wg);
        let w2 = Arc::clone(&wg);
        let t1 = std::thread::spawn(move || w1.add(-1));
        let t2 = std::thread::spawn(move || w2.add(-1));
        wg.wait();
        t1.join().unwrap();
        t2.join().unwrap();
    }
}
