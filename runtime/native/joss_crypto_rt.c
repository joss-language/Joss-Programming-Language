#include "joss_rt.h"

#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#if defined(__has_include)
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

#if defined(JOSS_HAS_OPENSSL)

const char* joss_sha256(const char* s) {
    if (!s) s = "";
    uint8_t digest[32];
    unsigned int dlen = 0;
    EVP_MD_CTX* ctx = EVP_MD_CTX_new();
    if (!ctx) return "";
    EVP_DigestInit_ex(ctx, EVP_sha256(), NULL);
    EVP_DigestUpdate(ctx, s, strlen(s));
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
    if (!password) password = "";
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
    if (!hash || !password) return 0;
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
    if (!payload_json) payload_json = "{}";
    if (!secret) secret = "";
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
    if (!token || !secret) return 0;
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

const char* joss_sha256(const char* s) {
    (void)s;
    joss_panic("JOSS-CRYPTO-003: OpenSSL libcrypto required for joss_sha256");
    return "";
}

const char* joss_random_hex(int64_t num_bytes) {
    (void)num_bytes;
    joss_panic("JOSS-CRYPTO-003: OpenSSL libcrypto required for joss_random_hex");
    return "";
}

const char* joss_password_hash(const char* password) {
    (void)password;
    joss_panic("JOSS-CRYPTO-003: OpenSSL libcrypto required for joss_password_hash");
    return "";
}

int64_t joss_password_verify(const char* hash, const char* password) {
    (void)hash;
    (void)password;
    joss_panic("JOSS-CRYPTO-003: OpenSSL libcrypto required for joss_password_verify");
    return 0;
}

const char* joss_jwt_sign(const char* payload_json, const char* secret) {
    (void)payload_json;
    (void)secret;
    joss_panic("JOSS-CRYPTO-003: OpenSSL libcrypto required for joss_jwt_sign");
    return "";
}

int64_t joss_jwt_verify(const char* token, const char* secret) {
    (void)token;
    (void)secret;
    joss_panic("JOSS-CRYPTO-003: OpenSSL libcrypto required for joss_jwt_verify");
    return 0;
}

const char* joss_totp_generate(const char* base32_secret, int64_t unix_time) {
    (void)base32_secret;
    (void)unix_time;
    joss_panic("JOSS-CRYPTO-003: OpenSSL libcrypto required for joss_totp_generate");
    return "";
}

int64_t joss_totp_verify(const char* base32_secret, const char* code, int64_t unix_time) {
    (void)base32_secret;
    (void)code;
    (void)unix_time;
    joss_panic("JOSS-CRYPTO-003: OpenSSL libcrypto required for joss_totp_verify");
    return 0;
}

#endif
