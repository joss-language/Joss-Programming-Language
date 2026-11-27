//! Memory-safe 5-field RFC/Vixie Cron expression matcher in pure Rust.

use std::ffi::{c_char, CStr};

fn match_field(field: &str, value: i64) -> bool {
    let clean = field.trim();
    if clean.is_empty() || clean == "*" {
        return true;
    }
    for part in clean.split(',') {
        let part = part.trim();
        let (range, step) = match part.split_once('/') {
            Some((r, s)) => (r, s.parse::<i64>().unwrap_or(1).max(1)),
            None => (part, 1),
        };
        if range == "*" {
            if value % step == 0 {
                return true;
            }
        } else if let Some((lo_s, hi_s)) = range.split_once('-') {
            if let (Ok(lo), Ok(hi)) = (lo_s.parse::<i64>(), hi_s.parse::<i64>()) {
                if value >= lo && value <= hi && (value - lo) % step == 0 {
                    return true;
                }
            }
        } else if let Ok(exact) = range.parse::<i64>() {
            if exact == value {
                return true;
            }
        }
    }
    false
}

pub fn matches_expr(expr: &str, minute: i64, hour: i64, day: i64, month: i64, dow: i64) -> bool {
    let fields: Vec<&str> = expr.split_whitespace().collect();
    if fields.len() < 5 {
        return false;
    }
    match_field(fields[0], minute)
        && match_field(fields[1], hour)
        && match_field(fields[2], day)
        && match_field(fields[3], month)
        && match_field(fields[4], dow)
}

/// Evaluates whether a 5-field cron expression matches the given timestamp components.
///
/// # Safety
/// `expr` must either be null or point to a valid NUL-terminated C string.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_cron_matches(
    expr: *const c_char,
    minute: i64,
    hour: i64,
    day: i64,
    month: i64,
    dow: i64,
) -> i64 {
    if expr.is_null() {
        return 0;
    }
    // SAFETY: Caller passes a valid NUL-terminated C string.
    let Ok(s) = unsafe { CStr::from_ptr(expr) }.to_str() else {
        return 0;
    };
    if matches_expr(s, minute, hour, day, month, dow) { 1 } else { 0 }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cron_expressions_match_accurately() {
        assert!(matches_expr("*/5 * * * *", 15, 10, 1, 1, 1));
        assert!(!matches_expr("*/5 * * * *", 14, 10, 1, 1, 1));
        assert!(matches_expr("0 4 * * 1", 0, 4, 20, 11, 1));
        assert!(!matches_expr("0 4 * * 1", 0, 4, 20, 11, 2));
        assert!(matches_expr("10-20/2 8,12 * * *", 14, 12, 5, 6, 3));
        assert!(!matches_expr("10-20/2 8,12 * * *", 15, 12, 5, 6, 3));
    }
}
