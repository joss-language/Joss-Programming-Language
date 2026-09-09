package core

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

func (r *Runtime) callBuiltinArray(name string, args []interface{}) (interface{}, bool) {
	switch name {
	case "isset":
		if len(args) == 0 || args[0] == nil {
			return false, true
		}
		if str, ok := args[0].(string); ok && str == "" {
			return false, true
		}
		return true, true

	case "empty":
		resEmpty := false
		if len(args) == 0 || args[0] == nil {
			resEmpty = true
		} else if b, ok := args[0].(bool); ok {
			resEmpty = !b
		} else if str, ok := args[0].(string); ok {
			resEmpty = (str == "" || str == "0")
		} else if num, ok := args[0].(int); ok {
			resEmpty = (num == 0)
		} else if num, ok := args[0].(int64); ok {
			resEmpty = (num == 0)
		} else if num, ok := args[0].(float64); ok {
			resEmpty = (num == 0)
		} else if list, ok := args[0].([]interface{}); ok {
			resEmpty = (len(list) == 0)
		} else if m, ok := args[0].(map[string]interface{}); ok {
			resEmpty = (len(m) == 0)
		} else {
			val := reflect.ValueOf(args[0])
			if val.Kind() == reflect.Slice || val.Kind() == reflect.Array || val.Kind() == reflect.Map {
				resEmpty = (val.Len() == 0)
			}
		}
		return resEmpty, true

	case "is_string":
		if len(args) == 1 {
			_, ok := args[0].(string)
			return ok, true
		}
		return false, true

	case "is_numeric":
		if len(args) == 1 {
			if args[0] == nil {
				return false, true
			}
			switch v := args[0].(type) {
			case int, int32, int64, float64, float32, decimal.Decimal:
				return true, true
			case string:
				vStr := strings.TrimSpace(v)
				if _, err := strconv.ParseFloat(vStr, 64); err == nil {
					return true, true
				}
				clean := strings.TrimRight(vStr, "mMdD")
				if _, err := decimal.NewFromString(clean); err == nil {
					return true, true
				}
				return false, true
			}
		}
		return false, true

	case "is_int", "is_integer":
		if len(args) == 1 {
			switch args[0].(type) {
			case int, int32, int64:
				return true, true
			}
		}
		return false, true

	case "is_float", "is_double":
		if len(args) == 1 {
			switch args[0].(type) {
			case float64, float32:
				return true, true
			}
		}
		return false, true

	case "is_decimal":
		if len(args) == 1 {
			switch args[0].(type) {
			case decimal.Decimal:
				return true, true
			}
		}
		return false, true

	case "decimal":
		if len(args) == 0 || args[0] == nil {
			return decimal.Zero, true
		}
		switch v := args[0].(type) {
		case decimal.Decimal:
			return v, true
		case int:
			return decimal.NewFromInt(int64(v)), true
		case int32:
			return decimal.NewFromInt(int64(v)), true
		case int64:
			return decimal.NewFromInt(v), true
		case float64:
			return decimal.NewFromFloat(v), true
		case float32:
			return decimal.NewFromFloat(float64(v)), true
		case bool:
			if v {
				return decimal.NewFromInt(1), true
			}
			return decimal.Zero, true
		case string:
			clean := strings.TrimRight(strings.TrimSpace(v), "mMdD")
			if d, err := decimal.NewFromString(clean); err == nil {
				return d, true
			}
		}
		return decimal.Zero, true

	case "intval":
		if len(args) == 0 || args[0] == nil {
			return int64(0), true
		}
		switch v := args[0].(type) {
		case int:
			return int64(v), true
		case int32:
			return int64(v), true
		case int64:
			return v, true
		case float64:
			return int64(v), true
		case float32:
			return int64(v), true
		case decimal.Decimal:
			return v.IntPart(), true
		case bool:
			if v {
				return int64(1), true
			}
			return int64(0), true
		case string:
			vStr := strings.TrimSpace(v)
			if n, err := strconv.ParseInt(vStr, 10, 64); err == nil {
				return n, true
			}
			if f, err := strconv.ParseFloat(vStr, 64); err == nil {
				return int64(f), true
			}
		}
		return int64(0), true

	case "floatval", "doubleval":
		if len(args) == 0 || args[0] == nil {
			return float64(0.0), true
		}
		switch v := args[0].(type) {
		case float64:
			return v, true
		case float32:
			return float64(v), true
		case int:
			return float64(v), true
		case int32:
			return float64(v), true
		case int64:
			return float64(v), true
		case decimal.Decimal:
			f, _ := v.Float64()
			return f, true
		case bool:
			if v {
				return float64(1.0), true
			}
			return float64(0.0), true
		case string:
			vStr := strings.TrimSpace(v)
			if f, err := strconv.ParseFloat(vStr, 64); err == nil {
				return f, true
			}
		}
		return float64(0.0), true

	case "strval":
		if len(args) == 0 || args[0] == nil {
			return "", true
		}
		return fmt.Sprintf("%v", args[0]), true

	case "boolval":
		if len(args) == 0 || args[0] == nil {
			return false, true
		}
		return isTruthy(args[0]), true

	case "is_array":
		if len(args) == 1 {
			if _, ok := args[0].([]interface{}); ok {
				return true, true
			}
			if _, ok := args[0].(map[string]interface{}); ok {
				return true, true
			}
			val := reflect.ValueOf(args[0])
			return val.Kind() == reflect.Slice || val.Kind() == reflect.Array || val.Kind() == reflect.Map, true
		}
		return false, true

	case "is_null":
		if len(args) == 1 {
			return args[0] == nil, true
		}
		return true, true

	case "len", "count":
		if len(args) == 1 {
			if args[0] == nil {
				return int64(0), true
			}
			if list, ok := args[0].([]interface{}); ok {
				return int64(len(list)), true
			}
			if listMap, ok := args[0].([]map[string]interface{}); ok {
				return int64(len(listMap)), true
			}
			if m, ok := args[0].(map[string]interface{}); ok {
				return int64(len(m)), true
			}
			if str, ok := args[0].(string); ok {
				return int64(len(str)), true
			}
			val := reflect.ValueOf(args[0])
			if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
				return int64(val.Len()), true
			}
		}
		return int64(0), true

	case "keys", "array_keys":
		if len(args) == 1 {
			if m, ok := args[0].(map[string]interface{}); ok {
				keys := []interface{}{}
				for k := range m {
					keys = append(keys, k)
				}
				return keys, true
			}
		}
		return []interface{}{}, true

	case "values", "array_values":
		if len(args) == 1 {
			if m, ok := args[0].(map[string]interface{}); ok {
				vals := []interface{}{}
				for _, v := range m {
					vals = append(vals, v)
				}
				return vals, true
			}
		}
		return []interface{}{}, true

	case "explode":
		if len(args) == 2 {
			sep, ok1 := args[0].(string)
			str, ok2 := args[1].(string)
			if ok1 && ok2 {
				parts := strings.Split(str, sep)
				result := []interface{}{}
				for _, p := range parts {
					result = append(result, p)
				}
				return result, true
			}
		}
		return nil, true

	case "end":
		if len(args) == 1 {
			if list, ok := args[0].([]interface{}); ok {
				if len(list) > 0 {
					return list[len(list)-1], true
				}
				return nil, true
			}
		}
		return nil, true

	case "append":
		if len(args) == 2 {
			if list, ok := args[0].([]interface{}); ok {
				newList := append(list, args[1])
				return newList, true
			}
		}
		return nil, true

	case "merge":
		if len(args) == 2 {
			l1, ok1 := args[0].([]interface{})
			l2, ok2 := args[1].([]interface{})
			if ok1 && ok2 {
				newList := make([]interface{}, len(l1)+len(l2))
				copy(newList, l1)
				copy(newList[len(l1):], l2)
				return newList, true
			}
		}
		return nil, true

	case "in_array":
		if len(args) >= 2 {
			target := args[0]
			if list, ok := args[1].([]interface{}); ok {
				for _, item := range list {
					if reflect.DeepEqual(item, target) || fmt.Sprintf("%v", item) == fmt.Sprintf("%v", target) {
						return true, true
					}
				}
				return false, true
			}
			val := reflect.ValueOf(args[1])
			if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
				for i := 0; i < val.Len(); i++ {
					if reflect.DeepEqual(val.Index(i).Interface(), target) {
						return true, true
					}
				}
			}
		}
		return false, true

	case "array_key_exists":
		if len(args) >= 2 {
			k := fmt.Sprintf("%v", args[0])
			if m, ok := args[1].(map[string]interface{}); ok {
				_, exists := m[k]
				return exists, true
			}
		}
		return false, true

	case "array_merge":
		if len(args) >= 2 {
			if l1, ok1 := args[0].([]interface{}); ok1 {
				res := append([]interface{}{}, l1...)
				for _, next := range args[1:] {
					if lNext, okN := next.([]interface{}); okN {
						res = append(res, lNext...)
					}
				}
				return res, true
			}
			if m1, ok1 := args[0].(map[string]interface{}); ok1 {
				res := make(map[string]interface{}, len(m1))
				for k, v := range m1 {
					res[k] = v
				}
				for _, next := range args[1:] {
					if mNext, okN := next.(map[string]interface{}); okN {
						for k, v := range mNext {
							res[k] = v
						}
					}
				}
				return res, true
			}
		}
		return []interface{}{}, true

	case "array_push":
		if len(args) >= 2 {
			if list, ok := args[0].([]interface{}); ok {
				return append(list, args[1:]...), true
			}
		}
		return nil, true

	case "array_pop":
		if len(args) >= 1 {
			if list, ok := args[0].([]interface{}); ok && len(list) > 0 {
				return list[len(list)-1], true
			}
		}
		return nil, true

	case "array_shift":
		if len(args) >= 1 {
			if list, ok := args[0].([]interface{}); ok && len(list) > 0 {
				return list[0], true
			}
		}
		return nil, true

	case "array_slice":
		if len(args) >= 2 {
			if list, ok := args[0].([]interface{}); ok {
				offset := 0
				switch v := args[1].(type) {
				case int:
					offset = v
				case int64:
					offset = int(v)
				}
				l := len(list)
				if offset < 0 {
					offset = l + offset
					if offset < 0 {
						offset = 0
					}
				}
				if offset > l {
					return []interface{}{}, true
				}
				if len(args) >= 3 {
					length := 0
					switch v := args[2].(type) {
					case int:
						length = v
					case int64:
						length = int(v)
					}
					if length < 0 {
						end := l + length
						if end <= offset {
							return []interface{}{}, true
						}
						return list[offset:end], true
					}
					end := offset + length
					if end > l {
						end = l
					}
					return list[offset:end], true
				}
				return list[offset:], true
			}
		}
		return []interface{}{}, true

	case "array_unique":
		if len(args) >= 1 {
			if list, ok := args[0].([]interface{}); ok {
				seen := make(map[string]bool)
				res := []interface{}{}
				for _, item := range list {
					key := fmt.Sprintf("%v", item)
					if !seen[key] {
						seen[key] = true
						res = append(res, item)
					}
				}
				return res, true
			}
		}
		return []interface{}{}, true

	case "array_reverse":
		if len(args) >= 1 {
			if list, ok := args[0].([]interface{}); ok {
				res := make([]interface{}, len(list))
				for i, j := 0, len(list)-1; i < len(list); i, j = i+1, j-1 {
					res[i] = list[j]
				}
				return res, true
			}
		}
		return []interface{}{}, true

	case "array_column":
		if len(args) >= 2 {
			col := fmt.Sprintf("%v", args[1])
			if list, ok := args[0].([]interface{}); ok {
				res := []interface{}{}
				for _, item := range list {
					if m, isMap := item.(map[string]interface{}); isMap {
						if val, has := m[col]; has {
							res = append(res, val)
						}
					}
				}
				return res, true
			}
		}
		return []interface{}{}, true

	case "map", "array_map":
		if len(args) < 2 {
			return []interface{}{}, true
		}
		coll := args[0]
		callback := args[1]
		if !isSliceValue(coll) && isSliceValue(args[1]) {
			coll = args[1]
			callback = args[0]
		}
		list := toSlice(coll)
		res := make([]interface{}, len(list))
		for i, item := range list {
			res[i] = r.CallCallable(callback, []interface{}{item})
		}
		return res, true

	case "filter", "array_filter":
		if len(args) < 1 {
			return []interface{}{}, true
		}
		coll := args[0]
		var callback interface{}
		if len(args) >= 2 {
			callback = args[1]
			if !isSliceValue(coll) && isSliceValue(args[1]) {
				coll = args[1]
				callback = args[0]
			}
		}
		list := toSlice(coll)
		res := []interface{}{}
		for _, item := range list {
			keep := false
			if callback == nil {
				keep = isTruthy(item)
			} else {
				callRes := r.CallCallable(callback, []interface{}{item})
				keep = isTruthy(callRes)
			}
			if keep {
				res = append(res, item)
			}
		}
		return res, true

	case "reduce", "array_reduce":
		if len(args) < 2 {
			return nil, true
		}
		coll := args[0]
		callback := args[1]
		var initial interface{}
		if len(args) >= 3 {
			initial = args[2]
		}
		if !isSliceValue(coll) && isSliceValue(args[1]) {
			coll = args[1]
			callback = args[0]
		}
		list := toSlice(coll)
		acc := initial
		startIndex := 0
		if acc == nil && len(list) > 0 {
			acc = list[0]
			startIndex = 1
		}
		for i := startIndex; i < len(list); i++ {
			acc = r.CallCallable(callback, []interface{}{acc, list[i]})
		}
		return acc, true

	case "find":
		if len(args) < 2 {
			return nil, true
		}
		coll := args[0]
		callback := args[1]
		if !isSliceValue(coll) && isSliceValue(args[1]) {
			coll = args[1]
			callback = args[0]
		}
		list := toSlice(coll)
		for _, item := range list {
			callRes := r.CallCallable(callback, []interface{}{item})
			if isTruthy(callRes) {
				return item, true
			}
		}
		return nil, true

	case "any":
		if len(args) < 1 {
			return false, true
		}
		coll := args[0]
		var callback interface{}
		if len(args) >= 2 {
			callback = args[1]
			if !isSliceValue(coll) && isSliceValue(args[1]) {
				coll = args[1]
				callback = args[0]
			}
		}
		list := toSlice(coll)
		for _, item := range list {
			if callback == nil {
				if isTruthy(item) {
					return true, true
				}
			} else {
				callRes := r.CallCallable(callback, []interface{}{item})
				if isTruthy(callRes) {
					return true, true
				}
			}
		}
		return false, true

	case "all":
		if len(args) < 1 {
			return true, true
		}
		coll := args[0]
		var callback interface{}
		if len(args) >= 2 {
			callback = args[1]
			if !isSliceValue(coll) && isSliceValue(args[1]) {
				coll = args[1]
				callback = args[0]
			}
		}
		list := toSlice(coll)
		for _, item := range list {
			if callback == nil {
				if !isTruthy(item) {
					return false, true
				}
			} else {
				callRes := r.CallCallable(callback, []interface{}{item})
				if !isTruthy(callRes) {
					return false, true
				}
			}
		}
		return true, true

	case "sum":
		if len(args) < 1 {
			return int64(0), true
		}
		list := toSlice(args[0])
		var intSum int64
		var floatSum float64
		isFloat := false
		for _, item := range list {
			switch v := item.(type) {
			case int:
				if isFloat {
					floatSum += float64(v)
				} else {
					intSum += int64(v)
				}
			case int64:
				if isFloat {
					floatSum += float64(v)
				} else {
					intSum += v
				}
			case float64:
				if !isFloat {
					isFloat = true
					floatSum = float64(intSum) + v
				} else {
					floatSum += v
				}
			}
		}
		if isFloat {
			return floatSum, true
		}
		return intSum, true
	}

	return nil, false
}

func isSliceValue(v interface{}) bool {
	if v == nil {
		return false
	}
	switch v.(type) {
	case []interface{}, []map[string]interface{}, []string, []int64, []int:
		return true
	}
	val := reflect.ValueOf(v)
	return val.Kind() == reflect.Slice || val.Kind() == reflect.Array
}

func toSlice(v interface{}) []interface{} {
	if v == nil {
		return []interface{}{}
	}
	if list, ok := v.([]interface{}); ok {
		return list
	}
	if listMap, ok := v.([]map[string]interface{}); ok {
		res := make([]interface{}, len(listMap))
		for i, m := range listMap {
			res[i] = m
		}
		return res
	}
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		res := make([]interface{}, val.Len())
		for i := 0; i < val.Len(); i++ {
			res[i] = val.Index(i).Interface()
		}
		return res
	}
	return []interface{}{}
}
