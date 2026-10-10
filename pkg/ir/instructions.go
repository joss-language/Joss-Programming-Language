package ir

import (
	"fmt"
	"strings"
)

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

	// Objetos y Clases Dinámicas en Heap
	OpNewObject  Opcode = "new_object"
	OpLoadField  Opcode = "load_field"
	OpStoreField Opcode = "store_field"

	// Arrays y Colecciones
	OpNewArray Opcode = "new_array"
	OpArrayGet Opcode = "array_get"
	OpArraySet Opcode = "array_set"

	// Mapas Asociativos
	OpNewMap Opcode = "new_map"
	OpMapGet Opcode = "map_get"
	OpMapSet Opcode = "map_set"

	// Excepciones y Control de Flujo Avanzado
	OpThrow    Opcode = "throw"
	OpTryCatch Opcode = "try_catch"

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
// Instrucciones de Objetos, Arrays y Mapas
// -------------------------------------------------------------

// NewObjectInst: %dst = new_object "ClassName"
type NewObjectInst struct {
	Dest      *TempValue
	ClassName string
}

func (i *NewObjectInst) Opcode() Opcode     { return OpNewObject }
func (i *NewObjectInst) Result() *TempValue { return i.Dest }
func (i *NewObjectInst) Operands() []Value  { return nil }
func (i *NewObjectInst) String() string {
	return fmt.Sprintf("%s = new_object %q", i.Dest, i.ClassName)
}

// LoadFieldInst: %dst = load_field %obj, "fieldName"
type LoadFieldInst struct {
	Dest      *TempValue
	Obj       Value
	FieldName string
}

func (i *LoadFieldInst) Opcode() Opcode     { return OpLoadField }
func (i *LoadFieldInst) Result() *TempValue { return i.Dest }
func (i *LoadFieldInst) Operands() []Value  { return []Value{i.Obj} }
func (i *LoadFieldInst) String() string {
	return fmt.Sprintf("%s = load_field %s, %q", i.Dest, i.Obj, i.FieldName)
}

// StoreFieldInst: store_field %val, %obj, "fieldName"
type StoreFieldInst struct {
	Val       Value
	Obj       Value
	FieldName string
}

func (i *StoreFieldInst) Opcode() Opcode     { return OpStoreField }
func (i *StoreFieldInst) Result() *TempValue { return nil }
func (i *StoreFieldInst) Operands() []Value  { return []Value{i.Val, i.Obj} }
func (i *StoreFieldInst) String() string {
	return fmt.Sprintf("store_field %s, %s, %q", i.Val, i.Obj, i.FieldName)
}

// NewArrayInst: %dst = new_array (elements...)
type NewArrayInst struct {
	Dest     *TempValue
	ElemType Type
	Elements []Value
}

func (i *NewArrayInst) Opcode() Opcode     { return OpNewArray }
func (i *NewArrayInst) Result() *TempValue { return i.Dest }
func (i *NewArrayInst) Operands() []Value  { return i.Elements }
func (i *NewArrayInst) String() string {
	elemStrs := make([]string, len(i.Elements))
	for idx, e := range i.Elements {
		elemStrs[idx] = e.String()
	}
	return fmt.Sprintf("%s = new_array<%s> [%s]", i.Dest, i.ElemType, strings.Join(elemStrs, ", "))
}

// ArrayGetInst: %dst = array_get %arr, %idx
type ArrayGetInst struct {
	Dest  *TempValue
	Array Value
	Index Value
}

func (i *ArrayGetInst) Opcode() Opcode     { return OpArrayGet }
func (i *ArrayGetInst) Result() *TempValue { return i.Dest }
func (i *ArrayGetInst) Operands() []Value  { return []Value{i.Array, i.Index} }
func (i *ArrayGetInst) String() string {
	return fmt.Sprintf("%s = array_get %s[%s]", i.Dest, i.Array, i.Index)
}

// ArraySetInst: array_set %val, %arr, %idx
type ArraySetInst struct {
	Val   Value
	Array Value
	Index Value
}

func (i *ArraySetInst) Opcode() Opcode     { return OpArraySet }
func (i *ArraySetInst) Result() *TempValue { return nil }
func (i *ArraySetInst) Operands() []Value  { return []Value{i.Val, i.Array, i.Index} }
func (i *ArraySetInst) String() string {
	return fmt.Sprintf("array_set %s[%s] = %s", i.Array, i.Index, i.Val)
}

// NewMapInst: %dst = new_map (keyValues...)
type NewMapInst struct {
	Dest  *TempValue
	Pairs [][2]Value // [key, value]
}

func (i *NewMapInst) Opcode() Opcode     { return OpNewMap }
func (i *NewMapInst) Result() *TempValue { return i.Dest }
func (i *NewMapInst) Operands() []Value {
	var ops []Value
	for _, p := range i.Pairs {
		ops = append(ops, p[0], p[1])
	}
	return ops
}
func (i *NewMapInst) String() string {
	pairStrs := make([]string, len(i.Pairs))
	for idx, p := range i.Pairs {
		pairStrs[idx] = fmt.Sprintf("%s: %s", p[0], p[1])
	}
	return fmt.Sprintf("%s = new_map {%s}", i.Dest, strings.Join(pairStrs, ", "))
}

// MapGetInst: %dst = map_get %map, %key
type MapGetInst struct {
	Dest *TempValue
	Map  Value
	Key  Value
}

func (i *MapGetInst) Opcode() Opcode     { return OpMapGet }
func (i *MapGetInst) Result() *TempValue { return i.Dest }
func (i *MapGetInst) Operands() []Value  { return []Value{i.Map, i.Key} }
func (i *MapGetInst) String() string {
	return fmt.Sprintf("%s = map_get %s[%s]", i.Dest, i.Map, i.Key)
}

// MapSetInst: map_set %val, %map, %key
type MapSetInst struct {
	Val Value
	Map Value
	Key Value
}

func (i *MapSetInst) Opcode() Opcode     { return OpMapSet }
func (i *MapSetInst) Result() *TempValue { return nil }
func (i *MapSetInst) Operands() []Value  { return []Value{i.Val, i.Map, i.Key} }
func (i *MapSetInst) String() string {
	return fmt.Sprintf("map_set %s[%s] = %s", i.Map, i.Key, i.Val)
}

// ThrowInst: throw %val
type ThrowInst struct {
	Val Value
}

func (i *ThrowInst) Opcode() Opcode     { return OpThrow }
func (i *ThrowInst) Result() *TempValue { return nil }
func (i *ThrowInst) Operands() []Value  { return []Value{i.Val} }
func (i *ThrowInst) String() string     { return fmt.Sprintf("throw %s", i.Val) }

// TryCatchInst: bloque estructurado try-catch
type TryCatchInst struct {
	ID          int
	TryBlocks   []*BasicBlock
	CatchVar    string
	CatchVarPtr Value
	CatchBlocks []*BasicBlock
}

func (i *TryCatchInst) Opcode() Opcode     { return OpTryCatch }
func (i *TryCatchInst) Result() *TempValue { return nil }
func (i *TryCatchInst) Operands() []Value  { return nil }
func (i *TryCatchInst) String() string     { return fmt.Sprintf("try_catch #%d ($%s)", i.ID, i.CatchVar) }

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
