package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

// resolveArrayMethod resolves fluent methods called on an array primitive.
func (r *Runtime) resolveArrayMethod(arr []interface{}, prop *parser.Identifier) (func([]interface{}) interface{}, bool) {
	method := prop.Value
	if _, _, exists := typesystem.PrimitiveMethod(typesystem.Type{Kind: typesystem.Array}, method); !exists {
		return nil, false
	}
	switch method {
	case "length", "count":
		return func(args []interface{}) interface{} {
			return int64(len(arr))
		}, true

	case "join":
		return func(args []interface{}) interface{} {
			sep := ""
			if len(args) > 0 {
				sep = fmt.Sprint(args[0])
			}
			strParts := make([]string, len(arr))
			for i, item := range arr {
				strParts[i] = fmt.Sprint(item)
			}
			return strings.Join(strParts, sep)
		}, true

	case "contains", "has":
		return func(args []interface{}) interface{} {
			if len(args) == 0 {
				return false
			}
			target := args[0]
			for _, item := range arr {
				if r.looseEquals(item, target) {
					return true
				}
			}
			return false
		}, true

	case "indexOf":
		return func(args []interface{}) interface{} {
			if len(args) == 0 {
				return int64(-1)
			}
			target := args[0]
			for i, item := range arr {
				if r.looseEquals(item, target) {
					return int64(i)
				}
			}
			return int64(-1)
		}, true

	case "slice":
		return func(args []interface{}) interface{} {
			if len(args) == 0 {
				copyArr := make([]interface{}, len(arr))
				copy(copyArr, arr)
				return copyArr
			}
			totalLen := int64(len(arr))
			start := toInt64(args[0])
			if start < 0 {
				start = totalLen + start
				if start < 0 {
					start = 0
				}
			}
			if start >= totalLen {
				return []interface{}{}
			}
			end := totalLen
			if len(args) >= 2 {
				length := toInt64(args[1])
				if length < 0 {
					end = totalLen + length
				} else {
					end = start + length
				}
				if end > totalLen {
					end = totalLen
				}
			}
			if start > end {
				return []interface{}{}
			}
			sub := arr[start:end]
			copyArr := make([]interface{}, len(sub))
			copy(copyArr, sub)
			return copyArr
		}, true

	case "reverse":
		return func(args []interface{}) interface{} {
			n := len(arr)
			res := make([]interface{}, n)
			for i := 0; i < n; i++ {
				res[i] = arr[n-1-i]
			}
			return res
		}, true

	case "first":
		return func(args []interface{}) interface{} {
			if len(arr) == 0 {
				return nil
			}
			return arr[0]
		}, true

	case "last":
		return func(args []interface{}) interface{} {
			if len(arr) == 0 {
				return nil
			}
			return arr[len(arr)-1]
		}, true

	case "map":
		return func(args []interface{}) interface{} {
			if len(args) == 0 {
				return arr
			}
			fn := args[0]
			res := make([]interface{}, len(arr))
			for i, item := range arr {
				res[i] = r.applyFunction(fn, []interface{}{item, int64(i)})
			}
			return res
		}, true

	case "filter":
		return func(args []interface{}) interface{} {
			if len(args) == 0 {
				return arr
			}
			fn := args[0]
			res := make([]interface{}, 0, len(arr))
			for i, item := range arr {
				cond := r.applyFunction(fn, []interface{}{item, int64(i)})
				if isTruthy(cond) {
					res = append(res, item)
				}
			}
			return res
		}, true

	case "reduce":
		return func(args []interface{}) interface{} {
			if len(args) == 0 {
				return nil
			}
			fn := args[0]
			var accumulator interface{}
			startIndex := 0
			if len(args) >= 2 {
				accumulator = args[1]
			} else if len(arr) > 0 {
				accumulator = arr[0]
				startIndex = 1
			} else {
				return nil
			}
			for i := startIndex; i < len(arr); i++ {
				accumulator = r.applyFunction(fn, []interface{}{accumulator, arr[i], int64(i)})
			}
			return accumulator
		}, true

	case "push":
		return func(args []interface{}) interface{} {
			res := make([]interface{}, len(arr)+len(args))
			copy(res, arr)
			for i, arg := range args {
				res[len(arr)+i] = arg
			}
			return res
		}, true
	}

	return nil, false
}

// resolveMapMethod resolves fluent methods called on a map primitive.
func (r *Runtime) resolveMapMethod(m map[string]interface{}, prop *parser.Identifier) (func([]interface{}) interface{}, bool) {
	method := prop.Value
	if _, _, exists := typesystem.PrimitiveMethod(typesystem.Type{Kind: typesystem.Map}, method); !exists {
		return nil, false
	}
	switch method {
	case "keys":
		return func(args []interface{}) interface{} {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			res := make([]interface{}, len(keys))
			for i, k := range keys {
				res[i] = k
			}
			return res
		}, true

	case "values":
		return func(args []interface{}) interface{} {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			res := make([]interface{}, len(keys))
			for i, k := range keys {
				res[i] = m[k]
			}
			return res
		}, true

	case "has", "contains":
		return func(args []interface{}) interface{} {
			if len(args) == 0 {
				return false
			}
			k := fmt.Sprint(args[0])
			_, exists := m[k]
			return exists
		}, true

	case "length", "count":
		return func(args []interface{}) interface{} {
			return int64(len(m))
		}, true

	case "get":
		return func(args []interface{}) interface{} {
			if len(args) == 0 {
				return nil
			}
			k := fmt.Sprint(args[0])
			if val, exists := m[k]; exists {
				return val
			}
			if len(args) > 1 {
				return args[1]
			}
			return nil
		}, true

	case "set":
		return func(args []interface{}) interface{} {
			if len(args) < 2 {
				return m
			}
			k := fmt.Sprint(args[0])
			m[k] = args[1]
			return m
		}, true

	case "remove":
		return func(args []interface{}) interface{} {
			if len(args) == 0 {
				return m
			}
			k := fmt.Sprint(args[0])
			delete(m, k)
			return m
		}, true

	case "merge":
		return func(args []interface{}) interface{} {
			res := make(map[string]interface{})
			for k, v := range m {
				res[k] = v
			}
			for _, arg := range args {
				if otherMap, ok := arg.(map[string]interface{}); ok {
					for k, v := range otherMap {
						res[k] = v
					}
				}
			}
			return res
		}, true
	}

	return nil, false
}

func (r *Runtime) looseEquals(a, b interface{}) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}
