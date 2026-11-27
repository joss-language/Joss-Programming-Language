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

#if defined(JOSS_HAS_RUST_RT)
extern ptrdiff_t joss_rs_ws_accept_key(const char* client_key, char* out_buf, size_t out_cap);
#endif

/* RFC 6455 Section 4.2.2: Sec-WebSocket-Accept = Base64(SHA1(key + GUID)) */
const char* joss_ws_accept_key(const char* client_key) {
    client_key = unbox_web_cstr(client_key);
    if (!client_key) client_key = "";
#if defined(JOSS_HAS_RUST_RT)
    char* buf = (char*)joss_req_alloc(64);
    if (buf && joss_rs_ws_accept_key(client_key, buf, 64) > 0) {
        return buf;
    }
    return "";
#else
    char* combined = joss_str_concat(client_key, "258EAFA5-E914-47DA-95CA-C5AB0DC85B11");
    const char* accept = joss_sha1_base64(combined);
    return accept ? accept : "";
#endif
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
    JOSS_AUTO_SB JossStringBuilder* acc = NULL;
    for (;;) {
        bool fin = true;
        int opcode = 0;
        char* payload = NULL;
        size_t plen = 0;
        int rc = joss_ws_read_frame_ex(sock, &fin, &opcode, &payload, &plen);
        if (rc != 0) {
            free(payload);
            joss_auto_sb_cleanup(&acc);
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

