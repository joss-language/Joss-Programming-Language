package core

import "encoding/json"

func (r *Runtime) callBuiltinSerialization(name string, args []interface{}) (interface{}, bool) {
	switch name {
	case "json_encode":
		if len(args) == 0 {
			return "null", true
		}
		var data []byte
		var err error
		if len(args) >= 2 && isTruthy(args[1]) {
			data, err = json.MarshalIndent(args[0], "", "  ")
		} else {
			data, err = json.Marshal(args[0])
		}
		if err != nil {
			return "{}", true
		}
		return string(data), true
	case "json_decode":
		if len(args) > 0 {
			if source, ok := args[0].(string); ok {
				return JsonDecode(source), true
			}
		}
		return nil, true
	case "json_verify":
		if len(args) == 1 {
			if source, ok := args[0].(string); ok {
				return JsonVerify(source), true
			}
		}
		return false, true
	default:
		return nil, false
	}
}
