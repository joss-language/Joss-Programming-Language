//go:build ignore
#ifndef JOSS_RAII_H
#define JOSS_RAII_H

/* =========================================================================
 * Joss Modular RAII Auto-Cleanup Module (GCC / Clang / Zig CC)
 *
 * Guarantees automatic deterministic resource release on every scope exit
 * (including early returns), eliminating manual cleanup boilerplate for
 * new C11 runtime modules (DB drivers, Redis, HTTP, Crypto, Files, Scopes).
 * ========================================================================= */

#include <stdio.h>
#include <stdlib.h>

#if defined(__GNUC__) || defined(__clang__)
#define JOSS_CLEANUP(fn) __attribute__((cleanup(fn)))
#else
#define JOSS_CLEANUP(fn)
#endif

/* Define a reusable RAII destructor for any pointer resource type:
 *   JOSS_DEFINE_AUTO_CLEANUP(MYSQL_STMT*, joss_auto_mysql_stmt, mysql_stmt_close)
 * Usage:
 *   JOSS_AUTO(joss_auto_mysql_stmt) MYSQL_STMT* stmt = mysql_stmt_init(conn);
 */
#define JOSS_DEFINE_AUTO_CLEANUP(PtrType, GuardName, FreeFn) \
    static inline void GuardName##_cleanup(PtrType* _p) {    \
        if (_p && *_p) {                                     \
            FreeFn(*_p);                                     \
            *_p = NULL;                                      \
        }                                                    \
    }

#define JOSS_AUTO(GuardName) JOSS_CLEANUP(GuardName##_cleanup)

/* Built-in standard RAII destructors */
static inline void joss_auto_ptr_cleanup(void* pp) {
    void** slot = (void**)pp;
    if (slot && *slot) {
        free(*slot);
        *slot = NULL;
    }
}

static inline void joss_auto_file_cleanup(FILE** fp) {
    if (fp && *fp) {
        fclose(*fp);
        *fp = NULL;
    }
}

#define JOSS_AUTO_PTR  JOSS_CLEANUP(joss_auto_ptr_cleanup)
#define JOSS_AUTO_FILE JOSS_CLEANUP(joss_auto_file_cleanup)

/* Built-in Joss runtime RAII destructors (bound after joss_rt.h types) */
static inline void joss_auto_sb_cleanup(JossStringBuilder** sb) {
    if (sb && *sb) {
        joss_sb_free(*sb);
        *sb = NULL;
    }
}

static inline void joss_auto_buf_cleanup(JossByteBuffer** buf) {
    if (buf && *buf) {
        joss_buf_free(*buf);
        *buf = NULL;
    }
}

static inline void joss_auto_scope_cleanup(JossOwnedScope** sc) {
    if (sc && *sc) {
        joss_owned_scope_destroy(*sc);
        *sc = NULL;
    }
}

#define JOSS_AUTO_SB    JOSS_CLEANUP(joss_auto_sb_cleanup)
#define JOSS_AUTO_BUF   JOSS_CLEANUP(joss_auto_buf_cleanup)
#define JOSS_AUTO_SCOPE JOSS_CLEANUP(joss_auto_scope_cleanup)

#endif /* JOSS_RAII_H */
