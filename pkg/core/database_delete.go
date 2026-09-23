package core

import (
	"fmt"
)

// executeDeleteMethod handles delete operations for GranDB
// Usage: $model.where("id", 1).delete()
func (r *Runtime) executeDeleteMethod(instance *Instance) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}

	// Get table and where conditions
	table := r.getTable(instance)
	wheres := instance.Fields["_wheres"].([]string)
	bindings := instance.Fields["_bindings"].([]interface{})

	// Build query
	query := fmt.Sprintf("DELETE FROM %s", table)

	// Add WHERE clause if present
	if len(wheres) > 0 {
		query += " WHERE " + buildWhereClause(wheres)
	} else {
		fmt.Println("[GranDB] Warning: delete() without WHERE clause will delete all rows")
		fmt.Println("[GranDB] Aborting delete for safety. Use deleteAll() to delete all rows.")
		return false
	}

	// Reset state before execution
	instance.Fields["_wheres"] = []string{}
	instance.Fields["_bindings"] = []interface{}{}

	// Execute query
	_, err := r.databaseExecutor().Exec(query, bindings...)
	if err != nil {
		panic(fmt.Sprintf("GranDB Error en delete: %v", err))
	}

	return true
}

// executeDeleteAllMethod handles delete all operations (without WHERE clause)
// Usage: $model.deleteAll()
func (r *Runtime) executeDeleteAllMethod(instance *Instance) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}

	table := r.getTable(instance)
	query := fmt.Sprintf("DELETE FROM %s", table)

	fmt.Println("[GranDB] Warning: Deleting ALL rows from table")

	// Reset state
	instance.Fields["_wheres"] = []string{}
	instance.Fields["_bindings"] = []interface{}{}

	// Execute query
	_, err := r.databaseExecutor().Exec(query)
	if err != nil {
		panic(fmt.Sprintf("GranDB Error en deleteAll: %v", err))
	}

	return true
}

// executeTruncateMethod handles truncate operations (faster than DELETE)
// Usage: $model.truncate()
// Note: TRUNCATE is faster but cannot be rolled back and resets auto-increment
func (r *Runtime) executeTruncateMethod(instance *Instance) interface{} {
	if r.GetDB() == nil {
		return false
	}

	table := instance.Fields["_table"].(string)

	// Get database driver
	dbDriver := "mysql"
	if val, ok := r.Env["DB"]; ok {
		dbDriver = normalizeDatabaseDriver(val)
	}

	var query string
	if dbDriver == "sqlite" {
		// SQLite doesn't have TRUNCATE, use DELETE + reset sequence
		query = fmt.Sprintf("DELETE FROM %s", table)
		_, err := r.databaseExecutor().Exec(query)
		if err != nil {
			fmt.Printf("[GranDB] Error truncate: %v\n", err)
			return false
		}

		// Reset auto-increment sequence. The table name is data in
		// sqlite_sequence, so it is bound rather than interpolated.
		_, _ = r.databaseExecutor().Exec("DELETE FROM sqlite_sequence WHERE name = ?", unquoteIdentifier(table))
	} else {
		// MySQL and PostgreSQL have native TRUNCATE
		query = fmt.Sprintf("TRUNCATE TABLE %s", table)
		_, err := r.databaseExecutor().Exec(query)
		if err != nil {
			fmt.Printf("[GranDB] Error truncate: %v\n", err)
			return false
		}
	}

	// Reset state
	instance.Fields["_wheres"] = []string{}
	instance.Fields["_bindings"] = []interface{}{}

	fmt.Println("[GranDB] Table truncated successfully")
	return true
}
