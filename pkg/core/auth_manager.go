package core

import (
	"fmt"
)

// EnsureAuthTables creates js_roles and js_users if they don't exist
func (r *Runtime) EnsureAuthTables() {
	if r.GetDB() == nil {
		return
	}

	prefix := "js_"
	if val, ok := r.Env["PREFIX"]; ok {
		prefix = val
	}
	rolesTable := prefix + "roles"
	usersTable := prefix + "users"

	dbDriver := "mysql"
	if val, ok := r.Env["DB"]; ok {
		dbDriver = normalizeDatabaseDriver(val)
	}

	// 1. Create Roles Table
	var queryRoles string
	if dbDriver == "sqlite" {
		queryRoles = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name VARCHAR(50) UNIQUE
		)`, rolesTable)
	} else if dbDriver == "postgres" {
		queryRoles = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(50) UNIQUE
		)`, rolesTable)
	} else {
		queryRoles = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(50) UNIQUE
		)`, rolesTable)
	}
	r.databaseExecutor().Exec(queryRoles)

	// Seed Roles
	// SQLite doesn't support INSERT IGNORE. Use INSERT OR IGNORE.
	var insertRole string
	if dbDriver == "sqlite" {
		insertRole = "INSERT OR IGNORE INTO"
	} else if dbDriver == "postgres" {
		insertRole = "INSERT INTO"
	} else {
		insertRole = "INSERT IGNORE INTO"
	}
	roleSuffix := ""
	if dbDriver == "postgres" {
		roleSuffix = " ON CONFLICT (id) DO NOTHING"
	}
	r.databaseExecutor().Exec(fmt.Sprintf("%s %s (id, name) VALUES (1, 'admin')%s", insertRole, rolesTable, roleSuffix))
	r.databaseExecutor().Exec(fmt.Sprintf("%s %s (id, name) VALUES (2, 'client')%s", insertRole, rolesTable, roleSuffix))

	// 2. Create Users Table
	var queryUsers string
	if dbDriver == "sqlite" {
		queryUsers = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name VARCHAR(255),
			email VARCHAR(255) UNIQUE,
			password VARCHAR(255),
			role_id INTEGER DEFAULT 2,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (role_id) REFERENCES %s(id)
		)`, usersTable, rolesTable)
	} else if dbDriver == "postgres" {
		queryUsers = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255),
			email VARCHAR(255) UNIQUE,
			password VARCHAR(255),
			role_id BIGINT DEFAULT 2,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (role_id) REFERENCES %s(id)
		)`, usersTable, rolesTable)
	} else {
		queryUsers = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255),
			email VARCHAR(255) UNIQUE,
			password VARCHAR(255),
			role_id INT DEFAULT 2,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (role_id) REFERENCES %s(id)
		)`, usersTable, rolesTable)
	}
	r.databaseExecutor().Exec(queryUsers)
}
