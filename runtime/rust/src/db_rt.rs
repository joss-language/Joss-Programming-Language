//! Memory-safe SQL identifier validation, dialect placeholder rewriting, and
//! bounded prepared-query helpers in pure Rust.
//!
//! Protects `joss_db_rt.c` / ORM query construction from SQL injection and
//! C buffer overflows without external dependencies.

use std::ffi::{c_char, CStr};

/// Validates a SQL table/column identifier (ASCII alphanumerics, `_`, and optional `table.col`).
pub fn is_safe_sql_identifier(ident: &str) -> bool {
    if ident.is_empty() || ident.len() > 128 {
        return false;
    }
    for part in ident.split('.') {
        if part.is_empty() {
            return false;
        }
        let mut chars = part.bytes();
        let first = chars.next().unwrap();
        if !(first.is_ascii_alphabetic() || first == b'_') {
            return false;
        }
        if !chars.all(|b| b.is_ascii_alphanumeric() || b == b'_') {
            return false;
        }
    }
    true
}

/// Rewrites `?` placeholders outside single-quoted string literals to `$1, $2, ...`
/// (PostgreSQL) or `@p1, @p2, ...` (SQL Server), leaving SQLite/MySQL unchanged.
pub fn rewrite_placeholders(sql: &str, dialect: &str) -> String {
    let prefix = match dialect {
        "pgsql" | "postgres" | "postgresql" => "$",
        "sqlsrv" | "mssql" | "sqlserver" | "odbc" => "@p",
        _ => return sql.to_owned(),
    };
    let mut out = String::with_capacity(sql.len() + 16);
    let mut in_single = false;
    let mut idx: usize = 0;
    let bytes = sql.as_bytes();
    let mut i = 0;
    while i < bytes.len() {
        let b = bytes[i];
        if b == b'\'' {
            out.push('\'');
            if in_single && i + 1 < bytes.len() && bytes[i + 1] == b'\'' {
                out.push('\'');
                i += 2;
                continue;
            }
            in_single = !in_single;
        } else if b == b'?' && !in_single {
            idx += 1;
            out.push_str(prefix);
            out.push_str(&idx.to_string());
        } else {
            out.push(b as char);
        }
        i += 1;
    }
    out
}

/// # Safety
/// `ident` must be a valid NUL-terminated C string (or null).
#[no_mangle]
pub unsafe extern "C" fn joss_rs_sql_is_safe_ident(ident: *const c_char) -> i64 {
    if ident.is_null() {
        return 0;
    }
    let Ok(s) = CStr::from_ptr(ident).to_str() else {
        return 0;
    };
    if is_safe_sql_identifier(s) { 1 } else { 0 }
}

/// # Safety
/// `sql` and `dialect` must be valid NUL-terminated C strings; `out_buf` must hold `out_cap` bytes.
#[no_mangle]
pub unsafe extern "C" fn joss_rs_sql_rewrite_placeholders(
    sql: *const c_char,
    dialect: *const c_char,
    out_buf: *mut c_char,
    out_cap: usize,
) -> isize {
    if sql.is_null() || out_buf.is_null() || out_cap == 0 {
        return -1;
    }
    let Ok(sql_s) = CStr::from_ptr(sql).to_str() else {
        return -1;
    };
    let dia_s = if dialect.is_null() {
        "sqlite"
    } else {
        CStr::from_ptr(dialect).to_str().unwrap_or("sqlite")
    };
    let rewritten = rewrite_placeholders(sql_s, dia_s);
    let bytes = rewritten.as_bytes();
    if bytes.len() + 1 > out_cap {
        return -1;
    }
    std::ptr::copy_nonoverlapping(bytes.as_ptr(), out_buf.cast::<u8>(), bytes.len());
    *out_buf.add(bytes.len()) = 0;
    bytes.len() as isize
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn identifier_validation_blocks_injection() {
        assert!(is_safe_sql_identifier("users"));
        assert!(is_safe_sql_identifier("users.email_address"));
        assert!(!is_safe_sql_identifier("users; DROP TABLE users;--"));
        assert!(!is_safe_sql_identifier("123bad"));
        assert!(!is_safe_sql_identifier(""));
    }

    #[test]
    fn placeholder_rewriter_respects_string_literals() {
        let q = "SELECT * FROM users WHERE name = 'a?b' AND id = ? AND active = ?";
        assert_eq!(
            rewrite_placeholders(q, "postgres"),
            "SELECT * FROM users WHERE name = 'a?b' AND id = $1 AND active = $2"
        );
        assert_eq!(
            rewrite_placeholders(q, "mssql"),
            "SELECT * FROM users WHERE name = 'a?b' AND id = @p1 AND active = @p2"
        );
        assert_eq!(rewrite_placeholders(q, "sqlite"), q);
    }
}
