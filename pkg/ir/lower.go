package ir

import (
	"fmt"
	"strings"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
	"github.com/jossecurity/joss/pkg/version"
)

// Lowerer es el compilador que transforma un PreparedProgram del analizador semántico en Joss Native IR.
type Lowerer struct {
	prep           *analyzer.PreparedProgram
	reachGraph     *analyzer.ReachabilityGraph
	currentFn      *Function
	currBlock      *BasicBlock
	prog           *Program
	varLocals      map[string]Value // Mapea variables a sus punteros locales (alloca)
	viewDataParam  Value            // Parámetro $__data activo al compilar vistas .joss.html a IR
	loopStack      []loopContext
	tryCatchCount  int
	closureCounter int
	foreachCounter int
	OptLevel       int
}

type loopContext struct {
	continueBlock *BasicBlock
	breakBlock    *BasicBlock
}

func NewLowerer(prep *analyzer.PreparedProgram, reachGraph *analyzer.ReachabilityGraph) *Lowerer {
	return &Lowerer{
		prep:       prep,
		reachGraph: reachGraph,
		varLocals:  make(map[string]Value),
		loopStack:  make([]loopContext, 0),
		OptLevel:   1, // Por defecto habilitar optimizaciones IR canónicas
	}
}

// LowerProgram compila el programa semántico validado a Joss Native IR.
func (l *Lowerer) LowerProgram(name string) (*Program, error) {
	if l.prep == nil {
		return nil, fmt.Errorf("lowerer: PreparedProgram es nil")
	}

	l.prog = NewProgram(name)

	// Procesar unidades fuente
	for _, unit := range l.prep.Units {
		// Si hay reachability graph, validar si el archivo es alcanzable
		if l.reachGraph != nil && !l.reachGraph.IsFileReachable(unit.Path) {
			continue
		}

		if unit.Program == nil {
			continue
		}

		// Primera pasada: recolectar y compilar funciones/métodos declarados a nivel superior
		for _, stmt := range unit.Program.Statements {
			switch s := stmt.(type) {
			case *parser.MethodStatement:
				fnName := s.Name.Value
				if l.reachGraph != nil && !l.reachGraph.IsFunctionReachable(fnName) {
					continue
				}
				if err := l.lowerMethod(s); err != nil {
					return nil, err
				}
			case *parser.InitStatement:
				mName := "main"
				if s.Name != nil && s.Name.Value != "" {
					mName = s.Name.Value
				}
				synthMethod := &parser.MethodStatement{
					Token:      s.Token,
					Name:       &parser.Identifier{Value: mName},
					Parameters: s.Parameters,
					Body:       s.Body,
					ReturnType: parser.Token{Literal: "int"},
				}
				if err := l.lowerMethod(synthMethod); err != nil {
					return nil, err
				}
			case *parser.ClassStatement:
				if s.Body != nil {
					for _, member := range s.Body.Statements {
						if m, ok := member.(*parser.MethodStatement); ok {
							if m.IsAbstract || m.Body == nil {
								continue
							}
							qualifiedName := s.Name.Value + "::" + m.Name.Value
							if l.reachGraph != nil && !l.reachGraph.IsFunctionReachable(qualifiedName) && !l.reachGraph.IsFunctionReachable(m.Name.Value) {
								continue
							}
							if err := l.lowerClassMethod(s.Name.Value, m); err != nil {
								return nil, err
							}
						} else if initStmt, ok := member.(*parser.InitStatement); ok {
							mName := "Init"
							if initStmt.Name != nil && initStmt.Name.Value != "" {
								mName = initStmt.Name.Value
							}
							synthMethod := &parser.MethodStatement{
								Token:      initStmt.Token,
								Name:       &parser.Identifier{Value: mName},
								Parameters: initStmt.Parameters,
								Body:       initStmt.Body,
								ReturnType: parser.Token{Literal: "void"},
							}
							if s.Name.Value == "Main" && (mName == "main" || mName == "Init") {
								if l.prog.Functions["main"] == nil {
									synthMethod.Name.Value = "main"
									synthMethod.ReturnType.Literal = "int"
									if err := l.lowerMethod(synthMethod); err != nil {
										return nil, err
									}
									continue
								}
							}
							if err := l.lowerClassMethod(s.Name.Value, synthMethod); err != nil {
								return nil, err
							}
						}
					}
				}
			case *parser.InterfaceStatement, *parser.EnumStatement:
				// Contratos nominales validados por frontend
				continue
			}
		}
	}

	// Si hay entrypoint principal con statements a nivel superior (script o main), sintetizar main()
	entryUnit := l.prep.Entrypoint()
	if entryUnit != nil {
		hasTopLevelStmts := false
		for _, stmt := range entryUnit.Statements {
			switch stmt.(type) {
			case *parser.MethodStatement, *parser.ClassStatement, *parser.InterfaceStatement, *parser.EnumStatement, *parser.InitStatement:
				continue
			default:
				hasTopLevelStmts = true
			}
			if hasTopLevelStmts {
				break
			}
		}

		if hasTopLevelStmts && l.prog.Functions["main"] == nil {
			if err := l.synthesizeMainFunction(entryUnit.Statements); err != nil {
				return nil, err
			}
		}
	}

	if err := l.lowerProjectViewsAndRoutes(); err != nil {
		return nil, err
	}

	if l.OptLevel > 0 {
		OptimizeProgram(l.prog)
	}

	return l.prog, nil
}

func (l *Lowerer) synthesizeMainFunction(stmts []parser.Statement) error {
	mainFn := NewFunction("main", TypeI64)
	l.currentFn = mainFn
	l.currBlock = mainFn.EntryBlock
	l.varLocals = make(map[string]Value)

	for _, stmt := range stmts {
		switch stmt.(type) {
		case *parser.MethodStatement, *parser.ClassStatement, *parser.InterfaceStatement, *parser.EnumStatement, *parser.InitStatement:
			continue
		}
		if err := l.lowerStatement(stmt); err != nil {
			return err
		}
	}

	// Si el bloque actual no tiene terminador, añadir return 0
	if l.currBlock.Terminator == nil {
		retVal := NewConstInt(0, TypeI64)
		l.currBlock.SetTerminator(&ReturnTerminator{Val: retVal})
	}

	l.prog.AddFunction(mainFn)
	return nil
}

func (l *Lowerer) lowerMethod(stmt *parser.MethodStatement) error {
	retType := l.mapType(stmt.ReturnType.Literal)
	fn := NewFunction(stmt.Name.Value, retType)
	l.currentFn = fn
	l.currBlock = fn.EntryBlock
	l.varLocals = make(map[string]Value)

	// Parámetros formales
	for _, param := range stmt.Parameters {
		paramType := l.mapType(param.Type.Literal)
		paramVal := fn.AddParam(param.Name.Value, paramType)

		// Crear alloca local para el parámetro de modo que sea mutable en el cuerpo
		ptrTemp := fn.NewTemp(PtrType(paramType), param.Name.Value+"_ptr")
		l.currBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: paramType})
		l.currBlock.AddInstruction(&StoreInst{Val: paramVal, Ptr: ptrTemp})
		l.varLocals[cleanVarName(param.Name.Value)] = ptrTemp
	}

	if stmt.Body != nil {
		for _, bodyStmt := range stmt.Body.Statements {
			if err := l.lowerStatement(bodyStmt); err != nil {
				return err
			}
		}
	}

	// Si el bloque de salida no tiene terminador
	if l.currBlock.Terminator == nil {
		if retType.Kind == TypeKindVoid {
			l.currBlock.SetTerminator(&ReturnTerminator{})
		} else if retType.Kind == TypeKindString {
			l.currBlock.SetTerminator(&ReturnTerminator{Val: NewConstString("")})
		} else if retType.Kind == TypeKindPtr || retType.Kind == TypeKindStruct || retType.Kind == TypeKindArray {
			l.currBlock.SetTerminator(&ReturnTerminator{Val: NewConstNull(retType)})
		} else {
			l.currBlock.SetTerminator(&ReturnTerminator{Val: NewConstInt(0, retType)})
		}
	}

	l.prog.AddFunction(fn)
	return nil
}

func (l *Lowerer) lowerClassMethod(className string, stmt *parser.MethodStatement) error {
	mName := className + "_" + stmt.Name.Value
	retType := l.mapType(stmt.ReturnType.Literal)
	fn := NewFunction(mName, retType)
	l.currentFn = fn
	l.currBlock = fn.EntryBlock
	l.varLocals = make(map[string]Value)

	if !stmt.IsStatic {
		thisType := PtrType(StructType(className, nil))
		thisVal := fn.AddParam("this", thisType)
		ptrTemp := fn.NewTemp(PtrType(thisType), "this_ptr")
		l.currBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: thisType})
		l.currBlock.AddInstruction(&StoreInst{Val: thisVal, Ptr: ptrTemp})
		l.varLocals["this"] = ptrTemp
	}

	for _, param := range stmt.Parameters {
		paramType := l.mapType(param.Type.Literal)
		paramVal := fn.AddParam(param.Name.Value, paramType)

		ptrTemp := fn.NewTemp(PtrType(paramType), param.Name.Value+"_ptr")
		l.currBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: paramType})
		l.currBlock.AddInstruction(&StoreInst{Val: paramVal, Ptr: ptrTemp})
		l.varLocals[cleanVarName(param.Name.Value)] = ptrTemp
	}

	if stmt.Body != nil {
		for _, bodyStmt := range stmt.Body.Statements {
			if err := l.lowerStatement(bodyStmt); err != nil {
				return err
			}
		}
	}

	if l.currBlock.Terminator == nil {
		if retType.Kind == TypeKindVoid {
			l.currBlock.SetTerminator(&ReturnTerminator{})
		} else if retType.Kind == TypeKindString {
			l.currBlock.SetTerminator(&ReturnTerminator{Val: NewConstString("")})
		} else if retType.Kind == TypeKindPtr || retType.Kind == TypeKindStruct || retType.Kind == TypeKindArray {
			l.currBlock.SetTerminator(&ReturnTerminator{Val: NewConstNull(retType)})
		} else {
			l.currBlock.SetTerminator(&ReturnTerminator{Val: NewConstInt(0, retType)})
		}
	}

	l.prog.AddFunction(fn)
	return nil
}

func (l *Lowerer) lowerStatement(stmt parser.Statement) error {
	switch s := stmt.(type) {
	case *parser.LetStatement:
		valType := l.mapType(s.Token.Literal)
		var val Value
		if s.Value != nil {
			v, err := l.lowerExpression(s.Value)
			if err != nil {
				return err
			}
			val = v
		}
		if valType.Kind == TypeKindUnknown {
			if s.Value != nil {
				valType = l.inferNodeType(s.Value)
				if val != nil && val.Type().Kind != TypeKindI64 && val.Type().Kind != TypeKindUnknown {
					valType = val.Type()
				}
			} else {
				valType = TypeI64
			}
		}
		if val == nil {
			val = NewConstInt(0, valType)
		}
		ptrTemp := l.currentFn.NewTemp(PtrType(valType), s.Name.Value+"_ptr")
		l.currBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: valType})
		l.currBlock.AddInstruction(&StoreInst{Val: val, Ptr: ptrTemp})
		l.varLocals[cleanVarName(s.Name.Value)] = ptrTemp
		return nil

	case *parser.MultiLetStatement:
		valType := l.mapType(s.TypeToken.Literal)
		for _, decl := range s.Declarations {
			ptrTemp := l.currentFn.NewTemp(PtrType(valType), decl.Name.Value+"_ptr")
			l.currBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: valType})

			var val Value
			if decl.Value != nil {
				v, err := l.lowerExpression(decl.Value)
				if err != nil {
					return err
				}
				val = v
			} else {
				val = NewConstInt(0, valType)
			}
			l.currBlock.AddInstruction(&StoreInst{Val: val, Ptr: ptrTemp})
			l.varLocals[cleanVarName(decl.Name.Value)] = ptrTemp
		}
		return nil

	case *parser.ExpressionStatement:
		if l.OptLevel > 0 {
			if ternary, ok := s.Expression.(*parser.TernaryExpression); ok {
				return l.lowerTernaryStatement(ternary)
			}
		}
		_, err := l.lowerExpression(s.Expression)
		return err

	case *parser.ReturnStatement:
		var retVal Value
		if s.ReturnValue != nil {
			v, err := l.lowerExpression(s.ReturnValue)
			if err != nil {
				return err
			}
			retVal = v
		}
		if l.currentFn.ReturnType.Kind == TypeKindVoid {
			retVal = nil
		} else if retVal == nil || retVal.Type().Kind == TypeKindVoid {
			if l.currentFn.ReturnType.Kind == TypeKindString {
				retVal = NewConstString("")
			} else if l.currentFn.ReturnType.Kind == TypeKindPtr || l.currentFn.ReturnType.Kind == TypeKindStruct || l.currentFn.ReturnType.Kind == TypeKindArray {
				retVal = NewConstNull(l.currentFn.ReturnType)
			} else {
				retVal = NewConstInt(0, l.currentFn.ReturnType)
			}
		}
		l.currBlock.SetTerminator(&ReturnTerminator{Val: retVal})
		// Crear un bloque inalcanzable temporal para sentencias muertas posteriores
		unreachBlock := l.currentFn.NewBlock("dead")
		l.currBlock = unreachBlock
		return nil

	case *parser.GuardStatement:
		// guard ($cond) else { ... }
		condVal, err := l.lowerExpression(s.Condition)
		if err != nil {
			return err
		}
		condVal = l.ensureBoolCond(condVal)

		guardPassBlock := l.currentFn.NewBlock("guard_pass")
		guardFailBlock := l.currentFn.NewBlock("guard_fail")

		// Si la condición es verdadera, pasa al guardPassBlock; si es falsa, ejecuta el bloque else
		l.currBlock.SetTerminator(&BranchTerminator{
			Cond:       condVal,
			TrueBlock:  guardPassBlock,
			FalseBlock: guardFailBlock,
		})

		// Guard fail block (debe contener return, break, throw, etc.)
		l.currBlock = guardFailBlock
		if err := l.lowerBlock(s.Body); err != nil {
			return err
		}
		if l.currBlock.Terminator == nil {
			l.currBlock.SetTerminator(&UnreachableTerminator{})
		}

		// Continuar en guardPassBlock
		l.currBlock = guardPassBlock
		return nil

	case *parser.WhileStatement:
		condBlock := l.currentFn.NewBlock("while_cond")
		bodyBlock := l.currentFn.NewBlock("while_body")
		afterBlock := l.currentFn.NewBlock("while_exit")

		l.currBlock.SetTerminator(&JumpTerminator{Target: condBlock})

		// Evaluar condición
		l.currBlock = condBlock
		condVal, err := l.lowerExpression(s.Condition)
		if err != nil {
			return err
		}
		condVal = l.ensureBoolCond(condVal)
		condBlock.SetTerminator(&BranchTerminator{
			Cond:       condVal,
			TrueBlock:  bodyBlock,
			FalseBlock: afterBlock,
		})

		// Cuerpo
		l.loopStack = append(l.loopStack, loopContext{continueBlock: condBlock, breakBlock: afterBlock})
		l.currBlock = bodyBlock
		if err := l.lowerBlock(s.Body); err != nil {
			return err
		}
		l.loopStack = l.loopStack[:len(l.loopStack)-1]

		if l.currBlock.Terminator == nil {
			l.currBlock.SetTerminator(&JumpTerminator{Target: condBlock})
		}

		l.currBlock = afterBlock
		return nil

	case *parser.DoWhileStatement:
		bodyBlock := l.currentFn.NewBlock("dowhile_body")
		condBlock := l.currentFn.NewBlock("dowhile_cond")
		afterBlock := l.currentFn.NewBlock("dowhile_exit")

		l.currBlock.SetTerminator(&JumpTerminator{Target: bodyBlock})

		l.loopStack = append(l.loopStack, loopContext{continueBlock: condBlock, breakBlock: afterBlock})
		l.currBlock = bodyBlock
		if err := l.lowerBlock(s.Body); err != nil {
			return err
		}
		l.loopStack = l.loopStack[:len(l.loopStack)-1]

		if l.currBlock.Terminator == nil {
			l.currBlock.SetTerminator(&JumpTerminator{Target: condBlock})
		}

		// Evaluar condición
		l.currBlock = condBlock
		condVal, err := l.lowerExpression(s.Condition)
		if err != nil {
			return err
		}
		condVal = l.ensureBoolCond(condVal)
		condBlock.SetTerminator(&BranchTerminator{
			Cond:       condVal,
			TrueBlock:  bodyBlock,
			FalseBlock: afterBlock,
		})

		l.currBlock = afterBlock
		return nil

	case *parser.ForeachStatement:
		return l.lowerForeach(s)

	case *parser.DestructureStatement:
		rhsVal, err := l.lowerExpression(s.Value)
		if err != nil {
			return err
		}
		for idx, ident := range s.Names {
			if ident == nil {
				continue
			}
			vName := cleanVarName(ident.Value)
			ptr, exists := l.varLocals[vName]
			if !exists {
				newPtr := l.currentFn.NewTemp(PtrType(TypeI64), vName+"_ptr")
				l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: newPtr, AllocType: TypeI64})
				l.varLocals[vName] = newPtr
				ptr = newPtr
			}
			elemTemp := l.currentFn.NewTemp(TypeI64, vName+"_elem")
			l.currBlock.AddInstruction(&ArrayGetInst{
				Dest:  elemTemp,
				Array: rhsVal,
				Index: NewConstInt(int64(idx), TypeI64),
			})
			l.currBlock.AddInstruction(&StoreInst{Val: elemTemp, Ptr: ptr})
		}
		return nil

	case *parser.BreakStatement:
		if len(l.loopStack) == 0 {
			return fmt.Errorf("lowerer: 'break' fuera de bucle")
		}
		target := l.loopStack[len(l.loopStack)-1].breakBlock
		l.currBlock.SetTerminator(&JumpTerminator{Target: target})
		deadBlock := l.currentFn.NewBlock("dead")
		l.currBlock = deadBlock
		return nil

	case *parser.ContinueStatement:
		if len(l.loopStack) == 0 {
			return fmt.Errorf("lowerer: 'continue' fuera de bucle")
		}
		target := l.loopStack[len(l.loopStack)-1].continueBlock
		l.currBlock.SetTerminator(&JumpTerminator{Target: target})
		deadBlock := l.currentFn.NewBlock("dead")
		l.currBlock = deadBlock
		return nil

	case *parser.EchoStatement:
		val, err := l.lowerExpression(s.Value)
		if err != nil {
			return err
		}
		l.prog.RequireRuntime("print")
		if val.Type().Kind == TypeKindString {
			l.currBlock.AddInstruction(&CallRuntimeInst{Func: "print_string", Args: []Value{val}})
		} else if val.Type().Kind == TypeKindBool {
			l.currBlock.AddInstruction(&CallRuntimeInst{Func: "print_bool", Args: []Value{val}})
		} else {
			l.currBlock.AddInstruction(&CallRuntimeInst{Func: "print_i64", Args: []Value{val}})
		}
		return nil

	case *parser.ClassStatement, *parser.InterfaceStatement, *parser.EnumStatement, *parser.InitStatement:
		return nil

	case *parser.ThrowStatement:
		var throwVal Value
		if s.Value != nil {
			v, err := l.lowerExpression(s.Value)
			if err != nil {
				return err
			}
			throwVal = v
		} else {
			throwVal = NewConstString("exception")
		}
		l.currBlock.AddInstruction(&ThrowInst{Val: throwVal})
		l.currBlock.SetTerminator(&UnreachableTerminator{})
		deadBlock := l.currentFn.NewBlock("dead")
		l.currBlock = deadBlock
		return nil

	case *parser.TryCatchStatement:
		return l.lowerTryCatch(s)

	case *parser.BlockStatement:
		return l.lowerBlock(s)

	default:
		return &UnsupportedCapabilityError{
			Feature: describeUnsupportedStatement(stmt),
		}
	}
}

func (l *Lowerer) ensureBoolCond(val Value) Value {
	if val == nil {
		return NewConstBool(false)
	}
	if val.Type().Kind == TypeKindBool {
		return val
	}
	dest := l.currentFn.NewTemp(TypeBool, "truthy")
	l.currBlock.AddInstruction(&CallRuntimeInst{
		Dest: dest,
		Func: "is_truthy",
		Args: []Value{val},
	})
	return dest
}

func (l *Lowerer) lowerForeach(s *parser.ForeachStatement) error {
	iterVal, err := l.lowerExpression(s.Iterable)
	if err != nil {
		return err
	}

	feID := l.foreachCounter
	l.foreachCounter++

	isMap := iterVal.Type().Kind == TypeKindPtr && iterVal.Type().Elem != nil && iterVal.Type().Elem.Name == "map"

	idxPtr := l.currentFn.NewTemp(PtrType(TypeI64), fmt.Sprintf("fe_idx_%d_ptr", feID))
	l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: idxPtr, AllocType: TypeI64})
	l.currBlock.AddInstruction(&StoreInst{Val: NewConstInt(0, TypeI64), Ptr: idxPtr})

	lenTemp := l.currentFn.NewTemp(TypeI64, fmt.Sprintf("fe_len_%d", feID))
	lenCallee := "len"
	if isMap {
		lenCallee = "joss_map_len"
	}
	l.currBlock.AddInstruction(&CallInst{
		Dest:   lenTemp,
		Callee: lenCallee,
		Args:   []Value{iterVal},
	})

	elemType := TypeI64
	if iterVal.Type().Kind == TypeKindArray && iterVal.Type().Elem != nil {
		elemType = *iterVal.Type().Elem
	} else if isMap {
		elemType = PtrType(StructType("map", nil))
	}

	valName := cleanVarName(s.Value)
	valPtr, ok := l.varLocals[valName]
	if !ok {
		newValPtr := l.currentFn.NewTemp(PtrType(elemType), valName+"_ptr")
		l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: newValPtr, AllocType: elemType})
		l.varLocals[valName] = newValPtr
		valPtr = newValPtr
	}

	var keyPtr Value
	keyType := TypeI64
	if isMap {
		keyType = TypeString
	}
	if s.Key != "" {
		keyName := cleanVarName(s.Key)
		kp, exists := l.varLocals[keyName]
		if !exists {
			newKeyPtr := l.currentFn.NewTemp(PtrType(keyType), keyName+"_ptr")
			l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: newKeyPtr, AllocType: keyType})
			l.varLocals[keyName] = newKeyPtr
			kp = newKeyPtr
		}
		keyPtr = kp
	}

	condBlock := l.currentFn.NewBlock(fmt.Sprintf("foreach_cond_%d", feID))
	bodyBlock := l.currentFn.NewBlock(fmt.Sprintf("foreach_body_%d", feID))
	stepBlock := l.currentFn.NewBlock(fmt.Sprintf("foreach_step_%d", feID))
	afterBlock := l.currentFn.NewBlock(fmt.Sprintf("foreach_exit_%d", feID))

	l.currBlock.SetTerminator(&JumpTerminator{Target: condBlock})

	// Condición: idx < len
	l.currBlock = condBlock
	curIdx := l.currentFn.NewTemp(TypeI64, fmt.Sprintf("fe_idx_%d", feID))
	l.currBlock.AddInstruction(&LoadInst{Dest: curIdx, Ptr: idxPtr})
	cmpTemp := l.currentFn.NewTemp(TypeBool, fmt.Sprintf("fe_cmp_%d", feID))
	l.currBlock.AddInstruction(&CompareInst{
		Op:    OpCmpLt,
		Dest:  cmpTemp,
		Left:  curIdx,
		Right: lenTemp,
	})
	condBlock.SetTerminator(&BranchTerminator{
		Cond:       cmpTemp,
		TrueBlock:  bodyBlock,
		FalseBlock: afterBlock,
	})

	// Cuerpo
	l.currBlock = bodyBlock
	bodyIdx := l.currentFn.NewTemp(TypeI64, fmt.Sprintf("fe_bidx_%d", feID))
	l.currBlock.AddInstruction(&LoadInst{Dest: bodyIdx, Ptr: idxPtr})
	if keyPtr != nil {
		if isMap {
			kVal := l.currentFn.NewTemp(TypeString, fmt.Sprintf("fe_mkey_%d", feID))
			l.currBlock.AddInstruction(&CallInst{
				Dest:   kVal,
				Callee: "joss_map_key_at",
				Args:   []Value{iterVal, bodyIdx},
			})
			l.currBlock.AddInstruction(&StoreInst{Val: kVal, Ptr: keyPtr})
		} else {
			l.currBlock.AddInstruction(&StoreInst{Val: bodyIdx, Ptr: keyPtr})
		}
	}
	elemVal := l.currentFn.NewTemp(elemType, fmt.Sprintf("fe_elem_%d", feID))
	if isMap {
		l.currBlock.AddInstruction(&CallInst{
			Dest:   elemVal,
			Callee: "joss_map_val_at",
			Args:   []Value{iterVal, bodyIdx},
		})
	} else {
		l.currBlock.AddInstruction(&ArrayGetInst{
			Dest:  elemVal,
			Array: iterVal,
			Index: bodyIdx,
		})
	}
	l.currBlock.AddInstruction(&StoreInst{Val: elemVal, Ptr: valPtr})

	l.loopStack = append(l.loopStack, loopContext{continueBlock: stepBlock, breakBlock: afterBlock})
	if err := l.lowerBlock(s.Body); err != nil {
		return err
	}
	l.loopStack = l.loopStack[:len(l.loopStack)-1]

	if l.currBlock.Terminator == nil {
		l.currBlock.SetTerminator(&JumpTerminator{Target: stepBlock})
	}

	// Step: idx = idx + 1
	l.currBlock = stepBlock
	stepIdx := l.currentFn.NewTemp(TypeI64, fmt.Sprintf("fe_sidx_%d", feID))
	l.currBlock.AddInstruction(&LoadInst{Dest: stepIdx, Ptr: idxPtr})
	nextIdx := l.currentFn.NewTemp(TypeI64, fmt.Sprintf("fe_nidx_%d", feID))
	l.currBlock.AddInstruction(&BinaryInst{
		Op:    OpAdd,
		Dest:  nextIdx,
		Left:  stepIdx,
		Right: NewConstInt(1, TypeI64),
	})
	l.currBlock.AddInstruction(&StoreInst{Val: nextIdx, Ptr: idxPtr})
	l.currBlock.SetTerminator(&JumpTerminator{Target: condBlock})

	l.currBlock = afterBlock
	return nil
}

func (l *Lowerer) lowerTryCatch(s *parser.TryCatchStatement) error {
	tcID := l.tryCatchCount
	l.tryCatchCount++

	savedBlock := l.currBlock

	// 1. Lower try block
	startTryIdx := len(l.currentFn.Blocks)
	tryBlock := l.currentFn.NewBlock(fmt.Sprintf("try_%d", tcID))
	l.currBlock = tryBlock

	if s.TryBlock != nil {
		for _, stmt := range s.TryBlock.Statements {
			if err := l.lowerStatement(stmt); err != nil {
				return err
			}
		}
	}
	tryBlocks := append([]*BasicBlock(nil), l.currentFn.Blocks[startTryIdx:]...)
	l.currentFn.Blocks = l.currentFn.Blocks[:startTryIdx]

	// 2. Setup catch variable
	catchVarName := cleanVarName(s.CatchVar)
	if catchVarName == "" {
		catchVarName = "e"
	}
	catchType := TypeString
	ptrTemp := l.currentFn.NewTemp(PtrType(catchType), catchVarName+"_ptr")
	l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: catchType})
	l.varLocals[catchVarName] = ptrTemp

	// 3. Lower catch block
	startCatchIdx := len(l.currentFn.Blocks)
	catchBlock := l.currentFn.NewBlock(fmt.Sprintf("catch_%d", tcID))
	l.currBlock = catchBlock

	if s.CatchBlock != nil {
		for _, stmt := range s.CatchBlock.Statements {
			if err := l.lowerStatement(stmt); err != nil {
				return err
			}
		}
	}
	catchBlocks := append([]*BasicBlock(nil), l.currentFn.Blocks[startCatchIdx:]...)
	l.currentFn.Blocks = l.currentFn.Blocks[:startCatchIdx]

	// 4. Restore current block and add TryCatchInst
	l.currBlock = savedBlock
	l.currBlock.AddInstruction(&TryCatchInst{
		ID:          tcID,
		TryBlocks:   tryBlocks,
		CatchVar:    catchVarName,
		CatchVarPtr: ptrTemp,
		CatchBlocks: catchBlocks,
	})

	// Check if both paths terminated with returns
	tryTerminates := false
	if len(tryBlocks) > 0 && tryBlocks[len(tryBlocks)-1].Terminator != nil {
		if _, ok := tryBlocks[len(tryBlocks)-1].Terminator.(*ReturnTerminator); ok {
			tryTerminates = true
		}
	}
	catchTerminates := false
	if len(catchBlocks) > 0 && catchBlocks[len(catchBlocks)-1].Terminator != nil {
		if _, ok := catchBlocks[len(catchBlocks)-1].Terminator.(*ReturnTerminator); ok {
			catchTerminates = true
		}
	}

	if tryTerminates && catchTerminates {
		l.currBlock.SetTerminator(&UnreachableTerminator{})
		deadBlock := l.currentFn.NewBlock("dead")
		l.currBlock = deadBlock
	}

	return nil
}

func (l *Lowerer) lowerBlock(block *parser.BlockStatement) error {
	if block == nil {
		return nil
	}
	for _, stmt := range block.Statements {
		if err := l.lowerStatement(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lowerer) lowerTernaryStatement(e *parser.TernaryExpression) error {
	condVal, err := l.lowerExpression(e.Condition)
	if err != nil {
		return err
	}
	condVal = l.ensureBoolCond(condVal)

	trueBlock := l.currentFn.NewBlock("ternary_true")
	mergeBlock := l.currentFn.NewBlock("ternary_merge")

	isNullFalse := false
	if e.False == nil {
		isNullFalse = true
	} else if _, ok := e.False.(*parser.NullLiteral); ok {
		isNullFalse = true
	}

	if isNullFalse {
		l.currBlock.SetTerminator(&BranchTerminator{
			Cond:       condVal,
			TrueBlock:  trueBlock,
			FalseBlock: mergeBlock,
		})

		l.currBlock = trueBlock
		if e.True != nil {
			if _, err := l.lowerExpression(e.True); err != nil {
				return err
			}
		}
		if l.currBlock.Terminator == nil {
			l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
		}
		l.currBlock = mergeBlock
		return nil
	}

	falseBlock := l.currentFn.NewBlock("ternary_false")
	l.currBlock.SetTerminator(&BranchTerminator{
		Cond:       condVal,
		TrueBlock:  trueBlock,
		FalseBlock: falseBlock,
	})

	l.currBlock = trueBlock
	if e.True != nil {
		if _, err := l.lowerExpression(e.True); err != nil {
			return err
		}
	}
	if l.currBlock.Terminator == nil {
		l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
	}

	l.currBlock = falseBlock
	if e.False != nil {
		if _, err := l.lowerExpression(e.False); err != nil {
			return err
		}
	}
	if l.currBlock.Terminator == nil {
		l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
	}

	l.currBlock = mergeBlock
	return nil
}

func (l *Lowerer) lowerExpression(expr parser.Expression) (Value, error) {
	if expr == nil {
		return NewConstInt(0, TypeI64), nil
	}
	switch e := expr.(type) {
	case *parser.IntegerLiteral:
		return NewConstInt(e.Value, TypeI64), nil

	case *parser.FloatLiteral:
		return NewConstFloat(e.Value, TypeF64), nil

	case *parser.DecimalLiteral:
		return NewConstInt(e.Value.IntPart(), TypeI64), nil

	case *parser.Boolean:
		return NewConstBool(e.Value), nil

	case *parser.StringLiteral:
		return NewConstString(e.Value), nil

	case *parser.NullLiteral:
		return NewConstNull(PtrType(TypeVoid)), nil

	case *parser.Identifier:
		varName := cleanVarName(e.Value)
		if varName == "JOSS_VERSION" {
			return NewConstString(version.Version), nil
		}
		if ptr, ok := l.varLocals[varName]; ok {
			dest := l.currentFn.NewTemp(ptr.Type().ElemType(), varName)
			l.currBlock.AddInstruction(&LoadInst{Dest: dest, Ptr: ptr})
			return dest, nil
		}
		if l.viewDataParam != nil {
			resType := TypeString
			switch varName {
			case "total", "current_page", "last_page", "total_pages", "docs_count", "packages_count", "posts_count", "products_count", "is_active_value":
				resType = TypeI64
			}
			dest := l.currentFn.NewTemp(resType, "vdata_"+varName)
			l.currBlock.AddInstruction(&MapGetInst{
				Dest: dest,
				Map:  l.viewDataParam,
				Key:  NewConstString(varName),
			})
			return dest, nil
		}
		// Si es un identificador no declarado localmente (p.ej. constante global, clase o captura implícita),
		// reservar un slot local inicializado en 0 en el EntryBlock para mantener la validez SSA.
		inferred := l.inferNodeType(e)
		if inferred.Kind == TypeKindUnknown || inferred.Kind == TypeKindVoid {
			inferred = TypeI64
		}
		ptr := l.currentFn.NewTemp(PtrType(inferred), varName+"_ptr")
		l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: ptr, AllocType: inferred})
		l.varLocals[varName] = ptr
		dest := l.currentFn.NewTemp(inferred, varName)
		l.currBlock.AddInstruction(&LoadInst{Dest: dest, Ptr: ptr})
		return dest, nil

	case *parser.InfixExpression:
		if e.Operator == "??" {
			left, err := l.lowerExpression(e.Left)
			if err != nil {
				return nil, err
			}
			cond := l.ensureBoolCond(left)
			trueBlock := l.currentFn.NewBlock("coal_keep")
			falseBlock := l.currentFn.NewBlock("coal_fallback")
			mergeBlock := l.currentFn.NewBlock("coal_merge")

			l.currBlock.SetTerminator(&BranchTerminator{
				Cond:       cond,
				TrueBlock:  trueBlock,
				FalseBlock: falseBlock,
			})

			l.currBlock = trueBlock
			resType := left.Type()
			resPtr := l.currentFn.NewTemp(PtrType(resType), "coal_ptr")
			l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: resPtr, AllocType: resType})
			l.currBlock.AddInstruction(&StoreInst{Val: left, Ptr: resPtr})
			l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})

			l.currBlock = falseBlock
			right, err := l.lowerExpression(e.Right)
			if err != nil {
				return nil, err
			}
			if l.currBlock.Terminator == nil {
				l.currBlock.AddInstruction(&StoreInst{Val: right, Ptr: resPtr})
				l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
			}

			l.currBlock = mergeBlock
			resTemp := l.currentFn.NewTemp(resType, "coal_res")
			l.currBlock.AddInstruction(&LoadInst{Dest: resTemp, Ptr: resPtr})
			return resTemp, nil
		}

		left, err := l.lowerExpression(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := l.lowerExpression(e.Right)
		if err != nil {
			return nil, err
		}

		if e.Operator == "." || (e.Operator == "+" && (left.Type().Kind == TypeKindString || right.Type().Kind == TypeKindString)) {
			l.prog.RequireRuntime("str_concat")
			if left.Type().Kind != TypeKindString {
				convLeft := l.currentFn.NewTemp(TypeString, "tostr_l")
				l.currBlock.AddInstruction(&CallRuntimeInst{
					Dest: convLeft,
					Func: "to_string",
					Args: []Value{left},
				})
				left = convLeft
			}
			if right.Type().Kind != TypeKindString {
				convRight := l.currentFn.NewTemp(TypeString, "tostr_r")
				l.currBlock.AddInstruction(&CallRuntimeInst{
					Dest: convRight,
					Func: "to_string",
					Args: []Value{right},
				})
				right = convRight
			}
			dest := l.currentFn.NewTemp(TypeString, "strcat")
			l.currBlock.AddInstruction(&CallRuntimeInst{
				Dest: dest,
				Func: "str_concat",
				Args: []Value{left, right},
			})
			return dest, nil
		}

		op, isCmp := mapInfixOp(e.Operator)
		if isCmp {
			destType := TypeBool
			if op == OpSpaceship {
				destType = TypeI64
			}
			dest := l.currentFn.NewTemp(destType, "cmp")
			l.currBlock.AddInstruction(&CompareInst{
				Op:    op,
				Dest:  dest,
				Left:  left,
				Right: right,
			})
			return dest, nil
		} else {
			destType := left.Type()
			if op == OpAnd || op == OpOr {
				if left.Type().Kind == TypeKindBool || right.Type().Kind == TypeKindBool {
					destType = TypeBool
				}
			}
			dest := l.currentFn.NewTemp(destType, "binop")
			l.currBlock.AddInstruction(&BinaryInst{
				Op:    op,
				Dest:  dest,
				Left:  left,
				Right: right,
			})
			return dest, nil
		}

	case *parser.PrefixExpression:
		right, err := l.lowerExpression(e.Right)
		if err != nil {
			return nil, err
		}
		switch e.Operator {
		case "!":
			dest := l.currentFn.NewTemp(TypeBool, "not")
			l.currBlock.AddInstruction(&UnaryInst{Op: OpNot, Dest: dest, Val: right})
			return dest, nil
		case "-":
			dest := l.currentFn.NewTemp(right.Type(), "neg")
			l.currBlock.AddInstruction(&UnaryInst{Op: OpNeg, Dest: dest, Val: right})
			return dest, nil
		case "+":
			return right, nil
		case "~":
			dest := l.currentFn.NewTemp(TypeI64, "bnot")
			l.currBlock.AddInstruction(&BinaryInst{Op: OpXor, Dest: dest, Left: right, Right: NewConstInt(-1, TypeI64)})
			return dest, nil
		case "++", "--":
			op := OpAdd
			if e.Operator == "--" {
				op = OpSub
			}
			dest := l.currentFn.NewTemp(TypeI64, "preinc")
			l.currBlock.AddInstruction(&BinaryInst{Op: op, Dest: dest, Left: right, Right: NewConstInt(1, TypeI64)})
			if ident, ok := e.Right.(*parser.Identifier); ok {
				if ptr, exists := l.varLocals[cleanVarName(ident.Value)]; exists {
					l.currBlock.AddInstruction(&StoreInst{Val: dest, Ptr: ptr})
				}
			}
			return dest, nil
		default:
			return right, nil
		}

	case *parser.PostfixExpression:
		origVal, err := l.lowerExpression(e.Left)
		if err != nil {
			return nil, err
		}
		op := OpAdd
		if e.Operator == "--" {
			op = OpSub
		}
		newVal := l.currentFn.NewTemp(TypeI64, "postinc")
		l.currBlock.AddInstruction(&BinaryInst{
			Op:    op,
			Dest:  newVal,
			Left:  origVal,
			Right: NewConstInt(1, TypeI64),
		})
		if ident, ok := e.Left.(*parser.Identifier); ok {
			varName := cleanVarName(ident.Value)
			if ptr, exists := l.varLocals[varName]; exists {
				l.currBlock.AddInstruction(&StoreInst{Val: newVal, Ptr: ptr})
			}
		} else if member, ok := e.Left.(*parser.MemberExpression); ok {
			if objVal, err := l.lowerExpression(member.Left); err == nil {
				l.currBlock.AddInstruction(&StoreFieldInst{
					Val:       newVal,
					Obj:       objVal,
					FieldName: member.Property.Value,
				})
			}
		}
		return origVal, nil

	case *parser.BlockExpression:
		if err := l.lowerBlock(e.Block); err != nil {
			return nil, err
		}
		return NewConstInt(0, TypeI64), nil

	case *parser.FunctionLiteral:
		return l.lowerFunctionLiteral(e)

	case *parser.IssetExpression:
		if len(e.Arguments) == 0 {
			return NewConstBool(false), nil
		}
		var combined Value
		for _, arg := range e.Arguments {
			val, err := l.lowerExpression(arg)
			if err != nil {
				return nil, err
			}
			cmp := l.ensureBoolCond(val)
			if combined == nil {
				combined = cmp
			} else {
				andTemp := l.currentFn.NewTemp(TypeBool, "isset_and")
				l.currBlock.AddInstruction(&BinaryInst{
					Op:    OpAnd,
					Dest:  andTemp,
					Left:  combined,
					Right: cmp,
				})
				combined = andTemp
			}
		}
		return combined, nil

	case *parser.EmptyExpression:
		val, err := l.lowerExpression(e.Argument)
		if err != nil {
			return nil, err
		}
		dest := l.currentFn.NewTemp(TypeBool, "is_empty")
		l.currBlock.AddInstruction(&CallRuntimeInst{
			Dest: dest,
			Func: "is_empty",
			Args: []Value{val},
		})
		return dest, nil

	case *parser.IsExpression:
		val, err := l.lowerExpression(e.Left)
		if err != nil {
			return nil, err
		}
		return l.ensureBoolCond(val), nil

	case *parser.MatchExpression:
		return l.lowerMatchExpression(e)

	case *parser.SpreadExpression:
		return l.lowerExpression(e.Expression)

	case *parser.NamedArgument:
		return l.lowerExpression(e.Value)

	case *parser.ReferenceExpression:
		return l.lowerExpression(e.Target)

	case *parser.TernaryExpression:
		condVal, err := l.lowerExpression(e.Condition)
		if err != nil {
			return nil, err
		}
		condVal = l.ensureBoolCond(condVal)

		trueBlock := l.currentFn.NewBlock("ternary_true")
		falseBlock := l.currentFn.NewBlock("ternary_false")
		mergeBlock := l.currentFn.NewBlock("ternary_merge")

		l.currBlock.SetTerminator(&BranchTerminator{
			Cond:       condVal,
			TrueBlock:  trueBlock,
			FalseBlock: falseBlock,
		})

		// Rama verdadera
		l.currBlock = trueBlock
		var trueVal Value = condVal
		if e.True != nil {
			tv, err := l.lowerExpression(e.True)
			if err != nil {
				return nil, err
			}
			trueVal = tv
		}
		resType := trueVal.Type()
		resTemp := l.currentFn.NewTemp(resType, "ternary_res")
		resPtr := l.currentFn.NewTemp(PtrType(resType), "ternary_ptr")
		l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: resPtr, AllocType: resType})
		if l.currBlock.Terminator == nil {
			l.currBlock.AddInstruction(&StoreInst{Val: trueVal, Ptr: resPtr})
			l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
		}

		// Rama falsa
		l.currBlock = falseBlock
		var falseVal Value = NewConstInt(0, resType)
		if resType.Kind == TypeKindString {
			falseVal = NewConstString("")
		}
		if e.False != nil {
			fv, err := l.lowerExpression(e.False)
			if err != nil {
				return nil, err
			}
			falseVal = fv
		}
		if l.currBlock.Terminator == nil {
			l.currBlock.AddInstruction(&StoreInst{Val: falseVal, Ptr: resPtr})
			l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
		}

		// Merge
		l.currBlock = mergeBlock
		l.currBlock.AddInstruction(&LoadInst{Dest: resTemp, Ptr: resPtr})
		return resTemp, nil

	case *parser.NewExpression:
		className := e.Class.Value
		objType := PtrType(StructType(className, nil))
		objTemp := l.currentFn.NewTemp(objType, "obj")
		l.currBlock.AddInstruction(&NewObjectInst{
			Dest:      objTemp,
			ClassName: className,
		})

		// Inicializar propiedades por defecto de la clase y detectar constructor
		hasCtor := false
		ctorName := ""
		if l.prog.Functions[className+"_Init"] != nil {
			hasCtor = true
			ctorName = className + "_Init"
		} else if l.prog.Functions[className+"_constructor"] != nil {
			hasCtor = true
			ctorName = className + "_constructor"
		} else if className == "Exception" {
			hasCtor = true
			ctorName = "Exception_constructor"
		}

		if l.prep != nil {
			for _, unit := range l.prep.Units {
				if unit.Program == nil {
					continue
				}
				for _, stmt := range unit.Program.Statements {
					if cs, ok := stmt.(*parser.ClassStatement); ok && cs.Name.Value == className && cs.Body != nil {
						for _, member := range cs.Body.Statements {
							if letStmt, ok := member.(*parser.LetStatement); ok && letStmt.Name != nil {
								fName := cleanVarName(letStmt.Name.Value)
								if letStmt.Value != nil {
									val, err := l.lowerExpression(letStmt.Value)
									if err == nil {
										l.currBlock.AddInstruction(&StoreFieldInst{
											Val:       val,
											Obj:       objTemp,
											FieldName: fName,
										})
									}
								}
							} else if _, isInit := member.(*parser.InitStatement); isInit && !hasCtor {
								hasCtor = true
								ctorName = className + "_Init"
							} else if mStmt, isM := member.(*parser.MethodStatement); isM && !hasCtor {
								if mStmt.Name.Value == "Init" || mStmt.Name.Value == "constructor" {
									hasCtor = true
									ctorName = className + "_" + mStmt.Name.Value
								}
							}
						}
					}
				}
			}
		}

		if hasCtor {
			ctorArgs := []Value{objTemp}
			for _, arg := range e.Arguments {
				v, err := l.lowerExpression(arg)
				if err != nil {
					return nil, err
				}
				ctorArgs = append(ctorArgs, v)
			}
			l.currBlock.AddInstruction(&CallInst{
				Dest:   nil,
				Callee: ctorName,
				Args:   ctorArgs,
			})
		}

		return objTemp, nil

	case *parser.MemberExpression:
		if identLeft, ok := e.Left.(*parser.Identifier); ok && !strings.HasPrefix(identLeft.Value, "$") {
			if _, isLocal := l.varLocals[cleanVarName(identLeft.Value)]; !isLocal {
				// Acceso estático a constante de clase o caso de enum: Class::CONST / Enum::CASE
				return NewConstString(identLeft.Value + "::" + e.Property.Value), nil
			}
		}
		objVal, err := l.lowerExpression(e.Left)
		if err != nil {
			return nil, err
		}
		propName := e.Property.Value
		fieldType := l.inferNodeType(e)
		if fieldType.Kind == TypeKindI64 && objVal.Type().Kind == TypeKindPtr && objVal.Type().Elem != nil {
			if resolvedPropType, ok := l.lookupClassPropertyType(objVal.Type().Elem.Name, propName); ok {
				fieldType = resolvedPropType
			}
		}
		resTemp := l.currentFn.NewTemp(fieldType, "prop")
		l.currBlock.AddInstruction(&LoadFieldInst{
			Dest:      resTemp,
			Obj:       objVal,
			FieldName: propName,
		})
		return resTemp, nil

	case *parser.ArrayLiteral:
		elemType := TypeI64
		elements := make([]Value, len(e.Elements))
		for idx, el := range e.Elements {
			v, err := l.lowerExpression(el)
			if err != nil {
				return nil, err
			}
			elements[idx] = v
			if idx == 0 || v.Type().Kind != TypeKindI64 {
				elemType = v.Type()
			}
		}
		arrTemp := l.currentFn.NewTemp(ArrayType(elemType), "arr")
		l.currBlock.AddInstruction(&NewArrayInst{
			Dest:     arrTemp,
			ElemType: elemType,
			Elements: elements,
		})
		return arrTemp, nil

	case *parser.MapLiteral:
		pairs := make([][2]Value, 0, len(e.Pairs))
		valType := TypeI64
		for k, v := range e.Pairs {
			kVal, err := l.lowerExpression(k)
			if err != nil {
				return nil, err
			}
			vVal, err := l.lowerExpression(v)
			if err != nil {
				return nil, err
			}
			if len(pairs) == 0 || vVal.Type().Kind != TypeKindI64 {
				valType = vVal.Type()
			}
			pairs = append(pairs, [2]Value{kVal, vVal})
		}
		mapTemp := l.currentFn.NewTemp(PtrType(StructType("map", []Type{valType})), "map")
		l.currBlock.AddInstruction(&NewMapInst{
			Dest:  mapTemp,
			Pairs: pairs,
		})
		return mapTemp, nil

	case *parser.IndexExpression:
		container, err := l.lowerExpression(e.Left)
		if err != nil {
			return nil, err
		}
		idxVal, err := l.lowerExpression(e.Index)
		if err != nil {
			return nil, err
		}
		resType := l.inferNodeType(e)
		isMapContainer := container.Type().Kind == TypeKindPtr && container.Type().Elem != nil && container.Type().Elem.Name == "map"
		if idxVal.Type().Kind == TypeKindString || isMapContainer {
			if resType.Kind == TypeKindI64 {
				if isMapContainer && len(container.Type().Elem.Fields) > 0 {
					resType = container.Type().Elem.Fields[0]
				}
			}
			resTemp := l.currentFn.NewTemp(resType, "elem")
			l.currBlock.AddInstruction(&MapGetInst{
				Dest: resTemp,
				Map:  container,
				Key:  idxVal,
			})
			return resTemp, nil
		}
		if resType.Kind == TypeKindI64 && container.Type().Kind == TypeKindArray && container.Type().Elem != nil {
			resType = *container.Type().Elem
		}
		resTemp := l.currentFn.NewTemp(resType, "elem")
		l.currBlock.AddInstruction(&ArrayGetInst{
			Dest:  resTemp,
			Array: container,
			Index: idxVal,
		})
		return resTemp, nil

	case *parser.CallExpression:
		calleeName := ""
		var receiverObj Value

		if member, ok := e.Function.(*parser.MemberExpression); ok {
			if identLeft, ok := member.Left.(*parser.Identifier); ok && member.Property != nil {
				// Podría ser estático Class::method o $obj->method
				if ptr, ok := l.varLocals[cleanVarName(identLeft.Value)]; ok {
					dest := l.currentFn.NewTemp(ptr.Type().ElemType(), cleanVarName(identLeft.Value))
					l.currBlock.AddInstruction(&LoadInst{Dest: dest, Ptr: ptr})
					receiverObj = dest
					calleeName = member.Property.Value
				} else {
					calleeName = identLeft.Value + "_" + member.Property.Value
				}
			} else {
				recv, err := l.lowerExpression(member.Left)
				if err != nil {
					return nil, err
				}
				receiverObj = recv
				calleeName = member.Property.Value
			}
			if receiverObj != nil && receiverObj.Type().Kind == TypeKindPtr && receiverObj.Type().Elem != nil {
				clsName := receiverObj.Type().Elem.Name
				if clsName != "" && clsName != "map" {
					if targetFn, _, found := l.resolveClassMethod(clsName, member.Property.Value); found {
						calleeName = targetFn
					} else {
						calleeName = clsName + "_" + member.Property.Value
					}
				}
			}
		} else if ident, ok := e.Function.(*parser.Identifier); ok {
			calleeName = ident.Value
		} else if fnLit, ok := e.Function.(*parser.FunctionLiteral); ok {
			fnVal, err := l.lowerFunctionLiteral(fnLit)
			if err != nil {
				return nil, err
			}
			if cfp, ok := fnVal.(*ConstFnPtr); ok {
				calleeName = cfp.FnName
			} else if cs, ok := fnVal.(*ConstString); ok {
				calleeName = cs.Val
			} else {
				calleeName = "__closure_0"
			}
		} else {
			calleeName = e.Function.String()
		}

		if l.prep != nil && l.prep.Facts != nil {
			if resolved, ok := l.prep.Facts.ResolvedCalls[e]; ok {
				if resolved.Kind == analyzer.CallKindMethod {
					if lastColons := strings.LastIndex(resolved.TargetID, "::"); lastColons != -1 {
						prefix := resolved.TargetID[:lastColons]
						className := prefix[strings.LastIndex(prefix, ":")+1:]
						methodName := resolved.TargetID[lastColons+2:]
						if targetFn, _, found := l.resolveClassMethod(className, methodName); found {
							calleeName = targetFn
						} else {
							calleeName = className + "_" + methodName
						}
					}
				}
			}
		}

		var args []Value
		if receiverObj != nil {
			args = append(args, receiverObj)
		}
		for _, a := range e.Arguments {
			v, err := l.lowerExpression(a)
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}

		if (calleeName == "GranDB_where" || calleeName == "where") && len(args) == 4 {
			// Normalize 3-argument where(col, op, val) -> where(col, val)
			args = []Value{args[0], args[1], args[3]}
		}
		if (calleeName == "GranDB_table" || calleeName == "table") && len(args) == 2 {
			calleeName = "GranDB_table"
			args = []Value{args[1]}
		}
		if calleeName == "View_render" {
			calleeName = "view"
		}
		if (calleeName == "env" || calleeName == "request" || calleeName == "Request_input" || calleeName == "Session_get") && len(args) == 1 {
			args = append(args, NewConstInt(0, TypeI64))
		}

		// Determinar tipo de retorno usando AnalysisFacts o resolución de método en jerarquía
		retType := TypeI64
		var isBuiltin, isHost, isPrimitive bool
		var targetID string

		if l.prep != nil && l.prep.Facts != nil {
			if resolved, ok := l.prep.Facts.ResolvedCalls[e]; ok {
				retType = l.mapTypeSystemType(resolved.ReturnType)
				targetID = resolved.TargetID
				if resolved.Kind == analyzer.CallKindBuiltin {
					isBuiltin = true
				} else if resolved.Kind == analyzer.CallKindPrimitive {
					isPrimitive = true
				} else if resolved.Kind == analyzer.CallKindNativeStatic {
					isHost = true
				}
			}
		}
		if retType.Kind == TypeKindI64 && receiverObj != nil && receiverObj.Type().Kind == TypeKindPtr && receiverObj.Type().Elem != nil {
			if member, ok := e.Function.(*parser.MemberExpression); ok && member.Property != nil {
				if _, mRet, found := l.resolveClassMethod(receiverObj.Type().Elem.Name, member.Property.Value); found {
					retType = mRet
				}
			}
		}
		switch calleeName {
		case "view", "SEO_render", "html_escape", "csrf_field", "Markdown_readFile", "Markdown_toHtml", "Str_trim":
			retType = TypeString
		case "array_push", "append":
			if len(args) > 0 {
				retType = args[0].Type()
			}
		case "abs", "intval", "strlen", "len", "count":
			retType = TypeI64
		case "boolval":
			retType = TypeBool
		case "min", "max":
			if len(args) > 0 {
				retType = args[0].Type()
			} else {
				retType = TypeI64
			}
		case "array_pop":
			if len(args) > 0 && args[0].Type().Kind == TypeKindArray && args[0].Type().Elem != nil {
				retType = *args[0].Type().Elem
			} else {
				retType = TypeI64
			}
		case "GranDB_table", "GranDB_where", "GranDB_orWhere", "GranDB_whereIn", "GranDB_orderBy", "GranDB_orderByDesc", "orderByDesc", "GranDB_whereLike", "whereLike", "GranDB_orWhereLike", "orWhereLike", "GranDB_limit", "GranDB_offset", "GranDB_select", "GranDB_join", "GranDB_leftJoin":
			retType = PtrType(StructType("GranDB", nil))
		case "GranDB_first", "GranDB_find", "GranDB_paginate", "paginate":
			retType = PtrType(StructType("map", nil))
		case "GranDB_get":
			retType = ArrayType(PtrType(StructType("map", nil)))
		}

		var dest *TempValue
		if retType.Kind != TypeKindVoid {
			dest = l.currentFn.NewTemp(retType, "call_ret")
		}

		if isBuiltin {
			l.currBlock.AddInstruction(&CallBuiltinInst{
				Dest:      dest,
				BuiltinID: calleeName,
				Args:      args,
			})
		} else if isPrimitive {
			parts := strings.Split(targetID, "::")
			typeKind := ""
			methodName := ""
			if len(parts) >= 3 {
				typeKind = parts[1]
				methodName = parts[2]
			} else if len(parts) == 2 {
				typeKind = parts[0]
				methodName = parts[1]
			}
			l.currBlock.AddInstruction(&CallPrimitiveMethodInst{
				Dest:       dest,
				TypeKind:   typeKind,
				MethodName: methodName,
				Recv:       receiverObj,
				Args:       args,
			})
		} else if isHost {
			parts := strings.Split(targetID, "::")
			className := ""
			methodName := ""
			if len(parts) >= 2 {
				className = parts[len(parts)-2]
				methodName = parts[len(parts)-1]
			} else {
				className = targetID
			}
			l.currBlock.AddInstruction(&CallHostServiceInst{
				Dest:       dest,
				ClassName:  className,
				MethodName: methodName,
				Recv:       receiverObj,
				Args:       args,
			})
		} else {
			if calleeName == "strlen" || calleeName == "count" || calleeName == "len" || calleeName == "array_push" || calleeName == "append" || calleeName == "to_string" ||
				calleeName == "abs" || calleeName == "min" || calleeName == "max" || calleeName == "intval" || calleeName == "boolval" || calleeName == "array_pop" {
				l.currBlock.AddInstruction(&CallBuiltinInst{
					Dest:      dest,
					BuiltinID: calleeName,
					Args:      args,
				})
			} else if strings.HasPrefix(calleeName, "GranDB_") {
				parts := strings.SplitN(calleeName, "_", 2)
				className := parts[0]
				methodName := parts[1]
				l.currBlock.AddInstruction(&CallHostServiceInst{
					Dest:       dest,
					ClassName:  className,
					MethodName: methodName,
					Recv:       receiverObj,
					Args:       args,
				})
			} else {
				// Check if calleeName is a local closure variable
				if ptr, ok := l.varLocals[cleanVarName(calleeName)]; ok {
					closureVal := l.currentFn.NewTemp(ptr.Type().ElemType(), cleanVarName(calleeName)+"_val")
					l.currBlock.AddInstruction(&LoadInst{Dest: closureVal, Ptr: ptr})
					l.currBlock.AddInstruction(&CallIndirectInst{
						Dest:      dest,
						CalleeVal: closureVal,
						Args:      args,
					})
				} else {
					l.currBlock.AddInstruction(&CallInst{
						Dest:   dest,
						Callee: calleeName,
						Args:   args,
					})
				}
			}
		}

		if dest != nil {
			return dest, nil
		}
		return NewConstInt(0, TypeVoid), nil

	case *parser.AssignExpression:
		val, err := l.lowerExpression(e.Value)
		if err != nil {
			return nil, err
		}
		if ident, ok := e.Left.(*parser.Identifier); ok {
			varName := cleanVarName(ident.Value)
			if ptr, ok := l.varLocals[varName]; ok {
				l.currBlock.AddInstruction(&StoreInst{Val: val, Ptr: ptr})
				return val, nil
			}
			allocType := val.Type()
			if allocType.Kind == TypeKindVoid || allocType.Kind == TypeKindUnknown {
				allocType = TypeI64
			}
			ptrTemp := l.currentFn.NewTemp(PtrType(allocType), varName+"_ptr")
			l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: allocType})
			l.currBlock.AddInstruction(&StoreInst{Val: val, Ptr: ptrTemp})
			l.varLocals[varName] = ptrTemp
			return val, nil
		} else if member, ok := e.Left.(*parser.MemberExpression); ok {
			objVal, err := l.lowerExpression(member.Left)
			if err != nil {
				return nil, err
			}
			l.currBlock.AddInstruction(&StoreFieldInst{
				Val:       val,
				Obj:       objVal,
				FieldName: member.Property.Value,
			})
			return val, nil
		} else if idxExpr, ok := e.Left.(*parser.IndexExpression); ok {
			colVal, err := l.lowerExpression(idxExpr.Left)
			if err != nil {
				return nil, err
			}
			if idxExpr.Index == nil {
				l.currBlock.AddInstruction(&CallInst{
					Dest:   nil,
					Callee: "array_push",
					Args:   []Value{colVal, val},
				})
				return val, nil
			}
			idxVal, err := l.lowerExpression(idxExpr.Index)
			if err != nil {
				return nil, err
			}
			isMapCol := colVal.Type().Kind == TypeKindPtr && colVal.Type().Elem != nil && colVal.Type().Elem.Name == "map"
			if idxVal.Type().Kind == TypeKindString || isMapCol {
				l.currBlock.AddInstruction(&MapSetInst{
					Val: val,
					Map: colVal,
					Key: idxVal,
				})
			} else {
				l.currBlock.AddInstruction(&ArraySetInst{
					Val:   val,
					Array: colVal,
					Index: idxVal,
				})
			}
			return val, nil
		}
		return nil, fmt.Errorf("lowerer: asignación a destino no soportado %T", e.Left)

	default:
		return nil, &UnsupportedCapabilityError{
			Feature: describeUnsupportedExpression(expr),
		}
	}
}

func (l *Lowerer) lowerFunctionLiteral(e *parser.FunctionLiteral) (Value, error) {
	closureID := l.closureCounter
	l.closureCounter++
	closureName := fmt.Sprintf("__closure_%d", closureID)

	retType := l.mapType(e.ReturnType.Literal)
	if e.ReturnType.Literal == "" {
		retType = TypeI64
	}

	savedFn := l.currentFn
	savedBlock := l.currBlock
	savedLocals := l.varLocals
	savedLoops := l.loopStack

	fn := NewFunction(closureName, retType)
	l.currentFn = fn
	l.currBlock = fn.EntryBlock
	l.varLocals = make(map[string]Value)
	l.loopStack = nil

	// Copiar declaraciones del entorno léxico exterior como slots locales para capturas
	for k, outerPtr := range savedLocals {
		elemT := outerPtr.Type().ElemType()
		ptrTemp := fn.NewTemp(PtrType(elemT), k+"_cap_ptr")
		l.currBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: elemT})
		var initVal Value = NewConstInt(0, elemT)
		if elemT.Kind == TypeKindString {
			initVal = NewConstString("")
		} else if elemT.Kind == TypeKindPtr || elemT.Kind == TypeKindStruct || elemT.Kind == TypeKindArray {
			initVal = NewConstNull(elemT)
		}
		l.currBlock.AddInstruction(&StoreInst{Val: initVal, Ptr: ptrTemp})
		l.varLocals[k] = ptrTemp
	}

	for _, param := range e.Parameters {
		paramType := l.mapType(param.Type.Literal)
		if paramType.Kind == TypeKindUnknown {
			paramType = TypeI64
		}
		paramVal := fn.AddParam(param.Name.Value, paramType)
		ptrTemp := fn.NewTemp(PtrType(paramType), param.Name.Value+"_ptr")
		l.currBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: paramType})
		l.currBlock.AddInstruction(&StoreInst{Val: paramVal, Ptr: ptrTemp})
		l.varLocals[cleanVarName(param.Name.Value)] = ptrTemp
	}

	if e.Body != nil {
		for _, bodyStmt := range e.Body.Statements {
			if err := l.lowerStatement(bodyStmt); err != nil {
				l.currentFn = savedFn
				l.currBlock = savedBlock
				l.varLocals = savedLocals
				l.loopStack = savedLoops
				return nil, err
			}
		}
	}

	if l.currBlock.Terminator == nil {
		if retType.Kind == TypeKindVoid {
			l.currBlock.SetTerminator(&ReturnTerminator{})
		} else if retType.Kind == TypeKindString {
			l.currBlock.SetTerminator(&ReturnTerminator{Val: NewConstString("")})
		} else if retType.Kind == TypeKindPtr || retType.Kind == TypeKindStruct || retType.Kind == TypeKindArray {
			l.currBlock.SetTerminator(&ReturnTerminator{Val: NewConstNull(retType)})
		} else {
			l.currBlock.SetTerminator(&ReturnTerminator{Val: NewConstInt(0, retType)})
		}
	}

	l.prog.AddFunction(fn)

	l.currentFn = savedFn
	l.currBlock = savedBlock
	l.varLocals = savedLocals
	l.loopStack = savedLoops

	return NewConstFnPtr(closureName, PtrType(TypeVoid)), nil
}

func (l *Lowerer) lowerMatchExpression(e *parser.MatchExpression) (Value, error) {
	subjVal, err := l.lowerExpression(e.Subject)
	if err != nil {
		return nil, err
	}

	resType := l.inferNodeType(e)
	if resType.Kind == TypeKindUnknown || resType.Kind == TypeKindVoid {
		resType = TypeI64
	}
	resPtr := l.currentFn.NewTemp(PtrType(resType), "match_ptr")
	l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: resPtr, AllocType: resType})

	mergeBlock := l.currentFn.NewBlock("match_merge")

	for idx, arm := range e.Arms {
		if arm.IsDefault || len(arm.Keys) == 0 {
			armVal, err := l.lowerExpression(arm.Value)
			if err != nil {
				return nil, err
			}
			if l.currBlock.Terminator == nil {
				l.currBlock.AddInstruction(&StoreInst{Val: armVal, Ptr: resPtr})
				l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
			}
			break
		}

		var armCond Value
		for _, kExpr := range arm.Keys {
			kVal, err := l.lowerExpression(kExpr)
			if err != nil {
				return nil, err
			}
			cmp := l.currentFn.NewTemp(TypeBool, fmt.Sprintf("match_cmp_%d", idx))
			l.currBlock.AddInstruction(&CompareInst{
				Op:    OpCmpEq,
				Dest:  cmp,
				Left:  subjVal,
				Right: kVal,
			})
			if armCond == nil {
				armCond = cmp
			} else {
				orTemp := l.currentFn.NewTemp(TypeBool, fmt.Sprintf("match_or_%d", idx))
				l.currBlock.AddInstruction(&BinaryInst{
					Op:    OpOr,
					Dest:  orTemp,
					Left:  armCond,
					Right: cmp,
				})
				armCond = orTemp
			}
		}

		armBlock := l.currentFn.NewBlock(fmt.Sprintf("match_arm_%d", idx))
		nextBlock := l.currentFn.NewBlock(fmt.Sprintf("match_next_%d", idx))
		l.currBlock.SetTerminator(&BranchTerminator{
			Cond:       armCond,
			TrueBlock:  armBlock,
			FalseBlock: nextBlock,
		})

		l.currBlock = armBlock
		armVal, err := l.lowerExpression(arm.Value)
		if err != nil {
			return nil, err
		}
		if l.currBlock.Terminator == nil {
			l.currBlock.AddInstruction(&StoreInst{Val: armVal, Ptr: resPtr})
			l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
		}

		l.currBlock = nextBlock
	}

	if l.currBlock.Terminator == nil {
		l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
	}

	l.currBlock = mergeBlock
	resTemp := l.currentFn.NewTemp(resType, "match_res")
	l.currBlock.AddInstruction(&LoadInst{Dest: resTemp, Ptr: resPtr})
	return resTemp, nil
}

func (l *Lowerer) inferNodeType(expr parser.Node) Type {
	if l.prep != nil && l.prep.Facts != nil {
		if t, ok := l.prep.Facts.InferredTypes[expr]; ok {
			return l.mapTypeSystemType(t)
		}
	}
	return TypeI64
}

func (l *Lowerer) mapTypeSystemType(t typesystem.Type) Type {
	switch t.Kind {
	case typesystem.Int:
		return TypeI64
	case typesystem.Float:
		return TypeF64
	case typesystem.Bool:
		return TypeBool
	case typesystem.String:
		return TypeString
	case typesystem.Void:
		return TypeVoid
	case typesystem.Class:
		return PtrType(StructType(t.Name, nil))
	case typesystem.Array:
		elem := TypeI64
		if t.Element != nil {
			elem = l.mapTypeSystemType(*t.Element)
		}
		return ArrayType(elem)
	case typesystem.Map:
		return PtrType(StructType("map", nil))
	case typesystem.Decimal:
		return TypeDecimal
	case typesystem.Mixed:
		return TypeMixed
	case typesystem.Union:
		return Type{Kind: TypeKindUnion}
	case typesystem.Channel:
		elem := TypeI64
		if t.Element != nil {
			elem = l.mapTypeSystemType(*t.Element)
		}
		return PtrType(StructType("channel", []Type{elem}))
	case typesystem.Callable:
		ret := TypeVoid
		if t.Element != nil {
			ret = l.mapTypeSystemType(*t.Element)
		}
		return FuncType(nil, ret)
	default:
		return TypeI64
	}
}

func (l *Lowerer) lookupClassPropertyType(className, propName string) (Type, bool) {
	if l.prep == nil {
		return TypeI64, false
	}
	curr := className
	for curr != "" {
		nextSuper := ""
		for _, unit := range l.prep.Units {
			if unit.Program == nil {
				continue
			}
			for _, stmt := range unit.Program.Statements {
				cls, ok := stmt.(*parser.ClassStatement)
				if !ok || cls.Name == nil || cls.Name.Value != curr {
					continue
				}
				if cls.SuperClass != nil {
					nextSuper = cls.SuperClass.Value
				}
				if cls.Body != nil {
					for _, member := range cls.Body.Statements {
						switch m := member.(type) {
						case *parser.LetStatement:
							if m.Name != nil && cleanVarName(m.Name.Value) == propName {
								return l.mapType(m.Token.Literal), true
							}
						case *parser.MultiLetStatement:
							for _, d := range m.Declarations {
								if d.Name != nil && cleanVarName(d.Name.Value) == propName {
									return l.mapType(m.TypeToken.Literal), true
								}
							}
						}
					}
				}
			}
		}
		curr = nextSuper
	}
	return TypeI64, false
}

func (l *Lowerer) resolveClassMethod(className, methodName string) (string, Type, bool) {
	if l.prep == nil {
		return "", TypeI64, false
	}
	curr := className
	for curr != "" {
		nextSuper := ""
		for _, unit := range l.prep.Units {
			if unit.Program == nil {
				continue
			}
			for _, stmt := range unit.Program.Statements {
				cls, ok := stmt.(*parser.ClassStatement)
				if !ok || cls.Name == nil || cls.Name.Value != curr {
					continue
				}
				if cls.SuperClass != nil {
					nextSuper = cls.SuperClass.Value
				}
				if cls.Body != nil {
					for _, member := range cls.Body.Statements {
						if m, ok := member.(*parser.MethodStatement); ok && m.Name != nil && m.Name.Value == methodName {
							return curr + "_" + methodName, l.mapType(m.ReturnType.Literal), true
						}
					}
				}
			}
		}
		curr = nextSuper
	}
	return "", TypeI64, false
}

func (l *Lowerer) mapType(lit string) Type {
	lit = strings.TrimSpace(lit)
	if strings.HasSuffix(lit, "[]") {
		elem := l.mapType(strings.TrimSuffix(lit, "[]"))
		return ArrayType(elem)
	}
	switch lit {
	case "int":
		return TypeI64
	case "float":
		return TypeF64
	case "bool":
		return TypeBool
	case "string":
		return TypeString
	case "void":
		return TypeVoid
	case "map":
		return PtrType(StructType("map", nil))
	case "array":
		return ArrayType(TypeI64)
	case "var", "mixed", "unknown":
		return TypeUnknown
	default:
		if l.prep != nil {
			if l.prep.Environment.Classes != nil {
				if _, ok := l.prep.Environment.Classes[lit]; ok {
					return PtrType(StructType(lit, nil))
				}
			}
			if l.prep.Environment.Interfaces != nil {
				if _, ok := l.prep.Environment.Interfaces[lit]; ok {
					return PtrType(StructType(lit, nil))
				}
			}
		}
		return TypeI64
	}
}

func mapInfixOp(op string) (Opcode, bool) {
	switch op {
	case "+":
		return OpAdd, false
	case "-":
		return OpSub, false
	case "*":
		return OpMul, false
	case "/":
		return OpDiv, false
	case "%":
		return OpMod, false
	case "&&", "&":
		return OpAnd, false
	case "||", "|":
		return OpOr, false
	case "^":
		return OpXor, false
	case "<<":
		return OpShl, false
	case ">>":
		return OpShr, false
	case "==", "===":
		return OpCmpEq, true
	case "!=", "!==":
		return OpCmpNe, true
	case "<":
		return OpCmpLt, true
	case "<=":
		return OpCmpLe, true
	case ">":
		return OpCmpGt, true
	case ">=":
		return OpCmpGe, true
	case "<=>":
		return OpSpaceship, true
	default:
		return OpAdd, false
	}
}

func cleanVarName(name string) string {
	return strings.TrimPrefix(name, "$")
}

func (t Type) ElemType() Type {
	if t.Elem != nil {
		return *t.Elem
	}
	return TypeI64
}

// UnsupportedCapabilityError representa el rechazo en tiempo de compilación nativa
// de una característica de Joss que aún no tiene soporte directo en Native IR.
type UnsupportedCapabilityError struct {
	Feature string
}

func (e *UnsupportedCapabilityError) Error() string {
	return analyzer.FormatCapabilityError(e.Feature)
}

func describeUnsupportedStatement(stmt parser.Statement) string {
	switch stmt.(type) {
	case *parser.EnumStatement:
		return "Enumeraciones (enum)"
	case *parser.SelectStatement:
		return "Selección concurrente de canales (select)"
	case *parser.ForeachStatement:
		return "Iteración de colecciones dinámicas (foreach)"
	case *parser.DeferStatement:
		return "Ejecución diferida (defer)"
	case *parser.InitStatement:
		return "Constructores Init de clases"
	case *parser.DestructureStatement:
		return "Desestructuración de colecciones"
	default:
		return fmt.Sprintf("Sentencia no soportada (%T)", stmt)
	}
}

func describeUnsupportedExpression(expr parser.Expression) string {
	switch expr.(type) {
	case *parser.MatchExpression:
		return "Expresiones de coincidencia de patrones (match)"
	case *parser.YieldExpression:
		return "Generadores y corrutinas (yield)"
	default:
		return fmt.Sprintf("Expresión no soportada (%T)", expr)
	}
}
