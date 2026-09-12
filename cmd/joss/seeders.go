package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/parser"
)

func runSeeders() {
	fmt.Println(i18n.Tr("seedersRunning"))

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("\n[Error de Ejecucion JOSS en Seeders] %v\n", r)
			os.Exit(1)
		}
	}()

	rt := core.NewRuntime()
	rt.LoadEnv(nil)

	if rt.GetDB() == nil {
		fmt.Println(i18n.Tr("dbConnError"))
		return
	}

	files, err := filepath.Glob("app/database/seeders/*.joss")
	if err != nil {
		fmt.Printf("Error buscando seeders: %v\n", err)
		return
	}
	if len(files) == 0 {
		fmt.Println(i18n.Tr("seedersNotFound"))
		return
	}

	count := 0
	for _, file := range files {
		fmt.Printf("Seeder: %s...\n", filepath.Base(file))
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Error leyendo %s: %v\n", file, err)
			continue
		}

		l := parser.NewLexer(string(data))
		p := parser.NewParser(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			fmt.Printf("Error de parseo en %s:\n", file)
			for _, msg := range p.Errors() {
				fmt.Printf("\t%s\n", msg)
			}
			continue
		}

		rt.Execute(program)

		var seederClass *parser.ClassStatement
		for _, stmt := range program.Statements {
			if classStmt, ok := stmt.(*parser.ClassStatement); ok {
				seederClass = classStmt
				break
			}
		}

		if seederClass == nil {
			count++
			continue
		}

		instance := core.NewInstance(seederClass)
		var runMethod *parser.MethodStatement
		for _, stmt := range seederClass.Body.Statements {
			if method, ok := stmt.(*parser.MethodStatement); ok && method.Name.Value == "run" {
				runMethod = method
				break
			}
		}
		if runMethod == nil {
			fmt.Printf("Advertencia: No se encontro el metodo 'run' en %s\n", seederClass.Name.Value)
			continue
		}

		rt.CallMethodEvaluated(runMethod, instance, []interface{}{})
		count++
	}

	fmt.Printf("Seeders completados: %d\n", count)
}
