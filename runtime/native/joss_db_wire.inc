/* =========================================================================
 * Joss Native C11 Pure-TCP Database Wire Clients (Zero External DLLs)
 *   - MySQL / MariaDB Wire Protocol v10 (mysql_native_password SHA-1 + COM_QUERY)
 *   - PostgreSQL Wire Protocol v3.0 (Startup + Cleartext Auth + Simple Query)
 * ========================================================================= */
#ifndef JOSS_DB_WIRE_INC
#define JOSS_DB_WIRE_INC

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

/* --- SQL Binding Substitution with Safe Escaping --- */
static char* joss_wire_bind_sql(const char* sql, void* bindings) {
    if (!sql) return NULL;
    int64_t nbind = bindings ? joss_arr_len(bindings) : 0;
    JossStringBuilder* sb = joss_sb_new();
    int64_t bidx = 0;
    for (const char* p = sql; *p; p++) {
        if (*p == '?' && bidx < nbind) {
            const char* val = joss_arr_get_str(bindings, bidx++);
            if (!val) {
                joss_sb_append(sb, "NULL");
            } else {
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
    if (fd == JOSS_WIRE_INVALID_SOCK) return JOSS_WIRE_INVALID_SOCK;

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
        size_t p_off = 1;
        while (p_off < alen && auth_resp[p_off] != 0) p_off++;
        p_off++;
        uint8_t sw_scramble[20];
        memset(sw_scramble, 0, sizeof(sw_scramble));
        if (p_off + 20 <= alen) memcpy(sw_scramble, auth_resp + p_off, 20);
        else if (p_off < alen) memcpy(sw_scramble, auth_resp + p_off, alen - p_off);
        free(auth_resp);
        uint8_t sw_out[20];
        size_t sw_len = 0;
        if (pass && *pass) {
            uint8_t s1[20], s2[20], buf40[40], s3[20];
            joss_wire_sha1((const uint8_t*)pass, strlen(pass), s1);
            joss_wire_sha1(s1, 20, s2);
            memcpy(buf40, sw_scramble, 20);
            memcpy(buf40 + 20, s2, 20);
            joss_wire_sha1(buf40, 40, s3);
            for (int i = 0; i < 20; i++) sw_out[i] = s1[i] ^ s3[i];
            sw_len = 20;
        }
        (void)mysql_wire_write_pkt(fd, (uint8_t)(aseq + 1), sw_out, sw_len);
        auth_resp = mysql_wire_read_pkt(fd, &alen, &aseq);
    }
    if (!auth_resp || alen == 0 || auth_resp[0] != 0x00) {
        fprintf(stderr, "[MySQL Wire] auth failed (host=%s, code=0x%02x)\n", host, (auth_resp && alen > 0) ? auth_resp[0] : 0xFF);
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
    joss_wire_sock_t fd = joss_wire_get_mysql();
    if (fd == JOSS_WIRE_INVALID_SOCK || !sql) return rows;
    char* bound = joss_wire_bind_sql(sql, bindings);
    if (!bound) return rows;
    size_t qlen = strlen(bound);
    uint8_t* cmd = (uint8_t*)malloc(qlen + 1);
    if (!cmd) { free(bound); return rows; }
    cmd[0] = 0x03; /* COM_QUERY */
    memcpy(cmd + 1, bound, qlen);
    free(bound);
    if (mysql_wire_write_pkt(fd, 0, cmd, qlen + 1) != 0) {
        free(cmd);
        joss_wire_close(g_wire_mysql_fd);
        g_wire_mysql_fd = JOSS_WIRE_INVALID_SOCK;
        return rows;
    }
    free(cmd);

    size_t plen = 0;
    uint8_t* pkt = mysql_wire_read_pkt(fd, &plen, NULL);
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
    joss_wire_sock_t fd = joss_wire_get_mysql();
    if (fd == JOSS_WIRE_INVALID_SOCK || !sql) return 0;
    char* bound = joss_wire_bind_sql(sql, bindings);
    if (!bound) return 0;
    size_t qlen = strlen(bound);
    uint8_t* cmd = (uint8_t*)malloc(qlen + 1);
    if (!cmd) { free(bound); return 0; }
    cmd[0] = 0x03;
    memcpy(cmd + 1, bound, qlen);
    free(bound);
    if (mysql_wire_write_pkt(fd, 0, cmd, qlen + 1) != 0) {
        free(cmd);
        joss_wire_close(g_wire_mysql_fd);
        g_wire_mysql_fd = JOSS_WIRE_INVALID_SOCK;
        return 0;
    }
    free(cmd);
    size_t plen = 0;
    uint8_t* pkt = mysql_wire_read_pkt(fd, &plen, NULL);
    int64_t affected = 0;
    if (pkt && plen > 1 && pkt[0] == 0x00) {
        size_t c = 0;
        affected = (int64_t)mysql_wire_read_lenenc(pkt + 1, plen - 1, &c);
        if (affected == 0) affected = 1;
    }
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
    if (fd == JOSS_WIRE_INVALID_SOCK) return JOSS_WIRE_INVALID_SOCK;

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
    joss_wire_sock_t fd = joss_wire_get_pg();
    if (fd == JOSS_WIRE_INVALID_SOCK || !sql) return rows;
    char* bound = joss_wire_bind_sql(sql, bindings);
    if (!bound) return rows;
    size_t ql = strlen(bound);
    uint8_t* qm = (uint8_t*)malloc(5 + ql + 1);
    if (!qm) { free(bound); return rows; }
    qm[0] = 'Q';
    pg_write_be32(qm + 1, (uint32_t)(4 + ql + 1));
    memcpy(qm + 5, bound, ql + 1);
    free(bound);
    if (joss_wire_send_all(fd, qm, 5 + ql + 1) != 0) {
        free(qm); joss_wire_close(g_wire_pg_fd); g_wire_pg_fd = JOSS_WIRE_INVALID_SOCK; return rows;
    }
    free(qm);

    char** cols = NULL;
    int ncols = 0;
    while (1) {
        uint8_t hdr[5];
        if (joss_wire_recv_all(fd, hdr, 5) != 0) break;
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
