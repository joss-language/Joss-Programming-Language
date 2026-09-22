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
		items := r.executeGetMethod(instance, nil).([]map[string]interface{})
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
	instance.Fields["_wheres"] = append(instance.Fields["_wheres"].([]string), "`id` = ?")
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
	list, ok := rows.([]map[string]interface{})
	if !ok {
		return []interface{}{}
	}

	if keyColumn != "" {
		result := map[string]interface{}{}
		for _, row := range list {
			result[fmt.Sprintf("%v", row[keyColumn])] = row[column]
		}
		return result
	}

	result := []interface{}{}
	for _, row := range list {
		result = append(result, row[column])
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
	items := r.executeGetMethod(instance, nil).([]map[string]interface{})
	if len(items) == 0 {
		panic("GranDB Error en sole(): No se encontró ningún registro para el criterio.")
	}
	if len(items) > 1 {
		panic(fmt.Sprintf("GranDB Error en sole(): Se encontraron %d registros, se esperaba exactamente 1.", len(items)))
	}
	return items[0]
}

// executeFindManyMethod handles .findMany([id1, id2, ...])
func (r *Runtime) executeFindManyMethod(instance *Instance, args []interface{}) interface{} {
	if len(args) == 0 {
		return []map[string]interface{}{}
	}
	ids := toInterfaceSlice(args[0])
	return r.executeGranDBMethod(instance, "wherein", []interface{}{"id", ids}).([]map[string]interface{})
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
