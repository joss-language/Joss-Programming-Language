package core

import (
	"fmt"
	"strings"
)

func (r *Runtime) addSQLiteForeign(tableName string, command schemaCommand) error {
	constraint, err := buildForeignConstraint(command, tableName, "sqlite")
	if err != nil {
		return err
	}
	var createSQL string
	if err := r.databaseExecutor().QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&createSQL); err != nil {
		return err
	}
	openParen := strings.Index(createSQL, "(")
	closeParen := strings.LastIndex(createSQL, ")")
	if openParen < 0 || closeParen <= openParen {
		return fmt.Errorf("no se pudo reconstruir CREATE TABLE de %s", tableName)
	}
	temporary := tableName + "__joss_fk"
	temporarySQL := "CREATE TABLE " + temporary + " " + createSQL[openParen:closeParen] + ", " + constraint + createSQL[closeParen:]

	return r.alterSQLiteTableStructure(tableName, temporarySQL, temporary)
}

func (r *Runtime) alterSQLiteTableStructure(tableName, temporarySQL, temporaryTable string) error {
	if r.activeTx != nil {
		return fmt.Errorf("la reconstrucción de tablas SQLite no está soportada dentro de GranDB.transaction")
	}
	quotedTable, err := quoteSchemaIdentifier(tableName, "sqlite")
	if err != nil {
		return err
	}
	quotedTemporary, err := quoteSchemaIdentifier(temporaryTable, "sqlite")
	if err != nil {
		return err
	}

	rows, err := r.databaseExecutor().Query(fmt.Sprintf("PRAGMA table_info(%s)", quotedTable))
	if err != nil {
		return err
	}
	columns := []string{}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		quoted, err := quoteSchemaIdentifier(name, "sqlite")
		if err != nil {
			_ = rows.Close()
			return err
		}
		columns = append(columns, quoted)
	}
	_ = rows.Close()

	objectRows, err := r.databaseExecutor().Query("SELECT sql FROM sqlite_master WHERE tbl_name=? AND type IN ('index','trigger') AND sql IS NOT NULL", tableName)
	if err != nil {
		return err
	}
	objects := []string{}
	for objectRows.Next() {
		var statement string
		if objectRows.Scan(&statement) == nil {
			objects = append(objects, statement)
		}
	}
	_ = objectRows.Close()

	if _, err := r.databaseExecutor().Exec("PRAGMA foreign_keys=OFF"); err != nil {
		return err
	}
	defer r.databaseExecutor().Exec("PRAGMA foreign_keys=ON")
	tx, err := r.GetDB().Begin()
	if err != nil {
		return err
	}
	rollback := func(cause error) error {
		_ = tx.Rollback()
		return cause
	}
	if _, err := tx.Exec(temporarySQL); err != nil {
		return rollback(err)
	}
	columnList := strings.Join(columns, ", ")
	if _, err := tx.Exec(fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s", quotedTemporary, columnList, columnList, quotedTable)); err != nil {
		return rollback(err)
	}
	if _, err := tx.Exec("DROP TABLE " + quotedTable); err != nil {
		return rollback(err)
	}
	if _, err := tx.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", quotedTemporary, quotedTable)); err != nil {
		return rollback(err)
	}
	for _, statement := range objects {
		if _, err := tx.Exec(statement); err != nil {
			return rollback(err)
		}
	}
	return tx.Commit()
}
