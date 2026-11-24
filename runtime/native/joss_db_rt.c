//go:build ignore
#if defined(__APPLE__) && !defined(_DARWIN_C_SOURCE)
#define _DARWIN_C_SOURCE
#include <strings.h>
#elif defined(__unix__)
#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif
#ifndef _POSIX_C_SOURCE
#define _POSIX_C_SOURCE 200809L
#endif
#include <strings.h>
#endif

#include "joss_rt.h"

#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#if defined(_WIN32)
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <windows.h>
#else
#include <sys/stat.h>
#endif

#if defined(__has_include)
#if !defined(JOSS_DB_DISABLE_SQLITE) && __has_include(<sqlite3.h>)
#include <sqlite3.h>
#define JOSS_HAS_SQLITE3 1
#endif
#if !defined(JOSS_DB_DISABLE_PG) && __has_include(<postgresql/libpq-fe.h>)
#include <postgresql/libpq-fe.h>
#define JOSS_HAS_LIBPQ 1
#elif !defined(JOSS_DB_DISABLE_PG) && __has_include(<libpq-fe.h>)
#include <libpq-fe.h>
#define JOSS_HAS_LIBPQ 1
#endif
#if !defined(JOSS_DB_DISABLE_MYSQL) && __has_include(<mysql/mysql.h>)
#include <mysql/mysql.h>
#define JOSS_HAS_MYSQL 1
#elif !defined(JOSS_DB_DISABLE_MYSQL) && __has_include(<mariadb/mysql.h>)
#include <mariadb/mysql.h>
#define JOSS_HAS_MYSQL 1
#endif
#if !defined(JOSS_DB_DISABLE_ODBC) && __has_include(<sql.h>) && __has_include(<sqlext.h>)
#include <sql.h>
#include <sqlext.h>
#define JOSS_HAS_ODBC 1
#endif
#endif

static int db_strncasecmp_ascii(const char* a, const char* b, size_t n) {
    for (size_t i = 0; i < n; i++) {
        unsigned char ca = (unsigned char)tolower((unsigned char)a[i]);
        unsigned char cb = (unsigned char)tolower((unsigned char)b[i]);
        if (ca != cb) return (int)ca - (int)cb;
        if (ca == '\0') return 0;
    }
    return 0;
}

extern const char* joss_strval(int64_t v);
extern int64_t     joss_file_exists(const char* path);

typedef enum {
    JOSS_DB_NONE     = 0,
    JOSS_DB_SQLITE   = 1,
    JOSS_DB_POSTGRES = 2,
    JOSS_DB_MYSQL    = 3,
    JOSS_DB_SQLSRV   = 4
} JossDBEngine;

extern const char* joss_env(const char* key, const char* defVal);

static JossDBEngine detect_db_engine(void) {
    const char* drv = joss_env("DB", "");
    if (!drv || !*drv) drv = joss_env("DB_MOTOR", "");
    if (!drv || !*drv) drv = joss_env("DB_CONNECTION", "");
    if (!drv || !*drv) drv = joss_env("DB_DRIVER", "");
    if (!drv || !*drv) drv = getenv("DB_CONNECTION");
    if (!drv || !*drv) drv = getenv("DB_DRIVER");
    if (drv && *drv) {
        if (strcasecmp(drv, "sqlite") == 0 || strcasecmp(drv, "sqlite3") == 0) return JOSS_DB_SQLITE;
        if (strcasecmp(drv, "pgsql") == 0 || strcasecmp(drv, "postgres") == 0 || strcasecmp(drv, "postgresql") == 0) return JOSS_DB_POSTGRES;
        if (strcasecmp(drv, "mysql") == 0 || strcasecmp(drv, "mariadb") == 0) return JOSS_DB_MYSQL;
        if (strcasecmp(drv, "sqlsrv") == 0 || strcasecmp(drv, "mssql") == 0 || strcasecmp(drv, "odbc") == 0) return JOSS_DB_SQLSRV;
    }
    const char* db_host = joss_env("DB_HOST", "");
    if (!db_host || !*db_host) db_host = getenv("DB_HOST");
    if (db_host && *db_host) {
        return JOSS_DB_MYSQL;
    }
    const char* db_path = joss_env("DB_DATABASE", "");
    if (!db_path || !*db_path) db_path = joss_env("DB_PATH", "");
    if (!db_path || !*db_path) db_path = getenv("DB_DATABASE");
    if ((db_path && *db_path) || joss_file_exists("database.sqlite")) {
        return JOSS_DB_SQLITE;
    }
    return JOSS_DB_NONE;
}

/* =========================================================================
 * 1. SQLite3 Native Driver
 * ========================================================================= */
#if defined(JOSS_HAS_SQLITE3)
static sqlite3* g_joss_sqlite = NULL;
static char     g_joss_sqlite_path[512] = {0};

static sqlite3* joss_get_sqlite(void) {
    const char* db_path = getenv("DB_DATABASE");
    if (!db_path || !*db_path) db_path = joss_env("DB_DATABASE", "");
    if (!db_path || !*db_path) db_path = joss_env("DB_PATH", "");
    if (!db_path || !*db_path) {
        const char* drv = joss_env("DB", "");
        if (!drv || !*drv) drv = joss_env("DB_CONNECTION", "");
        if (!drv || !*drv) drv = joss_env("DB_DRIVER", "");
        if ((drv && (strcasecmp(drv, "sqlite") == 0 || strcasecmp(drv, "sqlite3") == 0)) ||
            joss_file_exists("database.sqlite")) {
            db_path = "database.sqlite";
        } else {
            return g_joss_sqlite;
        }
    }
    if (g_joss_sqlite && strcmp(g_joss_sqlite_path, db_path) == 0) {
        return g_joss_sqlite;
    }
    if (g_joss_sqlite) {
        sqlite3_close(g_joss_sqlite);
        g_joss_sqlite = NULL;
        g_joss_sqlite_path[0] = '\0';
    }
    if (sqlite3_open(db_path, &g_joss_sqlite) != SQLITE_OK) {
        if (g_joss_sqlite) {
            sqlite3_close(g_joss_sqlite);
            g_joss_sqlite = NULL;
        }
        return NULL;
    }
    snprintf(g_joss_sqlite_path, sizeof(g_joss_sqlite_path), "%s", db_path);
    return g_joss_sqlite;
}

static void* sqlite_query(const char* sql, void* bindings) {
    void* rows = joss_arr_new(4);
    sqlite3* db = joss_get_sqlite();
    if (!db || !sql) return rows;
    sqlite3_stmt* stmt = NULL;
    if (sqlite3_prepare_v2(db, sql, -1, &stmt, NULL) != SQLITE_OK) return rows;
    if (bindings) {
        int64_t n = joss_arr_len(bindings);
        for (int64_t i = 0; i < n; i++) {
            const char* val = joss_strval(joss_arr_get_i64(bindings, i));
            sqlite3_bind_text(stmt, (int)(i + 1), val, -1, SQLITE_TRANSIENT);
        }
    }
    int cols = sqlite3_column_count(stmt);
    while (sqlite3_step(stmt) == SQLITE_ROW) {
        void* row = joss_map_new();
        for (int c = 0; c < cols; c++) {
            const char* cname = sqlite3_column_name(stmt, c);
            const unsigned char* cval = sqlite3_column_text(stmt, c);
            joss_map_set_str(row, cname ? cname : "", cval ? joss_req_strdup((const char*)cval) : "");
        }
        joss_arr_push_i64(rows, (int64_t)(uintptr_t)row);
    }
    sqlite3_finalize(stmt);
    return rows;
}

static int64_t sqlite_execute(const char* sql, void* bindings) {
    sqlite3* db = joss_get_sqlite();
    if (!db || !sql) return 0;
    sqlite3_stmt* stmt = NULL;
    if (sqlite3_prepare_v2(db, sql, -1, &stmt, NULL) != SQLITE_OK) return 0;
    if (bindings) {
        int64_t n = joss_arr_len(bindings);
        for (int64_t i = 0; i < n; i++) {
            const char* val = joss_strval(joss_arr_get_i64(bindings, i));
            sqlite3_bind_text(stmt, (int)(i + 1), val, -1, SQLITE_TRANSIENT);
        }
    }
    int rc = sqlite3_step(stmt);
    sqlite3_finalize(stmt);
    if (rc != SQLITE_DONE && rc != SQLITE_ROW) return 0;
    int changes = sqlite3_changes(db);
    if (changes == 0 && db_strncasecmp_ascii(sql, "INSERT", 6) == 0) {
        return (int64_t)sqlite3_last_insert_rowid(db);
    }
    return (int64_t)changes;
}
#else

static int db_strcasecmp_ascii(const char* a, const char* b) {
    if (a == b) return 0;
    if (!a) return -1;
    if (!b) return 1;
    while (*a || *b) {
        unsigned char ca = (unsigned char)tolower((unsigned char)*a++);
        unsigned char cb = (unsigned char)tolower((unsigned char)*b++);
        if (ca != cb) return (int)ca - (int)cb;
    }
    return 0;
}

static const char* db_strcasestr_ascii(const char* hay, const char* needle) {
    if (!hay || !needle) return NULL;
    size_t nlen = strlen(needle);
    if (nlen == 0) return hay;
    for (const char* p = hay; *p; p++) {
        if (db_strncasecmp_ascii(p, needle, nlen) == 0) {
            return p;
        }
    }
    return NULL;
}

static char* db_strdup_heap(const char* s) {
    if (!s) s = "";
    size_t n = strlen(s);
    char* out = (char*)malloc(n + 1U);
    if (!out) return NULL;
    memcpy(out, s, n + 1U);
    return out;
}

static const char* db_cstr(const char* s) {
    if (!s) return "";
    uintptr_t u = (uintptr_t)s;
    int64_t v = (int64_t)u;
    if (joss_is_boxed_ptr(v) || (uint64_t)u < JOSS_MIN_PTR_ADDR || ((uint64_t)u >> 47) != 0) {
        const char* r = joss_strval(v);
        return r ? r : "";
    }
    return s;
}

#define JOSS_MEM_SQLITE_MAX_TABLES 128
#define JOSS_MEM_SQLITE_MAX_COLS   128
#define JOSS_MEM_SQLITE_MAX_STMTS  512

typedef struct {
    char  col[128];
    char* val;
} JossMemSQLiteCell;

typedef struct {
    JossMemSQLiteCell* cells;
    int                cell_count;
} JossMemSQLiteRow;

typedef struct {
    bool              active;
    char              name[128];
    char              columns[JOSS_MEM_SQLITE_MAX_COLS][128];
    int               col_count;
    char              ddl[2048];
    JossMemSQLiteRow* rows;
    int               row_count;
    int               row_cap;
    int64_t           next_id;
} JossMemSQLiteTable;

static JossMemSQLiteTable g_sqlite_mem_tables[JOSS_MEM_SQLITE_MAX_TABLES];
static char               g_sqlite_mem_path[512] = {0};
static char*              g_sqlite_mem_stmts[JOSS_MEM_SQLITE_MAX_STMTS];
static int                g_sqlite_mem_stmt_count = 0;

static void db_mkdir_parents(const char* path) {
    if (!path || !*path) return;
    char tmp[512];
    snprintf(tmp, sizeof(tmp), "%s", path);
    for (char* p = tmp + 1; *p; p++) {
        if (*p == '/' || *p == '\\') {
            char saved = *p;
            *p = '\0';
            if (tmp[0] != '\0' && !(strlen(tmp) == 2 && tmp[1] == ':')) {
#if defined(_WIN32)
                (void)CreateDirectoryA(tmp, NULL);
#else
                (void)mkdir(tmp, 0755);
#endif
            }
            *p = saved;
        }
    }
}

static const char* sqlite_mem_resolve_path(void) {
    const char* db_path = getenv("DB_DATABASE");
    if (!db_path || !*db_path) db_path = joss_env("DB_DATABASE", "");
    if (!db_path || !*db_path) db_path = joss_env("DB_PATH", "");
    if (!db_path || !*db_path) {
        const char* drv = joss_env("DB", "");
        if (!drv || !*drv) drv = joss_env("DB_MOTOR", "");
        if (!drv || !*drv) drv = joss_env("DB_CONNECTION", "");
        if (!drv || !*drv) drv = joss_env("DB_DRIVER", "");
        if (!drv || !*drv) drv = getenv("DB_CONNECTION");
        if (!drv || !*drv) drv = getenv("DB_DRIVER");
        if ((drv && (db_strcasecmp_ascii(drv, "sqlite") == 0 || db_strcasecmp_ascii(drv, "sqlite3") == 0)) ||
            joss_file_exists("database.sqlite")) {
            db_path = "database.sqlite";
        } else {
            return NULL;
        }
    }
    return db_path;
}

static void sqlite_mem_free_table_rows(JossMemSQLiteTable* tbl) {
    if (!tbl) return;
    for (int r = 0; r < tbl->row_count; r++) {
        for (int c = 0; c < tbl->rows[r].cell_count; c++) {
            free(tbl->rows[r].cells[c].val);
        }
        free(tbl->rows[r].cells);
    }
    free(tbl->rows);
    tbl->rows = NULL;
    tbl->row_count = 0;
    tbl->row_cap = 0;
    tbl->next_id = 0;
}

static int64_t sqlite_mem_ensure_connected(void) {
    const char* db_path = sqlite_mem_resolve_path();
    if (!db_path || !*db_path) return 0;
    if (strcmp(db_path, ":memory:") != 0) {
        db_mkdir_parents(db_path);
        FILE* f = fopen(db_path, "rb");
        if (f) {
            fclose(f);
        } else {
            f = fopen(db_path, "ab");
            if (!f) return 0;
            fclose(f);
        }
    }
    if (strcmp(g_sqlite_mem_path, db_path) != 0) {
        for (int i = 0; i < JOSS_MEM_SQLITE_MAX_TABLES; i++) {
            sqlite_mem_free_table_rows(&g_sqlite_mem_tables[i]);
        }
        memset(g_sqlite_mem_tables, 0, sizeof(g_sqlite_mem_tables));
        for (int i = 0; i < g_sqlite_mem_stmt_count; i++) {
            free(g_sqlite_mem_stmts[i]);
            g_sqlite_mem_stmts[i] = NULL;
        }
        g_sqlite_mem_stmt_count = 0;
        snprintf(g_sqlite_mem_path, sizeof(g_sqlite_mem_path), "%s", db_path);
    }
    return 1;
}

static JossMemSQLiteTable* sqlite_mem_find_table(const char* name) {
    if (!name || !*name) return NULL;
    for (int i = 0; i < JOSS_MEM_SQLITE_MAX_TABLES; i++) {
        if (g_sqlite_mem_tables[i].active &&
            db_strcasecmp_ascii(g_sqlite_mem_tables[i].name, name) == 0) {
            return &g_sqlite_mem_tables[i];
        }
    }
    return NULL;
}

static JossMemSQLiteTable* sqlite_mem_alloc_table(const char* name) {
    if (!name || !*name) return NULL;
    JossMemSQLiteTable* existing = sqlite_mem_find_table(name);
    if (existing) return existing;
    for (int i = 0; i < JOSS_MEM_SQLITE_MAX_TABLES; i++) {
        if (!g_sqlite_mem_tables[i].active) {
            memset(&g_sqlite_mem_tables[i], 0, sizeof(JossMemSQLiteTable));
            g_sqlite_mem_tables[i].active = true;
            snprintf(g_sqlite_mem_tables[i].name, sizeof(g_sqlite_mem_tables[i].name), "%s", name);
            return &g_sqlite_mem_tables[i];
        }
    }
    return NULL;
}

static void sqlite_mem_add_column(JossMemSQLiteTable* tbl, const char* col) {
    if (!tbl || !col || !*col) return;
    for (int i = 0; i < tbl->col_count; i++) {
        if (db_strcasecmp_ascii(tbl->columns[i], col) == 0) {
            return;
        }
    }
    if (tbl->col_count < JOSS_MEM_SQLITE_MAX_COLS) {
        snprintf(tbl->columns[tbl->col_count], sizeof(tbl->columns[tbl->col_count]), "%s", col);
        tbl->col_count++;
    }
}

static const char* db_skip_ws(const char* p) {
    while (p && (*p == ' ' || *p == '\t' || *p == '\r' || *p == '\n')) p++;
    return p;
}

static bool db_is_ident_char(char c) {
    return isalnum((unsigned char)c) || c == '_' || c == '$';
}

static bool db_match_kw(const char** pp, const char* kw) {
    const char* p = db_skip_ws(*pp);
    size_t n = strlen(kw);
    if (db_strncasecmp_ascii(p, kw, n) != 0) return false;
    if (db_is_ident_char(p[n])) return false;
    *pp = p + n;
    return true;
}

static bool db_parse_ident(const char** pp, char* out, size_t out_cap) {
    if (!pp || !*pp || !out || out_cap == 0) return false;
    out[0] = '\0';
    const char* p = db_skip_ws(*pp);
    for (int part = 0; part < 2; part++) {
        size_t len = 0;
        if (*p == '"' || *p == '`' || *p == '\'' || *p == '[') {
            char close = (*p == '[') ? ']' : *p;
            p++;
            while (*p && *p != close) {
                if (len + 1U < out_cap) out[len++] = *p;
                p++;
            }
            if (*p == close) p++;
        } else {
            while (*p && db_is_ident_char(*p)) {
                if (len + 1U < out_cap) out[len++] = *p;
                p++;
            }
        }
        out[len] = '\0';
        const char* after = db_skip_ws(p);
        if (part == 0 && *after == '.') {
            p = db_skip_ws(after + 1);
            continue;
        }
        *pp = p;
        break;
    }
    return out[0] != '\0';
}

static bool db_is_table_constraint(const char* item) {
    const char* p = db_skip_ws(item);
    return db_match_kw(&p, "PRIMARY") ||
           db_match_kw(&p, "FOREIGN") ||
           db_match_kw(&p, "UNIQUE") ||
           db_match_kw(&p, "CHECK") ||
           db_match_kw(&p, "CONSTRAINT");
}

static void sqlite_mem_row_set_cell(JossMemSQLiteRow* row, const char* col, const char* val) {
    if (!row || !col || !*col) return;
    if (!val) val = "";
    for (int i = 0; i < row->cell_count; i++) {
        if (db_strcasecmp_ascii(row->cells[i].col, col) == 0) {
            free(row->cells[i].val);
            row->cells[i].val = db_strdup_heap(val);
            return;
        }
    }
    JossMemSQLiteCell* next = (JossMemSQLiteCell*)realloc(
        row->cells, (size_t)(row->cell_count + 1) * sizeof(JossMemSQLiteCell));
    if (!next) return;
    row->cells = next;
    snprintf(row->cells[row->cell_count].col, sizeof(row->cells[row->cell_count].col), "%s", col);
    row->cells[row->cell_count].val = db_strdup_heap(val);
    row->cell_count++;
}

static const char* sqlite_mem_row_get_cell(const JossMemSQLiteRow* row, const char* col) {
    if (!row || !col) return NULL;
    for (int i = 0; i < row->cell_count; i++) {
        if (db_strcasecmp_ascii(row->cells[i].col, col) == 0) {
            return row->cells[i].val ? row->cells[i].val : "";
        }
    }
    return NULL;
}

static bool sqlite_mem_parse_value(const char** pp, void* bindings, int* bind_idx, char* out, size_t out_cap) {
    if (!pp || !*pp || !out || out_cap == 0) return false;
    out[0] = '\0';
    const char* p = db_skip_ws(*pp);
    if (!*p) return false;
    if (*p == '?') {
        p++;
        int idx = bind_idx ? (*bind_idx)++ : 0;
        if (bindings && idx >= 0 && idx < joss_arr_len(bindings)) {
            const char* bval = joss_strval(joss_arr_get_i64(bindings, idx));
            snprintf(out, out_cap, "%s", bval ? bval : "");
        }
        *pp = p;
        return true;
    }
    if (*p == '\'' || *p == '"') {
        char quote = *p++;
        size_t len = 0;
        while (*p) {
            if (*p == quote) {
                if (p[1] == quote) {
                    if (len + 1U < out_cap) out[len++] = quote;
                    p += 2;
                    continue;
                }
                p++;
                break;
            }
            if (len + 1U < out_cap) out[len++] = *p;
            p++;
        }
        out[len] = '\0';
        *pp = p;
        return true;
    }
    size_t len = 0;
    while (*p && *p != ',' && *p != ')' && *p != ';' &&
           *p != ' ' && *p != '\t' && *p != '\r' && *p != '\n') {
        if (len + 1U < out_cap) out[len++] = *p;
        p++;
    }
    out[len] = '\0';
    if (db_strcasecmp_ascii(out, "NULL") == 0) {
        out[0] = '\0';
    }
    *pp = p;
    return len > 0;
}

static int sqlite_mem_count_qmarks_until(const char* start, const char* end) {
    int count = 0;
    bool in_sq = false;
    bool in_dq = false;
    for (const char* p = start; p && p < end && *p; p++) {
        if (*p == '\'' && !in_dq) in_sq = !in_sq;
        else if (*p == '"' && !in_sq) in_dq = !in_dq;
        else if (*p == '?' && !in_sq && !in_dq) count++;
    }
    return count;
}

static int sqlite_mem_parse_where_filters(
    const char* where_kw, void* bindings, int bind_idx,
    char filter_cols[][128], char filter_vals[][512], int max_filters
) {
    if (!where_kw) return 0;
    const char* p = db_skip_ws(where_kw);
    if (!db_match_kw(&p, "WHERE")) return 0;
    int nfilters = 0;
    while (*p && nfilters < max_filters) {
        p = db_skip_ws(p);
        if (!*p || *p == ';') break;
        const char* kw_check = p;
        if (db_match_kw(&kw_check, "ORDER") || db_match_kw(&kw_check, "LIMIT") ||
            db_match_kw(&kw_check, "OFFSET") || db_match_kw(&kw_check, "GROUP")) {
            break;
        }
        char col[128];
        if (!db_parse_ident(&p, col, sizeof(col))) break;
        p = db_skip_ws(p);
        if (*p == '=') {
            p++;
            char val[512];
            if (!sqlite_mem_parse_value(&p, bindings, &bind_idx, val, sizeof(val))) break;
            snprintf(filter_cols[nfilters], 128, "%s", col);
            snprintf(filter_vals[nfilters], 512, "%s", val);
            nfilters++;
        } else {
            break;
        }
        p = db_skip_ws(p);
        if (!db_match_kw(&p, "AND")) break;
    }
    return nfilters;
}

static bool sqlite_mem_row_matches(
    const JossMemSQLiteRow* row,
    char filter_cols[][128], char filter_vals[][512], int nfilters
) {
    for (int f = 0; f < nfilters; f++) {
        const char* cell_val = sqlite_mem_row_get_cell(row, filter_cols[f]);
        if (!cell_val || strcmp(cell_val, filter_vals[f]) != 0) {
            return false;
        }
    }
    return true;
}

static int64_t sqlite_mem_exec_single(const char* sql, void* bindings) {
    const char* p = db_skip_ws(sql);
    if (!p || !*p) return 1;

    if (db_match_kw(&p, "CREATE")) {
        (void)(db_match_kw(&p, "TEMP") || db_match_kw(&p, "TEMPORARY"));
        if (!db_match_kw(&p, "TABLE")) return 1;
        bool if_not_exists = false;
        if (db_match_kw(&p, "IF")) {
            (void)db_match_kw(&p, "NOT");
            (void)db_match_kw(&p, "EXISTS");
            if_not_exists = true;
        }
        char tbl_name[128];
        if (!db_parse_ident(&p, tbl_name, sizeof(tbl_name))) return 0;

        JossMemSQLiteTable* existing = sqlite_mem_find_table(tbl_name);
        if (existing && if_not_exists) {
            return 1;
        }
        JossMemSQLiteTable* tbl = existing ? existing : sqlite_mem_alloc_table(tbl_name);
        if (!tbl) return 0;
        if (existing) {
            sqlite_mem_free_table_rows(existing);
            tbl->col_count = 0;
        }
        snprintf(tbl->ddl, sizeof(tbl->ddl), "%s", sql);

        p = db_skip_ws(p);
        if (*p == '(') {
            p++;
            int depth = 1;
            bool in_sq = false;
            bool in_dq = false;
            const char* item_start = p;
            while (*p && depth > 0) {
                char c = *p;
                if (c == '\'' && !in_dq) {
                    in_sq = !in_sq;
                } else if (c == '"' && !in_sq) {
                    in_dq = !in_dq;
                } else if (!in_sq && !in_dq) {
                    if (c == '(') {
                        depth++;
                    } else if (c == ')') {
                        depth--;
                        if (depth == 0) {
                            size_t ilen = (size_t)(p - item_start);
                            if (ilen > 0) {
                                char item[512];
                                if (ilen >= sizeof(item)) ilen = sizeof(item) - 1;
                                memcpy(item, item_start, ilen);
                                item[ilen] = '\0';
                                if (!db_is_table_constraint(item)) {
                                    const char* ip = item;
                                    char col_name[128];
                                    if (db_parse_ident(&ip, col_name, sizeof(col_name))) {
                                        sqlite_mem_add_column(tbl, col_name);
                                    }
                                }
                            }
                            break;
                        }
                    } else if (c == ',' && depth == 1) {
                        size_t ilen = (size_t)(p - item_start);
                        if (ilen > 0) {
                            char item[512];
                            if (ilen >= sizeof(item)) ilen = sizeof(item) - 1;
                            memcpy(item, item_start, ilen);
                            item[ilen] = '\0';
                            if (!db_is_table_constraint(item)) {
                                const char* ip = item;
                                char col_name[128];
                                if (db_parse_ident(&ip, col_name, sizeof(col_name))) {
                                    sqlite_mem_add_column(tbl, col_name);
                                }
                            }
                        }
                        item_start = p + 1;
                    }
                }
                p++;
            }
        }
        return 1;
    }

    if (db_match_kw(&p, "ALTER")) {
        if (!db_match_kw(&p, "TABLE")) return 1;
        char tbl_name[128];
        if (!db_parse_ident(&p, tbl_name, sizeof(tbl_name))) return 0;

        if (db_match_kw(&p, "ADD")) {
            (void)db_match_kw(&p, "COLUMN");
            const char* check = p;
            if (db_is_table_constraint(check)) return 1;
            char col_name[128];
            if (!db_parse_ident(&p, col_name, sizeof(col_name))) return 0;
            JossMemSQLiteTable* tbl = sqlite_mem_find_table(tbl_name);
            if (!tbl) tbl = sqlite_mem_alloc_table(tbl_name);
            if (tbl) sqlite_mem_add_column(tbl, col_name);
            return 1;
        }
        if (db_match_kw(&p, "RENAME")) {
            if (db_match_kw(&p, "TO")) {
                char new_name[128];
                if (!db_parse_ident(&p, new_name, sizeof(new_name))) return 0;
                JossMemSQLiteTable* tbl = sqlite_mem_find_table(tbl_name);
                if (tbl) {
                    JossMemSQLiteTable* conflict = sqlite_mem_find_table(new_name);
                    if (conflict && conflict != tbl) {
                        conflict->active = false;
                        conflict->col_count = 0;
                        conflict->name[0] = '\0';
                    }
                    snprintf(tbl->name, sizeof(tbl->name), "%s", new_name);
                }
                return 1;
            }
            if (db_match_kw(&p, "COLUMN")) {
                char old_col[128], new_col[128];
                if (db_parse_ident(&p, old_col, sizeof(old_col)) &&
                    db_match_kw(&p, "TO") &&
                    db_parse_ident(&p, new_col, sizeof(new_col))) {
                    JossMemSQLiteTable* tbl = sqlite_mem_find_table(tbl_name);
                    if (tbl) {
                        for (int i = 0; i < tbl->col_count; i++) {
                            if (db_strcasecmp_ascii(tbl->columns[i], old_col) == 0) {
                                snprintf(tbl->columns[i], sizeof(tbl->columns[i]), "%s", new_col);
                                break;
                            }
                        }
                    }
                }
                return 1;
            }
        }
        if (db_match_kw(&p, "DROP")) {
            (void)db_match_kw(&p, "COLUMN");
            char col_name[128];
            if (db_parse_ident(&p, col_name, sizeof(col_name))) {
                JossMemSQLiteTable* tbl = sqlite_mem_find_table(tbl_name);
                if (tbl) {
                    for (int i = 0; i < tbl->col_count; i++) {
                        if (db_strcasecmp_ascii(tbl->columns[i], col_name) == 0) {
                            for (int j = i; j + 1 < tbl->col_count; j++) {
                                snprintf(tbl->columns[j], sizeof(tbl->columns[j]), "%s", tbl->columns[j + 1]);
                            }
                            tbl->col_count--;
                            break;
                        }
                    }
                }
            }
            return 1;
        }
        return 1;
    }

    if (db_match_kw(&p, "DROP")) {
        if (!db_match_kw(&p, "TABLE")) return 1;
        if (db_match_kw(&p, "IF")) {
            (void)db_match_kw(&p, "EXISTS");
        }
        char tbl_name[128];
        if (db_parse_ident(&p, tbl_name, sizeof(tbl_name))) {
            JossMemSQLiteTable* tbl = sqlite_mem_find_table(tbl_name);
            if (tbl) {
                sqlite_mem_free_table_rows(tbl);
                tbl->active = false;
                tbl->col_count = 0;
                tbl->name[0] = '\0';
            }
        }
        return 1;
    }

    if (db_match_kw(&p, "INSERT")) {
        (void)db_match_kw(&p, "OR");
        (void)db_match_kw(&p, "IGNORE");
        (void)db_match_kw(&p, "REPLACE");
        (void)db_match_kw(&p, "INTO");
        char tbl_name[128];
        if (!db_parse_ident(&p, tbl_name, sizeof(tbl_name))) return 0;
        JossMemSQLiteTable* tbl = sqlite_mem_find_table(tbl_name);
        if (!tbl) tbl = sqlite_mem_alloc_table(tbl_name);
        if (!tbl) return 0;

        char cols[JOSS_MEM_SQLITE_MAX_COLS][128];
        int ncol = 0;
        p = db_skip_ws(p);
        if (*p == '(') {
            p++;
            while (*p && *p != ')' && ncol < JOSS_MEM_SQLITE_MAX_COLS) {
                if (!db_parse_ident(&p, cols[ncol], sizeof(cols[ncol]))) break;
                ncol++;
                p = db_skip_ws(p);
                if (*p == ',') p++;
            }
            if (*p == ')') p++;
        } else {
            for (int i = 0; i < tbl->col_count && ncol < JOSS_MEM_SQLITE_MAX_COLS; i++) {
                snprintf(cols[ncol++], 128, "%s", tbl->columns[i]);
            }
        }

        if (!db_match_kw(&p, "VALUES")) return 1;
        int bind_idx = 0;
        int64_t inserted = 0;
        for (;;) {
            p = db_skip_ws(p);
            if (*p != '(') break;
            p++;
            JossMemSQLiteRow row;
            memset(&row, 0, sizeof(row));
            int vidx = 0;
            while (*p && *p != ')') {
                char val[512];
                if (!sqlite_mem_parse_value(&p, bindings, &bind_idx, val, sizeof(val))) break;
                if (vidx < ncol) {
                    sqlite_mem_row_set_cell(&row, cols[vidx], val);
                    sqlite_mem_add_column(tbl, cols[vidx]);
                }
                vidx++;
                p = db_skip_ws(p);
                if (*p == ',') p++;
            }
            if (*p == ')') p++;
            if (!sqlite_mem_row_get_cell(&row, "id")) {
                tbl->next_id++;
                char id_buf[32];
                snprintf(id_buf, sizeof(id_buf), "%lld", (long long)tbl->next_id);
                sqlite_mem_row_set_cell(&row, "id", id_buf);
            }
            if (tbl->row_count >= tbl->row_cap) {
                int new_cap = tbl->row_cap == 0 ? 16 : tbl->row_cap * 2;
                JossMemSQLiteRow* grown = (JossMemSQLiteRow*)realloc(
                    tbl->rows, (size_t)new_cap * sizeof(JossMemSQLiteRow));
                if (!grown) {
                    for (int c = 0; c < row.cell_count; c++) free(row.cells[c].val);
                    free(row.cells);
                    break;
                }
                tbl->rows = grown;
                tbl->row_cap = new_cap;
            }
            tbl->rows[tbl->row_count++] = row;
            inserted++;
            p = db_skip_ws(p);
            if (*p == ',') p++;
            else break;
        }
        return inserted > 0 ? inserted : 1;
    }

    if (db_match_kw(&p, "DELETE")) {
        (void)db_match_kw(&p, "FROM");
        char tbl_name[128];
        if (!db_parse_ident(&p, tbl_name, sizeof(tbl_name))) return 0;
        JossMemSQLiteTable* tbl = sqlite_mem_find_table(tbl_name);
        if (!tbl) return 0;
        const char* where_kw = db_strcasestr_ascii(p, "WHERE");
        char fcols[8][128];
        char fvals[8][512];
        int nfilters = 0;
        if (where_kw) {
            int bidx = sqlite_mem_count_qmarks_until(sql, where_kw);
            nfilters = sqlite_mem_parse_where_filters(where_kw, bindings, bidx, fcols, fvals, 8);
        }
        int kept = 0;
        int64_t deleted = 0;
        for (int r = 0; r < tbl->row_count; r++) {
            bool match = (nfilters == 0) || sqlite_mem_row_matches(&tbl->rows[r], fcols, fvals, nfilters);
            if (match) {
                for (int c = 0; c < tbl->rows[r].cell_count; c++) free(tbl->rows[r].cells[c].val);
                free(tbl->rows[r].cells);
                deleted++;
            } else {
                tbl->rows[kept++] = tbl->rows[r];
            }
        }
        tbl->row_count = kept;
        return deleted;
    }

    return 1;
}

static int64_t sqlite_mem_execute(const char* sql, void* bindings) {
    bindings = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)bindings);
    sql = db_cstr(sql);
    if (!sql || !*sql) return 0;
    if (!sqlite_mem_ensure_connected()) return 0;

    if (g_sqlite_mem_stmt_count < JOSS_MEM_SQLITE_MAX_STMTS) {
        char* copy = db_strdup_heap(sql);
        if (copy) g_sqlite_mem_stmts[g_sqlite_mem_stmt_count++] = copy;
    }

    bool in_sq = false;
    bool in_dq = false;
    const char* seg = sql;
    int64_t rc = 1;
    for (const char* p = sql; ; p++) {
        if (*p == '\'' && !in_dq) in_sq = !in_sq;
        else if (*p == '"' && !in_sq) in_dq = !in_dq;
        if (*p == '\0' || (*p == ';' && !in_sq && !in_dq)) {
            size_t len = (size_t)(p - seg);
            if (len > 0) {
                char* one = (char*)malloc(len + 1U);
                if (one) {
                    memcpy(one, seg, len);
                    one[len] = '\0';
                    rc = sqlite_mem_exec_single(one, bindings);
                    free(one);
                }
            }
            if (*p == '\0') break;
            seg = p + 1;
        }
    }
    return rc;
}

static bool sqlite_mem_extract_name_filter(const char* sql, void* bindings, char* out, size_t out_cap) {
    if (!sql || !out || out_cap == 0) return false;
    out[0] = '\0';
    for (const char* p = sql; *p; p++) {
        if (db_strncasecmp_ascii(p, "name", 4) == 0) {
            if (p > sql && db_is_ident_char(p[-1])) continue;
            if (db_is_ident_char(p[4])) continue;
            const char* q = db_skip_ws(p + 4);
            if (*q != '=') continue;
            q = db_skip_ws(q + 1);
            if (*q == '?') {
                if (bindings && joss_arr_len(bindings) > 0) {
                    const char* bval = joss_strval(joss_arr_get_i64(bindings, 0));
                    snprintf(out, out_cap, "%s", bval ? bval : "");
                    return out[0] != '\0';
                }
                return false;
            }
            if (*q == '\'' || *q == '"') {
                char quote = *q++;
                size_t len = 0;
                while (*q && *q != quote) {
                    if (len + 1U < out_cap) out[len++] = *q;
                    q++;
                }
                out[len] = '\0';
                return out[0] != '\0';
            }
        }
    }
    return false;
}

static void* sqlite_mem_query(const char* sql, void* bindings) {
    void* rows = joss_arr_new(4);
    bindings = (void*)(uintptr_t)joss_unbox_value((int64_t)(uintptr_t)bindings);
    sql = db_cstr(sql);
    if (!sql || !*sql) return rows;
    if (!sqlite_mem_ensure_connected()) return rows;

    if (db_strcasestr_ascii(sql, "sqlite_master") != NULL) {
        char target_name[128];
        bool has_target = sqlite_mem_extract_name_filter(sql, bindings, target_name, sizeof(target_name));
        bool want_count = db_strcasestr_ascii(sql, "count(*)") != NULL;
        int match_count = 0;

        for (int i = 0; i < JOSS_MEM_SQLITE_MAX_TABLES; i++) {
            JossMemSQLiteTable* t = &g_sqlite_mem_tables[i];
            if (!t->active) continue;
            if (has_target && db_strcasecmp_ascii(t->name, target_name) != 0) continue;
            match_count++;
            if (!want_count) {
                void* row = joss_map_new();
                joss_map_set_str(row, "name", joss_req_strdup(t->name));
                joss_map_set_str(row, "tbl_name", joss_req_strdup(t->name));
                joss_map_set_str(row, "type", "table");
                joss_map_set_str(row, "sql", joss_req_strdup(t->ddl));
                joss_arr_push_i64(rows, (int64_t)(uintptr_t)row);
            }
        }
        if (want_count) {
            char cbuf[32];
            snprintf(cbuf, sizeof(cbuf), "%d", match_count);
            void* row = joss_map_new();
            joss_map_set_str(row, "count(*)", joss_req_strdup(cbuf));
            joss_arr_push_i64(rows, (int64_t)(uintptr_t)row);
        }
        return rows;
    }

    const char* ti = db_strcasestr_ascii(sql, "table_info");
    if (ti != NULL) {
        const char* p = db_skip_ws(ti + 10);
        if (*p == '(') {
            p++;
            char tbl_name[128];
            if (db_parse_ident(&p, tbl_name, sizeof(tbl_name))) {
                JossMemSQLiteTable* tbl = sqlite_mem_find_table(tbl_name);
                if (tbl) {
                    for (int c = 0; c < tbl->col_count; c++) {
                        void* row = joss_map_new();
                        char cid_buf[16];
                        snprintf(cid_buf, sizeof(cid_buf), "%d", c);
                        joss_map_set_str(row, "cid", joss_req_strdup(cid_buf));
                        joss_map_set_str(row, "name", joss_req_strdup(tbl->columns[c]));
                        joss_map_set_str(row, "type", "TEXT");
                        joss_map_set_str(row, "notnull", "0");
                        joss_map_set_str(row, "pk", c == 0 ? "1" : "0");
                        joss_arr_push_i64(rows, (int64_t)(uintptr_t)row);
                    }
                }
            }
        }
        return rows;
    }

    const char* from_kw = db_strcasestr_ascii(sql, " FROM ");
    if (!from_kw) {
        const char* p = db_skip_ws(sql);
        if (db_match_kw(&p, "FROM")) from_kw = p - 4;
    }
    if (from_kw != NULL) {
        const char* p = db_skip_ws(from_kw);
        if (db_match_kw(&p, "FROM")) {
            char tbl_name[128];
            if (db_parse_ident(&p, tbl_name, sizeof(tbl_name))) {
                JossMemSQLiteTable* tbl = sqlite_mem_find_table(tbl_name);
                if (tbl) {
                    bool want_count = db_strcasestr_ascii(sql, "count(*)") != NULL;
                    const char* where_kw = db_strcasestr_ascii(p, " WHERE ");
                    if (!where_kw) {
                        const char* wp = db_skip_ws(p);
                        if (db_match_kw(&wp, "WHERE")) where_kw = wp - 5;
                    }
                    char fcols[8][128];
                    char fvals[8][512];
                    int nfilters = 0;
                    if (where_kw) {
                        int bidx = sqlite_mem_count_qmarks_until(sql, where_kw);
                        nfilters = sqlite_mem_parse_where_filters(where_kw, bindings, bidx, fcols, fvals, 8);
                    }
                    int limit_n = -1;
                    const char* lim_kw = db_strcasestr_ascii(p, " LIMIT ");
                    if (lim_kw) {
                        const char* lp = db_skip_ws(lim_kw + 6);
                        if (isdigit((unsigned char)*lp)) {
                            limit_n = atoi(lp);
                        }
                    }
                    int match_count = 0;
                    for (int r = 0; r < tbl->row_count; r++) {
                        if (!sqlite_mem_row_matches(&tbl->rows[r], fcols, fvals, nfilters)) continue;
                        match_count++;
                        if (!want_count) {
                            void* row = joss_map_new();
                            for (int c = 0; c < tbl->rows[r].cell_count; c++) {
                                joss_map_set_str(row, tbl->rows[r].cells[c].col,
                                                 joss_req_strdup(tbl->rows[r].cells[c].val ? tbl->rows[r].cells[c].val : ""));
                            }
                            joss_arr_push_i64(rows, (int64_t)(uintptr_t)row);
                            if (limit_n >= 0 && match_count >= limit_n) break;
                        }
                    }
                    if (want_count) {
                        char cbuf[32];
                        snprintf(cbuf, sizeof(cbuf), "%d", match_count);
                        void* row = joss_map_new();
                        joss_map_set_str(row, "count(*)", joss_req_strdup(cbuf));
                        joss_map_set_str(row, "total", joss_req_strdup(cbuf));
                        joss_arr_push_i64(rows, (int64_t)(uintptr_t)row);
                    }
                }
            }
        }
        return rows;
    }

    return rows;
}
#endif

/* =========================================================================
 * 2. PostgreSQL Native Driver (libpq, parameterized $1..$N)
 * ========================================================================= */
#if defined(JOSS_HAS_LIBPQ)
static PGconn* g_joss_pg = NULL;

static PGconn* joss_get_pg(void) {
    if (g_joss_pg && PQstatus(g_joss_pg) == CONNECTION_OK) return g_joss_pg;
    if (g_joss_pg) {
        PQfinish(g_joss_pg);
        g_joss_pg = NULL;
    }
    const char* host = getenv("DB_HOST"); if (!host || !*host) host = "127.0.0.1";
    const char* port = getenv("DB_PORT"); if (!port || !*port) port = "5432";
    const char* db   = getenv("DB_DATABASE"); if (!db) db = "";
    const char* user = getenv("DB_USERNAME"); if (!user) user = "";
    const char* pass = getenv("DB_PASSWORD"); if (!pass) pass = "";
    const char* ssl  = getenv("DB_SSLMODE");  if (!ssl || !*ssl) ssl = "prefer";

    char conninfo[1024];
    snprintf(conninfo, sizeof(conninfo), "host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
             host, port, db, user, pass, ssl);
    g_joss_pg = PQconnectdb(conninfo);
    if (PQstatus(g_joss_pg) != CONNECTION_OK) {
        PQfinish(g_joss_pg);
        g_joss_pg = NULL;
    }
    return g_joss_pg;
}

static char* rewrite_pg_placeholders(const char* sql) {
    JossStringBuilder* sb = joss_sb_new();
    int idx = 1;
    bool in_quote = false;
    for (const char* p = sql; *p; p++) {
        if (*p == '\'') in_quote = !in_quote;
        if (*p == '?' && !in_quote) {
            joss_buf_append_byte(sb, '$');
            joss_sb_append_i64(sb, idx++);
        } else {
            joss_buf_append_byte(sb, (uint8_t)*p);
        }
    }
    char* out = joss_sb_to_string(sb);
    joss_sb_free(sb);
    return out;
}

static void* pg_query(const char* sql, void* bindings) {
    void* rows = joss_arr_new(4);
    PGconn* conn = joss_get_pg();
    if (!conn || !sql) return rows;
    char* pg_sql = rewrite_pg_placeholders(sql);
    int nParams = bindings ? (int)joss_arr_len(bindings) : 0;
    const char** paramValues = nParams > 0 ? (const char**)calloc((size_t)nParams, sizeof(char*)) : NULL;
    for (int i = 0; i < nParams; i++) {
        paramValues[i] = joss_strval(joss_arr_get_i64(bindings, i));
    }
    PGresult* res = PQexecParams(conn, pg_sql, nParams, NULL, paramValues, NULL, NULL, 0);
    free(paramValues);
    free(pg_sql);
    if (!res) return rows;
    if (PQresultStatus(res) == PGRES_TUPLES_OK) {
        int nrows = PQntuples(res);
        int ncols = PQnfields(res);
        for (int r = 0; r < nrows; r++) {
            void* row = joss_map_new();
            for (int c = 0; c < ncols; c++) {
                const char* cname = PQfname(res, c);
                const char* cval = PQgetisnull(res, r, c) ? "" : PQgetvalue(res, r, c);
                joss_map_set_str(row, cname ? cname : "", joss_req_strdup(cval));
            }
            joss_arr_push_i64(rows, (int64_t)(uintptr_t)row);
        }
    }
    PQclear(res);
    return rows;
}

static int64_t pg_execute(const char* sql, void* bindings) {
    PGconn* conn = joss_get_pg();
    if (!conn || !sql) return 0;
    char* pg_sql = rewrite_pg_placeholders(sql);
    int nParams = bindings ? (int)joss_arr_len(bindings) : 0;
    const char** paramValues = nParams > 0 ? (const char**)calloc((size_t)nParams, sizeof(char*)) : NULL;
    for (int i = 0; i < nParams; i++) {
        paramValues[i] = joss_strval(joss_arr_get_i64(bindings, i));
    }
    PGresult* res = PQexecParams(conn, pg_sql, nParams, NULL, paramValues, NULL, NULL, 0);
    if ((!res || (PQresultStatus(res) != PGRES_COMMAND_OK && PQresultStatus(res) != PGRES_TUPLES_OK)) && PQstatus(conn) != CONNECTION_OK) {
        if (res) PQclear(res);
        conn = joss_get_pg();
        res = conn ? PQexecParams(conn, pg_sql, nParams, NULL, paramValues, NULL, NULL, 0) : NULL;
    }
    free(paramValues);
    free(pg_sql);
    if (!res) return 0;
    int64_t affected = 0;
    if (PQresultStatus(res) == PGRES_COMMAND_OK || PQresultStatus(res) == PGRES_TUPLES_OK) {
        const char* tuples = PQcmdTuples(res);
        affected = (tuples && *tuples) ? strtoll(tuples, NULL, 10) : 1;
    }
    PQclear(res);
    return affected;
}
#endif

/* =========================================================================
 * 3. MySQL / MariaDB Native Driver (libmysqlclient / libmariadb)
 * ========================================================================= */
#if defined(JOSS_HAS_MYSQL)
static MYSQL* g_joss_mysql = NULL;

static MYSQL* joss_get_mysql(void) {
    if (g_joss_mysql && mysql_ping(g_joss_mysql) == 0) return g_joss_mysql;
    if (g_joss_mysql) {
        mysql_close(g_joss_mysql);
        g_joss_mysql = NULL;
    }
    g_joss_mysql = mysql_init(NULL);
    if (!g_joss_mysql) return NULL;
    const char* host = getenv("DB_HOST"); if (!host || !*host) host = "127.0.0.1";
    const char* portStr = getenv("DB_PORT");
    unsigned int port = (portStr && *portStr) ? (unsigned int)atoi(portStr) : 3306U;
    const char* db   = getenv("DB_DATABASE"); if (!db) db = "";
    const char* user = getenv("DB_USERNAME"); if (!user) user = "root";
    const char* pass = getenv("DB_PASSWORD"); if (!pass) pass = "";

    if (!mysql_real_connect(g_joss_mysql, host, user, pass, db, port, NULL, 0)) {
        mysql_close(g_joss_mysql);
        g_joss_mysql = NULL;
    }
    return g_joss_mysql;
}

static void* mysql_query_native(const char* sql, void* bindings) {
    void* rows = joss_arr_new(4);
    MYSQL* conn = joss_get_mysql();
    if (!conn || !sql) return rows;
    MYSQL_STMT* stmt = mysql_stmt_init(conn);
    if (!stmt) return rows;
    if (mysql_stmt_prepare(stmt, sql, (unsigned long)strlen(sql)) != 0) {
        mysql_stmt_close(stmt);
        return rows;
    }
    int64_t nParams = bindings ? joss_arr_len(bindings) : 0;
    MYSQL_BIND* params = NULL;
    unsigned long* lengths = NULL;
    if (nParams > 0) {
        params = (MYSQL_BIND*)calloc((size_t)nParams, sizeof(MYSQL_BIND));
        lengths = (unsigned long*)calloc((size_t)nParams, sizeof(unsigned long));
        for (int64_t i = 0; i < nParams; i++) {
            const char* val = joss_strval(joss_arr_get_i64(bindings, i));
            lengths[i] = (unsigned long)strlen(val);
            params[i].buffer_type = MYSQL_TYPE_STRING;
            params[i].buffer = (void*)val;
            params[i].buffer_length = lengths[i];
            params[i].length = &lengths[i];
        }
        mysql_stmt_bind_param(stmt, params);
    }
    if (mysql_stmt_execute(stmt) == 0) {
        MYSQL_RES* meta = mysql_stmt_result_metadata(stmt);
        if (meta) {
            unsigned int ncols = mysql_num_fields(meta);
            MYSQL_FIELD* fields = mysql_fetch_fields(meta);
            MYSQL_BIND* resBinds = (MYSQL_BIND*)calloc(ncols, sizeof(MYSQL_BIND));
            char** bufs = (char**)calloc(ncols, sizeof(char*));
            unsigned long* resLens = (unsigned long*)calloc(ncols, sizeof(unsigned long));
            for (unsigned int c = 0; c < ncols; c++) {
                bufs[c] = (char*)calloc(1, 4096);
                resBinds[c].buffer_type = MYSQL_TYPE_STRING;
                resBinds[c].buffer = bufs[c];
                resBinds[c].buffer_length = 4095;
                resBinds[c].length = &resLens[c];
            }
            mysql_stmt_bind_result(stmt, resBinds);
            while (mysql_stmt_fetch(stmt) == 0) {
                void* row = joss_map_new();
                for (unsigned int c = 0; c < ncols; c++) {
                    bufs[c][resLens[c] < 4095 ? resLens[c] : 4095] = '\0';
                    joss_map_set_str(row, fields[c].name ? fields[c].name : "", joss_req_strdup(bufs[c]));
                }
                joss_arr_push_i64(rows, (int64_t)(uintptr_t)row);
            }
            for (unsigned int c = 0; c < ncols; c++) free(bufs[c]);
            free(bufs);
            free(resLens);
            free(resBinds);
            mysql_free_result(meta);
        }
    }
    free(params);
    free(lengths);
    mysql_stmt_close(stmt);
    return rows;
}

static int64_t mysql_execute_native(const char* sql, void* bindings) {
    MYSQL* conn = joss_get_mysql();
    if (!conn || !sql) return 0;
    MYSQL_STMT* stmt = mysql_stmt_init(conn);
    if (!stmt) return 0;
    if (mysql_stmt_prepare(stmt, sql, (unsigned long)strlen(sql)) != 0) {
        mysql_stmt_close(stmt);
        return 0;
    }
    int64_t nParams = bindings ? joss_arr_len(bindings) : 0;
    MYSQL_BIND* params = NULL;
    unsigned long* lengths = NULL;
    if (nParams > 0) {
        params = (MYSQL_BIND*)calloc((size_t)nParams, sizeof(MYSQL_BIND));
        lengths = (unsigned long*)calloc((size_t)nParams, sizeof(unsigned long));
        for (int64_t i = 0; i < nParams; i++) {
            const char* val = joss_strval(joss_arr_get_i64(bindings, i));
            lengths[i] = (unsigned long)strlen(val);
            params[i].buffer_type = MYSQL_TYPE_STRING;
            params[i].buffer = (void*)val;
            params[i].buffer_length = lengths[i];
            params[i].length = &lengths[i];
        }
        mysql_stmt_bind_param(stmt, params);
    }
    int64_t affected = 0;
    if (mysql_stmt_execute(stmt) == 0) {
        affected = (int64_t)mysql_stmt_affected_rows(stmt);
        if (strncasecmp(sql, "INSERT", 6) == 0) {
            int64_t lid = (int64_t)mysql_stmt_insert_id(stmt);
            if (lid > 0) affected = lid;
        }
    }
    free(params);
    free(lengths);
    mysql_stmt_close(stmt);
    return affected;
}
#endif

/* =========================================================================
 * 4. SQL Server / ODBC Native Driver (unixODBC)
 * ========================================================================= */
#if defined(JOSS_HAS_ODBC)
static SQLHENV g_odbc_env = SQL_NULL_HENV;
static SQLHDBC g_odbc_dbc = SQL_NULL_HDBC;

static SQLHDBC joss_get_odbc(void) {
    if (g_odbc_dbc != SQL_NULL_HDBC) return g_odbc_dbc;
    if (!SQL_SUCCEEDED(SQLAllocHandle(SQL_HANDLE_ENV, SQL_NULL_HANDLE, &g_odbc_env))) return SQL_NULL_HDBC;
    SQLSetEnvAttr(g_odbc_env, SQL_ATTR_ODBC_VERSION, (void*)SQL_OV_ODBC3, 0);
    if (!SQL_SUCCEEDED(SQLAllocHandle(SQL_HANDLE_DBC, g_odbc_env, &g_odbc_dbc))) return SQL_NULL_HDBC;

    const char* dsn = getenv("DB_CONNECTION_STRING");
    char connBuf[1024];
    if (!dsn || !*dsn) {
        const char* host = getenv("DB_HOST"); if (!host || !*host) host = "127.0.0.1";
        const char* port = getenv("DB_PORT"); if (!port || !*port) port = "1433";
        const char* db   = getenv("DB_DATABASE"); if (!db) db = "";
        const char* user = getenv("DB_USERNAME"); if (!user) user = "";
        const char* pass = getenv("DB_PASSWORD"); if (!pass) pass = "";
        snprintf(connBuf, sizeof(connBuf),
                 "DRIVER={ODBC Driver 18 for SQL Server};SERVER=%s,%s;DATABASE=%s;UID=%s;PWD=%s;TrustServerCertificate=yes;",
                 host, port, db, user, pass);
        dsn = connBuf;
    }
    SQLRETURN rc = SQLDriverConnect(g_odbc_dbc, NULL, (SQLCHAR*)dsn, SQL_NTS, NULL, 0, NULL, SQL_DRIVER_NOPROMPT);
    if (!SQL_SUCCEEDED(rc)) {
        SQLFreeHandle(SQL_HANDLE_DBC, g_odbc_dbc);
        g_odbc_dbc = SQL_NULL_HDBC;
    }
    return g_odbc_dbc;
}

static void* odbc_query(const char* sql, void* bindings) {
    void* rows = joss_arr_new(4);
    SQLHDBC dbc = joss_get_odbc();
    if (dbc == SQL_NULL_HDBC || !sql) return rows;
    SQLHSTMT stmt = SQL_NULL_HSTMT;
    if (!SQL_SUCCEEDED(SQLAllocHandle(SQL_HANDLE_STMT, dbc, &stmt))) return rows;
    if (SQL_SUCCEEDED(SQLPrepare(stmt, (SQLCHAR*)sql, SQL_NTS))) {
        int64_t nParams = bindings ? joss_arr_len(bindings) : 0;
        for (int64_t i = 0; i < nParams; i++) {
            const char* val = joss_strval(joss_arr_get_i64(bindings, i));
            SQLBindParameter(stmt, (SQLUSMALLINT)(i + 1), SQL_PARAM_INPUT, SQL_C_CHAR, SQL_VARCHAR,
                             (SQLULEN)strlen(val), 0, (SQLPOINTER)val, 0, NULL);
        }
        if (SQL_SUCCEEDED(SQLExecute(stmt))) {
            SQLSMALLINT ncols = 0;
            SQLNumResultCols(stmt, &ncols);
            while (SQL_SUCCEEDED(SQLFetch(stmt))) {
                void* row = joss_map_new();
                for (SQLSMALLINT c = 1; c <= ncols; c++) {
                    SQLCHAR colName[128] = {0};
                    SQLDescribeCol(stmt, (SQLUSMALLINT)c, colName, sizeof(colName), NULL, NULL, NULL, NULL, NULL);
                    char valBuf[4096] = {0};
                    SQLLEN ind = 0;
                    if (SQL_SUCCEEDED(SQLGetData(stmt, (SQLUSMALLINT)c, SQL_C_CHAR, valBuf, sizeof(valBuf), &ind)) && ind != SQL_NULL_DATA) {
                        joss_map_set_str(row, (const char*)colName, joss_req_strdup(valBuf));
                    } else {
                        joss_map_set_str(row, (const char*)colName, "");
                    }
                }
                joss_arr_push_i64(rows, (int64_t)(uintptr_t)row);
            }
        }
    }
    SQLFreeHandle(SQL_HANDLE_STMT, stmt);
    return rows;
}

static int64_t odbc_execute(const char* sql, void* bindings) {
    SQLHDBC dbc = joss_get_odbc();
    if (dbc == SQL_NULL_HDBC || !sql) return 0;
    SQLHSTMT stmt = SQL_NULL_HSTMT;
    if (!SQL_SUCCEEDED(SQLAllocHandle(SQL_HANDLE_STMT, dbc, &stmt))) return 0;
    int64_t affected = 0;
    if (SQL_SUCCEEDED(SQLPrepare(stmt, (SQLCHAR*)sql, SQL_NTS))) {
        int64_t nParams = bindings ? joss_arr_len(bindings) : 0;
        for (int64_t i = 0; i < nParams; i++) {
            const char* val = joss_strval(joss_arr_get_i64(bindings, i));
            SQLBindParameter(stmt, (SQLUSMALLINT)(i + 1), SQL_PARAM_INPUT, SQL_C_CHAR, SQL_VARCHAR,
                             (SQLULEN)strlen(val), 0, (SQLPOINTER)val, 0, NULL);
        }
        if (SQL_SUCCEEDED(SQLExecute(stmt))) {
            SQLLEN rcCount = 0;
            SQLRowCount(stmt, &rcCount);
            affected = rcCount >= 0 ? (int64_t)rcCount : 1;
        }
    }
    SQLFreeHandle(SQL_HANDLE_STMT, stmt);
    return affected;
}
#endif

#include "joss_db_wire.inc"

/* =========================================================================
 * Unified GranDB Native Dispatch (SQLite, PostgreSQL, MySQL, SQL Server)
 * ========================================================================= */
void*   joss___joss_db_query(const char* sql, void* bindings);
int64_t joss___joss_db_execute(const char* sql, void* bindings);

int64_t joss_db_ensure_connected(void) {
    joss_register_schema_db(joss___joss_db_query, joss___joss_db_execute);
    int64_t res = 0;
    joss_global_lock(JOSS_LOCK_DB);
    switch (detect_db_engine()) {
#if defined(JOSS_HAS_SQLITE3)
        case JOSS_DB_SQLITE:   res = joss_get_sqlite() != NULL ? 1 : 0; break;
#else
        case JOSS_DB_SQLITE:   res = sqlite_mem_ensure_connected(); break;
#endif
#if defined(JOSS_HAS_LIBPQ)
        case JOSS_DB_POSTGRES: res = joss_get_pg() != NULL ? 1 : 0; break;
#else
        case JOSS_DB_POSTGRES: res = joss_wire_get_pg() != JOSS_WIRE_INVALID_SOCK ? 1 : 0; break;
#endif
#if defined(JOSS_HAS_MYSQL)
        case JOSS_DB_MYSQL:    res = joss_get_mysql() != NULL ? 1 : 0; break;
#else
        case JOSS_DB_MYSQL:    res = joss_wire_get_mysql() != JOSS_WIRE_INVALID_SOCK ? 1 : 0; break;
#endif
#if defined(JOSS_HAS_ODBC)
        case JOSS_DB_SQLSRV:   res = joss_get_odbc() != SQL_NULL_HDBC ? 1 : 0; break;
#endif
        default:               res = 0; break;
    }
    joss_global_unlock(JOSS_LOCK_DB);
    return res;
}

#if defined(__clang__) || defined(__GNUC__)
__attribute__((constructor, used))
static void joss_db_auto_register_schema(void) {
    joss_register_schema_db(joss___joss_db_query, joss___joss_db_execute);
}
#elif defined(_MSC_VER)
static void __cdecl joss_db_auto_register_schema(void) {
    joss_register_schema_db(joss___joss_db_query, joss___joss_db_execute);
}
#pragma section(".CRT$XCU", read)
__declspec(allocate(".CRT$XCU")) void (__cdecl *joss_db_auto_register_schema_ptr)(void) = joss_db_auto_register_schema;
#endif

int64_t joss___joss_db_is_connected(void) {
    return joss_db_ensure_connected();
}

void* joss___joss_db_query(const char* sql, void* bindings) {
    joss_register_schema_db(joss___joss_db_query, joss___joss_db_execute);
    void* res = NULL;
    joss_global_lock(JOSS_LOCK_DB);
    switch (detect_db_engine()) {
#if defined(JOSS_HAS_SQLITE3)
        case JOSS_DB_SQLITE:   res = sqlite_query(sql, bindings); break;
#else
        case JOSS_DB_SQLITE:   res = sqlite_mem_query(sql, bindings); break;
#endif
#if defined(JOSS_HAS_LIBPQ)
        case JOSS_DB_POSTGRES: res = pg_query(sql, bindings); break;
#else
        case JOSS_DB_POSTGRES: res = pg_wire_query(sql, bindings); break;
#endif
#if defined(JOSS_HAS_MYSQL)
        case JOSS_DB_MYSQL:    res = mysql_query_native(sql, bindings); break;
#else
        case JOSS_DB_MYSQL:    res = mysql_wire_query(sql, bindings); break;
#endif
#if defined(JOSS_HAS_ODBC)
        case JOSS_DB_SQLSRV:   res = odbc_query(sql, bindings); break;
#endif
        default:               res = joss_arr_new(0); break;
    }
    joss_global_unlock(JOSS_LOCK_DB);
    return res;
}

int64_t joss___joss_db_execute(const char* sql, void* bindings) {
    joss_register_schema_db(joss___joss_db_query, joss___joss_db_execute);
    int64_t res = 0;
    joss_global_lock(JOSS_LOCK_DB);
    switch (detect_db_engine()) {
#if defined(JOSS_HAS_SQLITE3)
        case JOSS_DB_SQLITE:   res = sqlite_execute(sql, bindings); break;
#else
        case JOSS_DB_SQLITE:   res = sqlite_mem_execute(sql, bindings); break;
#endif
#if defined(JOSS_HAS_LIBPQ)
        case JOSS_DB_POSTGRES: res = pg_execute(sql, bindings); break;
#else
        case JOSS_DB_POSTGRES: res = pg_wire_execute(sql, bindings); break;
#endif
#if defined(JOSS_HAS_MYSQL)
        case JOSS_DB_MYSQL:    res = mysql_execute_native(sql, bindings); break;
#else
        case JOSS_DB_MYSQL:    res = mysql_wire_execute(sql, bindings); break;
#endif
#if defined(JOSS_HAS_ODBC)
        case JOSS_DB_SQLSRV:   res = odbc_execute(sql, bindings); break;
#endif
        default:               res = 0; break;
    }
    joss_global_unlock(JOSS_LOCK_DB);
    return res;
}

/* =========================================================================
 * In-Memory GranDB Fallback Tables
 * ========================================================================= */
static void* g_joss_inmem_tables = NULL;

void* joss___joss_db_get_table(const char* name) {
    if (!g_joss_inmem_tables) g_joss_inmem_tables = joss_map_new();
    if (!name) return joss_arr_new(4);
    int64_t ptr = joss_map_get_i64(g_joss_inmem_tables, name);
    if (ptr == 0) {
        void* tbl = joss_arr_new(4);
        joss_map_set_i64(g_joss_inmem_tables, name, (int64_t)(uintptr_t)tbl);
        return tbl;
    }
    return (void*)(uintptr_t)ptr;
}

int64_t joss___joss_db_set_table(const char* name, void* rows) {
    if (!g_joss_inmem_tables) g_joss_inmem_tables = joss_map_new();
    if (!name) return 0;
    joss_map_set_i64(g_joss_inmem_tables, name, (int64_t)(uintptr_t)rows);
    return 1;
}
