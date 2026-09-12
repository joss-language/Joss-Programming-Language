package pluginruntime

import (
	"fmt"
	"sync"
)

// PluginRegistry almacena los plugins cargados y resuelve invocaciones.
type PluginRegistry struct {
	state *pluginRegistryState
	host  HostContext
}

// pluginRegistryState owns the loaded plugin payloads. Runtime forks share
// this synchronized catalog, while each registry facade has its own host.
type pluginRegistryState struct {
	plugins map[string]*Plugin
	mu      sync.RWMutex
}

// NewPluginRegistry inicializa un nuevo registro de plugins.
func NewPluginRegistry(host HostContext) *PluginRegistry {
	return &PluginRegistry{
		state: &pluginRegistryState{plugins: make(map[string]*Plugin)},
		host:  host,
	}
}

// WithHost returns a runtime-bound view over the same synchronized catalog.
// Registration remains visible through every view, but execution uses the
// host of the runtime that initiated the call.
func (r *PluginRegistry) WithHost(host HostContext) *PluginRegistry {
	if r == nil {
		return nil
	}
	return &PluginRegistry{state: r.state, host: host}
}

// Register registra un plugin validado en el sistema.
func (r *PluginRegistry) Register(plugin *Plugin) error {
	if plugin == nil {
		return fmt.Errorf("pluginruntime: intento de registrar plugin nil")
	}

	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	if _, exists := r.state.plugins[plugin.Name]; exists {
		return fmt.Errorf("%w: %s v%s", ErrPluginAlreadyLoaded, plugin.Name, plugin.Version)
	}

	r.state.plugins[plugin.Name] = plugin
	return nil
}

// Get obtiene un plugin por su nombre.
func (r *PluginRegistry) Get(pluginName string) *Plugin {
	r.state.mu.RLock()
	defer r.state.mu.RUnlock()
	return r.state.plugins[pluginName]
}

// List retorna todos los plugins registrados.
func (r *PluginRegistry) List() []*Plugin {
	r.state.mu.RLock()
	defer r.state.mu.RUnlock()

	list := make([]*Plugin, 0, len(r.state.plugins))
	for _, p := range r.state.plugins {
		list = append(list, p)
	}
	return list
}

// CallFunction ejecuta una funcion exportada por un plugin de forma segura y aislada.
func (r *PluginRegistry) CallFunction(pluginName, fnName string, args []interface{}) (res interface{}, err error) {
	plugin := r.Get(pluginName)
	if plugin == nil {
		return nil, fmt.Errorf("%w: %s", ErrPluginNotFound, pluginName)
	}

	return SafeCall(pluginName, fnName, func() (interface{}, error) {
		switch plugin.Format {
		case FormatJossAST:
			if aware, ok := r.host.(PluginAwareHost); ok {
				return aware.CallPluginAST(pluginName, fnName, args)
			}
			if engine, ok := r.host.(ASTEngine); ok {
				return engine.CallFunction(fnName, args)
			}
			if plugin.jossExecutor == nil {
				return nil, fmt.Errorf("plugin %s no tiene ejecutor AST disponible", pluginName)
			}
			return plugin.jossExecutor.CallFunction(fnName, args)

		case FormatJPBC:
			guard := NewPermissionGuard(plugin.Metadata.Permissions)
			vm := NewJPBCVM(plugin.jpbcModule, guard, r.host)
			return vm.Execute(fnName, args)

		default:
			return nil, fmt.Errorf("formato no soportado para ejecucion: %s", plugin.Format)
		}
	})
}

// Instantiate crea una instancia de una clase exportada por un plugin.
func (r *PluginRegistry) Instantiate(pluginName, className string, args []interface{}) (res interface{}, err error) {
	plugin := r.Get(pluginName)
	if plugin == nil {
		return nil, fmt.Errorf("%w: %s", ErrPluginNotFound, pluginName)
	}

	return SafeCall(pluginName, className+".init", func() (interface{}, error) {
		switch plugin.Format {
		case FormatJossAST:
			if aware, ok := r.host.(PluginAwareHost); ok {
				return aware.InstantiatePluginAST(pluginName, className, args)
			}
			if engine, ok := r.host.(ASTEngine); ok {
				return engine.Instantiate(className, args)
			}
			if plugin.jossExecutor == nil {
				return nil, fmt.Errorf("plugin %s no tiene ejecutor AST disponible", pluginName)
			}
			return plugin.jossExecutor.Instantiate(className, args)

		case FormatJPBC:
			// En JPBC, las estructuras se instancian como mapas tipados
			instanceMap := make(map[string]interface{})
			instanceMap["__type__"] = className
			instanceMap["__plugin__"] = pluginName

			// Look for and execute constructor if it exists
			initName := fmt.Sprintf("%s/init", className)
			if _, exists := plugin.jpbcModule.Functions[initName]; exists {
				guard := NewPermissionGuard(plugin.Metadata.Permissions)
				vm := NewJPBCVM(plugin.jpbcModule, guard, r.host)
				initArgs := append([]interface{}{instanceMap}, args...)
				_, err := vm.Execute(initName, initArgs)
				if err != nil {
					return nil, fmt.Errorf("error al inicializar instancia: %w", err)
				}
			}

			return instanceMap, nil

		default:
			return nil, fmt.Errorf("formato no soportado para instanciacion: %s", plugin.Format)
		}
	})
}

// CallMethod ejecuta un metodo sobre una instancia creada previamente por un plugin.
func (r *PluginRegistry) CallMethod(pluginName, className, methodName string, instance interface{}, args []interface{}) (res interface{}, err error) {
	plugin := r.Get(pluginName)
	if plugin == nil {
		return nil, fmt.Errorf("%w: %s", ErrPluginNotFound, pluginName)
	}

	return SafeCall(pluginName, className+"."+methodName, func() (interface{}, error) {
		switch plugin.Format {
		case FormatJossAST:
			if aware, ok := r.host.(PluginAwareHost); ok {
				return aware.CallPluginASTMethod(pluginName, className, methodName, instance, args)
			}
			if engine, ok := r.host.(ASTEngine); ok {
				return engine.CallMethod(instance, methodName, args)
			}
			if plugin.jossExecutor == nil {
				return nil, fmt.Errorf("plugin %s no tiene ejecutor AST disponible", pluginName)
			}
			return plugin.jossExecutor.CallMethod(instance, methodName, args)

		case FormatJPBC:
			qualifiedName := fmt.Sprintf("%s/%s", className, methodName)
			guard := NewPermissionGuard(plugin.Metadata.Permissions)
			vm := NewJPBCVM(plugin.jpbcModule, guard, r.host)

			// Pass instance as first argument
			methodArgs := append([]interface{}{instance}, args...)

			if _, exists := plugin.jpbcModule.Functions[qualifiedName]; exists {
				return vm.Execute(qualifiedName, methodArgs)
			}
			return vm.Execute(methodName, methodArgs)

		default:
			return nil, fmt.Errorf("formato no soportado para llamadas de metodo: %s", plugin.Format)
		}
	})
}
