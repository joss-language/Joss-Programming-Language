//go:build ignore
#if defined(__APPLE__) && !defined(_DARWIN_C_SOURCE)
#define _DARWIN_C_SOURCE
#elif defined(__unix__)
#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif
#ifndef _POSIX_C_SOURCE
#define _POSIX_C_SOURCE 200809L
#endif
#endif

#include "joss_rt.h"
#include "joss_crypto_primitives.inc"

#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <time.h>

#if defined(__unix__) || defined(__APPLE__)
#include <arpa/inet.h>
#include <errno.h>
#include <fcntl.h>
#include <netdb.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <pthread.h>
#include <signal.h>
#include <stdatomic.h>
#include <strings.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/time.h>
#include <sys/wait.h>
#include <unistd.h>
#elif defined(_WIN32)
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <ws2tcpip.h>
#include <windows.h>
#include <direct.h>
#include <process.h>
#include <stdatomic.h>
#endif

/* =========================================================================
 * Active Request & Session Context
 * ========================================================================= */
#if defined(__STDC_VERSION__) && __STDC_VERSION__ >= 201112L
static _Thread_local void* g_joss_active_request = NULL;
static _Thread_local void* g_joss_active_session = NULL;
static _Thread_local void* g_joss_session_flash = NULL;
static _Thread_local void* g_joss_new_flash = NULL;
static _Thread_local void* g_joss_consumed_flash = NULL;
static _Thread_local char  g_joss_session_last_err[256] = {0};
static _Thread_local char* g_tls_temp_uploads[64];
static _Thread_local int   g_tls_temp_upload_count = 0;
#else
static __thread void* g_joss_active_request = NULL;
static __thread void* g_joss_active_session = NULL;
static __thread void* g_joss_session_flash = NULL;
static __thread void* g_joss_new_flash = NULL;
static __thread void* g_joss_consumed_flash = NULL;
static __thread char  g_joss_session_last_err[256] = {0};
static __thread char* g_tls_temp_uploads[64];
static __thread int   g_tls_temp_upload_count = 0;
#endif

void joss_req_tempfiles_cleanup(void) {
    for (int i = 0; i < g_tls_temp_upload_count; i++) {
        if (g_tls_temp_uploads[i]) {
            remove(g_tls_temp_uploads[i]);
            free(g_tls_temp_uploads[i]);
            g_tls_temp_uploads[i] = NULL;
        }
    }
    g_tls_temp_upload_count = 0;
}

void joss_req_state_reset(void) {
    joss_req_tempfiles_cleanup();
    joss_closure_env_tls_reset();
    g_joss_active_request = NULL;
    g_joss_active_session = NULL;
    g_joss_session_flash = NULL;
    g_joss_new_flash = NULL;
    g_joss_consumed_flash = NULL;
    g_joss_session_last_err[0] = '\0';
}

void joss___joss_rt_set_request_context(void* req) {
    g_joss_active_request = req;
}

void* joss_get_active_request(void) {
    return g_joss_active_request;
}

void joss_set_active_session(void* sess) {
    if (sess) {
        sess = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)sess);
    }
    g_joss_active_session = sess;
}

void* joss_get_active_session(void) {
    if (!g_joss_active_session) {
        g_joss_active_session = joss_map_new();
    }
    return g_joss_active_session;
}

/* =========================================================================
 * String & Conversion Helpers
 * ========================================================================= */
const char* joss_str_concat_i64(const char* a, int64_t b) {
    return joss_str_concat(a ? a : "", joss_i64_to_str(b));
}

static const char* unbox_web_cstr(const char* s) {
    if (!s) return NULL;
    size_t blen = 0;
    const uint8_t* bdata = joss_bytes_unwrap(s, &blen);
    if (bdata) return (const char*)bdata;
    uintptr_t u = (uintptr_t)s;
    int64_t v = (int64_t)u;
    if (joss_is_boxed_ptr(v)) {
        return joss_strval(v);
    }
    if (u < 0x10000ULL || (u >> JOSS_PTR_VA_BITS) != 0) {
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
    return (int64_t)(uintptr_t)joss_box_i64(rune_idx);
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
    char* out = (char*)joss_req_alloc(byte_len + 1);
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
    char* out = (char*)joss_req_alloc(n + 1);
    if (!out) return "";
    memcpy(out, s, n);
    out[n] = '\0';
    return out;
}

const char* joss_strtolower(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    size_t n = strlen(s);
    char* out = (char*)joss_req_alloc(n + 1);
    if (!out) return "";
    for (size_t i = 0; i < n; i++) out[i] = (char)tolower((unsigned char)s[i]);
    out[n] = '\0';
    return out;
}

const char* joss_strtoupper(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    size_t n = strlen(s);
    char* out = (char*)joss_req_alloc(n + 1);
    if (!out) return "";
    for (size_t i = 0; i < n; i++) out[i] = (char)toupper((unsigned char)s[i]);
    out[n] = '\0';
    return out;
}

const char* joss_urldecode(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    size_t n = strlen(s);
    char* out = (char*)joss_req_alloc(n + 1);
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
            char* ch = (char*)joss_req_alloc(rlen + 1);
            if (ch) {
                memcpy(ch, start, rlen);
                ch[rlen] = '\0';
                joss_arr_push_str(arr, ch);
            }
        }
        return arr;
    }
    size_t sep_len = strlen(sep);
    size_t subj_len = strlen(subj);
    char* buf = (char*)joss_req_alloc(subj_len + 1);
    if (!buf) return arr;
    memcpy(buf, subj, subj_len + 1);

    char* cur = buf;
    if (sep_len == 1) {
        char delim = sep[0];
        for (;;) {
            char* p = strchr(cur, delim);
            if (!p) {
                joss_arr_push_str(arr, cur);
                break;
            }
            *p = '\0';
            joss_arr_push_str(arr, cur);
            cur = p + 1;
        }
        return arr;
    }
    for (;;) {
        char* p = strstr(cur, sep);
        if (!p) {
            joss_arr_push_str(arr, cur);
            break;
        }
        *p = '\0';
        joss_arr_push_str(arr, cur);
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
    if (u >= JOSS_MIN_PTR_ADDR && (u >> JOSS_PTR_VA_BITS) == 0) {
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
        if (tag == JOSS_TAG_BYTES) {
            const JossBytesBuf* bb = (const JossBytesBuf*)(uintptr_t)box->payload;
            return (bb && bb->len > 0) ? 1 : 0;
        }
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
    if (u >= JOSS_MIN_PTR_ADDR && (u >> JOSS_PTR_VA_BITS) == 0) {
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
        uint32_t tag = (uint32_t)(box->header & 0xFFFFFFFFULL);
        return (tag == JOSS_TAG_STRING || tag == JOSS_TAG_BYTES) ? 1 : 0;
    }
    uint64_t u = (uint64_t)v;
    if (u < JOSS_MIN_PTR_ADDR || (u >> JOSS_PTR_VA_BITS) != 0) return 0;
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
    b = joss_unbox_value(b);
    uint8_t ch = (uint8_t)(b & 0xFF);
    if (ch == 0) {
        return (const char*)joss_bytes_new(&ch, 1);
    }
    char* out = (char*)joss_req_alloc(2);
    if (!out) return "";
    out[0] = (char)ch;
    out[1] = '\0';
    return out;
}

int64_t joss___joss_string_byte_at(const char* s, int64_t idx) {
    idx = joss_unbox_value(idx);
    if (!s || idx < 0) return 0;
    size_t blen = 0;
    const uint8_t* bdata = joss_bytes_unwrap(s, &blen);
    if (bdata) {
        if ((size_t)idx >= blen) return 0;
        return (int64_t)bdata[idx];
    }
    s = unbox_web_cstr(s);
    if (!s) return 0;
    size_t n = strlen(s);
    if ((size_t)idx >= n) return 0;
    return (int64_t)(unsigned char)s[idx];
}

double joss_floatval(int64_t v) {
    uint64_t u = (uint64_t)v;
    if (u >= JOSS_MIN_PTR_ADDR && (u >> JOSS_PTR_VA_BITS) == 0) {
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
    size_t msg_len = 0;
    const uint8_t* src = joss_bytes_unwrap(s, &msg_len);
    if (!src) {
        s = unbox_web_cstr(s);
        if (!s) s = "";
        src = (const uint8_t*)s;
        msg_len = strlen(s);
    }
    size_t padded_len = ((msg_len + 8U) / 64U + 1U) * 64U;
    uint8_t* msg = (uint8_t*)calloc(1, padded_len);
    if (!msg) return "";
    if (msg_len > 0) memcpy(msg, src, msg_len);
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
    char* out = (char*)joss_req_alloc(32);
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
    fmt = unbox_web_cstr(fmt);
    time_t t = ts > 0 ? (time_t)ts : time(NULL);
    struct tm tm_val;
#if defined(_WIN32)
    localtime_s(&tm_val, &t);
#else
    localtime_r(&t, &tm_val);
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
    char* out = joss_sb_to_req_string(sb);
    joss_sb_free(sb);
    return out;
}

static int joss_strncasecmp_ascii(const char* a, const char* b, size_t n);

static int joss_strcasecmp_ascii(const char* a, const char* b) {
    if (!a) a = "";
    if (!b) b = "";
    while (*a && *b) {
        unsigned char ca = (unsigned char)*a++;
        unsigned char cb = (unsigned char)*b++;
        if (ca >= 'A' && ca <= 'Z') ca = (unsigned char)(ca + ('a' - 'A'));
        if (cb >= 'A' && cb <= 'Z') cb = (unsigned char)(cb + ('a' - 'A'));
        if (ca != cb) return (int)ca - (int)cb;
    }
    return (int)(unsigned char)*a - (int)(unsigned char)*b;
}

int64_t joss_strtotime(const char* s) {
    s = unbox_web_cstr(s);
    if (!s || !*s) return 0;
    while (*s == ' ' || *s == '\t' || *s == '\r' || *s == '\n') s++;
    if (!*s) return 0;

    int64_t now_ts = (int64_t)time(NULL);
    if (joss_strcasecmp_ascii(s, "now") == 0) return now_ts;
    if (joss_strcasecmp_ascii(s, "tomorrow") == 0) return now_ts + 86400;
    if (joss_strcasecmp_ascii(s, "yesterday") == 0) return now_ts - 86400;

    /* Relative offset: e.g. "+8 hours", "-5 minutes", "+30 seconds", "+7 days" */
    if (*s == '+' || *s == '-') {
        char* endp = NULL;
        long long delta = strtoll(s, &endp, 10);
        if (endp && endp != s) {
            while (*endp == ' ' || *endp == '\t') endp++;
            int64_t mult = 0;
            if (joss_strncasecmp_ascii(endp, "sec", 3) == 0) mult = 1;
            else if (joss_strncasecmp_ascii(endp, "min", 3) == 0) mult = 60;
            else if (joss_strncasecmp_ascii(endp, "hour", 4) == 0) mult = 3600;
            else if (joss_strncasecmp_ascii(endp, "day", 3) == 0) mult = 86400;
            else if (joss_strncasecmp_ascii(endp, "week", 4) == 0) mult = 604800;
            else if (joss_strncasecmp_ascii(endp, "month", 5) == 0) mult = 2592000;
            else if (joss_strncasecmp_ascii(endp, "year", 4) == 0) mult = 31536000;
            if (mult > 0) {
                return now_ts + (int64_t)delta * mult;
            }
        }
    }

    int y = 0, m = 1, d = 1, H = 0, M = 0, S = 0;
    int parsed = sscanf(s, "%d-%d-%d %d:%d:%d", &y, &m, &d, &H, &M, &S);
    if (parsed < 3) {
        parsed = sscanf(s, "%d-%d-%dT%d:%d:%d", &y, &m, &d, &H, &M, &S);
    }
    if (parsed >= 3) {
        struct tm tm_val;
        memset(&tm_val, 0, sizeof(tm_val));
        tm_val.tm_year = y - 1900;
        tm_val.tm_mon = m - 1;
        tm_val.tm_mday = d;
        tm_val.tm_hour = H;
        tm_val.tm_min = M;
        tm_val.tm_sec = S;
        tm_val.tm_isdst = -1;
        return (int64_t)mktime(&tm_val);
    }
    return now_ts;
}

typedef const char* (*joss_embedded_asset_fn)(const char* path);
static joss_embedded_asset_fn g_joss_embedded_asset_resolver = NULL;

void joss_register_embedded_asset_resolver(void* fn_ptr) {
    g_joss_embedded_asset_resolver = (joss_embedded_asset_fn)fn_ptr;
}

static int joss_hex_nibble(char c) {
    if (c >= '0' && c <= '9') return c - '0';
    if (c >= 'a' && c <= 'f') return c - 'a' + 10;
    if (c >= 'A' && c <= 'F') return c - 'A' + 10;
    return -1;
}

static char* joss_decrypt_embedded_env(const char* enc) {
    if (!enc || strncmp(enc, "JOSSENV1:", 9) != 0) return NULL;
    const char* key_hex = enc + 9;
    if (strlen(key_hex) < 66 || key_hex[64] != ':') return NULL;
    uint8_t key[32];
    for (int i = 0; i < 32; i++) {
        int hi = joss_hex_nibble(key_hex[i * 2]);
        int lo = joss_hex_nibble(key_hex[i * 2 + 1]);
        if (hi < 0 || lo < 0) return NULL;
        key[i] = (uint8_t)((hi << 4) | lo);
    }
    const char* cipher_hex = key_hex + 65;
    size_t clen_hex = strlen(cipher_hex);
    if ((clen_hex & 1) != 0 || clen_hex == 0) return NULL;
    size_t n = clen_hex / 2;
    char* plain = (char*)malloc(n + 1);
    if (!plain) return NULL;
    uint64_t state = 0x9e3779b97f4a7c15ULL;
    for (int i = 0; i < 32; i++) {
        state ^= ((uint64_t)key[i]) << ((unsigned)(i % 8) * 8U);
        state *= 0xbf58476d1ce4e5b9ULL;
    }
    for (size_t i = 0; i < n; i++) {
        int hi = joss_hex_nibble(cipher_hex[i * 2]);
        int lo = joss_hex_nibble(cipher_hex[i * 2 + 1]);
        if (hi < 0 || lo < 0) {
            free(plain);
            return NULL;
        }
        uint8_t cb = (uint8_t)((hi << 4) | lo);
        state ^= state >> 12;
        state ^= state << 25;
        state ^= state >> 27;
        uint8_t ks = (uint8_t)((state * 0x2545f4914f6cdd1dULL) >> 56) ^ key[i % 32];
        plain[i] = (char)(cb ^ ks);
    }
    plain[n] = '\0';
    return plain;
}

static void* g_joss_env_map = NULL;

static void joss_parse_env_buffer(char* buf) {
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
            while (*p && *p != '\r' && *p != '\n') p++;
            char* val_end = p;
            while (val_end > val_start && (val_end[-1] == ' ' || val_end[-1] == '\t')) val_end--;
            if (*p) p++;
            *val_end = '\0';
        }
        joss_map_set_str(g_joss_env_map, key_start, val_start);
    }
}

static void joss_load_env_file_if_needed(void) {
    if (g_joss_env_map != NULL) return;
    bool _prev_arena = joss_req_arena_pause();
    g_joss_env_map = joss_map_new();
    const char* candidates[] = {"env.joss", ".env"};
    for (int i = 0; i < 2; i++) {
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
        joss_parse_env_buffer(buf);
        joss_req_arena_resume(_prev_arena);
        return;
    }
    if (g_joss_embedded_asset_resolver != NULL) {
        const char* enc = g_joss_embedded_asset_resolver("__joss_embedded_env_enc");
        if (enc && enc[0]) {
            char* plain = joss_decrypt_embedded_env(enc);
            if (plain) {
                joss_parse_env_buffer(plain);
                joss_req_arena_resume(_prev_arena);
                return;
            }
        }
    }
    joss_req_arena_resume(_prev_arena);
}

const char* joss_env(const char* key, const char* defVal) {
    key = unbox_web_cstr(key);
    if (!key) return defVal ? defVal : "";
    if (strcmp(key, "PREFIX") != 0) {
        const char* v = getenv(key);
        if (v && v[0] != '\0') return v;
    }
    const char* result = NULL;
    joss_global_lock(JOSS_LOCK_ENV);
    joss_load_env_file_if_needed();
    if (g_joss_env_map != NULL && joss_array_key_exists(key, g_joss_env_map)) {
        const char* mv = joss_map_get_str(g_joss_env_map, key);
        if (mv) result = mv;
    }
    joss_global_unlock(JOSS_LOCK_ENV);
    return result ? result : (defVal ? defVal : "");
}

static bool is_safe_relative_path(const char* path) {
    if (!path || !*path) return false;
    if (strstr(path, "..") != NULL) return false;
    return true;
}

static const char* joss_lookup_embedded_priv_or_asset(const char* path, bool decrypt) {
    if (!path || !*path || !g_joss_embedded_asset_resolver) return NULL;
    char norm[512];
    size_t n = 0;
    for (const char* p = path; *p && n + 1 < sizeof(norm); p++) {
        norm[n++] = (*p == '\\') ? '/' : *p;
    }
    norm[n] = '\0';
    const char* s = norm;
    while (strncmp(s, "./", 2) == 0) s += 2;
    while (strncmp(s, "../", 3) == 0) s += 3;
    char priv_key[560];
    snprintf(priv_key, sizeof(priv_key), "__joss_priv_%s", s);
    const char* enc = g_joss_embedded_asset_resolver(priv_key);
    if (enc && enc[0]) {
        if (!decrypt) return enc;
        char* plain = joss_decrypt_embedded_env(enc);
        return plain ? plain : "";
    }
    const char* pub = g_joss_embedded_asset_resolver(s);
    if (pub && (joss_bytes_unwrap(pub, NULL) != NULL || pub[0])) return pub;
    return NULL;
}

int64_t joss_file_exists(const char* path) {
    path = unbox_web_cstr(path);
    if (!path || !*path) return 0;
    if (is_safe_relative_path(path)) {
#if defined(__unix__) || defined(__APPLE__)
        struct stat st;
        if (stat(path, &st) == 0 && S_ISREG(st.st_mode)) return 1;
#else
        FILE* f = fopen(path, "rb");
        if (f) {
            fclose(f);
            return 1;
        }
#endif
    }
    if (joss_lookup_embedded_priv_or_asset(path, false) != NULL) return 1;
    return 0;
}

const char* joss_file_get_contents(const char* path) {
    path = unbox_web_cstr(path);
    if (!path || !*path) return "";
    if (is_safe_relative_path(path)) {
        FILE* f = fopen(path, "rb");
        if (f) {
            fseek(f, 0, SEEK_END);
            long sz = ftell(f);
            fseek(f, 0, SEEK_SET);
            if (sz > 0) {
                char* buf = (char*)joss_req_alloc((size_t)sz + 1);
                if (buf) {
                    size_t rd = fread(buf, 1, (size_t)sz, f);
                    buf[rd] = '\0';
                    fclose(f);
                    if (memchr(buf, '\0', rd) != NULL) {
                        void* boxed = joss_bytes_new(buf, rd);
                        return (const char*)boxed;
                    }
                    return buf;
                }
            }
            fclose(f);
        }
    }
    const char* emb = joss_lookup_embedded_priv_or_asset(path, true);
    if (emb) return emb;
    return "";
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
        if (tag == JOSS_TAG_FLOAT) {
            double d = 0.0;
            memcpy(&d, &box->payload, sizeof(d));
            joss_sb_append(sb, joss_f64_to_string(d));
            return;
        }
        if (tag == JOSS_TAG_STRING) { json_escape_append(sb, unbox_web_cstr((const char*)(uintptr_t)box->payload)); return; }
        if (tag == JOSS_TAG_BYTES) {
            const JossBytesBuf* bb = (const JossBytesBuf*)(uintptr_t)box->payload;
            json_escape_append(sb, bb ? (const char*)bb->data : "");
            return;
        }
        json_encode_value(sb, (int64_t)box->payload);
        return;
    }
    uint64_t u = (uint64_t)val;
    if (u >= JOSS_MIN_PTR_ADDR && (u >> JOSS_PTR_VA_BITS) == 0) {
        json_escape_append(sb, (const char*)(uintptr_t)u);
        return;
    }
    joss_sb_append_i64(sb, val);
}

const char* joss_json(int64_t data, int64_t status) {
    status = joss_intval(status);
    if (status < 100 || status > 599) status = 200;
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 ");
    joss_sb_append_i64(sb, status);
    joss_sb_append(sb, status == 200 ? " OK\r\n" : " Status\r\n");
    joss_sb_append(sb, "Content-Type: application/json; charset=utf-8\r\nConnection: close\r\n\r\n");
    json_encode_value(sb, data);
    char* out = joss_sb_to_req_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* fn_Response_raw(const char* body, int64_t status, const char* content_type);

const char* joss_response(const char* body, int64_t status, const char* content_type) {
    size_t bin_len = 0;
    if (joss_bytes_unwrap(body, &bin_len)) {
        const char* ct = unbox_web_cstr(content_type);
        return fn_Response_raw(body, status, (ct && *ct) ? ct : "application/octet-stream");
    }
    if (!body) body = "";
    if (strncmp(body, "HTTP/1.1 ", 9) == 0) return body;
    const char* ct = unbox_web_cstr(content_type);
    if (ct && *ct) {
        return fn_Response_raw(body, status, ct);
    }
    status = joss_intval(status);
    if (status < 100 || status > 599) status = 200;
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 ");
    joss_sb_append_i64(sb, status);
    joss_sb_append(sb, " OK\r\nContent-Type: text/html; charset=utf-8\r\nConnection: close\r\n\r\n");
    joss_sb_append(sb, body);
    char* out = joss_sb_to_req_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* joss_redirect(const char* url, int64_t status) {
    url = unbox_web_cstr(url);
    if (!url || !*url) url = "/";
    status = joss_intval(status);
    if (status < 300 || status > 399) status = 302;
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 ");
    joss_sb_append_i64(sb, status);
    joss_sb_append(sb, " Found\r\nLocation: ");
    joss_sb_append(sb, url);
    joss_sb_append(sb, "\r\nContent-Length: 0\r\nConnection: close\r\n\r\n");
    char* out = joss_sb_to_req_string(sb);
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

/* Ephemeral live view-chunk loader for surgical HTML hot-reload in dev mode.
 * Reads `assets/.joss_cache/views/<sanitized_key>.chunks` and automatically
 * refreshes when the file's mtime changes on disk, without restarting the server. */
typedef struct {
    char* key;
    char* file_path;
    time_t mtime;
    long long fsize;
    uint32_t count;
    char** chunks;
} joss_dev_view_cache_entry_t;

static joss_dev_view_cache_entry_t* g_dev_view_cache = NULL;
static size_t g_dev_view_cache_len = 0;
static size_t g_dev_view_cache_cap = 0;
#ifdef _WIN32
static SRWLOCK g_dev_view_cache_lock = SRWLOCK_INIT;
#define JOSS_DEV_VIEW_LOCK() AcquireSRWLockExclusive(&g_dev_view_cache_lock)
#define JOSS_DEV_VIEW_UNLOCK() ReleaseSRWLockExclusive(&g_dev_view_cache_lock)
#else
static pthread_mutex_t g_dev_view_cache_lock = PTHREAD_MUTEX_INITIALIZER;
#define JOSS_DEV_VIEW_LOCK() pthread_mutex_lock(&g_dev_view_cache_lock)
#define JOSS_DEV_VIEW_UNLOCK() pthread_mutex_unlock(&g_dev_view_cache_lock)
#endif

static void joss_dev_view_free_chunks(joss_dev_view_cache_entry_t* e) {
    if (!e || !e->chunks) return;
    for (uint32_t i = 0; i < e->count; i++) {
        free(e->chunks[i]);
    }
    free(e->chunks);
    e->chunks = NULL;
    e->count = 0;
}

static void joss_dev_view_reload_entry(joss_dev_view_cache_entry_t* e) {
    if (!e || !e->file_path) return;
    struct stat st;
    if (stat(e->file_path, &st) != 0) return;
    time_t now_ts = time(NULL);
    bool recently_modified = (now_ts >= st.st_mtime && (now_ts - st.st_mtime) <= 2);
    if (e->chunks != NULL && !recently_modified && st.st_mtime == e->mtime && (long long)st.st_size == e->fsize) {
        return;
    }
    FILE* fp = fopen(e->file_path, "rb");
    if (!fp) return;
    unsigned char hdr[4];
    if (fread(hdr, 1, 4, fp) != 4) {
        fclose(fp);
        return;
    }
    uint32_t count = (uint32_t)hdr[0] | ((uint32_t)hdr[1] << 8) | ((uint32_t)hdr[2] << 16) | ((uint32_t)hdr[3] << 24);
    if (count > 100000u) {
        fclose(fp);
        return;
    }
    char** next_chunks = (char**)calloc(count > 0 ? count : 1, sizeof(char*));
    if (!next_chunks) {
        fclose(fp);
        return;
    }
    bool ok = true;
    for (uint32_t i = 0; i < count; i++) {
        unsigned char lbuf[4];
        if (fread(lbuf, 1, 4, fp) != 4) {
            ok = false;
            break;
        }
        uint32_t clen = (uint32_t)lbuf[0] | ((uint32_t)lbuf[1] << 8) | ((uint32_t)lbuf[2] << 16) | ((uint32_t)lbuf[3] << 24);
        if (clen > (16u * 1024u * 1024u)) {
            ok = false;
            break;
        }
        char* s = (char*)malloc((size_t)clen + 1u);
        if (!s) {
            ok = false;
            break;
        }
        if (clen > 0 && fread(s, 1, clen, fp) != clen) {
            free(s);
            ok = false;
            break;
        }
        s[clen] = '\0';
        next_chunks[i] = s;
    }
    fclose(fp);
    if (!ok) {
        for (uint32_t i = 0; i < count; i++) free(next_chunks[i]);
        free(next_chunks);
        return;
    }
    joss_dev_view_free_chunks(e);
    e->chunks = next_chunks;
    e->count = count;
    e->mtime = st.st_mtime;
    e->fsize = (long long)st.st_size;
}

const char* joss_dev_view_chunk(const char* view_key, int64_t idx) {
    view_key = unbox_web_cstr(view_key);
    if (!view_key || !*view_key || idx < 0) return "";
    JOSS_DEV_VIEW_LOCK();
    joss_dev_view_cache_entry_t* found = NULL;
    for (size_t i = 0; i < g_dev_view_cache_len; i++) {
        if (strcmp(g_dev_view_cache[i].key, view_key) == 0) {
            found = &g_dev_view_cache[i];
            break;
        }
    }
    if (!found) {
        if (g_dev_view_cache_len >= g_dev_view_cache_cap) {
            size_t ncap = g_dev_view_cache_cap ? g_dev_view_cache_cap * 2 : 32;
            joss_dev_view_cache_entry_t* nmem = (joss_dev_view_cache_entry_t*)realloc(g_dev_view_cache, ncap * sizeof(joss_dev_view_cache_entry_t));
            if (!nmem) {
                JOSS_DEV_VIEW_UNLOCK();
                return "";
            }
            g_dev_view_cache = nmem;
            g_dev_view_cache_cap = ncap;
        }
        found = &g_dev_view_cache[g_dev_view_cache_len++];
        memset(found, 0, sizeof(*found));
        size_t klen = strlen(view_key);
        found->key = (char*)malloc(klen + 1);
        if (found->key) memcpy(found->key, view_key, klen + 1);
        const char* prefix = "assets/.joss_cache/views/";
        const char* suffix = ".chunks";
        size_t plen = strlen(prefix) + klen + strlen(suffix) + 1;
        found->file_path = (char*)malloc(plen);
        if (found->file_path) {
            size_t pos = strlen(prefix);
            memcpy(found->file_path, prefix, pos);
            for (size_t i = 0; i < klen; i++) {
                char c = view_key[i];
                if ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.') {
                    found->file_path[pos++] = c;
                } else {
                    found->file_path[pos++] = '_';
                }
            }
            memcpy(found->file_path + pos, suffix, strlen(suffix) + 1);
        }
    }
    joss_dev_view_reload_entry(found);
    char* result_copy = NULL;
    if (found->chunks && (uint64_t)idx < (uint64_t)found->count && found->chunks[idx]) {
        size_t slen = strlen(found->chunks[idx]);
        result_copy = (char*)joss_req_alloc(slen + 1);
        if (result_copy) memcpy(result_copy, found->chunks[idx], slen + 1);
    }
    JOSS_DEV_VIEW_UNLOCK();
    return result_copy ? result_copy : "";
}

const char* joss___joss_get_embedded_asset(const char* path) {
    if (!path) return "";
    return joss_file_get_contents(path);
}

static bool joss_ws_resolve_socket(int64_t sock_fd, joss_socket_t* out_sock);
static void joss_ws_close_socket(joss_socket_t sock);
static void joss_unregister_wrapped_socket(intptr_t handle_val);
static void joss_ws_hub_subscribe_raw(const char* channel, joss_socket_t sock);
static void joss_ws_hub_unsubscribe_all(joss_socket_t sock);

const char* joss___joss_http_dispatch(const char* method, const char* path, void* req) {
    method = unbox_web_cstr(method);
    path = unbox_web_cstr(path);
    if (method && strcmp(method, "__websocket__") == 0) {
        joss_socket_t sock;
        if (!joss_ws_resolve_socket((int64_t)(uintptr_t)req, &sock)) {
            return "";
        }
        const char* channel = (path && path[0]) ? path : "/";
        joss_ws_hub_subscribe_raw(channel, sock);
        for (;;) {
            int opcode = 0;
            char* payload = NULL;
            size_t plen = 0;
            int rc = joss_ws_read_frame(sock, &opcode, &payload, &plen);
            if (rc != 0) {
                free(payload);
                break;
            }
            if ((opcode == 0x1 || opcode == 0x2) && payload) {
                (void)fn_WebSocket_publish(channel, payload);
            }
            free(payload);
        }
        joss_ws_hub_unsubscribe_all(sock);
        joss_unregister_wrapped_socket((intptr_t)joss_unbox_value((int64_t)(uintptr_t)req));
        joss_ws_close_socket(sock);
        joss_req_state_reset();
        joss_req_arena_reset();
        return "__JOSS_WEBSOCKET_HANDLED__";
    }
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
    if (!pattern || !path) return "";
    if (!strchr(pattern, '{') && strchr(path, '{')) {
        const char* tmp = pattern;
        pattern = path;
        path = tmp;
    }
    uintptr_t u_name = (uintptr_t)name;
    int by_index = (u_name < 0x10000ULL);
    int64_t target_idx = (int64_t)u_name;
    size_t name_len = by_index ? 0 : strlen(name);
    const char* p = pattern;
    const char* s = path;
    int64_t cur_idx = 0;
    while (*p && *s) {
        if (*p == '{') {
            p++;
            const char* pname = p;
            while (*p && *p != '}') p++;
            size_t plen = (size_t)(p - pname);
            if (*p == '}') p++;
            const char* s_start = s;
            while (*s && *s != '/') s++;
            int matched = by_index ? (cur_idx == target_idx)
                                   : (plen == name_len && strncmp(pname, name, name_len) == 0);
            cur_idx++;
            if (matched) {
                size_t vlen = (size_t)(s - s_start);
                char* out = (char*)joss_req_alloc(vlen + 1);
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
 * Wrapped Socket Handle Registry & Socket Helpers
 * ========================================================================= */
#define JOSS_WRAPPED_SOCK_MAX 4096
static intptr_t g_joss_wrapped_sockets[JOSS_WRAPPED_SOCK_MAX];
static int      g_joss_wrapped_sock_count = 0;

static void joss_register_wrapped_socket(intptr_t handle_val) {
    if (handle_val <= 0) return;
    joss_global_lock(JOSS_LOCK_WS);
    for (int i = 0; i < g_joss_wrapped_sock_count; i++) {
        if (g_joss_wrapped_sockets[i] == handle_val) {
            joss_global_unlock(JOSS_LOCK_WS);
            return;
        }
    }
    if (g_joss_wrapped_sock_count < JOSS_WRAPPED_SOCK_MAX) {
        g_joss_wrapped_sockets[g_joss_wrapped_sock_count++] = handle_val;
    }
    joss_global_unlock(JOSS_LOCK_WS);
}

static void joss_unregister_wrapped_socket(intptr_t handle_val) {
    if (handle_val <= 0) return;
    joss_global_lock(JOSS_LOCK_WS);
    for (int i = 0; i < g_joss_wrapped_sock_count; i++) {
        if (g_joss_wrapped_sockets[i] == handle_val) {
            g_joss_wrapped_sockets[i] = g_joss_wrapped_sockets[--g_joss_wrapped_sock_count];
            break;
        }
    }
    joss_global_unlock(JOSS_LOCK_WS);
}

static bool joss_ws_resolve_socket(int64_t sock_fd, joss_socket_t* out_sock) {
    if (!out_sock) return false;
    int64_t u = joss_unbox_value(sock_fd);
    if (u <= 0) return false;
    bool is_wrapped = false;
    joss_global_lock(JOSS_LOCK_WS);
    for (int i = 0; i < g_joss_wrapped_sock_count; i++) {
        if (g_joss_wrapped_sockets[i] == (intptr_t)u) {
            is_wrapped = true;
            break;
        }
    }
    joss_global_unlock(JOSS_LOCK_WS);
    if (is_wrapped) {
        *out_sock = (joss_socket_t)((uintptr_t)u - 1U);
        return true;
    }
#if defined(_WIN32)
    if ((uint64_t)u >= JOSS_MIN_PTR_ADDR) {
        int stype = 0;
        int slen = (int)sizeof(stype);
        if (getsockopt((SOCKET)(uintptr_t)u, SOL_SOCKET, SO_TYPE, (char*)&stype, &slen) != 0) {
            return false;
        }
    }
    *out_sock = (joss_socket_t)(uintptr_t)u;
    return true;
#else
    if ((uint64_t)u >= JOSS_MIN_PTR_ADDR) {
        return false;
    }
    *out_sock = (joss_socket_t)(int)u;
    return true;
#endif
}

static void joss_ws_close_socket(joss_socket_t sock) {
#if defined(_WIN32)
    if (sock != (joss_socket_t)INVALID_SOCKET) closesocket((SOCKET)sock);
#else
    if (sock >= 0) close(sock);
#endif
}

/* =========================================================================
 * POSIX TCP Socket Server (with SO_REUSEADDR & TCP_NODELAY)
 * ========================================================================= */
void* fn_Socket_listen(const char* host, const char* port) {
    (void)host;
#if defined(__unix__) || defined(__APPLE__)
    signal(SIGPIPE, SIG_IGN);
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
    static atomic_int wsa_ready = 0;
    if (atomic_load(&wsa_ready) == 0) {
        joss_global_lock(JOSS_LOCK_GENERAL);
        if (atomic_load(&wsa_ready) == 0) {
            WSADATA wsa;
            if (WSAStartup(MAKEWORD(2, 2), &wsa) != 0) {
                joss_global_unlock(JOSS_LOCK_GENERAL);
                return NULL;
            }
            atomic_store(&wsa_ready, 1);
        }
        joss_global_unlock(JOSS_LOCK_GENERAL);
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

static void joss_configure_client_socket(intptr_t handle_val) {
#if defined(__unix__) || defined(__APPLE__)
    int cfd = (int)handle_val - 1;
    if (cfd < 0) return;
    int nodelay = 1;
    setsockopt(cfd, IPPROTO_TCP, TCP_NODELAY, &nodelay, sizeof(nodelay));
#ifdef SO_NOSIGPIPE
    int nosig = 1;
    setsockopt(cfd, SOL_SOCKET, SO_NOSIGPIPE, &nosig, sizeof(nosig));
#endif
    struct timeval tv;
    tv.tv_sec = 15;
    tv.tv_usec = 0;
    setsockopt(cfd, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
    setsockopt(cfd, SOL_SOCKET, SO_SNDTIMEO, &tv, sizeof(tv));
#elif defined(_WIN32)
    SOCKET cfd = (SOCKET)(handle_val - 1);
    BOOL nodelay = TRUE;
    setsockopt(cfd, IPPROTO_TCP, TCP_NODELAY, (const char*)&nodelay, sizeof(nodelay));
    DWORD ms = 15000;
    setsockopt(cfd, SOL_SOCKET, SO_RCVTIMEO, (const char*)&ms, sizeof(ms));
    setsockopt(cfd, SOL_SOCKET, SO_SNDTIMEO, (const char*)&ms, sizeof(ms));
#else
    (void)handle_val;
#endif
}

void* fn_Socket_accept(void* srv) {
#if defined(__unix__) || defined(__APPLE__)
    int sfd = (int)(intptr_t)srv - 1;
    if (sfd < 0) return NULL;
    int cfd = accept(sfd, NULL, NULL);
    if (cfd < 0) return NULL;
    joss_configure_client_socket((intptr_t)(cfd + 1));
    joss_register_wrapped_socket((intptr_t)(cfd + 1));
    joss_req_arena_begin();
    return (void*)(intptr_t)(cfd + 1);
#elif defined(_WIN32)
    if (!srv) return NULL;
    SOCKET sfd = (SOCKET)((uintptr_t)srv - 1);
    SOCKET cfd = accept(sfd, NULL, NULL);
    if (cfd == INVALID_SOCKET) return NULL;
    joss_configure_client_socket((intptr_t)(cfd + 1));
    joss_register_wrapped_socket((intptr_t)(cfd + 1));
    joss_req_arena_begin();
    return (void*)(uintptr_t)(cfd + 1);
#else
    (void)srv;
    return NULL;
#endif
}

const char* fn_Socket_receive(void* sock, int64_t maxBytes) {
    if (maxBytes > 10 * 1024 * 1024) maxBytes = 10 * 1024 * 1024;
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
    size_t blen = 0;
    const uint8_t* bdata = joss_bytes_unwrap(data, &blen);
    const char* send_ptr = bdata ? (const char*)bdata : data;
    size_t send_len = bdata ? blen : (data ? strlen(data) : 0);
    if (!sock || !send_ptr || send_len == 0) return 0;
#if defined(__unix__) || defined(__APPLE__)
    int fd = (int)(intptr_t)sock - 1;
    if (fd < 0) return -1;
    int flags = 0;
#ifdef MSG_NOSIGNAL
    flags |= MSG_NOSIGNAL;
#endif
    size_t sent = 0;
    while (sent < send_len) {
        ssize_t n = send(fd, send_ptr + sent, send_len - sent, flags);
        if (n < 0) {
            if (errno == EINTR) continue;
            return sent > 0 ? (int64_t)sent : -1;
        }
        if (n == 0) break;
        sent += (size_t)n;
    }
    return (int64_t)sent;
#elif defined(_WIN32)
    SOCKET fd = (SOCKET)((uintptr_t)sock - 1);
    size_t sent = 0;
    while (sent < send_len) {
        size_t chunk = send_len - sent;
        if (chunk > (size_t)0x3FFFFFFF) chunk = (size_t)0x3FFFFFFF;
        int n = send(fd, send_ptr + sent, (int)chunk, 0);
        if (n <= 0) {
            return sent > 0 ? (int64_t)sent : -1;
        }
        sent += (size_t)n;
    }
    return (int64_t)sent;
#else
    (void)sock;
    (void)send_ptr;
    (void)send_len;
    return 0;
#endif
}

void fn_Socket_close(void* sock) {
    joss_unregister_wrapped_socket((intptr_t)sock);
#if defined(__unix__) || defined(__APPLE__)
    int fd = (int)(intptr_t)sock - 1;
    if (fd >= 0) close(fd);
#elif defined(_WIN32)
    if (sock) {
        SOCKET fd = (SOCKET)((uintptr_t)sock - 1);
        closesocket(fd);
    }
#else
    (void)sock;
#endif
    joss_req_state_reset();
    joss_req_arena_reset();
}

const char* joss___joss_socket_peer_ip(void* sock) {
    sock = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)sock);
#if defined(__unix__) || defined(__APPLE__)
    int fd = (int)(intptr_t)sock - 1;
    if (fd >= 0) {
        struct sockaddr_storage ss;
        socklen_t slen = sizeof(ss);
        if (getpeername(fd, (struct sockaddr*)&ss, &slen) == 0) {
            char buf[INET6_ADDRSTRLEN] = {0};
            if (ss.ss_family == AF_INET) {
                struct sockaddr_in* in = (struct sockaddr_in*)&ss;
                if (inet_ntop(AF_INET, &in->sin_addr, buf, sizeof(buf))) {
                    return joss_req_strdup(buf);
                }
            } else if (ss.ss_family == AF_INET6) {
                struct sockaddr_in6* in6 = (struct sockaddr_in6*)&ss;
                if (inet_ntop(AF_INET6, &in6->sin6_addr, buf, sizeof(buf))) {
                    const char* out = buf;
                    if (strncmp(out, "::ffff:", 7) == 0) out += 7;
                    return joss_req_strdup(out);
                }
            }
        }
    }
#elif defined(_WIN32)
    if (sock) {
        SOCKET fd = (SOCKET)((uintptr_t)sock - 1);
        struct sockaddr_storage ss;
        int slen = (int)sizeof(ss);
        if (getpeername(fd, (struct sockaddr*)&ss, &slen) == 0) {
            char buf[INET6_ADDRSTRLEN] = {0};
            if (ss.ss_family == AF_INET) {
                struct sockaddr_in* in = (struct sockaddr_in*)&ss;
                if (inet_ntop(AF_INET, &in->sin_addr, buf, sizeof(buf))) {
                    return joss_req_strdup(buf);
                }
            } else if (ss.ss_family == AF_INET6) {
                struct sockaddr_in6* in6 = (struct sockaddr_in6*)&ss;
                if (inet_ntop(AF_INET6, &in6->sin6_addr, buf, sizeof(buf))) {
                    const char* out = buf;
                    if (strncmp(out, "::ffff:", 7) == 0) out += 7;
                    return joss_req_strdup(out);
                }
            }
        }
    }
#else
    (void)sock;
#endif
    return "127.0.0.1";
}

static const uint8_t* joss_memmem_bytes(const uint8_t* hay, size_t hlen, const uint8_t* needle, size_t nlen) {
    if (!hay || !needle || nlen == 0 || hlen < nlen) return NULL;
    for (size_t i = 0; i <= hlen - nlen; i++) {
        if (hay[i] == needle[0] && memcmp(hay + i, needle, nlen) == 0) {
            return hay + i;
        }
    }
    return NULL;
}

static int joss_strncasecmp_ascii(const char* a, const char* b, size_t n) {
    for (size_t i = 0; i < n; i++) {
        unsigned char ca = (unsigned char)tolower((unsigned char)a[i]);
        unsigned char cb = (unsigned char)tolower((unsigned char)b[i]);
        if (ca != cb) return (int)ca - (int)cb;
        if (ca == '\0') return 0;
    }
    return 0;
}

int64_t joss___joss_http_read_request(void* sock) {
    sock = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)sock);
    if (!sock) return (int64_t)(uintptr_t)"";
#if defined(__unix__) || defined(__APPLE__)
    int fd = (int)(intptr_t)sock - 1;
    if (fd < 0) return (int64_t)(uintptr_t)"";
#elif defined(_WIN32)
    SOCKET fd = (SOCKET)((uintptr_t)sock - 1);
#else
    return (int64_t)(uintptr_t)"";
#endif
    size_t cap = 8192;
    size_t len = 0;
    uint8_t* buf = (uint8_t*)malloc(cap + 1);
    if (!buf) return (int64_t)(uintptr_t)"";
    size_t hdr_end = 0;
    bool found_hdr = false;
    const size_t max_hdr_bytes = 131072U;
    const size_t max_body_bytes = 32U * 1024U * 1024U;

    while (!found_hdr) {
        if (len + 4096U > cap) {
            if (cap >= max_hdr_bytes) break;
            size_t ncap = cap * 2U;
            if (ncap > max_hdr_bytes + 4096U) ncap = max_hdr_bytes + 4096U;
            uint8_t* nb = (uint8_t*)realloc(buf, ncap + 1);
            if (!nb) break;
            buf = nb;
            cap = ncap;
        }
#if defined(__unix__) || defined(__APPLE__)
        ssize_t n = recv(fd, (char*)buf + len, cap - len, 0);
        if (n < 0 && errno == EINTR) continue;
#else
        int n = recv(fd, (char*)buf + len, (int)(cap - len), 0);
#endif
        if (n <= 0) break;
        len += (size_t)n;
        const uint8_t* sep = joss_memmem_bytes(buf, len, (const uint8_t*)"\r\n\r\n", 4);
        if (sep) {
            hdr_end = (size_t)(sep - buf) + 4U;
            found_hdr = true;
            break;
        }
    }
    if (!found_hdr) {
        buf[len] = '\0';
        char* req_copy = (char*)joss_req_alloc(len + 1);
        if (req_copy) memcpy(req_copy, buf, len + 1);
        free(buf);
        return req_copy ? (int64_t)(uintptr_t)req_copy : (int64_t)(uintptr_t)"";
    }

    int64_t content_len = -1;
    bool is_chunked = false;
    for (size_t pos = 0; pos + 2 < hdr_end; ) {
        const uint8_t* eol = joss_memmem_bytes(buf + pos, hdr_end - pos, (const uint8_t*)"\r\n", 2);
        size_t line_len = eol ? (size_t)(eol - (buf + pos)) : (hdr_end - pos);
        if (line_len == 0) break;
        const char* line = (const char*)(buf + pos);
        if (line_len >= 15 && joss_strncasecmp_ascii(line, "content-length:", 15) == 0) {
            const char* p = line + 15;
            while ((size_t)(p - line) < line_len && (*p == ' ' || *p == '\t')) p++;
            content_len = strtoll(p, NULL, 10);
            if (content_len < 0) content_len = 0;
            if ((size_t)content_len > max_body_bytes) content_len = (int64_t)max_body_bytes;
        } else if (line_len >= 18 && joss_strncasecmp_ascii(line, "transfer-encoding:", 18) == 0) {
            char tmp[256];
            size_t cpy = line_len - 18;
            if (cpy >= sizeof(tmp)) cpy = sizeof(tmp) - 1;
            memcpy(tmp, line + 18, cpy);
            tmp[cpy] = '\0';
            for (size_t k = 0; k < cpy; k++) tmp[k] = (char)tolower((unsigned char)tmp[k]);
            if (strstr(tmp, "chunked") != NULL) is_chunked = true;
        }
        pos += line_len + 2;
    }

    if (content_len > 0) {
        size_t need = hdr_end + (size_t)content_len;
        if (need > cap) {
            uint8_t* nb = (uint8_t*)realloc(buf, need + 1);
            if (nb) {
                buf = nb;
                cap = need;
            }
        }
        while (len < need && len < cap) {
#if defined(__unix__) || defined(__APPLE__)
            ssize_t n = recv(fd, (char*)buf + len, need - len, 0);
            if (n < 0 && errno == EINTR) continue;
#else
            size_t chunk = need - len;
            if (chunk > 65536U) chunk = 65536U;
            int n = recv(fd, (char*)buf + len, (int)chunk, 0);
#endif
            if (n <= 0) break;
            len += (size_t)n;
        }
    } else if (is_chunked) {
        /* Read until terminating 0\r\n\r\n chunk marker (bounded by max_body_bytes) */
        size_t max_total = hdr_end + max_body_bytes;
        while (len < max_total) {
            const uint8_t* body_ptr = buf + hdr_end;
            size_t body_len = len - hdr_end;
            if (body_len >= 5 && joss_memmem_bytes(body_ptr, body_len, (const uint8_t*)"0\r\n\r\n", 5) != NULL) {
                break;
            }
            if (len + 4096U > cap) {
                size_t ncap = cap * 2U;
                if (ncap > max_total) ncap = max_total;
                if (ncap <= cap) break;
                uint8_t* nb = (uint8_t*)realloc(buf, ncap + 1);
                if (!nb) break;
                buf = nb;
                cap = ncap;
            }
#if defined(__unix__) || defined(__APPLE__)
            ssize_t n = recv(fd, (char*)buf + len, cap - len, 0);
            if (n < 0 && errno == EINTR) continue;
#else
            int n = recv(fd, (char*)buf + len, (int)(cap - len), 0);
#endif
            if (n <= 0) break;
            len += (size_t)n;
        }
        /* Decode chunked encoding in place into out_buf */
        uint8_t* decoded = (uint8_t*)malloc(len + 1);
        if (decoded) {
            memcpy(decoded, buf, hdr_end);
            size_t dlen = hdr_end;
            size_t p = hdr_end;
            while (p < len) {
                const uint8_t* crlf = joss_memmem_bytes(buf + p, len - p, (const uint8_t*)"\r\n", 2);
                if (!crlf) break;
                size_t csz = (size_t)strtoul((const char*)(buf + p), NULL, 16);
                p = (size_t)(crlf - buf) + 2U;
                if (csz == 0) break;
                if (p + csz > len) csz = len - p;
                memcpy(decoded + dlen, buf + p, csz);
                dlen += csz;
                p += csz;
                if (p + 2 <= len && buf[p] == '\r' && buf[p + 1] == '\n') p += 2;
            }
            free(buf);
            buf = decoded;
            len = dlen;
        }
    }

    buf[len] = '\0';
    void* result = NULL;
    if (memchr(buf, '\0', len) != NULL) {
        result = joss_bytes_new(buf, len);
    } else {
        char* req_copy = (char*)joss_req_alloc(len + 1);
        if (req_copy) memcpy(req_copy, buf, len + 1);
        result = req_copy ? (void*)req_copy : (void*)"";
    }
    free(buf);
    return (int64_t)(uintptr_t)result;
}

typedef void (*JossConnHandlerFn)(void* client_sock);

#ifdef JOSS_BUILD_GUI
extern void joss_gui_launch_url(const char* title, const char* url, int64_t width, int64_t height);

static bool joss_should_launch_gui_window(void) {
    const char* prog_start = getenv("JOSS_PROGRAM_START");
    if (prog_start && (strcmp(prog_start, "true") == 0 || strcmp(prog_start, "1") == 0)) {
        return false;
    }
    const char* env_gui = joss_env("JOSS_GUI", "");
    if (env_gui && (strcmp(env_gui, "false") == 0 || strcmp(env_gui, "0") == 0)) {
        return false;
    }
    return true;
}
#endif

#define JOSS_HTTP_QUEUE_CAP 512

typedef struct {
    void*             queue[JOSS_HTTP_QUEUE_CAP];
    int               head;
    int               tail;
    int               count;
    bool              stopping;
    JossConnHandlerFn handler;
#if defined(__unix__) || defined(__APPLE__)
    pthread_mutex_t   mu;
    pthread_cond_t    not_empty;
    pthread_cond_t    not_full;
#elif defined(_WIN32)
    CRITICAL_SECTION   cs;
    CONDITION_VARIABLE not_empty;
    CONDITION_VARIABLE not_full;
#endif
} JossHttpWorkerPool;

#if defined(__unix__) || defined(__APPLE__)
static void* joss_http_worker_main(void* arg) {
    JossHttpWorkerPool* pool = (JossHttpWorkerPool*)arg;
    for (;;) {
        pthread_mutex_lock(&pool->mu);
        while (pool->count == 0 && !pool->stopping) {
            pthread_cond_wait(&pool->not_empty, &pool->mu);
        }
        if (pool->count == 0 && pool->stopping) {
            pthread_mutex_unlock(&pool->mu);
            break;
        }
        void* client = pool->queue[pool->head];
        pool->head = (pool->head + 1) % JOSS_HTTP_QUEUE_CAP;
        pool->count--;
        pthread_cond_signal(&pool->not_full);
        pthread_mutex_unlock(&pool->mu);

        joss_req_arena_begin();
        joss_try_reset();
        joss_req_state_reset();
        if (pool->handler) {
            pool->handler(client);
        }
        joss_req_state_reset();
        joss_req_arena_reset();
    }
    return NULL;
}
#elif defined(_WIN32)
static DWORD WINAPI joss_http_worker_main(LPVOID arg) {
    JossHttpWorkerPool* pool = (JossHttpWorkerPool*)arg;
    for (;;) {
        EnterCriticalSection(&pool->cs);
        while (pool->count == 0 && !pool->stopping) {
            SleepConditionVariableCS(&pool->not_empty, &pool->cs, INFINITE);
        }
        if (pool->count == 0 && pool->stopping) {
            LeaveCriticalSection(&pool->cs);
            break;
        }
        void* client = pool->queue[pool->head];
        pool->head = (pool->head + 1) % JOSS_HTTP_QUEUE_CAP;
        pool->count--;
        WakeConditionVariable(&pool->not_full);
        LeaveCriticalSection(&pool->cs);

        joss_req_arena_begin();
        joss_try_reset();
        joss_req_state_reset();
        if (pool->handler) {
            pool->handler(client);
        }
        joss_req_state_reset();
        joss_req_arena_reset();
    }
    return 0;
}
#endif

#if defined(__unix__) || defined(__APPLE__)
typedef struct {
    int                 sfd;
    JossHttpWorkerPool* pool;
} JossHttpAcceptCtx;

static void* joss_http_accept_loop(void* arg) {
    JossHttpAcceptCtx* ctx = (JossHttpAcceptCtx*)arg;
    int sfd = ctx->sfd;
    JossHttpWorkerPool* pool = ctx->pool;
    for (;;) {
        int cfd = accept(sfd, NULL, NULL);
        if (cfd < 0) {
            if (errno == EINTR) continue;
            break;
        }
        intptr_t h = (intptr_t)(cfd + 1);
        joss_configure_client_socket(h);
        joss_register_wrapped_socket(h);
        pthread_mutex_lock(&pool->mu);
        while (pool->count == JOSS_HTTP_QUEUE_CAP && !pool->stopping) {
            pthread_cond_wait(&pool->not_full, &pool->mu);
        }
        if (pool->stopping) {
            pthread_mutex_unlock(&pool->mu);
            close(cfd);
            break;
        }
        pool->queue[pool->tail] = (void*)h;
        pool->tail = (pool->tail + 1) % JOSS_HTTP_QUEUE_CAP;
        pool->count++;
        pthread_cond_signal(&pool->not_empty);
        pthread_mutex_unlock(&pool->mu);
    }
    return NULL;
}
#elif defined(_WIN32)
typedef struct {
    SOCKET              sfd;
    JossHttpWorkerPool* pool;
} JossHttpAcceptCtx;

static DWORD WINAPI joss_http_accept_loop(LPVOID arg) {
    JossHttpAcceptCtx* ctx = (JossHttpAcceptCtx*)arg;
    SOCKET sfd = ctx->sfd;
    JossHttpWorkerPool* pool = ctx->pool;
    for (;;) {
        SOCKET cfd = accept(sfd, NULL, NULL);
        if (cfd == INVALID_SOCKET) {
            break;
        }
        intptr_t h = (intptr_t)(cfd + 1);
        joss_configure_client_socket(h);
        joss_register_wrapped_socket(h);
        EnterCriticalSection(&pool->cs);
        while (pool->count == JOSS_HTTP_QUEUE_CAP && !pool->stopping) {
            SleepConditionVariableCS(&pool->not_full, &pool->cs, INFINITE);
        }
        if (pool->stopping) {
            LeaveCriticalSection(&pool->cs);
            closesocket(cfd);
            break;
        }
        pool->queue[pool->tail] = (void*)(uintptr_t)h;
        pool->tail = (pool->tail + 1) % JOSS_HTTP_QUEUE_CAP;
        pool->count++;
        WakeConditionVariable(&pool->not_empty);
        LeaveCriticalSection(&pool->cs);
    }
    return 0;
}
#endif

void joss___joss_http_serve_concurrent(void* srv, void* handler_fn_ptr) {
    srv = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)srv);
    if (!srv || !handler_fn_ptr) return;
    joss_gc_set_multithreaded(true);
#if defined(__unix__) || defined(__APPLE__)
    int sfd = (int)(intptr_t)srv - 1;
    if (sfd < 0) return;
    signal(SIGPIPE, SIG_IGN);
    JossHttpWorkerPool* pool = (JossHttpWorkerPool*)calloc(1, sizeof(JossHttpWorkerPool));
    if (!pool) return;
    pool->handler = (JossConnHandlerFn)handler_fn_ptr;
    pthread_mutex_init(&pool->mu, NULL);
    pthread_cond_init(&pool->not_empty, NULL);
    pthread_cond_init(&pool->not_full, NULL);

    long ncpu = sysconf(_SC_NPROCESSORS_ONLN);
    if (ncpu < 4) ncpu = 4;
    long nworkers = ncpu * 4;
    if (nworkers < 16) nworkers = 16;
    if (nworkers > 64) nworkers = 64;
    for (long i = 0; i < nworkers; i++) {
        pthread_t tid;
        if (pthread_create(&tid, NULL, joss_http_worker_main, pool) == 0) {
            pthread_detach(tid);
        }
    }
    JossHttpAcceptCtx* accept_ctx = (JossHttpAcceptCtx*)calloc(1, sizeof(JossHttpAcceptCtx));
    if (!accept_ctx) return;
    accept_ctx->sfd = sfd;
    accept_ctx->pool = pool;
#ifdef JOSS_BUILD_GUI
    if (joss_should_launch_gui_window()) {
        pthread_t accept_tid;
        if (pthread_create(&accept_tid, NULL, joss_http_accept_loop, accept_ctx) == 0) {
            pthread_detach(accept_tid);
            struct sockaddr_in sin;
            socklen_t sin_len = (socklen_t)sizeof(sin);
            unsigned short bound_port = 8080;
            if (getsockname(sfd, (struct sockaddr*)&sin, &sin_len) == 0 && sin.sin_port != 0) {
                bound_port = ntohs(sin.sin_port);
            }
            char url[128];
            snprintf(url, sizeof(url), "http://127.0.0.1:%u", (unsigned)bound_port);
            const char* title = joss_env("APP_NAME", "Joss Application");
            if (!title || !*title) title = "Joss Application";
            joss_gui_launch_url(title, url, 1120, 760);
            pthread_mutex_lock(&pool->mu);
            pool->stopping = true;
            pthread_cond_broadcast(&pool->not_empty);
            pthread_cond_broadcast(&pool->not_full);
            pthread_mutex_unlock(&pool->mu);
            close(sfd);
            return;
        }
    }
#endif
    joss_http_accept_loop(accept_ctx);
    free(accept_ctx);
#elif defined(_WIN32)
    SOCKET sfd = (SOCKET)((uintptr_t)srv - 1);
    JossHttpWorkerPool* pool = (JossHttpWorkerPool*)calloc(1, sizeof(JossHttpWorkerPool));
    if (!pool) return;
    pool->handler = (JossConnHandlerFn)handler_fn_ptr;
    InitializeCriticalSection(&pool->cs);
    InitializeConditionVariable(&pool->not_empty);
    InitializeConditionVariable(&pool->not_full);

    SYSTEM_INFO si;
    GetSystemInfo(&si);
    DWORD ncpu = si.dwNumberOfProcessors;
    if (ncpu < 4) ncpu = 4;
    DWORD nworkers = ncpu * 4;
    if (nworkers < 16) nworkers = 16;
    if (nworkers > 64) nworkers = 64;
    for (DWORD i = 0; i < nworkers; i++) {
        HANDLE th = CreateThread(NULL, 0, joss_http_worker_main, pool, 0, NULL);
        if (th) CloseHandle(th);
    }
    JossHttpAcceptCtx* accept_ctx = (JossHttpAcceptCtx*)calloc(1, sizeof(JossHttpAcceptCtx));
    if (!accept_ctx) return;
    accept_ctx->sfd = sfd;
    accept_ctx->pool = pool;
#ifdef JOSS_BUILD_GUI
    if (joss_should_launch_gui_window()) {
        HANDLE accept_th = CreateThread(NULL, 0, joss_http_accept_loop, accept_ctx, 0, NULL);
        if (accept_th) {
            CloseHandle(accept_th);
            struct sockaddr_in sin;
            int sin_len = (int)sizeof(sin);
            unsigned short bound_port = 8080;
            if (getsockname(sfd, (struct sockaddr*)&sin, &sin_len) == 0 && sin.sin_port != 0) {
                bound_port = ntohs(sin.sin_port);
            }
            char url[128];
            snprintf(url, sizeof(url), "http://127.0.0.1:%u", (unsigned)bound_port);
            const char* title = joss_env("APP_NAME", "Joss Application");
            if (!title || !*title) title = "Joss Application";
            joss_gui_launch_url(title, url, 1120, 760);
            EnterCriticalSection(&pool->cs);
            pool->stopping = true;
            WakeAllConditionVariable(&pool->not_empty);
            WakeAllConditionVariable(&pool->not_full);
            LeaveCriticalSection(&pool->cs);
            closesocket(sfd);
            return;
        }
    }
#endif
    joss_http_accept_loop(accept_ctx);
    free(accept_ctx);
#else
    (void)srv;
    (void)handler_fn_ptr;
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
    recv = (void*)unbox_web_cstr((const char*)recv);
    k = unbox_web_cstr(k);
    if (k && k[0]) {
        const char* sv = joss_strval(v);
        if (!g_joss_session_flash) g_joss_session_flash = joss_map_new();
        joss_map_set_str(g_joss_session_flash, k, sv ? sv : "");
        if (!g_joss_new_flash) g_joss_new_flash = joss_map_new();
        joss_map_set_str(g_joss_new_flash, k, sv ? sv : "");
    }
    return recv;
}

void* withCookie(void* recv, const char* k, const char* v) {
    recv = (void*)unbox_web_cstr((const char*)recv);
    k = unbox_web_cstr(k);
    v = unbox_web_cstr(v);
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
    joss_sb_append(sb, "; Path=/; HttpOnly; SameSite=Lax; Max-Age=");
    if (!v || !v[0]) {
        joss_sb_append(sb, "0");
    } else {
        int64_t ttl = strtoll(joss_env("JWT_TTL_SECONDS", joss_env("SESSION_TTL_SECONDS", "2592000")), NULL, 10);
        if (ttl <= 0) ttl = 2592000;
        joss_sb_append_i64(sb, ttl);
    }
    const char* app_env = joss_env("APP_ENV", "");
    const char* sess_sec = joss_env("SESSION_SECURE", "");
    if ((app_env && strcasecmp(app_env, "production") == 0) || (sess_sec && strcasecmp(sess_sec, "true") == 0)) {
        joss_sb_append(sb, "; Secure");
    }
    joss_sb_append(sb, sep);
    char* out = joss_sb_to_req_string(sb);
    joss_sb_free(sb);
    return out;
}

typedef int64_t (*JossUnaryCb)(int64_t);

void* onSuccess(void* recv, int64_t cb) {
    void* obj = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)recv);
    if (obj && joss_is_map_ptr((int64_t)(uintptr_t)obj) && joss_is_not_null(cb)) {
        bool ok = joss_unbox_value(joss_map_get_i64(obj, "success")) != 0;
        bool req2fa = joss_unbox_value(joss_map_get_i64(obj, "requires_2fa")) != 0;
        if (ok && !req2fa) {
            void* fn = joss_closure_prepare_call((void*)(uintptr_t)cb);
            if (fn) {
                int64_t jwt = joss_map_get_i64(obj, "jwt");
                int64_t res = ((JossUnaryCb)fn)(jwt);
                joss_map_set_i64(obj, "_response", res);
            }
        }
    }
    return recv;
}

void* onChallenge(void* recv, int64_t cb) {
    void* obj = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)recv);
    if (obj && joss_is_map_ptr((int64_t)(uintptr_t)obj) && joss_is_not_null(cb)) {
        bool ok = joss_unbox_value(joss_map_get_i64(obj, "success")) != 0;
        bool req2fa = joss_unbox_value(joss_map_get_i64(obj, "requires_2fa")) != 0;
        if (ok && req2fa) {
            void* fn = joss_closure_prepare_call((void*)(uintptr_t)cb);
            if (!fn) return recv;
            int64_t uid = joss_intval(joss_map_get_i64(obj, "user_id"));
            const char* email = joss_map_get_str(obj, "email");
            const char* tok = joss_map_get_str(obj, "challenge_token");
            if (!tok || !tok[0]) {
                char payload[512];
                int64_t now = (int64_t)time(NULL);
                snprintf(payload, sizeof(payload),
                         "{\"token_type\":\"mfa_challenge\",\"user_id\":%lld,\"sub\":%lld,\"email\":\"%s\",\"jti\":\"mfa_%lld_%lld\",\"iat\":%lld,\"exp\":%lld}",
                         (long long)uid, (long long)uid, email ? email : "", (long long)uid, (long long)now, (long long)now, (long long)(now + 300));
                const char* sec = joss_env("JWT_SECRET", "");
                if (!sec || !sec[0]) sec = joss_env("APP_KEY", "");
                if (!sec || !sec[0]) sec = "joss_ephemeral_jwt_key";
                tok = payload;
                /* Sign HS256 inline using joss_prim_hmac_sha256 */
                static const char B64U[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
                const char* hdr_json = "{\"alg\":\"HS256\",\"typ\":\"JWT\"}";
                char h_b64[64];
                joss_prim_base64_encode_alphabet((const uint8_t*)hdr_json, strlen(hdr_json), B64U, false, h_b64);
                size_t plen = strlen(payload);
                char* p_b64 = (char*)malloc(plen * 2 + 16);
                if (p_b64) {
                    joss_prim_base64_encode_alphabet((const uint8_t*)payload, plen, B64U, false, p_b64);
                    size_t mlen = strlen(h_b64) + 1 + strlen(p_b64);
                    char* msg = (char*)malloc(mlen + 1);
                    if (msg) {
                        snprintf(msg, mlen + 1, "%s.%s", h_b64, p_b64);
                        uint8_t mac[32];
                        joss_prim_hmac_sha256((const uint8_t*)sec, strlen(sec), (const uint8_t*)msg, mlen, mac);
                        char s_b64[64];
                        joss_prim_base64_encode_alphabet(mac, 32, B64U, false, s_b64);
                        char* full = (char*)malloc(mlen + 1 + strlen(s_b64) + 1);
                        if (full) {
                            snprintf(full, mlen + 1 + strlen(s_b64) + 1, "%s.%s", msg, s_b64);
                            tok = full;
                        }
                        free(msg);
                    }
                    free(p_b64);
                }
            }
            int64_t res = ((JossUnaryCb)fn)((int64_t)(uintptr_t)tok);
            joss_map_set_i64(obj, "_response", res);
        }
    }
    return recv;
}

void* onFail(void* recv, int64_t cb) {
    void* obj = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)recv);
    if (obj && joss_is_map_ptr((int64_t)(uintptr_t)obj) && joss_is_not_null(cb)) {
        bool ok = joss_unbox_value(joss_map_get_i64(obj, "success")) != 0;
        if (!ok) {
            void* fn = joss_closure_prepare_call((void*)(uintptr_t)cb);
            if (fn) {
                int64_t err_v = joss_map_get_i64(obj, "error");
                int64_t res = ((JossUnaryCb)fn)(err_v);
                joss_map_set_i64(obj, "_response", res);
            }
        }
    }
    return recv;
}

void* response(void* recv) {
    void* obj = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)recv);
    if (obj && joss_is_map_ptr((int64_t)(uintptr_t)obj)) {
        int64_t r = joss_map_get_i64(obj, "_response");
        if (r != 0) return (void*)(uintptr_t)r;
    }
    return recv;
}

const char* html_escape(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    size_t n = strlen(s);
    char* out = (char*)joss_req_alloc(n * 6 + 1);
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
    const char* tok = "";
    void* sess = joss_get_active_session();
    if (sess && joss_is_map_ptr((int64_t)(uintptr_t)sess)) {
        const char* s = joss_map_get_str(sess, "csrf_token");
        if (s && s[0]) {
            tok = s;
        } else {
            s = joss_map_get_str(sess, "_csrf_token");
            if (s && s[0]) tok = s;
        }
    }
    const char* escaped = html_escape(tok);
    size_t len = strlen(escaped);
    const char* prefix = "<input type=\"hidden\" name=\"_token\" value=\"";
    const char* suffix = "\">";
    size_t plen = strlen(prefix);
    size_t slen = strlen(suffix);
    char* out = (char*)joss_req_alloc(plen + len + slen + 1);
    if (!out) return "<input type=\"hidden\" name=\"_token\" value=\"\">";
    memcpy(out, prefix, plen);
    memcpy(out + plen, escaped, len);
    memcpy(out + plen + len, suffix, slen + 1);
    return out;
}

const char* joss_csrf_field(void) {
    return csrf_field();
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
    char* out = joss_sb_to_req_string(sb);
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

/* =========================================================================
 * Native C11 Host Services: Cache, Markdown, Response, System::Run,
 * Http, WebSocket, AI, Notify & Plugin
 * ========================================================================= */

static void* g_joss_cache_values = NULL;
static void* g_joss_cache_expiry = NULL;
static void* g_joss_cache_owners = NULL;

static void joss_cache_init(void) {
    bool _prev_arena = joss_req_arena_pause();
    if (!g_joss_cache_values) {
        g_joss_cache_values = joss_map_new();
        joss_gc_add_root(&g_joss_cache_values);
    }
    if (!g_joss_cache_expiry) {
        g_joss_cache_expiry = joss_map_new();
        joss_gc_add_root(&g_joss_cache_expiry);
    }
    if (!g_joss_cache_owners) {
        g_joss_cache_owners = joss_map_new();
        joss_gc_add_root(&g_joss_cache_owners);
    }
    joss_req_arena_resume(_prev_arena);
}

static void joss_cache_remove_locked(const char* key) {
    if (!key) return;
    if (g_joss_cache_owners && joss_array_key_exists(key, g_joss_cache_owners)) {
        JossOwnedScope* old_owner = (JossOwnedScope*)(uintptr_t)joss_map_get_i64(g_joss_cache_owners, key);
        joss_map_delete_i64(g_joss_cache_owners, key);
        if (old_owner) joss_owned_scope_destroy(old_owner);
    }
    if (g_joss_cache_values) joss_map_delete_i64(g_joss_cache_values, key);
    if (g_joss_cache_expiry) joss_map_delete_i64(g_joss_cache_expiry, key);
}

static int64_t joss_cache_has_locked(const char* key) {
    if (!key) return 0;
    joss_cache_init();
    if (!joss_array_key_exists(key, g_joss_cache_values)) return 0;
    int64_t exp = joss_map_get_i64(g_joss_cache_expiry, key);
    if (exp > 0 && (int64_t)time(NULL) > exp) {
        joss_cache_remove_locked(key);
        return 0;
    }
    return 1;
}

static int64_t joss_clone_session_value(int64_t v);

int64_t fn_Cache_put(const char* key, int64_t val, int64_t ttl_seconds) {
    key = unbox_web_cstr(key);
    if (!key) return 0;
    joss_global_lock(JOSS_LOCK_CACHE);
    joss_cache_init();
    JossOwnedScope* owner = joss_owned_scope_begin();
    int64_t persisted_val = joss_clone_session_value(val);
    joss_owned_scope_end(owner);
    bool _prev_arena = joss_req_arena_pause();
    if (joss_array_key_exists(key, g_joss_cache_owners)) {
        JossOwnedScope* old_owner = (JossOwnedScope*)(uintptr_t)joss_map_get_i64(g_joss_cache_owners, key);
        if (old_owner) joss_owned_scope_destroy(old_owner);
    }
    joss_map_set_i64(g_joss_cache_values, key, persisted_val);
    joss_map_set_i64(g_joss_cache_owners, key, (int64_t)(uintptr_t)owner);
    int64_t exp = (ttl_seconds > 0) ? ((int64_t)time(NULL) + ttl_seconds) : 0;
    joss_map_set_i64(g_joss_cache_expiry, key, exp);
    joss_req_arena_resume(_prev_arena);
    joss_global_unlock(JOSS_LOCK_CACHE);
    return 1;
}

int64_t fn_Cache_has(const char* key) {
    key = unbox_web_cstr(key);
    if (!key) return 0;
    joss_global_lock(JOSS_LOCK_CACHE);
    int64_t ok = joss_cache_has_locked(key);
    joss_global_unlock(JOSS_LOCK_CACHE);
    return ok;
}

int64_t fn_Cache_get(const char* key, int64_t defVal) {
    key = unbox_web_cstr(key);
    if (!key) return defVal;
    joss_global_lock(JOSS_LOCK_CACHE);
    int64_t val = defVal;
    if (joss_cache_has_locked(key)) {
        /* Clone into the caller's active request arena (or Mini-GC) so the cache's
         * private JossOwnedScope can always be safely destroyed on overwrite/expiry/forget. */
        val = joss_clone_session_value(joss_map_get_i64(g_joss_cache_values, key));
    }
    joss_global_unlock(JOSS_LOCK_CACHE);
    return val;
}

int64_t fn_Cache_forget(const char* key) {
    key = unbox_web_cstr(key);
    if (!key) return 0;
    joss_global_lock(JOSS_LOCK_CACHE);
    joss_cache_init();
    joss_cache_remove_locked(key);
    joss_global_unlock(JOSS_LOCK_CACHE);
    return 1;
}

/* Markdown::readFile and Markdown::toHtml now live in Pure Joss (std/markdown/markdown.joss). */

const char* fn_Response_json(int64_t data, int64_t status) {
    return joss_json(data, status);
}

const char* fn_Response_raw(const char* body, int64_t status, const char* content_type) {
    size_t bin_len = 0;
    const uint8_t* bin_data = joss_bytes_unwrap(body, &bin_len);
    if (!bin_data) {
        body = unbox_web_cstr(body);
        if (!body) body = "";
    }
    status = joss_intval(status);
    if (status < 100 || status > 599) status = 200;
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 ");
    joss_sb_append_i64(sb, status);
    joss_sb_append(sb, " OK\r\n");
    int has_ct = 0;
    int64_t hv = (int64_t)(uintptr_t)content_type;
    if (joss_is_map_ptr(hv)) {
        /* Headers map: emit every header (parity with Response::raw in pkg/core). */
        int64_t n = joss_map_len((void*)content_type);
        for (int64_t i = 0; i < n; i++) {
            const char* k = unbox_web_cstr(joss_map_key_at((void*)content_type, i));
            const char* v = unbox_web_cstr((const char*)(uintptr_t)joss_map_val_at((void*)content_type, i));
            if (!k || !*k || !v) continue;
            if (strcasecmp(k, "Content-Type") == 0) has_ct = 1;
            joss_sb_append(sb, k);
            joss_sb_append(sb, ": ");
            joss_sb_append(sb, v);
            joss_sb_append(sb, "\r\n");
        }
    } else {
        content_type = unbox_web_cstr(content_type);
        if (content_type && *content_type) {
            joss_sb_append(sb, "Content-Type: ");
            joss_sb_append(sb, content_type);
            joss_sb_append(sb, "\r\n");
            has_ct = 1;
        }
    }
    if (!has_ct) joss_sb_append(sb, "Content-Type: text/plain; charset=utf-8\r\n");
    if (bin_data) {
        joss_sb_append(sb, "Content-Length: ");
        joss_sb_append_i64(sb, (int64_t)bin_len);
        joss_sb_append(sb, "\r\nConnection: close\r\n\r\n");
        joss_buf_append_bytes(sb, bin_data, (int64_t)bin_len);
        void* out = joss_bytes_new(sb->data, (size_t)sb->len);
        joss_sb_free(sb);
        return (const char*)out;
    }
    joss_sb_append(sb, "Connection: close\r\n\r\n");
    joss_sb_append(sb, body);
    char* out = joss_sb_to_req_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* fn_Response_error(const char* msg, int64_t status) {
    msg = unbox_web_cstr(msg);
    status = joss_intval(status);
    if (status < 100 || status > 599) status = 500;
    return fn_Response_raw(msg ? msg : "Internal Server Error", status, "text/plain; charset=utf-8");
}

const char* fn_Response_download(const char* path, const char* filename) {
    path = unbox_web_cstr(path);
    filename = unbox_web_cstr(filename);
    const char* content = joss_file_get_contents(path);
    if (!filename || !*filename) filename = "download.bin";
    size_t bin_len = 0;
    const uint8_t* bin_data = joss_bytes_unwrap(content, &bin_len);
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=\"");
    joss_sb_append(sb, filename);
    joss_sb_append(sb, "\"\r\nConnection: close\r\n\r\n");
    if (bin_data) {
        joss_buf_append_bytes(sb, bin_data, (int64_t)bin_len);
        void* out = joss_bytes_new(sb->data, (size_t)sb->len);
        joss_sb_free(sb);
        return (const char*)out;
    }
    joss_sb_append(sb, content ? content : "");
    char* out = joss_sb_to_req_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* fn_Response_stream(const char* body, const char* content_type) {
    return fn_Response_raw(body, 200, content_type);
}

const char* fn_Response_redirect(const char* url, int64_t status) {
    return joss_redirect(url, status);
}

const char* fn_Response_back(void) {
    return joss_back();
}

void* fn_WebResponse_with(void* resp, const char* key, int64_t value) {
    return with(resp, key, value);
}

void* fn_WebResponse_withCookie(void* resp, const char* name, const char* value) {
    return withCookie(resp, unbox_web_cstr(name), unbox_web_cstr(value));
}

#if defined(_WIN32)
static int joss_win_run_hidden_cmd(const char* cmd) {
    if (!cmd || !*cmd) return -1;
    const char* comspec = getenv("COMSPEC");
    if (!comspec || !*comspec) comspec = "cmd.exe";
    size_t clen = strlen(comspec) + strlen(cmd) + 16;
    char* cmdline = (char*)malloc(clen);
    if (!cmdline) return -1;
    snprintf(cmdline, clen, "\"%s\" /c %s", comspec, cmd);

    STARTUPINFOA si;
    PROCESS_INFORMATION pi;
    memset(&si, 0, sizeof(si));
    memset(&pi, 0, sizeof(pi));
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;

    BOOL ok = CreateProcessA(NULL, cmdline, NULL, NULL, FALSE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi);
    free(cmdline);
    if (!ok) return -1;
    WaitForSingleObject(pi.hProcess, INFINITE);
    DWORD exit_code = 1;
    GetExitCodeProcess(pi.hProcess, &exit_code);
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);
    return (int)exit_code;
}
#endif

static char* joss_system_run_bytes(const char* cmd, size_t* out_len) {
    if (out_len) *out_len = 0;
    cmd = unbox_web_cstr(cmd);
    if (!cmd || !*cmd) return NULL;
#if defined(_WIN32)
    const char* comspec = getenv("COMSPEC");
    if (!comspec || !*comspec) comspec = "cmd.exe";
    size_t clen = strlen(comspec) + strlen(cmd) + 16;
    char* cmdline = (char*)malloc(clen);
    if (!cmdline) return NULL;
    snprintf(cmdline, clen, "\"%s\" /c %s", comspec, cmd);

    SECURITY_ATTRIBUTES sa;
    memset(&sa, 0, sizeof(sa));
    sa.nLength = sizeof(sa);
    sa.bInheritHandle = TRUE;
    HANDLE hRead = NULL, hWrite = NULL;
    if (!CreatePipe(&hRead, &hWrite, &sa, 0)) {
        free(cmdline);
        return NULL;
    }
    SetHandleInformation(hRead, HANDLE_FLAG_INHERIT, 0);

    STARTUPINFOA si;
    PROCESS_INFORMATION pi;
    memset(&si, 0, sizeof(si));
    memset(&pi, 0, sizeof(pi));
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESTDHANDLES | STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;
    si.hStdOutput = hWrite;
    si.hStdError = GetStdHandle(STD_ERROR_HANDLE);
    si.hStdInput = GetStdHandle(STD_INPUT_HANDLE);

    BOOL ok = CreateProcessA(NULL, cmdline, NULL, NULL, TRUE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi);
    free(cmdline);
    CloseHandle(hWrite);
    if (!ok) {
        CloseHandle(hRead);
        return NULL;
    }

    JossStringBuilder* sb = joss_sb_new();
    char buf[4096];
    DWORD nread = 0;
    while (ReadFile(hRead, buf, sizeof(buf), &nread, NULL) && nread > 0) {
        joss_buf_append_bytes(sb, (const uint8_t*)buf, (int64_t)nread);
    }
    CloseHandle(hRead);
    WaitForSingleObject(pi.hProcess, INFINITE);
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);
    if (out_len) *out_len = (size_t)sb->len;
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
#else
    FILE* pipe = popen(cmd, "r");
    if (!pipe) return NULL;
    JossStringBuilder* sb = joss_sb_new();
    char buf[4096];
    size_t n;
    while ((n = fread(buf, 1, sizeof(buf), pipe)) > 0) {
        joss_buf_append_bytes(sb, (const uint8_t*)buf, (int64_t)n);
    }
    pclose(pipe);
    if (out_len) *out_len = (size_t)sb->len;
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
#endif
}

static bool joss_http_write_temp_file(const uint8_t* data, size_t len, char* out_path, size_t out_cap);
static char* joss_web_strdup(const char* s);

static const char* joss_maybe_materialize_embedded_arg(const char* arg, char* tmp_out, size_t tmp_cap) {
    if (!arg || !arg[0] || !g_joss_embedded_asset_resolver) return arg;
    size_t alen = strlen(arg);
    if (alen == 0 || alen >= 400 || strchr(arg, '\n') != NULL || strchr(arg, '\r') != NULL) return arg;
    /* If it already exists on disk, use it directly. */
    FILE* f = fopen(arg, "rb");
    if (f) {
        fclose(f);
        return arg;
    }
    const char* emb = joss_lookup_embedded_priv_or_asset(arg, true);
    if (!emb || !emb[0]) return arg;
    if (joss_http_write_temp_file((const uint8_t*)emb, strlen(emb), tmp_out, tmp_cap)) {
        return tmp_out;
    }
    return arg;
}

#if defined(_WIN32)
static void joss_win_append_quoted_arg(JossStringBuilder* sb, const char* arg);

static char* joss_system_run_argv(const char* prog, void* args_arr) {
    if (!prog || !*prog) return NULL;
    JossStringBuilder* cmd_sb = joss_sb_new();
    joss_win_append_quoted_arg(cmd_sb, prog);
    int64_t argc = (args_arr && joss_is_arr_ptr((int64_t)(uintptr_t)args_arr)) ? joss_arr_len(args_arr) : 0;
    char** temp_files = (argc > 0) ? (char**)calloc((size_t)argc, sizeof(char*)) : NULL;
    for (int64_t i = 0; i < argc; i++) {
        joss_buf_append_byte(cmd_sb, ' ');
        const char* a = joss_strval(joss_arr_get_i64(args_arr, i));
        char tmp_path[512];
        tmp_path[0] = '\0';
        const char* resolved = joss_maybe_materialize_embedded_arg(a ? a : "", tmp_path, sizeof(tmp_path));
        if (resolved == tmp_path && temp_files) {
            temp_files[i] = joss_web_strdup(tmp_path);
        }
        joss_win_append_quoted_arg(cmd_sb, resolved ? resolved : "");
    }
    char* cmdline = joss_sb_to_string(cmd_sb);
    joss_sb_free(cmd_sb);
    if (!cmdline) {
        if (temp_files) {
            for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
            free(temp_files);
        }
        return NULL;
    }

    SECURITY_ATTRIBUTES sa;
    memset(&sa, 0, sizeof(sa));
    sa.nLength = sizeof(sa);
    sa.bInheritHandle = TRUE;
    HANDLE hRead = NULL, hWrite = NULL;
    if (!CreatePipe(&hRead, &hWrite, &sa, 0)) {
        free(cmdline);
        if (temp_files) {
            for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
            free(temp_files);
        }
        return NULL;
    }
    SetHandleInformation(hRead, HANDLE_FLAG_INHERIT, 0);

    STARTUPINFOA si;
    PROCESS_INFORMATION pi;
    memset(&si, 0, sizeof(si));
    memset(&pi, 0, sizeof(pi));
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESTDHANDLES;
    si.hStdOutput = hWrite;
    si.hStdError = hWrite;
    si.hStdInput = GetStdHandle(STD_INPUT_HANDLE);

    BOOL ok = CreateProcessA(NULL, cmdline, NULL, NULL, TRUE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi);
    free(cmdline);
    CloseHandle(hWrite);
    if (!ok) {
        CloseHandle(hRead);
        if (temp_files) {
            for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
            free(temp_files);
        }
        return NULL;
    }

    JossStringBuilder* out_sb = joss_sb_new();
    char buf[4096];
    DWORD nread = 0;
    while (ReadFile(hRead, buf, sizeof(buf), &nread, NULL) && nread > 0) {
        joss_buf_append_bytes(out_sb, (const uint8_t*)buf, (int64_t)nread);
    }
    CloseHandle(hRead);
    WaitForSingleObject(pi.hProcess, 30000);
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);
    if (temp_files) {
        for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
        free(temp_files);
    }
    char* out = joss_sb_to_string(out_sb);
    joss_sb_free(out_sb);
    return out;
}
#else
#include <sys/wait.h>
static char* joss_system_run_argv(const char* prog, void* args_arr) {
    if (!prog || !*prog) return NULL;
    int64_t argc = (args_arr && joss_is_arr_ptr((int64_t)(uintptr_t)args_arr)) ? joss_arr_len(args_arr) : 0;
    char** argv = (char**)calloc((size_t)(argc + 2), sizeof(char*));
    char** temp_files = (argc > 0) ? (char**)calloc((size_t)argc, sizeof(char*)) : NULL;
    if (!argv) { free(temp_files); return NULL; }
    argv[0] = (char*)prog;
    for (int64_t i = 0; i < argc; i++) {
        const char* a = joss_strval(joss_arr_get_i64(args_arr, i));
        char tmp_path[512];
        tmp_path[0] = '\0';
        const char* resolved = joss_maybe_materialize_embedded_arg(a ? a : "", tmp_path, sizeof(tmp_path));
        if (resolved == tmp_path && temp_files) {
            temp_files[i] = joss_web_strdup(tmp_path);
            argv[i + 1] = temp_files[i];
        } else {
            argv[i + 1] = (char*)(resolved ? resolved : "");
        }
    }
    argv[argc + 1] = NULL;

    int pipefd[2];
    if (pipe(pipefd) != 0) {
        free(argv);
        if (temp_files) {
            for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
            free(temp_files);
        }
        return NULL;
    }
    pid_t pid = fork();
    if (pid == 0) {
        close(pipefd[0]);
        dup2(pipefd[1], STDOUT_FILENO);
        dup2(pipefd[1], STDERR_FILENO);
        close(pipefd[1]);
        execvp(prog, argv);
        _exit(127);
    }
    free(argv);
    close(pipefd[1]);
    if (pid < 0) {
        close(pipefd[0]);
        if (temp_files) {
            for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
            free(temp_files);
        }
        return NULL;
    }
    JossStringBuilder* out_sb = joss_sb_new();
    char buf[4096];
    ssize_t nread = 0;
    while ((nread = read(pipefd[0], buf, sizeof(buf))) > 0) {
        joss_buf_append_bytes(out_sb, (const uint8_t*)buf, (int64_t)nread);
    }
    close(pipefd[0]);
    int status = 0;
    waitpid(pid, &status, 0);
    if (temp_files) {
        for (int64_t i = 0; i < argc; i++) { if (temp_files[i]) remove(temp_files[i]); }
        free(temp_files);
    }
    char* out = joss_sb_to_string(out_sb);
    joss_sb_free(out_sb);
    return out;
}
#endif

const char* fn_System_Run(const char* cmd, void* args_arr) {
    cmd = unbox_web_cstr(cmd);
    if (!cmd || !*cmd) return "";
    int64_t raw_args = joss_unbox_value((int64_t)(uintptr_t)args_arr);
    if (raw_args && joss_is_arr_ptr(raw_args)) {
        char* out = joss_system_run_argv(cmd, (void*)(uintptr_t)raw_args);
        return out ? out : "";
    }
    char* out = joss_system_run_bytes(cmd, NULL);
    return out ? out : "";
}

static int append_curl_headers_checked(JossStringBuilder* cmd, void* hdr_map) {
    int has_ct = 0;
    if (!hdr_map || !joss_is_map_ptr((int64_t)(uintptr_t)hdr_map)) return 0;
    int64_t n = joss_map_len(hdr_map);
    for (int64_t i = 0; i < n; i++) {
        const char* k = unbox_web_cstr(joss_map_key_at(hdr_map, i));
        const char* v = unbox_web_cstr((const char*)(uintptr_t)joss_map_val_at(hdr_map, i));
        if (!k || !*k || !v) continue;
        if (strcasecmp(k, "Content-Type") == 0) has_ct = 1;
        joss_sb_append(cmd, " -H \"");
        joss_sb_append(cmd, k);
        joss_sb_append(cmd, ": ");
        for (const char* p = v; *p; p++) {
            if (*p == '"' || *p == '\\') joss_buf_append_byte(cmd, '\\');
            joss_buf_append_byte(cmd, (uint8_t)*p);
        }
        joss_buf_append_byte(cmd, '"');
    }
    return has_ct;
}

static bool joss_http_write_temp_file(const uint8_t* data, size_t len, char* out_path, size_t out_cap) {
    if (!out_path || out_cap < 64) return false;
    static volatile int64_t g_tmp_seq = 0;
#if defined(_WIN32)
    char tmp_dir[MAX_PATH];
    DWORD dlen = GetTempPathA((DWORD)sizeof(tmp_dir), tmp_dir);
    if (dlen == 0 || dlen >= sizeof(tmp_dir)) strcpy(tmp_dir, ".\\");
    DWORD pid = GetCurrentProcessId();
    for (int attempt = 0; attempt < 32; attempt++) {
        LONG seq = (LONG)InterlockedIncrement64(&g_tmp_seq);
        snprintf(out_path, out_cap, "%sjoss_http_%lu_%ld.bin", tmp_dir, (unsigned long)pid, (long)seq);
        HANDLE h = CreateFileA(out_path, GENERIC_WRITE, 0, NULL, CREATE_NEW, FILE_ATTRIBUTE_TEMPORARY, NULL);
        if (h == INVALID_HANDLE_VALUE) continue;
        DWORD written = 0;
        BOOL ok = (len == 0) ? TRUE : WriteFile(h, data, (DWORD)len, &written, NULL);
        CloseHandle(h);
        if (ok && (size_t)written == len) return true;
        remove(out_path);
        return false;
    }
    return false;
#else
    const char* tmp_dir = getenv("TMPDIR");
    if (!tmp_dir || !*tmp_dir) tmp_dir = "/tmp";
    unsigned long pid = (unsigned long)getpid();
    for (int attempt = 0; attempt < 32; attempt++) {
        long long seq = (long long)__sync_add_and_fetch(&g_tmp_seq, 1);
        snprintf(out_path, out_cap, "%s/joss_http_%lu_%lld.bin", tmp_dir, pid, seq);
        int fd = open(out_path, O_WRONLY | O_CREAT | O_EXCL, 0600);
        if (fd < 0) continue;
        size_t off = 0;
        bool ok = true;
        while (off < len) {
            ssize_t w = write(fd, data + off, len - off);
            if (w <= 0) { ok = false; break; }
            off += (size_t)w;
        }
        close(fd);
        if (ok) return true;
        remove(out_path);
        return false;
    }
    return false;
#endif
}

static void append_url_encoded(JossStringBuilder* sb, const char* s) {
    if (!sb || !s) return;
    static const char hex[] = "0123456789ABCDEF";
    for (const unsigned char* p = (const unsigned char*)s; *p; p++) {
        unsigned char c = *p;
        if ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
            c == '-' || c == '_' || c == '.' || c == '~') {
            joss_buf_append_byte(sb, c);
        } else {
            joss_buf_append_byte(sb, '%');
            joss_buf_append_byte(sb, (uint8_t)hex[c >> 4]);
            joss_buf_append_byte(sb, (uint8_t)hex[c & 0x0F]);
        }
    }
}

void* fn_Http_request(const char* method, const char* url, const char* body, void* headers) {
    method = unbox_web_cstr(method);
    url = unbox_web_cstr(url);
    int64_t hdr_raw = joss_unbox_value((int64_t)(uintptr_t)headers);
    headers = (hdr_raw && joss_is_map_ptr(hdr_raw)) ? (void*)(uintptr_t)hdr_raw : NULL;
    void* res_map = joss_map_new();
    joss_map_set_i64(res_map, "status", 0);
    joss_map_set_i64(res_map, "success", (int64_t)(uintptr_t)joss_box_bool(false));
    joss_map_set_str(res_map, "body", "");
    if (!url || !*url) return res_map;
    if (!method || !*method) method = "GET";

    void* opt_map = NULL;
    void* form_map = NULL;
    const uint8_t* bin_body = NULL;
    size_t bin_body_len = 0;
    const char* body_str = NULL;
    int64_t timeout_sec = 30;
    int64_t body_raw = joss_unbox_value((int64_t)(uintptr_t)body);
    if (body_raw && joss_is_map_ptr(body_raw)) {
        opt_map = (void*)(uintptr_t)body_raw;
        if (joss_array_key_exists("body", opt_map)) {
            int64_t bv = joss_map_get_i64(opt_map, "body");
            bin_body = joss_bytes_unwrap((const void*)(uintptr_t)bv, &bin_body_len);
            if (!bin_body) {
                body_str = unbox_web_cstr((const char*)(uintptr_t)bv);
            }
        }
        if (joss_array_key_exists("form", opt_map)) {
            int64_t fm = joss_unbox_value(joss_map_get_i64(opt_map, "form"));
            if (fm && joss_is_map_ptr(fm)) form_map = (void*)(uintptr_t)fm;
        }
        if (!headers && joss_array_key_exists("headers", opt_map)) {
            int64_t hm = joss_unbox_value(joss_map_get_i64(opt_map, "headers"));
            if (hm && joss_is_map_ptr(hm)) headers = (void*)(uintptr_t)hm;
        }
        if (joss_array_key_exists("timeout", opt_map)) {
            int64_t tv = joss_intval(joss_map_get_i64(opt_map, "timeout"));
            if (tv > 0) timeout_sec = tv;
        }
    } else {
        bin_body = joss_bytes_unwrap(body, &bin_body_len);
        if (!bin_body) {
            body_str = unbox_web_cstr(body);
        }
    }

    JossStringBuilder* cmd = joss_sb_new();
    joss_sb_append(cmd, "curl -sS --max-time ");
    joss_sb_append_i64(cmd, timeout_sec);
    joss_sb_append(cmd, " -w \"\\n%{http_code}\" -X \"");
    for (const char* p = method; *p; p++) {
        if (*p == '"' || *p == '\\') joss_buf_append_byte(cmd, '\\');
        joss_buf_append_byte(cmd, (uint8_t)*p);
    }
    joss_buf_append_byte(cmd, '"');
    int has_ct = append_curl_headers_checked(cmd, headers);

    char tmp_body_path[512];
    tmp_body_path[0] = '\0';

    if (form_map) {
        JossStringBuilder* fb = joss_sb_new();
        int64_t fn = joss_map_len(form_map);
        for (int64_t i = 0; i < fn; i++) {
            const char* fk = unbox_web_cstr(joss_map_key_at(form_map, i));
            const char* fv = unbox_web_cstr(joss_strval(joss_map_val_at(form_map, i)));
            if (!fk || !*fk) continue;
            if (fb->len > 0) joss_buf_append_byte(fb, '&');
            append_url_encoded(fb, fk);
            joss_buf_append_byte(fb, '=');
            if (fv) append_url_encoded(fb, fv);
        }
        char* form_str = joss_sb_to_string(fb);
        size_t flen = (size_t)fb->len;
        joss_sb_free(fb);
        if (form_str) {
            if (!has_ct) {
                joss_sb_append(cmd, " -H \"Content-Type: application/x-www-form-urlencoded\"");
            }
            if (joss_http_write_temp_file((const uint8_t*)form_str, flen, tmp_body_path, sizeof(tmp_body_path))) {
                joss_sb_append(cmd, " --data-binary \"@");
                joss_sb_append(cmd, tmp_body_path);
                joss_buf_append_byte(cmd, '"');
            }
            free(form_str);
        }
    } else if (bin_body || (body_str && *body_str)) {
        const uint8_t* ptr = bin_body ? bin_body : (const uint8_t*)body_str;
        size_t plen = bin_body ? bin_body_len : strlen(body_str);
        if (!has_ct) {
            joss_sb_append(cmd, bin_body ? " -H \"Content-Type: application/octet-stream\"" : " -H \"Content-Type: application/json\"");
        }
        if (joss_http_write_temp_file(ptr, plen, tmp_body_path, sizeof(tmp_body_path))) {
            joss_sb_append(cmd, " --data-binary \"@");
            joss_sb_append(cmd, tmp_body_path);
            joss_buf_append_byte(cmd, '"');
        }
    }

    joss_sb_append(cmd, " \"");
    for (const char* p = url; *p; p++) {
        if (*p == '"' || *p == '\\') joss_buf_append_byte(cmd, '\\');
        joss_buf_append_byte(cmd, (uint8_t)*p);
    }
    joss_buf_append_byte(cmd, '"');
    char* cmd_str = joss_sb_to_string(cmd);
    joss_sb_free(cmd);

    size_t raw_len = 0;
    char* raw = joss_system_run_bytes(cmd_str, &raw_len);
    free(cmd_str);
    if (tmp_body_path[0] != '\0') {
        remove(tmp_body_path);
    }
    if (!raw || raw_len == 0) {
        free(raw);
        return res_map;
    }

    size_t nl_pos = raw_len;
    while (nl_pos > 0 && raw[nl_pos - 1] != '\n') {
        nl_pos--;
    }
    if (nl_pos > 0) {
        int64_t code = atoll(raw + nl_pos);
        joss_map_set_i64(res_map, "status", code);
        joss_map_set_i64(res_map, "success", (int64_t)(uintptr_t)joss_box_bool(code >= 200 && code < 300));
        size_t blen = nl_pos - 1;
        if (memchr(raw, '\0', blen) != NULL) {
            void* boxed = joss_bytes_new(raw, blen);
            joss_map_set_i64(res_map, "body", (int64_t)(uintptr_t)boxed);
        } else {
            char* bcopy = (char*)malloc(blen + 1);
            if (bcopy) {
                memcpy(bcopy, raw, blen);
                bcopy[blen] = '\0';
                joss_map_set_str(res_map, "body", bcopy);
            }
        }
    } else {
        joss_map_set_i64(res_map, "status", 200);
        joss_map_set_str(res_map, "body", raw);
    }
    free(raw);
    return res_map;
}

const char* fn_Http_get(const char* url, void* headers) {
    void* res = fn_Http_request("GET", url, "", headers);
    return joss_map_get_str(res, "body");
}

const char* fn_Http_post(const char* url, const char* body, void* headers) {
    void* res = fn_Http_request("POST", url, body, headers);
    return joss_map_get_str(res, "body");
}

/* =========================================================================
 * RFC 6455 WebSocket Frame Codec, Socket Reader/Writer & Pub/Sub Hub
 * ========================================================================= */
#define JOSS_WS_MAX_FRAME_BYTES (16U * 1024U * 1024U)
#define JOSS_WS_MAX_CHANNELS    128

typedef struct {
    char*          name;
    char*          last_message;
    bool           has_published;
    bool           had_explicit_subs;
    joss_socket_t* sockets;
    int            sock_count;
    int            sock_cap;
} JossWsChannel;

static JossWsChannel g_joss_ws_hub[JOSS_WS_MAX_CHANNELS];
static int           g_joss_ws_hub_count = 0;

static char* joss_ws_strdup_heap(const char* s) {
    if (!s) s = "";
    size_t n = strlen(s);
    char* out = (char*)malloc(n + 1U);
    if (!out) return NULL;
    memcpy(out, s, n + 1U);
    return out;
}

static bool joss_ws_recv_exact(joss_socket_t sock, uint8_t* buf, size_t len) {
    size_t got = 0;
    while (got < len) {
#if defined(_WIN32)
        if (sock == (joss_socket_t)INVALID_SOCKET) return false;
        size_t rem = len - got;
        int chunk = (rem > (size_t)0x3FFFFFFFU) ? 0x3FFFFFFF : (int)rem;
        int n = recv((SOCKET)sock, (char*)(buf + got), chunk, 0);
        if (n <= 0) return false;
        got += (size_t)n;
#else
        if (sock < 0) return false;
        ssize_t n = recv(sock, buf + got, len - got, 0);
        if (n < 0 && errno == EINTR) continue;
        if (n <= 0) return false;
        got += (size_t)n;
#endif
    }
    return true;
}

static bool joss_ws_send_all(joss_socket_t sock, const uint8_t* buf, size_t len) {
    size_t sent = 0;
    while (sent < len) {
#if defined(_WIN32)
        if (sock == (joss_socket_t)INVALID_SOCKET) return false;
        size_t rem = len - sent;
        int chunk = (rem > (size_t)0x3FFFFFFFU) ? 0x3FFFFFFF : (int)rem;
        int n = send((SOCKET)sock, (const char*)(buf + sent), chunk, 0);
        if (n <= 0) return false;
        sent += (size_t)n;
#else
        if (sock < 0) return false;
        int flags = 0;
#ifdef MSG_NOSIGNAL
        flags |= MSG_NOSIGNAL;
#endif
        ssize_t n = send(sock, buf + sent, len - sent, flags);
        if (n < 0 && errno == EINTR) continue;
        if (n <= 0) return false;
        sent += (size_t)n;
#endif
    }
    return true;
}

static bool joss_ws_send_control_frame(joss_socket_t sock, uint8_t first_byte, const uint8_t* payload, size_t len) {
    if (len > 125U) len = 125U;
    uint8_t frame[2 + 125];
    frame[0] = first_byte;
    frame[1] = (uint8_t)len;
    if (len > 0 && payload) {
        memcpy(frame + 2, payload, len);
    }
    return joss_ws_send_all(sock, frame, 2U + len);
}

/* RFC 6455 Section 4.2.2: Sec-WebSocket-Accept = Base64(SHA1(key + GUID)) */
const char* joss_ws_accept_key(const char* client_key) {
    client_key = unbox_web_cstr(client_key);
    if (!client_key) client_key = "";
    char* combined = joss_str_concat(client_key, "258EAFA5-E914-47DA-95CA-C5AB0DC85B11");
    const char* accept = joss_sha1_base64(combined);
    return accept ? accept : "";
}

/* RFC 6455 Section 5.2: Encode an unmasked server->client text frame (opcode 0x81) */
size_t joss_ws_encode_text_frame(const char* payload, uint8_t* out_buf, size_t out_cap) {
    if (!payload) payload = "";
    size_t len = strlen(payload);
    size_t hdr_len = (len < 126) ? 2 : (len <= 65535 ? 4 : 10);
    if (!out_buf || out_cap < hdr_len + len) return 0;
    out_buf[0] = 0x81; /* FIN=1, opcode=text */
    if (len < 126) {
        out_buf[1] = (uint8_t)len;
    } else if (len <= 65535) {
        out_buf[1] = 126;
        out_buf[2] = (uint8_t)((len >> 8) & 0xFF);
        out_buf[3] = (uint8_t)(len & 0xFF);
    } else {
        out_buf[1] = 127;
        for (int i = 0; i < 8; i++) {
            out_buf[2 + i] = (uint8_t)(((uint64_t)len >> (56 - 8 * i)) & 0xFF);
        }
    }
    memcpy(out_buf + hdr_len, payload, len);
    return hdr_len + len;
}

static int joss_ws_read_frame_ex(joss_socket_t sock, bool* out_fin, int* out_opcode, char** out_payload, size_t* out_len) {
    if (out_fin) *out_fin = true;
    if (out_opcode) *out_opcode = 0;
    if (out_payload) *out_payload = NULL;
    if (out_len) *out_len = 0;
#if defined(_WIN32)
    if (sock == (joss_socket_t)INVALID_SOCKET) return -1;
#else
    if (sock < 0) return -1;
#endif

    uint8_t hdr[2];
    if (!joss_ws_recv_exact(sock, hdr, 2U)) {
        return -1;
    }
    bool fin = (hdr[0] & 0x80U) != 0;
    int opcode = (int)(hdr[0] & 0x0FU);
    bool masked = (hdr[1] & 0x80U) != 0;
    uint64_t len7 = (uint64_t)(hdr[1] & 0x7FU);
    uint64_t payload_len = len7;

    if (len7 == 126U) {
        uint8_t ext[2];
        if (!joss_ws_recv_exact(sock, ext, 2U)) return -1;
        payload_len = ((uint64_t)ext[0] << 8) | (uint64_t)ext[1];
    } else if (len7 == 127U) {
        uint8_t ext[8];
        if (!joss_ws_recv_exact(sock, ext, 8U)) return -1;
        if ((ext[0] & 0x80U) != 0) return -1;
        payload_len = ((uint64_t)ext[0] << 56) |
                      ((uint64_t)ext[1] << 48) |
                      ((uint64_t)ext[2] << 40) |
                      ((uint64_t)ext[3] << 32) |
                      ((uint64_t)ext[4] << 24) |
                      ((uint64_t)ext[5] << 16) |
                      ((uint64_t)ext[6] << 8)  |
                      (uint64_t)ext[7];
    }

    if (payload_len > (uint64_t)JOSS_WS_MAX_FRAME_BYTES) {
        return -1;
    }
    if (opcode >= 0x8 && (!fin || payload_len > 125U)) {
        return -1;
    }

    uint8_t mask[4] = {0, 0, 0, 0};
    if (masked) {
        if (!joss_ws_recv_exact(sock, mask, 4U)) return -1;
    }

    char* payload = (char*)malloc((size_t)payload_len + 1U);
    if (!payload) return -1;
    if (payload_len > 0) {
        if (!joss_ws_recv_exact(sock, (uint8_t*)payload, (size_t)payload_len)) {
            free(payload);
            return -1;
        }
        if (masked) {
            for (size_t i = 0; i < (size_t)payload_len; i++) {
                payload[i] = (char)((uint8_t)payload[i] ^ mask[i & 3U]);
            }
        }
    }
    payload[payload_len] = '\0';

    if (opcode == 0x9) {
        (void)joss_ws_send_control_frame(sock, 0x8AU, (const uint8_t*)payload, (size_t)payload_len);
    } else if (opcode == 0x8) {
        int close_code = 1000;
        if (payload_len >= 2U) {
            uint16_t raw_code = (uint16_t)(((uint16_t)(uint8_t)payload[0] << 8) | (uint16_t)(uint8_t)payload[1]);
            if (raw_code > 0) close_code = (int)raw_code;
            (void)joss_ws_send_control_frame(sock, 0x88U, (const uint8_t*)payload, 2U);
        } else {
            (void)joss_ws_send_control_frame(sock, 0x88U, NULL, 0U);
        }
        if (out_fin) *out_fin = fin;
        if (out_opcode) *out_opcode = opcode;
        if (out_len) *out_len = (size_t)payload_len;
        if (out_payload) *out_payload = payload;
        else free(payload);
        return close_code;
    }

    if (out_fin) *out_fin = fin;
    if (out_opcode) *out_opcode = opcode;
    if (out_len) *out_len = (size_t)payload_len;
    if (out_payload) *out_payload = payload;
    else free(payload);
    return 0;
}

int joss_ws_read_frame(joss_socket_t sock, int* out_opcode, char** out_payload, size_t* out_len) {
    return joss_ws_read_frame_ex(sock, NULL, out_opcode, out_payload, out_len);
}

const char* joss___joss_ws_read_text(int64_t sock_fd) {
    joss_socket_t sock;
    if (!joss_ws_resolve_socket(sock_fd, &sock)) {
        return "";
    }
    JossStringBuilder* acc = NULL;
    for (;;) {
        bool fin = true;
        int opcode = 0;
        char* payload = NULL;
        size_t plen = 0;
        int rc = joss_ws_read_frame_ex(sock, &fin, &opcode, &payload, &plen);
        if (rc != 0) {
            free(payload);
            if (acc) joss_sb_free(acc);
            return "";
        }
        if (opcode == 0x9 || opcode == 0xA) {
            free(payload);
            continue;
        }
        if (fin && !acc) {
            char* out = (char*)joss_req_alloc(plen + 1U);
            if (!out) {
                free(payload);
                return "";
            }
            if (plen > 0 && payload) memcpy(out, payload, plen);
            out[plen] = '\0';
            free(payload);
            return out;
        }
        if (!acc) acc = joss_sb_new();
        if (plen > 0 && payload) {
            joss_buf_append_bytes(acc, (const uint8_t*)payload, (int64_t)plen);
        }
        free(payload);
        if (fin) {
            size_t total = (size_t)acc->len;
            char* out = (char*)joss_req_alloc(total + 1U);
            if (out) {
                if (total > 0) memcpy(out, acc->data, total);
                out[total] = '\0';
            }
            joss_sb_free(acc);
            return out ? out : "";
        }
    }
}

int64_t joss___joss_ws_send_text(int64_t sock_fd, const char* text) {
    joss_socket_t sock;
    if (!joss_ws_resolve_socket(sock_fd, &sock)) {
        return 0;
    }
    text = unbox_web_cstr(text);
    if (!text) text = "";
    size_t len = strlen(text);
    size_t cap = len + 16U;
    uint8_t stack_buf[4112];
    uint8_t* frame = (cap <= sizeof(stack_buf)) ? stack_buf : (uint8_t*)malloc(cap);
    if (!frame) return 0;
    size_t flen = joss_ws_encode_text_frame(text, frame, cap);
    bool ok = (flen > 0) ? joss_ws_send_all(sock, frame, flen) : false;
    if (frame != stack_buf) free(frame);
    return ok ? 1 : 0;
}

static JossWsChannel* joss_ws_hub_find_locked(const char* channel, bool create) {
    for (int i = 0; i < g_joss_ws_hub_count; i++) {
        if (g_joss_ws_hub[i].name && strcmp(g_joss_ws_hub[i].name, channel) == 0) {
            return &g_joss_ws_hub[i];
        }
    }
    if (!create || g_joss_ws_hub_count >= JOSS_WS_MAX_CHANNELS) return NULL;
    JossWsChannel* ch = &g_joss_ws_hub[g_joss_ws_hub_count++];
    memset(ch, 0, sizeof(*ch));
    ch->name = joss_ws_strdup_heap(channel);
    return ch;
}

static void joss_ws_hub_subscribe_raw(const char* channel, joss_socket_t sock) {
    if (!channel || !channel[0]) return;
    joss_global_lock(JOSS_LOCK_WS);
    JossWsChannel* ch = joss_ws_hub_find_locked(channel, true);
    if (ch) {
        ch->had_explicit_subs = true;
        ch->has_published = false;
        bool exists = false;
        for (int i = 0; i < ch->sock_count; i++) {
            if (ch->sockets[i] == sock) {
                exists = true;
                break;
            }
        }
        if (!exists) {
            if (ch->sock_count >= ch->sock_cap) {
                int ncap = ch->sock_cap > 0 ? ch->sock_cap * 2 : 16;
                joss_socket_t* next = (joss_socket_t*)realloc(ch->sockets, (size_t)ncap * sizeof(joss_socket_t));
                if (next) {
                    ch->sockets = next;
                    ch->sock_cap = ncap;
                }
            }
            if (ch->sock_count < ch->sock_cap) {
                ch->sockets[ch->sock_count++] = sock;
            }
        }
    }
    joss_global_unlock(JOSS_LOCK_WS);
}

static int64_t joss_ws_hub_unsubscribe_raw(const char* channel, joss_socket_t sock) {
    if (!channel || !channel[0]) return 0;
    int64_t removed = 0;
    joss_global_lock(JOSS_LOCK_WS);
    JossWsChannel* ch = joss_ws_hub_find_locked(channel, false);
    if (ch) {
        ch->had_explicit_subs = true;
        ch->has_published = false;
        for (int i = 0; i < ch->sock_count; i++) {
            if (ch->sockets[i] == sock) {
                for (int j = i; j + 1 < ch->sock_count; j++) {
                    ch->sockets[j] = ch->sockets[j + 1];
                }
                ch->sock_count--;
                removed = 1;
                break;
            }
        }
    }
    joss_global_unlock(JOSS_LOCK_WS);
    return removed;
}

static void joss_ws_hub_unsubscribe_all(joss_socket_t sock) {
    joss_global_lock(JOSS_LOCK_WS);
    for (int c = 0; c < g_joss_ws_hub_count; c++) {
        JossWsChannel* ch = &g_joss_ws_hub[c];
        for (int i = 0; i < ch->sock_count; i++) {
            if (ch->sockets[i] == sock) {
                for (int j = i; j + 1 < ch->sock_count; j++) {
                    ch->sockets[j] = ch->sockets[j + 1];
                }
                ch->sock_count--;
                break;
            }
        }
    }
    joss_global_unlock(JOSS_LOCK_WS);
}

int64_t fn_WebSocket_subscribe(const char* channel, int64_t sock_fd) {
    channel = unbox_web_cstr(channel);
    if (!channel || !channel[0]) return 0;
    joss_socket_t sock;
    if (!joss_ws_resolve_socket(sock_fd, &sock)) return 0;
    joss_ws_hub_subscribe_raw(channel, sock);
    return 1;
}

int64_t fn_WebSocket_unsubscribe(const char* channel, int64_t sock_fd) {
    channel = unbox_web_cstr(channel);
    if (!channel || !channel[0]) return 0;
    joss_socket_t sock;
    if (!joss_ws_resolve_socket(sock_fd, &sock)) return 0;
    return joss_ws_hub_unsubscribe_raw(channel, sock);
}

int64_t fn_WebSocket_publish(const char* channel, const char* message) {
    channel = unbox_web_cstr(channel);
    message = unbox_web_cstr(message);
    if (!channel || !channel[0]) return 0;
    if (!message) message = "";

    size_t msg_len = strlen(message);
    size_t frame_cap = msg_len + 16U;
    uint8_t stack_frame[4112];
    uint8_t* frame = (frame_cap <= sizeof(stack_frame)) ? stack_frame : (uint8_t*)malloc(frame_cap);
    if (!frame) return 0;
    size_t frame_len = joss_ws_encode_text_frame(message, frame, frame_cap);

    joss_socket_t* snap = NULL;
    int snap_count = 0;
    bool in_mem_compat = false;

    joss_global_lock(JOSS_LOCK_WS);
    JossWsChannel* ch = joss_ws_hub_find_locked(channel, true);
    if (ch) {
        free(ch->last_message);
        ch->last_message = joss_ws_strdup_heap(message);
        if (ch->sock_count > 0) {
            snap = (joss_socket_t*)malloc((size_t)ch->sock_count * sizeof(joss_socket_t));
            if (snap) {
                memcpy(snap, ch->sockets, (size_t)ch->sock_count * sizeof(joss_socket_t));
                snap_count = ch->sock_count;
            }
        } else if (!ch->had_explicit_subs) {
            ch->has_published = true;
            in_mem_compat = true;
        }
    }
    joss_global_unlock(JOSS_LOCK_WS);

    int64_t delivered = 0;
    if (snap_count > 0 && frame_len > 0) {
        bool* dead = (bool*)calloc((size_t)snap_count, sizeof(bool));
        bool any_dead = false;
        for (int i = 0; i < snap_count; i++) {
            if (joss_ws_send_all(snap[i], frame, frame_len)) {
                delivered++;
            } else if (dead) {
                dead[i] = true;
                any_dead = true;
            }
        }
        if (any_dead && dead) {
            joss_global_lock(JOSS_LOCK_WS);
            JossWsChannel* ch_purge = joss_ws_hub_find_locked(channel, false);
            if (ch_purge) {
                for (int i = 0; i < snap_count; i++) {
                    if (!dead[i]) continue;
                    for (int k = 0; k < ch_purge->sock_count; k++) {
                        if (ch_purge->sockets[k] == snap[i]) {
                            for (int m = k; m + 1 < ch_purge->sock_count; m++) {
                                ch_purge->sockets[m] = ch_purge->sockets[m + 1];
                            }
                            ch_purge->sock_count--;
                            break;
                        }
                    }
                }
            }
            joss_global_unlock(JOSS_LOCK_WS);
        }
        free(dead);
    } else if (in_mem_compat) {
        delivered = 1;
    }

    free(snap);
    if (frame != stack_frame) free(frame);
    return delivered;
}

int64_t fn_WebSocket_subscriberCount(const char* channel) {
    channel = unbox_web_cstr(channel);
    if (!channel || !channel[0]) return 0;
    joss_global_lock(JOSS_LOCK_WS);
    int64_t count = 0;
    JossWsChannel* ch = joss_ws_hub_find_locked(channel, false);
    if (ch) {
        if (ch->sock_count > 0) {
            count = (int64_t)ch->sock_count;
        } else if (ch->has_published && !ch->had_explicit_subs) {
            count = 1;
        }
    }
    joss_global_unlock(JOSS_LOCK_WS);
    return count;
}

/* =========================================================================
 * Cron Scheduler & Background Ticker (5-field RFC/Vixie Cron)
 * ========================================================================= */
#define JOSS_MAX_CRON_JOBS 64

typedef void (*JossCronCallbackFn)(void);

typedef struct {
    bool    active;
    char*   expr;
    void*   callback_raw;
    int64_t last_run_minute;
} JossCronJob;

static JossCronJob g_joss_cron_jobs[JOSS_MAX_CRON_JOBS];
static int         g_joss_cron_count = 0;
static atomic_flag g_joss_cron_ticker_started = ATOMIC_FLAG_INIT;

static bool joss_cron_parse_int(const char* s, int min_v, int max_v, bool is_dow, int* out) {
    if (!s || !*s) return false;
    char* endp = NULL;
    long v = strtol(s, &endp, 10);
    if (endp == s || *endp != '\0') return false;
    if (is_dow && v == 7) v = 0;
    if (v < min_v || v > max_v) return false;
    *out = (int)v;
    return true;
}

static bool joss_cron_field_eval(const char* field, int val, int min_v, int max_v, bool is_dow, bool* out_valid) {
    if (!field || !*field) {
        *out_valid = false;
        return false;
    }
    char buf[128];
    size_t flen = strlen(field);
    if (flen >= sizeof(buf)) {
        *out_valid = false;
        return false;
    }
    memcpy(buf, field, flen + 1U);

    bool matched = false;
    char* p = buf;
    while (p && *p) {
        char* comma = strchr(p, ',');
        if (comma) *comma = '\0';
        char* tok = p;
        p = comma ? (comma + 1) : NULL;
        if (!*tok) {
            *out_valid = false;
            return false;
        }
        char* slash = strchr(tok, '/');
        int step = 1;
        if (slash) {
            *slash = '\0';
            char* step_str = slash + 1;
            char* endp = NULL;
            long sv = strtol(step_str, &endp, 10);
            if (*step_str == '\0' || *endp != '\0' || sv <= 0 || sv > 1000) {
                *out_valid = false;
                return false;
            }
            step = (int)sv;
        }
        if (strcmp(tok, "*") == 0) {
            if (val >= min_v && val <= max_v && ((val - min_v) % step) == 0) {
                matched = true;
            }
        } else {
            char* dash = strchr(tok, '-');
            if (dash) {
                *dash = '\0';
                const char* left = tok;
                const char* right = dash + 1;
                char* end1 = NULL;
                char* end2 = NULL;
                long a = strtol(left, &end1, 10);
                long b = strtol(right, &end2, 10);
                if (*left == '\0' || *end1 != '\0' || *right == '\0' || *end2 != '\0') {
                    *out_valid = false;
                    return false;
                }
                if (is_dow && a == 7 && b == 7) {
                    a = 0;
                    b = 0;
                }
                int max_allowed = is_dow ? 7 : max_v;
                if (a < min_v || b > max_allowed || a > b) {
                    *out_valid = false;
                    return false;
                }
                if (is_dow && b == 7 && val == 0) {
                    if (((7L - a) % step) == 0) matched = true;
                }
                if (val >= (int)a && val <= (int)b && ((val - (int)a) % step) == 0) {
                    matched = true;
                }
            } else {
                int num = 0;
                if (!joss_cron_parse_int(tok, min_v, max_v, is_dow, &num)) {
                    *out_valid = false;
                    return false;
                }
                if (slash) {
                    if (val >= num && val <= max_v && ((val - num) % step) == 0) {
                        matched = true;
                    }
                } else if (val == num) {
                    matched = true;
                }
            }
        }
    }
    *out_valid = true;
    return matched;
}

static bool joss_cron_eval(const char* expr, const struct tm* tm_val, bool* out_valid) {
    if (out_valid) *out_valid = false;
    if (!expr) return false;
    while (*expr && isspace((unsigned char)*expr)) expr++;
    if (!*expr) return false;

    const char* canonical = expr;
    if (strcasecmp(expr, "hourly") == 0 || strcasecmp(expr, "@hourly") == 0) canonical = "0 * * * *";
    else if (strcasecmp(expr, "daily") == 0 || strcasecmp(expr, "@daily") == 0) canonical = "0 0 * * *";
    else if (strcasecmp(expr, "weekly") == 0 || strcasecmp(expr, "@weekly") == 0) canonical = "0 0 * * 0";
    else if (strcasecmp(expr, "monthly") == 0 || strcasecmp(expr, "@monthly") == 0) canonical = "0 0 1 * *";
    else if (strcasecmp(expr, "yearly") == 0 || strcasecmp(expr, "@yearly") == 0 || strcasecmp(expr, "@annually") == 0) canonical = "0 0 1 1 *";

    char copy[256];
    size_t len = strlen(canonical);
    if (len >= sizeof(copy)) return false;
    memcpy(copy, canonical, len + 1U);

    char* fields[5] = {NULL, NULL, NULL, NULL, NULL};
    int nf = 0;
    char* p = copy;
    while (*p) {
        while (*p && isspace((unsigned char)*p)) p++;
        if (!*p) break;
        if (nf >= 5) return false;
        fields[nf++] = p;
        while (*p && !isspace((unsigned char)*p)) p++;
        if (*p) *p++ = '\0';
    }
    if (nf != 5) return false;

    int min_v = tm_val ? tm_val->tm_min : 0;
    int hour_v = tm_val ? tm_val->tm_hour : 0;
    int dom_v = tm_val ? tm_val->tm_mday : 1;
    int mon_v = tm_val ? (tm_val->tm_mon + 1) : 1;
    int dow_v = tm_val ? tm_val->tm_wday : 0;

    bool v0 = false, v1 = false, v2 = false, v3 = false, v4 = false;
    bool m0 = joss_cron_field_eval(fields[0], min_v,  0, 59, false, &v0);
    bool m1 = joss_cron_field_eval(fields[1], hour_v, 0, 23, false, &v1);
    bool m2 = joss_cron_field_eval(fields[2], dom_v,  1, 31, false, &v2);
    bool m3 = joss_cron_field_eval(fields[3], mon_v,  1, 12, false, &v3);
    bool m4 = joss_cron_field_eval(fields[4], dow_v,  0, 6,  true,  &v4);

    if (!v0 || !v1 || !v2 || !v3 || !v4) return false;
    if (out_valid) *out_valid = true;
    return tm_val ? (m0 && m1 && m2 && m3 && m4) : true;
}

static int64_t joss_cron_tick_once(bool force_due) {
    time_t now = time(NULL);
    struct tm tm_now;
#if defined(_WIN32)
    localtime_s(&tm_now, &now);
#else
    localtime_r(&now, &tm_now);
#endif
    int64_t cur_min = (int64_t)(now / 60);

    void* due[JOSS_MAX_CRON_JOBS];
    int due_count = 0;

    joss_global_lock(JOSS_LOCK_CRON);
    for (int i = 0; i < g_joss_cron_count; i++) {
        JossCronJob* job = &g_joss_cron_jobs[i];
        if (!job->active || !job->callback_raw || !job->expr) continue;
        bool valid = false;
        if (joss_cron_eval(job->expr, &tm_now, &valid) && valid) {
            if (force_due || job->last_run_minute != cur_min) {
                job->last_run_minute = cur_min;
                if (due_count < JOSS_MAX_CRON_JOBS) {
                    due[due_count++] = job->callback_raw;
                }
            }
        }
    }
    joss_global_unlock(JOSS_LOCK_CRON);

    for (int i = 0; i < due_count; i++) {
        joss_req_arena_begin();
        joss_try_reset();
        void* fn_raw = joss_closure_prepare_call(due[i]);
        if (fn_raw) {
            ((JossCronCallbackFn)fn_raw)();
        }
        joss_req_state_reset();
        joss_req_arena_reset();
    }
    return (int64_t)due_count;
}

#if defined(_WIN32)
static unsigned __stdcall joss_cron_ticker_main(void* arg) {
    (void)arg;
    for (;;) {
        Sleep(1000);
        (void)joss_cron_tick_once(false);
    }
    return 0;
}
#elif defined(__unix__) || defined(__APPLE__)
static void* joss_cron_ticker_main(void* arg) {
    (void)arg;
    for (;;) {
        sleep(1);
        (void)joss_cron_tick_once(false);
    }
    return NULL;
}
#endif

static void joss_cron_ensure_ticker_started(void) {
    if (!atomic_flag_test_and_set(&g_joss_cron_ticker_started)) {
#if defined(_WIN32)
        uintptr_t th = _beginthreadex(NULL, 0, joss_cron_ticker_main, NULL, 0, NULL);
        if (th != 0) CloseHandle((HANDLE)th);
#elif defined(__unix__) || defined(__APPLE__)
        pthread_t tid;
        if (pthread_create(&tid, NULL, joss_cron_ticker_main, NULL) == 0) {
            pthread_detach(tid);
        }
#endif
    }
}

void* fn_Cron_schedule(const char* expr, void* callback) {
    expr = unbox_web_cstr(expr);
    if (!expr || !expr[0] || !joss_is_not_null((int64_t)(uintptr_t)callback)) return NULL;
    bool valid = false;
    (void)joss_cron_eval(expr, NULL, &valid);
    if (!valid) return NULL;

    joss_global_lock(JOSS_LOCK_CRON);
    void* result = NULL;
    if (g_joss_cron_count < JOSS_MAX_CRON_JOBS) {
        JossCronJob* job = &g_joss_cron_jobs[g_joss_cron_count++];
        job->active = true;
        job->expr = joss_ws_strdup_heap(expr);
        job->callback_raw = callback;
        job->last_run_minute = -1;
        result = (void*)(intptr_t)1;
    }
    joss_global_unlock(JOSS_LOCK_CRON);
    if (result) {
        joss_cron_ensure_ticker_started();
    }
    return result;
}

int64_t fn_Cron_runDue(void) {
    return joss_cron_tick_once(true);
}

void* fn_AI_client(const char* provider, const char* api_key) {
    void* m = joss_map_new();
    joss_map_set_str(m, "provider", unbox_web_cstr(provider) ? unbox_web_cstr(provider) : "openai");
    joss_map_set_str(m, "api_key", unbox_web_cstr(api_key) ? unbox_web_cstr(api_key) : "");
    return m;
}

const char* fn_AI_chat(const char* prompt, void* opts) {
    prompt = unbox_web_cstr(prompt);
    (void)opts;
    return prompt ? prompt : "";
}

static void* g_joss_notify_devices = NULL;
static void* g_joss_notify_pending = NULL;

void* fn_Notify_app(const char* app_id) {
    void* m = joss_map_new();
    joss_map_set_str(m, "app_id", unbox_web_cstr(app_id) ? unbox_web_cstr(app_id) : "default");
    return m;
}

int64_t fn_Notify_registerDevice(const char* user_id, const char* token) {
    user_id = unbox_web_cstr(user_id);
    token = unbox_web_cstr(token);
    if (!user_id || !token) return 0;
    joss_global_lock(JOSS_LOCK_NOTIFY);
    bool _prev_arena = joss_req_arena_pause();
    if (!g_joss_notify_devices) g_joss_notify_devices = joss_map_new();
    joss_map_set_str(g_joss_notify_devices, user_id, joss_web_strdup(token));
    joss_req_arena_resume(_prev_arena);
    joss_global_unlock(JOSS_LOCK_NOTIFY);
    return 1;
}

int64_t fn_Notify_unregisterDevice(const char* user_id) {
    user_id = unbox_web_cstr(user_id);
    if (!user_id) return 0;
    joss_global_lock(JOSS_LOCK_NOTIFY);
    int64_t ok = 0;
    if (g_joss_notify_devices) {
        joss_map_delete_i64(g_joss_notify_devices, user_id);
        ok = 1;
    }
    joss_global_unlock(JOSS_LOCK_NOTIFY);
    return ok;
}

void* fn_Notify_pendingMessages(const char* user_id) {
    user_id = unbox_web_cstr(user_id);
    if (!user_id) return joss_arr_new(0);
    joss_global_lock(JOSS_LOCK_NOTIFY);
    int64_t v = g_joss_notify_pending ? joss_map_get_i64(g_joss_notify_pending, user_id) : 0;
    joss_global_unlock(JOSS_LOCK_NOTIFY);
    if (joss_is_arr_ptr(v)) return (void*)(uintptr_t)v;
    return joss_arr_new(0);
}

int64_t fn_Notify_acknowledge(const char* user_id, const char* msg_id) {
    (void)msg_id;
    user_id = unbox_web_cstr(user_id);
    if (!user_id) return 1;
    joss_global_lock(JOSS_LOCK_NOTIFY);
    if (g_joss_notify_pending) joss_map_delete_i64(g_joss_notify_pending, user_id);
    joss_global_unlock(JOSS_LOCK_NOTIFY);
    return 1;
}

typedef int64_t (*JossPluginDispatchFn)(const char*, const char*, void*);
static JossPluginDispatchFn g_joss_plugin_dispatcher = NULL;

void joss_register_plugin_dispatcher(void* fn) {
    g_joss_plugin_dispatcher = (JossPluginDispatchFn)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)fn);
}

int64_t fn_Plugin_call(const char* plugin_name, const char* func_name, void* args) {
    plugin_name = unbox_web_cstr(plugin_name);
    func_name = unbox_web_cstr(func_name);
    if (g_joss_plugin_dispatcher && plugin_name && func_name) {
        return g_joss_plugin_dispatcher(plugin_name, func_name, args);
    }
    return 0;
}

int64_t fn_Plugin_stream(const char* name, const char* func_name, void* args) {
    return fn_Plugin_call(name, func_name, args);
}

const char* fn_Plugin_platform(void) {
#if defined(_WIN32)
    #if defined(__aarch64__) || defined(_M_ARM64)
        return "windows-arm64";
    #elif defined(__x86_64__) || defined(_M_X64)
        return "windows-amd64";
    #elif defined(__i386__) || defined(_M_IX86)
        return "windows-386";
    #else
        return "windows-unknown";
    #endif
#elif defined(__APPLE__)
    #if defined(__aarch64__) || defined(__arm64__)
        return "darwin-arm64";
    #elif defined(__x86_64__)
        return "darwin-amd64";
    #else
        return "darwin-unknown";
    #endif
#elif defined(__ANDROID__)
    #if defined(__aarch64__)
        return "android-arm64";
    #elif defined(__x86_64__)
        return "android-amd64";
    #elif defined(__arm__)
        return "android-arm";
    #else
        return "android-unknown";
    #endif
#elif defined(__linux__)
    #if defined(__aarch64__)
        return "linux-arm64";
    #elif defined(__x86_64__)
        return "linux-amd64";
    #elif defined(__i386__)
        return "linux-386";
    #elif defined(__arm__)
        return "linux-arm";
    #else
        return "linux-unknown";
    #endif
#else
    return "unknown-unknown";
#endif
}

static bool joss_plugin_clean_rel(const char* val, char* out, size_t out_cap, bool allow_subdirs) {
    if (!val || !val[0] || !out || out_cap < 2) return false;
    if (val[0] == '/' || val[0] == '\\') return false;
    if (isalpha((unsigned char)val[0]) && val[1] == ':') return false;

    const char* seg_start[64];
    size_t seg_len[64];
    int seg_count = 0;

    const char* p = val;
    while (*p) {
        if (*p == '/' || *p == '\\') {
            if (!allow_subdirs) return false;
            p++;
            continue;
        }
        const char* start = p;
        while (*p && *p != '/' && *p != '\\') p++;
        size_t len = (size_t)(p - start);
        if (len == 1 && start[0] == '.') {
            continue;
        }
        if (len == 2 && start[0] == '.' && start[1] == '.') {
            return false;
        }
        if (seg_count >= 64) return false;
        seg_start[seg_count] = start;
        seg_len[seg_count] = len;
        seg_count++;
    }
    if (seg_count == 0) return false;

#if defined(_WIN32)
    const char sep = '\\';
#else
    const char sep = '/';
#endif
    size_t pos = 0;
    for (int i = 0; i < seg_count; i++) {
        if (i > 0) {
            if (pos + 1 >= out_cap) return false;
            out[pos++] = sep;
        }
        if (pos + seg_len[i] >= out_cap) return false;
        memcpy(out + pos, seg_start[i], seg_len[i]);
        pos += seg_len[i];
    }
    out[pos] = '\0';
    return true;
}

const char* fn_Plugin_path(const char* name, const char* rel) {
    name = unbox_web_cstr(name);
    rel = unbox_web_cstr(rel);

    char cwd[1024];
    cwd[0] = '\0';
#if defined(_WIN32)
    if (!_getcwd(cwd, (int)sizeof(cwd))) snprintf(cwd, sizeof(cwd), ".");
    const char sep = '\\';
#else
    if (!getcwd(cwd, sizeof(cwd))) snprintf(cwd, sizeof(cwd), ".");
    const char sep = '/';
#endif
    size_t cwd_len = strlen(cwd);
    while (cwd_len > 1 && (cwd[cwd_len - 1] == '/' || cwd[cwd_len - 1] == '\\')) {
        cwd[--cwd_len] = '\0';
    }

    char clean_name[256];
    char clean_rel[1024];
    bool ok_name = joss_plugin_clean_rel(name, clean_name, sizeof(clean_name), false);
    bool ok_rel = joss_plugin_clean_rel(rel, clean_rel, sizeof(clean_rel), true);

    char out[2400];
    if (!ok_name || !ok_rel) {
        snprintf(out, sizeof(out), "%s%cplugins", cwd, sep);
    } else {
        snprintf(out, sizeof(out), "%s%cplugins%c%s%c%s", cwd, sep, sep, clean_name, sep, clean_rel);
    }
    size_t n = strlen(out);
    char* copy = (char*)joss_req_alloc(n + 1);
    if (!copy) return "plugins";
    memcpy(copy, out, n + 1);
    return copy;
}

/* =========================================================================
 * Phase 1: Native Global Builtins & Base Host Classes
 * ========================================================================= */

#if !defined(_WIN32)
#include <sys/stat.h>
#include <sys/time.h>
#endif

static char* joss_web_strdup(const char* s) {
    if (!s) s = "";
    size_t n = strlen(s);
    char* out = (char*)malloc(n + 1);
    if (!out) return "";
    memcpy(out, s, n + 1);
    return out;
}

static double joss_i64_to_double_bits(int64_t bits) {
    double d = 0.0;
    memcpy(&d, &bits, sizeof(double));
    return d;
}

static int64_t joss_double_to_i64_bits(double d) {
    int64_t bits = 0;
    memcpy(&bits, &d, sizeof(double));
    return bits;
}

int64_t joss_microtime(void) {
#if defined(_WIN32)
    FILETIME ft;
    GetSystemTimeAsFileTime(&ft);
    ULARGE_INTEGER uli;
    uli.LowPart = ft.dwLowDateTime;
    uli.HighPart = ft.dwHighDateTime;
    double sec = (double)(uli.QuadPart - 116444736000000000ULL) / 10000000.0;
    return joss_double_to_i64_bits(sec);
#else
    struct timeval tv;
    gettimeofday(&tv, NULL);
    double sec = (double)tv.tv_sec + (double)tv.tv_usec / 1000000.0;
    return joss_double_to_i64_bits(sec);
#endif
}

int64_t joss_sleep(int64_t seconds) {
    if (seconds <= 0) return 0;
#if defined(_WIN32)
    Sleep((DWORD)(seconds * 1000));
#else
    sleep((unsigned int)seconds);
#endif
    return 0;
}

int64_t fn_System_sleep(int64_t seconds) {
    joss_sleep(joss_unbox_value(seconds));
    return 1;
}

const char* fn_System_env(const char* key, int64_t defVal) {
    key = unbox_web_cstr(key);
    const char* def_str = "";
    if (joss_is_not_null(defVal)) {
        const char* s = joss_strval(defVal);
        if (s) def_str = s;
    }
    return joss_env(key, def_str);
}

void fn_System_log(int64_t msg) {
    const char* s = joss_strval(msg);
    printf("[System Log] %s\n", s ? s : "");
    fflush(stdout);
}

const char* fn_System_now(int64_t days_offset) {
    int64_t offset = joss_is_not_null(days_offset) ? joss_intval(days_offset) : 0;
    time_t now = time(NULL);
    struct tm tm_val;
#if defined(_WIN32)
    localtime_s(&tm_val, &now);
#else
    localtime_r(&now, &tm_val);
#endif
    if (offset != 0) {
        tm_val.tm_mday += (int)offset;
        tm_val.tm_isdst = -1;
        mktime(&tm_val);
    }
    char buf[80];
    snprintf(buf, sizeof(buf), "%04d-%02d-%02d %02d:%02d:%02d",
             tm_val.tm_year + 1900, tm_val.tm_mon + 1, tm_val.tm_mday,
             tm_val.tm_hour, tm_val.tm_min, tm_val.tm_sec);
    return joss_req_strdup(buf);
}

#if !defined(_WIN32)
static char* joss_proc_strdup_heap(const char* s) {
    if (!s) s = "";
    size_t n = strlen(s);
    char* out = (char*)malloc(n + 1);
    if (!out) return NULL;
    memcpy(out, s, n + 1);
    return out;
}
#else
static void joss_win_append_quoted_arg(JossStringBuilder* sb, const char* arg) {
    if (!arg || !*arg) {
        joss_sb_append(sb, "\"\"");
        return;
    }
    bool needs_quote = false;
    for (const char* p = arg; *p; p++) {
        if (*p == ' ' || *p == '\t' || *p == '\n' || *p == '\v' || *p == '"') {
            needs_quote = true;
            break;
        }
    }
    if (!needs_quote) {
        joss_sb_append(sb, arg);
        return;
    }
    joss_buf_append_byte(sb, '"');
    for (const char* p = arg; ; ) {
        size_t bs = 0;
        while (*p == '\\') {
            bs++;
            p++;
        }
        if (*p == '\0') {
            for (size_t k = 0; k < bs * 2U; k++) joss_buf_append_byte(sb, '\\');
            break;
        } else if (*p == '"') {
            for (size_t k = 0; k < bs * 2U + 1U; k++) joss_buf_append_byte(sb, '\\');
            joss_buf_append_byte(sb, '"');
            p++;
        } else {
            for (size_t k = 0; k < bs; k++) joss_buf_append_byte(sb, '\\');
            joss_buf_append_byte(sb, (uint8_t)*p);
            p++;
        }
    }
    joss_buf_append_byte(sb, '"');
}
#endif

void* fn_Process_constructor(void* proc, const char* cmd, void* args) {
    proc = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)proc);
    if (!proc || !joss_is_map_ptr((int64_t)(uintptr_t)proc)) {
        proc = joss_obj_new("Process");
    }
    cmd = unbox_web_cstr(cmd);
    void* args_arr = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)args);
    joss_obj_set_field_str(proc, "_cmd", cmd ? joss_web_strdup(cmd) : "");
    joss_obj_set_field_i64(proc, "_args", (args_arr && joss_is_arr_ptr((int64_t)(uintptr_t)args_arr)) ? (int64_t)(uintptr_t)args_arr : 0);
    joss_obj_set_field_i64(proc, "_started", 0);
    joss_obj_set_field_i64(proc, "_pid", 0);
    joss_obj_set_field_i64(proc, "_handle", 0);
    joss_obj_set_field_i64(proc, "_waited", 0);
    joss_obj_set_field_i64(proc, "_exit_code", -1);
    return proc;
}

int64_t fn_Process_start(void* proc) {
    proc = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)proc);
    if (!proc || !joss_is_map_ptr((int64_t)(uintptr_t)proc)) return 0;
    if (joss_obj_get_field_i64(proc, "_started") != 0) return 0;
    const char* cmd = joss_obj_get_field_str(proc, "_cmd");
    if (!cmd || !cmd[0]) return 0;
    void* args_arr = (void*)(uintptr_t)joss_obj_get_field_i64(proc, "_args");
    int64_t nargs = (args_arr && joss_is_arr_ptr((int64_t)(uintptr_t)args_arr)) ? joss_arr_len(args_arr) : 0;

#if defined(_WIN32)
    JossStringBuilder* sb = joss_sb_new();
    joss_win_append_quoted_arg(sb, cmd);
    for (int64_t i = 0; i < nargs; i++) {
        joss_buf_append_byte(sb, ' ');
        const char* a = joss_strval(joss_arr_get_i64(args_arr, i));
        joss_win_append_quoted_arg(sb, a ? a : "");
    }
    char* cmdline = joss_sb_to_string(sb);
    joss_sb_free(sb);
    if (!cmdline) return 0;

    STARTUPINFOA si;
    PROCESS_INFORMATION pi;
    memset(&si, 0, sizeof(si));
    si.cb = (DWORD)sizeof(si);
    memset(&pi, 0, sizeof(pi));
    BOOL ok = CreateProcessA(NULL, cmdline, NULL, NULL, FALSE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi);
    free(cmdline);
    if (!ok) return 0;
    CloseHandle(pi.hThread);
    joss_obj_set_field_i64(proc, "_started", 1);
    joss_obj_set_field_i64(proc, "_pid", (int64_t)pi.dwProcessId);
    joss_obj_set_field_i64(proc, "_handle", (int64_t)(uintptr_t)pi.hProcess);
    return 1;
#elif defined(__unix__) || defined(__APPLE__)
    char** argv = (char**)calloc((size_t)nargs + 2U, sizeof(char*));
    if (!argv) return 0;
    argv[0] = joss_proc_strdup_heap(cmd);
    if (!argv[0]) { free(argv); return 0; }
    for (int64_t i = 0; i < nargs; i++) {
        const char* a = joss_strval(joss_arr_get_i64(args_arr, i));
        argv[i + 1] = joss_proc_strdup_heap(a ? a : "");
        if (!argv[i + 1]) {
            for (int64_t j = 0; j <= i; j++) free(argv[j]);
            free(argv);
            return 0;
        }
    }
    argv[nargs + 1] = NULL;

    int err_pipe[2] = {-1, -1};
    if (pipe(err_pipe) == 0) {
        (void)fcntl(err_pipe[0], F_SETFD, FD_CLOEXEC);
        (void)fcntl(err_pipe[1], F_SETFD, FD_CLOEXEC);
    }
    pid_t pid = fork();
    if (pid < 0) {
        if (err_pipe[0] >= 0) { close(err_pipe[0]); close(err_pipe[1]); }
        for (int64_t i = 0; i <= nargs; i++) free(argv[i]);
        free(argv);
        return 0;
    }
    if (pid == 0) {
        if (err_pipe[0] >= 0) close(err_pipe[0]);
        execvp(cmd, argv);
        int err_val = errno;
        if (err_pipe[1] >= 0) {
            ssize_t wr = write(err_pipe[1], &err_val, sizeof(err_val));
            (void)wr;
            close(err_pipe[1]);
        }
        _exit(127);
    }
    for (int64_t i = 0; i <= nargs; i++) free(argv[i]);
    free(argv);
    if (err_pipe[1] >= 0) close(err_pipe[1]);
    if (err_pipe[0] >= 0) {
        int child_err = 0;
        ssize_t rd = 0;
        do {
            rd = read(err_pipe[0], &child_err, sizeof(child_err));
        } while (rd < 0 && errno == EINTR);
        close(err_pipe[0]);
        if (rd > 0) {
            int st = 0;
            while (waitpid(pid, &st, 0) < 0 && errno == EINTR) {}
            return 0;
        }
    }
    joss_obj_set_field_i64(proc, "_started", 1);
    joss_obj_set_field_i64(proc, "_pid", (int64_t)pid);
    return 1;
#else
    (void)nargs;
    return 0;
#endif
}

int64_t fn_Process_wait(void* proc) {
    proc = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)proc);
    if (!proc || !joss_is_map_ptr((int64_t)(uintptr_t)proc)) return -1;
    if (joss_obj_get_field_i64(proc, "_started") == 0) return -1;
    if (joss_obj_get_field_i64(proc, "_waited") != 0) {
        return joss_obj_get_field_i64(proc, "_exit_code");
    }
#if defined(_WIN32)
    HANDLE hProc = (HANDLE)(uintptr_t)joss_obj_get_field_i64(proc, "_handle");
    if (!hProc) return -1;
    WaitForSingleObject(hProc, INFINITE);
    DWORD ec = 0;
    int64_t exit_code = GetExitCodeProcess(hProc, &ec) ? (int64_t)(int32_t)ec : -1;
    CloseHandle(hProc);
    joss_obj_set_field_i64(proc, "_handle", 0);
    joss_obj_set_field_i64(proc, "_waited", 1);
    joss_obj_set_field_i64(proc, "_exit_code", exit_code);
    return exit_code;
#elif defined(__unix__) || defined(__APPLE__)
    pid_t pid = (pid_t)joss_obj_get_field_i64(proc, "_pid");
    if (pid <= 0) return -1;
    int status = 0;
    while (waitpid(pid, &status, 0) < 0) {
        if (errno != EINTR) return -1;
    }
    int64_t exit_code = -1;
    if (WIFEXITED(status)) {
        exit_code = (int64_t)WEXITSTATUS(status);
    }
    joss_obj_set_field_i64(proc, "_waited", 1);
    joss_obj_set_field_i64(proc, "_exit_code", exit_code);
    return exit_code;
#else
    return -1;
#endif
}

int64_t fn_Process_kill(void* proc) {
    proc = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)proc);
    if (!proc || !joss_is_map_ptr((int64_t)(uintptr_t)proc)) return 0;
    if (joss_obj_get_field_i64(proc, "_started") == 0) return 0;
#if defined(_WIN32)
    HANDLE hProc = (HANDLE)(uintptr_t)joss_obj_get_field_i64(proc, "_handle");
    if (!hProc) return 0;
    return TerminateProcess(hProc, 1) ? 1 : 0;
#elif defined(__unix__) || defined(__APPLE__)
    if (joss_obj_get_field_i64(proc, "_waited") != 0) return 0;
    pid_t pid = (pid_t)joss_obj_get_field_i64(proc, "_pid");
    if (pid <= 0) return 0;
    return (kill(pid, SIGKILL) == 0) ? 1 : 0;
#else
    return 0;
#endif
}

int64_t fn_Process_pid(void* proc) {
    proc = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)proc);
    if (!proc || !joss_is_map_ptr((int64_t)(uintptr_t)proc)) return 0;
    return joss_obj_get_field_i64(proc, "_pid");
}

int64_t joss_usleep(int64_t usec) {
    if (usec <= 0) return 0;
#if defined(_WIN32)
    Sleep((DWORD)((usec + 999) / 1000));
#else
    usleep((useconds_t)usec);
#endif
    return 0;
}

int64_t joss_is_dir(const char* path) {
    path = unbox_web_cstr(path);
    if (!path || !path[0]) return 0;
#if defined(_WIN32)
    DWORD attr = GetFileAttributesA(path);
    return (attr != INVALID_FILE_ATTRIBUTES && (attr & FILE_ATTRIBUTE_DIRECTORY)) ? 1 : 0;
#else
    struct stat st;
    if (stat(path, &st) != 0) return 0;
    return S_ISDIR(st.st_mode) ? 1 : 0;
#endif
}

int64_t joss_is_file(const char* path) {
    path = unbox_web_cstr(path);
    if (!path || !path[0]) return 0;
#if defined(_WIN32)
    DWORD attr = GetFileAttributesA(path);
    return (attr != INVALID_FILE_ATTRIBUTES && !(attr & FILE_ATTRIBUTE_DIRECTORY)) ? 1 : 0;
#else
    struct stat st;
    if (stat(path, &st) != 0) return 0;
    return S_ISREG(st.st_mode) ? 1 : 0;
#endif
}

static uint64_t g_joss_rand_state = 0;

int64_t joss_rand(int64_t min_val, int64_t max_val) {
    joss_global_lock(JOSS_LOCK_RAND);
    if (g_joss_rand_state == 0) {
        g_joss_rand_state = (uint64_t)time(NULL) ^ 0x9E3779B97F4A7C15ULL;
    }
    /* xorshift64* */
    uint64_t x = g_joss_rand_state;
    x ^= x >> 12;
    x ^= x << 25;
    x ^= x >> 27;
    g_joss_rand_state = x;
    joss_global_unlock(JOSS_LOCK_RAND);
    uint64_t r = x * 0x2545F4914F6CDD1DULL;
    if (max_val <= min_val) return min_val;
    uint64_t span = (uint64_t)(max_val - min_val) + 1ULL;
    return min_val + (int64_t)(r % span);
}

int64_t joss_ord(const char* s) {
    s = unbox_web_cstr(s);
    if (!s || !s[0]) return 0;
    return (int64_t)(unsigned char)s[0];
}

int64_t joss_floor(int64_t val_bits) {
    double d = joss_i64_to_double_bits(val_bits);
    int64_t i = (int64_t)d;
    if (d < (double)i) i--;
    return joss_double_to_i64_bits((double)i);
}

int64_t joss_ceil(int64_t val_bits) {
    double d = joss_i64_to_double_bits(val_bits);
    int64_t i = (int64_t)d;
    if (d > (double)i) i++;
    return joss_double_to_i64_bits((double)i);
}

int64_t joss_round(int64_t val_bits) {
    double d = joss_i64_to_double_bits(val_bits);
    double r = (d >= 0.0) ? (double)((int64_t)(d + 0.5)) : (double)((int64_t)(d - 0.5));
    return joss_double_to_i64_bits(r);
}

const char* joss_config(const char* key, const char* def) {
    key = unbox_web_cstr(key);
    def = unbox_web_cstr(def);
    if (!key) return def ? def : "";
    const char* v = joss_env(key, def ? def : "");
    return v ? v : (def ? def : "");
}

const char* joss_toon_encode(const char* data) {
    data = unbox_web_cstr(data);
    if (!data) return "";
    return joss_str_concat("TOON:", data);
}

int64_t joss_toon_verify(const char* token) {
    token = unbox_web_cstr(token);
    if (!token) return 0;
    return (strncmp(token, "TOON:", 5) == 0) ? 1 : 0;
}

/* --- Console Host Class --- */
void fn_Console_log(const char* msg) {
    msg = unbox_web_cstr(msg);
    printf("%s\n", msg ? msg : "");
    fflush(stdout);
}

void fn_Console_error(const char* msg) {
    msg = unbox_web_cstr(msg);
    fprintf(stderr, "%s\n", msg ? msg : "");
    fflush(stderr);
}

const char* fn_Console_readLine(void) {
    char buf[4096];
    if (!fgets(buf, sizeof(buf), stdin)) return "";
    size_t n = strlen(buf);
    while (n > 0 && (buf[n - 1] == '\n' || buf[n - 1] == '\r')) buf[--n] = '\0';
    return joss_req_strdup(buf);
}

static const char* joss_console_wrap_ansi(const char* code, int64_t text_raw) {
    if (!code) code = "";
    if (text_raw == 0) {
        return joss_req_strdup(code);
    }
    const char* text = joss_strval(text_raw);
    if (!text) text = "";
    size_t clen = strlen(code);
    size_t tlen = strlen(text);
    const char* reset = "\033[0m";
    size_t rlen = 4;
    char* out = (char*)joss_req_alloc(clen + tlen + rlen + 1);
    if (!out) return text;
    memcpy(out, code, clen);
    memcpy(out + clen, text, tlen);
    memcpy(out + clen + tlen, reset, rlen);
    out[clen + tlen + rlen] = '\0';
    return out;
}

const char* fn_Console_green(int64_t text)   { return joss_console_wrap_ansi("\033[32m", text); }
const char* fn_Console_red(int64_t text)     { return joss_console_wrap_ansi("\033[31m", text); }
const char* fn_Console_yellow(int64_t text)  { return joss_console_wrap_ansi("\033[33m", text); }
const char* fn_Console_blue(int64_t text)    { return joss_console_wrap_ansi("\033[34m", text); }
const char* fn_Console_cyan(int64_t text)    { return joss_console_wrap_ansi("\033[36m", text); }
const char* fn_Console_magenta(int64_t text) { return joss_console_wrap_ansi("\033[35m", text); }
const char* fn_Console_gray(int64_t text)    { return joss_console_wrap_ansi("\033[90m", text); }
const char* fn_Console_bold(int64_t text)    { return joss_console_wrap_ansi("\033[1m", text); }
const char* fn_Console_clear(void)           { return joss_req_strdup("\033[2J\033[H"); }

const char* fn_Console_color(int64_t text_raw, int64_t code_raw) {
    const char* text = text_raw ? joss_strval(text_raw) : "";
    const char* code = code_raw ? joss_strval(code_raw) : "";
    if (!code || !code[0]) return joss_req_strdup(text ? text : "");
    char prefix[64];
    snprintf(prefix, sizeof(prefix), "\033[%sm", code);
    return joss_console_wrap_ansi(prefix, text_raw);
}

/* --- FileStream, StreamReader & StreamWriter Host Classes --- */
#define JOSS_FILESTREAM_MAGIC 0x4A4653545631ULL

typedef struct {
    uint64_t magic;
    FILE*    fp;
    int      owned;
} JossFileStream;

static JossFileStream* joss_fs_unwrap(void* handle) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)handle);
    if (raw >= (int64_t)JOSS_MIN_PTR_ADDR && ((uint64_t)raw >> JOSS_PTR_VA_BITS) == 0) {
        JossFileStream* fs = (JossFileStream*)(uintptr_t)(uint64_t)raw;
        if (fs->magic == JOSS_FILESTREAM_MAGIC) {
            return fs;
        }
    }
    return NULL;
}

static void* joss_fs_wrap(FILE* fp, int owned) {
    if (!fp) return NULL;
    JossFileStream* fs = (JossFileStream*)malloc(sizeof(JossFileStream));
    if (!fs) {
        if (owned) fclose(fp);
        return NULL;
    }
    fs->magic = JOSS_FILESTREAM_MAGIC;
    fs->fp = fp;
    fs->owned = owned;
    return (void*)fs;
}

void* fn_FileStream_open(const char* path, const char* mode) {
    path = unbox_web_cstr(path);
    mode = unbox_web_cstr(mode);
    if (!path || !path[0]) return NULL;
    const char* cmode = "rb";
    if (mode && mode[0]) {
        if (strcmp(mode, "w") == 0) cmode = "wb";
        else if (strcmp(mode, "a") == 0) cmode = "ab";
        else if (strcmp(mode, "r+") == 0) cmode = "r+b";
        else if (strcmp(mode, "w+") == 0) cmode = "w+b";
        else if (strcmp(mode, "a+") == 0) cmode = "a+b";
        else if (strcmp(mode, "r") == 0) cmode = "rb";
        else cmode = mode;
    }
    return joss_fs_wrap(fopen(path, cmode), 1);
}

const char* fn_FileStream_read(void* handle, int64_t max_bytes) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (!fs || !fs->fp) return "";
    max_bytes = joss_unbox_value(max_bytes);
    if (max_bytes <= 0) {
        JossStringBuilder* sb = joss_sb_new();
        uint8_t chunk[4096];
        for (;;) {
            size_t n = fread(chunk, 1, sizeof(chunk), fs->fp);
            if (n > 0) joss_buf_append_bytes(sb, chunk, (int64_t)n);
            if (n < sizeof(chunk)) break;
        }
        char* out = joss_sb_to_req_string(sb);
        joss_sb_free(sb);
        return out;
    }
    if (max_bytes > 16 * 1024 * 1024) max_bytes = 16 * 1024 * 1024;
    char* buf = (char*)joss_req_alloc((size_t)max_bytes + 1);
    if (!buf) return "";
    size_t n = fread(buf, 1, (size_t)max_bytes, fs->fp);
    buf[n] = '\0';
    return buf;
}

int64_t fn_FileStream_write(void* handle, const char* data) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    data = unbox_web_cstr(data);
    if (!fs || !fs->fp || !data) return 0;
    size_t n = strlen(data);
    size_t w = fwrite(data, 1, n, fs->fp);
    fflush(fs->fp);
    return (int64_t)w;
}

int64_t fn_FileStream_seek(void* handle, int64_t offset, int64_t whence) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (!fs || !fs->fp) return -1;
    int w = SEEK_SET;
    int64_t wv = joss_unbox_value(whence);
    if (wv == 1) w = SEEK_CUR;
    else if (wv == 2) w = SEEK_END;
    if (fseek(fs->fp, (long)joss_unbox_value(offset), w) != 0) return -1;
    return (int64_t)ftell(fs->fp);
}

int64_t fn_FileStream_size(void* handle) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (!fs || !fs->fp) return 0;
    long cur = ftell(fs->fp);
    if (fseek(fs->fp, 0, SEEK_END) != 0) return 0;
    long sz = ftell(fs->fp);
    if (cur >= 0) fseek(fs->fp, cur, SEEK_SET);
    return sz >= 0 ? (int64_t)sz : 0;
}

void fn_FileStream_flush(void* handle) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (fs && fs->fp) {
        fflush(fs->fp);
    }
}

void fn_FileStream_close(void* handle) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (fs && fs->fp) {
        if (fs->owned) fclose(fs->fp);
        fs->fp = NULL;
    }
}

void* fn_StreamReader_open(void* source) {
    JossFileStream* existing = joss_fs_unwrap(source);
    if (existing && existing->fp) {
        return joss_fs_wrap(existing->fp, 0);
    }
    const char* path = unbox_web_cstr((const char*)source);
    if (!path || !path[0]) return NULL;
    return joss_fs_wrap(fopen(path, "rb"), 1);
}

const char* fn_StreamReader_readLine(void* handle) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (!fs || !fs->fp) return NULL;
    char buf[4096];
    if (!fgets(buf, (int)sizeof(buf), fs->fp)) {
        return NULL;
    }
    size_t len = strlen(buf);
    if (len > 0 && buf[len - 1] == '\n') {
        len--;
        if (len > 0 && buf[len - 1] == '\r') len--;
        char* out = (char*)joss_req_alloc(len + 1);
        if (!out) return "";
        memcpy(out, buf, len);
        out[len] = '\0';
        return out;
    }
    if (feof(fs->fp)) {
        if (len > 0 && buf[len - 1] == '\r') len--;
        char* out = (char*)joss_req_alloc(len + 1);
        if (!out) return "";
        memcpy(out, buf, len);
        out[len] = '\0';
        return out;
    }
    /* Long line (> 4095 bytes): accumulate remaining chunks */
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, buf);
    while (fgets(buf, (int)sizeof(buf), fs->fp)) {
        size_t clen = strlen(buf);
        int done = (clen > 0 && buf[clen - 1] == '\n');
        if (done) buf[--clen] = '\0';
        joss_sb_append(sb, buf);
        if (done) break;
    }
    if (sb->len > 0 && sb->data[sb->len - 1] == '\r') {
        sb->len--;
        sb->data[sb->len] = '\0';
    }
    char* out = joss_sb_to_req_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* fn_StreamReader_readToEnd(void* handle) {
    return fn_FileStream_read(handle, 0);
}

void fn_StreamReader_close(void* handle) {
    fn_FileStream_close(handle);
}

void* fn_StreamWriter_open(void* dest, int64_t append) {
    JossFileStream* existing = joss_fs_unwrap(dest);
    if (existing && existing->fp) {
        return joss_fs_wrap(existing->fp, 0);
    }
    const char* path = unbox_web_cstr((const char*)dest);
    if (!path || !path[0]) return NULL;
    const char* cmode = (joss_unbox_value(append) != 0) ? "ab" : "wb";
    return joss_fs_wrap(fopen(path, cmode), 1);
}

int64_t fn_StreamWriter_write(void* handle, const char* text) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    text = unbox_web_cstr(text);
    if (!fs || !fs->fp || !text) return 0;
    size_t n = strlen(text);
    return (int64_t)fwrite(text, 1, n, fs->fp);
}

int64_t fn_StreamWriter_writeLine(void* handle, const char* text) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    text = unbox_web_cstr(text);
    if (!fs || !fs->fp) return 0;
    size_t n = text ? strlen(text) : 0;
    size_t w = n > 0 ? fwrite(text, 1, n, fs->fp) : 0;
    w += fwrite("\n", 1, 1, fs->fp);
    return (int64_t)w;
}

void fn_StreamWriter_flush(void* handle) {
    fn_FileStream_flush(handle);
}

void fn_StreamWriter_close(void* handle) {
    JossFileStream* fs = joss_fs_unwrap(handle);
    if (fs && fs->fp) {
        fflush(fs->fp);
        if (fs->owned) fclose(fs->fp);
        fs->fp = NULL;
    }
}

/* --- DateTime & DateInterval Host Classes --- */
#define JOSS_DATETIME_MAGIC     0x4A44544D5631ULL
#define JOSS_DATEINTERVAL_MAGIC 0x4A44494E5631ULL

typedef struct {
    uint64_t magic;
    int64_t  unix_sec;
} JossDateTime;

typedef struct {
    uint64_t magic;
    int64_t  total_sec;
} JossDateInterval;

static void* joss_dt_new(int64_t unix_sec) {
    JossDateTime* dt = (JossDateTime*)joss_req_alloc(sizeof(JossDateTime));
    if (!dt) return NULL;
    dt->magic = JOSS_DATETIME_MAGIC;
    dt->unix_sec = unix_sec;
    return (void*)dt;
}

static void* joss_di_new(int64_t total_sec) {
    JossDateInterval* di = (JossDateInterval*)joss_req_alloc(sizeof(JossDateInterval));
    if (!di) return NULL;
    di->magic = JOSS_DATEINTERVAL_MAGIC;
    di->total_sec = total_sec;
    return (void*)di;
}

static int64_t joss_dt_extract_sec(void* self) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)self);
    if (raw >= (int64_t)JOSS_MIN_PTR_ADDR && ((uint64_t)raw >> JOSS_PTR_VA_BITS) == 0) {
        const JossDateTime* dt = (const JossDateTime*)(uintptr_t)(uint64_t)raw;
        if (dt->magic == JOSS_DATETIME_MAGIC) {
            return dt->unix_sec;
        }
    }
    return (int64_t)time(NULL);
}

static int64_t joss_di_extract_sec(int64_t val) {
    int64_t raw = joss_unbox_value(val);
    if (raw >= (int64_t)JOSS_MIN_PTR_ADDR && ((uint64_t)raw >> JOSS_PTR_VA_BITS) == 0) {
        const JossDateInterval* di = (const JossDateInterval*)(uintptr_t)(uint64_t)raw;
        if (di->magic == JOSS_DATEINTERVAL_MAGIC) {
            return di->total_sec;
        }
    }
    return raw;
}

static void joss_dt_to_local_tm(int64_t unix_sec, struct tm* out_tm) {
    time_t t = (time_t)unix_sec;
#if defined(_WIN32)
    localtime_s(out_tm, &t);
#else
    localtime_r(&t, out_tm);
#endif
}

void* fn_DateInterval_days(int64_t n) {
    return joss_di_new(joss_unbox_value(n) * 86400LL);
}

void* fn_DateInterval_hours(int64_t n) {
    return joss_di_new(joss_unbox_value(n) * 3600LL);
}

void* fn_DateInterval_minutes(int64_t n) {
    return joss_di_new(joss_unbox_value(n) * 60LL);
}

void* fn_DateInterval_seconds(int64_t n) {
    return joss_di_new(joss_unbox_value(n));
}

int64_t fn_DateInterval_totalSeconds(void* self) {
    return joss_di_extract_sec((int64_t)(uintptr_t)self);
}

void* fn_DateTime_now(void) {
    return joss_dt_new((int64_t)time(NULL));
}

void* fn_DateTime_create(int64_t year, int64_t month, int64_t day) {
    struct tm tm_val;
    memset(&tm_val, 0, sizeof(tm_val));
    tm_val.tm_year = (int)joss_unbox_value(year) - 1900;
    tm_val.tm_mon  = (int)joss_unbox_value(month) - 1;
    tm_val.tm_mday = (int)joss_unbox_value(day);
    tm_val.tm_isdst = -1;
    time_t t = mktime(&tm_val);
    return joss_dt_new((int64_t)t);
}

void* fn_DateTime_parse(const char* str) {
    str = unbox_web_cstr(str);
    if (!str || !str[0]) {
        return joss_dt_new((int64_t)time(NULL));
    }
    int y = 0, mo = 1, d = 1, h = 0, mi = 0, s = 0;
    int tzh = 0, tzm = 0;
    char tzsign = 0;
    if (sscanf(str, "%d-%d-%dT%d:%d:%d%c%d:%d", &y, &mo, &d, &h, &mi, &s, &tzsign, &tzh, &tzm) >= 7 &&
        (tzsign == 'Z' || tzsign == 'z' || tzsign == '+' || tzsign == '-')) {
        struct tm tm_val;
        memset(&tm_val, 0, sizeof(tm_val));
        tm_val.tm_year = y - 1900;
        tm_val.tm_mon  = mo - 1;
        tm_val.tm_mday = d;
        tm_val.tm_hour = h;
        tm_val.tm_min  = mi;
        tm_val.tm_sec  = s;
#if defined(_WIN32)
        int64_t utc_sec = (int64_t)_mkgmtime(&tm_val);
#else
        int64_t utc_sec = (int64_t)timegm(&tm_val);
#endif
        int64_t off = (int64_t)tzh * 3600LL + (int64_t)tzm * 60LL;
        if (tzsign == '+') utc_sec -= off;
        else if (tzsign == '-') utc_sec += off;
        return joss_dt_new(utc_sec);
    }
    if (sscanf(str, "%d-%d-%d %d:%d:%d", &y, &mo, &d, &h, &mi, &s) >= 3 ||
        sscanf(str, "%d-%d-%dT%d:%d:%d", &y, &mo, &d, &h, &mi, &s) >= 3) {
        struct tm tm_val;
        memset(&tm_val, 0, sizeof(tm_val));
        tm_val.tm_year = y - 1900;
        tm_val.tm_mon  = mo - 1;
        tm_val.tm_mday = d;
        tm_val.tm_hour = h;
        tm_val.tm_min  = mi;
        tm_val.tm_sec  = s;
        tm_val.tm_isdst = -1;
        return joss_dt_new((int64_t)mktime(&tm_val));
    }
    return joss_dt_new(joss_strtotime(str));
}

int64_t fn_DateTime_timestamp(void* self) {
    return joss_dt_extract_sec(self);
}

const char* fn_DateTime_format(void* self, const char* fmt) {
    int64_t ts = joss_dt_extract_sec(self);
    fmt = unbox_web_cstr(fmt);
    if (!fmt || !fmt[0]) fmt = "Y-m-d H:i:s";
    struct tm tm_val;
    joss_dt_to_local_tm(ts, &tm_val);
    JossStringBuilder* sb = joss_sb_new();
    char tmp[32];
    for (const char* p = fmt; *p; p++) {
        switch (*p) {
            case 'Y':
                snprintf(tmp, sizeof(tmp), "%04d", tm_val.tm_year + 1900);
                joss_sb_append(sb, tmp);
                break;
            case 'y':
                snprintf(tmp, sizeof(tmp), "%02d", (tm_val.tm_year + 1900) % 100);
                joss_sb_append(sb, tmp);
                break;
            case 'm':
                snprintf(tmp, sizeof(tmp), "%02d", tm_val.tm_mon + 1);
                joss_sb_append(sb, tmp);
                break;
            case 'd':
                snprintf(tmp, sizeof(tmp), "%02d", tm_val.tm_mday);
                joss_sb_append(sb, tmp);
                break;
            case 'H':
                snprintf(tmp, sizeof(tmp), "%02d", tm_val.tm_hour);
                joss_sb_append(sb, tmp);
                break;
            case 'h': {
                int h12 = tm_val.tm_hour % 12;
                if (h12 == 0) h12 = 12;
                snprintf(tmp, sizeof(tmp), "%02d", h12);
                joss_sb_append(sb, tmp);
                break;
            }
            case 'i':
                snprintf(tmp, sizeof(tmp), "%02d", tm_val.tm_min);
                joss_sb_append(sb, tmp);
                break;
            case 's':
                snprintf(tmp, sizeof(tmp), "%02d", tm_val.tm_sec);
                joss_sb_append(sb, tmp);
                break;
            case 'a':
                joss_sb_append(sb, tm_val.tm_hour < 12 ? "am" : "pm");
                break;
            case 'A':
                joss_sb_append(sb, tm_val.tm_hour < 12 ? "AM" : "PM");
                break;
            default:
                joss_buf_append_byte(sb, (uint8_t)*p);
                break;
        }
    }
    char* out = joss_sb_to_req_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* fn_DateTime_iso(void* self) {
    int64_t ts = joss_dt_extract_sec(self);
    struct tm loc_tm, utc_tm;
    joss_dt_to_local_tm(ts, &loc_tm);
    time_t t = (time_t)ts;
#if defined(_WIN32)
    gmtime_s(&utc_tm, &t);
#else
    gmtime_r(&t, &utc_tm);
#endif
    utc_tm.tm_isdst = 0;
    int64_t loc_as_utc = 0, utc_as_utc = 0;
#if defined(_WIN32)
    loc_as_utc = (int64_t)_mkgmtime(&loc_tm);
    utc_as_utc = (int64_t)_mkgmtime(&utc_tm);
#else
    loc_as_utc = (int64_t)timegm(&loc_tm);
    utc_as_utc = (int64_t)timegm(&utc_tm);
#endif
    int64_t off_sec = loc_as_utc - utc_as_utc;
    char buf[64];
    if (off_sec == 0) {
        snprintf(buf, sizeof(buf), "%04d-%02d-%02dT%02d:%02d:%02dZ",
            loc_tm.tm_year + 1900, loc_tm.tm_mon + 1, loc_tm.tm_mday,
            loc_tm.tm_hour, loc_tm.tm_min, loc_tm.tm_sec);
    } else {
        char sign = off_sec >= 0 ? '+' : '-';
        int64_t abs_off = off_sec >= 0 ? off_sec : -off_sec;
        int oh = (int)(abs_off / 3600);
        int om = (int)((abs_off % 3600) / 60);
        snprintf(buf, sizeof(buf), "%04d-%02d-%02dT%02d:%02d:%02d%c%02d:%02d",
            loc_tm.tm_year + 1900, loc_tm.tm_mon + 1, loc_tm.tm_mday,
            loc_tm.tm_hour, loc_tm.tm_min, loc_tm.tm_sec, sign, oh, om);
    }
    return joss_req_strdup(buf);
}

int64_t fn_DateTime_year(void* self) {
    struct tm tm_val;
    joss_dt_to_local_tm(joss_dt_extract_sec(self), &tm_val);
    return (int64_t)(tm_val.tm_year + 1900);
}

int64_t fn_DateTime_month(void* self) {
    struct tm tm_val;
    joss_dt_to_local_tm(joss_dt_extract_sec(self), &tm_val);
    return (int64_t)(tm_val.tm_mon + 1);
}

int64_t fn_DateTime_day(void* self) {
    struct tm tm_val;
    joss_dt_to_local_tm(joss_dt_extract_sec(self), &tm_val);
    return (int64_t)tm_val.tm_mday;
}

int64_t fn_DateTime_hour(void* self) {
    struct tm tm_val;
    joss_dt_to_local_tm(joss_dt_extract_sec(self), &tm_val);
    return (int64_t)tm_val.tm_hour;
}

int64_t fn_DateTime_minute(void* self) {
    struct tm tm_val;
    joss_dt_to_local_tm(joss_dt_extract_sec(self), &tm_val);
    return (int64_t)tm_val.tm_min;
}

int64_t fn_DateTime_second(void* self) {
    struct tm tm_val;
    joss_dt_to_local_tm(joss_dt_extract_sec(self), &tm_val);
    return (int64_t)tm_val.tm_sec;
}

void* fn_DateTime_add(void* self, int64_t interval_val) {
    int64_t base = joss_dt_extract_sec(self);
    int64_t delta = joss_di_extract_sec(interval_val);
    return joss_dt_new(base + delta);
}

void* fn_DateTime_sub(void* self, int64_t interval_val) {
    int64_t base = joss_dt_extract_sec(self);
    int64_t delta = joss_di_extract_sec(interval_val);
    return joss_dt_new(base - delta);
}

void* fn_DateTime_diff(void* self, void* other) {
    int64_t t1 = joss_dt_extract_sec(self);
    int64_t t2 = joss_dt_extract_sec(other);
    int64_t d = t1 - t2;
    if (d < 0) d = -d;
    return joss_di_new(d);
}

/* --- Lang Host Class --- */
static char g_joss_locale[32] = "es";

const char* fn_Lang_locale(void) {
    joss_global_lock(JOSS_LOCK_GENERAL);
    char* copy = joss_req_strdup(g_joss_locale);
    joss_global_unlock(JOSS_LOCK_GENERAL);
    return copy;
}

int64_t fn_Lang_setLocale(const char* loc) {
    loc = unbox_web_cstr(loc);
    if (!loc || !loc[0]) return 0;
    joss_global_lock(JOSS_LOCK_GENERAL);
    snprintf(g_joss_locale, sizeof(g_joss_locale), "%s", loc);
    joss_global_unlock(JOSS_LOCK_GENERAL);
    return 1;
}

const char* fn_Lang_get(const char* key) {
    key = unbox_web_cstr(key);
    return key ? key : "";
}

/* --- WebSocket Connection Object ($ws) & Event Loop --- */
void* joss_ws_wrap(void* sock_val) {
    void* ws = joss_obj_new("WebSocket");
    int64_t fd = joss_unbox_value((int64_t)(uintptr_t)sock_val);
    joss_map_set_i64(ws, "_sock", (int64_t)(uintptr_t)joss_box_i64(fd));
    joss_map_set_i64(ws, "_closed", (int64_t)(uintptr_t)joss_box_i64(0));
    joss_map_set_i64(ws, "_on_message", 0);
    joss_map_set_i64(ws, "_on_close", 0);
    return ws;
}

static bool joss_ws_obj_get_socket(void* ws_ptr, joss_socket_t* out_sock) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)ws_ptr);
    if (raw && joss_is_map_ptr(raw)) {
        int64_t fd = joss_intval(joss_map_get_i64((void*)(uintptr_t)raw, "_sock"));
        return joss_ws_resolve_socket(fd, out_sock);
    }
    return joss_ws_resolve_socket(raw, out_sock);
}

int64_t joss_ws_obj_send(void* ws_ptr, const char* msg) {
    msg = unbox_web_cstr(msg);
    if (!msg) msg = "";
    joss_socket_t sock;
    if (!joss_ws_obj_get_socket(ws_ptr, &sock)) return 0;
    size_t plen = strlen(msg);
    uint8_t* frame = (uint8_t*)malloc(plen + 16U);
    if (!frame) return 0;
    size_t flen = joss_ws_encode_text_frame(msg, frame, plen + 16U);
    bool ok = (flen > 0) && joss_ws_send_all(sock, frame, flen);
    free(frame);
    return ok ? 1 : 0;
}

int64_t joss_ws_obj_on_message(void* ws_ptr, int64_t cb) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)ws_ptr);
    if (!raw || !joss_is_map_ptr(raw)) return 0;
    joss_map_set_i64((void*)(uintptr_t)raw, "_on_message", cb);
    return 1;
}

int64_t joss_ws_obj_on_close(void* ws_ptr, int64_t cb) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)ws_ptr);
    if (!raw || !joss_is_map_ptr(raw)) return 0;
    joss_map_set_i64((void*)(uintptr_t)raw, "_on_close", cb);
    return 1;
}

int64_t joss_ws_obj_subscribe(void* ws_ptr, const char* channel) {
    channel = unbox_web_cstr(channel);
    if (!channel || !channel[0]) return 0;
    joss_socket_t sock;
    if (!joss_ws_obj_get_socket(ws_ptr, &sock)) return 0;
    joss_ws_hub_subscribe_raw(channel, sock);
    return 1;
}

int64_t joss_ws_obj_close(void* ws_ptr) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)ws_ptr);
    if (raw && joss_is_map_ptr(raw)) {
        joss_map_set_i64((void*)(uintptr_t)raw, "_closed", (int64_t)(uintptr_t)joss_box_i64(1));
    }
    joss_socket_t sock;
    if (joss_ws_obj_get_socket(ws_ptr, &sock)) {
        (void)joss_ws_send_control_frame(sock, 0x88U, NULL, 0U);
        joss_ws_hub_unsubscribe_all(sock);
        joss_ws_close_socket(sock);
    }
    return 1;
}

const char* joss_ws_run_loop(void* ws_ptr) {
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)ws_ptr);
    void* ws_map = (raw && joss_is_map_ptr(raw)) ? (void*)(uintptr_t)raw : NULL;
    joss_socket_t sock;
    if (!ws_map || !joss_ws_obj_get_socket(ws_map, &sock)) {
        return "__JOSS_WEBSOCKET_HANDLED__";
    }
    int64_t fd = joss_intval(joss_map_get_i64(ws_map, "_sock"));
    while (joss_intval(joss_map_get_i64(ws_map, "_closed")) == 0) {
        int opcode = 0;
        char* payload = NULL;
        size_t plen = 0;
        int rc = joss_ws_read_frame(sock, &opcode, &payload, &plen);
        if (rc != 0) {
            free(payload);
            break;
        }
        if ((opcode == 0x1 || opcode == 0x2) && payload) {
            int64_t cb = joss_map_get_i64(ws_map, "_on_message");
            if (cb != 0) {
                void* fn = joss_closure_prepare_call((void*)(uintptr_t)cb);
                if (fn) {
                    (void)((JossUnaryCb)fn)((int64_t)(uintptr_t)payload);
                    joss_closure_env_tls_reset();
                }
            } else {
                (void)joss_ws_obj_send(ws_map, payload);
            }
        }
        free(payload);
    }
    int64_t close_cb = joss_map_get_i64(ws_map, "_on_close");
    if (close_cb != 0) {
        void* fn = joss_closure_prepare_call((void*)(uintptr_t)close_cb);
        if (fn) {
            ((JossCronCallbackFn)fn)();
            joss_closure_env_tls_reset();
        }
    }
    joss_ws_hub_unsubscribe_all(sock);
    joss_unregister_wrapped_socket((intptr_t)fd);
    if (joss_intval(joss_map_get_i64(ws_map, "_closed")) == 0) {
        joss_ws_close_socket(sock);
    }
    joss_req_state_reset();
    joss_req_arena_reset();
    return "__JOSS_WEBSOCKET_HANDLED__";
}

/* --- Http::json & Schema Host Services --- */
int64_t joss_json_decode(const char* s);

int64_t fn_Http_json(const char* method, const char* url, int64_t payload, void* headers) {
    method = unbox_web_cstr(method);
    url = unbox_web_cstr(url);
    if (!method || !method[0]) method = "POST";
    if (!url || !url[0]) return 0;

    const char* body_json = "";
    if (joss_is_not_null(payload)) {
        if (joss_is_map_ptr(payload) || joss_is_arr_ptr(payload)) {
            body_json = joss_json_encode(payload);
        } else {
            body_json = unbox_web_cstr((const char*)(uintptr_t)payload);
            if (!body_json) body_json = "";
        }
    }

    int64_t hdr_raw = joss_unbox_value((int64_t)(uintptr_t)headers);
    void* hdrs = (hdr_raw && joss_is_map_ptr(hdr_raw)) ? (void*)(uintptr_t)hdr_raw : joss_map_new();
    if (!joss_array_key_exists("Content-Type", hdrs) && !joss_array_key_exists("content-type", hdrs)) {
        joss_map_set_str(hdrs, "Content-Type", "application/json");
    }
    if (!joss_array_key_exists("Accept", hdrs) && !joss_array_key_exists("accept", hdrs)) {
        joss_map_set_str(hdrs, "Accept", "application/json");
    }

    void* resp = fn_Http_request(method, url, body_json, hdrs);
    if (!resp || !joss_is_map_ptr((int64_t)(uintptr_t)resp)) return 0;
    const char* resp_body = joss_map_get_str(resp, "body");
    if (!resp_body || !resp_body[0]) return 0;
    return joss_json_decode(resp_body);
}

static void* joss_schema_db_query(const char* sql, void* bindings) {
    return g_joss_schema_db_query ? g_joss_schema_db_query(sql, bindings) : joss_arr_new(0);
}

static int64_t joss_schema_db_exec(const char* sql, void* bindings) {
    return g_joss_schema_db_exec ? g_joss_schema_db_exec(sql, bindings) : -1;
}

static const char* joss_schema_dialect(void) {
    const char* drv = joss_env("DB", "");
    if (!drv || !*drv) drv = joss_env("DB_MOTOR", "");
    if (!drv || !*drv) drv = joss_env("DB_CONNECTION", "");
    if (!drv || !*drv) drv = joss_env("DB_DRIVER", "");
    if (drv && *drv) {
        if (strcasecmp(drv, "sqlite") == 0 || strcasecmp(drv, "sqlite3") == 0) return "sqlite";
        if (strcasecmp(drv, "pgsql") == 0 || strcasecmp(drv, "postgres") == 0 || strcasecmp(drv, "postgresql") == 0) return "postgres";
        if (strcasecmp(drv, "mysql") == 0 || strcasecmp(drv, "mariadb") == 0) return "mysql";
        if (strcasecmp(drv, "sqlsrv") == 0 || strcasecmp(drv, "mssql") == 0 || strcasecmp(drv, "odbc") == 0) return "sqlsrv";
    }
    const char* db_host = joss_env("DB_HOST", "");
    if (db_host && *db_host) return "mysql";
    const char* db_path = joss_env("DB_DATABASE", "");
    if (!db_path || !*db_path) db_path = joss_env("DB_PATH", "");
    if ((db_path && *db_path) || joss_file_exists("database.sqlite")) return "sqlite";
    return "mysql";
}

static void* joss_bp_add_col(void* bp, const char* name, const char* type_sql, bool is_pk) {
    bp = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)bp);
    name = unbox_web_cstr(name);
    if (!bp || !name || !name[0]) return bp;
    int64_t cols_v = joss_map_get_i64(bp, "_cols");
    void* cols = joss_is_arr_ptr(cols_v) ? (void*)(uintptr_t)cols_v : joss_arr_new(8);
    if (!joss_is_arr_ptr(cols_v)) joss_map_set_i64(bp, "_cols", (int64_t)(uintptr_t)cols);
    void* col = joss_map_new();
    joss_map_set_str(col, "name", joss_web_strdup(name));
    joss_map_set_str(col, "type", joss_web_strdup(type_sql ? type_sql : "TEXT"));
    joss_map_set_i64(col, "pk", is_pk ? 1 : 0);
    joss_map_set_i64(col, "nullable", is_pk ? 0 : 0);
    joss_map_set_i64(col, "unique", 0);
    joss_map_set_str(col, "default", "");
    joss_arr_push_i64(cols, (int64_t)(uintptr_t)col);
    joss_map_set_i64(bp, "_last_col", (int64_t)(uintptr_t)col);
    return bp;
}

void* fn_Blueprint_id(void* bp, const char* name) {
    name = unbox_web_cstr(name);
    if (!name || !name[0]) name = "id";
    const char* d = joss_schema_dialect();
    if (strcmp(d, "sqlite") == 0) return joss_bp_add_col(bp, name, "INTEGER PRIMARY KEY AUTOINCREMENT", true);
    if (strcmp(d, "postgres") == 0) return joss_bp_add_col(bp, name, "SERIAL PRIMARY KEY", true);
    return joss_bp_add_col(bp, name, "INT AUTO_INCREMENT PRIMARY KEY", true);
}
void* fn_Blueprint_increments(void* bp, const char* name) { return fn_Blueprint_id(bp, name); }
void* fn_Blueprint_char(void* bp, const char* name, int64_t len) {
    int64_t n = joss_intval(len);
    if (n <= 0) n = 255;
    char tb[48];
    snprintf(tb, sizeof(tb), "CHAR(%lld)", (long long)n);
    return joss_bp_add_col(bp, name, tb, false);
}
void* fn_Blueprint_string(void* bp, const char* name, int64_t len) {
    int64_t n = joss_intval(len);
    if (n <= 0) n = 255;
    char tb[48];
    snprintf(tb, sizeof(tb), "VARCHAR(%lld)", (long long)n);
    return joss_bp_add_col(bp, name, tb, false);
}
void* fn_Blueprint_integer(void* bp, const char* name) { return joss_bp_add_col(bp, name, "INTEGER", false); }
void* fn_Blueprint_tinyInteger(void* bp, const char* name) { return joss_bp_add_col(bp, name, "SMALLINT", false); }
void* fn_Blueprint_smallInteger(void* bp, const char* name) { return joss_bp_add_col(bp, name, "SMALLINT", false); }
void* fn_Blueprint_mediumInteger(void* bp, const char* name) { return joss_bp_add_col(bp, name, "INTEGER", false); }
void* fn_Blueprint_bigInteger(void* bp, const char* name) { return joss_bp_add_col(bp, name, "BIGINT", false); }
void* fn_Blueprint_unsignedInteger(void* bp, const char* name) { return joss_bp_add_col(bp, name, "INTEGER", false); }
void* fn_Blueprint_unsignedBigInteger(void* bp, const char* name) { return joss_bp_add_col(bp, name, "BIGINT", false); }
void* fn_Blueprint_text(void* bp, const char* name) { return joss_bp_add_col(bp, name, "TEXT", false); }
void* fn_Blueprint_mediumText(void* bp, const char* name) {
    const char* d = joss_schema_dialect();
    return joss_bp_add_col(bp, name, (strcmp(d, "sqlite") == 0 || strcmp(d, "postgres") == 0) ? "TEXT" : "MEDIUMTEXT", false);
}
void* fn_Blueprint_longText(void* bp, const char* name) {
    const char* d = joss_schema_dialect();
    return joss_bp_add_col(bp, name, (strcmp(d, "sqlite") == 0 || strcmp(d, "postgres") == 0) ? "TEXT" : "LONGTEXT", false);
}
void* fn_Blueprint_date(void* bp, const char* name) { return joss_bp_add_col(bp, name, "DATE", false); }
void* fn_Blueprint_dateTime(void* bp, const char* name) {
    const char* d = joss_schema_dialect();
    return joss_bp_add_col(bp, name, (strcmp(d, "postgres") == 0) ? "TIMESTAMP" : "DATETIME", false);
}
void* fn_Blueprint_time(void* bp, const char* name) { return joss_bp_add_col(bp, name, "TIME", false); }
void* fn_Blueprint_boolean(void* bp, const char* name) { return joss_bp_add_col(bp, name, "BOOLEAN", false); }
void* fn_Blueprint_json(void* bp, const char* name) {
    const char* d = joss_schema_dialect();
    if (strcmp(d, "postgres") == 0) return joss_bp_add_col(bp, name, "JSONB", false);
    if (strcmp(d, "sqlite") == 0) return joss_bp_add_col(bp, name, "TEXT", false);
    return joss_bp_add_col(bp, name, "JSON", false);
}
void* fn_Blueprint_enum(void* bp, const char* name, void* vals) { (void)vals; return joss_bp_add_col(bp, name, "TEXT", false); }
void* fn_Blueprint_float(void* bp, const char* name) { return joss_bp_add_col(bp, name, "DOUBLE PRECISION", false); }
void* fn_Blueprint_double(void* bp, const char* name) { return joss_bp_add_col(bp, name, "DOUBLE PRECISION", false); }
void* fn_Blueprint_decimal(void* bp, const char* name, int64_t p, int64_t s) {
    int64_t pp = joss_intval(p), ss = joss_intval(s);
    if (pp <= 0) pp = 10;
    if (ss < 0) ss = 2;
    char tb[64];
    snprintf(tb, sizeof(tb), "DECIMAL(%lld,%lld)", (long long)pp, (long long)ss);
    return joss_bp_add_col(bp, name, tb, false);
}
void* fn_Blueprint_timestamp(void* bp, const char* name) { return joss_bp_add_col(bp, name, "TIMESTAMP", false); }
void* fn_Blueprint_timestamps(void* bp) {
    bp = joss_bp_add_col(bp, "created_at", "TIMESTAMP", false);
    return joss_bp_add_col(bp, "updated_at", "TIMESTAMP", false);
}
void* fn_Blueprint_softDeletes(void* bp) {
    bp = joss_bp_add_col(bp, "deleted_at", "TIMESTAMP", false);
    int64_t lv = bp ? joss_map_get_i64(bp, "_last_col") : 0;
    if (joss_is_map_ptr(lv)) joss_map_set_i64((void*)(uintptr_t)lv, "nullable", 1);
    return bp;
}
void* fn_Blueprint_nullable(void* bp) {
    bp = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)bp);
    int64_t lv = bp ? joss_map_get_i64(bp, "_last_col") : 0;
    if (joss_is_map_ptr(lv)) joss_map_set_i64((void*)(uintptr_t)lv, "nullable", 1);
    return bp;
}
void* fn_Blueprint_unique(void* bp, int64_t cols) {
    bp = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)bp);
    if (cols == 0) {
        int64_t lv = bp ? joss_map_get_i64(bp, "_last_col") : 0;
        if (joss_is_map_ptr(lv)) joss_map_set_i64((void*)(uintptr_t)lv, "unique", 1);
    }
    return bp;
}
void* fn_Blueprint_default(void* bp, int64_t val) {
    bp = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)bp);
    int64_t lv = bp ? joss_map_get_i64(bp, "_last_col") : 0;
    if (joss_is_map_ptr(lv)) joss_map_set_str((void*)(uintptr_t)lv, "default", joss_web_strdup(joss_strval(val)));
    return bp;
}
void* fn_Blueprint_unsigned(void* bp) { return bp; }
void* fn_Blueprint_comment(void* bp, const char* text) { (void)text; return bp; }
void* fn_Blueprint_dropColumn(void* bp, int64_t cols) { (void)cols; return bp; }
void* fn_Blueprint_renameColumn(void* bp, const char* from_col, const char* to_col) { (void)from_col; (void)to_col; return bp; }
void* fn_Blueprint_index(void* bp, int64_t cols, const char* name) { (void)cols; (void)name; return bp; }
void* fn_Blueprint_uniqueIndex(void* bp, int64_t cols, const char* name) { (void)cols; (void)name; return bp; }
void* fn_Blueprint_dropIndex(void* bp, const char* name) { (void)name; return bp; }
void* fn_Blueprint_foreign(void* bp, int64_t cols, const char* name) { (void)cols; (void)name; return bp; }
void* fn_Blueprint_references(void* bp, int64_t cols) { (void)cols; return bp; }
void* fn_Blueprint_on(void* bp, const char* table) { (void)table; return bp; }
void* fn_Blueprint_onDelete(void* bp, const char* action) { (void)action; return bp; }
void* fn_Blueprint_onUpdate(void* bp, const char* action) { (void)action; return bp; }
void* fn_Blueprint_dropForeign(void* bp, const char* name) { (void)name; return bp; }

static const char* joss_schema_prefixed_table(const char* table, char* buf, size_t cap) {
    if (!table || !table[0]) return "";
    const char* prefix = joss_env("PREFIX", "");
    if (!prefix || !prefix[0]) prefix = joss_env("DB_PREFIX", "");
    if (prefix && prefix[0]) {
        size_t plen = strlen(prefix);
        if (strncmp(table, prefix, plen) != 0) {
            snprintf(buf, cap, "%s%s", prefix, table);
            return buf;
        }
    }
    snprintf(buf, cap, "%s", table);
    return buf;
}

static void joss_schema_append_col_def(JossStringBuilder* sb, void* col) {
    const char* cname = joss_map_get_str(col, "name");
    const char* ctype = joss_map_get_str(col, "type");
    bool is_pk = joss_map_get_i64(col, "pk") != 0;
    bool nullable = joss_map_get_i64(col, "nullable") != 0;
    bool uniq = joss_map_get_i64(col, "unique") != 0;
    const char* defv = joss_map_get_str(col, "default");
    joss_sb_append(sb, cname);
    joss_sb_append(sb, " ");
    joss_sb_append(sb, ctype);
    if (!is_pk) {
        if (!nullable) joss_sb_append(sb, " NOT NULL");
        if (uniq) joss_sb_append(sb, " UNIQUE");
        if (defv && defv[0]) {
            if (ctype && strcmp(ctype, "BOOLEAN") == 0) {
                if (joss_strcasecmp_ascii(defv, "true") == 0 || strcmp(defv, "1") == 0) {
                    joss_sb_append(sb, " DEFAULT 1");
                } else {
                    joss_sb_append(sb, " DEFAULT 0");
                }
            } else {
                joss_sb_append(sb, " DEFAULT '");
                joss_sb_append(sb, defv);
                joss_sb_append(sb, "'");
            }
        }
    }
}

int64_t fn_Schema_create(const char* table, void* callback) {
    table = unbox_web_cstr(table);
    void* cb = joss_closure_prepare_call(callback);
    if (!table || !table[0] || !cb) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    void* bp = joss_obj_new("Blueprint");
    joss_map_set_str(bp, "_table", joss_web_strdup(tbl));
    joss_map_set_i64(bp, "_cols", (int64_t)(uintptr_t)joss_arr_new(8));
    ((void (*)(void*))cb)(bp);
    void* cols = (void*)(uintptr_t)joss_map_get_i64(bp, "_cols");
    int64_t n = cols ? joss_arr_len(cols) : 0;
    if (n <= 0) return 0;
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "CREATE TABLE IF NOT EXISTS ");
    joss_sb_append(sb, tbl);
    joss_sb_append(sb, " (");
    for (int64_t i = 0; i < n; i++) {
        if (i > 0) joss_sb_append(sb, ", ");
        void* col = (void*)(uintptr_t)joss_arr_get_i64(cols, i);
        if (joss_is_map_ptr((int64_t)(uintptr_t)col)) joss_schema_append_col_def(sb, col);
    }
    joss_sb_append(sb, ")");
    char* sql = joss_sb_to_string(sb);
    joss_sb_free(sb);
    int64_t rc = joss_schema_db_exec(sql, joss_arr_new(0));
    free(sql);
    return rc >= 0 ? 1 : 0;
}

int64_t fn_Schema_table(const char* table, void* callback) {
    table = unbox_web_cstr(table);
    void* cb = joss_closure_prepare_call(callback);
    if (!table || !table[0] || !cb) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    void* bp = joss_obj_new("Blueprint");
    joss_map_set_str(bp, "_table", joss_web_strdup(tbl));
    joss_map_set_i64(bp, "_cols", (int64_t)(uintptr_t)joss_arr_new(4));
    ((void (*)(void*))cb)(bp);
    void* cols = (void*)(uintptr_t)joss_map_get_i64(bp, "_cols");
    int64_t n = cols ? joss_arr_len(cols) : 0;
    for (int64_t i = 0; i < n; i++) {
        void* col = (void*)(uintptr_t)joss_arr_get_i64(cols, i);
        if (!joss_is_map_ptr((int64_t)(uintptr_t)col)) continue;
        JossStringBuilder* sb = joss_sb_new();
        joss_sb_append(sb, "ALTER TABLE ");
        joss_sb_append(sb, tbl);
        joss_sb_append(sb, " ADD COLUMN ");
        joss_schema_append_col_def(sb, col);
        char* sql = joss_sb_to_string(sb);
        joss_sb_free(sb);
        joss_schema_db_exec(sql, joss_arr_new(0));
        free(sql);
    }
    return 1;
}

int64_t fn_Schema_drop(const char* table) {
    table = unbox_web_cstr(table);
    if (!table || !table[0]) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    char sql[256];
    snprintf(sql, sizeof(sql), "DROP TABLE %s", tbl);
    return joss_schema_db_exec(sql, joss_arr_new(0)) >= 0 ? 1 : 0;
}

int64_t fn_Schema_dropIfExists(const char* table) {
    table = unbox_web_cstr(table);
    if (!table || !table[0]) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    char sql[256];
    snprintf(sql, sizeof(sql), "DROP TABLE IF EXISTS %s", tbl);
    return joss_schema_db_exec(sql, joss_arr_new(0)) >= 0 ? 1 : 0;
}

int64_t fn_Schema_rename(const char* from, const char* to) {
    from = unbox_web_cstr(from);
    to = unbox_web_cstr(to);
    if (!from || !from[0] || !to || !to[0]) return 0;
    char full_from[192], full_to[192];
    const char* f = joss_schema_prefixed_table(from, full_from, sizeof(full_from));
    const char* t = joss_schema_prefixed_table(to, full_to, sizeof(full_to));
    char sql[448];
    snprintf(sql, sizeof(sql), "ALTER TABLE %s RENAME TO %s", f, t);
    return joss_schema_db_exec(sql, joss_arr_new(0)) >= 0 ? 1 : 0;
}

int64_t fn_Schema_hasTable(const char* table) {
    table = unbox_web_cstr(table);
    if (!table || !table[0]) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    const char* d = joss_schema_dialect();
    char sql[256];
    if (strcmp(d, "sqlite") == 0) {
        snprintf(sql, sizeof(sql), "SELECT name FROM sqlite_master WHERE type='table' AND name='%s'", tbl);
    } else if (strcmp(d, "postgres") == 0) {
        snprintf(sql, sizeof(sql), "SELECT table_name FROM information_schema.tables WHERE table_name='%s'", tbl);
    } else {
        snprintf(sql, sizeof(sql), "SHOW TABLES LIKE '%s'", tbl);
    }
    void* rows = joss_schema_db_query(sql, joss_arr_new(0));
    return (rows && joss_arr_len(rows) > 0) ? 1 : 0;
}

int64_t fn_Schema_hasColumn(const char* table, const char* column) {
    table = unbox_web_cstr(table);
    column = unbox_web_cstr(column);
    if (!table || !table[0] || !column || !column[0]) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    const char* d = joss_schema_dialect();
    char sql[256];
    if (strcmp(d, "sqlite") == 0) {
        snprintf(sql, sizeof(sql), "PRAGMA table_info(%s)", tbl);
        void* rows = joss_schema_db_query(sql, joss_arr_new(0));
        int64_t n = rows ? joss_arr_len(rows) : 0;
        for (int64_t i = 0; i < n; i++) {
            void* r = (void*)(uintptr_t)joss_arr_get_i64(rows, i);
            if (joss_is_map_ptr((int64_t)(uintptr_t)r) && strcmp(joss_map_get_str(r, "name"), column) == 0) return 1;
        }
        return 0;
    } else if (strcmp(d, "postgres") == 0) {
        snprintf(sql, sizeof(sql), "SELECT column_name FROM information_schema.columns WHERE table_name='%s' AND column_name='%s'", tbl, column);
    } else {
        snprintf(sql, sizeof(sql), "SHOW COLUMNS FROM %s LIKE '%s'", tbl, column);
    }
    void* rows = joss_schema_db_query(sql, joss_arr_new(0));
    return (rows && joss_arr_len(rows) > 0) ? 1 : 0;
}

int64_t fn_Schema_hasForeign(const char* table, const char* constraint_name) {
    table = unbox_web_cstr(table);
    constraint_name = unbox_web_cstr(constraint_name);
    if (!table || !table[0] || !constraint_name || !constraint_name[0]) return 0;
    char full_table[192];
    const char* tbl = joss_schema_prefixed_table(table, full_table, sizeof(full_table));
    const char* d = joss_schema_dialect();
    if (strcmp(d, "sqlite") == 0) {
        void* params = joss_arr_new(1);
        joss_arr_push_i64(params, (int64_t)(uintptr_t)tbl);
        void* rows = joss_schema_db_query("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", params);
        if (!rows || joss_arr_len(rows) == 0) return 0;
        void* r = (void*)(uintptr_t)joss_arr_get_i64(rows, 0);
        if (!joss_is_map_ptr((int64_t)(uintptr_t)r)) return 0;
        const char* def = joss_map_get_str(r, "sql");
        if (!def || !def[0]) return 0;
        char pat1[256], pat2[256], pat3[256];
        snprintf(pat1, sizeof(pat1), "CONSTRAINT `%s`", constraint_name);
        snprintf(pat2, sizeof(pat2), "CONSTRAINT \"%s\"", constraint_name);
        snprintf(pat3, sizeof(pat3), "CONSTRAINT %s ", constraint_name);
        return (strstr(def, pat1) || strstr(def, pat2) || strstr(def, pat3)) ? 1 : 0;
    }
    void* params = joss_arr_new(2);
    joss_arr_push_i64(params, (int64_t)(uintptr_t)tbl);
    joss_arr_push_i64(params, (int64_t)(uintptr_t)constraint_name);
    const char* sql = NULL;
    if (strcmp(d, "postgres") == 0) {
        sql = "SELECT constraint_name FROM information_schema.table_constraints WHERE table_schema = current_schema() AND table_name = ? AND constraint_name = ? AND constraint_type = 'FOREIGN KEY'";
    } else if (strcmp(d, "sqlserver") == 0) {
        sql = "SELECT fk.name FROM sys.foreign_keys fk JOIN sys.tables t ON t.object_id = fk.parent_object_id WHERE t.name = ? AND fk.name = ?";
    } else {
        sql = "SELECT constraint_name FROM information_schema.table_constraints WHERE table_schema = DATABASE() AND table_name = ? AND constraint_name = ? AND constraint_type = 'FOREIGN KEY'";
    }
    void* rows = joss_schema_db_query(sql, params);
    return (rows && joss_arr_len(rows) > 0) ? 1 : 0;
}

/* --- Additional C11 Web/Utility Builtins for Full App Linking --- */
void* joss_end(void* arr) {
    if (!arr) return NULL;
    int64_t n = joss_count(arr);
    if (n <= 0) return NULL;
    return (void*)(intptr_t)joss_arr_get_i64(arr, n - 1);
}

int64_t joss_is_numeric(const char* s) {
    s = unbox_web_cstr(s);
    if (!s || !s[0]) return 0;
    char* endp = NULL;
    (void)strtod(s, &endp);
    return (endp && *endp == '\0') ? 1 : 0;
}

const char* joss_html_escape(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    size_t len = strlen(s);
    size_t cap = len * 6 + 1;
    char* out = (char*)joss_req_alloc(cap);
    if (!out) return s;
    size_t j = 0;
    for (size_t i = 0; i < len; i++) {
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

void* joss_append(void* arr, void* val) {
    if (!arr) arr = joss_arr_new(4);
    joss_arr_push_i64(arr, (int64_t)(intptr_t)val);
    return arr;
}

int64_t joss_len(void* v) {
    if (!v) return 0;
    int64_t raw = joss_unbox_value((int64_t)(uintptr_t)v);
    if (!raw) return 0;
    if (joss_is_arr_ptr(raw) || joss_is_map_ptr(raw)) {
        return joss_count((void*)(uintptr_t)raw);
    }
    const char* s = unbox_web_cstr((const char*)v);
    return s ? (int64_t)strlen(s) : 0;
}

int64_t joss_unlink(const char* path) {
    path = unbox_web_cstr(path);
    if (!path || !path[0]) return 0;
    return (remove(path) == 0) ? 1 : 0;
}

int64_t joss_mkdir(const char* path, int64_t mode, int64_t recursive) {
    (void)mode;
    (void)recursive;
    path = unbox_web_cstr(path);
    if (!path || !path[0]) return 0;
#if defined(_WIN32)
    return CreateDirectoryA(path, NULL) ? 1 : 0;
#else
    return (mkdir(path, 0755) == 0) ? 1 : 0;
#endif
}

int64_t joss_in_array(void* needle, void* haystack) {
    if (!haystack) return 0;
    int64_t n = joss_count(haystack);
    const char* ns = unbox_web_cstr((const char*)needle);
    for (int64_t i = 0; i < n; i++) {
        void* item = (void*)(intptr_t)joss_arr_get_i64(haystack, i);
        if (item == needle) return 1;
        const char* is = unbox_web_cstr((const char*)item);
        if (ns && is && strcmp(ns, is) == 0) return 1;
    }
    return 0;
}

const char* joss_rawurlencode(const char* s);

const char* joss_urlencode(const char* s) {
    return joss_rawurlencode(s);
}

const char* joss_rawurlencode(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    size_t len = strlen(s);
    char* out = (char*)joss_req_alloc(len * 3 + 1);
    if (!out) return s;
    static const char hex[] = "0123456789ABCDEF";
    size_t j = 0;
    for (size_t i = 0; i < len; i++) {
        unsigned char c = (unsigned char)s[i];
        if ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
            c == '-' || c == '_' || c == '.' || c == '~') {
            out[j++] = (char)c;
        } else {
            out[j++] = '%';
            out[j++] = hex[c >> 4];
            out[j++] = hex[c & 0x0F];
        }
    }
    out[j] = '\0';
    return out;
}

const char* joss_gmdate(const char* fmt, int64_t ts) {
    fmt = unbox_web_cstr(fmt);
    time_t t = (ts > 946684800LL && ts < 4102444800LL) ? (time_t)ts : time(NULL);
    struct tm tm_val;
#if defined(_WIN32)
    gmtime_s(&tm_val, &t);
#else
    gmtime_r(&t, &tm_val);
#endif
    char buf[128];
    if (!fmt || !fmt[0] || strcmp(fmt, "Y-m-d H:i:s") == 0) {
        snprintf(buf, sizeof(buf), "%04d-%02d-%02d %02d:%02d:%02d",
            tm_val.tm_year + 1900, tm_val.tm_mon + 1, tm_val.tm_mday,
            tm_val.tm_hour, tm_val.tm_min, tm_val.tm_sec);
        return joss_req_strdup(buf);
    }
    if (strcmp(fmt, "D, d M Y H:i:s") == 0) {
        static const char* days[] = {"Sun","Mon","Tue","Wed","Thu","Fri","Sat"};
        static const char* mons[] = {"Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"};
        snprintf(buf, sizeof(buf), "%s, %02d %s %04d %02d:%02d:%02d",
            days[tm_val.tm_wday % 7], tm_val.tm_mday, mons[tm_val.tm_mon % 12],
            tm_val.tm_year + 1900, tm_val.tm_hour, tm_val.tm_min, tm_val.tm_sec);
        return joss_req_strdup(buf);
    }
    strftime(buf, sizeof(buf), fmt, &tm_val);
    return joss_req_strdup(buf);
}

const char* joss_md5(const char* s) {
    size_t len = 0;
    const uint8_t* ptr = joss_bytes_unwrap(s, &len);
    if (!ptr) {
        s = unbox_web_cstr(s);
        if (!s) s = "";
        ptr = (const uint8_t*)s;
        len = strlen(s);
    }
    uint8_t digest[16];
    joss_prim_md5(ptr, len, digest);
    static const char hex[] = "0123456789abcdef";
    char buf[33];
    for (int i = 0; i < 16; i++) {
        buf[i * 2]     = hex[(digest[i] >> 4) & 0x0F];
        buf[i * 2 + 1] = hex[digest[i] & 0x0F];
    }
    buf[32] = '\0';
    return joss_req_strdup(buf);
}

const char* joss_sha256_base64(const char* s) {
    size_t len = 0;
    const uint8_t* ptr = joss_bytes_unwrap(s, &len);
    if (!ptr) {
        s = unbox_web_cstr(s);
        if (!s) s = "";
        ptr = (const uint8_t*)s;
        len = strlen(s);
    }
    uint8_t digest[32];
    joss_prim_sha256(ptr, len, digest);
    char b64[64];
    joss_prim_base64_encode(digest, sizeof(digest), b64);
    return joss_req_strdup(b64);
}

#if defined(__has_include) && !defined(JOSS_DISABLE_OPENSSL_SIGN)
#if __has_include(<openssl/evp.h>) && __has_include(<openssl/pem.h>)
#define JOSS_HAS_OPENSSL_SIGN 1
#include <openssl/bio.h>
#include <openssl/evp.h>
#include <openssl/pem.h>
#endif
#endif

#if !defined(JOSS_HAS_OPENSSL_SIGN) && defined(_WIN32)
#include <wincrypt.h>
#include <bcrypt.h>
#endif

#define JOSS_RSA_MAX_LIMBS 132

static int joss_rsa_cmp_k(const uint32_t* a, const uint32_t* b, int k) {
    for (int i = k - 1; i >= 0; i--) {
        if (a[i] < b[i]) return -1;
        if (a[i] > b[i]) return 1;
    }
    return 0;
}

static void joss_rsa_sub_k(uint32_t* a, const uint32_t* b, int k) {
    uint64_t borrow = 0;
    for (int i = 0; i < k; i++) {
        uint64_t diff = (uint64_t)a[i] - (uint64_t)b[i] - borrow;
        a[i] = (uint32_t)diff;
        borrow = (diff >> 63) & 1ULL;
    }
}

static uint32_t joss_rsa_mont_n0inv(uint32_t n0) {
    uint32_t x = n0;
    x *= 2U - n0 * x;
    x *= 2U - n0 * x;
    x *= 2U - n0 * x;
    x *= 2U - n0 * x;
    return (uint32_t)(0U - x);
}

static void joss_rsa_mont_mul(uint32_t* out, const uint32_t* a, const uint32_t* b,
                              const uint32_t* n, int k, uint32_t n0inv) {
    uint32_t t[JOSS_RSA_MAX_LIMBS + 2];
    memset(t, 0, (size_t)(k + 2) * sizeof(uint32_t));
    for (int i = 0; i < k; i++) {
        uint64_t carry = 0;
        uint64_t ai = (uint64_t)a[i];
        for (int j = 0; j < k; j++) {
            uint64_t s = (uint64_t)t[j] + ai * (uint64_t)b[j] + carry;
            t[j] = (uint32_t)s;
            carry = s >> 32;
        }
        uint64_t s = (uint64_t)t[k] + carry;
        t[k] = (uint32_t)s;
        t[k + 1] = (uint32_t)(s >> 32);

        uint32_t m = t[0] * n0inv;
        carry = ((uint64_t)t[0] + (uint64_t)m * (uint64_t)n[0]) >> 32;
        for (int j = 1; j < k; j++) {
            uint64_t u = (uint64_t)t[j] + (uint64_t)m * (uint64_t)n[j] + carry;
            t[j - 1] = (uint32_t)u;
            carry = u >> 32;
        }
        uint64_t u = (uint64_t)t[k] + carry;
        t[k - 1] = (uint32_t)u;
        t[k] = t[k + 1] + (uint32_t)(u >> 32);
    }
    if (t[k] != 0 || joss_rsa_cmp_k(t, n, k) >= 0) {
        joss_rsa_sub_k(t, n, k);
    }
    memcpy(out, t, (size_t)k * sizeof(uint32_t));
}

static bool joss_rsa_bytes_to_limbs(const uint8_t* be, size_t len, uint32_t* limbs, int k) {
    memset(limbs, 0, (size_t)k * sizeof(uint32_t));
    while (len > 0 && *be == 0) {
        be++;
        len--;
    }
    if (len > (size_t)k * 4U) return false;
    for (size_t i = 0; i < len; i++) {
        size_t pos = len - 1 - i;
        limbs[pos / 4U] |= ((uint32_t)be[i]) << ((pos % 4U) * 8U);
    }
    return true;
}

static void joss_rsa_limbs_to_bytes(const uint32_t* limbs, int k, uint8_t* out, size_t out_len) {
    memset(out, 0, out_len);
    for (size_t i = 0; i < out_len; i++) {
        size_t pos = out_len - 1 - i;
        int limb_idx = (int)(pos / 4U);
        if (limb_idx < k) {
            out[i] = (uint8_t)((limbs[limb_idx] >> ((pos % 4U) * 8U)) & 0xFFU);
        }
    }
}

static bool joss_asn1_read_tlv(const uint8_t* buf, size_t len, size_t* off,
                               uint8_t* tag, const uint8_t** val, size_t* vlen) {
    if (*off + 2 > len) return false;
    *tag = buf[(*off)++];
    uint8_t lb = buf[(*off)++];
    size_t n = 0;
    if ((lb & 0x80U) == 0) {
        n = lb;
    } else {
        int nb = (int)(lb & 0x7FU);
        if (nb <= 0 || nb > 4 || *off + (size_t)nb > len) return false;
        for (int i = 0; i < nb; i++) {
            n = (n << 8) | buf[(*off)++];
        }
    }
    if (*off + n > len) return false;
    *val = buf + *off;
    *vlen = n;
    *off += n;
    return true;
}

static bool joss_rsa_parse_pem_key(const char* pem,
                                   const uint8_t** out_der_buf,
                                   const uint8_t** out_n, size_t* out_n_len,
                                   const uint8_t** out_d, size_t* out_d_len) {
    static const char B64_STD[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    const char* begin = strstr(pem, "-----BEGIN ");
    if (!begin) return false;
    const char* line_end = strstr(begin, "-----");
    if (!line_end) return false;
    line_end = strstr(line_end + 5, "-----");
    if (!line_end) return false;
    const char* body = line_end + 5;
    const char* end = strstr(body, "-----END ");
    if (!end || end <= body) return false;

    size_t body_len = (size_t)(end - body);
    char* clean = (char*)malloc(body_len + 1);
    if (!clean) return false;
    size_t clen = 0;
    for (size_t i = 0; i < body_len; i++) {
        unsigned char c = (unsigned char)body[i];
        if (c != '\r' && c != '\n' && c != ' ' && c != '\t') {
            clean[clen++] = (char)c;
        }
    }
    clean[clen] = '\0';

    size_t der_cap = (clen * 3U) / 4U + 8U;
    uint8_t* der = (uint8_t*)malloc(der_cap);
    if (!der) {
        free(clean);
        return false;
    }
    size_t der_len = joss_prim_base64_decode_alphabet(clean, clen, B64_STD, der, der_cap);
    free(clean);
    if (der_len < 16) {
        free(der);
        return false;
    }

    size_t off = 0;
    uint8_t tag = 0;
    const uint8_t* seq = NULL;
    size_t seq_len = 0;
    if (!joss_asn1_read_tlv(der, der_len, &off, &tag, &seq, &seq_len) || tag != 0x30U) {
        free(der);
        return false;
    }

    size_t soff = 0;
    const uint8_t* v0 = NULL; size_t v0_len = 0;
    const uint8_t* v1 = NULL; size_t v1_len = 0;
    uint8_t t0 = 0, t1 = 0;
    if (!joss_asn1_read_tlv(seq, seq_len, &soff, &t0, &v0, &v0_len) || t0 != 0x02U ||
        !joss_asn1_read_tlv(seq, seq_len, &soff, &t1, &v1, &v1_len)) {
        free(der);
        return false;
    }

    /* PKCS#8 PrivateKeyInfo: version(INT), algId(SEQ 0x30), privateKey(OCTET STRING 0x04) */
    if (t1 == 0x30U) {
        uint8_t t2 = 0;
        const uint8_t* oct = NULL; size_t oct_len = 0;
        if (!joss_asn1_read_tlv(seq, seq_len, &soff, &t2, &oct, &oct_len) || t2 != 0x04U) {
            free(der);
            return false;
        }
        size_t ioff = 0;
        if (!joss_asn1_read_tlv(oct, oct_len, &ioff, &tag, &seq, &seq_len) || tag != 0x30U) {
            free(der);
            return false;
        }
        soff = 0;
        if (!joss_asn1_read_tlv(seq, seq_len, &soff, &t0, &v0, &v0_len) || t0 != 0x02U ||
            !joss_asn1_read_tlv(seq, seq_len, &soff, &t1, &v1, &v1_len)) {
            free(der);
            return false;
        }
    }

    /* PKCS#1 RSAPrivateKey: version(INT), n(INT), e(INT), d(INT), ... */
    if (t1 != 0x02U) {
        free(der);
        return false;
    }
    uint8_t te = 0, td = 0;
    const uint8_t* ve = NULL; size_t ve_len = 0;
    const uint8_t* vd = NULL; size_t vd_len = 0;
    if (!joss_asn1_read_tlv(seq, seq_len, &soff, &te, &ve, &ve_len) || te != 0x02U ||
        !joss_asn1_read_tlv(seq, seq_len, &soff, &td, &vd, &vd_len) || td != 0x02U) {
        free(der);
        return false;
    }
    while (v1_len > 0 && *v1 == 0) { v1++; v1_len--; }
    while (vd_len > 0 && *vd == 0) { vd++; vd_len--; }
    if (v1_len < 64 || v1_len > 512 || vd_len == 0 || vd_len > v1_len) {
        free(der);
        return false;
    }

    *out_der_buf = der;
    *out_n = v1;
    *out_n_len = v1_len;
    *out_d = vd;
    *out_d_len = vd_len;
    return true;
}

const char* joss_rsa_sign_sha256_pure(const char* data, const char* priv_key) {
    if (!data || !priv_key || !*priv_key) return "";
    const uint8_t* der_buf = NULL;
    const uint8_t* n_be = NULL; size_t n_len = 0;
    const uint8_t* d_be = NULL; size_t d_len = 0;
    if (!joss_rsa_parse_pem_key(priv_key, &der_buf, &n_be, &n_len, &d_be, &d_len)) {
        return "";
    }

    int k = (int)((n_len + 3U) / 4U);
    if (k <= 0 || k > JOSS_RSA_MAX_LIMBS) {
        free((void*)der_buf);
        return "";
    }

    uint32_t mod_n[JOSS_RSA_MAX_LIMBS];
    uint32_t exp_d[JOSS_RSA_MAX_LIMBS];
    if (!joss_rsa_bytes_to_limbs(n_be, n_len, mod_n, k) ||
        !joss_rsa_bytes_to_limbs(d_be, d_len, exp_d, k) ||
        (mod_n[0] & 1U) == 0) {
        free((void*)der_buf);
        return "";
    }
    free((void*)der_buf);

    /* EMSA-PKCS1-v1_5 with SHA-256 DigestInfo (19-byte prefix + 32-byte digest) */
    static const uint8_t SHA256_DER_PREFIX[19] = {
        0x30, 0x31, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01,
        0x65, 0x03, 0x04, 0x02, 0x01, 0x05, 0x00, 0x04, 0x20
    };
    if (n_len < 11U + sizeof(SHA256_DER_PREFIX) + 32U) return "";

    uint8_t digest[32];
    joss_prim_sha256((const uint8_t*)data, strlen(data), digest);

    uint8_t em[512];
    size_t ps_len = n_len - 3U - sizeof(SHA256_DER_PREFIX) - 32U;
    em[0] = 0x00;
    em[1] = 0x01;
    memset(em + 2, 0xFF, ps_len);
    em[2 + ps_len] = 0x00;
    memcpy(em + 3 + ps_len, SHA256_DER_PREFIX, sizeof(SHA256_DER_PREFIX));
    memcpy(em + 3 + ps_len + sizeof(SHA256_DER_PREFIX), digest, 32);

    uint32_t base[JOSS_RSA_MAX_LIMBS];
    if (!joss_rsa_bytes_to_limbs(em, n_len, base, k)) return "";

    uint32_t n0inv = joss_rsa_mont_n0inv(mod_n[0]);

    /* Compute R^2 mod n via repeated doubling */
    uint32_t r2[JOSS_RSA_MAX_LIMBS];
    memset(r2, 0, (size_t)k * sizeof(uint32_t));
    r2[0] = 1U;
    int total_bits = 2 * k * 32;
    for (int bit = 0; bit < total_bits; bit++) {
        uint32_t carry = 0;
        for (int j = 0; j < k; j++) {
            uint32_t next_c = r2[j] >> 31;
            r2[j] = (r2[j] << 1) | carry;
            carry = next_c;
        }
        if (carry || joss_rsa_cmp_k(r2, mod_n, k) >= 0) {
            joss_rsa_sub_k(r2, mod_n, k);
        }
    }

    uint32_t one[JOSS_RSA_MAX_LIMBS];
    memset(one, 0, (size_t)k * sizeof(uint32_t));
    one[0] = 1U;

    uint32_t m_base[JOSS_RSA_MAX_LIMBS];
    uint32_t acc[JOSS_RSA_MAX_LIMBS];
    joss_rsa_mont_mul(m_base, base, r2, mod_n, k, n0inv);
    joss_rsa_mont_mul(acc, one, r2, mod_n, k, n0inv);

    int top_bit = k * 32 - 1;
    while (top_bit > 0 && ((exp_d[top_bit / 32] >> (top_bit % 32)) & 1U) == 0) {
        top_bit--;
    }
    for (int b = top_bit; b >= 0; b--) {
        joss_rsa_mont_mul(acc, acc, acc, mod_n, k, n0inv);
        if ((exp_d[b / 32] >> (b % 32)) & 1U) {
            joss_rsa_mont_mul(acc, acc, m_base, mod_n, k, n0inv);
        }
    }

    uint32_t sig_limbs[JOSS_RSA_MAX_LIMBS];
    joss_rsa_mont_mul(sig_limbs, acc, one, mod_n, k, n0inv);

    uint8_t sig_bytes[512];
    joss_rsa_limbs_to_bytes(sig_limbs, k, sig_bytes, n_len);

    size_t b64_len = ((n_len + 2U) / 3U) * 4U + 1U;
    char* b64_out = (char*)joss_req_alloc(b64_len);
    if (!b64_out) return "";
    joss_prim_base64_encode(sig_bytes, n_len, b64_out);
    return b64_out;
}

const char* joss_rsa_sign_sha256(const char* data, const char* priv_key) {
    data = unbox_web_cstr(data);
    priv_key = unbox_web_cstr(priv_key);
    if (!data || !priv_key || !*priv_key) return "";
#ifdef JOSS_HAS_OPENSSL_SIGN
    BIO* bio = BIO_new_mem_buf(priv_key, -1);
    if (!bio) return "";
    EVP_PKEY* pkey = PEM_read_bio_PrivateKey(bio, NULL, NULL, NULL);
    BIO_free(bio);
    if (!pkey) return "";
    EVP_MD_CTX* ctx = EVP_MD_CTX_new();
    if (!ctx) { EVP_PKEY_free(pkey); return ""; }
    size_t sig_len = 0;
    char* b64_out = NULL;
    if (EVP_DigestSignInit(ctx, NULL, EVP_sha256(), NULL, pkey) == 1 &&
        EVP_DigestSignUpdate(ctx, data, strlen(data)) == 1 &&
        EVP_DigestSignFinal(ctx, NULL, &sig_len) == 1 && sig_len > 0) {
        unsigned char* sig = (unsigned char*)malloc(sig_len);
        if (sig && EVP_DigestSignFinal(ctx, sig, &sig_len) == 1) {
            size_t b64_len = ((sig_len + 2) / 3) * 4 + 1;
            b64_out = (char*)joss_req_alloc(b64_len);
            if (b64_out) {
                int out_n = EVP_EncodeBlock((unsigned char*)b64_out, sig, (int)sig_len);
                b64_out[out_n >= 0 ? out_n : 0] = '\0';
            }
        }
        free(sig);
    }
    EVP_MD_CTX_free(ctx);
    EVP_PKEY_free(pkey);
    return b64_out ? b64_out : "";
#elif defined(_WIN32)
    DWORD der_len = 0;
    if (!CryptStringToBinaryA(priv_key, (DWORD)strlen(priv_key), CRYPT_STRING_BASE64HEADER, NULL, &der_len, NULL, NULL)) {
        return "";
    }
    BYTE* der = (BYTE*)malloc(der_len);
    if (!der) return "";
    if (!CryptStringToBinaryA(priv_key, (DWORD)strlen(priv_key), CRYPT_STRING_BASE64HEADER, der, &der_len, NULL, NULL)) {
        free(der);
        return "";
    }
    BYTE* rsa_blob = NULL;
    DWORD rsa_blob_len = 0;
    CRYPT_PRIVATE_KEY_INFO* pki = NULL;
    DWORD pki_len = 0;
    if (CryptDecodeObjectEx(X509_ASN_ENCODING | PKCS_7_ASN_ENCODING, PKCS_PRIVATE_KEY_INFO,
                            der, der_len, CRYPT_DECODE_ALLOC_FLAG, NULL, &pki, &pki_len) && pki) {
        CryptDecodeObjectEx(X509_ASN_ENCODING | PKCS_7_ASN_ENCODING, PKCS_RSA_PRIVATE_KEY,
                            pki->PrivateKey.pbData, pki->PrivateKey.cbData, CRYPT_DECODE_ALLOC_FLAG, NULL, &rsa_blob, &rsa_blob_len);
        LocalFree(pki);
    } else {
        CryptDecodeObjectEx(X509_ASN_ENCODING | PKCS_7_ASN_ENCODING, PKCS_RSA_PRIVATE_KEY,
                            der, der_len, CRYPT_DECODE_ALLOC_FLAG, NULL, &rsa_blob, &rsa_blob_len);
    }
    free(der);
    if (!rsa_blob) return "";
    BCRYPT_ALG_HANDLE hAlg = NULL, hHashAlg = NULL;
    BCRYPT_KEY_HANDLE hKey = NULL;
    BCRYPT_HASH_HANDLE hHash = NULL;
    char* b64_out = NULL;
    if (BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_RSA_ALGORITHM, NULL, 0) == 0 &&
        BCryptOpenAlgorithmProvider(&hHashAlg, BCRYPT_SHA256_ALGORITHM, NULL, 0) == 0 &&
        BCryptImportKeyPair(hAlg, NULL, LEGACY_RSAPRIVATE_BLOB, &hKey, rsa_blob, rsa_blob_len, 0) == 0 &&
        BCryptCreateHash(hHashAlg, &hHash, NULL, 0, NULL, 0, 0) == 0) {
        BYTE hash[32];
        if (BCryptHashData(hHash, (PUCHAR)data, (ULONG)strlen(data), 0) == 0 &&
            BCryptFinishHash(hHash, hash, sizeof(hash), 0) == 0) {
            BCRYPT_PKCS1_PADDING_INFO pad = { BCRYPT_SHA256_ALGORITHM };
            ULONG sig_len = 0;
            if (BCryptSignHash(hKey, &pad, hash, sizeof(hash), NULL, 0, &sig_len, BCRYPT_PAD_PKCS1) == 0 && sig_len > 0) {
                BYTE* sig = (BYTE*)malloc(sig_len);
                if (sig && BCryptSignHash(hKey, &pad, hash, sizeof(hash), sig, sig_len, &sig_len, BCRYPT_PAD_PKCS1) == 0) {
                    DWORD b64_len = 0;
                    if (CryptBinaryToStringA(sig, sig_len, CRYPT_STRING_BASE64 | CRYPT_STRING_NOCRLF, NULL, &b64_len)) {
                        b64_out = (char*)joss_req_alloc((size_t)b64_len + 1U);
                        if (b64_out && !CryptBinaryToStringA(sig, sig_len, CRYPT_STRING_BASE64 | CRYPT_STRING_NOCRLF, b64_out, &b64_len)) {
                            b64_out = NULL;
                        }
                    }
                }
                free(sig);
            }
        }
    }
    if (hHash) BCryptDestroyHash(hHash);
    if (hKey) BCryptDestroyKey(hKey);
    if (hHashAlg) BCryptCloseAlgorithmProvider(hHashAlg, 0);
    if (hAlg) BCryptCloseAlgorithmProvider(hAlg, 0);
    if (rsa_blob) LocalFree(rsa_blob);
    return b64_out ? b64_out : "";
#else
    return joss_rsa_sign_sha256_pure(data, priv_key);
#endif
}

/* --- Native Sitemap Service (parity with pkg/core/native_seo.go) --- */
typedef struct {
    char* loc;
    char* changefreq;
    char* priority;
} JossSitemapEntry;

static JossSitemapEntry g_sitemap_entries[256];
static int g_sitemap_count = 0;

void fn_Sitemap_add(const char* loc, const char* changefreq, const char* priority) {
    loc = unbox_web_cstr(loc);
    changefreq = unbox_web_cstr(changefreq);
    priority = unbox_web_cstr(priority);
    if (!loc || !loc[0]) return;
    joss_global_lock(JOSS_LOCK_SITEMAP);
    if (g_sitemap_count < 256) {
        g_sitemap_entries[g_sitemap_count].loc = joss_web_strdup(loc);
        g_sitemap_entries[g_sitemap_count].changefreq = joss_web_strdup((changefreq && changefreq[0]) ? changefreq : "weekly");
        g_sitemap_entries[g_sitemap_count].priority = joss_web_strdup((priority && priority[0]) ? priority : "0.8");
        g_sitemap_count++;
    }
    joss_global_unlock(JOSS_LOCK_SITEMAP);
}

typedef void* (*JossSitemapProviderFn)(void);
static void* g_sitemap_providers[32];
static int g_sitemap_provider_count = 0;
static char* g_sitemap_excludes[64];
static int g_sitemap_exclude_count = 0;

static bool joss_sitemap_is_excluded(const char* loc) {
    if (!loc) return false;
    for (int i = 0; i < g_sitemap_exclude_count; i++) {
        const char* pat = g_sitemap_excludes[i];
        if (!pat || !pat[0]) continue;
        size_t plen = strlen(pat);
        if (pat[plen - 1] == '*') {
            if (strncmp(loc, pat, plen - 1) == 0) return true;
        } else if (strcmp(loc, pat) == 0) {
            return true;
        }
    }
    return false;
}

void* fn_Sitemap_provider(void* callback) {
    if (!callback || !joss_is_not_null((int64_t)(uintptr_t)callback)) return NULL;
    joss_global_lock(JOSS_LOCK_SITEMAP);
    if (g_sitemap_provider_count < 32) {
        g_sitemap_providers[g_sitemap_provider_count++] = callback;
    }
    joss_global_unlock(JOSS_LOCK_SITEMAP);
    return NULL;
}

void fn_Sitemap_exclude(void* paths) {
    int64_t pv = joss_unbox_value((int64_t)(uintptr_t)paths);
    joss_global_lock(JOSS_LOCK_SITEMAP);
    if (joss_is_arr_ptr(pv)) {
        int64_t n = joss_arr_len((void*)(uintptr_t)pv);
        for (int64_t i = 0; i < n && g_sitemap_exclude_count < 64; i++) {
            const char* s = joss_strval(joss_arr_get_i64((void*)(uintptr_t)pv, i));
            if (s && s[0]) g_sitemap_excludes[g_sitemap_exclude_count++] = joss_web_strdup(s);
        }
    } else {
        const char* s = unbox_web_cstr((const char*)(uintptr_t)pv);
        if (s && s[0] && g_sitemap_exclude_count < 64) {
            g_sitemap_excludes[g_sitemap_exclude_count++] = joss_web_strdup(s);
        }
    }
    joss_global_unlock(JOSS_LOCK_SITEMAP);
}

const char* fn_Sitemap_xsl(void) {
    return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"
           "<xsl:stylesheet version=\"2.0\" xmlns:xsl=\"http://www.w3.org/1999/XSL/Transform\" "
           "xmlns:sitemap=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n"
           "<xsl:output method=\"html\" version=\"1.0\" encoding=\"UTF-8\" indent=\"yes\"/>\n"
           "<xsl:template match=\"/\"><html lang=\"es\"><head><title>Sitemap</title></head>"
           "<body><h1>Sitemap XML</h1><ul><xsl:for-each select=\"sitemap:urlset/sitemap:url\">"
           "<li><a href=\"{sitemap:loc}\"><xsl:value-of select=\"sitemap:loc\"/></a></li>"
           "</xsl:for-each></ul></body></html></xsl:template></xsl:stylesheet>\n";
}

const char* fn_Sitemap_generate(const char* base_url) {
    base_url = unbox_web_cstr(base_url);
    if (!base_url || !base_url[0]) {
        base_url = joss_env("APP_URL", "http://localhost");
    }
    if (!base_url || !base_url[0]) base_url = "http://localhost";
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n");
    joss_sb_append(sb, "<?xml-stylesheet type=\"text/xsl\" href=\"/sitemap.xsl\"?>\n");
    joss_sb_append(sb, "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n");
    joss_global_lock(JOSS_LOCK_SITEMAP);
    if (g_sitemap_count == 0 && g_sitemap_provider_count == 0) {
        static const char* defaults[] = {"/", "/lenguaje", "/docs", "/blog", "/creador", "/catalogs", "/pub"};
        for (int i = 0; i < 7; i++) {
            if (joss_sitemap_is_excluded(defaults[i])) continue;
            joss_sb_append(sb, "  <url><loc>");
            joss_sb_append(sb, base_url);
            joss_sb_append(sb, defaults[i]);
            joss_sb_append(sb, "</loc><changefreq>weekly</changefreq><priority>0.8</priority></url>\n");
        }
    } else {
        for (int i = 0; i < g_sitemap_count; i++) {
            if (joss_sitemap_is_excluded(g_sitemap_entries[i].loc)) continue;
            joss_sb_append(sb, "  <url><loc>");
            if (strncmp(g_sitemap_entries[i].loc, "http", 4) != 0) joss_sb_append(sb, base_url);
            joss_sb_append(sb, g_sitemap_entries[i].loc);
            joss_sb_append(sb, "</loc><changefreq>");
            joss_sb_append(sb, g_sitemap_entries[i].changefreq);
            joss_sb_append(sb, "</changefreq><priority>");
            joss_sb_append(sb, g_sitemap_entries[i].priority);
            joss_sb_append(sb, "</priority></url>\n");
        }
        for (int p = 0; p < g_sitemap_provider_count; p++) {
            void* fn_raw = joss_closure_prepare_call(g_sitemap_providers[p]);
            if (!fn_raw) continue;
            void* items = ((JossSitemapProviderFn)fn_raw)();
            int64_t iv = joss_unbox_value((int64_t)(uintptr_t)items);
            if (!joss_is_arr_ptr(iv)) continue;
            int64_t n = joss_arr_len((void*)(uintptr_t)iv);
            for (int64_t i = 0; i < n; i++) {
                int64_t el = joss_arr_get_i64((void*)(uintptr_t)iv, i);
                const char* loc = NULL;
                const char* cf = "weekly";
                const char* prio = "0.8";
                if (joss_is_map_ptr(el)) {
                    loc = joss_map_get_str((void*)(uintptr_t)el, "loc");
                    const char* mcf = joss_map_get_str((void*)(uintptr_t)el, "changefreq");
                    const char* mpr = joss_map_get_str((void*)(uintptr_t)el, "priority");
                    if (mcf && mcf[0]) cf = mcf;
                    if (mpr && mpr[0]) prio = mpr;
                } else {
                    loc = joss_strval(el);
                }
                if (!loc || !loc[0] || joss_sitemap_is_excluded(loc)) continue;
                joss_sb_append(sb, "  <url><loc>");
                if (strncmp(loc, "http", 4) != 0) joss_sb_append(sb, base_url);
                joss_sb_append(sb, loc);
                joss_sb_append(sb, "</loc><changefreq>");
                joss_sb_append(sb, cf);
                joss_sb_append(sb, "</changefreq><priority>");
                joss_sb_append(sb, prio);
                joss_sb_append(sb, "</priority></url>\n");
            }
        }
    }
    joss_global_unlock(JOSS_LOCK_SITEMAP);
    joss_sb_append(sb, "</urlset>\n");
    char* out = joss_sb_to_req_string(sb);
    joss_sb_free(sb);
    return out;
}

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
    FILE* f = fopen(path_out, "wb");
    if (!f) return false;
    if (len > 0 && fwrite(data, 1, len, f) != len) {
        fclose(f);
        remove(path_out);
        return false;
    }
    fclose(f);
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
                joss_map_set_str(fentry, "name", joss_web_strdup(file_name));
                joss_map_set_str(fentry, "path", joss_web_strdup(tmp_path));
                joss_map_set_str(fentry, "tmp_name", joss_web_strdup(tmp_path));
                joss_map_set_str(fentry, "type", joss_web_strdup(part_ct));
                joss_map_set_i64(fentry, "size", (int64_t)pdata_len);
                joss_map_set_i64(fentry, "error", 0);
                if (memchr(pbody, '\0', pdata_len) != NULL) {
                    joss_map_set_i64(fentry, "content", (int64_t)(uintptr_t)joss_bytes_new(pbody, pdata_len));
                } else {
                    char* sc = (char*)malloc(pdata_len + 1);
                    if (sc) {
                        memcpy(sc, pbody, pdata_len);
                        sc[pdata_len] = '\0';
                        joss_map_set_str(fentry, "content", sc);
                    }
                }
                joss_map_set_i64(files_map, field_name, (int64_t)(uintptr_t)fentry);
            } else {
                char* val_copy = (char*)malloc(pdata_len + 1);
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

#define JOSS_RATE_BUCKETS 1024
typedef struct {
    char    ip[192];
    int64_t window_start;
    int64_t count;
    bool    used;
} JossRateEntry;

static JossRateEntry g_rate_entries[JOSS_RATE_BUCKETS];

int64_t joss___joss_rate_limit_check(const char* ip, int64_t max_req, int64_t window_sec) {
    ip = unbox_web_cstr(ip);
    if (!ip || !ip[0]) ip = "127.0.0.1";
    if (max_req <= 0) max_req = 60;
    if (window_sec <= 0) window_sec = 60;
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
    JossStringBuilder* sb = joss_sb_new();
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
    joss_sb_free(sb);
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
    JossStringBuilder* sb = joss_sb_new();
    for (int i = 0; i < g_joss_custom_mw_count; i++) {
        if (i > 0) joss_sb_append(sb, ", ");
        if (g_joss_custom_mw[i].name) joss_sb_append(sb, g_joss_custom_mw[i].name);
    }
    char* res = joss_sb_to_req_string(sb);
    joss_sb_free(sb);
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
                joss_map_set_str(sess_map, "user_email", joss_web_strdup(joss_map_get_str(claims, "email")));
            }
            if (joss_array_key_exists("name", claims)) {
                joss_map_set_str(sess_map, "user_name", joss_web_strdup(joss_map_get_str(claims, "name")));
            }
            if (joss_array_key_exists("role", claims)) {
                joss_map_set_str(sess_map, "user_role", joss_web_strdup(joss_map_get_str(claims, "role")));
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
    return joss_web_strdup(s ? s : "");
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
    FILE* f = fopen(path, "rb");
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
        fclose(f);
        g_joss_session_mem_store = joss_map_new();
        snprintf(g_joss_session_loaded_path, sizeof(g_joss_session_loaded_path), "%s", path);
        joss_req_arena_resume(_prev_arena);
        return true;
    }
    char* buf = (char*)malloc((size_t)sz + 1);
    if (!buf) {
        fclose(f);
        joss_set_session_err("out of memory reading session file");
        joss_req_arena_resume(_prev_arena);
        return false;
    }
    size_t rd = fread(buf, 1, (size_t)sz, f);
    fclose(f);
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
    JossStringBuilder* sb = joss_sb_new();
    json_encode_value(sb, (int64_t)(uintptr_t)(g_joss_session_mem_store ? g_joss_session_mem_store : joss_map_new()));
    uint64_t seq = atomic_fetch_add_explicit(&s_sess_tmp_seq, 1ULL, memory_order_relaxed);
    char tmp_path[560];
    snprintf(tmp_path, sizeof(tmp_path), "%s.%llu.%lu.tmp", path, (unsigned long long)seq, (unsigned long)time(NULL));
    FILE* f = fopen(tmp_path, "wb");
    if (!f) {
        joss_sb_free(sb);
        joss_set_session_err("failed to open temp session file for write");
        return false;
    }
    size_t wr = fwrite(sb->data, 1, (size_t)sb->len, f);
    int flush_rc = fflush(f);
    int close_rc = fclose(f);
    if (wr != (size_t)sb->len || flush_rc != 0 || close_rc != 0) {
        joss_sb_free(sb);
        remove(tmp_path);
        joss_set_session_err("failed writing session file");
        return false;
    }
    joss_sb_free(sb);
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
    JossStringBuilder* sb = joss_sb_new();
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
    joss_sb_free(sb);
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

#include "joss_redis_rt.inc"

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
        JossStringBuilder* jsb = joss_sb_new();
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
        joss_sb_free(jsb);
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
        FILE* ef = fopen(tmp_eml, "wb");
        if (!ef) return 0;
        fprintf(ef, "From: <%s>\r\nTo: <%s>\r\nSubject: %s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s\r\n",
                from, to, subject ? subject : "", body ? body : "");
        fclose(ef);
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

    JossStringBuilder* msg = joss_sb_new();
    joss_sb_append(msg, "From: "); joss_sb_append(msg, from); joss_sb_append(msg, "\r\n");
    joss_sb_append(msg, "To: "); joss_sb_append(msg, to); joss_sb_append(msg, "\r\n");
    joss_sb_append(msg, "Subject: "); joss_sb_append(msg, subject ? subject : ""); joss_sb_append(msg, "\r\n\r\n");
    joss_sb_append(msg, body ? body : "");
    joss_sb_append(msg, "\r\n.\r\n");
    bool ok = joss_redis_send_all(fd, (const char*)msg->data, (size_t)msg->len);
    if (!ok) {
        fprintf(stderr, "[SmtpClient] send DATA body failed\n");
    }
    joss_sb_free(msg);
    (void)joss_redis_read_line(fd, line, sizeof(line));
    joss_redis_send_all(fd, "QUIT\r\n", 6);
    joss_redis_close_fd(fd);
    return ok ? 1 : 0;
}

int64_t fn_Mail_send(const char* to, const char* subject, const char* body) {
    return fn_SmtpClient_send(to, subject, body);
}

#include "joss_zip_rt.inc"
