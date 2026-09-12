package main

import (
	"fmt"
	"os"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/i18n"
)

func runMigrateFresh() {
	fmt.Println(i18n.Tr("migrateFreshStarting"))

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("\n[Error de Ejecución JOSS en Migraciones] %v\n", r)
			os.Exit(1)
		}
	}()

	// 1. Initialize Runtime
	rt := core.NewRuntime()
	rt.LoadEnv(nil)

	if rt.GetDB() == nil {
		fmt.Println(i18n.Tr("dbConnError"))
		return
	}
	fmt.Println(i18n.Tr("dbConnSuccess"))

	// 2. Drop all tables
	fmt.Println(i18n.Tr("migrateFreshDroppingTables"))
	rt.DropAllTables()

	// 3. Recreate migration table
	if err := rt.EnsureMigrationTable(); err != nil {
		fmt.Printf("Error creando tabla de migraciones: %v\n", err)
		os.Exit(1)
	}
	rt.EnsureAuthTables()

	// 4. Run migrations
	if err := performMigrations(rt); err != nil {
		fmt.Printf("Error ejecutando migraciones: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(i18n.Tr("migrateFreshSuccess"))
}
