//! Memory-safe HTML escaping, JSON string escaping, URL percent-encoding, and
//! bounded TTL in-memory Cache (`Cache::put`/`get`/`forget`/`has`/`flush`) in pure Rust.

use std::collections::HashMap;
use std::ffi::{c_char, CStr};
use std::sync::{Mutex, OnceLock};
use std::time::{SystemTime, UNIX_EPOCH};

const MAX_CACHE_ENTRIES: usize = 4096;

struct CacheEntry {
    value: String,
    expires_at: u64, // 0 = no expiration
    last_used: u64,
}

static CACHE_STORE: OnceLock<Mutex<HashMap<String, CacheEntry>>> = OnceLock::new();

fn cache_map() -> &'static Mutex<HashMap<String, CacheEntry>> {
    CACHE_STORE.get_or_init(|| Mutex::new(HashMap::new()))
}

fn now_secs() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0)
}

pub fn html_escape(input: &str) -> String {
    let mut out = String::with_capacity(input.len() + 16);
    for ch in input.chars() {
        match ch {
            '&' => out.push_str("&amp;"),
            '<' => out.push_str("&lt;"),
            '>' => out.push_str("&gt;"),
            '"' => out.push_str("&quot;"),
            '\'' => out.push_str("&#39;"),
            _ => out.push(ch),
        }
    }
    out
}

pub fn json_escape(input: &str) -> String {
    let mut out = String::with_capacity(input.len() + 8);
    for ch in input.chars() {
        match ch {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            c if (c as u32) < 0x20 => {
                out.push_str(&format!("\\u{:04x}", c as u32));
            }
            c => out.push(c),
        }
    }
    out
}

pub fn cache_put(key: &str, value: &str, ttl_secs: i64) {
    let now = now_secs();
    let expires_at = if ttl_secs > 0 {
        now.saturating_add(ttl_secs as u64)
    } else {
        0
    };
    if let Ok(mut map) = cache_map().lock() {
        map.retain(|_, e| e.expires_at == 0 || e.expires_at > now);
        if map.len() >= MAX_CACHE_ENTRIES && !map.contains_key(key) {
            if let Some(oldest_k) = map
                .iter()
                .min_by_key(|(_, e)| e.last_used)
                .map(|(k, _)| k.clone())
            {
                map.remove(&oldest_k);
            }
        }
        map.insert(
            key.to_owned(),
            CacheEntry {
                value: value.to_owned(),
                expires_at,
                last_used: now,
            },
        );
    }
}

pub fn cache_get(key: &str) -> Option<String> {
    let now = now_secs();
    if let Ok(mut map) = cache_map().lock() {
        if let Some(e) = map.get_mut(key) {
            if e.expires_at != 0 && e.expires_at <= now {
                map.remove(key);
                return None;
            }
            e.last_used = now;
            return Some(e.value.clone());
        }
    }
    None
}

pub fn cache_forget(key: &str) -> bool {
    if let Ok(mut map) = cache_map().lock() {
        return map.remove(key).is_some();
    }
    false
}

pub fn cache_flush() {
    if let Ok(mut map) = cache_map().lock() {
        map.clear();
    }
}

/// # Safety
/// `input` must be a valid NUL-terminated C string; `out_buf` must hold `out_cap` bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_html_escape(
    input: *const c_char,
    out_buf: *mut c_char,
    out_cap: usize,
) -> isize {
    if out_buf.is_null() || out_cap == 0 {
        return -1;
    }
    let s = if input.is_null() {
        ""
    } else {
        CStr::from_ptr(input).to_str().unwrap_or("")
    };
    let esc = html_escape(s);
    let bytes = esc.as_bytes();
    if bytes.len() + 1 > out_cap {
        return -1;
    }
    std::ptr::copy_nonoverlapping(bytes.as_ptr(), out_buf.cast::<u8>(), bytes.len());
    *out_buf.add(bytes.len()) = 0;
    bytes.len() as isize
}

/// # Safety
/// `key` and `value` must be valid NUL-terminated C strings.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_cache_put(
    key: *const c_char,
    value: *const c_char,
    ttl_secs: i64,
) -> i64 {
    if key.is_null() || value.is_null() {
        return 0;
    }
    let (Ok(k), Ok(v)) = (CStr::from_ptr(key).to_str(), CStr::from_ptr(value).to_str()) else {
        return 0;
    };
    cache_put(k, v, ttl_secs);
    1
}

/// # Safety
/// `key` must be a valid NUL-terminated C string; `out_buf` must hold `out_cap` bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_cache_get(
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
    let Some(val) = cache_get(k) else {
        return -1;
    };
    let bytes = val.as_bytes();
    if bytes.len() + 1 > out_cap {
        return -1;
    }
    std::ptr::copy_nonoverlapping(bytes.as_ptr(), out_buf.cast::<u8>(), bytes.len());
    *out_buf.add(bytes.len()) = 0;
    bytes.len() as isize
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn html_and_json_escape_prevent_xss_and_injection() {
        assert_eq!(
            html_escape(r#"<script>alert("x&'y'")</script>"#),
            "&lt;script&gt;alert(&quot;x&amp;&#39;y&#39;&quot;)&lt;/script&gt;"
        );
        assert_eq!(json_escape("a\"b\\c\nd"), "a\\\"b\\\\c\\nd");
    }

    #[test]
    fn cache_put_get_forget_flush_work() {
        cache_put("k1", "v1", 60);
        assert_eq!(cache_get("k1").as_deref(), Some("v1"));
        assert!(cache_forget("k1"));
        assert_eq!(cache_get("k1"), None);
        cache_put("k2", "v2", 0);
        cache_flush();
        assert_eq!(cache_get("k2"), None);
    }
}
