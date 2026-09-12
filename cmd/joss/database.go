package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/i18n"
	_ "modernc.org/sqlite"
)

func changeDatabaseEngine(target string) {
	fmt.Println(i18n.Tr("dbChangingEngine", map[string]interface{}{"target": target}))

	envMap := readEnvFile(GetEnvFile())
	currentDB := envMap["DB"]
	if currentDB == "" {
		currentDB = "mysql"
	}

	if currentDB == target {
		fmt.Println(i18n.Tr("dbAlreadyEngine", map[string]interface{}{"target": target}))
		return
	}

	srcDB, err := connectToDB(currentDB, envMap)
	if err != nil {
		fmt.Println(i18n.Tr("dbErrConnectingSource", map[string]interface{}{"source": currentDB, "err": err}))
		return
	}
	defer srcDB.Close()

	destDB, err := connectToDB(target, envMap)
	if err != nil {
		fmt.Println(i18n.Tr("dbErrConnectingDest", map[string]interface{}{"dest": target, "err": err}))
		return
	}
	defer destDB.Close()

	fmt.Println(i18n.Tr("dbConnectedSourceDest"))
	prepareDestinationSchema(destDB, target, envMap)

	fmt.Println(i18n.Tr("dbStartingDataMigration"))
	if err := migrateTablesToDatabase(srcDB, destDB, currentDB, target, envMap); err != nil {
		fmt.Println(i18n.Tr("dbMigrationWarning", i18n.M{"error": err}))
	}

	updateEnvFile(GetEnvFile(), "DB", target)
	fmt.Println(i18n.Tr("dbMigrationCompleted", map[string]interface{}{"file": GetEnvFile()}))
}

func prepareDestinationSchema(destDB *sql.DB, target string, envMap map[string]string) {
	fmt.Println(i18n.Tr("dbPreparingDestSchema"))
	destRt := core.NewRuntime()
	destRt.DB = destDB
	destRt.Env = make(map[string]string)
	for k, v := range envMap {
		destRt.Env[k] = v
	}
	destRt.Env["DB"] = target

	destRt.EnsureMigrationTable()
	destRt.EnsureAuthTables()
	destRt.EnsureCronTable()
	performMigrations(destRt)
}

func changeDatabaseMigrate() {
	fmt.Println(i18n.Tr("dbMigratingBetweenConfigured"))

	envFile := GetEnvFile()
	envMap := readEnvFile(envFile)

	srcDriver := envMap["DB"]
	if srcDriver == "" {
		srcDriver = "sqlite"
	}

	destDriver := envMap["DEST_DB"]
	if destDriver == "" {
		destDriver = envMap["TARGET_DB"]
	}
	if destDriver == "" {
		if srcDriver == "sqlite" {
			destDriver = "mysql"
		} else {
			destDriver = "sqlite"
		}
	}

	fmt.Printf("%s : %s\n", i18n.Tr("dbOrigin"), srcDriver)
	fmt.Printf("%s: %s\n", i18n.Tr("dbDestination"), destDriver)

	srcDB, err := connectToDB(srcDriver, envMap)
	if err != nil {
		fmt.Println(i18n.Tr("dbErrConnectingSource", map[string]interface{}{"source": srcDriver, "err": err}))
		return
	}
	defer srcDB.Close()

	destEnv := make(map[string]string)
	for k, v := range envMap {
		destEnv[k] = v
	}

	if val := envMap["DEST_DB_HOST"]; val != "" {
		destEnv["DB_HOST"] = val
	}
	if val := envMap["DEST_DB_USER"]; val != "" {
		destEnv["DB_USER"] = val
	}
	if val := envMap["DEST_DB_PASSWORD"]; val != "" {
		destEnv["DB_PASSWORD"] = val
	}
	if val := envMap["DEST_DB_NAME"]; val != "" {
		destEnv["DB_NAME"] = val
	}
	if val := envMap["DEST_DB_PORT"]; val != "" {
		destEnv["DB_PORT"] = val
	}

	destDB, err := connectToDB(destDriver, destEnv)
	if err != nil {
		fmt.Printf("Error conectando a destino (%s): %v\n", destDriver, err)
		return
	}
	defer destDB.Close()

	fmt.Println(i18n.Tr("dbConnectedSourceDest"))
	prepareDestinationSchema(destDB, destDriver, destEnv)

	if err := migrateTablesToDatabase(srcDB, destDB, srcDriver, destDriver, envMap); err != nil {
		fmt.Println(i18n.Tr("dbMigrationError", i18n.M{"error": err}))
		return
	}

	backupEnvFile(envFile)
	updateEnvFile(envFile, "DB", destDriver)

	if val := destEnv["DB_HOST"]; val != "" {
		updateEnvFile(envFile, "DB_HOST", val)
	}
	if val := destEnv["DB_USER"]; val != "" {
		updateEnvFile(envFile, "DB_USER", val)
	}
	if val := destEnv["DB_PASSWORD"]; val != "" {
		updateEnvFile(envFile, "DB_PASSWORD", val)
	}
	if val := destEnv["DB_NAME"]; val != "" {
		updateEnvFile(envFile, "DB_NAME", val)
	}
	if val := destEnv["DB_PORT"]; val != "" {
		updateEnvFile(envFile, "DB_PORT", val)
	}

	fmt.Println(i18n.Tr("dbMigrationCompleted", map[string]interface{}{"file": envFile}))
}

func migrateTablesToDatabase(srcDB, destDB *sql.DB, srcDriver, destDriver string, envMap map[string]string) error {
	fmt.Println(i18n.Tr("dbStartingDataMigration"))

	tables, err := getTables(srcDB, srcDriver)
	if err != nil {
		return fmt.Errorf("error obteniendo tablas: %w", err)
	}

	prefix := "js_"
	if val, ok := envMap["PREFIX"]; ok {
		prefix = val
	} else if val, ok := envMap["DB_PREFIX"]; ok {
		prefix = val
	}

	for _, table := range tables {
		if table == "sqlite_sequence" || table == prefix+"migration" || table == prefix+"cron" {
			continue
		}
		if err := migrateSingleTableData(srcDB, destDB, srcDriver, destDriver, table); err != nil {
			fmt.Println(i18n.Tr("dbMigrateTableError", i18n.M{"table": table, "error": err}))
		}

	}
	return nil
}

func migrateSingleTableData(srcDB, destDB *sql.DB, srcDriver, destDriver, table string) error {
	fmt.Printf("%s ", i18n.Tr("dbMigratingTable", map[string]interface{}{"table": table}))
	rows, err := srcDB.Query(fmt.Sprintf("SELECT * FROM %s", quoteSQLName(srcDriver, table)))
	if err != nil {
		return err
	}

	if err := ensureTableSchema(destDB, destDriver, table, rows); err != nil {
		rows.Close()
		return err
	}

	cols, _ := rows.Columns()
	vals := make([]interface{}, len(cols))
	valPtrs := make([]interface{}, len(cols))
	placeholders := make([]string, len(cols))
	quotedCols := make([]string, len(cols))
	for i, col := range cols {
		valPtrs[i] = &vals[i]
		placeholders[i] = "?"
		quotedCols[i] = quoteSQLName(destDriver, col)
	}

	var insertCmd string
	switch destDriver {
	case "mysql":
		insertCmd = "INSERT IGNORE INTO"
	case "sqlite":
		insertCmd = "INSERT OR IGNORE INTO"
	default:
		insertCmd = "INSERT INTO"
	}
	query := fmt.Sprintf("%s %s (%s) VALUES (%s)", insertCmd, quoteSQLName(destDriver, table), strings.Join(quotedCols, ", "), strings.Join(placeholders, ", "))

	tx, err := destDB.Begin()
	if err != nil {
		rows.Close()
		return err
	}
	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		rows.Close()
		return err
	}
	defer stmt.Close()

	count := 0
	for rows.Next() {
		if err := rows.Scan(valPtrs...); err == nil {
			if _, errExec := stmt.Exec(vals...); errExec == nil {
				count++
			}
		}
	}

	if errScan := rows.Err(); errScan != nil {
		tx.Rollback()
		rows.Close()
		return errScan
	}

	tx.Commit()
	rows.Close()
	fmt.Printf("%s %s\n", i18n.Tr("dbTableDone"), i18n.Tr("dbMigratedRows", map[string]interface{}{"count": count}))
	return nil
}

func quoteSQLName(driver, name string) string {
	switch driver {
	case "mysql":
		return "`" + name + "`"
	case "postgres", "postgresql", "pgx":
		return `"` + name + `"`
	default:
		return name
	}
}

func backupEnvFile(envFile string) {
	data, err := os.ReadFile(envFile)
	if err != nil {
		return
	}
	backupPath := fmt.Sprintf("%s.bak.%d", envFile, time.Now().Unix())
	os.WriteFile(backupPath, data, 0644)
}

func connectToDB(driver string, env map[string]string) (*sql.DB, error) {
	return core.OpenConfiguredDatabase(driver, env)
}

func getTables(db *sql.DB, driver string) ([]string, error) {
	var query string
	switch driver {
	case "sqlite":
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
	case "postgres", "postgresql", "pgx":
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'"
	default:
		query = "SHOW TABLES"
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tables, nil
}

func changeDatabasePrefix(newPrefix string) {
	fmt.Println(i18n.Tr("dbPrefixUpdated", map[string]interface{}{"prefix": newPrefix}))

	envMap := readEnvFile(GetEnvFile())
	currentPrefix := envMap["PREFIX"]
	if currentPrefix == "" {
		currentPrefix = envMap["DB_PREFIX"]
	}
	if currentPrefix == "" {
		currentPrefix = "js_"
	}

	if currentPrefix == newPrefix {
		fmt.Println("El prefijo ya es " + newPrefix)
		return
	}

	dbDriver := envMap["DB"]
	if dbDriver == "" {
		dbDriver = "mysql"
	}

	db, err := connectToDB(dbDriver, envMap)
	if err != nil {
		fmt.Println(i18n.Tr("dbConnectError", i18n.M{"error": err}))
		return
	}
	defer db.Close()

	tables, err := getTables(db, dbDriver)
	if err != nil {
		fmt.Println(i18n.Tr("dbGetTablesError", i18n.M{"error": err}))
		return
	}

	count := 0
	for _, table := range tables {
		if strings.HasPrefix(table, currentPrefix) {
			if err := renameSingleTablePrefix(db, dbDriver, currentPrefix, newPrefix, table); err == nil {
				count++
			}
		}
	}

	fmt.Println(i18n.Tr("dbUpdatingSourceCodePrefix"))
	updateSourceCodePrefix(currentPrefix, newPrefix)

	updateEnvFile(GetEnvFile(), "PREFIX", newPrefix)
	updateEnvFile(GetEnvFile(), "DB_PREFIX", newPrefix)
	fmt.Println(i18n.Tr("dbPrefixTablesRenamed", i18n.M{"count": count}))
}

func renameSingleTablePrefix(db *sql.DB, dbDriver, currentPrefix, newPrefix, table string) error {
	newTableName := strings.Replace(table, currentPrefix, newPrefix, 1)
	fmt.Print(i18n.Tr("dbRenamingTable", i18n.M{"old": table, "new": newTableName}) + " ")

	var query string
	switch dbDriver {
	case "sqlite", "postgres", "postgresql", "pgx":
		query = fmt.Sprintf("ALTER TABLE %s RENAME TO %s", table, newTableName)
	default:
		query = fmt.Sprintf("RENAME TABLE %s TO %s", table, newTableName)
	}

	_, err := db.Exec(query)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}
	fmt.Println("OK")
	return nil
}

func updateSourceCodePrefix(oldPrefix, newPrefix string) {
	dirs := []string{"app/models", "app/database/migrations"}
	for _, dir := range dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".joss") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err == nil {
				strContent := string(content)
				newContent := strings.ReplaceAll(strContent, "\""+oldPrefix, "\""+newPrefix)
				newContent = strings.ReplaceAll(newContent, "'"+oldPrefix, "'"+newPrefix)
				if strContent != newContent {
					os.WriteFile(path, []byte(newContent), 0644)
				}
			}
			return nil
		})
	}
}

func ensureTableSchema(destDB *sql.DB, driver, table string, rows *sql.Rows) error {
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return err
	}

	tableExists := checkTableExists(destDB, driver, table)
	if !tableExists {
		return createMissingTableSchema(destDB, driver, table, colTypes)
	}
	return addMissingColumnsToTable(destDB, driver, table, colTypes)
}

func checkTableExists(destDB *sql.DB, driver, table string) bool {
	var name string
	switch driver {
	case "sqlite":
		err := destDB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		return err == nil
	case "postgres", "postgresql", "pgx":
		err := destDB.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?", table).Scan(&name)
		return err == nil
	default:
		err := destDB.QueryRow("SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", table).Scan(&name)
		return err == nil
	}
}

func createMissingTableSchema(destDB *sql.DB, driver, table string, colTypes []*sql.ColumnType) error {
	fmt.Println(i18n.Tr("dbTableMissingCreating", i18n.M{"table": table}))

	var colsDef []string
	for _, ct := range colTypes {
		sqlType := mapTypeToSQL(ct, driver)
		colDef := fmt.Sprintf("%s %s", ct.Name(), sqlType)
		if strings.EqualFold(ct.Name(), "id") {
			switch driver {
			case "sqlite":
				colDef += " INTEGER PRIMARY KEY AUTOINCREMENT"
			case "postgres", "postgresql", "pgx":
				colDef = "id BIGSERIAL PRIMARY KEY"
			default:
				colDef = "id BIGINT AUTO_INCREMENT PRIMARY KEY"
			}
		}
		colsDef = append(colsDef, colDef)
	}
	query := fmt.Sprintf("CREATE TABLE %s (%s)", table, strings.Join(colsDef, ", "))
	_, err := destDB.Exec(query)
	return err
}

func addMissingColumnsToTable(destDB *sql.DB, driver, table string, colTypes []*sql.ColumnType) error {
	destCols, err := getColumnNames(destDB, driver, table)
	if err != nil {
		return nil
	}
	destColMap := make(map[string]bool)
	for _, c := range destCols {
		destColMap[c] = true
	}

	for _, ct := range colTypes {
		if !destColMap[ct.Name()] {
			fmt.Printf("Columna %s no existe en %s. Agregándola...\n", ct.Name(), table)
			sqlType := mapTypeToSQL(ct, driver)
			query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, ct.Name(), sqlType)
			destDB.Exec(query)
		}
	}
	return nil
}

func mapTypeToSQL(ct *sql.ColumnType, driver string) string {
	srcType := strings.ToUpper(ct.DatabaseTypeName())
	switch driver {
	case "sqlite":
		return mapTypeToSQLite(srcType)
	case "postgres", "postgresql", "pgx":
		return mapTypeToPostgres(srcType)
	default:
		return mapTypeToMySQL(srcType, ct)
	}
}

func mapTypeToSQLite(srcType string) string {
	if strings.Contains(srcType, "INT") {
		return "INTEGER"
	}
	if strings.Contains(srcType, "CHAR") || strings.Contains(srcType, "TEXT") || srcType == "BLOB" || strings.Contains(srcType, "TIME") || strings.Contains(srcType, "DATE") {
		return "TEXT"
	}
	if strings.Contains(srcType, "FLOAT") || strings.Contains(srcType, "DOUBLE") || strings.Contains(srcType, "REAL") {
		return "REAL"
	}
	return "TEXT"
}

func mapTypeToPostgres(srcType string) string {
	if strings.Contains(srcType, "INT") {
		return "BIGINT"
	}
	if strings.Contains(srcType, "BOOL") {
		return "BOOLEAN"
	}
	if strings.Contains(srcType, "TIME") || strings.Contains(srcType, "DATE") {
		return "TIMESTAMP"
	}
	if strings.Contains(srcType, "FLOAT") || strings.Contains(srcType, "DOUBLE") || strings.Contains(srcType, "REAL") || strings.Contains(srcType, "DECIMAL") {
		return "DOUBLE PRECISION"
	}
	if strings.Contains(srcType, "JSON") {
		return "JSONB"
	}
	return "TEXT"
}

func mapTypeToMySQL(srcType string, ct *sql.ColumnType) string {
	if strings.Contains(srcType, "INT") {
		if strings.Contains(srcType, "UNSIGNED") {
			return mapMySQLUnsignedInt(srcType)
		}
		return mapMySQLSignedInt(srcType)
	}

	if strings.Contains(srcType, "TEXT") || strings.Contains(srcType, "BLOB") {
		return srcType
	}

	if strings.Contains(srcType, "CHAR") {
		length, ok := ct.Length()
		if ok && length > 0 {
			if strings.Contains(srcType, "VARCHAR") {
				return fmt.Sprintf("VARCHAR(%d)", length)
			}
			return fmt.Sprintf("CHAR(%d)", length)
		}
		return "VARCHAR(255)"
	}

	if strings.Contains(srcType, "TIME") || strings.Contains(srcType, "DATE") {
		return "TIMESTAMP NULL"
	}
	if strings.Contains(srcType, "BOOL") {
		return "TINYINT(1)"
	}
	if strings.Contains(srcType, "FLOAT") || strings.Contains(srcType, "DOUBLE") || strings.Contains(srcType, "DECIMAL") {
		return srcType
	}
	return "VARCHAR(255)"
}

func mapMySQLUnsignedInt(srcType string) string {
	switch {
	case strings.Contains(srcType, "TINYINT"):
		return "TINYINT UNSIGNED"
	case strings.Contains(srcType, "SMALLINT"):
		return "SMALLINT UNSIGNED"
	case strings.Contains(srcType, "MEDIUMINT"):
		return "MEDIUMINT UNSIGNED"
	case strings.Contains(srcType, "BIGINT"):
		return "BIGINT UNSIGNED"
	default:
		return "INT UNSIGNED"
	}
}

func mapMySQLSignedInt(srcType string) string {
	switch {
	case strings.Contains(srcType, "TINYINT"):
		return "TINYINT"
	case strings.Contains(srcType, "SMALLINT"):
		return "SMALLINT"
	case strings.Contains(srcType, "MEDIUMINT"):
		return "MEDIUMINT"
	case strings.Contains(srcType, "BIGINT"):
		return "BIGINT"
	default:
		return "INT"
	}
}

func getColumnNames(db *sql.DB, _ string, table string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s LIMIT 0", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rows.Columns()
}
