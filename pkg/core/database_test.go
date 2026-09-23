package core

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

func TestGranDBCallableWhere(t *testing.T) {
	r := NewRuntime()
	inst := &Instance{
		Class:  nil,
		Fields: make(map[string]interface{}),
	}

	// Test case-insensitive orWhere and orWhereLike
	r.executeGranDBMethod(inst, "table", []interface{}{"pub_packages"})
	r.executeGranDBMethod(inst, "where", []interface{}{"is_deprecated", 0})
	r.executeGranDBMethod(inst, "orWhereLike", []interface{}{"name", "joss"})

	wheres := inst.Fields["_wheres"].([]string)
	if len(wheres) != 2 {
		t.Fatalf("Se esperaban 2 condiciones where, se obtuvieron: %d", len(wheres))
	}

	sqlStr, _ := r.buildSelectQuery(inst, "*")
	expected := "SELECT * FROM `js_pub_packages` WHERE `is_deprecated` = ? OR `name` LIKE ?"
	if sqlStr != expected {
		t.Errorf("SQL generado incorrecto.\nEsperado: %s\nObtenido: %s", expected, sqlStr)
	}
}

func TestGranDBAcceptsCanonicalMapInsertAndRejectsParallelArrays(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (name TEXT, amount INTEGER)`); err != nil {
		t.Fatal(err)
	}

	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"items"})

	removed := r.executeInsertMethod(instance, []interface{}{[]interface{}{"name", "amount"}, []interface{}{"old", int64(1)}}, false)
	if removed != false {
		t.Fatalf("parallel-array insert returned %v, want false", removed)
	}
	if inserted := r.executeInsertMethod(instance, []interface{}{map[string]interface{}{"name": "current", "amount": int64(2)}}, false); inserted != true {
		t.Fatalf("map insert returned %v, want true", inserted)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("row count = %d, err = %v", count, err)
	}
}

func TestGranDBCountPreservesWhere(t *testing.T) {
	r := NewRuntime()
	inst := &Instance{
		Class:  nil,
		Fields: make(map[string]interface{}),
	}

	r.executeGranDBMethod(inst, "table", []interface{}{"pub_packages"})
	r.executeGranDBMethod(inst, "where", []interface{}{"is_deprecated", 0})
	r.executeGranDBMethod(inst, "whereLike", []interface{}{"name", "backup"})

	// Verify buildSelectQuery before count
	sqlBefore, _ := r.buildSelectQuery(inst, "COUNT(*)")
	expectedBefore := "SELECT COUNT(*) FROM `js_pub_packages` WHERE `is_deprecated` = ? AND `name` LIKE ?"
	if sqlBefore != expectedBefore {
		t.Errorf("SQL de count incorrecto.\nEsperado: %s\nObtenido: %s", expectedBefore, sqlBefore)
	}

	// Verify wheres are preserved for subsequent get()
	wheres := inst.Fields["_wheres"].([]string)
	if len(wheres) != 2 {
		t.Fatalf("Wheres se borraron tras count! Se esperaban 2, se obtuvieron %d", len(wheres))
	}
}

func TestGranDBTableResetsQueryStateForNewQuery(t *testing.T) {
	r := NewRuntime()
	inst := &Instance{
		Class:  nil,
		Fields: make(map[string]interface{}),
	}

	// 1st query: sync_change_log with user_id and client_change_id
	r.executeGranDBMethod(inst, "table", []interface{}{"sync_change_log"})
	r.executeGranDBMethod(inst, "where", []interface{}{"user_id", 60})
	r.executeGranDBMethod(inst, "where", []interface{}{"client_change_id", "abc-123"})

	wheres := inst.Fields["_wheres"].([]string)
	if len(wheres) != 2 {
		t.Fatalf("Expected 2 where clauses, got %d", len(wheres))
	}

	// 2nd query: switch table to user_recent_plays
	r.executeGranDBMethod(inst, "table", []interface{}{"user_recent_plays"})
	r.executeGranDBMethod(inst, "where", []interface{}{"user_id", 60})
	r.executeGranDBMethod(inst, "where", []interface{}{"track_id", 79})

	sqlStr, bindings := r.buildSelectQuery(inst, "*")
	expected := "SELECT * FROM `js_user_recent_plays` WHERE `user_id` = ? AND `track_id` = ?"
	if sqlStr != expected {
		t.Errorf("SQL incorrecto tras cambiar de tabla.\nEsperado: %s\nObtenido: %s", expected, sqlStr)
	}
	if len(bindings) != 2 || bindings[0] != 60 || bindings[1] != 79 {
		t.Errorf("Bindings incorrectos tras cambiar de tabla: %v", bindings)
	}
}

func TestGranDBFindManyExecutesWhereIn(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT); INSERT INTO items VALUES (1, 'one'), (2, 'two'), (3, 'three')`); err != nil {
		t.Fatal(err)
	}

	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"items"})

	rows := r.executeGranDBMethod(instance, "findMany", []interface{}{[]interface{}{int64(1), int64(3)}}).([]map[string]interface{})
	if len(rows) != 2 {
		t.Fatalf("findMany returned %d rows, want 2: %#v", len(rows), rows)
	}
}

func TestGranDBFirstOfFailAliasReachesFirstOrFail(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}

	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"items"})

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("firstofail should fail when no row exists")
		}
		if message := fmt.Sprint(recovered); !strings.Contains(message, "No se encontró") {
			t.Fatalf("firstofail reached wrong failure: %v", recovered)
		}
	}()
	r.executeGranDBMethod(instance, "firstofail", nil)
}

func TestGranDBUpdateWithoutWhereIsRejected(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT); INSERT INTO items VALUES (1, 'one'), (2, 'two')`); err != nil {
		t.Fatal(err)
	}

	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"items"})
	if updated := r.executeGranDBMethod(instance, "update", []interface{}{map[string]interface{}{"name": "changed"}}); updated != false {
		t.Fatalf("update without WHERE returned %v, want false", updated)
	}
	var changed int
	if err := db.QueryRow(`SELECT COUNT(*) FROM items WHERE name = 'changed'`).Scan(&changed); err != nil || changed != 0 {
		t.Fatalf("unsafe update changed %d rows: %v", changed, err)
	}
}

func TestGranDBRejectsStructuralSQLInjection(t *testing.T) {
	r := NewRuntime()
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}

	for name, run := range map[string]func(){
		"table": func() {
			instance := &Instance{Fields: make(map[string]interface{})}
			r.executeGranDBMethod(instance, "table", []interface{}{"users; DROP TABLE users"})
		},
		"column": func() {
			instance := &Instance{Fields: make(map[string]interface{})}
			r.executeGranDBMethod(instance, "table", []interface{}{"users"})
			r.executeGranDBMethod(instance, "where", []interface{}{"name) OR 1=1 --", "Ada"})
		},
		"operator": func() {
			instance := &Instance{Fields: make(map[string]interface{})}
			r.executeGranDBMethod(instance, "table", []interface{}{"users"})
			r.executeGranDBMethod(instance, "where", []interface{}{"name", "= ? OR 1=1 --", "Ada"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("unsafe SQL structure was accepted")
				}
			}()
			run()
		})
	}
}

func TestGranDBBindsSQLLookingStringsAsData(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"items"})
	if inserted := r.executeGranDBMethod(instance, "insert", []interface{}{map[string]interface{}{"value": "CURRENT_TIMESTAMP"}}); inserted != true {
		t.Fatalf("insert returned %v", inserted)
	}
	var value string
	if err := db.QueryRow(`SELECT value FROM items`).Scan(&value); err != nil || value != "CURRENT_TIMESTAMP" {
		t.Fatalf("stored value = %q, err = %v", value, err)
	}
}

func TestGranDBSimpleAndCursorPagination(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, active INTEGER); INSERT INTO items VALUES (1, 1), (2, 1), (3, 1), (4, 0), (5, 1)`); err != nil {
		t.Fatal(err)
	}
	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"items"})
	r.executeGranDBMethod(instance, "where", []interface{}{"active", int64(1)})

	simple := r.executeGranDBMethod(instance, "simplePaginate", []interface{}{int64(2), int64(1)}).(map[string]interface{})
	if len(simple["data"].([]map[string]interface{})) != 2 || simple["has_more"] != true {
		t.Fatalf("simplePaginate = %#v", simple)
	}
	// Pagination runs on a clone, so the original filter remains available.
	if len(instance.Fields["_wheres"].([]string)) != 1 {
		t.Fatal("simplePaginate consumed the original builder")
	}

	first := r.executeGranDBMethod(instance, "cursorPaginate", []interface{}{int64(2), nil, "id"}).(map[string]interface{})
	if first["next_cursor"] == nil || first["has_more"] != true {
		t.Fatalf("first cursor page = %#v", first)
	}
	second := r.executeGranDBMethod(instance, "cursorPaginate", []interface{}{int64(2), first["next_cursor"], "id"}).(map[string]interface{})
	items := second["data"].([]map[string]interface{})
	if len(items) != 2 || second["has_more"] != false || toInt64(items[1]["id"]) != 5 {
		t.Fatalf("second cursor page = %#v", second)
	}
}

func TestGranDBExplainSQLitePreservesBuilder(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, active INTEGER); CREATE INDEX items_active ON items(active)`); err != nil {
		t.Fatal(err)
	}
	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"items"})
	r.executeGranDBMethod(instance, "where", []interface{}{"active", int64(1)})
	plan := r.executeGranDBMethod(instance, "explain", nil).([]map[string]interface{})
	if len(plan) == 0 {
		t.Fatal("explain returned an empty plan")
	}
	if len(instance.Fields["_wheres"].([]string)) != 1 {
		t.Fatal("explain consumed the builder")
	}
}
