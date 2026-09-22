package core

import (
	"database/sql"
	"fmt"
)

func (r *Runtime) migrationTableIdentifier() (string, string, error) {
	driver := normalizeDatabaseDriver(r.Env["DB"])
	if driver == "" {
		driver = "mysql"
	}
	table, err := quoteSchemaIdentifier(r.dbPrefix()+"migration", driver)
	return table, driver, err
}

// EnsureMigrationTable creates the migration table if it doesn't exist
func (r *Runtime) EnsureMigrationTable() error {
	if r.GetDB() == nil {
		return fmt.Errorf("no hay conexion a la base de datos")
	}

	return r.ensureInternalSchemaTable("migration", []schemaColumn{
		{name: "id", definition: "increments"},
		{name: "migration", definition: "string(255)"},
		{name: "batch", definition: "integer"},
		{name: "executed_at", definition: "timestamp|default(CURRENT_TIMESTAMP)"},
	})
}

// GetExecutedMigrations returns a map of executed migration filenames
func (r *Runtime) GetExecutedMigrations() map[string]bool {
	executed := make(map[string]bool)
	if r.GetDB() == nil {
		return executed
	}

	tableName, _, err := r.migrationTableIdentifier()
	if err != nil {
		return executed
	}

	rows, err := r.databaseExecutor().Query(fmt.Sprintf("SELECT migration FROM %s", tableName))
	if err != nil {
		return executed
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			executed[name] = true
		}
	}
	return executed
}

// GetNextBatch returns the next batch number
func (r *Runtime) GetNextBatch() int {
	if r.GetDB() == nil {
		return 1
	}

	tableName, _, tableErr := r.migrationTableIdentifier()
	if tableErr != nil {
		return 1
	}

	var maxBatch sql.NullInt64
	err := r.databaseExecutor().QueryRow(fmt.Sprintf("SELECT MAX(batch) FROM %s", tableName)).Scan(&maxBatch)
	if err != nil {
		return 1
	}
	if maxBatch.Valid {
		return int(maxBatch.Int64) + 1
	}
	return 1
}

// LogMigration logs a successful migration
func (r *Runtime) LogMigration(migration string, batch int) error {
	if r.GetDB() == nil {
		return fmt.Errorf("no hay conexion a la base de datos")
	}

	tableName, driver, err := r.migrationTableIdentifier()
	if err != nil {
		return err
	}

	placeholders := "?, ?"
	if driver == "postgres" {
		placeholders = "$1, $2"
	} else if driver == "sqlserver" {
		placeholders = "@p1, @p2"
	}
	_, err = r.databaseExecutor().Exec(fmt.Sprintf("INSERT INTO %s (migration, batch) VALUES (%s)", tableName, placeholders), migration, batch)
	return err
}

// DropAllTables drops all user tables from the database
func (r *Runtime) DropAllTables() {
	if r.GetDB() == nil {
		return
	}

	dbDriver := "mysql"
	if val, ok := r.Env["DB"]; ok {
		dbDriver = normalizeDatabaseDriver(val)
	}

	var tables []string

	if dbDriver == "sqlite" {
		// SQLite: Get all tables except sqlite_* system tables
		rows, err := r.databaseExecutor().Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
		if err != nil {
			fmt.Printf("[Migration] Error obteniendo tablas: %v\n", err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var tableName string
			if err := rows.Scan(&tableName); err == nil {
				tables = append(tables, tableName)
			}
		}

		// Drop each table
		for _, table := range tables {
			_, err := r.databaseExecutor().Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
			if err != nil {
				fmt.Printf("[Migration] Error eliminando tabla %s: %v\n", table, err)
			} else {
				fmt.Printf("[Migration] Tabla %s eliminada\n", table)
			}
		}
	} else if dbDriver == "postgres" {
		rows, err := r.databaseExecutor().Query("SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'")
		if err != nil {
			fmt.Printf("[Migration] Error obteniendo tablas: %v\n", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var tableName string
			if rows.Scan(&tableName) == nil {
				tables = append(tables, tableName)
			}
		}
		for _, table := range tables {
			quoted, err := quoteSchemaIdentifier(table, "postgres")
			if err == nil {
				_, _ = r.databaseExecutor().Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", quoted))
			}
		}
	} else {
		// MySQL: Get all tables from current database
		dbName := r.Env["DB_NAME"]
		if dbName == "" {
			fmt.Println("[Migration] Error: DB_NAME no está configurado")
			return
		}

		rows, err := r.databaseExecutor().Query("SELECT table_name FROM information_schema.tables WHERE table_schema = ?", dbName)
		if err != nil {
			fmt.Printf("[Migration] Error obteniendo tablas: %v\n", err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var tableName string
			if err := rows.Scan(&tableName); err == nil {
				tables = append(tables, tableName)
			}
		}

		// Disable foreign key checks for MySQL
		r.databaseExecutor().Exec("SET FOREIGN_KEY_CHECKS = 0")

		// Drop each table
		for _, table := range tables {
			_, err := r.databaseExecutor().Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", table))
			if err != nil {
				fmt.Printf("[Migration] Error eliminando tabla %s: %v\n", table, err)
			} else {
				fmt.Printf("[Migration] Tabla %s eliminada\n", table)
			}
		}

		// Re-enable foreign key checks
		r.databaseExecutor().Exec("SET FOREIGN_KEY_CHECKS = 1")
	}
}
