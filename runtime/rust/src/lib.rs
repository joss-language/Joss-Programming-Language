//! Joss Native Runtime — Rust Staticlib Micro-Kernel (`libjoss_rt_rs.a`).
//!
//! Exports C-ABI (`extern "C"`) symbols linked directly into Joss standalone
//! binaries with zero DLLs, zero garbage-collector overhead, and compile-time
//! memory & thread safety guaranteed by Rust's borrow checker.

pub mod arena;
pub mod collections;
pub mod concurrency;
pub mod cron;
pub mod crypto;
pub mod http_server;
pub mod jwt_totp;
pub mod memory;
pub mod multipart;
pub mod net_tcp;
pub mod ratelimit;
pub mod services;
pub mod session_store;
pub mod sync_rt;
pub mod ws;
pub mod zip_rt;

use std::ffi::c_char;

static JOSS_RS_VERSION: &[u8] = b"joss-rt-rs/0.1.0\0";

/// Returns the static version banner of the Rust runtime module.
#[no_mangle]
pub extern "C" fn joss_rs_runtime_version() -> *const c_char {
    JOSS_RS_VERSION.as_ptr() as *const c_char
}

/// Pure, memory-safe IEEE 802.3 CRC32 (used by ZIP archive reader/writer).
///
/// # Safety
/// `data` must either be null or point to at least `len` readable bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_crc32(data: *const u8, len: usize) -> u32 {
    if data.is_null() || len == 0 {
        return 0;
    }
    // SAFETY: Caller guarantees `data` points to `len` readable bytes.
    let slice = unsafe { std::slice::from_raw_parts(data, len) };
    crc32_bytes(slice)
}

pub fn crc32_bytes(bytes: &[u8]) -> u32 {
    let mut crc: u32 = 0xFFFF_FFFF;
    for &b in bytes {
        crc ^= b as u32;
        for _ in 0..8 {
            let mask = (crc & 1).wrapping_neg();
            crc = (crc >> 1) ^ (0xEDB8_8320 & mask);
        }
    }
    !crc
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::ffi::CStr;

    #[test]
    fn runtime_version_banner_is_valid_cstr() {
        let ptr = joss_rs_runtime_version();
        assert!(!ptr.is_null());
        let s = unsafe { CStr::from_ptr(ptr) }.to_str().unwrap();
        assert!(s.starts_with("joss-rt-rs/"));
    }

    #[test]
    fn crc32_matches_standard_vector() {
        assert_eq!(crc32_bytes(b"123456789"), 0xCBF4_3926);
        assert_eq!(crc32_bytes(b""), 0x0000_0000);
    }
}
