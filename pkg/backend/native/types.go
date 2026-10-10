package native

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/ir"
)

// MapLLVMType convierte un ir.Type al identificador de tipo textual estándar de LLVM IR.
func MapLLVMType(t ir.Type) string {
	switch t.Kind {
	case ir.TypeKindVoid:
		return "void"
	case ir.TypeKindBool:
		return "i1"
	case ir.TypeKindI8, ir.TypeKindU8:
		return "i8"
	case ir.TypeKindI16, ir.TypeKindU16:
		return "i16"
	case ir.TypeKindI32, ir.TypeKindU32:
		return "i32"
	case ir.TypeKindI64, ir.TypeKindU64:
		return "i64"
	case ir.TypeKindF32:
		return "float"
	case ir.TypeKindF64:
		return "double"
	case ir.TypeKindString:
		return "i8*"
	case ir.TypeKindStruct, ir.TypeKindArray:
		return "i8*"
	case ir.TypeKindPtr:
		if t.Elem == nil {
			return "i8*"
		}
		if t.Elem.Kind == ir.TypeKindStruct || t.Elem.Kind == ir.TypeKindArray {
			return "i8*"
		}
		return fmt.Sprintf("%s*", MapLLVMType(*t.Elem))
	default:
		return "i64"
	}
}

// MapLLVMZeroValue retorna la constante cero de LLVM para inicializaciones.
func MapLLVMZeroValue(t ir.Type) string {
	switch t.Kind {
	case ir.TypeKindVoid:
		return ""
	case ir.TypeKindBool:
		return "false"
	case ir.TypeKindF32, ir.TypeKindF64:
		return "0.0"
	case ir.TypeKindPtr, ir.TypeKindString:
		return "null"
	default:
		return "0"
	}
}
