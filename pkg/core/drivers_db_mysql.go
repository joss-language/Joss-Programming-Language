//go:build !nodb && !nomysql

package core

import (
	_ "github.com/go-sql-driver/mysql"
)
