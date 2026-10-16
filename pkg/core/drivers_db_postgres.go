//go:build !nodb && !nopostgres

package core

import (
	"database/sql"

	"github.com/jackc/pgx/v5/stdlib"
)

func init() {
	sql.Register(postgresSQLDriver, &postgresRebindingDriver{base: stdlib.GetDefaultDriver()})
}
