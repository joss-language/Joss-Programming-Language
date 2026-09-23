package core

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDatabaseExecutorUsesExecutionContext(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "cancel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := NewRuntime()
	defer r.Free()
	r.DB = db
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.SetExecutionContext(ctx)
	if _, err := r.databaseExecutor().Exec("CREATE TABLE blocked (id INTEGER)"); err == nil {
		t.Fatal("expected cancelled execution context to stop SQL operation")
	}
}
