package ir

import (
	"fmt"
	"strings"
)

// TypeKind identifica la categoría del tipo en la IR nativa.
type TypeKind string

const (
	TypeKindVoid    TypeKind = "void"
	TypeKindBool    TypeKind = "bool"
	TypeKindI8      TypeKind = "i8"
	TypeKindI16     TypeKind = "i16"
	TypeKindI32     TypeKind = "i32"
	TypeKindI64     TypeKind = "i64"
	TypeKindU8      TypeKind = "u8"
	TypeKindU16     TypeKind = "u16"
	TypeKindU32     TypeKind = "u32"
	TypeKindU64     TypeKind = "u64"
	TypeKindF32     TypeKind = "f32"
	TypeKindF64     TypeKind = "f64"
	TypeKindString  TypeKind = "string"
	TypeKindPtr     TypeKind = "ptr"
	TypeKindFunc    TypeKind = "func"
	TypeKindArray   TypeKind = "array"
	TypeKindStruct  TypeKind = "struct"
	TypeKindMixed   TypeKind = "mixed"
	TypeKindUnknown TypeKind = "unknown"
)

// Type representa un tipo con información de layout o parámetros para IR nativa.
type Type struct {
	Kind   TypeKind
	Name   string // Nombre de struct o tipo nominal
	Elem   *Type  // Tipo apuntado o elemento de array
	Params []Type // Parámetros de función
	Return *Type  // Retorno de función
	Fields []Type // Campos de struct anónimo o tuple
}

func (t Type) String() string {
	switch t.Kind {
	case TypeKindVoid, TypeKindBool, TypeKindI8, TypeKindI16, TypeKindI32, TypeKindI64,
		TypeKindU8, TypeKindU16, TypeKindU32, TypeKindU64, TypeKindF32, TypeKindF64,
		TypeKindString, TypeKindMixed, TypeKindUnknown:
		return string(t.Kind)
	case TypeKindPtr:
		if t.Elem == nil {
			return "ptr<void>"
		}
		return fmt.Sprintf("ptr<%s>", t.Elem.String())
	case TypeKindArray:
		elemStr := "void"
		if t.Elem != nil {
			elemStr = t.Elem.String()
		}
		return fmt.Sprintf("array<%s>", elemStr)
	case TypeKindStruct:
		if t.Name != "" {
			return t.Name
		}
		fieldStrs := make([]string, len(t.Fields))
		for i, f := range t.Fields {
			fieldStrs[i] = f.String()
		}
		return fmt.Sprintf("struct{%s}", strings.Join(fieldStrs, ", "))
	case TypeKindFunc:
		paramStrs := make([]string, len(t.Params))
		for i, p := range t.Params {
			paramStrs[i] = p.String()
		}
		retStr := "void"
		if t.Return != nil {
			retStr = t.Return.String()
		}
		return fmt.Sprintf("func(%s) -> %s", strings.Join(paramStrs, ", "), retStr)
	default:
		return string(t.Kind)
	}
}

func (t Type) Equal(other Type) bool {
	if t.Kind != other.Kind {
		return false
	}
	switch t.Kind {
	case TypeKindPtr, TypeKindArray:
		if t.Elem == nil && other.Elem == nil {
			return true
		}
		if t.Elem == nil || other.Elem == nil {
			return false
		}
		return t.Elem.Equal(*other.Elem)
	case TypeKindStruct:
		if t.Name != "" || other.Name != "" {
			return t.Name == other.Name
		}
		if len(t.Fields) != len(other.Fields) {
			return false
		}
		for i := range t.Fields {
			if !t.Fields[i].Equal(other.Fields[i]) {
				return false
			}
		}
		return true
	case TypeKindFunc:
		if len(t.Params) != len(other.Params) {
			return false
		}
		for i := range t.Params {
			if !t.Params[i].Equal(other.Params[i]) {
				return false
			}
		}
		if (t.Return == nil) != (other.Return == nil) {
			return false
		}
		if t.Return != nil && !t.Return.Equal(*other.Return) {
			return false
		}
		return true
	default:
		return true
	}
}

func (t Type) IsNumeric() bool {
	switch t.Kind {
	case TypeKindI8, TypeKindI16, TypeKindI32, TypeKindI64,
		TypeKindU8, TypeKindU16, TypeKindU32, TypeKindU64,
		TypeKindF32, TypeKindF64:
		return true
	default:
		return false
	}
}

func (t Type) IsInteger() bool {
	switch t.Kind {
	case TypeKindI8, TypeKindI16, TypeKindI32, TypeKindI64,
		TypeKindU8, TypeKindU16, TypeKindU32, TypeKindU64:
		return true
	default:
		return false
	}
}

func (t Type) IsFloat() bool {
	return t.Kind == TypeKindF32 || t.Kind == TypeKindF64
}

// Constructores de tipos canónicos en IR
var (
	TypeVoid    = Type{Kind: TypeKindVoid}
	TypeBool    = Type{Kind: TypeKindBool}
	TypeI8      = Type{Kind: TypeKindI8}
	TypeI16     = Type{Kind: TypeKindI16}
	TypeI32     = Type{Kind: TypeKindI32}
	TypeI64     = Type{Kind: TypeKindI64}
	TypeU8      = Type{Kind: TypeKindU8}
	TypeU16     = Type{Kind: TypeKindU16}
	TypeU32     = Type{Kind: TypeKindU32}
	TypeU64     = Type{Kind: TypeKindU64}
	TypeF32     = Type{Kind: TypeKindF32}
	TypeF64     = Type{Kind: TypeKindF64}
	TypeString  = Type{Kind: TypeKindString}
	TypeMixed   = Type{Kind: TypeKindMixed}
	TypeUnknown = Type{Kind: TypeKindUnknown}
)

func PtrType(elem Type) Type {
	return Type{Kind: TypeKindPtr, Elem: &elem}
}

func ArrayType(elem Type) Type {
	return Type{Kind: TypeKindArray, Elem: &elem}
}

func FuncType(params []Type, ret Type) Type {
	return Type{Kind: TypeKindFunc, Params: params, Return: &ret}
}

func StructType(name string, fields []Type) Type {
	return Type{Kind: TypeKindStruct, Name: name, Fields: fields}
}
