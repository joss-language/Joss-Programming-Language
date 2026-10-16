package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/url"
	"strings"
)

const sqlServerDriverName = "joss-sqlserver"

// sqlServerRebindingDriver wraps the standard mssql driver for Joss compatibility.
type sqlServerRebindingDriver struct {
	base driver.Driver
}

func (d *sqlServerRebindingDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &sqlServerRebindingConn{Conn: conn}, nil
}

type sqlServerRebindingConn struct {
	driver.Conn
}

func (c *sqlServerRebindingConn) Prepare(query string) (driver.Stmt, error) {
	return c.Conn.Prepare(rebindSQLServer(query))
}

func (c *sqlServerRebindingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if preparer, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return preparer.PrepareContext(ctx, rebindSQLServer(query))
	}
	return c.Prepare(query)
}

func (c *sqlServerRebindingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if execer, ok := c.Conn.(driver.ExecerContext); ok {
		return execer.ExecContext(ctx, rebindSQLServer(query), args)
	}
	return nil, driver.ErrSkip
}

func (c *sqlServerRebindingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if queryer, ok := c.Conn.(driver.QueryerContext); ok {
		return queryer.QueryContext(ctx, rebindSQLServer(query), args)
	}
	return nil, driver.ErrSkip
}

func (c *sqlServerRebindingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if beginner, ok := c.Conn.(driver.ConnBeginTx); ok {
		return beginner.BeginTx(ctx, opts)
	}
	return c.Conn.Begin()
}

func (c *sqlServerRebindingConn) Ping(ctx context.Context) error {
	if pinger, ok := c.Conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c *sqlServerRebindingConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *sqlServerRebindingConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

func (c *sqlServerRebindingConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func rebindSQLServer(query string) string {
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
		if ch == '"' && !inSingle && !inBacktick {
			out.WriteByte(ch)
			inDouble = !inDouble
			continue
		}
		if ch == '`' && !inSingle && !inDouble {
			// Convert backticks to MSSQL square brackets
			if !inBacktick {
				out.WriteByte('[')
			} else {
				out.WriteByte(']')
			}
			inBacktick = !inBacktick
			continue
		}
		if ch == '?' && !inSingle && !inDouble && !inBacktick {
			out.WriteString(fmt.Sprintf("@p%d", index))
			index++
			continue
		}
		out.WriteByte(ch)
	}
	return out.String()
}

func openSQLServerDatabase(env map[string]string) (*sql.DB, error) {
	host := strings.TrimSpace(env["DB_HOST"])
	port := "1433"
	if p, ok := env["DB_PORT"]; ok && strings.TrimSpace(p) != "" {
		port = strings.TrimSpace(p)
	}
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		host = parts[0]
		port = parts[1]
	}
	if host == "" {
		host = "localhost"
	}

	user := strings.TrimSpace(env["DB_USER"])
	if user == "" {
		user = strings.TrimSpace(env["DB_USERNAME"])
	}
	if user == "" {
		user = "sa"
	}

	password := strings.TrimSpace(env["DB_PASS"])
	if password == "" {
		password = strings.TrimSpace(env["DB_PASSWORD"])
	}

	dbName := strings.TrimSpace(env["DB_NAME"])
	if dbName == "" {
		dbName = strings.TrimSpace(env["DB_DATABASE"])
	}

	query := url.Values{}
	if dbName != "" {
		query.Add("database", dbName)
	}
	encrypt := "disable"
	if enc, ok := env["DB_ENCRYPT"]; ok && strings.TrimSpace(enc) != "" {
		encrypt = strings.TrimSpace(enc)
	}
	query.Add("encrypt", encrypt)
	query.Add("app name", "Joss Framework")

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(user, password),
		Host:     fmt.Sprintf("%s:%s", host, port),
		RawQuery: query.Encode(),
	}

	return sql.Open(sqlServerDriverName, u.String())
}
