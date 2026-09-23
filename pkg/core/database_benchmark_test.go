package core

import (
	"database/sql"
	"fmt"
	"testing"
)

func BenchmarkGranDBCompileSelect(b *testing.B) {
	r := NewRuntime()
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	instance := &Instance{Fields: make(map[string]interface{})}
	r.executeGranDBMethod(instance, "table", []interface{}{"users"})
	r.executeGranDBMethod(instance, "where", []interface{}{"active", int64(1)})
	r.executeGranDBMethod(instance, "whereLike", []interface{}{"name", "ada"})
	r.executeGranDBMethod(instance, "orderBy", []interface{}{"id", "desc"})
	r.executeGranDBMethod(instance, "limit", []interface{}{int64(25)})
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_, _ = r.buildSelectQuery(instance, "*")
	}
}

func BenchmarkGranDBCompileUpsert100Rows(b *testing.B) {
	rows := make([]map[string]interface{}, 100)
	for index := range rows {
		rows[index] = map[string]interface{}{
			"id":     int64(index + 1),
			"name":   fmt.Sprintf("user-%d", index),
			"active": true,
		}
	}
	dialect := dialectFor("postgres")
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_, _, err := dialect.compileUpsert("`users`", rows, []string{"id"}, []string{"name", "active"})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGranDBReadRows(b *testing.B) {
	for _, rowCount := range []int{100, 1000} {
		b.Run(fmt.Sprintf("Rows%d", rowCount), func(b *testing.B) {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				b.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, active INTEGER)`); err != nil {
				b.Fatal(err)
			}
			tx, err := db.Begin()
			if err != nil {
				b.Fatal(err)
			}
			statement, err := tx.Prepare(`INSERT INTO items (id, name, active) VALUES (?, ?, ?)`)
			if err != nil {
				b.Fatal(err)
			}
			for index := 0; index < rowCount; index++ {
				if _, err := statement.Exec(index+1, "item", 1); err != nil {
					b.Fatal(err)
				}
			}
			_ = statement.Close()
			if err := tx.Commit(); err != nil {
				b.Fatal(err)
			}

			r := NewRuntime()
			r.DB = db
			r.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				instance := &Instance{Fields: make(map[string]interface{})}
				r.executeGranDBMethod(instance, "table", []interface{}{"items"})
				rows := r.executeGranDBMethod(instance, "get", nil).([]map[string]interface{})
				if len(rows) != rowCount {
					b.Fatalf("got %d rows, want %d", len(rows), rowCount)
				}
			}
		})
	}
}
