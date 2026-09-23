package core

import (
	"database/sql"
	"fmt"
	"strings"
)

// executeGetMethod handles .get()
func (r *Runtime) executeGetMethod(instance *Instance, args []interface{}) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}

	sel := instance.Fields["_select"].(string)
	query, bindings := r.buildSelectQuery(instance, sel)
	resetReadState(instance)

	rows, err := r.databaseExecutor().Query(query, bindings...)
	if err != nil {
		panic(fmt.Sprintf("GranDB Error en get: %v", err))
	}
	defer rows.Close()

	result := rowsToMap(rows)
	if instance.model != nil && instance.model.query {
		models := r.hydrateModelRows(instance.model.metadata, result)
		r.applyEagerLoads(instance, models)
		return models
	}
	return result
}

func (r *Runtime) executeExplainMethod(instance *Instance) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}
	sel, _ := instance.Fields["_select"].(string)
	if sel == "" {
		sel = "*"
	}
	query, bindings := r.buildSelectQuery(instance, sel)
	explain, err := dialectFor(r.Env["DB"]).explainQuery(query)
	if err != nil {
		panic(fmt.Sprintf("GranDB Error: %v", err))
	}
	rows, err := r.databaseExecutor().Query(explain, bindings...)
	if err != nil {
		panic(fmt.Sprintf("GranDB Error en explain: %v", err))
	}
	defer rows.Close()
	return rowsToMap(rows)
}

// executeFirstMethod handles .first()
func (r *Runtime) executeFirstMethod(instance *Instance, args []interface{}) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}

	sel := instance.Fields["_select"].(string)
	instance.Fields["_limit"] = 1
	delete(instance.Fields, "_offset")
	query, bindings := r.buildSelectQuery(instance, sel)
	resetReadState(instance)

	rows, err := r.databaseExecutor().Query(query, bindings...)
	if err != nil {
		panic(fmt.Sprintf("GranDB Error en first: %v", err))
	}
	defer rows.Close()

	results := rowsToMap(rows)
	if len(results) > 0 {
		if instance.model != nil && instance.model.query {
			model := r.hydrateModel(instance.model.metadata, results[0])
			r.applyEagerLoads(instance, []interface{}{model})
			return model
		}
		return results[0]
	}
	return nil
}

// executeFirstOrFailMethod handles .firstOrFail()
func (r *Runtime) executeFirstOrFailMethod(instance *Instance, args []interface{}) interface{} {
	res := r.executeFirstMethod(instance, args)
	if res == nil {
		tbl := instance.Fields["_table"]
		panic(fmt.Sprintf("GranDB Error: No se encontró ningún registro en la tabla %v.", tbl))
	}
	return res
}

// executePaginateMethod handles .paginate($perPage, $page)
func (r *Runtime) executePaginateMethod(instance *Instance, args []interface{}) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}
	perPage := 15
	page := 1
	if len(args) >= 1 {
		perPage = toInt(args[0])
	}
	if len(args) >= 2 {
		page = toInt(args[1])
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}

	totalVal := r.executeCountMethod(instance, nil)
	total := int(toInt64(totalVal))

	offset := (page - 1) * perPage
	instance.Fields["_limit"] = perPage
	instance.Fields["_offset"] = offset

	items := r.executeGetMethod(instance, nil)

	lastPage := 1
	if total > 0 {
		lastPage = (total + perPage - 1) / perPage
	}

	return map[string]interface{}{
		"data":         items,
		"total":        total,
		"per_page":     perPage,
		"current_page": page,
		"last_page":    lastPage,
	}
}

func (r *Runtime) executeSimplePaginateMethod(instance *Instance, args []interface{}) interface{} {
	perPage, page := paginationArguments(args)
	query := cloneGranDBBuilder(instance)
	query.Fields["_limit"] = perPage + 1
	query.Fields["_offset"] = (page - 1) * perPage
	rawItems := r.executeGetMethod(query, nil)
	items := resultItems(rawItems)
	hasMore := len(items) > perPage
	if hasMore {
		items = items[:perPage]
	}
	return map[string]interface{}{"data": preserveResultType(rawItems, items), "per_page": perPage, "current_page": page, "has_more": hasMore}
}

func (r *Runtime) executeCursorPaginateMethod(instance *Instance, args []interface{}) interface{} {
	perPage := 15
	var cursor interface{}
	column := "id"
	if len(args) >= 1 && toInt(args[0]) > 0 {
		perPage = toInt(args[0])
	}
	if len(args) >= 2 {
		cursor = args[1]
	}
	if len(args) >= 3 {
		column = fmt.Sprint(args[2])
	}
	query := cloneGranDBBuilder(instance)
	if cursor != nil {
		r.executeGranDBMethod(query, "where", []interface{}{column, ">", cursor})
	}
	r.executeGranDBMethod(query, "orderBy", []interface{}{column, "asc"})
	query.Fields["_limit"] = perPage + 1
	rawItems := r.executeGetMethod(query, nil)
	items := resultItems(rawItems)
	hasMore := len(items) > perPage
	if hasMore {
		items = items[:perPage]
	}
	var next interface{}
	if hasMore && len(items) > 0 {
		next = resultField(items[len(items)-1], column)
	}
	return map[string]interface{}{"data": preserveResultType(rawItems, items), "per_page": perPage, "next_cursor": next, "has_more": hasMore}
}

func paginationArguments(args []interface{}) (int, int) {
	perPage, page := 15, 1
	if len(args) >= 1 && toInt(args[0]) > 0 {
		perPage = toInt(args[0])
	}
	if len(args) >= 2 && toInt(args[1]) > 0 {
		page = toInt(args[1])
	}
	return perPage, page
}

// executeChunkMethod handles .chunk($size, func($items) { ... })
func (r *Runtime) executeChunkMethod(instance *Instance, args []interface{}) interface{} {
	if len(args) < 2 {
		panic("GranDB Error: chunk() requiere tamaño y función de callback")
	}
	size := toInt(args[0])
	if !r.isCallable(args[1]) || size <= 0 {
		panic("GranDB Error: argumentos inválidos para chunk()")
	}

	page := 1
	for {
		instance.Fields["_limit"] = size
		instance.Fields["_offset"] = (page - 1) * size
		items := resultItems(r.executeGetMethod(instance, nil))
		if len(items) == 0 {
			break
		}
		r.CallFunction(args[1], []interface{}{items})
		if len(items) < size {
			break
		}
		page++
	}
	return true
}

func (r *Runtime) executeChunkByIDMethod(instance *Instance, args []interface{}) interface{} {
	if len(args) < 2 || toInt(args[0]) <= 0 || !r.isCallable(args[1]) {
		panic("GranDB Error: chunkById requiere tamaño positivo y callback")
	}
	size := toInt(args[0])
	column := "id"
	if len(args) >= 3 {
		column = fmt.Sprint(args[2])
	}
	var cursor interface{}
	for {
		query := cloneGranDBBuilder(instance)
		if cursor != nil {
			r.executeGranDBMethod(query, "where", []interface{}{column, ">", cursor})
		}
		r.executeGranDBMethod(query, "orderBy", []interface{}{column, "asc"})
		query.Fields["_limit"] = size
		items := resultItems(r.executeGetMethod(query, nil))
		if len(items) == 0 {
			break
		}
		r.CallFunction(args[1], []interface{}{items})
		next := resultField(items[len(items)-1], column)
		if next == nil || fmt.Sprint(next) == fmt.Sprint(cursor) {
			panic(fmt.Sprintf("GranDB Error: chunkById requiere la columna única y creciente %q", column))
		}
		cursor = next
		if len(items) < size {
			break
		}
	}
	return true
}

func cloneGranDBBuilder(instance *Instance) *Instance {
	clone := &Instance{Class: instance.Class, Fields: make(map[string]interface{}), Constants: make(map[string]bool)}
	for key, value := range instance.Fields {
		switch typed := value.(type) {
		case []string:
			clone.Fields[key] = append([]string(nil), typed...)
		case []interface{}:
			clone.Fields[key] = append([]interface{}(nil), typed...)
		default:
			clone.Fields[key] = value
		}
	}
	for key, value := range instance.Constants {
		clone.Constants[key] = value
	}
	clone.model = instance.model.clone()
	return clone
}

// executeCountMethod handles .count()
func (r *Runtime) executeCountMethod(instance *Instance, args []interface{}) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}

	savedOrder := instance.Fields["_order"]
	savedLimit := instance.Fields["_limit"]
	savedOffset := instance.Fields["_offset"]

	delete(instance.Fields, "_order")
	delete(instance.Fields, "_limit")
	delete(instance.Fields, "_offset")

	query, bindings := r.buildSelectQuery(instance, "COUNT(*)")

	if savedOrder != nil {
		instance.Fields["_order"] = savedOrder
	}
	if savedLimit != nil {
		instance.Fields["_limit"] = savedLimit
	}
	if savedOffset != nil {
		instance.Fields["_offset"] = savedOffset
	}

	var count int
	err := r.databaseExecutor().QueryRow(query, bindings...).Scan(&count)
	if err != nil {
		panic(fmt.Sprintf("GranDB Error en count: %v", err))
	}
	return count
}

func (r *Runtime) executeFindMethod(instance *Instance, args []interface{}) interface{} {
	if len(args) == 0 {
		return nil
	}
	key := "id"
	if instance.model != nil && instance.model.metadata != nil {
		key = instance.model.metadata.PrimaryKey
	}
	instance.Fields["_wheres"] = append(instance.Fields["_wheres"].([]string), quoteIdentifier(key)+" = ?")
	instance.Fields["_bindings"] = append(instance.Fields["_bindings"].([]interface{}), args[0])
	return r.executeFirstMethod(instance, nil)
}

func (r *Runtime) executeValueMethod(instance *Instance, args []interface{}) interface{} {
	if len(args) == 0 {
		return nil
	}
	column := fmt.Sprintf("%v", args[0])
	instance.Fields["_select"] = quoteIdentifier(r.applyColumnPrefix(column))
	row := r.executeFirstMethod(instance, nil)
	if result, ok := row.(map[string]interface{}); ok {
		return result[column]
	}
	if result, ok := row.(*Instance); ok {
		return result.Fields[column]
	}
	return nil
}

func (r *Runtime) executePluckMethod(instance *Instance, args []interface{}) interface{} {
	if len(args) == 0 {
		return []interface{}{}
	}
	column := fmt.Sprintf("%v", args[0])
	keyColumn := ""
	if len(args) >= 2 {
		keyColumn = fmt.Sprintf("%v", args[1])
		instance.Fields["_select"] = strings.Join([]string{quoteIdentifier(r.applyColumnPrefix(column)), quoteIdentifier(r.applyColumnPrefix(keyColumn))}, ", ")
	} else {
		instance.Fields["_select"] = quoteIdentifier(r.applyColumnPrefix(column))
	}

	rows := r.executeGetMethod(instance, nil)
	list := resultItems(rows)

	if keyColumn != "" {
		result := map[string]interface{}{}
		for _, row := range list {
			result[fmt.Sprintf("%v", resultField(row, keyColumn))] = resultField(row, column)
		}
		return result
	}

	result := []interface{}{}
	for _, row := range list {
		result = append(result, resultField(row, column))
	}
	return result
}

func (r *Runtime) executeExistsMethod(instance *Instance, invert bool) interface{} {
	instance.Fields["_select"] = "1"
	instance.Fields["_limit"] = 1
	row := r.executeFirstMethod(instance, nil)
	exists := row != nil
	if invert {
		return !exists
	}
	return exists
}

func (r *Runtime) executeAggregateMethod(instance *Instance, method string, args []interface{}) interface{} {
	if r.GetDB() == nil {
		panic("GranDB Error: No hay conexión a la base de datos configurada")
	}
	if len(args) == 0 {
		return nil
	}

	savedOrder := instance.Fields["_order"]
	savedLimit := instance.Fields["_limit"]
	savedOffset := instance.Fields["_offset"]

	delete(instance.Fields, "_order")
	delete(instance.Fields, "_limit")
	delete(instance.Fields, "_offset")

	fn := strings.ToUpper(method)
	column := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
	query, bindings := r.buildSelectQuery(instance, fmt.Sprintf("%s(%s) as aggregate_value", fn, column))

	if savedOrder != nil {
		instance.Fields["_order"] = savedOrder
	}
	if savedLimit != nil {
		instance.Fields["_limit"] = savedLimit
	}
	if savedOffset != nil {
		instance.Fields["_offset"] = savedOffset
	}

	var value sql.NullFloat64
	err := r.databaseExecutor().QueryRow(query, bindings...).Scan(&value)
	if err != nil {
		panic(fmt.Sprintf("GranDB Error en %s: %v", method, err))
	}
	if !value.Valid {
		return nil
	}
	return value.Float64
}

// executeSoleMethod handles .sole() (returns 1 item or panics if count != 1)
func (r *Runtime) executeSoleMethod(instance *Instance, args []interface{}) interface{} {
	items := resultItems(r.executeGetMethod(instance, nil))
	if len(items) == 0 {
		panic("GranDB Error en sole(): No se encontró ningún registro para el criterio.")
	}
	if len(items) > 1 {
		panic(fmt.Sprintf("GranDB Error en sole(): Se encontraron %d registros, se esperaba exactamente 1.", len(items)))
	}
	return items[0]
}

func resultItems(value interface{}) []interface{} {
	switch items := value.(type) {
	case []interface{}:
		return items
	case []map[string]interface{}:
		result := make([]interface{}, len(items))
		for index, item := range items {
			result[index] = item
		}
		return result
	default:
		return []interface{}{}
	}
}
func resultField(value interface{}, name string) interface{} {
	switch item := value.(type) {
	case map[string]interface{}:
		return item[name]
	case *Instance:
		return item.Fields[name]
	}
	return nil
}
func preserveResultType(original interface{}, items []interface{}) interface{} {
	if _, ok := original.([]map[string]interface{}); ok {
		result := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			if row, ok := item.(map[string]interface{}); ok {
				result = append(result, row)
			}
		}
		return result
	}
	return items
}

// executeFindManyMethod handles .findMany([id1, id2, ...])
func (r *Runtime) executeFindManyMethod(instance *Instance, args []interface{}) interface{} {
	if len(args) == 0 {
		return []map[string]interface{}{}
	}
	ids := toInterfaceSlice(args[0])
	key := "id"
	if instance.model != nil && instance.model.metadata != nil {
		key = instance.model.metadata.PrimaryKey
	}
	r.executeGranDBMethod(instance, "wherein", []interface{}{key, ids})
	return r.executeGetMethod(instance, nil)
}

// executeFindOrFailMethod handles .findOrFail($id)
func (r *Runtime) executeFindOrFailMethod(instance *Instance, args []interface{}) interface{} {
	res := r.executeFindMethod(instance, args)
	if res == nil {
		tbl := instance.Fields["_table"]
		idVal := ""
		if len(args) > 0 {
			idVal = fmt.Sprintf("%v", args[0])
		}
		panic(fmt.Sprintf("GranDB Error: No se encontró ningún registro con ID %v en la tabla %v.", idVal, tbl))
	}
	return res
}
