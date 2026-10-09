package ir

import "fmt"

// IRVerifier valida la consistencia estructural del programa en IR.
type IRVerifier struct {
	errors []string
}

func NewVerifier() *IRVerifier {
	return &IRVerifier{errors: make([]string, 0)}
}

func (v *IRVerifier) Verify(prog *Program) error {
	v.errors = nil
	if prog == nil {
		return fmt.Errorf("verifier: programa nil")
	}

	for fnName, fn := range prog.Functions {
		if fn == nil {
			v.addError("función %q es nil", fnName)
			continue
		}
		v.verifyFunction(fn)
	}

	if len(v.errors) > 0 {
		return fmt.Errorf("verifier: errores detectados:\n  - %s", joinErrors(v.errors, "\n  - "))
	}
	return nil
}

func (v *IRVerifier) verifyFunction(fn *Function) {
	if fn.EntryBlock == nil {
		v.addError("función @%s no tiene entry block", fn.Name)
	}
	if len(fn.Blocks) == 0 {
		v.addError("función @%s no tiene bloques básicos", fn.Name)
		return
	}

	blockLabels := make(map[string]bool)
	blockMap := make(map[*BasicBlock]bool)

	for _, block := range fn.Blocks {
		if blockLabels[block.Label] {
			v.addError("función @%s tiene bloque duplicado con etiqueta %q", fn.Name, block.Label)
		}
		blockLabels[block.Label] = true
		blockMap[block] = true

		// Verificar que el bloque termine en un terminator
		if block.Terminator == nil {
			v.addError("bloque %s en @%s carece de terminador", block.Label, fn.Name)
		} else {
			// Verificar tipo del terminador y sucesores
			for _, succ := range block.Successors() {
				if succ == nil {
					v.addError("bloque %s en @%s tiene sucesor nil", block.Label, fn.Name)
				}
			}

			// Validar ReturnTerminator contra ReturnType
			if ret, ok := block.Terminator.(*ReturnTerminator); ok {
				if ret.Val == nil {
					if fn.ReturnType.Kind != TypeKindVoid {
						v.addError("función @%s declara retorno %s pero tiene return void en %s", fn.Name, fn.ReturnType, block.Label)
					}
				} else {
					if fn.ReturnType.Kind == TypeKindVoid {
						v.addError("función @%s declara void pero retorna valor %s en %s", fn.Name, ret.Val.Type(), block.Label)
					}
				}
			}

			// Validar BranchTerminator
			if br, ok := block.Terminator.(*BranchTerminator); ok {
				if br.Cond == nil {
					v.addError("branch en bloque %s de @%s tiene condición nil", block.Label, fn.Name)
				} else if br.Cond.Type().Kind != TypeKindBool {
					v.addError("branch en bloque %s de @%s requiere condición bool, pero recibió %s", block.Label, fn.Name, br.Cond.Type())
				}
				if br.TrueBlock == nil || br.FalseBlock == nil {
					v.addError("branch en bloque %s de @%s tiene destino nil", block.Label, fn.Name)
				}
			}
		}

		// Validar instrucciones
		for _, inst := range block.Instructions {
			v.verifyInstruction(fn, block, inst)
		}
	}
}

func (v *IRVerifier) verifyInstruction(fn *Function, block *BasicBlock, inst Instruction) {
	if inst == nil {
		v.addError("instrucción nil en bloque %s de @%s", block.Label, fn.Name)
		return
	}

	switch i := inst.(type) {
	case *BinaryInst:
		if i.Left == nil || i.Right == nil {
			v.addError("instrucción binaria %s en %s tiene operandos nulos", i.Op, block.Label)
		}
	case *StoreInst:
		if i.Val == nil || i.Ptr == nil {
			v.addError("store en %s tiene valor o puntero nulo", block.Label)
		}
	case *LoadInst:
		if i.Ptr == nil {
			v.addError("load en %s tiene puntero nulo", block.Label)
		}
	}
}

func (v *IRVerifier) addError(format string, args ...interface{}) {
	v.errors = append(v.errors, fmt.Sprintf(format, args...))
}

func joinErrors(errs []string, sep string) string {
	res := ""
	for i, e := range errs {
		if i > 0 {
			res += sep
		}
		res += e
	}
	return res
}
