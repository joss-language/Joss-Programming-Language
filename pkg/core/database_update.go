package core

import (
	"fmt"
	"strings"
	"time"
)

// executeUpdateMethod handles update operations for GranDB
// Usage: $model.where("id", 1).update({"name": "Jane", "email": "jane@example.com"})
func (r *Runtime) executeUpdateMethod(instance *Instance, args []interface{}) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}

	// Get table and where conditions
	table := r.getTable(instance)
	wheres := instance.Fields["_wheres"].([]string)
	bindings := instance.Fields["_bindings"].([]interface{})

	// Validate: update requires data
	if len(args) == 0 {
		fmt.Println("[GranDB] Error: update() requires data argument")
		return false
	}

	// Get update data (must be a map)
	data, ok := args[0].(map[string]interface{})
	if !ok {
		fmt.Println("[GranDB] Error: update() requires map argument")
		return false
	}

	if len(data) == 0 {
		fmt.Println("[GranDB] Error: update() data is empty")
		return false
	}
	if len(wheres) == 0 {
		fmt.Println("[GranDB] Aborting update without WHERE. Use an explicit bulk operation for all rows.")
		return false
	}

	// Do not assume every table has Laravel-style timestamps. Older and
	// externally-managed tables are valid GranDB targets too.
	updateData := make(map[string]interface{}, len(data)+1)
	for key, value := range data {
		updateData[key] = value
	}
	if _, hasUpdatedAt := updateData["updated_at"]; !hasUpdatedAt && r.tableHasColumn(table, "updated_at") {
		updateData["updated_at"] = sqlExpression("CURRENT_TIMESTAMP")
	}

	// Build SET clause
	setClauses := []string{}
	updateBindings := []interface{}{}

	for _, colName := range sortedMapKeys(updateData) {
		value := updateData[colName]
		if expression, ok := value.(sqlExpression); ok {
			setClauses = append(setClauses, fmt.Sprintf("%s = %s", quoteIdentifier(colName), expression))
		} else {
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(colName)))
			updateBindings = append(updateBindings, value)
		}
	}

	// Build query
	query := fmt.Sprintf("UPDATE %s SET %s", table, strings.Join(setClauses, ", "))

	// Add WHERE clause if present
	query += " WHERE " + buildWhereClause(wheres)
	// Append where bindings after update bindings
	updateBindings = append(updateBindings, bindings...)

	// Reset state before execution
	instance.Fields["_wheres"] = []string{}
	instance.Fields["_bindings"] = []interface{}{}

	// Execute query
	_, err := r.databaseExecutor().Exec(query, updateBindings...)
	if err != nil {
		panic(fmt.Sprintf("GranDB Error en update: %v", err))
	}

	return true
}

// tableHasColumn asks the active database for the real result metadata. The
// LIMIT 0 query works with both SQLite and MySQL and avoids driver-specific
// schema commands.
func (r *Runtime) tableHasColumn(table, column string) bool {
	if r.GetDB() == nil || strings.TrimSpace(table) == "" {
		return false
	}

	rows, err := r.databaseExecutor().Query(dialectFor(r.Env["DB"]).columnProbe(table))
	if err != nil {
		fmt.Printf("[GranDB] No se pudo inspeccionar columnas de %s: %v\n", table, err)
		return false
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return false
	}
	for _, current := range columns {
		if strings.EqualFold(current, column) {
			return true
		}
	}
	return false
}

// executeUpsertMethod performs an atomic dialect-specific upsert. Its public
// contract is upsert(rows, uniqueBy, updateColumns = []). rows accepts a map or
// an array of maps; every row must have the same columns.
func (r *Runtime) executeUpsertMethod(instance *Instance, args []interface{}) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}
	if len(args) < 2 {
		panic("GranDB Error: upsert requiere filas y clave única")
	}

	rows := mapsFromArgument(args[0])
	uniqueBy := stringsFromArgument(args[1])
	var updateColumns []string
	if len(args) >= 3 {
		updateColumns = stringsFromArgument(args[2])
	}
	if len(rows) == 0 {
		return int64(0)
	}
	columnCount := len(rows[0])
	if columnCount == 0 {
		panic("GranDB Error: upsert no acepta filas vacías")
	}
	dialect := dialectFor(r.Env["DB"])
	batchSize := dialect.parameterLimit() / columnCount
	if batchSize < 1 {
		panic("GranDB Error: una fila excede el límite de parámetros del motor")
	}

	createdTx := r.activeTx == nil && len(rows) > batchSize
	if createdTx {
		tx, err := r.GetDB().Begin()
		if err != nil {
			panic(fmt.Sprintf("GranDB Error: no se pudo iniciar upsert: %v", err))
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
		query, bindings, err := dialect.compileUpsert(r.getTable(instance), rows[start:end], uniqueBy, updateColumns)
		if err != nil {
			panic(fmt.Sprintf("GranDB Error: %v", err))
		}
		result, err := r.databaseExecutor().Exec(query, bindings...)
		if err != nil {
			panic(fmt.Sprintf("GranDB Error en upsert: %v", err))
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
			panic(fmt.Sprintf("GranDB Error: no se pudo confirmar upsert: %v", err))
		}
		r.activeTx = nil
	}
	return affected
}

func mapsFromArgument(value interface{}) []map[string]interface{} {
	if row, ok := value.(map[string]interface{}); ok {
		return []map[string]interface{}{row}
	}
	items, ok := value.([]interface{})
	if !ok {
		panic("GranDB Error: upsert requiere un mapa o array de mapas")
	}
	rows := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]interface{})
		if !ok {
			panic("GranDB Error: cada fila de upsert debe ser un mapa")
		}
		rows = append(rows, row)
	}
	return rows
}

func stringsFromArgument(value interface{}) []string {
	if text, ok := value.(string); ok {
		return []string{text}
	}
	items := toInterfaceSlice(value)
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, fmt.Sprint(item))
	}
	return result
}

// executeIncrementMethod handles atomic .increment() and .decrement()
func (r *Runtime) executeIncrementMethod(instance *Instance, args []interface{}, isDecrement bool) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}
	if len(args) == 0 {
		panic("GranDB Error: increment/decrement requiere el nombre de la columna")
	}

	col := quoteIdentifier(r.applyColumnPrefix(args[0].(string)))
	amount := int64(1)
	if len(args) >= 2 {
		amount = toInt64(args[1])
	}

	table := r.getTable(instance)
	wheres := instance.Fields["_wheres"].([]string)
	bindings := instance.Fields["_bindings"].([]interface{})

	op := "+"
	if isDecrement {
		op = "-"
	}

	query := fmt.Sprintf("UPDATE %s SET %s = %s %s ?", table, col, col, op)
	if len(wheres) > 0 {
		query += " WHERE " + buildWhereClause(wheres)
	}

	resetReadState(instance)
	execBindings := append([]interface{}{amount}, bindings...)
	res, err := r.databaseExecutor().Exec(query, execBindings...)
	if err != nil {
		panic(fmt.Sprintf("GranDB Error en increment/decrement: %v", err))
	}
	affected, _ := res.RowsAffected()
	return affected
}

// executeUpdateOrInsertMethod handles .updateOrInsert($attributes, $values)
func (r *Runtime) executeUpdateOrInsertMethod(instance *Instance, args []interface{}) interface{} {
	if len(args) < 1 {
		panic("GranDB Error: updateOrInsert requiere arreglo o mapa de atributos")
	}

	attributes, ok1 := args[0].(map[string]interface{})
	if !ok1 {
		panic("GranDB Error: primer argumento de updateOrInsert debe ser un mapa de atributos")
	}

	values := map[string]interface{}{}
	if len(args) >= 2 {
		if vMap, ok := args[1].(map[string]interface{}); ok {
			values = vMap
		}
	}

	// 1. Check if record exists
	searchInst := &Instance{Class: instance.Class, Fields: make(map[string]interface{})}
	searchInst.Fields["_wheres"] = []string{}
	searchInst.Fields["_bindings"] = []interface{}{}
	searchInst.Fields["_table"] = instance.Fields["_table"]

	for k, v := range attributes {
		r.executeGranDBMethod(searchInst, "where", []interface{}{k, v})
	}

	existsVal := r.executeExistsMethod(searchInst, false).(bool)

	if existsVal {
		// Update matching record
		if len(values) > 0 {
			updateInst := &Instance{Class: instance.Class, Fields: make(map[string]interface{})}
			updateInst.Fields["_wheres"] = []string{}
			updateInst.Fields["_bindings"] = []interface{}{}
			updateInst.Fields["_table"] = instance.Fields["_table"]
			for k, v := range attributes {
				r.executeGranDBMethod(updateInst, "where", []interface{}{k, v})
			}
			return r.executeUpdateMethod(updateInst, []interface{}{values})
		}
		return true
	}

	// Insert new record combining attributes + values
	insertData := make(map[string]interface{})
	for k, v := range attributes {
		insertData[k] = v
	}
	for k, v := range values {
		insertData[k] = v
	}

	insertInst := &Instance{Class: instance.Class, Fields: make(map[string]interface{})}
	insertInst.Fields["_table"] = instance.Fields["_table"]
	return r.executeInsertMethod(insertInst, []interface{}{insertData}, false)
}

// executeTouchMethod handles .touch()
func (r *Runtime) executeTouchMethod(instance *Instance, args []interface{}) interface{} {
	now := time.Now().Format("2006-01-02 15:04:05")
	return r.executeUpdateMethod(instance, []interface{}{
		map[string]interface{}{"updated_at": now},
	})
}
