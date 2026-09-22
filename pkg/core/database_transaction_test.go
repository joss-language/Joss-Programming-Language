package core

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestGranDBTransactionUsesCallbackQueriesAndRollsBack(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "transaction.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE items (name TEXT)"); err != nil {
		t.Fatal(err)
	}

	r := newRuntimeState().(*Runtime)
	defer r.Free()
	r.DB = db
	r.Env["DB"] = "sqlite"
	r.Env["PREFIX"] = ""
	source := `
public func rollback(mixed $db) {
    $db->table("items")->insert({"name": "rolled back"})
    throw "abort"
}
public func commit(mixed $db) {
    $db->table("items")->insert({"name": "committed"})
}
`
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse: %v", p.Errors())
	}
	r.Execute(program)
	instance := &Instance{Class: &parser.ClassStatement{Name: &parser.Identifier{Value: "GranDB"}}, Fields: make(map[string]interface{})}
	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Error("expected callback failure")
			}
		}()
		r.executeGranDBMethod(instance, "transaction", []interface{}{r.Functions["rollback"]})
	}()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback left %d rows: %v", count, err)
	}
	r.executeGranDBMethod(instance, "transaction", []interface{}{r.Functions["commit"]})
	if err := db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count); err != nil || count != 1 {
		t.Fatalf("commit left %d rows: %v", count, err)
	}
	if r.activeTx != nil {
		t.Fatal("transaction state survived callback")
	}
	outer, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	r.activeTx = outer
	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Error("expected nested transaction rejection")
			}
		}()
		r.executeGranDBMethod(instance, "transaction", []interface{}{r.Functions["commit"]})
	}()
	r.activeTx = nil
	_ = outer.Rollback()
}
