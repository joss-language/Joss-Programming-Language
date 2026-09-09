package core

import (
	"encoding/json"
)

// executeJSONMethod handles JSON methods (parse, stringify, encode, decode)
func (r *Runtime) executeJSONMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "parse", "decode":
		if len(args) > 0 {
			if str, ok := args[0].(string); ok {
				return JsonDecode(str)
			}
		}
		return nil

	case "stringify", "encode":
		if len(args) > 0 {
			var bytes []byte
			var err error
			if len(args) >= 2 && isTruthy(args[1]) {
				bytes, err = json.MarshalIndent(args[0], "", "  ")
			} else {
				bytes, err = json.Marshal(args[0])
			}
			if err == nil {
				return string(bytes)
			}
		}
		return ""
	}
	return nil
}
