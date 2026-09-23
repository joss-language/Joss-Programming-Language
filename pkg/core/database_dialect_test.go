package core

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

func TestDatabaseDialectExpressionsAndColumnProbes(t *testing.T) {
	tests := []struct {
		driver string
		random string
		probe  string
	}{
		{"sqlite", "RANDOM()", "SELECT * FROM `items` LIMIT 0"},
		{"mysql", "RAND()", "SELECT * FROM `items` LIMIT 0"},
		{"postgres", "RANDOM()", "SELECT * FROM `items` LIMIT 0"},
		{"sqlserver", "NEWID()", "SELECT TOP 0 * FROM `items`"},
	}
	for _, test := range tests {
		t.Run(test.driver, func(t *testing.T) {
			dialect := dialectFor(test.driver)
			if got := dialect.randomExpression(); got != test.random {
				t.Fatalf("randomExpression = %q, want %q", got, test.random)
			}
			if got := dialect.columnProbe("`items`"); got != test.probe {
				t.Fatalf("columnProbe = %q, want %q", got, test.probe)
			}
		})
	}
}

func TestDatabaseDialectDateParts(t *testing.T) {
	tests := map[string]string{
		"sqlite":    "strftime('%Y', `created_at`)",
		"mysql":     "YEAR(`created_at`)",
		"postgres":  "EXTRACT(YEAR FROM `created_at`)",
		"sqlserver": "DATEPART(year, `created_at`)",
	}
	for driver, want := range tests {
		if got := dialectFor(driver).datePartExpression("year", "`created_at`"); got != want {
			t.Errorf("%s year expression = %q, want %q", driver, got, want)
		}
	}
}

func TestGranDBWhereYearSQLiteAcceptsNumericYear(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE events (id INTEGER PRIMARY KEY, happened_at TEXT); INSERT INTO events VALUES (1, '2026-09-23 03:00:00')`); err != nil {
		t.Fatal(err)
	}
	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"events"})
	r.executeGranDBMethod(instance, "whereYear", []interface{}{"happened_at", int64(2026)})
	rows := r.executeGranDBMethod(instance, "get", nil).([]map[string]interface{})
	if len(rows) != 1 {
		t.Fatalf("whereYear returned %d rows, want 1", len(rows))
	}
}

func TestDatabaseDialectJSONContains(t *testing.T) {
	tests := map[string]string{
		"sqlite":    "EXISTS (SELECT 1 FROM json_each(`tags`) WHERE json_each.value = ?)",
		"mysql":     "JSON_CONTAINS(`tags`, ?)",
		"postgres":  "`tags`::jsonb @> ?::jsonb",
		"sqlserver": "EXISTS (SELECT 1 FROM OPENJSON(`tags`) WHERE value = ?)",
	}
	for driver, want := range tests {
		predicate, _, err := dialectFor(driver).jsonContainsPredicate("`tags`", "admin")
		if err != nil {
			t.Fatal(err)
		}
		if predicate != want {
			t.Errorf("%s predicate = %q, want %q", driver, predicate, want)
		}
	}
}

func TestGranDBWhereJSONContainsSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, roles TEXT); INSERT INTO users VALUES (1, '["admin","editor"]'), (2, '["reader"]')`); err != nil {
		t.Fatal(err)
	}
	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"users"})
	r.executeGranDBMethod(instance, "whereJsonContains", []interface{}{"roles", "admin"})
	rows := r.executeGranDBMethod(instance, "get", nil).([]map[string]interface{})
	if len(rows) != 1 || toInt64(rows[0]["id"]) != 1 {
		t.Fatalf("whereJsonContains returned %#v", rows)
	}
}

func TestDatabaseDialectsCompileUpsertWithStableBindings(t *testing.T) {
	rows := []map[string]interface{}{
		{"name": "Ada", "id": int64(1)},
		{"id": int64(2), "name": "Lin"},
	}
	tests := map[string][]string{
		"sqlite":    {"ON CONFLICT (`id`) DO UPDATE", "`name` = excluded.`name`"},
		"postgres":  {"ON CONFLICT (`id`) DO UPDATE", "`name` = excluded.`name`"},
		"mysql":     {"ON DUPLICATE KEY UPDATE", "`name` = VALUES(`name`)"},
		"sqlserver": {"MERGE INTO `items`", "WHEN MATCHED THEN UPDATE", "WHEN NOT MATCHED THEN INSERT"},
	}
	for driver, fragments := range tests {
		t.Run(driver, func(t *testing.T) {
			query, bindings, err := dialectFor(driver).compileUpsert("`items`", rows, []string{"id"}, []string{"name"})
			if err != nil {
				t.Fatal(err)
			}
			for _, fragment := range fragments {
				if !strings.Contains(query, fragment) {
					t.Fatalf("query %q does not contain %q", query, fragment)
				}
			}
			want := []interface{}{int64(1), "Ada", int64(2), "Lin"}
			if len(bindings) != len(want) {
				t.Fatalf("bindings = %#v, want %#v", bindings, want)
			}
			for index := range want {
				if bindings[index] != want[index] {
					t.Fatalf("bindings = %#v, want %#v", bindings, want)
				}
			}
		})
	}
}

func TestGranDBUpsertSQLiteInsertsAndUpdates(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}

	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"items"})
	rows := []interface{}{
		map[string]interface{}{"id": int64(1), "name": "Ada"},
		map[string]interface{}{"id": int64(2), "name": "Lin"},
	}
	if affected := r.executeGranDBMethod(instance, "upsert", []interface{}{rows, "id", []interface{}{"name"}}).(int64); affected != 2 {
		t.Fatalf("first upsert affected %d rows, want 2", affected)
	}
	if affected := r.executeGranDBMethod(instance, "upsert", []interface{}{map[string]interface{}{"id": int64(1), "name": "Ada Lovelace"}, "id"}).(int64); affected != 1 {
		t.Fatalf("second upsert affected %d rows, want 1", affected)
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM items WHERE id = 1`).Scan(&name); err != nil || name != "Ada Lovelace" {
		t.Fatalf("name = %q, err = %v", name, err)
	}
}

func TestGranDBUpsertBatchesAtomically(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT UNIQUE, active INTEGER)`); err != nil {
		t.Fatal(err)
	}
	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"items"})
	rows := make([]interface{}, 400)
	for index := range rows {
		rows[index] = map[string]interface{}{"id": int64(index + 1), "name": fmt.Sprintf("item-%d", index+1), "active": int64(1)}
	}
	if affected := r.executeGranDBMethod(instance, "upsert", []interface{}{rows, "id", []interface{}{"name", "active"}}).(int64); affected != 400 {
		t.Fatalf("upsert affected %d rows, want 400", affected)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&count); err != nil || count != 400 {
		t.Fatalf("count = %d, err = %v", count, err)
	}
}

func TestDatabaseDialectRejectsMismatchedUpsertRows(t *testing.T) {
	_, _, err := dialectFor("sqlite").compileUpsert("`items`", []map[string]interface{}{
		{"id": int64(1), "name": "Ada"},
		{"id": int64(2)},
	}, []string{"id"}, nil)
	if err == nil {
		t.Fatal("mismatched row columns were accepted")
	}
}

func TestGranDBInsertManyBatchesAndCommits(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, active INTEGER)`); err != nil {
		t.Fatal(err)
	}
	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"items"})

	rows := make([]interface{}, 400) // 1200 bindings: two SQLite-safe batches.
	for index := range rows {
		rows[index] = map[string]interface{}{"id": int64(index + 1), "name": "item", "active": int64(1)}
	}
	if affected := r.executeGranDBMethod(instance, "insertMany", []interface{}{rows}).(int64); affected != 400 {
		t.Fatalf("insertMany affected %d rows, want 400", affected)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&count); err != nil || count != 400 {
		t.Fatalf("count = %d, err = %v", count, err)
	}
	if r.activeTx != nil {
		t.Fatal("insertMany left a transaction active")
	}
}

func TestGranDBInsertManyRollsBackAllBatches(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"items"})

	rows := make([]interface{}, 501) // two batches at SQLite's 999 parameter limit.
	for index := range rows {
		id := index + 1
		if index == 500 {
			id = 1
		}
		rows[index] = map[string]interface{}{"id": int64(id), "name": "item"}
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("duplicate key should fail insertMany")
			}
		}()
		r.executeGranDBMethod(instance, "insertMany", []interface{}{rows})
	}()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed insertMany left %d rows, err = %v", count, err)
	}
	if r.activeTx != nil {
		t.Fatal("failed insertMany left a transaction active")
	}
}
