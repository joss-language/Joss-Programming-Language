package jit

import (
	"database/sql"
	"testing"
)

func TestJITDatabaseBridgeUsesSharedResultNormalization(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE bridge_values (id INTEGER PRIMARY KEY, value TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}

	dbBridgeMu.Lock()
	previousDB := dbInstance
	dbInstance = db
	dbBridgeMu.Unlock()
	t.Cleanup(func() {
		dbBridgeMu.Lock()
		dbInstance = previousDB
		dbBridgeMu.Unlock()
	})

	if result := handleDBExecute(nativeString("INSERT INTO bridge_values (value) VALUES (?)"), nativeSlice([]interface{}{"joss"})); result != 1 {
		t.Fatalf("JIT INSERT result = %d, want generated row ID 1", result)
	}
	query := nativeString("SELECT id AS number, value AS text, NULL AS empty FROM bridge_values WHERE id = ?")
	result := handleDBQuery(query, nativeSlice([]interface{}{int64(1)}))
	rows, ok := readNativeValue(result).([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("JIT query result = %#v, want one row", readNativeValue(result))
	}
	row, ok := rows[0].(map[string]interface{})
	if !ok {
		t.Fatalf("JIT query row = %#v, want map", rows[0])
	}
	if row["number"] != int64(1) || row["text"] != "joss" || row["empty"] != nil {
		t.Fatalf("JIT row normalization = %#v, want number=1 text=joss empty=null", row)
	}
}
