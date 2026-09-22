package core

import (
	"fmt"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
)

// GranDB Implementation
func (r *Runtime) executeGranDBMethod(instance *Instance, method string, args []interface{}) interface{} {
	if instance == nil {
		panic("Internal Error: Native method called with nil instance")
	}
	if instance.Fields == nil {
		instance.Fields = make(map[string]interface{})
	}

	// Initialize internal state if needed
	if _, ok := instance.Fields["_wheres"]; !ok {
		instance.Fields["_wheres"] = []string{}
		instance.Fields["_bindings"] = []interface{}{}
		instance.Fields["_select"] = "*"
		instance.Fields["_table"] = ""
	}

	methodLower := strings.ToLower(method)

	switch methodLower {
	case "changedb", "connection", "use":
		if len(args) > 0 {
			driverName := fmt.Sprintf("%v", args[0])
			var config map[string]string
			if len(args) > 1 {
				if m, ok := args[1].(map[string]interface{}); ok {
					config = make(map[string]string, len(m))
					for k, v := range m {
						config[k] = fmt.Sprintf("%v", v)
					}
				}
			}
			err := r.ChangeDB(driverName, config)
			if err != nil {
				fmt.Printf("[GranDB Error] No se pudo cambiar el motor a %s: %v\n", driverName, err)
				return false
			}
			return instance
		}
		return instance

	case "distinct":
		instance.Fields["_distinct"] = true
		return instance

	case "crossjoin":
		if len(args) >= 1 {
			table := r.applyTablePrefix(fmt.Sprintf("%v", args[0]))
			if _, ok := instance.Fields["_joins"]; !ok {
				instance.Fields["_joins"] = []string{}
			}
			join := fmt.Sprintf("CROSS JOIN %s", table)
			instance.Fields["_joins"] = append(instance.Fields["_joins"].([]string), join)
		}
		return instance

	case "transaction":
		if len(args) > 0 && r.isCallable(args[0]) {
			if r.activeTx != nil {
				panic(&JossError{Type: "TransactionError", Message: "Las transacciones anidadas de GranDB no están soportadas"})
			}
			db := r.GetDB()
			if db == nil {
				panic(&JossError{Type: "TransactionError", Message: "GranDB requiere una conexión de base de datos"})
			}
			tx, err := db.Begin()
			if err != nil {
				panic(&JossError{Type: "TransactionError", Message: fmt.Sprintf("No se pudo iniciar la transacción: %v", err)})
			}
			var res interface{}
			r.activeTx = tx
			defer func() {
				r.activeTx = nil
				if p := recover(); p != nil {
					_ = tx.Rollback()
					panic(p)
				}
			}()
			res = r.CallFunction(args[0], []interface{}{instance})
			if err := tx.Commit(); err != nil {
				_ = tx.Rollback()
				panic(&JossError{Type: "TransactionError", Message: fmt.Sprintf("No se pudo confirmar la transacción: %v", err)})
			}
			return res
		}
		return nil

	case "when":
		if len(args) >= 2 {
			condition := isTruthy(args[0])
			if condition {
				if r.isCallable(args[1]) {
					r.CallFunction(args[1], []interface{}{instance, args[0]})
				}
			} else if len(args) >= 3 {
				if r.isCallable(args[2]) {
					r.CallFunction(args[2], []interface{}{instance, args[0]})
				}
			}
		}
		return instance

	case "unless":
		if len(args) >= 2 {
			condition := isTruthy(args[0])
			if !condition {
				if r.isCallable(args[1]) {
					r.CallFunction(args[1], []interface{}{instance, args[0]})
				}
			}
		}
		return instance

	case "table", "from":
		if len(args) > 0 {
			tableName := fmt.Sprintf("%v", args[0])
			instance.Fields["_table"] = quoteIdentifier(r.applyTablePrefix(tableName))
			resetReadState(instance)
		}
		return instance

	case "select":
		if len(args) > 0 {
			if cols, ok := args[0].(string); ok {
				instance.Fields["_select"] = cols
			} else if cols, ok := args[0].([]interface{}); ok {
				strCols := []string{}
				for _, c := range cols {
					colStr := fmt.Sprintf("%v", c)
					if strings.Contains(strings.ToLower(colStr), " as ") {
						parts := strings.Split(colStr, " ")
						for i, p := range parts {
							if strings.Contains(p, ".") {
								parts[i] = r.applyColumnPrefix(p)
							}
						}
						strCols = append(strCols, strings.Join(parts, " "))
					} else {
						strCols = append(strCols, r.applyColumnPrefix(colStr))
					}
				}
				instance.Fields["_select"] = strings.Join(strCols, ", ")
			}
		}
		return instance

	case "where", "orwhere":
		isOr := methodLower == "orwhere"
		prefix := ""
		if isOr {
			prefix = "OR "
		}

		if len(args) == 1 {
			if r.isCallable(args[0]) {
				subInst := &Instance{Class: instance.Class, Fields: make(map[string]interface{})}
				subInst.Fields["_wheres"] = []string{}
				subInst.Fields["_bindings"] = []interface{}{}
				subInst.Fields["_table"] = instance.Fields["_table"]

				r.CallFunction(args[0], []interface{}{subInst})

				subWheres := subInst.Fields["_wheres"].([]string)
				subBindings := subInst.Fields["_bindings"].([]interface{})

				if len(subWheres) > 0 {
					wheres := instance.Fields["_wheres"].([]string)
					bindings := instance.Fields["_bindings"].([]interface{})

					clause := fmt.Sprintf("%s(%s)", prefix, buildWhereClause(subWheres))
					wheres = append(wheres, clause)
					bindings = append(bindings, subBindings...)

					instance.Fields["_wheres"] = wheres
					instance.Fields["_bindings"] = bindings
				}
				return instance
			}
			return instance
		}

		wheres := instance.Fields["_wheres"].([]string)
		bindings := instance.Fields["_bindings"].([]interface{})

		if len(args) == 2 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			val := args[1]
			wheres = append(wheres, fmt.Sprintf("%s%s = ?", prefix, col))
			bindings = append(bindings, val)
		} else if len(args) == 3 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			op := fmt.Sprintf("%v", args[1])
			val := args[2]
			wheres = append(wheres, fmt.Sprintf("%s%s %s ?", prefix, col, op))
			bindings = append(bindings, val)
		}

		instance.Fields["_wheres"] = wheres
		instance.Fields["_bindings"] = bindings
		return instance

	case "wherecolumn", "orwherecolumn":
		prefix := ""
		if methodLower == "orwherecolumn" {
			prefix = "OR "
		}
		if len(args) >= 2 {
			col1 := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			col2 := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[1])))
			op := "="
			if len(args) >= 3 {
				op = fmt.Sprintf("%v", args[1])
				col2 = quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[2])))
			}
			wheres := instance.Fields["_wheres"].([]string)
			wheres = append(wheres, fmt.Sprintf("%s%s %s %s", prefix, col1, op, col2))
			instance.Fields["_wheres"] = wheres
		}
		return instance

	case "wherenot", "orwherenot":
		prefix := ""
		if methodLower == "orwherenot" {
			prefix = "OR "
		}
		if len(args) >= 2 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			val := args[1]
			wheres := instance.Fields["_wheres"].([]string)
			bindings := instance.Fields["_bindings"].([]interface{})
			wheres = append(wheres, fmt.Sprintf("%sNOT (%s = ?)", prefix, col))
			bindings = append(bindings, val)
			instance.Fields["_wheres"] = wheres
			instance.Fields["_bindings"] = bindings
		}
		return instance

	case "wherelike", "orwherelike":
		prefix := ""
		if methodLower == "orwherelike" {
			prefix = "OR "
		}
		if len(args) >= 2 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			val := fmt.Sprintf("%v", args[1])
			if !strings.Contains(val, "%") {
				val = "%" + val + "%"
			}
			wheres := instance.Fields["_wheres"].([]string)
			bindings := instance.Fields["_bindings"].([]interface{})
			wheres = append(wheres, fmt.Sprintf("%s%s LIKE ?", prefix, col))
			bindings = append(bindings, val)
			instance.Fields["_wheres"] = wheres
			instance.Fields["_bindings"] = bindings
		}
		return instance

	case "wherein", "orwherein", "wherenotin", "orwherenotin":
		if len(args) >= 2 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			values := toInterfaceSlice(args[1])
			wheres := instance.Fields["_wheres"].([]string)
			bindings := instance.Fields["_bindings"].([]interface{})

			prefix := ""
			if strings.HasPrefix(methodLower, "or") {
				prefix = "OR "
			}
			operator := "IN"
			if strings.Contains(methodLower, "notin") {
				operator = "NOT IN"
			}

			if len(values) == 0 {
				emptyClause := "1 = 0"
				if strings.Contains(methodLower, "notin") {
					emptyClause = "1 = 1"
				}
				wheres = append(wheres, prefix+emptyClause)
			} else {
				wheres = append(wheres, fmt.Sprintf("%s%s %s (%s)", prefix, col, operator, placeholders(len(values))))
				bindings = append(bindings, values...)
			}
			instance.Fields["_wheres"] = wheres
			instance.Fields["_bindings"] = bindings
		}
		return instance

	case "wherenull", "orwherenull", "wherenotnull", "orwherenotnull":
		if len(args) >= 1 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			prefix := ""
			if strings.HasPrefix(methodLower, "or") {
				prefix = "OR "
			}
			op := "IS NULL"
			if strings.Contains(methodLower, "notnull") {
				op = "IS NOT NULL"
			}
			wheres := instance.Fields["_wheres"].([]string)
			wheres = append(wheres, fmt.Sprintf("%s%s %s", prefix, col, op))
			instance.Fields["_wheres"] = wheres
		}
		return instance

	case "wherebetween", "orwherebetween", "wherenotbetween", "orwherenotbetween":
		if len(args) >= 2 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			values := toInterfaceSlice(args[1])
			if len(values) >= 2 {
				prefix := ""
				if strings.HasPrefix(methodLower, "or") {
					prefix = "OR "
				}
				op := "BETWEEN"
				if strings.Contains(methodLower, "notbetween") {
					op = "NOT BETWEEN"
				}
				wheres := instance.Fields["_wheres"].([]string)
				bindings := instance.Fields["_bindings"].([]interface{})
				wheres = append(wheres, fmt.Sprintf("%s%s %s ? AND ?", prefix, col, op))
				bindings = append(bindings, values[0], values[1])
				instance.Fields["_wheres"] = wheres
				instance.Fields["_bindings"] = bindings
			}
		}
		return instance

	case "wheredate", "orwheredate", "whereyear", "orwhereyear", "wheremonth", "orwheremonth", "whereday", "orwhereday", "wheretime", "orwheretime":
		if len(args) >= 2 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			prefix := ""
			if strings.HasPrefix(methodLower, "or") {
				prefix = "OR "
			}
			fn := "DATE"
			if strings.Contains(methodLower, "year") {
				fn = "YEAR"
			} else if strings.Contains(methodLower, "month") {
				fn = "MONTH"
			} else if strings.Contains(methodLower, "day") {
				fn = "DAY"
			} else if strings.Contains(methodLower, "time") {
				fn = "TIME"
			}
			wheres := instance.Fields["_wheres"].([]string)
			bindings := instance.Fields["_bindings"].([]interface{})
			wheres = append(wheres, fmt.Sprintf("%s%s(%s) = ?", prefix, fn, col))
			bindings = append(bindings, args[1])
			instance.Fields["_wheres"] = wheres
			instance.Fields["_bindings"] = bindings
		}
		return instance

	case "wherejsoncontains", "orwherejsoncontains":
		if len(args) >= 2 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			val := args[1]
			prefix := ""
			if strings.HasPrefix(methodLower, "or") {
				prefix = "OR "
			}
			wheres := instance.Fields["_wheres"].([]string)
			bindings := instance.Fields["_bindings"].([]interface{})
			wheres = append(wheres, fmt.Sprintf("%sJSON_CONTAINS(%s, ?)", prefix, col))
			bindings = append(bindings, val)
			instance.Fields["_wheres"] = wheres
			instance.Fields["_bindings"] = bindings
		}
		return instance

	case "join", "innerjoin":
		if len(args) >= 4 {
			table := r.applyTablePrefix(fmt.Sprintf("%v", args[0]))
			first := r.applyColumnPrefix(fmt.Sprintf("%v", args[1]))
			op := fmt.Sprintf("%v", args[2])
			second := r.applyColumnPrefix(fmt.Sprintf("%v", args[3]))
			if _, ok := instance.Fields["_joins"]; !ok {
				instance.Fields["_joins"] = []string{}
			}
			join := fmt.Sprintf("INNER JOIN %s ON %s %s %s", table, first, op, second)
			instance.Fields["_joins"] = append(instance.Fields["_joins"].([]string), join)
		}
		return instance

	case "leftjoin":
		if len(args) >= 4 {
			table := r.applyTablePrefix(fmt.Sprintf("%v", args[0]))
			first := r.applyColumnPrefix(fmt.Sprintf("%v", args[1]))
			op := fmt.Sprintf("%v", args[2])
			second := r.applyColumnPrefix(fmt.Sprintf("%v", args[3]))
			if _, ok := instance.Fields["_joins"]; !ok {
				instance.Fields["_joins"] = []string{}
			}
			join := fmt.Sprintf("LEFT JOIN %s ON %s %s %s", table, first, op, second)
			instance.Fields["_joins"] = append(instance.Fields["_joins"].([]string), join)
		}
		return instance

	case "rightjoin":
		if len(args) >= 4 {
			table := r.applyTablePrefix(fmt.Sprintf("%v", args[0]))
			first := r.applyColumnPrefix(fmt.Sprintf("%v", args[1]))
			op := fmt.Sprintf("%v", args[2])
			second := r.applyColumnPrefix(fmt.Sprintf("%v", args[3]))
			if _, ok := instance.Fields["_joins"]; !ok {
				instance.Fields["_joins"] = []string{}
			}
			join := fmt.Sprintf("RIGHT JOIN %s ON %s %s %s", table, first, op, second)
			instance.Fields["_joins"] = append(instance.Fields["_joins"].([]string), join)
		}
		return instance

	case "groupby":
		if len(args) > 0 {
			cols := []string{}
			for _, a := range args {
				cols = append(cols, quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", a))))
			}
			instance.Fields["_groupBy"] = strings.Join(cols, ", ")
		}
		return instance

	case "having", "orhaving":
		if len(args) >= 3 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			op := fmt.Sprintf("%v", args[1])
			val := args[2]
			prefix := ""
			if methodLower == "orhaving" {
				prefix = "OR "
			}
			bindings := instance.Fields["_bindings"].([]interface{})
			if _, ok := instance.Fields["_having"]; !ok {
				instance.Fields["_having"] = []string{}
			}
			having := instance.Fields["_having"].([]string)
			having = append(having, fmt.Sprintf("%s%s %s ?", prefix, col, op))
			instance.Fields["_having"] = having
			bindings = append(bindings, val)
			instance.Fields["_bindings"] = bindings
		}
		return instance

	case "orderby":
		if len(args) >= 2 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			dir := strings.ToUpper(fmt.Sprintf("%v", args[1]))
			if dir != "ASC" && dir != "DESC" {
				dir = "ASC"
			}
			instance.Fields["_order"] = fmt.Sprintf("%s %s", col, dir)
		}
		return instance

	case "orderbydesc":
		if len(args) >= 1 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			instance.Fields["_order"] = fmt.Sprintf("%s DESC", col)
		}
		return instance

	case "orderbyasc":
		if len(args) >= 1 {
			col := quoteIdentifier(r.applyColumnPrefix(fmt.Sprintf("%v", args[0])))
			instance.Fields["_order"] = fmt.Sprintf("%s ASC", col)
		}
		return instance

	case "reorder":
		delete(instance.Fields, "_order")
		return instance

	case "latest", "oldest":
		col := "created_at"
		if len(args) >= 1 {
			col = fmt.Sprintf("%v", args[0])
		}
		dir := "DESC"
		if methodLower == "oldest" {
			dir = "ASC"
		}
		instance.Fields["_order"] = fmt.Sprintf("%s %s", quoteIdentifier(r.applyColumnPrefix(col)), dir)
		return instance

	case "inrandomorder":
		if r.Env != nil && strings.ToLower(r.Env["DB"]) == "sqlite" {
			instance.Fields["_order"] = "RANDOM()"
		} else {
			instance.Fields["_order"] = "RAND()"
		}
		return instance

	case "limit", "take":
		if len(args) >= 1 {
			instance.Fields["_limit"] = toInt(args[0])
		}
		return instance

	case "offset", "skip":
		if len(args) >= 1 {
			instance.Fields["_offset"] = toInt(args[0])
		}
		return instance

	case "forpage":
		if len(args) >= 2 {
			page := toInt(args[0])
			perPage := toInt(args[1])
			if page < 1 {
				page = 1
			}
			if perPage < 1 {
				perPage = 15
			}
			instance.Fields["_limit"] = perPage
			instance.Fields["_offset"] = (page - 1) * perPage
		}
		return instance

	case "tosql":
		sel := "*"
		if s, ok := instance.Fields["_select"].(string); ok {
			sel = s
		}
		sqlStr, _ := r.buildSelectQuery(instance, sel)
		return sqlStr

	case "getbindings":
		return instance.Fields["_bindings"].([]interface{})

	case "dump":
		sel := "*"
		if s, ok := instance.Fields["_select"].(string); ok {
			sel = s
		}
		sqlStr, bindings := r.buildSelectQuery(instance, sel)
		fmt.Printf("[GranDB Dump] SQL: %s | Bindings: %v\n", sqlStr, bindings)
		return instance

	case "dd":
		sel := "*"
		if s, ok := instance.Fields["_select"].(string); ok {
			sel = s
		}
		sqlStr, bindings := r.buildSelectQuery(instance, sel)
		panic(fmt.Sprintf("[GranDB DD] SQL: %s | Bindings: %v", sqlStr, bindings))

	case "get":
		return r.executeGetMethod(instance, args)

	case "paginate":
		return r.executePaginateMethod(instance, args)

	case "chunk":
		return r.executeChunkMethod(instance, args)

	case "count":
		return r.executeCountMethod(instance, args)

	case "sum", "avg", "min", "max":
		return r.executeAggregateMethod(instance, methodLower, args)

	case "first":
		return r.executeFirstMethod(instance, args)

	case "firstwhere":
		if len(args) >= 2 {
			r.executeGranDBMethod(instance, "where", args)
			return r.executeFirstMethod(instance, nil)
		}
		return nil

	case "sole":
		return r.executeSoleMethod(instance, args)

	case "firstorfail":
		return r.executeFirstOrFailMethod(instance, args)

	case "find":
		return r.executeFindMethod(instance, args)

	case "findmany":
		return r.executeFindManyMethod(instance, args)

	case "findorfail":
		return r.executeFindOrFailMethod(instance, args)

	case "value":
		return r.executeValueMethod(instance, args)

	case "pluck":
		return r.executePluckMethod(instance, args)

	case "exists":
		return r.executeExistsMethod(instance, false)

	case "doesntexist":
		return r.executeExistsMethod(instance, true)

	case "insert":
		return r.executeInsertMethod(instance, args, false)

	case "insertgetid":
		return r.executeInsertMethod(instance, args, true)

	case "update":
		return r.executeUpdateMethod(instance, args)

	case "updateorinsert":
		return r.executeUpdateOrInsertMethod(instance, args)

	case "touch":
		return r.executeTouchMethod(instance, args)

	case "increment":
		return r.executeIncrementMethod(instance, args, false)

	case "decrement":
		return r.executeIncrementMethod(instance, args, true)

	case "delete":
		return r.executeDeleteMethod(instance)

	case "deleteall":
		return r.executeDeleteAllMethod(instance)

	case "truncate":
		return r.executeTruncateMethod(instance)

	default:
		panic(fmt.Sprintf("GranDB Error: Método '%s' no encontrado", method))
	}
}

func (r *Runtime) isCallable(val interface{}) bool {
	if val == nil {
		return false
	}
	switch val.(type) {
	case *CapturedFunction, *parser.FunctionLiteral, *parser.MethodStatement, *BoundMethod:
		return true
	default:
		return false
	}
}
