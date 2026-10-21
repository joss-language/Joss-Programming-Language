package core

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
)

func (r *Runtime) executeMathMethod(instance *Instance, method string, args []interface{}) interface{} {
	toFloat := func(val interface{}) (float64, bool) {
		if i, ok := val.(int64); ok {
			return float64(i), true
		}
		if i, ok := val.(int); ok {
			return float64(i), true
		}
		if f, ok := val.(float64); ok {
			return f, true
		}
		if s, ok := val.(string); ok {
			var f float64
			if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
				return f, true
			}
		}
		return 0, false
	}

	switch method {
	case "random":
		// random(min, max)
		if len(args) != 2 {
			fmt.Println("Error: Math.random requiere 2 argumentos (min, max)")
			return nil
		}
		min, ok1 := args[0].(int64)
		max, ok2 := args[1].(int64)
		if !ok1 || !ok2 {
			fmt.Println("Error: Argumentos de Math.random deben ser enteros")
			return nil
		}
		if max < min {
			panic(&JossError{Type: "ValueError", Message: "Math.random requiere que min sea menor o igual que max"})
		}
		return randomInt64Inclusive(min, max)

	case "floor":
		if len(args) != 1 {
			return nil
		}
		if f, ok := toFloat(args[0]); ok {
			return math.Floor(f)
		}
		return args[0]

	case "ceil":
		if len(args) != 1 {
			return nil
		}
		if f, ok := toFloat(args[0]); ok {
			return math.Ceil(f)
		}
		return args[0]

	case "abs":
		if len(args) != 1 {
			return nil
		}
		if f, ok := toFloat(args[0]); ok {
			return math.Abs(f)
		}
		return nil
	}
	return nil
}

func (r *Runtime) executeSessionMethod(instance *Instance, method string, args []interface{}) interface{} {
	// We need access to the session map.
	// In `auth.go`, we inject `$__session` into `r.Variables`.
	// We can access it from there.

	sessionVal, ok := r.Variables["$__session"]
	if !ok {
		// fmt.Println("Error: Sesión no disponible en este contexto")
		return nil
	}

	var sessionMap map[string]interface{}

	if sessMap, ok := sessionVal.(map[string]interface{}); ok {
		sessionMap = sessMap
	} else if sessInst, ok := sessionVal.(*Instance); ok {
		sessionMap = sessInst.Fields
	} else {
		return nil
	}

	switch method {
	case "get":
		if len(args) != 1 {
			return nil
		}
		key, ok := args[0].(string)
		if !ok {
			return nil
		}
		return sessionMap[key]

	case "put":
		if len(args) != 2 {
			return nil
		}
		key, ok := args[0].(string)
		if !ok {
			return nil
		}
		sessionMap[key] = args[1]

	case "has":
		if len(args) != 1 {
			return false
		}
		key, ok := args[0].(string)
		if !ok {
			return false
		}
		_, exists := sessionMap[key]
		return exists

	case "forget":
		if len(args) != 1 {
			return nil
		}
		key, ok := args[0].(string)
		if !ok {
			return nil
		}
		delete(sessionMap, key)

	case "all":
		return sessionMap
	}

	return nil
}

func (r *Runtime) executeStrMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "length":
		if len(args) != 1 {
			return int64(0)
		}
		if s, ok := args[0].(string); ok {
			return int64(len(s))
		}
		return int64(0)

	case "random":
		length := 16
		if len(args) > 0 {
			if l, ok := args[0].(int64); ok {
				length = int(l)
			}
			if l, ok := args[0].(int); ok {
				length = l
			}
		}
		if length < 0 {
			panic(&JossError{Type: "ValueError", Message: "Str.random requiere una longitud no negativa"})
		}
		const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		b := make([]byte, length)
		for i := range b {
			b[i] = charset[rand.Intn(len(charset))]
		}
		return string(b)

	case "startsWith":
		if len(args) != 2 {
			return false
		}
		s, ok1 := args[0].(string)
		prefix, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return false
		}
		return strings.HasPrefix(s, prefix)

	case "endsWith":
		if len(args) != 2 {
			return false
		}
		s, ok1 := args[0].(string)
		suffix, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return false
		}
		return strings.HasSuffix(s, suffix)

	case "substring", "substr":
		if len(args) < 2 {
			return nil
		}
		s, ok := args[0].(string)
		start, ok2 := args[1].(int64)
		if !ok || !ok2 {
			return nil
		}
		runes := []rune(s)
		if start < 0 {
			start = 0
		}
		if start >= int64(len(runes)) {
			return ""
		}
		end := int64(len(runes))
		if len(args) >= 3 {
			if l, ok := args[2].(int64); ok {
				if l < 0 {
					panic(&JossError{Type: "ValueError", Message: "Str.substring requiere una longitud no negativa"})
				}
				if l >= int64(len(runes))-start {
					end = int64(len(runes))
				} else {
					end = start + l
				}
			}
		}
		return string(runes[start:end])
	case "lower":
		if len(args) != 1 {
			return ""
		}
		if s, ok := args[0].(string); ok {
			return strings.ToLower(s)
		}
		return ""
	case "indexOf":
		if len(args) != 2 {
			return int64(-1)
		}
		s, ok1 := args[0].(string)
		substr, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return int64(-1)
		}
		byteIdx := strings.Index(s, substr)
		if byteIdx == -1 {
			return int64(-1)
		}
		return int64(len([]rune(s[:byteIdx])))
	case "contains":
		if len(args) != 2 {
			return false
		}
		s, ok1 := args[0].(string)
		substr, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return false
		}
		return strings.Contains(s, substr)
	case "trim":
		if len(args) != 1 {
			return ""
		}
		if s, ok := args[0].(string); ok {
			return strings.TrimSpace(s)
		}
		return ""
	case "replace":
		if len(args) != 3 {
			return ""
		}
		arg0 := fmt.Sprint(args[0])
		arg1 := fmt.Sprint(args[1])
		arg2 := fmt.Sprint(args[2])

		// Str::replace(subject, search, replace) vs Str::replace(search, replace, subject):
		// Si arg2 es el texto largo/sujeto y arg0 es la cadena a buscar:
		if strings.Contains(arg2, arg0) || (!strings.Contains(arg0, arg1) && strings.Contains(arg2, arg1)) {
			return strings.ReplaceAll(arg2, arg0, arg1)
		}
		// Orden canónico: arg0 es subject, arg1 es search, arg2 es replace
		if strings.Contains(arg0, arg1) {
			return strings.ReplaceAll(arg0, arg1, arg2)
		}
		// Fallback
		return strings.ReplaceAll(arg2, arg0, arg1)
	}

	return nil
}

func randomInt64Inclusive(min, max int64) int64 {
	span := uint64(max) - uint64(min) + 1
	if span == 0 {
		return int64(rand.Uint64())
	}
	// Reject the short prefix that would bias a modulo reduction.
	threshold := -span % span
	for {
		candidate := rand.Uint64()
		if candidate >= threshold {
			return int64(uint64(min) + candidate%span)
		}
	}
}
