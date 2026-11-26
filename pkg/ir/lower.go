package ir

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
	"github.com/jossecurity/joss/pkg/version"
)

// Lowerer es el compilador que transforma un PreparedProgram del analizador semántico en Joss Native IR.
type pendingVirtualMethod struct {
	className string
	method    *parser.MethodStatement
}

type Lowerer struct {
	prep                  *analyzer.PreparedProgram
	reachGraph            *analyzer.ReachabilityGraph
	currentFn             *Function
	currBlock             *BasicBlock
	prog                  *Program
	varLocals             map[string]Value // Mapea variables a sus punteros locales (alloca)
	externDecls           map[string]externDecl
	viewDataParam         Value // Parámetro $__data activo al compilar vistas .joss.html a IR
	loopStack             []loopContext
	tryCatchCount         int
	closureCounter        int
	foreachCounter        int
	OptLevel              int
	activeClosureCaptured map[string]bool
	activeClosureEnvMap   Value
	pendingVirtualMethods []pendingVirtualMethod
	currentFnStmts        []parser.Statement
}

type externDecl struct {
	Symbol      string
	RetType     Type
	ArgTypes    []Type
	LibraryPath string
}

type loopContext struct {
	continueBlock *BasicBlock
	breakBlock    *BasicBlock
}

func NewLowerer(prep *analyzer.PreparedProgram, reachGraph *analyzer.ReachabilityGraph) *Lowerer {
	if reachGraph == nil && prep != nil {
		ep := ""
		if len(prep.Units) > 0 {
			ep = prep.Units[0].Path
		}
		reachGraph = analyzer.BuildReachabilityGraph(prep, analyzer.ReachabilityOptions{
			Entrypoint: ep,
		})
	}
	return &Lowerer{
		prep:        prep,
		reachGraph:  reachGraph,
		varLocals:   make(map[string]Value),
		externDecls: make(map[string]externDecl),
		loopStack:   make([]loopContext, 0),
		OptLevel:    1, // Por defecto habilitar optimizaciones IR canónicas
	}
}

// LowerProgram compila el programa semántico validado a Joss Native IR.
func (l *Lowerer) LowerProgram(name string) (*Program, error) {
	if l.prep == nil {
		return nil, fmt.Errorf("lowerer: PreparedProgram es nil")
	}

	l.prog = NewProgram(name)

	// Registrar layouts de clases antes de procesar métodos
	l.buildAllClassLayouts()
	l.registerStaticClassProperties()

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
				if l.reachGraph != nil && !l.reachGraph.IsClassReachable(s.Name.Value) {
					continue
				}
				if s.Body != nil {
					for _, member := range s.Body.Statements {
						if m, ok := member.(*parser.MethodStatement); ok {
							if m.IsAbstract || m.Body == nil {
								continue
							}
							qualifiedName := s.Name.Value + "::" + m.Name.Value
							isModelCB := l.isModelClass(s.Name.Value) && isModelCallbackMethodName(m.Name.Value, len(m.Parameters))
							if !isModelCB && l.reachGraph != nil && !l.reachGraph.IsMethodReachable(s.Name.Value, m.Name.Value) && !l.reachGraph.IsFunctionReachable(qualifiedName) && !l.reachGraph.IsFunctionReachable(m.Name.Value) {
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
			case *parser.ExternLibraryStatement:
				l.registerExternLibrary(s)
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
			case *parser.MethodStatement, *parser.ClassStatement, *parser.InterfaceStatement, *parser.EnumStatement, *parser.InitStatement, *parser.ExternLibraryStatement:
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
	l.emitModelMetadataRegistrations()
	if err := l.flushPendingVirtualMethods(); err != nil {
		return nil, err
	}
	l.lowerGenericCollectionBuiltins()
	l.lowerRequestSessionContext()
	l.lowerRequestSEOContext()
	l.lowerJossHostServiceCalls()

	// Prune dead functions for every execution mode. Restricting this to HTTP
	// projects left unrelated standard-library methods in CLI IR and could pull
	// in unavailable platform services (for example DB calls beside pure toSql).
	l.PruneUnreachableFunctions()
	if err := l.lowerStaticPropertyInitializer(); err != nil {
		return nil, err
	}

	if l.OptLevel > 0 {
		OptimizeProgram(l.prog)
	}

	return l.prog, nil
}

func (l *Lowerer) registerExternLibrary(stmt *parser.ExternLibraryStatement) {
	if stmt == nil {
		return
	}
	if stmt.LibraryPath != "" && l.prog != nil {
		l.prog.AddExternLibrary(stmt.LibraryPath)
	}
	for _, fn := range stmt.Functions {
		if fn == nil || fn.Name == nil {
			continue
		}
		retType := l.mapType(fn.ReturnType.Literal)
		argTypes := make([]Type, 0, len(fn.Parameters))
		for _, p := range fn.Parameters {
			argTypes = append(argTypes, l.mapType(p.Type.Literal))
		}
		l.externDecls[fn.Name.Value] = externDecl{
			Symbol:      fn.Name.Value,
			RetType:     retType,
			ArgTypes:    argTypes,
			LibraryPath: stmt.LibraryPath,
		}
	}
}

func (l *Lowerer) synthesizeMainFunction(stmts []parser.Statement) error {
	mainFn := NewFunction("main", TypeI64)
	l.currentFn = mainFn
	l.currBlock = mainFn.EntryBlock
	l.varLocals = make(map[string]Value)
	savedStmts := l.currentFnStmts
	l.currentFnStmts = stmts
	defer func() { l.currentFnStmts = savedStmts }()

	for _, stmt := range stmts {
		switch stmt.(type) {
		case *parser.MethodStatement, *parser.ClassStatement, *parser.InterfaceStatement, *parser.EnumStatement, *parser.InitStatement, *parser.ExternLibraryStatement:
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
	retType := l.inferMethodReturnType(stmt)
	fn := NewFunction(stmt.Name.Value, retType)
	l.currentFn = fn
	l.currBlock = fn.EntryBlock
	l.varLocals = make(map[string]Value)
	savedStmts := l.currentFnStmts
	if stmt.Body != nil {
		l.currentFnStmts = stmt.Body.Statements
	} else {
		l.currentFnStmts = nil
	}
	defer func() { l.currentFnStmts = savedStmts }()

	// Parámetros formales
	for _, param := range stmt.Parameters {
		paramType := l.mapType(param.Type.Literal)
		paramVal := fn.AddParam(param.Name.Value, paramType)
		fn.DefaultArgs = append(fn.DefaultArgs, l.lowerConstantDefault(param.DefaultValue, paramType))

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
	retType := l.inferMethodReturnType(stmt)
	fn := NewFunction(mName, retType)
	l.currentFn = fn
	l.currBlock = fn.EntryBlock
	l.varLocals = make(map[string]Value)
	savedStmts := l.currentFnStmts
	if stmt.Body != nil {
		l.currentFnStmts = stmt.Body.Statements
	} else {
		l.currentFnStmts = nil
	}
	defer func() { l.currentFnStmts = savedStmts }()

	if !stmt.IsStatic {
		thisType := PtrType(StructType(className, nil))
		thisVal := fn.AddParam("this", thisType)
		fn.DefaultArgs = append(fn.DefaultArgs, nil)
		ptrTemp := fn.NewTemp(PtrType(thisType), "this_ptr")
		l.currBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: thisType})
		l.currBlock.AddInstruction(&StoreInst{Val: thisVal, Ptr: ptrTemp})
		l.varLocals["this"] = ptrTemp
	}

	for pIdx, param := range stmt.Parameters {
		paramType := l.mapType(param.Type.Literal)
		if pIdx == 0 && l.isModelClass(className) && strings.HasPrefix(stmt.Name.Value, "scope") {
			paramType = PtrType(StructType(className, nil))
		}
		paramVal := fn.AddParam(param.Name.Value, paramType)
		fn.DefaultArgs = append(fn.DefaultArgs, l.lowerConstantDefault(param.DefaultValue, paramType))

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

func (l *Lowerer) registerStaticClassProperties() {
	if l.prep == nil || l.prep.Units == nil {
		return
	}
	for _, unit := range l.prep.Units {
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			class, ok := stmt.(*parser.ClassStatement)
			if !ok || class.Name == nil || class.Body == nil {
				continue
			}
			for _, member := range class.Body.Statements {
				field, ok := member.(*parser.LetStatement)
				if !ok || !field.IsStatic || field.Name == nil {
					continue
				}
				name := staticPropertyGlobalName(class.Name.Value, field.Name.Value)
				l.prog.AddGlobal(name, l.mapType(field.Token.Literal), nil, field.IsConst)
			}
		}
	}
}

func (l *Lowerer) lowerStaticPropertyInitializer() error {
	if l.prep == nil || l.prog == nil || len(l.prog.Globals) == 0 {
		return nil
	}
	fn := NewFunction("__joss_init_static_globals", TypeVoid)
	l.currentFn = fn
	l.currBlock = fn.EntryBlock
	l.varLocals = make(map[string]Value)
	initialized := make(map[string]bool)
	for _, unit := range l.prep.Units {
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			class, ok := stmt.(*parser.ClassStatement)
			if !ok || class.Name == nil || class.Body == nil {
				continue
			}
			for _, member := range class.Body.Statements {
				field, ok := member.(*parser.LetStatement)
				if !ok || !field.IsStatic || field.Name == nil {
					continue
				}
				globalName := staticPropertyGlobalName(class.Name.Value, field.Name.Value)
				if l.prog.Globals[globalName] == nil {
					continue
				}
				initialized[globalName] = true
				var value Value
				if field.Value != nil {
					lowered, err := l.lowerExpression(field.Value)
					if err != nil {
						return fmt.Errorf("lowerer: static initializer %s: %w", globalName, err)
					}
					value = lowered
				} else if l.prog.Globals[globalName].Type.Kind == TypeKindPtr && l.prog.Globals[globalName].Type.Elem != nil && l.prog.Globals[globalName].Type.Elem.Name == "map" {
					mapValue := l.currentFn.NewTemp(l.prog.Globals[globalName].Type, "static_map")
					l.currBlock.AddInstruction(&NewMapInst{Dest: mapValue})
					value = mapValue
				} else {
					value = NewConstNull(l.prog.Globals[globalName].Type)
				}
				l.currBlock.AddInstruction(&GlobalStoreInst{GlobalName: globalName, Val: value})
			}
		}
	}
	for globalName, global := range l.prog.Globals {
		if initialized[globalName] {
			continue
		}
		var value Value
		if global.Type.Kind == TypeKindPtr && global.Type.Elem != nil && global.Type.Elem.Name == "map" {
			mapValue := l.currentFn.NewTemp(global.Type, "static_map")
			l.currBlock.AddInstruction(&NewMapInst{Dest: mapValue})
			value = mapValue
		} else {
			value = NewConstNull(global.Type)
		}
		l.currBlock.AddInstruction(&GlobalStoreInst{GlobalName: globalName, Val: value})
	}
	fn.EntryBlock.SetTerminator(&ReturnTerminator{})
	l.prog.AddFunction(fn)
	return nil
}

func staticPropertyGlobalName(className, propertyName string) string {
	return className + "::$" + strings.TrimPrefix(propertyName, "$")
}

func (l *Lowerer) ensureStaticPropertyGlobal(className, propertyName string) *GlobalVar {
	if l.prog == nil || l.prep == nil {
		return nil
	}
	globalName := staticPropertyGlobalName(className, propertyName)
	if global := l.prog.Globals[globalName]; global != nil {
		return global
	}
	class, exists := l.prep.Environment.Classes[className]
	if !exists {
		return nil
	}
	field, exists := class.Fields[propertyName]
	if !exists {
		return nil
	}
	return l.prog.AddGlobal(globalName, l.mapTypeSystemType(field.Type), nil, field.Constant)
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
		declTok := strings.TrimSpace(s.Token.Literal)
		if (declTok == "var" || declTok == "let" || declTok == "") && valType.Kind != TypeKindMixed && s.Name != nil {
			if l.isVarReassignedIncompatible(cleanVarName(s.Name.Value), valType, l.currentFnStmts) {
				valType = TypeMixed
			}
		}
		if val == nil {
			val = NewConstInt(0, valType)
		}
		ptrTemp := l.currentFn.NewTemp(PtrType(valType), s.Name.Value+"_ptr")
		l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: valType})
		val = l.boxIfMixed(val, valType)
		val = l.unboxIfTyped(val, valType)
		l.currBlock.AddInstruction(&StoreInst{Val: val, Ptr: ptrTemp})
		l.varLocals[cleanVarName(s.Name.Value)] = ptrTemp
		return nil

	case *parser.MultiLetStatement:
		valType := l.mapType(s.TypeToken.Literal)
		for _, decl := range s.Declarations {
			ptrTemp := l.currentFn.NewTemp(PtrType(valType), decl.Name.Value+"_ptr")
			l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: valType})

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
			val = l.boxIfMixed(val, valType)
			val = l.unboxIfTyped(val, valType)
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
		if l.OptLevel > 0 && s.ReturnValue != nil {
			if ternary, ok := s.ReturnValue.(*parser.TernaryExpression); ok && ternary.True != nil && ternary.False != nil {
				return l.lowerReturnTernary(ternary)
			}
		}
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
		} else {
			retVal = l.boxIfMixed(retVal, l.currentFn.ReturnType)
			retVal = l.unboxIfTyped(retVal, l.currentFn.ReturnType)
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
		l.currBlock.SetTerminator(&BranchTerminator{
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
			if strings.HasPrefix(l.currBlock.Label, "dead") {
				l.currBlock.SetTerminator(&UnreachableTerminator{})
			} else {
				l.currBlock.SetTerminator(&JumpTerminator{Target: condBlock})
			}
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
			if strings.HasPrefix(l.currBlock.Label, "dead") {
				l.currBlock.SetTerminator(&UnreachableTerminator{})
			} else {
				l.currBlock.SetTerminator(&JumpTerminator{Target: condBlock})
			}
		}

		// Evaluar condición
		l.currBlock = condBlock
		condVal, err := l.lowerExpression(s.Condition)
		if err != nil {
			return err
		}
		condVal = l.ensureBoolCond(condVal)
		l.currBlock.SetTerminator(&BranchTerminator{
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
		} else if val.Type().Kind == TypeKindPtr && val.Type().Elem != nil && val.Type().Elem.Kind == TypeKindVoid {
			l.currBlock.AddInstruction(&CallRuntimeInst{Func: "print_string", Args: []Value{NewConstString("<nil>")}})
		} else if val.Type().Kind == TypeKindMixed {
			l.currBlock.AddInstruction(&CallRuntimeInst{Func: "print_mixed", Args: []Value{val}})
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
	isDynamic := !isMap && iterVal.Type().Kind != TypeKindArray

	idxPtr := l.currentFn.NewTemp(PtrType(TypeI64), fmt.Sprintf("fe_idx_%d_ptr", feID))
	l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: idxPtr, AllocType: TypeI64})
	l.currBlock.AddInstruction(&StoreInst{Val: NewConstInt(0, TypeI64), Ptr: idxPtr})

	var lenTemp Value
	var isMapRt Value
	if isDynamic {
		isMapRtTemp := l.currentFn.NewTemp(TypeBool, fmt.Sprintf("fe_ismap_%d", feID))
		isMapRt = isMapRtTemp
		l.currBlock.AddInstruction(&CallRuntimeInst{
			Dest: isMapRtTemp,
			Func: "is_map",
			Args: []Value{iterVal},
		})
		lenPtr := l.currentFn.NewTemp(PtrType(TypeI64), fmt.Sprintf("fe_len_%d_ptr", feID))
		l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: lenPtr, AllocType: TypeI64})

		lenMapBlock := l.currentFn.NewBlock(fmt.Sprintf("foreach_len_map_%d_", feID))
		lenArrBlock := l.newDetachedBlock(fmt.Sprintf("foreach_len_arr_%d_", feID))
		lenMergeBlock := l.newDetachedBlock(fmt.Sprintf("foreach_len_merge_%d_", feID))

		l.currBlock.SetTerminator(&BranchTerminator{
			Cond:       isMapRt,
			TrueBlock:  lenMapBlock,
			FalseBlock: lenArrBlock,
		})

		l.currBlock = lenMapBlock
		mLen := l.currentFn.NewTemp(TypeI64, fmt.Sprintf("fe_mlen_%d", feID))
		l.currBlock.AddInstruction(&CallInst{
			Dest:   mLen,
			Callee: "joss_map_len",
			Args:   []Value{iterVal},
		})
		l.currBlock.AddInstruction(&StoreInst{Val: mLen, Ptr: lenPtr})
		l.currBlock.SetTerminator(&JumpTerminator{Target: lenMergeBlock})

		l.currentFn.Blocks = append(l.currentFn.Blocks, lenArrBlock)
		l.currBlock = lenArrBlock
		aLen := l.currentFn.NewTemp(TypeI64, fmt.Sprintf("fe_alen_%d", feID))
		l.currBlock.AddInstruction(&CallInst{
			Dest:   aLen,
			Callee: "len",
			Args:   []Value{iterVal},
		})
		l.currBlock.AddInstruction(&StoreInst{Val: aLen, Ptr: lenPtr})
		l.currBlock.SetTerminator(&JumpTerminator{Target: lenMergeBlock})

		l.currentFn.Blocks = append(l.currentFn.Blocks, lenMergeBlock)
		l.currBlock = lenMergeBlock
		lt := l.currentFn.NewTemp(TypeI64, fmt.Sprintf("fe_len_%d", feID))
		l.currBlock.AddInstruction(&LoadInst{Dest: lt, Ptr: lenPtr})
		lenTemp = lt
	} else {
		lt := l.currentFn.NewTemp(TypeI64, fmt.Sprintf("fe_len_%d", feID))
		lenCallee := "len"
		if isMap {
			lenCallee = "joss_map_len"
		}
		l.currBlock.AddInstruction(&CallInst{
			Dest:   lt,
			Callee: lenCallee,
			Args:   []Value{iterVal},
		})
		lenTemp = lt
	}

	elemType := TypeMixed
	if iterVal.Type().Kind == TypeKindArray && iterVal.Type().Elem != nil {
		elemType = *iterVal.Type().Elem
	} else if isMap && iterVal.Type().Elem != nil && len(iterVal.Type().Elem.Fields) > 0 {
		elemType = iterVal.Type().Elem.Fields[0]
	} else if l.prep != nil && l.prep.Facts != nil {
		if t, ok := l.prep.Facts.InferredTypes[s.Iterable]; ok && t.Element != nil && t.Element.Kind != typesystem.Unknown {
			elemType = l.mapTypeSystemType(*t.Element)
		}
	}

	valName := cleanVarName(s.Value)
	valPtr, ok := l.varLocals[valName]
	if !ok {
		newValPtr := l.currentFn.NewTemp(PtrType(elemType), valName+"_ptr")
		l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: newValPtr, AllocType: elemType})
		l.varLocals[valName] = newValPtr
		valPtr = newValPtr
	}
	targetElemType := valPtr.Type().ElemType()

	var keyPtr Value
	keyType := TypeI64
	if isMap {
		keyType = TypeString
	} else if isDynamic {
		keyType = TypeMixed
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

	condBlock := l.currentFn.NewBlock(fmt.Sprintf("foreach_cond_%d_", feID))
	bodyBlock := l.currentFn.NewBlock(fmt.Sprintf("foreach_body_%d_", feID))
	stepBlock := l.newDetachedBlock(fmt.Sprintf("foreach_step_%d_", feID))
	afterBlock := l.newDetachedBlock(fmt.Sprintf("foreach_exit_%d_", feID))

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

	if isDynamic {
		bodyMapBlock := l.currentFn.NewBlock(fmt.Sprintf("foreach_bmap_%d_", feID))
		bodyArrBlock := l.newDetachedBlock(fmt.Sprintf("foreach_barr_%d_", feID))
		bodyContBlock := l.newDetachedBlock(fmt.Sprintf("foreach_bcont_%d_", feID))

		l.currBlock.SetTerminator(&BranchTerminator{
			Cond:       isMapRt,
			TrueBlock:  bodyMapBlock,
			FalseBlock: bodyArrBlock,
		})

		l.currBlock = bodyMapBlock
		if keyPtr != nil {
			kVal := l.currentFn.NewTemp(TypeString, fmt.Sprintf("fe_mkey_%d", feID))
			l.currBlock.AddInstruction(&CallInst{
				Dest:   kVal,
				Callee: "joss_map_key_at",
				Args:   []Value{iterVal, bodyIdx},
			})
			targetKeyType := keyPtr.Type().ElemType()
			storeKey := Value(kVal)
			if isScalarBoxedKind(targetKeyType.Kind) {
				storeKey = l.unboxIfTyped(l.boxIfMixed(kVal, TypeMixed), targetKeyType)
			}
			l.currBlock.AddInstruction(&StoreInst{Val: storeKey, Ptr: keyPtr})
		}
		mVal := l.currentFn.NewTemp(TypeMixed, fmt.Sprintf("fe_mval_%d", feID))
		l.currBlock.AddInstruction(&CallInst{
			Dest:   mVal,
			Callee: "joss_map_val_at",
			Args:   []Value{iterVal, bodyIdx},
		})
		storeMVal := l.boxIfMixed(mVal, targetElemType)
		storeMVal = l.unboxIfTyped(storeMVal, targetElemType)
		l.currBlock.AddInstruction(&StoreInst{Val: storeMVal, Ptr: valPtr})
		l.currBlock.SetTerminator(&JumpTerminator{Target: bodyContBlock})

		l.currentFn.Blocks = append(l.currentFn.Blocks, bodyArrBlock)
		l.currBlock = bodyArrBlock
		if keyPtr != nil {
			targetKeyType := keyPtr.Type().ElemType()
			storeKey := l.boxIfMixed(bodyIdx, targetKeyType)
			storeKey = l.unboxIfTyped(storeKey, targetKeyType)
			l.currBlock.AddInstruction(&StoreInst{Val: storeKey, Ptr: keyPtr})
		}
		aVal := l.currentFn.NewTemp(TypeMixed, fmt.Sprintf("fe_aval_%d", feID))
		l.currBlock.AddInstruction(&ArrayGetInst{
			Dest:  aVal,
			Array: iterVal,
			Index: bodyIdx,
		})
		storeAVal := l.boxIfMixed(aVal, targetElemType)
		storeAVal = l.unboxIfTyped(storeAVal, targetElemType)
		l.currBlock.AddInstruction(&StoreInst{Val: storeAVal, Ptr: valPtr})
		l.currBlock.SetTerminator(&JumpTerminator{Target: bodyContBlock})

		l.currentFn.Blocks = append(l.currentFn.Blocks, bodyContBlock)
		l.currBlock = bodyContBlock
	} else {
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
				targetKeyType := keyPtr.Type().ElemType()
				storeKey := l.boxIfMixed(bodyIdx, targetKeyType)
				storeKey = l.unboxIfTyped(storeKey, targetKeyType)
				l.currBlock.AddInstruction(&StoreInst{Val: storeKey, Ptr: keyPtr})
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
		storeVal := l.boxIfMixed(elemVal, targetElemType)
		storeVal = l.unboxIfTyped(storeVal, targetElemType)
		l.currBlock.AddInstruction(&StoreInst{Val: storeVal, Ptr: valPtr})
	}

	l.loopStack = append(l.loopStack, loopContext{continueBlock: stepBlock, breakBlock: afterBlock})
	if err := l.lowerBlock(s.Body); err != nil {
		return err
	}
	l.loopStack = l.loopStack[:len(l.loopStack)-1]

	if l.currBlock.Terminator == nil {
		if strings.HasPrefix(l.currBlock.Label, "dead") {
			l.currBlock.SetTerminator(&UnreachableTerminator{})
		} else {
			l.currBlock.SetTerminator(&JumpTerminator{Target: stepBlock})
		}
	}

	// Step: idx = idx + 1
	l.currentFn.Blocks = append(l.currentFn.Blocks, stepBlock)
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

	l.currentFn.Blocks = append(l.currentFn.Blocks, afterBlock)
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
	tryBlocks := cleanTryCatchBlocks(append([]*BasicBlock(nil), l.currentFn.Blocks[startTryIdx:]...))
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
	catchBlocks := cleanTryCatchBlocks(append([]*BasicBlock(nil), l.currentFn.Blocks[startCatchIdx:]...))
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
	mergeBlock := l.newDetachedBlock("ternary_merge")

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
			if strings.HasPrefix(l.currBlock.Label, "dead") {
				l.currBlock.SetTerminator(&UnreachableTerminator{})
			} else {
				l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
			}
		}
		l.currentFn.Blocks = append(l.currentFn.Blocks, mergeBlock)
		l.currBlock = mergeBlock
		return nil
	}

	falseBlock := l.newDetachedBlock("ternary_false")
	l.currBlock.SetTerminator(&BranchTerminator{
		Cond:       condVal,
		TrueBlock:  trueBlock,
		FalseBlock: falseBlock,
	})

	mergeReachable := false
	l.currBlock = trueBlock
	if e.True != nil {
		if _, err := l.lowerExpression(e.True); err != nil {
			return err
		}
	}
	if l.currBlock.Terminator == nil {
		if strings.HasPrefix(l.currBlock.Label, "dead") {
			l.currBlock.SetTerminator(&UnreachableTerminator{})
		} else {
			l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
			mergeReachable = true
		}
	}

	l.currentFn.Blocks = append(l.currentFn.Blocks, falseBlock)
	l.currBlock = falseBlock
	if e.False != nil {
		if _, err := l.lowerExpression(e.False); err != nil {
			return err
		}
	}
	if l.currBlock.Terminator == nil {
		if strings.HasPrefix(l.currBlock.Label, "dead") {
			l.currBlock.SetTerminator(&UnreachableTerminator{})
		} else {
			l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
			mergeReachable = true
		}
	}

	if !mergeReachable {
		l.currBlock = l.currentFn.NewBlock("dead")
		return nil
	}
	l.currentFn.Blocks = append(l.currentFn.Blocks, mergeBlock)
	l.currBlock = mergeBlock
	return nil
}

func (l *Lowerer) lowerReturnTernary(e *parser.TernaryExpression) error {
	condVal, err := l.lowerExpression(e.Condition)
	if err != nil {
		return err
	}
	condVal = l.ensureBoolCond(condVal)

	trueBlock := l.currentFn.NewBlock("ret_true")
	falseBlock := l.currentFn.NewBlock("ret_false")
	l.currBlock.SetTerminator(&BranchTerminator{
		Cond:       condVal,
		TrueBlock:  trueBlock,
		FalseBlock: falseBlock,
	})

	l.currBlock = trueBlock
	trueVal, err := l.lowerExpression(e.True)
	if err != nil {
		return err
	}
	if l.currBlock.Terminator == nil {
		l.currBlock.SetTerminator(&ReturnTerminator{Val: trueVal})
	}

	l.currBlock = falseBlock
	falseVal, err := l.lowerExpression(e.False)
	if err != nil {
		return err
	}
	if l.currBlock.Terminator == nil {
		l.currBlock.SetTerminator(&ReturnTerminator{Val: falseVal})
	}

	l.currBlock = l.currentFn.NewBlock("dead")
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
			case "auth_check", "auth_guest":
				resType = TypeBool
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
		if e.Operator == "|>" {
			if callExpr, ok := e.Right.(*parser.CallExpression); ok {
				args := make([]parser.Expression, 0, len(callExpr.Arguments)+1)
				args = append(args, e.Left)
				args = append(args, callExpr.Arguments...)
				synth := &parser.CallExpression{
					Token:     callExpr.Token,
					Function:  callExpr.Function,
					Arguments: args,
				}
				return l.lowerExpression(synth)
			}
			synth := &parser.CallExpression{
				Token:     e.Token,
				Function:  e.Right,
				Arguments: []parser.Expression{e.Left},
			}
			return l.lowerExpression(synth)
		}
		if e.Operator == "??" {
			left, err := l.lowerExpression(e.Left)
			if err != nil {
				return nil, err
			}
			if isScalarBoxedKind(left.Type().Kind) {
				return left, nil
			}
			cond := l.currentFn.NewTemp(TypeBool, "not_null")
			l.currBlock.AddInstruction(&CallRuntimeInst{
				Dest: cond,
				Func: "joss_is_not_null",
				Args: []Value{left},
			})
			trueBlock := l.currentFn.NewBlock("coal_keep")
			falseBlock := l.newDetachedBlock("coal_fallback")
			mergeBlock := l.newDetachedBlock("coal_merge")

			l.currBlock.SetTerminator(&BranchTerminator{
				Cond:       cond,
				TrueBlock:  trueBlock,
				FalseBlock: falseBlock,
			})

			l.currentFn.Blocks = append(l.currentFn.Blocks, falseBlock)
			l.currBlock = falseBlock
			right, err := l.lowerExpression(e.Right)
			if err != nil {
				return nil, err
			}
			falseEndBlock := l.currBlock

			resType := left.Type()
			if resType.Kind == TypeKindPtr && resType.Elem != nil && resType.Elem.Kind == TypeKindVoid {
				if l.prep != nil && l.prep.Facts != nil {
					if inferred, ok := l.prep.Facts.InferredTypes[e]; ok && inferred.Kind != typesystem.Unknown && inferred.Kind != typesystem.Union {
						mapped := l.mapTypeSystemType(inferred)
						if mapped.Kind != TypeKindUnknown && mapped.Kind != TypeKindVoid {
							resType = mapped
						}
					}
				}
				if resType.Kind == TypeKindPtr && resType.Elem != nil && resType.Elem.Kind == TypeKindVoid {
					resType = right.Type()
				}
			}
			if resType.Kind == TypeKindUnknown || resType.Kind == TypeKindUnion {
				resType = right.Type()
			}

			resPtr := l.currentFn.NewTemp(PtrType(resType), "coal_ptr")
			l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: resPtr, AllocType: resType})

			keepVal := l.boxIfMixed(left, resType)
			if _, isConstNull := left.(*ConstNull); isConstNull && !(resType.Kind == TypeKindPtr && resType.Elem != nil && resType.Elem.Kind == TypeKindVoid) {
				if resType.Kind == TypeKindString {
					keepVal = NewConstString("")
				} else if isScalarBoxedKind(resType.Kind) {
					keepVal = NewConstInt(0, resType)
				} else {
					keepVal = NewConstNull(resType)
				}
			}
			trueBlock.AddInstruction(&StoreInst{Val: keepVal, Ptr: resPtr})
			trueBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})

			if falseEndBlock.Terminator == nil {
				if strings.HasPrefix(falseEndBlock.Label, "dead") {
					falseEndBlock.SetTerminator(&UnreachableTerminator{})
				} else {
					l.currBlock = falseEndBlock
					storeRight := l.boxIfMixed(right, resType)
					storeRight = l.unboxIfTyped(storeRight, resType)
					falseEndBlock.AddInstruction(&StoreInst{Val: storeRight, Ptr: resPtr})
					falseEndBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
				}
			}

			l.currentFn.Blocks = append(l.currentFn.Blocks, mergeBlock)
			l.currBlock = mergeBlock
			resTemp := l.currentFn.NewTemp(resType, "coal_res")
			l.currBlock.AddInstruction(&LoadInst{Dest: resTemp, Ptr: resPtr})
			return resTemp, nil
		}

		if e.Operator == "&&" || e.Operator == "and" {
			left, err := l.lowerExpression(e.Left)
			if err != nil {
				return nil, err
			}
			cond := l.ensureBoolCond(left)
			rightBlock := l.currentFn.NewBlock("and_right")
			mergeBlock := l.newDetachedBlock("and_merge")

			resPtr := l.currentFn.NewTemp(PtrType(TypeBool), "and_ptr")
			l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: resPtr, AllocType: TypeBool})
			l.currBlock.AddInstruction(&StoreInst{Val: NewConstBool(false), Ptr: resPtr})

			l.currBlock.SetTerminator(&BranchTerminator{
				Cond:       cond,
				TrueBlock:  rightBlock,
				FalseBlock: mergeBlock,
			})

			l.currBlock = rightBlock
			right, err := l.lowerExpression(e.Right)
			if err != nil {
				return nil, err
			}
			rCond := l.ensureBoolCond(right)
			if l.currBlock.Terminator == nil {
				if strings.HasPrefix(l.currBlock.Label, "dead") {
					l.currBlock.SetTerminator(&UnreachableTerminator{})
				} else {
					l.currBlock.AddInstruction(&StoreInst{Val: rCond, Ptr: resPtr})
					l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
				}
			}

			l.currentFn.Blocks = append(l.currentFn.Blocks, mergeBlock)
			l.currBlock = mergeBlock
			resTemp := l.currentFn.NewTemp(TypeBool, "and_res")
			l.currBlock.AddInstruction(&LoadInst{Dest: resTemp, Ptr: resPtr})
			return resTemp, nil
		}

		if e.Operator == "||" || e.Operator == "or" {
			left, err := l.lowerExpression(e.Left)
			if err != nil {
				return nil, err
			}
			cond := l.ensureBoolCond(left)
			rightBlock := l.currentFn.NewBlock("or_right")
			mergeBlock := l.newDetachedBlock("or_merge")

			resPtr := l.currentFn.NewTemp(PtrType(TypeBool), "or_ptr")
			l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: resPtr, AllocType: TypeBool})
			l.currBlock.AddInstruction(&StoreInst{Val: NewConstBool(true), Ptr: resPtr})

			l.currBlock.SetTerminator(&BranchTerminator{
				Cond:       cond,
				TrueBlock:  mergeBlock,
				FalseBlock: rightBlock,
			})

			l.currBlock = rightBlock
			right, err := l.lowerExpression(e.Right)
			if err != nil {
				return nil, err
			}
			rCond := l.ensureBoolCond(right)
			if l.currBlock.Terminator == nil {
				if strings.HasPrefix(l.currBlock.Label, "dead") {
					l.currBlock.SetTerminator(&UnreachableTerminator{})
				} else {
					l.currBlock.AddInstruction(&StoreInst{Val: rCond, Ptr: resPtr})
					l.currBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
				}
			}

			l.currentFn.Blocks = append(l.currentFn.Blocks, mergeBlock)
			l.currBlock = mergeBlock
			resTemp := l.currentFn.NewTemp(TypeBool, "or_res")
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
			if left.Type().Kind == TypeKindString && right.Type().Kind == TypeKindI64 {
				dest := l.currentFn.NewTemp(TypeString, "strcat_i64")
				l.currBlock.AddInstruction(&CallRuntimeInst{
					Dest: dest,
					Func: "str_concat_i64",
					Args: []Value{left, right},
				})
				return dest, nil
			}
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
			cmpLeft := left
			cmpRight := right
			isStrictBoolMixed := (e.Operator == "===" || e.Operator == "!==") &&
				((cmpLeft.Type().Kind == TypeKindMixed && cmpRight.Type().Kind == TypeKindBool) ||
					(cmpRight.Type().Kind == TypeKindMixed && cmpLeft.Type().Kind == TypeKindBool))
			if !isStrictBoolMixed {
				if cmpLeft.Type().Kind == TypeKindMixed && cmpRight.Type().Kind != TypeKindString && cmpRight.Type().Kind != TypeKindMixed {
					cmpLeft = l.unboxIfMixed(cmpLeft)
				}
				if cmpRight.Type().Kind == TypeKindMixed && cmpLeft.Type().Kind != TypeKindString && cmpLeft.Type().Kind != TypeKindMixed {
					cmpRight = l.unboxIfMixed(cmpRight)
				}
			}
			destType := TypeBool
			if op == OpSpaceship {
				destType = TypeI64
			}
			dest := l.currentFn.NewTemp(destType, "cmp")
			l.currBlock.AddInstruction(&CompareInst{
				Op:    op,
				Dest:  dest,
				Left:  cmpLeft,
				Right: cmpRight,
			})
			return dest, nil
		}

		isIntArith := (op == OpAdd || op == OpSub || op == OpMul || op == OpDiv || op == OpMod) &&
			!left.Type().IsFloat() && !right.Type().IsFloat() &&
			left.Type().Kind != TypeKindDecimal && right.Type().Kind != TypeKindDecimal
		if isIntArith && (left.Type().Kind == TypeKindMixed || left.Type().Kind == TypeKindString) {
			convL := l.currentFn.NewTemp(TypeI64, "arith_l")
			l.currBlock.AddInstruction(&CallBuiltinInst{
				Dest:      convL,
				BuiltinID: "intval",
				Args:      []Value{left},
			})
			left = convL
		} else {
			left = l.unboxIfMixed(left)
		}
		if isIntArith && (right.Type().Kind == TypeKindMixed || right.Type().Kind == TypeKindString) {
			convR := l.currentFn.NewTemp(TypeI64, "arith_r")
			l.currBlock.AddInstruction(&CallBuiltinInst{
				Dest:      convR,
				BuiltinID: "intval",
				Args:      []Value{right},
			})
			right = convR
		} else {
			right = l.unboxIfMixed(right)
		}
		{
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
		right = l.unboxIfMixed(right)
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
				varName := cleanVarName(ident.Value)
				if ptr, exists := l.varLocals[varName]; exists {
					l.currBlock.AddInstruction(&StoreInst{Val: dest, Ptr: ptr})
					if l.activeClosureEnvMap != nil && l.activeClosureCaptured[varName] {
						l.currBlock.AddInstruction(&MapSetInst{
							Map: l.activeClosureEnvMap,
							Key: NewConstString(varName),
							Val: l.boxIfMixed(dest, TypeMixed),
						})
					}
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
		origVal = l.unboxIfMixed(origVal)
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
				if l.activeClosureEnvMap != nil && l.activeClosureCaptured[varName] {
					l.currBlock.AddInstruction(&MapSetInst{
						Map: l.activeClosureEnvMap,
						Key: NewConstString(varName),
						Val: l.boxIfMixed(newVal, TypeMixed),
					})
				}
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
		falseBlock := l.newDetachedBlock("ternary_false")
		mergeBlock := l.newDetachedBlock("ternary_merge")

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
		trueEndBlock := l.currBlock
		trueDead := strings.HasPrefix(trueEndBlock.Label, "dead")

		// Rama falsa
		l.currentFn.Blocks = append(l.currentFn.Blocks, falseBlock)
		l.currBlock = falseBlock
		var falseVal Value
		if e.False != nil {
			fv, err := l.lowerExpression(e.False)
			if err != nil {
				return nil, err
			}
			falseVal = fv
		}
		falseEndBlock := l.currBlock
		falseDead := strings.HasPrefix(falseEndBlock.Label, "dead")

		resType := trueVal.Type()
		if trueDead && !falseDead && falseVal != nil {
			resType = falseVal.Type()
		}
		if falseVal == nil {
			if resType.Kind == TypeKindString {
				falseVal = NewConstString("")
			} else if resType.Kind == TypeKindPtr || resType.Kind == TypeKindStruct || resType.Kind == TypeKindArray {
				falseVal = NewConstNull(resType)
			} else {
				falseVal = NewConstInt(0, resType)
			}
		}

		resPtr := l.currentFn.NewTemp(PtrType(resType), "ternary_ptr")
		l.currentFn.EntryBlock.AddInstruction(&AllocaInst{Dest: resPtr, AllocType: resType})

		mergeReachable := false
		if trueEndBlock.Terminator == nil {
			if trueDead {
				trueEndBlock.SetTerminator(&UnreachableTerminator{})
			} else {
				l.currBlock = trueEndBlock
				storeT := l.boxIfMixed(trueVal, resType)
				storeT = l.unboxIfTyped(storeT, resType)
				trueEndBlock.AddInstruction(&StoreInst{Val: storeT, Ptr: resPtr})
				trueEndBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
				mergeReachable = true
			}
		}

		if falseEndBlock.Terminator == nil {
			if falseDead {
				falseEndBlock.SetTerminator(&UnreachableTerminator{})
			} else {
				l.currBlock = falseEndBlock
				storeF := l.boxIfMixed(falseVal, resType)
				storeF = l.unboxIfTyped(storeF, resType)
				falseEndBlock.AddInstruction(&StoreInst{Val: storeF, Ptr: resPtr})
				falseEndBlock.SetTerminator(&JumpTerminator{Target: mergeBlock})
				mergeReachable = true
			}
		}

		if !mergeReachable {
			l.currBlock = l.currentFn.NewBlock("dead")
			return NewConstInt(0, resType), nil
		}

		// Merge (después de bajar trueBlock y falseBlock)
		l.currentFn.Blocks = append(l.currentFn.Blocks, mergeBlock)
		l.currBlock = mergeBlock
		resTemp := l.currentFn.NewTemp(resType, "ternary_res")
		l.currBlock.AddInstruction(&LoadInst{Dest: resTemp, Ptr: resPtr})
		return resTemp, nil

	case *parser.NewExpression:
		className := e.Class.Value
		if className == "ByteBuffer" {
			var capVal Value = NewConstInt(64, TypeI64)
			if len(e.Arguments) > 0 {
				v, err := l.lowerExpression(e.Arguments[0])
				if err != nil {
					return nil, err
				}
				capVal = l.unboxIfMixed(v)
			}
			bufType := PtrType(StructType("ByteBuffer", nil))
			bufTemp := l.currentFn.NewTemp(bufType, "buf")
			l.currBlock.AddInstruction(&AllocBufferInst{Dest: bufTemp, Cap: capVal})
			return bufTemp, nil
		}
		if className == "StringBuilder" {
			sbType := PtrType(StructType("StringBuilder", nil))
			sbTemp := l.currentFn.NewTemp(sbType, "sb")
			l.currBlock.AddInstruction(&CallRuntimeInst{Dest: sbTemp, Func: "joss_sb_new"})
			return sbTemp, nil
		}
		if l.isModelClass(className) {
			objType := PtrType(StructType(className, nil))
			objTemp := l.currentFn.NewTemp(objType, "model_obj")
			l.currBlock.AddInstruction(&CallInst{
				Dest:   objTemp,
				Callee: "fn_Model_newInstance",
				Args:   []Value{NewConstString(className)},
			})
			if l.prog.Functions[className+"_Init"] != nil {
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
					Callee: className + "_Init",
					Args:   ctorArgs,
				})
			}
			return objTemp, nil
		}
		objType := PtrType(StructType(className, nil))
		objTemp := l.currentFn.NewTemp(objType, "obj")
		l.currBlock.AddInstruction(&NewObjectInst{
			Dest:      objTemp,
			ClassName: className,
		})

		if className == "Process" {
			ctorArgs := []Value{objTemp}
			for _, arg := range e.Arguments {
				v, err := l.lowerExpression(arg)
				if err != nil {
					return nil, err
				}
				ctorArgs = append(ctorArgs, v)
			}
			l.currBlock.AddInstruction(&CallHostServiceInst{
				ClassName:  "Process",
				MethodName: "constructor",
				Recv:       objTemp,
				Args:       ctorArgs,
			})
			return objTemp, nil
		}

		// Inicializar propiedades por defecto de la jerarquía y detectar constructor propio
		hierarchy := l.collectClassHierarchy(className)
		if len(hierarchy) > 1 || l.classHasSubclasses(className) {
			l.currBlock.AddInstruction(&StoreFieldInst{
				Val:       NewConstString(className),
				Obj:       objTemp,
				FieldName: "__joss_cls",
			})
		}

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

		inheritsGranDB := false
		explicitTableName := ""
		for _, cs := range hierarchy {
			if cs == nil || cs.Name == nil {
				continue
			}
			if cs.Name.Value == "GranDB" || cs.Name.Value == "Model" {
				inheritsGranDB = true
			}
			if cs.Body == nil {
				continue
			}
			isOwnClass := cs.Name.Value == className
			for _, member := range cs.Body.Statements {
				switch m := member.(type) {
				case *parser.LetStatement:
					if m.IsStatic || m.Name == nil {
						continue
					}
					fName := cleanVarName(m.Name.Value)
					if isOwnClass && (fName == "tabla" || fName == "tableName") {
						if sl, ok := m.Value.(*parser.StringLiteral); ok && sl.Value != "" {
							explicitTableName = sl.Value
						}
					}
					if m.Value != nil {
						val, err := l.lowerExpression(m.Value)
						if err == nil && !isZeroInitConstant(val) {
							if m.Token.Literal != "" {
								fType := l.mapType(m.Token.Literal)
								val = l.boxIfMixed(val, fType)
								val = l.unboxIfTyped(val, fType)
							}
							l.currBlock.AddInstruction(&StoreFieldInst{
								Val:       val,
								Obj:       objTemp,
								FieldName: fName,
							})
						}
					}
				case *parser.MultiLetStatement:
					if m.IsStatic {
						continue
					}
					for _, d := range m.Declarations {
						if d.Name == nil || d.Value == nil {
							continue
						}
						fName := cleanVarName(d.Name.Value)
						val, err := l.lowerExpression(d.Value)
						if err == nil && !isZeroInitConstant(val) {
							if m.TypeToken.Literal != "" {
								fType := l.mapType(m.TypeToken.Literal)
								val = l.boxIfMixed(val, fType)
								val = l.unboxIfTyped(val, fType)
							}
							l.currBlock.AddInstruction(&StoreFieldInst{
								Val:       val,
								Obj:       objTemp,
								FieldName: fName,
							})
						}
					}
				case *parser.InitStatement:
					if isOwnClass && !hasCtor {
						hasCtor = true
						initName := "Init"
						if m.Name != nil && m.Name.Value != "" {
							initName = m.Name.Value
						}
						ctorName = className + "_" + initName
					}
				case *parser.MethodStatement:
					if isOwnClass && !hasCtor && (m.Name.Value == "Init" || m.Name.Value == "constructor") {
						hasCtor = true
						ctorName = className + "_" + m.Name.Value
					}
				}
			}
		}

		if inheritsGranDB && className != "GranDB" && className != "Model" {
			inferredTable := explicitTableName
			if inferredTable == "" {
				inferredTable = strings.ToLower(pluralizeIRClassName(className))
			}
			if inferredTable != "" {
				l.currBlock.AddInstruction(&StoreFieldInst{
					Val:       NewConstString(inferredTable),
					Obj:       objTemp,
					FieldName: "tableName",
				})
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
		if e.Token.Type == parser.DOUBLE_COLON {
			if identLeft, ok := e.Left.(*parser.Identifier); ok {
				if global := l.ensureStaticPropertyGlobal(identLeft.Value, e.Property.Value); global != nil {
					globalName := staticPropertyGlobalName(identLeft.Value, e.Property.Value)
					resTemp := l.currentFn.NewTemp(global.Type, "static_prop")
					l.currBlock.AddInstruction(&GlobalLoadInst{Dest: resTemp, GlobalName: globalName})
					return resTemp, nil
				}
				// Acceso estático a constante de clase o caso de enum: Class::CONST / Enum::CASE
				return NewConstString(identLeft.Value + "::" + e.Property.Value), nil
			}
		}
		objVal, err := l.lowerExpression(e.Left)
		if err != nil {
			return nil, err
		}
		propName := e.Property.Value
		if l.isModelType(objVal.Type()) {
			resTemp := l.currentFn.NewTemp(TypeMixed, "model_attr")
			l.currBlock.AddInstruction(&CallInst{
				Dest:   resTemp,
				Callee: "fn_Model_getAttr",
				Args:   []Value{objVal, NewConstString(propName)},
			})
			return resTemp, nil
		}
		fieldType := l.inferNodeType(e)
		resolvedClassProp := false
		if (fieldType.Kind == TypeKindI64 || fieldType.Kind == TypeKindUnion || fieldType.Kind == TypeKindUnknown) && objVal.Type().Kind == TypeKindPtr && objVal.Type().Elem != nil && objVal.Type().Elem.Name != "map" {
			if resolvedPropType, ok := l.lookupClassPropertyType(objVal.Type().Elem.Name, propName); ok {
				fieldType = resolvedPropType
				resolvedClassProp = true
			}
		}
		if fieldType.Kind == TypeKindI64 && !resolvedClassProp {
			isExplicitInt := false
			if l.prep != nil && l.prep.Facts != nil {
				if t, ok := l.prep.Facts.InferredTypes[e]; ok && t.Kind == typesystem.Int {
					isExplicitInt = true
				}
			}
			if !isExplicitInt {
				fieldType = TypeMixed
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
		elemType := TypeMixed
		if len(e.Elements) > 0 {
			elemType = TypeI64
		}
		mixedElems := false
		elements := make([]Value, len(e.Elements))
		for idx, el := range e.Elements {
			v, err := l.lowerExpression(el)
			if err != nil {
				return nil, err
			}
			elements[idx] = v
			if idx == 0 {
				elemType = v.Type()
			} else if !mixedElems && v.Type().Kind != elemType.Kind {
				if (elemType.IsInteger() && v.Type().IsInteger()) || (elemType.IsFloat() && v.Type().IsFloat()) {
					continue
				}
				mixedElems = true
				elemType = TypeMixed
			}
		}
		if elemType.Kind == TypeKindMixed {
			for idx := range elements {
				elements[idx] = l.boxIfMixed(elements[idx], TypeMixed)
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
		valType := TypeMixed
		for k, v := range e.Pairs {
			kVal, err := l.lowerExpression(k)
			if err != nil {
				return nil, err
			}
			vVal, err := l.lowerExpression(v)
			if err != nil {
				return nil, err
			}
			if len(pairs) == 0 {
				valType = vVal.Type()
			} else if valType.Kind != vVal.Type().Kind {
				valType = TypeMixed
			}
			pairs = append(pairs, [2]Value{kVal, vVal})
		}
		for idx := range pairs {
			if isScalarBoxedKind(pairs[idx][1].Type().Kind) {
				pairs[idx][1] = l.boxIfMixed(pairs[idx][1], TypeMixed)
				valType = TypeMixed
			}
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
				resType = TypeMixed
			}
			resTemp := l.currentFn.NewTemp(resType, "elem")
			l.currBlock.AddInstruction(&MapGetInst{
				Dest: resTemp,
				Map:  container,
				Key:  idxVal,
			})
			return resTemp, nil
		}
		if container.Type().Kind == TypeKindArray && container.Type().Elem != nil && l.isModelType(*container.Type().Elem) {
			resType = *container.Type().Elem
		} else if resType.Kind == TypeKindI64 {
			if container.Type().Kind == TypeKindArray && container.Type().Elem != nil {
				resType = *container.Type().Elem
			} else if container.Type().Kind == TypeKindMixed {
				resType = TypeMixed
			}
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
		var resolvedMethodStmt *parser.MethodStatement
		var indirectCallee Value

		if member, ok := e.Function.(*parser.MemberExpression); ok {
			if identLeft, ok := member.Left.(*parser.Identifier); ok && member.Property != nil {
				// Podría ser estático Class::method o $obj->method
				if ptr, ok := l.varLocals[cleanVarName(identLeft.Value)]; ok {
					dest := l.currentFn.NewTemp(ptr.Type().ElemType(), cleanVarName(identLeft.Value))
					l.currBlock.AddInstruction(&LoadInst{Dest: dest, Ptr: ptr})
					receiverObj = dest
					calleeName = member.Property.Value
				} else {
					if l.isModelClass(identLeft.Value) {
						if _, _, _, found := l.resolveClassMethod(identLeft.Value, member.Property.Value); !found {
							if res, handled, err := l.lowerModelStaticCall(identLeft.Value, member.Property.Value, e.Arguments); handled {
								return res, err
							}
						}
					}
					calleeName = identLeft.Value + "_" + member.Property.Value
					if targetFn, _, mStmt, found := l.resolveClassMethod(identLeft.Value, member.Property.Value); found {
						calleeName = targetFn
						resolvedMethodStmt = mStmt
					}
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
					if targetFn, _, mStmt, found := l.resolveClassMethod(clsName, member.Property.Value); found {
						calleeName = targetFn
						resolvedMethodStmt = mStmt
					} else if l.isModelType(receiverObj.Type()) {
						if res, handled, err := l.lowerModelInstanceCall(receiverObj, member.Property.Value, e.Arguments); handled {
							return res, err
						}
						calleeName = clsName + "_" + member.Property.Value
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
				indirectCallee = fnVal
			}
		} else {
			fnVal, err := l.lowerExpression(e.Function)
			if err != nil {
				return nil, err
			}
			indirectCallee = fnVal
		}

		if indirectCallee != nil {
			var args []Value
			for _, a := range e.Arguments {
				v, err := l.lowerExpression(a)
				if err != nil {
					return nil, err
				}
				args = append(args, v)
			}
			retType := TypeMixed
			resolvedIndirectRet := false
			if l.prep != nil && l.prep.Facts != nil {
				if resolved, ok := l.prep.Facts.ResolvedCalls[e]; ok && resolved.ReturnType.Kind != typesystem.Unknown {
					retType = l.mapTypeSystemType(resolved.ReturnType)
					resolvedIndirectRet = true
				}
				if !resolvedIndirectRet {
					if inferred, ok := l.prep.Facts.InferredTypes[e]; ok && inferred.Kind != typesystem.Unknown {
						retType = l.mapTypeSystemType(inferred)
						resolvedIndirectRet = true
					}
				}
			}
			if !resolvedIndirectRet && indirectCallee.Type().Kind == TypeKindFunc && indirectCallee.Type().Return != nil && indirectCallee.Type().Return.Kind != TypeKindUnknown {
				retType = *indirectCallee.Type().Return
			}
			var dest *TempValue
			if retType.Kind != TypeKindVoid {
				dest = l.currentFn.NewTemp(retType, "call_ret")
			}
			l.currBlock.AddInstruction(&CallIndirectInst{
				Dest:      dest,
				CalleeVal: indirectCallee,
				Args:      args,
			})
			if dest != nil {
				return dest, nil
			}
			return NewConstInt(0, TypeVoid), nil
		}

		if l.prep != nil && l.prep.Facts != nil {
			if resolved, ok := l.prep.Facts.ResolvedCalls[e]; ok {
				if resolved.Kind == analyzer.CallKindMethod {
					if lastColons := strings.LastIndex(resolved.TargetID, "::"); lastColons != -1 {
						prefix := resolved.TargetID[:lastColons]
						className := prefix[strings.LastIndex(prefix, ":")+1:]
						methodName := resolved.TargetID[lastColons+2:]
						if targetFn, _, mStmt, found := l.resolveClassMethod(className, methodName); found {
							calleeName = targetFn
							resolvedMethodStmt = mStmt
						} else {
							calleeName = className + "_" + methodName
						}
					}
				}
			}
		}

		if receiverObj != nil && calleeName == "GranDB_table" {
			if targetFn, _, mStmt, found := l.resolveClassMethod("GranDB", "setTable"); found {
				calleeName = targetFn
				resolvedMethodStmt = mStmt
			}
		}

		var args []Value
		if receiverObj != nil && (resolvedMethodStmt == nil || !resolvedMethodStmt.IsStatic) {
			args = append(args, receiverObj)
		}
		for _, a := range e.Arguments {
			v, err := l.lowerExpression(a)
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}

		// Auto-fill default arguments when fewer arguments were provided than callee declared
		if resolvedMethodStmt != nil {
			expectedParams := resolvedMethodStmt.Parameters
			startIdx := len(args)
			if receiverObj != nil && !resolvedMethodStmt.IsStatic {
				startIdx = len(args) - 1
			}
			for pIdx := startIdx; pIdx < len(expectedParams); pIdx++ {
				param := expectedParams[pIdx]
				if param.DefaultValue != nil {
					defVal, err := l.lowerExpression(param.DefaultValue)
					if err == nil && defVal != nil {
						args = append(args, defVal)
						continue
					}
				}
				args = append(args, NewConstNull(PtrType(TypeVoid)))
			}
		} else if fnStmt, foundFn := l.resolveFunction(calleeName); foundFn {
			for pIdx := len(args); pIdx < len(fnStmt.Parameters); pIdx++ {
				param := fnStmt.Parameters[pIdx]
				if param.DefaultValue != nil {
					defVal, err := l.lowerExpression(param.DefaultValue)
					if err == nil && defVal != nil {
						args = append(args, defVal)
						continue
					}
				}
				args = append(args, NewConstNull(PtrType(TypeVoid)))
			}
		}
		// GranDB::select accepts both strings and arrays for interpreter
		// semantics. At statically typed call sites, lower directly to the
		// corresponding Pure Joss helper so native modes do not need a runtime
		// mixed-value type tag just to choose between those two forms.
		if calleeName == "GranDB_select" && len(args) > 1 {
			switch args[1].Type().Kind {
			case TypeKindString:
				calleeName = "GranDB_selectString"
			case TypeKindArray:
				calleeName = "GranDB_selectArray"
			}
		}

		if (calleeName == "GranDB_table" || calleeName == "table") && len(args) == 2 {
			calleeName = "GranDB_table"
			args = []Value{args[1]}
		}
		if calleeName == "View_render" {
			calleeName = "view"
		}
		if (calleeName == "env" || calleeName == "request" || calleeName == "Request_input" || calleeName == "Session_get" || calleeName == "gmdate" || calleeName == "date") && len(args) == 1 {
			if calleeName == "Session_get" {
				args = append(args, NewConstNull(PtrType(TypeVoid)))
			} else {
				args = append(args, NewConstInt(0, TypeI64))
			}
		} else if calleeName == "response" {
			if len(args) == 1 {
				args = append(args, NewConstInt(200, TypeI64), NewConstString(""))
			} else if len(args) == 2 {
				args = append(args, NewConstString(""))
			}
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
		if !isBuiltin && !isHost && !isPrimitive && (retType.Kind == TypeKindI64 || retType.Kind == TypeKindMixed) {
			if ident, ok := e.Function.(*parser.Identifier); ok {
				if ptr, ok := l.varLocals[cleanVarName(ident.Value)]; ok {
					elemT := ptr.Type().ElemType()
					if elemT.Kind == TypeKindFunc && elemT.Return != nil && elemT.Return.Kind != TypeKindUnknown {
						retType = *elemT.Return
					} else if l.prep != nil && l.prep.Facts != nil {
						if inferred, ok := l.prep.Facts.InferredTypes[e]; ok && inferred.Kind != typesystem.Unknown {
							retType = l.mapTypeSystemType(inferred)
						} else if elemT.Kind == TypeKindMixed {
							retType = TypeMixed
						}
					} else if elemT.Kind == TypeKindMixed {
						retType = TypeMixed
					}
				}
			}
		}
		if !isBuiltin && !isHost && !isPrimitive && retType.Kind == TypeKindI64 {
			if resolvedMethodStmt != nil && strings.TrimSpace(resolvedMethodStmt.ReturnType.Literal) == "" {
				retType = l.inferMethodReturnType(resolvedMethodStmt)
			} else if receiverObj == nil {
				if fnStmt, foundFn := l.resolveFunction(calleeName); foundFn && fnStmt != nil && strings.TrimSpace(fnStmt.ReturnType.Literal) == "" {
					retType = l.inferMethodReturnType(fnStmt)
				}
			}
		}
		if receiverObj == nil {
			switch calleeName {
			case "Session_get", "Session_put", "Session_has", "Session_forget":
				isHost = true
				isBuiltin = false
				isPrimitive = false
				targetID = "Session::" + strings.TrimPrefix(calleeName, "Session_")
			}
		}
		if retType.Kind == TypeKindI64 && receiverObj != nil && receiverObj.Type().Kind == TypeKindPtr && receiverObj.Type().Elem != nil {
			if member, ok := e.Function.(*parser.MemberExpression); ok && member.Property != nil {
				if _, mRet, _, found := l.resolveClassMethod(receiverObj.Type().Elem.Name, member.Property.Value); found {
					retType = mRet
				}
			}
		}

		isBlueprintCall := false
		if member, ok := e.Function.(*parser.MemberExpression); ok && member.Property != nil && receiverObj != nil {
			mName := member.Property.Value
			if isBlueprintMethod(mName) {
				recvT := receiverObj.Type()
				isBPStatic := recvT.Kind == TypeKindPtr && recvT.Elem != nil && recvT.Elem.Name == "Blueprint"
				isDynamicBP := (recvT.Kind == TypeKindMixed || recvT.Kind == TypeKindI64 ||
					(recvT.Kind == TypeKindPtr && recvT.Elem != nil && recvT.Elem.Kind == TypeKindVoid)) &&
					calleeName == mName && l.prog.Functions[calleeName] == nil
				if isBPStatic || isDynamicBP {
					isBlueprintCall = true
					isHost = true
					isBuiltin = false
					isPrimitive = false
					targetID = "Blueprint::" + mName
					calleeName = "Blueprint_" + mName
					retType = PtrType(StructType("Blueprint", nil))
					switch mName {
					case "id":
						if len(args) == 1 {
							args = append(args, NewConstString("id"))
						}
					case "string", "char":
						if len(args) == 2 {
							args = append(args, NewConstInt(255, TypeI64))
						}
					case "decimal":
						if len(args) == 2 {
							args = append(args, NewConstInt(10, TypeI64), NewConstInt(2, TypeI64))
						} else if len(args) == 3 {
							args = append(args, NewConstInt(2, TypeI64))
						}
					case "index", "uniqueIndex", "foreign":
						if len(args) == 1 {
							args = append(args, NewConstInt(0, TypeI64), NewConstString(""))
						} else if len(args) == 2 {
							args = append(args, NewConstString(""))
						}
					case "dropIndex", "dropForeign":
						if len(args) == 1 {
							args = append(args, NewConstString(""))
						}
					case "unique":
						if len(args) == 1 {
							args = append(args, NewConstInt(0, TypeI64))
						}
					case "default":
						if len(args) >= 2 {
							args[1] = l.boxIfMixed(args[1], TypeMixed)
						}
					}
				}
			}
		}

		if calleeName == "Router_registerMiddleware" && len(args) >= 2 {
			regTemp := l.currentFn.NewTemp(TypeI64, "mw_reg")
			l.currBlock.AddInstruction(&CallRuntimeInst{
				Dest: regTemp,
				Func: "joss_router_register_middleware",
				Args: []Value{args[0], args[1]},
			})
			return regTemp, nil
		}

		// Native WebSocket connection object ($ws) method dispatch:
		// onMessage(cb), onClose(cb), subscribe(channel), send(msg) on mixed $ws, close() on mixed $ws.
		if receiverObj != nil {
			recvIsDynamicWS := receiverObj.Type().Kind == TypeKindMixed ||
				(receiverObj.Type().Kind == TypeKindPtr && receiverObj.Type().Elem != nil && receiverObj.Type().Elem.Name == "WebSocket")
			switch calleeName {
			case "onMessage":
				if len(args) >= 2 {
					retTmp := l.currentFn.NewTemp(TypeI64, "ws_onmsg")
					l.currBlock.AddInstruction(&CallRuntimeInst{
						Dest: retTmp,
						Func: "joss_ws_obj_on_message",
						Args: []Value{args[0], args[1]},
					})
					return retTmp, nil
				}
			case "onClose":
				if len(args) >= 2 {
					retTmp := l.currentFn.NewTemp(TypeI64, "ws_onclose")
					l.currBlock.AddInstruction(&CallRuntimeInst{
						Dest: retTmp,
						Func: "joss_ws_obj_on_close",
						Args: []Value{args[0], args[1]},
					})
					return retTmp, nil
				}
			case "subscribe":
				if len(args) >= 2 {
					retTmp := l.currentFn.NewTemp(TypeI64, "ws_sub")
					l.currBlock.AddInstruction(&CallRuntimeInst{
						Dest: retTmp,
						Func: "joss_ws_obj_subscribe",
						Args: []Value{args[0], args[1]},
					})
					return retTmp, nil
				}
			case "send":
				if recvIsDynamicWS && len(args) == 2 {
					retTmp := l.currentFn.NewTemp(TypeI64, "ws_send")
					l.currBlock.AddInstruction(&CallRuntimeInst{
						Dest: retTmp,
						Func: "joss_ws_obj_send",
						Args: []Value{args[0], args[1]},
					})
					return retTmp, nil
				}
			case "close":
				if recvIsDynamicWS && len(args) == 1 {
					retTmp := l.currentFn.NewTemp(TypeI64, "ws_close")
					l.currBlock.AddInstruction(&CallRuntimeInst{
						Dest: retTmp,
						Func: "joss_ws_obj_close",
						Args: []Value{args[0]},
					})
					return retTmp, nil
				}
			}
		}

		if !isBlueprintCall {
			if calleeName == "html_escape" && len(args) > 0 &&
				(args[0].Type().IsInteger() || args[0].Type().IsFloat() || args[0].Type().Kind == TypeKindBool) {
				convArg := l.currentFn.NewTemp(TypeString, "hesc_str")
				l.currBlock.AddInstruction(&CallRuntimeInst{
					Dest: convArg,
					Func: "to_string",
					Args: []Value{args[0]},
				})
				args[0] = convArg
			}
			switch calleeName {
			case "view", "SEO_render", "html_escape", "csrf_field", "Markdown_readFile", "Markdown_toHtml", "Str_trim", "file_get_contents", "__joss_byte_string", "toSql", "GranDB_toSql", "json", "response", "redirect", "with", "withCookie", "WebResponse_with", "WebResponse_withCookie", "Response_redirect", "Response_back", "substr", "str_replace", "strtolower", "strtoupper", "trim", "ltrim", "rtrim", "implode", "sprintf", "date", "now", "ucfirst", "ucwords", "urlencode", "urldecode", "base64_encode", "base64_decode", "strval", "to_string":
				retType = TypeString
			case "explode":
				retType = ArrayType(TypeString)
			case "__joss_file_get_bytes", "__joss_timezone_data":
				retType = ArrayType(TypeI64)
			case "floatval", "microtime":
				retType = TypeF64
			case "async":
				retType = TypeI64
			case "await":
				retType = TypeI64
			case "strpos":
				retType = TypeMixed
			case "array_push", "append":
				if len(args) > 0 {
					retType = args[0].Type()
				}
			case "abs", "intval", "strlen", "len", "count":
				retType = TypeI64
			case "boolval", "str_contains", "contains":
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
			case "GranDB_table", "GranDB_setTable", "GranDB_where", "GranDB_orWhere", "GranDB_whereIn", "GranDB_orderBy", "GranDB_orderByDesc", "orderByDesc", "GranDB_whereLike", "whereLike", "GranDB_orWhereLike", "orWhereLike", "GranDB_limit", "GranDB_offset", "GranDB_select", "GranDB_selectString", "GranDB_selectArray", "GranDB_join", "GranDB_leftJoin":
				retType = PtrType(StructType("GranDB", nil))
			case "GranDB_first", "GranDB_find", "GranDB_paginate", "paginate":
				retType = PtrType(StructType("map", nil))
			case "GranDB_get":
				retType = ArrayType(PtrType(StructType("map", nil)))
			}
		}

		if ext, ok := l.externDecls[calleeName]; ok {
			retType = ext.RetType
		}
		var dest *TempValue
		if retType.Kind != TypeKindVoid {
			dest = l.currentFn.NewTemp(retType, "call_ret")
		}

		if isBuiltin {
			unboxedArgs := make([]Value, len(args))
			for idx, a := range args {
				if calleeName == "floatval" || calleeName == "boolval" || calleeName == "abs" || calleeName == "min" || calleeName == "max" || calleeName == "chr" || (idx > 0 && (calleeName == "substr" || calleeName == "array_slice")) {
					unboxedArgs[idx] = l.unboxIfMixed(a)
				} else if idx == 0 && (calleeName == "is_string" || calleeName == "is_array" || calleeName == "is_map") {
					unboxedArgs[idx] = l.boxIfMixed(a, TypeMixed)
				} else {
					unboxedArgs[idx] = a
				}
			}
			if (calleeName == "array_push" || calleeName == "append") && len(unboxedArgs) >= 2 &&
				unboxedArgs[0].Type().Kind == TypeKindArray && unboxedArgs[0].Type().Elem != nil &&
				unboxedArgs[0].Type().Elem.Kind == TypeKindMixed {
				unboxedArgs[1] = l.boxIfMixed(unboxedArgs[1], TypeMixed)
			}
			l.currBlock.AddInstruction(&CallBuiltinInst{
				Dest:      dest,
				BuiltinID: calleeName,
				Args:      unboxedArgs,
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
			unboxedArgs := make([]Value, len(args))
			for idx, a := range args {
				unboxedArgs[idx] = l.unboxIfMixed(a)
			}
			l.currBlock.AddInstruction(&CallPrimitiveMethodInst{
				Dest:       dest,
				TypeKind:   typeKind,
				MethodName: methodName,
				Recv:       l.unboxIfMixed(receiverObj),
				Args:       unboxedArgs,
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
			methodArgs := args
			if receiverObj != nil && len(args) > 0 {
				methodArgs = args[1:]
			}
			if className == "ByteBuffer" {
				switch methodName {
				case "new", "Init":
					var capVal Value = NewConstInt(64, TypeI64)
					if len(methodArgs) > 0 {
						capVal = l.unboxIfMixed(methodArgs[0])
					}
					if dest == nil {
						dest = l.currentFn.NewTemp(PtrType(StructType("ByteBuffer", nil)), "buf")
					}
					l.currBlock.AddInstruction(&AllocBufferInst{Dest: dest, Cap: capVal})
					return dest, nil
				case "len":
					if dest == nil {
						dest = l.currentFn.NewTemp(TypeI64, "buflen")
					}
					l.currBlock.AddInstruction(&BufferLenInst{Dest: dest, Buf: receiverObj})
					return dest, nil
				case "get":
					if dest == nil {
						dest = l.currentFn.NewTemp(TypeI64, "bufget")
					}
					var idxVal Value = NewConstInt(0, TypeI64)
					if len(methodArgs) > 0 {
						idxVal = l.unboxIfMixed(methodArgs[0])
					}
					l.currBlock.AddInstruction(&BufferGetInst{Dest: dest, Buf: receiverObj, Index: idxVal})
					return dest, nil
				case "set":
					if len(methodArgs) >= 2 {
						l.currBlock.AddInstruction(&BufferSetInst{Buf: receiverObj, Index: l.unboxIfMixed(methodArgs[0]), Val: l.unboxIfMixed(methodArgs[1])})
					}
					return NewConstInt(0, TypeVoid), nil
				case "free":
					l.currBlock.AddInstruction(&FreeBufferInst{Buf: receiverObj})
					return NewConstInt(0, TypeVoid), nil
				case "appendByte":
					if len(methodArgs) > 0 {
						l.currBlock.AddInstruction(&CallRuntimeInst{Func: "joss_buf_append_byte", Args: []Value{receiverObj, l.unboxIfMixed(methodArgs[0])}})
					}
					return NewConstInt(0, TypeVoid), nil
				case "appendString":
					if len(methodArgs) > 0 {
						l.currBlock.AddInstruction(&CallRuntimeInst{Func: "joss_buf_append_cstr", Args: []Value{receiverObj, l.unboxIfMixed(methodArgs[0])}})
					}
					return NewConstInt(0, TypeVoid), nil
				case "toString":
					if dest == nil {
						dest = l.currentFn.NewTemp(TypeString, "bufstr")
					}
					l.currBlock.AddInstruction(&CallRuntimeInst{Dest: dest, Func: "joss_buf_to_cstr", Args: []Value{receiverObj}})
					return dest, nil
				}
			}
			if className == "StringBuilder" {
				switch methodName {
				case "new", "Init":
					if dest == nil {
						dest = l.currentFn.NewTemp(PtrType(StructType("StringBuilder", nil)), "sb")
					}
					l.currBlock.AddInstruction(&CallRuntimeInst{Dest: dest, Func: "joss_sb_new"})
					return dest, nil
				case "append":
					if len(methodArgs) > 0 {
						l.currBlock.AddInstruction(&CallRuntimeInst{Func: "joss_sb_append", Args: []Value{receiverObj, l.unboxIfMixed(methodArgs[0])}})
					}
					return NewConstInt(0, TypeVoid), nil
				case "appendInt":
					if len(methodArgs) > 0 {
						l.currBlock.AddInstruction(&CallRuntimeInst{Func: "joss_sb_append_i64", Args: []Value{receiverObj, l.unboxIfMixed(methodArgs[0])}})
					}
					return NewConstInt(0, TypeVoid), nil
				case "len":
					if dest == nil {
						dest = l.currentFn.NewTemp(TypeI64, "sblen")
					}
					l.currBlock.AddInstruction(&BufferLenInst{Dest: dest, Buf: receiverObj})
					return dest, nil
				case "toString":
					if dest == nil {
						dest = l.currentFn.NewTemp(TypeString, "sbstr")
					}
					l.currBlock.AddInstruction(&CallRuntimeInst{Dest: dest, Func: "joss_sb_to_string", Args: []Value{receiverObj}})
					return dest, nil
				case "free":
					l.currBlock.AddInstruction(&FreeBufferInst{Buf: receiverObj})
					return NewConstInt(0, TypeVoid), nil
				}
			}
			hostArgs := args
			hostRecv := receiverObj
			if receiverObj != nil && len(hostArgs) > 0 && hostArgs[0] == receiverObj {
				switch className {
				case "Request", "Session", "SEO", "Cache", "Config", "Log", "Cookie", "Hash", "JWT", "Auth", "Schema":
					hostArgs = hostArgs[1:]
					hostRecv = nil
				}
			}
			if hostFn := l.prog.Functions["__joss_host_"+className+"_"+methodName]; hostFn != nil && len(hostFn.Params) > 0 {
				paramOffset := 0
				if className == "Session" && len(hostFn.Params) > len(hostArgs) {
					paramOffset = 1
				}
				boxedArgs := make([]Value, len(hostArgs))
				for idx, a := range hostArgs {
					if idx+paramOffset < len(hostFn.Params) {
						boxedArgs[idx] = l.boxIfMixed(a, hostFn.Params[idx+paramOffset].Type())
					} else {
						boxedArgs[idx] = a
					}
				}
				hostArgs = boxedArgs
			}
			l.currBlock.AddInstruction(&CallHostServiceInst{
				Dest:       dest,
				ClassName:  className,
				MethodName: methodName,
				Recv:       hostRecv,
				Args:       hostArgs,
			})
		} else {
			if l.prog.Functions[calleeName] == nil && (calleeName == "strlen" || calleeName == "count" || calleeName == "len" || calleeName == "array_push" || calleeName == "append" || calleeName == "to_string" ||
				calleeName == "abs" || calleeName == "min" || calleeName == "max" || calleeName == "intval" || calleeName == "floatval" || calleeName == "boolval" || calleeName == "array_pop" ||
				calleeName == "microtime" || calleeName == "array_fill" || calleeName == "file_get_contents" || calleeName == "explode" || calleeName == "async" || calleeName == "await" ||
				calleeName == "str_contains" || calleeName == "contains" || calleeName == "map_delete") {
				unboxedArgs := make([]Value, len(args))
				for idx, a := range args {
					if idx > 0 && (calleeName == "array_push" || calleeName == "append") {
						unboxedArgs[idx] = a
					} else {
						unboxedArgs[idx] = l.unboxIfMixed(a)
					}
				}
				l.currBlock.AddInstruction(&CallBuiltinInst{
					Dest:      dest,
					BuiltinID: calleeName,
					Args:      unboxedArgs,
				})
			} else if ext, ok := l.externDecls[calleeName]; ok {
				externArgs := make([]Value, len(args))
				for idx, a := range args {
					externArgs[idx] = l.unboxIfMixed(a)
				}
				l.currBlock.AddInstruction(&ExternCallInst{
					Dest:     dest,
					Symbol:   ext.Symbol,
					Args:     externArgs,
					RetType:  ext.RetType,
					ArgTypes: ext.ArgTypes,
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
					if receiverObj != nil && (resolvedMethodStmt == nil || !resolvedMethodStmt.IsStatic) {
						if member, ok := e.Function.(*parser.MemberExpression); ok && member.Property != nil &&
							receiverObj.Type().Kind == TypeKindPtr && receiverObj.Type().Elem != nil &&
							receiverObj.Type().Elem.Name != "" && receiverObj.Type().Elem.Name != "map" {
							staticCls := receiverObj.Type().Elem.Name
							mName := member.Property.Value
							if overrides := l.collectVirtualOverrides(staticCls, mName, calleeName); len(overrides) > 0 {
								paramTypes := make([]Type, 0, len(args))
								if resolvedMethodStmt != nil {
									for _, p := range resolvedMethodStmt.Parameters {
										if p != nil {
											paramTypes = append(paramTypes, l.mapType(p.Type.Literal))
										}
									}
								}
								for idx := 1 + len(paramTypes); idx < len(args); idx++ {
									paramTypes = append(paramTypes, args[idx].Type())
								}
								calleeName = l.ensureVirtualDispatchHelper(staticCls, mName, retType, paramTypes, overrides)
							}
						}
					}
					callArgs := args
					if targetFn := l.prog.Functions[calleeName]; targetFn != nil && len(targetFn.Params) > 0 {
						callArgs = make([]Value, len(args))
						for idx, a := range args {
							if idx < len(targetFn.Params) {
								callArgs[idx] = l.boxIfMixed(a, targetFn.Params[idx].Type())
							} else {
								callArgs[idx] = a
							}
						}
					} else {
						var expectedParams []*parser.Parameter
						paramOffset := 0
						if resolvedMethodStmt != nil {
							expectedParams = resolvedMethodStmt.Parameters
							if receiverObj != nil && !resolvedMethodStmt.IsStatic {
								paramOffset = 1
							}
						} else if receiverObj == nil {
							if fnStmt, ok := l.resolveFunction(calleeName); ok && fnStmt != nil {
								expectedParams = fnStmt.Parameters
							}
						}
						if len(expectedParams) > 0 {
							callArgs = make([]Value, len(args))
							for idx, a := range args {
								pIdx := idx - paramOffset
								if pIdx >= 0 && pIdx < len(expectedParams) && expectedParams[pIdx] != nil && expectedParams[pIdx].Type.Literal != "" {
									callArgs[idx] = l.boxIfMixed(a, l.mapType(expectedParams[pIdx].Type.Literal))
								} else {
									callArgs[idx] = a
								}
							}
						} else if (calleeName == "with" || calleeName == "WebResponse_with") && len(args) >= 3 {
							callArgs = append([]Value(nil), args...)
							callArgs[2] = l.boxIfMixed(callArgs[2], TypeMixed)
						}
					}
					l.currBlock.AddInstruction(&CallInst{
						Dest:   dest,
						Callee: calleeName,
						Args:   callArgs,
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
				storeVal := val
				if ptr.Type().Kind == TypeKindPtr && ptr.Type().Elem != nil {
					storeVal = l.boxIfMixed(val, *ptr.Type().Elem)
					storeVal = l.unboxIfTyped(storeVal, *ptr.Type().Elem)
				}
				l.currBlock.AddInstruction(&StoreInst{Val: storeVal, Ptr: ptr})
				if l.activeClosureEnvMap != nil && l.activeClosureCaptured[varName] {
					l.currBlock.AddInstruction(&MapSetInst{
						Map: l.activeClosureEnvMap,
						Key: NewConstString(varName),
						Val: l.boxIfMixed(storeVal, TypeMixed),
					})
				}
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
			if member.Token.Type == parser.DOUBLE_COLON {
				if ident, isIdent := member.Left.(*parser.Identifier); isIdent {
					if l.ensureStaticPropertyGlobal(ident.Value, member.Property.Value) != nil {
						globalName := staticPropertyGlobalName(ident.Value, member.Property.Value)
						l.currBlock.AddInstruction(&GlobalStoreInst{GlobalName: globalName, Val: val})
						return val, nil
					}
				}
			}
			objVal, err := l.lowerExpression(member.Left)
			if err != nil {
				return nil, err
			}
			if l.isModelType(objVal.Type()) {
				l.currBlock.AddInstruction(&CallInst{
					Dest:   nil,
					Callee: "fn_Model_setAttr",
					Args:   []Value{objVal, NewConstString(member.Property.Value), l.boxIfMixed(val, TypeMixed)},
				})
				return val, nil
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
				pushVal := val
				if colVal.Type().Kind == TypeKindArray && colVal.Type().Elem != nil && colVal.Type().Elem.Kind == TypeKindMixed {
					pushVal = l.boxIfMixed(val, TypeMixed)
				}
				l.currBlock.AddInstruction(&CallBuiltinInst{
					Dest:      nil,
					BuiltinID: "array_push",
					Args:      []Value{colVal, pushVal},
				})
				return val, nil
			}
			idxVal, err := l.lowerExpression(idxExpr.Index)
			if err != nil {
				return nil, err
			}
			isMapCol := colVal.Type().Kind == TypeKindPtr && colVal.Type().Elem != nil && colVal.Type().Elem.Name == "map"
			if idxVal.Type().Kind == TypeKindString || isMapCol {
				mapVal := val
				if isMapCol && len(colVal.Type().Elem.Fields) > 0 && colVal.Type().Elem.Fields[0].Kind == TypeKindMixed {
					mapVal = l.boxIfMixed(val, TypeMixed)
				}
				l.currBlock.AddInstruction(&MapSetInst{
					Val: mapVal,
					Map: colVal,
					Key: idxVal,
				})
			} else {
				arrVal := val
				if colVal.Type().Kind == TypeKindArray && colVal.Type().Elem != nil && colVal.Type().Elem.Kind == TypeKindMixed {
					arrVal = l.boxIfMixed(val, TypeMixed)
				}
				l.currBlock.AddInstruction(&ArraySetInst{
					Val:   arrVal,
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
		if l.prep != nil && l.prep.Facts != nil {
			if inferred, ok := l.prep.Facts.InferredTypes[e]; ok && inferred.Kind == typesystem.Callable && inferred.Element != nil && inferred.Element.Kind != typesystem.Unknown {
				retType = l.mapTypeSystemType(*inferred.Element)
			}
		}
	}

	savedFn := l.currentFn
	savedBlock := l.currBlock
	savedLocals := l.varLocals
	savedLoops := l.loopStack
	savedFnStmts := l.currentFnStmts
	savedClosureCaptured := l.activeClosureCaptured
	savedClosureEnvMap := l.activeClosureEnvMap

	// Collect outer local variable names referenced inside the closure body.
	paramSet := make(map[string]bool, len(e.Parameters))
	for _, p := range e.Parameters {
		if p != nil && p.Name != nil {
			paramSet[cleanVarName(p.Name.Value)] = true
		}
	}
	capturedSet := make(map[string]bool)
	if e.Body != nil {
		walkASTNode(e.Body, false, func(n parser.Node) {
			if v, ok := n.(*parser.Identifier); ok {
				name := cleanVarName(v.Value)
				if !paramSet[name] && savedLocals[name] != nil {
					capturedSet[name] = true
				}
			}
		})
	}
	capturedKeys := make([]string, 0, len(capturedSet))
	for k := range capturedSet {
		capturedKeys = append(capturedKeys, k)
	}
	sort.Strings(capturedKeys)

	fn := NewFunction(closureName, retType)
	l.currentFn = fn
	l.currBlock = fn.EntryBlock
	l.varLocals = make(map[string]Value)
	l.loopStack = nil
	if e.Body != nil {
		l.currentFnStmts = e.Body.Statements
	} else {
		l.currentFnStmts = nil
	}
	l.activeClosureCaptured = nil
	l.activeClosureEnvMap = nil

	if len(capturedKeys) > 0 {
		envMap := fn.NewTemp(PtrType(StructType("map", nil)), "closure_env")
		l.currBlock.AddInstruction(&CallRuntimeInst{
			Dest: envMap,
			Func: "joss_closure_env_get",
			Args: []Value{NewConstFnPtr(closureName, PtrType(TypeVoid))},
		})
		l.activeClosureCaptured = capturedSet
		l.activeClosureEnvMap = envMap
		for _, k := range capturedKeys {
			outerPtr := savedLocals[k]
			elemT := outerPtr.Type().ElemType()
			ptrTemp := fn.NewTemp(PtrType(elemT), k+"_cap_ptr")
			l.currBlock.AddInstruction(&AllocaInst{Dest: ptrTemp, AllocType: elemT})
			var capVal Value
			if isScalarBoxedKind(elemT.Kind) {
				capRaw := fn.NewTemp(TypeMixed, k+"_cap_raw")
				l.currBlock.AddInstruction(&MapGetInst{
					Dest: capRaw,
					Map:  envMap,
					Key:  NewConstString(k),
				})
				capVal = l.unboxIfTyped(capRaw, elemT)
			} else {
				capTmp := fn.NewTemp(elemT, k+"_cap_val")
				l.currBlock.AddInstruction(&MapGetInst{
					Dest: capTmp,
					Map:  envMap,
					Key:  NewConstString(k),
				})
				capVal = capTmp
			}
			l.currBlock.AddInstruction(&StoreInst{Val: capVal, Ptr: ptrTemp})
			l.varLocals[k] = ptrTemp
		}
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
				l.currentFnStmts = savedFnStmts
				l.activeClosureCaptured = savedClosureCaptured
				l.activeClosureEnvMap = savedClosureEnvMap
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
	l.currentFnStmts = savedFnStmts
	l.activeClosureCaptured = savedClosureCaptured
	l.activeClosureEnvMap = savedClosureEnvMap

	fnPtr := NewConstFnPtr(closureName, PtrType(TypeVoid))
	if len(capturedKeys) > 0 {
		pairs := make([][2]Value, 0, len(capturedKeys))
		for _, k := range capturedKeys {
			outerPtr := savedLocals[k]
			elemT := outerPtr.Type().ElemType()
			loaded := l.currentFn.NewTemp(elemT, k+"_cap_out")
			l.currBlock.AddInstruction(&LoadInst{Dest: loaded, Ptr: outerPtr})
			pairs = append(pairs, [2]Value{NewConstString(k), l.boxIfMixed(loaded, TypeMixed)})
		}
		envMap := l.currentFn.NewTemp(PtrType(StructType("map", nil)), "closure_env_out")
		l.currBlock.AddInstruction(&NewMapInst{Dest: envMap, Pairs: pairs})
		closureBox := l.currentFn.NewTemp(PtrType(TypeVoid), "closure_box")
		l.currBlock.AddInstruction(&CallRuntimeInst{
			Dest: closureBox,
			Func: "joss_closure_alloc",
			Args: []Value{fnPtr, envMap},
		})
		return closureBox, nil
	}
	return fnPtr, nil
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

func walkASTNode(node parser.Node, skipClosures bool, visit func(parser.Node)) {
	if node == nil {
		return
	}
	visit(node)
	switch v := node.(type) {
	case *parser.BlockStatement:
		for _, s := range v.Statements {
			walkASTNode(s, skipClosures, visit)
		}
	case *parser.ExpressionStatement:
		walkASTNode(v.Expression, skipClosures, visit)
	case *parser.ReturnStatement:
		walkASTNode(v.ReturnValue, skipClosures, visit)
	case *parser.LetStatement:
		walkASTNode(v.Value, skipClosures, visit)
	case *parser.MultiLetStatement:
		for _, d := range v.Declarations {
			walkASTNode(d.Value, skipClosures, visit)
		}
	case *parser.EchoStatement:
		walkASTNode(v.Value, skipClosures, visit)
	case *parser.ThrowStatement:
		walkASTNode(v.Value, skipClosures, visit)
	case *parser.GuardStatement:
		walkASTNode(v.Condition, skipClosures, visit)
		walkASTNode(v.Body, skipClosures, visit)
	case *parser.AssignExpression:
		walkASTNode(v.Left, skipClosures, visit)
		walkASTNode(v.Value, skipClosures, visit)
	case *parser.InfixExpression:
		walkASTNode(v.Left, skipClosures, visit)
		walkASTNode(v.Right, skipClosures, visit)
	case *parser.PrefixExpression:
		walkASTNode(v.Right, skipClosures, visit)
	case *parser.PostfixExpression:
		walkASTNode(v.Left, skipClosures, visit)
	case *parser.BlockExpression:
		walkASTNode(v.Block, skipClosures, visit)
	case *parser.TernaryExpression:
		walkASTNode(v.Condition, skipClosures, visit)
		walkASTNode(v.True, skipClosures, visit)
		walkASTNode(v.False, skipClosures, visit)
	case *parser.CallExpression:
		walkASTNode(v.Function, skipClosures, visit)
		for _, a := range v.Arguments {
			walkASTNode(a, skipClosures, visit)
		}
	case *parser.NewExpression:
		for _, a := range v.Arguments {
			walkASTNode(a, skipClosures, visit)
		}
	case *parser.IssetExpression:
		for _, a := range v.Arguments {
			walkASTNode(a, skipClosures, visit)
		}
	case *parser.EmptyExpression:
		walkASTNode(v.Argument, skipClosures, visit)
	case *parser.IsExpression:
		walkASTNode(v.Left, skipClosures, visit)
	case *parser.MatchExpression:
		walkASTNode(v.Subject, skipClosures, visit)
		for _, arm := range v.Arms {
			for _, k := range arm.Keys {
				walkASTNode(k, skipClosures, visit)
			}
			walkASTNode(arm.Value, skipClosures, visit)
		}
	case *parser.MemberExpression:
		walkASTNode(v.Left, skipClosures, visit)
	case *parser.IndexExpression:
		walkASTNode(v.Left, skipClosures, visit)
		walkASTNode(v.Index, skipClosures, visit)
	case *parser.SpreadExpression:
		walkASTNode(v.Expression, skipClosures, visit)
	case *parser.NamedArgument:
		walkASTNode(v.Value, skipClosures, visit)
	case *parser.ReferenceExpression:
		walkASTNode(v.Target, skipClosures, visit)
	case *parser.ArrayLiteral:
		for _, el := range v.Elements {
			walkASTNode(el, skipClosures, visit)
		}
	case *parser.MapLiteral:
		for k, val := range v.Pairs {
			walkASTNode(k, skipClosures, visit)
			walkASTNode(val, skipClosures, visit)
		}
	case *parser.WhileStatement:
		walkASTNode(v.Condition, skipClosures, visit)
		walkASTNode(v.Body, skipClosures, visit)
	case *parser.DoWhileStatement:
		walkASTNode(v.Body, skipClosures, visit)
		walkASTNode(v.Condition, skipClosures, visit)
	case *parser.ForeachStatement:
		walkASTNode(v.Iterable, skipClosures, visit)
		walkASTNode(v.Body, skipClosures, visit)
	case *parser.TryCatchStatement:
		walkASTNode(v.TryBlock, skipClosures, visit)
		walkASTNode(v.CatchBlock, skipClosures, visit)
	case *parser.FunctionLiteral:
		if !skipClosures && v.Body != nil {
			walkASTNode(v.Body, skipClosures, visit)
		}
	}
}

func (l *Lowerer) inferMethodReturnType(stmt *parser.MethodStatement) Type {
	return l.inferMethodReturnTypeVisited(stmt, make(map[*parser.MethodStatement]bool))
}

func (l *Lowerer) inferMethodReturnTypeVisited(stmt *parser.MethodStatement, visited map[*parser.MethodStatement]bool) Type {
	if stmt == nil {
		return TypeI64
	}
	if lit := strings.TrimSpace(stmt.ReturnType.Literal); lit != "" {
		return l.mapType(lit)
	}
	if stmt.Body == nil || visited[stmt] {
		return TypeI64
	}
	visited[stmt] = true

	var inferred []Type
	hasConcrete := false
	walkASTNode(stmt.Body, true, func(n parser.Node) {
		ret, ok := n.(*parser.ReturnStatement)
		if !ok || ret.ReturnValue == nil {
			return
		}
		if t, known := l.inferExprConcreteType(ret.ReturnValue, visited); known {
			inferred = append(inferred, t)
			hasConcrete = true
		}
	})
	if !hasConcrete || len(inferred) == 0 {
		return TypeI64
	}
	first := inferred[0]
	for _, t := range inferred[1:] {
		if t.Kind == TypeKindMixed || first.Kind == TypeKindMixed || t.Kind != first.Kind {
			return TypeMixed
		}
	}
	if first.Kind == TypeKindUnknown {
		return TypeMixed
	}
	return first
}

func (l *Lowerer) inferExprConcreteType(expr parser.Expression, visited map[*parser.MethodStatement]bool) (Type, bool) {
	if expr == nil {
		return TypeUnknown, false
	}
	if l.prep != nil && l.prep.Facts != nil {
		if t, ok := l.prep.Facts.InferredTypes[expr]; ok && t.Kind != typesystem.Unknown {
			return l.mapTypeSystemType(t), true
		}
		if call, ok := expr.(*parser.CallExpression); ok {
			if resolved, ok := l.prep.Facts.ResolvedCalls[call]; ok && resolved.ReturnType.Kind != typesystem.Unknown {
				return l.mapTypeSystemType(resolved.ReturnType), true
			}
		}
	}
	switch e := expr.(type) {
	case *parser.IntegerLiteral:
		return TypeI64, true
	case *parser.FloatLiteral:
		return TypeF64, true
	case *parser.DecimalLiteral:
		return TypeDecimal, true
	case *parser.StringLiteral:
		return TypeString, true
	case *parser.Boolean:
		return TypeBool, true
	case *parser.MapLiteral:
		return PtrType(StructType("map", nil)), true
	case *parser.ArrayLiteral:
		return ArrayType(TypeMixed), true
	case *parser.NewExpression:
		if e.Class != nil {
			return l.mapType(e.Class.Value), true
		}
	case *parser.IndexExpression:
		return TypeMixed, true
	case *parser.InfixExpression:
		switch e.Operator {
		case ".":
			return TypeString, true
		case "==", "!=", "<", "<=", ">", ">=", "&&", "||":
			return TypeBool, true
		case "+", "-", "*", "/", "%":
			lt, lOk := l.inferExprConcreteType(e.Left, visited)
			rt, rOk := l.inferExprConcreteType(e.Right, visited)
			if (lOk && lt.Kind == TypeKindF64) || (rOk && rt.Kind == TypeKindF64) {
				return TypeF64, true
			}
			if (lOk && lt.Kind == TypeKindI64) || (rOk && rt.Kind == TypeKindI64) {
				return TypeI64, true
			}
		}
	case *parser.TernaryExpression:
		lt, lOk := l.inferExprConcreteType(e.True, visited)
		rt, rOk := l.inferExprConcreteType(e.False, visited)
		if lOk && rOk {
			if lt.Kind == rt.Kind && lt.Kind != TypeKindMixed {
				return lt, true
			}
			return TypeMixed, true
		}
		if lOk {
			return lt, true
		}
		if rOk {
			return rt, true
		}
	case *parser.CallExpression:
		name := ""
		className := ""
		switch fn := e.Function.(type) {
		case *parser.Identifier:
			name = fn.Value
		case *parser.MemberExpression:
			if fn.Property != nil {
				name = fn.Property.Value
			}
			if id, ok := fn.Left.(*parser.Identifier); ok {
				className = id.Value
			}
		}
		switch name {
		case "view", "json", "response", "redirect", "back", "with", "withCookie",
			"render", "html_escape", "csrf_field", "readFile", "toHtml", "trim",
			"file_get_contents", "__joss_byte_string", "toSql", "strval", "to_string",
			"strtolower", "strtoupper", "substr", "implode", "sprintf", "format",
			"base64_encode", "hash", "env", "str_replace":
			return TypeString, true
		case "intval", "strlen", "len", "count", "time":
			return TypeI64, true
		case "floatval", "microtime":
			return TypeF64, true
		case "boolval", "str_contains", "contains", "str_starts_with", "str_ends_with",
			"is_string", "is_array", "is_map", "empty", "isset":
			return TypeBool, true
		}
		if visited != nil {
			if className == "" {
				if fnStmt, ok := l.resolveFunction(name); ok && fnStmt != nil && !visited[fnStmt] {
					return l.inferMethodReturnTypeVisited(fnStmt, visited), true
				}
			} else if _, mRet, mStmt, ok := l.resolveClassMethod(className, name); ok && mStmt != nil && !visited[mStmt] {
				return mRet, true
			}
		}
	}
	return TypeUnknown, false
}

func (l *Lowerer) isVarReassignedIncompatible(varName string, initType Type, stmts []parser.Statement) bool {
	if len(stmts) == 0 || varName == "" {
		return false
	}
	incompatible := false
	for _, st := range stmts {
		walkASTNode(st, true, func(n parser.Node) {
			if incompatible {
				return
			}
			assign, ok := n.(*parser.AssignExpression)
			if !ok {
				return
			}
			ident, ok := assign.Left.(*parser.Identifier)
			if !ok || cleanVarName(ident.Value) != varName {
				return
			}
			rhsType, known := l.inferExprConcreteType(assign.Value, nil)
			if !known || rhsType.Kind == TypeKindMixed || rhsType.Kind == TypeKindUnknown || rhsType.Kind != initType.Kind {
				incompatible = true
			}
		})
		if incompatible {
			return true
		}
	}
	return false
}

func pluralizeIRClassName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	switch lower {
	case "person":
		return "people"
	case "man":
		return "men"
	case "child":
		return "children"
	case "foot":
		return "feet"
	case "tooth":
		return "teeth"
	case "mouse":
		return "mice"
	}
	if strings.HasSuffix(lower, "y") && len(lower) > 1 {
		secondLast := lower[len(lower)-2]
		if secondLast != 'a' && secondLast != 'e' && secondLast != 'i' && secondLast != 'o' && secondLast != 'u' {
			return s[:len(s)-1] + "ies"
		}
	}
	if strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") || strings.HasSuffix(lower, "z") || strings.HasSuffix(lower, "ch") || strings.HasSuffix(lower, "sh") {
		return s + "es"
	}
	return s + "s"
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
		elem := TypeMixed
		if t.Element != nil {
			mapped := l.mapTypeSystemType(*t.Element)
			if mapped.Kind != TypeKindUnknown {
				elem = mapped
			}
		}
		return ArrayType(elem)
	case typesystem.Map:
		return PtrType(StructType("map", nil))
	case typesystem.Decimal:
		return TypeDecimal
	case typesystem.Mixed:
		return TypeMixed
	case typesystem.Union:
		if t.Name != "" {
			mapped := l.mapType(t.Name)
			if mapped.Kind != TypeKindI64 || strings.Contains(t.Name, "int") {
				return mapped
			}
		}
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

func (l *Lowerer) buildAllClassLayouts() {
	if l.prep == nil {
		return
	}
	classMap := make(map[string]*parser.ClassStatement)
	interfaceNames := make(map[string]bool)
	for _, unit := range l.prep.Units {
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			switch s := stmt.(type) {
			case *parser.ClassStatement:
				if s.Name != nil {
					classMap[s.Name.Value] = s
				}
			case *parser.InterfaceStatement:
				if s.Name != nil {
					interfaceNames[s.Name.Value] = true
				}
			}
		}
	}

	var getOrBuildLayout func(name string) *ClassLayout
	getOrBuildLayout = func(name string) *ClassLayout {
		if layout, ok := l.prog.ClassLayouts[name]; ok {
			return layout
		}
		cs := classMap[name]
		if cs == nil {
			return nil
		}

		layout := &ClassLayout{
			Name:         name,
			FieldOffsets: map[string]int32{"__joss_cls": 0},
		}

		var currentOffset int32 = 8 // [0..7] reservado para puntero de clase/RTTI
		if cs.SuperClass != nil && cs.SuperClass.Value != "" {
			parentLayout := getOrBuildLayout(cs.SuperClass.Value)
			if parentLayout != nil {
				for fName, off := range parentLayout.FieldOffsets {
					layout.FieldOffsets[fName] = off
					if off+8 > currentOffset {
						currentOffset = off + 8
					}
				}
			}
		}

		if cs.Body != nil {
			for _, member := range cs.Body.Statements {
				switch m := member.(type) {
				case *parser.LetStatement:
					if m.Name != nil {
						fName := cleanVarName(m.Name.Value)
						if _, exists := layout.FieldOffsets[fName]; !exists {
							layout.FieldOffsets[fName] = currentOffset
							currentOffset += 8
						}
					}
				case *parser.MultiLetStatement:
					for _, d := range m.Declarations {
						if d.Name != nil {
							fName := cleanVarName(d.Name.Value)
							if _, exists := layout.FieldOffsets[fName]; !exists {
								layout.FieldOffsets[fName] = currentOffset
								currentOffset += 8
							}
						}
					}
				}
			}
		}

		allocSize := uint32((currentOffset + 15) & -16)
		if allocSize < 16 {
			allocSize = 16
		}
		layout.AllocSize = allocSize
		l.prog.ClassLayouts[name] = layout
		return layout
	}

	for className := range classMap {
		getOrBuildLayout(className)
	}
	for ifaceName := range interfaceNames {
		if _, exists := l.prog.ClassLayouts[ifaceName]; !exists {
			l.prog.ClassLayouts[ifaceName] = &ClassLayout{
				Name:         ifaceName,
				FieldOffsets: map[string]int32{"__joss_cls": 0},
				AllocSize:    16,
			}
		}
	}
}

func (l *Lowerer) resolveClassMethod(className, methodName string) (string, Type, *parser.MethodStatement, bool) {
	if l.prep == nil {
		return "", TypeI64, nil, false
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
							return curr + "_" + methodName, l.inferMethodReturnType(m), m, true
						}
					}
				}
			}
		}
		curr = nextSuper
	}
	return "", TypeI64, nil, false
}

func (l *Lowerer) resolveFunction(fnName string) (*parser.MethodStatement, bool) {
	if l.prep == nil {
		return nil, false
	}
	for _, unit := range l.prep.Units {
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			if fn, ok := stmt.(*parser.MethodStatement); ok && fn.Name != nil && fn.Name.Value == fnName {
				return fn, true
			}
		}
	}
	return nil, false
}

func (l *Lowerer) mapType(lit string) Type {
	lit = strings.TrimSpace(lit)
	if strings.HasPrefix(lit, "map<") {
		return PtrType(StructType("map", nil))
	}
	if strings.HasPrefix(lit, "array<") && strings.HasSuffix(lit, ">") {
		element := strings.TrimSpace(lit[len("array<") : len(lit)-1])
		if element == "" {
			return ArrayType(TypeI64)
		}
		return ArrayType(l.mapType(element))
	}
	if strings.HasSuffix(lit, "?") {
		return l.mapType(strings.TrimSuffix(lit, "?"))
	}
	if strings.Contains(lit, "|") {
		for _, part := range strings.Split(lit, "|") {
			part = strings.TrimSpace(part)
			if part != "" && part != "null" && part != "nil" {
				return l.mapType(part)
			}
		}
	}
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
		return ArrayType(TypeMixed)
	case "mixed":
		return TypeMixed
	case "var", "unknown":
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

func (l *Lowerer) lowerConstantDefault(expression parser.Expression, parameterType Type) Value {
	switch value := expression.(type) {
	case *parser.IntegerLiteral:
		return NewConstInt(value.Value, parameterType)
	case *parser.FloatLiteral:
		return NewConstFloat(value.Value, parameterType)
	case *parser.StringLiteral:
		return NewConstString(value.Value)
	case *parser.Boolean:
		return NewConstBool(value.Value)
	case *parser.NullLiteral:
		return NewConstNull(PtrType(TypeVoid))
	case *parser.PrefixExpression:
		if value.Operator != "-" {
			return nil
		}
		switch operand := value.Right.(type) {
		case *parser.IntegerLiteral:
			return NewConstInt(-operand.Value, parameterType)
		case *parser.FloatLiteral:
			return NewConstFloat(-operand.Value, parameterType)
		}
	}
	return nil
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

func isZeroInitConstant(v Value) bool {
	switch c := v.(type) {
	case *ConstInt:
		return c.Val == 0
	case *ConstFloat:
		return c.Val == 0.0
	case *ConstBool:
		return !c.Val
	case *ConstNull:
		return true
	}
	return false
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

func (l *Lowerer) isKnownInt(val Value) bool {
	return l.isKnownIntVisited(val, make(map[int]bool))
}

func (l *Lowerer) isKnownIntVisited(val Value, visited map[int]bool) bool {
	if c, ok := val.(*ConstInt); ok {
		k := c.Type().Kind
		return k == TypeKindI64 || k == TypeKindI32 || k == TypeKindU8
	}
	if p, ok := val.(*ParamValue); ok {
		k := p.Type().Kind
		return k == TypeKindI64 || k == TypeKindI32 || k == TypeKindU8
	}
	t, ok := val.(*TempValue)
	if !ok || l.currentFn == nil {
		return false
	}
	if visited[t.ID] {
		return true
	}
	visited[t.ID] = true

	walkFnBlocks := func(fn func(inst Instruction) bool) {
		var walk func(blocks []*BasicBlock) bool
		walk = func(blocks []*BasicBlock) bool {
			for _, blk := range blocks {
				for _, inst := range blk.Instructions {
					if tc, ok := inst.(*TryCatchInst); ok {
						if !walk(tc.TryBlocks) || !walk(tc.CatchBlocks) {
							return false
						}
					}
					if !fn(inst) {
						return false
					}
				}
			}
			return true
		}
		walk(l.currentFn.Blocks)
	}

	var def Instruction
	walkFnBlocks(func(inst Instruction) bool {
		res := inst.Result()
		if res != nil && res.ID == t.ID {
			def = inst
			return false
		}
		return true
	})
	if def == nil {
		return false
	}

	switch i := def.(type) {
	case *BinaryInst:
		switch i.Op {
		case OpAdd, OpSub, OpMul, OpDiv, OpMod, OpAnd, OpOr, OpXor, OpShl, OpShr:
			k := i.Dest.Type().Kind
			return k == TypeKindI64 || k == TypeKindI32 || k == TypeKindU8
		default:
			return false
		}
	case *UnaryInst, *CompareInst:
		return true
	case *CallBuiltinInst:
		switch i.BuiltinID {
		case "strlen", "len", "count", "intval", "abs", "min", "max":
			return true
		}
		return false
	case *LoadInst:
		ptrT, ok := i.Ptr.(*TempValue)
		if !ok {
			return false
		}
		var alloc *AllocaInst
		walkFnBlocks(func(inst Instruction) bool {
			if a, isAlloca := inst.(*AllocaInst); isAlloca && a.Dest != nil && a.Dest.ID == ptrT.ID {
				alloc = a
				return false
			}
			return true
		})
		if alloc == nil {
			return false
		}
		ak := alloc.AllocType.Kind
		if ak != TypeKindI64 && ak != TypeKindI32 && ak != TypeKindU8 {
			return false
		}

		storeCount := 0
		okAll := true
		walkFnBlocks(func(inst Instruction) bool {
			switch ins := inst.(type) {
			case *StoreInst:
				if valT, okV := ins.Val.(*TempValue); okV && valT.ID == ptrT.ID {
					okAll = false
					return false
				}
				if stPtr, okP := ins.Ptr.(*TempValue); okP && stPtr.ID == ptrT.ID {
					storeCount++
					if !l.isKnownIntVisited(ins.Val, visited) {
						okAll = false
						return false
					}
				}
			case *LoadInst, *TryCatchInst:
				// Allowed read or container
			default:
				for _, op := range inst.Operands() {
					if opT, okOp := op.(*TempValue); okOp && opT.ID == ptrT.ID {
						okAll = false
						return false
					}
				}
			}
			return true
		})
		return okAll && storeCount > 0
	default:
		return false
	}
}

func isScalarBoxedKind(k TypeKind) bool {
	return k == TypeKindI64 || k == TypeKindI32 || k == TypeKindU8 ||
		k == TypeKindBool || k == TypeKindF64 || k == TypeKindF32 || k == TypeKindDecimal
}

func (l *Lowerer) boxIfMixed(val Value, targetType Type) Value {
	if val == nil || targetType.Kind != TypeKindMixed {
		return val
	}
	if !isScalarBoxedKind(val.Type().Kind) {
		return val
	}
	boxed := l.currentFn.NewTemp(TypeMixed, "boxed")
	l.currBlock.AddInstruction(&BoxValueInst{Dest: boxed, Val: val})
	return boxed
}

func (l *Lowerer) unboxIfMixed(val Value) Value {
	if val == nil || val.Type().Kind != TypeKindMixed {
		return val
	}
	unboxed := l.currentFn.NewTemp(TypeI64, "unboxed")
	l.currBlock.AddInstruction(&UnboxValueInst{Dest: unboxed, Val: val})
	return unboxed
}

func (l *Lowerer) unboxIfTyped(val Value, targetType Type) Value {
	if val == nil || val.Type().Kind != TypeKindMixed {
		return val
	}
	if isScalarBoxedKind(targetType.Kind) {
		return l.unboxIfMixed(val)
	}
	return val
}

func (l *Lowerer) newDetachedBlock(prefix string) *BasicBlock {
	id := l.currentFn.NextBlockID
	l.currentFn.NextBlockID++
	return NewBasicBlock(id, fmt.Sprintf("%s%d", prefix, id))
}

func cleanTryCatchBlocks(blocks []*BasicBlock) []*BasicBlock {
	if len(blocks) == 0 {
		return blocks
	}
	inRegion := make(map[*BasicBlock]bool, len(blocks))
	for _, b := range blocks {
		inRegion[b] = true
	}
	reachable := make(map[*BasicBlock]bool, len(blocks))
	queue := []*BasicBlock{blocks[0]}
	reachable[blocks[0]] = true
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, succ := range curr.Successors() {
			if succ != nil && inRegion[succ] && !reachable[succ] {
				reachable[succ] = true
				queue = append(queue, succ)
			}
		}
	}
	cleaned := make([]*BasicBlock, 0, len(blocks))
	for _, b := range blocks {
		if reachable[b] {
			cleaned = append(cleaned, b)
		}
	}
	if len(cleaned) == 0 {
		return blocks[:1]
	}
	return cleaned
}

func (l *Lowerer) collectClassHierarchy(className string) []*parser.ClassStatement {
	if l.prep == nil || className == "" {
		return nil
	}
	classMap := make(map[string]*parser.ClassStatement)
	for _, unit := range l.prep.Units {
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			if cs, ok := stmt.(*parser.ClassStatement); ok && cs.Name != nil {
				classMap[cs.Name.Value] = cs
			}
		}
	}
	var revChain []*parser.ClassStatement
	visited := make(map[string]bool)
	curr := className
	for curr != "" && !visited[curr] {
		visited[curr] = true
		cs := classMap[curr]
		if cs == nil {
			break
		}
		revChain = append(revChain, cs)
		if cs.SuperClass != nil {
			curr = cs.SuperClass.Value
		} else {
			curr = ""
		}
	}
	chain := make([]*parser.ClassStatement, len(revChain))
	for i, cs := range revChain {
		chain[len(revChain)-1-i] = cs
	}
	return chain
}

func (l *Lowerer) classHasSubclasses(className string) bool {
	if l.prep == nil || className == "" {
		return false
	}
	for _, unit := range l.prep.Units {
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			cs, ok := stmt.(*parser.ClassStatement)
			if !ok {
				continue
			}
			if cs.SuperClass != nil && cs.SuperClass.Value == className {
				return true
			}
			for _, iface := range cs.Interfaces {
				if iface != nil && iface.Value == className {
					return true
				}
			}
		}
	}
	return false
}

type virtualOverride struct {
	subClass  string
	implFn    string
	implOwner string
	implRet   Type
	implStmt  *parser.MethodStatement
}

func (l *Lowerer) collectVirtualOverrides(staticCls, methodName, staticImplFn string) []virtualOverride {
	if l.prep == nil || staticCls == "" || methodName == "" {
		return nil
	}
	childrenMap := make(map[string][]*parser.ClassStatement)
	for _, unit := range l.prep.Units {
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			cs, ok := stmt.(*parser.ClassStatement)
			if !ok || cs.Name == nil {
				continue
			}
			if cs.SuperClass != nil && cs.SuperClass.Value != "" {
				childrenMap[cs.SuperClass.Value] = append(childrenMap[cs.SuperClass.Value], cs)
			}
			for _, iface := range cs.Interfaces {
				if iface != nil && iface.Value != "" {
					childrenMap[iface.Value] = append(childrenMap[iface.Value], cs)
				}
			}
		}
	}

	var overrides []virtualOverride
	visited := map[string]bool{staticCls: true}
	queue := append([]*parser.ClassStatement(nil), childrenMap[staticCls]...)
	for len(queue) > 0 {
		subCS := queue[0]
		queue = queue[1:]
		if subCS == nil || subCS.Name == nil {
			continue
		}
		subName := subCS.Name.Value
		if visited[subName] {
			continue
		}
		visited[subName] = true
		queue = append(queue, childrenMap[subName]...)
		if subCS.IsAbstract {
			continue
		}
		implFn, implRet, implStmt, found := l.resolveClassMethod(subName, methodName)
		if !found || implStmt == nil || implStmt.IsStatic || implStmt.IsAbstract || implStmt.Body == nil {
			continue
		}
		if implFn == staticImplFn {
			continue
		}
		implOwner := strings.TrimSuffix(implFn, "_"+methodName)
		overrides = append(overrides, virtualOverride{
			subClass:  subName,
			implFn:    implFn,
			implOwner: implOwner,
			implRet:   implRet,
			implStmt:  implStmt,
		})
	}
	sort.Slice(overrides, func(i, j int) bool {
		return overrides[i].subClass < overrides[j].subClass
	})
	return overrides
}

func (l *Lowerer) ensureVirtualDispatchHelper(staticCls, methodName string, retType Type, paramTypes []Type, overrides []virtualOverride) string {
	vcallName := "__joss_vcall_" + staticCls + "_" + methodName
	if l.prog.Functions[vcallName] != nil {
		return vcallName
	}

	savedFn := l.currentFn
	savedBlock := l.currBlock
	savedLocals := l.varLocals
	savedLoops := l.loopStack
	savedCap := l.activeClosureCaptured
	savedEnv := l.activeClosureEnvMap
	defer func() {
		l.currentFn = savedFn
		l.currBlock = savedBlock
		l.varLocals = savedLocals
		l.loopStack = savedLoops
		l.activeClosureCaptured = savedCap
		l.activeClosureEnvMap = savedEnv
	}()

	fn := NewFunction(vcallName, retType)
	l.currentFn = fn
	l.varLocals = nil
	l.loopStack = nil
	l.activeClosureCaptured = nil
	l.activeClosureEnvMap = nil

	thisVal := fn.AddParam("this", PtrType(StructType(staticCls, nil)))
	fn.DefaultArgs = append(fn.DefaultArgs, nil)
	params := make([]Value, 1, 1+len(paramTypes))
	params[0] = thisVal
	for idx, pt := range paramTypes {
		pv := fn.AddParam(fmt.Sprintf("arg%d", idx), pt)
		fn.DefaultArgs = append(fn.DefaultArgs, nil)
		params = append(params, pv)
	}

	curBlk := fn.EntryBlock
	clsName := fn.NewTemp(TypeString, "vcall_cls")
	curBlk.AddInstruction(&LoadFieldInst{
		Dest:      clsName,
		Obj:       thisVal,
		FieldName: "__joss_cls",
	})

	emitCallAndReturn := func(blk *BasicBlock, callee string, mStmt *parser.MethodStatement, targetRet Type) {
		l.currBlock = blk
		callArgs := make([]Value, len(params))
		copy(callArgs, params)
		if mStmt != nil {
			for pIdx, p := range mStmt.Parameters {
				argIdx := pIdx + 1
				if argIdx < len(callArgs) && p != nil && p.Type.Literal != "" {
					pType := l.mapType(p.Type.Literal)
					v := l.boxIfMixed(callArgs[argIdx], pType)
					v = l.unboxIfTyped(v, pType)
					callArgs[argIdx] = v
				}
			}
		}
		var callDest *TempValue
		if targetRet.Kind != TypeKindVoid {
			callDest = fn.NewTemp(targetRet, "vcall_ret")
		}
		blk.AddInstruction(&CallInst{
			Dest:   callDest,
			Callee: callee,
			Args:   callArgs,
		})
		if retType.Kind == TypeKindVoid {
			blk.SetTerminator(&ReturnTerminator{})
			return
		}
		if callDest == nil {
			if retType.Kind == TypeKindString {
				blk.SetTerminator(&ReturnTerminator{Val: NewConstString("")})
			} else if retType.Kind == TypeKindPtr || retType.Kind == TypeKindStruct || retType.Kind == TypeKindArray {
				blk.SetTerminator(&ReturnTerminator{Val: NewConstNull(retType)})
			} else {
				blk.SetTerminator(&ReturnTerminator{Val: NewConstInt(0, retType)})
			}
			return
		}
		var retVal Value = callDest
		retVal = l.boxIfMixed(retVal, retType)
		retVal = l.unboxIfTyped(retVal, retType)
		blk.SetTerminator(&ReturnTerminator{Val: retVal})
	}

	for _, ov := range overrides {
		l.pendingVirtualMethods = append(l.pendingVirtualMethods, pendingVirtualMethod{
			className: ov.implOwner,
			method:    ov.implStmt,
		})
		cmp := fn.NewTemp(TypeBool, "is_sub")
		curBlk.AddInstruction(&CompareInst{
			Op:    OpCmpEq,
			Dest:  cmp,
			Left:  clsName,
			Right: NewConstString(ov.subClass),
		})
		hitBlk := fn.NewBlock("vcall_hit")
		nextBlk := fn.NewBlock("vcall_next")
		curBlk.SetTerminator(&BranchTerminator{
			Cond:       cmp,
			TrueBlock:  hitBlk,
			FalseBlock: nextBlk,
		})
		emitCallAndReturn(hitBlk, ov.implFn, ov.implStmt, ov.implRet)
		curBlk = nextBlk
	}

	baseFnName, baseRet, baseStmt, baseFound := l.resolveClassMethod(staticCls, methodName)
	if baseFound && baseStmt != nil && !baseStmt.IsStatic && !baseStmt.IsAbstract && baseStmt.Body != nil {
		baseOwner := strings.TrimSuffix(baseFnName, "_"+methodName)
		l.pendingVirtualMethods = append(l.pendingVirtualMethods, pendingVirtualMethod{
			className: baseOwner,
			method:    baseStmt,
		})
		emitCallAndReturn(curBlk, baseFnName, baseStmt, baseRet)
	} else {
		if retType.Kind == TypeKindVoid {
			curBlk.SetTerminator(&ReturnTerminator{})
		} else if retType.Kind == TypeKindString {
			curBlk.SetTerminator(&ReturnTerminator{Val: NewConstString("")})
		} else if retType.Kind == TypeKindPtr || retType.Kind == TypeKindStruct || retType.Kind == TypeKindArray {
			curBlk.SetTerminator(&ReturnTerminator{Val: NewConstNull(retType)})
		} else {
			curBlk.SetTerminator(&ReturnTerminator{Val: NewConstInt(0, retType)})
		}
	}

	l.prog.AddFunction(fn)
	return vcallName
}

func (l *Lowerer) flushPendingVirtualMethods() error {
	if len(l.pendingVirtualMethods) == 0 {
		return nil
	}
	savedFn := l.currentFn
	savedBlock := l.currBlock
	savedLocals := l.varLocals
	savedLoops := l.loopStack
	savedCap := l.activeClosureCaptured
	savedEnv := l.activeClosureEnvMap
	defer func() {
		l.currentFn = savedFn
		l.currBlock = savedBlock
		l.varLocals = savedLocals
		l.loopStack = savedLoops
		l.activeClosureCaptured = savedCap
		l.activeClosureEnvMap = savedEnv
	}()

	seen := make(map[string]bool)
	for len(l.pendingVirtualMethods) > 0 {
		batch := l.pendingVirtualMethods
		l.pendingVirtualMethods = nil
		for _, item := range batch {
			if item.method == nil || item.method.Name == nil || item.className == "" {
				continue
			}
			fnName := item.className + "_" + item.method.Name.Value
			if seen[fnName] || l.prog.Functions[fnName] != nil {
				continue
			}
			seen[fnName] = true
			l.loopStack = nil
			l.activeClosureCaptured = nil
			l.activeClosureEnvMap = nil
			if err := l.lowerClassMethod(item.className, item.method); err != nil {
				return err
			}
		}
	}
	return nil
}

func isBlueprintMethod(name string) bool {
	switch name {
	case "id", "increments", "integer", "tinyInteger", "smallInteger", "mediumInteger",
		"bigInteger", "unsignedInteger", "unsignedBigInteger", "float", "double", "decimal",
		"char", "string", "text", "mediumText", "longText", "date", "dateTime", "time",
		"timestamp", "timestamps", "softDeletes", "boolean", "json", "enum", "nullable",
		"unsigned", "unique", "default", "comment", "dropColumn", "renameColumn", "index",
		"uniqueIndex", "dropIndex", "foreign", "references", "on", "onDelete", "onUpdate",
		"dropForeign":
		return true
	}
	return false
}
