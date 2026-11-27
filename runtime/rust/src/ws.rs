//! RFC 6455 WebSocket handshake & frame masking primitives in pure Rust.

use crate::crypto::{base64_encode, sha1};
use std::ffi::{c_char, CStr};

const WS_MAGIC_GUID: &str = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11";

pub fn accept_key(client_key: &str) -> String {
    let mut input = String::with_capacity(client_key.len() + WS_MAGIC_GUID.len());
    input.push_str(client_key.trim());
    input.push_str(WS_MAGIC_GUID);
    let digest = sha1(input.as_bytes());
    base64_encode(&digest)
}

pub fn unmask_in_place(payload: &mut [u8], mask: [u8; 4]) {
    for (i, byte) in payload.iter_mut().enumerate() {
        *byte ^= mask[i & 3];
    }
}

/// Computes the RFC 6455 `Sec-WebSocket-Accept` header value into `out_buf`.
///
/// # Safety
/// `client_key` must be a valid NUL-terminated C string; `out_buf` must point to `out_cap` writable bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_ws_accept_key(
    client_key: *const c_char,
    out_buf: *mut c_char,
    out_cap: usize,
) -> usize {
    if client_key.is_null() || out_buf.is_null() || out_cap == 0 {
        return 0;
    }
    let Ok(key_str) = CStr::from_ptr(client_key).to_str() else {
        return 0;
    };
    let acc = accept_key(key_str);
    let bytes = acc.as_bytes();
    if bytes.len() + 1 > out_cap {
        return 0;
    }
    std::ptr::copy_nonoverlapping(bytes.as_ptr(), out_buf.cast::<u8>(), bytes.len());
    *out_buf.add(bytes.len()) = 0;
    bytes.len()
}

/// XOR-unmasks a WebSocket frame payload in place.
///
/// # Safety
/// `payload` must point to `len` writable bytes; `mask4` must point to 4 readable bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_ws_unmask(payload: *mut u8, len: usize, mask4: *const u8) {
    if payload.is_null() || mask4.is_null() || len == 0 {
        return;
    }
    let mask = [
        *mask4,
        *mask4.add(1),
        *mask4.add(2),
        *mask4.add(3),
    ];
    let slice = std::slice::from_raw_parts_mut(payload, len);
    unmask_in_place(slice, mask);
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rfc6455_accept_key_vector() {
        // Official RFC 6455 Section 1.3 test vector
        assert_eq!(
            accept_key("dGhlIHNhbXBsZSBub25jZQ=="),
            "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
        );
    }

    #[test]
    fn unmask_roundtrip() {
        let mut buf = b"Hello Joss WS!".to_vec();
        let mask = [0x37, 0xFA, 0x21, 0x3D];
        unmask_in_place(&mut buf, mask);
        assert_ne!(&buf, b"Hello Joss WS!");
        unmask_in_place(&mut buf, mask);
        assert_eq!(&buf, b"Hello Joss WS!");
    }
}
