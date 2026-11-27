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
    let output = Command::new("cmd.exe").args(["/C", cmd]).output().ok()?;
    #[cfg(not(target_os = "windows"))]
    let output = Command::new("/bin/sh").args(["-c", cmd]).output().ok()?;
    Some(output.stdout)
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
}
