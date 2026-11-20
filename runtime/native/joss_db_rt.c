//go:build ignore
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

static sqlite3* joss_get_sqlite(void) {
    if (g_joss_sqlite) return g_joss_sqlite;
    const char* db_path = getenv("DB_DATABASE");
    if (!db_path || !*db_path) {
        if (joss_file_exists("database.sqlite")) {
            db_path = "database.sqlite";
        } else {
            return NULL;
        }
    }
    if (sqlite3_open(db_path, &g_joss_sqlite) != SQLITE_OK) {
        if (g_joss_sqlite) {
            sqlite3_close(g_joss_sqlite);
            g_joss_sqlite = NULL;
        }
        return NULL;
    }
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
    if (changes == 0 && strncasecmp(sql, "INSERT", 6) == 0) {
        return (int64_t)sqlite3_last_insert_rowid(db);
    }
    return (int64_t)changes;
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
        if (affected == 0 && strncasecmp(sql, "INSERT", 6) == 0) {
            affected = (int64_t)mysql_stmt_insert_id(stmt);
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
int64_t joss___joss_db_is_connected(void) {
    switch (detect_db_engine()) {
#if defined(JOSS_HAS_SQLITE3)
        case JOSS_DB_SQLITE:   return joss_get_sqlite() != NULL ? 1 : 0;
#endif
#if defined(JOSS_HAS_LIBPQ)
        case JOSS_DB_POSTGRES: return joss_get_pg() != NULL ? 1 : 0;
#else
        case JOSS_DB_POSTGRES: return joss_wire_get_pg() != JOSS_WIRE_INVALID_SOCK ? 1 : 0;
#endif
#if defined(JOSS_HAS_MYSQL)
        case JOSS_DB_MYSQL:    return joss_get_mysql() != NULL ? 1 : 0;
#else
        case JOSS_DB_MYSQL:    return joss_wire_get_mysql() != JOSS_WIRE_INVALID_SOCK ? 1 : 0;
#endif
#if defined(JOSS_HAS_ODBC)
        case JOSS_DB_SQLSRV:   return joss_get_odbc() != SQL_NULL_HDBC ? 1 : 0;
#endif
        default:               return 0;
    }
}

void* joss___joss_db_query(const char* sql, void* bindings) {
    switch (detect_db_engine()) {
#if defined(JOSS_HAS_SQLITE3)
        case JOSS_DB_SQLITE:   return sqlite_query(sql, bindings);
#endif
#if defined(JOSS_HAS_LIBPQ)
        case JOSS_DB_POSTGRES: return pg_query(sql, bindings);
#else
        case JOSS_DB_POSTGRES: return pg_wire_query(sql, bindings);
#endif
#if defined(JOSS_HAS_MYSQL)
        case JOSS_DB_MYSQL:    return mysql_query_native(sql, bindings);
#else
        case JOSS_DB_MYSQL:    return mysql_wire_query(sql, bindings);
#endif
#if defined(JOSS_HAS_ODBC)
        case JOSS_DB_SQLSRV:   return odbc_query(sql, bindings);
#endif
        default:               return joss_arr_new(0);
    }
}

int64_t joss___joss_db_execute(const char* sql, void* bindings) {
    switch (detect_db_engine()) {
#if defined(JOSS_HAS_SQLITE3)
        case JOSS_DB_SQLITE:   return sqlite_execute(sql, bindings);
#endif
#if defined(JOSS_HAS_LIBPQ)
        case JOSS_DB_POSTGRES: return pg_execute(sql, bindings);
#else
        case JOSS_DB_POSTGRES: return pg_wire_execute(sql, bindings);
#endif
#if defined(JOSS_HAS_MYSQL)
        case JOSS_DB_MYSQL:    return mysql_execute_native(sql, bindings);
#else
        case JOSS_DB_MYSQL:    return mysql_wire_execute(sql, bindings);
#endif
#if defined(JOSS_HAS_ODBC)
        case JOSS_DB_SQLSRV:   return odbc_execute(sql, bindings);
#endif
        default:               return 0;
    }
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
