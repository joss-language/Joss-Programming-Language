package golite

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

	"github.com/jossecurity/joss/pkg/ir"
)

// CompileToIR translates a pure algorithmic Go-Lite source file into Joss Native IR.
// Supported subset:
// - Package-level functions with int64/int/bool parameters and return types
// - Local var declarations (:= and var), assignments (=, +=, -=)
// - Control flow: if/else, for (init; cond; post and while-style), return
// - Expressions: integer/bool literals, identifiers, binary ops (+,-,*,/,%,==,!=,<,<=,>,>=,&&,||), unary (-,!), calls
func CompileToIR(programName string, goSource string) (*ir.Program, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, programName+".go", goSource, 0)
	if err != nil {
		return nil, fmt.Errorf("golite parse error: %w", err)
	}

	prog := ir.NewProgram(programName)
	c := &compiler{
		prog:      prog,
		fnSigs:    make(map[string]ir.Type),
		varLocals: make(map[string]ir.Value),
	}

	// First pass: register function signatures
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		retType := ir.TypeVoid
		if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
			retType = mapGoType(fn.Type.Results.List[0].Type)
		}
		c.fnSigs[fn.Name.Name] = retType
	}

	// Second pass: lower function bodies
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Body == nil {
			continue
		}
		if err := c.compileFunc(fn); err != nil {
			return nil, err
		}
	}

	return prog, nil
}

type compiler struct {
	prog      *ir.Program
	fnSigs    map[string]ir.Type
	currFn    *ir.Function
	currBlock *ir.BasicBlock
	varLocals map[string]ir.Value
	blockSeq  int
}

func mapGoType(expr ast.Expr) ir.Type {
	if ident, ok := expr.(*ast.Ident); ok {
		switch ident.Name {
		case "int", "int64", "int32", "uint64":
			return ir.TypeI64
		case "float64":
			return ir.TypeF64
		case "bool":
			return ir.TypeBool
		case "string":
			return ir.TypeString
		}
	}
	return ir.TypeI64
}

func (c *compiler) newBlock(prefix string) *ir.BasicBlock {
	c.blockSeq++
	return c.currFn.NewBlock(fmt.Sprintf("%s_%d", prefix, c.blockSeq))
}

func (c *compiler) compileFunc(decl *ast.FuncDecl) error {
	retType := c.fnSigs[decl.Name.Name]
	fn := ir.NewFunction(decl.Name.Name, retType)
	c.currFn = fn
	c.currBlock = fn.EntryBlock
	c.varLocals = make(map[string]ir.Value)

	if decl.Type.Params != nil {
		for _, field := range decl.Type.Params.List {
			pType := mapGoType(field.Type)
			for _, name := range field.Names {
				paramVal := fn.AddParam(name.Name, pType)
				ptr := fn.NewTemp(ir.PtrType(pType), name.Name+"_ptr")
				fn.EntryBlock.AddInstruction(&ir.AllocaInst{Dest: ptr, AllocType: pType})
				c.currBlock.AddInstruction(&ir.StoreInst{Val: paramVal, Ptr: ptr})
				c.varLocals[name.Name] = ptr
			}
		}
	}

	for _, stmt := range decl.Body.List {
		if c.currBlock.Terminator != nil {
			break
		}
		if err := c.compileStmt(stmt); err != nil {
			return err
		}
	}

	if c.currBlock.Terminator == nil {
		if retType.Kind == ir.TypeKindVoid {
			c.currBlock.SetTerminator(&ir.ReturnTerminator{})
		} else {
			c.currBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, retType)})
		}
	}

	c.prog.AddFunction(fn)
	return nil
}

func (c *compiler) compileStmt(stmt ast.Stmt) error {
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		if len(s.Results) == 0 {
			c.currBlock.SetTerminator(&ir.ReturnTerminator{})
			return nil
		}
		val, err := c.compileExpr(s.Results[0])
		if err != nil {
			return err
		}
		c.currBlock.SetTerminator(&ir.ReturnTerminator{Val: val})
		return nil

	case *ast.AssignStmt:
		if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
			return fmt.Errorf("golite: multi-assignment not supported")
		}
		ident, ok := s.Lhs[0].(*ast.Ident)
		if !ok {
			return fmt.Errorf("golite: assignment LHS must be identifier")
		}
		rhs, err := c.compileExpr(s.Rhs[0])
		if err != nil {
			return err
		}
		if s.Tok == token.DEFINE {
			ptr := c.currFn.NewTemp(ir.PtrType(rhs.Type()), ident.Name+"_ptr")
			c.currFn.EntryBlock.AddInstruction(&ir.AllocaInst{Dest: ptr, AllocType: rhs.Type()})
			c.currBlock.AddInstruction(&ir.StoreInst{Val: rhs, Ptr: ptr})
			c.varLocals[ident.Name] = ptr
			return nil
		}
		ptr, ok := c.varLocals[ident.Name]
		if !ok {
			return fmt.Errorf("golite: undefined variable %s", ident.Name)
		}
		if s.Tok == token.ADD_ASSIGN || s.Tok == token.SUB_ASSIGN {
			cur := c.currFn.NewTemp(ptr.Type().ElemType(), ident.Name+"_cur")
			c.currBlock.AddInstruction(&ir.LoadInst{Dest: cur, Ptr: ptr})
			res := c.currFn.NewTemp(cur.Type(), ident.Name+"_upd")
			op := ir.OpAdd
			if s.Tok == token.SUB_ASSIGN {
				op = ir.OpSub
			}
			c.currBlock.AddInstruction(&ir.BinaryInst{Op: op, Dest: res, Left: cur, Right: rhs})
			rhs = res
		}
		c.currBlock.AddInstruction(&ir.StoreInst{Val: rhs, Ptr: ptr})
		return nil

	case *ast.IncDecStmt:
		ident, ok := s.X.(*ast.Ident)
		if !ok {
			return fmt.Errorf("golite: inc/dec target must be identifier")
		}
		ptr, ok := c.varLocals[ident.Name]
		if !ok {
			return fmt.Errorf("golite: undefined variable %s", ident.Name)
		}
		cur := c.currFn.NewTemp(ptr.Type().ElemType(), ident.Name+"_cur")
		c.currBlock.AddInstruction(&ir.LoadInst{Dest: cur, Ptr: ptr})
		op := ir.OpAdd
		if s.Tok == token.DEC {
			op = ir.OpSub
		}
		res := c.currFn.NewTemp(cur.Type(), ident.Name+"_inc")
		c.currBlock.AddInstruction(&ir.BinaryInst{Op: op, Dest: res, Left: cur, Right: ir.NewConstInt(1, cur.Type())})
		c.currBlock.AddInstruction(&ir.StoreInst{Val: res, Ptr: ptr})
		return nil

	case *ast.DeclStmt:
		gen, ok := s.Decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			return fmt.Errorf("golite: unsupported declaration")
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			varType := ir.TypeI64
			if vs.Type != nil {
				varType = mapGoType(vs.Type)
			}
			for i, name := range vs.Names {
				var val ir.Value = ir.NewConstInt(0, varType)
				if i < len(vs.Values) {
					v, err := c.compileExpr(vs.Values[i])
					if err != nil {
						return err
					}
					val = v
					if vs.Type == nil {
						varType = v.Type()
					}
				}
				ptr := c.currFn.NewTemp(ir.PtrType(varType), name.Name+"_ptr")
				c.currFn.EntryBlock.AddInstruction(&ir.AllocaInst{Dest: ptr, AllocType: varType})
				c.currBlock.AddInstruction(&ir.StoreInst{Val: val, Ptr: ptr})
				c.varLocals[name.Name] = ptr
			}
		}
		return nil

	case *ast.IfStmt:
		if s.Init != nil {
			if err := c.compileStmt(s.Init); err != nil {
				return err
			}
		}
		cond, err := c.compileExpr(s.Cond)
		if err != nil {
			return err
		}
		thenBlk := c.newBlock("if_then")
		elseBlk := c.newBlock("if_else")
		mergeBlk := c.newBlock("if_merge")

		c.currBlock.SetTerminator(&ir.BranchTerminator{Cond: cond, TrueBlock: thenBlk, FalseBlock: elseBlk})

		c.currBlock = thenBlk
		for _, st := range s.Body.List {
			if c.currBlock.Terminator != nil {
				break
			}
			if err := c.compileStmt(st); err != nil {
				return err
			}
		}
		if c.currBlock.Terminator == nil {
			c.currBlock.SetTerminator(&ir.JumpTerminator{Target: mergeBlk})
		}

		c.currBlock = elseBlk
		if s.Else != nil {
			switch e := s.Else.(type) {
			case *ast.BlockStmt:
				for _, st := range e.List {
					if c.currBlock.Terminator != nil {
						break
					}
					if err := c.compileStmt(st); err != nil {
						return err
					}
				}
			default:
				if err := c.compileStmt(e); err != nil {
					return err
				}
			}
		}
		if c.currBlock.Terminator == nil {
			c.currBlock.SetTerminator(&ir.JumpTerminator{Target: mergeBlk})
		}

		c.currBlock = mergeBlk
		return nil

	case *ast.ForStmt:
		if s.Init != nil {
			if err := c.compileStmt(s.Init); err != nil {
				return err
			}
		}
		condBlk := c.newBlock("for_cond")
		bodyBlk := c.newBlock("for_body")
		postBlk := c.newBlock("for_post")
		exitBlk := c.newBlock("for_exit")

		c.currBlock.SetTerminator(&ir.JumpTerminator{Target: condBlk})
		c.currBlock = condBlk
		if s.Cond != nil {
			cond, err := c.compileExpr(s.Cond)
			if err != nil {
				return err
			}
			c.currBlock.SetTerminator(&ir.BranchTerminator{Cond: cond, TrueBlock: bodyBlk, FalseBlock: exitBlk})
		} else {
			c.currBlock.SetTerminator(&ir.JumpTerminator{Target: bodyBlk})
		}

		c.currBlock = bodyBlk
		for _, st := range s.Body.List {
			if c.currBlock.Terminator != nil {
				break
			}
			if err := c.compileStmt(st); err != nil {
				return err
			}
		}
		if c.currBlock.Terminator == nil {
			c.currBlock.SetTerminator(&ir.JumpTerminator{Target: postBlk})
		}

		c.currBlock = postBlk
		if s.Post != nil {
			if err := c.compileStmt(s.Post); err != nil {
				return err
			}
		}
		c.currBlock.SetTerminator(&ir.JumpTerminator{Target: condBlk})

		c.currBlock = exitBlk
		return nil

	case *ast.ExprStmt:
		_, err := c.compileExpr(s.X)
		return err
	}

	return fmt.Errorf("golite: unsupported statement %T", stmt)
}

func (c *compiler) compileExpr(expr ast.Expr) (ir.Value, error) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			v, err := strconv.ParseInt(e.Value, 0, 64)
			if err != nil {
				return nil, err
			}
			return ir.NewConstInt(v, ir.TypeI64), nil
		case token.STRING:
			s, err := strconv.Unquote(e.Value)
			if err != nil {
				return nil, err
			}
			return ir.NewConstString(s), nil
		}

	case *ast.Ident:
		if e.Name == "true" {
			return ir.NewConstBool(true), nil
		}
		if e.Name == "false" {
			return ir.NewConstBool(false), nil
		}
		ptr, ok := c.varLocals[e.Name]
		if !ok {
			return nil, fmt.Errorf("golite: undefined identifier %s", e.Name)
		}
		dest := c.currFn.NewTemp(ptr.Type().ElemType(), e.Name)
		c.currBlock.AddInstruction(&ir.LoadInst{Dest: dest, Ptr: ptr})
		return dest, nil

	case *ast.ParenExpr:
		return c.compileExpr(e.X)

	case *ast.UnaryExpr:
		val, err := c.compileExpr(e.X)
		if err != nil {
			return nil, err
		}
		switch e.Op {
		case token.SUB:
			dest := c.currFn.NewTemp(val.Type(), "neg")
			c.currBlock.AddInstruction(&ir.BinaryInst{Op: ir.OpSub, Dest: dest, Left: ir.NewConstInt(0, val.Type()), Right: val})
			return dest, nil
		case token.NOT:
			dest := c.currFn.NewTemp(ir.TypeBool, "not")
			c.currBlock.AddInstruction(&ir.UnaryInst{Op: ir.OpNot, Dest: dest, Val: val})
			return dest, nil
		}

	case *ast.BinaryExpr:
		left, err := c.compileExpr(e.X)
		if err != nil {
			return nil, err
		}
		right, err := c.compileExpr(e.Y)
		if err != nil {
			return nil, err
		}
		var op ir.Opcode
		resType := left.Type()
		isCmp := false
		switch e.Op {
		case token.ADD:
			op = ir.OpAdd
		case token.SUB:
			op = ir.OpSub
		case token.MUL:
			op = ir.OpMul
		case token.QUO:
			op = ir.OpDiv
		case token.REM:
			op = ir.OpMod
		case token.EQL:
			op = ir.OpCmpEq
			resType = ir.TypeBool
			isCmp = true
		case token.NEQ:
			op = ir.OpCmpNe
			resType = ir.TypeBool
			isCmp = true
		case token.LSS:
			op = ir.OpCmpLt
			resType = ir.TypeBool
			isCmp = true
		case token.LEQ:
			op = ir.OpCmpLe
			resType = ir.TypeBool
			isCmp = true
		case token.GTR:
			op = ir.OpCmpGt
			resType = ir.TypeBool
			isCmp = true
		case token.GEQ:
			op = ir.OpCmpGe
			resType = ir.TypeBool
			isCmp = true
		case token.LAND:
			op = ir.OpAnd
			resType = ir.TypeBool
		case token.LOR:
			op = ir.OpOr
			resType = ir.TypeBool
		default:
			return nil, fmt.Errorf("golite: unsupported binary op %s", e.Op)
		}
		dest := c.currFn.NewTemp(resType, "bin")
		if isCmp {
			c.currBlock.AddInstruction(&ir.CompareInst{Op: op, Dest: dest, Left: left, Right: right})
		} else {
			c.currBlock.AddInstruction(&ir.BinaryInst{Op: op, Dest: dest, Left: left, Right: right})
		}
		return dest, nil

	case *ast.CallExpr:
		ident, ok := e.Fun.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("golite: only direct function calls supported")
		}
		args := make([]ir.Value, 0, len(e.Args))
		for _, a := range e.Args {
			v, err := c.compileExpr(a)
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}
		if (ident.Name == "int64" || ident.Name == "int" || ident.Name == "int32") && len(args) == 1 {
			return args[0], nil
		}
		if ident.Name == "println" && len(args) == 1 {
			c.currBlock.AddInstruction(&ir.CallRuntimeInst{Func: "joss_print_i64", Args: []ir.Value{args[0]}})
			return ir.NewConstInt(0, ir.TypeVoid), nil
		}
		retType, ok := c.fnSigs[ident.Name]
		if !ok {
			return nil, fmt.Errorf("golite: unknown function %s", ident.Name)
		}
		var dest *ir.TempValue
		if retType.Kind != ir.TypeKindVoid {
			dest = c.currFn.NewTemp(retType, "call")
		}
		c.currBlock.AddInstruction(&ir.CallInst{Dest: dest, Callee: ident.Name, Args: args})
		if dest != nil {
			return dest, nil
		}
		return ir.NewConstInt(0, ir.TypeVoid), nil
	}

	return nil, fmt.Errorf("golite: unsupported expression %T", expr)
}
