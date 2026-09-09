package core

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"math/rand"
	"os"
	"strings"
	"unicode"

	"github.com/jossecurity/joss/pkg/i18n"
)

func (r *Runtime) callBuiltinString(name string, args []interface{}) (interface{}, bool) {
	switch name {
	case "html_escape":
		if len(args) > 0 {
			if args[0] == nil {
				return "", true
			}
			return html.EscapeString(fmt.Sprintf("%v", args[0])), true
		}
		return "", true

	case "__":
		if len(args) > 0 {
			if key, ok := args[0].(string); ok {
				locale := r.GetLocale()
				return i18n.GlobalManager.Get(locale, key, nil), true
			}
		}
		return "", true

	case "csrf_field":
		tokenVal := ""
		if sessVal, ok := r.Variables["$__session"]; ok {
			if sessInst, ok := sessVal.(*Instance); ok {
				if tok, ok := sessInst.Fields["csrf_token"]; ok {
					tokenVal = fmt.Sprintf("%v", tok)
				}
			}
		}
		return fmt.Sprintf(`<input type="hidden" name="_token" value="%s">`, tokenVal), true

	case "print", "echo":
		for _, arg := range args {
			fmt.Println(arg)
		}
		return nil, true

	case "cout":
		for _, arg := range args {
			fmt.Print(arg)
		}
		return nil, true

	case "cerr":
		for _, arg := range args {
			fmt.Fprint(os.Stderr, arg)
		}
		return nil, true

	case "printf":
		if len(args) > 0 {
			if fmtStr, ok := args[0].(string); ok {
				fmt.Printf(fmtStr, args[1:]...)
			}
		}
		return nil, true

	case "str_contains", "contains":
		if len(args) >= 2 {
			s1 := fmt.Sprintf("%v", args[0])
			s2 := fmt.Sprintf("%v", args[1])
			return strings.Contains(s1, s2), true
		}
		return false, true

	case "str_starts_with", "starts_with":
		if len(args) >= 2 {
			s1 := fmt.Sprintf("%v", args[0])
			s2 := fmt.Sprintf("%v", args[1])
			return strings.HasPrefix(s1, s2), true
		}
		return false, true

	case "str_ends_with", "ends_with":
		if len(args) >= 2 {
			s1 := fmt.Sprintf("%v", args[0])
			s2 := fmt.Sprintf("%v", args[1])
			return strings.HasSuffix(s1, s2), true
		}
		return false, true

	case "str_replace":
		if len(args) >= 3 {
			search := fmt.Sprintf("%v", args[0])
			replace := fmt.Sprintf("%v", args[1])
			subject := fmt.Sprintf("%v", args[2])
			return strings.ReplaceAll(subject, search, replace), true
		}
		return "", true

	case "strtolower", "to_lower":
		if len(args) > 0 {
			return strings.ToLower(fmt.Sprintf("%v", args[0])), true
		}
		return "", true

	case "strtoupper", "to_upper":
		if len(args) > 0 {
			return strings.ToUpper(fmt.Sprintf("%v", args[0])), true
		}
		return "", true

	case "trim":
		if len(args) > 0 {
			str := fmt.Sprintf("%v", args[0])
			if len(args) > 1 {
				cutset := fmt.Sprintf("%v", args[1])
				return strings.Trim(str, cutset), true
			}
			return strings.TrimSpace(str), true
		}
		return "", true

	case "ltrim":
		if len(args) > 0 {
			str := fmt.Sprintf("%v", args[0])
			if len(args) > 1 {
				cutset := fmt.Sprintf("%v", args[1])
				return strings.TrimLeft(str, cutset), true
			}
			return strings.TrimLeft(str, " \t\n\r\v\f"), true
		}
		return "", true

	case "rtrim":
		if len(args) > 0 {
			str := fmt.Sprintf("%v", args[0])
			if len(args) > 1 {
				cutset := fmt.Sprintf("%v", args[1])
				return strings.TrimRight(str, cutset), true
			}
			return strings.TrimRight(str, " \t\n\r\v\f"), true
		}
		return "", true

	case "substr":
		if len(args) >= 2 {
			str := fmt.Sprintf("%v", args[0])
			start := 0
			switch v := args[1].(type) {
			case int:
				start = v
			case int64:
				start = int(v)
			}
			runes := []rune(str)
			rLen := len(runes)
			if start < 0 {
				start = rLen + start
				if start < 0 {
					start = 0
				}
			}
			if start > rLen {
				return "", true
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
					end := rLen + length
					if end <= start {
						return "", true
					}
					return string(runes[start:end]), true
				}
				end := start + length
				if end > rLen {
					end = rLen
				}
				return string(runes[start:end]), true
			}
			return string(runes[start:]), true
		}
		return "", true

	case "strpos":
		if len(args) >= 2 {
			haystack := fmt.Sprintf("%v", args[0])
			needle := fmt.Sprintf("%v", args[1])
			byteIdx := strings.Index(haystack, needle)
			if byteIdx == -1 {
				return false, true
			}
			runeIdx := len([]rune(haystack[:byteIdx]))
			return int64(runeIdx), true
		}
		return false, true

	case "implode", "join":
		if len(args) >= 2 {
			glue := fmt.Sprintf("%v", args[0])
			if list, ok := args[1].([]interface{}); ok {
				strs := make([]string, len(list))
				for i, item := range list {
					strs[i] = fmt.Sprintf("%v", item)
				}
				return strings.Join(strs, glue), true
			}
		}
		return "", true

	case "md5":
		if len(args) > 0 {
			h := md5.Sum([]byte(fmt.Sprintf("%v", args[0])))
			return hex.EncodeToString(h[:]), true
		}
		return "", true

	case "sha1":
		if len(args) > 0 {
			h := sha1.Sum([]byte(fmt.Sprintf("%v", args[0])))
			return hex.EncodeToString(h[:]), true
		}
		return "", true

	case "sha256":
		if len(args) > 0 {
			h := sha256.Sum256([]byte(fmt.Sprintf("%v", args[0])))
			return hex.EncodeToString(h[:]), true
		}
		return "", true

	case "base64_encode":
		if len(args) > 0 {
			return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%v", args[0]))), true
		}
		return "", true

	case "base64_decode":
		if len(args) > 0 {
			b, err := base64.StdEncoding.DecodeString(fmt.Sprintf("%v", args[0]))
			if err != nil {
				return false, true
			}
			return string(b), true
		}
		return false, true

	case "json_encode":
		if len(args) > 0 {
			var b []byte
			var err error
			if len(args) >= 2 && isTruthy(args[1]) {
				b, err = json.MarshalIndent(args[0], "", "  ")
			} else {
				b, err = json.Marshal(args[0])
			}
			if err != nil {
				return "{}", true
			}
			return string(b), true
		}
		return "null", true

	case "json_decode":
		if len(args) > 0 {
			if str, ok := args[0].(string); ok {
				return JsonDecode(str), true
			}
		}
		return nil, true

	case "strlen":
		if len(args) > 0 {
			if args[0] == nil {
				return int64(0), true
			}
			str := fmt.Sprintf("%v", args[0])
			return int64(len([]rune(str))), true
		}
		return int64(0), true

	case "ucfirst":
		if len(args) > 0 {
			str := fmt.Sprintf("%v", args[0])
			if len(str) == 0 {
				return "", true
			}
			runes := []rune(str)
			runes[0] = unicode.ToUpper(runes[0])
			return string(runes), true
		}
		return "", true

	case "lcfirst":
		if len(args) > 0 {
			str := fmt.Sprintf("%v", args[0])
			if len(str) == 0 {
				return "", true
			}
			runes := []rune(str)
			runes[0] = unicode.ToLower(runes[0])
			return string(runes), true
		}
		return "", true

	case "ucwords":
		if len(args) > 0 {
			str := fmt.Sprintf("%v", args[0])
			return strings.Title(str), true
		}
		return "", true

	case "str_pad":
		if len(args) >= 2 {
			input := fmt.Sprintf("%v", args[0])
			padLen := 0
			switch v := args[1].(type) {
			case int:
				padLen = v
			case int64:
				padLen = int(v)
			}
			padStr := " "
			if len(args) >= 3 {
				padStr = fmt.Sprintf("%v", args[2])
			}
			if len(input) >= padLen {
				return input, true
			}
			diff := padLen - len(input)
			pad := strings.Repeat(padStr, (diff/len(padStr))+1)
			return input + pad[:diff], true
		}
		return "", true

	case "str_repeat":
		if len(args) >= 2 {
			str := fmt.Sprintf("%v", args[0])
			count := 0
			switch v := args[1].(type) {
			case int:
				count = v
			case int64:
				count = int(v)
			}
			if count < 0 {
				count = 0
			}
			return strings.Repeat(str, count), true
		}
		return "", true

	case "round":
		if len(args) > 0 {
			f := 0.0
			switch v := args[0].(type) {
			case float64:
				f = v
			case float32:
				f = float64(v)
			case int:
				f = float64(v)
			case int64:
				f = float64(v)
			}
			precision := 0
			if len(args) > 1 {
				switch p := args[1].(type) {
				case int:
					precision = p
				case int64:
					precision = int(p)
				}
			}
			ratio := math.Pow(10, float64(precision))
			return math.Round(f*ratio) / ratio, true
		}
		return float64(0), true

	case "floor":
		if len(args) > 0 {
			f := 0.0
			switch v := args[0].(type) {
			case float64:
				f = v
			case float32:
				f = float64(v)
			case int:
				return int64(v), true
			case int64:
				return v, true
			}
			return int64(math.Floor(f)), true
		}
		return int64(0), true

	case "ceil":
		if len(args) > 0 {
			f := 0.0
			switch v := args[0].(type) {
			case float64:
				f = v
			case float32:
				f = float64(v)
			case int:
				return int64(v), true
			case int64:
				return v, true
			}
			return int64(math.Ceil(f)), true
		}
		return int64(0), true

	case "abs":
		if len(args) > 0 {
			switch v := args[0].(type) {
			case int:
				if v < 0 {
					return int64(-v), true
				}
				return int64(v), true
			case int64:
				if v < 0 {
					return -v, true
				}
				return v, true
			case float64:
				return math.Abs(v), true
			case float32:
				return math.Abs(float64(v)), true
			}
		}
		return int64(0), true

	case "min":
		if len(args) == 0 {
			return nil, true
		}
		if len(args) == 1 {
			if list, ok := args[0].([]interface{}); ok && len(list) > 0 {
				m := list[0]
				for _, item := range list[1:] {
					if compareLessThan(item, m) {
						m = item
					}
				}
				return m, true
			}
			return args[0], true
		}
		m := args[0]
		for _, item := range args[1:] {
			if compareLessThan(item, m) {
				m = item
			}
		}
		return m, true

	case "max":
		if len(args) == 0 {
			return nil, true
		}
		if len(args) == 1 {
			if list, ok := args[0].([]interface{}); ok && len(list) > 0 {
				m := list[0]
				for _, item := range list[1:] {
					if compareGreaterThan(item, m) {
						m = item
					}
				}
				return m, true
			}
			return args[0], true
		}
		m := args[0]
		for _, item := range args[1:] {
			if compareGreaterThan(item, m) {
				m = item
			}
		}
		return m, true

	case "rand":
		minVal := int64(0)
		maxVal := int64(math.MaxInt32)
		if len(args) >= 2 {
			if m1, ok := toInt64Safe(args[0]); ok {
				minVal = m1
			}
			if m2, ok := toInt64Safe(args[1]); ok {
				maxVal = m2
			}
		}
		if maxVal <= minVal {
			return minVal, true
		}
		return minVal + rand.Int63n(maxVal-minVal+1), true
	}

	return nil, false
}
