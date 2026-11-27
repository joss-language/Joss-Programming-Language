/* =========================================================================
 * Zip::extract — Delegated 100% to Rust (`zip_rt.rs`) when JOSS_HAS_RUST_RT
 * ========================================================================= */
#if defined(JOSS_HAS_RUST_RT)
extern int64_t joss_rs_zip_extract(const char* zip_path, const char* dest_dir);

int64_t fn_Zip_extract(const char* src_raw, const char* dest_raw) {
    const char* zip_path = unbox_web_cstr(src_raw);
    const char* dest_path = unbox_web_cstr(dest_raw);
    if (!zip_path || !*zip_path || !dest_path || !*dest_path) return 0;
    return joss_rs_zip_extract(zip_path, dest_path);
}
#else
static uint16_t joss_zip_u16(const uint8_t* p) {
    return (uint16_t)((uint16_t)p[0] | ((uint16_t)p[1] << 8));
}

static uint32_t joss_zip_u32(const uint8_t* p) {
    return (uint32_t)p[0] | ((uint32_t)p[1] << 8) | ((uint32_t)p[2] << 16) | ((uint32_t)p[3] << 24);
}

#if defined(JOSS_HAS_RUST_RT)
extern uint32_t joss_rs_crc32(const uint8_t* data, size_t len);
#endif

static uint32_t joss_zip_crc32(const uint8_t* data, size_t len) {
#if defined(JOSS_HAS_RUST_RT)
    return joss_rs_crc32(data, len);
#else
    uint32_t crc = 0xFFFFFFFFu;
    for (size_t i = 0; i < len; i++) {
        crc ^= (uint32_t)data[i];
        for (int b = 0; b < 8; b++) {
            uint32_t mask = (uint32_t)(-(int32_t)(crc & 1u));
            crc = (crc >> 1) ^ (0xEDB88320u & mask);
        }
    }
    return ~crc;
#endif
}

static int joss_zip_mkdir_all(const char* path) {
    if (!path || !*path) return 0;
    size_t len = strlen(path);
    char* tmp = (char*)malloc(len + 1);
    if (!tmp) return -1;
    memcpy(tmp, path, len + 1);
    for (size_t i = 1; i < len; i++) {
        if (tmp[i] == '/' || tmp[i] == '\\') {
            char saved = tmp[i];
            tmp[i] = '\0';
            if (tmp[0] != '\0' && !(i == 2 && tmp[1] == ':')) {
#if defined(_WIN32)
                _mkdir(tmp);
#else
                mkdir(tmp, 0755);
#endif
            }
            tmp[i] = saved;
        }
    }
#if defined(_WIN32)
    _mkdir(tmp);
#else
    mkdir(tmp, 0755);
#endif
    free(tmp);
    return 0;
}

static bool joss_zip_path_is_safe(const char* name, size_t name_len) {
    if (!name || name_len == 0) return false;
    for (size_t i = 0; i < name_len; i++) {
        if (name[i] == '\0') return false;
    }
    if (name[0] == '/' || name[0] == '\\') return false;
    if (name_len >= 2 && isalpha((unsigned char)name[0]) && name[1] == ':') return false;
    int depth = 0;
    size_t i = 0;
    while (i < name_len) {
        while (i < name_len && (name[i] == '/' || name[i] == '\\')) i++;
        if (i >= name_len) break;
        size_t start = i;
        while (i < name_len && name[i] != '/' && name[i] != '\\') i++;
        size_t seg_len = i - start;
        if (seg_len == 1 && name[start] == '.') {
            continue;
        } else if (seg_len == 2 && name[start] == '.' && name[start + 1] == '.') {
            depth--;
            if (depth < 0) return false;
        } else {
            depth++;
        }
    }
    return true;
}

typedef struct {
    const uint8_t* src;
    size_t src_len;
    size_t byte_pos;
    uint32_t bit_buf;
    int bit_cnt;
} JossInflateBits;

static int joss_inf_bits(JossInflateBits* bs, int n, uint32_t* out) {
    while (bs->bit_cnt < n) {
        if (bs->byte_pos >= bs->src_len) return -1;
        bs->bit_buf |= ((uint32_t)bs->src[bs->byte_pos++]) << bs->bit_cnt;
        bs->bit_cnt += 8;
    }
    *out = bs->bit_buf & ((1u << n) - 1u);
    bs->bit_buf >>= n;
    bs->bit_cnt -= n;
    return 0;
}

typedef struct {
    int count[16];
    int symbol[288];
} JossHuffTable;

static int joss_huff_build(JossHuffTable* tbl, const uint8_t* lengths, int num_syms) {
    int offs[16];
    memset(tbl->count, 0, sizeof(tbl->count));
    for (int i = 0; i < num_syms; i++) {
        if (lengths[i] > 15) return -1;
        tbl->count[lengths[i]]++;
    }
    if (tbl->count[0] == num_syms) return 0;
    int left = 1;
    for (int len = 1; len <= 15; len++) {
        left <<= 1;
        left -= tbl->count[len];
        if (left < 0) return -1;
    }
    offs[1] = 0;
    for (int len = 1; len < 15; len++) {
        offs[len + 1] = offs[len] + tbl->count[len];
    }
    for (int sym = 0; sym < num_syms; sym++) {
        if (lengths[sym] != 0) {
            tbl->symbol[offs[lengths[sym]]++] = sym;
        }
    }
    return 0;
}

static int joss_huff_decode(JossInflateBits* bs, const JossHuffTable* tbl, int* out_sym) {
    int code = 0;
    int first = 0;
    int index = 0;
    for (int len = 1; len <= 15; len++) {
        uint32_t bit = 0;
        if (joss_inf_bits(bs, 1, &bit) != 0) return -1;
        code |= (int)bit;
        int count = tbl->count[len];
        if (code - count < first) {
            *out_sym = tbl->symbol[index + (code - first)];
            return 0;
        }
        index += count;
        first += count;
        first <<= 1;
        code <<= 1;
    }
    return -1;
}

static int joss_zip_inflate(const uint8_t* src, size_t src_len,
                            uint8_t** out_buf, size_t* out_len, size_t max_out) {
    static const uint16_t len_base[29] = {
        3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31,
        35, 43, 51, 59, 67, 83, 99, 115, 131, 163, 195, 227, 258
    };
    static const uint8_t len_extra[29] = {
        0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2,
        3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 0
    };
    static const uint16_t dist_base[30] = {
        1, 2, 3, 4, 5, 7, 9, 13, 17, 25, 33, 49, 65, 97, 129, 193,
        257, 385, 513, 769, 1025, 1537, 2049, 3073, 4097, 6145, 8193, 12289, 16385, 24577
    };
    static const uint8_t dist_extra[30] = {
        0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6,
        7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13
    };
    static const uint8_t cl_order[19] = {
        16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15
    };

    JossInflateBits bs = { src, src_len, 0, 0, 0 };
    size_t cap = 4096;
    if (cap > max_out + 1) cap = max_out + 1;
    uint8_t* dst = (uint8_t*)malloc(cap > 0 ? cap : 1);
    if (!dst) return -1;
    size_t pos = 0;

    uint32_t bfinal = 0;
    do {
        uint32_t btype = 0;
        if (joss_inf_bits(&bs, 1, &bfinal) != 0 || joss_inf_bits(&bs, 2, &btype) != 0) {
            free(dst);
            return -1;
        }
        if (btype == 0) {
            /* Stored (uncompressed) block */
            bs.bit_buf = 0;
            bs.bit_cnt = 0;
            if (bs.byte_pos + 4 > bs.src_len) { free(dst); return -1; }
            uint16_t blen = joss_zip_u16(bs.src + bs.byte_pos);
            uint16_t nlen = joss_zip_u16(bs.src + bs.byte_pos + 2);
            bs.byte_pos += 4;
            if ((uint16_t)(~blen) != nlen || bs.byte_pos + blen > bs.src_len) {
                free(dst);
                return -1;
            }
            if (pos + blen > max_out) { free(dst); return -1; }
            if (pos + blen > cap) {
                while (cap < pos + blen) cap = cap * 2;
                uint8_t* nd = (uint8_t*)realloc(dst, cap);
                if (!nd) { free(dst); return -1; }
                dst = nd;
            }
            memcpy(dst + pos, bs.src + bs.byte_pos, blen);
            pos += blen;
            bs.byte_pos += blen;
        } else if (btype == 1 || btype == 2) {
            JossHuffTable lit_tbl, dist_tbl;
            uint8_t lengths[288 + 32];
            if (btype == 1) {
                for (int i = 0; i <= 143; i++) lengths[i] = 8;
                for (int i = 144; i <= 255; i++) lengths[i] = 9;
                for (int i = 256; i <= 279; i++) lengths[i] = 7;
                for (int i = 280; i <= 287; i++) lengths[i] = 8;
                if (joss_huff_build(&lit_tbl, lengths, 288) != 0) { free(dst); return -1; }
                for (int i = 0; i < 32; i++) lengths[i] = 5;
                if (joss_huff_build(&dist_tbl, lengths, 32) != 0) { free(dst); return -1; }
            } else {
                uint32_t hlit = 0, hdist = 0, hclen = 0;
                if (joss_inf_bits(&bs, 5, &hlit) != 0 ||
                    joss_inf_bits(&bs, 5, &hdist) != 0 ||
                    joss_inf_bits(&bs, 4, &hclen) != 0) {
                    free(dst);
                    return -1;
                }
                int nlit = (int)hlit + 257;
                int ndist = (int)hdist + 1;
                int nclen = (int)hclen + 4;
                if (nlit > 286 || ndist > 30) { free(dst); return -1; }
                uint8_t cl_lens[19];
                memset(cl_lens, 0, sizeof(cl_lens));
                for (int i = 0; i < nclen; i++) {
                    uint32_t v = 0;
                    if (joss_inf_bits(&bs, 3, &v) != 0) { free(dst); return -1; }
                    cl_lens[cl_order[i]] = (uint8_t)v;
                }
                JossHuffTable cl_tbl;
                if (joss_huff_build(&cl_tbl, cl_lens, 19) != 0) { free(dst); return -1; }
                int idx = 0;
                int total = nlit + ndist;
                while (idx < total) {
                    int sym = 0;
                    if (joss_huff_decode(&bs, &cl_tbl, &sym) != 0) { free(dst); return -1; }
                    if (sym < 16) {
                        lengths[idx++] = (uint8_t)sym;
                    } else if (sym == 16) {
                        if (idx == 0) { free(dst); return -1; }
                        uint32_t rep = 0;
                        if (joss_inf_bits(&bs, 2, &rep) != 0) { free(dst); return -1; }
                        int cnt = (int)rep + 3;
                        uint8_t prev = lengths[idx - 1];
                        if (idx + cnt > total) { free(dst); return -1; }
                        while (cnt-- > 0) lengths[idx++] = prev;
                    } else if (sym == 17) {
                        uint32_t rep = 0;
                        if (joss_inf_bits(&bs, 3, &rep) != 0) { free(dst); return -1; }
                        int cnt = (int)rep + 3;
                        if (idx + cnt > total) { free(dst); return -1; }
                        while (cnt-- > 0) lengths[idx++] = 0;
                    } else if (sym == 18) {
                        uint32_t rep = 0;
                        if (joss_inf_bits(&bs, 7, &rep) != 0) { free(dst); return -1; }
                        int cnt = (int)rep + 11;
                        if (idx + cnt > total) { free(dst); return -1; }
                        while (cnt-- > 0) lengths[idx++] = 0;
                    } else {
                        free(dst);
                        return -1;
                    }
                }
                if (joss_huff_build(&lit_tbl, lengths, nlit) != 0 ||
                    joss_huff_build(&dist_tbl, lengths + nlit, ndist) != 0) {
                    free(dst);
                    return -1;
                }
            }

            for (;;) {
                int sym = 0;
                if (joss_huff_decode(&bs, &lit_tbl, &sym) != 0) { free(dst); return -1; }
                if (sym < 256) {
                    if (pos + 1 > max_out) { free(dst); return -1; }
                    if (pos + 1 > cap) {
                        cap = cap * 2;
                        uint8_t* nd = (uint8_t*)realloc(dst, cap);
                        if (!nd) { free(dst); return -1; }
                        dst = nd;
                    }
                    dst[pos++] = (uint8_t)sym;
                } else if (sym == 256) {
                    break;
                } else {
                    int lidx = sym - 257;
                    if (lidx < 0 || lidx >= 29) { free(dst); return -1; }
                    uint32_t lex = 0;
                    if (len_extra[lidx] > 0 && joss_inf_bits(&bs, len_extra[lidx], &lex) != 0) {
                        free(dst);
                        return -1;
                    }
                    size_t match_len = (size_t)len_base[lidx] + (size_t)lex;

                    int dsym = 0;
                    if (joss_huff_decode(&bs, &dist_tbl, &dsym) != 0 || dsym < 0 || dsym >= 30) {
                        free(dst);
                        return -1;
                    }
                    uint32_t dex = 0;
                    if (dist_extra[dsym] > 0 && joss_inf_bits(&bs, dist_extra[dsym], &dex) != 0) {
                        free(dst);
                        return -1;
                    }
                    size_t dist = (size_t)dist_base[dsym] + (size_t)dex;
                    if (dist == 0 || dist > pos) { free(dst); return -1; }
                    if (pos + match_len > max_out) { free(dst); return -1; }
                    if (pos + match_len > cap) {
                        while (cap < pos + match_len) cap = cap * 2;
                        uint8_t* nd = (uint8_t*)realloc(dst, cap);
                        if (!nd) { free(dst); return -1; }
                        dst = nd;
                    }
                    for (size_t k = 0; k < match_len; k++) {
                        dst[pos] = dst[pos - dist];
                        pos++;
                    }
                }
            }
        } else {
            free(dst);
            return -1;
        }
    } while (!bfinal);

    *out_buf = dst;
    *out_len = pos;
    return 0;
}

int64_t fn_Zip_extract(const char* src_raw, const char* dest_raw) {
    const char* zip_path = unbox_web_cstr(src_raw);
    const char* dest_path = unbox_web_cstr(dest_raw);
    if (!zip_path || !*zip_path || !dest_path || !*dest_path) {
        return 0;
    }

    uint8_t* buf = NULL;
    size_t fsz = 0;
    {
        JOSS_AUTO_FILE FILE* fp = fopen(zip_path, "rb");
        if (!fp) {
            return 0;
        }
        if (fseek(fp, 0, SEEK_END) != 0) return 0;
        long fsz_l = ftell(fp);
        if (fsz_l < 22 || (uint64_t)fsz_l > (512ULL << 20)) {
            return 0;
        }
        fsz = (size_t)fsz_l;
        rewind(fp);
        buf = (uint8_t*)malloc(fsz);
        if (!buf) return 0;
        if (fread(buf, 1, fsz, fp) != fsz) {
            free(buf);
            return 0;
        }
    }

    /* Locate End of Central Directory (EOCD) */
    size_t min_eocd = (fsz > 65557) ? (fsz - 65557) : 0;
    size_t eocd_pos = fsz;
    for (size_t i = fsz - 22; ; i--) {
        if (joss_zip_u32(buf + i) == 0x06054b50u) {
            eocd_pos = i;
            break;
        }
        if (i == min_eocd) break;
    }
    if (eocd_pos == fsz) {
        free(buf);
        return 0;
    }

    uint16_t cd_entries = joss_zip_u16(buf + eocd_pos + 10);
    uint32_t cd_size = joss_zip_u32(buf + eocd_pos + 12);
    uint32_t cd_offset = joss_zip_u32(buf + eocd_pos + 16);
    if ((uint64_t)cd_offset + (uint64_t)cd_size > (uint64_t)fsz) {
        free(buf);
        return 0;
    }

    const size_t max_single = 128ULL << 20;
    const size_t max_total = 512ULL << 20;
    size_t total_extracted = 0;

    size_t cd_pos = (size_t)cd_offset;
    for (uint16_t e = 0; e < cd_entries; e++) {
        if (cd_pos + 46 > fsz || joss_zip_u32(buf + cd_pos) != 0x02014b50u) {
            free(buf);
            return 0;
        }
        uint16_t ver_made = joss_zip_u16(buf + cd_pos + 4);
        uint16_t method = joss_zip_u16(buf + cd_pos + 10);
        uint32_t expected_crc = joss_zip_u32(buf + cd_pos + 16);
        uint32_t comp_size = joss_zip_u32(buf + cd_pos + 20);
        uint32_t uncomp_size = joss_zip_u32(buf + cd_pos + 24);
        uint16_t name_len = joss_zip_u16(buf + cd_pos + 28);
        uint16_t extra_len = joss_zip_u16(buf + cd_pos + 30);
        uint16_t comment_len = joss_zip_u16(buf + cd_pos + 32);
        uint32_t ext_attr = joss_zip_u32(buf + cd_pos + 38);
        uint32_t local_off = joss_zip_u32(buf + cd_pos + 42);

        if (cd_pos + 46 + name_len + extra_len + comment_len > fsz) {
            free(buf);
            return 0;
        }
        char* entry_name = (char*)malloc((size_t)name_len + 1);
        if (!entry_name) { free(buf); return 0; }
        memcpy(entry_name, buf + cd_pos + 46, name_len);
        entry_name[name_len] = '\0';
        cd_pos += 46 + (size_t)name_len + (size_t)extra_len + (size_t)comment_len;

        /* Reject Unix symlinks */
        if (((ver_made >> 8) == 3) && (((ext_attr >> 16) & 0170000u) == 0120000u)) {
            free(entry_name);
            free(buf);
            return 0;
        }

        /* Prevent Zip Slip */
        if (!joss_zip_path_is_safe(entry_name, name_len)) {
            free(entry_name);
            free(buf);
            return 0;
        }

        /* Build destination full path */
        size_t dlen = strlen(dest_path);
        while (dlen > 1 && (dest_path[dlen - 1] == '/' || dest_path[dlen - 1] == '\\')) dlen--;
        size_t full_len = dlen + 1 + (size_t)name_len;
        char* full_path = (char*)malloc(full_len + 1);
        if (!full_path) { free(entry_name); free(buf); return 0; }
        memcpy(full_path, dest_path, dlen);
        full_path[dlen] = '/';
        memcpy(full_path + dlen + 1, entry_name, name_len);
        full_path[full_len] = '\0';
        for (size_t k = dlen + 1; k < full_len; k++) {
            if (full_path[k] == '\\') full_path[k] = '/';
        }

        bool is_dir = (name_len > 0 && (entry_name[name_len - 1] == '/' || entry_name[name_len - 1] == '\\'));
        if (!is_dir && ((ver_made >> 8) == 3) && (((ext_attr >> 16) & 0170000u) == 0040000u)) {
            is_dir = true;
        }
        if (is_dir) {
            joss_zip_mkdir_all(full_path);
            free(full_path);
            free(entry_name);
            continue;
        }

        /* Ensure parent directory exists */
        char* last_slash = strrchr(full_path, '/');
        if (last_slash && last_slash > full_path) {
            *last_slash = '\0';
            joss_zip_mkdir_all(full_path);
            *last_slash = '/';
        }

        /* Parse Local File Header */
        if ((uint64_t)local_off + 30 > (uint64_t)fsz || joss_zip_u32(buf + local_off) != 0x04034b50u) {
            free(full_path);
            free(entry_name);
            free(buf);
            return 0;
        }
        uint16_t lh_name_len = joss_zip_u16(buf + local_off + 26);
        uint16_t lh_extra_len = joss_zip_u16(buf + local_off + 28);
        uint64_t data_off = (uint64_t)local_off + 30 + (uint64_t)lh_name_len + (uint64_t)lh_extra_len;
        if (data_off + (uint64_t)comp_size > (uint64_t)fsz) {
            free(full_path);
            free(entry_name);
            free(buf);
            return 0;
        }

        size_t remaining = (total_extracted < max_total) ? (max_total - total_extracted) : 0;
        size_t limit = (remaining < max_single) ? remaining : max_single;

        const uint8_t* out_data = NULL;
        uint8_t* inflated = NULL;
        size_t out_size = 0;

        if (method == 0) {
            if ((size_t)comp_size > limit || comp_size != uncomp_size) {
                free(full_path);
                free(entry_name);
                free(buf);
                return 0;
            }
            out_data = buf + data_off;
            out_size = (size_t)comp_size;
        } else if (method == 8) {
            if (joss_zip_inflate(buf + data_off, (size_t)comp_size, &inflated, &out_size, limit) != 0) {
                free(full_path);
                free(entry_name);
                free(buf);
                return 0;
            }
            out_data = inflated;
        } else {
            free(full_path);
            free(entry_name);
            free(buf);
            return 0;
        }

        uint32_t actual_crc = joss_zip_crc32(out_data, out_size);
        if (actual_crc != expected_crc) {
            free(inflated);
            free(full_path);
            free(entry_name);
            free(buf);
            return 0;
        }

        {
            JOSS_AUTO_FILE FILE* out_fp = fopen(full_path, "wb");
            if (!out_fp) {
                free(inflated);
                free(full_path);
                free(entry_name);
                free(buf);
                return 0;
            }
            if (out_size > 0 && fwrite(out_data, 1, out_size, out_fp) != out_size) {
                joss_auto_file_cleanup(&out_fp);
                remove(full_path);
                free(inflated);
                free(full_path);
                free(entry_name);
                free(buf);
                return 0;
            }
        }
        total_extracted += out_size;
        free(inflated);
        free(full_path);
        free(entry_name);
    }

    free(buf);
    return 1;
}
#endif /* JOSS_HAS_RUST_RT */





