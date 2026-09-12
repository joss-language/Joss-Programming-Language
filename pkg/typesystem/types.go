// Package typesystem is the canonical source of truth for Joss type names,
// inference and assignment compatibility.
package typesystem

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Kind string

// ArithmeticFault identifies a safety failure in canonical integer
// operations. Analyzer constant evaluation and runtime execution use the same
// implementation so overflow semantics cannot drift.
type ArithmeticFault string

const (
	ArithmeticOK             ArithmeticFault = ""
	ArithmeticOverflow       ArithmeticFault = "overflow"
	ArithmeticDivisionByZero ArithmeticFault = "division_by_zero"
)

const (
	Unknown Kind = "unknown"
	Mixed   Kind = "mixed"
	Null    Kind = "null"
	Int     Kind = "int"
	Float   Kind = "float"
	Decimal Kind = "decimal"
	String  Kind = "string"
	Bool    Kind = "bool"
	Array   Kind = "array"
	Map     Kind = "map"
	Object  Kind = "object"
	Channel Kind = "channel"
	Class   Kind = "class"
	Union   Kind = "union"
)

// Type represents a semantic type. Name is populated for class types.
type Type struct {
	Kind    Kind
	Name    string
	Element *Type
	Key     *Type
}

func (t Type) String() string {
	if (t.Kind == Class || t.Kind == Union) && t.Name != "" {
		return t.Name
	}
	if t.Kind == Array {
		if t.Element != nil {
			return fmt.Sprintf("array<%s>", t.Element.String())
		}
		return string(Array)
	}
	if t.Kind == Map {
		if t.Key != nil && t.Element != nil {
			return fmt.Sprintf("map<%s, %s>", t.Key.String(), t.Element.String())
		}
		return string(Map)
	}
	if t.Kind == "" {
		return string(Unknown)
	}
	return string(t.Kind)
}

func (t Type) IsKnown() bool {
	return t.Kind != "" && t.Kind != Unknown && t.Kind != Null && t.Kind != Mixed
}
func (t Type) IsDynamic() bool { return t.Kind == Mixed }

// IsNumeric reports whether every alternative is one of Joss's canonical
// numeric types. Keeping this rule in typesystem prevents analyzers and tools
// from maintaining their own primitive-type lists.
func (t Type) IsNumeric() bool {
	if t.Kind == Union {
		members := t.Members()
		if len(members) == 0 {
			return false
		}
		for _, member := range members {
			if !member.IsNumeric() {
				return false
			}
		}
		return true
	}
	return t.Kind == Int || t.Kind == Float || t.Kind == Decimal
}

// Members returns the normalized alternatives of a union. Non-union types
// return themselves as a single member.
func (t Type) Members() []Type {
	if t.Kind != Union {
		return []Type{t}
	}
	parts := splitUnion(t.Name)
	result := make([]Type, 0, len(parts))
	for _, part := range parts {
		result = append(result, Parse(part))
	}
	return result
}

// Without returns a type with the given Kind removed from union alternatives.
func (t Type) Without(remove Kind) Type {
	if t.Kind != Union {
		if t.Kind == remove {
			return Type{Kind: Unknown}
		}
		return t
	}
	members := t.Members()
	remaining := make([]string, 0, len(members))
	for _, m := range members {
		if m.Kind != remove {
			remaining = append(remaining, m.String())
		}
	}
	if len(remaining) == 0 {
		return Type{Kind: Unknown}
	}
	if len(remaining) == 1 {
		return Parse(remaining[0])
	}
	return Type{Kind: Union, Name: strings.Join(remaining, "|")}
}

// Only returns the member matching keep Kind, or Unknown if not found.
func (t Type) Only(keep Kind) Type {
	if t.Kind != Union {
		if t.Kind == keep {
			return t
		}
		return Type{Kind: Unknown}
	}
	members := t.Members()
	for _, m := range members {
		if m.Kind == keep {
			return m
		}
	}
	return Type{Kind: Unknown}
}

// Parse returns the canonical meaning of a source-level type name.
func Parse(name string) Type {
	name = strings.TrimSpace(name)
	if strings.HasSuffix(name, "?") {
		name = strings.TrimSpace(strings.TrimSuffix(name, "?")) + "|null"
	}
	unionParts := splitUnion(name)
	if len(unionParts) > 1 {
		seen := make(map[string]bool)
		members := make([]string, 0)
		for _, part := range unionParts {
			member := Parse(part)
			for _, flattened := range member.Members() {
				canonical := flattened.String()
				if canonical == "" || seen[canonical] {
					continue
				}
				seen[canonical] = true
				members = append(members, canonical)
			}
		}
		if len(members) == 1 {
			return Parse(members[0])
		}
		return Type{Kind: Union, Name: strings.Join(members, "|")}
	}

	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "array<") && strings.HasSuffix(name, ">") {
		inner := strings.TrimSpace(name[6 : len(name)-1])
		elem := Parse(inner)
		return Type{Kind: Array, Element: &elem}
	}
	if strings.HasPrefix(lower, "map<") && strings.HasSuffix(name, ">") {
		inner := strings.TrimSpace(name[4 : len(name)-1])
		parts := splitGenericArgs(inner)
		if len(parts) == 2 {
			k := Parse(parts[0])
			v := Parse(parts[1])
			return Type{Kind: Map, Key: &k, Element: &v}
		}
		return Type{Kind: Map}
	}

	switch lower {
	case "", "unknown", "var":
		return Type{Kind: Unknown}
	case "mixed":
		return Type{Kind: Mixed}
	case "null", "nil":
		return Type{Kind: Null}
	case "int":
		return Type{Kind: Int}
	case "float":
		return Type{Kind: Float}
	case "decimal":
		return Type{Kind: Decimal}
	case "string":
		return Type{Kind: String}
	case "bool":
		return Type{Kind: Bool}
	case "array":
		return Type{Kind: Array}
	case "map":
		return Type{Kind: Map}
	case "object":
		return Type{Kind: Object}
	case "channel":
		return Type{Kind: Channel}
	default:
		return Type{Kind: Class, Name: name}
	}
}

func splitUnion(s string) []string {
	parts := []string{}
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			depth--
		case '|':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, strings.TrimSpace(s[start:]))
	}
	return parts
}

func splitGenericArgs(s string) []string {
	parts := []string{}
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, strings.TrimSpace(s[start:]))
	}
	return parts
}

// Assignable reports whether a value of source type can be assigned to a
// destination. Unknown is deliberately non-accusatory: lack of information is
// not evidence of invalid user code.
func Assignable(destination, source Type) bool {
	if destination.Kind == Mixed || source.Kind == Mixed || destination.Kind == Unknown || source.Kind == Unknown {
		return true
	}
	if source.Kind == Union {
		for _, sourceMember := range source.Members() {
			if !Assignable(destination, sourceMember) {
				return false
			}
		}
		return true
	}
	if destination.Kind == Union {
		for _, destinationMember := range destination.Members() {
			if Assignable(destinationMember, source) {
				return true
			}
		}
		return false
	}
	if destination.Kind == source.Kind {
		if destination.Kind == Class {
			return destination.Name == source.Name
		}
		if destination.Kind == Array {
			if destination.Element == nil || source.Element == nil {
				return true
			}
			return Assignable(*destination.Element, *source.Element)
		}
		if destination.Kind == Map {
			if destination.Key == nil || destination.Element == nil || source.Key == nil || source.Element == nil {
				return true
			}
			return Assignable(*destination.Key, *source.Key) && Assignable(*destination.Element, *source.Element)
		}
		return true
	}
	// Integer values are losslessly accepted by float variables, matching the
	// runtime's existing numeric compatibility rule.
	if destination.Kind == Float && source.Kind == Int {
		return true
	}
	if destination.Kind == Decimal && (source.Kind == Int || source.Kind == Float) {
		return true
	}
	if destination.Kind == Object && source.Kind == Class {
		return true
	}
	return false
}

// MergeInference keeps the first concrete inferred type. Null/unknown
// initializers postpone inference until a concrete value is assigned.
func MergeInference(current, candidate Type) Type {
	if current.Kind == Mixed {
		return current
	}
	if !current.IsKnown() && candidate.IsKnown() {
		return candidate
	}
	if current.Kind == "" {
		return Type{Kind: Unknown}
	}
	return current
}

// CoerceString applies the language's explicit typed-input conversion policy.
// It is shared by the runtime and semantic analyzer so a literal accepted by
// one phase cannot be rejected by the other.
func CoerceString(destination Type, value string) (interface{}, bool) {
	if destination.Kind == Union {
		for _, member := range destination.Members() {
			if coerced, ok := CoerceString(member, value); ok {
				return coerced, true
			}
		}
		return value, false
	}
	value = strings.TrimSpace(value)
	switch destination.Kind {
	case Int:
		if integer, err := strconv.ParseInt(value, 10, 64); err == nil {
			return integer, true
		}
		if number, err := strconv.ParseFloat(value, 64); err == nil &&
			math.Trunc(number) == number && number >= float64(math.MinInt64) && number < float64(math.MaxInt64) {
			return int64(number), true
		}
	case Float:
		if number, err := strconv.ParseFloat(value, 64); err == nil {
			return number, true
		}
	case Decimal:
		clean := strings.TrimRight(value, "mMdD")
		if _, err := strconv.ParseFloat(clean, 64); err == nil {
			return clean, true
		}
	case Bool:
		switch strings.ToLower(value) {
		case "true", "1", "yes":
			return true, true
		case "false", "0", "no", "":
			return false, true
		}
	}
	return value, false
}

// CheckedIntBinary evaluates an integer operator without silent overflow.
// Division is included for zero checking even though the Joss `/` operator
// returns float at runtime.
func CheckedIntBinary(operator string, left, right int64) (int64, ArithmeticFault) {
	switch operator {
	case "+":
		if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
			return 0, ArithmeticOverflow
		}
		return left + right, ArithmeticOK
	case "-":
		if (right < 0 && left > math.MaxInt64+right) || (right > 0 && left < math.MinInt64+right) {
			return 0, ArithmeticOverflow
		}
		return left - right, ArithmeticOK
	case "*":
		if left == 0 || right == 0 {
			return 0, ArithmeticOK
		}
		if (left == math.MinInt64 && right == -1) || (right == math.MinInt64 && left == -1) {
			return 0, ArithmeticOverflow
		}
		result := left * right
		if result/right != left {
			return 0, ArithmeticOverflow
		}
		return result, ArithmeticOK
	case "/":
		if right == 0 {
			return 0, ArithmeticDivisionByZero
		}
		if left == math.MinInt64 && right == -1 {
			return 0, ArithmeticOverflow
		}
		return left / right, ArithmeticOK
	case "%":
		if right == 0 {
			return 0, ArithmeticDivisionByZero
		}
		if left == math.MinInt64 && right == -1 {
			return 0, ArithmeticOK
		}
		return left % right, ArithmeticOK
	default:
		return 0, ArithmeticOK
	}
}

// CheckedIntNegate implements the unary-minus overflow rule.
func CheckedIntNegate(value int64) (int64, ArithmeticFault) {
	if value == math.MinInt64 {
		return 0, ArithmeticOverflow
	}
	return -value, ArithmeticOK
}

// SourceTypeNames returns the supported source-level type spellings used by
// editor/tooling catalog generation.
func SourceTypeNames() []string {
	return []string{"array", "bool", "channel", "decimal", "float", "int", "map", "mixed", "object", "string", "var"}
}
