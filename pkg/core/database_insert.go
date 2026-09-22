package core

import (
	"fmt"
	"strings"
)

// executeInsertMethod handles map-based inserts for GranDB.
func (r *Runtime) executeInsertMethod(instance *Instance, args []interface{}, returnID bool) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}

	table := r.getTable(instance)

	// Usage: $model->insert({"name": "John", "email": "john@example.com"})
	if len(args) == 1 {
		if data, ok := args[0].(map[string]interface{}); ok {
			return r.insertFromMap(table, data, returnID)
		}
	}

	return false
}

// insertFromMap performs insert using a map of column-value pairs
func (r *Runtime) insertFromMap(table string, data map[string]interface{}, returnID bool) interface{} {
	if len(data) == 0 {
		return false
	}

	colNames := []string{}
	placeholders := []string{}
	bindings := []interface{}{}

	// Preserve the caller's map and only add timestamps that exist in the
	// physical table. GranDB also supports legacy/external schemas.
	insertData := make(map[string]interface{}, len(data)+2)
	for key, value := range data {
		insertData[key] = value
	}
	if _, hasCreatedAt := insertData["created_at"]; !hasCreatedAt && r.tableHasColumn(table, "created_at") {
		insertData["created_at"] = "CURRENT_TIMESTAMP"
	}
	if _, hasUpdatedAt := insertData["updated_at"]; !hasUpdatedAt && r.tableHasColumn(table, "updated_at") {
		insertData["updated_at"] = "CURRENT_TIMESTAMP"
	}

	// Build column names, placeholders, and bindings
	for colName, value := range insertData {
		// Skip unsupported types (like maps)
		if _, ok := value.(map[string]interface{}); ok {
			continue
		}

		colNames = append(colNames, quoteIdentifier(colName))

		// Check if value is a SQL function (like CURRENT_TIMESTAMP)
		if strVal, ok := value.(string); ok && isSQLFunction(strVal) {
			placeholders = append(placeholders, strVal)
		} else {
			placeholders = append(placeholders, "?")
			bindings = append(bindings, value)
		}
	}

	// Build and execute query
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(colNames, ", "),
		strings.Join(placeholders, ", "))

	fmt.Printf("[GranDB] Insert Query: %s\n", query)
	fmt.Printf("[GranDB] Bindings: %v\n", bindings)
	driver := normalizeDatabaseDriver(r.Env["DB"])
	if returnID {
		if driver == "postgres" {
			var id int64
			if err := r.databaseExecutor().QueryRow(query+" RETURNING id", bindings...).Scan(&id); err != nil {
				panic(fmt.Sprintf("GranDB Error en insert: %v", err))
			}
			return id
		}
		if driver == "sqlserver" {
			var id int64
			outputQuery := strings.Replace(query, " VALUES", " OUTPUT INSERTED.id VALUES", 1)
			if err := r.databaseExecutor().QueryRow(outputQuery, bindings...).Scan(&id); err == nil && id > 0 {
				return id
			}
			if _, err := r.databaseExecutor().Exec(query, bindings...); err == nil {
				_ = r.databaseExecutor().QueryRow("SELECT SCOPE_IDENTITY()").Scan(&id)
				if id > 0 {
					return id
				}
			}
			return false
		}
	}

	result, err := r.databaseExecutor().Exec(query, bindings...)
	if err != nil {
		panic(fmt.Sprintf("GranDB Error en insert: %v", err))
	}

	if returnID {
		if id, err := result.LastInsertId(); err == nil && id > 0 {
			return id
		}
		return false
	}
	return true
}

// isSQLFunction checks if a string is a SQL function that should not be quoted
func isSQLFunction(value string) bool {
	upperValue := strings.ToUpper(strings.TrimSpace(value))
	sqlFunctions := []string{
		"CURRENT_TIMESTAMP",
		"NOW()",
		"CURRENT_DATE",
		"CURRENT_TIME",
		"NULL",
	}

	for _, fn := range sqlFunctions {
		if upperValue == fn || strings.HasPrefix(upperValue, fn) {
			return true
		}
	}

	return false
}
