//! Memory-safe core runtime collections in pure Rust:
//! - `RustArray`: dynamic `Vec<i64>` replacing `JossArray`
//! - `RustByteBuf`: dynamic `Vec<u8>` replacing `JossStringBuilder` / `JossByteBuffer`
//! - `RustMap`: insertion-ordered `HashMap<String, usize>` + `Vec<(String, i64)>`
//!   replacing `JossMap` with zero tombstone bloat and automatic `Drop`.

use std::collections::HashMap;
use std::ffi::{c_char, CStr, CString};

pub struct RustArray {
    items: Vec<i64>,
}

impl RustArray {
    pub fn new(cap: usize) -> Self {
        Self {
            items: Vec::with_capacity(cap.min(4096)),
        }
    }
    pub fn push(&mut self, v: i64) {
        self.items.push(v);
    }
    pub fn pop(&mut self) -> i64 {
        self.items.pop().unwrap_or(0)
    }
    pub fn get(&self, idx: i64) -> i64 {
        if idx < 0 {
            return 0;
        }
        self.items.get(idx as usize).copied().unwrap_or(0)
    }
    pub fn set(&mut self, idx: i64, v: i64) {
        if idx < 0 {
            return;
        }
        let u = idx as usize;
        if u >= self.items.len() {
            self.items.resize(u + 1, 0);
        }
        self.items[u] = v;
    }
    pub fn len(&self) -> usize {
        self.items.len()
    }
    pub fn is_empty(&self) -> bool {
        self.items.is_empty()
    }
}

pub struct RustByteBuf {
    data: Vec<u8>,
}

impl RustByteBuf {
    pub fn new(cap: usize) -> Self {
        Self {
            data: Vec::with_capacity(cap.max(64)),
        }
    }
    pub fn append_bytes(&mut self, slice: &[u8]) {
        self.data.extend_from_slice(slice);
    }
    pub fn append_byte(&mut self, b: u8) {
        self.data.push(b);
    }
    pub fn len(&self) -> usize {
        self.data.len()
    }
    pub fn is_empty(&self) -> bool {
        self.data.is_empty()
    }
    pub fn as_ptr(&self) -> *const u8 {
        self.data.as_ptr()
    }
}

pub struct RustMap {
    index: HashMap<String, usize>,
    entries: Vec<(CString, i64)>,
}

impl RustMap {
    pub fn new() -> Self {
        Self {
            index: HashMap::new(),
            entries: Vec::new(),
        }
    }

    pub fn set(&mut self, key: &str, val: i64) {
        if let Some(&pos) = self.index.get(key) {
            self.entries[pos].1 = val;
            return;
        }
        let ckey = CString::new(key).unwrap_or_else(|_| CString::new("").unwrap());
        let pos = self.entries.len();
        self.entries.push((ckey, val));
        self.index.insert(key.to_owned(), pos);
    }

    pub fn get(&self, key: &str) -> Option<i64> {
        let &pos = self.index.get(key)?;
        Some(self.entries[pos].1)
    }

    pub fn contains(&self, key: &str) -> bool {
        self.index.contains_key(key)
    }

    pub fn delete(&mut self, key: &str) -> bool {
        let Some(pos) = self.index.remove(key) else {
            return false;
        };
        self.entries.remove(pos);
        // Rebuild indices after removed slot (preserves insertion order with 0 tombstones).
        for (i, (k, _)) in self.entries.iter().enumerate().skip(pos) {
            if let Ok(ks) = k.to_str() {
                self.index.insert(ks.to_owned(), i);
            }
        }
        true
    }

    pub fn len(&self) -> usize {
        self.entries.len()
    }

    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    pub fn key_at(&self, idx: usize) -> *const c_char {
        match self.entries.get(idx) {
            Some((k, _)) => k.as_ptr(),
            None => std::ptr::null(),
        }
    }

    pub fn val_at(&self, idx: usize) -> i64 {
        self.entries.get(idx).map(|(_, v)| *v).unwrap_or(0)
    }
}

impl Default for RustMap {
    fn default() -> Self {
        Self::new()
    }
}

/* --- C-ABI exports --- */

#[no_mangle]
pub extern "C" fn joss_rs_arr_new(cap: i64) -> *mut RustArray {
    Box::into_raw(Box::new(RustArray::new(cap.max(0) as usize)))
}

/// # Safety
/// `a` must be a valid pointer returned by `joss_rs_arr_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_arr_push(a: *mut RustArray, v: i64) {
    if let Some(arr) = a.as_mut() {
        arr.push(v);
    }
}

/// # Safety
/// `a` must be a valid pointer returned by `joss_rs_arr_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_arr_get(a: *const RustArray, idx: i64) -> i64 {
    match a.as_ref() {
        Some(arr) => arr.get(idx),
        None => 0,
    }
}

/// # Safety
/// `a` must be a valid pointer returned by `joss_rs_arr_new`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_arr_len(a: *const RustArray) -> i64 {
    match a.as_ref() {
        Some(arr) => arr.len() as i64,
        None => 0,
    }
}

/// # Safety
/// `a` must be a valid pointer returned by `joss_rs_arr_new` and not previously freed.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_arr_free(a: *mut RustArray) {
    if !a.is_null() {
        drop(Box::from_raw(a));
    }
}

#[no_mangle]
pub extern "C" fn joss_rs_map_new() -> *mut RustMap {
    Box::into_raw(Box::new(RustMap::new()))
}

/// # Safety
/// `m` must be a valid pointer returned by `joss_rs_map_new`; `key` must be NUL-terminated.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_map_set(m: *mut RustMap, key: *const c_char, val: i64) {
    if key.is_null() {
        return;
    }
    if let (Some(map), Ok(ks)) = (m.as_mut(), CStr::from_ptr(key).to_str()) {
        map.set(ks, val);
    }
}

/// # Safety
/// `m` must be a valid pointer returned by `joss_rs_map_new`; `key` must be NUL-terminated.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_map_get(m: *const RustMap, key: *const c_char) -> i64 {
    if key.is_null() {
        return 0;
    }
    match (m.as_ref(), CStr::from_ptr(key).to_str()) {
        (Some(map), Ok(ks)) => map.get(ks).unwrap_or(0),
        _ => 0,
    }
}

/// # Safety
/// `m` must be a valid pointer returned by `joss_rs_map_new`; `key` must be NUL-terminated.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_map_del(m: *mut RustMap, key: *const c_char) -> i64 {
    if key.is_null() {
        return 0;
    }
    if let (Some(map), Ok(ks)) = (m.as_mut(), CStr::from_ptr(key).to_str()) {
        i64::from(map.delete(ks))
    } else {
        0
    }
}

/// # Safety
/// `m` must be a valid pointer returned by `joss_rs_map_new` and not previously freed.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_map_free(m: *mut RustMap) {
    if !m.is_null() {
        drop(Box::from_raw(m));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn array_and_map_preserve_order_without_tombstones() {
        let mut a = RustArray::new(2);
        a.push(10);
        a.push(20);
        a.set(1, 25);
        assert_eq!(a.len(), 2);
        assert_eq!(a.get(1), 25);
        assert_eq!(a.pop(), 25);

        let mut m = RustMap::new();
        m.set("a", 1);
        m.set("b", 2);
        m.set("c", 3);
        assert!(m.delete("b"));
        assert_eq!(m.len(), 2);
        assert!(!m.contains("b"));
        assert_eq!(m.get("c"), Some(3));
        assert_eq!(m.val_at(1), 3);
    }

    #[test]
    fn byte_buf_appends_cleanly() {
        let mut b = RustByteBuf::new(16);
        b.append_bytes(b"hello ");
        b.append_byte(b'!');
        assert_eq!(b.len(), 7);
    }
}
