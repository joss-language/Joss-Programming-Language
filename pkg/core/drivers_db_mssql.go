//go:build !nodb && !nomssql

package core

import (
	"database/sql"

	mssql "github.com/microsoft/go-mssqldb"
)

func init() {
	sql.Register(sqlServerDriverName, &sqlServerRebindingDriver{base: &mssql.Driver{}})
}
