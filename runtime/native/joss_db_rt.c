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
            if (sqlite3_column_type(stmt, c) == SQLITE_NULL) {
                joss_map_set_i64(row, cname ? cname : "", (int64_t)(uintptr_t)joss_box_value(JOSS_TAG_NULL, 0));
            } else {
                const unsigned char* cval = sqlite3_column_text(stmt, c);
                joss_map_set_str(row, cname ? cname : "", cval ? joss_req_strdup((const char*)cval) : "");
            }
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
            int64_t bv = joss_arr_get_i64(bindings, i);
            if (bv == 0 || !joss_is_not_null(bv)) {
                sqlite3_bind_null(stmt, (int)(i + 1));
            } else {
                const char* val = joss_strval(bv);
                sqlite3_bind_text(stmt, (int)(i + 1), val, -1, SQLITE_TRANSIENT);
            }
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
    if (joss_is_boxed_ptr(v) || (uint64_t)u < JOSS_MIN_PTR_ADDR || ((uint64_t)u >> JOSS_PTR_VA_BITS) != 0) {
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
                                if (tbl->rows[r].cells[c].val == NULL) {
                                    joss_map_set_i64(row, tbl->rows[r].cells[c].col,
                                                     (int64_t)(uintptr_t)joss_box_value(JOSS_TAG_NULL, 0));
                                } else {
                                    joss_map_set_str(row, tbl->rows[r].cells[c].col,
                                                     joss_req_strdup(tbl->rows[r].cells[c].val));
                                }
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
    if ((!res || PQresultStatus(res) != PGRES_TUPLES_OK) && PQstatus(conn) != CONNECTION_OK) {
        if (res) PQclear(res);
        conn = joss_get_pg();
        res = conn ? PQexecParams(conn, pg_sql, nParams, NULL, paramValues, NULL, NULL, 0) : NULL;
    }
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
                if (PQgetisnull(res, r, c)) {
                    joss_map_set_i64(row, cname ? cname : "", (int64_t)(uintptr_t)joss_box_value(JOSS_TAG_NULL, 0));
                } else {
                    const char* cval = PQgetvalue(res, r, c);
                    joss_map_set_str(row, cname ? cname : "", joss_req_strdup(cval ? cval : ""));
                }
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
            my_bool* resNulls = (my_bool*)calloc(ncols, sizeof(my_bool));
            for (unsigned int c = 0; c < ncols; c++) {
                bufs[c] = (char*)calloc(1, 4096);
                resBinds[c].buffer_type = MYSQL_TYPE_STRING;
                resBinds[c].buffer = bufs[c];
                resBinds[c].buffer_length = 4095;
                resBinds[c].length = &resLens[c];
                resBinds[c].is_null = &resNulls[c];
            }
            mysql_stmt_bind_result(stmt, resBinds);
            while (mysql_stmt_fetch(stmt) == 0) {
                void* row = joss_map_new();
                for (unsigned int c = 0; c < ncols; c++) {
                    if (resNulls[c]) {
                        joss_map_set_i64(row, fields[c].name ? fields[c].name : "", (int64_t)(uintptr_t)joss_box_value(JOSS_TAG_NULL, 0));
                    } else {
                        bufs[c][resLens[c] < 4095 ? resLens[c] : 4095] = '\0';
                        joss_map_set_str(row, fields[c].name ? fields[c].name : "", joss_req_strdup(bufs[c]));
                    }
                }
                joss_arr_push_i64(rows, (int64_t)(uintptr_t)row);
            }
            for (unsigned int c = 0; c < ncols; c++) free(bufs[c]);
            free(bufs);
            free(resLens);
            free(resNulls);
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
                        joss_map_set_i64(row, (const char*)colName, (int64_t)(uintptr_t)joss_box_value(JOSS_TAG_NULL, 0));
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

/* =========================================================================
 * Native Model ORM Engine (full parity with pkg/core/model.go)
 * Supports: metadata, casts, mass assignment, dirty tracking, INSERT/UPDATE,
 * soft deletes, global & local scopes, lifecycle hooks, accessors/mutators,
 * relations (hasOne/hasMany/belongsTo/belongsToMany + attach/detach/sync),
 * eager loading (with/load/loadMissing), and toMap/toJSON serialization.
 * ========================================================================= */
#include <time.h>

extern int64_t     joss_is_null(int64_t v);
extern int64_t     joss_boolval(int64_t v);
extern int64_t     joss_intval(int64_t v);
extern double      joss_floatval(int64_t v);
extern int64_t     joss_array_key_exists(const char* key, void* m);
extern const char* joss_json_encode(int64_t data);
extern int64_t     joss_json_decode(const char* s);

#define JOSS_MODEL_MAGIC 0x4A4F53534D4F444CULL
#define JOSS_MODEL_MAX_CLASSES 128
#define JOSS_MODEL_MAX_METHODS 128

typedef struct {
    char*   name;
    void*   fn_ptr;
    int64_t arity;
} JossModelMethodEntry;

typedef struct {
    char*  class_name;
    char*  table;
    char*  primary_key;
    char*  key_type;
    bool   incrementing;
    bool   timestamps;
    char*  created_at;
    char*  updated_at;
    char*  deleted_at;
    bool   soft_deletes;
    void*  fillable;       /* JossMap* set */
    void*  guarded;        /* JossMap* set */
    void*  hidden;         /* JossMap* set */
    void*  visible;        /* JossMap* set */
    void*  casts;          /* JossMap* field -> cast_type */
    void*  global_scopes;  /* JossArray* of scope names */
    JossModelMethodEntry methods[JOSS_MODEL_MAX_METHODS];
    int    method_count;
} JossModelMeta;

typedef enum {
    JOSS_REL_NONE = 0,
    JOSS_REL_HAS_ONE = 1,
    JOSS_REL_HAS_MANY = 2,
    JOSS_REL_BELONGS_TO = 3,
    JOSS_REL_BELONGS_TO_MANY = 4
} JossModelRelKind;

typedef struct JossModel JossModel;

typedef struct {
    JossModelRelKind kind;
    JossModel*       parent;
    char*            foreign_key;
    char*            local_key;
    char*            pivot_table;
    char*            foreign_pivot_key;
    char*            related_pivot_key;
    char*            related_key;
} JossModelRelation;

struct JossModel {
    uint64_t           magic;
    JossModelMeta*     meta;
    void*              attrs;
    void*              original;
    void*              last_changes;
    void*              relations;
    void*              loaded;
    bool               exists;
    bool               deleted;
    bool               is_query;
    void*              wheres;
    void*              bindings;
    void*              orders;
    int64_t            limit_val;
    int64_t            offset_val;
    void*              with_rels;
    int                soft_delete_mode; /* 0=withoutTrashed, 1=withTrashed, 2=onlyTrashed */
    void*              excluded_global_scopes;
    bool               global_scopes_applied;
    JossModelRelation* relation;
};

static JossModelMeta g_model_classes[JOSS_MODEL_MAX_CLASSES];
static int           g_model_class_count = 0;

static bool joss_is_model_ptr(int64_t v) {
    int64_t u = joss_unbox_value(v);
    if (u == 0 || (uint64_t)u < JOSS_MIN_PTR_ADDR || ((uint64_t)u >> JOSS_PTR_VA_BITS) != 0) return false;
    JossModel* m = (JossModel*)(uintptr_t)u;
    return m->magic == JOSS_MODEL_MAGIC;
}

static JossModel* joss_as_model(void* p) {
    int64_t u = joss_unbox_value((int64_t)(uintptr_t)p);
    if (u == 0 || (uint64_t)u < JOSS_MIN_PTR_ADDR || ((uint64_t)u >> JOSS_PTR_VA_BITS) != 0) return NULL;
    JossModel* m = (JossModel*)(uintptr_t)u;
    return (m->magic == JOSS_MODEL_MAGIC) ? m : NULL;
}

static const char* joss_model_cstr(const void* p) {
    if (!p) return "";
    int64_t v = (int64_t)(uintptr_t)p;
    if (joss_is_null(v)) return "";
    if (joss_is_boxed_ptr(v)) {
        const char* s = joss_strval(v);
        return s ? s : "";
    }
    uintptr_t u = (uintptr_t)p;
    if ((uint64_t)u < JOSS_MIN_PTR_ADDR || ((uint64_t)u >> JOSS_PTR_VA_BITS) != 0) {
        const char* s = joss_strval(v);
        return s ? s : "";
    }
    return (const char*)p;
}

static char* joss_model_strdup(const char* s) {
    if (!s) s = "";
    size_t n = strlen(s);
    char* out = (char*)malloc(n + 1U);
    if (!out) return NULL;
    memcpy(out, s, n + 1U);
    return out;
}

static bool joss_model_valid_ident(const char* s) {
    if (!s || !*s) return false;
    for (const char* p = s; *p; p++) {
        unsigned char c = (unsigned char)*p;
        if (!(isalnum(c) || c == '_' || c == '.')) return false;
    }
    return true;
}

static void joss_model_append_ident(JossStringBuilder* sb, const char* ident) {
    if (!ident || !*ident) return;
    joss_sb_append(sb, "\"");
    for (const char* p = ident; *p; p++) {
        if (*p == '.') {
            joss_sb_append(sb, "\".\"");
        } else if (*p != '"') {
            char ch[2] = { *p, '\0' };
            joss_sb_append(sb, ch);
        }
    }
    joss_sb_append(sb, "\"");
}

static void joss_model_snake_case(const char* src, char* dst, size_t cap) {
    if (!src || !dst || cap == 0) return;
    size_t o = 0;
    for (size_t i = 0; src[i] && o + 2 < cap; i++) {
        unsigned char c = (unsigned char)src[i];
        if (i > 0 && c >= 'A' && c <= 'Z') {
            dst[o++] = '_';
        }
        dst[o++] = (char)tolower(c);
    }
    dst[o] = '\0';
}

static void joss_model_studly_case(const char* src, char* dst, size_t cap) {
    if (!src || !dst || cap == 0) return;
    size_t o = 0;
    bool cap_next = true;
    for (size_t i = 0; src[i] && o + 1 < cap; i++) {
        unsigned char c = (unsigned char)src[i];
        if (c == '_' || c == '-') {
            cap_next = true;
        } else if (cap_next) {
            dst[o++] = (char)toupper(c);
            cap_next = false;
        } else {
            dst[o++] = (char)c;
        }
    }
    dst[o] = '\0';
}

static void joss_model_pluralize(const char* word, char* dst, size_t cap) {
    if (!word || !dst || cap == 0) return;
    size_t n = strlen(word);
    if (n == 0) { dst[0] = '\0'; return; }
    char lower[256];
    size_t lim = (n < sizeof(lower) - 1) ? n : (sizeof(lower) - 1);
    for (size_t i = 0; i < lim; i++) lower[i] = (char)tolower((unsigned char)word[i]);
    lower[lim] = '\0';
    if (lim > 1 && lower[lim - 1] == 'y') {
        lower[lim - 1] = '\0';
        snprintf(dst, cap, "%sies", lower);
    } else if (lower[lim - 1] == 's') {
        snprintf(dst, cap, "%ses", lower);
    } else {
        snprintf(dst, cap, "%ss", lower);
    }
}

static JossModelMeta* joss_model_find_meta(const char* class_name) {
    if (!class_name || !*class_name) return NULL;
    for (int i = 0; i < g_model_class_count; i++) {
        if (g_model_classes[i].class_name && strcmp(g_model_classes[i].class_name, class_name) == 0) {
            return &g_model_classes[i];
        }
    }
    return NULL;
}

static JossModelMethodEntry* joss_model_find_method(JossModelMeta* meta, const char* name) {
    if (!meta || !name) return NULL;
    for (int i = 0; i < meta->method_count; i++) {
        if (meta->methods[i].name && strcmp(meta->methods[i].name, name) == 0) {
            return &meta->methods[i];
        }
    }
    return NULL;
}

static void* joss_model_map_clone(void* src) {
    void* dst = joss_map_new();
    if (!src) return dst;
    int64_t n = joss_map_len(src);
    for (int64_t i = 0; i < n; i++) {
        const char* k = joss_map_key_at(src, i);
        int64_t v = joss_map_val_at(src, i);
        if (k) joss_map_set_i64(dst, k, v);
    }
    return dst;
}

static bool joss_model_values_equal(int64_t a, int64_t b) {
    if (a == b) return true;
    bool an = (a == 0 || joss_is_null(a));
    bool bn = (b == 0 || joss_is_null(b));
    if (an && bn) return true;
    if (an || bn) return false;
    const char* sa = joss_strval(a);
    const char* sb = joss_strval(b);
    if (!sa) sa = "";
    if (!sb) sb = "";
    return strcmp(sa, sb) == 0;
}

static int64_t joss_model_cast_value(JossModelMeta* meta, const char* field, int64_t raw) {
    if (!meta || !field || !meta->casts) return raw;
    if (raw == 0 || joss_is_null(raw)) return (int64_t)(uintptr_t)joss_box_value(0, 0);
    if (!joss_array_key_exists(field, meta->casts)) return raw;
    const char* cast = joss_model_cstr((const void*)(uintptr_t)joss_map_get_i64(meta->casts, field));
    if (!cast || !*cast) return raw;
    if (strcmp(cast, "string") == 0) {
        return (int64_t)(uintptr_t)joss_box_str(joss_strval(raw));
    }
    if (strcmp(cast, "int") == 0) {
        return (int64_t)(uintptr_t)joss_box_i64(joss_intval(raw));
    }
    if (strcmp(cast, "float") == 0) {
        return (int64_t)(uintptr_t)joss_box_f64(joss_floatval(raw));
    }
    if (strcmp(cast, "bool") == 0) {
        const char* s = joss_strval(raw);
        bool b = false;
        if (s && (strcmp(s, "1") == 0 || strcasecmp(s, "true") == 0)) b = true;
        else if (s && (strcmp(s, "0") == 0 || strcasecmp(s, "false") == 0 || *s == '\0')) b = false;
        else b = (joss_boolval(raw) != 0);
        return (int64_t)(uintptr_t)joss_box_bool(b);
    }
    if (strcmp(cast, "json") == 0 || strcmp(cast, "array") == 0 || strcmp(cast, "object") == 0) {
        int64_t u = joss_unbox_value(raw);
        if (joss_is_map_ptr(u) || joss_is_arr_ptr(u)) return raw;
        const char* s = joss_strval(raw);
        if (s && *s) return joss_json_decode(s);
    }
    return raw;
}

static int64_t joss_model_db_value(JossModelMeta* meta, const char* field, int64_t val) {
    if (val == 0 || joss_is_null(val)) return 0;
    if (!meta || !field || !meta->casts || !joss_array_key_exists(field, meta->casts)) {
        int64_t u = joss_unbox_value(val);
        if (joss_is_map_ptr(u) || joss_is_arr_ptr(u)) {
            return (int64_t)(uintptr_t)joss_box_str(joss_json_encode(val));
        }
        return val;
    }
    const char* cast = joss_model_cstr((const void*)(uintptr_t)joss_map_get_i64(meta->casts, field));
    if (strcmp(cast, "json") == 0 || strcmp(cast, "array") == 0 || strcmp(cast, "object") == 0) {
        return (int64_t)(uintptr_t)joss_box_str(joss_json_encode(val));
    }
    if (strcmp(cast, "bool") == 0) {
        return (int64_t)(uintptr_t)joss_box_i64(joss_boolval(val) ? 1 : 0);
    }
    return val;
}

static bool joss_model_mass_assignable(JossModelMeta* meta, const char* name) {
    if (!meta || !name) return false;
    if (meta->fillable && joss_array_key_exists(name, meta->fillable)) return true;
    if (meta->fillable && joss_map_len(meta->fillable) > 0) return false;
    if (meta->guarded && joss_array_key_exists("*", meta->guarded)) return false;
    if (meta->guarded && joss_array_key_exists(name, meta->guarded)) return false;
    return true;
}

static void* joss_model_db_query(const char* sql, void* bindings) {
#if defined(JOSS_HAS_SQLITE3)
    if (detect_db_engine() == JOSS_DB_SQLITE) {
        void* rows = joss_arr_new(4);
        joss_global_lock(JOSS_LOCK_DB);
        sqlite3* db = joss_get_sqlite();
        if (!db || !sql) { joss_global_unlock(JOSS_LOCK_DB); return rows; }
        sqlite3_stmt* stmt = NULL;
        if (sqlite3_prepare_v2(db, sql, -1, &stmt, NULL) != SQLITE_OK) {
            joss_global_unlock(JOSS_LOCK_DB);
            return rows;
        }
        if (bindings) {
            int64_t n = joss_arr_len(bindings);
            for (int64_t i = 0; i < n; i++) {
                int64_t bv = joss_arr_get_i64(bindings, i);
                if (bv == 0 || joss_is_null(bv)) {
                    sqlite3_bind_null(stmt, (int)(i + 1));
                } else {
                    const char* val = joss_strval(bv);
                    sqlite3_bind_text(stmt, (int)(i + 1), val, -1, SQLITE_TRANSIENT);
                }
            }
        }
        int cols = sqlite3_column_count(stmt);
        while (sqlite3_step(stmt) == SQLITE_ROW) {
            void* row = joss_map_new();
            for (int c = 0; c < cols; c++) {
                const char* cname = sqlite3_column_name(stmt, c);
                if (!cname) cname = "";
                int ctype = sqlite3_column_type(stmt, c);
                if (ctype == SQLITE_NULL) {
                    joss_map_set_i64(row, cname, (int64_t)(uintptr_t)joss_box_value(0, 0));
                } else if (ctype == SQLITE_INTEGER) {
                    joss_map_set_i64(row, cname, (int64_t)(uintptr_t)joss_box_i64(sqlite3_column_int64(stmt, c)));
                } else if (ctype == SQLITE_FLOAT) {
                    joss_map_set_i64(row, cname, (int64_t)(uintptr_t)joss_box_f64(sqlite3_column_double(stmt, c)));
                } else {
                    const unsigned char* cval = sqlite3_column_text(stmt, c);
                    joss_map_set_i64(row, cname, (int64_t)(uintptr_t)joss_box_str(cval ? joss_req_strdup((const char*)cval) : ""));
                }
            }
            joss_arr_push_i64(rows, (int64_t)(uintptr_t)row);
        }
        sqlite3_finalize(stmt);
        joss_global_unlock(JOSS_LOCK_DB);
        return rows;
    }
#endif
    return joss___joss_db_query(sql, bindings);
}

static int64_t joss_model_last_insert_id(void) {
#if defined(JOSS_HAS_SQLITE3)
    if (detect_db_engine() == JOSS_DB_SQLITE) {
        joss_global_lock(JOSS_LOCK_DB);
        int64_t id = g_joss_sqlite ? (int64_t)sqlite3_last_insert_rowid(g_joss_sqlite) : 0;
        joss_global_unlock(JOSS_LOCK_DB);
        return id;
    }
#endif
    void* rows = joss_model_db_query("SELECT last_insert_rowid() AS id", NULL);
    if (rows && joss_arr_len(rows) > 0) {
        void* row = (void*)(uintptr_t)joss_arr_get_i64(rows, 0);
        if (row) return joss_intval(joss_map_get_i64(row, "id"));
    }
    return 0;
}

static JossModel* joss_model_alloc(JossModelMeta* meta, bool exists, bool is_query) {
    JossModel* m = (JossModel*)calloc(1, sizeof(JossModel));
    if (!m) return NULL;
    m->magic = JOSS_MODEL_MAGIC;
    m->meta = meta;
    m->attrs = joss_map_new();
    m->original = joss_map_new();
    m->last_changes = joss_map_new();
    m->relations = joss_map_new();
    m->loaded = joss_map_new();
    m->exists = exists;
    m->deleted = false;
    m->is_query = is_query;
    m->wheres = joss_arr_new(4);
    m->bindings = joss_arr_new(4);
    m->orders = joss_arr_new(2);
    m->limit_val = -1;
    m->offset_val = -1;
    m->with_rels = joss_arr_new(2);
    m->soft_delete_mode = 0;
    m->excluded_global_scopes = joss_map_new();
    m->global_scopes_applied = false;
    m->relation = NULL;
    return m;
}

static JossModel* joss_model_hydrate(JossModelMeta* meta, void* row) {
    JossModel* m = joss_model_alloc(meta, true, false);
    if (!m || !row) return m;
    int64_t n = joss_map_len(row);
    for (int64_t i = 0; i < n; i++) {
        const char* k = joss_map_key_at(row, i);
        int64_t v = joss_map_val_at(row, i);
        if (!k) continue;
        int64_t casted = joss_model_cast_value(meta, k, v);
        joss_map_set_i64(m->attrs, k, casted);
        joss_map_set_i64(m->original, k, casted);
    }
    return m;
}

static bool joss_model_run_hook(JossModel* m, const char* hook_name) {
    if (!m || !m->meta || !hook_name) return true;
    JossModelMethodEntry* entry = joss_model_find_method(m->meta, hook_name);
    if (!entry || !entry->fn_ptr) return true;
    typedef uint8_t (*HookFn)(void*);
    HookFn fn = (HookFn)entry->fn_ptr;
    uint8_t res = fn(m);
    return (res & 1u) != 0;
}

static void joss_model_apply_global_scopes(JossModel* q) {
    if (!q || !q->meta || q->global_scopes_applied) return;
    q->global_scopes_applied = true;
    if (!q->meta->global_scopes) return;
    int64_t n = joss_arr_len(q->meta->global_scopes);
    for (int64_t i = 0; i < n; i++) {
        const char* sname = joss_model_cstr((const void*)(uintptr_t)joss_arr_get_i64(q->meta->global_scopes, i));
        if (!sname || !*sname) continue;
        if (q->excluded_global_scopes && joss_array_key_exists(sname, q->excluded_global_scopes)) continue;
        char mname[256];
        char studly[200];
        joss_model_studly_case(sname, studly, sizeof(studly));
        snprintf(mname, sizeof(mname), "scope%s", studly);
        JossModelMethodEntry* entry = joss_model_find_method(q->meta, mname);
        if (entry && entry->fn_ptr) {
            typedef void* (*ScopeFn)(void*, void*);
            ScopeFn fn = (ScopeFn)entry->fn_ptr;
            (void)fn(q, q);
        }
    }
}

static void* joss_model_compute_changes(JossModel* m) {
    void* diff = joss_map_new();
    if (!m || !m->attrs) return diff;
    int64_t n = joss_map_len(m->attrs);
    for (int64_t i = 0; i < n; i++) {
        const char* k = joss_map_key_at(m->attrs, i);
        if (!k || k[0] == '_') continue;
        int64_t v = joss_map_val_at(m->attrs, i);
        if (!m->original || !joss_array_key_exists(k, m->original)) {
            joss_map_set_i64(diff, k, v);
        } else {
            int64_t orig = joss_map_get_i64(m->original, k);
            if (!joss_model_values_equal(orig, v)) {
                joss_map_set_i64(diff, k, v);
            }
        }
    }
    return diff;
}

/* Registration */
void fn_Model_registerClass(const char* class_name_raw, const char* meta_json_raw) {
    const char* cls = joss_model_cstr(class_name_raw);
    const char* json_str = joss_model_cstr(meta_json_raw);
    if (!cls || !*cls) return;

    JossModelMeta* meta = joss_model_find_meta(cls);
    if (!meta) {
        if (g_model_class_count >= JOSS_MODEL_MAX_CLASSES) return;
        meta = &g_model_classes[g_model_class_count++];
        memset(meta, 0, sizeof(JossModelMeta));
        meta->class_name = joss_model_strdup(cls);
    }

    char def_tbl[256];
    joss_model_pluralize(cls, def_tbl, sizeof(def_tbl));
    meta->table = joss_model_strdup(def_tbl);
    meta->primary_key = joss_model_strdup("id");
    meta->key_type = joss_model_strdup("int");
    meta->incrementing = true;
    meta->timestamps = true;
    meta->created_at = joss_model_strdup("created_at");
    meta->updated_at = joss_model_strdup("updated_at");
    meta->deleted_at = joss_model_strdup("deleted_at");
    meta->soft_deletes = false;
    meta->fillable = joss_map_new();
    meta->guarded = joss_map_new();
    joss_map_set_i64(meta->guarded, "*", 1);
    meta->hidden = joss_map_new();
    meta->visible = joss_map_new();
    meta->casts = joss_map_new();
    meta->global_scopes = joss_arr_new(2);

    if (!json_str || !*json_str) return;
    int64_t decoded = joss_json_decode(json_str);
    int64_t u = joss_unbox_value(decoded);
    if (!u || !joss_is_map_ptr(u)) return;
    void* cfg = (void*)(uintptr_t)u;

    if (joss_array_key_exists("table", cfg)) {
        const char* t = joss_model_cstr((const void*)(uintptr_t)joss_map_get_i64(cfg, "table"));
        if (t && *t) { free(meta->table); meta->table = joss_model_strdup(t); }
    }
    if (joss_array_key_exists("primaryKey", cfg)) {
        const char* pk = joss_model_cstr((const void*)(uintptr_t)joss_map_get_i64(cfg, "primaryKey"));
        if (pk && *pk) { free(meta->primary_key); meta->primary_key = joss_model_strdup(pk); }
    }
    if (joss_array_key_exists("keyType", cfg)) {
        const char* kt = joss_model_cstr((const void*)(uintptr_t)joss_map_get_i64(cfg, "keyType"));
        if (kt && *kt) { free(meta->key_type); meta->key_type = joss_model_strdup(kt); }
    }
    if (joss_array_key_exists("incrementing", cfg)) {
        meta->incrementing = (joss_boolval(joss_map_get_i64(cfg, "incrementing")) != 0);
    }
    if (joss_array_key_exists("timestamps", cfg)) {
        meta->timestamps = (joss_boolval(joss_map_get_i64(cfg, "timestamps")) != 0);
    }
    if (joss_array_key_exists("softDeletes", cfg)) {
        meta->soft_deletes = (joss_boolval(joss_map_get_i64(cfg, "softDeletes")) != 0);
    }
    if (joss_array_key_exists("createdAt", cfg)) {
        const char* s = joss_model_cstr((const void*)(uintptr_t)joss_map_get_i64(cfg, "createdAt"));
        if (s && *s) { free(meta->created_at); meta->created_at = joss_model_strdup(s); }
    }
    if (joss_array_key_exists("updatedAt", cfg)) {
        const char* s = joss_model_cstr((const void*)(uintptr_t)joss_map_get_i64(cfg, "updatedAt"));
        if (s && *s) { free(meta->updated_at); meta->updated_at = joss_model_strdup(s); }
    }
    if (joss_array_key_exists("deletedAt", cfg)) {
        const char* s = joss_model_cstr((const void*)(uintptr_t)joss_map_get_i64(cfg, "deletedAt"));
        if (s && *s) { free(meta->deleted_at); meta->deleted_at = joss_model_strdup(s); }
    }
    if (joss_array_key_exists("fillable", cfg)) {
        int64_t av = joss_unbox_value(joss_map_get_i64(cfg, "fillable"));
        if (av && joss_is_arr_ptr(av)) {
            void* arr = (void*)(uintptr_t)av;
            for (int64_t i = 0; i < joss_arr_len(arr); i++) {
                const char* item = joss_model_cstr((const void*)(uintptr_t)joss_arr_get_i64(arr, i));
                if (item && *item) joss_map_set_i64(meta->fillable, item, 1);
            }
        }
    }
    if (joss_array_key_exists("guarded", cfg)) {
        meta->guarded = joss_map_new();
        int64_t av = joss_unbox_value(joss_map_get_i64(cfg, "guarded"));
        if (av && joss_is_arr_ptr(av)) {
            void* arr = (void*)(uintptr_t)av;
            for (int64_t i = 0; i < joss_arr_len(arr); i++) {
                const char* item = joss_model_cstr((const void*)(uintptr_t)joss_arr_get_i64(arr, i));
                if (item && *item) joss_map_set_i64(meta->guarded, item, 1);
            }
        }
    }
    if (joss_array_key_exists("hidden", cfg)) {
        int64_t av = joss_unbox_value(joss_map_get_i64(cfg, "hidden"));
        if (av && joss_is_arr_ptr(av)) {
            void* arr = (void*)(uintptr_t)av;
            for (int64_t i = 0; i < joss_arr_len(arr); i++) {
                const char* item = joss_model_cstr((const void*)(uintptr_t)joss_arr_get_i64(arr, i));
                if (item && *item) joss_map_set_i64(meta->hidden, item, 1);
            }
        }
    }
    if (joss_array_key_exists("visible", cfg)) {
        int64_t av = joss_unbox_value(joss_map_get_i64(cfg, "visible"));
        if (av && joss_is_arr_ptr(av)) {
            void* arr = (void*)(uintptr_t)av;
            for (int64_t i = 0; i < joss_arr_len(arr); i++) {
                const char* item = joss_model_cstr((const void*)(uintptr_t)joss_arr_get_i64(arr, i));
                if (item && *item) joss_map_set_i64(meta->visible, item, 1);
            }
        }
    }
    if (joss_array_key_exists("casts", cfg)) {
        int64_t mv = joss_unbox_value(joss_map_get_i64(cfg, "casts"));
        if (mv && joss_is_map_ptr(mv)) {
            meta->casts = joss_model_map_clone((void*)(uintptr_t)mv);
        }
    }
    if (joss_array_key_exists("globalScopes", cfg)) {
        int64_t av = joss_unbox_value(joss_map_get_i64(cfg, "globalScopes"));
        if (av && joss_is_arr_ptr(av)) {
            void* arr = (void*)(uintptr_t)av;
            for (int64_t i = 0; i < joss_arr_len(arr); i++) {
                joss_arr_push_i64(meta->global_scopes, joss_arr_get_i64(arr, i));
            }
        }
    }
}

void fn_Model_registerMethod(const char* class_name_raw, const char* method_name_raw, void* fn_ptr, int64_t arity) {
    const char* cls = joss_model_cstr(class_name_raw);
    const char* mname = joss_model_cstr(method_name_raw);
    if (!cls || !*cls || !mname || !*mname || !fn_ptr) return;
    JossModelMeta* meta = joss_model_find_meta(cls);
    if (!meta) return;
    for (int i = 0; i < meta->method_count; i++) {
        if (meta->methods[i].name && strcmp(meta->methods[i].name, mname) == 0) {
            meta->methods[i].fn_ptr = fn_ptr;
            meta->methods[i].arity = arity;
            return;
        }
    }
    if (meta->method_count < JOSS_MODEL_MAX_METHODS) {
        meta->methods[meta->method_count].name = joss_model_strdup(mname);
        meta->methods[meta->method_count].fn_ptr = fn_ptr;
        meta->methods[meta->method_count].arity = arity;
        meta->method_count++;
    }
}

void* fn_Model_newInstance(const char* class_name_raw) {
    const char* cls = joss_model_cstr(class_name_raw);
    JossModelMeta* meta = joss_model_find_meta(cls);
    if (!meta) {
        fn_Model_registerClass(cls, "{}");
        meta = joss_model_find_meta(cls);
    }
    return joss_model_alloc(meta, false, false);
}

void* fn_Model_query(const char* class_name_raw) {
    const char* cls = joss_model_cstr(class_name_raw);
    JossModelMeta* meta = joss_model_find_meta(cls);
    if (!meta) {
        fn_Model_registerClass(cls, "{}");
        meta = joss_model_find_meta(cls);
    }
    return joss_model_alloc(meta, false, true);
}

/* Dynamic attribute access */
int64_t fn_Model_getAttr(void* self_raw, const char* name_raw) {
    JossModel* m = joss_as_model(self_raw);
    const char* name = joss_model_cstr(name_raw);
    if (!m || !name || !*name) return (int64_t)(uintptr_t)joss_box_value(0, 0);
    if (m->loaded && joss_array_key_exists(name, m->loaded)) {
        return joss_map_get_i64(m->relations, name);
    }
    int64_t val = (int64_t)(uintptr_t)joss_box_value(0, 0);
    if (m->attrs && joss_array_key_exists(name, m->attrs)) {
        val = joss_map_get_i64(m->attrs, name);
    }
    char studly[200];
    char accessor_name[256];
    joss_model_studly_case(name, studly, sizeof(studly));
    snprintf(accessor_name, sizeof(accessor_name), "get%sAttribute", studly);
    JossModelMethodEntry* acc = joss_model_find_method(m->meta, accessor_name);
    if (acc && acc->fn_ptr) {
        typedef int64_t (*AccessorFn)(void*, int64_t);
        AccessorFn fn = (AccessorFn)acc->fn_ptr;
        return fn(m, val);
    }
    if (joss_is_boxed_ptr(val)) {
        const JossBoxedValue* box = (const JossBoxedValue*)(uintptr_t)(uint64_t)val;
        uint32_t tag = (uint32_t)(box->header & 0xFFFFFFFFULL);
        if (tag == JOSS_TAG_INT) {
            return (int64_t)box->payload;
        }
    }
    return val;
}

void fn_Model_setAttr(void* self_raw, const char* name_raw, int64_t val) {
    JossModel* m = joss_as_model(self_raw);
    const char* name = joss_model_cstr(name_raw);
    if (!m || !name || !*name) return;
    char studly[200];
    char mutator_name[256];
    joss_model_studly_case(name, studly, sizeof(studly));
    snprintf(mutator_name, sizeof(mutator_name), "set%sAttribute", studly);
    JossModelMethodEntry* mut = joss_model_find_method(m->meta, mutator_name);
    if (mut && mut->fn_ptr) {
        typedef int64_t (*MutatorFn)(void*, int64_t);
        MutatorFn fn = (MutatorFn)mut->fn_ptr;
        val = fn(m, val);
    }
    val = joss_model_cast_value(m->meta, name, val);
    joss_map_set_i64(m->attrs, name, val);
}

/* Fill / ForceFill */
static void* joss_model_fill_internal(JossModel* m, void* attrs_raw, bool force) {
    if (!m || !m->meta) return m;
    int64_t u = joss_unbox_value((int64_t)(uintptr_t)attrs_raw);
    if (!u || !joss_is_map_ptr(u)) return m;
    void* attrs = (void*)(uintptr_t)u;
    int64_t n = joss_map_len(attrs);
    for (int64_t i = 0; i < n; i++) {
        const char* k = joss_map_key_at(attrs, i);
        int64_t v = joss_map_val_at(attrs, i);
        if (!k) continue;
        if (!force && !joss_model_mass_assignable(m->meta, k)) {
            fprintf(stderr, "MassAssignmentError: attribute '%s' is not mass-assignable on %s\n", k, m->meta->class_name);
            exit(1);
        }
        int64_t casted = joss_model_cast_value(m->meta, k, v);
        joss_map_set_i64(m->attrs, k, casted);
    }
    return m;
}

void* fn_Model_fill(void* self_raw, void* attrs_raw) {
    return joss_model_fill_internal(joss_as_model(self_raw), attrs_raw, false);
}

void* fn_Model_forceFill(void* self_raw, void* attrs_raw) {
    return joss_model_fill_internal(joss_as_model(self_raw), attrs_raw, true);
}

/* Query builder helpers */
static JossModel* joss_model_ensure_query(void* self_raw) {
    JossModel* m = joss_as_model(self_raw);
    if (!m) return NULL;
    if (m->is_query) return m;
    return joss_model_alloc(m->meta, false, true);
}

static void joss_model_add_where(JossModel* q, const char* conj, const char* col, const char* op, int64_t val) {
    if (!q || !joss_model_valid_ident(col)) return;
    JossStringBuilder* sb = joss_sb_new();
    if (joss_arr_len(q->wheres) > 0) {
        joss_sb_append(sb, conj);
        joss_sb_append(sb, " ");
    }
    joss_model_append_ident(sb, col);
    joss_sb_append(sb, " ");
    joss_sb_append(sb, op);
    joss_sb_append(sb, " ?");
    char* frag = joss_sb_to_string(sb);
    joss_sb_free(sb);
    joss_arr_push_str(q->wheres, frag);
    joss_arr_push_i64(q->bindings, val);
}

void* fn_Model_where(void* self_raw, const char* col_raw, int64_t arg2, int64_t arg3, int64_t has_arg3) {
    JossModel* q = joss_model_ensure_query(self_raw);
    const char* col = joss_model_cstr(col_raw);
    if (!q || !col || !*col) return q;
    if (has_arg3) {
        const char* op = joss_model_cstr((const void*)(uintptr_t)arg2);
        joss_model_add_where(q, "AND", col, (op && *op) ? op : "=", arg3);
    } else {
        joss_model_add_where(q, "AND", col, "=", arg2);
    }
    return q;
}

void* fn_Model_orWhere(void* self_raw, const char* col_raw, int64_t arg2, int64_t arg3, int64_t has_arg3) {
    JossModel* q = joss_model_ensure_query(self_raw);
    const char* col = joss_model_cstr(col_raw);
    if (!q || !col || !*col) return q;
    if (has_arg3) {
        const char* op = joss_model_cstr((const void*)(uintptr_t)arg2);
        joss_model_add_where(q, "OR", col, (op && *op) ? op : "=", arg3);
    } else {
        joss_model_add_where(q, "OR", col, "=", arg2);
    }
    return q;
}

void* fn_Model_whereIn(void* self_raw, const char* col_raw, void* arr_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    const char* col = joss_model_cstr(col_raw);
    if (!q || !joss_model_valid_ident(col)) return q;
    int64_t u = joss_unbox_value((int64_t)(uintptr_t)arr_raw);
    void* arr = (u && joss_is_arr_ptr(u)) ? (void*)(uintptr_t)u : NULL;
    int64_t n = arr ? joss_arr_len(arr) : 0;
    JossStringBuilder* sb = joss_sb_new();
    if (joss_arr_len(q->wheres) > 0) joss_sb_append(sb, "AND ");
    if (n == 0) {
        joss_sb_append(sb, "1 = 0");
    } else {
        joss_model_append_ident(sb, col);
        joss_sb_append(sb, " IN (");
        for (int64_t i = 0; i < n; i++) {
            if (i > 0) joss_sb_append(sb, ", ");
            joss_sb_append(sb, "?");
            joss_arr_push_i64(q->bindings, joss_arr_get_i64(arr, i));
        }
        joss_sb_append(sb, ")");
    }
    char* frag = joss_sb_to_string(sb);
    joss_sb_free(sb);
    joss_arr_push_str(q->wheres, frag);
    return q;
}

void* fn_Model_whereNull(void* self_raw, const char* col_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    const char* col = joss_model_cstr(col_raw);
    if (!q || !joss_model_valid_ident(col)) return q;
    JossStringBuilder* sb = joss_sb_new();
    if (joss_arr_len(q->wheres) > 0) joss_sb_append(sb, "AND ");
    joss_model_append_ident(sb, col);
    joss_sb_append(sb, " IS NULL");
    char* frag = joss_sb_to_string(sb);
    joss_sb_free(sb);
    joss_arr_push_str(q->wheres, frag);
    return q;
}

void* fn_Model_whereNotNull(void* self_raw, const char* col_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    const char* col = joss_model_cstr(col_raw);
    if (!q || !joss_model_valid_ident(col)) return q;
    JossStringBuilder* sb = joss_sb_new();
    if (joss_arr_len(q->wheres) > 0) joss_sb_append(sb, "AND ");
    joss_model_append_ident(sb, col);
    joss_sb_append(sb, " IS NOT NULL");
    char* frag = joss_sb_to_string(sb);
    joss_sb_free(sb);
    joss_arr_push_str(q->wheres, frag);
    return q;
}

void* fn_Model_orderBy(void* self_raw, const char* col_raw, const char* dir_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    const char* col = joss_model_cstr(col_raw);
    const char* dir = joss_model_cstr(dir_raw);
    if (!q || !joss_model_valid_ident(col)) return q;
    const char* safe_dir = (dir && strcasecmp(dir, "desc") == 0) ? "DESC" : "ASC";
    JossStringBuilder* sb = joss_sb_new();
    joss_model_append_ident(sb, col);
    joss_sb_append(sb, " ");
    joss_sb_append(sb, safe_dir);
    char* ord = joss_sb_to_string(sb);
    joss_sb_free(sb);
    joss_arr_push_str(q->orders, ord);
    return q;
}

void* fn_Model_orderByAsc(void* self_raw, const char* col_raw) {
    return fn_Model_orderBy(self_raw, col_raw, "ASC");
}

void* fn_Model_orderByDesc(void* self_raw, const char* col_raw) {
    return fn_Model_orderBy(self_raw, col_raw, "DESC");
}

void* fn_Model_limit(void* self_raw, int64_t n) {
    JossModel* q = joss_model_ensure_query(self_raw);
    if (q) q->limit_val = n;
    return q;
}

void* fn_Model_offset(void* self_raw, int64_t n) {
    JossModel* q = joss_model_ensure_query(self_raw);
    if (q) q->offset_val = n;
    return q;
}

void* fn_Model_withTrashed(void* self_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    if (q) q->soft_delete_mode = 1;
    return q;
}

void* fn_Model_onlyTrashed(void* self_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    if (q) q->soft_delete_mode = 2;
    return q;
}

void* fn_Model_withoutTrashed(void* self_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    if (q) q->soft_delete_mode = 0;
    return q;
}

void* fn_Model_withoutGlobalScope(void* self_raw, const char* name_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    const char* name = joss_model_cstr(name_raw);
    if (q && name && *name) joss_map_set_i64(q->excluded_global_scopes, name, 1);
    return q;
}

void* fn_Model_withoutGlobalScopes(void* self_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    if (q && q->meta && q->meta->global_scopes) {
        int64_t n = joss_arr_len(q->meta->global_scopes);
        for (int64_t i = 0; i < n; i++) {
            const char* s = joss_model_cstr((const void*)(uintptr_t)joss_arr_get_i64(q->meta->global_scopes, i));
            if (s && *s) joss_map_set_i64(q->excluded_global_scopes, s, 1);
        }
    }
    return q;
}

void* fn_Model_scope(void* self_raw, const char* name_raw, int64_t arg1, int64_t has_arg1) {
    JossModel* q = joss_model_ensure_query(self_raw);
    const char* name = joss_model_cstr(name_raw);
    if (!q || !q->meta || !name || !*name) return q;
    char studly[200];
    char mname[256];
    joss_model_studly_case(name, studly, sizeof(studly));
    snprintf(mname, sizeof(mname), "scope%s", studly);
    JossModelMethodEntry* entry = joss_model_find_method(q->meta, mname);
    if (entry && entry->fn_ptr) {
        if (has_arg1) {
            typedef void* (*ScopeFn1)(void*, void*, int64_t);
            ScopeFn1 fn = (ScopeFn1)entry->fn_ptr;
            void* r = fn(q, q, arg1);
            if (r && joss_as_model(r)) return r;
        } else {
            typedef void* (*ScopeFn0)(void*, void*);
            ScopeFn0 fn = (ScopeFn0)entry->fn_ptr;
            void* r = fn(q, q);
            if (r && joss_as_model(r)) return r;
        }
    }
    return q;
}

void* fn_Model_with(void* self_raw, int64_t rels_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    if (!q) return q;
    int64_t u = joss_unbox_value(rels_raw);
    if (u && joss_is_arr_ptr(u)) {
        void* arr = (void*)(uintptr_t)u;
        for (int64_t i = 0; i < joss_arr_len(arr); i++) {
            const char* r = joss_model_cstr((const void*)(uintptr_t)joss_arr_get_i64(arr, i));
            if (r && *r) joss_arr_push_str(q->with_rels, r);
        }
    } else {
        const char* r = joss_model_cstr((const void*)(uintptr_t)rels_raw);
        if (r && *r) joss_arr_push_str(q->with_rels, r);
    }
    return q;
}

/* Forward declarations for relations & eager loading */
void* fn_Model_get(void* self_raw);
static void joss_model_eager_load(void* parents_arr, void* rel_names, bool missing_only);

static void joss_model_build_where_clause(JossModel* q, JossStringBuilder* sb) {
    bool has_where = false;
    int64_t wn = joss_arr_len(q->wheres);
    if (wn > 0) {
        joss_sb_append(sb, " WHERE ");
        for (int64_t i = 0; i < wn; i++) {
            if (i > 0) joss_sb_append(sb, " ");
            joss_sb_append(sb, joss_arr_get_str(q->wheres, i));
        }
        has_where = true;
    }
    if (q->meta && q->meta->soft_deletes && q->soft_delete_mode != 1) {
        joss_sb_append(sb, has_where ? " AND " : " WHERE ");
        joss_model_append_ident(sb, q->meta->deleted_at);
        joss_sb_append(sb, (q->soft_delete_mode == 2) ? " IS NOT NULL" : " IS NULL");
    }
}

static void* joss_model_get_belongs_to_many(JossModel* q) {
    void* out = joss_arr_new(4);
    if (!q || !q->relation || !q->relation->parent || !q->meta) return out;
    JossModelRelation* rel = q->relation;
    int64_t pkey = joss_map_get_i64(rel->parent->attrs, rel->local_key);
    if (pkey == 0 || joss_is_null(pkey)) return out;

    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "SELECT * FROM ");
    joss_model_append_ident(sb, rel->pivot_table);
    joss_sb_append(sb, " WHERE ");
    joss_model_append_ident(sb, rel->foreign_pivot_key);
    joss_sb_append(sb, " = ?");
    char* psql = joss_sb_to_string(sb);
    joss_sb_free(sb);
    void* pbind = joss_arr_new(1);
    joss_arr_push_i64(pbind, pkey);
    void* pivot_rows = joss_model_db_query(psql, pbind);
    free(psql);

    int64_t pn = pivot_rows ? joss_arr_len(pivot_rows) : 0;
    if (pn == 0) return out;
    void* rel_ids = joss_arr_new(pn);
    for (int64_t i = 0; i < pn; i++) {
        void* prow = (void*)(uintptr_t)joss_arr_get_i64(pivot_rows, i);
        joss_arr_push_i64(rel_ids, joss_map_get_i64(prow, rel->related_pivot_key));
    }

    JossModel* rq = joss_model_alloc(q->meta, false, true);
    fn_Model_whereIn(rq, rel->related_key, rel_ids);
    void* related_models = fn_Model_get(rq);
    int64_t rn = related_models ? joss_arr_len(related_models) : 0;
    for (int64_t i = 0; i < rn; i++) {
        JossModel* rm = joss_as_model((void*)(uintptr_t)joss_arr_get_i64(related_models, i));
        if (!rm) continue;
        int64_t rkey = joss_map_get_i64(rm->attrs, rel->related_key);
        for (int64_t j = 0; j < pn; j++) {
            void* prow = (void*)(uintptr_t)joss_arr_get_i64(pivot_rows, j);
            if (joss_model_values_equal(joss_map_get_i64(prow, rel->related_pivot_key), rkey)) {
                joss_map_set_i64(rm->attrs, "pivot", (int64_t)(uintptr_t)prow);
                break;
            }
        }
        joss_arr_push_i64(out, (int64_t)(uintptr_t)rm);
    }
    return out;
}

void* fn_Model_get(void* self_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    void* out = joss_arr_new(4);
    if (!q || !q->meta) return out;
    if (q->relation && q->relation->kind == JOSS_REL_BELONGS_TO_MANY) {
        return joss_model_get_belongs_to_many(q);
    }
    joss_model_apply_global_scopes(q);

    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "SELECT * FROM ");
    joss_model_append_ident(sb, q->meta->table);
    joss_model_build_where_clause(q, sb);
    int64_t on = joss_arr_len(q->orders);
    if (on > 0) {
        joss_sb_append(sb, " ORDER BY ");
        for (int64_t i = 0; i < on; i++) {
            if (i > 0) joss_sb_append(sb, ", ");
            joss_sb_append(sb, joss_arr_get_str(q->orders, i));
        }
    }
    if (q->limit_val >= 0) {
        joss_sb_append(sb, " LIMIT ");
        joss_sb_append_i64(sb, q->limit_val);
    }
    if (q->offset_val >= 0) {
        joss_sb_append(sb, " OFFSET ");
        joss_sb_append_i64(sb, q->offset_val);
    }
    char* sql = joss_sb_to_string(sb);
    joss_sb_free(sb);

    void* rows = joss_model_db_query(sql, q->bindings);
    free(sql);
    int64_t rn = rows ? joss_arr_len(rows) : 0;
    for (int64_t i = 0; i < rn; i++) {
        void* row = (void*)(uintptr_t)joss_arr_get_i64(rows, i);
        JossModel* inst = joss_model_hydrate(q->meta, row);
        joss_arr_push_i64(out, (int64_t)(uintptr_t)inst);
    }
    if (q->with_rels && joss_arr_len(q->with_rels) > 0 && joss_arr_len(out) > 0) {
        joss_model_eager_load(out, q->with_rels, false);
    }
    return out;
}

void* fn_Model_all(const char* class_name_raw) {
    return fn_Model_get(fn_Model_query(class_name_raw));
}

void* fn_Model_first(void* self_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    if (!q) return NULL;
    if (q->relation && q->relation->kind == JOSS_REL_BELONGS_TO_MANY) {
        void* items = joss_model_get_belongs_to_many(q);
        return (items && joss_arr_len(items) > 0) ? (void*)(uintptr_t)joss_arr_get_i64(items, 0) : NULL;
    }
    q->limit_val = 1;
    void* items = fn_Model_get(q);
    if (!items || joss_arr_len(items) == 0) return NULL;
    return (void*)(uintptr_t)joss_arr_get_i64(items, 0);
}

void* fn_Model_find(void* self_raw, int64_t id_val) {
    JossModel* q = joss_model_ensure_query(self_raw);
    if (!q || !q->meta) return NULL;
    fn_Model_where(q, q->meta->primary_key, id_val, 0, 0);
    return fn_Model_first(q);
}

int64_t fn_Model_count(void* self_raw) {
    JossModel* q = joss_model_ensure_query(self_raw);
    if (!q || !q->meta) return 0;
    joss_model_apply_global_scopes(q);
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "SELECT COUNT(*) AS cnt FROM ");
    joss_model_append_ident(sb, q->meta->table);
    joss_model_build_where_clause(q, sb);
    char* sql = joss_sb_to_string(sb);
    joss_sb_free(sb);
    void* rows = joss_model_db_query(sql, q->bindings);
    free(sql);
    if (!rows || joss_arr_len(rows) == 0) return 0;
    void* row = (void*)(uintptr_t)joss_arr_get_i64(rows, 0);
    return row ? joss_intval(joss_map_get_i64(row, "cnt")) : 0;
}

int64_t fn_Model_exists(void* self_raw) {
    return fn_Model_count(self_raw) > 0 ? 1 : 0;
}

/* Persistence: save, create, firstOrNew, firstOrCreate, updateOrCreate, refresh, delete, restore, forceDelete */
int64_t fn_Model_save(void* self_raw) {
    JossModel* m = joss_as_model(self_raw);
    if (!m || !m->meta || m->deleted) return 0;
    void* changes = joss_model_compute_changes(m);
    if (m->exists && joss_map_len(changes) == 0) {
        m->last_changes = joss_map_new();
        return 1;
    }
    bool creating = !m->exists;
    if (!joss_model_run_hook(m, "saving")) return 0;
    if (creating) {
        if (!joss_model_run_hook(m, "creating")) return 0;
    } else {
        if (!joss_model_run_hook(m, "updating")) return 0;
    }

    if (m->exists) {
        /* Dirty partial UPDATE */
        void* dirty = joss_model_compute_changes(m);
        int64_t pk_val = joss_map_get_i64(m->attrs, m->meta->primary_key);
        if (pk_val == 0 || joss_is_null(pk_val)) return 0;
        JossStringBuilder* sb = joss_sb_new();
        void* binds = joss_arr_new(4);
        joss_sb_append(sb, "UPDATE ");
        joss_model_append_ident(sb, m->meta->table);
        joss_sb_append(sb, " SET ");
        int64_t n = joss_map_len(dirty);
        int64_t set_cnt = 0;
        for (int64_t i = 0; i < n; i++) {
            const char* k = joss_map_key_at(dirty, i);
            if (!k || strcmp(k, m->meta->primary_key) == 0 || !joss_model_valid_ident(k)) continue;
            if (set_cnt > 0) joss_sb_append(sb, ", ");
            joss_model_append_ident(sb, k);
            joss_sb_append(sb, " = ?");
            joss_arr_push_i64(binds, joss_model_db_value(m->meta, k, joss_map_val_at(dirty, i)));
            set_cnt++;
        }
        if (set_cnt == 0) {
            joss_sb_free(sb);
            m->last_changes = joss_map_new();
            return 1;
        }
        joss_sb_append(sb, " WHERE ");
        joss_model_append_ident(sb, m->meta->primary_key);
        joss_sb_append(sb, " = ?");
        joss_arr_push_i64(binds, pk_val);
        char* sql = joss_sb_to_string(sb);
        joss_sb_free(sb);
        int64_t aff = joss___joss_db_execute(sql, binds);
        free(sql);
        if (aff < 0) return 0;
        m->last_changes = joss_model_map_clone(dirty);
    } else {
        /* INSERT */
        JossStringBuilder* cols = joss_sb_new();
        JossStringBuilder* ph = joss_sb_new();
        void* binds = joss_arr_new(4);
        int64_t n = joss_map_len(m->attrs);
        int64_t col_cnt = 0;
        for (int64_t i = 0; i < n; i++) {
            const char* k = joss_map_key_at(m->attrs, i);
            if (!k || k[0] == '_' || !joss_model_valid_ident(k)) continue;
            if (col_cnt > 0) { joss_sb_append(cols, ", "); joss_sb_append(ph, ", "); }
            joss_model_append_ident(cols, k);
            joss_sb_append(ph, "?");
            joss_arr_push_i64(binds, joss_model_db_value(m->meta, k, joss_map_val_at(m->attrs, i)));
            col_cnt++;
        }
        JossStringBuilder* sb = joss_sb_new();
        joss_sb_append(sb, "INSERT INTO ");
        joss_model_append_ident(sb, m->meta->table);
        char* cs = joss_sb_to_string(cols);
        char* ps = joss_sb_to_string(ph);
        joss_sb_free(cols);
        joss_sb_free(ph);
        joss_sb_append(sb, " ("); joss_sb_append(sb, cs); joss_sb_append(sb, ") VALUES ("); joss_sb_append(sb, ps); joss_sb_append(sb, ")");
        free(cs);
        free(ps);
        char* sql = joss_sb_to_string(sb);
        joss_sb_free(sb);
        int64_t res = joss___joss_db_execute(sql, binds);
        free(sql);
        if (res < 0) return 0;
        if (m->meta->incrementing && !joss_array_key_exists(m->meta->primary_key, m->attrs)) {
            int64_t lid = joss_model_last_insert_id();
            joss_map_set_i64(m->attrs, m->meta->primary_key, (int64_t)(uintptr_t)joss_box_i64(lid));
        }
        m->exists = true;
        m->last_changes = joss_model_map_clone(m->attrs);
    }

    m->original = joss_model_map_clone(m->attrs);
    if (creating) {
        (void)joss_model_run_hook(m, "created");
    } else {
        (void)joss_model_run_hook(m, "updated");
    }
    (void)joss_model_run_hook(m, "saved");
    return 1;
}

void* fn_Model_create(const char* class_name_raw, void* attrs_raw) {
    void* inst = fn_Model_newInstance(class_name_raw);
    fn_Model_fill(inst, attrs_raw);
    fn_Model_save(inst);
    return inst;
}

static void* joss_model_first_or_write(const char* class_name_raw, void* search_raw, void* vals_raw, bool do_save, bool do_update) {
    JossModel* q = joss_as_model(fn_Model_query(class_name_raw));
    int64_t su = joss_unbox_value((int64_t)(uintptr_t)search_raw);
    void* smap = (su && joss_is_map_ptr(su)) ? (void*)(uintptr_t)su : NULL;
    if (smap) {
        int64_t n = joss_map_len(smap);
        for (int64_t i = 0; i < n; i++) {
            const char* k = joss_map_key_at(smap, i);
            int64_t v = joss_map_val_at(smap, i);
            if (k) fn_Model_where(q, k, v, 0, 0);
        }
    }
    JossModel* found = joss_as_model(fn_Model_first(q));
    if (found) {
        if (do_update && vals_raw) {
            fn_Model_fill(found, vals_raw);
            fn_Model_save(found);
        }
        return found;
    }
    JossModel* created = joss_as_model(fn_Model_newInstance(class_name_raw));
    if (smap) fn_Model_fill(created, smap);
    if (vals_raw) fn_Model_fill(created, vals_raw);
    if (do_save) fn_Model_save(created);
    return created;
}

void* fn_Model_firstOrNew(const char* class_name_raw, void* search_raw, void* vals_raw) {
    return joss_model_first_or_write(class_name_raw, search_raw, vals_raw, false, false);
}

void* fn_Model_firstOrCreate(const char* class_name_raw, void* search_raw, void* vals_raw) {
    return joss_model_first_or_write(class_name_raw, search_raw, vals_raw, true, false);
}

void* fn_Model_updateOrCreate(const char* class_name_raw, void* search_raw, void* vals_raw) {
    return joss_model_first_or_write(class_name_raw, search_raw, vals_raw, true, true);
}

void* fn_Model_refresh(void* self_raw) {
    JossModel* m = joss_as_model(self_raw);
    if (!m || !m->meta || !m->exists) return m;
    int64_t pk = joss_map_get_i64(m->attrs, m->meta->primary_key);
    JossModel* q = joss_model_alloc(m->meta, false, true);
    q->soft_delete_mode = 1;
    JossModel* fresh = joss_as_model(fn_Model_find(q, pk));
    if (fresh) {
        m->attrs = joss_model_map_clone(fresh->attrs);
        m->original = joss_model_map_clone(fresh->original);
        m->last_changes = joss_map_new();
        m->relations = joss_map_new();
        m->loaded = joss_map_new();
    }
    return m;
}

static void joss_model_utc_now(char* buf, size_t cap) {
    time_t now = time(NULL);
    struct tm tmv;
#if defined(_WIN32)
    gmtime_s(&tmv, &now);
#else
    gmtime_r(&now, &tmv);
#endif
    snprintf(buf, cap, "%04d-%02d-%02d %02d:%02d:%02d",
             tmv.tm_year + 1900, tmv.tm_mon + 1, tmv.tm_mday,
             tmv.tm_hour, tmv.tm_min, tmv.tm_sec);
}

int64_t fn_Model_delete(void* self_raw) {
    JossModel* m = joss_as_model(self_raw);
    if (!m || !m->meta || !m->exists || m->deleted) return 0;
    if (!joss_model_run_hook(m, "deleting")) return 0;
    int64_t pk = joss_map_get_i64(m->attrs, m->meta->primary_key);
    if (pk == 0 || joss_is_null(pk)) return 0;

    if (m->meta->soft_deletes) {
        char now_buf[32];
        joss_model_utc_now(now_buf, sizeof(now_buf));
        JossStringBuilder* sb = joss_sb_new();
        joss_sb_append(sb, "UPDATE ");
        joss_model_append_ident(sb, m->meta->table);
        joss_sb_append(sb, " SET ");
        joss_model_append_ident(sb, m->meta->deleted_at);
        joss_sb_append(sb, " = ? WHERE ");
        joss_model_append_ident(sb, m->meta->primary_key);
        joss_sb_append(sb, " = ?");
        char* sql = joss_sb_to_string(sb);
        joss_sb_free(sb);
        void* binds = joss_arr_new(2);
        joss_arr_push_str(binds, now_buf);
        joss_arr_push_i64(binds, pk);
        (void)joss___joss_db_execute(sql, binds);
        free(sql);
        joss_map_set_i64(m->attrs, m->meta->deleted_at, (int64_t)(uintptr_t)joss_box_str(joss_req_strdup(now_buf)));
        m->original = joss_model_map_clone(m->attrs);
        m->deleted = true;
        (void)joss_model_run_hook(m, "deleted");
        return 1;
    }

    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "DELETE FROM ");
    joss_model_append_ident(sb, m->meta->table);
    joss_sb_append(sb, " WHERE ");
    joss_model_append_ident(sb, m->meta->primary_key);
    joss_sb_append(sb, " = ?");
    char* sql = joss_sb_to_string(sb);
    joss_sb_free(sb);
    void* binds = joss_arr_new(1);
    joss_arr_push_i64(binds, pk);
    (void)joss___joss_db_execute(sql, binds);
    free(sql);
    m->deleted = true;
    m->exists = false;
    (void)joss_model_run_hook(m, "deleted");
    return 1;
}

int64_t fn_Model_forceDelete(void* self_raw) {
    JossModel* m = joss_as_model(self_raw);
    if (!m || !m->meta) return 0;
    int64_t pk = joss_map_get_i64(m->attrs, m->meta->primary_key);
    if (pk == 0 || joss_is_null(pk)) return 0;
    if (!joss_model_run_hook(m, "deleting")) return 0;
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "DELETE FROM ");
    joss_model_append_ident(sb, m->meta->table);
    joss_sb_append(sb, " WHERE ");
    joss_model_append_ident(sb, m->meta->primary_key);
    joss_sb_append(sb, " = ?");
    char* sql = joss_sb_to_string(sb);
    joss_sb_free(sb);
    void* binds = joss_arr_new(1);
    joss_arr_push_i64(binds, pk);
    (void)joss___joss_db_execute(sql, binds);
    free(sql);
    m->deleted = true;
    m->exists = false;
    (void)joss_model_run_hook(m, "deleted");
    return 1;
}

int64_t fn_Model_restore(void* self_raw) {
    JossModel* m = joss_as_model(self_raw);
    if (!m || !m->meta || !m->meta->soft_deletes || !m->exists) return 0;
    int64_t pk = joss_map_get_i64(m->attrs, m->meta->primary_key);
    if (pk == 0 || joss_is_null(pk)) return 0;
    if (!joss_model_run_hook(m, "restoring")) return 0;
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "UPDATE ");
    joss_model_append_ident(sb, m->meta->table);
    joss_sb_append(sb, " SET ");
    joss_model_append_ident(sb, m->meta->deleted_at);
    joss_sb_append(sb, " = ? WHERE ");
    joss_model_append_ident(sb, m->meta->primary_key);
    joss_sb_append(sb, " = ?");
    char* sql = joss_sb_to_string(sb);
    joss_sb_free(sb);
    void* binds = joss_arr_new(2);
    joss_arr_push_i64(binds, 0); /* SQL NULL */
    joss_arr_push_i64(binds, pk);
    (void)joss___joss_db_execute(sql, binds);
    free(sql);
    joss_map_set_i64(m->attrs, m->meta->deleted_at, (int64_t)(uintptr_t)joss_box_value(0, 0));
    m->original = joss_model_map_clone(m->attrs);
    m->deleted = false;
    (void)joss_model_run_hook(m, "restored");
    return 1;
}

/* State inspection & serialization */
int64_t fn_Model_isDirty(void* self_raw, const char* col_raw) {
    JossModel* m = joss_as_model(self_raw);
    if (!m) return 0;
    void* ch = joss_model_compute_changes(m);
    const char* col = joss_model_cstr(col_raw);
    if (col && *col) return joss_array_key_exists(col, ch) ? 1 : 0;
    return joss_map_len(ch) > 0 ? 1 : 0;
}

int64_t fn_Model_isClean(void* self_raw, const char* col_raw) {
    return fn_Model_isDirty(self_raw, col_raw) ? 0 : 1;
}

int64_t fn_Model_wasChanged(void* self_raw, const char* col_raw) {
    JossModel* m = joss_as_model(self_raw);
    if (!m || !m->last_changes) return 0;
    const char* col = joss_model_cstr(col_raw);
    if (col && *col) return joss_array_key_exists(col, m->last_changes) ? 1 : 0;
    return joss_map_len(m->last_changes) > 0 ? 1 : 0;
}

int64_t fn_Model_getOriginal(void* self_raw, const char* col_raw) {
    JossModel* m = joss_as_model(self_raw);
    if (!m || !m->original) return (int64_t)(uintptr_t)joss_map_new();
    const char* col = joss_model_cstr(col_raw);
    if (col && *col) return joss_map_get_i64(m->original, col);
    return (int64_t)(uintptr_t)joss_model_map_clone(m->original);
}

void* fn_Model_getChanges(void* self_raw) {
    return joss_model_compute_changes(joss_as_model(self_raw));
}

void* fn_Model_toMap(void* self_raw) {
    JossModel* m = joss_as_model(self_raw);
    void* out = joss_map_new();
    if (!m || !m->meta) return out;
    bool has_visible = (m->meta->visible && joss_map_len(m->meta->visible) > 0);
    int64_t n = joss_map_len(m->attrs);
    for (int64_t i = 0; i < n; i++) {
        const char* k = joss_map_key_at(m->attrs, i);
        if (!k || k[0] == '_') continue;
        if (has_visible && !joss_array_key_exists(k, m->meta->visible)) continue;
        if (m->meta->hidden && joss_array_key_exists(k, m->meta->hidden)) continue;
        int64_t val = fn_Model_getAttr(m, k);
        joss_map_set_i64(out, k, val);
    }
    if (m->loaded) {
        int64_t ln = joss_map_len(m->loaded);
        for (int64_t i = 0; i < ln; i++) {
            const char* rk = joss_map_key_at(m->loaded, i);
            if (!rk) continue;
            int64_t rv = joss_map_get_i64(m->relations, rk);
            int64_t ru = joss_unbox_value(rv);
            if (joss_is_model_ptr(ru)) {
                joss_map_set_i64(out, rk, (int64_t)(uintptr_t)fn_Model_toMap((void*)(uintptr_t)ru));
            } else if (joss_is_arr_ptr(ru)) {
                void* arr = (void*)(uintptr_t)ru;
                void* mapped = joss_arr_new(joss_arr_len(arr));
                for (int64_t j = 0; j < joss_arr_len(arr); j++) {
                    int64_t el = joss_arr_get_i64(arr, j);
                    if (joss_is_model_ptr(el)) {
                        joss_arr_push_i64(mapped, (int64_t)(uintptr_t)fn_Model_toMap((void*)(uintptr_t)joss_unbox_value(el)));
                    } else {
                        joss_arr_push_i64(mapped, el);
                    }
                }
                joss_map_set_i64(out, rk, (int64_t)(uintptr_t)mapped);
            } else {
                joss_map_set_i64(out, rk, rv);
            }
        }
    }
    return out;
}

const char* fn_Model_toJSON(void* self_raw) {
    void* mp = fn_Model_toMap(self_raw);
    return joss_json_encode((int64_t)(uintptr_t)mp);
}

/* Relations */
static void* joss_model_make_rel(void* parent_raw, JossModelRelKind kind,
                                 const char* related_raw, const char* fk_raw, const char* lk_raw) {
    JossModel* parent = joss_as_model(parent_raw);
    const char* rel_cls = joss_model_cstr(related_raw);
    JossModelMeta* rmeta = joss_model_find_meta(rel_cls);
    if (!parent || !parent->meta || !rmeta) return NULL;
    JossModel* q = joss_model_alloc(rmeta, false, true);
    JossModelRelation* rel = (JossModelRelation*)calloc(1, sizeof(JossModelRelation));
    rel->kind = kind;
    rel->parent = parent;

    char def_fk[256];
    const char* fk = joss_model_cstr(fk_raw);
    const char* lk = joss_model_cstr(lk_raw);

    if (kind == JOSS_REL_BELONGS_TO) {
        if (!fk || !*fk) {
            joss_model_snake_case(rmeta->class_name, def_fk, sizeof(def_fk) - 4);
            strcat(def_fk, "_id");
            fk = def_fk;
        }
        if (!lk || !*lk) lk = rmeta->primary_key;
        rel->foreign_key = joss_model_strdup(fk);
        rel->local_key = joss_model_strdup(lk);
        int64_t v = joss_map_get_i64(parent->attrs, fk);
        if (v == 0 || joss_is_null(v)) fn_Model_whereNull(q, lk);
        else fn_Model_where(q, lk, v, 0, 0);
    } else {
        if (!fk || !*fk) {
            joss_model_snake_case(parent->meta->class_name, def_fk, sizeof(def_fk) - 4);
            strcat(def_fk, "_id");
            fk = def_fk;
        }
        if (!lk || !*lk) lk = parent->meta->primary_key;
        rel->foreign_key = joss_model_strdup(fk);
        rel->local_key = joss_model_strdup(lk);
        int64_t v = joss_map_get_i64(parent->attrs, lk);
        fn_Model_where(q, fk, v, 0, 0);
    }
    q->relation = rel;
    return q;
}

void* fn_Model_hasOne(void* self_raw, const char* rel_raw, const char* fk_raw, const char* lk_raw) {
    return joss_model_make_rel(self_raw, JOSS_REL_HAS_ONE, rel_raw, fk_raw, lk_raw);
}

void* fn_Model_hasMany(void* self_raw, const char* rel_raw, const char* fk_raw, const char* lk_raw) {
    return joss_model_make_rel(self_raw, JOSS_REL_HAS_MANY, rel_raw, fk_raw, lk_raw);
}

void* fn_Model_belongsTo(void* self_raw, const char* rel_raw, const char* fk_raw, const char* lk_raw) {
    return joss_model_make_rel(self_raw, JOSS_REL_BELONGS_TO, rel_raw, fk_raw, lk_raw);
}

void* fn_Model_belongsToMany(void* self_raw, const char* rel_raw, const char* pivot_raw,
                             const char* fpk_raw, const char* rpk_raw, const char* lk_raw, const char* rk_raw) {
    JossModel* parent = joss_as_model(self_raw);
    const char* rel_cls = joss_model_cstr(rel_raw);
    JossModelMeta* rmeta = joss_model_find_meta(rel_cls);
    if (!parent || !parent->meta || !rmeta) return NULL;
    JossModel* q = joss_model_alloc(rmeta, false, true);
    JossModelRelation* rel = (JossModelRelation*)calloc(1, sizeof(JossModelRelation));
    rel->kind = JOSS_REL_BELONGS_TO_MANY;
    rel->parent = parent;

    char p_snake[128], r_snake[128], def_pivot[260], def_fpk[140], def_rpk[140];
    joss_model_snake_case(parent->meta->class_name, p_snake, sizeof(p_snake));
    joss_model_snake_case(rmeta->class_name, r_snake, sizeof(r_snake));
    if (strcmp(p_snake, r_snake) <= 0) snprintf(def_pivot, sizeof(def_pivot), "%s_%s", p_snake, r_snake);
    else snprintf(def_pivot, sizeof(def_pivot), "%s_%s", r_snake, p_snake);
    snprintf(def_fpk, sizeof(def_fpk), "%s_id", p_snake);
    snprintf(def_rpk, sizeof(def_rpk), "%s_id", r_snake);

    const char* pivot = joss_model_cstr(pivot_raw);
    const char* fpk = joss_model_cstr(fpk_raw);
    const char* rpk = joss_model_cstr(rpk_raw);
    const char* lk = joss_model_cstr(lk_raw);
    const char* rk = joss_model_cstr(rk_raw);

    rel->pivot_table = joss_model_strdup((pivot && *pivot) ? pivot : def_pivot);
    rel->foreign_pivot_key = joss_model_strdup((fpk && *fpk) ? fpk : def_fpk);
    rel->related_pivot_key = joss_model_strdup((rpk && *rpk) ? rpk : def_rpk);
    rel->local_key = joss_model_strdup((lk && *lk) ? lk : parent->meta->primary_key);
    rel->related_key = joss_model_strdup((rk && *rk) ? rk : rmeta->primary_key);
    q->relation = rel;
    return q;
}

int64_t fn_Model_attach(void* self_raw, int64_t ids_raw) {
    JossModel* q = joss_as_model(self_raw);
    if (!q || !q->relation || q->relation->kind != JOSS_REL_BELONGS_TO_MANY) return 0;
    JossModelRelation* rel = q->relation;
    int64_t pkey = joss_map_get_i64(rel->parent->attrs, rel->local_key);
    void* ids = joss_arr_new(2);
    int64_t u = joss_unbox_value(ids_raw);
    if (u && joss_is_arr_ptr(u)) ids = (void*)(uintptr_t)u;
    else joss_arr_push_i64(ids, ids_raw);

    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "INSERT INTO ");
    joss_model_append_ident(sb, rel->pivot_table);
    joss_sb_append(sb, " (");
    joss_model_append_ident(sb, rel->foreign_pivot_key);
    joss_sb_append(sb, ", ");
    joss_model_append_ident(sb, rel->related_pivot_key);
    joss_sb_append(sb, ") VALUES (?, ?)");
    char* sql = joss_sb_to_string(sb);
    joss_sb_free(sb);
    for (int64_t i = 0; i < joss_arr_len(ids); i++) {
        void* binds = joss_arr_new(2);
        joss_arr_push_i64(binds, pkey);
        joss_arr_push_i64(binds, joss_arr_get_i64(ids, i));
        (void)joss___joss_db_execute(sql, binds);
    }
    free(sql);
    return 1;
}

int64_t fn_Model_detach(void* self_raw, int64_t ids_raw, int64_t has_ids) {
    JossModel* q = joss_as_model(self_raw);
    if (!q || !q->relation || q->relation->kind != JOSS_REL_BELONGS_TO_MANY) return 0;
    JossModelRelation* rel = q->relation;
    int64_t pkey = joss_map_get_i64(rel->parent->attrs, rel->local_key);
    if (!has_ids) {
        JossStringBuilder* sb = joss_sb_new();
        joss_sb_append(sb, "DELETE FROM ");
        joss_model_append_ident(sb, rel->pivot_table);
        joss_sb_append(sb, " WHERE ");
        joss_model_append_ident(sb, rel->foreign_pivot_key);
        joss_sb_append(sb, " = ?");
        char* sql = joss_sb_to_string(sb);
        joss_sb_free(sb);
        void* binds = joss_arr_new(1);
        joss_arr_push_i64(binds, pkey);
        (void)joss___joss_db_execute(sql, binds);
        free(sql);
        return 1;
    }
    int64_t u = joss_unbox_value(ids_raw);
    void* ids = (u && joss_is_arr_ptr(u)) ? (void*)(uintptr_t)u : NULL;
    if (ids && joss_arr_len(ids) == 0) return 1;
    if (!ids) {
        ids = joss_arr_new(1);
        joss_arr_push_i64(ids, ids_raw);
    }
    JossStringBuilder* sb = joss_sb_new();
    joss_sb_append(sb, "DELETE FROM ");
    joss_model_append_ident(sb, rel->pivot_table);
    joss_sb_append(sb, " WHERE ");
    joss_model_append_ident(sb, rel->foreign_pivot_key);
    joss_sb_append(sb, " = ? AND ");
    joss_model_append_ident(sb, rel->related_pivot_key);
    joss_sb_append(sb, " = ?");
    char* sql = joss_sb_to_string(sb);
    joss_sb_free(sb);
    for (int64_t i = 0; i < joss_arr_len(ids); i++) {
        void* binds = joss_arr_new(2);
        joss_arr_push_i64(binds, pkey);
        joss_arr_push_i64(binds, joss_arr_get_i64(ids, i));
        (void)joss___joss_db_execute(sql, binds);
    }
    free(sql);
    return 1;
}

int64_t fn_Model_sync(void* self_raw, int64_t ids_raw) {
    if (!fn_Model_detach(self_raw, 0, 0)) return 0;
    return fn_Model_attach(self_raw, ids_raw);
}

static void joss_model_eager_load(void* parents_arr, void* rel_names, bool missing_only) {
    if (!parents_arr || !rel_names) return;
    int64_t pn = joss_arr_len(parents_arr);
    int64_t rn = joss_arr_len(rel_names);
    if (pn == 0 || rn == 0) return;

    for (int64_t r = 0; r < rn; r++) {
        const char* rname = joss_model_cstr((const void*)(uintptr_t)joss_arr_get_i64(rel_names, r));
        if (!rname || !*rname) continue;
        JossModel* first_p = joss_as_model((void*)(uintptr_t)joss_arr_get_i64(parents_arr, 0));
        if (!first_p || !first_p->meta) continue;
        JossModelMethodEntry* meth = joss_model_find_method(first_p->meta, rname);
        if (!meth || !meth->fn_ptr) continue;
        typedef void* (*RelFn)(void*);
        RelFn fn = (RelFn)meth->fn_ptr;
        JossModel* rel_q = joss_as_model(fn(first_p));
        if (!rel_q || !rel_q->relation) continue;
        JossModelRelation* rel = rel_q->relation;

        if (rel->kind == JOSS_REL_BELONGS_TO_MANY) {
            for (int64_t i = 0; i < pn; i++) {
                JossModel* p = joss_as_model((void*)(uintptr_t)joss_arr_get_i64(parents_arr, i));
                if (!p || (missing_only && joss_array_key_exists(rname, p->loaded))) continue;
                JossModel* pq = joss_as_model(fn(p));
                void* items = fn_Model_get(pq);
                joss_map_set_i64(p->relations, rname, (int64_t)(uintptr_t)items);
                joss_map_set_i64(p->loaded, rname, 1);
            }
            continue;
        }

        const char* pkey_col = (rel->kind == JOSS_REL_BELONGS_TO) ? rel->foreign_key : rel->local_key;
        const char* match_col = (rel->kind == JOSS_REL_BELONGS_TO) ? rel->local_key : rel->foreign_key;

        void* keys = joss_arr_new(pn);
        for (int64_t i = 0; i < pn; i++) {
            JossModel* p = joss_as_model((void*)(uintptr_t)joss_arr_get_i64(parents_arr, i));
            if (!p || (missing_only && joss_array_key_exists(rname, p->loaded))) continue;
            int64_t kv = joss_map_get_i64(p->attrs, pkey_col);
            if (kv != 0 && !joss_is_null(kv)) joss_arr_push_i64(keys, kv);
        }

        JossModel* batch_q = joss_model_alloc(rel_q->meta, false, true);
        fn_Model_whereIn(batch_q, match_col, keys);
        void* children = (joss_arr_len(keys) > 0) ? fn_Model_get(batch_q) : joss_arr_new(0);
        int64_t cn = children ? joss_arr_len(children) : 0;

        for (int64_t i = 0; i < pn; i++) {
            JossModel* p = joss_as_model((void*)(uintptr_t)joss_arr_get_i64(parents_arr, i));
            if (!p || (missing_only && joss_array_key_exists(rname, p->loaded))) continue;
            int64_t kv = joss_map_get_i64(p->attrs, pkey_col);
            if (rel->kind == JOSS_REL_HAS_MANY) {
                void* matched = joss_arr_new(2);
                for (int64_t j = 0; j < cn; j++) {
                    JossModel* ch = joss_as_model((void*)(uintptr_t)joss_arr_get_i64(children, j));
                    if (ch && joss_model_values_equal(joss_map_get_i64(ch->attrs, match_col), kv)) {
                        joss_arr_push_i64(matched, (int64_t)(uintptr_t)ch);
                    }
                }
                joss_map_set_i64(p->relations, rname, (int64_t)(uintptr_t)matched);
            } else {
                int64_t found_ch = (int64_t)(uintptr_t)joss_box_value(0, 0);
                for (int64_t j = 0; j < cn; j++) {
                    JossModel* ch = joss_as_model((void*)(uintptr_t)joss_arr_get_i64(children, j));
                    if (ch && joss_model_values_equal(joss_map_get_i64(ch->attrs, match_col), kv)) {
                        found_ch = (int64_t)(uintptr_t)ch;
                        break;
                    }
                }
                joss_map_set_i64(p->relations, rname, found_ch);
            }
            joss_map_set_i64(p->loaded, rname, 1);
        }
    }
}

void* fn_Model_load(void* self_raw, int64_t rels_raw) {
    JossModel* m = joss_as_model(self_raw);
    if (!m) return self_raw;
    void* parents = joss_arr_new(1);
    joss_arr_push_i64(parents, (int64_t)(uintptr_t)m);
    void* rels = joss_arr_new(1);
    int64_t u = joss_unbox_value(rels_raw);
    if (u && joss_is_arr_ptr(u)) rels = (void*)(uintptr_t)u;
    else joss_arr_push_str(rels, joss_model_cstr((const void*)(uintptr_t)rels_raw));
    joss_model_eager_load(parents, rels, false);
    return m;
}

void* fn_Model_loadMissing(void* self_raw, int64_t rels_raw) {
    JossModel* m = joss_as_model(self_raw);
    if (!m) return self_raw;
    void* parents = joss_arr_new(1);
    joss_arr_push_i64(parents, (int64_t)(uintptr_t)m);
    void* rels = joss_arr_new(1);
    int64_t u = joss_unbox_value(rels_raw);
    if (u && joss_is_arr_ptr(u)) rels = (void*)(uintptr_t)u;
    else joss_arr_push_str(rels, joss_model_cstr((const void*)(uintptr_t)rels_raw));
    joss_model_eager_load(parents, rels, true);
    return m;
}

