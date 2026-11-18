package core

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
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
				if !strings.Contains(strings.ToLower(dsn), "test") {
					t.Fatalf("refusing database integration test: %s must identify a disposable test database", test.dsnEnv)
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

			suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
			table := "joss_grandb_contract_" + suffix
			jossTable := "joss_grandb_joss_contract_" + suffix
			booleanType := "BOOLEAN"
			if test.driver == "sqlserver" {
				booleanType = "BIT"
			}
			_, _ = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
			_, _ = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", jossTable))
			if _, err := db.Exec(fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY, name VARCHAR(100) NOT NULL UNIQUE, active %s NOT NULL)", table, booleanType)); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY, name VARCHAR(100) NOT NULL UNIQUE, active %s NOT NULL)", jossTable, booleanType)); err != nil {
				t.Fatal(err)
			}
			defer db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
			defer db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", jossTable))

			r := NewRuntime()
			defer r.Free()
			r.DB = db
			r.Env = map[string]string{"DB": test.driver, "PREFIX": ""}
			var jossOutput strings.Builder
			r.Out = &jossOutput
			jossSource := fmt.Sprintf(`
GranDB::table("%s")->insert({"id": 1, "name": "Joss", "active": true});
var $first = GranDB::table("%s")->where("id", 1)->first();
echo $first["name"];
GranDB::table("%s")->where("id", 1)->update({"name": "Joss Updated"});
var $updated = GranDB::table("%s")->where("id", 1)->first();
echo $updated["name"];
GranDB::table("%s")->where("id", 1)->delete();
echo GranDB::table("%s")->count();
`, jossTable, jossTable, jossTable, jossTable, jossTable, jossTable)
			parsed := parser.NewParser(parser.NewLexer(jossSource))
			jossProgram := parsed.ParseProgram()
			if issues := parsed.Diagnostics(); len(issues) > 0 {
				t.Fatalf("parse canonical GranDB Joss contract: %v", issues)
			}
			units := append([]analyzer.SourceUnit{{Path: "grandb_crossdb.joss", Program: jossProgram}}, std.GetStdlibUnits()...)
			report := AnalyzeSourceUnits(units)
			if report.Prepared == nil || report.Prepared.HasErrors() {
				t.Fatalf("analyze canonical GranDB Joss contract: %v", report.Diagnostics)
			}
			r.ExecutePrepared(report.Prepared)
			if got, want := strings.TrimSpace(jossOutput.String()), "Joss\nJoss Updated\n0"; got != want {
				t.Fatalf("canonical GranDB Joss contract output=%q, want %q", got, want)
			}

			builder := func() *Instance {
				instance := &Instance{Fields: make(map[string]interface{})}
				r.executeGranDBMethod(instance, "table", []interface{}{table})
				return instance
			}

			rows := []interface{}{
				map[string]interface{}{"id": int64(1), "name": "Ada", "active": true},
				map[string]interface{}{"id": int64(2), "name": "Lin", "active": true},
			}
			if affected := r.executeGranDBMethod(builder(), "insertMany", []interface{}{rows}).(int64); affected < 1 {
				t.Fatalf("insertMany affected %d rows", affected)
			}
			if count := r.executeGranDBMethod(builder(), "count", nil); toInt64(count) != 2 {
				t.Fatalf("count = %v, want 2", count)
			}
			if affected := r.executeGranDBMethod(builder(), "upsert", []interface{}{map[string]interface{}{"id": int64(1), "name": "Ada", "active": false}, "id", []interface{}{"active"}}).(int64); affected < 1 {
				t.Fatalf("upsert affected %d rows", affected)
			}
			query := builder()
			r.executeGranDBMethod(query, "where", []interface{}{"id", int64(1)})
			row := r.executeGranDBMethod(query, "first", nil).(map[string]interface{})
			if castModelValue(&modelMetadata{Casts: map[string]string{"active": "bool"}}, "active", row["active"]) != false {
				t.Fatalf("upserted active = %v, want 0", row["active"])
			}

			modelClass := &parser.ClassStatement{Name: &parser.Identifier{Value: "ContractModel"}, Body: &parser.BlockStatement{}}
			metadata := &modelMetadata{Class: modelClass, ClassName: "ContractModel", Table: table, PrimaryKey: "id", KeyType: "int", Incrementing: false, Fillable: map[string]struct{}{"id": {}, "name": {}, "active": {}}, Guarded: map[string]struct{}{}, Hidden: map[string]struct{}{}, Visible: map[string]struct{}{}, Casts: map[string]string{"active": "bool"}}
			created := &Instance{Class: modelClass, Fields: map[string]interface{}{}, Constants: map[string]bool{}, model: newModelState(metadata, false, false)}
			r.fillModel(created, map[string]interface{}{"id": int64(3), "name": "Model", "active": true}, false)
			if !r.saveModel(created) {
				t.Fatal("model insert failed")
			}
			found := r.executeFindMethod(r.newModelQuery(metadata), []interface{}{int64(3)}).(*Instance)
			if found.Fields["active"] != true || !found.model.exists {
				t.Fatalf("hydrated model=%#v", found.Fields)
			}
			found.Fields["name"] = "Model Updated"
			if !r.saveModel(found) || len(modelChanges(found)) != 0 {
				t.Fatal("model dirty update failed")
			}
			query = builder()
			r.executeGranDBMethod(query, "where", []interface{}{"id", int64(2)})
			if updated := r.executeGranDBMethod(query, "update", []interface{}{map[string]interface{}{"active": false}}); updated != true {
				t.Fatalf("update returned %v", updated)
			}
			query = builder()
			r.executeGranDBMethod(query, "where", []interface{}{"id", int64(2)})
			if deleted := r.executeGranDBMethod(query, "delete", nil); deleted != true {
				t.Fatalf("delete returned %v", deleted)
			}
			if count := r.executeGranDBMethod(builder(), "count", nil); toInt64(count) != 2 {
				t.Fatalf("final count = %v, want 2", count)
			}
		})
	}
}

func TestPureJossGranDBBuildsPortableDialectSQL(t *testing.T) {
	cases := []struct {
		name       string
		driver     string
		pagination string
	}{
		{name: "sqlite", driver: "sqlite", pagination: "LIMIT 10 OFFSET 5"},
		{name: "mysql", driver: "mysql", pagination: "LIMIT 10 OFFSET 5"},
		{name: "postgres", driver: "postgres", pagination: "LIMIT 10 OFFSET 5"},
		{name: "sqlserver", driver: "sqlserver", pagination: "OFFSET 5 ROWS FETCH NEXT 10 ROWS ONLY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := NewRuntime()
			defer runtime.Free()
			runtime.Env = map[string]string{"DB": tc.driver, "PREFIX": ""}
			var output strings.Builder
			runtime.Out = &output
			parsed := parser.NewParser(parser.NewLexer(`echo GranDB::table("users")->select("id")->where("email", "ada@example.test")->orderBy("id")->limit(10)->offset(5)->toSql();`))
			program := parsed.ParseProgram()
			if issues := parsed.Diagnostics(); len(issues) > 0 {
				t.Fatalf("parse GranDB SQL contract: %v", issues)
			}
			units := append([]analyzer.SourceUnit{{Path: "grandb_dialect_sql.joss", Program: program}}, std.GetStdlibUnits()...)
			report := AnalyzeSourceUnits(units)
			if report.Prepared == nil || report.Prepared.HasErrors() {
				t.Fatalf("analyze GranDB SQL contract: %v", report.Diagnostics)
			}
			runtime.ExecutePrepared(report.Prepared)
			sql := strings.TrimSpace(output.String())
			if !strings.Contains(sql, "SELECT id FROM users WHERE email = ?") {
				t.Fatalf("Joss GranDB SQL for %s = %q; expected a bound parameter", tc.driver, sql)
			}
			if !strings.HasSuffix(sql, tc.pagination) {
				t.Fatalf("Joss GranDB pagination for %s = %q; want suffix %q", tc.driver, sql, tc.pagination)
			}
		})
	}
}

func TestPureJossGranDBRejectsUnsafeSQLStructure(t *testing.T) {
	validSQL := `var $query = GranDB::table("users")->select(["id", "email"])->where("email", "ada@example.test")->whereIn("id", [1, 2])->orderBy("email")->toSql(); echo $query;`
	output, failure := executePureJossDatabaseSource(t, validSQL)
	if failure != nil {
		t.Fatalf("valid GranDB query failed: %v", failure)
	}
	if got, want := strings.TrimSpace(output), "SELECT id, email FROM users WHERE email = ? AND id IN (?, ?) ORDER BY email ASC"; got != want {
		t.Fatalf("safe query SQL = %q, want %q", got, want)
	}

	invalidQueries := map[string]string{
		"table":                    `GranDB::table("users; DROP TABLE users")->toSql();`,
		"select":                   `GranDB::table("users")->select("id, (SELECT password FROM users)")->toSql();`,
		"where_column":             `GranDB::table("users")->where("email OR 1=1 --", "x")->toSql();`,
		"where_operator":           `GranDB::table("users")->where("email", "= ? OR 1=1 --", "x")->toSql();`,
		"join_table":               `GranDB::table("users")->join("roles; DROP", "users.id", "=", "roles.user_id")->toSql();`,
		"order_column":             `GranDB::table("users")->orderBy("id; DROP")->toSql();`,
		"updated_without_filter":   `GranDB::table("users")->update({"name": "Grace"});`,
		"deleted_without_filter":   `GranDB::table("users")->delete();`,
		"increment_without_filter": `GranDB::table("users")->increment("counter", "1; DROP TABLE users");`,
		"decrement_without_filter": `GranDB::table("users")->decrement("counter", "1; DROP TABLE users");`,
	}
	for name, source := range invalidQueries {
		t.Run(name, func(t *testing.T) {
			if _, failure := executePureJossDatabaseSource(t, source); failure == nil {
				t.Fatal("unsafe SQL structure was accepted")
			}
		})
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, counter INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO users (id, counter) VALUES (1, 0)"); err != nil {
		t.Fatal(err)
	}
	output, failure = executePureJossDatabaseSource(t, `GranDB::table("users")->where("id", 1)->increment("counter", "1; DROP TABLE users"); echo GranDB::table("users")->count();`, db)
	if failure != nil {
		t.Fatalf("SQL-looking increment binding failed: %v", failure)
	}
	if got := strings.TrimSpace(output); got != "1" {
		t.Fatalf("SQL-looking increment binding output = %q, want intact table count 1", got)
	}
}

func executePureJossDatabaseSource(t *testing.T, source string, databases ...*sql.DB) (output string, failure interface{}) {
	t.Helper()
	parsed := parser.NewParser(parser.NewLexer(source))
	program := parsed.ParseProgram()
	if issues := parsed.Diagnostics(); len(issues) > 0 {
		t.Fatalf("parse Pure Joss GranDB source: %v", issues)
	}
	units := append([]analyzer.SourceUnit{{Path: "grandb_security_test.joss", Program: program}}, std.GetStdlibUnits()...)
	report := AnalyzeSourceUnits(units)
	if report.Prepared == nil || report.Prepared.HasErrors() {
		t.Fatalf("analyze Pure Joss GranDB source: %v", report.Diagnostics)
	}
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.Env = map[string]string{"DB": "mysql", "PREFIX": ""}
	if len(databases) > 0 {
		runtime.DB = databases[0]
		runtime.Env["DB"] = "sqlite"
	}
	var stdout strings.Builder
	runtime.Out = &stdout
	defer func() {
		failure = recover()
		output = stdout.String()
	}()
	runtime.ExecutePrepared(report.Prepared)
	return stdout.String(), nil
}
