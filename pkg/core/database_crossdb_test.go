package core

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
)

// TestGranDBCrossDatabaseContract is always exercised with SQLite. The same
// contract runs against server engines when their DSN variables are present:
// JOSS_TEST_MYSQL_DSN, JOSS_TEST_POSTGRES_DSN and JOSS_TEST_SQLSERVER_DSN.
func TestGranDBCrossDatabaseContract(t *testing.T) {
	tests := []struct {
		name       string
		driver     string
		sqlDriver  string
		dsnEnv     string
		defaultDSN string
	}{
		{name: "sqlite", driver: "sqlite", sqlDriver: "sqlite", defaultDSN: ":memory:"},
		{name: "mysql", driver: "mysql", sqlDriver: "mysql", dsnEnv: "JOSS_TEST_MYSQL_DSN"},
		{name: "postgres", driver: "postgres", sqlDriver: postgresSQLDriver, dsnEnv: "JOSS_TEST_POSTGRES_DSN"},
		{name: "sqlserver", driver: "sqlserver", sqlDriver: sqlServerDriverName, dsnEnv: "JOSS_TEST_SQLSERVER_DSN"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := test.defaultDSN
			if test.dsnEnv != "" {
				dsn = os.Getenv(test.dsnEnv)
				if dsn == "" {
					t.Skipf("set %s to run the %s integration contract", test.dsnEnv, test.name)
				}
			}
			db, err := sql.Open(test.sqlDriver, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if err := db.Ping(); err != nil {
				t.Fatal(err)
			}

			const table = "joss_grandb_contract"
			_, _ = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
			if _, err := db.Exec(fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY, name VARCHAR(100) NOT NULL UNIQUE, active INTEGER NOT NULL)", table)); err != nil {
				t.Fatal(err)
			}
			defer db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table))

			r := NewRuntime()
			r.DB = db
			r.Env = map[string]string{"DB": test.driver, "PREFIX": ""}
			builder := func() *Instance {
				instance := &Instance{Fields: make(map[string]interface{})}
				r.executeGranDBMethod(instance, "table", []interface{}{table})
				return instance
			}

			rows := []interface{}{
				map[string]interface{}{"id": int64(1), "name": "Ada", "active": int64(1)},
				map[string]interface{}{"id": int64(2), "name": "Lin", "active": int64(1)},
			}
			if affected := r.executeGranDBMethod(builder(), "insertMany", []interface{}{rows}).(int64); affected < 1 {
				t.Fatalf("insertMany affected %d rows", affected)
			}
			if count := r.executeGranDBMethod(builder(), "count", nil); toInt64(count) != 2 {
				t.Fatalf("count = %v, want 2", count)
			}
			if affected := r.executeGranDBMethod(builder(), "upsert", []interface{}{map[string]interface{}{"id": int64(1), "name": "Ada", "active": int64(0)}, "id", []interface{}{"active"}}).(int64); affected < 1 {
				t.Fatalf("upsert affected %d rows", affected)
			}
			query := builder()
			r.executeGranDBMethod(query, "where", []interface{}{"id", int64(1)})
			row := r.executeGranDBMethod(query, "first", nil).(map[string]interface{})
			if toInt64(row["active"]) != 0 {
				t.Fatalf("upserted active = %v, want 0", row["active"])
			}
			query = builder()
			r.executeGranDBMethod(query, "where", []interface{}{"id", int64(2)})
			if updated := r.executeGranDBMethod(query, "update", []interface{}{map[string]interface{}{"active": int64(0)}}); updated != true {
				t.Fatalf("update returned %v", updated)
			}
			query = builder()
			r.executeGranDBMethod(query, "where", []interface{}{"id", int64(2)})
			if deleted := r.executeGranDBMethod(query, "delete", nil); deleted != true {
				t.Fatalf("delete returned %v", deleted)
			}
			if count := r.executeGranDBMethod(builder(), "count", nil); toInt64(count) != 1 {
				t.Fatalf("final count = %v, want 1", count)
			}
		})
	}
}
