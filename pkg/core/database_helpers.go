package core

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

type sqlQueryExecutor interface {
	Exec(string, ...interface{}) (sql.Result, error)
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) *sql.Row
}

type contextSQLTarget interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

type contextSQLExecutor struct {
	ctx    context.Context
	target contextSQLTarget
}

func (e contextSQLExecutor) Exec(query string, args ...interface{}) (sql.Result, error) {
	return e.target.ExecContext(e.ctx, query, args...)
}

func (e contextSQLExecutor) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return e.target.QueryContext(e.ctx, query, args...)
}

func (e contextSQLExecutor) QueryRow(query string, args ...interface{}) *sql.Row {
	return e.target.QueryRowContext(e.ctx, query, args...)
}

// databaseExecutor routes GranDB operations through the transaction owned by
// the current callback. Other runtimes keep using their own DB connection.
func (r *Runtime) databaseExecutor() sqlQueryExecutor {
	ctx := r.executionContext
	if ctx == nil {
		ctx = context.Background()
	}
	if r.activeTx != nil {
		return contextSQLExecutor{ctx: ctx, target: r.activeTx}
	}
	if db := r.GetDB(); db != nil {
		return contextSQLExecutor{ctx: ctx, target: db}
	}
	return nil
}

// rowsToMap converts SQL rows to []map[string]interface{}
func rowsToMap(rows *sql.Rows) []map[string]interface{} {
	var results []map[string]interface{}
	cols, _ := rows.Columns()
	vals := make([]interface{}, len(cols))
	valPtrs := make([]interface{}, len(cols))
	for i := range cols {
		valPtrs[i] = &vals[i]
	}

	for rows.Next() {
		rows.Scan(valPtrs...)
		row := make(map[string]interface{})
		for i, colName := range cols {
			valVal := vals[i]
			if b, ok := valVal.([]byte); ok {
				row[colName] = string(b)
			} else {
				row[colName] = valVal
			}
		}
		results = append(results, row)
	}
	return results
}

var sqlIdentifierPart = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

// quoteIdentifier quotes a simple, qualified SQL identifier. SQL expressions
// are deliberately rejected here: callers that need trusted SQL must use an
// explicit expression path instead of smuggling it through an identifier.
func quoteIdentifier(name string) string {
	name = strings.TrimSpace(name)
	if name == "*" {
		return "*"
	}
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		for i, p := range parts {
			parts[i] = quoteIdentifier(p)
		}
		return strings.Join(parts, ".")
	}
	if !sqlIdentifierPart.MatchString(name) {
		panic(fmt.Sprintf("GranDB Error: identificador SQL inválido %q", name))
	}
	return "`" + name + "`"
}

func comparisonOperator(value interface{}) string {
	op := strings.ToUpper(strings.Join(strings.Fields(fmt.Sprint(value)), " "))
	switch op {
	case "=", "!=", "<>", "<", "<=", ">", ">=", "LIKE", "NOT LIKE", "IS", "IS NOT":
		return op
	default:
		panic(fmt.Sprintf("GranDB Error: operador SQL inválido %q", value))
	}
}

func unquoteIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '`' && value[len(value)-1] == '`') ||
			(value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '[' && value[len(value)-1] == ']') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func buildWhereClause(wheres []string) string {
	if len(wheres) == 0 {
		return ""
	}

	parts := []string{}
	for i, where := range wheres {
		trimmed := strings.TrimSpace(where)
		upper := strings.ToUpper(trimmed)

		if i == 0 {
			if strings.HasPrefix(upper, "OR ") {
				trimmed = strings.TrimSpace(where[3:])
			} else if strings.HasPrefix(upper, "AND ") {
				trimmed = strings.TrimSpace(where[4:])
			}
			parts = append(parts, trimmed)
			continue
		}

		if strings.HasPrefix(upper, "OR ") || strings.HasPrefix(upper, "AND ") {
			parts = append(parts, trimmed)
		} else {
			parts = append(parts, "AND "+trimmed)
		}
	}

	return strings.Join(parts, " ")
}

func placeholders(count int) string {
	if count <= 0 {
		return ""
	}
	items := make([]string, count)
	for i := range items {
		items[i] = "?"
	}
	return strings.Join(items, ", ")
}

func toInterfaceSlice(value interface{}) []interface{} {
	if value == nil {
		return []interface{}{}
	}
	if list, ok := value.([]interface{}); ok {
		return list
	}
	return []interface{}{value}
}

func resetReadState(instance *Instance) {
	instance.Fields["_wheres"] = []string{}
	instance.Fields["_bindings"] = []interface{}{}
	instance.Fields["_select"] = "*"
	instance.Fields["_joins"] = []string{}
	delete(instance.Fields, "_distinct")
	delete(instance.Fields, "_groupBy")
	delete(instance.Fields, "_having")
	delete(instance.Fields, "_order")
	delete(instance.Fields, "_limit")
	delete(instance.Fields, "_offset")
}

func (r *Runtime) buildSelectQuery(instance *Instance, sel string) (string, []interface{}) {
	table := r.getTable(instance)
	wheres := instance.Fields["_wheres"].([]string)
	bindings := instance.Fields["_bindings"].([]interface{})

	driver := "mysql"
	if r.Env != nil {
		if val, ok := r.Env["DB"]; ok {
			driver = normalizeDatabaseDriver(val)
		}
	}

	distinctStr := ""
	if dist, ok := instance.Fields["_distinct"].(bool); ok && dist {
		distinctStr = "DISTINCT "
	}

	limitVal, hasLimit := instance.Fields["_limit"].(int)
	offsetVal, hasOffset := instance.Fields["_offset"].(int)

	topClause := ""
	if driver == "sqlserver" && hasLimit && !hasOffset {
		topClause = fmt.Sprintf("TOP %d ", limitVal)
	}

	query := fmt.Sprintf("SELECT %s%s%s FROM %s", topClause, distinctStr, sel, table)

	if joins, ok := instance.Fields["_joins"]; ok {
		for _, j := range joins.([]string) {
			query += " " + j
		}
	}

	if len(wheres) > 0 {
		query += " WHERE " + buildWhereClause(wheres)
	}

	if groupBy, ok := instance.Fields["_groupBy"]; ok {
		query += " GROUP BY " + groupBy.(string)
	}

	if having, ok := instance.Fields["_having"]; ok {
		query += " HAVING " + buildWhereClause(having.([]string))
	}

	order, hasOrder := instance.Fields["_order"].(string)
	if hasOrder {
		query += " ORDER BY " + order
	}

	if driver == "sqlserver" {
		if hasOffset {
			if !hasOrder {
				query += " ORDER BY (SELECT NULL)"
			}
			query += fmt.Sprintf(" OFFSET %d ROWS", offsetVal)
			if hasLimit {
				query += fmt.Sprintf(" FETCH NEXT %d ROWS ONLY", limitVal)
			}
		}
	} else {
		if hasLimit {
			query += fmt.Sprintf(" LIMIT %d", limitVal)
		}
		if hasOffset {
			query += fmt.Sprintf(" OFFSET %d", offsetVal)
		}
	}

	return query, bindings
}

// applyTablePrefix adds prefix to table names
func (r *Runtime) applyTablePrefix(name string) string {
	if r.Env == nil {
		return name
	}
	prefix := r.dbPrefix()
	if prefix == "" {
		return name
	}
	if !strings.HasPrefix(name, prefix) {
		return prefix + name
	}
	return name
}

// applyColumnPrefix adds prefix to table part of column name
func (r *Runtime) applyColumnPrefix(name string) string {
	if r.Env == nil {
		return name
	}
	prefix := r.dbPrefix()
	if prefix == "" {
		return name
	}

	if strings.Contains(name, ".") {
		parts := strings.SplitN(name, ".", 2)
		tablePart := parts[0]
		colPart := parts[1]

		if !strings.HasPrefix(tablePart, prefix) {
			tablePart = prefix + tablePart
		}
		return tablePart + "." + colPart
	}
	return name
}

// getTable determines the table name from the instance
func (r *Runtime) getTable(instance *Instance) string {
	if val, ok := instance.Fields["_table"]; ok {
		if str, ok := val.(string); ok && str != "" {
			return str
		}
	}
	if val, ok := instance.Fields["tabla"]; ok {
		if str, ok := val.(string); ok && str != "" {
			quoted := quoteIdentifier(r.applyTablePrefix(str))
			instance.Fields["_table"] = quoted
			return quoted
		}
	}

	className := instance.Class.Name.Value
	if className == "GranDB" || className == "Model" {
		return ""
	}

	prefix := r.dbPrefix()
	tableName := prefix + strings.ToLower(r.pluralize(className))
	quoted := quoteIdentifier(tableName)
	instance.Fields["_table"] = quoted
	return quoted
}

func (r *Runtime) dbPrefix() string {
	if r == nil || r.Env == nil {
		return "js_"
	}
	if val, ok := r.Env["PREFIX"]; ok {
		return val
	}
	if val, ok := r.Env["DB_PREFIX"]; ok {
		return val
	}
	return "js_"
}

// pluralize helper method on Runtime
func (r *Runtime) pluralize(s string) string {
	return Pluralize(s)
}

// Pluralize converts an English word to its plural form
func Pluralize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	irregular := map[string]string{
		"person": "people",
		"man":    "men",
		"child":  "children",
		"foot":   "feet",
		"tooth":  "teeth",
		"mouse":  "mice",
	}
	if val, ok := irregular[lower]; ok {
		return val
	}
	if strings.HasSuffix(lower, "y") && len(lower) > 1 {
		lastChar := lower[len(lower)-1]
		secondLast := lower[len(lower)-2]
		if lastChar == 'y' && !IsVowel(secondLast) {
			return s[:len(s)-1] + "ies"
		}
	}
	if strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") || strings.HasSuffix(lower, "z") || strings.HasSuffix(lower, "ch") || strings.HasSuffix(lower, "sh") {
		return s + "es"
	}
	return s + "s"
}

// Singularize converts an English plural word back to singular form
func Singularize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	irregular := map[string]string{
		"people":   "person",
		"men":      "man",
		"children": "child",
		"feet":     "foot",
		"teeth":    "tooth",
		"mice":     "mouse",
	}
	if val, ok := irregular[lower]; ok {
		return val
	}
	if strings.HasSuffix(lower, "ies") && len(lower) > 3 {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(lower, "es") && len(lower) > 2 {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(lower, "s") && len(lower) > 1 {
		return s[:len(s)-1]
	}
	return s
}

func IsVowel(b byte) bool {
	switch b {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return true
	default:
		return false
	}
}

func toInt64(val interface{}) int64 {
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	default:
		return int64(toInt(val))
	}
}
