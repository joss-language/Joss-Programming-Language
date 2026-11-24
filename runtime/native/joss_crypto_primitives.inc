//go:build ignore
#ifndef JOSS_CRYPTO_PRIMITIVES_INC
#define JOSS_CRYPTO_PRIMITIVES_INC

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

/* =========================================================================
 * 1. FIPS 180-4 SHA-256, HMAC-SHA256 & PBKDF2-HMAC-SHA256
 * ========================================================================= */
static inline uint32_t joss_prim_rotr32(uint32_t x, uint32_t n) {
    return (x >> n) | (x << (32U - n));
}

static inline void joss_prim_sha256_transform(uint32_t state[8], const uint8_t block[64]) {
    static const uint32_t K[64] = {
        0x428a2f98U,0x71374491U,0xb5c0fbcfU,0xe9b5dba5U,0x3956c25bU,0x59f111f1U,0x923f82a4U,0xab1c5ed5U,
        0xd807aa98U,0x12835b01U,0x243185beU,0x550c7dc3U,0x72be5d74U,0x80deb1feU,0x9bdc06a7U,0xc19bf174U,
        0xe49b69c1U,0xefbe4786U,0x0fc19dc6U,0x240ca1ccU,0x2de92c6fU,0x4a7484aaU,0x5cb0a9dcU,0x76f988daU,
        0x983e5152U,0xa831c66dU,0xb00327c8U,0xbf597fc7U,0xc6e00bf3U,0xd5a79147U,0x06ca6351U,0x14292967U,
        0x27b70a85U,0x2e1b2138U,0x4d2c6dfcU,0x53380d13U,0x650a7354U,0x766a0abbU,0x81c2c92eU,0x92722c85U,
        0xa2bfe8a1U,0xa81a664bU,0xc24b8b70U,0xc76c51a3U,0xd192e819U,0xd6990624U,0xf40e3585U,0x106aa070U,
        0x19a4c116U,0x1e376c08U,0x2748774cU,0x34b0bcb5U,0x391c0cb3U,0x4ed8aa4aU,0x5b9cca4fU,0x682e6ff3U,
        0x748f82eeU,0x78a5636fU,0x84c87814U,0x8cc70208U,0x90befffaU,0xa4506cebU,0xbef9a3f7U,0xc67178f2U
    };
    uint32_t w[64];
    for (int i = 0; i < 16; i++) {
        w[i] = ((uint32_t)block[i * 4] << 24) |
               ((uint32_t)block[i * 4 + 1] << 16) |
               ((uint32_t)block[i * 4 + 2] << 8) |
               ((uint32_t)block[i * 4 + 3]);
    }
    for (int i = 16; i < 64; i++) {
        uint32_t s0 = joss_prim_rotr32(w[i - 15], 7) ^ joss_prim_rotr32(w[i - 15], 18) ^ (w[i - 15] >> 3);
        uint32_t s1 = joss_prim_rotr32(w[i - 2], 17) ^ joss_prim_rotr32(w[i - 2], 19) ^ (w[i - 2] >> 10);
        w[i] = w[i - 16] + s0 + w[i - 7] + s1;
    }
    uint32_t a = state[0], b = state[1], c = state[2], d = state[3];
    uint32_t e = state[4], f = state[5], g = state[6], h = state[7];
    for (int i = 0; i < 64; i++) {
        uint32_t S1 = joss_prim_rotr32(e, 6) ^ joss_prim_rotr32(e, 11) ^ joss_prim_rotr32(e, 25);
        uint32_t ch = (e & f) ^ ((~e) & g);
        uint32_t temp1 = h + S1 + ch + K[i] + w[i];
        uint32_t S0 = joss_prim_rotr32(a, 2) ^ joss_prim_rotr32(a, 13) ^ joss_prim_rotr32(a, 22);
        uint32_t maj = (a & b) ^ (a & c) ^ (b & c);
        uint32_t temp2 = S0 + maj;
        h = g; g = f; f = e; e = d + temp1;
        d = c; c = b; b = a; a = temp1 + temp2;
    }
    state[0] += a; state[1] += b; state[2] += c; state[3] += d;
    state[4] += e; state[5] += f; state[6] += g; state[7] += h;
}

static inline void joss_prim_sha256(const uint8_t* data, size_t len, uint8_t out[32]) {
    uint32_t state[8] = {
        0x6a09e667U, 0xbb67ae85U, 0x3c6ef372U, 0xa54ff53aU,
        0x510e527fU, 0x9b05688cU, 0x1f83d9abU, 0x5be0cd19U
    };
    size_t i = 0;
    if (data && len >= 64) {
        while (i + 64 <= len) {
            joss_prim_sha256_transform(state, data + i);
            i += 64;
        }
    }
    uint8_t tail[128];
    memset(tail, 0, sizeof(tail));
    size_t rem = len - i;
    if (data && rem > 0) memcpy(tail, data + i, rem);
    tail[rem] = 0x80;
    size_t total_blocks = (rem < 56) ? 1 : 2;
    uint64_t bit_len = (uint64_t)len * 8ULL;
    for (int j = 0; j < 8; j++) {
        tail[total_blocks * 64 - 1 - j] = (uint8_t)(bit_len >> (j * 8));
    }
    joss_prim_sha256_transform(state, tail);
    if (total_blocks == 2) joss_prim_sha256_transform(state, tail + 64);
    for (int j = 0; j < 8; j++) {
        out[j * 4]     = (uint8_t)(state[j] >> 24);
        out[j * 4 + 1] = (uint8_t)(state[j] >> 16);
        out[j * 4 + 2] = (uint8_t)(state[j] >> 8);
        out[j * 4 + 3] = (uint8_t)(state[j]);
    }
}

static inline void joss_prim_hmac_sha256(const uint8_t* key, size_t key_len, const uint8_t* msg, size_t msg_len, uint8_t out[32]) {
    uint8_t k_pad[64];
    memset(k_pad, 0, sizeof(k_pad));
    if (key && key_len > 64) {
        joss_prim_sha256(key, key_len, k_pad);
    } else if (key && key_len > 0) {
        memcpy(k_pad, key, key_len);
    }
    uint8_t* inner = (uint8_t*)malloc(64 + msg_len);
    if (!inner) { memset(out, 0, 32); return; }
    for (int i = 0; i < 64; i++) inner[i] = k_pad[i] ^ 0x36;
    if (msg && msg_len > 0) memcpy(inner + 64, msg, msg_len);
    uint8_t ih[32];
    joss_prim_sha256(inner, 64 + msg_len, ih);
    free(inner);

    uint8_t outer[64 + 32];
    for (int i = 0; i < 64; i++) outer[i] = k_pad[i] ^ 0x5C;
    memcpy(outer + 64, ih, 32);
    joss_prim_sha256(outer, sizeof(outer), out);
}

static inline void joss_prim_pbkdf2_sha256(const uint8_t* pass, size_t pass_len, const uint8_t* salt, size_t salt_len, int iterations, uint8_t out[32]) {
    uint8_t* salt_blk = (uint8_t*)malloc(salt_len + 4);
    if (!salt_blk) { memset(out, 0, 32); return; }
    if (salt && salt_len > 0) memcpy(salt_blk, salt, salt_len);
    salt_blk[salt_len]     = 0;
    salt_blk[salt_len + 1] = 0;
    salt_blk[salt_len + 2] = 0;
    salt_blk[salt_len + 3] = 1;
    uint8_t u[32];
    joss_prim_hmac_sha256(pass, pass_len, salt_blk, salt_len + 4, u);
    free(salt_blk);
    memcpy(out, u, 32);
    for (int i = 1; i < iterations; i++) {
        joss_prim_hmac_sha256(pass, pass_len, u, 32, u);
        for (int j = 0; j < 32; j++) out[j] ^= u[j];
    }
}

/* =========================================================================
 * 2. RFC 1321 MD5
 * ========================================================================= */
static inline uint32_t joss_prim_rotl32(uint32_t x, uint32_t n) {
    return (x << n) | (x >> (32U - n));
}

static inline void joss_prim_md5_transform(uint32_t state[4], const uint8_t block[64]) {
    static const uint32_t K[64] = {
        0xd76aa478U,0xe8c7b756U,0x242070dbU,0xc1bdceeeU,0xf57c0fafU,0x4787c62aU,0xa8304613U,0xfd469501U,
        0x698098d8U,0x8b44f7afU,0xffff5bb1U,0x895cd7beU,0x6b901122U,0xfd987193U,0xa679438eU,0x49b40821U,
        0xf61e2562U,0xc040b340U,0x265e5a51U,0xe9b6c7aaU,0xd62f105dU,0x02441453U,0xd8a1e681U,0xe7d3fbc8U,
        0x21e1cde6U,0xc33707d6U,0xf4d50d87U,0x455a14edU,0xa9e3e905U,0xfcefa3f8U,0x676f02d9U,0x8d2a4c8aU,
        0xfffa3942U,0x8771f681U,0x6d9d6122U,0xfde5380cU,0xa4beea44U,0x4bdecfa9U,0xf6bb4b60U,0xbebfbc70U,
        0x289b7ec6U,0xeaa127faU,0xd4ef3085U,0x04881d05U,0xd9d4d039U,0xe6db99e5U,0x1fa27cf8U,0xc4ac5665U,
        0xf4292244U,0x432aff97U,0xab9423a7U,0xfc93a039U,0x655b59c3U,0x8f0ccc92U,0xffeff47dU,0x85845dd1U,
        0x6fa87e4fU,0xfe2ce6e0U,0xa3014314U,0x4e0811a1U,0xf7537e82U,0xbd3af235U,0x2ad7d2bbU,0xeb86d391U
    };
    static const uint32_t S[64] = {
        7,12,17,22, 7,12,17,22, 7,12,17,22, 7,12,17,22,
        5, 9,14,20, 5, 9,14,20, 5, 9,14,20, 5, 9,14,20,
        4,11,16,23, 4,11,16,23, 4,11,16,23, 4,11,16,23,
        6,10,15,21, 6,10,15,21, 6,10,15,21, 6,10,15,21
    };
    uint32_t M[16];
    for (int i = 0; i < 16; i++) {
        M[i] = (uint32_t)block[i * 4] |
               ((uint32_t)block[i * 4 + 1] << 8) |
               ((uint32_t)block[i * 4 + 2] << 16) |
               ((uint32_t)block[i * 4 + 3] << 24);
    }
    uint32_t A = state[0], B = state[1], C = state[2], D = state[3];
    for (int i = 0; i < 64; i++) {
        uint32_t F, g;
        if (i < 16)      { F = (B & C) | ((~B) & D); g = (uint32_t)i; }
        else if (i < 32) { F = (D & B) | ((~D) & C); g = (uint32_t)((5 * i + 1) % 16); }
        else if (i < 48) { F = B ^ C ^ D;            g = (uint32_t)((3 * i + 5) % 16); }
        else             { F = C ^ (B | (~D));       g = (uint32_t)((7 * i) % 16); }
        uint32_t dTemp = D;
        D = C;
        C = B;
        B = B + joss_prim_rotl32(A + F + K[i] + M[g], S[i]);
        A = dTemp;
    }
    state[0] += A; state[1] += B; state[2] += C; state[3] += D;
}

static inline void joss_prim_md5(const uint8_t* data, size_t len, uint8_t out[16]) {
    uint32_t state[4] = {0x67452301U, 0xefcdab89U, 0x98badcfeU, 0x10325476U};
    size_t i = 0;
    if (data && len >= 64) {
        while (i + 64 <= len) {
            joss_prim_md5_transform(state, data + i);
            i += 64;
        }
    }
    uint8_t tail[128];
    memset(tail, 0, sizeof(tail));
    size_t rem = len - i;
    if (data && rem > 0) memcpy(tail, data + i, rem);
    tail[rem] = 0x80;
    size_t total_blocks = (rem < 56) ? 1 : 2;
    uint64_t bit_len = (uint64_t)len * 8ULL;
    for (int j = 0; j < 8; j++) {
        tail[total_blocks * 64 - 8 + j] = (uint8_t)(bit_len >> (j * 8));
    }
    joss_prim_md5_transform(state, tail);
    if (total_blocks == 2) joss_prim_md5_transform(state, tail + 64);
    for (int j = 0; j < 4; j++) {
        out[j * 4]     = (uint8_t)(state[j] & 0xFFU);
        out[j * 4 + 1] = (uint8_t)((state[j] >> 8) & 0xFFU);
        out[j * 4 + 2] = (uint8_t)((state[j] >> 16) & 0xFFU);
        out[j * 4 + 3] = (uint8_t)((state[j] >> 24) & 0xFFU);
    }
}

/* =========================================================================
 * 3. RFC 4648 & Custom-Alphabet Base64 Encode/Decode
 * ========================================================================= */
static inline size_t joss_prim_base64_encode_alphabet(const uint8_t* in, size_t in_len, const char alphabet[65], bool pad, char* out) {
    size_t j = 0;
    for (size_t i = 0; i < in_len; i += 3) {
        uint32_t v = ((uint32_t)in[i] << 16) |
                     (i + 1 < in_len ? ((uint32_t)in[i + 1] << 8) : 0U) |
                     (i + 2 < in_len ? (uint32_t)in[i + 2] : 0U);
        out[j++] = alphabet[(v >> 18) & 0x3FU];
        out[j++] = alphabet[(v >> 12) & 0x3FU];
        if (i + 1 < in_len) out[j++] = alphabet[(v >> 6) & 0x3FU];
        else if (pad) out[j++] = '=';
        if (i + 2 < in_len) out[j++] = alphabet[v & 0x3FU];
        else if (pad) out[j++] = '=';
    }
    out[j] = '\0';
    return j;
}

static inline size_t joss_prim_base64_encode(const uint8_t* in, size_t in_len, char* out) {
    static const char std_tbl[65] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    return joss_prim_base64_encode_alphabet(in, in_len, std_tbl, true, out);
}

static inline int joss_prim_b64_find(unsigned char c, const char alphabet[65]) {
    for (int i = 0; i < 64; i++) {
        if ((unsigned char)alphabet[i] == c) return i;
    }
    return -1;
}

static inline size_t joss_prim_base64_decode_alphabet(const char* in, size_t in_len, const char alphabet[65], uint8_t* out, size_t out_cap) {
    size_t out_len = 0;
    uint32_t buf = 0;
    int bits = 0;
    for (size_t i = 0; i < in_len; i++) {
        unsigned char c = (unsigned char)in[i];
        if (c == '=') break;
        if (c == ' ' || c == '\t' || c == '\r' || c == '\n') continue;
        int val = joss_prim_b64_find(c, alphabet);
        if (val < 0) return 0;
        buf = (buf << 6) | (uint32_t)val;
        bits += 6;
        if (bits >= 8) {
            bits -= 8;
            if (out_len < out_cap) {
                out[out_len++] = (uint8_t)((buf >> bits) & 0xFFU);
            }
        }
    }
    return out_len;
}

static inline size_t joss_prim_base64_decode(const char* in, uint8_t* out, size_t out_cap) {
    static const char std_tbl[65] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    if (!in) return 0;
    return joss_prim_base64_decode_alphabet(in, strlen(in), std_tbl, out, out_cap);
}

/* =========================================================================
 * 4. OpenBSD Bcrypt ($2a$, $2b$, $2y$) Verification (Eksblowfish)
 * ========================================================================= */
typedef struct {
    uint32_t p[18];
    uint32_t s[4][256];
} JossBlowfishCtx;

static inline uint32_t joss_bf_f(const JossBlowfishCtx* c, uint32_t x) {
    uint32_t a = c->s[0][(x >> 24) & 0xFFU];
    uint32_t b = c->s[1][(x >> 16) & 0xFFU];
    uint32_t cc = c->s[2][(x >> 8) & 0xFFU];
    uint32_t d = c->s[3][x & 0xFFU];
    return ((a + b) ^ cc) + d;
}

static inline void joss_bf_encrypt(const JossBlowfishCtx* c, uint32_t* l_io, uint32_t* r_io) {
    uint32_t l = *l_io;
    uint32_t r = *r_io;
    for (int i = 0; i < 16; i += 2) {
        l ^= c->p[i];
        r ^= joss_bf_f(c, l);
        r ^= c->p[i + 1];
        l ^= joss_bf_f(c, r);
    }
    l ^= c->p[16];
    r ^= c->p[17];
    *l_io = r;
    *r_io = l;
}

static inline uint32_t joss_bf_next_word(const uint8_t* data, size_t len, size_t* pos) {
    uint32_t w = 0;
    size_t j = *pos;
    for (int i = 0; i < 4; i++) {
        w = (w << 8) | (uint32_t)data[j];
        j++;
        if (j >= len) j = 0;
    }
    *pos = j;
    return w;
}

static inline void joss_bf_expand_key(JossBlowfishCtx* c, const uint8_t* key, size_t key_len) {
    size_t j = 0;
    for (int i = 0; i < 18; i++) c->p[i] ^= joss_bf_next_word(key, key_len, &j);
    uint32_t l = 0, r = 0;
    for (int i = 0; i < 18; i += 2) {
        joss_bf_encrypt(c, &l, &r);
        c->p[i] = l; c->p[i + 1] = r;
    }
    for (int t = 0; t < 4; t++) {
        for (int i = 0; i < 256; i += 2) {
            joss_bf_encrypt(c, &l, &r);
            c->s[t][i] = l; c->s[t][i + 1] = r;
        }
    }
}

static inline void joss_bf_expand_key_salt(JossBlowfishCtx* c, const uint8_t* key, size_t key_len, const uint8_t* salt, size_t salt_len) {
    size_t j = 0;
    for (int i = 0; i < 18; i++) c->p[i] ^= joss_bf_next_word(key, key_len, &j);
    j = 0;
    uint32_t l = 0, r = 0;
    for (int i = 0; i < 18; i += 2) {
        l ^= joss_bf_next_word(salt, salt_len, &j);
        r ^= joss_bf_next_word(salt, salt_len, &j);
        joss_bf_encrypt(c, &l, &r);
        c->p[i] = l; c->p[i + 1] = r;
    }
    for (int t = 0; t < 4; t++) {
        for (int i = 0; i < 256; i += 2) {
            l ^= joss_bf_next_word(salt, salt_len, &j);
            r ^= joss_bf_next_word(salt, salt_len, &j);
            joss_bf_encrypt(c, &l, &r);
            c->s[t][i] = l; c->s[t][i + 1] = r;
        }
    }
}

static inline void joss_bf_init(JossBlowfishCtx* c) {
    static const uint32_t P0[18] = {
        0x243f6a88U,0x85a308d3U,0x13198a2eU,0x03707344U,0xa4093822U,0x299f31d0U,
        0x082efa98U,0xec4e6c89U,0x452821e6U,0x38d01377U,0xbe5466cfU,0x34e90c6cU,
        0xc0ac29b7U,0xc97c50ddU,0x3f84d5b5U,0xb5470917U,0x9216d5d9U,0x8979fb1bU
    };
    static const uint32_t S0_INIT[4][256] = {
        {
            0xd1310ba6U,0x98dfb5acU,0x2ffd72dbU,0xd01adfb7U,0xb8e1afedU,0x6a267e96U,
            0xba7c9045U,0xf12c7f99U,0x24a19947U,0xb3916cf7U,0x0801f2e2U,0x858efc16U,
            0x636920d8U,0x71574e69U,0xa458fea3U,0xf4933d7eU,0x0d95748fU,0x728eb658U,
            0x718bcd58U,0x82154aeeU,0x7b54a41dU,0xc25a59b5U,0x9c30d539U,0x2af26013U,
            0xc5d1b023U,0x286085f0U,0xca417918U,0xb8db38efU,0x8e79dcb0U,0x603a180eU,
            0x6c9e0e8bU,0xb01e8a3eU,0xd71577c1U,0xbd314b27U,0x78af2fdaU,0x55605c60U,
            0xe65525f3U,0xaa55ab94U,0x57489862U,0x63e81440U,0x55ca396aU,0x2aab10b6U,
            0xb4cc5c34U,0x1141e8ceU,0xa15486afU,0x7c72e993U,0xb3ee1411U,0x636fbc2aU,
            0x2ba9c55dU,0x741831f6U,0xce5c3e16U,0x9b87931eU,0xafd6ba33U,0x6c24cf5cU,
            0x7a325381U,0x28958677U,0x3b8f4898U,0x6b4bb9afU,0xc4bfe81bU,0x66282193U,
            0x61d809ccU,0xfb21a991U,0x487cac60U,0x5dec8032U,0xef845d5dU,0xe98575b1U,
            0xdc262302U,0xeb651b88U,0x23893e81U,0xd396acc5U,0x0f6d6ff3U,0x83f44239U,
            0x2e0b4482U,0xa4842004U,0x69c8f04aU,0x9e1f9b5eU,0x21c66842U,0xf6e96c9aU,
            0x670c9c61U,0xabd388f0U,0x6a51a0d2U,0xd8542f68U,0x960fa728U,0xab5133a3U,
            0x6eef0b6cU,0x137a3be4U,0xba3bf050U,0x7efb2a98U,0xa1f1651dU,0x39af0176U,
            0x66ca593eU,0x82430e88U,0x8cee8619U,0x456f9fb4U,0x7d84a5c3U,0x3b8b5ebeU,
            0xe06f75d8U,0x85c12073U,0x401a449fU,0x56c16aa6U,0x4ed3aa62U,0x363f7706U,
            0x1bfedf72U,0x429b023dU,0x37d0d724U,0xd00a1248U,0xdb0fead3U,0x49f1c09bU,
            0x075372c9U,0x80991b7bU,0x25d479d8U,0xf6e8def7U,0xe3fe501aU,0xb6794c3bU,
            0x976ce0bdU,0x04c006baU,0xc1a94fb6U,0x409f60c4U,0x5e5c9ec2U,0x196a2463U,
            0x68fb6fafU,0x3e6c53b5U,0x1339b2ebU,0x3b52ec6fU,0x6dfc511fU,0x9b30952cU,
            0xcc814544U,0xaf5ebd09U,0xbee3d004U,0xde334afdU,0x660f2807U,0x192e4bb3U,
            0xc0cba857U,0x45c8740fU,0xd20b5f39U,0xb9d3fbdbU,0x5579c0bdU,0x1a60320aU,
            0xd6a100c6U,0x402c7279U,0x679f25feU,0xfb1fa3ccU,0x8ea5e9f8U,0xdb3222f8U,
            0x3c7516dfU,0xfd616b15U,0x2f501ec8U,0xad0552abU,0x323db5faU,0xfd238760U,
            0x53317b48U,0x3e00df82U,0x9e5c57bbU,0xca6f8ca0U,0x1a87562eU,0xdf1769dbU,
            0xd542a8f6U,0x287effc3U,0xac6732c6U,0x8c4f5573U,0x695b27b0U,0xbbca58c8U,
            0xe1ffa35dU,0xb8f011a0U,0x10fa3d98U,0xfd2183b8U,0x4afcb56cU,0x2dd1d35bU,
            0x9a53e479U,0xb6f84565U,0xd28e49bcU,0x4bfb9790U,0xe1ddf2daU,0xa4cb7e33U,
            0x62fb1341U,0xcee4c6e8U,0xef20cadaU,0x36774c01U,0xd07e9efeU,0x2bf11fb4U,
            0x95dbda4dU,0xae909198U,0xeaad8e71U,0x6b93d5a0U,0xd08ed1d0U,0xafc725e0U,
            0x8e3c5b2fU,0x8e7594b7U,0x8ff6e2fbU,0xf2122b64U,0x8888b812U,0x900df01cU,
            0x4fad5ea0U,0x688fc31cU,0xd1cff191U,0xb3a8c1adU,0x2f2f2218U,0xbe0e1777U,
            0xea752dfeU,0x8b021fa1U,0xe5a0cc0fU,0xb56f74e8U,0x18acf3d6U,0xce89e299U,
            0xb4a84fe0U,0xfd13e0b7U,0x7cc43b81U,0xd2ada8d9U,0x165fa266U,0x80957705U,
            0x93cc7314U,0x211a1477U,0xe6ad2065U,0x77b5fa86U,0xc75442f5U,0xfb9d35cfU,
            0xebcdaf0cU,0x7b3e89a0U,0xd6411bd3U,0xae1e7e49U,0x00250e2dU,0x2071b35eU,
            0x226800bbU,0x57b8e0afU,0x2464369bU,0xf009b91eU,0x5563911dU,0x59dfa6aaU,
            0x78c14389U,0xd95a537fU,0x207d5ba2U,0x02e5b9c5U,0x83260376U,0x6295cfa9U,
            0x11c81968U,0x4e734a41U,0xb3472dcaU,0x7b14a94aU,0x1b510052U,0x9a532915U,
            0xd60f573fU,0xbc9bc6e4U,0x2b60a476U,0x81e67400U,0x08ba6fb5U,0x571be91fU,
            0xf296ec6bU,0x2a0dd915U,0xb6636521U,0xe7b9f9b6U,0xff34052eU,0xc5855664U,
            0x53b02d5dU,0xa99f8fa1U,0x08ba4799U,0x6e85076aU
        },
        {
            0x4b7a70e9U,0xb5b32944U,0xdb75092eU,0xc4192623U,0xad6ea6b0U,0x49a7df7dU,
            0x9cee60b8U,0x8fedb266U,0xecaa8c71U,0x699a17ffU,0x5664526cU,0xc2b19ee1U,
            0x193602a5U,0x75094c29U,0xa0591340U,0xe4183a3eU,0x3f54989aU,0x5b429d65U,
            0x6b8fe4d6U,0x99f73fd6U,0xa1d29c07U,0xefe830f5U,0x4d2d38e6U,0xf0255dc1U,
            0x4cdd2086U,0x8470eb26U,0x6382e9c6U,0x021ecc5eU,0x09686b3fU,0x3ebaefc9U,
            0x3c971814U,0x6b6a70a1U,0x687f3584U,0x52a0e286U,0xb79c5305U,0xaa500737U,
            0x3e07841cU,0x7fdeae5cU,0x8e7d44ecU,0x5716f2b8U,0xb03ada37U,0xf0500c0dU,
            0xf01c1f04U,0x0200b3ffU,0xae0cf51aU,0x3cb574b2U,0x25837a58U,0xdc0921bdU,
            0xd19113f9U,0x7ca92ff6U,0x94324773U,0x22f54701U,0x3ae5e581U,0x37c2dadcU,
            0xc8b57634U,0x9af3dda7U,0xa9446146U,0x0fd0030eU,0xecc8c73eU,0xa4751e41U,
            0xe238cd99U,0x3bea0e2fU,0x3280bba1U,0x183eb331U,0x4e548b38U,0x4f6db908U,
            0x6f420d03U,0xf60a04bfU,0x2cb81290U,0x24977c79U,0x5679b072U,0xbcaf89afU,
            0xde9a771fU,0xd9930810U,0xb38bae12U,0xdccf3f2eU,0x5512721fU,0x2e6b7124U,
            0x501adde6U,0x9f84cd87U,0x7a584718U,0x7408da17U,0xbc9f9abcU,0xe94b7d8cU,
            0xec7aec3aU,0xdb851dfaU,0x63094366U,0xc464c3d2U,0xef1c1847U,0x3215d908U,
            0xdd433b37U,0x24c2ba16U,0x12a14d43U,0x2a65c451U,0x50940002U,0x133ae4ddU,
            0x71dff89eU,0x10314e55U,0x81ac77d6U,0x5f11199bU,0x043556f1U,0xd7a3c76bU,
            0x3c11183bU,0x5924a509U,0xf28fe6edU,0x97f1fbfaU,0x9ebabf2cU,0x1e153c6eU,
            0x86e34570U,0xeae96fb1U,0x860e5e0aU,0x5a3e2ab3U,0x771fe71cU,0x4e3d06faU,
            0x2965dcb9U,0x99e71d0fU,0x803e89d6U,0x5266c825U,0x2e4cc978U,0x9c10b36aU,
            0xc6150ebaU,0x94e2ea78U,0xa5fc3c53U,0x1e0a2df4U,0xf2f74ea7U,0x361d2b3dU,
            0x1939260fU,0x19c27960U,0x5223a708U,0xf71312b6U,0xebadfe6eU,0xeac31f66U,
            0xe3bc4595U,0xa67bc883U,0xb17f37d1U,0x018cff28U,0xc332ddefU,0xbe6c5aa5U,
            0x65582185U,0x68ab9802U,0xeecea50fU,0xdb2f953bU,0x2aef7dadU,0x5b6e2f84U,
            0x1521b628U,0x29076170U,0xecdd4775U,0x619f1510U,0x13cca830U,0xeb61bd96U,
            0x0334fe1eU,0xaa0363cfU,0xb5735c90U,0x4c70a239U,0xd59e9e0bU,0xcbaade14U,
            0xeecc86bcU,0x60622ca7U,0x9cab5cabU,0xb2f3846eU,0x648b1eafU,0x19bdf0caU,
            0xa02369b9U,0x655abb50U,0x40685a32U,0x3c2ab4b3U,0x319ee9d5U,0xc021b8f7U,
            0x9b540b19U,0x875fa099U,0x95f7997eU,0x623d7da8U,0xf837889aU,0x97e32d77U,
            0x11ed935fU,0x16681281U,0x0e358829U,0xc7e61fd6U,0x96dedfa1U,0x7858ba99U,
            0x57f584a5U,0x1b227263U,0x9b83c3ffU,0x1ac24696U,0xcdb30aebU,0x532e3054U,
            0x8fd948e4U,0x6dbc3128U,0x58ebf2efU,0x34c6ffeaU,0xfe28ed61U,0xee7c3c73U,
            0x5d4a14d9U,0xe864b7e3U,0x42105d14U,0x203e13e0U,0x45eee2b6U,0xa3aaabeaU,
            0xdb6c4f15U,0xfacb4fd0U,0xc742f442U,0xef6abbb5U,0x654f3b1dU,0x41cd2105U,
            0xd81e799eU,0x86854dc7U,0xe44b476aU,0x3d816250U,0xcf62a1f2U,0x5b8d2646U,
            0xfc8883a0U,0xc1c7b6a3U,0x7f1524c3U,0x69cb7492U,0x47848a0bU,0x5692b285U,
            0x095bbf00U,0xad19489dU,0x1462b174U,0x23820e00U,0x58428d2aU,0x0c55f5eaU,
            0x1dadf43eU,0x233f7061U,0x3372f092U,0x8d937e41U,0xd65fecf1U,0x6c223bdbU,
            0x7cde3759U,0xcbee7460U,0x4085f2a7U,0xce77326eU,0xa6078084U,0x19f8509eU,
            0xe8efd855U,0x61d99735U,0xa969a7aaU,0xc50c06c2U,0x5a04abfcU,0x800bcadcU,
            0x9e447a2eU,0xc3453484U,0xfdd56705U,0x0e1e9ec9U,0xdb73dbd3U,0x105588cdU,
            0x675fda79U,0xe3674340U,0xc5c43465U,0x713e38d8U,0x3d28f89eU,0xf16dff20U,
            0x153e21e7U,0x8fb03d4aU,0xe6e39f2bU,0xdb83adf7U
        },
        {
            0xe93d5a68U,0x948140f7U,0xf64c261cU,0x94692934U,0x411520f7U,0x7602d4f7U,
            0xbcf46b2eU,0xd4a20068U,0xd4082471U,0x3320f46aU,0x43b7d4b7U,0x500061afU,
            0x1e39f62eU,0x97244546U,0x14214f74U,0xbf8b8840U,0x4d95fc1dU,0x96b591afU,
            0x70f4ddd3U,0x66a02f45U,0xbfbc09ecU,0x03bd9785U,0x7fac6dd0U,0x31cb8504U,
            0x96eb27b3U,0x55fd3941U,0xda2547e6U,0xabca0a9aU,0x28507825U,0x530429f4U,
            0x0a2c86daU,0xe9b66dfbU,0x68dc1462U,0xd7486900U,0x680ec0a4U,0x27a18deeU,
            0x4f3ffea2U,0xe887ad8cU,0xb58ce006U,0x7af4d6b6U,0xaace1e7cU,0xd3375fecU,
            0xce78a399U,0x406b2a42U,0x20fe9e35U,0xd9f385b9U,0xee39d7abU,0x3b124e8bU,
            0x1dc9faf7U,0x4b6d1856U,0x26a36631U,0xeae397b2U,0x3a6efa74U,0xdd5b4332U,
            0x6841e7f7U,0xca7820fbU,0xfb0af54eU,0xd8feb397U,0x454056acU,0xba489527U,
            0x55533a3aU,0x20838d87U,0xfe6ba9b7U,0xd096954bU,0x55a867bcU,0xa1159a58U,
            0xcca92963U,0x99e1db33U,0xa62a4a56U,0x3f3125f9U,0x5ef47e1cU,0x9029317cU,
            0xfdf8e802U,0x04272f70U,0x80bb155cU,0x05282ce3U,0x95c11548U,0xe4c66d22U,
            0x48c1133fU,0xc70f86dcU,0x07f9c9eeU,0x41041f0fU,0x404779a4U,0x5d886e17U,
            0x325f51ebU,0xd59bc0d1U,0xf2bcc18fU,0x41113564U,0x257b7834U,0x602a9c60U,
            0xdff8e8a3U,0x1f636c1bU,0x0e12b4c2U,0x02e1329eU,0xaf664fd1U,0xcad18115U,
            0x6b2395e0U,0x333e92e1U,0x3b240b62U,0xeebeb922U,0x85b2a20eU,0xe6ba0d99U,
            0xde720c8cU,0x2da2f728U,0xd0127845U,0x95b794fdU,0x647d0862U,0xe7ccf5f0U,
            0x5449a36fU,0x877d48faU,0xc39dfd27U,0xf33e8d1eU,0x0a476341U,0x992eff74U,
            0x3a6f6eabU,0xf4f8fd37U,0xa812dc60U,0xa1ebddf8U,0x991be14cU,0xdb6e6b0dU,
            0xc67b5510U,0x6d672c37U,0x2765d43bU,0xdcd0e804U,0xf1290dc7U,0xcc00ffa3U,
            0xb5390f92U,0x690fed0bU,0x667b9ffbU,0xcedb7d9cU,0xa091cf0bU,0xd9155ea3U,
            0xbb132f88U,0x515bad24U,0x7b9479bfU,0x763bd6ebU,0x37392eb3U,0xcc115979U,
            0x8026e297U,0xf42e312dU,0x6842ada7U,0xc66a2b3bU,0x12754cccU,0x782ef11cU,
            0x6a124237U,0xb79251e7U,0x06a1bbe6U,0x4bfb6350U,0x1a6b1018U,0x11caedfaU,
            0x3d25bdd8U,0xe2e1c3c9U,0x44421659U,0x0a121386U,0xd90cec6eU,0xd5abea2aU,
            0x64af674eU,0xda86a85fU,0xbebfe988U,0x64e4c3feU,0x9dbc8057U,0xf0f7c086U,
            0x60787bf8U,0x6003604dU,0xd1fd8346U,0xf6381fb0U,0x7745ae04U,0xd736fcccU,
            0x83426b33U,0xf01eab71U,0xb0804187U,0x3c005e5fU,0x77a057beU,0xbde8ae24U,
            0x55464299U,0xbf582e61U,0x4e58f48fU,0xf2ddfda2U,0xf474ef38U,0x8789bdc2U,
            0x5366f9c3U,0xc8b38e74U,0xb475f255U,0x46fcd9b9U,0x7aeb2661U,0x8b1ddf84U,
            0x846a0e79U,0x915f95e2U,0x466e598eU,0x20b45770U,0x8cd55591U,0xc902de4cU,
            0xb90bace1U,0xbb8205d0U,0x11a86248U,0x7574a99eU,0xb77f19b6U,0xe0a9dc09U,
            0x662d09a1U,0xc4324633U,0xe85a1f02U,0x09f0be8cU,0x4a99a025U,0x1d6efe10U,
            0x1ab93d1dU,0x0ba5a4dfU,0xa186f20fU,0x2868f169U,0xdcb7da83U,0x573906feU,
            0xa1e2ce9bU,0x4fcd7f52U,0x50115e01U,0xa70683faU,0xa002b5c4U,0x0de6d027U,
            0x9af88c27U,0x773f8641U,0xc3604c06U,0x61a806b5U,0xf0177a28U,0xc0f586e0U,
            0x006058aaU,0x30dc7d62U,0x11e69ed7U,0x2338ea63U,0x53c2dd94U,0xc2c21634U,
            0xbbcbee56U,0x90bcb6deU,0xebfc7da1U,0xce591d76U,0x6f05e409U,0x4b7c0188U,
            0x39720a3dU,0x7c927c24U,0x86e3725fU,0x724d9db9U,0x1ac15bb4U,0xd39eb8fcU,
            0xed545578U,0x08fca5b5U,0xd83d7cd3U,0x4dad0fc4U,0x1e50ef5eU,0xb161e6f8U,
            0xa28514d9U,0x6c51133cU,0x6fd5c7e7U,0x56e14ec4U,0x362abfceU,0xddc6c837U,
            0xd79a3234U,0x92638212U,0x670efa8eU,0x406000e0U
        },
        {
            0x3a39ce37U,0xd3faf5cfU,0xabc27737U,0x5ac52d1bU,0x5cb0679eU,0x4fa33742U,
            0xd3822740U,0x99bc9bbeU,0xd5118e9dU,0xbf0f7315U,0xd62d1c7eU,0xc700c47bU,
            0xb78c1b6bU,0x21a19045U,0xb26eb1beU,0x6a366eb4U,0x5748ab2fU,0xbc946e79U,
            0xc6a376d2U,0x6549c2c8U,0x530ff8eeU,0x468dde7dU,0xd5730a1dU,0x4cd04dc6U,
            0x2939bbdbU,0xa9ba4650U,0xac9526e8U,0xbe5ee304U,0xa1fad5f0U,0x6a2d519aU,
            0x63ef8ce2U,0x9a86ee22U,0xc089c2b8U,0x43242ef6U,0xa51e03aaU,0x9cf2d0a4U,
            0x83c061baU,0x9be96a4dU,0x8fe51550U,0xba645bd6U,0x2826a2f9U,0xa73a3ae1U,
            0x4ba99586U,0xef5562e9U,0xc72fefd3U,0xf752f7daU,0x3f046f69U,0x77fa0a59U,
            0x80e4a915U,0x87b08601U,0x9b09e6adU,0x3b3ee593U,0xe990fd5aU,0x9e34d797U,
            0x2cf0b7d9U,0x022b8b51U,0x96d5ac3aU,0x017da67dU,0xd1cf3ed6U,0x7c7d2d28U,
            0x1f9f25cfU,0xadf2b89bU,0x5ad6b472U,0x5a88f54cU,0xe029ac71U,0xe019a5e6U,
            0x47b0acfdU,0xed93fa9bU,0xe8d3c48dU,0x283b57ccU,0xf8d56629U,0x79132e28U,
            0x785f0191U,0xed756055U,0xf7960e44U,0xe3d35e8cU,0x15056dd4U,0x88f46dbaU,
            0x03a16125U,0x0564f0bdU,0xc3eb9e15U,0x3c9057a2U,0x97271aecU,0xa93a072aU,
            0x1b3f6d9bU,0x1e6321f5U,0xf59c66fbU,0x26dcf319U,0x7533d928U,0xb155fdf5U,
            0x03563482U,0x8aba3cbbU,0x28517711U,0xc20ad9f8U,0xabcc5167U,0xccad925fU,
            0x4de81751U,0x3830dc8eU,0x379d5862U,0x9320f991U,0xea7a90c2U,0xfb3e7bceU,
            0x5121ce64U,0x774fbe32U,0xa8b6e37eU,0xc3293d46U,0x48de5369U,0x6413e680U,
            0xa2ae0810U,0xdd6db224U,0x69852dfdU,0x09072166U,0xb39a460aU,0x6445c0ddU,
            0x586cdecfU,0x1c20c8aeU,0x5bbef7ddU,0x1b588d40U,0xccd2017fU,0x6bb4e3bbU,
            0xdda26a7eU,0x3a59ff45U,0x3e350a44U,0xbcb4cdd5U,0x72eacea8U,0xfa6484bbU,
            0x8d6612aeU,0xbf3c6f47U,0xd29be463U,0x542f5d9eU,0xaec2771bU,0xf64e6370U,
            0x740e0d8dU,0xe75b1357U,0xf8721671U,0xaf537d5dU,0x4040cb08U,0x4eb4e2ccU,
            0x34d2466aU,0x0115af84U,0xe1b00428U,0x95983a1dU,0x06b89fb4U,0xce6ea048U,
            0x6f3f3b82U,0x3520ab82U,0x011a1d4bU,0x277227f8U,0x611560b1U,0xe7933fdcU,
            0xbb3a792bU,0x344525bdU,0xa08839e1U,0x51ce794bU,0x2f32c9b7U,0xa01fbac9U,
            0xe01cc87eU,0xbcc7d1f6U,0xcf0111c3U,0xa1e8aac7U,0x1a908749U,0xd44fbd9aU,
            0xd0dadecbU,0xd50ada38U,0x0339c32aU,0xc6913667U,0x8df9317cU,0xe0b12b4fU,
            0xf79e59b7U,0x43f5bb3aU,0xf2d519ffU,0x27d9459cU,0xbf97222cU,0x15e6fc2aU,
            0x0f91fc71U,0x9b941525U,0xfae59361U,0xceb69cebU,0xc2a86459U,0x12baa8d1U,
            0xb6c1075eU,0xe3056a0cU,0x10d25065U,0xcb03a442U,0xe0ec6e0eU,0x1698db3bU,
            0x4c98a0beU,0x3278e964U,0x9f1f9532U,0xe0d392dfU,0xd3a0342bU,0x8971f21eU,
            0x1b0a7441U,0x4ba3348cU,0xc5be7120U,0xc37632d8U,0xdf359f8dU,0x9b992f2eU,
            0xe60b6f47U,0x0fe3f11dU,0xe54cda54U,0x1edad891U,0xce6279cfU,0xcd3e7e6fU,
            0x1618b166U,0xfd2c1d05U,0x848fd2c5U,0xf6fb2299U,0xf523f357U,0xa6327623U,
            0x93a83531U,0x56cccd02U,0xacf08162U,0x5a75ebb5U,0x6e163697U,0x88d273ccU,
            0xde966292U,0x81b949d0U,0x4c50901bU,0x71c65614U,0xe6c6c7bdU,0x327a140aU,
            0x45e1d006U,0xc3f27b9aU,0xc9aa53fdU,0x62a80f00U,0xbb25bfe2U,0x35bdd2f6U,
            0x71126905U,0xb2040222U,0xb6cbcf7cU,0xcd769c2bU,0x53113ec0U,0x1640e3d3U,
            0x38abbd60U,0x2547adf0U,0xba38209cU,0xf746ce76U,0x77afa1c5U,0x20756060U,
            0x85cbfe4eU,0x8ae88dd8U,0x7aaaf9b0U,0x4cf9aa7eU,0x1948c25cU,0x02fb8a8cU,
            0x01c36ae4U,0xd6ebe1f9U,0x90d4f869U,0xa65cdea0U,0x3f09252dU,0xc208e69fU,
            0xb74e6132U,0xce77e25bU,0x578fdfe3U,0x3ac372e6U
        }
    };
    memcpy(c->p, P0, sizeof(P0));
    memcpy(c->s, S0_INIT, sizeof(S0_INIT));
}

static inline int64_t joss_prim_bcrypt_verify(const char* hash, const char* password) {
    static const char bc_tbl[65] = "./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
    if (!hash || !password) return 0;
    if (strlen(hash) != 60) return 0;
    if (hash[0] != '$' || hash[1] != '2' ||
        (hash[2] != 'a' && hash[2] != 'b' && hash[2] != 'y') ||
        hash[3] != '$' || hash[6] != '$') {
        return 0;
    }
    if (hash[4] < '0' || hash[4] > '9' || hash[5] < '0' || hash[5] > '9') return 0;
    int cost = (hash[4] - '0') * 10 + (hash[5] - '0');
    if (cost < 4 || cost > 31) return 0;

    uint8_t salt[16];
    if (joss_prim_base64_decode_alphabet(hash + 7, 22, bc_tbl, salt, sizeof(salt)) != 16) {
        return 0;
    }

    size_t pw_len = strlen(password);
    if (pw_len > 72) pw_len = 72;
    uint8_t key[73];
    if (pw_len > 0) memcpy(key, password, pw_len);
    key[pw_len] = 0;
    size_t key_len = pw_len + 1;

    JossBlowfishCtx c;
    joss_bf_init(&c);
    joss_bf_expand_key_salt(&c, key, key_len, salt, 16);

    uint64_t rounds = 1ULL << (unsigned)cost;
    for (uint64_t i = 0; i < rounds; i++) {
        joss_bf_expand_key(&c, key, key_len);
        joss_bf_expand_key(&c, salt, 16);
    }

    uint32_t cdata[6] = {
        0x4f727068U, 0x65616e42U, 0x65686f6cU,
        0x64657253U, 0x63727944U, 0x6f756274U
    };
    for (int blk = 0; blk < 6; blk += 2) {
        for (int r = 0; r < 64; r++) {
            joss_bf_encrypt(&c, &cdata[blk], &cdata[blk + 1]);
        }
    }
    uint8_t out_bytes[24];
    for (int i = 0; i < 6; i++) {
        out_bytes[i * 4]     = (uint8_t)(cdata[i] >> 24);
        out_bytes[i * 4 + 1] = (uint8_t)(cdata[i] >> 16);
        out_bytes[i * 4 + 2] = (uint8_t)(cdata[i] >> 8);
        out_bytes[i * 4 + 3] = (uint8_t)(cdata[i]);
    }
    char computed_hash[36];
    size_t enc_len = joss_prim_base64_encode_alphabet(out_bytes, 23, bc_tbl, false, computed_hash);
    if (enc_len != 31) return 0;

    uint8_t diff = 0;
    const char* expected = hash + 29;
    for (int i = 0; i < 31; i++) {
        diff |= (uint8_t)(computed_hash[i] ^ expected[i]);
    }
    return diff == 0 ? 1 : 0;
}

#endif /* JOSS_CRYPTO_PRIMITIVES_INC */
