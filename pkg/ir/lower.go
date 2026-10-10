package ir

import (
	"fmt"
	"strings"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

// Lowerer es el compilador que transforma un PreparedProgram del analizador semántico en Joss Native IR.
type Lowerer struct {
	prep          *analyzer.PreparedProgram
	reachGraph    *analyzer.ReachabilityGraph
	currentFn     *Function
	currBlock     *BasicBlock
	prog          *Program
	varLocals     map[string]Value // Mapea variables a sus punteros locales (alloca)
	loopStack     []loopContext
	tryCatchCount int
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
			case *parser.ClassStatement:
				if s.Body != nil {
					for _, member := range s.Body.Statements {
						if m, ok := member.(*parser.MethodStatement); ok {
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
			case *parser.InterfaceStatement:
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
			if _, isFn := stmt.(*parser.MethodStatement); isFn {
				continue
			}
			if _, isClass := stmt.(*parser.ClassStatement); isClass {
				continue
			}
			if _, isIface := stmt.(*parser.InterfaceStatement); isIface {
				continue
			}
			hasTopLevelStmts = true
			break
		}

		if hasTopLevelStmts && l.prog.Functions["main"] == nil {
			if err := l.synthesizeMainFunction(entryUnit.Statements); err != nil {
				return nil, err
			}
		}
	}

	return l.prog, nil
}

func (l *Lowerer) synthesizeMainFunction(stmts []parser.Statement) error {
	mainFn := NewFunction("main", TypeI64)
	l.currentFn = mainFn
	l.currBlock = mainFn.EntryBlock
	l.varLocals = make(map[string]Value)

	for _, stmt := range stmts {
		if _, isFn := stmt.(*parser.MethodStatement); isFn {
			continue
		}
		if _, isClass := stmt.(*parser.ClassStatement); isClass {
			continue
		}
		if _, isIface := stmt.(*parser.InterfaceStatement); isIface {
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
		if valType.Kind == TypeKindUnknown && s.Value != nil {
			valType = l.inferNodeType(s.Value)
		}
		ptrTemp := l.currentFn.NewTemp(PtrType(valType), s.Name.Value+"_ptr")
		l.currBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: valType})

		var val Value
		if s.Value != nil {
			v, err := l.lowerExpression(s.Value)
			if err != nil {
				return err
			}
			val = v
		} else {
			val = NewConstInt(0, valType)
		}
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
		condBlock.SetTerminator(&BranchTerminator{
			Cond:       condVal,
			TrueBlock:  bodyBlock,
			FalseBlock: afterBlock,
		})

		l.currBlock = afterBlock
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
		} else {
			l.currBlock.AddInstruction(&CallRuntimeInst{Func: "print_i64", Args: []Value{val}})
		}
		return nil

	case *parser.ClassStatement:
		return nil

	case *parser.InterfaceStatement:
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

func (l *Lowerer) lowerExpression(expr parser.Expression) (Value, error) {
	switch e := expr.(type) {
	case *parser.IntegerLiteral:
		return NewConstInt(e.Value, TypeI64), nil

	case *parser.FloatLiteral:
		return NewConstFloat(e.Value, TypeF64), nil

	case *parser.Boolean:
		return NewConstBool(e.Value), nil

	case *parser.StringLiteral:
		return NewConstString(e.Value), nil

	case *parser.NullLiteral:
		return NewConstNull(PtrType(TypeVoid)), nil

	case *parser.Identifier:
		varName := cleanVarName(e.Value)
		if ptr, ok := l.varLocals[varName]; ok {
			dest := l.currentFn.NewTemp(ptr.Type().ElemType(), varName)
			l.currBlock.AddInstruction(&LoadInst{Dest: dest, Ptr: ptr})
			return dest, nil
		}
		return nil, fmt.Errorf("lowerer: variable no resuelta %s", e.Value)

	case *parser.InfixExpression:
		left, err := l.lowerExpression(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := l.lowerExpression(e.Right)
		if err != nil {
			return nil, err
		}

		if (e.Operator == "+" || e.Operator == ".") && (left.Type().Kind == TypeKindString || right.Type().Kind == TypeKindString) {
			l.prog.RequireRuntime("str_concat")
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
		if e.Operator == "!" {
			dest := l.currentFn.NewTemp(TypeBool, "not")
			l.currBlock.AddInstruction(&UnaryInst{Op: OpNot, Dest: dest, Val: right})
			return dest, nil
		} else if e.Operator == "-" {
			dest := l.currentFn.NewTemp(right.Type(), "neg")
			l.currBlock.AddInstruction(&UnaryInst{Op: OpNeg, Dest: dest, Val: right})
			return dest, nil
		}
		return nil, fmt.Errorf("operador prefijo '%s' no soportado", e.Operator)

	case *parser.TernaryExpression:
		condVal, err := l.lowerExpression(e.Condition)
		if err != nil {
			return nil, err
		}

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
		trueVal, err := l.lowerExpression(e.True)
		if err != nil {
			return nil, err
		}
		// Reservar alloca para el resultado ternario en el entry block si es posible
		resTemp := l.currentFn.NewTemp(trueVal.Type(), "ternary_res")
		resPtr := l.currentFn.NewTemp(PtrType(trueVal.Type()), "ternary_ptr")
		l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: resPtr, AllocType: trueVal.Type()})
		l.currBlock.AddInstruction(&StoreInst{Val: trueVal, Ptr: resPtr})
		l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})

		// Rama falsa
		l.currBlock = falseBlock
		falseVal, err := l.lowerExpression(e.False)
		if err != nil {
			return nil, err
		}
		l.currBlock.AddInstruction(&StoreInst{Val: falseVal, Ptr: resPtr})
		l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})

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

		// Inicializar propiedades por defecto de la clase si existen
		if l.prep != nil && l.prep.Environment.Classes != nil {
			if classDef, ok := l.prep.Environment.Classes[className]; ok {
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
								}
							}
						}
					}
				}
				_ = classDef
			}
		}

		// Si tiene constructor (Init o constructor), invocarlo
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
		objVal, err := l.lowerExpression(e.Left)
		if err != nil {
			return nil, err
		}
		propName := e.Property.Value
		fieldType := l.inferNodeType(e)
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
			if v.Type().Kind == TypeKindString {
				elemType = TypeString
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
		for k, v := range e.Pairs {
			kVal, err := l.lowerExpression(k)
			if err != nil {
				return nil, err
			}
			vVal, err := l.lowerExpression(v)
			if err != nil {
				return nil, err
			}
			pairs = append(pairs, [2]Value{kVal, vVal})
		}
		mapTemp := l.currentFn.NewTemp(PtrType(StructType("map", nil)), "map")
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
		resTemp := l.currentFn.NewTemp(resType, "elem")
		if idxVal.Type().Kind == TypeKindString {
			l.currBlock.AddInstruction(&MapGetInst{
				Dest: resTemp,
				Map:  container,
				Key:  idxVal,
			})
		} else {
			l.currBlock.AddInstruction(&ArrayGetInst{
				Dest:  resTemp,
				Array: container,
				Index: idxVal,
			})
		}
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
				}
				calleeName = identLeft.Value + "_" + member.Property.Value
			} else {
				recv, err := l.lowerExpression(member.Left)
				if err != nil {
					return nil, err
				}
				receiverObj = recv
				calleeName = member.Property.Value
			}
		} else if ident, ok := e.Function.(*parser.Identifier); ok {
			calleeName = ident.Value
		} else {
			calleeName = e.Function.String()
		}

		if l.prep != nil && l.prep.Facts != nil {
			if resolved, ok := l.prep.Facts.ResolvedCalls[e]; ok {
				if resolved.Kind == "method" {
					if lastColons := strings.LastIndex(resolved.TargetID, "::"); lastColons != -1 {
						prefix := resolved.TargetID[:lastColons]
						className := prefix[strings.LastIndex(prefix, ":")+1:]
						methodName := resolved.TargetID[lastColons+2:]
						calleeName = className + "_" + methodName
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

		// Determinar tipo de retorno usando AnalysisFacts si está disponible
		retType := TypeI64
		if l.prep != nil && l.prep.Facts != nil {
			if resolved, ok := l.prep.Facts.ResolvedCalls[e]; ok {
				retType = l.mapTypeSystemType(resolved.ReturnType)
			}
		}

		var dest *TempValue
		if retType.Kind != TypeKindVoid {
			dest = l.currentFn.NewTemp(retType, "call_ret")
		}

		l.currBlock.AddInstruction(&CallInst{
			Dest:   dest,
			Callee: calleeName,
			Args:   args,
		})
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
			return nil, fmt.Errorf("lowerer: asignación a variable no declarada %s", ident.Value)
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
			idxVal, err := l.lowerExpression(idxExpr.Index)
			if err != nil {
				return nil, err
			}
			if idxVal.Type().Kind == TypeKindString {
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
	default:
		return TypeI64
	}
}

func (l *Lowerer) mapType(lit string) Type {
	lit = strings.TrimSpace(lit)
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
	case "==":
		return OpCmpEq, true
	case "!=":
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
