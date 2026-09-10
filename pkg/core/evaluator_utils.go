package core

import (
	"strconv"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
	"github.com/shopspring/decimal"
)

func (r *Runtime) checkType(val interface{}, typeName string) bool {
	return r.checkParsedType(val, typesystem.Parse(typeName))
}

func (r *Runtime) checkParsedType(val interface{}, destination typesystem.Type) bool {
	if destination.Kind == typesystem.Array && destination.Element != nil {
		list, ok := val.([]interface{})
		if !ok {
			return false
		}
		for _, item := range list {
			if !r.checkParsedType(item, *destination.Element) {
				return false
			}
		}
		return true
	}
	if destination.Kind == typesystem.Map && destination.Element != nil {
		m, ok := val.(map[string]interface{})
		if !ok {
			return false
		}
		for _, v := range m {
			if !r.checkParsedType(v, *destination.Element) {
				return false
			}
		}
		return true
	}
	source := runtimeTypeOf(val)
	if typesystem.Assignable(destination, source) {
		return true
	}
	inst, ok := val.(*Instance)
	if !ok || inst == nil {
		return false
	}
	destinations := destination.Members()
	for class := inst.Class; class != nil; {
		if class.Name != nil {
			for _, candidate := range destinations {
				if candidate.Kind == typesystem.Class {
					if class.Name.Value == candidate.Name {
						return true
					}
					if r.classImplements(class, candidate.Name) {
						return true
					}
				}
			}
		}
		if class.SuperClass == nil {
			break
		}
		class = r.Classes[class.SuperClass.Value]
	}
	return false
}

func (r *Runtime) classImplements(class *parser.ClassStatement, targetInterface string) bool {
	if class == nil {
		return false
	}
	for _, iface := range class.Interfaces {
		if iface != nil && r.interfaceInherits(iface.Value, targetInterface, map[string]bool{}) {
			return true
		}
	}
	return false
}

func (r *Runtime) interfaceInherits(currentIface, targetIface string, visited map[string]bool) bool {
	if currentIface == targetIface {
		return true
	}
	if visited[currentIface] {
		return false
	}
	visited[currentIface] = true
	iface, exists := r.Interfaces[currentIface]
	if !exists || iface == nil {
		return false
	}
	for _, ext := range iface.Extends {
		if ext != nil && r.interfaceInherits(ext.Value, targetIface, visited) {
			return true
		}
	}
	return false
}

func runtimeTypeOf(value interface{}) typesystem.Type {
	switch typed := value.(type) {
	case nil:
		return typesystem.Type{Kind: typesystem.Null}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return typesystem.Type{Kind: typesystem.Int}
	case float32, float64:
		return typesystem.Type{Kind: typesystem.Float}
	case decimal.Decimal:
		return typesystem.Type{Kind: typesystem.Decimal}
	case string:
		return typesystem.Type{Kind: typesystem.String}
	case bool:
		return typesystem.Type{Kind: typesystem.Bool}
	case []interface{}:
		return typesystem.Type{Kind: typesystem.Array}
	case map[string]interface{}:
		return typesystem.Type{Kind: typesystem.Map}
	case *Channel:
		return typesystem.Type{Kind: typesystem.Channel}
	case *Instance:
		if typed != nil && typed.Class != nil && typed.Class.Name != nil {
			return typesystem.Type{Kind: typesystem.Class, Name: typed.Class.Name.Value}
		}
		return typesystem.Type{Kind: typesystem.Object}
	default:
		return typesystem.Type{Kind: typesystem.Unknown}
	}
}

func runtimeTypeName(value interface{}) string {
	valueType := runtimeTypeOf(value)
	if !valueType.IsKnown() {
		return ""
	}
	return valueType.String()
}

func (r *Runtime) checkExistence(exp parser.Expression) bool {
	switch e := exp.(type) {
	case *parser.Identifier:
		if exists, initialized := r.localBindingExists(e); exists {
			return initialized
		}
		if !r.sourceMapVisible(e.Value) {
			return false
		}
		_, ok := r.Variables[e.Value]
		return ok
	case *parser.IndexExpression:
		left := r.safeEvaluate(e.Left)
		if left == nil {
			return false
		}
		if list, ok := left.([]interface{}); ok {
			index := r.safeEvaluate(e.Index)
			if idx, ok := index.(int64); ok {
				return idx >= 0 && idx < int64(len(list))
			}
		}
		if m, ok := left.(map[string]interface{}); ok {
			index := r.safeEvaluate(e.Index)
			var key string
			if k, ok := index.(string); ok {
				key = k
			} else if n, ok := toInt64Safe(index); ok {
				key = strconv.FormatInt(n, 10)
			}
			if key != "" {
				_, exists := m[key]
				return exists
			}
		}
		return false
	case *parser.MemberExpression:
		left := r.safeEvaluate(e.Left)
		if instance, ok := left.(*Instance); ok {
			_, ok := instance.Fields[e.Property.Value]
			return ok
		}
		return false
	}
	return false
}

// safeEvaluate evaluates an expression and returns nil if it panics.
// Used only for existence checks (isset/empty) where undefined is expected.
func (r *Runtime) safeEvaluate(exp parser.Expression) (result interface{}) {
	defer func() {
		if rec := recover(); rec != nil {
			result = nil
		}
	}()
	return r.evaluateExpression(exp)
}

func isFalsy(val interface{}) bool {
	if val == nil {
		return true
	}
	if _, ok := val.(*Instance); ok {
		return false // Instances are always Truthy
	}
	if d, ok := val.(decimal.Decimal); ok {
		return d.IsZero()
	}
	if b, ok := val.(bool); ok {
		return !b
	}
	if s, ok := val.(string); ok {
		return s == "" || s == "0"
	}
	if i, ok := val.(int64); ok {
		return i == 0
	}
	if list, ok := val.([]interface{}); ok {
		return len(list) == 0
	}
	return false
}

func isTruthy(val interface{}) bool {
	return !isFalsy(val)
}

// coerceToTypedValue attempts to cast val to the declared type when val is a string.
// This allows Console::input() (which returns string) to work with int/float declarations.
func (r *Runtime) coerceToTypedValue(val interface{}, typeName string) interface{} {
	return r.coerceToParsedType(val, typesystem.Parse(typeName))
}

func (r *Runtime) coerceToParsedType(val interface{}, destination typesystem.Type) interface{} {
	if val == nil {
		return val
	}
	if destination.Kind == typesystem.Decimal {
		switch v := val.(type) {
		case decimal.Decimal:
			return v
		case int64:
			return decimal.NewFromInt(v)
		case int:
			return decimal.NewFromInt(int64(v))
		case float64:
			return decimal.NewFromFloat(v)
		case float32:
			return decimal.NewFromFloat(float64(v))
		case string:
			clean := strings.TrimRight(strings.TrimSpace(v), "mMdD")
			if d, err := decimal.NewFromString(clean); err == nil {
				return d
			}
		}
	}
	str, isString := val.(string)
	if !isString {
		return val // Already a non-string, no coercion needed
	}
	if destination.Kind == typesystem.Decimal {
		clean := strings.TrimRight(strings.TrimSpace(str), "mMdD")
		if d, err := decimal.NewFromString(clean); err == nil {
			return d
		}
	}
	if coerced, ok := typesystem.CoerceString(destination, str); ok {
		return coerced
	}
	return val // Return original if no coercion possible
}

// getZeroValue returns the zero/default value for a given type name.
// Used when a variable is declared without an initializer (e.g., int $x).
func (r *Runtime) getZeroValue(typeName string) interface{} {
	parsed := typesystem.Parse(typeName)
	if parsed.Kind == typesystem.Union {
		members := parsed.Members()
		for _, member := range members {
			if member.Kind == typesystem.Null {
				return nil
			}
		}
		if len(members) > 0 {
			return r.getZeroValue(members[0].String())
		}
	}
	switch parsed.Kind {
	case typesystem.Int:
		return int64(0)
	case typesystem.Float:
		return float64(0.0)
	case typesystem.Decimal:
		return decimal.Zero
	case typesystem.String:
		return ""
	case typesystem.Bool:
		return false
	case typesystem.Array:
		return []interface{}{}
	case typesystem.Map:
		return map[string]interface{}{}
	default:
		return nil
	}
}
