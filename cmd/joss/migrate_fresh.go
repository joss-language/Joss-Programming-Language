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
			fmt.Println()
			fmt.Println(i18n.Tr("migrateExecError", i18n.M{"error": r}))
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
		fmt.Println(i18n.Tr("migrateTableCreateError", i18n.M{"error": err}))
		os.Exit(1)
	}
	rt.EnsureAuthTables()

	// 4. Run migrations
	if err := performMigrations(rt); err != nil {
		fmt.Println(i18n.Tr("migrateRunError", i18n.M{"error": err}))
		os.Exit(1)
	}

	fmt.Println(i18n.Tr("migrateFreshSuccess"))
}
