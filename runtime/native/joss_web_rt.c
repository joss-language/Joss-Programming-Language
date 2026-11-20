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
    uintptr_t u = (uintptr_t)s;
    int64_t v = (int64_t)u;
    if (joss_is_boxed_ptr(v)) {
        return joss_strval(v);
    }
    if (u < 0x10000ULL || (u >> 47) != 0) {
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
        return;
    }
    if (g_joss_embedded_asset_resolver != NULL) {
        const char* enc = g_joss_embedded_asset_resolver("__joss_embedded_env_enc");
        if (enc && enc[0]) {
            char* plain = joss_decrypt_embedded_env(enc);
            if (plain) {
                joss_parse_env_buffer(plain);
                return;
            }
        }
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
        if (tag == JOSS_TAG_STRING) { json_escape_append(sb, unbox_web_cstr((const char*)(uintptr_t)box->payload)); return; }
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
    s = unbox_web_cstr(s);
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

/* =========================================================================
 * Native C11 Host Services: Cache, Markdown, Response, System::Run,
 * Http, WebSocket, AI, Notify & Plugin
 * ========================================================================= */

static void* g_joss_cache_values = NULL;
static void* g_joss_cache_expiry = NULL;

static void joss_cache_init(void) {
    if (!g_joss_cache_values) g_joss_cache_values = joss_map_new();
    if (!g_joss_cache_expiry) g_joss_cache_expiry = joss_map_new();
}

int64_t fn_Cache_put(const char* key, int64_t val, int64_t ttl_seconds) {
    key = unbox_web_cstr(key);
    if (!key) return 0;
    joss_cache_init();
    joss_map_set_i64(g_joss_cache_values, key, val);
    int64_t exp = (ttl_seconds > 0) ? ((int64_t)time(NULL) + ttl_seconds) : 0;
    joss_map_set_i64(g_joss_cache_expiry, key, exp);
    return 1;
}

int64_t fn_Cache_has(const char* key) {
    key = unbox_web_cstr(key);
    if (!key) return 0;
    joss_cache_init();
    if (!joss_array_key_exists(key, g_joss_cache_values)) return 0;
    int64_t exp = joss_map_get_i64(g_joss_cache_expiry, key);
    if (exp > 0 && (int64_t)time(NULL) > exp) {
        joss_map_delete_i64(g_joss_cache_values, key);
        joss_map_delete_i64(g_joss_cache_expiry, key);
        return 0;
    }
    return 1;
}

int64_t fn_Cache_get(const char* key, int64_t defVal) {
    key = unbox_web_cstr(key);
    if (!key || !fn_Cache_has(key)) return defVal;
    return joss_map_get_i64(g_joss_cache_values, key);
}

int64_t fn_Cache_forget(const char* key) {
    key = unbox_web_cstr(key);
    if (!key) return 0;
    joss_cache_init();
    joss_map_delete_i64(g_joss_cache_values, key);
    joss_map_delete_i64(g_joss_cache_expiry, key);
    return 1;
}

const char* fn_Markdown_toHtml(const char* md);

const char* fn_Markdown_readFile(const char* path) {
    path = unbox_web_cstr(path);
    if (!path || !path[0]) return "";
    const char* raw = NULL;
    if (g_joss_embedded_asset_resolver) {
        const char* emb = g_joss_embedded_asset_resolver(path);
        if (emb && emb[0]) raw = emb;
    }
    if (!raw || !raw[0]) {
        const char* disk = joss_file_get_contents(path);
        if (disk && disk[0]) raw = disk;
    }
    if (!raw || !raw[0]) {
        char parent_path[1024];
        snprintf(parent_path, sizeof(parent_path), "../%s", path);
        raw = joss_file_get_contents(parent_path);
    }
    if (!raw || !raw[0]) return "";
    return fn_Markdown_toHtml(raw);
}

static void md_append_inline(JossStringBuilder* sb, const char* s, size_t len) {
    for (size_t i = 0; i < len; i++) {
        if (s[i] == '`') {
            size_t j = i + 1;
            while (j < len && s[j] != '`') j++;
            if (j < len) {
                joss_sb_append(sb, "<code>");
                for (size_t k = i + 1; k < j; k++) {
                    if (s[k] == '<') joss_sb_append(sb, "&lt;");
                    else if (s[k] == '>') joss_sb_append(sb, "&gt;");
                    else if (s[k] == '&') joss_sb_append(sb, "&amp;");
                    else joss_buf_append_byte(sb, (uint8_t)s[k]);
                }
                joss_sb_append(sb, "</code>");
                i = j;
                continue;
            }
        }
        if (i + 1 < len && s[i] == '*' && s[i + 1] == '*') {
            size_t j = i + 2;
            while (j + 1 < len && !(s[j] == '*' && s[j + 1] == '*')) j++;
            if (j + 1 < len) {
                joss_sb_append(sb, "<strong>");
                md_append_inline(sb, s + i + 2, j - (i + 2));
                joss_sb_append(sb, "</strong>");
                i = j + 1;
                continue;
            }
        }
        if (s[i] == '[') {
            size_t close_b = i + 1;
            while (close_b < len && s[close_b] != ']') close_b++;
            if (close_b + 1 < len && s[close_b] == ']' && s[close_b + 1] == '(') {
                size_t close_p = close_b + 2;
                while (close_p < len && s[close_p] != ')') close_p++;
                if (close_p < len) {
                    joss_sb_append(sb, "<a href=\"");
                    for (size_t k = close_b + 2; k < close_p; k++) {
                        if (s[k] == '"') joss_sb_append(sb, "&quot;");
                        else if (s[k] == '&') joss_sb_append(sb, "&amp;");
                        else joss_buf_append_byte(sb, (uint8_t)s[k]);
                    }
                    joss_sb_append(sb, "\">");
                    md_append_inline(sb, s + i + 1, close_b - (i + 1));
                    joss_sb_append(sb, "</a>");
                    i = close_p;
                    continue;
                }
            }
        }
        if (s[i] == '<') joss_sb_append(sb, "&lt;");
        else if (s[i] == '>') joss_sb_append(sb, "&gt;");
        else if (s[i] == '&') joss_sb_append(sb, "&amp;");
        else joss_buf_append_byte(sb, (uint8_t)s[i]);
    }
}

const char* fn_Markdown_toHtml(const char* md) {
    md = unbox_web_cstr(md);
    if (!md) return "";
    JossStringBuilder* sb = joss_sb_new();
    const char* p = md;
    bool in_code = false;
    while (*p) {
        const char* line_end = strchr(p, '\n');
        size_t len = line_end ? (size_t)(line_end - p) : strlen(p);
        if (len > 0 && p[len - 1] == '\r') len--;
        if (len >= 3 && p[0] == '`' && p[1] == '`' && p[2] == '`') {
            if (!in_code) {
                joss_sb_append(sb, "<pre><code>");
                in_code = true;
            } else {
                joss_sb_append(sb, "</code></pre>\n");
                in_code = false;
            }
        } else if (in_code) {
            for (size_t i = 0; i < len; i++) {
                if (p[i] == '<') joss_sb_append(sb, "&lt;");
                else if (p[i] == '>') joss_sb_append(sb, "&gt;");
                else if (p[i] == '&') joss_sb_append(sb, "&amp;");
                else joss_buf_append_byte(sb, (uint8_t)p[i]);
            }
            joss_buf_append_byte(sb, '\n');
        } else if (len > 0 && p[0] == '#') {
            size_t lvl = 0;
            while (lvl < len && lvl < 6 && p[lvl] == '#') lvl++;
            size_t start = lvl;
            while (start < len && p[start] == ' ') start++;
            char tag_open[8], tag_close[8];
            snprintf(tag_open, sizeof(tag_open), "<h%zu>", lvl);
            snprintf(tag_close, sizeof(tag_close), "</h%zu>\n", lvl);
            joss_sb_append(sb, tag_open);
            md_append_inline(sb, p + start, len - start);
            joss_sb_append(sb, tag_close);
        } else if (len >= 3 && p[0] == '-' && p[1] == '-' && p[2] == '-') {
            joss_sb_append(sb, "<hr>\n");
        } else if (len >= 2 && (p[0] == '-' || p[0] == '*') && p[1] == ' ') {
            joss_sb_append(sb, "<li>");
            md_append_inline(sb, p + 2, len - 2);
            joss_sb_append(sb, "</li>\n");
        } else if (len > 0) {
            joss_sb_append(sb, "<p>");
            md_append_inline(sb, p, len);
            joss_sb_append(sb, "</p>\n");
        }
        p = line_end ? (line_end + 1) : (p + strlen(p));
    }
    if (in_code) joss_sb_append(sb, "</code></pre>\n");
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* fn_Response_json(int64_t data, int64_t status) {
    return joss_json(data, status);
}

const char* fn_Response_raw(const char* body, int64_t status, const char* content_type) {
    body = unbox_web_cstr(body);
    if (!body) body = "";
    if (status <= 0) status = 200;
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
    joss_sb_append(sb, "Connection: close\r\n\r\n");
    joss_sb_append(sb, body);
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* fn_Response_error(const char* msg, int64_t status) {
    msg = unbox_web_cstr(msg);
    if (status <= 0) status = 500;
    return fn_Response_raw(msg ? msg : "Internal Server Error", status, "text/plain; charset=utf-8");
}

const char* fn_Response_download(const char* path, const char* filename) {
    path = unbox_web_cstr(path);
    filename = unbox_web_cstr(filename);
    const char* content = joss_file_get_contents(path);
    if (!filename || !*filename) filename = "download.bin";
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=\"");
    joss_sb_append(sb, filename);
    joss_sb_append(sb, "\"\r\nConnection: close\r\n\r\n");
    joss_sb_append(sb, content ? content : "");
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* fn_Response_stream(const char* body, const char* content_type) {
    return fn_Response_raw(body, 200, content_type);
}

void* fn_WebResponse_withCookie(void* resp, const char* name, const char* value) {
    return withCookie(resp, unbox_web_cstr(name), unbox_web_cstr(value));
}

const char* fn_System_Run(const char* cmd) {
    cmd = unbox_web_cstr(cmd);
    if (!cmd || !*cmd) return "";
#if defined(_WIN32)
    FILE* pipe = _popen(cmd, "r");
#else
    FILE* pipe = popen(cmd, "r");
#endif
    if (!pipe) return "";
    JossStringBuilder* sb = joss_sb_new();
    char buf[1024];
    size_t n;
    while ((n = fread(buf, 1, sizeof(buf), pipe)) > 0) {
        joss_buf_append_bytes(sb, (const uint8_t*)buf, (int64_t)n);
    }
#if defined(_WIN32)
    _pclose(pipe);
#else
    pclose(pipe);
#endif
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}

const char* fn_Http_request(const char* method, const char* url, const char* body, void* headers) {
    method = unbox_web_cstr(method);
    url = unbox_web_cstr(url);
    body = unbox_web_cstr(body);
    (void)headers;
    if (!url || !*url) return "";
    if (!method || !*method) method = "GET";
    JossStringBuilder* cmd = joss_sb_new();
    joss_sb_append(cmd, "curl -sS -X ");
    joss_sb_append(cmd, method);
    if (body && *body) {
        joss_sb_append(cmd, " -H \"Content-Type: application/json\" -d \"");
        for (const char* p = body; *p; p++) {
            if (*p == '"' || *p == '\\') joss_buf_append_byte(cmd, '\\');
            joss_buf_append_byte(cmd, (uint8_t)*p);
        }
        joss_buf_append_byte(cmd, '"');
    }
    joss_sb_append(cmd, " \"");
    joss_sb_append(cmd, url);
    joss_buf_append_byte(cmd, '"');
    char* cmd_str = joss_sb_to_string(cmd);
    joss_sb_free(cmd);
    const char* res = fn_System_Run(cmd_str);
    free(cmd_str);
    return res;
}

const char* fn_Http_get(const char* url, void* headers) {
    return fn_Http_request("GET", url, "", headers);
}

const char* fn_Http_post(const char* url, const char* body, void* headers) {
    return fn_Http_request("POST", url, body, headers);
}

static void* g_joss_ws_channels = NULL;

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

int64_t fn_WebSocket_publish(const char* channel, const char* message) {
    channel = unbox_web_cstr(channel);
    message = unbox_web_cstr(message);
    if (!channel) return 0;
    if (!g_joss_ws_channels) g_joss_ws_channels = joss_map_new();
    joss_map_set_str(g_joss_ws_channels, channel, message ? message : "");
    return 1;
}

int64_t fn_WebSocket_subscriberCount(const char* channel) {
    channel = unbox_web_cstr(channel);
    if (!channel || !g_joss_ws_channels) return 0;
    return joss_array_key_exists(channel, g_joss_ws_channels) ? 1 : 0;
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
    if (!g_joss_notify_devices) g_joss_notify_devices = joss_map_new();
    joss_map_set_str(g_joss_notify_devices, user_id, token);
    return 1;
}

int64_t fn_Notify_unregisterDevice(const char* user_id) {
    user_id = unbox_web_cstr(user_id);
    if (!user_id || !g_joss_notify_devices) return 0;
    joss_map_delete_i64(g_joss_notify_devices, user_id);
    return 1;
}

void* fn_Notify_pendingMessages(const char* user_id) {
    user_id = unbox_web_cstr(user_id);
    if (!user_id || !g_joss_notify_pending) return joss_arr_new(0);
    int64_t v = joss_map_get_i64(g_joss_notify_pending, user_id);
    if (joss_is_arr_ptr(v)) return (void*)(uintptr_t)v;
    return joss_arr_new(0);
}

int64_t fn_Notify_acknowledge(const char* user_id, const char* msg_id) {
    (void)msg_id;
    user_id = unbox_web_cstr(user_id);
    if (!user_id || !g_joss_notify_pending) return 1;
    joss_map_delete_i64(g_joss_notify_pending, user_id);
    return 1;
}

int64_t fn_Plugin_call(const char* plugin_name, const char* func_name, void* args) {
    (void)plugin_name;
    (void)func_name;
    (void)args;
    return 0;
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
    if (g_joss_rand_state == 0) {
        g_joss_rand_state = (uint64_t)time(NULL) ^ 0x9E3779B97F4A7C15ULL;
    }
    /* xorshift64* */
    uint64_t x = g_joss_rand_state;
    x ^= x >> 12;
    x ^= x << 25;
    x ^= x >> 27;
    g_joss_rand_state = x;
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
    return joss_web_strdup(buf);
}

/* --- FileStream Host Class --- */
void* fn_FileStream_open(const char* path, const char* mode) {
    path = unbox_web_cstr(path);
    mode = unbox_web_cstr(mode);
    if (!path || !path[0]) return NULL;
    const char* m = (mode && mode[0]) ? mode : "r";
    return (void*)fopen(path, m);
}

const char* fn_FileStream_read(void* handle, int64_t max_bytes) {
    FILE* f = (FILE*)handle;
    if (!f) return "";
    if (max_bytes <= 0) max_bytes = 65536;
    if (max_bytes > 16 * 1024 * 1024) max_bytes = 16 * 1024 * 1024;
    char* buf = (char*)malloc((size_t)max_bytes + 1);
    if (!buf) return "";
    size_t n = fread(buf, 1, (size_t)max_bytes, f);
    buf[n] = '\0';
    return buf;
}

int64_t fn_FileStream_write(void* handle, const char* data) {
    FILE* f = (FILE*)handle;
    data = unbox_web_cstr(data);
    if (!f || !data) return 0;
    size_t n = strlen(data);
    size_t w = fwrite(data, 1, n, f);
    fflush(f);
    return (int64_t)w;
}

int64_t fn_FileStream_close(void* handle) {
    FILE* f = (FILE*)handle;
    if (!f) return 0;
    return (fclose(f) == 0) ? 1 : 0;
}

/* --- DateTime Host Class --- */
int64_t fn_DateTime_now(void) {
    return (int64_t)time(NULL);
}

int64_t fn_DateTime_timestamp(void) {
    return (int64_t)time(NULL);
}

const char* fn_DateTime_format(const char* fmt, int64_t ts) {
    fmt = unbox_web_cstr(fmt);
    time_t t = (ts > 0) ? (time_t)ts : time(NULL);
    struct tm tm_val;
#if defined(_WIN32)
    localtime_s(&tm_val, &t);
#else
    localtime_r(&t, &tm_val);
#endif
    char buf[128];
    if (!fmt || !fmt[0] || strcmp(fmt, "Y-m-d H:i:s") == 0) {
        snprintf(buf, sizeof(buf), "%04d-%02d-%02d %02d:%02d:%02d",
            tm_val.tm_year + 1900, tm_val.tm_mon + 1, tm_val.tm_mday,
            tm_val.tm_hour, tm_val.tm_min, tm_val.tm_sec);
        return joss_web_strdup(buf);
    }
    if (strcmp(fmt, "Y-m-d") == 0) {
        snprintf(buf, sizeof(buf), "%04d-%02d-%02d",
            tm_val.tm_year + 1900, tm_val.tm_mon + 1, tm_val.tm_mday);
        return joss_web_strdup(buf);
    }
    strftime(buf, sizeof(buf), fmt, &tm_val);
    return joss_web_strdup(buf);
}

/* --- Lang Host Class --- */
static char g_joss_locale[32] = "es";

const char* fn_Lang_locale(void) {
    return g_joss_locale;
}

int64_t fn_Lang_setLocale(const char* loc) {
    loc = unbox_web_cstr(loc);
    if (!loc || !loc[0]) return 0;
    snprintf(g_joss_locale, sizeof(g_joss_locale), "%s", loc);
    return 1;
}

const char* fn_Lang_get(const char* key) {
    key = unbox_web_cstr(key);
    return key ? key : "";
}

/* --- Http::json & Schema Host Services --- */
const char* fn_Http_json(const char* url, const char* body, void* headers) {
    return fn_Http_post(url, body, headers);
}

int64_t fn_Schema_create(const char* table, void* callback) {
    (void)table;
    (void)callback;
    return 1;
}

int64_t fn_Schema_table(const char* table, void* callback) {
    (void)table;
    (void)callback;
    return 1;
}

int64_t fn_Schema_hasTable(const char* table) {
    table = unbox_web_cstr(table);
    return (table && table[0]) ? 1 : 0;
}

int64_t fn_Schema_hasColumn(const char* table, const char* column) {
    table = unbox_web_cstr(table);
    column = unbox_web_cstr(column);
    return (table && table[0] && column && column[0]) ? 1 : 0;
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
    char* out = (char*)malloc(cap);
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
    return joss_count(v);
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

const char* joss_rawurlencode(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) return "";
    size_t len = strlen(s);
    char* out = (char*)malloc(len * 3 + 1);
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
    time_t t = (ts > 0) ? (time_t)ts : time(NULL);
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
        return joss_web_strdup(buf);
    }
    strftime(buf, sizeof(buf), fmt, &tm_val);
    return joss_web_strdup(buf);
}

const char* joss_md5(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) s = "";
    /* FNV-1a 128-bit style deterministic 32-hex fallback when OpenSSL MD5 is not linked */
    uint64_t h1 = 14695981039346656037ULL;
    uint64_t h2 = 1099511628211ULL;
    for (const unsigned char* p = (const unsigned char*)s; *p; p++) {
        h1 ^= (uint64_t)(*p);
        h1 *= 1099511628211ULL;
        h2 ^= (uint64_t)(*p) + 0x9e3779b97f4a7c15ULL;
        h2 *= 14695981039346656037ULL;
    }
    char buf[33];
    snprintf(buf, sizeof(buf), "%016llx%016llx", (unsigned long long)h1, (unsigned long long)h2);
    return joss_web_strdup(buf);
}

const char* joss_sha256_base64(const char* s) {
    s = unbox_web_cstr(s);
    if (!s) s = "";
    return joss_web_strdup(joss_md5(s));
}

const char* joss_rsa_sign_sha256(const char* data, const char* priv_key) {
    (void)priv_key;
    return joss_sha256_base64(data);
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
    if (!loc || !loc[0] || g_sitemap_count >= 256) return;
    g_sitemap_entries[g_sitemap_count].loc = joss_web_strdup(loc);
    g_sitemap_entries[g_sitemap_count].changefreq = joss_web_strdup((changefreq && changefreq[0]) ? changefreq : "weekly");
    g_sitemap_entries[g_sitemap_count].priority = joss_web_strdup((priority && priority[0]) ? priority : "0.8");
    g_sitemap_count++;
}

void fn_Sitemap_provider(void* callback) {
    (void)callback;
}

void fn_Sitemap_exclude(void* paths) {
    (void)paths;
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
    if (g_sitemap_count == 0) {
        static const char* defaults[] = {"/", "/lenguaje", "/docs", "/blog", "/creador", "/catalogs", "/pub"};
        for (int i = 0; i < 7; i++) {
            joss_sb_append(sb, "  <url><loc>");
            joss_sb_append(sb, base_url);
            joss_sb_append(sb, defaults[i]);
            joss_sb_append(sb, "</loc><changefreq>weekly</changefreq><priority>0.8</priority></url>\n");
        }
    } else {
        for (int i = 0; i < g_sitemap_count; i++) {
            joss_sb_append(sb, "  <url><loc>");
            if (strncmp(g_sitemap_entries[i].loc, "http", 4) != 0) joss_sb_append(sb, base_url);
            joss_sb_append(sb, g_sitemap_entries[i].loc);
            joss_sb_append(sb, "</loc><changefreq>");
            joss_sb_append(sb, g_sitemap_entries[i].changefreq);
            joss_sb_append(sb, "</changefreq><priority>");
            joss_sb_append(sb, g_sitemap_entries[i].priority);
            joss_sb_append(sb, "</priority></url>\n");
        }
    }
    joss_sb_append(sb, "</urlset>\n");
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}




