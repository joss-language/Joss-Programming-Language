//! Bounded-memory, TTL/LRU thread-safe Session & Cache store in pure Rust.
//!
//! Eliminates manual `JossOwnedScope` / `malloc` string leaks: every overwritten,
//! expired, or evicted entry is deterministically freed by Rust's `Drop`.

use std::collections::HashMap;
use std::ffi::{c_char, CStr};
use std::fs;
use std::path::Path;
use std::sync::{Mutex, OnceLock};
use std::time::{SystemTime, UNIX_EPOCH};

const DEFAULT_MAX_ENTRIES: usize = 2048;

struct StoreEntry {
    value: String,
    touched_at: i64,
    expires_at: i64, // 0 = no expiry
}

pub struct BoundedStore {
    entries: HashMap<String, StoreEntry>,
    max_entries: usize,
}

impl BoundedStore {
    pub fn new(max_entries: usize) -> Self {
        Self {
            entries: HashMap::new(),
            max_entries: max_entries.max(64),
        }
    }

    fn now_secs() -> i64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_secs() as i64)
            .unwrap_or(0)
    }

    pub fn put(&mut self, key: String, value: String, ttl_secs: i64) {
        let now = Self::now_secs();
        if self.entries.len() >= self.max_entries && !self.entries.contains_key(&key) {
            // 1. Drop all expired entries first.
            self.entries
                .retain(|_, e| e.expires_at == 0 || e.expires_at >= now);
            // 2. If still at capacity, evict oldest 25% by LRU `touched_at`.
            if self.entries.len() >= self.max_entries {
                let target_evict = (self.max_entries / 4).max(16);
                let mut by_age: Vec<(i64, String)> = self
                    .entries
                    .iter()
                    .map(|(k, v)| (v.touched_at, k.clone()))
                    .collect();
                by_age.sort_unstable_by_key(|(ts, _)| *ts);
                for (_, old_k) in by_age.into_iter().take(target_evict) {
                    self.entries.remove(&old_k);
                }
            }
        }
        let expires_at = if ttl_secs > 0 { now + ttl_secs } else { 0 };
        self.entries.insert(
            key,
            StoreEntry {
                value,
                touched_at: now,
                expires_at,
            },
        );
    }

    pub fn get(&mut self, key: &str) -> Option<&str> {
        let now = Self::now_secs();
        let expired = self
            .entries
            .get(key)
            .is_some_and(|e| e.expires_at > 0 && now > e.expires_at);
        if expired {
            self.entries.remove(key);
            return None;
        }
        let entry = self.entries.get_mut(key)?;
        entry.touched_at = now;
        Some(entry.value.as_str())
    }

    pub fn remove(&mut self, key: &str) -> bool {
        self.entries.remove(key).is_some()
    }

    pub fn len(&self) -> usize {
        self.entries.len()
    }

    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }
}

/// Atomically persists UTF-8 payload to `path` via temp file + rename (with Windows retries).
pub fn atomic_write_file(path: &Path, content: &[u8]) -> std::io::Result<()> {
    if let Some(parent) = path.parent() {
        if !parent.as_os_str().is_empty() {
            fs::create_dir_all(parent)?;
        }
    }
    let tmp = path.with_extension(format!("tmp-{}", std::process::id()));
    fs::write(&tmp, content)?;
    let mut last_err = None;
    for _ in 0..5 {
        match fs::rename(&tmp, path) {
            Ok(()) => return Ok(()),
            Err(e) => {
                last_err = Some(e);
                std::thread::sleep(std::time::Duration::from_millis(5));
            }
        }
    }
    let _ = fs::remove_file(&tmp);
    Err(last_err.unwrap_or_else(|| std::io::Error::other("atomic rename failed")))
}

static GLOBAL_KV: OnceLock<Mutex<BoundedStore>> = OnceLock::new();

fn kv() -> &'static Mutex<BoundedStore> {
    GLOBAL_KV.get_or_init(|| {
        let max_entries = std::env::var("SESSION_MAX_ENTRIES")
            .ok()
            .and_then(|s| s.parse::<usize>().ok())
            .unwrap_or(DEFAULT_MAX_ENTRIES);
        Mutex::new(BoundedStore::new(max_entries))
    })
}

/// # Safety
/// `key` and `val` must be valid NUL-terminated C strings.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_kv_put(
    key: *const c_char,
    val: *const c_char,
    ttl_secs: i64,
) -> i64 {
    if key.is_null() || val.is_null() {
        return 0;
    }
    let (Ok(k), Ok(v)) = (CStr::from_ptr(key).to_str(), CStr::from_ptr(val).to_str()) else {
        return 0;
    };
    let mut guard = kv().lock().unwrap_or_else(|e| e.into_inner());
    guard.put(k.to_owned(), v.to_owned(), ttl_secs);
    1
}

/// # Safety
/// `key` must be a valid NUL-terminated C string; `out_buf` must hold `out_cap` bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_kv_get(
    key: *const c_char,
    out_buf: *mut c_char,
    out_cap: usize,
) -> isize {
    if key.is_null() || out_buf.is_null() || out_cap == 0 {
        return -1;
    }
    let Ok(k) = CStr::from_ptr(key).to_str() else {
        return -1;
    };
    let mut guard = kv().lock().unwrap_or_else(|e| e.into_inner());
    let Some(val) = guard.get(k) else {
        return -1;
    };
    let bytes = val.as_bytes();
    if bytes.len() + 1 > out_cap {
        return -1;
    }
    std::ptr::copy_nonoverlapping(bytes.as_ptr(), out_buf as *mut u8, bytes.len());
    *out_buf.add(bytes.len()) = 0;
    bytes.len() as isize
}

/// # Safety
/// `key` must be a valid NUL-terminated C string.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_kv_del(key: *const c_char) -> i64 {
    if key.is_null() {
        return 0;
    }
    let Ok(k) = CStr::from_ptr(key).to_str() else {
        return 0;
    };
    let mut guard = kv().lock().unwrap_or_else(|e| e.into_inner());
    if guard.remove(k) { 1 } else { 0 }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bounded_store_enforces_lru_cap() {
        let mut store = BoundedStore::new(64);
        for i in 0..200 {
            store.put(format!("sid_{i}"), format!("{{\"u\":{i}}}"), 3600);
        }
        assert!(store.len() <= 64, "expected bounded len <= 64, got {}", store.len());
        assert!(store.get("sid_199").is_some());
    }

    #[test]
    fn atomic_write_persists_cleanly() {
        let dir = std::env::temp_dir().join(format!("joss-rs-sess-{}", std::process::id()));
        let file = dir.join("sessions.json");
        atomic_write_file(&file, br#"{"ok":true}"#).unwrap();
        assert_eq!(fs::read_to_string(&file).unwrap(), r#"{"ok":true}"#);
        let _ = fs::remove_dir_all(&dir);
    }
}
