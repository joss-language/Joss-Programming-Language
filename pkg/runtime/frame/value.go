// Package frame provides compact callable-local storage independent from the
// AST evaluator and framework layers.
package frame

import "github.com/jossecurity/joss/pkg/typesystem"

type ValueKind uint8

const (
	Unset ValueKind = iota
	Null
	Bool
	Int
	Float
	String
	Dynamic
)

// Value keeps primitive locals unboxed while retaining a dynamic escape hatch
// for collections, objects, functions, references and mixed values.
type Value struct {
	Kind    ValueKind
	Integer int64
	Float   float64
	Boolean bool
	String  string
	Dynamic interface{}
}

func FromInterface(value interface{}) Value {
	switch typed := value.(type) {
	case nil:
		return Value{Kind: Null}
	case int64:
		return Value{Kind: Int, Integer: typed}
	case int:
		return Value{Kind: Int, Integer: int64(typed)}
	case float64:
		return Value{Kind: Float, Float: typed}
	case float32:
		return Value{Kind: Float, Float: float64(typed)}
	case bool:
		return Value{Kind: Bool, Boolean: typed}
	case string:
		return Value{Kind: String, String: typed}
	default:
		return Value{Kind: Dynamic, Dynamic: value}
	}
}

func (value Value) Interface() interface{} {
	switch value.Kind {
	case Null, Unset:
		return nil
	case Bool:
		return value.Boolean
	case Int:
		return value.Integer
	case Float:
		return value.Float
	case String:
		return value.String
	default:
		return value.Dynamic
	}
}

type Slot struct {
	Name        string
	Value       Value
	Type        typesystem.Type
	TypeName    string
	Constant    bool
	Inferred    bool
	Initialized bool
	ByReference bool
}

func (slot *Slot) Set(value interface{}) {
	slot.Value = FromInterface(value)
	slot.Initialized = true
}

func (slot *Slot) SetInt(value int64) {
	slot.Value.Kind = Int
	slot.Value.Integer = value
	slot.Value.Dynamic = nil
	slot.Initialized = true
}

func (slot *Slot) SetBool(value bool) {
	slot.Value.Kind = Bool
	slot.Value.Boolean = value
	slot.Value.Dynamic = nil
	slot.Initialized = true
}

func (slot *Slot) Clear() {
	*slot = Slot{}
}
