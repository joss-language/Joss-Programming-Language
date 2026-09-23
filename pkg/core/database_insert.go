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
	return r.insertFromMapWithKey(table, data, returnID, "id")
}

func (r *Runtime) insertFromMapWithKey(table string, data map[string]interface{}, returnID bool, primaryKey string) interface{} {
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
		insertData["created_at"] = sqlExpression("CURRENT_TIMESTAMP")
	}
	if _, hasUpdatedAt := insertData["updated_at"]; !hasUpdatedAt && r.tableHasColumn(table, "updated_at") {
		insertData["updated_at"] = sqlExpression("CURRENT_TIMESTAMP")
	}

	// Build column names, placeholders, and bindings
	for _, colName := range sortedMapKeys(insertData) {
		value := insertData[colName]
		if _, ok := value.(map[string]interface{}); ok {
			panic(fmt.Sprintf("GranDB Error: la columna %q contiene un map; serialícelo como JSON explícitamente", colName))
		}

		colNames = append(colNames, quoteIdentifier(colName))

		if expression, ok := value.(sqlExpression); ok {
			placeholders = append(placeholders, string(expression))
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

	driver := normalizeDatabaseDriver(r.Env["DB"])
	if returnID {
		if driver == "postgres" {
			var id int64
			if err := r.databaseExecutor().QueryRow(query+" RETURNING "+quoteIdentifier(primaryKey), bindings...).Scan(&id); err != nil {
				panic(fmt.Sprintf("GranDB Error en insert: %v", err))
			}
			return id
		}
		if driver == "sqlserver" {
			var id int64
			outputQuery := strings.Replace(query, " VALUES", " OUTPUT INSERTED."+quoteIdentifier(primaryKey)+" VALUES", 1)
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

type sqlExpression string

// executeInsertManyMethod inserts homogeneous rows in batches chosen from the
// active dialect's parameter limit. Multiple batches are atomic when GranDB is
// not already inside a transaction; an existing transaction is reused.
func (r *Runtime) executeInsertManyMethod(instance *Instance, args []interface{}) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}
	if len(args) == 0 {
		panic("GranDB Error: insertMany requiere un array de mapas")
	}
	rows := mapsFromArgument(args[0])
	if len(rows) == 0 {
		return int64(0)
	}
	columnCount := len(rows[0])
	if columnCount == 0 {
		panic("GranDB Error: insertMany no acepta filas vacías")
	}
	dialect := dialectFor(r.Env["DB"])
	batchSize := dialect.parameterLimit() / columnCount
	if batchSize < 1 {
		panic("GranDB Error: una fila excede el límite de parámetros del motor")
	}

	createdTx := r.activeTx == nil
	if createdTx {
		tx, err := r.GetDB().Begin()
		if err != nil {
			panic(fmt.Sprintf("GranDB Error: no se pudo iniciar insertMany: %v", err))
		}
		r.activeTx = tx
		defer func() {
			if recovered := recover(); recovered != nil {
				_ = tx.Rollback()
				r.activeTx = nil
				panic(recovered)
			}
		}()
	}

	var affected int64
	for start := 0; start < len(rows); start += batchSize {
		end := start + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		query, bindings, err := compileInsertRows(r.getTable(instance), rows[start:end])
		if err != nil {
			panic(fmt.Sprintf("GranDB Error: %v", err))
		}
		result, err := r.databaseExecutor().Exec(query, bindings...)
		if err != nil {
			panic(fmt.Sprintf("GranDB Error en insertMany: %v", err))
		}
		if count, err := result.RowsAffected(); err == nil {
			affected += count
		}
	}
	if createdTx {
		tx := r.activeTx
		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			r.activeTx = nil
			panic(fmt.Sprintf("GranDB Error: no se pudo confirmar insertMany: %v", err))
		}
		r.activeTx = nil
	}
	return affected
}
