package ir

// OptimizeProgram aplica pasadas de optimización sobre el programa IR:
// 1. Inlining de funciones pequeñas (hoja) de un solo bloque básico.
// 2. Eliminación de cargas redundantes (forwarding de store a load en un mismo bloque básico).
func OptimizeProgram(prog *Program) {
	if prog == nil {
		return
	}

	// Pase 1: Inlining de funciones hoja simples (ej. swap, getters/setters elementales)
	InlineLeafFunctions(prog)

	// Pase 2: Forwarding de cargas locales en cada bloque básico
	for _, fn := range prog.Functions {
		ForwardLocalLoads(fn)
	}

	// Pase 3: Elevación de cargas de memoria invariantes en bucles (LICM)
	for _, fn := range prog.Functions {
		HoistLoopInvariantLoads(fn)
	}

	// Pase 4: Eliminación de variables locales muertas (alloca/store sin loads)
	for _, fn := range prog.Functions {
		EliminateDeadAllocasAndStores(fn)
	}

	// Pase 5: Plegado y propagación de constantes (Constant Folding)
	for _, fn := range prog.Functions {
		FoldConstants(fn)
		PropagateCopies(fn)
		FoldConstants(fn)
	}
}

type inlineCandidate struct {
	fn     *Function
	bb     *BasicBlock
	retVal Value
}

// InlineLeafFunctions identifica funciones que tienen un solo bloque básico, terminan en Return,
// no son recursivas y tienen <= 20 instrucciones, e inyecta su cuerpo directamente en las llamadas @fn.
func InlineLeafFunctions(prog *Program) {
	candidates := make(map[string]*inlineCandidate)

	for name, fn := range prog.Functions {
		if name == "main" {
			continue
		}
		if len(fn.Blocks) != 1 {
			continue
		}
		bb := fn.Blocks[0]
		retTerm, ok := bb.Terminator.(*ReturnTerminator)
		if !ok {
			continue
		}

		// Solo inlinear funciones con instrucciones computacionales/memoria puras
		hasDisallowedInst := false
		for _, inst := range bb.Instructions {
			switch inst.(type) {
			case *AllocaInst, *StoreInst, *LoadInst, *MoveInst, *BinaryInst, *CompareInst, *UnaryInst, *ArrayGetInst, *ArraySetInst, *LoadFieldInst, *StoreFieldInst:
				// Permitidas para inlining ultrarrápido
			default:
				hasDisallowedInst = true
			}
		}
		if hasDisallowedInst {
			continue
		}

		if len(bb.Instructions) <= 30 {
			candidates[name] = &inlineCandidate{
				fn:     fn,
				bb:     bb,
				retVal: retTerm.Val,
			}
		}
	}

	if len(candidates) == 0 {
		return
	}

	// Inlinear en todas las funciones que llamen a estos candidatos
	for _, caller := range prog.Functions {
		inlineInFunction(caller, candidates)
	}
}

func inlineInFunction(caller *Function, candidates map[string]*inlineCandidate) {
	for _, blk := range caller.Blocks {
		newInsts := make([]Instruction, 0, len(blk.Instructions))
		changed := false

		for _, inst := range blk.Instructions {
			call, ok := inst.(*CallInst)
			if !ok {
				newInsts = append(newInsts, inst)
				continue
			}

			cand, isCand := candidates[call.Callee]
			if !isCand {
				newInsts = append(newInsts, inst)
				continue
			}

			// Inlinear el cuerpo de cand
			changed = true
			valMap := make(map[Value]Value)

			// Mapear parámetros a argumentos de la llamada
			for idx, p := range cand.fn.Params {
				if idx < len(call.Args) {
					valMap[p] = call.Args[idx]
				}
			}

			// Clonar instrucciones sustituyendo valores
			for _, cInst := range cand.bb.Instructions {
				cloned := cloneInstructionWithRemap(caller, cInst, valMap)
				if cloned != nil {
					newInsts = append(newInsts, cloned)
				}
			}

			// Si la llamada esperaba un resultado, asignarlo del retorno del callee
			if call.Dest != nil && cand.retVal != nil {
				retV := remapVal(cand.retVal, valMap)
				newInsts = append(newInsts, &MoveInst{
					Dest: call.Dest,
					Src:  retV,
				})
			}
		}

		if changed {
			blk.Instructions = newInsts
		}
	}
}

func remapVal(v Value, valMap map[Value]Value) Value {
	if v == nil {
		return nil
	}
	if mapped, exists := valMap[v]; exists {
		return mapped
	}
	return v
}

func cloneInstructionWithRemap(caller *Function, inst Instruction, valMap map[Value]Value) Instruction {
	remap := func(v Value) Value {
		return remapVal(v, valMap)
	}

	switch i := inst.(type) {
	case *AllocaInst:
		newDest := caller.NewTemp(i.AllocType, i.Dest.Hint)
		valMap[i.Dest] = newDest
		return &AllocaInst{
			Dest:      newDest,
			AllocType: i.AllocType,
		}

	case *StoreInst:
		return &StoreInst{
			Val: remap(i.Val),
			Ptr: remap(i.Ptr),
		}

	case *LoadInst:
		newDest := caller.NewTemp(i.Dest.Type(), i.Dest.Hint)
		valMap[i.Dest] = newDest
		return &LoadInst{
			Dest: newDest,
			Ptr:  remap(i.Ptr),
		}

	case *MoveInst:
		newDest := caller.NewTemp(i.Dest.Type(), i.Dest.Hint)
		valMap[i.Dest] = newDest
		return &MoveInst{
			Dest: newDest,
			Src:  remap(i.Src),
		}

	case *BinaryInst:
		newDest := caller.NewTemp(i.Dest.Type(), i.Dest.Hint)
		valMap[i.Dest] = newDest
		return &BinaryInst{
			Dest:  newDest,
			Op:    i.Op,
			Left:  remap(i.Left),
			Right: remap(i.Right),
		}

	case *CompareInst:
		newDest := caller.NewTemp(i.Dest.Type(), i.Dest.Hint)
		valMap[i.Dest] = newDest
		return &CompareInst{
			Dest:  newDest,
			Op:    i.Op,
			Left:  remap(i.Left),
			Right: remap(i.Right),
		}

	case *UnaryInst:
		newDest := caller.NewTemp(i.Dest.Type(), i.Dest.Hint)
		valMap[i.Dest] = newDest
		return &UnaryInst{
			Dest: newDest,
			Op:   i.Op,
			Val:  remap(i.Val),
		}

	case *LoadFieldInst:
		newDest := caller.NewTemp(i.Dest.Type(), i.Dest.Hint)
		valMap[i.Dest] = newDest
		return &LoadFieldInst{
			Dest:      newDest,
			Obj:       remap(i.Obj),
			FieldName: i.FieldName,
		}

	case *StoreFieldInst:
		return &StoreFieldInst{
			Val:       remap(i.Val),
			Obj:       remap(i.Obj),
			FieldName: i.FieldName,
		}

	case *ArrayGetInst:
		newDest := caller.NewTemp(i.Dest.Type(), i.Dest.Hint)
		valMap[i.Dest] = newDest
		return &ArrayGetInst{
			Dest:  newDest,
			Array: remap(i.Array),
			Index: remap(i.Index),
		}

	case *ArraySetInst:
		return &ArraySetInst{
			Val:   remap(i.Val),
			Array: remap(i.Array),
			Index: remap(i.Index),
		}

	case *CallInst:
		var newDest *TempValue
		if i.Dest != nil {
			newDest = caller.NewTemp(i.Dest.Type(), i.Dest.Hint)
			valMap[i.Dest] = newDest
		}
		newArgs := make([]Value, len(i.Args))
		for idx, a := range i.Args {
			newArgs[idx] = remap(a)
		}
		return &CallInst{
			Dest:   newDest,
			Callee: i.Callee,
			Args:   newArgs,
		}

	case *CallPrimitiveMethodInst:
		var newDest *TempValue
		if i.Dest != nil {
			newDest = caller.NewTemp(i.Dest.Type(), i.Dest.Hint)
			valMap[i.Dest] = newDest
		}
		newArgs := make([]Value, len(i.Args))
		for idx, a := range i.Args {
			newArgs[idx] = remap(a)
		}
		return &CallPrimitiveMethodInst{
			Dest:       newDest,
			TypeKind:   i.TypeKind,
			MethodName: i.MethodName,
			Recv:       remap(i.Recv),
			Args:       newArgs,
		}

	case *CallBuiltinInst:
		var newDest *TempValue
		if i.Dest != nil {
			newDest = caller.NewTemp(i.Dest.Type(), i.Dest.Hint)
			valMap[i.Dest] = newDest
		}
		newArgs := make([]Value, len(i.Args))
		for idx, a := range i.Args {
			newArgs[idx] = remap(a)
		}
		return &CallBuiltinInst{
			Dest:      newDest,
			BuiltinID: i.BuiltinID,
			Args:      newArgs,
		}

	case *CallRuntimeInst:
		var newDest *TempValue
		if i.Dest != nil {
			newDest = caller.NewTemp(i.Dest.Type(), i.Dest.Hint)
			valMap[i.Dest] = newDest
		}
		newArgs := make([]Value, len(i.Args))
		for idx, a := range i.Args {
			newArgs[idx] = remap(a)
		}
		return &CallRuntimeInst{
			Dest: newDest,
			Func: i.Func,
			Args: newArgs,
		}
	}

	return inst
}

// ForwardLocalLoads rastrea los valores disponibles (almacenados o cargados) en punteros locales
// de tipo AllocaInst dentro del mismo bloque básico, reemplazando cargas redundantes consecutivas
// por un MoveInst o reuso del valor existente.
func ForwardLocalLoads(fn *Function) {
	for _, blk := range fn.Blocks {
		availMap := make(map[int]Value) // allocaTempID -> latest Value available in this basic block

		newInsts := make([]Instruction, 0, len(blk.Instructions))
		for _, inst := range blk.Instructions {
			switch i := inst.(type) {
			case *StoreInst:
				if ptrTemp, ok := i.Ptr.(*TempValue); ok {
					availMap[ptrTemp.ID] = i.Val
				}
				newInsts = append(newInsts, inst)

			case *LoadInst:
				if ptrTemp, ok := i.Ptr.(*TempValue); ok {
					if existingVal, exists := availMap[ptrTemp.ID]; exists {
						// Transformar en MoveInst directo: %dest = move %existingVal
						newInsts = append(newInsts, &MoveInst{
							Dest: i.Dest,
							Src:  existingVal,
						})
						continue
					}
					// Registrar este valor como disponible para cargas posteriores del mismo puntero
					availMap[ptrTemp.ID] = i.Dest
				}
				newInsts = append(newInsts, inst)

			case *CallInst, *CallIndirectInst, *CallPrimitiveMethodInst, *CallHostServiceInst:
				newInsts = append(newInsts, inst)

			default:
				newInsts = append(newInsts, inst)
			}
		}
		blk.Instructions = newInsts
	}
}

// canReachWithout returns true if 'from' can reach 'to' without passing through 'avoid'.
func canReachWithout(from, to, avoid *BasicBlock) bool {
	visited := make(map[*BasicBlock]bool)
	if avoid != nil {
		visited[avoid] = true
	}
	var queue []*BasicBlock
	for _, succ := range from.Successors() {
		if succ == to {
			return true
		}
		if !visited[succ] {
			visited[succ] = true
			queue = append(queue, succ)
		}
	}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, succ := range curr.Successors() {
			if succ == to {
				return true
			}
			if !visited[succ] {
				visited[succ] = true
				queue = append(queue, succ)
			}
		}
	}
	return false
}

// HoistLoopInvariantLoads identifica cargas de memoria (*ir.LoadInst) dentro de bucles
// que leen punteros locales que NUNCA son modificados dentro del bucle, y las eleva
// al bloque preheader para ejecutarlas una sola vez antes de entrar al bucle.
func HoistLoopInvariantLoads(fn *Function) {
	for _, hdr := range fn.Blocks {
		var backEdgeBlocks []*BasicBlock
		var preheader *BasicBlock

		// Analizar todos los bloques que transfieren el control a 'hdr'
		for _, blk := range fn.Blocks {
			jumpsToHdr := false
			for _, succ := range blk.Successors() {
				if succ == hdr {
					jumpsToHdr = true
					break
				}
			}
			if !jumpsToHdr {
				continue
			}

			// Si blk es el EntryBlock o es alcanzable desde EntryBlock sin pasar por 'hdr',
			// es un predecesor externo (preheader o entrada al bucle).
			// Si no, 'hdr' domina a 'blk', por lo que 'blk' está en el bucle y es back-edge.
			isBackEdge := blk != fn.EntryBlock && !canReachWithout(fn.EntryBlock, blk, hdr)
			if isBackEdge {
				backEdgeBlocks = append(backEdgeBlocks, blk)
			} else {
				// Es un predecesor externo (preheader o entrada al bucle)
				if preheader == nil {
					preheader = blk
				}
			}
		}

		if len(backEdgeBlocks) == 0 || preheader == nil {
			continue
		}

		// Construir el conjunto de bloques que componen el bucle natural
		loopBlocksMap := make(map[*BasicBlock]bool)
		loopBlocksMap[hdr] = true
		for _, b := range backEdgeBlocks {
			loopBlocksMap[b] = true
		}
		for changed := true; changed; {
			changed = false
			for _, blk := range fn.Blocks {
				if loopBlocksMap[blk] || blk == preheader {
					continue
				}
				// Si blk puede alcanzar hdr sin pasar por preheader y es alcanzable desde hdr
				if canReachWithout(hdr, blk, preheader) && canReachWithout(blk, hdr, preheader) {
					loopBlocksMap[blk] = true
					changed = true
				}
			}
		}

		// Identificar todos los punteros destino modificados por StoreInst en el bucle
		storedPtrs := make(map[int]bool)
		for blk := range loopBlocksMap {
			for _, inst := range blk.Instructions {
				if store, ok := inst.(*StoreInst); ok {
					if ptrTemp, isTemp := store.Ptr.(*TempValue); isTemp {
						storedPtrs[ptrTemp.ID] = true
					}
				}
			}
		}

		// Elevar cargas invariantes al preheader
		hoistedMap := make(map[int]*TempValue) // ptrTempID -> hoisted Dest TempValue
		for blk := range loopBlocksMap {
			remainingInsts := make([]Instruction, 0, len(blk.Instructions))
			for _, inst := range blk.Instructions {
				if load, ok := inst.(*LoadInst); ok {
					if ptrTemp, isTemp := load.Ptr.(*TempValue); isTemp {
						if !storedPtrs[ptrTemp.ID] {
							if alreadyHoistedVal, exists := hoistedMap[ptrTemp.ID]; exists {
								// Ya fue elevado para este mismo puntero: reemplazar por un Move directo
								remainingInsts = append(remainingInsts, &MoveInst{
									Dest: load.Dest,
									Src:  alreadyHoistedVal,
								})
								continue
							}
							// Elevar carga al preheader
							hoistedMap[ptrTemp.ID] = load.Dest
							preheader.AddInstruction(load)
							continue
						}
					}
				}
				remainingInsts = append(remainingInsts, inst)
			}
			blk.Instructions = remainingInsts
		}
	}
}

type arrayAccessKey struct {
	arrVal string
	idxVal string
}

// EliminateRedundantArrayGets rastrea lecturas de array (array_get) dentro de un bloque básico
// y entre bloques con predecesor único, reutilizando el valor previamente cargado siempre que
// el array no haya sido modificado por un array_set o una llamada a función.
func EliminateRedundantArrayGets(fn *Function) {
	predsMap := make(map[*BasicBlock][]*BasicBlock, len(fn.Blocks))
	for _, blk := range fn.Blocks {
		for _, succ := range blk.Successors() {
			predsMap[succ] = append(predsMap[succ], blk)
		}
	}

	blockOutMap := make(map[*BasicBlock]map[arrayAccessKey]Value, len(fn.Blocks))
	moveMap := make(map[int]Value)

	canonicalVal := func(v Value) Value {
		for {
			if temp, ok := v.(*TempValue); ok {
				if src, exists := moveMap[temp.ID]; exists {
					v = src
					continue
				}
			}
			break
		}
		return v
	}

	for _, blk := range fn.Blocks {
		availMap := make(map[arrayAccessKey]Value)

		preds := predsMap[blk]
		if len(preds) == 1 {
			if predOut, exists := blockOutMap[preds[0]]; exists {
				for k, v := range predOut {
					availMap[k] = v
				}
			}
		}

		newInsts := make([]Instruction, 0, len(blk.Instructions))
		for _, inst := range blk.Instructions {
			switch i := inst.(type) {
			case *MoveInst:
				if i.Dest != nil {
					moveMap[i.Dest.ID] = i.Src
				}
				newInsts = append(newInsts, inst)

			case *ArrayGetInst:
				cArr := canonicalVal(i.Array)
				cIdx := canonicalVal(i.Index)
				if cArr != nil && cIdx != nil {
					key := arrayAccessKey{
						arrVal: cArr.String(),
						idxVal: cIdx.String(),
					}
					if existingVal, exists := availMap[key]; exists {
						newInsts = append(newInsts, &MoveInst{
							Dest: i.Dest,
							Src:  existingVal,
						})
						moveMap[i.Dest.ID] = existingVal
						continue
					}
					availMap[key] = i.Dest
				}
				newInsts = append(newInsts, inst)

			case *ArraySetInst:
				cArr := canonicalVal(i.Array)
				if cArr != nil {
					arrKey := cArr.String()
					for k := range availMap {
						if k.arrVal == arrKey {
							delete(availMap, k)
						}
					}
				} else {
					availMap = make(map[arrayAccessKey]Value)
				}
				newInsts = append(newInsts, inst)

			case *CallInst, *CallIndirectInst, *CallPrimitiveMethodInst, *CallHostServiceInst, *CallRuntimeInst:
				availMap = make(map[arrayAccessKey]Value)
				newInsts = append(newInsts, inst)

			default:
				newInsts = append(newInsts, inst)
			}
		}
		blk.Instructions = newInsts
		blockOutMap[blk] = availMap
	}
}

// EliminateDeadAllocasAndStores elimina variables locales (alloca) y sus escrituras (store)
// cuando el puntero asignado nunca es leído por ninguna instrucción LoadInst de la función.
func EliminateDeadAllocasAndStores(fn *Function) {
	loadCount := make(map[int]int)
	var scanLoads func(blocks []*BasicBlock)
	scanLoads = func(blocks []*BasicBlock) {
		for _, blk := range blocks {
			for _, inst := range blk.Instructions {
				switch i := inst.(type) {
				case *LoadInst:
					if ptrTemp, okPtr := i.Ptr.(*TempValue); okPtr {
						loadCount[ptrTemp.ID]++
					}
				case *TryCatchInst:
					if ptrTemp, okPtr := i.CatchVarPtr.(*TempValue); okPtr {
						loadCount[ptrTemp.ID]++
					}
					scanLoads(i.TryBlocks)
					scanLoads(i.CatchBlocks)
				}
			}
		}
	}
	scanLoads(fn.Blocks)

	var pruneBlocks func(blocks []*BasicBlock)
	pruneBlocks = func(blocks []*BasicBlock) {
		for _, blk := range blocks {
			newInsts := make([]Instruction, 0, len(blk.Instructions))
			for _, inst := range blk.Instructions {
				switch i := inst.(type) {
				case *AllocaInst:
					if i.Dest != nil && loadCount[i.Dest.ID] == 0 {
						continue
					}
				case *StoreInst:
					if ptrTemp, okPtr := i.Ptr.(*TempValue); okPtr && loadCount[ptrTemp.ID] == 0 {
						continue
					}
				case *TryCatchInst:
					pruneBlocks(i.TryBlocks)
					pruneBlocks(i.CatchBlocks)
				}
				newInsts = append(newInsts, inst)
			}
			blk.Instructions = newInsts
		}
	}
	pruneBlocks(fn.Blocks)
}

// PropagateCopies rastrea MoveInsts redundantes (%dest = move %src) en la función
// y propaga el valor original %src en todos los usos de %dest (incluyendo a través de
// bloques básicos cuando %dest tiene una única definición SSA), eliminando la instrucción move.
func PropagateCopies(fn *Function) {
	defCount := make(map[Value]int)
	var countDefs func(blocks []*BasicBlock)
	countDefs = func(blocks []*BasicBlock) {
		for _, blk := range blocks {
			for _, inst := range blk.Instructions {
				if tc, ok := inst.(*TryCatchInst); ok {
					countDefs(tc.TryBlocks)
					countDefs(tc.CatchBlocks)
					continue
				}
				if d := getInstDest(inst); d != nil {
					defCount[d]++
				}
			}
		}
	}
	countDefs(fn.Blocks)

	globalMap := make(map[Value]Value)
	resolveGlobal := func(v Value) Value {
		for {
			if mapped, exists := globalMap[v]; exists && mapped != v {
				v = mapped
				continue
			}
			break
		}
		return v
	}

	var propBlocks func(blocks []*BasicBlock)
	propBlocks = func(blocks []*BasicBlock) {
		for _, blk := range blocks {
			localMap := make(map[Value]Value)
			combinedMap := make(map[Value]Value)
			for k, v := range globalMap {
				combinedMap[k] = resolveGlobal(v)
			}
			newInsts := make([]Instruction, 0, len(blk.Instructions))
			for _, inst := range blk.Instructions {
				if tc, ok := inst.(*TryCatchInst); ok {
					propBlocks(tc.TryBlocks)
					propBlocks(tc.CatchBlocks)
					newInsts = append(newInsts, inst)
					continue
				}
				// Si esta instrucción redefine un destino con múltiples definiciones, invalidar mapeo local previo
				if d := getInstDest(inst); d != nil && defCount[d] > 1 {
					delete(localMap, d)
					delete(combinedMap, d)
				}
				remapInstructionValues(inst, combinedMap)
				if mv, ok := inst.(*MoveInst); ok && mv.Dest != nil {
					src := mv.Src
					for {
						if mapped, exists := combinedMap[src]; exists && mapped != src {
							src = mapped
							continue
						}
						break
					}
					// Solo eliminar MoveInst si Dest tiene una única definición en toda la función
					// y Src también es inmutable (constante o temp de única definición).
					srcSingleDef := true
					if _, isTemp := src.(*TempValue); isTemp && defCount[src] > 1 {
						srcSingleDef = false
					}
					if defCount[mv.Dest] == 1 && srcSingleDef {
						globalMap[mv.Dest] = src
						combinedMap[mv.Dest] = src
						continue
					}
					localMap[mv.Dest] = src
					combinedMap[mv.Dest] = src
				}
				newInsts = append(newInsts, inst)
			}
			blk.Instructions = newInsts
			if blk.Terminator != nil {
				remapTerminatorValues(blk.Terminator, combinedMap)
			}
		}
	}
	propBlocks(fn.Blocks)

	// Segunda pasada para remapeo de usos en bloques posteriores (ej. tras ternary_merge / coal_merge)
	if len(globalMap) > 0 {
		resolvedGlobal := make(map[Value]Value, len(globalMap))
		for k, v := range globalMap {
			resolvedGlobal[k] = resolveGlobal(v)
		}
		var remapAll func(blocks []*BasicBlock)
		remapAll = func(blocks []*BasicBlock) {
			for _, blk := range blocks {
				for _, inst := range blk.Instructions {
					if tc, ok := inst.(*TryCatchInst); ok {
						remapAll(tc.TryBlocks)
						remapAll(tc.CatchBlocks)
						continue
					}
					remapInstructionValues(inst, resolvedGlobal)
				}
				if blk.Terminator != nil {
					remapTerminatorValues(blk.Terminator, resolvedGlobal)
				}
			}
		}
		remapAll(fn.Blocks)
	}
}

func getInstDest(inst Instruction) Value {
	if inst == nil {
		return nil
	}
	if res := inst.Result(); res != nil {
		return res
	}
	return nil
}

func remapInstructionValues(inst Instruction, valMap map[Value]Value) {
	remap := func(v Value) Value {
		if mapped, exists := valMap[v]; exists {
			return mapped
		}
		return v
	}

	switch i := inst.(type) {
	case *StoreInst:
		i.Val = remap(i.Val)
	case *LoadInst:
		// Do not remap i.Ptr (must remain AllocaInst pointer)
	case *MoveInst:
		i.Src = remap(i.Src)
	case *BinaryInst:
		i.Left = remap(i.Left)
		i.Right = remap(i.Right)
	case *CompareInst:
		i.Left = remap(i.Left)
		i.Right = remap(i.Right)
	case *UnaryInst:
		i.Val = remap(i.Val)
	case *ArrayGetInst:
		i.Array = remap(i.Array)
		i.Index = remap(i.Index)
	case *ArraySetInst:
		i.Val = remap(i.Val)
		i.Array = remap(i.Array)
		i.Index = remap(i.Index)
	case *CallInst:
		for idx := range i.Args {
			i.Args[idx] = remap(i.Args[idx])
		}
	case *CallPrimitiveMethodInst:
		i.Recv = remap(i.Recv)
		for idx := range i.Args {
			i.Args[idx] = remap(i.Args[idx])
		}
	case *CallBuiltinInst:
		for idx := range i.Args {
			i.Args[idx] = remap(i.Args[idx])
		}
	case *CallHostServiceInst:
		if i.Recv != nil {
			i.Recv = remap(i.Recv)
		}
		for idx := range i.Args {
			i.Args[idx] = remap(i.Args[idx])
		}
	case *CastInst:
		i.Val = remap(i.Val)
	case *CallRuntimeInst:
		for idx := range i.Args {
			i.Args[idx] = remap(i.Args[idx])
		}
	case *LoadFieldInst:
		i.Obj = remap(i.Obj)
	case *StoreFieldInst:
		i.Val = remap(i.Val)
		i.Obj = remap(i.Obj)
	case *MapGetInst:
		i.Map = remap(i.Map)
		i.Key = remap(i.Key)
	case *MapSetInst:
		i.Val = remap(i.Val)
		i.Map = remap(i.Map)
		i.Key = remap(i.Key)
	case *NewArrayInst:
		for idx := range i.Elements {
			i.Elements[idx] = remap(i.Elements[idx])
		}
	case *NewMapInst:
		for idx := range i.Pairs {
			i.Pairs[idx][0] = remap(i.Pairs[idx][0])
			i.Pairs[idx][1] = remap(i.Pairs[idx][1])
		}
	case *CallIndirectInst:
		i.CalleeVal = remap(i.CalleeVal)
		for idx := range i.Args {
			i.Args[idx] = remap(i.Args[idx])
		}
	case *ThrowInst:
		i.Val = remap(i.Val)
	}
}

func remapTerminatorValues(term Terminator, valMap map[Value]Value) {
	remap := func(v Value) Value {
		if mapped, exists := valMap[v]; exists {
			return mapped
		}
		return v
	}
	switch t := term.(type) {
	case *ReturnTerminator:
		if t.Val != nil {
			t.Val = remap(t.Val)
		}
	case *BranchTerminator:
		t.Cond = remap(t.Cond)
	}
}

// FoldConstants evalúa en tiempo de compilación operaciones aritméticas, lógicas, comparaciones
// y concatenación de strings entre constantes literales inmediatas, sustituyéndolas por MoveInst
// de la constante calculada y propagándolas inmediatamente dentro del mismo bloque básico.
func FoldConstants(fn *Function) {
	var foldBlocks func(blocks []*BasicBlock)
	foldBlocks = func(blocks []*BasicBlock) {
		for _, blk := range blocks {
			constMap := make(map[Value]Value)
			newInsts := make([]Instruction, 0, len(blk.Instructions))
			for _, inst := range blk.Instructions {
				if tc, ok := inst.(*TryCatchInst); ok {
					foldBlocks(tc.TryBlocks)
					foldBlocks(tc.CatchBlocks)
					newInsts = append(newInsts, inst)
					continue
				}
				remapInstructionValues(inst, constMap)
				if mv, ok := inst.(*MoveInst); ok && mv.Dest != nil {
					constMap[mv.Dest] = mv.Src
					newInsts = append(newInsts, inst)
					continue
				}
				switch i := inst.(type) {
				case *BinaryInst:
					cLeft, okLeft := i.Left.(*ConstInt)
					cRight, okRight := i.Right.(*ConstInt)
					if okLeft && okRight && i.Dest != nil {
						var res int64
						valid := true
						switch i.Op {
						case OpAdd:
							res = cLeft.Val + cRight.Val
						case OpSub:
							res = cLeft.Val - cRight.Val
						case OpMul:
							res = cLeft.Val * cRight.Val
						case OpDiv:
							if cRight.Val != 0 {
								res = cLeft.Val / cRight.Val
							} else {
								valid = false
							}
						case OpMod:
							if cRight.Val != 0 {
								res = cLeft.Val % cRight.Val
							} else {
								valid = false
							}
						case OpAnd:
							res = cLeft.Val & cRight.Val
						case OpOr:
							res = cLeft.Val | cRight.Val
						case OpXor:
							res = cLeft.Val ^ cRight.Val
						case OpShl:
							res = cLeft.Val << uint(cRight.Val)
						case OpShr:
							res = cLeft.Val >> uint(cRight.Val)
						default:
							valid = false
						}
						if valid {
							folded := NewConstInt(res, i.Dest.Type())
							constMap[i.Dest] = folded
							newInsts = append(newInsts, &MoveInst{
								Dest: i.Dest,
								Src:  folded,
							})
							continue
						}
					}

				case *CompareInst:
					cLeft, okLeft := i.Left.(*ConstInt)
					cRight, okRight := i.Right.(*ConstInt)
					if okLeft && okRight && i.Dest != nil {
						var res bool
						switch i.Op {
						case OpCmpEq:
							res = cLeft.Val == cRight.Val
						case OpCmpNe:
							res = cLeft.Val != cRight.Val
						case OpCmpLt:
							res = cLeft.Val < cRight.Val
						case OpCmpLe:
							res = cLeft.Val <= cRight.Val
						case OpCmpGt:
							res = cLeft.Val > cRight.Val
						case OpCmpGe:
							res = cLeft.Val >= cRight.Val
						default:
							newInsts = append(newInsts, inst)
							continue
						}
						folded := NewConstBool(res)
						constMap[i.Dest] = folded
						newInsts = append(newInsts, &MoveInst{
							Dest: i.Dest,
							Src:  folded,
						})
						continue
					}

				case *UnaryInst:
					if i.Op == OpNeg {
						if cVal, ok := i.Val.(*ConstInt); ok && i.Dest != nil {
							folded := NewConstInt(-cVal.Val, i.Dest.Type())
							constMap[i.Dest] = folded
							newInsts = append(newInsts, &MoveInst{
								Dest: i.Dest,
								Src:  folded,
							})
							continue
						}
					} else if i.Op == OpNot {
						if cVal, ok := i.Val.(*ConstBool); ok && i.Dest != nil {
							folded := NewConstBool(!cVal.Val)
							constMap[i.Dest] = folded
							newInsts = append(newInsts, &MoveInst{
								Dest: i.Dest,
								Src:  folded,
							})
							continue
						}
					}
				case *CallRuntimeInst:
					if i.Func == "str_concat" && len(i.Args) == 2 && i.Dest != nil {
						sLeft, okLeft := i.Args[0].(*ConstString)
						sRight, okRight := i.Args[1].(*ConstString)
						if okLeft && okRight {
							folded := NewConstString(sLeft.Val + sRight.Val)
							constMap[i.Dest] = folded
							newInsts = append(newInsts, &MoveInst{
								Dest: i.Dest,
								Src:  folded,
							})
							continue
						}
						if okLeft && sLeft.Val == "" && i.Args[1].Type().Kind == TypeKindString {
							constMap[i.Dest] = i.Args[1]
							newInsts = append(newInsts, &MoveInst{
								Dest: i.Dest,
								Src:  i.Args[1],
							})
							continue
						}
						if okRight && sRight.Val == "" && i.Args[0].Type().Kind == TypeKindString {
							constMap[i.Dest] = i.Args[0]
							newInsts = append(newInsts, &MoveInst{
								Dest: i.Dest,
								Src:  i.Args[0],
							})
							continue
						}
					}
				}
				newInsts = append(newInsts, inst)
			}
			blk.Instructions = newInsts
			if blk.Terminator != nil {
				remapTerminatorValues(blk.Terminator, constMap)
			}
		}
	}
	foldBlocks(fn.Blocks)
}
