//! Cross-platform OS memory diagnostics & heap trimming (Linux, macOS, Windows).
//!
//! Replaces `runtime/native/joss_memory_rt.inc` OS primitives with safe Rust.

#[cfg(target_os = "windows")]
mod os {
    use std::ffi::c_void;

    #[repr(C)]
    #[derive(Default)]
    struct ProcessMemoryCounters {
        cb: u32,
        page_fault_count: u32,
        peak_working_set_size: usize,
        working_set_size: usize,
        quota_peak_paged_pool_usage: usize,
        quota_paged_pool_usage: usize,
        quota_peak_non_paged_pool_usage: usize,
        quota_non_paged_pool_usage: usize,
        pagefile_usage: usize,
        peak_pagefile_usage: usize,
    }

    extern "system" {
        fn GetCurrentProcess() -> *mut c_void;
        fn GetProcessHeap() -> *mut c_void;
        fn HeapCompact(h_heap: *mut c_void, dw_flags: u32) -> usize;
        fn K32GetProcessMemoryInfo(
            process: *mut c_void,
            ppsmem_counters: *mut ProcessMemoryCounters,
            cb: u32,
        ) -> i32;
    }

    pub fn rss_bytes() -> i64 {
        let mut pmc = ProcessMemoryCounters {
            cb: std::mem::size_of::<ProcessMemoryCounters>() as u32,
            ..Default::default()
        };
        let ok = unsafe { K32GetProcessMemoryInfo(GetCurrentProcess(), &mut pmc, pmc.cb) };
        if ok != 0 {
            pmc.working_set_size as i64
        } else {
            0
        }
    }

    pub fn trim_heap() -> i64 {
        let compacted = unsafe { HeapCompact(GetProcessHeap(), 0) };
        if compacted > 0 { 1 } else { 0 }
    }
}

#[cfg(target_os = "linux")]
mod os {
    extern "C" {
        fn sysconf(name: i32) -> i64;
        #[cfg(target_env = "gnu")]
        fn malloc_trim(pad: usize) -> i32;
    }

    const _SC_PAGESIZE: i32 = 30;

    pub fn rss_bytes() -> i64 {
        let Ok(content) = std::fs::read_to_string("/proc/self/statm") else {
            return 0;
        };
        let mut parts = content.split_whitespace();
        let _size = parts.next();
        let Some(rss_str) = parts.next() else {
            return 0;
        };
        let Ok(rss_pages) = rss_str.parse::<u64>() else {
            return 0;
        };
        let page_sz = unsafe { sysconf(_SC_PAGESIZE) };
        let page_sz = if page_sz > 0 { page_sz as u64 } else { 4096 };
        (rss_pages.saturating_mul(page_sz)) as i64
    }

    pub fn trim_heap() -> i64 {
        #[cfg(target_env = "gnu")]
        {
            if unsafe { malloc_trim(0) } != 0 { 1 } else { 0 }
        }
        #[cfg(not(target_env = "gnu"))]
        {
            0
        }
    }
}

#[cfg(target_os = "macos")]
mod os {
    use std::ffi::c_void;

    #[repr(C)]
    #[derive(Default)]
    struct Timeval {
        tv_sec: i64,
        tv_usec: i32,
        _pad: i32,
    }

    #[repr(C)]
    #[derive(Default)]
    struct Rusage {
        ru_utime: Timeval,
        ru_stime: Timeval,
        ru_maxrss: i64,
        _rest: [i64; 13],
    }

    const RUSAGE_SELF: i32 = 0;

    extern "C" {
        fn getrusage(who: i32, usage: *mut Rusage) -> i32;
        fn malloc_zone_pressure_relief(zone: *mut c_void, goal: usize) -> usize;
    }

    pub fn rss_bytes() -> i64 {
        let mut usage = Rusage::default();
        if unsafe { getrusage(RUSAGE_SELF, &mut usage) } == 0 && usage.ru_maxrss > 0 {
            usage.ru_maxrss
        } else {
            0
        }
    }

    pub fn trim_heap() -> i64 {
        unsafe {
            malloc_zone_pressure_relief(std::ptr::null_mut(), 0);
        }
        1
    }
}

#[cfg(not(any(target_os = "windows", target_os = "linux", target_os = "macos")))]
mod os {
    pub fn rss_bytes() -> i64 {
        0
    }
    pub fn trim_heap() -> i64 {
        0
    }
}

#[no_mangle]
pub extern "C" fn joss_rs_memory_rss() -> i64 {
    os::rss_bytes()
}

#[no_mangle]
pub extern "C" fn joss_rs_memory_trim() -> i64 {
    os::trim_heap()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rss_reports_positive_memory() {
        let rss = joss_rs_memory_rss();
        assert!(rss > 0, "expected positive RSS bytes, got {rss}");
    }

    #[test]
    fn trim_does_not_panic() {
        let _ = joss_rs_memory_trim();
    }
}
