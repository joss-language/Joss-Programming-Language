package php

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jossecurity/joss/pkg/plugincompiler/ir"
)

// PHPBackend compila código fuente PHP a Joss Plugin IR con grafo de llamadas para Tree Shaking.
type PHPBackend struct{}

func NewPHPBackend() *PHPBackend {
	return &PHPBackend{}
}

var (
	// Regex para detectar declaraciones de funciones en PHP
	funcDeclRegex = regexp.MustCompile(`(?:public\s+|private\s+|protected\s+|static\s+)*function\s+([a-zA-Z0-9_]+)\s*\(([^)]*)\)`)
	// Regex para detectar invocaciones de funciones o métodos estáticos/instancia
	callRegex = regexp.MustCompile(`(?:->|::)?\b([a-zA-Z0-9_]+)\s*\(`)
)

func (b *PHPBackend) Compile(sourcePath string, manifestName, version string, exports []string, permissions []string) (*ir.IRModule, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("php backend: error al leer %s: %w", sourcePath, err)
	}

	module := ir.NewModule(manifestName, version, "php")
	module.Exports = exports
	module.Permissions = permissions

	code := string(data)

	// Extraer todas las funciones y sus cuerpos
	type parsedFunc struct {
		name       string
		body       string
		isExported bool
	}

	exportSet := make(map[string]bool)
	for _, exp := range exports {
		exportSet[exp] = true
	}

	funcs := make(map[string]*parsedFunc)

	// Buscar declaraciones de función y sus bloques
	matches := funcDeclRegex.FindAllStringSubmatchIndex(code, -1)
	for i, loc := range matches {
		fnName := code[loc[2]:loc[3]]
		// Ignorar palabras clave que parecen llamadas
		if fnName == "if" || fnName == "while" || fnName == "for" || fnName == "foreach" || fnName == "switch" {
			continue
		}

		// Encontrar inicio de cuerpo '{' después de la firma
		bodyStart := strings.Index(code[loc[1]:], "{")
		if bodyStart == -1 {
			continue
		}
		startPos := loc[1] + bodyStart

		// Delimitar cuerpo hasta la siguiente función o balance de llaves
		var body string
		if i+1 < len(matches) {
			body = code[startPos:matches[i+1][0]]
		} else {
			body = code[startPos:]
		}

		funcs[fnName] = &parsedFunc{
			name:       fnName,
			body:       body,
			isExported: exportSet[fnName],
		}
	}

	// Asegurar que las funciones exportadas declaradas en exports existan en funcs
	for _, exp := range exports {
		if _, exists := funcs[exp]; !exists {
			funcs[exp] = &parsedFunc{
				name:       exp,
				body:       "",
				isExported: true,
			}
		}
	}

	// Construir IRFunction para cada función con sus dependencias OpCallStatic
	for name, pf := range funcs {
		instructions := make([]ir.IRInstruction, 0)
		instructions = append(instructions, ir.IRInstruction{
			Op:       ir.OpConst,
			Target:   "r0",
			ConstIdx: module.AddConstant(fmt.Sprintf("PHP context for %s", name)),
		})

		// Extraer llamadas internas en el cuerpo de la función
		if pf.body != "" {
			callMatches := callRegex.FindAllStringSubmatch(pf.body, -1)
			calledSet := make(map[string]bool)
			for _, cm := range callMatches {
				if len(cm) > 1 {
					called := cm[1]
					// Evitar palabras clave de control y recursión inmediata
					if called == "if" || called == "while" || called == "for" || called == "foreach" ||
						called == "switch" || called == "echo" || called == "print" || called == "return" ||
						called == "array" || called == "isset" || called == "empty" {
						continue
					}
					if !calledSet[called] {
						calledSet[called] = true
						instructions = append(instructions, ir.IRInstruction{
							Op:   ir.OpCallStatic,
							Args: []string{called},
						})
					}
				}
			}
		}

		instructions = append(instructions, ir.IRInstruction{Op: ir.OpReturn})

		module.Functions[name] = &ir.IRFunction{
			Name:       name,
			Params:     make([]ir.IRField, 0),
			ReturnType: "mixed",
			IsExported: pf.isExported,
			Blocks: []*ir.IRBlock{
				{
					Label:        "entry",
					Instructions: instructions,
				},
			},
		}
	}

	return module, nil
}
