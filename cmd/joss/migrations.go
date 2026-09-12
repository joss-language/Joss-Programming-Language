package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/parser"
)

func runMigrations() {
	fmt.Println(i18n.Tr("migrateStarting"))

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
	fmt.Println(i18n.Tr("migrateDbSuccess"))

	// Ensure migration table exists
	if err := rt.EnsureMigrationTable(); err != nil {
		fmt.Println(i18n.Tr("migrateTableCreateError", i18n.M{"error": err}))
		os.Exit(1)
	}
	rt.EnsureAuthTables()

	if err := performMigrations(rt); err != nil {
		fmt.Println(i18n.Tr("migrateRunError", i18n.M{"error": err}))
		os.Exit(1)
	}
}

func performMigrations(rt *core.Runtime) error {
	// 2. Find migration files
	files, err := filepath.Glob("app/database/migrations/*.joss")
	if err != nil {
		return fmt.Errorf("buscando migraciones: %w", err)
	}

	if len(files) == 0 {
		fmt.Println(i18n.Tr("migrateNoneFound"))
		return nil
	}

	// 3. Get executed migrations
	executed := rt.GetExecutedMigrations()
	batch := rt.GetNextBatch()
	count := 0

	// 4. Execute pending migrations
	for _, file := range files {
		filename := filepath.Base(file)
		if executed[filename] {
			continue
		}

		fmt.Println(i18n.Tr("migrateItem", i18n.M{"name": filename, "batch": batch}))

		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("leyendo %s: %w", file, err)
		}

		l := parser.NewLexer(string(data))
		p := parser.NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) != 0 {
			return fmt.Errorf("parseando %s: %v", file, p.Errors())
		}

		rt.Execute(program)

		// Find the migration class and execute 'up'
		var migrationClass *parser.ClassStatement
		for _, stmt := range program.Statements {
			if classStmt, ok := stmt.(*parser.ClassStatement); ok {
				migrationClass = classStmt
				break
			}
		}

		if migrationClass != nil {
			// Instantiate
			instance := core.NewInstance(migrationClass)

			// Find 'up' method
			var upMethod *parser.MethodStatement
			for _, stmt := range migrationClass.Body.Statements {
				if method, ok := stmt.(*parser.MethodStatement); ok {
					if method.Name.Value == "up" {
						upMethod = method
						break
					}
				}
			}

			if upMethod != nil {
				fmt.Println(i18n.Tr("migrateRunningUp", i18n.M{"class": migrationClass.Name.Value}))
				rt.CallMethodEvaluated(upMethod, instance, []interface{}{})
			} else {
				return fmt.Errorf("la migracion %s no define up()", filename)
			}

		} else {
			return fmt.Errorf("la migracion %s no define una clase", filename)
		}

		if err := rt.LogMigration(filename, batch); err != nil {
			return fmt.Errorf("registrando migracion %s: %w", filename, err)
		}
		count++
	}

	if count == 0 {
		fmt.Println(i18n.Tr("migrateNoPending"))
	} else {
		fmt.Println(i18n.Tr("migrateCompleted"))
	}
	return nil
}
