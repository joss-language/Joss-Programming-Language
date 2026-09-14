package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// System Implementation
func (r *Runtime) executeSystemMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "change_db", "use_db":
		if len(args) > 0 {
			driverName := fmt.Sprintf("%v", args[0])
			var config map[string]string
			if len(args) > 1 {
				if m, ok := args[1].(map[string]interface{}); ok {
					config = make(map[string]string, len(m))
					for k, v := range m {
						config[k] = fmt.Sprintf("%v", v)
					}
				}
			}
			err := r.ChangeDB(driverName, config)
			if err != nil {
				fmt.Printf("[System Error] Fallo al cambiar base de datos a '%s': %v\n", driverName, err)
				return false
			}
			return true
		}
		return false
	case "env":
		if len(args) > 0 {
			key := args[0].(string)
			if val, ok := r.Env[key]; ok {
				return val
			}
			if len(args) > 1 {
				return args[1] // Default value
			}
			return ""
		}
	case "Run":
		// Security Check
		allow, ok := r.Env["ALLOW_SYSTEM_RUN"]
		if !ok || (allow != "true" && allow != "1") {
			fmt.Println("[System::Security] Error: Ejecución de comandos bloqueada. Configure ALLOW_SYSTEM_RUN=true en su entorno.")
			return ""
		}

		if len(args) > 0 {
			cmdName := args[0].(string)
			cmdArgs := []string{}

			// Auto-correct 'joss' to current executable
			if cmdName == "joss" {
				exe, err := os.Executable()
				if err == nil {
					cmdName = exe
				}
			}

			if len(args) > 1 {
				if list, ok := args[1].([]interface{}); ok {
					for _, arg := range list {
						cmdArgs = append(cmdArgs, fmt.Sprintf("%v", arg))
					}
				}
			}

			cmd := exec.Command(cmdName, cmdArgs...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("[System] Error ejecutando '%s': %v\n", cmdName, err)
				return ""
			}
			return string(output)
		}
	case "load_driver":
		if len(args) > 0 {
			path, ok := args[0].(string)
			if !ok {
				return false
			}
			name := ""
			if len(args) > 1 {
				name, _ = args[1].(string)
			}
			if name == "" {
				name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			}
			driver, err := loadNativeDriver(name, path)
			if err != nil {
				fmt.Printf("[System] No se pudo cargar driver %s: %v\n", path, err)
				return false
			}
			if err := r.installNativeDriver(name, driver); err != nil {
				fmt.Printf("[System] No se pudo registrar driver %s: %v\n", path, err)
				return false
			}
			return true
		}
	case "driver_call":
		if len(args) < 2 {
			return nil
		}
		name, nameOK := args[0].(string)
		driverMethod, methodOK := args[1].(string)
		if !nameOK || !methodOK {
			return nil
		}
		callArgs := interface{}([]interface{}{})
		if len(args) > 2 {
			callArgs = args[2]
		}
		encoded, err := json.Marshal(callArgs)
		if err != nil {
			return nil
		}
		result, err := callLoadedNativeDriver(r.NativeDrivers[name], driverMethod, string(encoded))
		if err != nil {
			fmt.Printf("[System] Driver %s: %v\n", name, err)
			return nil
		}
		var decoded interface{}
		if err := json.Unmarshal([]byte(result), &decoded); err != nil {
			return result
		}
		return normalizePluginJSON(decoded)
	case "log":
		if len(args) > 0 {
			msg := fmt.Sprintf("%v", args[0])
			fmt.Println("[System Log] " + msg)
			return nil
		}
	case "sleep":
		if len(args) > 0 {
			seconds := toInt(args[0])
			time.Sleep(time.Duration(seconds) * time.Second)
			return true
		}
	case "now":
		current := time.Now()
		if len(args) > 0 {
			current = current.AddDate(0, 0, toInt(args[0]))
		}
		return current.Format("2006-01-02 15:04:05")
	}
	return nil
}
