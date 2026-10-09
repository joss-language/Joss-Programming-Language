package core

import (
	"database/sql"
	"fmt"
)

// SQLite Native Class Implementation
// Usage:
//
//	$db = new SQLite()
//	$db->open("temp_backup/song.db")
//	$songs = $db->query("SELECT * FROM song")
//	$db->close()
func (r *Runtime) executeSQLiteMethod(instance *Instance, method string, args []interface{}) interface{} {
	if instance == nil {
		panic("Internal Error: SQLite method called with nil instance")
	}
	if instance.Fields == nil {
		instance.Fields = make(map[string]interface{})
	}

	switch method {
	case "open":
		if len(args) < 1 {
			fmt.Println("[SQLite Error] open() requires database path")
			return false
		}
		path := fmt.Sprintf("%v", args[0])
		db, err := sql.Open("sqlite", path)
		if err != nil {
			fmt.Printf("[SQLite Error] Open failed: %v\n", err)
			return false
		}
		instance.Fields["_db"] = db
		return true

	case "query":
		if len(args) < 1 {
			fmt.Println("[SQLite Error] query() requires SQL string")
			return nil
		}
		sqlStr := fmt.Sprintf("%v", args[0])
		dbVal, ok := instance.Fields["_db"]
		if !ok || dbVal == nil {
			fmt.Println("[SQLite Error] Query called on closed or uninitialized connection")
			return nil
		}
		db := dbVal.(*sql.DB)

		bindings := []interface{}{}
		if len(args) > 1 {
			if bList, ok := args[1].([]interface{}); ok {
				bindings = bList
			}
		}

		rows, err := db.Query(sqlStr, bindings...)
		if err != nil {
			fmt.Printf("[SQLite Error] Query failed: %v\n", err)
			return nil
		}
		defer rows.Close()

		rowsMap := rowsToMap(rows)
		var result []interface{}
		for _, r := range rowsMap {
			result = append(result, r)
		}
		return result

	case "close":
		dbVal, ok := instance.Fields["_db"]
		if ok && dbVal != nil {
			db := dbVal.(*sql.DB)
			db.Close()
			instance.Fields["_db"] = nil
		}
		return true
	}
	return nil
}
