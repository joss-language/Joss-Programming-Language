//! Constant-time cryptographic primitives in pure Rust (FIPS 180-4 SHA-256,
//! RFC 3174 SHA-1, RFC 2104 HMAC-SHA256, RFC 2898 PBKDF2-HMAC-SHA256, Base64).
//!
//! Replaces `runtime/native/joss_crypto_primitives.inc` with memory-safe Rust.
#![allow(unknown_lints, clippy::chunks_exact_to_as_chunks)]

const SHA256_K: [u32; 64] = [
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
    0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
    0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
    0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
    0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
    0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
];

fn sha256_compress(state: &mut [u32; 8], block: &[u8; 64]) {
    let mut w = [0u32; 64];
    for (i, chunk) in block.chunks_exact(4).enumerate() {
        w[i] = u32::from_be_bytes([chunk[0], chunk[1], chunk[2], chunk[3]]);
    }
    for i in 16..64 {
        let s0 = w[i - 15].rotate_right(7) ^ w[i - 15].rotate_right(18) ^ (w[i - 15] >> 3);
        let s1 = w[i - 2].rotate_right(17) ^ w[i - 2].rotate_right(19) ^ (w[i - 2] >> 10);
        w[i] = w[i - 16]
            .wrapping_add(s0)
            .wrapping_add(w[i - 7])
            .wrapping_add(s1);
    }
    let [mut a, mut b, mut c, mut d, mut e, mut f, mut g, mut h] = *state;
    for i in 0..64 {
        let s1 = e.rotate_right(6) ^ e.rotate_right(11) ^ e.rotate_right(25);
        let ch = (e & f) ^ ((!e) & g);
        let temp1 = h
            .wrapping_add(s1)
            .wrapping_add(ch)
            .wrapping_add(SHA256_K[i])
            .wrapping_add(w[i]);
        let s0 = a.rotate_right(2) ^ a.rotate_right(13) ^ a.rotate_right(22);
        let maj = (a & b) ^ (a & c) ^ (b & c);
        let temp2 = s0.wrapping_add(maj);
        h = g;
        g = f;
        f = e;
        e = d.wrapping_add(temp1);
        d = c;
        c = b;
        b = a;
        a = temp1.wrapping_add(temp2);
    }
    state[0] = state[0].wrapping_add(a);
    state[1] = state[1].wrapping_add(b);
    state[2] = state[2].wrapping_add(c);
    state[3] = state[3].wrapping_add(d);
    state[4] = state[4].wrapping_add(e);
    state[5] = state[5].wrapping_add(f);
    state[6] = state[6].wrapping_add(g);
    state[7] = state[7].wrapping_add(h);
}

pub fn sha256(data: &[u8]) -> [u8; 32] {
    let mut state: [u32; 8] = [
        0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
        0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19,
    ];
    let mut chunks = data.chunks_exact(64);
    for block in &mut chunks {
        let arr: &[u8; 64] = block.try_into().unwrap();
        sha256_compress(&mut state, arr);
    }
    let rem = chunks.remainder();
    let mut tail = [0u8; 128];
    tail[..rem.len()].copy_from_slice(rem);
    tail[rem.len()] = 0x80;
    let total_blocks = if rem.len() >= 56 { 2 } else { 1 };
    let bit_len = (data.len() as u64).wrapping_mul(8);
    let len_off = total_blocks * 64 - 8;
    tail[len_off..len_off + 8].copy_from_slice(&bit_len.to_be_bytes());
    sha256_compress(&mut state, tail[0..64].try_into().unwrap());
    if total_blocks == 2 {
        sha256_compress(&mut state, tail[64..128].try_into().unwrap());
    }
    let mut out = [0u8; 32];
    for (i, word) in state.iter().enumerate() {
        out[i * 4..i * 4 + 4].copy_from_slice(&word.to_be_bytes());
    }
    out
}

pub fn sha1(data: &[u8]) -> [u8; 20] {
    let mut h: [u32; 5] = [0x67452301, 0xEFCDAB89, 0x98BADCFE, 0x10325476, 0xC3D2E1F0];
    let bit_len = (data.len() as u64).wrapping_mul(8);
    let mut msg = Vec::with_capacity(data.len() + 72);
    msg.extend_from_slice(data);
    msg.push(0x80);
    while (msg.len() % 64) != 56 {
        msg.push(0);
    }
    msg.extend_from_slice(&bit_len.to_be_bytes());
    for chunk in msg.chunks_exact(64) {
        let mut w = [0u32; 80];
        for i in 0..16 {
            w[i] = u32::from_be_bytes([chunk[i * 4], chunk[i * 4 + 1], chunk[i * 4 + 2], chunk[i * 4 + 3]]);
        }
        for i in 16..80 {
            w[i] = (w[i - 3] ^ w[i - 8] ^ w[i - 14] ^ w[i - 16]).rotate_left(1);
        }
        let [mut a, mut b, mut c, mut d, mut e] = h;
        for (i, &wi) in w.iter().enumerate() {
            let (f, k) = match i {
                0..=19 => ((b & c) | ((!b) & d), 0x5A827999u32),
                20..=39 => (b ^ c ^ d, 0x6ED9EBA1u32),
                40..=59 => ((b & c) | (b & d) | (c & d), 0x8F1BBCDCu32),
                _ => (b ^ c ^ d, 0xCA62C1D6u32),
            };
            let temp = a
                .rotate_left(5)
                .wrapping_add(f)
                .wrapping_add(e)
                .wrapping_add(k)
                .wrapping_add(wi);
            e = d;
            d = c;
            c = b.rotate_left(30);
            b = a;
            a = temp;
        }
        h[0] = h[0].wrapping_add(a);
        h[1] = h[1].wrapping_add(b);
        h[2] = h[2].wrapping_add(c);
        h[3] = h[3].wrapping_add(d);
        h[4] = h[4].wrapping_add(e);
    }
    let mut out = [0u8; 20];
    for (i, word) in h.iter().enumerate() {
        out[i * 4..i * 4 + 4].copy_from_slice(&word.to_be_bytes());
    }
    out
}

pub fn hmac_sha256(key: &[u8], msg: &[u8]) -> [u8; 32] {
    let mut k_pad = [0u8; 64];
    if key.len() > 64 {
        let kh = sha256(key);
        k_pad[..32].copy_from_slice(&kh);
    } else {
        k_pad[..key.len()].copy_from_slice(key);
    }
    let mut inner = Vec::with_capacity(64 + msg.len());
    for &b in &k_pad {
        inner.push(b ^ 0x36);
    }
    inner.extend_from_slice(msg);
    let ih = sha256(&inner);
    let mut outer = [0u8; 96];
    for i in 0..64 {
        outer[i] = k_pad[i] ^ 0x5c;
    }
    outer[64..96].copy_from_slice(&ih);
    sha256(&outer)
}

pub fn pbkdf2_hmac_sha256_32(pass: &[u8], salt: &[u8], iterations: u32) -> [u8; 32] {
    let iters = iterations.max(1);
    let mut salt_blk = Vec::with_capacity(salt.len() + 4);
    salt_blk.extend_from_slice(salt);
    salt_blk.extend_from_slice(&1u32.to_be_bytes());
    let mut u = hmac_sha256(pass, &salt_blk);
    let mut out = u;
    for _ in 1..iters {
        u = hmac_sha256(pass, &u);
        for (o, &ub) in out.iter_mut().zip(u.iter()) {
            *o ^= ub;
        }
    }
    out
}

pub fn base64_encode(data: &[u8]) -> String {
    const TBL: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::with_capacity(data.len().div_ceil(3) * 4);
    let mut i = 0;
    while i < data.len() {
        let b0 = data[i] as u32;
        let b1 = if i + 1 < data.len() { data[i + 1] as u32 } else { 0 };
        let b2 = if i + 2 < data.len() { data[i + 2] as u32 } else { 0 };
        let triple = (b0 << 16) | (b1 << 8) | b2;
        out.push(TBL[((triple >> 18) & 0x3F) as usize] as char);
        out.push(TBL[((triple >> 12) & 0x3F) as usize] as char);
        out.push(if i + 1 < data.len() {
            TBL[((triple >> 6) & 0x3F) as usize] as char
        } else {
            '='
        });
        out.push(if i + 2 < data.len() {
            TBL[(triple & 0x3F) as usize] as char
        } else {
            '='
        });
        i += 3;
    }
    out
}

pub fn ct_eq(a: &[u8], b: &[u8]) -> bool {
    if a.len() != b.len() {
        return false;
    }
    let mut diff = 0u8;
    for (&x, &y) in a.iter().zip(b.iter()) {
        diff |= x ^ y;
    }
    diff == 0
}

/// # Safety
/// `out` must point to 32 writable bytes; `data` must point to `len` readable bytes (or be null when `len == 0`).
#[no_mangle]
pub unsafe extern "C" fn joss_rs_sha256(data: *const u8, len: usize, out: *mut u8) {
    if out.is_null() {
        return;
    }
    let slice = if data.is_null() || len == 0 {
        &[]
    } else {
        std::slice::from_raw_parts(data, len)
    };
    let digest = sha256(slice);
    std::ptr::copy_nonoverlapping(digest.as_ptr(), out, 32);
}

/// # Safety
/// `out` must point to 20 writable bytes; `data` must point to `len` readable bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_sha1(data: *const u8, len: usize, out: *mut u8) {
    if out.is_null() {
        return;
    }
    let slice = if data.is_null() || len == 0 {
        &[]
    } else {
        std::slice::from_raw_parts(data, len)
    };
    let digest = sha1(slice);
    std::ptr::copy_nonoverlapping(digest.as_ptr(), out, 20);
}

/// # Safety
/// `out` must point to 32 writable bytes; `key`/`msg` must point to `key_len`/`msg_len` readable bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_hmac_sha256(
    key: *const u8,
    key_len: usize,
    msg: *const u8,
    msg_len: usize,
    out: *mut u8,
) {
    if out.is_null() {
        return;
    }
    let k = if key.is_null() || key_len == 0 {
        &[]
    } else {
        std::slice::from_raw_parts(key, key_len)
    };
    let m = if msg.is_null() || msg_len == 0 {
        &[]
    } else {
        std::slice::from_raw_parts(msg, msg_len)
    };
    let digest = hmac_sha256(k, m);
    std::ptr::copy_nonoverlapping(digest.as_ptr(), out, 32);
}

/// # Safety
/// `out` must point to 32 writable bytes; `pass`/`salt` must point to `pass_len`/`salt_len` readable bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_pbkdf2_sha256(
    pass: *const u8,
    pass_len: usize,
    salt: *const u8,
    salt_len: usize,
    iterations: u32,
    out: *mut u8,
) {
    if out.is_null() {
        return;
    }
    let p = if pass.is_null() || pass_len == 0 {
        &[]
    } else {
        std::slice::from_raw_parts(pass, pass_len)
    };
    let s = if salt.is_null() || salt_len == 0 {
        &[]
    } else {
        std::slice::from_raw_parts(salt, salt_len)
    };
    let dk = pbkdf2_hmac_sha256_32(p, s, iterations);
    std::ptr::copy_nonoverlapping(dk.as_ptr(), out, 32);
}

/// # Safety
/// `a` and `b` must point to `len` readable bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_ct_eq(a: *const u8, b: *const u8, len: usize) -> i64 {
    if len == 0 {
        return 1;
    }
    if a.is_null() || b.is_null() {
        return 0;
    }
    let sa = std::slice::from_raw_parts(a, len);
    let sb = std::slice::from_raw_parts(b, len);
    if ct_eq(sa, sb) { 1 } else { 0 }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn to_hex(bytes: &[u8]) -> String {
        bytes.iter().map(|b| format!("{b:02x}")).collect()
    }

    #[test]
    fn sha256_nist_vector() {
        assert_eq!(
            to_hex(&sha256(b"abc")),
            "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
        );
        assert_eq!(
            to_hex(&sha256(b"")),
            "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
        );
    }

    #[test]
    fn sha1_rfc3174_vector() {
        assert_eq!(
            to_hex(&sha1(b"abc")),
            "a9993e364706816aba3e25717850c26c9cd0d89d"
        );
    }

    #[test]
    fn hmac_sha256_rfc4231_vector() {
        let mac = hmac_sha256(&[0x0bu8; 20], b"Hi There");
        assert_eq!(
            to_hex(&mac),
            "b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7"
        );
    }

    #[test]
    fn pbkdf2_hmac_sha256_rfc6070_vector() {
        let dk = pbkdf2_hmac_sha256_32(b"password", b"salt", 1);
        assert_eq!(
            to_hex(&dk),
            "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"
        );
    }

    #[test]
    fn base64_and_ct_eq_work() {
        assert_eq!(base64_encode(b"foobar"), "Zm9vYmFy");
        assert!(ct_eq(b"secret", b"secret"));
        assert!(!ct_eq(b"secret", b"secrex"));
    }
}
