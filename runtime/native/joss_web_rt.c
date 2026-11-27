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
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
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
    joss_global_lock(JOSS_LOCK_GENERAL);
    if (g_joss_env_map != NULL) {
        joss_global_unlock(JOSS_LOCK_GENERAL);
        return;
    }
    bool _prev_arena = joss_req_arena_pause();
    g_joss_env_map = joss_map_new();
    const char* candidates[] = {"env.joss", ".env"};
    for (int i = 0; i < 2; i++) {
        JOSS_AUTO_FILE FILE* f = fopen(candidates[i], "rb");
        if (!f) continue;
        fseek(f, 0, SEEK_END);
        long sz = ftell(f);
        fseek(f, 0, SEEK_SET);
        if (sz <= 0) {
            continue;
        }
        /* buf intentionally persists for process lifetime: g_joss_env_map stores slices into it */
        char* buf = (char*)malloc((size_t)sz + 1);
        if (!buf) {
            continue;
        }
        size_t rd = fread(buf, 1, (size_t)sz, f);
        buf[rd] = '\0';
        joss_parse_env_buffer(buf);
        joss_req_arena_resume(_prev_arena);
        joss_global_unlock(JOSS_LOCK_GENERAL);
        return;
    }
    if (g_joss_embedded_asset_resolver != NULL) {
        const char* enc = g_joss_embedded_asset_resolver("__joss_embedded_env_enc");
        if (enc && enc[0]) {
            char* plain = joss_decrypt_embedded_env(enc);
            if (plain) {
                joss_parse_env_buffer(plain);
                joss_req_arena_resume(_prev_arena);
                joss_global_unlock(JOSS_LOCK_GENERAL);
                return;
            }
        }
    }
    joss_req_arena_resume(_prev_arena);
    joss_global_unlock(JOSS_LOCK_GENERAL);
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
        JOSS_AUTO_FILE FILE* f = fopen(path, "rb");
        if (f) {
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
        JOSS_AUTO_FILE FILE* f = fopen(path, "rb");
        if (f) {
            fseek(f, 0, SEEK_END);
            long sz = ftell(f);
            fseek(f, 0, SEEK_SET);
            if (sz > 0) {
                char* buf = (char*)joss_req_alloc((size_t)sz + 1);
                if (buf) {
                    size_t rd = fread(buf, 1, (size_t)sz, f);
                    buf[rd] = '\0';
                    if (memchr(buf, '\0', rd) != NULL) {
                        void* boxed = joss_bytes_new(buf, rd);
                        return (const char*)boxed;
                    }
                    return buf;
                }
            }
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
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 ");
    joss_sb_append_i64(sb, status);
    joss_sb_append(sb, status == 200 ? " OK\r\n" : " Status\r\n");
    joss_sb_append(sb, "Content-Type: application/json; charset=utf-8\r\nConnection: close\r\n\r\n");
    json_encode_value(sb, data);
    char* out = joss_sb_to_req_string(sb);
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
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 ");
    joss_sb_append_i64(sb, status);
    joss_sb_append(sb, " OK\r\nContent-Type: text/html; charset=utf-8\r\nConnection: close\r\n\r\n");
    joss_sb_append(sb, body);
    char* out = joss_sb_to_req_string(sb);
    return out;
}

const char* joss_redirect(const char* url, int64_t status) {
    url = unbox_web_cstr(url);
    if (!url || !*url) url = "/";
    status = joss_intval(status);
    if (status < 300 || status > 399) status = 302;
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 ");
    joss_sb_append_i64(sb, status);
    joss_sb_append(sb, " Found\r\nLocation: ");
    joss_sb_append(sb, url);
    joss_sb_append(sb, "\r\nContent-Length: 0\r\nConnection: close\r\n\r\n");
    char* out = joss_sb_to_req_string(sb);
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
    JOSS_AUTO_FILE FILE* fp = fopen(e->file_path, "rb");
    if (!fp) return;
    unsigned char hdr[4];
    if (fread(hdr, 1, 4, fp) != 4) {
        return;
    }
    uint32_t count = (uint32_t)hdr[0] | ((uint32_t)hdr[1] << 8) | ((uint32_t)hdr[2] << 16) | ((uint32_t)hdr[3] << 24);
    if (count > 100000u) {
        return;
    }
    char** next_chunks = (char**)calloc(count > 0 ? count : 1, sizeof(char*));
    if (!next_chunks) {
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
 * Minimal Micro-Kernel TCP Client Primitives (for pure-Joss Redis/SMTP/HTTP)
 * ========================================================================= */
#if defined(JOSS_HAS_RUST_RT)
extern int64_t joss_rs_tcp_connect(const char* host, int64_t port, int64_t timeout_ms);
extern int64_t joss_rs_tcp_write(int64_t fd, const uint8_t* data, size_t len);
extern int64_t joss_rs_tcp_read(int64_t fd, uint8_t* buf, size_t max_bytes);
extern int64_t joss_rs_tcp_read_line(int64_t fd, uint8_t* buf, size_t cap);
extern int64_t joss_rs_tcp_close(int64_t fd);
#endif

int64_t joss___joss_tcp_connect(const char* host_raw, int64_t port, int64_t timeout_ms) {
    const char* host = unbox_web_cstr(host_raw);
    if (!host || !*host || port <= 0 || port > 65535) return -1;
    if (timeout_ms <= 0) timeout_ms = 3000;
#if defined(JOSS_HAS_RUST_RT)
    return joss_rs_tcp_connect(host, port, timeout_ms);
#else
#if defined(_WIN32)
    static bool s_tcp_wsa = false;
    if (!s_tcp_wsa) {
        WSADATA wd;
        if (WSAStartup(MAKEWORD(2, 2), &wd) == 0) s_tcp_wsa = true;
    }
#endif
    char port_str[16];
    snprintf(port_str, sizeof(port_str), "%lld", (long long)port);
    struct addrinfo hints;
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;
    struct addrinfo* res = NULL;
    if (getaddrinfo(host, port_str, &hints, &res) != 0 || !res) return -1;

    int64_t out_fd = -1;
    for (struct addrinfo* rp = res; rp != NULL; rp = rp->ai_next) {
#if defined(_WIN32)
        SOCKET s = socket(rp->ai_family, rp->ai_socktype, rp->ai_protocol);
        if (s == INVALID_SOCKET) continue;
        DWORD tv = (DWORD)timeout_ms;
        setsockopt(s, SOL_SOCKET, SO_RCVTIMEO, (const char*)&tv, sizeof(tv));
        setsockopt(s, SOL_SOCKET, SO_SNDTIMEO, (const char*)&tv, sizeof(tv));
        if (connect(s, rp->ai_addr, (int)rp->ai_addrlen) == 0) {
            out_fd = (int64_t)(uintptr_t)s;
            break;
        }
        closesocket(s);
#else
        int s = socket(rp->ai_family, rp->ai_socktype, rp->ai_protocol);
        if (s < 0) continue;
        struct timeval tv;
        tv.tv_sec = timeout_ms / 1000;
        tv.tv_usec = (timeout_ms % 1000) * 1000;
        setsockopt(s, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
        setsockopt(s, SOL_SOCKET, SO_SNDTIMEO, &tv, sizeof(tv));
        if (connect(s, rp->ai_addr, rp->ai_addrlen) == 0) {
            out_fd = (int64_t)s;
            break;
        }
        close(s);
#endif
    }
    freeaddrinfo(res);
    return out_fd;
#endif
}

int64_t joss___joss_tcp_write(int64_t fd, const char* data_raw) {
    if (fd < 0) return -1;
    const char* data = unbox_web_cstr(data_raw);
    if (!data) return 0;
    size_t len = strlen(data);
#if defined(JOSS_HAS_RUST_RT)
    return joss_rs_tcp_write(fd, (const uint8_t*)data, len);
#else
    size_t sent = 0;
    while (sent < len) {
#if defined(_WIN32)
        int n = send((SOCKET)(uintptr_t)fd, data + sent, (int)(len - sent), 0);
#else
        ssize_t n = send((int)fd, data + sent, len - sent, 0);
#endif
        if (n <= 0) return -1;
        sent += (size_t)n;
    }
    return (int64_t)sent;
#endif
}

const char* joss___joss_tcp_read(int64_t fd, int64_t max_bytes) {
    if (fd < 0 || max_bytes <= 0) return "";
    if (max_bytes > (64LL << 20)) max_bytes = (64LL << 20);
    char* buf = (char*)joss_req_alloc((size_t)max_bytes + 1U);
    if (!buf) return "";
#if defined(JOSS_HAS_RUST_RT)
    int64_t got = joss_rs_tcp_read(fd, (uint8_t*)buf, (size_t)max_bytes);
    buf[got > 0 ? (size_t)got : 0U] = '\0';
    return buf;
#else
    size_t got = 0;
    while (got < (size_t)max_bytes) {
#if defined(_WIN32)
        int n = recv((SOCKET)(uintptr_t)fd, buf + got, (int)((size_t)max_bytes - got), 0);
#else
        ssize_t n = recv((int)fd, buf + got, (size_t)max_bytes - got, 0);
#endif
        if (n <= 0) break;
        got += (size_t)n;
    }
    buf[got] = '\0';
    return buf;
#endif
}

const char* joss___joss_tcp_read_line(int64_t fd) {
    if (fd < 0) return "";
#if defined(JOSS_HAS_RUST_RT)
    char stack_buf[4096];
    int64_t n = joss_rs_tcp_read_line(fd, (uint8_t*)stack_buf, sizeof(stack_buf) - 1U);
    if (n <= 0) return "";
    stack_buf[(size_t)n] = '\0';
    return joss_req_strdup(stack_buf);
#else
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    for (size_t i = 0; i < (1U << 20); i++) {
        char c = 0;
#if defined(_WIN32)
        int n = recv((SOCKET)(uintptr_t)fd, &c, 1, 0);
#else
        ssize_t n = recv((int)fd, &c, 1, 0);
#endif
        if (n <= 0) break;
        if (c == '\n') break;
        if (c != '\r') joss_buf_append_byte(sb, (uint8_t)c);
    }
    return joss_sb_to_req_string(sb);
#endif
}

int64_t joss___joss_tcp_close(int64_t fd) {
    if (fd < 0) return 0;
#if defined(JOSS_HAS_RUST_RT)
    return joss_rs_tcp_close(fd);
#elif defined(_WIN32)
    closesocket((SOCKET)(uintptr_t)fd);
    return 1;
#else
    close((int)fd);
    return 1;
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
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
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
                char* p_b64 = (char*)joss_req_alloc(plen * 2 + 16);
                if (p_b64) {
                    joss_prim_base64_encode_alphabet((const uint8_t*)payload, plen, B64U, false, p_b64);
                    size_t mlen = strlen(h_b64) + 1 + strlen(p_b64);
                    char* msg = (char*)joss_req_alloc(mlen + 1);
                    if (msg) {
                        snprintf(msg, mlen + 1, "%s.%s", h_b64, p_b64);
                        uint8_t mac[32];
                        joss_prim_hmac_sha256((const uint8_t*)sec, strlen(sec), (const uint8_t*)msg, mlen, mac);
                        char s_b64[64];
                        joss_prim_base64_encode_alphabet(mac, 32, B64U, false, s_b64);
                        char* full = (char*)joss_req_alloc(mlen + 1 + strlen(s_b64) + 1);
                        if (full) {
                            snprintf(full, mlen + 1 + strlen(s_b64) + 1, "%s.%s", msg, s_b64);
                            tok = full;
                        }
                    }
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
    JOSS_AUTO_FILE FILE* f = fopen(path, "wb");
    if (!f) return 0;
    size_t n = content ? strlen(content) : 0;
    size_t wr = n > 0 ? fwrite(content, 1, n, f) : 0;
    return (wr == n) ? 1 : 0;
}

const char* joss_json_encode(int64_t data) {
    JOSS_AUTO_SB JossStringBuilder* sb = joss_sb_new();
    json_encode_value(sb, data);
    char* out = joss_sb_to_req_string(sb);
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
 * Modular Host Services (Micro-Kernel Composition)
 * Each .inc unit is independently audited by tests/architecture and ready
 * to be migrated 1-to-1 to Joss stdlib modules or Rust staticlib crates.
 * ========================================================================= */
#include "joss_services_rt.inc"
#include "joss_ws_rt.inc"
#include "joss_cron_rt.inc"
#if defined(JOSS_NO_DB)
void fn_Model_registerClass(const char* name, const char* meta_json) {
    (void)name;
    (void)meta_json;
}
#endif

#include "joss_builtins_rt.inc"
#include "joss_session_rt.inc"
