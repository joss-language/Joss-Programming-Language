package core

import (
	"database/sql"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestSchemaCommandsExecuteOnSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE "x_items" ("id" INTEGER PRIMARY KEY, "old_name" TEXT, "group_id" INTEGER)`); err != nil {
		t.Fatal(err)
	}

	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": "x_"}

	commands := []schemaCommand{
		{"type": "renameColumn", "from": "old_name", "to": "name"},
		{"type": "index", "columns": []string{"group_id", "name"}, "name": "items_group_name_index"},
		{"type": "dropIndex", "name": "items_group_name_index"},
		{"type": "dropColumn", "columns": []string{"group_id"}},
	}
	for _, command := range commands {
		if err := r.executeSchemaCommand(`"x_items"`, "x_items", "sqlite", command); err != nil {
			t.Fatalf("executeSchemaCommand(%v): %v", command, err)
		}
	}

	if got := r.executeSchemaMethod(nil, "hasColumn", []interface{}{"items", "name"}); got != true {
		t.Fatalf("renamed column was not found: %v", got)
	}
	if got := r.executeSchemaMethod(nil, "hasColumn", []interface{}{"items", "group_id"}); got != false {
		t.Fatalf("dropped column still exists: %v", got)
	}
}

func TestSchemaRejectsRemovedMapDefinition(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite"}

	result := r.executeSchemaMethod(nil, "create", []interface{}{"removed_shape", map[string]interface{}{"id": "increments"}})
	if result != nil {
		t.Fatalf("map schema result = %v, want nil", result)
	}
	if exists := r.executeSchemaMethod(nil, "hasTable", []interface{}{"removed_shape"}); exists != false {
		t.Fatalf("removed map schema unexpectedly created table: %v", exists)
	}
}

func TestSchemaTableRejectsRemovedMapDefinition(t *testing.T) {
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

	result := r.executeSchemaMethod(nil, "table", []interface{}{"items", map[string]interface{}{"name": "string"}})
	if result != nil {
		t.Fatalf("map schema result = %v, want nil", result)
	}
	if exists := r.executeSchemaMethod(nil, "hasColumn", []interface{}{"items", "name"}); exists != false {
		t.Fatalf("removed map schema unexpectedly added column: %v", exists)
	}
}

func TestInternalMigrationSchemaUsesHostDefinitions(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": "x_"}

	if err := r.EnsureMigrationTable(); err != nil {
		t.Fatal(err)
	}
	if err := r.LogMigration("001_create_items.joss", 1); err != nil {
		t.Fatal(err)
	}
	if !r.GetExecutedMigrations()["001_create_items.joss"] {
		t.Fatal("logged migration was not read back")
	}
}

func TestBuildCompoundForeignConstraint(t *testing.T) {
	command := schemaCommand{
		"type":       "foreign",
		"columns":    []string{"tenant_id", "owner_id"},
		"references": []string{"tenant_id", "id"},
		"table":      "owners",
		"onDelete":   "cascade",
	}
	got, err := buildForeignConstraint(command, "documents", "postgres")
	if err != nil {
		t.Fatal(err)
	}
	want := `CONSTRAINT "documents_tenant_id_owner_id_foreign" FOREIGN KEY ("tenant_id", "owner_id") REFERENCES "owners" ("tenant_id", "id") ON DELETE CASCADE`
	if got != want {
		t.Fatalf("constraint = %q, want %q", got, want)
	}
}

func TestSQLiteCanAddForeignKeyByRebuildingTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE "x_parents" ("id" INTEGER PRIMARY KEY); CREATE TABLE "x_children" ("id" INTEGER PRIMARY KEY, "parent_id" INTEGER);`); err != nil {
		t.Fatal(err)
	}
	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": "x_"}
	command := schemaCommand{"type": "foreign", "columns": []string{"parent_id"}, "references": []string{"id"}, "table": "x_parents", "onDelete": "cascade"}
	if err := r.executeSchemaCommand(`"x_children"`, "x_children", "sqlite", command); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`PRAGMA foreign_key_list("x_children")`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("foreign key was not added")
	}
}

func TestSchemaCreateWithCapturedFunction(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := NewRuntime()
	r.DB = db
	r.Env = map[string]string{"DB": "sqlite", "PREFIX": "app_"}

	fn := &parser.FunctionLiteral{
		Parameters: []*parser.Parameter{{Name: &parser.Identifier{Value: "$table"}, Type: parser.Token{Literal: "mixed"}}},
		Body: &parser.BlockStatement{
			Statements: []parser.Statement{
				&parser.ExpressionStatement{
					Expression: &parser.CallExpression{
						Function: &parser.MemberExpression{
							Left:     &parser.Identifier{Value: "$table"},
							Property: &parser.Identifier{Value: "id"},
							Token:    parser.Token{Type: parser.ARROW, Literal: "->"},
						},
						Arguments: []parser.Expression{},
					},
				},
				&parser.ExpressionStatement{
					Expression: &parser.CallExpression{
						Function: &parser.MemberExpression{
							Left:     &parser.Identifier{Value: "$table"},
							Property: &parser.Identifier{Value: "string"},
							Token:    parser.Token{Type: parser.ARROW, Literal: "->"},
						},
						Arguments: []parser.Expression{
							&parser.StringLiteral{Value: "name"},
						},
					},
				},
			},
		},
	}
	captured := r.captureFunction(fn)
	result := r.executeSchemaMethod(nil, "create", []interface{}{"products", captured})
	if result != true {
		t.Fatalf("Schema::create returned %v, want true", result)
	}
	if exists := r.executeSchemaMethod(nil, "hasTable", []interface{}{"products"}); exists != true {
		t.Fatalf("Schema::hasTable('products') = false, want true")
	}
	if exists := r.executeSchemaMethod(nil, "hasColumn", []interface{}{"products", "name"}); exists != true {
		t.Fatalf("Schema::hasColumn('products', 'name') = false, want true")
	}
}
