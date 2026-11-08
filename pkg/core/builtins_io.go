package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) == 1 {
			if path, ok := args[0].(string); ok {
				_, err := os.Stat(path)
				return err == nil, true
			}
		}
		return false, true

	case "file_get_contents":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) == 1 {
			if path, ok := args[0].(string); ok {
				content, err := ReadFileBytes(path)
				if err != nil {
					return nil, true
				}
				return string(content), true
			}
		}
		return nil, true

	case "file_put_contents":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
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
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) > 0 {
			path := fmt.Sprintf("%v", args[0])
			err := os.Remove(path)
			return err == nil, true
		}
		return false, true

	case "mkdir":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) > 0 {
			path := fmt.Sprintf("%v", args[0])
			err := os.MkdirAll(path, 0755)
			return err == nil, true
		}
		return false, true

	case "is_dir":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) > 0 {
			path := fmt.Sprintf("%v", args[0])
			fi, err := os.Stat(path)
			return err == nil && fi.IsDir(), true
		}
		return false, true

	case "is_file":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
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

	case "hive_read_box":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
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
			if err := r.RequireCapability("process"); err != nil {
				panic(err)
			}
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

	case "__joss_db_query":
		if len(args) == 0 {
			return []interface{}{}, true
		}
		sqlStr, ok := args[0].(string)
		if !ok {
			return []interface{}{}, true
		}
		var bindings []interface{}
		if len(args) > 1 {
			if bArr, isArr := args[1].([]interface{}); isArr {
				bindings = bArr
			}
		}
		return r.executeRawDBQuery(sqlStr, bindings), true

	case "__joss_db_execute":
		if len(args) == 0 {
			return int64(0), true
		}
		sqlStr, ok := args[0].(string)
		if !ok {
			return int64(0), true
		}
		var bindings []interface{}
		if len(args) > 1 {
			if bArr, isArr := args[1].([]interface{}); isArr {
				bindings = bArr
			}
		}
		return r.executeRawDBExecute(sqlStr, bindings), true

	case "__joss_db_get_table":
		if len(args) == 0 {
			return []interface{}{}, true
		}
		tblName := fmt.Sprintf("%v", args[0])
		if r.Variables == nil {
			r.Variables = make(map[string]interface{})
		}
		tables, ok := r.Variables["__joss_db_tables"].(map[string]interface{})
		if !ok {
			tables = make(map[string]interface{})
			r.Variables["__joss_db_tables"] = tables
		}
		if rows, exists := tables[tblName]; exists {
			return rows, true
		}
		newRows := []interface{}{}
		tables[tblName] = newRows
		return newRows, true

	case "__joss_db_set_table":
		if len(args) < 2 {
			return true, true
		}
		tblName := fmt.Sprintf("%v", args[0])
		rows := args[1]
		if r.Variables == nil {
			r.Variables = make(map[string]interface{})
		}
		tables, ok := r.Variables["__joss_db_tables"].(map[string]interface{})
		if !ok {
			tables = make(map[string]interface{})
			r.Variables["__joss_db_tables"] = tables
		}
		tables[tblName] = rows
		return true, true

	case "__joss_db_is_connected":
		return r.GetDB() != nil, true

	case "__joss_storage_get":
		if len(args) < 3 {
			return nil, true
		}
		userId := fmt.Sprintf("%v", args[1])
		objectPath := fmt.Sprintf("%v", args[2])
			localPath := filepath.Join("storage", "user_storage", userId, objectPath)
		if data, err := ReadFileBytes(localPath); err == nil {
			return string(data), true
		}
		return nil, true

	case "__joss_storage_put":
		if len(args) < 4 {
			return false, true
		}
		userId := fmt.Sprintf("%v", args[1])
		objectPath := fmt.Sprintf("%v", args[2])
		content := fmt.Sprintf("%v", args[3])
			localPath := filepath.Join("storage", "user_storage", userId, objectPath)
		_ = os.MkdirAll(filepath.Dir(localPath), 0755)
		err := os.WriteFile(localPath, []byte(content), 0644)
		return err == nil, true

	case "__joss_storage_delete":
		if len(args) < 3 {
			return false, true
		}
		userId := fmt.Sprintf("%v", args[1])
		objectPath := fmt.Sprintf("%v", args[2])
			localPath := filepath.Join("storage", "user_storage", userId, objectPath)
		_ = os.Remove(localPath)
		return true, true
	}

	return nil, false
}
