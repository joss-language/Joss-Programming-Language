//! Complete CSPRNG, PBKDF2 password hashing, HS256 JWT, and RFC 6238 TOTP in pure Rust.
//!
//! Replaces `runtime/native/joss_crypto_rt.c` with zero OpenSSL dependency and
//! constant-time verification.

use crate::crypto::{ct_eq, hmac_sha256, pbkdf2_hmac_sha256_32, sha1};
use std::ffi::{c_char, CStr};
use std::time::{SystemTime, UNIX_EPOCH};

#[cfg(target_os = "windows")]
pub fn fill_csprng(out: &mut [u8]) -> bool {
    #[link(name = "bcrypt")]
    extern "system" {
        fn BCryptGenRandom(
            h_algorithm: *mut std::ffi::c_void,
            pb_buffer: *mut u8,
            cb_buffer: u32,
            dw_flags: u32,
        ) -> i32;
    }
    const BCRYPT_USE_SYSTEM_PREFERRED_RNG: u32 = 0x0000_0002;
    let status = unsafe {
        BCryptGenRandom(
            std::ptr::null_mut(),
            out.as_mut_ptr(),
            out.len() as u32,
            BCRYPT_USE_SYSTEM_PREFERRED_RNG,
        )
    };
    status == 0
}

#[cfg(not(target_os = "windows"))]
pub fn fill_csprng(out: &mut [u8]) -> bool {
    use std::io::Read;
    let Ok(mut f) = std::fs::File::open("/dev/urandom") else {
        return false;
    };
    f.read_exact(out).is_ok()
}

pub fn to_hex(bytes: &[u8]) -> String {
    const HEX: &[u8; 16] = b"0123456789abcdef";
    let mut s = String::with_capacity(bytes.len() * 2);
    for &b in bytes {
        s.push(HEX[(b >> 4) as usize] as char);
        s.push(HEX[(b & 0x0F) as usize] as char);
    }
    s
}

pub fn from_hex(hex: &str) -> Option<Vec<u8>> {
    if !hex.len().is_multiple_of(2) {
        return None;
    }
    let bytes = hex.as_bytes();
    let mut out = Vec::with_capacity(bytes.len() / 2);
    let mut i = 0;
    while i < bytes.len() {
        let hi = hex_nibble(bytes[i])?;
        let lo = hex_nibble(bytes[i + 1])?;
        out.push((hi << 4) | lo);
        i += 2;
    }
    Some(out)
}

fn hex_nibble(c: u8) -> Option<u8> {
    match c {
        b'0'..=b'9' => Some(c - b'0'),
        b'a'..=b'f' => Some(c - b'a' + 10),
        b'A'..=b'F' => Some(c - b'A' + 10),
        _ => None,
    }
}

pub fn b64url_encode(data: &[u8]) -> String {
    const TBL: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
    let mut out = String::with_capacity(data.len().div_ceil(3) * 4);
    let mut i = 0;
    while i < data.len() {
        let b0 = data[i] as u32;
        let b1 = if i + 1 < data.len() { data[i + 1] as u32 } else { 0 };
        let b2 = if i + 2 < data.len() { data[i + 2] as u32 } else { 0 };
        let triple = (b0 << 16) | (b1 << 8) | b2;
        out.push(TBL[((triple >> 18) & 0x3F) as usize] as char);
        out.push(TBL[((triple >> 12) & 0x3F) as usize] as char);
        if i + 1 < data.len() {
            out.push(TBL[((triple >> 6) & 0x3F) as usize] as char);
        }
        if i + 2 < data.len() {
            out.push(TBL[(triple & 0x3F) as usize] as char);
        }
        i += 3;
    }
    out
}

pub fn password_hash(password: &str) -> Option<String> {
    const ITERATIONS: u32 = 100_000;
    let mut salt = [0u8; 16];
    if !fill_csprng(&mut salt) {
        return None;
    }
    let dk = pbkdf2_hmac_sha256_32(password.as_bytes(), &salt, ITERATIONS);
    Some(format!(
        "$pbkdf2-sha256${ITERATIONS}${}${}",
        to_hex(&salt),
        to_hex(&dk)
    ))
}

pub fn password_verify_pbkdf2(hash: &str, password: &str) -> bool {
    let Some(rest) = hash.strip_prefix("$pbkdf2-sha256$") else {
        return false;
    };
    let mut parts = rest.split('$');
    let (Some(iter_s), Some(salt_hex), Some(dk_hex), None) =
        (parts.next(), parts.next(), parts.next(), parts.next())
    else {
        return false;
    };
    let Ok(iterations) = iter_s.parse::<u32>() else {
        return false;
    };
    if !(1000..=5_000_000).contains(&iterations) || salt_hex.len() != 32 || dk_hex.len() != 64 {
        return false;
    }
    let (Some(salt), Some(expected_dk)) = (from_hex(salt_hex), from_hex(dk_hex)) else {
        return false;
    };
    let actual_dk = pbkdf2_hmac_sha256_32(password.as_bytes(), &salt, iterations);
    ct_eq(&actual_dk, &expected_dk)
}

pub fn jwt_sign(payload_json: &str, secret: &str) -> String {
    let payload = if payload_json.is_empty() {
        "{}"
    } else {
        payload_json
    };
    let header_b64 = b64url_encode(br#"{"alg":"HS256","typ":"JWT"}"#);
    let payload_b64 = b64url_encode(payload.as_bytes());
    let msg = format!("{header_b64}.{payload_b64}");
    let mac = hmac_sha256(secret.as_bytes(), msg.as_bytes());
    let sig_b64 = b64url_encode(&mac);
    format!("{msg}.{sig_b64}")
}

pub fn jwt_verify(token: &str, secret: &str) -> bool {
    let mut parts = token.split('.');
    let (Some(h), Some(p), Some(given_sig), None) =
        (parts.next(), parts.next(), parts.next(), parts.next())
    else {
        return false;
    };
    if given_sig.is_empty() {
        return false;
    }
    let msg = format!("{h}.{p}");
    let mac = hmac_sha256(secret.as_bytes(), msg.as_bytes());
    let expected_sig = b64url_encode(&mac);
    ct_eq(expected_sig.as_bytes(), given_sig.as_bytes())
}

fn hmac_sha1(key: &[u8], msg: &[u8]) -> [u8; 20] {
    let mut k_pad = [0u8; 64];
    if key.len() > 64 {
        let kh = sha1(key);
        k_pad[..20].copy_from_slice(&kh);
    } else {
        k_pad[..key.len()].copy_from_slice(key);
    }
    let mut inner = Vec::with_capacity(64 + msg.len());
    for &b in &k_pad {
        inner.push(b ^ 0x36);
    }
    inner.extend_from_slice(msg);
    let ih = sha1(&inner);
    let mut outer = [0u8; 84];
    for i in 0..64 {
        outer[i] = k_pad[i] ^ 0x5c;
    }
    outer[64..84].copy_from_slice(&ih);
    sha1(&outer)
}

fn base32_decode(input: &str) -> Vec<u8> {
    let mut out = Vec::new();
    let mut buffer: u32 = 0;
    let mut bits_left: u32 = 0;
    for b in input.bytes() {
        let val = match b {
            b'A'..=b'Z' => b - b'A',
            b'a'..=b'z' => b - b'a',
            b'2'..=b'7' => b - b'2' + 26,
            _ => continue,
        };
        buffer = (buffer << 5) | (val as u32);
        bits_left += 5;
        if bits_left >= 8 {
            bits_left -= 8;
            out.push(((buffer >> bits_left) & 0xFF) as u8);
        }
    }
    out
}

fn totp_for_step(key: &[u8], step: u64) -> String {
    let msg = step.to_be_bytes();
    let digest = hmac_sha1(key, &msg);
    let offset = (digest[19] & 0x0F) as usize;
    let bin_code = (((digest[offset] & 0x7F) as u32) << 24)
        | ((digest[offset + 1] as u32) << 16)
        | ((digest[offset + 2] as u32) << 8)
        | (digest[offset + 3] as u32);
    format!("{:06}", bin_code % 1_000_000)
}

fn resolve_totp_key(secret: &str) -> Vec<u8> {
    let decoded = base32_decode(secret);
    if decoded.is_empty() {
        secret.as_bytes().to_vec()
    } else {
        decoded
    }
}

pub fn totp_generate(secret: &str, unix_time: i64) -> String {
    if secret.is_empty() {
        return "000000".to_owned();
    }
    let ts = if unix_time > 0 {
        unix_time
    } else {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_secs() as i64)
            .unwrap_or(0)
    };
    let key = resolve_totp_key(secret);
    totp_for_step(&key, (ts / 30) as u64)
}

pub fn totp_verify(secret: &str, code: &str, unix_time: i64) -> bool {
    let clean = code.trim();
    if secret.is_empty() || clean.len() != 6 || !clean.bytes().all(|b| b.is_ascii_digit()) {
        return false;
    }
    let ts = if unix_time > 0 {
        unix_time
    } else {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_secs() as i64)
            .unwrap_or(0)
    };
    let key = resolve_totp_key(secret);
    let base_step = ts / 30;
    for delta in -1..=1 {
        if base_step + delta < 0 {
            continue;
        }
        let expected = totp_for_step(&key, (base_step + delta) as u64);
        if ct_eq(expected.as_bytes(), clean.as_bytes()) {
            return true;
        }
    }
    false
}

/// # Safety
/// `out` must point to `len` writable bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_csprng_bytes(out: *mut u8, len: usize) -> i64 {
    if out.is_null() || len == 0 {
        return 0;
    }
    let slice = std::slice::from_raw_parts_mut(out, len);
    if fill_csprng(slice) { 1 } else { 0 }
}

/// # Safety
/// `payload` and `secret` must be valid NUL-terminated C strings; `out_buf` must hold `out_cap` bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_jwt_sign(
    payload: *const c_char,
    secret: *const c_char,
    out_buf: *mut c_char,
    out_cap: usize,
) -> isize {
    if out_buf.is_null() || out_cap == 0 {
        return -1;
    }
    let p_str = if payload.is_null() {
        "{}"
    } else {
        CStr::from_ptr(payload).to_str().unwrap_or("{}")
    };
    let s_str = if secret.is_null() {
        ""
    } else {
        CStr::from_ptr(secret).to_str().unwrap_or("")
    };
    let token = jwt_sign(p_str, s_str);
    let bytes = token.as_bytes();
    if bytes.len() + 1 > out_cap {
        return -1;
    }
    std::ptr::copy_nonoverlapping(bytes.as_ptr(), out_buf as *mut u8, bytes.len());
    *out_buf.add(bytes.len()) = 0;
    bytes.len() as isize
}

/// # Safety
/// `token` and `secret` must be valid NUL-terminated C strings.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_jwt_verify(
    token: *const c_char,
    secret: *const c_char,
) -> i64 {
    if token.is_null() {
        return 0;
    }
    let Ok(t_str) = CStr::from_ptr(token).to_str() else {
        return 0;
    };
    let s_str = if secret.is_null() {
        ""
    } else {
        CStr::from_ptr(secret).to_str().unwrap_or("")
    };
    if jwt_verify(t_str, s_str) { 1 } else { 0 }
}

/// # Safety
/// `secret` must be a valid NUL-terminated C string; `out7` must point to at least 7 writable bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_totp_generate(
    secret: *const c_char,
    unix_time: i64,
    out7: *mut c_char,
) -> i64 {
    if out7.is_null() {
        return 0;
    }
    let s_str = if secret.is_null() {
        ""
    } else {
        CStr::from_ptr(secret).to_str().unwrap_or("")
    };
    let code = totp_generate(s_str, unix_time);
    let bytes = code.as_bytes();
    std::ptr::copy_nonoverlapping(bytes.as_ptr(), out7 as *mut u8, 6);
    *out7.add(6) = 0;
    1
}

/// # Safety
/// `secret` and `code` must be valid NUL-terminated C strings.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_totp_verify(
    secret: *const c_char,
    code: *const c_char,
    unix_time: i64,
) -> i64 {
    if secret.is_null() || code.is_null() {
        return 0;
    }
    let (Ok(s_str), Ok(c_str)) = (CStr::from_ptr(secret).to_str(), CStr::from_ptr(code).to_str()) else {
        return 0;
    };
    if totp_verify(s_str, c_str, unix_time) { 1 } else { 0 }
}

/// # Safety
/// `hash` and `password` must be valid NUL-terminated C strings.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_password_verify_pbkdf2(
    hash: *const c_char,
    password: *const c_char,
) -> i64 {
    if hash.is_null() || password.is_null() {
        return 0;
    }
    let (Ok(h_str), Ok(p_str)) = (CStr::from_ptr(hash).to_str(), CStr::from_ptr(password).to_str()) else {
        return 0;
    };
    if password_verify_pbkdf2(h_str, p_str) { 1 } else { 0 }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn jwt_sign_and_verify_roundtrip() {
        let tok = jwt_sign(r#"{"sub":"42","role":"admin"}"#, "super-secret");
        assert!(jwt_verify(&tok, "super-secret"));
        assert!(!jwt_verify(&tok, "wrong-secret"));
        assert!(!jwt_verify("not.a.valid.jwt", "super-secret"));
    }

    #[test]
    fn totp_rfc6238_vector() {
        // RFC 6238 test secret "12345678901234567890" (Base32: GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ)
        let secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ";
        let code = totp_generate(secret, 59);
        assert_eq!(code, "287082");
        assert!(totp_verify(secret, "287082", 59));
        assert!(totp_verify(secret, "287082", 89)); // +1 step tolerance
        assert!(!totp_verify(secret, "000000", 59));
    }

    #[test]
    fn password_hash_and_verify_roundtrip() {
        let h = password_hash("JossSuperPass!2026").expect("csprng failed");
        assert!(h.starts_with("$pbkdf2-sha256$100000$"));
        assert!(password_verify_pbkdf2(&h, "JossSuperPass!2026"));
        assert!(!password_verify_pbkdf2(&h, "wrong-password"));
    }
}
