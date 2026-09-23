package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// databaseDialect owns SQL differences that cannot be handled by placeholder
// rebinding alone. It intentionally remains small while GranDB migrates from
// string state to structured query specifications.
type databaseDialect interface {
	randomExpression() string
	columnProbe(table string) string
	parameterLimit() int
	datePartExpression(part, column string) string
	datePartBinding(part string, value interface{}) interface{}
	jsonContainsPredicate(column string, value interface{}) (string, interface{}, error)
	explainQuery(query string) (string, error)
	compileUpsert(table string, rows []map[string]interface{}, uniqueBy, updateColumns []string) (string, []interface{}, error)
}

func (d namedDatabaseDialect) explainQuery(query string) (string, error) {
	switch string(d) {
	case "sqlite":
		return "EXPLAIN QUERY PLAN " + query, nil
	case "mysql", "postgres":
		return "EXPLAIN " + query, nil
	case "sqlserver":
		return "", fmt.Errorf("explain aún no está disponible para SQL Server; SHOWPLAN requiere un batch dedicado")
	default:
		return "", fmt.Errorf("dialecto no soportado")
	}
}

func (d namedDatabaseDialect) jsonContainsPredicate(column string, value interface{}) (string, interface{}, error) {
	switch value.(type) {
	case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
	default:
		return "", nil, fmt.Errorf("whereJsonContains admite valores escalares; serialice estructuras explícitamente")
	}
	switch string(d) {
	case "sqlite":
		return fmt.Sprintf("EXISTS (SELECT 1 FROM json_each(%s) WHERE json_each.value = ?)", column), value, nil
	case "postgres":
		encoded, err := json.Marshal([]interface{}{value})
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s::jsonb @> ?::jsonb", column), string(encoded), nil
	case "sqlserver":
		return fmt.Sprintf("EXISTS (SELECT 1 FROM OPENJSON(%s) WHERE value = ?)", column), value, nil
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("JSON_CONTAINS(%s, ?)", column), string(encoded), nil
	}
}

func (d namedDatabaseDialect) datePartBinding(part string, value interface{}) interface{} {
	if string(d) != "sqlite" {
		return value
	}
	switch strings.ToLower(part) {
	case "year":
		return fmt.Sprintf("%04d", toInt(value))
	case "month", "day":
		return fmt.Sprintf("%02d", toInt(value))
	default:
		return value
	}
}

func (d namedDatabaseDialect) datePartExpression(part, column string) string {
	part = strings.ToLower(part)
	switch string(d) {
	case "sqlite":
		formats := map[string]string{"date": "%Y-%m-%d", "year": "%Y", "month": "%m", "day": "%d", "time": "%H:%M:%S"}
		return fmt.Sprintf("strftime('%s', %s)", formats[part], column)
	case "postgres":
		if part == "date" || part == "time" {
			return fmt.Sprintf("CAST(%s AS %s)", column, strings.ToUpper(part))
		}
		return fmt.Sprintf("EXTRACT(%s FROM %s)", strings.ToUpper(part), column)
	case "sqlserver":
		if part == "date" || part == "time" {
			return fmt.Sprintf("CAST(%s AS %s)", column, part)
		}
		return fmt.Sprintf("DATEPART(%s, %s)", part, column)
	default:
		return fmt.Sprintf("%s(%s)", strings.ToUpper(part), column)
	}
}

type namedDatabaseDialect string

func dialectFor(driver string) databaseDialect {
	return namedDatabaseDialect(normalizeDatabaseDriver(driver))
}

func (d namedDatabaseDialect) randomExpression() string {
	switch string(d) {
	case "sqlite", "postgres":
		return "RANDOM()"
	case "sqlserver":
		return "NEWID()"
	default:
		return "RAND()"
	}
}

func (d namedDatabaseDialect) columnProbe(table string) string {
	if string(d) == "sqlserver" {
		return fmt.Sprintf("SELECT TOP 0 * FROM %s", table)
	}
	return fmt.Sprintf("SELECT * FROM %s LIMIT 0", table)
}

func (d namedDatabaseDialect) parameterLimit() int {
	switch string(d) {
	case "sqlite":
		return 999
	case "sqlserver":
		return 2100
	default:
		return 65535
	}
}

func compileInsertRows(table string, rows []map[string]interface{}) (string, []interface{}, error) {
	if len(rows) == 0 {
		return "", nil, fmt.Errorf("insertMany requiere al menos una fila")
	}
	columns := sortedMapKeys(rows[0])
	if len(columns) == 0 {
		return "", nil, fmt.Errorf("insertMany no acepta filas vacías")
	}
	expected := strings.Join(columns, "\x00")
	bindings := make([]interface{}, 0, len(rows)*len(columns))
	groups := make([]string, 0, len(rows))
	for index, row := range rows {
		if strings.Join(sortedMapKeys(row), "\x00") != expected {
			return "", nil, fmt.Errorf("insertMany: la fila %d no tiene las mismas columnas", index+1)
		}
		groups = append(groups, "("+placeholders(len(columns))+")")
		for _, column := range columns {
			value := row[column]
			if _, unsupported := value.(map[string]interface{}); unsupported {
				return "", nil, fmt.Errorf("insertMany: la columna %q contiene un map; serialícelo como JSON explícitamente", column)
			}
			bindings = append(bindings, value)
		}
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", table, strings.Join(quoteIdentifiers(columns), ", "), strings.Join(groups, ", ")), bindings, nil
}

func (d namedDatabaseDialect) compileUpsert(table string, rows []map[string]interface{}, uniqueBy, updateColumns []string) (string, []interface{}, error) {
	if len(rows) == 0 {
		return "", nil, fmt.Errorf("upsert requiere al menos una fila")
	}
	if len(uniqueBy) == 0 {
		return "", nil, fmt.Errorf("upsert requiere una clave única")
	}

	columns := sortedMapKeys(rows[0])
	if len(columns) == 0 {
		return "", nil, fmt.Errorf("upsert no acepta filas vacías")
	}
	expected := strings.Join(columns, "\x00")
	bindings := make([]interface{}, 0, len(rows)*len(columns))
	valueGroups := make([]string, 0, len(rows))
	for index, row := range rows {
		if strings.Join(sortedMapKeys(row), "\x00") != expected {
			return "", nil, fmt.Errorf("upsert: la fila %d no tiene las mismas columnas", index+1)
		}
		valueGroups = append(valueGroups, "("+placeholders(len(columns))+")")
		for _, column := range columns {
			value := row[column]
			if _, unsupported := value.(map[string]interface{}); unsupported {
				return "", nil, fmt.Errorf("upsert: la columna %q contiene un map; serialícelo como JSON explícitamente", column)
			}
			bindings = append(bindings, value)
		}
	}

	quotedColumns := quoteIdentifiers(columns)
	quotedUnique := quoteIdentifiers(uniqueBy)
	for _, column := range uniqueBy {
		if !containsString(columns, column) {
			return "", nil, fmt.Errorf("upsert: la clave única %q no está presente", column)
		}
	}
	if len(updateColumns) == 0 {
		for _, column := range columns {
			if !containsString(uniqueBy, column) {
				updateColumns = append(updateColumns, column)
			}
		}
	}
	for _, column := range updateColumns {
		if !containsString(columns, column) {
			return "", nil, fmt.Errorf("upsert: la columna de actualización %q no está presente", column)
		}
	}

	base := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", table, strings.Join(quotedColumns, ", "), strings.Join(valueGroups, ", "))
	switch string(d) {
	case "sqlite", "postgres":
		if len(updateColumns) == 0 {
			return base + fmt.Sprintf(" ON CONFLICT (%s) DO NOTHING", strings.Join(quotedUnique, ", ")), bindings, nil
		}
		sets := make([]string, 0, len(updateColumns))
		for _, column := range updateColumns {
			quoted := quoteIdentifier(column)
			sets = append(sets, fmt.Sprintf("%s = excluded.%s", quoted, quoted))
		}
		return base + fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s", strings.Join(quotedUnique, ", "), strings.Join(sets, ", ")), bindings, nil
	case "sqlserver":
		conditions := make([]string, 0, len(uniqueBy))
		for _, column := range uniqueBy {
			quoted := quoteIdentifier(column)
			conditions = append(conditions, fmt.Sprintf("target.%s = source.%s", quoted, quoted))
		}
		parts := []string{fmt.Sprintf("MERGE INTO %s AS target USING (VALUES %s) AS source (%s) ON %s", table, strings.Join(valueGroups, ", "), strings.Join(quotedColumns, ", "), strings.Join(conditions, " AND "))}
		if len(updateColumns) > 0 {
			sets := make([]string, 0, len(updateColumns))
			for _, column := range updateColumns {
				quoted := quoteIdentifier(column)
				sets = append(sets, fmt.Sprintf("target.%s = source.%s", quoted, quoted))
			}
			parts = append(parts, "WHEN MATCHED THEN UPDATE SET "+strings.Join(sets, ", "))
		}
		sourceValues := make([]string, 0, len(columns))
		for _, column := range columns {
			sourceValues = append(sourceValues, "source."+quoteIdentifier(column))
		}
		parts = append(parts, fmt.Sprintf("WHEN NOT MATCHED THEN INSERT (%s) VALUES (%s);", strings.Join(quotedColumns, ", "), strings.Join(sourceValues, ", ")))
		return strings.Join(parts, " "), bindings, nil
	default:
		if len(updateColumns) == 0 {
			first := quoteIdentifier(uniqueBy[0])
			return base + fmt.Sprintf(" ON DUPLICATE KEY UPDATE %s = %s", first, first), bindings, nil
		}
		sets := make([]string, 0, len(updateColumns))
		for _, column := range updateColumns {
			quoted := quoteIdentifier(column)
			sets = append(sets, fmt.Sprintf("%s = VALUES(%s)", quoted, quoted))
		}
		return base + " ON DUPLICATE KEY UPDATE " + strings.Join(sets, ", "), bindings, nil
	}
}

func sortedMapKeys(row map[string]interface{}) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func quoteIdentifiers(columns []string) []string {
	quoted := make([]string, len(columns))
	for index, column := range columns {
		quoted[index] = quoteIdentifier(column)
	}
	return quoted
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
