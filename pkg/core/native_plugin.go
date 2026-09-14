package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var pluginMaterializeMu sync.Mutex

func (r *Runtime) registerPluginABIPayload(name, version, root string, targets map[string]string, files map[string][]byte) error {
	if len(targets) == 0 {
		return nil
	}
	if existing := r.NativePlugins[name]; existing != nil && existing.Driver != "" {
		return fmt.Errorf("plugin %s %s: no puede declarar driver duplicado", name, version)
	}
	target := runtime.GOOS + "-" + runtime.GOARCH
	library, ok := targets[target]
	if !ok {
		return fmt.Errorf("plugin %s %s: no incluye biblioteca ABI para %s; disponibles: %v", name, version, target, sortedStringKeys(targets))
	}
	clean, err := safePluginRelativePath(library)
	if err != nil {
		return err
	}
	definition := &NativePluginDefinition{Name: name, Version: version, Root: root, Driver: name, ArchiveFiles: files}
	r.NativePlugins[name] = definition
	resolved, err := materializePluginPath(definition, clean)
	if err != nil {
		return err
	}
	driver, err := loadNativeDriver(name, resolved)
	if err != nil {
		return fmt.Errorf("plugin %s %s: no se pudo cargar ABI: %w", name, version, err)
	}
	if err := r.installNativeDriver(name, driver); err != nil {
		return fmt.Errorf("plugin %s %s: no se pudo registrar ABI: %w", name, version, err)
	}
	return nil
}

func (r *Runtime) executePluginMethod(_ *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "platform":
		return runtime.GOOS + "-" + runtime.GOARCH
	case "path":
		if len(args) != 2 {
			panic("Plugin::path requiere (plugin, ruta)")
		}
		name, okName := args[0].(string)
		relative, okPath := args[1].(string)
		if !okName || !okPath {
			panic("Plugin::path requiere dos strings")
		}
		definition := r.NativePlugins[name]
		if definition != nil {
			resolved, err := materializePluginPath(definition, relative)
			if err == nil {
				return resolved
			}
		}
		// Fallback para paquetes .jp cargados como AST/JPBC
		cwd, _ := os.Getwd()
		return filepath.Clean(filepath.Join(cwd, "plugins", name, relative))
	case "call":
		if len(args) < 2 || len(args) > 3 {
			panic("Plugin::call requiere (plugin, metodo, args_opcionales)")
		}
		name, okName := args[0].(string)
		rpcMethod, okMethod := args[1].(string)
		if !okName || !okMethod || strings.TrimSpace(rpcMethod) == "" {
			panic("Plugin::call requiere plugin y metodo string")
		}
		callArgs := []interface{}{}
		if len(args) == 3 {
			switch value := args[2].(type) {
			case []interface{}:
				callArgs = value
			default:
				callArgs = []interface{}{value}
			}
		}
		result, err := r.callNativePlugin(name, rpcMethod, callArgs)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}
		}
		return result
	case "stream":
		if len(args) < 2 {
			panic("Plugin::stream requiere (plugin, metodo, args_opcionales, callback_opcional)")
		}
		name, okName := args[0].(string)
		rpcMethod, okMethod := args[1].(string)
		if !okName || !okMethod {
			panic("Plugin::stream requiere plugin y metodo string")
		}
		callArgs := []interface{}{}
		if len(args) >= 3 {
			if list, ok := args[2].([]interface{}); ok {
				callArgs = list
			} else {
				callArgs = []interface{}{args[2]}
			}
		}
		var callback interface{}
		if len(args) >= 4 {
			callback = args[3]
		}
		res, err := r.callNativePluginStream(name, rpcMethod, callArgs, callback)
		if err != nil {
			return nil
		}
		return res
	}
	panic(fmt.Sprintf("Plugin::%s no existe", method))
}

func (r *Runtime) callNativePluginStream(name, method string, args []interface{}, callback interface{}) (interface{}, error) {
	if r.PluginRegistry != nil && r.PluginRegistry.Get(name) != nil {
		fullArgs := append(args, callback)
		return r.PluginRegistry.CallFunction(name, method, fullArgs)
	}
	return nil, nil
}

func (r *Runtime) callNativePlugin(name, method string, args []interface{}) (interface{}, error) {
	if r.PluginRegistry != nil && r.PluginRegistry.Get(name) != nil {
		res, err := r.PluginRegistry.CallFunction(name, method, args)
		if err == nil {
			return res, nil
		}
	}

	definition := r.NativePlugins[name]
	if definition != nil && definition.Driver != "" {
		encoded, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		response, err := callLoadedNativeDriver(r.NativeDrivers[definition.Driver], method, string(encoded))
		if err != nil {
			return nil, err
		}
		var decoded interface{}
		if err := json.Unmarshal([]byte(response), &decoded); err != nil {
			return response, nil
		}
		return normalizePluginJSON(decoded), nil
	}

	return nil, fmt.Errorf("plugin %s no encontrado", name)
}

func materializePluginPath(definition *NativePluginDefinition, relative string) (string, error) {
	clean, err := safePluginRelativePath(relative)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(clean) {
		if _, err := os.Stat(clean); err == nil {
			return clean, nil
		}
	}
	if strings.HasPrefix(clean, "http://") || strings.HasPrefix(clean, "https://") {
		return clean, nil
	}
	resolved := filepath.Join(definition.Root, filepath.FromSlash(clean))
	if _, err := os.Stat(resolved); err == nil {
		return resolved, nil
	}
	pluginMaterializeMu.Lock()
	defer pluginMaterializeMu.Unlock()
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(home, ".joss", "native", definition.Name, definition.Version, runtime.GOOS+"-"+runtime.GOARCH)
	for archivePath, content := range definition.ArchiveFiles {
		asset, pathErr := safePluginRelativePath(archivePath)
		if pathErr != nil || strings.HasPrefix(asset, "META-INF/") || strings.HasPrefix(asset, "bytecode/") || asset == "joss.yaml" {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(asset))
		if err := writeVerifiedPluginAsset(target, content, asset == definition.Executable); err != nil {
			return "", err
		}
	}
	resolved = filepath.Join(root, filepath.FromSlash(clean))
	if _, err := os.Stat(resolved); err != nil {
		if strings.HasPrefix(clean, "http://") || strings.HasPrefix(clean, "https://") {
			return clean, nil
		}
		return "", fmt.Errorf("plugin %s: asset %q no existe", definition.Name, clean)
	}
	return resolved, nil
}

func writeVerifiedPluginAsset(target string, content []byte, executable bool) error {
	expected := sha256.Sum256(content)
	if existing, err := os.ReadFile(target); err == nil {
		actual := sha256.Sum256(existing)
		if actual == expected {
			if executable && runtime.GOOS != "windows" {
				_ = os.Chmod(target, 0755)
			}
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	temp := target + ".tmp-" + hex.EncodeToString(expected[:6])
	mode := os.FileMode(0600)
	if executable {
		mode = 0700
	}
	if err := os.WriteFile(temp, content, mode); err != nil {
		return err
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}

func safePluginRelativePath(value string) (string, error) {
	if strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
		return value, nil
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if clean == "." || clean == ".." || filepath.IsAbs(value) || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("ruta de plugin insegura: %q", value)
	}
	return clean, nil
}

func normalizePluginJSON(value interface{}) interface{} {
	switch typed := value.(type) {
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer
		}
		if floating, err := typed.Float64(); err == nil {
			return floating
		}
	case []interface{}:
		for i := range typed {
			typed[i] = normalizePluginJSON(typed[i])
		}
		return typed
	case map[string]interface{}:
		for key := range typed {
			typed[key] = normalizePluginJSON(typed[key])
		}
		return typed
	}
	return value
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
