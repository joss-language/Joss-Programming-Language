/* =========================================================================
 * Zip::extract — 100% delegated to memory-safe Rust (`runtime/rust/src/zip_rt.rs`)
 * ========================================================================= */
#if defined(JOSS_HAS_RUST_RT)
extern int64_t joss_rs_zip_extract(const char* zip_path, const char* dest_dir);

int64_t fn_Zip_extract(const char* src_raw, const char* dest_raw) {
    const char* zip_path = unbox_web_cstr(src_raw);
    const char* dest_path = unbox_web_cstr(dest_raw);
    if (!zip_path || !*zip_path || !dest_path || !*dest_path) return 0;
    return joss_rs_zip_extract(zip_path, dest_path);
}
#else
int64_t fn_Zip_extract(const char* src_raw, const char* dest_raw) {
    (void)src_raw;
    (void)dest_raw;
    return 0;
}
#endif
