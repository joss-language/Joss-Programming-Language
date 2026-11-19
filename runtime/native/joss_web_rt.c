//go:build ignore
#include "joss_rt.h"

#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#if defined(__unix__) || defined(__APPLE__)
#include <arpa/inet.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <unistd.h>
#elif defined(_WIN32)
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <ws2tcpip.h>
#endif

/* =========================================================================
 * Active Request & Session Context
 * ========================================================================= */
#if defined(__STDC_VERSION__) && __STDC_VERSION__ >= 201112L
static _Thread_local void* g_joss_active_request = NULL;
static _Thread_local void* g_joss_session_flash = NULL;
#else
static __thread void* g_joss_active_request = NULL;
static __thread void* g_joss_session_flash = NULL;
#endif

void joss___joss_rt_set_request_context(void* req) {
    g_joss_active_request = req;
}

/* =========================================================================
 * String & Conversion Helpers
 * ========================================================================= */
const char* joss_str_concat_i64(const char* a, int64_t b) {
    char buf[64];
    snprintf(buf, sizeof(buf), "%lld", (long long)b);
    return joss_str_concat(a ? a : "", buf);
}

static const char* unbox_web_cstr(const char* s) {
    if (!s) return NULL;
    int64_t v = (int64_t)(uintptr_t)s;
    if (joss_is_boxed_ptr(v)) {
        return joss_strval(v);
    }
    return s;
}

int64_t joss_strpos(const char* haystack, const char* needle) {
    haystack = unbox_web_cstr(haystack);
    needle = unbox_web_cstr(needle);
    if (!haystack || !needle) return 0;
    const char* found = strstr(haystack, needle);
    if (!found) return 0;
    int64_t rune_idx = 0;
    for (const char* p = haystack; p < found; p++) {
        if (((unsigned char)*p & 0xC0) != 0x80) {
            rune_idx++;
        }
    }
    return rune_idx;
}

const char* joss_substr(const char* s, int64_t start, int64_t length) {
    s = unbox_web_cstr(s);
    start = joss_unbox_value(start);
    if (length != INT64_MIN) length = joss_unbox_value(length);
    if (!s) return "";
    int64_t total_runes = 0;
    for (const char* p = s; *p; p++) {
        if (((unsigned char)*p & 0xC0) != 0x80) {
            total_runes++;
        }
    }
    if (start < 0) start = total_runes + start;
    if (start < 0) start = 0;
    if (start >= total_runes) return "";
    int64_t end_rune;
    if (length == INT64_MIN) {
        end_rune = total_runes;
    } else if (length < 0) {
        end_rune = total_runes + length;
    } else {
        end_rune = start + length;
    }
    if (end_rune > total_runes) end_rune = total_runes;
    if (end_rune <= start) return "";

    const char* start_ptr = s;
    int64_t cur_rune = 0;
    while (*start_ptr && cur_rune < start) {
        start_ptr++;
        while (*start_ptr && ((unsigned char)*start_ptr & 0xC0) == 0x80) {
            start_ptr++;
        }
        cur_rune++;
    }
    const char* end_ptr = start_ptr;
    while (*end_ptr && cur_rune < end_rune) {
        end_ptr++;
        while (*end_ptr && ((unsigned char)*end_ptr & 0xC0) == 0x80) {
            end_ptr++;
        }
        cur_rune++;
    }
    size_t byte_len = (size_t)(end_ptr - start_ptr);
    char* out = (char*)malloc(byte_len + 1);
    if (!out) return "";
    memcpy(out, start_ptr, byte_len);
    out[byte_len] = '\0';
    return out;
}

const char* joss_trim(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    while (*s && isspace((unsigned char)*s)) s++;
    size_t n = strlen(s);
    while (n > 0 && isspace((unsigned char)s[n - 1])) n--;
    char* out = (char*)malloc(n + 1);
    if (!out) return "";
    memcpy(out, s, n);
    out[n] = '\0';
    return out;
}

const char* joss_strtolower(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    size_t n = strlen(s);
    char* out = (char*)malloc(n + 1);
    if (!out) return "";
    for (size_t i = 0; i < n; i++) out[i] = (char)tolower((unsigned char)s[i]);
    out[n] = '\0';
    return out;
}

const char* joss_strtoupper(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    size_t n = strlen(s);
    char* out = (char*)malloc(n + 1);
    if (!out) return "";
    for (size_t i = 0; i < n; i++) out[i] = (char)toupper((unsigned char)s[i]);
    out[n] = '\0';
    return out;
}

const char* joss_urldecode(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    size_t n = strlen(s);
    char* out = (char*)malloc(n + 1);
    if (!out) return "";
    size_t j = 0;
    for (size_t i = 0; i < n; i++) {
        if (s[i] == '+') {
            out[j++] = ' ';
        } else if (s[i] == '%' && i + 2 < n && isxdigit((unsigned char)s[i + 1]) && isxdigit((unsigned char)s[i + 2])) {
            char hex[3] = {s[i + 1], s[i + 2], '\0'};
            out[j++] = (char)strtol(hex, NULL, 16);
            i += 2;
        } else {
            out[j++] = s[i];
        }
    }
    out[j] = '\0';
    return out;
}

const char* joss_rawurldecode(const char* s) {
    return joss_urldecode(s);
}

void* joss_explode(const char* sep, const char* subj) {
    sep = unbox_web_cstr(sep);
    subj = unbox_web_cstr(subj);
    void* arr = joss_arr_new(4);
    if (!subj) return arr;
    if (!sep || !*sep) {
        const char* p = subj;
        while (*p) {
            const char* start = p++;
            while (*p && ((unsigned char)*p & 0xC0) == 0x80) {
                p++;
            }
            size_t rlen = (size_t)(p - start);
            char* ch = (char*)malloc(rlen + 1);
            if (ch) {
                memcpy(ch, start, rlen);
                ch[rlen] = '\0';
                joss_arr_push_str(arr, ch);
            }
        }
        return arr;
    }
    size_t sep_len = strlen(sep);
    const char* cur = subj;
    while (1) {
        const char* p = strstr(cur, sep);
        if (!p) {
            joss_arr_push_str(arr, strdup(cur));
            break;
        }
        size_t part_len = (size_t)(p - cur);
        char* part = (char*)malloc(part_len + 1);
        memcpy(part, cur, part_len);
        part[part_len] = '\0';
        joss_arr_push_str(arr, part);
        cur = p + sep_len;
    }
    return arr;
}

/* =========================================================================
 * Container, Type & Truthiness Helpers
 * ========================================================================= */
int64_t joss_count(void* container) {
    if (!container) return 0;
    int64_t v = joss_unbox_value((int64_t)(uintptr_t)container);
    if (joss_is_map_ptr(v)) return joss_map_len((void*)(uintptr_t)v);
    if (joss_is_arr_ptr(v)) return joss_arr_len((void*)(uintptr_t)v);
    return 0;
}

int64_t len(void* container) {
    if (!container) return 0;
    int64_t v = joss_unbox_value((int64_t)(uintptr_t)container);
    if (joss_is_map_ptr(v)) return joss_map_len((void*)(uintptr_t)v);
    if (joss_is_arr_ptr(v)) return joss_arr_len((void*)(uintptr_t)v);
    uint64_t u = (uint64_t)v;
    if (u >= JOSS_MIN_PTR_ADDR && (u >> 47) == 0) {
        return joss_strlen((const char*)(uintptr_t)u);
    }
    return 0;
}

int64_t joss_is_truthy(int64_t v) {
    if (v == 0) return 0;
    if (joss_is_boxed_ptr(v)) {
        const JossBoxedValue* box = (const JossBoxedValue*)(uintptr_t)(uint64_t)v;
        uint32_t tag = (uint32_t)(box->header & 0xFFFFFFFFULL);
        if (tag == JOSS_TAG_NULL) return 0;
        if (tag == JOSS_TAG_BOOL || tag == JOSS_TAG_INT) return box->payload != 0 ? 1 : 0;
        if (tag == JOSS_TAG_STRING) {
            const char* s = (const char*)(uintptr_t)box->payload;
            return (s && s[0] != '\0') ? 1 : 0;
        }
        v = (int64_t)box->payload;
    }
    if (v == 0) return 0;
    if (joss_is_arr_ptr(v)) return joss_arr_len((void*)(uintptr_t)v) > 0 ? 1 : 0;
    if (joss_is_map_ptr(v)) return joss_map_len((void*)(uintptr_t)v) > 0 ? 1 : 0;
    uint64_t u = (uint64_t)v;
    if (u >= JOSS_MIN_PTR_ADDR && (u >> 47) == 0) {
        const char* s = (const char*)(uintptr_t)u;
        return s[0] != '\0' ? 1 : 0;
    }
    return 1;
}

int64_t joss_is_array(int64_t v) {
    v = joss_unbox_value(v);
    return joss_is_arr_ptr(v) ? 1 : 0;
}

int64_t joss_is_map(int64_t v) {
    v = joss_unbox_value(v);
    return joss_is_map_ptr(v) ? 1 : 0;
}

int64_t joss_is_string(int64_t v) {
    if (joss_is_arr_ptr(v) || joss_is_map_ptr(v)) return 0;
    if (joss_is_boxed_ptr(v)) {
        const JossBoxedValue* box = (const JossBoxedValue*)(uintptr_t)(uint64_t)v;
        return (uint32_t)(box->header & 0xFFFFFFFFULL) == JOSS_TAG_STRING ? 1 : 0;
    }
    uint64_t u = (uint64_t)v;
    if (u < JOSS_MIN_PTR_ADDR || (u >> 47) != 0) return 0;
    return 1;
}

int64_t joss_is_null(int64_t v) {
    return v == 0 ? 1 : 0;
}

int64_t joss_boolval(int64_t v) {
    return joss_is_truthy(v);
}

void* joss_array_push(void* arr, int64_t val) {
    if (!arr) arr = joss_arr_new(4);
    joss_arr_push_i64(arr, val);
    return arr;
}

void* array_push(void* arr, int64_t val) {
    return joss_array_push(arr, val);
}

const char* joss___joss_byte_string(int64_t b) {
    char* out = (char*)malloc(2);
    if (!out) return "";
    out[0] = (char)(b & 0xFF);
    out[1] = '\0';
    return out;
}

int64_t joss___joss_string_byte_at(const char* s, int64_t idx) {
    if (!s || idx < 0) return 0;
    size_t n = strlen(s);
    if ((size_t)idx >= n) return 0;
    return (int64_t)(unsigned char)s[idx];
}

double joss_floatval(int64_t v) {
    uint64_t u = (uint64_t)v;
    if (u >= JOSS_MIN_PTR_ADDR && (u >> 47) == 0) {
        const char* s = joss_strval(v);
        return s ? strtod(s, NULL) : 0.0;
    }
    return (double)v;
}

/* =========================================================================
 * RFC 3174 SHA-1 + Base64 (WebSocket Sec-WebSocket-Accept)
 * ========================================================================= */
static uint32_t sha1_rol(uint32_t v, unsigned int bits) {
    return (v << bits) | (v >> (32U - bits));
}

const char* joss_sha1_base64(const char* s) {
    if (!s) s = "";
    size_t msg_len = strlen(s);
    size_t padded_len = ((msg_len + 8U) / 64U + 1U) * 64U;
    uint8_t* msg = (uint8_t*)calloc(1, padded_len);
    if (!msg) return "";
    memcpy(msg, s, msg_len);
    msg[msg_len] = 0x80;
    uint64_t bit_len = (uint64_t)msg_len * 8ULL;
    for (int i = 0; i < 8; i++) {
        msg[padded_len - 1U - (size_t)i] = (uint8_t)((bit_len >> (i * 8)) & 0xFFU);
    }

    uint32_t h0 = 0x67452301U, h1 = 0xEFCDAB89U, h2 = 0x98BADCFEU, h3 = 0x10325476U, h4 = 0xC3D2E1F0U;
    for (size_t offset = 0; offset < padded_len; offset += 64U) {
        uint32_t w[80];
        for (int i = 0; i < 16; i++) {
            w[i] = ((uint32_t)msg[offset + i * 4] << 24) |
                   ((uint32_t)msg[offset + i * 4 + 1] << 16) |
                   ((uint32_t)msg[offset + i * 4 + 2] << 8) |
                   ((uint32_t)msg[offset + i * 4 + 3]);
        }
        for (int i = 16; i < 80; i++) {
            w[i] = sha1_rol(w[i - 3] ^ w[i - 8] ^ w[i - 14] ^ w[i - 16], 1);
        }
        uint32_t a = h0, b = h1, c = h2, d = h3, e = h4;
        for (int i = 0; i < 80; i++) {
            uint32_t f, k;
            if (i < 20)      { f = (b & c) | ((~b) & d); k = 0x5A827999U; }
            else if (i < 40) { f = b ^ c ^ d;            k = 0x6ED9EBA1U; }
            else if (i < 60) { f = (b & c) | (b & d) | (c & d); k = 0x8F1BBCDCU; }
            else             { f = b ^ c ^ d;            k = 0xCA62C1D6U; }
            uint32_t tmp = sha1_rol(a, 5) + f + e + k + w[i];
            e = d; d = c; c = sha1_rol(b, 30); b = a; a = tmp;
        }
        h0 += a; h1 += b; h2 += c; h3 += d; h4 += e;
    }
    free(msg);

    uint8_t digest[20];
    uint32_t hs[5] = {h0, h1, h2, h3, h4};
    for (int i = 0; i < 5; i++) {
        digest[i * 4]     = (uint8_t)((hs[i] >> 24) & 0xFFU);
        digest[i * 4 + 1] = (uint8_t)((hs[i] >> 16) & 0xFFU);
        digest[i * 4 + 2] = (uint8_t)((hs[i] >> 8) & 0xFFU);
        digest[i * 4 + 3] = (uint8_t)(hs[i] & 0xFFU);
    }

    static const char b64[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    char* out = (char*)malloc(32);
    if (!out) return "";
    size_t j = 0;
    for (size_t i = 0; i < 20; i += 3) {
        uint32_t v = ((uint32_t)digest[i] << 16) |
                     (i + 1 < 20 ? ((uint32_t)digest[i + 1] << 8) : 0U) |
                     (i + 2 < 20 ? (uint32_t)digest[i + 2] : 0U);
        out[j++] = b64[(v >> 18) & 0x3FU];
        out[j++] = b64[(v >> 12) & 0x3FU];
        out[j++] = (i + 1 < 20) ? b64[(v >> 6) & 0x3FU] : '=';
        out[j++] = (i + 2 < 20) ? b64[v & 0x3FU] : '=';
    }
    out[j] = '\0';
    return out;
}

/* =========================================================================
 * Time, Environment & Safe File System Helpers
 * ========================================================================= */
int64_t joss_time(void) {
    return (int64_t)time(NULL);
}

const char* joss_date(const char* fmt, int64_t ts) {
    time_t t = ts > 0 ? (time_t)ts : time(NULL);
    struct tm tm_val;
#if defined(_WIN32)
    gmtime_s(&tm_val, &t);
#else
    gmtime_r(&t, &tm_val);
#endif
    if (!fmt || !*fmt) fmt = "Y-m-d H:i:s";
    JossStringBuilder* sb = joss_sb_new();
    char num[16];
    for (const char* p = fmt; *p; p++) {
        switch (*p) {
            case 'Y': snprintf(num, sizeof(num), "%04d", tm_val.tm_year + 1900); joss_sb_append(sb, num); break;
            case 'm': snprintf(num, sizeof(num), "%02d", tm_val.tm_mon + 1); joss_sb_append(sb, num); break;
            case 'd': snprintf(num, sizeof(num), "%02d", tm_val.tm_mday); joss_sb_append(sb, num); break;
            case 'H': snprintf(num, sizeof(num), "%02d", tm_val.tm_hour); joss_sb_append(sb, num); break;
            case 'i': snprintf(num, sizeof(num), "%02d", tm_val.tm_min); joss_sb_append(sb, num); break;
            case 's': snprintf(num, sizeof(num), "%02d", tm_val.tm_sec); joss_sb_append(sb, num); break;
            default: joss_buf_append_byte(sb, (uint8_t)*p); break;
        }
    }
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}

int64_t joss_strtotime(const char* s) {
    if (!s || !*s) return 0;
    int y = 0, m = 1, d = 1, H = 0, M = 0, S = 0;
    int parsed = sscanf(s, "%d-%d-%d %d:%d:%d", &y, &m, &d, &H, &M, &S);
    if (parsed >= 3) {
        struct tm tm_val;
        memset(&tm_val, 0, sizeof(tm_val));
        tm_val.tm_year = y - 1900;
        tm_val.tm_mon = m - 1;
        tm_val.tm_mday = d;
        tm_val.tm_hour = H;
        tm_val.tm_min = M;
        tm_val.tm_sec = S;
#if defined(__unix__) || defined(__APPLE__)
        return (int64_t)timegm(&tm_val);
#else
        return (int64_t)mktime(&tm_val);
#endif
    }
    return (int64_t)time(NULL);
}

static void* g_joss_env_map = NULL;

static void joss_load_env_file_if_needed(void) {
    if (g_joss_env_map != NULL) return;
    g_joss_env_map = joss_map_new();
    const char* candidates[] = {"env.joss", ".env", "../env.joss", "../.env"};
    for (int i = 0; i < 4; i++) {
        FILE* f = fopen(candidates[i], "rb");
        if (!f) continue;
        fseek(f, 0, SEEK_END);
        long sz = ftell(f);
        fseek(f, 0, SEEK_SET);
        if (sz <= 0) {
            fclose(f);
            continue;
        }
        char* buf = (char*)malloc((size_t)sz + 1);
        if (!buf) {
            fclose(f);
            continue;
        }
        size_t rd = fread(buf, 1, (size_t)sz, f);
        buf[rd] = '\0';
        fclose(f);
        char* p = buf;
        while (*p) {
            while (*p == ' ' || *p == '\t' || *p == '\r' || *p == '\n') p++;
            if (!*p) break;
            if (*p == '#') {
                while (*p && *p != '\n') p++;
                continue;
            }
            char* key_start = p;
            while (*p && *p != '=' && *p != '\n' && *p != '\r') p++;
            if (*p != '=') {
                while (*p && *p != '\n') p++;
                continue;
            }
            char* key_end = p;
            while (key_end > key_start && (key_end[-1] == ' ' || key_end[-1] == '\t')) key_end--;
            *key_end = '\0';
            p++;
            while (*p == ' ' || *p == '\t') p++;
            char* val_start = p;
            if (*p == '"' || *p == '\'') {
                char q = *p++;
                val_start = p;
                while (*p && *p != q) p++;
                if (*p == q) *p++ = '\0';
                while (*p && *p != '\n') p++;
            } else {
                while (*p && *p != '\r' && *p != '\n' && *p != '#') p++;
                char* val_end = p;
                while (val_end > val_start && (val_end[-1] == ' ' || val_end[-1] == '\t')) val_end--;
                if (*p == '#') {
                    while (*p && *p != '\n') p++;
                }
                if (*p) p++;
                *val_end = '\0';
            }
            joss_map_set_str(g_joss_env_map, key_start, val_start);
        }
        break;
    }
}

const char* joss_env(const char* key, const char* defVal) {
    if (!key) return defVal ? defVal : "";
    joss_load_env_file_if_needed();
    if (g_joss_env_map != NULL && joss_array_key_exists(key, g_joss_env_map)) {
        const char* mv = joss_map_get_str(g_joss_env_map, key);
        if (mv) return mv;
    }
    if (strcmp(key, "PREFIX") != 0) {
        const char* v = getenv(key);
        if (v) return v;
    }
    return defVal ? defVal : "";
}

static bool is_safe_relative_path(const char* path) {
    if (!path || !*path) return false;
    if (strstr(path, "..") != NULL) return false;
    return true;
}

int64_t joss_file_exists(const char* path) {
    if (!is_safe_relative_path(path)) return 0;
#if defined(__unix__) || defined(__APPLE__)
    struct stat st;
    return (stat(path, &st) == 0 && S_ISREG(st.st_mode)) ? 1 : 0;
#else
    FILE* f = fopen(path, "rb");
    if (f) {
        fclose(f);
        return 1;
    }
    return 0;
#endif
}

const char* joss_file_get_contents(const char* path) {
    if (!is_safe_relative_path(path)) return "";
    FILE* f = fopen(path, "rb");
    if (!f) return "";
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);
    if (sz <= 0) {
        fclose(f);
        return "";
    }
    char* buf = (char*)malloc((size_t)sz + 1);
    if (!buf) {
        fclose(f);
        return "";
    }
    size_t rd = fread(buf, 1, (size_t)sz, f);
    buf[rd] = '\0';
    fclose(f);
    return buf;
}

/* =========================================================================
 * JSON Serialization & HTTP Responses (200, 302, Cookies, Flash)
 * ========================================================================= */
static void json_escape_append(JossStringBuilder* sb, const char* s) {
    joss_buf_append_byte(sb, '"');
    if (s) {
        for (const char* p = s; *p; p++) {
            switch (*p) {
                case '"':  joss_sb_append(sb, "\\\""); break;
                case '\\': joss_sb_append(sb, "\\\\"); break;
                case '\n': joss_sb_append(sb, "\\n"); break;
                case '\r': joss_sb_append(sb, "\\r"); break;
                case '\t': joss_sb_append(sb, "\\t"); break;
                default:   joss_buf_append_byte(sb, (uint8_t)*p); break;
            }
        }
    }
    joss_buf_append_byte(sb, '"');
}

static void json_encode_value(JossStringBuilder* sb, int64_t val) {
    if (val == 0) {
        joss_sb_append(sb, "null");
        return;
    }
    if (joss_is_map_ptr(val)) {
        void* m = (void*)(uintptr_t)val;
        int64_t n = joss_map_len(m);
        joss_buf_append_byte(sb, '{');
        for (int64_t i = 0; i < n; i++) {
            if (i > 0) joss_buf_append_byte(sb, ',');
            json_escape_append(sb, joss_map_key_at(m, i));
            joss_buf_append_byte(sb, ':');
            json_encode_value(sb, joss_map_val_at(m, i));
        }
        joss_buf_append_byte(sb, '}');
        return;
    }
    if (joss_is_arr_ptr(val)) {
        void* a = (void*)(uintptr_t)val;
        int64_t n = joss_arr_len(a);
        joss_buf_append_byte(sb, '[');
        for (int64_t i = 0; i < n; i++) {
            if (i > 0) joss_buf_append_byte(sb, ',');
            json_encode_value(sb, joss_arr_get_i64(a, i));
        }
        joss_buf_append_byte(sb, ']');
        return;
    }
    if (joss_is_boxed_ptr(val)) {
        const JossBoxedValue* box = (const JossBoxedValue*)(uintptr_t)(uint64_t)val;
        uint32_t tag = (uint32_t)(box->header & 0xFFFFFFFFULL);
        if (tag == JOSS_TAG_NULL) { joss_sb_append(sb, "null"); return; }
        if (tag == JOSS_TAG_BOOL) { joss_sb_append(sb, box->payload ? "true" : "false"); return; }
        if (tag == JOSS_TAG_INT)  { joss_sb_append_i64(sb, (int64_t)box->payload); return; }
        if (tag == JOSS_TAG_STRING) { json_escape_append(sb, (const char*)(uintptr_t)box->payload); return; }
        json_encode_value(sb, (int64_t)box->payload);
        return;
    }
    uint64_t u = (uint64_t)val;
    if (u >= JOSS_MIN_PTR_ADDR && (u >> 47) == 0) {
        json_escape_append(sb, (const char*)(uintptr_t)u);
        return;
    }
    joss_sb_append_i64(sb, val);
}

const char* joss_json(int64_t data, int64_t status) {
    if (status <= 0) status = 200;
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 ");
    joss_sb_append_i64(sb, status);
    joss_sb_append(sb, status == 200 ? " OK\r\n" : " Status\r\n");
    joss_sb_append(sb, "Content-Type: application/json; charset=utf-8\r\nConnection: close\r\n\r\n");
    json_encode_value(sb, data);
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* joss_response(const char* body, int64_t status) {
    if (!body) body = "";
    if (strncmp(body, "HTTP/1.1 ", 9) == 0) return body;
    if (status <= 0) status = 200;
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 ");
    joss_sb_append_i64(sb, status);
    joss_sb_append(sb, " OK\r\nContent-Type: text/html; charset=utf-8\r\nConnection: close\r\n\r\n");
    joss_sb_append(sb, body);
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* joss_redirect(const char* url, int64_t status) {
    if (!url || !*url) url = "/";
    if (status <= 0) status = 302;
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 ");
    joss_sb_append_i64(sb, status);
    joss_sb_append(sb, " Found\r\nLocation: ");
    joss_sb_append(sb, url);
    joss_sb_append(sb, "\r\nContent-Length: 0\r\nConnection: close\r\n\r\n");
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* joss_back(void) {
    const char* ref = "/";
    if (g_joss_active_request && joss_is_map_ptr((int64_t)(uintptr_t)g_joss_active_request)) {
        int64_t hdrs = joss_map_get_i64(g_joss_active_request, "_headers");
        if (joss_is_map_ptr(hdrs)) {
            const char* r = joss_map_get_str((void*)(uintptr_t)hdrs, "referer");
            if (r && *r) ref = r;
        }
    }
    return joss_redirect(ref, 302);
}

const char* joss_view(const char* name, void* data) {
    (void)data;
    return name ? name : "";
}

const char* joss___joss_get_embedded_asset(const char* path) {
    if (!path) return "";
    return joss_file_get_contents(path);
}

const char* joss___joss_http_dispatch(const char* method, const char* path, void* req) {
    (void)method;
    (void)path;
    (void)req;
    return "";
}

/* =========================================================================
 * Dynamic Route Matching & Parameter Extraction ({param})
 * ========================================================================= */
int64_t joss___joss_rt_route_matches(const char* pattern, const char* path) {
    if (!pattern || !path) return 0;
    if (strcmp(pattern, path) == 0) return 1;
    if (!strchr(pattern, '{') && strchr(path, '{')) {
        const char* tmp = pattern;
        pattern = path;
        path = tmp;
    }
    const char* p = pattern;
    const char* s = path;
    while (*p && *s) {
        if (*p == '{') {
            while (*p && *p != '}') p++;
            if (*p == '}') p++;
            if (*s == '/') return 0;
            while (*s && *s != '/') s++;
        } else {
            if (*p != *s) return 0;
            p++;
            s++;
        }
    }
    return (*p == '\0' && *s == '\0') ? 1 : 0;
}

const char* joss___joss_rt_route_param(const char* pattern, const char* path, const char* name) {
    if (!pattern || !path || !name) return "";
    if (!strchr(pattern, '{') && strchr(path, '{')) {
        const char* tmp = pattern;
        pattern = path;
        path = tmp;
    }
    size_t name_len = strlen(name);
    const char* p = pattern;
    const char* s = path;
    while (*p && *s) {
        if (*p == '{') {
            p++;
            const char* pname = p;
            while (*p && *p != '}') p++;
            size_t plen = (size_t)(p - pname);
            if (*p == '}') p++;
            const char* s_start = s;
            while (*s && *s != '/') s++;
            if (plen == name_len && strncmp(pname, name, name_len) == 0) {
                size_t vlen = (size_t)(s - s_start);
                char* out = (char*)malloc(vlen + 1);
                if (!out) return "";
                memcpy(out, s_start, vlen);
                out[vlen] = '\0';
                return out;
            }
        } else {
            if (*p != *s) return "";
            p++;
            s++;
        }
    }
    return "";
}

int64_t __joss_rt_route_matches(const char* pattern, const char* path) {
    return joss___joss_rt_route_matches(pattern, path);
}

const char* __joss_rt_route_param(const char* pattern, const char* path, const char* name) {
    return joss___joss_rt_route_param(pattern, path, name);
}

/* =========================================================================
 * POSIX TCP Socket Server (with SO_REUSEADDR & TCP_NODELAY)
 * ========================================================================= */
void* fn_Socket_listen(const char* host, const char* port) {
    (void)host;
#if defined(__unix__) || defined(__APPLE__)
    int p = port ? atoi(port) : 8000;
    if (p <= 0) p = 8000;
    int fd = socket(AF_INET, SOCK_STREAM, 0);
    if (fd < 0) return NULL;
    int opt = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = INADDR_ANY;
    addr.sin_port = htons((uint16_t)p);
    if (bind(fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        close(fd);
        return NULL;
    }
    if (listen(fd, 512) < 0) {
        close(fd);
        return NULL;
    }
    return (void*)(intptr_t)(fd + 1);
#elif defined(_WIN32)
    static int wsa_ready = 0;
    if (!wsa_ready) {
        WSADATA wsa;
        if (WSAStartup(MAKEWORD(2, 2), &wsa) != 0) return NULL;
        wsa_ready = 1;
    }
    int p = port ? atoi(port) : 8000;
    if (p <= 0) p = 8000;
    SOCKET fd = socket(AF_INET, SOCK_STREAM, IPPROTO_TCP);
    if (fd == INVALID_SOCKET) return NULL;
    BOOL opt = TRUE;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, (const char*)&opt, sizeof(opt));
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = INADDR_ANY;
    addr.sin_port = htons((uint16_t)p);
    if (bind(fd, (struct sockaddr*)&addr, sizeof(addr)) == SOCKET_ERROR) {
        closesocket(fd);
        return NULL;
    }
    if (listen(fd, 512) == SOCKET_ERROR) {
        closesocket(fd);
        return NULL;
    }
    return (void*)(uintptr_t)(fd + 1);
#else
    (void)port;
    return NULL;
#endif
}

void* fn_Socket_accept(void* srv) {
#if defined(__unix__) || defined(__APPLE__)
    int sfd = (int)(intptr_t)srv - 1;
    if (sfd < 0) return NULL;
    int cfd = accept(sfd, NULL, NULL);
    if (cfd < 0) return NULL;
    int nodelay = 1;
    setsockopt(cfd, IPPROTO_TCP, TCP_NODELAY, &nodelay, sizeof(nodelay));
    joss_req_arena_begin();
    return (void*)(intptr_t)(cfd + 1);
#elif defined(_WIN32)
    if (!srv) return NULL;
    SOCKET sfd = (SOCKET)((uintptr_t)srv - 1);
    SOCKET cfd = accept(sfd, NULL, NULL);
    if (cfd == INVALID_SOCKET) return NULL;
    BOOL nodelay = TRUE;
    setsockopt(cfd, IPPROTO_TCP, TCP_NODELAY, (const char*)&nodelay, sizeof(nodelay));
    joss_req_arena_begin();
    return (void*)(uintptr_t)(cfd + 1);
#else
    (void)srv;
    return NULL;
#endif
}

const char* fn_Socket_receive(void* sock, int64_t maxBytes) {
#if defined(__unix__) || defined(__APPLE__)
    int fd = (int)(intptr_t)sock - 1;
    if (fd < 0 || maxBytes <= 0) return "";
    char* buf = (char*)joss_req_alloc((size_t)maxBytes + 1);
    if (!buf) return "";
    ssize_t n = recv(fd, buf, (size_t)maxBytes, 0);
    if (n <= 0) {
        return "";
    }
    buf[n] = '\0';
    return buf;
#elif defined(_WIN32)
    if (!sock || maxBytes <= 0) return "";
    SOCKET fd = (SOCKET)((uintptr_t)sock - 1);
    char* buf = (char*)joss_req_alloc((size_t)maxBytes + 1);
    if (!buf) return "";
    int n = recv(fd, buf, (int)maxBytes, 0);
    if (n <= 0) return "";
    buf[n] = '\0';
    return buf;
#else
    (void)sock;
    (void)maxBytes;
    return "";
#endif
}

int64_t fn_Socket_send(void* sock, const char* data) {
#if defined(__unix__) || defined(__APPLE__)
    int fd = (int)(intptr_t)sock - 1;
    if (fd < 0 || !data) return 0;
    return (int64_t)send(fd, data, strlen(data), 0);
#elif defined(_WIN32)
    if (!sock || !data) return 0;
    SOCKET fd = (SOCKET)((uintptr_t)sock - 1);
    return (int64_t)send(fd, data, (int)strlen(data), 0);
#else
    (void)sock;
    (void)data;
    return 0;
#endif
}

void fn_Socket_close(void* sock) {
#if defined(__unix__) || defined(__APPLE__)
    int fd = (int)(intptr_t)sock - 1;
    if (fd >= 0) close(fd);
    joss_req_arena_reset();
#elif defined(_WIN32)
    if (sock) {
        SOCKET fd = (SOCKET)((uintptr_t)sock - 1);
        closesocket(fd);
    }
    joss_req_arena_reset();
#else
    (void)sock;
#endif
}

/* =========================================================================
 * Web Context Helpers
 * ========================================================================= */

void joss_set_active_request(void* req) {
    if (req) {
        req = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)req);
    }
    g_joss_active_request = req;
}

const char* joss_request(const char* key, int64_t defVal) {
    void* active_req = g_joss_active_request;
    if (key && strcmp(key, "__joss_request_all_fields") == 0) {
        if (active_req && joss_is_map_ptr((int64_t)(uintptr_t)active_req)) {
            return (const char*)active_req;
        }
        if (defVal != 0 && joss_is_map_ptr(defVal)) {
            return (const char*)(uintptr_t)defVal;
        }
        return (const char*)joss_map_new();
    }
    if (key && active_req && joss_is_map_ptr((int64_t)(uintptr_t)active_req)) {
        if (joss_array_key_exists(key, active_req)) {
            int64_t found = joss_map_get_i64(active_req, key);
            if (found != 0) {
                return (const char*)(uintptr_t)found;
            }
        }
    }
    if (defVal != 0) {
        if (joss_is_map_ptr(defVal) || joss_is_arr_ptr(defVal)) {
            return (const char*)(uintptr_t)defVal;
        }
        return joss_strval(defVal);
    }
    return (const char*)0;
}

const char* fn_Request_input(const char* key, int64_t defVal) {
    return joss_request(key, defVal);
}

int64_t joss_is_empty(int64_t v) {
    return joss_is_truthy(v) ? 0 : 1;
}

const char* joss_session(const char* key, int64_t defVal) {
    if (key && g_joss_session_flash && joss_is_map_ptr((int64_t)(uintptr_t)g_joss_session_flash)) {
        const char* v = joss_map_get_str(g_joss_session_flash, key);
        if (v && *v) return v;
    }
    if (defVal != 0) return joss_strval(defVal);
    return "";
}

void* with(void* recv, const char* k, int64_t v) {
    if (k) {
        if (!g_joss_session_flash) g_joss_session_flash = joss_map_new();
        joss_map_set_str(g_joss_session_flash, k, joss_strval(v));
    }
    return recv;
}

void* withCookie(void* recv, const char* k, const char* v) {
    if (!recv || !k) return recv;
    const char* resp = (const char*)recv;
    const char* sep = strstr(resp, "\r\n\r\n");
    if (!sep) return recv;
    JossStringBuilder* sb = joss_sb_new();
    size_t head_len = (size_t)(sep - resp);
    joss_buf_append_bytes(sb, (const uint8_t*)resp, (int64_t)head_len);
    joss_sb_append(sb, "\r\nSet-Cookie: ");
    joss_sb_append(sb, k);
    joss_sb_append(sb, "=");
    joss_sb_append(sb, v ? v : "");
    joss_sb_append(sb, "; Path=/; HttpOnly");
    joss_sb_append(sb, sep);
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}

void* onSuccess(void* recv, int64_t cb) {
    (void)cb;
    return recv;
}

void* onChallenge(void* recv, int64_t cb) {
    (void)cb;
    return recv;
}

void* onFail(void* recv, int64_t cb) {
    (void)cb;
    return recv;
}

void* response(void* recv) {
    return recv;
}

const char* html_escape(const char* s) {
    if (!s) return "";
    size_t n = strlen(s);
    char* out = (char*)malloc(n * 6 + 1);
    if (!out) return "";
    size_t j = 0;
    for (size_t i = 0; i < n; i++) {
        switch (s[i]) {
            case '&': memcpy(out + j, "&amp;", 5); j += 5; break;
            case '<': memcpy(out + j, "&lt;", 4); j += 4; break;
            case '>': memcpy(out + j, "&gt;", 4); j += 4; break;
            case '"': memcpy(out + j, "&quot;", 6); j += 6; break;
            case '\'': memcpy(out + j, "&#39;", 5); j += 5; break;
            default: out[j++] = s[i]; break;
        }
    }
    out[j] = '\0';
    return out;
}

const char* csrf_field(void) {
    return "<input type=\"hidden\" name=\"_token\" value=\"joss-native-csrf\">";
}

int64_t joss___joss_local_utc_offset(void) {
    return 0;
}

void* joss___joss_timezone_data(const char* tz) {
    (void)tz;
    return joss_arr_new(0);
}

int64_t joss_file_put_contents(const char* path, const char* content) {
    if (!is_safe_relative_path(path)) return 0;
    FILE* f = fopen(path, "wb");
    if (!f) return 0;
    size_t n = content ? strlen(content) : 0;
    size_t wr = n > 0 ? fwrite(content, 1, n, f) : 0;
    fclose(f);
    return (wr == n) ? 1 : 0;
}

const char* joss_json_encode(int64_t data) {
    JossStringBuilder* sb = joss_sb_new();
    json_encode_value(sb, data);
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}

int64_t joss_json_verify(const char* s) {
    if (!s) return 0;
    while (*s && isspace((unsigned char)*s)) s++;
    return (*s == '{' || *s == '[' || *s == '"' || isdigit((unsigned char)*s) || *s == '-' || *s == 't' || *s == 'f' || *s == 'n') ? 1 : 0;
}

double joss_decimal(int64_t v) {
    return joss_floatval(v);
}
