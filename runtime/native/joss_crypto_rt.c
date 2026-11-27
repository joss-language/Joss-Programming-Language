//go:build ignore
#include "joss_rt.h"
#include "joss_crypto_primitives.inc"

#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

static const char* crypto_cstr(const char* s) {
    if (!s) return "";
    size_t blen = 0;
    const uint8_t* bdata = joss_bytes_unwrap(s, &blen);
    if (bdata) return (const char*)bdata;
    uintptr_t u = (uintptr_t)s;
    int64_t v = (int64_t)u;
    if (joss_is_boxed_ptr(v) || u < JOSS_MIN_PTR_ADDR || (u >> JOSS_PTR_VA_BITS) != 0) {
        const char* r = joss_strval(v);
        return r ? r : "";
    }
    return s;
}

#if defined(__has_include) && !defined(JOSS_FORCE_BUILTIN_CRYPTO)
#if __has_include(<openssl/evp.h>) && __has_include(<openssl/rand.h>) && __has_include(<openssl/hmac.h>)
#include <openssl/crypto.h>
#include <openssl/evp.h>
#include <openssl/hmac.h>
#include <openssl/rand.h>
#define JOSS_HAS_OPENSSL 1
#endif
#endif

static void bytes_to_hex(const uint8_t* in, size_t len, char* out) {
    static const char hex[] = "0123456789abcdef";
    for (size_t i = 0; i < len; i++) {
        out[i * 2]     = hex[(in[i] >> 4) & 0x0F];
        out[i * 2 + 1] = hex[in[i] & 0x0F];
    }
    out[len * 2] = '\0';
}

#if !defined(JOSS_HAS_RUST_RT)
static int hex_val(char c) {
    if (c >= '0' && c <= '9') return c - '0';
    if (c >= 'a' && c <= 'f') return c - 'a' + 10;
    if (c >= 'A' && c <= 'F') return c - 'A' + 10;
    return -1;
}

static bool hex_to_bytes(const char* hex, uint8_t* out, size_t out_len) {
    if (!hex || strlen(hex) != out_len * 2) return false;
    for (size_t i = 0; i < out_len; i++) {
        int hi = hex_val(hex[i * 2]);
        int lo = hex_val(hex[i * 2 + 1]);
        if (hi < 0 || lo < 0) return false;
        out[i] = (uint8_t)((hi << 4) | lo);
    }
    return true;
}

static char* b64url_encode(const uint8_t* in, size_t len) {
    static const char tbl[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
    size_t out_cap = ((len + 2) / 3) * 4 + 1;
    char* out = (char*)joss_req_alloc(out_cap);
    if (!out) return NULL;
    size_t j = 0;
    for (size_t i = 0; i < len; i += 3) {
        uint32_t v = ((uint32_t)in[i]) << 16;
        if (i + 1 < len) v |= ((uint32_t)in[i + 1]) << 8;
        if (i + 2 < len) v |= (uint32_t)in[i + 2];
        out[j++] = tbl[(v >> 18) & 0x3F];
        out[j++] = tbl[(v >> 12) & 0x3F];
        if (i + 1 < len) out[j++] = tbl[(v >> 6) & 0x3F];
        if (i + 2 < len) out[j++] = tbl[v & 0x3F];
    }
    out[j] = '\0';
    return out;
}

static size_t base32_decode(const char* in, uint8_t* out, size_t max_out) {
    if (!in) return 0;
    uint32_t buffer = 0;
    int bits_left = 0;
    size_t out_len = 0;
    for (size_t i = 0; in[i] != '\0'; i++) {
        unsigned char c = (unsigned char)in[i];
        if (c == ' ' || c == '-' || c == '=' || c == '\t' || c == '\r' || c == '\n') continue;
        int val = -1;
        if (c >= 'A' && c <= 'Z') val = c - 'A';
        else if (c >= 'a' && c <= 'z') val = c - 'a';
        else if (c >= '2' && c <= '7') val = (c - '2') + 26;
        if (val < 0) continue;
        buffer = (buffer << 5) | (uint32_t)val;
        bits_left += 5;
        if (bits_left >= 8) {
            bits_left -= 8;
            if (out_len < max_out) {
                out[out_len++] = (uint8_t)((buffer >> bits_left) & 0xFF);
            }
        }
    }
    return out_len;
}
#endif /* !JOSS_HAS_RUST_RT */

#if defined(JOSS_HAS_RUST_RT)

extern void    joss_rs_sha256(const uint8_t* data, size_t len, uint8_t out[32]);
extern int64_t joss_rs_csprng_bytes(uint8_t* out, size_t len);
extern void    joss_rs_pbkdf2_sha256(const uint8_t* pass, size_t pass_len, const uint8_t* salt, size_t salt_len, uint32_t iterations, uint8_t out[32]);
extern int64_t joss_rs_password_verify_pbkdf2(const char* hash, const char* password);
extern ptrdiff_t joss_rs_jwt_sign(const char* payload, const char* secret, char* out_buf, size_t out_cap);
extern int64_t joss_rs_jwt_verify(const char* token, const char* secret);
extern int64_t joss_rs_totp_generate(const char* secret, int64_t unix_time, char* out7);
extern int64_t joss_rs_totp_verify(const char* secret, const char* code, int64_t unix_time);

const char* joss_sha256(const char* s) {
    size_t slen = 0;
    const uint8_t* ptr = joss_bytes_unwrap(s, &slen);
    if (!ptr) {
        s = crypto_cstr(s);
        ptr = (const uint8_t*)s;
        slen = strlen(s);
    }
    uint8_t digest[32];
    joss_rs_sha256(ptr, slen, digest);
    char* out = (char*)joss_req_alloc(65);
    if (!out) return "";
    bytes_to_hex(digest, 32, out);
    return out;
}

const char* joss_random_hex(int64_t num_bytes) {
    if (num_bytes <= 0) num_bytes = 16;
    if (num_bytes > 256) num_bytes = 256;
    uint8_t buf[256];
    if (!joss_rs_csprng_bytes(buf, (size_t)num_bytes)) {
        joss_panic("JOSS-CRYPTO-001: Rust CSPRNG failed");
    }
    char* out = (char*)joss_req_alloc((size_t)num_bytes * 2 + 1);
    if (!out) return "";
    bytes_to_hex(buf, (size_t)num_bytes, out);
    return out;
}

const char* joss_password_hash(const char* password) {
    password = crypto_cstr(password);
    const uint32_t iterations = 100000U;
    uint8_t salt[16];
    uint8_t dk[32];
    if (!joss_rs_csprng_bytes(salt, sizeof(salt))) {
        joss_panic("JOSS-CRYPTO-001: Rust CSPRNG salt generation failed");
    }
    joss_rs_pbkdf2_sha256((const uint8_t*)password, strlen(password), salt, sizeof(salt), iterations, dk);
    char salt_hex[33], dk_hex[65];
    bytes_to_hex(salt, sizeof(salt), salt_hex);
    bytes_to_hex(dk, sizeof(dk), dk_hex);
    char* out = (char*)joss_req_alloc(140);
    if (!out) return "";
    snprintf(out, 140, "$pbkdf2-sha256$%u$%s$%s", iterations, salt_hex, dk_hex);
    return out;
}

int64_t joss_password_verify(const char* hash, const char* password) {
    hash = crypto_cstr(hash);
    password = crypto_cstr(password);
    if (!hash[0]) return 0;
    if (hash[0] == '$' && hash[1] == '2' &&
        (hash[2] == 'a' || hash[2] == 'b' || hash[2] == 'y') &&
        hash[3] == '$') {
        return joss_prim_bcrypt_verify(hash, password);
    }
    return joss_rs_password_verify_pbkdf2(hash, password);
}

const char* joss_jwt_sign(const char* payload_json, const char* secret) {
    payload_json = crypto_cstr(payload_json);
    if (!payload_json[0]) payload_json = "{}";
    secret = crypto_cstr(secret);
    size_t cap = strlen(payload_json) * 2 + 256;
    char* out = (char*)joss_req_alloc(cap);
    if (!out) return "";
    if (joss_rs_jwt_sign(payload_json, secret, out, cap) < 0) return "";
    return out;
}

int64_t joss_jwt_verify(const char* token, const char* secret) {
    token = crypto_cstr(token);
    secret = crypto_cstr(secret);
    return joss_rs_jwt_verify(token, secret);
}

const char* joss_totp_generate(const char* base32_secret, int64_t unix_time) {
    base32_secret = crypto_cstr(base32_secret);
    char* out = (char*)joss_req_alloc(7);
    if (!out) return "000000";
    if (!joss_rs_totp_generate(base32_secret, unix_time, out)) return "000000";
    return out;
}

int64_t joss_totp_verify(const char* base32_secret, const char* code, int64_t unix_time) {
    base32_secret = crypto_cstr(base32_secret);
    code = crypto_cstr(code);
    return joss_rs_totp_verify(base32_secret, code, unix_time);
}

#elif defined(JOSS_HAS_OPENSSL)

const char* joss_sha256(const char* s) {
    size_t slen = 0;
    const uint8_t* ptr = joss_bytes_unwrap(s, &slen);
    if (!ptr) {
        s = crypto_cstr(s);
        ptr = (const uint8_t*)s;
        slen = strlen(s);
    }
    uint8_t digest[32];
    unsigned int dlen = 0;
    EVP_MD_CTX* ctx = EVP_MD_CTX_new();
    if (!ctx) return "";
    EVP_DigestInit_ex(ctx, EVP_sha256(), NULL);
    EVP_DigestUpdate(ctx, ptr, slen);
    EVP_DigestFinal_ex(ctx, digest, &dlen);
    EVP_MD_CTX_free(ctx);
    char* out = (char*)joss_req_alloc(65);
    if (!out) return "";
    bytes_to_hex(digest, 32, out);
    return out;
}

const char* joss_random_hex(int64_t num_bytes) {
    if (num_bytes <= 0) num_bytes = 16;
    if (num_bytes > 256) num_bytes = 256;
    uint8_t buf[256];
    if (RAND_bytes(buf, (int)num_bytes) != 1) {
        joss_panic("JOSS-CRYPTO-001: CSPRNG RAND_bytes failed");
    }
    char* out = (char*)joss_req_alloc((size_t)num_bytes * 2 + 1);
    if (!out) return "";
    bytes_to_hex(buf, (size_t)num_bytes, out);
    return out;
}

const char* joss_password_hash(const char* password) {
    password = crypto_cstr(password);
    const int iterations = 100000;
    uint8_t salt[16];
    uint8_t dk[32];
    if (RAND_bytes(salt, sizeof(salt)) != 1) {
        joss_panic("JOSS-CRYPTO-001: CSPRNG salt generation failed");
    }
    if (PKCS5_PBKDF2_HMAC(password, (int)strlen(password),
                          salt, sizeof(salt),
                          iterations, EVP_sha256(),
                          sizeof(dk), dk) != 1) {
        joss_panic("JOSS-CRYPTO-002: PBKDF2-HMAC-SHA256 failed");
    }
    char salt_hex[33];
    char dk_hex[65];
    bytes_to_hex(salt, sizeof(salt), salt_hex);
    bytes_to_hex(dk, sizeof(dk), dk_hex);

    char* out = (char*)joss_req_alloc(140);
    if (!out) return "";
    snprintf(out, 140, "$pbkdf2-sha256$%d$%s$%s", iterations, salt_hex, dk_hex);
    return out;
}

int64_t joss_password_verify(const char* hash, const char* password) {
    hash = crypto_cstr(hash);
    password = crypto_cstr(password);
    if (!hash[0]) return 0;
    if (hash[0] == '$' && hash[1] == '2' &&
        (hash[2] == 'a' || hash[2] == 'b' || hash[2] == 'y') &&
        hash[3] == '$') {
        return joss_prim_bcrypt_verify(hash, password);
    }
    const char* prefix = "$pbkdf2-sha256$";
    size_t plen = strlen(prefix);
    if (strncmp(hash, prefix, plen) != 0) return 0;

    const char* p = hash + plen;
    const char* d1 = strchr(p, '$');
    if (!d1) return 0;
    int iterations = atoi(p);
    if (iterations < 1000 || iterations > 5000000) return 0;

    const char* salt_start = d1 + 1;
    const char* d2 = strchr(salt_start, '$');
    if (!d2 || (size_t)(d2 - salt_start) != 32) return 0;
    char salt_hex[33];
    memcpy(salt_hex, salt_start, 32);
    salt_hex[32] = '\0';

    const char* dk_hex = d2 + 1;
    uint8_t salt[16];
    uint8_t expected_dk[32];
    uint8_t actual_dk[32];
    if (!hex_to_bytes(salt_hex, salt, 16)) return 0;
    if (!hex_to_bytes(dk_hex, expected_dk, 32)) return 0;

    if (PKCS5_PBKDF2_HMAC(password, (int)strlen(password),
                          salt, sizeof(salt),
                          iterations, EVP_sha256(),
                          sizeof(actual_dk), actual_dk) != 1) {
        return 0;
    }
    return CRYPTO_memcmp(actual_dk, expected_dk, sizeof(actual_dk)) == 0 ? 1 : 0;
}

const char* joss_jwt_sign(const char* payload_json, const char* secret) {
    payload_json = crypto_cstr(payload_json);
    if (!payload_json[0]) payload_json = "{}";
    secret = crypto_cstr(secret);
    const char* header_json = "{\"alg\":\"HS256\",\"typ\":\"JWT\"}";
    char* h_b64 = b64url_encode((const uint8_t*)header_json, strlen(header_json));
    char* p_b64 = b64url_encode((const uint8_t*)payload_json, strlen(payload_json));
    if (!h_b64 || !p_b64) return "";

    size_t h_len = strlen(h_b64);
    size_t p_len = strlen(p_b64);
    size_t msg_len = h_len + 1 + p_len;
    char* msg = (char*)joss_req_alloc(msg_len + 1);
    if (!msg) return "";
    memcpy(msg, h_b64, h_len);
    msg[h_len] = '.';
    memcpy(msg + h_len + 1, p_b64, p_len);
    msg[msg_len] = '\0';

    uint8_t mac[32];
    unsigned int mac_len = 0;
    if (!HMAC(EVP_sha256(), secret, (int)strlen(secret),
              (const uint8_t*)msg, msg_len, mac, &mac_len) || mac_len != 32) {
        return "";
    }
    char* s_b64 = b64url_encode(mac, 32);
    if (!s_b64) return "";

    size_t s_len = strlen(s_b64);
    char* token = (char*)joss_req_alloc(msg_len + 1 + s_len + 1);
    if (!token) return "";
    memcpy(token, msg, msg_len);
    token[msg_len] = '.';
    memcpy(token + msg_len + 1, s_b64, s_len);
    token[msg_len + 1 + s_len] = '\0';
    return token;
}

int64_t joss_jwt_verify(const char* token, const char* secret) {
    token = crypto_cstr(token);
    secret = crypto_cstr(secret);
    if (!token[0]) return 0;
    const char* dot1 = strchr(token, '.');
    if (!dot1) return 0;
    const char* dot2 = strchr(dot1 + 1, '.');
    if (!dot2 || strchr(dot2 + 1, '.') != NULL) return 0;

    size_t msg_len = (size_t)(dot2 - token);
    const char* given_sig = dot2 + 1;
    size_t given_len = strlen(given_sig);
    if (given_len == 0) return 0;

    uint8_t mac[32];
    unsigned int mac_len = 0;
    if (!HMAC(EVP_sha256(), secret, (int)strlen(secret),
              (const uint8_t*)token, msg_len, mac, &mac_len) || mac_len != 32) {
        return 0;
    }
    char* expected_sig = b64url_encode(mac, 32);
    if (!expected_sig) return 0;
    size_t exp_len = strlen(expected_sig);
    if (exp_len != given_len) return 0;
    return CRYPTO_memcmp(expected_sig, given_sig, exp_len) == 0 ? 1 : 0;
}

static void totp_for_step(const uint8_t* key, size_t key_len, uint64_t step, char out[7]) {
    uint8_t msg[8];
    for (int i = 7; i >= 0; i--) {
        msg[i] = (uint8_t)(step & 0xFF);
        step >>= 8;
    }
    uint8_t digest[20];
    unsigned int dlen = 0;
    if (!HMAC(EVP_sha1(), key, (int)key_len, msg, sizeof(msg), digest, &dlen) || dlen != 20) {
        memcpy(out, "000000", 7);
        return;
    }
    int offset = digest[19] & 0x0F;
    uint32_t bin_code = (((uint32_t)(digest[offset] & 0x7F)) << 24) |
                        (((uint32_t)digest[offset + 1]) << 16) |
                        (((uint32_t)digest[offset + 2]) << 8) |
                        ((uint32_t)digest[offset + 3]);
    uint32_t code = bin_code % 1000000U;
    snprintf(out, 7, "%06u", code);
}

const char* joss_totp_generate(const char* base32_secret, int64_t unix_time) {
    if (!base32_secret || !*base32_secret) return "000000";
    if (unix_time <= 0) unix_time = (int64_t)time(NULL);
    uint8_t key[128];
    size_t key_len = base32_decode(base32_secret, key, sizeof(key));
    if (key_len == 0) {
        key_len = strlen(base32_secret);
        if (key_len > sizeof(key)) key_len = sizeof(key);
        memcpy(key, base32_secret, key_len);
    }
    char* out = (char*)joss_req_alloc(7);
    if (!out) return "000000";
    totp_for_step(key, key_len, (uint64_t)(unix_time / 30), out);
    return out;
}

int64_t joss_totp_verify(const char* base32_secret, const char* code, int64_t unix_time) {
    if (!base32_secret || !*base32_secret || !code) return 0;
    while (*code == ' ' || *code == '\t') code++;
    char clean[7];
    size_t clen = 0;
    while (code[clen] != '\0' && code[clen] != ' ' && code[clen] != '\t' && clen < 6) {
        if (code[clen] < '0' || code[clen] > '9') return 0;
        clean[clen] = code[clen];
        clen++;
    }
    if (clen != 6 || (code[clen] != '\0' && code[clen] != ' ' && code[clen] != '\t')) return 0;
    clean[6] = '\0';

    if (unix_time <= 0) unix_time = (int64_t)time(NULL);
    uint8_t key[128];
    size_t key_len = base32_decode(base32_secret, key, sizeof(key));
    if (key_len == 0) {
        key_len = strlen(base32_secret);
        if (key_len > sizeof(key)) key_len = sizeof(key);
        memcpy(key, base32_secret, key_len);
    }
    int64_t base_step = unix_time / 30;
    for (int64_t delta = -1; delta <= 1; delta++) {
        if (base_step + delta < 0) continue;
        char expected[7];
        totp_for_step(key, key_len, (uint64_t)(base_step + delta), expected);
        if (CRYPTO_memcmp(expected, clean, 6) == 0) {
            return 1;
        }
    }
    return 0;
}

#else

#if defined(_WIN32)
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <windows.h>
#include <bcrypt.h>
#endif

static int joss_ct_memcmp(const void* a, const void* b, size_t n) {
    const uint8_t* x = (const uint8_t*)a;
    const uint8_t* y = (const uint8_t*)b;
    uint8_t diff = 0;
    for (size_t i = 0; i < n; i++) diff |= (uint8_t)(x[i] ^ y[i]);
    return diff;
}

static bool joss_os_random_bytes(uint8_t* buf, size_t len) {
#if defined(_WIN32)
    return BCryptGenRandom(NULL, buf, (ULONG)len, BCRYPT_USE_SYSTEM_PREFERRED_RNG) == 0;
#else
    JOSS_AUTO_FILE FILE* f = fopen("/dev/urandom", "rb");
    if (!f) return false;
    size_t n = fread(buf, 1, len, f);
    return n == len;
#endif
}

/* --- Built-in FIPS 180-4 SHA-256 (provided by joss_crypto_primitives.inc) --- */

/* --- Built-in RFC 3174 SHA-1 + HMAC-SHA1 (for TOTP RFC 6238) --- */
static uint32_t rotl32(uint32_t x, uint32_t n) { return (x << n) | (x >> (32 - n)); }

static void sha1_transform(uint32_t state[5], const uint8_t block[64]) {
    uint32_t w[80];
    for (int i = 0; i < 16; i++) {
        w[i] = ((uint32_t)block[i*4] << 24) | ((uint32_t)block[i*4+1] << 16) |
               ((uint32_t)block[i*4+2] << 8) | (uint32_t)block[i*4+3];
    }
    for (int i = 16; i < 80; i++) w[i] = rotl32(w[i-3] ^ w[i-8] ^ w[i-14] ^ w[i-16], 1);
    uint32_t a=state[0], b=state[1], c=state[2], d=state[3], e=state[4];
    for (int i = 0; i < 80; i++) {
        uint32_t f, k;
        if (i < 20)      { f = (b & c) | ((~b) & d); k = 0x5A827999; }
        else if (i < 40) { f = b ^ c ^ d;            k = 0x6ED9EBA1; }
        else if (i < 60) { f = (b & c) | (b & d) | (c & d); k = 0x8F1BBCDC; }
        else             { f = b ^ c ^ d;            k = 0xCA62C1D6; }
        uint32_t tmp = rotl32(a, 5) + f + e + k + w[i];
        e = d; d = c; c = rotl32(b, 30); b = a; a = tmp;
    }
    state[0]+=a; state[1]+=b; state[2]+=c; state[3]+=d; state[4]+=e;
}

static void sha1_raw(const uint8_t* data, size_t len, uint8_t out[20]) {
    uint32_t state[5] = {0x67452301, 0xEFCDAB89, 0x98BADCFE, 0x10325476, 0xC3D2E1F0};
    size_t i = 0;
    while (i + 64 <= len) { sha1_transform(state, data + i); i += 64; }
    uint8_t tail[128];
    memset(tail, 0, sizeof(tail));
    size_t rem = len - i;
    if (rem > 0) memcpy(tail, data + i, rem);
    tail[rem] = 0x80;
    size_t total_blocks = (rem < 56) ? 1 : 2;
    uint64_t bit_len = (uint64_t)len * 8ULL;
    for (int j = 0; j < 8; j++) {
        tail[total_blocks * 64 - 1 - j] = (uint8_t)(bit_len >> (j * 8));
    }
    sha1_transform(state, tail);
    if (total_blocks == 2) sha1_transform(state, tail + 64);
    for (int j = 0; j < 5; j++) {
        out[j*4]   = (uint8_t)(state[j] >> 24);
        out[j*4+1] = (uint8_t)(state[j] >> 16);
        out[j*4+2] = (uint8_t)(state[j] >> 8);
        out[j*4+3] = (uint8_t)(state[j]);
    }
}

static void hmac_sha1_raw(const uint8_t* key, size_t key_len, const uint8_t* msg, size_t msg_len, uint8_t out[20]) {
    uint8_t k_pad[64];
    memset(k_pad, 0, sizeof(k_pad));
    if (key_len > 64) sha1_raw(key, key_len, k_pad);
    else if (key_len > 0) memcpy(k_pad, key, key_len);
    uint8_t* inner = (uint8_t*)malloc(64 + msg_len);
    if (!inner) { memset(out, 0, 20); return; }
    for (int i = 0; i < 64; i++) inner[i] = k_pad[i] ^ 0x36;
    if (msg_len > 0) memcpy(inner + 64, msg, msg_len);
    uint8_t ih[20];
    sha1_raw(inner, 64 + msg_len, ih);
    free(inner);

    uint8_t outer[64 + 20];
    for (int i = 0; i < 64; i++) outer[i] = k_pad[i] ^ 0x5C;
    memcpy(outer + 64, ih, 20);
    sha1_raw(outer, sizeof(outer), out);
}

static void totp_for_step_builtin(const uint8_t* key, size_t key_len, uint64_t step, char out[7]) {
    uint8_t msg[8];
    for (int i = 7; i >= 0; i--) { msg[i] = (uint8_t)(step & 0xFF); step >>= 8; }
    uint8_t digest[20];
    hmac_sha1_raw(key, key_len, msg, sizeof(msg), digest);
    int offset = digest[19] & 0x0F;
    uint32_t bin_code = (((uint32_t)(digest[offset] & 0x7F)) << 24) |
                        (((uint32_t)digest[offset + 1]) << 16) |
                        (((uint32_t)digest[offset + 2]) << 8) |
                        ((uint32_t)digest[offset + 3]);
    snprintf(out, 7, "%06u", bin_code % 1000000U);
}

const char* joss_sha256(const char* s) {
    size_t slen = 0;
    const uint8_t* ptr = joss_bytes_unwrap(s, &slen);
    if (!ptr) {
        s = crypto_cstr(s);
        ptr = (const uint8_t*)s;
        slen = strlen(s);
    }
    uint8_t digest[32];
    joss_prim_sha256(ptr, slen, digest);
    char* out = (char*)joss_req_alloc(65);
    if (!out) return "";
    bytes_to_hex(digest, 32, out);
    return out;
}

const char* joss_random_hex(int64_t num_bytes) {
    if (num_bytes <= 0) num_bytes = 16;
    if (num_bytes > 256) num_bytes = 256;
    uint8_t buf[256];
    if (!joss_os_random_bytes(buf, (size_t)num_bytes)) {
        joss_panic("JOSS-CRYPTO-001: OS CSPRNG failed");
    }
    char* out = (char*)joss_req_alloc((size_t)num_bytes * 2 + 1);
    if (!out) return "";
    bytes_to_hex(buf, (size_t)num_bytes, out);
    return out;
}

const char* joss_password_hash(const char* password) {
    password = crypto_cstr(password);
    const int iterations = 100000;
    uint8_t salt[16];
    uint8_t dk[32];
    if (!joss_os_random_bytes(salt, sizeof(salt))) {
        joss_panic("JOSS-CRYPTO-001: OS CSPRNG salt generation failed");
    }
    joss_prim_pbkdf2_sha256((const uint8_t*)password, strlen(password), salt, sizeof(salt), iterations, dk);
    char salt_hex[33], dk_hex[65];
    bytes_to_hex(salt, sizeof(salt), salt_hex);
    bytes_to_hex(dk, sizeof(dk), dk_hex);
    char* out = (char*)joss_req_alloc(140);
    if (!out) return "";
    snprintf(out, 140, "$pbkdf2-sha256$%d$%s$%s", iterations, salt_hex, dk_hex);
    return out;
}

int64_t joss_password_verify(const char* hash, const char* password) {
    hash = crypto_cstr(hash);
    password = crypto_cstr(password);
    if (!hash[0]) return 0;
    if (hash[0] == '$' && hash[1] == '2' &&
        (hash[2] == 'a' || hash[2] == 'b' || hash[2] == 'y') &&
        hash[3] == '$') {
        return joss_prim_bcrypt_verify(hash, password);
    }
    const char* prefix = "$pbkdf2-sha256$";
    size_t plen = strlen(prefix);
    if (strncmp(hash, prefix, plen) != 0) return 0;
    const char* p = hash + plen;
    const char* d1 = strchr(p, '$');
    if (!d1) return 0;
    int iterations = atoi(p);
    if (iterations < 1000 || iterations > 5000000) return 0;
    const char* salt_start = d1 + 1;
    const char* d2 = strchr(salt_start, '$');
    if (!d2 || (size_t)(d2 - salt_start) != 32) return 0;
    char salt_hex[33];
    memcpy(salt_hex, salt_start, 32);
    salt_hex[32] = '\0';
    uint8_t salt[16], expected_dk[32], actual_dk[32];
    if (!hex_to_bytes(salt_hex, salt, 16)) return 0;
    if (!hex_to_bytes(d2 + 1, expected_dk, 32)) return 0;
    joss_prim_pbkdf2_sha256((const uint8_t*)password, strlen(password), salt, sizeof(salt), iterations, actual_dk);
    return joss_ct_memcmp(actual_dk, expected_dk, 32) == 0 ? 1 : 0;
}

const char* joss_jwt_sign(const char* payload_json, const char* secret) {
    payload_json = crypto_cstr(payload_json);
    if (!payload_json[0]) payload_json = "{}";
    secret = crypto_cstr(secret);
    const char* header_json = "{\"alg\":\"HS256\",\"typ\":\"JWT\"}";
    char* h_b64 = b64url_encode((const uint8_t*)header_json, strlen(header_json));
    char* p_b64 = b64url_encode((const uint8_t*)payload_json, strlen(payload_json));
    if (!h_b64 || !p_b64) return "";
    size_t h_len = strlen(h_b64), p_len = strlen(p_b64), msg_len = h_len + 1 + p_len;
    char* msg = (char*)joss_req_alloc(msg_len + 1);
    if (!msg) return "";
    memcpy(msg, h_b64, h_len); msg[h_len] = '.'; memcpy(msg + h_len + 1, p_b64, p_len); msg[msg_len] = '\0';
    uint8_t mac[32];
    joss_prim_hmac_sha256((const uint8_t*)secret, strlen(secret), (const uint8_t*)msg, msg_len, mac);
    char* s_b64 = b64url_encode(mac, 32);
    if (!s_b64) return "";
    size_t s_len = strlen(s_b64);
    char* token = (char*)joss_req_alloc(msg_len + 1 + s_len + 1);
    if (!token) return "";
    memcpy(token, msg, msg_len); token[msg_len] = '.'; memcpy(token + msg_len + 1, s_b64, s_len); token[msg_len + 1 + s_len] = '\0';
    return token;
}

int64_t joss_jwt_verify(const char* token, const char* secret) {
    token = crypto_cstr(token);
    secret = crypto_cstr(secret);
    if (!token[0]) return 0;
    const char* dot1 = strchr(token, '.');
    if (!dot1) return 0;
    const char* dot2 = strchr(dot1 + 1, '.');
    if (!dot2 || strchr(dot2 + 1, '.') != NULL) return 0;
    size_t msg_len = (size_t)(dot2 - token);
    const char* given_sig = dot2 + 1;
    size_t given_len = strlen(given_sig);
    if (given_len == 0) return 0;
    uint8_t mac[32];
    joss_prim_hmac_sha256((const uint8_t*)secret, strlen(secret), (const uint8_t*)token, msg_len, mac);
    char* expected_sig = b64url_encode(mac, 32);
    if (!expected_sig || strlen(expected_sig) != given_len) return 0;
    return joss_ct_memcmp(expected_sig, given_sig, given_len) == 0 ? 1 : 0;
}

const char* joss_totp_generate(const char* base32_secret, int64_t unix_time) {
    if (!base32_secret || !*base32_secret) return "000000";
    if (unix_time <= 0) unix_time = (int64_t)time(NULL);
    uint8_t key[128];
    size_t key_len = base32_decode(base32_secret, key, sizeof(key));
    if (key_len == 0) {
        key_len = strlen(base32_secret);
        if (key_len > sizeof(key)) key_len = sizeof(key);
        memcpy(key, base32_secret, key_len);
    }
    char* out = (char*)joss_req_alloc(7);
    if (!out) return "000000";
    totp_for_step_builtin(key, key_len, (uint64_t)(unix_time / 30), out);
    return out;
}

int64_t joss_totp_verify(const char* base32_secret, const char* code, int64_t unix_time) {
    if (!base32_secret || !*base32_secret || !code) return 0;
    while (*code == ' ' || *code == '\t') code++;
    char clean[7];
    size_t clen = 0;
    while (code[clen] != '\0' && code[clen] != ' ' && code[clen] != '\t' && clen < 6) {
        if (code[clen] < '0' || code[clen] > '9') return 0;
        clean[clen] = code[clen];
        clen++;
    }
    if (clen != 6 || (code[clen] != '\0' && code[clen] != ' ' && code[clen] != '\t')) return 0;
    clean[6] = '\0';
    if (unix_time <= 0) unix_time = (int64_t)time(NULL);
    uint8_t key[128];
    size_t key_len = base32_decode(base32_secret, key, sizeof(key));
    if (key_len == 0) {
        key_len = strlen(base32_secret);
        if (key_len > sizeof(key)) key_len = sizeof(key);
        memcpy(key, base32_secret, key_len);
    }
    int64_t base_step = unix_time / 30;
    for (int64_t delta = -1; delta <= 1; delta++) {
        if (base_step + delta < 0) continue;
        char expected[7];
        totp_for_step_builtin(key, key_len, (uint64_t)(base_step + delta), expected);
        if (joss_ct_memcmp(expected, clean, 6) == 0) return 1;
    }
    return 0;
}

#endif
