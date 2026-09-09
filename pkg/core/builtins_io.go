package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (r *Runtime) callBuiltinIO(name string, args []interface{}) (interface{}, bool) {
	switch name {
	case "env", "config":
		if len(args) > 0 {
			if key, ok := args[0].(string); ok {
				if val, exists := r.Env[key]; exists && val != "" {
					return val, true
				}
				if len(args) > 1 {
					return args[1], true
				}
				if val, exists := r.Env[key]; exists {
					return val, true
				}
				return "", true
			}
		}
		return "", true

	case "view":
		return r.executeViewMethod(nil, "render", args), true

	case "json":
		return r.executeResponseMethod(nil, "json", args), true

	case "back":
		return r.executeResponseMethod(nil, "back", nil), true

	case "response":
		return r.executeResponseMethod(nil, "raw", args), true

	case "request":
		if len(args) == 0 {
			return r.executeRequestMethod(nil, "all", nil), true
		}
		return r.executeRequestMethod(nil, "input", args), true

	case "session":
		if len(args) == 0 {
			if sessVal, ok := r.Variables["$__session"]; ok {
				return sessVal, true
			}
			return nil, true
		}
		return r.executeSessionMethod(nil, "get", args), true

	case "redirect":
		return r.executeResponseMethod(nil, "redirect", args), true

	case "file_exists":
		if len(args) == 1 {
			if path, ok := args[0].(string); ok {
				_, err := os.Stat(path)
				return err == nil, true
			}
		}
		return false, true

	case "file_get_contents":
		if len(args) == 1 {
			if path, ok := args[0].(string); ok {
				content, err := os.ReadFile(path)
				if err != nil {
					return nil, true
				}
				return string(content), true
			}
		}
		return nil, true

	case "file_put_contents":
		if len(args) == 2 {
			path, ok1 := args[0].(string)
			content, ok2 := args[1].(string)
			if ok1 && ok2 {
				err := os.WriteFile(path, []byte(content), 0644)
				if err != nil {
					return false, true
				}
				return true, true
			}
		}
		return false, true

	case "unlink", "file_delete":
		if len(args) > 0 {
			path := fmt.Sprintf("%v", args[0])
			err := os.Remove(path)
			return err == nil, true
		}
		return false, true

	case "mkdir":
		if len(args) > 0 {
			path := fmt.Sprintf("%v", args[0])
			err := os.MkdirAll(path, 0755)
			return err == nil, true
		}
		return false, true

	case "is_dir":
		if len(args) > 0 {
			path := fmt.Sprintf("%v", args[0])
			fi, err := os.Stat(path)
			return err == nil && fi.IsDir(), true
		}
		return false, true

	case "is_file":
		if len(args) > 0 {
			path := fmt.Sprintf("%v", args[0])
			fi, err := os.Stat(path)
			return err == nil && !fi.IsDir(), true
		}
		return false, true

	case "toon_encode":
		if len(args) == 1 {
			return ToonEncode(args[0]), true
		}
		return "", true

	case "toon_decode":
		if len(args) == 1 {
			if str, ok := args[0].(string); ok {
				return ToonDecode(str), true
			}
		}
		return nil, true

	case "toon_verify":
		if len(args) == 1 {
			if str, ok := args[0].(string); ok {
				return ToonVerify(str), true
			}
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
		if len(args) == 1 {
			if str, ok := args[0].(string); ok {
				return JsonDecode(str), true
			}
		}
		return nil, true

	case "json_verify":
		if len(args) == 1 {
			if str, ok := args[0].(string); ok {
				return JsonVerify(str), true
			}
		}
		return false, true

	case "hive_read_box":
		if len(args) < 1 {
			return nil, true
		}
		filePath := fmt.Sprintf("%v", args[0])
		entries, err := ReadHiveBox(filePath)
		if err != nil {
			fmt.Printf("[hive_read_box Error] %v\n", err)
			return nil, true
		}
		result := make([]interface{}, len(entries))
		for i, entry := range entries {
			result[i] = entry
		}
		return result, true

	case "run":
		if len(args) > 0 {
			scriptPath, ok := args[0].(string)
			if !ok {
				return "", true
			}

			allow, ok := r.Env["ALLOW_SYSTEM_RUN"]
			if !ok || (allow != "true" && allow != "1") {
				fmt.Println("[Security] Error: Ejecución de scripts bloqueada. Configure ALLOW_SYSTEM_RUN=true en su entorno.")
				return "", true
			}

			runner := ""
			if strings.HasSuffix(scriptPath, ".py") {
				runner = "python"
			} else if strings.HasSuffix(scriptPath, ".php") {
				runner = "php"
			} else {
				fmt.Println("[Error] Tipo de archivo no soportado para 'run'. Use .py o .php")
				return "", true
			}

			cmdArgs := []string{scriptPath}
			if len(args) > 1 {
				for _, arg := range args[1:] {
					cmdArgs = append(cmdArgs, fmt.Sprintf("%v", arg))
				}
			}

			cmd := exec.Command(runner, cmdArgs...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("[Run] Error ejecutando script: %v\n", err)
			}
			return string(output), true
		}
		return "", true
	}

	return nil, false
}
