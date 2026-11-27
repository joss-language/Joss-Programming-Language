//! Cross-platform process execution (`System::run`) in pure Rust.
//!
//! Replaces POSIX `popen` / Windows `CreatePipe + CreateProcessA` boilerplate
//! in `runtime/native/joss_services_rt.inc` with `std::process::Command`.

use std::ffi::{c_char, CStr};
use std::process::Command;

pub fn run_shell_command(cmd: &str) -> Option<Vec<u8>> {
    if cmd.trim().is_empty() {
        return None;
    }
    #[cfg(target_os = "windows")]
    let output = {
        use std::os::windows::process::CommandExt;
        const CREATE_NO_WINDOW: u32 = 0x0800_0000;
        let trimmed = cmd.trim_start();
        let has_shell_ops = trimmed.bytes().any(|b| {
            matches!(b, b'|' | b'&' | b'>' | b'<' | b';' | b'%' | b'\n' | b'\r')
        });
        if let Some(rest) = trimmed.strip_prefix("curl ").filter(|_| !has_shell_ops) {
            Command::new("curl.exe")
                .raw_arg(rest)
                .creation_flags(CREATE_NO_WINDOW)
                .output()
                .ok()?
        } else {
            Command::new("cmd.exe")
                .raw_arg("/C")
                .raw_arg(cmd)
                .creation_flags(CREATE_NO_WINDOW)
                .output()
                .ok()?
        }
    };
    #[cfg(not(target_os = "windows"))]
    let output = Command::new("/bin/sh").args(["-c", cmd]).output().ok()?;
    Some(output.stdout)
}

/// Executes a shell command on Windows/Linux/macOS (`x86_64`/`aarch64`) and returns
/// a Rust-allocated buffer of the exact stdout size (plus NUL terminator).
/// Caller MUST release it with `joss_rs_bytes_free(ptr, len)`.
///
/// # Safety
/// `cmd` must be a valid NUL-terminated C string; `out_len` may be null.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_system_run_alloc(
    cmd: *const c_char,
    out_len: *mut usize,
) -> *mut u8 {
    if !out_len.is_null() {
        *out_len = 0;
    }
    if cmd.is_null() {
        return std::ptr::null_mut();
    }
    let Ok(cmd_str) = CStr::from_ptr(cmd).to_str() else {
        return std::ptr::null_mut();
    };
    let Some(mut bytes) = run_shell_command(cmd_str) else {
        return std::ptr::null_mut();
    };
    let len = bytes.len();
    bytes.push(0);
    let mut boxed = bytes.into_boxed_slice();
    let ptr = boxed.as_mut_ptr();
    std::mem::forget(boxed);
    if !out_len.is_null() {
        *out_len = len;
    }
    ptr
}

/// Frees a buffer previously returned by `joss_rs_system_run_alloc`.
///
/// # Safety
/// `ptr` must have been returned by `joss_rs_system_run_alloc` with `len`.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_bytes_free(ptr: *mut u8, len: usize) {
    if ptr.is_null() {
        return;
    }
    let slice = std::ptr::slice_from_raw_parts_mut(ptr, len + 1);
    drop(Box::from_raw(slice));
}

/// Executes a shell command and copies stdout into `out_buf` (NUL-terminated).
/// Returns bytes written (or `-1` on error/truncation).
///
/// # Safety
/// `cmd` must be a valid NUL-terminated C string; `out_buf` must hold `out_cap` bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_system_run(
    cmd: *const c_char,
    out_buf: *mut u8,
    out_cap: usize,
) -> isize {
    if cmd.is_null() || out_buf.is_null() || out_cap == 0 {
        return -1;
    }
    let Ok(cmd_str) = CStr::from_ptr(cmd).to_str() else {
        return -1;
    };
    let Some(bytes) = run_shell_command(cmd_str) else {
        return -1;
    };
    let copy_len = bytes.len().min(out_cap - 1);
    if copy_len > 0 {
        std::ptr::copy_nonoverlapping(bytes.as_ptr(), out_buf, copy_len);
    }
    *out_buf.add(copy_len) = 0;
    copy_len as isize
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn system_run_captures_stdout() {
        let out = run_shell_command("echo JOSS_RUST_OK").expect("cmd failed");
        let text = String::from_utf8_lossy(&out);
        assert!(text.contains("JOSS_RUST_OK"), "unexpected output: {text}");
    }

    #[test]
    fn system_run_preserves_quoted_arguments() {
        let out = run_shell_command(r#"echo "OCI_HEADER: Signature keyId=test""#).expect("cmd failed");
        let text = String::from_utf8_lossy(&out);
        assert!(text.contains("OCI_HEADER: Signature keyId=test"), "quoted args mangled: {text}");
    }
}
