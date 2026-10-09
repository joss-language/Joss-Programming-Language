package ir

import "fmt"

// Opcode identifica la operación realizada por una instrucción o terminador.
type Opcode string

const (
	// Asignación / Movimiento
	OpMove   Opcode = "move"
	OpAlloca Opcode = "alloca"
	OpLoad   Opcode = "load"
	OpStore  Opcode = "store"

	// Aritmética
	OpAdd Opcode = "add"
	OpSub Opcode = "sub"
	OpMul Opcode = "mul"
	OpDiv Opcode = "div"
	OpMod Opcode = "mod"
	OpNeg Opcode = "neg"

	// Lógica / Bits
	OpNot Opcode = "not"
	OpAnd Opcode = "and"
	OpOr  Opcode = "or"
	OpXor Opcode = "xor"
	OpShl Opcode = "shl"
	OpShr Opcode = "shr"

	// Comparaciones
	OpCmpEq     Opcode = "cmp_eq"
	OpCmpNe     Opcode = "cmp_ne"
	OpCmpLt     Opcode = "cmp_lt"
	OpCmpLe     Opcode = "cmp_le"
	OpCmpGt     Opcode = "cmp_gt"
	OpCmpGe     Opcode = "cmp_ge"
	OpSpaceship Opcode = "cmp_spaceship"

	// Llamadas
	OpCall        Opcode = "call"
	OpCallRuntime Opcode = "call_runtime"

	// Conversión de tipos
	OpCast Opcode = "cast"

	// Terminadores de Bloque Básico
	OpReturn      Opcode = "return"
	OpJump        Opcode = "jump"
	OpBranch      Opcode = "branch"
	OpUnreachable Opcode = "unreachable"
)

// Instruction es la interfaz para cualquier operación dentro de un bloque básico.
type Instruction interface {
	Opcode() Opcode
	Result() *TempValue // Puede ser nil si la instrucción no produce valor
	Operands() []Value
	String() string
}

// Terminator es una instrucción especial que transfiere el flujo de control al salir del bloque.
type Terminator interface {
	Instruction
	Successors() []*BasicBlock
}

// -------------------------------------------------------------
// Instrucciones Regulares
// -------------------------------------------------------------

// MoveInst: %dst = move %src
type MoveInst struct {
	Dest *TempValue
	Src  Value
}

func (i *MoveInst) Opcode() Opcode     { return OpMove }
func (i *MoveInst) Result() *TempValue { return i.Dest }
func (i *MoveInst) Operands() []Value  { return []Value{i.Src} }
func (i *MoveInst) String() string     { return fmt.Sprintf("%s = move %s", i.Dest, i.Src) }

// AllocaInst: %dst = alloca Type
type AllocaInst struct {
	Dest      *TempValue
	AllocType Type
}

func (i *AllocaInst) Opcode() Opcode     { return OpAlloca }
func (i *AllocaInst) Result() *TempValue { return i.Dest }
func (i *AllocaInst) Operands() []Value  { return nil }
func (i *AllocaInst) String() string     { return fmt.Sprintf("%s = alloca %s", i.Dest, i.AllocType) }

// LoadInst: %dst = load %ptr
type LoadInst struct {
	Dest *TempValue
	Ptr  Value
}

func (i *LoadInst) Opcode() Opcode     { return OpLoad }
func (i *LoadInst) Result() *TempValue { return i.Dest }
func (i *LoadInst) Operands() []Value  { return []Value{i.Ptr} }
func (i *LoadInst) String() string     { return fmt.Sprintf("%s = load %s", i.Dest, i.Ptr) }

// StoreInst: store %val, %ptr
type StoreInst struct {
	Val Value
	Ptr Value
}

func (i *StoreInst) Opcode() Opcode     { return OpStore }
func (i *StoreInst) Result() *TempValue { return nil }
func (i *StoreInst) Operands() []Value  { return []Value{i.Val, i.Ptr} }
func (i *StoreInst) String() string     { return fmt.Sprintf("store %s, %s", i.Val, i.Ptr) }

// BinaryInst: %dst = op %left, %right
type BinaryInst struct {
	Op    Opcode
	Dest  *TempValue
	Left  Value
	Right Value
}

func (i *BinaryInst) Opcode() Opcode     { return i.Op }
func (i *BinaryInst) Result() *TempValue { return i.Dest }
func (i *BinaryInst) Operands() []Value  { return []Value{i.Left, i.Right} }
func (i *BinaryInst) String() string {
	return fmt.Sprintf("%s = %s %s, %s", i.Dest, i.Op, i.Left, i.Right)
}

// UnaryInst: %dst = op %val
type UnaryInst struct {
	Op   Opcode
	Dest *TempValue
	Val  Value
}

func (i *UnaryInst) Opcode() Opcode     { return i.Op }
func (i *UnaryInst) Result() *TempValue { return i.Dest }
func (i *UnaryInst) Operands() []Value  { return []Value{i.Val} }
func (i *UnaryInst) String() string     { return fmt.Sprintf("%s = %s %s", i.Dest, i.Op, i.Val) }

// CompareInst: %dst = cmp_cond %left, %right
type CompareInst struct {
	Op    Opcode
	Dest  *TempValue
	Left  Value
	Right Value
}

func (i *CompareInst) Opcode() Opcode     { return i.Op }
func (i *CompareInst) Result() *TempValue { return i.Dest }
func (i *CompareInst) Operands() []Value  { return []Value{i.Left, i.Right} }
func (i *CompareInst) String() string {
	return fmt.Sprintf("%s = %s %s, %s", i.Dest, i.Op, i.Left, i.Right)
}

// CallInst: [%dst =] call Callee(args...)
type CallInst struct {
	Dest   *TempValue // Puede ser nil si retorna void
	Callee string
	Args   []Value
}

func (i *CallInst) Opcode() Opcode     { return OpCall }
func (i *CallInst) Result() *TempValue { return i.Dest }
func (i *CallInst) Operands() []Value  { return i.Args }
func (i *CallInst) String() string {
	argsStr := ""
	for idx, a := range i.Args {
		if idx > 0 {
			argsStr += ", "
		}
		argsStr += a.String()
	}
	if i.Dest != nil {
		return fmt.Sprintf("%s = call @%s(%s)", i.Dest, i.Callee, argsStr)
	}
	return fmt.Sprintf("call @%s(%s)", i.Callee, argsStr)
}

// CallRuntimeInst: [%dst =] call_runtime "func"(args...)
type CallRuntimeInst struct {
	Dest *TempValue
	Func string
	Args []Value
}

func (i *CallRuntimeInst) Opcode() Opcode     { return OpCallRuntime }
func (i *CallRuntimeInst) Result() *TempValue { return i.Dest }
func (i *CallRuntimeInst) Operands() []Value  { return i.Args }
func (i *CallRuntimeInst) String() string {
	argsStr := ""
	for idx, a := range i.Args {
		if idx > 0 {
			argsStr += ", "
		}
		argsStr += a.String()
	}
	if i.Dest != nil {
		return fmt.Sprintf("%s = call_runtime %q(%s)", i.Dest, i.Func, argsStr)
	}
	return fmt.Sprintf("call_runtime %q(%s)", i.Func, argsStr)
}

// CastInst: %dst = cast %val to TargetType
type CastInst struct {
	Dest       *TempValue
	Val        Value
	TargetType Type
}

func (i *CastInst) Opcode() Opcode     { return OpCast }
func (i *CastInst) Result() *TempValue { return i.Dest }
func (i *CastInst) Operands() []Value  { return []Value{i.Val} }
func (i *CastInst) String() string {
	return fmt.Sprintf("%s = cast %s to %s", i.Dest, i.Val, i.TargetType)
}

// -------------------------------------------------------------
// Terminadores
// -------------------------------------------------------------

// ReturnTerminator: return [%val]
type ReturnTerminator struct {
	Val Value // Puede ser nil si la función es void
}

func (t *ReturnTerminator) Opcode() Opcode     { return OpReturn }
func (t *ReturnTerminator) Result() *TempValue { return nil }
func (t *ReturnTerminator) Operands() []Value {
	if t.Val != nil {
		return []Value{t.Val}
	}
	return nil
}
func (t *ReturnTerminator) Successors() []*BasicBlock { return nil }
func (t *ReturnTerminator) String() string {
	if t.Val != nil {
		return fmt.Sprintf("return %s", t.Val)
	}
	return "return"
}

// JumpTerminator: jump TargetBlock
type JumpTerminator struct {
	Target *BasicBlock
}

func (t *JumpTerminator) Opcode() Opcode            { return OpJump }
func (t *JumpTerminator) Result() *TempValue        { return nil }
func (t *JumpTerminator) Operands() []Value         { return nil }
func (t *JumpTerminator) Successors() []*BasicBlock { return []*BasicBlock{t.Target} }
func (t *JumpTerminator) String() string            { return fmt.Sprintf("jump %s", t.Target.Label) }

// BranchTerminator: branch %cond, TrueBlock, FalseBlock
type BranchTerminator struct {
	Cond       Value
	TrueBlock  *BasicBlock
	FalseBlock *BasicBlock
}

func (t *BranchTerminator) Opcode() Opcode     { return OpBranch }
func (t *BranchTerminator) Result() *TempValue { return nil }
func (t *BranchTerminator) Operands() []Value  { return []Value{t.Cond} }
func (t *BranchTerminator) Successors() []*BasicBlock {
	return []*BasicBlock{t.TrueBlock, t.FalseBlock}
}
func (t *BranchTerminator) String() string {
	return fmt.Sprintf("branch %s, %s, %s", t.Cond, t.TrueBlock.Label, t.FalseBlock.Label)
}

// UnreachableTerminator: unreachable
type UnreachableTerminator struct{}

func (t *UnreachableTerminator) Opcode() Opcode            { return OpUnreachable }
func (t *UnreachableTerminator) Result() *TempValue        { return nil }
func (t *UnreachableTerminator) Operands() []Value         { return nil }
func (t *UnreachableTerminator) Successors() []*BasicBlock { return nil }
func (t *UnreachableTerminator) String() string            { return "unreachable" }
