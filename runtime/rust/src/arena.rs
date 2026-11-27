//! Per-request bump Arena allocator in pure Rust with automatic oversized chunk
//! shrinking and RAII `Drop` cleanup (zero manual `free()` calls).
//!
//! Foundation for replacing `JossArena` in `runtime/native/joss_rt.c`.

use std::alloc::{alloc_zeroed, dealloc, Layout};

const DEFAULT_CHUNK_CAP: usize = 65_536;
const ALIGN: usize = 16;

struct ArenaChunk {
    ptr: *mut u8,
    cap: usize,
    used: usize,
}

impl ArenaChunk {
    fn new(cap: usize) -> Option<Self> {
        let layout = Layout::from_size_align(cap, ALIGN).ok()?;
        let ptr = unsafe { alloc_zeroed(layout) };
        if ptr.is_null() {
            return None;
        }
        Some(Self { ptr, cap, used: 0 })
    }
}

impl Drop for ArenaChunk {
    fn drop(&mut self) {
        if !self.ptr.is_null() && self.cap > 0 {
            if let Ok(layout) = Layout::from_size_align(self.cap, ALIGN) {
                unsafe { dealloc(self.ptr, layout) };
            }
            self.ptr = std::ptr::null_mut();
        }
    }
}

pub struct RustArena {
    chunks: Vec<ArenaChunk>,
    default_cap: usize,
}

impl RustArena {
    pub fn new(default_cap: usize) -> Self {
        let cap = if default_cap == 0 {
            DEFAULT_CHUNK_CAP
        } else {
            default_cap
        };
        let mut chunks = Vec::with_capacity(4);
        if let Some(first) = ArenaChunk::new(cap) {
            chunks.push(first);
        }
        Self {
            chunks,
            default_cap: cap,
        }
    }

    pub fn alloc_bytes(&mut self, size: usize) -> *mut u8 {
        let aligned = (size.max(1) + (ALIGN - 1)) & !(ALIGN - 1);
        if let Some(last) = self.chunks.last_mut() {
            if last.used + aligned <= last.cap {
                let p = unsafe { last.ptr.add(last.used) };
                last.used += aligned;
                return p;
            }
        }
        let next_cap = aligned.max(self.default_cap);
        let Some(mut chunk) = ArenaChunk::new(next_cap) else {
            return std::ptr::null_mut();
        };
        let p = chunk.ptr;
        chunk.used = aligned;
        self.chunks.push(chunk);
        p
    }

    pub fn reset(&mut self) {
        // Drop all secondary chunks; also shrink primary chunk if it grew > 4x default.
        self.chunks.truncate(1);
        if let Some(first) = self.chunks.first_mut() {
            if first.cap > self.default_cap * 4 {
                self.chunks.clear();
                if let Some(fresh) = ArenaChunk::new(self.default_cap) {
                    self.chunks.push(fresh);
                }
            } else {
                if first.used > 0 {
                    unsafe { std::ptr::write_bytes(first.ptr, 0, first.used) };
                }
                first.used = 0;
            }
        }
    }

    pub fn bytes_used(&self) -> usize {
        self.chunks.iter().map(|c| c.used).sum()
    }
}

#[no_mangle]
pub extern "C" fn joss_rs_arena_new(default_cap: usize) -> *mut RustArena {
    Box::into_raw(Box::new(RustArena::new(default_cap)))
}

/// # Safety
/// `arena` must be a valid pointer returned by `joss_rs_arena_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_arena_alloc(arena: *mut RustArena, size: usize) -> *mut u8 {
    match arena.as_mut() {
        Some(a) => a.alloc_bytes(size),
        None => std::ptr::null_mut(),
    }
}

/// # Safety
/// `arena` must be a valid pointer returned by `joss_rs_arena_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_arena_reset(arena: *mut RustArena) {
    if let Some(a) = arena.as_mut() {
        a.reset();
    }
}

/// # Safety
/// `arena` must be a valid pointer returned by `joss_rs_arena_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_arena_bytes_used(arena: *const RustArena) -> usize {
    match arena.as_ref() {
        Some(a) => a.bytes_used(),
        None => 0,
    }
}

/// # Safety
/// `arena` must be a valid pointer returned by `joss_rs_arena_new` and not freed previously.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_arena_destroy(arena: *mut RustArena) {
    if !arena.is_null() {
        drop(Box::from_raw(arena));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn arena_alloc_reset_and_shrink_work() {
        let mut a = RustArena::new(4096);
        let p1 = a.alloc_bytes(128);
        assert!(!p1.is_null());
        assert_eq!(a.bytes_used(), 128);
        let p2 = a.alloc_bytes(32_768); // forces oversized secondary chunk
        assert!(!p2.is_null());
        assert!(a.bytes_used() >= 32_896);
        a.reset();
        assert_eq!(a.bytes_used(), 0);
        assert_eq!(a.chunks.len(), 1);
    }
}
