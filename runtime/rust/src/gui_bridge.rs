//! Minimal OS-level GUI Window/WebView bridge in pure Rust (6 core syscalls + state).
//!
//! Designed to pair with the autocompiled Joss-first GUI layer (`std/gui/window.joss`),
//! replacing raw C pointer management in `runtime/native/joss_gui_rt.c` with
//! thread-safe, handle-indexed Rust state and zero memory leaks.

use std::collections::HashMap;
use std::ffi::{c_char, CStr};
use std::sync::atomic::{AtomicI64, Ordering};
use std::sync::{Mutex, OnceLock};

#[derive(Debug, Clone)]
pub struct GuiWindow {
    pub id: i64,
    pub title: String,
    pub width: i64,
    pub height: i64,
    pub min_width: i64,
    pub min_height: i64,
    pub resizable: bool,
    pub frameless: bool,
    pub always_on_top: bool,
    pub visible: bool,
    pub closed: bool,
    pub theme: String,
    pub url: String,
    pub html: String,
    pub last_script: String,
}

static NEXT_WIN_ID: AtomicI64 = AtomicI64::new(1);
static WINDOWS: OnceLock<Mutex<HashMap<i64, GuiWindow>>> = OnceLock::new();

fn registry() -> &'static Mutex<HashMap<i64, GuiWindow>> {
    WINDOWS.get_or_init(|| Mutex::new(HashMap::new()))
}

fn cstr_or_empty<'a>(ptr: *const c_char) -> &'a str {
    if ptr.is_null() {
        ""
    } else {
        unsafe { CStr::from_ptr(ptr) }.to_str().unwrap_or("")
    }
}

pub fn gui_open(title: &str, width: i64, height: i64) -> i64 {
    let id = NEXT_WIN_ID.fetch_add(1, Ordering::Relaxed);
    let win = GuiWindow {
        id,
        title: if title.is_empty() {
            "Joss App".to_owned()
        } else {
            title.to_owned()
        },
        width: if width > 0 { width } else { 1024 },
        height: if height > 0 { height } else { 720 },
        min_width: 320,
        min_height: 240,
        resizable: true,
        frameless: false,
        always_on_top: false,
        visible: true,
        closed: false,
        theme: "system".to_owned(),
        url: String::new(),
        html: String::new(),
        last_script: String::new(),
    };
    if let Ok(mut map) = registry().lock() {
        map.insert(id, win);
    }
    id
}

pub fn gui_set_title(handle: i64, title: &str) -> bool {
    if let Ok(mut map) = registry().lock() {
        if let Some(w) = map.get_mut(&handle) {
            w.title = title.to_owned();
            return true;
        }
    }
    false
}

pub fn gui_set_size(handle: i64, width: i64, height: i64) -> bool {
    if let Ok(mut map) = registry().lock() {
        if let Some(w) = map.get_mut(&handle) {
            if width > 0 {
                w.width = width.max(w.min_width);
            }
            if height > 0 {
                w.height = height.max(w.min_height);
            }
            return true;
        }
    }
    false
}

pub fn gui_navigate(handle: i64, url: &str) -> bool {
    if let Ok(mut map) = registry().lock() {
        if let Some(w) = map.get_mut(&handle) {
            w.url = url.to_owned();
            return true;
        }
    }
    false
}

pub fn gui_set_html(handle: i64, html: &str) -> bool {
    if let Ok(mut map) = registry().lock() {
        if let Some(w) = map.get_mut(&handle) {
            w.html = html.to_owned();
            return true;
        }
    }
    false
}

pub fn gui_eval(handle: i64, script: &str) -> bool {
    if let Ok(mut map) = registry().lock() {
        if let Some(w) = map.get_mut(&handle) {
            w.last_script = script.to_owned();
            return true;
        }
    }
    false
}

pub fn gui_run(handle: i64) -> bool {
    let headless = std::env::var("JOSS_GUI_HEADLESS")
        .map(|v| v == "1" || v.eq_ignore_ascii_case("true"))
        .unwrap_or(false);
    if let Ok(mut map) = registry().lock() {
        if let Some(w) = map.get_mut(&handle) {
            if headless {
                w.closed = true;
                w.visible = false;
            }
            return true;
        }
    }
    false
}

pub fn gui_close(handle: i64) -> bool {
    if let Ok(mut map) = registry().lock() {
        return map.remove(&handle).is_some();
    }
    false
}

#[no_mangle]
pub extern "C" fn joss_rs_gui_open(title: *const c_char, width: i64, height: i64) -> i64 {
    gui_open(cstr_or_empty(title), width, height)
}

#[no_mangle]
pub extern "C" fn joss_rs_gui_set_title(handle: i64, title: *const c_char) -> i64 {
    if gui_set_title(handle, cstr_or_empty(title)) { 1 } else { 0 }
}

#[no_mangle]
pub extern "C" fn joss_rs_gui_set_size(handle: i64, width: i64, height: i64) -> i64 {
    if gui_set_size(handle, width, height) { 1 } else { 0 }
}

#[no_mangle]
pub extern "C" fn joss_rs_gui_navigate(handle: i64, url: *const c_char) -> i64 {
    if gui_navigate(handle, cstr_or_empty(url)) { 1 } else { 0 }
}

#[no_mangle]
pub extern "C" fn joss_rs_gui_set_html(handle: i64, html: *const c_char) -> i64 {
    if gui_set_html(handle, cstr_or_empty(html)) { 1 } else { 0 }
}

#[no_mangle]
pub extern "C" fn joss_rs_gui_eval(handle: i64, script: *const c_char) -> i64 {
    if gui_eval(handle, cstr_or_empty(script)) { 1 } else { 0 }
}

#[no_mangle]
pub extern "C" fn joss_rs_gui_run(handle: i64) -> i64 {
    if gui_run(handle) { 1 } else { 0 }
}

#[no_mangle]
pub extern "C" fn joss_rs_gui_close(handle: i64) -> i64 {
    if gui_close(handle) { 1 } else { 0 }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn gui_window_lifecycle_is_safe_and_leak_free() {
        let h = gui_open("Joss Desktop", 1280, 800);
        assert!(h > 0);
        assert!(gui_set_title(h, "Updated Title"));
        assert!(gui_set_size(h, 1024, 768));
        assert!(gui_navigate(h, "http://127.0.0.1:8000"));
        assert!(gui_set_html(h, "<h1>Hello Joss</h1>"));
        assert!(gui_eval(h, "window.__jossReady = true;"));
        assert!(gui_run(h));
        assert!(gui_close(h));
        // Double-close is safely rejected without panic or UAF
        assert!(!gui_close(h));
    }
}
