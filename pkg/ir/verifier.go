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
	v.verifyDominance(fn)
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
	case *GlobalLoadInst:
		if i.Dest == nil || i.GlobalName == "" {
			v.addError("global_load en %s tiene destino o nombre vacío", block.Label)
		}
	case *GlobalStoreInst:
		if i.Val == nil || i.GlobalName == "" {
			v.addError("global_store en %s tiene valor o nombre vacío", block.Label)
		}
	case *BoxValueInst:
		if i.Dest == nil || i.Val == nil {
			v.addError("box_value en %s tiene operando nulo", block.Label)
		}
	case *UnboxValueInst:
		if i.Dest == nil || i.Val == nil {
			v.addError("unbox_value en %s tiene operando nulo", block.Label)
		}
	case *AllocBufferInst:
		if i.Dest == nil || i.Cap == nil {
			v.addError("alloc_buffer en %s tiene operando nulo", block.Label)
		}
	case *FreeBufferInst:
		if i.Buf == nil {
			v.addError("free_buffer en %s tiene operando nulo", block.Label)
		}
	case *BufferLenInst:
		if i.Dest == nil || i.Buf == nil {
			v.addError("buffer_len en %s tiene operando nulo", block.Label)
		}
	case *BufferGetInst:
		if i.Dest == nil || i.Buf == nil || i.Index == nil {
			v.addError("buffer_get en %s tiene operando nulo", block.Label)
		}
	case *BufferSetInst:
		if i.Buf == nil || i.Index == nil || i.Val == nil {
			v.addError("buffer_set en %s tiene operando nulo", block.Label)
		}
	case *ExternCallInst:
		if i.Symbol == "" {
			v.addError("extern_call en %s tiene símbolo vacío", block.Label)
		}
	case *NewObjectInst:
		if i.Dest == nil || i.ClassName == "" {
			v.addError("new_object en %s tiene destino o nombre de clase inválido", block.Label)
		}
	case *LoadFieldInst:
		if i.Dest == nil || i.Obj == nil || i.FieldName == "" {
			v.addError("load_field en %s tiene operandos inválidos", block.Label)
		}
	case *StoreFieldInst:
		if i.Obj == nil || i.Val == nil || i.FieldName == "" {
			v.addError("store_field en %s tiene operandos inválidos", block.Label)
		}
	case *NewArrayInst:
		if i.Dest == nil {
			v.addError("new_array en %s tiene destino nulo", block.Label)
		}
	case *ArrayGetInst:
		if i.Dest == nil || i.Array == nil || i.Index == nil {
			v.addError("array_get en %s tiene operandos inválidos", block.Label)
		}
	case *ArraySetInst:
		if i.Array == nil || i.Index == nil || i.Val == nil {
			v.addError("array_set en %s tiene operandos inválidos", block.Label)
		}
	case *NewMapInst:
		if i.Dest == nil {
			v.addError("new_map en %s tiene destino nulo", block.Label)
		}
	case *MapGetInst:
		if i.Dest == nil || i.Map == nil || i.Key == nil {
			v.addError("map_get en %s tiene operandos inválidos", block.Label)
		}
	case *MapSetInst:
		if i.Map == nil || i.Key == nil || i.Val == nil {
			v.addError("map_set en %s tiene operandos inválidos", block.Label)
		}
	case *MapDeleteInst:
		if i.Map == nil || i.Key == nil {
			v.addError("map_delete en %s tiene operandos inválidos", block.Label)
		}
	case *ThrowInst:
		if i.Val == nil {
			v.addError("throw en %s tiene valor nulo", block.Label)
		}
	case *TryCatchInst:
		if i.CatchVar == "" {
			v.addError("try_catch en %s tiene nombre de variable catch vacío", block.Label)
		}
	case *CallBuiltinInst:
		if i.BuiltinID == "" {
			v.addError("call_builtin en %s tiene ID vacío", block.Label)
		}
	case *CallPrimitiveMethodInst:
		if i.TypeKind == "" || i.MethodName == "" || i.Recv == nil {
			v.addError("call_primitive en %s tiene operandos inválidos", block.Label)
		}
	case *CallHostServiceInst:
		if i.ClassName == "" || i.MethodName == "" {
			v.addError("call_host en %s tiene operandos inválidos", block.Label)
		}
	case *CallIndirectInst:
		if i.CalleeVal == nil {
			v.addError("call_indirect en %s tiene callee nulo", block.Label)
		}
	}
}

func (v *IRVerifier) addError(format string, args ...interface{}) {
	v.errors = append(v.errors, fmt.Sprintf(format, args...))
}

func (v *IRVerifier) verifyDominance(fn *Function) {
	if fn.EntryBlock == nil || len(fn.Blocks) == 0 {
		return
	}
	// 1. Collect reachable blocks and predecessors via DFS
	reachable := make(map[*BasicBlock]bool)
	var rpo []*BasicBlock
	var dfs func(b *BasicBlock)
	dfs = func(b *BasicBlock) {
		if b == nil || reachable[b] {
			return
		}
		reachable[b] = true
		for _, succ := range b.Successors() {
			dfs(succ)
		}
		rpo = append(rpo, b)
	}
	dfs(fn.EntryBlock)

	// Reverse postorder
	for i, j := 0, len(rpo)-1; i < j; i, j = i+1, j-1 {
		rpo[i], rpo[j] = rpo[j], rpo[i]
	}

	preds := make(map[*BasicBlock][]*BasicBlock)
	for _, b := range rpo {
		for _, succ := range b.Successors() {
			if reachable[succ] {
				preds[succ] = append(preds[succ], b)
			}
		}
	}

	// 2. Iterative dominator computation
	dom := make(map[*BasicBlock]map[*BasicBlock]bool, len(rpo))
	for _, b := range rpo {
		if b == fn.EntryBlock {
			dom[b] = map[*BasicBlock]bool{b: true}
		} else {
			all := make(map[*BasicBlock]bool, len(rpo))
			for _, other := range rpo {
				all[other] = true
			}
			dom[b] = all
		}
	}
	changed := true
	for changed {
		changed = false
		for _, b := range rpo {
			if b == fn.EntryBlock || len(preds[b]) == 0 {
				continue
			}
			newDom := make(map[*BasicBlock]bool)
			firstPred := preds[b][0]
			for candidate := range dom[firstPred] {
				inAll := true
				for _, p := range preds[b][1:] {
					if !dom[p][candidate] {
						inAll = false
						break
					}
				}
				if inAll {
					newDom[candidate] = true
				}
			}
			newDom[b] = true
			if len(newDom) != len(dom[b]) {
				dom[b] = newDom
				changed = true
			} else {
				for k := range newDom {
					if !dom[b][k] {
						dom[b] = newDom
						changed = true
						break
					}
				}
			}
		}
	}

	// 3. Index definitions in reachable blocks (and nested try/catch blocks)
	type defLoc struct {
		block *BasicBlock
		index int
	}
	defs := make(map[int]defLoc)
	var indexNested func(host *BasicBlock, idx int, inst Instruction)
	indexNested = func(host *BasicBlock, idx int, inst Instruction) {
		if res := inst.Result(); res != nil {
			defs[res.ID] = defLoc{block: host, index: idx}
		}
		if tc, ok := inst.(*TryCatchInst); ok {
			for _, tb := range tc.TryBlocks {
				for _, ti := range tb.Instructions {
					indexNested(host, idx, ti)
				}
			}
			for _, cb := range tc.CatchBlocks {
				for _, ci := range cb.Instructions {
					indexNested(host, idx, ci)
				}
			}
		}
	}
	for _, b := range rpo {
		for idx, inst := range b.Instructions {
			indexNested(b, idx, inst)
		}
	}

	// 4. Verify every TempValue use is dominated by its definition
	checkUse := func(useBlock *BasicBlock, useIdx int, op Value) {
		t, ok := op.(*TempValue)
		if !ok {
			return
		}
		loc, defined := defs[t.ID]
		if !defined {
			v.addError("SSA dominance: temporal %s usado en %s de @%s sin definición alcanzable", t, useBlock.Label, fn.Name)
			return
		}
		if loc.block == useBlock {
			if loc.index >= useIdx {
				v.addError("SSA dominance: temporal %s usado antes de su definición en %s de @%s", t, useBlock.Label, fn.Name)
			}
			return
		}
		if !dom[useBlock][loc.block] {
			v.addError("SSA dominance: definición de %s en %s no domina su uso en %s de @%s", t, loc.block.Label, useBlock.Label, fn.Name)
		}
	}

	for _, b := range rpo {
		for idx, inst := range b.Instructions {
			for _, op := range inst.Operands() {
				checkUse(b, idx, op)
			}
		}
		if b.Terminator != nil {
			for _, op := range b.Terminator.Operands() {
				checkUse(b, len(b.Instructions), op)
			}
		}
	}
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
