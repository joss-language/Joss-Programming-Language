package core

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/parser"
)

// Task Implementation (Hit-based)
func (r *Runtime) executeTaskMethod(instance *Instance, method string, args []interface{}) interface{} {
	if method == "on_request" {
		if len(args) >= 3 {
			name := args[0].(string)
			// interval := args[1].(string)

			// The 3rd argument is now a *parser.BlockStatement because we updated Evaluator
			if block, ok := args[2].(*parser.BlockStatement); ok {
				fmt.Printf("[Task] Registrada tarea: %s\n", name)

				// Execute immediately in a goroutine for PoC
				newR := r.Fork()
				go func() {
					defer newR.Free()
					defer func() {
						if r := recover(); r != nil {
							fmt.Printf("[Task] Error en tarea %s: %v\n", name, r)
						}
					}()
					newR.executeBlock(block)
				}()
			}
		}
	}
	return nil
}
