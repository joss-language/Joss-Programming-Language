package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"net/url"
	"strconv"
	"strings"
)

const postgresSQLDriver = "joss-postgres"

// postgresRebindingDriver keeps the public database API portable. Joss queries use
// '?' placeholders, while PostgreSQL requires '$1', '$2', ... placeholders.
type postgresRebindingDriver struct {
	base driver.Driver
}

func (d *postgresRebindingDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &postgresRebindingConn{Conn: conn}, nil
}

type postgresRebindingConn struct {
	driver.Conn
}

func (c *postgresRebindingConn) Prepare(query string) (driver.Stmt, error) {
	return c.Conn.Prepare(rebindPostgres(query))
}

func (c *postgresRebindingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if preparer, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return preparer.PrepareContext(ctx, rebindPostgres(query))
	}
	return c.Prepare(query)
}

func (c *postgresRebindingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if execer, ok := c.Conn.(driver.ExecerContext); ok {
		return execer.ExecContext(ctx, rebindPostgres(query), args)
	}
	return nil, driver.ErrSkip
}

func (c *postgresRebindingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if queryer, ok := c.Conn.(driver.QueryerContext); ok {
		return queryer.QueryContext(ctx, rebindPostgres(query), args)
	}
	return nil, driver.ErrSkip
}

func (c *postgresRebindingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if beginner, ok := c.Conn.(driver.ConnBeginTx); ok {
		return beginner.BeginTx(ctx, opts)
	}
	return c.Conn.Begin()
}

func (c *postgresRebindingConn) Ping(ctx context.Context) error {
	if pinger, ok := c.Conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c *postgresRebindingConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

func (c *postgresRebindingConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *postgresRebindingConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func normalizeDatabaseDriver(driverName string) string {
	switch strings.ToLower(strings.TrimSpace(driverName)) {
	case "postgres", "postgresql", "pgx":
		return "postgres"
	case "sqlite", "sqlite3":
		return "sqlite"
	case "sqlserver", "mssql", "mssqlserver":
		return "sqlserver"
	default:
		return "mysql"
	}
}

func sqlDriverName(driverName string) string {
	normalized := normalizeDatabaseDriver(driverName)
	if normalized == "postgres" {
		return postgresSQLDriver
	}
	if normalized == "sqlserver" {
		return sqlServerDriverName
	}
	return normalized
}

// OpenConfiguredDatabase opens SQLite, MySQL, PostgreSQL or SQL Server using environment keys.
func OpenConfiguredDatabase(driverName string, env map[string]string) (*sql.DB, error) {
	driverName = normalizeDatabaseDriver(driverName)
	if driverName == "sqlite" {
		path := strings.Trim(strings.TrimSpace(env["DB_PATH"]), "\"'")
		if path == "" {
			path = "database.sqlite"
		}
		return sql.Open("sqlite", path)
	}
	if driverName == "sqlserver" {
		return openSQLServerDatabase(env)
	}
	host := strings.TrimSpace(env["DB_HOST"])
	if driverName == "postgres" {
		if !strings.Contains(host, ":") {
			host += ":5432"
		}
		sslMode := strings.TrimSpace(env["DB_SSLMODE"])
		if sslMode == "" {
			sslMode = "disable"
		}
		dsn := &url.URL{Scheme: "postgres", User: url.UserPassword(env["DB_USER"], env["DB_PASS"]), Host: host, Path: "/" + env["DB_NAME"]}
		query := dsn.Query()
		query.Set("sslmode", sslMode)
		dsn.RawQuery = query.Encode()
		return sql.Open(postgresSQLDriver, dsn.String())
	}
	if !strings.Contains(host, ":") {
		host += ":3306"
	}
	dsn := env["DB_USER"] + ":" + env["DB_PASS"] + "@tcp(" + host + ")/" + env["DB_NAME"] + "?parseTime=true&multiStatements=true"
	return sql.Open("mysql", dsn)
}

func rebindPostgres(query string) string {
	var out strings.Builder
	out.Grow(len(query) + 8)
	index := 1
	inSingle := false
	inDouble := false
	inBacktick := false
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '\'' && !inDouble && !inBacktick {
			out.WriteByte(ch)
			if inSingle && i+1 < len(query) && query[i+1] == '\'' {
				out.WriteByte(query[i+1])
				i++
				continue
			}
			inSingle = !inSingle
			continue
		}
		if ch == '`' && !inSingle && !inDouble {
			inBacktick = !inBacktick
			out.WriteByte('"')
			continue
		}
		if ch == '"' && !inSingle && !inBacktick {
			inDouble = !inDouble
			out.WriteByte(ch)
			continue
		}
		if ch == '?' && !inSingle && !inDouble && !inBacktick {
			out.WriteByte('$')
			out.WriteString(strconv.Itoa(index))
			index++
			continue
		}
		out.WriteByte(ch)
	}
	return out.String()
}
