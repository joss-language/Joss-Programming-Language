//! Cross-platform HTTP/1.1 TCP Server & Bounded Worker Pool in pure Rust.
//!
//! Uses `std::net::TcpListener`, `Mutex<VecDeque<TcpStream>>`, and `Condvar`
//! with automatic RAII socket closing and per-request timeouts (zero `#ifdef`s).

use std::collections::VecDeque;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::{Arc, Condvar, Mutex};
use std::time::Duration;

const MAX_HEADER_BYTES: usize = 64 * 1024;
const DEFAULT_MAX_BODY_BYTES: usize = 16 * 1024 * 1024;
const MAX_QUEUE_LEN: usize = 1024;

/// Reads a complete HTTP/1.1 request (headers + Content-Length body) from `stream`.
pub fn read_http_request(stream: &mut TcpStream, max_body_bytes: usize) -> Option<Vec<u8>> {
    let mut buf = Vec::with_capacity(4096);
    let mut tmp = [0u8; 4096];
    let mut hdr_end: Option<usize> = None;

    while buf.len() < MAX_HEADER_BYTES {
        match stream.read(&mut tmp) {
            Ok(0) => break,
            Ok(n) => {
                buf.extend_from_slice(&tmp[..n]);
                if let Some(pos) = find_subslice(&buf, b"\r\n\r\n") {
                    hdr_end = Some(pos + 4);
                    break;
                }
            }
            Err(_) => break,
        }
    }
    let hdr_end = hdr_end?;
    let content_len = parse_content_length(&buf[..hdr_end]).min(max_body_bytes);
    let need = hdr_end + content_len;
    while buf.len() < need {
        let to_read = (need - buf.len()).min(tmp.len());
        match stream.read(&mut tmp[..to_read]) {
            Ok(0) => break,
            Ok(n) => buf.extend_from_slice(&tmp[..n]),
            Err(_) => break,
        }
    }
    Some(buf)
}

fn find_subslice(haystack: &[u8], needle: &[u8]) -> Option<usize> {
    haystack.windows(needle.len()).position(|w| w == needle)
}

fn parse_content_length(headers: &[u8]) -> usize {
    let Ok(text) = std::str::from_utf8(headers) else {
        return 0;
    };
    for line in text.split("\r\n") {
        if let Some((k, v)) = line.split_once(':') {
            if k.eq_ignore_ascii_case("content-length") {
                return v.trim().parse::<usize>().unwrap_or(0);
            }
        }
    }
    0
}

pub struct HttpWorkerPool {
    queue: Mutex<VecDeque<TcpStream>>,
    not_empty: Condvar,
    not_full: Condvar,
}

impl HttpWorkerPool {
    pub fn start<F>(nworkers: usize, handler: F) -> Arc<Self>
    where
        F: Fn(TcpStream) + Send + Sync + 'static,
    {
        let pool = Arc::new(Self {
            queue: Mutex::new(VecDeque::with_capacity(64)),
            not_empty: Condvar::new(),
            not_full: Condvar::new(),
        });
        let handler = Arc::new(handler);
        let workers = nworkers.clamp(2, 64);
        for _ in 0..workers {
            let p = Arc::clone(&pool);
            let h = Arc::clone(&handler);
            let _ = std::thread::Builder::new()
                .name("joss-http-worker".to_owned())
                .spawn(move || loop {
                    let conn = {
                        let mut q = p.queue.lock().unwrap_or_else(|e| e.into_inner());
                        while q.is_empty() {
                            q = p.not_empty.wait(q).unwrap_or_else(|e| e.into_inner());
                        }
                        let c = q.pop_front();
                        p.not_full.notify_one();
                        c
                    };
                    if let Some(stream) = conn {
                        h(stream);
                    }
                });
        }
        pool
    }

    pub fn enqueue(&self, stream: TcpStream) {
        let mut q = self.queue.lock().unwrap_or_else(|e| e.into_inner());
        while q.len() >= MAX_QUEUE_LEN {
            q = self.not_full.wait(q).unwrap_or_else(|e| e.into_inner());
        }
        q.push_back(stream);
        self.not_empty.notify_one();
    }
}

/// Runs a simple concurrent HTTP server loop on `listener`, invoking `respond` for each request.
pub fn serve_listener<F>(listener: TcpListener, nworkers: usize, respond: F)
where
    F: Fn(&[u8], &str) -> Vec<u8> + Send + Sync + 'static,
{
    let respond = Arc::new(respond);
    let pool = HttpWorkerPool::start(nworkers, move |mut stream| {
        let _ = stream.set_nodelay(true);
        let _ = stream.set_read_timeout(Some(Duration::from_secs(15)));
        let _ = stream.set_write_timeout(Some(Duration::from_secs(15)));
        let peer_ip = stream
            .peer_addr()
            .map(|a| a.ip().to_string())
            .unwrap_or_else(|_| "127.0.0.1".to_owned());
        if let Some(req) = read_http_request(&mut stream, DEFAULT_MAX_BODY_BYTES) {
            let resp = respond(&req, &peer_ip);
            let _ = stream.write_all(&resp);
            let _ = stream.flush();
        }
    });
    for stream in listener.incoming().flatten() {
        pool.enqueue(stream);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn http_server_serves_concurrent_requests_cleanly() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        std::thread::spawn(move || {
            serve_listener(listener, 4, |req, _ip| {
                assert!(req.starts_with(b"GET /ping HTTP/1.1"));
                b"HTTP/1.1 200 OK\r\nContent-Length: 4\r\nConnection: close\r\n\r\nPONG".to_vec()
            });
        });

        for _ in 0..8 {
            let mut client = TcpStream::connect(("127.0.0.1", port)).unwrap();
            client
                .write_all(b"GET /ping HTTP/1.1\r\nHost: localhost\r\n\r\n")
                .unwrap();
            let mut out = String::new();
            client.read_to_string(&mut out).unwrap();
            assert!(out.ends_with("PONG"), "unexpected response: {out}");
        }
    }
}
