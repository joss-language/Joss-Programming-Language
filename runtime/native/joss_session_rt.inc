/* =========================================================================
 * Espiral 2: Binary Multipart Parser, Rate Limiter, Middlewares & Sessions
 * ========================================================================= */

static void joss_register_temp_upload(const char* path) {
    if (!path || !path[0]) return;
    if (g_tls_temp_upload_count < 64) {
        size_t n = strlen(path);
        char* copy = (char*)malloc(n + 1);
        if (copy) {
            memcpy(copy, path, n + 1);
            g_tls_temp_uploads[g_tls_temp_upload_count++] = copy;
        }
    }
}

static void joss_sanitize_upload_name(const char* in, char* out, size_t out_cap) {
    if (!out || out_cap == 0) return;
    size_t j = 0;
    if (in) {
        const char* base = in;
        for (const char* p = in; *p; p++) {
            if (*p == '/' || *p == '\\') base = p + 1;
        }
        for (const char* p = base; *p && j + 1 < out_cap; p++) {
            unsigned char c = (unsigned char)*p;
            if (isalnum(c) || c == '.' || c == '-' || c == '_') {
                out[j++] = (char)c;
            } else {
                out[j++] = '_';
            }
        }
    }
    if (j == 0) {
        snprintf(out, out_cap, "upload.bin");
    } else {
        out[j] = '\0';
    }
}

static bool joss_write_temp_upload(const uint8_t* data, size_t len, const char* orig_name, char* path_out, size_t path_cap) {
    static atomic_uint_fast64_t g_upload_seq = 0;
    uint64_t seq = atomic_fetch_add(&g_upload_seq, 1ULL);
    char safe_name[64];
    joss_sanitize_upload_name(orig_name, safe_name, sizeof(safe_name));
    const char* tmpdir = NULL;
#if defined(_WIN32)
    char win_tmp[512];
    DWORD n = GetTempPathA((DWORD)sizeof(win_tmp), win_tmp);
    if (n > 0 && n < sizeof(win_tmp)) {
        tmpdir = win_tmp;
    } else {
        tmpdir = ".";
    }
    size_t tlen = strlen(tmpdir);
    const char* sep = (tlen > 0 && (tmpdir[tlen - 1] == '\\' || tmpdir[tlen - 1] == '/')) ? "" : "\\";
    snprintf(path_out, path_cap, "%s%sjoss_up_%llu_%llu_%s",
             tmpdir, sep, (unsigned long long)GetCurrentProcessId(), (unsigned long long)seq, safe_name);
#else
    tmpdir = getenv("TMPDIR");
    if (!tmpdir || !tmpdir[0]) tmpdir = "/tmp";
    snprintf(path_out, path_cap, "%s/joss_up_%llu_%llu_%s",
             tmpdir, (unsigned long long)getpid(), (unsigned long long)seq, safe_name);
#endif
    JOSS_AUTO_FILE FILE* f = fopen(path_out, "wb");
    if (!f) return false;
    if (len > 0 && fwrite(data, 1, len, f) != len) {
        remove(path_out);
        return false;
    }
    joss_register_temp_upload(path_out);
    return true;
}

static bool joss_extract_header_param(const char* hdr_line, size_t hdr_len, const char* param_name, char* out, size_t out_cap) {
    if (!hdr_line || !param_name || !out || out_cap == 0) return false;
    out[0] = '\0';
    size_t plen = strlen(param_name);
    for (size_t i = 0; i + plen + 1 < hdr_len; i++) {
        if (joss_strncasecmp_ascii(hdr_line + i, param_name, plen) == 0 && hdr_line[i + plen] == '=') {
            if (i > 0 && isalnum((unsigned char)hdr_line[i - 1])) continue;
            size_t pos = i + plen + 1;
            if (pos >= hdr_len) return false;
            size_t j = 0;
            if (hdr_line[pos] == '"') {
                pos++;
                while (pos < hdr_len && hdr_line[pos] != '"' && j + 1 < out_cap) {
                    if (hdr_line[pos] == '\\' && pos + 1 < hdr_len) pos++;
                    out[j++] = hdr_line[pos++];
                }
            } else {
                while (pos < hdr_len && hdr_line[pos] != ';' && hdr_line[pos] != '\r' && hdr_line[pos] != '\n' &&
                       !isspace((unsigned char)hdr_line[pos]) && j + 1 < out_cap) {
                    out[j++] = hdr_line[pos++];
                }
            }
            out[j] = '\0';
            return true;
        }
    }
    return false;
}

void joss___joss_http_parse_multipart(int64_t raw_val, void* req_map_ptr) {
    const void* raw_req = (const void*)(uintptr_t)raw_val;
    if (!raw_req || !req_map_ptr) return;
    void* req_map = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)req_map_ptr);
    if (!req_map || !joss_is_map_ptr((int64_t)(uintptr_t)req_map)) return;

    size_t raw_len = 0;
    const uint8_t* raw_bytes = joss_bytes_unwrap(raw_req, &raw_len);
    const char* raw_str = NULL;
    if (!raw_bytes) {
        raw_str = unbox_web_cstr((const char*)raw_req);
        if (!raw_str) return;
        raw_bytes = (const uint8_t*)raw_str;
        raw_len = strlen(raw_str);
    }

    const uint8_t* hdr_sep = joss_memmem_bytes(raw_bytes, raw_len, (const uint8_t*)"\r\n\r\n", 4);
    if (!hdr_sep) return;
    size_t hdr_len = (size_t)(hdr_sep - raw_bytes);
    const uint8_t* body = hdr_sep + 4;
    size_t body_len = raw_len - (size_t)(body - raw_bytes);

    char boundary[256] = {0};
    for (size_t pos = 0; pos < hdr_len; ) {
        const uint8_t* eol = joss_memmem_bytes(raw_bytes + pos, hdr_len - pos, (const uint8_t*)"\r\n", 2);
        size_t line_len = eol ? (size_t)(eol - (raw_bytes + pos)) : (hdr_len - pos);
        const char* line = (const char*)(raw_bytes + pos);
        if (line_len >= 13 && joss_strncasecmp_ascii(line, "content-type:", 13) == 0) {
            if (joss_extract_header_param(line + 13, line_len - 13, "boundary", boundary, sizeof(boundary))) {
                break;
            }
        }
        pos += line_len + 2;
    }
    if (!boundary[0]) return;

    char dash_boundary[264];
    int db_len_i = snprintf(dash_boundary, sizeof(dash_boundary), "--%s", boundary);
    if (db_len_i <= 2) return;
    size_t db_len = (size_t)db_len_i;

    void* files_map = NULL;
    if (joss_array_key_exists("_files", req_map)) {
        int64_t fv = joss_unbox_value(joss_map_get_i64(req_map, "_files"));
        if (fv && joss_is_map_ptr(fv)) files_map = (void*)(uintptr_t)fv;
    }
    if (!files_map) {
        files_map = joss_map_new();
        joss_map_set_i64(req_map, "_files", (int64_t)(uintptr_t)files_map);
    }

    const uint8_t* cur = joss_memmem_bytes(body, body_len, (const uint8_t*)dash_boundary, db_len);
    while (cur) {
        const uint8_t* after = cur + db_len;
        size_t rem = (size_t)((body + body_len) - after);
        if (rem >= 2 && after[0] == '-' && after[1] == '-') break; /* --boundary-- terminator */
        if (rem >= 2 && after[0] == '\r' && after[1] == '\n') {
            after += 2;
            rem -= 2;
        } else if (rem >= 1 && after[0] == '\n') {
            after += 1;
            rem -= 1;
        } else {
            break;
        }

        const uint8_t* part_sep = joss_memmem_bytes(after, rem, (const uint8_t*)"\r\n\r\n", 4);
        if (!part_sep) break;
        size_t phdr_len = (size_t)(part_sep - after);
        const uint8_t* pbody = part_sep + 4;
        size_t p_rem = (size_t)((body + body_len) - pbody);

        const uint8_t* next_b = joss_memmem_bytes(pbody, p_rem, (const uint8_t*)dash_boundary, db_len);
        if (!next_b) break;
        size_t pdata_len = (size_t)(next_b - pbody);
        if (pdata_len >= 2 && pbody[pdata_len - 2] == '\r' && pbody[pdata_len - 1] == '\n') {
            pdata_len -= 2;
        } else if (pdata_len >= 1 && pbody[pdata_len - 1] == '\n') {
            pdata_len -= 1;
        }

        char field_name[128] = {0};
        char file_name[256] = {0};
        bool has_filename = false;
        char part_ct[128] = "application/octet-stream";

        for (size_t p = 0; p < phdr_len; ) {
            const uint8_t* peol = joss_memmem_bytes(after + p, phdr_len - p, (const uint8_t*)"\r\n", 2);
            size_t pl_len = peol ? (size_t)(peol - (after + p)) : (phdr_len - p);
            const char* pl = (const char*)(after + p);
            if (pl_len >= 20 && joss_strncasecmp_ascii(pl, "content-disposition:", 20) == 0) {
                joss_extract_header_param(pl + 20, pl_len - 20, "name", field_name, sizeof(field_name));
                has_filename = joss_extract_header_param(pl + 20, pl_len - 20, "filename", file_name, sizeof(file_name));
            } else if (pl_len >= 13 && joss_strncasecmp_ascii(pl, "content-type:", 13) == 0) {
                const char* v = pl + 13;
                size_t vl = pl_len - 13;
                while (vl > 0 && (*v == ' ' || *v == '\t')) {
                    v++;
                    vl--;
                }
                size_t cpy = vl < sizeof(part_ct) - 1 ? vl : sizeof(part_ct) - 1;
                memcpy(part_ct, v, cpy);
                part_ct[cpy] = '\0';
            }
            p += pl_len + 2;
        }

        if (field_name[0]) {
            if (has_filename) {
                char tmp_path[600] = {0};
                if (!joss_write_temp_upload(pbody, pdata_len, file_name, tmp_path, sizeof(tmp_path))) {
                    tmp_path[0] = '\0';
                }
                void* fentry = joss_map_new();
                joss_map_set_str(fentry, "name", joss_req_strdup(file_name));
                joss_map_set_str(fentry, "path", joss_req_strdup(tmp_path));
                joss_map_set_str(fentry, "tmp_name", joss_req_strdup(tmp_path));
                joss_map_set_str(fentry, "type", joss_req_strdup(part_ct));
                joss_map_set_i64(fentry, "size", (int64_t)pdata_len);
                joss_map_set_i64(fentry, "error", 0);
                if (memchr(pbody, '\0', pdata_len) != NULL) {
                    joss_map_set_i64(fentry, "content", (int64_t)(uintptr_t)joss_bytes_new(pbody, pdata_len));
                } else {
                    char* sc = (char*)joss_req_alloc(pdata_len + 1);
                    if (sc) {
                        memcpy(sc, pbody, pdata_len);
                        sc[pdata_len] = '\0';
                        joss_map_set_str(fentry, "content", sc);
                    }
                }
                joss_map_set_i64(files_map, field_name, (int64_t)(uintptr_t)fentry);
            } else {
                char* val_copy = (char*)joss_req_alloc(pdata_len + 1);
                if (val_copy) {
                    memcpy(val_copy, pbody, pdata_len);
                    val_copy[pdata_len] = '\0';
                    joss_map_set_str(req_map, field_name, val_copy);
                }
            }
        }
        cur = next_b;
    }
}

#if defined(JOSS_HAS_RUST_RT)
extern int64_t joss_rs_rate_limit_check(const char* ip, int64_t max_req, int64_t window_sec);
#else
#define JOSS_RATE_BUCKETS 1024
typedef struct {
    char    ip[192];
    int64_t window_start;
    int64_t count;
    bool    used;
} JossRateEntry;

static JossRateEntry g_rate_entries[JOSS_RATE_BUCKETS];
#endif

int64_t joss___joss_rate_limit_check(const char* ip, int64_t max_req, int64_t window_sec) {
    ip = unbox_web_cstr(ip);
    if (!ip || !ip[0]) ip = "127.0.0.1";
    if (max_req <= 0) max_req = 60;
    if (window_sec <= 0) window_sec = 60;
#if defined(JOSS_HAS_RUST_RT)
    return joss_rs_rate_limit_check(ip, max_req, window_sec);
#else
    int64_t now = (int64_t)time(NULL);

    joss_global_lock(JOSS_LOCK_RATELIMIT);
    int empty_idx = -1;
    int oldest_idx = 0;
    int match_idx = -1;
    for (int i = 0; i < JOSS_RATE_BUCKETS; i++) {
        if (!g_rate_entries[i].used) {
            if (empty_idx < 0) empty_idx = i;
            continue;
        }
        if (strcmp(g_rate_entries[i].ip, ip) == 0) {
            match_idx = i;
            break;
        }
        if (g_rate_entries[i].window_start < g_rate_entries[oldest_idx].window_start) {
            oldest_idx = i;
        }
    }
    if (match_idx < 0) {
        match_idx = (empty_idx >= 0) ? empty_idx : oldest_idx;
        memset(&g_rate_entries[match_idx], 0, sizeof(JossRateEntry));
        snprintf(g_rate_entries[match_idx].ip, sizeof(g_rate_entries[match_idx].ip), "%s", ip);
        g_rate_entries[match_idx].used = true;
        g_rate_entries[match_idx].window_start = now;
        g_rate_entries[match_idx].count = 0;
    } else if (now - g_rate_entries[match_idx].window_start > window_sec) {
        g_rate_entries[match_idx].window_start = now;
        g_rate_entries[match_idx].count = 0;
    }
    g_rate_entries[match_idx].count++;
    int64_t cur_count = g_rate_entries[match_idx].count;
    joss_global_unlock(JOSS_LOCK_RATELIMIT);

    if (cur_count > max_req) {
        return -window_sec;
    }
    return max_req - cur_count;
#endif
}

/* Minimal recursive-descent C11 JSON parser (used for JWT claims & file sessions) */
static const char* joss_json_skip_ws(const char* p) {
    while (p && (*p == ' ' || *p == '\t' || *p == '\r' || *p == '\n')) p++;
    return p;
}

static char* joss_json_parse_string_val(const char** pp, bool req_alloc) {
    const char* p = joss_json_skip_ws(*pp);
    if (!p || *p != '"') return NULL;
    p++;
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    while (*p && *p != '"') {
        if (*p == '\\' && p[1]) {
            p++;
            switch (*p) {
            case '"': joss_buf_append_byte(sb, '"'); break;
            case '\\': joss_buf_append_byte(sb, '\\'); break;
            case '/': joss_buf_append_byte(sb, '/'); break;
            case 'b': joss_buf_append_byte(sb, '\b'); break;
            case 'f': joss_buf_append_byte(sb, '\f'); break;
            case 'n': joss_buf_append_byte(sb, '\n'); break;
            case 'r': joss_buf_append_byte(sb, '\r'); break;
            case 't': joss_buf_append_byte(sb, '\t'); break;
            default: joss_buf_append_byte(sb, (uint8_t)*p); break;
            }
            p++;
        } else {
            joss_buf_append_byte(sb, (uint8_t)*p++);
        }
    }
    if (*p == '"') p++;
    *pp = p;
    char* res = req_alloc ? joss_sb_to_req_string(sb) : joss_sb_to_string(sb);
    return res;
}

static int64_t joss_json_parse_any(const char** pp);

static void* joss_json_parse_object(const char** pp) {
    const char* p = joss_json_skip_ws(*pp);
    if (!p || *p != '{') return NULL;
    p++;
    void* m = joss_map_new();
    p = joss_json_skip_ws(p);
    while (*p && *p != '}') {
        char* key = joss_json_parse_string_val(&p, false);
        if (!key) break;
        p = joss_json_skip_ws(p);
        if (*p == ':') p++;
        int64_t val = joss_json_parse_any(&p);
        joss_map_set_i64(m, key, val);
        free(key);
        p = joss_json_skip_ws(p);
        if (*p == ',') {
            p = joss_json_skip_ws(p + 1);
        } else {
            break;
        }
    }
    if (*p == '}') p++;
    *pp = p;
    return m;
}

static void* joss_json_parse_array(const char** pp) {
    const char* p = joss_json_skip_ws(*pp);
    if (!p || *p != '[') return NULL;
    p++;
    void* arr = joss_arr_new(8);
    p = joss_json_skip_ws(p);
    while (*p && *p != ']') {
        int64_t val = joss_json_parse_any(&p);
        joss_arr_push_i64(arr, val);
        p = joss_json_skip_ws(p);
        if (*p == ',') {
            p = joss_json_skip_ws(p + 1);
        } else {
            break;
        }
    }
    if (*p == ']') p++;
    *pp = p;
    return arr;
}

static int64_t joss_json_parse_any(const char** pp) {
    const char* p = joss_json_skip_ws(*pp);
    if (!p || !*p) return 0;
    if (*p == '"') {
        *pp = p;
        return (int64_t)(uintptr_t)joss_json_parse_string_val(pp, true);
    }
    if (*p == '{') {
        *pp = p;
        return (int64_t)(uintptr_t)joss_json_parse_object(pp);
    }
    if (*p == '[') {
        *pp = p;
        return (int64_t)(uintptr_t)joss_json_parse_array(pp);
    }
    if (strncmp(p, "true", 4) == 0) {
        *pp = p + 4;
        return (int64_t)(uintptr_t)joss_box_bool(true);
    }
    if (strncmp(p, "false", 5) == 0) {
        *pp = p + 5;
        return (int64_t)(uintptr_t)joss_box_bool(false);
    }
    if (strncmp(p, "null", 4) == 0) {
        *pp = p + 4;
        return 0;
    }
    char* endptr = NULL;
    int is_float = 0;
    for (const char* q = p; *q && (isdigit((unsigned char)*q) || *q == '-' || *q == '+' || *q == '.' || *q == 'e' || *q == 'E'); q++) {
        if (*q == '.' || *q == 'e' || *q == 'E') is_float = 1;
    }
    if (is_float) {
        double d = strtod(p, &endptr);
        if (endptr > p) {
            *pp = endptr;
            return (int64_t)(uintptr_t)joss_box_f64(d);
        }
    } else {
        int64_t v = strtoll(p, &endptr, 10);
        if (endptr > p) {
            *pp = endptr;
            return (int64_t)(uintptr_t)joss_box_i64(v);
        }
    }
    *pp = p;
    return 0;
}

static bool joss_is_weak_jwt_secret(const char* sec) {
    if (!sec || !*sec) return true;
    while (*sec && isspace((unsigned char)*sec)) sec++;
    if (!*sec) return true;
    return strcmp(sec, "joss_default_secret_change_in_production") == 0 || strcmp(sec, "joss_default_secret") == 0;
}

static void* joss_web_verify_hs256_jwt(const char* token) {
    static const char B64URL_ALPHA[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
    if (!token || !token[0]) return NULL;
    const char* dot1 = strchr(token, '.');
    if (!dot1) return NULL;
    const char* dot2 = strchr(dot1 + 1, '.');
    if (!dot2 || strchr(dot2 + 1, '.') != NULL) return NULL;

    const char* secret = joss_env("JWT_SECRET", "");
    if (joss_is_weak_jwt_secret(secret)) {
        secret = joss_env("APP_KEY", "");
    }
    if (joss_is_weak_jwt_secret(secret)) {
        return NULL;
    }

    size_t h_len = (size_t)(dot1 - token);
    char hdr_buf[512];
    size_t hdr_dec_len = joss_prim_base64_decode_alphabet(token, h_len, B64URL_ALPHA, (uint8_t*)hdr_buf, sizeof(hdr_buf) - 1);
    hdr_buf[hdr_dec_len] = '\0';
    const char* hp = hdr_buf;
    void* hdr_map = joss_json_parse_object(&hp);
    if (!hdr_map) return NULL;
    const char* alg = joss_map_get_str(hdr_map, "alg");
    if (!alg || strcmp(alg, "HS256") != 0) return NULL;

    size_t msg_len = (size_t)(dot2 - token);
    uint8_t mac[32];
    joss_prim_hmac_sha256((const uint8_t*)secret, strlen(secret), (const uint8_t*)token, msg_len, mac);
    char expected_sig[64];
    size_t exp_sig_len = joss_prim_base64_encode_alphabet(mac, 32, B64URL_ALPHA, false, expected_sig);
    const char* given_sig = dot2 + 1;
    size_t given_len = strlen(given_sig);
    if (given_len != exp_sig_len) return NULL;
    uint8_t diff = 0;
    for (size_t i = 0; i < exp_sig_len; i++) {
        diff |= (uint8_t)(expected_sig[i] ^ given_sig[i]);
    }
    if (diff != 0) return NULL;

    size_t p_b64_len = (size_t)(dot2 - (dot1 + 1));
    size_t p_cap = (p_b64_len * 3U) / 4U + 8U;
    char* p_json = (char*)malloc(p_cap);
    if (!p_json) return NULL;
    size_t p_len = joss_prim_base64_decode_alphabet(dot1 + 1, p_b64_len, B64URL_ALPHA, (uint8_t*)p_json, p_cap - 1);
    p_json[p_len] = '\0';
    const char* pp = p_json;
    void* claims = joss_json_parse_object(&pp);
    free(p_json);
    if (!claims) return NULL;

    if (joss_array_key_exists("exp", claims)) {
        int64_t exp = joss_intval(joss_map_get_i64(claims, "exp"));
        if (exp > 0 && (int64_t)time(NULL) > exp) {
            return NULL;
        }
    }
    const char* ttype = joss_map_get_str(claims, "token_type");
    if (ttype && strcmp(ttype, "mfa_challenge") == 0) {
        return NULL;
    }
    return claims;
}

static const char* joss_get_header_ci(void* headers_map, const char* name) {
    if (!headers_map || !name || !joss_is_map_ptr((int64_t)(uintptr_t)headers_map)) return "";
    if (joss_array_key_exists(name, headers_map)) {
        const char* v = joss_map_get_str(headers_map, name);
        if (v && v[0]) return v;
    }
    int64_t n = joss_map_len(headers_map);
    size_t nlen = strlen(name);
    for (int64_t i = 0; i < n; i++) {
        const char* k = joss_map_key_at(headers_map, i);
        if (k && strlen(k) == nlen && joss_strncasecmp_ascii(k, name, nlen) == 0) {
            return joss_strval(joss_map_val_at(headers_map, i));
        }
    }
    return "";
}

#define JOSS_MAX_CUSTOM_MW 128
typedef struct {
    char* name;
    int64_t closure_val;
} JossCustomMiddlewareEntry;

static JossCustomMiddlewareEntry g_joss_custom_mw[JOSS_MAX_CUSTOM_MW];
static int g_joss_custom_mw_count = 0;

int64_t joss_router_register_middleware(const char* name, int64_t closure_val) {
    name = unbox_web_cstr(name);
    if (!name || !name[0]) return 0;
    for (int i = 0; i < g_joss_custom_mw_count; i++) {
        if (g_joss_custom_mw[i].name && strcmp(g_joss_custom_mw[i].name, name) == 0) {
            g_joss_custom_mw[i].closure_val = closure_val;
            return 1;
        }
    }
    if (g_joss_custom_mw_count >= JOSS_MAX_CUSTOM_MW) return 0;
    g_joss_custom_mw[g_joss_custom_mw_count].name = joss_web_strdup(name);
    g_joss_custom_mw[g_joss_custom_mw_count].closure_val = closure_val;
    g_joss_custom_mw_count++;
    return 1;
}

const char* joss_router_list_custom_middlewares(void) {
    if (g_joss_custom_mw_count <= 0) return "";
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    for (int i = 0; i < g_joss_custom_mw_count; i++) {
        if (i > 0) joss_sb_append(sb, ", ");
        if (g_joss_custom_mw[i].name) joss_sb_append(sb, g_joss_custom_mw[i].name);
    }
    char* res = joss_sb_to_req_string(sb);
    return res ? res : "";
}

const char* joss___joss_http_check_middleware(const char* mw_spec, void* req_ptr, void* sess_ptr) {
    mw_spec = unbox_web_cstr(mw_spec);
    if (!mw_spec || !mw_spec[0]) return "";
    void* req_map = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)req_ptr);
    void* sess_map = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)sess_ptr);
    if (!sess_map || !joss_is_map_ptr((int64_t)(uintptr_t)sess_map)) sess_map = joss_map_new();

    const char* p = mw_spec;
    while (*p) {
        while (*p == '\n' || *p == '\r' || *p == ' ' || *p == '\t') p++;
        if (!*p) break;
        const char* sep = strchr(p, '\n');
        size_t tlen = sep ? (size_t)(sep - p) : strlen(p);
        while (tlen > 0 && (p[tlen - 1] == '\r' || p[tlen - 1] == ' ' || p[tlen - 1] == '\t')) tlen--;
        char mw[128];
        if (tlen >= sizeof(mw)) tlen = sizeof(mw) - 1;
        memcpy(mw, p, tlen);
        mw[tlen] = '\0';
        p += sep ? (size_t)(sep - p) + 1 : strlen(p);

        if (strcmp(mw, "auth") == 0) {
            int64_t uid = joss_intval(joss_map_get_i64(sess_map, "user_id"));
            if (uid <= 0) {
                return (const char*)with((void*)joss_redirect("/login", 302), "error", (int64_t)(uintptr_t)"Debes iniciar sesi\xC3\xB3n.");
            }
        } else if (strcmp(mw, "guest") == 0) {
            int64_t uid = joss_intval(joss_map_get_i64(sess_map, "user_id"));
            if (uid > 0) {
                return joss_redirect("/dashboard", 302);
            }
        } else if (strcmp(mw, "admin") == 0) {
            const char* role = joss_map_get_str(sess_map, "user_role");
            if (!role || strcmp(role, "admin") != 0) {
                return (const char*)with((void*)joss_redirect("/", 302), "error", (int64_t)(uintptr_t)"Acceso denegado.");
            }
        } else if (strcmp(mw, "auth_api") == 0) {
            const char* auth_hdr = "";
            if (req_map && joss_is_map_ptr((int64_t)(uintptr_t)req_map)) {
                int64_t hv = joss_unbox_value(joss_map_get_i64(req_map, "_headers"));
                if (hv && joss_is_map_ptr(hv)) {
                    auth_hdr = joss_get_header_ci((void*)(uintptr_t)hv, "authorization");
                }
                if ((!auth_hdr || !auth_hdr[0]) && joss_array_key_exists("Authorization", req_map)) {
                    auth_hdr = joss_map_get_str(req_map, "Authorization");
                }
            }
            const char* tok = "";
            if (auth_hdr && joss_strncasecmp_ascii(auth_hdr, "Bearer ", 7) == 0) {
                tok = auth_hdr + 7;
            } else if (auth_hdr && auth_hdr[0]) {
                tok = auth_hdr;
            }
            if ((!tok || !tok[0]) && req_map && joss_is_map_ptr((int64_t)(uintptr_t)req_map)) {
                int64_t cv = joss_unbox_value(joss_map_get_i64(req_map, "_cookies"));
                if (cv && joss_is_map_ptr(cv) && joss_array_key_exists("joss_token", (void*)(uintptr_t)cv)) {
                    tok = joss_map_get_str((void*)(uintptr_t)cv, "joss_token");
                }
            }
            if (!tok || !tok[0]) {
                tok = joss_map_get_str(sess_map, "user_token");
            }
            if (!tok || !tok[0]) {
                return "HTTP/1.1 401 Unauthorized\r\nContent-Type: application/json; charset=utf-8\r\nConnection: close\r\n\r\n{\"error\":\"Unauthorized: Missing Token\"}";
            }
            void* claims = joss_web_verify_hs256_jwt(tok);
            if (!claims) {
                return "HTTP/1.1 401 Unauthorized\r\nContent-Type: application/json; charset=utf-8\r\nConnection: close\r\n\r\n{\"error\":\"Unauthorized: Invalid Token\"}";
            }
            if (joss_array_key_exists("user_id", claims)) {
                int64_t uid = joss_intval(joss_map_get_i64(claims, "user_id"));
                joss_map_set_i64(sess_map, "user_id", (int64_t)(uintptr_t)joss_box_i64(uid));
            } else if (joss_array_key_exists("sub", claims)) {
                int64_t uid = joss_intval(joss_map_get_i64(claims, "sub"));
                joss_map_set_i64(sess_map, "user_id", (int64_t)(uintptr_t)joss_box_i64(uid));
            }
            if (joss_array_key_exists("email", claims)) {
                joss_map_set_str(sess_map, "user_email", joss_req_strdup(joss_map_get_str(claims, "email")));
            }
            if (joss_array_key_exists("name", claims)) {
                joss_map_set_str(sess_map, "user_name", joss_req_strdup(joss_map_get_str(claims, "name")));
            }
            if (joss_array_key_exists("role", claims)) {
                joss_map_set_str(sess_map, "user_role", joss_req_strdup(joss_map_get_str(claims, "role")));
            }
        } else if (strncmp(mw, "throttle:", 9) == 0) {
            int64_t max_req = 60;
            int64_t win_sec = 60;
            const char* spec = mw + 9;
            max_req = strtoll(spec, NULL, 10);
            const char* c = strchr(spec, ',');
            if (c) win_sec = strtoll(c + 1, NULL, 10);
            if (max_req <= 0) max_req = 60;
            if (win_sec <= 0) win_sec = 60;
            const char* ip = "127.0.0.1";
            if (req_map && joss_is_map_ptr((int64_t)(uintptr_t)req_map)) {
                const char* rip = joss_map_get_str(req_map, "_ip");
                if (rip && rip[0]) ip = rip;
            }
            char rl_key[192];
            snprintf(rl_key, sizeof(rl_key), "%s|%s", mw, ip);
            int64_t rem = joss___joss_rate_limit_check(rl_key, max_req, win_sec);
            if (rem < 0) {
                char hdr[256];
                snprintf(hdr, sizeof(hdr),
                         "HTTP/1.1 429 Too Many Requests\r\nContent-Type: text/html; charset=utf-8\r\nRetry-After: %lld\r\nConnection: close\r\n\r\n<h1>429 Too Many Requests</h1>",
                         (long long)win_sec);
                return joss_req_strdup(hdr);
            }
        } else {
            for (int i = 0; i < g_joss_custom_mw_count; i++) {
                if (g_joss_custom_mw[i].name && strcmp(g_joss_custom_mw[i].name, mw) == 0) {
                    void* fn = joss_closure_prepare_call((void*)(uintptr_t)g_joss_custom_mw[i].closure_val);
                    if (fn) {
                        int64_t res = ((JossUnaryCb)fn)((int64_t)(uintptr_t)mw);
                        joss_closure_env_tls_reset();
                        if (joss_is_not_null(res)) {
                            const char* s = unbox_web_cstr((const char*)(uintptr_t)res);
                            if (s && s[0]) {
                                return s;
                            }
                        }
                    }
                    break;
                }
            }
        }
    }
    return "";
}

static void* g_joss_session_mem_store = NULL;
static void* g_joss_session_owners    = NULL;
static void* g_joss_session_touched   = NULL;

static void joss_session_store_init_locked(void) {
    bool _prev = joss_req_arena_pause();
    if (!g_joss_session_mem_store) {
        g_joss_session_mem_store = joss_map_new();
        joss_gc_add_root(&g_joss_session_mem_store);
    }
    if (!g_joss_session_owners) {
        g_joss_session_owners = joss_map_new();
        joss_gc_add_root(&g_joss_session_owners);
    }
    if (!g_joss_session_touched) {
        g_joss_session_touched = joss_map_new();
        joss_gc_add_root(&g_joss_session_touched);
    }
    joss_req_arena_resume(_prev);
}

static void joss_session_remove_locked(const char* sid);

static void joss_session_evict_if_needed_locked(void) {
    if (!g_joss_session_mem_store || !g_joss_session_touched) return;
    int64_t max_entries = strtoll(joss_env("SESSION_MAX_ENTRIES", "2048"), NULL, 10);
    if (max_entries < 64) max_entries = 64;
    int64_t cur_len = joss_map_len(g_joss_session_mem_store);
    if (cur_len < max_entries) return;

    int64_t ttl = strtoll(joss_env("SESSION_TTL_SECONDS", "86400"), NULL, 10);
    if (ttl <= 0) ttl = 86400;
    int64_t now = (int64_t)time(NULL);

    /* Evict up to 25% oldest or expired entries in one sweep */
    int64_t target_evict = (cur_len - max_entries) + (max_entries / 4);
    if (target_evict < 16) target_evict = 16;
    for (int64_t e = 0; e < target_evict; e++) {
        int64_t n = joss_map_len(g_joss_session_mem_store);
        if (n <= 0) break;
        const char* oldest_sid = NULL;
        int64_t oldest_ts = INT64_MAX;
        for (int64_t i = 0; i < n; i++) {
            const char* k = joss_map_key_at(g_joss_session_mem_store, i);
            if (!k || !k[0]) continue;
            int64_t ts = joss_unbox_value(joss_map_get_i64(g_joss_session_touched, k));
            if (ts <= 0 || (now - ts) > ttl) {
                oldest_sid = k;
                break;
            }
            if (ts < oldest_ts) {
                oldest_ts = ts;
                oldest_sid = k;
            }
        }
        if (!oldest_sid) break;
        char evict_copy[256];
        snprintf(evict_copy, sizeof(evict_copy), "%s", oldest_sid);
        joss_session_remove_locked(evict_copy);
    }
}

static void joss_session_replace_locked(const char* sid, void* copy, JossOwnedScope* owner) {
    joss_session_store_init_locked();
    bool _prev = joss_req_arena_pause();
    if (!joss_array_key_exists(sid, g_joss_session_mem_store)) {
        joss_session_evict_if_needed_locked();
    }
    if (joss_array_key_exists(sid, g_joss_session_owners)) {
        JossOwnedScope* old_owner = (JossOwnedScope*)(uintptr_t)joss_map_get_i64(g_joss_session_owners, sid);
        if (old_owner && old_owner != owner) joss_owned_scope_destroy(old_owner);
    }
    joss_map_set_i64(g_joss_session_mem_store, sid, (int64_t)(uintptr_t)copy);
    joss_map_set_i64(g_joss_session_touched, sid, (int64_t)time(NULL));
    if (owner) {
        joss_map_set_i64(g_joss_session_owners, sid, (int64_t)(uintptr_t)owner);
    } else {
        joss_map_delete_i64(g_joss_session_owners, sid);
    }
    joss_req_arena_resume(_prev);
}

static void joss_session_remove_locked(const char* sid) {
    if (g_joss_session_owners && joss_array_key_exists(sid, g_joss_session_owners)) {
        JossOwnedScope* old_owner = (JossOwnedScope*)(uintptr_t)joss_map_get_i64(g_joss_session_owners, sid);
        joss_map_delete_i64(g_joss_session_owners, sid);
        if (old_owner) joss_owned_scope_destroy(old_owner);
    }
    if (g_joss_session_touched) {
        joss_map_delete_i64(g_joss_session_touched, sid);
    }
    if (g_joss_session_mem_store) {
        joss_map_delete_i64(g_joss_session_mem_store, sid);
    }
}

static char* joss_clone_session_string(const char* s) {
    if (joss_req_arena_is_active()) {
        return joss_req_strdup(s ? s : "");
    }
    return joss_web_strdup(s ? s : ""); /* joss-mem: persistent-ok */
}

static int64_t joss_clone_session_value(int64_t v);

static void* joss_clone_flat_session_map(void* src) {
    void* dst = joss_map_new();
    if (!src || !joss_is_map_ptr((int64_t)(uintptr_t)src)) {
        return dst;
    }
    int64_t n = joss_map_len(src);
    for (int64_t i = 0; i < n; i++) {
        const char* k = joss_map_key_at(src, i);
        int64_t v = joss_map_val_at(src, i);
        if (!k || !k[0]) continue;
        joss_map_set_i64(dst, k, joss_clone_session_value(v));
    }
    return dst;
}

static int64_t joss_clone_session_value(int64_t v) {
    if (v == 0) return 0;
    if (joss_is_boxed_ptr(v)) {
        const JossBoxedValue* b = (const JossBoxedValue*)(uintptr_t)(uint64_t)v;
        uint32_t tag = (uint32_t)(b->header & 0xFFFFFFFFULL);
        if (tag == JOSS_TAG_NULL) return 0;
        if (tag == JOSS_TAG_INT) return (int64_t)(uintptr_t)joss_box_i64((int64_t)b->payload);
        if (tag == JOSS_TAG_FLOAT) {
            double d = 0.0;
            memcpy(&d, &b->payload, sizeof(d));
            return (int64_t)(uintptr_t)joss_box_f64(d);
        }
        if (tag == JOSS_TAG_BOOL) return (int64_t)(uintptr_t)joss_box_bool(b->payload != 0);
        if (tag == JOSS_TAG_STRING) {
            const char* s = (const char*)(uintptr_t)b->payload;
            return (int64_t)(uintptr_t)joss_clone_session_string(s ? s : "");
        }
        return (int64_t)(uintptr_t)joss_clone_session_string(joss_strval(v));
    }
    if (joss_is_map_ptr(v)) {
        return (int64_t)(uintptr_t)joss_clone_flat_session_map((void*)(uintptr_t)v);
    }
    if (joss_is_arr_ptr(v)) {
        void* src_arr = (void*)(uintptr_t)v;
        int64_t len = joss_arr_len(src_arr);
        void* dst_arr = joss_arr_new(len > 0 ? len : 4);
        for (int64_t i = 0; i < len; i++) {
            joss_arr_push_i64(dst_arr, joss_clone_session_value(joss_arr_get_i64(src_arr, i)));
        }
        return (int64_t)(uintptr_t)dst_arr;
    }
    uint64_t u = (uint64_t)v;
    if (u < JOSS_MIN_PTR_ADDR || (u >> JOSS_PTR_VA_BITS) != 0) {
        return (int64_t)(uintptr_t)joss_box_i64(v);
    }
    return (int64_t)(uintptr_t)joss_clone_session_string(joss_strval(v));
}

int64_t joss_json_decode(const char* s) {
    s = unbox_web_cstr(s);
    if (!s || !*s) return 0;
    const char* p = s;
    return joss_json_parse_any(&p);
}

void* joss___joss_jwt_parse_verified(const char* token, const char* secret) {
    static const char B64URL_ALPHA[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
    token = unbox_web_cstr(token);
    secret = unbox_web_cstr(secret);
    if (!token || !token[0]) return NULL;
    if (!secret || !secret[0]) {
        secret = joss_env("JWT_SECRET", "");
        if (!secret || !secret[0]) secret = joss_env("APP_KEY", "");
        if (!secret || !secret[0]) secret = "joss_ephemeral_jwt_key";
    }
    const char* dot1 = strchr(token, '.');
    if (!dot1) return NULL;
    const char* dot2 = strchr(dot1 + 1, '.');
    if (!dot2 || strchr(dot2 + 1, '.') != NULL) return NULL;

    size_t h_len = (size_t)(dot1 - token);
    char hdr_buf[512];
    size_t hdr_dec_len = joss_prim_base64_decode_alphabet(token, h_len, B64URL_ALPHA, (uint8_t*)hdr_buf, sizeof(hdr_buf) - 1);
    hdr_buf[hdr_dec_len] = '\0';
    const char* hp = hdr_buf;
    void* hdr_map = joss_json_parse_object(&hp);
    if (!hdr_map) return NULL;
    const char* alg = joss_map_get_str(hdr_map, "alg");
    if (!alg || strcmp(alg, "HS256") != 0) return NULL;

    size_t msg_len = (size_t)(dot2 - token);
    uint8_t mac[32];
    joss_prim_hmac_sha256((const uint8_t*)secret, strlen(secret), (const uint8_t*)token, msg_len, mac);
    char expected_sig[64];
    size_t exp_sig_len = joss_prim_base64_encode_alphabet(mac, 32, B64URL_ALPHA, false, expected_sig);
    const char* given_sig = dot2 + 1;
    if (strlen(given_sig) != exp_sig_len) return NULL;
    uint8_t diff = 0;
    for (size_t i = 0; i < exp_sig_len; i++) {
        diff |= (uint8_t)(expected_sig[i] ^ given_sig[i]);
    }
    if (diff != 0) return NULL;

    size_t p_b64_len = (size_t)(dot2 - (dot1 + 1));
    size_t p_cap = (p_b64_len * 3U) / 4U + 8U;
    char* p_json = (char*)malloc(p_cap);
    if (!p_json) return NULL;
    size_t p_len = joss_prim_base64_decode_alphabet(dot1 + 1, p_b64_len, B64URL_ALPHA, (uint8_t*)p_json, p_cap - 1);
    p_json[p_len] = '\0';
    const char* pp = p_json;
    void* claims = joss_json_parse_object(&pp);
    free(p_json);
    if (!claims) return NULL;
    if (joss_array_key_exists("exp", claims)) {
        int64_t exp = joss_intval(joss_map_get_i64(claims, "exp"));
        if (exp > 0 && (int64_t)time(NULL) > exp) return NULL;
    }
    return claims;
}

int64_t joss___joss_http_inject_header(int64_t resp_val, const char* header_line) {
    header_line = unbox_web_cstr(header_line);
    if (!header_line || !header_line[0]) return resp_val;
    size_t bin_len = 0;
    const uint8_t* bin = joss_bytes_unwrap((const void*)(uintptr_t)resp_val, &bin_len);
    const uint8_t* data = bin;
    size_t total_len = bin_len;
    const char* cstr = NULL;
    if (!data) {
        cstr = unbox_web_cstr((const char*)(uintptr_t)resp_val);
        if (!cstr) return resp_val;
        data = (const uint8_t*)cstr;
        total_len = strlen(cstr);
    }
    if (total_len < 5 || memcmp(data, "HTTP/", 5) != 0) return resp_val;
    const uint8_t* sep = joss_memmem_bytes(data, total_len, (const uint8_t*)"\r\n\r\n", 4);
    if (!sep) return resp_val;
    size_t head_len = (size_t)(sep - data);
    if (joss_memmem_bytes(data, head_len, (const uint8_t*)"joss_session=", 13) != NULL) {
        return resp_val;
    }
    size_t hlen = strlen(header_line);
    size_t tail_len = total_len - head_len;
    size_t new_len = head_len + 2 + hlen + tail_len;
    uint8_t* buf = (uint8_t*)joss_req_alloc(new_len + 1);
    if (!buf) return resp_val;
    memcpy(buf, data, head_len);
    memcpy(buf + head_len, "\r\n", 2);
    memcpy(buf + head_len + 2, header_line, hlen);
    memcpy(buf + head_len + 2 + hlen, sep, tail_len);
    buf[new_len] = '\0';
    if (bin) {
        void* boxed = joss_bytes_new(buf, new_len);
        return (int64_t)(uintptr_t)boxed;
    }
    return (int64_t)(uintptr_t)buf;
}

static char g_joss_session_loaded_path[512] = {0};

static void joss_set_session_err(const char* msg) {
    snprintf(g_joss_session_last_err, sizeof(g_joss_session_last_err), "%s", msg ? msg : "session error");
}

static int joss_session_resolve_driver(void) {
    g_joss_session_last_err[0] = '\0';
    const char* raw = joss_env("SESSION_DRIVER", "file");
    if (!raw || !raw[0]) return 1; /* file */
    while (*raw && isspace((unsigned char)*raw)) raw++;
    char drv[64];
    size_t n = strlen(raw);
    while (n > 0 && isspace((unsigned char)raw[n - 1])) n--;
    if (n == 0) return 1;
    if (n >= sizeof(drv)) n = sizeof(drv) - 1;
    for (size_t i = 0; i < n; i++) drv[i] = (char)tolower((unsigned char)raw[i]);
    drv[n] = '\0';
    if (strcmp(drv, "memory") == 0) return 0;
    if (strcmp(drv, "file") == 0) return 1;
    if (strcmp(drv, "redis") == 0) return 2;
    char msg[160];
    snprintf(msg, sizeof(msg), "session driver \"%s\" is not supported", drv);
    joss_set_session_err(msg);
    return -1;
}

static void joss_mkdir_parents(const char* path) {
    if (!path) return;
    char tmp[512];
    snprintf(tmp, sizeof(tmp), "%s", path);
    for (char* p = tmp + 1; *p; p++) {
        if (*p == '/' || *p == '\\') {
            char saved = *p;
            *p = '\0';
#if defined(_WIN32)
            CreateDirectoryA(tmp, NULL);
#else
            mkdir(tmp, 0700);
#endif
            *p = saved;
        }
    }
}

static const char* joss_session_file_path(void) {
    const char* p = joss_env("SESSION_FILE", "");
    if (!p || !p[0]) return "storage/sessions.json";
    return p;
}

static bool joss_file_sessions_ensure_loaded_locked(const char* path) {
    if (strcmp(g_joss_session_loaded_path, path) == 0 && g_joss_session_mem_store) {
        return true;
    }
    bool _prev_arena = joss_req_arena_pause();
    JOSS_AUTO_FILE FILE* f = fopen(path, "rb");
    if (!f) {
        if (!g_joss_session_mem_store) g_joss_session_mem_store = joss_map_new();
        snprintf(g_joss_session_loaded_path, sizeof(g_joss_session_loaded_path), "%s", path);
        joss_req_arena_resume(_prev_arena);
        return true;
    }
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);
    if (sz <= 0) {
        g_joss_session_mem_store = joss_map_new();
        snprintf(g_joss_session_loaded_path, sizeof(g_joss_session_loaded_path), "%s", path);
        joss_req_arena_resume(_prev_arena);
        return true;
    }
    char* buf = (char*)malloc((size_t)sz + 1);
    if (!buf) {
        joss_set_session_err("out of memory reading session file");
        joss_req_arena_resume(_prev_arena);
        return false;
    }
    size_t rd = fread(buf, 1, (size_t)sz, f);
    buf[rd] = '\0';
    const char* p = joss_json_skip_ws(buf);
    if (!p || !*p) {
        free(buf);
        g_joss_session_mem_store = joss_map_new();
        snprintf(g_joss_session_loaded_path, sizeof(g_joss_session_loaded_path), "%s", path);
        joss_req_arena_resume(_prev_arena);
        return true;
    }
    if (*p != '{') {
        free(buf);
        joss_set_session_err("invalid JSON in session file");
        joss_req_arena_resume(_prev_arena);
        return false;
    }
    void* loaded = joss_json_parse_object(&p);
    free(buf);
    if (!loaded) {
        joss_set_session_err("failed to parse session file");
        joss_req_arena_resume(_prev_arena);
        return false;
    }
    g_joss_session_mem_store = loaded;
    snprintf(g_joss_session_loaded_path, sizeof(g_joss_session_loaded_path), "%s", path);
    joss_req_arena_resume(_prev_arena);
    return true;
}

static bool joss_file_sessions_persist_locked(const char* path) {
    static _Atomic(uint64_t) s_sess_tmp_seq = 0;
    joss_mkdir_parents(path);
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    json_encode_value(sb, (int64_t)(uintptr_t)(g_joss_session_mem_store ? g_joss_session_mem_store : joss_map_new()));
    uint64_t seq = atomic_fetch_add_explicit(&s_sess_tmp_seq, 1ULL, memory_order_relaxed);
    char tmp_path[560];
    snprintf(tmp_path, sizeof(tmp_path), "%s.%llu.%lu.tmp", path, (unsigned long long)seq, (unsigned long)time(NULL));
    {
        JOSS_AUTO_FILE FILE* f = fopen(tmp_path, "wb");
        if (!f) {
            joss_set_session_err("failed to open temp session file for write");
            return false;
        }
        size_t wr = fwrite(sb->data, 1, (size_t)sb->len, f);
        int flush_rc = fflush(f);
        if (wr != (size_t)sb->len || flush_rc != 0) {
            joss_auto_file_cleanup(&f);
            remove(tmp_path);
            joss_set_session_err("failed writing session file");
            return false;
        }
    }
#if defined(_WIN32)
    BOOL moved = FALSE;
    for (int attempt = 0; attempt < 8; attempt++) {
        if (MoveFileExA(tmp_path, path, MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH)) {
            moved = TRUE;
            break;
        }
        Sleep(5);
    }
    if (!moved) {
        remove(tmp_path);
        joss_set_session_err("failed replacing session file");
        return false;
    }
#else
    if (rename(tmp_path, path) != 0) {
        remove(tmp_path);
        joss_set_session_err("failed renaming session file");
        return false;
    }
#endif
    return true;
}

/* Minimal C11 TCP RESP2 Redis Client */
static bool g_redis_manual_set = false;
static char g_redis_manual_host[256] = "127.0.0.1";
static char g_redis_manual_port[32] = "6379";
static char g_redis_manual_pass[256] = "";
static int  g_redis_manual_db = 0;

static int joss_redis_connect_fd(void) {
    char host[256];
    char port[32];
    if (g_redis_manual_set) {
        snprintf(host, sizeof(host), "%s", g_redis_manual_host[0] ? g_redis_manual_host : "127.0.0.1");
        snprintf(port, sizeof(port), "%s", g_redis_manual_port[0] ? g_redis_manual_port : "6379");
    } else {
        const char* host_env = joss_env("REDIS_HOST", "127.0.0.1");
        const char* port_env = joss_env("REDIS_PORT", "6379");
        snprintf(host, sizeof(host), "%s", (host_env && host_env[0]) ? host_env : "127.0.0.1");
        snprintf(port, sizeof(port), "%s", (port_env && port_env[0]) ? port_env : "6379");
    }
    char* colon = strrchr(host, ':');
    if (colon && !strchr(host, ']')) {
        *colon = '\0';
        if (colon[1]) snprintf(port, sizeof(port), "%s", colon + 1);
    }
#if defined(_WIN32)
    static atomic_int wsa_init = 0;
    if (atomic_load(&wsa_init) == 0) {
        WSADATA wd;
        if (WSAStartup(MAKEWORD(2, 2), &wd) == 0) atomic_store(&wsa_init, 1);
    }
#endif
    struct addrinfo hints;
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;
    struct addrinfo* res = NULL;
    if (getaddrinfo(host, port, &hints, &res) != 0 || !res) {
        joss_set_session_err("redis host resolution failed");
        return -1;
    }
    int fd = -1;
    for (struct addrinfo* rp = res; rp; rp = rp->ai_next) {
#if defined(_WIN32)
        SOCKET s = socket(rp->ai_family, rp->ai_socktype, rp->ai_protocol);
        if (s == INVALID_SOCKET) continue;
        DWORD tv = 3000;
        setsockopt(s, SOL_SOCKET, SO_RCVTIMEO, (const char*)&tv, sizeof(tv));
        setsockopt(s, SOL_SOCKET, SO_SNDTIMEO, (const char*)&tv, sizeof(tv));
        if (connect(s, rp->ai_addr, (int)rp->ai_addrlen) == 0) {
            fd = (int)s;
            break;
        }
        closesocket(s);
#else
        int s = socket(rp->ai_family, rp->ai_socktype, rp->ai_protocol);
        if (s < 0) continue;
        struct timeval tv = {3, 0};
        setsockopt(s, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
        setsockopt(s, SOL_SOCKET, SO_SNDTIMEO, &tv, sizeof(tv));
        if (connect(s, rp->ai_addr, rp->ai_addrlen) == 0) {
            fd = s;
            break;
        }
        close(s);
#endif
    }
    freeaddrinfo(res);
    if (fd < 0) {
        joss_set_session_err("redis connection failed");
    }
    return fd;
}

static void joss_redis_close_fd(int fd) {
    if (fd < 0) return;
#if defined(_WIN32)
    closesocket((SOCKET)fd);
#else
    close(fd);
#endif
}

static bool joss_redis_send_all(int fd, const char* buf, size_t len) {
    size_t sent = 0;
    while (sent < len) {
#if defined(_WIN32)
        int n = send((SOCKET)fd, buf + sent, (int)(len - sent), 0);
#else
        ssize_t n = send(fd, buf + sent, len - sent, 0);
#endif
        if (n <= 0) return false;
        sent += (size_t)n;
    }
    return true;
}

static bool joss_redis_read_line(int fd, char* out, size_t cap) {
    size_t idx = 0;
    while (idx + 1 < cap) {
        char ch = 0;
#if defined(_WIN32)
        int n = recv((SOCKET)fd, &ch, 1, 0);
#else
        ssize_t n = recv(fd, &ch, 1, 0);
#endif
        if (n <= 0) return false;
        out[idx++] = ch;
        if (idx >= 2 && out[idx - 2] == '\r' && out[idx - 1] == '\n') {
            out[idx - 2] = '\0';
            return true;
        }
    }
    return false;
}

static bool joss_redis_send_cmd(int fd, int argc, const char** argv, const size_t* lens) {
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    joss_buf_append_byte(sb, '*');
    joss_sb_append_i64(sb, argc);
    joss_sb_append(sb, "\r\n");
    for (int i = 0; i < argc; i++) {
        joss_buf_append_byte(sb, '$');
        joss_sb_append_i64(sb, (int64_t)lens[i]);
        joss_sb_append(sb, "\r\n");
        joss_buf_append_bytes(sb, (const uint8_t*)argv[i], (int64_t)lens[i]);
        joss_sb_append(sb, "\r\n");
    }
    bool ok = joss_redis_send_all(fd, (const char*)sb->data, (size_t)sb->len);
    return ok;
}

static bool joss_redis_auth_and_select(int fd) {
    const char* pwd = g_redis_manual_set ? g_redis_manual_pass : joss_env("REDIS_PASSWORD", "");
    if (pwd && pwd[0]) {
        const char* av[2] = {"AUTH", pwd};
        size_t al[2] = {4, strlen(pwd)};
        char line[256];
        if (!joss_redis_send_cmd(fd, 2, av, al) || !joss_redis_read_line(fd, line, sizeof(line)) || line[0] != '+') {
            joss_set_session_err("redis AUTH failed");
            return false;
        }
    }
    int db_num = g_redis_manual_set ? g_redis_manual_db : atoi(joss_env("REDIS_DB", "0"));
    if (db_num > 0) {
        char db_buf[32];
        snprintf(db_buf, sizeof(db_buf), "%d", db_num);
        const char* av[2] = {"SELECT", db_buf};
        size_t al[2] = {6, strlen(db_buf)};
        char line[256];
        if (!joss_redis_send_cmd(fd, 2, av, al) || !joss_redis_read_line(fd, line, sizeof(line)) || line[0] != '+') {
            joss_set_session_err("redis SELECT failed");
            return false;
        }
    }
    return true;
}

/* Redis::* host service migrated 100% to std/system/redis_host.joss over Rust net_tcp.rs */

void* joss___joss_session_load(const char* sid) {
    sid = unbox_web_cstr(sid);
    int drv = joss_session_resolve_driver();
    if (drv < 0) return joss_map_new();
    void* out = NULL;
    if (drv == 0) {
        joss_global_lock(JOSS_LOCK_SESSION);
        if (g_joss_session_mem_store && sid && sid[0] && joss_array_key_exists(sid, g_joss_session_mem_store)) {
            int64_t stored = joss_map_get_i64(g_joss_session_mem_store, sid);
            if (stored && joss_is_map_ptr(stored)) {
                out = joss_clone_flat_session_map((void*)(uintptr_t)stored);
            }
        }
        joss_global_unlock(JOSS_LOCK_SESSION);
    } else if (drv == 1) {
        const char* path = joss_session_file_path();
        joss_global_lock(JOSS_LOCK_SESSION);
        if (joss_file_sessions_ensure_loaded_locked(path) && g_joss_session_mem_store && sid && sid[0] &&
            joss_array_key_exists(sid, g_joss_session_mem_store)) {
            int64_t stored = joss_map_get_i64(g_joss_session_mem_store, sid);
            if (stored && joss_is_map_ptr(stored)) {
                out = joss_clone_flat_session_map((void*)(uintptr_t)stored);
            }
        }
        joss_global_unlock(JOSS_LOCK_SESSION);
    } else if (drv == 2 && sid && sid[0]) {
        int fd = joss_redis_connect_fd();
        if (fd >= 0) {
            if (joss_redis_auth_and_select(fd)) {
                char rkey[320];
                snprintf(rkey, sizeof(rkey), "session:%s", sid);
                const char* av[2] = {"GET", rkey};
                size_t al[2] = {3, strlen(rkey)};
                char line[256];
                if (!joss_redis_send_cmd(fd, 2, av, al) || !joss_redis_read_line(fd, line, sizeof(line))) {
                    joss_set_session_err("redis GET failed");
                } else if (line[0] == '$') {
                    long blen = strtol(line + 1, NULL, 10);
                    if (blen >= 0) {
                        char* payload = (char*)malloc((size_t)blen + 3);
                        if (payload) {
                            size_t got = 0;
                            while (got < (size_t)blen + 2) {
#if defined(_WIN32)
                                int n = recv((SOCKET)fd, payload + got, (int)((size_t)blen + 2 - got), 0);
#else
                                ssize_t n = recv(fd, payload + got, (size_t)blen + 2 - got, 0);
#endif
                                if (n <= 0) break;
                                got += (size_t)n;
                            }
                            if (got == (size_t)blen + 2) {
                                payload[blen] = '\0';
                                const char* pp = payload;
                                out = joss_json_parse_object(&pp);
                            } else {
                                joss_set_session_err("redis GET truncated");
                            }
                            free(payload);
                        }
                    }
                } else if (line[0] == '-') {
                    joss_set_session_err(line + 1);
                }
            }
            joss_redis_close_fd(fd);
        }
    }
    if (!out) out = joss_map_new();
    /* Hydrate pending flash keys (once-only) into both session map and TLS flash */
    if (joss_array_key_exists("_flash", out)) {
        int64_t fv = joss_map_get_i64(out, "_flash");
        if (fv && joss_is_map_ptr(fv)) {
            void* fm = (void*)(uintptr_t)fv;
            g_joss_session_flash = joss_clone_flat_session_map(fm);
            g_joss_consumed_flash = joss_clone_flat_session_map(fm);
            int64_t fn = joss_map_len(fm);
            for (int64_t i = 0; i < fn; i++) {
                const char* fk = joss_map_key_at(fm, i);
                if (fk && fk[0]) {
                    joss_map_set_i64(out, fk, joss_clone_session_value(joss_map_val_at(fm, i)));
                }
            }
            joss_map_delete_i64(out, "_flash");
        }
    }
    return out;
}

int64_t joss___joss_session_save(const char* sid, void* data_ptr) {
    sid = unbox_web_cstr(sid);
    if (!sid || !sid[0]) return 0;
    int drv = joss_session_resolve_driver();
    if (drv < 0) return 0;
    void* src = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)data_ptr);
    JossOwnedScope* owner = joss_owned_scope_begin();
    void* copy = joss_clone_flat_session_map(src);
    if (g_joss_consumed_flash && joss_is_map_ptr((int64_t)(uintptr_t)g_joss_consumed_flash)) {
        int64_t cn = joss_map_len(g_joss_consumed_flash);
        for (int64_t i = 0; i < cn; i++) {
            const char* fk = joss_map_key_at(g_joss_consumed_flash, i);
            if (!fk || !fk[0]) continue;
            if (joss_array_key_exists(fk, copy) &&
                strcmp(joss_map_get_str(copy, fk), joss_map_get_str(g_joss_consumed_flash, fk)) == 0) {
                joss_map_delete_i64(copy, fk);
            }
        }
    }
    if (g_joss_new_flash && joss_is_map_ptr((int64_t)(uintptr_t)g_joss_new_flash) &&
        joss_map_len(g_joss_new_flash) > 0) {
        joss_map_set_i64(copy, "_flash", (int64_t)(uintptr_t)joss_clone_flat_session_map(g_joss_new_flash));
    }
    joss_owned_scope_end(owner);
    if (drv == 0) {
        joss_global_lock(JOSS_LOCK_SESSION);
        joss_session_replace_locked(sid, copy, owner);
        joss_global_unlock(JOSS_LOCK_SESSION);
        return 1;
    }
    if (drv == 1) {
        const char* path = joss_session_file_path();
        joss_global_lock(JOSS_LOCK_SESSION);
        bool ok = joss_file_sessions_ensure_loaded_locked(path);
        if (ok) {
            joss_session_replace_locked(sid, copy, owner);
            ok = joss_file_sessions_persist_locked(path);
        } else {
            joss_owned_scope_destroy(owner);
        }
        joss_global_unlock(JOSS_LOCK_SESSION);
        return ok ? 1 : 0;
    }
    int fd = joss_redis_connect_fd();
    if (fd < 0) {
        joss_owned_scope_destroy(owner);
        return 0;
    }
    bool ok = false;
    if (joss_redis_auth_and_select(fd)) {
        JOSS_AUTO_SB JossStringBuilder* jsb = joss_sb_new();
        json_encode_value(jsb, (int64_t)(uintptr_t)copy);
        char rkey[320];
        snprintf(rkey, sizeof(rkey), "session:%s", sid);
        int64_t sess_ttl = strtoll(joss_env("SESSION_TTL_SECONDS", "2592000"), NULL, 10);
        if (sess_ttl <= 0) sess_ttl = 2592000;
        char ttl_buf[32];
        snprintf(ttl_buf, sizeof(ttl_buf), "%lld", (long long)sess_ttl);
        const char* av[4] = {"SETEX", rkey, ttl_buf, (const char*)jsb->data};
        size_t al[4] = {5, strlen(rkey), strlen(ttl_buf), (size_t)jsb->len};
        char line[256];
        if (joss_redis_send_cmd(fd, 4, av, al) && joss_redis_read_line(fd, line, sizeof(line)) && line[0] == '+') {
            ok = true;
        } else {
            joss_set_session_err("redis SETEX failed");
        }
    }
    joss_redis_close_fd(fd);
    joss_owned_scope_destroy(owner);
    return ok ? 1 : 0;
}

int64_t joss___joss_session_destroy(const char* sid) {
    sid = unbox_web_cstr(sid);
    if (!sid || !sid[0]) return 0;
    int drv = joss_session_resolve_driver();
    if (drv < 0) return 0;
    if (drv == 0) {
        joss_global_lock(JOSS_LOCK_SESSION);
        joss_session_remove_locked(sid);
        joss_global_unlock(JOSS_LOCK_SESSION);
        return 1;
    }
    if (drv == 1) {
        const char* path = joss_session_file_path();
        joss_global_lock(JOSS_LOCK_SESSION);
        bool ok = joss_file_sessions_ensure_loaded_locked(path);
        if (ok && g_joss_session_mem_store) {
            joss_session_remove_locked(sid);
            ok = joss_file_sessions_persist_locked(path);
        }
        joss_global_unlock(JOSS_LOCK_SESSION);
        return ok ? 1 : 0;
    }
    int fd = joss_redis_connect_fd();
    if (fd < 0) return 0;
    bool ok = false;
    if (joss_redis_auth_and_select(fd)) {
        char rkey[320];
        snprintf(rkey, sizeof(rkey), "session:%s", sid);
        const char* av[2] = {"DEL", rkey};
        size_t al[2] = {3, strlen(rkey)};
        char line[256];
        if (joss_redis_send_cmd(fd, 2, av, al) && joss_redis_read_line(fd, line, sizeof(line)) &&
            (line[0] == ':' || line[0] == '+')) {
            ok = true;
        } else {
            joss_set_session_err("redis DEL failed");
        }
    }
    joss_redis_close_fd(fd);
    return ok ? 1 : 0;
}

const char* joss___joss_session_last_error(void) {
    return g_joss_session_last_err;
}

const char* fn_IndexNow_key(void) {
    return joss_env("INDEXNOW_KEY", "");
}

int64_t fn_IndexNow_submit(void* urls) {
    const char* key = joss_env("INDEXNOW_KEY", "");
    if (!key || !key[0]) return 0;
    const char* endpoint = joss_env("INDEXNOW_ENDPOINT", "https://api.indexnow.org/indexnow");
    if (!endpoint || !endpoint[0]) endpoint = "https://api.indexnow.org/indexnow";
    const char* app_url = joss_env("APP_URL", "http://localhost");
    const char* host = app_url;
    const char* sl = strstr(host, "://");
    if (sl) host = sl + 3;
    char host_buf[256];
    snprintf(host_buf, sizeof(host_buf), "%s", host);
    char* slash = strchr(host_buf, '/');
    if (slash) *slash = '\0';

    void* payload = joss_map_new();
    joss_map_set_str(payload, "host", host_buf);
    joss_map_set_str(payload, "key", key);
    joss_map_set_i64(payload, "urlList", (int64_t)(uintptr_t)urls);

    void* hdrs = joss_map_new();
    joss_map_set_str(hdrs, "Content-Type", "application/json");
    const char* body_json = joss_json_encode((int64_t)(uintptr_t)payload);
    void* resp = fn_Http_request("POST", endpoint, body_json, hdrs);
    if (resp && joss_is_map_ptr((int64_t)(uintptr_t)resp)) {
        int64_t st = joss_intval(joss_map_get_i64(resp, "status"));
        return (st >= 200 && st < 300) ? 1 : 0;
    }
    return 0;
}

int64_t fn_SmtpClient_send(const char* to, const char* subject, const char* body) {
    to = unbox_web_cstr(to);
    subject = unbox_web_cstr(subject);
    body = unbox_web_cstr(body);
    if (!to || !to[0]) {
        fprintf(stderr, "[SmtpClient] empty recipient\n");
        return 0;
    }
    const char* host = joss_env("MAIL_HOST", "");
    if (!host || !host[0]) host = joss_env("SMTP_HOST", "127.0.0.1");
    const char* port = joss_env("MAIL_PORT", "");
    if (!port || !port[0]) port = joss_env("SMTP_PORT", "25");
    const char* from = joss_env("MAIL_FROM_ADDRESS", "");
    if (!from || !from[0]) from = "noreply@joss.local";
    const char* user = joss_env("MAIL_USERNAME", "");
    if (!user || !user[0]) user = joss_env("SMTP_USER", "");
    const char* pass = joss_env("MAIL_PASSWORD", "");
    if (!pass || !pass[0]) pass = joss_env("SMTP_PASS", "");

    if ((strcmp(port, "465") == 0 || strcmp(port, "587") == 0) && user && user[0] && pass && pass[0]) {
        char tmp_eml[256];
        snprintf(tmp_eml, sizeof(tmp_eml), "joss_smtp_%lu.eml", (unsigned long)time(NULL));
        JOSS_AUTO_FILE FILE* ef = fopen(tmp_eml, "wb");
        if (!ef) return 0;
        fprintf(ef, "From: <%s>\r\nTo: <%s>\r\nSubject: %s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s\r\n",
                from, to, subject ? subject : "", body ? body : "");
        joss_auto_file_cleanup(&ef);
        char cmd_buf[2048];
        const char* scheme = (strcmp(port, "465") == 0) ? "smtps" : "smtp";
        const char* ssl_flag = (strcmp(port, "587") == 0) ? " --ssl-reqd" : "";
        snprintf(cmd_buf, sizeof(cmd_buf),
                 "curl -sS%s --url \"%s://%s:%s\" --user \"%s:%s\" --mail-from \"%s\" --mail-rcpt \"%s\" --upload-file \"%s\"",
                 ssl_flag, scheme, host, port, user, pass, from, to, tmp_eml);
#if defined(_WIN32)
        int rc = joss_win_run_hidden_cmd(cmd_buf);
#else
        int rc = system(cmd_buf);
#endif
        remove(tmp_eml);
        return rc == 0 ? 1 : 0;
    }

#if defined(_WIN32)
    static atomic_int smtp_wsa = 0;
    if (atomic_load(&smtp_wsa) == 0) {
        WSADATA wd;
        if (WSAStartup(MAKEWORD(2, 2), &wd) == 0) atomic_store(&smtp_wsa, 1);
    }
#endif
    struct addrinfo hints;
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;
    struct addrinfo* res = NULL;
    if (getaddrinfo(host, port, &hints, &res) != 0 || !res) {
        fprintf(stderr, "[SmtpClient] getaddrinfo failed for %s:%s\n", host, port);
        return 0;
    }
    int fd = -1;
    for (struct addrinfo* rp = res; rp; rp = rp->ai_next) {
#if defined(_WIN32)
        SOCKET s = socket(rp->ai_family, rp->ai_socktype, rp->ai_protocol);
        if (s == INVALID_SOCKET) continue;
        if (connect(s, rp->ai_addr, (int)rp->ai_addrlen) == 0) { fd = (int)s; break; }
        closesocket(s);
#else
        int s = socket(rp->ai_family, rp->ai_socktype, rp->ai_protocol);
        if (s < 0) continue;
        if (connect(s, rp->ai_addr, rp->ai_addrlen) == 0) { fd = s; break; }
        close(s);
#endif
    }
    freeaddrinfo(res);
    if (fd < 0) {
        fprintf(stderr, "[SmtpClient] connect failed to %s:%s\n", host, port);
        return 0;
    }

    char line[512];
    (void)joss_redis_read_line(fd, line, sizeof(line));
    char cmd[512];
    snprintf(cmd, sizeof(cmd), "EHLO localhost\r\n");
    joss_redis_send_all(fd, cmd, strlen(cmd));
    while (joss_redis_read_line(fd, line, sizeof(line))) {
        if (strlen(line) >= 4 && line[3] == ' ') break;
    }
    snprintf(cmd, sizeof(cmd), "MAIL FROM:<%s>\r\n", from);
    joss_redis_send_all(fd, cmd, strlen(cmd));
    (void)joss_redis_read_line(fd, line, sizeof(line));
    snprintf(cmd, sizeof(cmd), "RCPT TO:<%s>\r\n", to);
    joss_redis_send_all(fd, cmd, strlen(cmd));
    (void)joss_redis_read_line(fd, line, sizeof(line));
    joss_redis_send_all(fd, "DATA\r\n", 6);
    (void)joss_redis_read_line(fd, line, sizeof(line));

    JOSS_AUTO_SB JossStringBuilder* msg = joss_sb_new();
    joss_sb_append(msg, "From: "); joss_sb_append(msg, from); joss_sb_append(msg, "\r\n");
    joss_sb_append(msg, "To: "); joss_sb_append(msg, to); joss_sb_append(msg, "\r\n");
    joss_sb_append(msg, "Subject: "); joss_sb_append(msg, subject ? subject : ""); joss_sb_append(msg, "\r\n\r\n");
    joss_sb_append(msg, body ? body : "");
    joss_sb_append(msg, "\r\n.\r\n");
    bool ok = joss_redis_send_all(fd, (const char*)msg->data, (size_t)msg->len);
    if (!ok) {
        fprintf(stderr, "[SmtpClient] send DATA body failed\n");
    }
    (void)joss_redis_read_line(fd, line, sizeof(line));
    joss_redis_send_all(fd, "QUIT\r\n", 6);
    joss_redis_close_fd(fd);
    return ok ? 1 : 0;
}

int64_t fn_Mail_send(const char* to, const char* subject, const char* body) {
    return fn_SmtpClient_send(to, subject, body);
}

#include "joss_zip_rt.inc"
