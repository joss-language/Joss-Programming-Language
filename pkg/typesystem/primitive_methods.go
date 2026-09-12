package typesystem

import "sort"

// PrimitiveReturn describes only the metadata needed to project a primitive
// method's return type. Runtime behavior remains owned by pkg/core.
type PrimitiveReturn string

const (
	PrimitiveReturnsString      PrimitiveReturn = "string"
	PrimitiveReturnsInt         PrimitiveReturn = "int"
	PrimitiveReturnsBool        PrimitiveReturn = "bool"
	PrimitiveReturnsArray       PrimitiveReturn = "array"
	PrimitiveReturnsStringArray PrimitiveReturn = "array<string>"
	PrimitiveReturnsReceiver    PrimitiveReturn = "receiver"
	PrimitiveReturnsElement     PrimitiveReturn = "element"
	PrimitiveReturnsUnknown     PrimitiveReturn = "unknown"
)

// PrimitiveMethodDefinition is the canonical name and return contract shared
// by static analysis and primitive runtime dispatch.
type PrimitiveMethodDefinition struct {
	Name   string
	Return PrimitiveReturn
}

var primitiveMethods = map[Kind]map[string]PrimitiveMethodDefinition{
	String: primitiveMethodMap(
		PrimitiveMethodDefinition{Name: "trim", Return: PrimitiveReturnsString},
		PrimitiveMethodDefinition{Name: "lower", Return: PrimitiveReturnsString},
		PrimitiveMethodDefinition{Name: "upper", Return: PrimitiveReturnsString},
		PrimitiveMethodDefinition{Name: "replace", Return: PrimitiveReturnsString},
		PrimitiveMethodDefinition{Name: "substring", Return: PrimitiveReturnsString},
		PrimitiveMethodDefinition{Name: "substr", Return: PrimitiveReturnsString},
		PrimitiveMethodDefinition{Name: "repeat", Return: PrimitiveReturnsString},
		PrimitiveMethodDefinition{Name: "length", Return: PrimitiveReturnsInt},
		PrimitiveMethodDefinition{Name: "indexOf", Return: PrimitiveReturnsInt},
		PrimitiveMethodDefinition{Name: "contains", Return: PrimitiveReturnsBool},
		PrimitiveMethodDefinition{Name: "startsWith", Return: PrimitiveReturnsBool},
		PrimitiveMethodDefinition{Name: "endsWith", Return: PrimitiveReturnsBool},
		PrimitiveMethodDefinition{Name: "split", Return: PrimitiveReturnsStringArray},
		PrimitiveMethodDefinition{Name: "lines", Return: PrimitiveReturnsStringArray},
	),
	Array: primitiveMethodMap(
		PrimitiveMethodDefinition{Name: "length", Return: PrimitiveReturnsInt},
		PrimitiveMethodDefinition{Name: "count", Return: PrimitiveReturnsInt},
		PrimitiveMethodDefinition{Name: "indexOf", Return: PrimitiveReturnsInt},
		PrimitiveMethodDefinition{Name: "join", Return: PrimitiveReturnsString},
		PrimitiveMethodDefinition{Name: "contains", Return: PrimitiveReturnsBool},
		PrimitiveMethodDefinition{Name: "has", Return: PrimitiveReturnsBool},
		PrimitiveMethodDefinition{Name: "slice", Return: PrimitiveReturnsReceiver},
		PrimitiveMethodDefinition{Name: "reverse", Return: PrimitiveReturnsReceiver},
		PrimitiveMethodDefinition{Name: "push", Return: PrimitiveReturnsReceiver},
		PrimitiveMethodDefinition{Name: "filter", Return: PrimitiveReturnsReceiver},
		PrimitiveMethodDefinition{Name: "map", Return: PrimitiveReturnsArray},
		PrimitiveMethodDefinition{Name: "first", Return: PrimitiveReturnsElement},
		PrimitiveMethodDefinition{Name: "last", Return: PrimitiveReturnsElement},
		PrimitiveMethodDefinition{Name: "reduce", Return: PrimitiveReturnsUnknown},
	),
	Map: primitiveMethodMap(
		PrimitiveMethodDefinition{Name: "keys", Return: PrimitiveReturnsStringArray},
		PrimitiveMethodDefinition{Name: "values", Return: PrimitiveReturnsArray},
		PrimitiveMethodDefinition{Name: "has", Return: PrimitiveReturnsBool},
		PrimitiveMethodDefinition{Name: "contains", Return: PrimitiveReturnsBool},
		PrimitiveMethodDefinition{Name: "length", Return: PrimitiveReturnsInt},
		PrimitiveMethodDefinition{Name: "count", Return: PrimitiveReturnsInt},
		PrimitiveMethodDefinition{Name: "get", Return: PrimitiveReturnsElement},
		PrimitiveMethodDefinition{Name: "set", Return: PrimitiveReturnsReceiver},
		PrimitiveMethodDefinition{Name: "remove", Return: PrimitiveReturnsReceiver},
		PrimitiveMethodDefinition{Name: "merge", Return: PrimitiveReturnsReceiver},
	),
}

func primitiveMethodMap(definitions ...PrimitiveMethodDefinition) map[string]PrimitiveMethodDefinition {
	result := make(map[string]PrimitiveMethodDefinition, len(definitions))
	for _, definition := range definitions {
		if _, duplicate := result[definition.Name]; duplicate {
			panic("duplicate primitive method definition: " + definition.Name)
		}
		result[definition.Name] = definition
	}
	return result
}

// PrimitiveMethod resolves canonical metadata and projects its receiver-aware
// return type.
func PrimitiveMethod(receiver Type, name string) (PrimitiveMethodDefinition, Type, bool) {
	definition, exists := primitiveMethods[receiver.Kind][name]
	if !exists {
		return PrimitiveMethodDefinition{}, Type{}, false
	}
	return definition, primitiveReturnType(definition.Return, receiver), true
}

// PrimitiveMethodDefinitions returns a stable copy for validation and tooling.
func PrimitiveMethodDefinitions(kind Kind) []PrimitiveMethodDefinition {
	methods := primitiveMethods[kind]
	result := make([]PrimitiveMethodDefinition, 0, len(methods))
	for _, definition := range methods {
		result = append(result, definition)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func primitiveReturnType(result PrimitiveReturn, receiver Type) Type {
	switch result {
	case PrimitiveReturnsString:
		return Type{Kind: String}
	case PrimitiveReturnsInt:
		return Type{Kind: Int}
	case PrimitiveReturnsBool:
		return Type{Kind: Bool}
	case PrimitiveReturnsArray:
		return Type{Kind: Array}
	case PrimitiveReturnsStringArray:
		stringType := Type{Kind: String}
		return Type{Kind: Array, Element: &stringType}
	case PrimitiveReturnsReceiver:
		return receiver
	case PrimitiveReturnsElement:
		if receiver.Element != nil {
			return *receiver.Element
		}
		return Type{Kind: Unknown}
	default:
		return Type{Kind: Unknown}
	}
}
