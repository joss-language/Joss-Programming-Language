//! Thread-safe, bounded-memory HTTP Rate Limiter in pure Rust.
//!
//! Replaces `g_rate_entries` in `runtime/native/joss_session_rt.inc` with
//! automatic stale-bucket eviction (max 4096 IPs) and zero fixed-buffer overflows.

use std::collections::HashMap;
use std::ffi::{c_char, CStr};
use std::sync::{Mutex, OnceLock};
use std::time::{SystemTime, UNIX_EPOCH};

const MAX_RATE_BUCKETS: usize = 4096;

struct RateBucket {
    window_start: i64,
    count: i64,
}

static BUCKETS: OnceLock<Mutex<HashMap<String, RateBucket>>> = OnceLock::new();

fn buckets() -> &'static Mutex<HashMap<String, RateBucket>> {
    BUCKETS.get_or_init(|| Mutex::new(HashMap::new()))
}

fn now_secs() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs() as i64)
        .unwrap_or(0)
}

pub fn check_rate_limit(ip: &str, max_req: i64, window_sec: i64) -> i64 {
    let key = if ip.is_empty() { "127.0.0.1" } else { ip };
    let limit = if max_req > 0 { max_req } else { 60 };
    let window = if window_sec > 0 { window_sec } else { 60 };
    let now = now_secs();

    let mut guard = buckets().lock().unwrap_or_else(|e| e.into_inner());
    if guard.len() >= MAX_RATE_BUCKETS && !guard.contains_key(key) {
        guard.retain(|_, b| now - b.window_start <= window);
        if guard.len() >= MAX_RATE_BUCKETS {
            if let Some(oldest_key) = guard
                .iter()
                .min_by_key(|(_, b)| b.window_start)
                .map(|(k, _)| k.clone())
            {
                guard.remove(&oldest_key);
            }
        }
    }

    let entry = guard.entry(key.to_owned()).or_insert(RateBucket {
        window_start: now,
        count: 0,
    });
    if now - entry.window_start > window {
        entry.window_start = now;
        entry.count = 0;
    }
    entry.count += 1;
    let cur = entry.count;
    if cur > limit {
        -window
    } else {
        limit - cur
    }
}

/// # Safety
/// `ip` must either be null or point to a valid NUL-terminated C string.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_rate_limit_check(
    ip: *const c_char,
    max_req: i64,
    window_sec: i64,
) -> i64 {
    let ip_str = if ip.is_null() {
        "127.0.0.1"
    } else {
        CStr::from_ptr(ip).to_str().unwrap_or("127.0.0.1")
    };
    check_rate_limit(ip_str, max_req, window_sec)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rate_limiter_enforces_limit_and_returns_negative_window() {
        let ip = "10.99.88.77";
        assert_eq!(check_rate_limit(ip, 2, 60), 1);
        assert_eq!(check_rate_limit(ip, 2, 60), 0);
        assert_eq!(check_rate_limit(ip, 2, 60), -60);
    }
}
