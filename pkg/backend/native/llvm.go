package native

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jossecurity/joss/pkg/ir"
)

// LLVMEmitter traduce un ir.Program válido a LLVM IR textual (.ll).
type LLVMEmitter struct {
	stringConstants map[string]string // texto -> @.str.N
	nextStringID    int
}

func NewLLVMEmitter() *LLVMEmitter {
	return &LLVMEmitter{
		stringConstants: make(map[string]string),
	}
}

func (e *LLVMEmitter) Emit(prog *ir.Program) (string, error) {
	if prog == nil {
		return "", fmt.Errorf("llvm emitter: programa nil")
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("; ModuleID = '%s'\n", prog.Name))
	buf.WriteString("source_filename = \"" + prog.Name + ".joss\"\n\n")

	// Pre-escanear constantes de string en todo el programa para declararlas como globales
	for _, fn := range prog.Functions {
		for _, block := range fn.Blocks {
			for _, inst := range block.Instructions {
				for _, op := range inst.Operands() {
					if cs, ok := op.(*ir.ConstString); ok {
						e.registerString(cs.Val)
					}
				}
			}
			if block.Terminator != nil {
				for _, op := range block.Terminator.Operands() {
					if cs, ok := op.(*ir.ConstString); ok {
						e.registerString(cs.Val)
					}
				}
			}
		}
	}

	// Declarar constantes globales de string
	if len(e.stringConstants) > 0 {
		buf.WriteString("; String Constants\n")
		// Ordenar para determinismo
		keys := make([]string, 0, len(e.stringConstants))
		for k := range e.stringConstants {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, str := range keys {
			sym := e.stringConstants[str]
			escaped, length := formatLLVMStringLiteral(str)
			buf.WriteString(fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\", align 1\n", sym, length, escaped))
		}
		buf.WriteString("\n")
	}

	// Declarar funciones de runtime requeridas
	buf.WriteString("; Declaraciones de Runtime Nativo Joss\n")
	buf.WriteString("declare void @joss_print_i64(i64)\n")
	buf.WriteString("declare void @joss_print_f64(double)\n")
	buf.WriteString("declare void @joss_print_bool(i1)\n")
	buf.WriteString("declare void @joss_print_string(i8*)\n")
	buf.WriteString("declare void @joss_panic(i8*)\n\n")

	// Emitir funciones de usuario
	fnNames := make([]string, 0, len(prog.Functions))
	for name := range prog.Functions {
		fnNames = append(fnNames, name)
	}
	sort.Strings(fnNames)

	for _, name := range fnNames {
		fn := prog.Functions[name]
		if err := e.emitFunction(&buf, fn); err != nil {
			return "", err
		}
		buf.WriteString("\n")
	}

	return buf.String(), nil
}

func (e *LLVMEmitter) registerString(val string) string {
	if sym, ok := e.stringConstants[val]; ok {
		return sym
	}
	sym := fmt.Sprintf("@.str.%d", e.nextStringID)
	e.nextStringID++
	e.stringConstants[val] = sym
	return sym
}

func (e *LLVMEmitter) emitFunction(buf *bytes.Buffer, fn *ir.Function) error {
	retType := MapLLVMType(fn.ReturnType)

	// Parámetros
	paramStrs := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		paramStrs[i] = fmt.Sprintf("%s %s", MapLLVMType(p.ValueType), e.formatValue(p))
	}

	buf.WriteString(fmt.Sprintf("define %s @%s(%s) {\n", retType, fn.Name, strings.Join(paramStrs, ", ")))

	for _, block := range fn.Blocks {
		buf.WriteString(fmt.Sprintf("%s:\n", block.Label))

		for _, inst := range block.Instructions {
			line, err := e.emitInstruction(inst)
			if err != nil {
				return err
			}
			if line != "" {
				buf.WriteString("    " + line + "\n")
			}
		}

		if block.Terminator != nil {
			termLine, err := e.emitTerminator(fn, block.Terminator)
			if err != nil {
				return err
			}
			buf.WriteString("    " + termLine + "\n")
		} else {
			return fmt.Errorf("llvm emitter: bloque %s en @%s no tiene terminador", block.Label, fn.Name)
		}
	}

	buf.WriteString("}\n")
	return nil
}

func (e *LLVMEmitter) emitInstruction(inst ir.Instruction) (string, error) {
	switch i := inst.(type) {
	case *ir.AllocaInst:
		allocType := MapLLVMType(i.AllocType)
		return fmt.Sprintf("%s = alloca %s, align 8", e.formatValue(i.Dest), allocType), nil

	case *ir.StoreInst:
		valStr := e.formatTypedValue(i.Val)
		ptrType := MapLLVMType(i.Ptr.Type())
		return fmt.Sprintf("store %s, %s %s, align 8", valStr, ptrType, e.formatValue(i.Ptr)), nil

	case *ir.LoadInst:
		destType := MapLLVMType(i.Dest.ValueType)
		ptrType := MapLLVMType(i.Ptr.Type())
		return fmt.Sprintf("%s = load %s, %s %s, align 8", e.formatValue(i.Dest), destType, ptrType, e.formatValue(i.Ptr)), nil

	case *ir.MoveInst:
		destType := MapLLVMType(i.Dest.ValueType)
		valStr := e.formatValue(i.Src)
		return fmt.Sprintf("%s = add %s 0, %s", e.formatValue(i.Dest), destType, valStr), nil

	case *ir.BinaryInst:
		llvmOp := llvmArithmeticOp(i.Op, i.Dest.ValueType)
		tStr := MapLLVMType(i.Dest.ValueType)
		return fmt.Sprintf("%s = %s %s %s, %s", e.formatValue(i.Dest), llvmOp, tStr, e.formatValue(i.Left), e.formatValue(i.Right)), nil

	case *ir.UnaryInst:
		if i.Op == ir.OpNot {
			return fmt.Sprintf("%s = xor i1 %s, true", e.formatValue(i.Dest), e.formatValue(i.Val)), nil
		}
		if i.Op == ir.OpNeg {
			tStr := MapLLVMType(i.Dest.ValueType)
			if i.Dest.ValueType.IsFloat() {
				return fmt.Sprintf("%s = fsub %s 0.0, %s", e.formatValue(i.Dest), tStr, e.formatValue(i.Val)), nil
			}
			return fmt.Sprintf("%s = sub %s 0, %s", e.formatValue(i.Dest), tStr, e.formatValue(i.Val)), nil
		}

	case *ir.CompareInst:
		if i.Op == ir.OpSpaceship {
			destVal := e.formatValue(i.Dest)
			ltTemp := destVal + "_lt"
			gtTemp := destVal + "_gt"
			selTemp := destVal + "_sel"
			tStr := MapLLVMType(i.Left.Type())
			return fmt.Sprintf("%s = icmp slt %s %s, %s\n\t%s = icmp sgt %s %s, %s\n\t%s = select i1 %s, i64 1, i64 0\n\t%s = select i1 %s, i64 -1, i64 %s",
				ltTemp, tStr, e.formatValue(i.Left), e.formatValue(i.Right),
				gtTemp, tStr, e.formatValue(i.Left), e.formatValue(i.Right),
				selTemp, gtTemp,
				destVal, ltTemp, selTemp), nil
		}
		cmpOp := llvmCmpOp(i.Op, i.Left.Type())
		tStr := MapLLVMType(i.Left.Type())
		return fmt.Sprintf("%s = %s %s %s, %s", e.formatValue(i.Dest), cmpOp, tStr, e.formatValue(i.Left), e.formatValue(i.Right)), nil

	case *ir.CallInst:
		args := make([]string, len(i.Args))
		for idx, a := range i.Args {
			args[idx] = e.formatTypedValue(a)
		}
		argsJoined := strings.Join(args, ", ")
		if i.Dest != nil {
			destType := MapLLVMType(i.Dest.ValueType)
			return fmt.Sprintf("%s = call %s @%s(%s)", e.formatValue(i.Dest), destType, i.Callee, argsJoined), nil
		}
		return fmt.Sprintf("call void @%s(%s)", i.Callee, argsJoined), nil

	case *ir.CallRuntimeInst:
		rtFunc := "joss_" + i.Func
		args := make([]string, len(i.Args))
		for idx, a := range i.Args {
			args[idx] = e.formatTypedValue(a)
		}
		argsJoined := strings.Join(args, ", ")
		if i.Dest != nil {
			destType := MapLLVMType(i.Dest.ValueType)
			return fmt.Sprintf("%s = call %s @%s(%s)", e.formatValue(i.Dest), destType, rtFunc, argsJoined), nil
		}
		return fmt.Sprintf("call void @%s(%s)", rtFunc, argsJoined), nil
	}

	return "", fmt.Errorf("llvm emitter: instrucción no soportada %s", inst)
}

func (e *LLVMEmitter) emitTerminator(fn *ir.Function, term ir.Terminator) (string, error) {
	switch t := term.(type) {
	case *ir.ReturnTerminator:
		if t.Val == nil || fn.ReturnType.Kind == ir.TypeKindVoid {
			return "ret void", nil
		}
		return fmt.Sprintf("ret %s", e.formatTypedValue(t.Val)), nil

	case *ir.JumpTerminator:
		return fmt.Sprintf("br label %%%s", t.Target.Label), nil

	case *ir.BranchTerminator:
		return fmt.Sprintf("br i1 %s, label %%%s, label %%%s", e.formatValue(t.Cond), t.TrueBlock.Label, t.FalseBlock.Label), nil

	case *ir.UnreachableTerminator:
		return "unreachable", nil
	}

	return "", fmt.Errorf("llvm emitter: terminador no soportado %s", term)
}

func (e *LLVMEmitter) formatTypedValue(val ir.Value) string {
	if cs, ok := val.(*ir.ConstString); ok {
		sym := e.registerString(cs.Val)
		_, length := formatLLVMStringLiteral(cs.Val)
		return fmt.Sprintf("i8* getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0)", length, length, sym)
	}
	return fmt.Sprintf("%s %s", MapLLVMType(val.Type()), e.formatValue(val))
}

func (e *LLVMEmitter) formatValue(val ir.Value) string {
	switch v := val.(type) {
	case *ir.TempValue:
		return fmt.Sprintf("%%t%d", v.ID)
	case *ir.ParamValue:
		if v.Name != "" {
			return "%" + cleanIdent(v.Name)
		}
		return fmt.Sprintf("%%arg%d", v.Index)
	case *ir.ConstInt:
		return strconv.FormatInt(v.Val, 10)
	case *ir.ConstFloat:
		return strconv.FormatFloat(v.Val, 'g', -1, 64)
	case *ir.ConstBool:
		if v.Val {
			return "true"
		}
		return "false"
	case *ir.ConstString:
		sym := e.registerString(v.Val)
		_, length := formatLLVMStringLiteral(v.Val)
		return fmt.Sprintf("getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0)", length, length, sym)
	case *ir.ConstNull:
		return "null"
	default:
		return val.String()
	}
}

func llvmArithmeticOp(op ir.Opcode, t ir.Type) string {
	isFloat := t.IsFloat()
	switch op {
	case ir.OpAdd:
		if isFloat {
			return "fadd"
		}
		return "add"
	case ir.OpSub:
		if isFloat {
			return "fsub"
		}
		return "sub"
	case ir.OpMul:
		if isFloat {
			return "fmul"
		}
		return "mul"
	case ir.OpDiv:
		if isFloat {
			return "fdiv"
		}
		return "sdiv"
	case ir.OpMod:
		if isFloat {
			return "frem"
		}
		return "srem"
	case ir.OpShl:
		return "shl"
	case ir.OpShr:
		return "ashr"
	case ir.OpAnd:
		return "and"
	case ir.OpOr:
		return "or"
	case ir.OpXor:
		return "xor"
	default:
		return "add"
	}
}

func llvmCmpOp(op ir.Opcode, t ir.Type) string {
	isFloat := t.IsFloat()
	if isFloat {
		switch op {
		case ir.OpCmpEq:
			return "fcmp oeq"
		case ir.OpCmpNe:
			return "fcmp one"
		case ir.OpCmpLt:
			return "fcmp olt"
		case ir.OpCmpLe:
			return "fcmp ole"
		case ir.OpCmpGt:
			return "fcmp ogt"
		case ir.OpCmpGe:
			return "fcmp oge"
		}
	}
	switch op {
	case ir.OpCmpEq:
		return "icmp eq"
	case ir.OpCmpNe:
		return "icmp ne"
	case ir.OpCmpLt:
		return "icmp slt"
	case ir.OpCmpLe:
		return "icmp sle"
	case ir.OpCmpGt:
		return "icmp sgt"
	case ir.OpCmpGe:
		return "icmp sge"
	default:
		return "icmp eq"
	}
}

func cleanIdent(name string) string {
	return strings.TrimPrefix(name, "$")
}

func formatLLVMStringLiteral(s string) (string, int) {
	var buf bytes.Buffer
	length := len(s) + 1 // + null terminator
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 32 && b <= 126 && b != '"' && b != '\\' {
			buf.WriteByte(b)
		} else {
			buf.WriteString(fmt.Sprintf("\\%02X", b))
		}
	}
	buf.WriteString("\\00")
	return buf.String(), length
}
