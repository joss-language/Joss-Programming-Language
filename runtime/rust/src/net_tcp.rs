//! Cross-platform, memory-safe TCP client primitives in pure Rust (`std::net::TcpStream`).
//!
//! Replaces POSIX/Winsock `#ifdef` boilerplate in `joss_web_rt.c` with a single
//! thread-safe handle table (`Mutex<HashMap<i64, TcpStream>>`) and automatic
//! RAII socket closing on drop.

use std::collections::HashMap;
use std::ffi::{c_char, CStr};
use std::io::{Read, Write};
use std::net::{TcpStream, ToSocketAddrs};
use std::sync::atomic::{AtomicI64, Ordering};
use std::sync::{Mutex, OnceLock};
use std::time::Duration;

static NEXT_FD: AtomicI64 = AtomicI64::new(1);
static SOCKETS: OnceLock<Mutex<HashMap<i64, TcpStream>>> = OnceLock::new();

fn table() -> &'static Mutex<HashMap<i64, TcpStream>> {
    SOCKETS.get_or_init(|| Mutex::new(HashMap::new()))
}

pub fn tcp_connect(host: &str, port: u16, timeout_ms: u64) -> Option<i64> {
    let timeout = Duration::from_millis(timeout_ms.max(100));
    let addrs = (host, port).to_socket_addrs().ok()?;
    for addr in addrs {
        if let Ok(stream) = TcpStream::connect_timeout(&addr, timeout) {
            let _ = stream.set_read_timeout(Some(timeout));
            let _ = stream.set_write_timeout(Some(timeout));
            let _ = stream.set_nodelay(true);
            let fd = NEXT_FD.fetch_add(1, Ordering::Relaxed);
            let mut guard = table().lock().unwrap_or_else(|e| e.into_inner());
            guard.insert(fd, stream);
            return Some(fd);
        }
    }
    None
}

pub fn tcp_write(fd: i64, data: &[u8]) -> isize {
    let mut guard = table().lock().unwrap_or_else(|e| e.into_inner());
    let Some(stream) = guard.get_mut(&fd) else {
        return -1;
    };
    match stream.write_all(data) {
        Ok(()) => data.len() as isize,
        Err(_) => -1,
    }
}

pub fn tcp_read(fd: i64, out: &mut [u8]) -> isize {
    let mut guard = table().lock().unwrap_or_else(|e| e.into_inner());
    let Some(stream) = guard.get_mut(&fd) else {
        return -1;
    };
    let mut got = 0usize;
    while got < out.len() {
        match stream.read(&mut out[got..]) {
            Ok(0) => break,
            Ok(n) => got += n,
            Err(_) => break,
        }
    }
    got as isize
}

pub fn tcp_read_line(fd: i64, out: &mut [u8]) -> isize {
    if out.is_empty() {
        return 0;
    }
    let mut guard = table().lock().unwrap_or_else(|e| e.into_inner());
    let Some(stream) = guard.get_mut(&fd) else {
        return -1;
    };
    let mut got = 0usize;
    let mut byte = [0u8; 1];
    while got < out.len() {
        match stream.read(&mut byte) {
            Ok(0) | Err(_) => break,
            Ok(_) => {
                if byte[0] == b'\n' {
                    break;
                }
                if byte[0] != b'\r' {
                    out[got] = byte[0];
                    got += 1;
                }
            }
        }
    }
    got as isize
}

pub fn tcp_close(fd: i64) -> i64 {
    let mut guard = table().lock().unwrap_or_else(|e| e.into_inner());
    if guard.remove(&fd).is_some() { 1 } else { 0 }
}

/// # Safety
/// `host` must be a valid NUL-terminated C string.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_tcp_connect(
    host: *const c_char,
    port: i64,
    timeout_ms: i64,
) -> i64 {
    if host.is_null() || port <= 0 || port > 65535 {
        return -1;
    }
    let Ok(host_str) = CStr::from_ptr(host).to_str() else {
        return -1;
    };
    let t_ms = if timeout_ms > 0 { timeout_ms as u64 } else { 3000 };
    tcp_connect(host_str, port as u16, t_ms).unwrap_or(-1)
}

/// # Safety
/// `data` must point to `len` readable bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_tcp_write(fd: i64, data: *const u8, len: usize) -> i64 {
    if fd < 0 || data.is_null() {
        return -1;
    }
    let slice = std::slice::from_raw_parts(data, len);
    tcp_write(fd, slice) as i64
}

/// # Safety
/// `buf` must point to `max_bytes` writable bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_tcp_read(fd: i64, buf: *mut u8, max_bytes: usize) -> i64 {
    if fd < 0 || buf.is_null() || max_bytes == 0 {
        return 0;
    }
    let slice = std::slice::from_raw_parts_mut(buf, max_bytes);
    tcp_read(fd, slice) as i64
}

/// # Safety
/// `buf` must point to `cap` writable bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_tcp_read_line(fd: i64, buf: *mut u8, cap: usize) -> i64 {
    if fd < 0 || buf.is_null() || cap == 0 {
        return 0;
    }
    let slice = std::slice::from_raw_parts_mut(buf, cap);
    tcp_read_line(fd, slice) as i64
}

#[no_mangle]
pub extern "C" fn joss_rs_tcp_close(fd: i64) -> i64 {
    if fd < 0 {
        return 0;
    }
    tcp_close(fd)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::TcpListener;

    #[test]
    fn tcp_client_loopback_roundtrip() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        let server = std::thread::spawn(move || {
            let (mut conn, _) = listener.accept().unwrap();
            let mut req = [0u8; 6];
            conn.read_exact(&mut req).unwrap();
            assert_eq!(&req, b"PING\r\n");
            conn.write_all(b"+PONG\r\n").unwrap();
        });

        let fd = tcp_connect("127.0.0.1", port, 2000).expect("connect failed");
        assert_eq!(tcp_write(fd, b"PING\r\n"), 6);
        let mut line = [0u8; 32];
        let n = tcp_read_line(fd, &mut line);
        assert_eq!(&line[..n as usize], b"+PONG");
        assert_eq!(tcp_close(fd), 1);
        assert_eq!(tcp_close(fd), 0);
        server.join().unwrap();
    }
}
