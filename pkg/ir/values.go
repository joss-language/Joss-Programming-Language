package ir

import (
	"fmt"
	"strconv"
)

// ValueKind identifica la naturaleza del valor en la IR.
type ValueKind string

const (
	ValueKindConst  ValueKind = "const"
	ValueKindTemp   ValueKind = "temp"
	ValueKindParam  ValueKind = "param"
	ValueKindGlobal ValueKind = "global"
)

// Value representa un operando o resultado de instrucción en la IR.
type Value interface {
	ValueKind() ValueKind
	Type() Type
	String() string
}

// TempValue representa un registro temporal o resultado SSA-friendly (%0, %1, %x).
type TempValue struct {
	ID        int
	ValueType Type
	Hint      string // Nombre original opcional para diagnóstico o legibilidad
}

func (v *TempValue) ValueKind() ValueKind { return ValueKindTemp }
func (v *TempValue) Type() Type           { return v.ValueType }
func (v *TempValue) String() string {
	if v.Hint != "" {
		return fmt.Sprintf("%%%d_%s", v.ID, v.Hint)
	}
	return fmt.Sprintf("%%%d", v.ID)
}

// ParamValue representa un parámetro formal recibido por una función.
type ParamValue struct {
	Index     int
	Name      string
	ValueType Type
}

func (v *ParamValue) ValueKind() ValueKind { return ValueKindParam }
func (v *ParamValue) Type() Type           { return v.ValueType }
func (v *ParamValue) String() string {
	if v.Name != "" {
		return fmt.Sprintf("%%arg_%s", v.Name)
	}
	return fmt.Sprintf("%%arg%d", v.Index)
}

// GlobalValue representa un símbolo global (@foo, @g_var).
type GlobalValue struct {
	Name      string
	ValueType Type
}

func (v *GlobalValue) ValueKind() ValueKind { return ValueKindGlobal }
func (v *GlobalValue) Type() Type           { return v.ValueType }
func (v *GlobalValue) String() string {
	return "@" + v.Name
}

// ConstInt representa un literal entero constante.
type ConstInt struct {
	Val       int64
	ValueType Type
}

func (c *ConstInt) ValueKind() ValueKind { return ValueKindConst }
func (c *ConstInt) Type() Type           { return c.ValueType }
func (c *ConstInt) String() string {
	return strconv.FormatInt(c.Val, 10)
}

// ConstFloat representa un literal de punto flotante constante.
type ConstFloat struct {
	Val       float64
	ValueType Type
}

func (c *ConstFloat) ValueKind() ValueKind { return ValueKindConst }
func (c *ConstFloat) Type() Type           { return c.ValueType }
func (c *ConstFloat) String() string {
	return strconv.FormatFloat(c.Val, 'g', -1, 64)
}

// ConstBool representa un literal booleano constante.
type ConstBool struct {
	Val bool
}

func (c *ConstBool) ValueKind() ValueKind { return ValueKindConst }
func (c *ConstBool) Type() Type           { return TypeBool }
func (c *ConstBool) String() string {
	if c.Val {
		return "true"
	}
	return "false"
}

// ConstString representa una constante literal de texto.
type ConstString struct {
	Val string
}

func (c *ConstString) ValueKind() ValueKind { return ValueKindConst }
func (c *ConstString) Type() Type           { return TypeString }
func (c *ConstString) String() string {
	return strconv.Quote(c.Val)
}

// ConstNull representa un valor nulo o puntero nulo.
type ConstNull struct {
	ValueType Type
}

func (c *ConstNull) ValueKind() ValueKind { return ValueKindConst }
func (c *ConstNull) Type() Type           { return c.ValueType }
func (c *ConstNull) String() string {
	return "null"
}

// Constructores de conveniencia
func NewConstInt(val int64, t Type) *ConstInt {
	if t.Kind == TypeKindUnknown || t.Kind == "" {
		t = TypeI64
	}
	return &ConstInt{Val: val, ValueType: t}
}

func NewConstFloat(val float64, t Type) *ConstFloat {
	if t.Kind == TypeKindUnknown || t.Kind == "" {
		t = TypeF64
	}
	return &ConstFloat{Val: val, ValueType: t}
}

func NewConstBool(val bool) *ConstBool {
	return &ConstBool{Val: val}
}

func NewConstString(val string) *ConstString {
	return &ConstString{Val: val}
}

func NewConstNull(t Type) *ConstNull {
	if t.Kind == TypeKindUnknown || t.Kind == "" {
		t = PtrType(TypeVoid)
	}
	return &ConstNull{ValueType: t}
}
