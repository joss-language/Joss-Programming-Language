package core

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// TOON Helpers
func ToonEncode(data interface{}) string {
	// Simple implementation
	// If array of maps:
	// entity[count]{keys}:
	//   val1,val2

	if list, ok := data.([]interface{}); ok {
		if len(list) == 0 {
			return ""
		}
		// Assume homogeneous list of maps
		first := list[0]
		if m, ok := first.(map[string]interface{}); ok {
			keys := []string{}
			for k := range m {
				keys = append(keys, k)
			}

			header := fmt.Sprintf("entity[%d]{%s}:\n", len(list), strings.Join(keys, ","))
			body := ""
			for _, item := range list {
				if row, ok := item.(map[string]interface{}); ok {
					vals := []string{}
					for _, k := range keys {
						vals = append(vals, fmt.Sprintf("%v", row[k]))
					}
					body += "  " + strings.Join(vals, ",") + "\n"
				}
			}
			return header + body
		}
	}
	return fmt.Sprintf("%v", data)
}

func ToonDecode(str string) interface{} {
	// Handle literal \n if parser didn't unescape it
	str = strings.ReplaceAll(str, "\\n", "\n")

	// Very basic parser for "entity[N]{k1,k2}:\n v1,v2..."
	lines := strings.Split(strings.TrimSpace(str), "\n")
	if len(lines) < 2 {
		return nil
	}

	header := lines[0]
	// Parse header: name[count]{keys}:
	// Regex or simple string manipulation
	startBracket := strings.Index(header, "[")
	endBracket := strings.Index(header, "]")
	startBrace := strings.Index(header, "{")
	endBrace := strings.Index(header, "}")

	if startBracket == -1 || endBracket == -1 || startBrace == -1 || endBrace == -1 {
		return nil
	}

	// countStr := header[startBracket+1 : endBracket]
	keysStr := header[startBrace+1 : endBrace]
	keys := strings.Split(keysStr, ",")

	result := []interface{}{}

	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		vals := strings.Split(line, ",")
		if len(vals) != len(keys) {
			continue
		}

		obj := make(map[string]interface{})
		for i, k := range keys {
			obj[k] = vals[i]
		}
		result = append(result, obj)
	}

	return result
}

func ToonVerify(str string) bool {
	// Handle literal \n
	str = strings.ReplaceAll(str, "\\n", "\n")

	// Simple verification: check structure
	lines := strings.Split(strings.TrimSpace(str), "\n")
	if len(lines) < 2 {
		return false
	}
	header := lines[0]
	// Must contain [ ] { } :
	if !strings.Contains(header, "[") || !strings.Contains(header, "]") ||
		!strings.Contains(header, "{") || !strings.Contains(header, "}") ||
		!strings.Contains(header, ":") {
		return false
	}
	return true
}

// NormalizeDecodedJSON recursively normalizes JSON data so that:
// - Whole numbers (float64 without decimals) become int64
// - JSON objects remain map[string]interface{}
// - JSON arrays remain []interface{}
func NormalizeDecodedJSON(val interface{}) interface{} {
	switch v := val.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{}, len(v))
		for k, item := range v {
			m[k] = NormalizeDecodedJSON(item)
		}
		return m
	case []interface{}:
		arr := make([]interface{}, len(v))
		for i, item := range v {
			arr[i] = NormalizeDecodedJSON(item)
		}
		return arr
	case float64:
		if v == math.Trunc(v) && !math.IsNaN(v) && !math.IsInf(v, 0) && v >= math.MinInt64 && v <= math.MaxInt64 {
			return int64(v)
		}
		return v
	default:
		return v
	}
}

// JSON Helpers
func JsonEncode(data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(b)
}

func JsonDecode(str string) interface{} {
	str = strings.TrimSpace(str)
	if strings.HasPrefix(str, "\xef\xbb\xbf") {
		str = str[3:]
	}
	var result interface{}
	err := json.Unmarshal([]byte(str), &result)
	if err != nil {
		return nil
	}
	return NormalizeDecodedJSON(result)
}

func JsonVerify(str string) bool {
	return json.Valid([]byte(str))
}
