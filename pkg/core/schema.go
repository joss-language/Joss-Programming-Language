package core

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
)

type schemaCommand map[string]interface{}

type schemaColumn struct {
	name       string
	definition string
}

var safeSchemaIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func quoteSchemaIdentifier(name, driver string) (string, error) {
	if !safeSchemaIdentifier.MatchString(name) {
		return "", fmt.Errorf("identificador SQL no valido: %q", name)
	}
	if driver == "mysql" {
		return "`" + name + "`", nil
	}
	if driver == "sqlserver" {
		return "[" + name + "]", nil
	}
	return `"` + name + `"`, nil
}

func schemaStringList(value interface{}) []string {
	result := []string{}
	switch values := value.(type) {
	case string:
		return []string{values}
	case []interface{}:
		for _, item := range values {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
	case []string:
		return append(result, values...)
	}
	return result
}

func quoteSchemaList(values []string, driver string) ([]string, error) {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		item, err := quoteSchemaIdentifier(value, driver)
		if err != nil {
			return nil, err
		}
		quoted = append(quoted, item)
	}
	return quoted, nil
}

func schemaIndexName(table string, columns []string, suffix string) string {
	name := table + "_" + strings.Join(columns, "_") + "_" + suffix
	if len(name) > 60 {
		name = name[:60]
	}
	return name
}

// ensureInternalSchemaTable is the host-side counterpart of Schema::create.
// Runtime infrastructure uses this function instead of feeding legacy maps to
// the public Joss API, whose only accepted shape is a blueprint callback.
func (r *Runtime) ensureInternalSchemaTable(table string, columns []schemaColumn) error {
	driver := normalizeDatabaseDriver(r.Env["DB"])
	if driver == "" {
		driver = "mysql"
	}
	if prefix := r.dbPrefix(); !strings.HasPrefix(table, prefix) {
		table = prefix + table
	}
	quotedTable, err := quoteSchemaIdentifier(table, driver)
	if err != nil {
		return err
	}
	definitions := make([]string, 0, len(columns))
	for _, column := range columns {
		quotedColumn, quoteErr := quoteSchemaIdentifier(column.name, driver)
		if quoteErr != nil {
			return quoteErr
		}
		definitions = append(definitions, r.buildColumnDefinition(quotedColumn, column.definition, driver))
	}
	_, err = r.databaseExecutor().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", quotedTable, strings.Join(definitions, ", ")))
	return err
}

func (r *Runtime) addInternalSchemaColumn(table string, column schemaColumn) error {
	driver := normalizeDatabaseDriver(r.Env["DB"])
	if driver == "" {
		driver = "mysql"
	}
	if prefix := r.dbPrefix(); !strings.HasPrefix(table, prefix) {
		table = prefix + table
	}
	quotedTable, err := quoteSchemaIdentifier(table, driver)
	if err != nil {
		return err
	}
	quotedColumn, err := quoteSchemaIdentifier(column.name, driver)
	if err != nil {
		return err
	}
	definition := r.buildColumnDefinition(quotedColumn, column.definition, driver)
	_, err = r.databaseExecutor().Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", quotedTable, definition))
	return err
}

// Schema Implementation
func (r *Runtime) executeSchemaMethod(instance *Instance, method string, args []interface{}) interface{} {
	if r.GetDB() == nil {
		return nil
	}

	dbDriver := "mysql"
	if val, ok := r.Env["DB"]; ok {
		dbDriver = normalizeDatabaseDriver(val)
	}

	switch method {
	case "create":
		if len(args) >= 2 {
			tableName, ok := args[0].(string)
			if !ok {
				return nil
			}

			prefix := r.dbPrefix()

			if !strings.HasPrefix(tableName, prefix) {
				tableName = prefix + tableName
			}

			quotedTable, err := quoteSchemaIdentifier(tableName, dbDriver)
			if err != nil {
				fmt.Printf("[Schema] %v\n", err)
				return false
			}
			var definitions []string
			var commands []schemaCommand

			fnLit, ok := args[1].(*parser.FunctionLiteral)
			if !ok {
				fmt.Println("[Schema] Error: El segundo argumento debe ser una función de blueprint.")
				return nil
			}
			blueprint := r.runBlueprint(fnLit)
			if blueprint == nil {
				return false
			}

			if cols, ok := blueprint.Fields["_columns"].([]map[string]string); ok {
				for _, col := range cols {
					columnName, quoteErr := quoteSchemaIdentifier(col["name"], dbDriver)
					if quoteErr != nil {
						fmt.Printf("[Schema] %v\n", quoteErr)
						return false
					}
					def := r.buildColumnDefinition(columnName, col["type"], dbDriver)
					definitions = append(definitions, def)
				}
			}
			commands, _ = blueprint.Fields["_commands"].([]schemaCommand)

			for _, command := range commands {
				if command["type"] != "foreign" {
					continue
				}
				constraint, constraintErr := buildForeignConstraint(command, tableName, dbDriver)
				if constraintErr != nil {
					fmt.Printf("[Schema] %v\n", constraintErr)
					return false
				}
				definitions = append(definitions, constraint)
			}

			query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", quotedTable, strings.Join(definitions, ", "))

			fmt.Printf("[Schema] Ejecutando: %s\n", query)
			_, err = r.databaseExecutor().Exec(query)
			if err != nil {
				fmt.Printf("[Schema] Error creando tabla %s: %v\n", tableName, err)
				return false
			}
			for _, command := range commands {
				if command["type"] == "index" || command["type"] == "uniqueIndex" {
					if err := r.executeSchemaCommand(quotedTable, tableName, dbDriver, command); err != nil {
						fmt.Printf("[Schema] Error creando indice: %v\n", err)
						return false
					}
				}
			}
			return true
		}

	case "table":
		if len(args) >= 2 {
			tableName, ok := args[0].(string)
			if !ok {
				return false
			}
			prefix := r.dbPrefix()
			if !strings.HasPrefix(tableName, prefix) {
				tableName = prefix + tableName
			}

			quotedTable, quoteErr := quoteSchemaIdentifier(tableName, dbDriver)
			if quoteErr != nil {
				fmt.Printf("[Schema] %v\n", quoteErr)
				return false
			}

			if fnLit, ok := args[1].(*parser.FunctionLiteral); ok {
				blueprint := r.runBlueprint(fnLit)
				if blueprint == nil {
					return false
				}

				if cols, ok := blueprint.Fields["_columns"].([]map[string]string); ok {
					for _, col := range cols {
						columnName, err := quoteSchemaIdentifier(col["name"], dbDriver)
						if err != nil {
							fmt.Printf("[Schema] %v\n", err)
							return false
						}
						def := r.buildColumnDefinition(columnName, col["type"], dbDriver)
						query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", quotedTable, def)
						fmt.Printf("[Schema] Ejecutando: %s\n", query)
						if _, err := r.databaseExecutor().Exec(query); err != nil {
							fmt.Printf("[Schema] Error: %v\n", err)
							return false
						}
					}
				}

				commands, _ := blueprint.Fields["_commands"].([]schemaCommand)
				for _, command := range commands {
					if err := r.executeSchemaCommand(quotedTable, tableName, dbDriver, command); err != nil {
						fmt.Printf("[Schema] Error: %v\n", err)
						return false
					}
				}
				return true
			}
		}

	case "rename":
		if len(args) >= 2 {
			from := args[0].(string)
			to := args[1].(string)
			prefix := r.dbPrefix()
			if !strings.HasPrefix(from, prefix) {
				from = prefix + from
			}
			if !strings.HasPrefix(to, prefix) {
				to = prefix + to
			}

			quotedFrom, err := quoteSchemaIdentifier(from, dbDriver)
			if err != nil {
				return false
			}
			quotedTo, err := quoteSchemaIdentifier(to, dbDriver)
			if err != nil {
				return false
			}
			query := fmt.Sprintf("ALTER TABLE %s RENAME TO %s", quotedFrom, quotedTo)
			if dbDriver == "mysql" {
				query = fmt.Sprintf("RENAME TABLE %s TO %s", quotedFrom, quotedTo)
			}
			_, err = r.databaseExecutor().Exec(query)
			return err == nil
		}

	case "drop", "dropIfExists":
		if len(args) >= 1 {
			tableName := args[0].(string)
			prefix := r.dbPrefix()
			if !strings.HasPrefix(tableName, prefix) {
				tableName = prefix + tableName
			}
			quotedTable, err := quoteSchemaIdentifier(tableName, dbDriver)
			if err != nil {
				return false
			}
			query := fmt.Sprintf("DROP TABLE IF EXISTS %s", quotedTable)
			_, err = r.databaseExecutor().Exec(query)
			return err == nil
		}

	case "hasTable":
		if len(args) >= 1 {
			tableName := args[0].(string)
			prefix := r.dbPrefix()
			if !strings.HasPrefix(tableName, prefix) {
				tableName = prefix + tableName
			}

			var exists bool
			if dbDriver == "sqlite" {
				query := "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?"
				r.databaseExecutor().QueryRow(query, tableName).Scan(&exists)
			} else if dbDriver == "postgres" {
				query := "SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?"
				r.databaseExecutor().QueryRow(query, tableName).Scan(&exists)
			} else if dbDriver == "sqlserver" {
				query := "SELECT count(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = ?"
				r.databaseExecutor().QueryRow(query, tableName).Scan(&exists)
			} else {
				query := "SELECT count(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?"
				r.databaseExecutor().QueryRow(query, tableName).Scan(&exists)
			}
			return exists
		}

	case "hasColumn":
		if len(args) >= 2 {
			tableName := args[0].(string)
			columnName := args[1].(string)
			prefix := r.dbPrefix()
			if !strings.HasPrefix(tableName, prefix) {
				tableName = prefix + tableName
			}

			if dbDriver == "sqlite" {
				rows, err := r.databaseExecutor().Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var cid int
						var name string
						var typeStr string
						var notnull int
						var dfltValue interface{}
						var pk int
						rows.Scan(&cid, &name, &typeStr, &notnull, &dfltValue, &pk)
						if name == columnName {
							return true
						}
					}
				}
				return false
			} else if dbDriver == "postgres" {
				var count int
				query := "SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?"
				r.databaseExecutor().QueryRow(query, tableName, columnName).Scan(&count)
				return count > 0
			} else if dbDriver == "sqlserver" {
				var count int
				query := "SELECT count(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? AND COLUMN_NAME = ?"
				r.databaseExecutor().QueryRow(query, tableName, columnName).Scan(&count)
				return count > 0
			} else {
				var count int
				query := "SELECT count(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?"
				r.databaseExecutor().QueryRow(query, tableName, columnName).Scan(&count)
				return count > 0
			}
		}
	}
	return nil
}

func buildForeignConstraint(command schemaCommand, tableName, driver string) (string, error) {
	columns := schemaStringList(command["columns"])
	references := schemaStringList(command["references"])
	foreignTable, _ := command["table"].(string)
	if len(columns) == 0 || len(references) == 0 || foreignTable == "" || len(columns) != len(references) {
		return "", fmt.Errorf("foreign requiere columnas, references y on compatibles")
	}
	quotedColumns, err := quoteSchemaList(columns, driver)
	if err != nil {
		return "", err
	}
	quotedReferences, err := quoteSchemaList(references, driver)
	if err != nil {
		return "", err
	}
	quotedForeignTable, err := quoteSchemaIdentifier(foreignTable, driver)
	if err != nil {
		return "", err
	}
	name, _ := command["name"].(string)
	if name == "" {
		name = schemaIndexName(tableName, columns, "foreign")
	}
	quotedName, err := quoteSchemaIdentifier(name, driver)
	if err != nil {
		return "", err
	}
	constraint := fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)", quotedName, strings.Join(quotedColumns, ", "), quotedForeignTable, strings.Join(quotedReferences, ", "))
	for _, rule := range []struct {
		key string
		sql string
	}{{"onDelete", "ON DELETE"}, {"onUpdate", "ON UPDATE"}} {
		if value, ok := command[rule.key].(string); ok && value != "" {
			normalized := strings.ToUpper(strings.ReplaceAll(value, "_", " "))
			switch normalized {
			case "CASCADE", "RESTRICT", "SET NULL", "NO ACTION", "SET DEFAULT":
				constraint += " " + rule.sql + " " + normalized
			default:
				return "", fmt.Errorf("accion referencial no valida: %s", value)
			}
		}
	}
	return constraint, nil
}

func (r *Runtime) executeSchemaCommand(quotedTable, tableName, driver string, command schemaCommand) error {
	typeName, _ := command["type"].(string)
	switch typeName {
	case "dropColumn":
		for _, column := range schemaStringList(command["columns"]) {
			quoted, err := quoteSchemaIdentifier(column, driver)
			if err != nil {
				return err
			}
			if _, err := r.databaseExecutor().Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", quotedTable, quoted)); err != nil {
				return err
			}
		}
	case "renameColumn":
		from, _ := command["from"].(string)
		to, _ := command["to"].(string)
		quotedFrom, err := quoteSchemaIdentifier(from, driver)
		if err != nil {
			return err
		}
		quotedTo, err := quoteSchemaIdentifier(to, driver)
		if err != nil {
			return err
		}
		_, err = r.databaseExecutor().Exec(fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", quotedTable, quotedFrom, quotedTo))
		return err
	case "index", "uniqueIndex":
		columns := schemaStringList(command["columns"])
		quotedColumns, err := quoteSchemaList(columns, driver)
		if err != nil {
			return err
		}
		name, _ := command["name"].(string)
		if name == "" {
			suffix := "index"
			if typeName == "uniqueIndex" {
				suffix = "unique"
			}
			name = schemaIndexName(tableName, columns, suffix)
		}
		quotedName, err := quoteSchemaIdentifier(name, driver)
		if err != nil {
			return err
		}
		unique := ""
		if typeName == "uniqueIndex" {
			unique = "UNIQUE "
		}
		_, err = r.databaseExecutor().Exec(fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)", unique, quotedName, quotedTable, strings.Join(quotedColumns, ", ")))
		return err
	case "dropIndex":
		name, _ := command["name"].(string)
		quotedName, err := quoteSchemaIdentifier(name, driver)
		if err != nil {
			return err
		}
		if driver == "mysql" {
			_, err = r.databaseExecutor().Exec(fmt.Sprintf("ALTER TABLE %s DROP INDEX %s", quotedTable, quotedName))
		} else {
			_, err = r.databaseExecutor().Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", quotedName))
		}
		return err
	case "foreign":
		if driver == "sqlite" {
			return r.addSQLiteForeign(tableName, command)
		}
		constraint, err := buildForeignConstraint(command, tableName, driver)
		if err != nil {
			return err
		}
		_, err = r.databaseExecutor().Exec(fmt.Sprintf("ALTER TABLE %s ADD %s", quotedTable, constraint))
		return err
	}
	return nil
}
