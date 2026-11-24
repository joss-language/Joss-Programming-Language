/* =========================================================================
 * Joss Native C11 Pure-TCP Database Wire Clients (Zero External DLLs)
 *   - MySQL / MariaDB Wire Protocol v10 (mysql_native_password SHA-1 + COM_QUERY)
 *   - PostgreSQL Wire Protocol v3.0 (Startup + Cleartext Auth + Simple Query)
 * ========================================================================= */
#ifndef JOSS_DB_WIRE_INC
#define JOSS_DB_WIRE_INC

#include "joss_crypto_primitives.inc"
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#if defined(_WIN32)
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <ws2tcpip.h>
typedef SOCKET joss_wire_sock_t;
#define JOSS_WIRE_INVALID_SOCK INVALID_SOCKET
static void joss_wire_close(joss_wire_sock_t s) { if (s != INVALID_SOCKET) closesocket(s); }
static void joss_wire_net_init(void) {
    static int inited = 0;
    if (!inited) {
        WSADATA wsa;
        (void)WSAStartup(MAKEWORD(2, 2), &wsa);
        inited = 1;
    }
}
#else
#include <unistd.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <netdb.h>
#include <arpa/inet.h>
typedef int joss_wire_sock_t;
#define JOSS_WIRE_INVALID_SOCK (-1)
static void joss_wire_close(joss_wire_sock_t s) { if (s >= 0) close(s); }
static void joss_wire_net_init(void) {}
#endif

static joss_wire_sock_t joss_wire_connect(const char* host, const char* port) {
    joss_wire_net_init();
    char host_buf[256];
    char port_buf[32];
    if (host && strchr(host, ':') != NULL) {
        const char* colon = strrchr(host, ':');
        size_t hlen = (size_t)(colon - host);
        if (hlen > 0 && hlen < sizeof(host_buf)) {
            memcpy(host_buf, host, hlen);
            host_buf[hlen] = '\0';
            host = host_buf;
            if (colon[1] != '\0') {
                snprintf(port_buf, sizeof(port_buf), "%s", colon + 1);
                port = port_buf;
            }
        }
    }
    struct addrinfo hints;
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;
    struct addrinfo* res = NULL;
    if (getaddrinfo(host, port, &hints, &res) != 0 || !res) return JOSS_WIRE_INVALID_SOCK;
    joss_wire_sock_t fd = JOSS_WIRE_INVALID_SOCK;
    for (struct addrinfo* p = res; p != NULL; p = p->ai_next) {
        fd = socket(p->ai_family, p->ai_socktype, p->ai_protocol);
        if (fd == JOSS_WIRE_INVALID_SOCK) continue;
        if (connect(fd, p->ai_addr, (int)p->ai_addrlen) == 0) break;
        joss_wire_close(fd);
        fd = JOSS_WIRE_INVALID_SOCK;
    }
    freeaddrinfo(res);
    return fd;
}

static int joss_wire_send_all(joss_wire_sock_t fd, const uint8_t* buf, size_t len) {
    size_t sent = 0;
    while (sent < len) {
        int n = (int)send(fd, (const char*)(buf + sent), (int)(len - sent), 0);
        if (n <= 0) return -1;
        sent += (size_t)n;
    }
    return 0;
}

static int joss_wire_recv_all(joss_wire_sock_t fd, uint8_t* buf, size_t len) {
    size_t got = 0;
    while (got < len) {
        int n = (int)recv(fd, (char*)(buf + got), (int)(len - got), 0);
        if (n <= 0) return -1;
        got += (size_t)n;
    }
    return 0;
}

/* --- Minimal SHA-1 (RFC 3174) for mysql_native_password --- */
static uint32_t joss_sha1_rol(uint32_t v, unsigned bits) {
    return (v << bits) | (v >> (32U - bits));
}

static void joss_wire_sha1(const uint8_t* data, size_t len, uint8_t out[20]) {
    uint32_t h0 = 0x67452301U, h1 = 0xEFCDAB89U, h2 = 0x98BADCFEU, h3 = 0x10325476U, h4 = 0xC3D2E1F0U;
    uint64_t bit_len = (uint64_t)len * 8ULL;
    size_t padded_len = ((len + 9 + 63) / 64) * 64;
    uint8_t* msg = (uint8_t*)calloc(1, padded_len);
    if (!msg) { memset(out, 0, 20); return; }
    if (len > 0) memcpy(msg, data, len);
    msg[len] = 0x80;
    for (int i = 0; i < 8; i++) msg[padded_len - 1 - i] = (uint8_t)(bit_len >> (i * 8));
    for (size_t off = 0; off < padded_len; off += 64) {
        uint32_t w[80];
        for (int i = 0; i < 16; i++) {
            w[i] = ((uint32_t)msg[off + i*4] << 24) | ((uint32_t)msg[off + i*4 + 1] << 16) |
                   ((uint32_t)msg[off + i*4 + 2] << 8) | (uint32_t)msg[off + i*4 + 3];
        }
        for (int i = 16; i < 80; i++) w[i] = joss_sha1_rol(w[i-3] ^ w[i-8] ^ w[i-14] ^ w[i-16], 1);
        uint32_t a = h0, b = h1, c = h2, d = h3, e = h4;
        for (int i = 0; i < 80; i++) {
            uint32_t f, k;
            if (i < 20)      { f = (b & c) | ((~b) & d); k = 0x5A827999U; }
            else if (i < 40) { f = b ^ c ^ d;            k = 0x6ED9EBA1U; }
            else if (i < 60) { f = (b & c) | (b & d) | (c & d); k = 0x8F1BBCDCU; }
            else             { f = b ^ c ^ d;            k = 0xCA62C1D6U; }
            uint32_t tmp = joss_sha1_rol(a, 5) + f + e + k + w[i];
            e = d; d = c; c = joss_sha1_rol(b, 30); b = a; a = tmp;
        }
        h0 += a; h1 += b; h2 += c; h3 += d; h4 += e;
    }
    free(msg);
    uint32_t hs[5] = {h0, h1, h2, h3, h4};
    for (int i = 0; i < 5; i++) {
        out[i*4]   = (uint8_t)(hs[i] >> 24);
        out[i*4+1] = (uint8_t)(hs[i] >> 16);
        out[i*4+2] = (uint8_t)(hs[i] >> 8);
        out[i*4+3] = (uint8_t)(hs[i]);
    }
}

/* --- Minimal SHA-256 (FIPS 180-4) for MySQL 8 caching_sha2_password --- */
static uint32_t joss_sha256_ror(uint32_t v, unsigned bits) {
    return (v >> bits) | (v << (32U - bits));
}

static void joss_wire_sha256(const uint8_t* data, size_t len, uint8_t out[32]) {
    static const uint32_t k[64] = {
        0x428a2f98U,0x71374491U,0xb5c0fbcfU,0xe9b5dba5U,0x3956c25bU,0x59f111f1U,0x923f82a4U,0xab1c5ed5U,
        0xd807aa98U,0x12835b01U,0x243185beU,0x550c7dc3U,0x72be5d74U,0x80deb1feU,0x9bdc06a7U,0xc19bf174U,
        0xe49b69c1U,0xefbe4786U,0x0fc19dc6U,0x240ca1ccU,0x2de92c6fU,0x4a7484aaU,0x5cb0a9dcU,0x76f988daU,
        0x983e5152U,0xa831c66dU,0xb00327c8U,0xbf597fc7U,0xc6e00bf3U,0xd5a79147U,0x06ca6351U,0x14292967U,
        0x27b70a85U,0x2e1b2138U,0x4d2c6dfcU,0x53380d13U,0x650a7354U,0x766a0abbU,0x81c2c92eU,0x92722c85U,
        0xa2bfe8a1U,0xa81a664bU,0xc24b8b70U,0xc76c51a3U,0xd192e819U,0xd6990624U,0xf40e3585U,0x106aa070U,
        0x19a4c116U,0x1e376c08U,0x2748774cU,0x34b0bcb5U,0x391c0cb3U,0x4ed8aa4aU,0x5b9cca4fU,0x682e6ff3U,
        0x748f82eeU,0x78a5636fU,0x84c87814U,0x8cc70208U,0x90befffaU,0xa4506cebU,0xbef9a3f7U,0xc67178f2U
    };
    uint32_t h[8] = {
        0x6a09e667U,0xbb67ae85U,0x3c6ef372U,0xa54ff53aU,
        0x510e527fU,0x9b05688cU,0x1f83d9abU,0x5be0cd19U
    };
    uint64_t bit_len = (uint64_t)len * 8ULL;
    size_t padded_len = ((len + 9 + 63) / 64) * 64;
    uint8_t* msg = (uint8_t*)calloc(1, padded_len);
    if (!msg) { memset(out, 0, 32); return; }
    if (len > 0) memcpy(msg, data, len);
    msg[len] = 0x80;
    for (int i = 0; i < 8; i++) msg[padded_len - 1 - i] = (uint8_t)(bit_len >> (i * 8));
    for (size_t off = 0; off < padded_len; off += 64) {
        uint32_t w[64];
        for (int i = 0; i < 16; i++) {
            w[i] = ((uint32_t)msg[off + i*4] << 24) | ((uint32_t)msg[off + i*4 + 1] << 16) |
                   ((uint32_t)msg[off + i*4 + 2] << 8) | (uint32_t)msg[off + i*4 + 3];
        }
        for (int i = 16; i < 64; i++) {
            uint32_t s0 = joss_sha256_ror(w[i-15], 7) ^ joss_sha256_ror(w[i-15], 18) ^ (w[i-15] >> 3);
            uint32_t s1 = joss_sha256_ror(w[i-2], 17) ^ joss_sha256_ror(w[i-2], 19) ^ (w[i-2] >> 10);
            w[i] = w[i-16] + s0 + w[i-7] + s1;
        }
        uint32_t a=h[0], b=h[1], c=h[2], d=h[3], e=h[4], f=h[5], g=h[6], hh=h[7];
        for (int i = 0; i < 64; i++) {
            uint32_t S1 = joss_sha256_ror(e, 6) ^ joss_sha256_ror(e, 11) ^ joss_sha256_ror(e, 25);
            uint32_t ch = (e & f) ^ ((~e) & g);
            uint32_t t1 = hh + S1 + ch + k[i] + w[i];
            uint32_t S0 = joss_sha256_ror(a, 2) ^ joss_sha256_ror(a, 13) ^ joss_sha256_ror(a, 22);
            uint32_t maj = (a & b) ^ (a & c) ^ (b & c);
            uint32_t t2 = S0 + maj;
            hh = g; g = f; f = e; e = d + t1; d = c; c = b; b = a; a = t1 + t2;
        }
        h[0]+=a; h[1]+=b; h[2]+=c; h[3]+=d; h[4]+=e; h[5]+=f; h[6]+=g; h[7]+=hh;
    }
    free(msg);
    for (int i = 0; i < 8; i++) {
        out[i*4]   = (uint8_t)(h[i] >> 24);
        out[i*4+1] = (uint8_t)(h[i] >> 16);
        out[i*4+2] = (uint8_t)(h[i] >> 8);
        out[i*4+3] = (uint8_t)(h[i]);
    }
}

/* --- SQL Binding Substitution with Safe Escaping --- */
static char* joss_wire_bind_sql(const char* sql, void* bindings) {
    if (!sql) return NULL;
    int64_t nbind = bindings ? joss_arr_len(bindings) : 0;
    JossStringBuilder* sb = joss_sb_new();
    int64_t bidx = 0;
    for (const char* p = sql; *p; p++) {
        if (*p == '?' && bidx < nbind) {
            int64_t raw_v = joss_arr_get_i64(bindings, bidx++);
            bool is_sql_null = (raw_v == 0);
            if (joss_is_boxed_ptr(raw_v)) {
                const JossBoxedValue* box = (const JossBoxedValue*)(uintptr_t)(uint64_t)raw_v;
                if ((uint32_t)(box->header & 0xFFFFFFFFULL) == JOSS_TAG_NULL) is_sql_null = true;
            }
            if (is_sql_null) {
                joss_sb_append(sb, "NULL");
            } else {
                const char* val = joss_strval(raw_v);
                if (!val) val = "";
                joss_buf_append_byte(sb, '\'');
                for (const char* q = val; *q; q++) {
                    if (*q == '\'') joss_sb_append(sb, "''");
                    else if (*q == '\\') joss_sb_append(sb, "\\\\");
                    else if (*q == '\n') joss_sb_append(sb, "\\n");
                    else if (*q == '\r') joss_sb_append(sb, "\\r");
                    else joss_buf_append_byte(sb, (uint8_t)*q);
                }
                joss_buf_append_byte(sb, '\'');
            }
        } else {
            joss_buf_append_byte(sb, (uint8_t)*p);
        }
    }
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}

/* =========================================================================
 * MySQL / MariaDB Pure-TCP Wire Client (Protocol v10)
 * ========================================================================= */
static joss_wire_sock_t g_wire_mysql_fd = JOSS_WIRE_INVALID_SOCK;

static uint8_t* mysql_wire_read_pkt(joss_wire_sock_t fd, size_t* out_len, uint8_t* out_seq) {
    uint8_t hdr[4];
    if (joss_wire_recv_all(fd, hdr, 4) != 0) return NULL;
    size_t len = (size_t)hdr[0] | ((size_t)hdr[1] << 8) | ((size_t)hdr[2] << 16);
    if (out_seq) *out_seq = hdr[3];
    if (out_len) *out_len = len;
    uint8_t* body = (uint8_t*)malloc(len + 1);
    if (!body) return NULL;
    if (len > 0 && joss_wire_recv_all(fd, body, len) != 0) {
        free(body);
        return NULL;
    }
    body[len] = 0;
    return body;
}

static int mysql_wire_write_pkt(joss_wire_sock_t fd, uint8_t seq, const uint8_t* body, size_t len) {
    uint8_t hdr[4];
    hdr[0] = (uint8_t)(len & 0xFF);
    hdr[1] = (uint8_t)((len >> 8) & 0xFF);
    hdr[2] = (uint8_t)((len >> 16) & 0xFF);
    hdr[3] = seq;
    if (joss_wire_send_all(fd, hdr, 4) != 0) return -1;
    if (len > 0 && joss_wire_send_all(fd, body, len) != 0) return -1;
    return 0;
}

static uint64_t mysql_wire_read_lenenc(const uint8_t* p, size_t avail, size_t* consumed) {
    if (avail == 0) { *consumed = 0; return 0; }
    uint8_t b = p[0];
    if (b < 0xFB) { *consumed = 1; return b; }
    if (b == 0xFC && avail >= 3) { *consumed = 3; return (uint64_t)p[1] | ((uint64_t)p[2] << 8); }
    if (b == 0xFD && avail >= 4) { *consumed = 4; return (uint64_t)p[1] | ((uint64_t)p[2] << 8) | ((uint64_t)p[3] << 16); }
    if (b == 0xFE && avail >= 9) {
        *consumed = 9;
        uint64_t v = 0;
        for (int i = 0; i < 8; i++) v |= ((uint64_t)p[1 + i] << (i * 8));
        return v;
    }
    *consumed = 1;
    return 0;
}

static joss_wire_sock_t joss_wire_get_mysql(void) {
    if (g_wire_mysql_fd != JOSS_WIRE_INVALID_SOCK) return g_wire_mysql_fd;
    const char* host = joss_env("DB_HOST", "127.0.0.1");
    if (!host || !*host) host = "127.0.0.1";
    const char* port = joss_env("DB_PORT", "3306");
    if (!port || !*port) port = "3306";
    const char* user = joss_env("DB_USERNAME", "");
    if (!user || !*user) user = joss_env("DB_USER", "root");
    const char* pass = joss_env("DB_PASSWORD", "");
    if (!pass || !*pass) pass = joss_env("DB_PASS", "");
    const char* db   = joss_env("DB_DATABASE", "");
    if (!db || !*db) db = joss_env("DB_NAME", "");

    joss_wire_sock_t fd = joss_wire_connect(host, port);
    if (fd == JOSS_WIRE_INVALID_SOCK) {
        fprintf(stderr, "[MySQL Wire] connect failed (host=%s, port=%s)\n", host, port);
        return JOSS_WIRE_INVALID_SOCK;
    }

    size_t hlen = 0;
    uint8_t seq = 0;
    uint8_t* hs = mysql_wire_read_pkt(fd, &hlen, &seq);
    if (!hs || hlen < 30 || hs[0] != 10) {
        free(hs);
        joss_wire_close(fd);
        return JOSS_WIRE_INVALID_SOCK;
    }
    /* Parse HandshakeV10 scramble (8 + 12 = 20 bytes) */
    size_t pos = 1;
    while (pos < hlen && hs[pos] != 0) pos++;
    pos++; /* skip NUL after server version */
    pos += 4; /* connection id */
    uint8_t scramble[20];
    memset(scramble, 0, sizeof(scramble));
    if (pos + 8 <= hlen) memcpy(scramble, hs + pos, 8);
    pos += 8 + 1; /* auth-plugin-data-part-1 + filler */
    pos += 2 + 1 + 2 + 2 + 1 + 10; /* caps_lo, charset, status, caps_hi, auth_len, reserved */
    if (pos + 12 <= hlen) memcpy(scramble + 8, hs + pos, 12);
    free(hs);

    /* Build HandshakeResponse41 */
    uint32_t caps = 0x00000001U | 0x00000200U | 0x00008000U | 0x00080000U;
    if (db && *db) caps |= 0x00000008U;
    uint8_t resp[1024];
    size_t rpos = 0;
    resp[rpos++] = (uint8_t)(caps & 0xFF);
    resp[rpos++] = (uint8_t)((caps >> 8) & 0xFF);
    resp[rpos++] = (uint8_t)((caps >> 16) & 0xFF);
    resp[rpos++] = (uint8_t)((caps >> 24) & 0xFF);
    /* max_packet_size = 16MB */
    resp[rpos++] = 0x00; resp[rpos++] = 0x00; resp[rpos++] = 0x00; resp[rpos++] = 0x01;
    resp[rpos++] = 45; /* utf8mb4_general_ci */
    memset(resp + rpos, 0, 23); rpos += 23;
    size_t ulen = strlen(user);
    memcpy(resp + rpos, user, ulen + 1); rpos += ulen + 1;
    if (pass && *pass) {
        uint8_t s1[20], s2[20], buf40[40], s3[20];
        joss_wire_sha1((const uint8_t*)pass, strlen(pass), s1);
        joss_wire_sha1(s1, 20, s2);
        memcpy(buf40, scramble, 20);
        memcpy(buf40 + 20, s2, 20);
        joss_wire_sha1(buf40, 40, s3);
        resp[rpos++] = 20;
        for (int i = 0; i < 20; i++) resp[rpos++] = s1[i] ^ s3[i];
    } else {
        resp[rpos++] = 0;
    }
    if (db && *db) {
        size_t dlen = strlen(db);
        memcpy(resp + rpos, db, dlen + 1); rpos += dlen + 1;
    }
    const char* plugin = "mysql_native_password";
    size_t plen = strlen(plugin);
    memcpy(resp + rpos, plugin, plen + 1); rpos += plen + 1;

    if (mysql_wire_write_pkt(fd, (uint8_t)(seq + 1), resp, rpos) != 0) {
        joss_wire_close(fd);
        return JOSS_WIRE_INVALID_SOCK;
    }
    size_t alen = 0;
    uint8_t aseq = 0;
    uint8_t* auth_resp = mysql_wire_read_pkt(fd, &alen, &aseq);
    if (auth_resp && alen > 1 && auth_resp[0] == 0xFE) {
        /* AuthSwitchRequest: plugin_name\0 + scramble */
        const char* sw_plugin = (const char*)(auth_resp + 1);
        int is_sha2 = (strcmp(sw_plugin, "caching_sha2_password") == 0);
        size_t p_off = 1;
        while (p_off < alen && auth_resp[p_off] != 0) p_off++;
        p_off++;
        uint8_t sw_scramble[20];
        memset(sw_scramble, 0, sizeof(sw_scramble));
        if (p_off + 20 <= alen) memcpy(sw_scramble, auth_resp + p_off, 20);
        else if (p_off < alen) memcpy(sw_scramble, auth_resp + p_off, alen - p_off);
        free(auth_resp);
        uint8_t sw_out[32];
        size_t sw_len = 0;
        if (pass && *pass) {
            if (is_sha2) {
                uint8_t s1[32], s2[32], buf52[52], s3[32];
                joss_wire_sha256((const uint8_t*)pass, strlen(pass), s1);
                joss_wire_sha256(s1, 32, s2);
                memcpy(buf52, s2, 32);
                memcpy(buf52 + 32, sw_scramble, 20);
                joss_wire_sha256(buf52, 52, s3);
                for (int i = 0; i < 32; i++) sw_out[i] = s1[i] ^ s3[i];
                sw_len = 32;
            } else {
                uint8_t s1[20], s2[20], buf40[40], s3[20];
                joss_wire_sha1((const uint8_t*)pass, strlen(pass), s1);
                joss_wire_sha1(s1, 20, s2);
                memcpy(buf40, sw_scramble, 20);
                memcpy(buf40 + 20, s2, 20);
                joss_wire_sha1(buf40, 40, s3);
                for (int i = 0; i < 20; i++) sw_out[i] = s1[i] ^ s3[i];
                sw_len = 20;
            }
        }
        (void)mysql_wire_write_pkt(fd, (uint8_t)(aseq + 1), sw_out, sw_len);
        auth_resp = mysql_wire_read_pkt(fd, &alen, &aseq);
        if (auth_resp && alen >= 2 && auth_resp[0] == 0x01 && auth_resp[1] == 0x03) {
            /* caching_sha2_password fast auth success -> read final OK packet */
            free(auth_resp);
            auth_resp = mysql_wire_read_pkt(fd, &alen, &aseq);
        }
    }
    if (!auth_resp || alen == 0 || auth_resp[0] != 0x00) {
        const char* errmsg = (auth_resp && alen > 9 && auth_resp[0] == 0xFF) ? (const char*)(auth_resp + 9) : "";
        fprintf(stderr, "[MySQL Wire] auth failed (host=%s, code=0x%02x, msg=%s)\n", host, (auth_resp && alen > 0) ? auth_resp[0] : 0xFF, errmsg);
        free(auth_resp);
        joss_wire_close(fd);
        return JOSS_WIRE_INVALID_SOCK;
    }
    free(auth_resp);
    g_wire_mysql_fd = fd;
    return g_wire_mysql_fd;
}

static void* mysql_wire_query(const char* sql, void* bindings) {
    void* rows = joss_arr_new(4);
    if (!sql) return rows;
    char* bound = joss_wire_bind_sql(sql, bindings);
    if (!bound) return rows;
    size_t qlen = strlen(bound);
    uint8_t* cmd = (uint8_t*)malloc(qlen + 1);
    if (!cmd) { free(bound); return rows; }
    cmd[0] = 0x03; /* COM_QUERY */
    memcpy(cmd + 1, bound, qlen);
    free(bound);

    uint8_t* pkt = NULL;
    size_t plen = 0;
    joss_wire_sock_t fd = JOSS_WIRE_INVALID_SOCK;
    for (int attempt = 0; attempt < 2; attempt++) {
        fd = joss_wire_get_mysql();
        if (fd == JOSS_WIRE_INVALID_SOCK) break;
        if (mysql_wire_write_pkt(fd, 0, cmd, qlen + 1) != 0) {
            joss_wire_close(g_wire_mysql_fd);
            g_wire_mysql_fd = JOSS_WIRE_INVALID_SOCK;
            continue;
        }
        pkt = mysql_wire_read_pkt(fd, &plen, NULL);
        if (!pkt && attempt == 0) {
            joss_wire_close(g_wire_mysql_fd);
            g_wire_mysql_fd = JOSS_WIRE_INVALID_SOCK;
            continue;
        }
        break;
    }
    free(cmd);
    if (!pkt || plen == 0 || pkt[0] == 0xFF || pkt[0] == 0x00) {
        free(pkt);
        return rows;
    }
    size_t c = 0;
    uint64_t ncols = mysql_wire_read_lenenc(pkt, plen, &c);
    free(pkt);
    if (ncols == 0 || ncols > 256) return rows;

    char** col_names = (char**)calloc((size_t)ncols, sizeof(char*));
    for (uint64_t i = 0; i < ncols; i++) {
        uint8_t* cp = mysql_wire_read_pkt(fd, &plen, NULL);
        if (!cp) break;
        /* ColumnDefinition41: catalog, schema, table, org_table, name */
        size_t pos = 0;
        for (int field = 0; field < 5 && pos < plen; field++) {
            size_t fc = 0;
            uint64_t flen = mysql_wire_read_lenenc(cp + pos, plen - pos, &fc);
            pos += fc;
            if (field == 4 && pos + flen <= plen) {
                col_names[i] = (char*)malloc((size_t)flen + 1);
                if (col_names[i]) {
                    memcpy(col_names[i], cp + pos, (size_t)flen);
                    col_names[i][flen] = '\0';
                }
            }
            pos += (size_t)flen;
        }
        free(cp);
    }
    /* Read EOF packet after column definitions */
    uint8_t* eof = mysql_wire_read_pkt(fd, &plen, NULL);
    free(eof);

    /* Read resultset rows until EOF (0xFE with len < 9) or ERR (0xFF) */
    while (1) {
        uint8_t* rp = mysql_wire_read_pkt(fd, &plen, NULL);
        if (!rp || plen == 0) { free(rp); break; }
        if ((rp[0] == 0xFE && plen < 9) || rp[0] == 0xFF) { free(rp); break; }
        void* row_map = joss_map_new();
        size_t pos = 0;
        for (uint64_t i = 0; i < ncols && pos < plen; i++) {
            const char* cname = col_names[i] ? col_names[i] : "";
            if (rp[pos] == 0xFB) {
                pos++;
                joss_map_set_str(row_map, cname, "");
            } else {
                size_t vc = 0;
                uint64_t vlen = mysql_wire_read_lenenc(rp + pos, plen - pos, &vc);
                pos += vc;
                if (pos + vlen <= plen) {
                    char* val = (char*)malloc((size_t)vlen + 1);
                    if (val) {
                        memcpy(val, rp + pos, (size_t)vlen);
                        val[vlen] = '\0';
                        joss_map_set_str(row_map, cname, val);
                    }
                    pos += (size_t)vlen;
                }
            }
        }
        joss_arr_push_i64(rows, (int64_t)(uintptr_t)row_map);
        free(rp);
    }
    for (uint64_t i = 0; i < ncols; i++) free(col_names[i]);
    free(col_names);
    return rows;
}

static int64_t mysql_wire_execute(const char* sql, void* bindings) {
    if (!sql) return 0;
    char* bound = joss_wire_bind_sql(sql, bindings);
    if (!bound) return 0;
    size_t qlen = strlen(bound);
    uint8_t* cmd = (uint8_t*)malloc(qlen + 1);
    if (!cmd) { free(bound); return 0; }
    cmd[0] = 0x03;
    memcpy(cmd + 1, bound, qlen);

    uint8_t* pkt = NULL;
    size_t plen = 0;
    for (int attempt = 0; attempt < 2; attempt++) {
        joss_wire_sock_t fd = joss_wire_get_mysql();
        if (fd == JOSS_WIRE_INVALID_SOCK) break;
        if (mysql_wire_write_pkt(fd, 0, cmd, qlen + 1) != 0) {
            joss_wire_close(g_wire_mysql_fd);
            g_wire_mysql_fd = JOSS_WIRE_INVALID_SOCK;
            continue;
        }
        pkt = mysql_wire_read_pkt(fd, &plen, NULL);
        if (!pkt && attempt == 0) {
            joss_wire_close(g_wire_mysql_fd);
            g_wire_mysql_fd = JOSS_WIRE_INVALID_SOCK;
            continue;
        }
        break;
    }
    free(cmd);
    int64_t affected = 0;
    if (pkt && plen > 1 && pkt[0] == 0x00) {
        size_t c = 0;
        affected = (int64_t)mysql_wire_read_lenenc(pkt + 1, plen - 1, &c);
        int64_t last_insert_id = 0;
        if (1 + c < plen) {
            size_t c2 = 0;
            last_insert_id = (int64_t)mysql_wire_read_lenenc(pkt + 1 + c, plen - 1 - c, &c2);
        }
        const char* p_sql = sql;
        while (*p_sql == ' ' || *p_sql == '\t' || *p_sql == '\r' || *p_sql == '\n') p_sql++;
        if (db_strncasecmp_ascii(p_sql, "INSERT", 6) == 0 && last_insert_id > 0) {
            affected = last_insert_id;
        } else if (affected == 0) {
            affected = 1;
        }
    } else if (pkt && plen > 3 && pkt[0] == 0xFF) {
        uint16_t err_code = (uint16_t)pkt[1] | ((uint16_t)pkt[2] << 8);
        size_t msg_off = (plen > 9 && pkt[3] == '#') ? 9 : 3;
        size_t msg_len = (plen > msg_off) ? (plen - msg_off) : 0;
        fprintf(stderr, "[GranDB MySQL Error %u] %.*s | SQL=%.240s\n", (unsigned)err_code, (int)msg_len, (const char*)(pkt + msg_off), bound);
        fflush(stderr);
    }
    free(bound);
    free(pkt);
    return affected;
}

/* =========================================================================
 * PostgreSQL Pure-TCP Wire Client (Protocol v3.0)
 * ========================================================================= */
static joss_wire_sock_t g_wire_pg_fd = JOSS_WIRE_INVALID_SOCK;

static uint32_t pg_read_be32(const uint8_t* p) {
    return ((uint32_t)p[0] << 24) | ((uint32_t)p[1] << 16) | ((uint32_t)p[2] << 8) | (uint32_t)p[3];
}
static void pg_write_be32(uint8_t* p, uint32_t v) {
    p[0] = (uint8_t)(v >> 24); p[1] = (uint8_t)(v >> 16); p[2] = (uint8_t)(v >> 8); p[3] = (uint8_t)v;
}

static void joss_wire_random_bytes(uint8_t* out, size_t len) {
    if (!out || len == 0) return;
#if defined(_WIN32)
    /* Use RtlGenRandom/BCrypt-free fallback or rand_s/time+pid mix if bcrypt not linked */
    for (size_t i = 0; i < len; i++) {
        out[i] = (uint8_t)((rand() ^ (int)(i * 31)) & 0xFF);
    }
#else
    FILE* f = fopen("/dev/urandom", "rb");
    if (f) {
        size_t rd = fread(out, 1, len, f);
        fclose(f);
        if (rd == len) return;
    }
    for (size_t i = 0; i < len; i++) {
        out[i] = (uint8_t)((rand() ^ (int)(i * 31)) & 0xFF);
    }
#endif
}

static void joss_wire_md5_hex(const uint8_t* data, size_t len, char out_hex[33]) {
    static const char hex[] = "0123456789abcdef";
    uint8_t dig[16];
    joss_prim_md5(data, len, dig);
    for (int i = 0; i < 16; i++) {
        out_hex[i * 2] = hex[(dig[i] >> 4) & 0xF];
        out_hex[i * 2 + 1] = hex[dig[i] & 0xF];
    }
    out_hex[32] = '\0';
}

static joss_wire_sock_t joss_wire_get_pg(void) {
    if (g_wire_pg_fd != JOSS_WIRE_INVALID_SOCK) return g_wire_pg_fd;
    const char* host = joss_env("DB_HOST", "127.0.0.1");
    if (!host || !*host) host = "127.0.0.1";
    const char* port = joss_env("DB_PORT", "5432");
    if (!port || !*port) port = "5432";
    const char* user = joss_env("DB_USERNAME", "");
    if (!user || !*user) user = joss_env("DB_USER", "postgres");
    const char* pass = joss_env("DB_PASSWORD", "");
    if (!pass || !*pass) pass = joss_env("DB_PASS", "");
    const char* db   = joss_env("DB_DATABASE", "");
    if (!db || !*db) db = joss_env("DB_NAME", "postgres");

    joss_wire_sock_t fd = joss_wire_connect(host, port);
    if (fd == JOSS_WIRE_INVALID_SOCK) {
        fprintf(stderr, "[MySQL Wire] connect failed (host=%s, port=%s)\n", host, port);
        return JOSS_WIRE_INVALID_SOCK;
    }

    uint8_t msg[512];
    size_t pos = 8;
    memcpy(msg + pos, "user", 5); pos += 5;
    size_t ul = strlen(user); memcpy(msg + pos, user, ul + 1); pos += ul + 1;
    memcpy(msg + pos, "database", 9); pos += 9;
    size_t dl = strlen(db); memcpy(msg + pos, db, dl + 1); pos += dl + 1;
    msg[pos++] = 0;
    pg_write_be32(msg, (uint32_t)pos);
    pg_write_be32(msg + 4, 196608U); /* 3.0 */
    if (joss_wire_send_all(fd, msg, pos) != 0) { joss_wire_close(fd); return JOSS_WIRE_INVALID_SOCK; }

    char scram_client_first_bare[128] = {0};
    char scram_expected_server_sig[64] = {0};
    while (1) {
        uint8_t hdr[5];
        if (joss_wire_recv_all(fd, hdr, 5) != 0) { joss_wire_close(fd); return JOSS_WIRE_INVALID_SOCK; }
        char tag = (char)hdr[0];
        uint32_t mlen = pg_read_be32(hdr + 1);
        if (mlen < 4) { joss_wire_close(fd); return JOSS_WIRE_INVALID_SOCK; }
        size_t blen = (size_t)(mlen - 4);
        uint8_t* body = blen > 0 ? (uint8_t*)malloc(blen) : NULL;
        if (blen > 0 && (!body || joss_wire_recv_all(fd, body, blen) != 0)) {
            free(body); joss_wire_close(fd); return JOSS_WIRE_INVALID_SOCK;
        }
        if (tag == 'R' && blen >= 4) {
            uint32_t auth_type = pg_read_be32(body);
            if (auth_type == 3) { /* CleartextPassword */
                size_t pl = strlen(pass);
                uint8_t* pm = (uint8_t*)malloc(5 + pl + 1);
                if (pm) {
                    pm[0] = 'p';
                    pg_write_be32(pm + 1, (uint32_t)(4 + pl + 1));
                    memcpy(pm + 5, pass, pl + 1);
                    (void)joss_wire_send_all(fd, pm, 5 + pl + 1);
                    free(pm);
                }
            } else if (auth_type == 5 && blen >= 8) { /* MD5Password */
                size_t pl = strlen(pass);
                size_t ul2 = strlen(user);
                char* pu = (char*)malloc(pl + ul2 + 1);
                if (pu) {
                    memcpy(pu, pass, pl);
                    memcpy(pu + pl, user, ul2);
                    pu[pl + ul2] = '\0';
                    char inner_hex[33];
                    joss_wire_md5_hex((const uint8_t*)pu, pl + ul2, inner_hex);
                    free(pu);
                    uint8_t outer_in[36];
                    memcpy(outer_in, inner_hex, 32);
                    memcpy(outer_in + 32, body + 4, 4);
                    char outer_hex[33];
                    joss_wire_md5_hex(outer_in, 36, outer_hex);
                    char md5_resp[37];
                    snprintf(md5_resp, sizeof(md5_resp), "md5%s", outer_hex);
                    uint8_t pm[5 + 36];
                    pm[0] = 'p';
                    pg_write_be32(pm + 1, 4 + 36);
                    memcpy(pm + 5, md5_resp, 36);
                    (void)joss_wire_send_all(fd, pm, 5 + 36);
                }
            } else if (auth_type == 10) { /* AuthenticationSASL */
                uint8_t rnd[18];
                joss_wire_random_bytes(rnd, sizeof(rnd));
                char cnonce[32];
                joss_prim_base64_encode(rnd, sizeof(rnd), cnonce);
                snprintf(scram_client_first_bare, sizeof(scram_client_first_bare), "n=,r=%s", cnonce);
                char client_first[140];
                snprintf(client_first, sizeof(client_first), "n,,%s", scram_client_first_bare);
                size_t cflen = strlen(client_first);
                const char* mech = "SCRAM-SHA-256";
                size_t mech_len = strlen(mech) + 1;
                size_t pay_len = mech_len + 4 + cflen;
                uint8_t* pm = (uint8_t*)malloc(5 + pay_len);
                if (pm) {
                    pm[0] = 'p';
                    pg_write_be32(pm + 1, (uint32_t)(4 + pay_len));
                    memcpy(pm + 5, mech, mech_len);
                    pg_write_be32(pm + 5 + mech_len, (uint32_t)cflen);
                    memcpy(pm + 5 + mech_len + 4, client_first, cflen);
                    (void)joss_wire_send_all(fd, pm, 5 + pay_len);
                    free(pm);
                }
            } else if (auth_type == 11 && blen > 4) { /* AuthenticationSASLContinue */
                char* sfirst = (char*)malloc(blen - 4 + 1);
                if (sfirst) {
                    memcpy(sfirst, body + 4, blen - 4);
                    sfirst[blen - 4] = '\0';
                    char snonce[128] = {0}, salt_b64[128] = {0};
                    int iter = 4096;
                    const char* rp = strstr(sfirst, "r=");
                    if (rp) { rp += 2; const char* c = strchr(rp, ','); size_t l = c ? (size_t)(c - rp) : strlen(rp); if (l >= sizeof(snonce)) l = sizeof(snonce) - 1; memcpy(snonce, rp, l); snonce[l] = '\0'; }
                    const char* sp = strstr(sfirst, "s=");
                    if (sp) { sp += 2; const char* c = strchr(sp, ','); size_t l = c ? (size_t)(c - sp) : strlen(sp); if (l >= sizeof(salt_b64)) l = sizeof(salt_b64) - 1; memcpy(salt_b64, sp, l); salt_b64[l] = '\0'; }
                    const char* ip = strstr(sfirst, "i=");
                    if (ip) iter = atoi(ip + 2);
                    if (iter <= 0) iter = 4096;
                    uint8_t salt_raw[64];
                    size_t salt_len = joss_prim_base64_decode(salt_b64, salt_raw, sizeof(salt_raw));
                    uint8_t salted_pw[32];
                    joss_prim_pbkdf2_sha256((const uint8_t*)pass, strlen(pass), salt_raw, salt_len, iter, salted_pw);
                    uint8_t client_key[32], stored_key[32], server_key[32];
                    joss_prim_hmac_sha256(salted_pw, 32, (const uint8_t*)"Client Key", 10, client_key);
                    joss_prim_sha256(client_key, 32, stored_key);
                    joss_prim_hmac_sha256(salted_pw, 32, (const uint8_t*)"Server Key", 10, server_key);
                    char cf_wo_p[192];
                    snprintf(cf_wo_p, sizeof(cf_wo_p), "c=biws,r=%s", snonce);
                    size_t am_len = strlen(scram_client_first_bare) + 1 + strlen(sfirst) + 1 + strlen(cf_wo_p);
                    char* auth_msg = (char*)malloc(am_len + 1);
                    if (auth_msg) {
                        snprintf(auth_msg, am_len + 1, "%s,%s,%s", scram_client_first_bare, sfirst, cf_wo_p);
                        uint8_t csig[32], cproof[32], ssig[32];
                        joss_prim_hmac_sha256(stored_key, 32, (const uint8_t*)auth_msg, am_len, csig);
                        for (int i = 0; i < 32; i++) cproof[i] = client_key[i] ^ csig[i];
                        joss_prim_hmac_sha256(server_key, 32, (const uint8_t*)auth_msg, am_len, ssig);
                        joss_prim_base64_encode(ssig, 32, scram_expected_server_sig);
                        char proof_b64[64];
                        joss_prim_base64_encode(cproof, 32, proof_b64);
                        char cfinal[320];
                        snprintf(cfinal, sizeof(cfinal), "%s,p=%s", cf_wo_p, proof_b64);
                        size_t cfl = strlen(cfinal);
                        uint8_t* pm = (uint8_t*)malloc(5 + cfl);
                        if (pm) {
                            pm[0] = 'p';
                            pg_write_be32(pm + 1, (uint32_t)(4 + cfl));
                            memcpy(pm + 5, cfinal, cfl);
                            (void)joss_wire_send_all(fd, pm, 5 + cfl);
                            free(pm);
                        }
                        free(auth_msg);
                    }
                    free(sfirst);
                }
            } else if (auth_type == 12 && blen > 4) { /* AuthenticationSASLFinal */
                size_t vlen = blen - 4;
                if (scram_expected_server_sig[0]) {
                    char vbuf[128];
                    size_t cpy = vlen < sizeof(vbuf) - 1 ? vlen : sizeof(vbuf) - 1;
                    memcpy(vbuf, body + 4, cpy);
                    vbuf[cpy] = '\0';
                    const char* vp = strstr(vbuf, "v=");
                    if (!vp || strncmp(vp + 2, scram_expected_server_sig, strlen(scram_expected_server_sig)) != 0) {
                        free(body); joss_wire_close(fd); return JOSS_WIRE_INVALID_SOCK;
                    }
                }
            }
        } else if (tag == 'Z') {
            free(body);
            g_wire_pg_fd = fd;
            return g_wire_pg_fd;
        } else if (tag == 'E') {
            free(body); joss_wire_close(fd); return JOSS_WIRE_INVALID_SOCK;
        }
        free(body);
    }
}

static void* pg_wire_query(const char* sql, void* bindings) {
    void* rows = joss_arr_new(4);
    if (!sql) return rows;
    char* bound = joss_wire_bind_sql(sql, bindings);
    if (!bound) return rows;
    size_t ql = strlen(bound);
    uint8_t* qm = (uint8_t*)malloc(5 + ql + 1);
    if (!qm) { free(bound); return rows; }
    qm[0] = 'Q';
    pg_write_be32(qm + 1, (uint32_t)(4 + ql + 1));
    memcpy(qm + 5, bound, ql + 1);
    free(bound);

    joss_wire_sock_t fd = JOSS_WIRE_INVALID_SOCK;
    uint8_t first_hdr[5];
    bool have_first = false;
    for (int attempt = 0; attempt < 2; attempt++) {
        fd = joss_wire_get_pg();
        if (fd == JOSS_WIRE_INVALID_SOCK) break;
        if (joss_wire_send_all(fd, qm, 5 + ql + 1) != 0 || joss_wire_recv_all(fd, first_hdr, 5) != 0) {
            joss_wire_close(g_wire_pg_fd);
            g_wire_pg_fd = JOSS_WIRE_INVALID_SOCK;
            continue;
        }
        have_first = true;
        break;
    }
    free(qm);
    if (!have_first) return rows;

    char** cols = NULL;
    int ncols = 0;
    bool use_first = true;
    while (1) {
        uint8_t hdr[5];
        if (use_first) {
            memcpy(hdr, first_hdr, 5);
            use_first = false;
        } else if (joss_wire_recv_all(fd, hdr, 5) != 0) {
            break;
        }
        char tag = (char)hdr[0];
        uint32_t mlen = pg_read_be32(hdr + 1);
        if (mlen < 4) break;
        size_t blen = (size_t)(mlen - 4);
        uint8_t* body = blen > 0 ? (uint8_t*)malloc(blen) : NULL;
        if (blen > 0 && (!body || joss_wire_recv_all(fd, body, blen) != 0)) { free(body); break; }
        if (tag == 'T' && blen >= 2) {
            ncols = (int)((body[0] << 8) | body[1]);
            cols = (char**)calloc((size_t)ncols, sizeof(char*));
            size_t pos = 2;
            for (int i = 0; i < ncols && pos < blen; i++) {
                const char* nm = (const char*)(body + pos);
                cols[i] = strdup(nm);
                pos += strlen(nm) + 1 + 18;
            }
        } else if (tag == 'D' && blen >= 2 && cols) {
            int fc = (int)((body[0] << 8) | body[1]);
            void* row_map = joss_map_new();
            size_t pos = 2;
            for (int i = 0; i < fc && i < ncols && pos + 4 <= blen; i++) {
                int32_t flen = (int32_t)pg_read_be32(body + pos);
                pos += 4;
                const char* cname = cols[i] ? cols[i] : "";
                if (flen < 0) {
                    joss_map_set_str(row_map, cname, "");
                } else if (pos + (size_t)flen <= blen) {
                    char* val = (char*)malloc((size_t)flen + 1);
                    if (val) { memcpy(val, body + pos, (size_t)flen); val[flen] = '\0'; joss_map_set_str(row_map, cname, val); }
                    pos += (size_t)flen;
                }
            }
            joss_arr_push_i64(rows, (int64_t)(uintptr_t)row_map);
        } else if (tag == 'Z') {
            free(body);
            break;
        }
        free(body);
    }
    if (cols) {
        for (int i = 0; i < ncols; i++) free(cols[i]);
        free(cols);
    }
    return rows;
}

static int64_t pg_wire_execute(const char* sql, void* bindings) {
    (void)pg_wire_query(sql, bindings);
    return 1;
}

#endif /* JOSS_DB_WIRE_INC */
