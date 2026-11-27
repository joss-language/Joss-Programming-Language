/* =========================================================================
 * Native Redis Host Service (fn_Redis_*) â€” Pure C11 RESP2 TCP Client
 * ========================================================================= */
int64_t fn_Redis_connect(const char* addr_raw, const char* pass_raw, int64_t db) {
    const char* addr = unbox_web_cstr(addr_raw);
    const char* pass = unbox_web_cstr(pass_raw);
    if (!addr || !*addr) return 0;
    snprintf(g_redis_manual_host, sizeof(g_redis_manual_host), "%s", addr);
    snprintf(g_redis_manual_port, sizeof(g_redis_manual_port), "6379");
    char* colon = strrchr(g_redis_manual_host, ':');
    if (colon && !strchr(g_redis_manual_host, ']')) {
        *colon = '\0';
        if (colon[1]) snprintf(g_redis_manual_port, sizeof(g_redis_manual_port), "%s", colon + 1);
    }
    snprintf(g_redis_manual_pass, sizeof(g_redis_manual_pass), "%s", (pass && *pass) ? pass : "");
    g_redis_manual_db = (db > 0) ? (int)db : 0;
    g_redis_manual_set = true;
    return 1;
}

int64_t fn_Redis_set(const char* key_raw, int64_t val_raw, int64_t ttl_sec) {
    const char* key = unbox_web_cstr(key_raw);
    const char* val = joss_strval(val_raw);
    if (!key || !*key) return 0;
    if (!val) val = "";
    int fd = joss_redis_connect_fd();
    if (fd < 0) return 0;
    int64_t ok = 0;
    if (joss_redis_auth_and_select(fd)) {
        char line[256];
        if (ttl_sec > 0) {
            char ttl_buf[32];
            snprintf(ttl_buf, sizeof(ttl_buf), "%lld", (long long)ttl_sec);
            const char* av[5] = {"SET", key, val, "EX", ttl_buf};
            size_t al[5] = {3, strlen(key), strlen(val), 2, strlen(ttl_buf)};
            if (joss_redis_send_cmd(fd, 5, av, al) && joss_redis_read_line(fd, line, sizeof(line)) && line[0] == '+') {
                ok = 1;
            }
        } else {
            const char* av[3] = {"SET", key, val};
            size_t al[3] = {3, strlen(key), strlen(val)};
            if (joss_redis_send_cmd(fd, 3, av, al) && joss_redis_read_line(fd, line, sizeof(line)) && line[0] == '+') {
                ok = 1;
            }
        }
    }
    joss_redis_close_fd(fd);
    return ok;
}

const char* fn_Redis_get(const char* key_raw) {
    const char* key = unbox_web_cstr(key_raw);
    if (!key || !*key) return NULL;
    int fd = joss_redis_connect_fd();
    if (fd < 0) return NULL;
    char* result = NULL;
    if (joss_redis_auth_and_select(fd)) {
        const char* av[2] = {"GET", key};
        size_t al[2] = {3, strlen(key)};
        char line[256];
        if (joss_redis_send_cmd(fd, 2, av, al) && joss_redis_read_line(fd, line, sizeof(line))) {
            if (line[0] == '$') {
                long blen = strtol(line + 1, NULL, 10);
                if (blen >= 0 && blen < (64L << 20)) {
                    result = (char*)joss_req_alloc((size_t)blen + 1U);
                    if (result) {
                        size_t got = 0;
                        while (got < (size_t)blen) {
#if defined(_WIN32)
                            int r = recv((SOCKET)fd, result + got, (int)((size_t)blen - got), 0);
#else
                            ssize_t r = recv(fd, result + got, (size_t)blen - got, 0);
#endif
                            if (r <= 0) { result = NULL; break; }
                            got += (size_t)r;
                        }
                        if (result) {
                            result[blen] = '\0';
                            char crlf[4];
                            (void)joss_redis_read_line(fd, crlf, sizeof(crlf));
                        }
                    }
                }
            }
        }
    }
    joss_redis_close_fd(fd);
    return result;
}

int64_t fn_Redis_del(const char* key_raw) {
    const char* key = unbox_web_cstr(key_raw);
    if (!key || !*key) return 0;
    int fd = joss_redis_connect_fd();
    if (fd < 0) return 0;
    int64_t ok = 0;
    if (joss_redis_auth_and_select(fd)) {
        const char* av[2] = {"DEL", key};
        size_t al[2] = {3, strlen(key)};
        char line[256];
        if (joss_redis_send_cmd(fd, 2, av, al) && joss_redis_read_line(fd, line, sizeof(line)) &&
            (line[0] == ':' || line[0] == '+')) {
            ok = 1;
        }
    }
    joss_redis_close_fd(fd);
    return ok;
}

int64_t fn_Redis_forget(const char* key_raw) {
    return fn_Redis_del(key_raw);
}

int64_t fn_Redis_has(const char* key_raw) {
    const char* key = unbox_web_cstr(key_raw);
    if (!key || !*key) return 0;
    int fd = joss_redis_connect_fd();
    if (fd < 0) return 0;
    int64_t exists = 0;
    if (joss_redis_auth_and_select(fd)) {
        const char* av[2] = {"EXISTS", key};
        size_t al[2] = {6, strlen(key)};
        char line[256];
        if (joss_redis_send_cmd(fd, 2, av, al) && joss_redis_read_line(fd, line, sizeof(line)) && line[0] == ':') {
            exists = (strtoll(line + 1, NULL, 10) > 0) ? 1 : 0;
        }
    }
    joss_redis_close_fd(fd);
    return exists;
}

int64_t fn_Redis_ttl(const char* key_raw) {
    const char* key = unbox_web_cstr(key_raw);
    if (!key || !*key) return -1;
    int fd = joss_redis_connect_fd();
    if (fd < 0) return -1;
    int64_t ttl = -1;
    if (joss_redis_auth_and_select(fd)) {
        const char* av[2] = {"TTL", key};
        size_t al[2] = {3, strlen(key)};
        char line[256];
        if (joss_redis_send_cmd(fd, 2, av, al) && joss_redis_read_line(fd, line, sizeof(line)) && line[0] == ':') {
            ttl = (int64_t)strtoll(line + 1, NULL, 10);
        }
    }
    joss_redis_close_fd(fd);
    return ttl;
}

int64_t fn_Redis_flush(void) {
    int fd = joss_redis_connect_fd();
    if (fd < 0) return 0;
    int64_t ok = 0;
    if (joss_redis_auth_and_select(fd)) {
        const char* av[1] = {"FLUSHDB"};
        size_t al[1] = {7};
        char line[256];
        if (joss_redis_send_cmd(fd, 1, av, al) && joss_redis_read_line(fd, line, sizeof(line)) && line[0] == '+') {
            ok = 1;
        }
    }
    joss_redis_close_fd(fd);
    return ok;
}

