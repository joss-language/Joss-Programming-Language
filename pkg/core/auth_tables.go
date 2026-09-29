package core

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

var authTablesEnsured sync.Map

// ensureAuthTables creates and auto-migrates Auth tables using Schema & GranDB (100% DB-agnostic)
func (r *Runtime) ensureAuthTables(usersTable, rolesTable, prefix string) {
	db := r.GetDB()
	if db == nil {
		return
	}
	ensureKey := fmt.Sprintf("%p:%s", db, usersTable)
	if _, exists := authTablesEnsured.Load(ensureKey); exists {
		return
	}

	// 1. Tabla de Roles con Schema::create
	if err := r.ensureInternalSchemaTable("roles", []schemaColumn{
		{name: "id", definition: "increments"},
		{name: "name", definition: "string(50)|unique"},
	}); err != nil {
		fmt.Printf("[Auth] Error creando tabla de roles: %v\n", err)
		return
	}

	// 2. Tabla de Usuarios con Schema::create
	if err := r.ensureInternalSchemaTable("users", []schemaColumn{
		{name: "id", definition: "bigIncrements"},
		{name: "user_token", definition: "string(128)|nullable"},
		{name: "username", definition: "string(50)|nullable"},
		{name: "first_name", definition: "string(100)|nullable"},
		{name: "last_name", definition: "string(100)|nullable"},
		{name: "email", definition: "string(191)|unique"},
		{name: "phone", definition: "string(20)|nullable"},
		{name: "password", definition: "string(255)"},
		{name: "role_id", definition: "integer|default(2)"},
		{name: "verificado", definition: "integer|default(0)"},
		{name: "token_expires_at", definition: "timestamp|nullable"},
		{name: "last_login_at", definition: "timestamp|nullable"},
		{name: "created_at", definition: "timestamp|nullable"},
		{name: "updated_at", definition: "timestamp|nullable"},
	}); err != nil {
		fmt.Printf("[Auth] Error creando tabla de usuarios: %v\n", err)
		return
	}

	// 3. Tabla de Password Resets con Schema::create
	if err := r.ensureInternalSchemaTable("password_resets", []schemaColumn{
		{name: "id", definition: "increments"},
		{name: "email", definition: "string(191)"},
		{name: "token", definition: "string(255)"},
		{name: "created_at", definition: "timestamp|nullable"},
		{name: "expires_at", definition: "timestamp|nullable"},
		{name: "used", definition: "integer|default(0)"},
	}); err != nil {
		fmt.Printf("[Auth] Error creando tabla de recuperacion: %v\n", err)
		return
	}

	// 4. Tabla de Cuentas Sociales (OAuth) con Schema::create
	if err := r.ensureInternalSchemaTable("user_social_accounts", []schemaColumn{
		{name: "id", definition: "bigIncrements"},
		{name: "user_id", definition: "bigInteger"},
		{name: "provider", definition: "string(50)"},
		{name: "provider_user_id", definition: "string(191)"},
		{name: "avatar", definition: "text|nullable"},
		{name: "created_at", definition: "timestamp|nullable"},
		{name: "updated_at", definition: "timestamp|nullable"},
	}); err != nil {
		fmt.Printf("[Auth] Error creando tabla user_social_accounts: %v\n", err)
		return
	}

	// 4. Auto-Patching de columnas en tablas existentes usando Schema::hasColumn y Schema::table
	patchColumns := map[string]string{
		"username":         "string(50)|nullable",
		"user_token":       "string(128)|nullable",
		"first_name":       "string(100)|nullable",
		"last_name":        "string(100)|nullable",
		"phone":            "string(20)|nullable",
		"verificado":       "integer|default(0)",
		"token_expires_at": "timestamp|nullable",
		"last_login_at":    "timestamp|nullable",
		"created_at":       "timestamp|nullable",
		"updated_at":       "timestamp|nullable",
	}

	for colName, colType := range patchColumns {
		hasCol, _ := r.executeSchemaMethod(nil, "hasColumn", []interface{}{"users", colName}).(bool)
		if !hasCol {
			if err := r.addInternalSchemaColumn("users", schemaColumn{name: colName, definition: colType}); err != nil {
				fmt.Printf("[Auth] Error agregando users.%s: %v\n", colName, err)
				return
			}
		}
	}

	// 5. Inserción de Roles por defecto usando GranDB
	for _, roleName := range []string{"admin", "client"} {
		var count int
		checkQuery := fmt.Sprintf("SELECT count(*) FROM %s WHERE name = ?", rolesTable)
		if err := db.QueryRow(checkQuery, roleName).Scan(&count); err == nil && count == 0 {
			r.insertFromMap(rolesTable, map[string]interface{}{
				"name": roleName,
			}, false)
		}
	}

	authTablesEnsured.Store(ensureKey, true)
}

func normalizeAuthEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func parseAuthExpiry(raw string) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false
	}

	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return parsed, true
		}
	}

	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func getString(data map[string]interface{}, key, def string) string {
	if val, ok := data[key]; ok {
		return fmt.Sprintf("%v", val)
	}
	return def
}
