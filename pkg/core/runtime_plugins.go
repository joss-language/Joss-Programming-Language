package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/pluginpkg"
	"github.com/jossecurity/joss/pkg/pluginruntime"
	"github.com/jossecurity/joss/pkg/typesystem"
)

// PluginCallable representa una funcion o metodo exportado por un plugin .jp.
type PluginCallable struct {
	PluginName string
	Function   string
	ClassName  string
}

// PluginNamespace representa el espacio de nombres de un plugin (ej: MiPlugin::metodo).
type PluginNamespace struct {
	Name    string
	Plugin  *pluginruntime.Plugin
	Runtime *Runtime
}

// PluginASTEngine conecta el ejecutor AST de un plugin nativo con el Runtime de Joss con aislamiento.
type PluginASTEngine struct {
	Runtime    *Runtime
	PluginName string
	Classes    map[string]*parser.ClassStatement
	Functions  map[string]*parser.MethodStatement
}

// NewPluginASTEngine crea una instancia de PluginASTEngine con tablas aisladas por plugin.
func NewPluginASTEngine(r *Runtime, pluginName string) *PluginASTEngine {
	return &PluginASTEngine{
		Runtime:    r,
		PluginName: pluginName,
		Classes:    make(map[string]*parser.ClassStatement),
		Functions:  make(map[string]*parser.MethodStatement),
	}
}

func (e *PluginASTEngine) RegisterProgram(prog *parser.Program) error {
	if prog == nil {
		return nil
	}
	for _, stmt := range prog.Statements {
		if classStmt, ok := stmt.(*parser.ClassStatement); ok {
			e.Classes[classStmt.Name.Value] = classStmt
			if e.Runtime != nil {
				if _, exists := e.Runtime.Classes[classStmt.Name.Value]; !exists {
					e.Runtime.Classes[classStmt.Name.Value] = classStmt
				}
			}
		}
		if methodStmt, ok := stmt.(*parser.MethodStatement); ok {
			e.Functions[methodStmt.Name.Value] = methodStmt
			if e.Runtime != nil {
				if _, exists := e.Runtime.Functions[methodStmt.Name.Value]; !exists {
					e.Runtime.Functions[methodStmt.Name.Value] = methodStmt
				}
			}
		}
	}
	return nil
}

func (e *PluginASTEngine) CallFunction(fnName string, args []interface{}) (interface{}, error) {
	if fn, ok := e.Functions[fnName]; ok {
		return e.Runtime.CallMethodEvaluated(fn, nil, args), nil
	}
	if e.Runtime != nil {
		if fn, ok := e.Runtime.Functions[fnName]; ok {
			return e.Runtime.CallMethodEvaluated(fn, nil, args), nil
		}
	}
	return nil, fmt.Errorf("funcion %s no encontrada en plugin %s", fnName, e.PluginName)
}

func (e *PluginASTEngine) Instantiate(className string, args []interface{}) (interface{}, error) {
	classStmt, ok := e.Classes[className]
	if !ok && e.Runtime != nil {
		classStmt = e.Runtime.Classes[className]
	}
	if classStmt == nil {
		return nil, fmt.Errorf("clase %s no encontrada en plugin %s", className, e.PluginName)
	}
	inst := &Instance{
		Class:     classStmt,
		Fields:    make(map[string]interface{}),
		Constants: make(map[string]bool),
	}
	inst.Fields["__plugin__"] = e.PluginName

	if classStmt.Body != nil {
		for _, member := range classStmt.Body.Statements {
			if letStmt, ok := member.(*parser.LetStatement); ok && letStmt.Name != nil {
				var val interface{}
				if letStmt.Value != nil && e.Runtime != nil {
					val = e.Runtime.evaluateExpression(letStmt.Value)
				}
				if letStmt.Token.Literal != "" && letStmt.Token.Literal != "var" && letStmt.Token.Literal != "mixed" && e.Runtime != nil {
					val = e.Runtime.coerceToParsedType(val, typesystem.Parse(letStmt.Token.Literal))
				}
				inst.Fields[letStmt.Name.Value] = val
				if letStmt.IsConst {
					inst.Constants[letStmt.Name.Value] = true
				}
			}
		}

		for _, member := range classStmt.Body.Statements {
			if initStmt, ok := member.(*parser.InitStatement); ok {
				method := &parser.MethodStatement{
					Token:      initStmt.Token,
					Name:       initStmt.Name,
					Parameters: initStmt.Parameters,
					Body:       initStmt.Body,
				}
				e.Runtime.CallMethodEvaluated(method, inst, args)
				break
			}
			if methodStmt, ok := member.(*parser.MethodStatement); ok && methodStmt.Name != nil && (methodStmt.Name.Value == "constructor" || methodStmt.Name.Value == "init") {
				e.Runtime.CallMethodEvaluated(methodStmt, inst, args)
				break
			}
		}
	}
	return inst, nil
}

func (e *PluginASTEngine) CallMethod(instance interface{}, methodName string, args []interface{}) (interface{}, error) {
	var inst *Instance
	if instance != nil {
		if i, ok := instance.(*Instance); ok {
			inst = i
		}
	}

	// 1. If an instance is provided, search its class
	if inst != nil && inst.Class != nil && inst.Class.Body != nil {
		for _, member := range inst.Class.Body.Statements {
			if methodStmt, ok := member.(*parser.MethodStatement); ok {
				if methodStmt.Name.Value == methodName {
					return e.Runtime.CallMethodEvaluated(methodStmt, inst, args), nil
				}
			}
		}
		return nil, fmt.Errorf("metodo %s no encontrado en clase %s", methodName, inst.Class.Name.Value)
	}

	// 2. Static method call (instance is nil): search in plugin classes
	for _, classStmt := range e.Classes {
		if classStmt != nil && classStmt.Body != nil {
			for _, member := range classStmt.Body.Statements {
				if methodStmt, ok := member.(*parser.MethodStatement); ok {
					if methodStmt.Name.Value == methodName {
						dummyInst := &Instance{
							Class:  classStmt,
							Fields: make(map[string]interface{}),
						}
						dummyInst.Fields["__plugin__"] = e.PluginName
						return e.Runtime.CallMethodEvaluated(methodStmt, dummyInst, args), nil
					}
				}
			}
		}
	}

	// 3. Fallback: search in functions
	if fn, ok := e.Functions[methodName]; ok {
		return e.Runtime.CallMethodEvaluated(fn, nil, args), nil
	}

	return nil, fmt.Errorf("metodo %s no encontrado en plugin %s", methodName, e.PluginName)
}

// CallHostFunction implementa pluginruntime.HostContext para callbacks de plugins.
func (r *Runtime) CallHostFunction(name string, args []interface{}) (interface{}, error) {
	if fn, ok := r.Functions[name]; ok {
		return r.CallMethodEvaluated(fn, nil, args), nil
	}
	if res, ok := r.callBuiltin(name, args); ok {
		return res, nil
	}
	return nil, fmt.Errorf("funcion de host '%s' no encontrada", name)
}

func (r *Runtime) ensurePluginASTEngines() {
	if r.pluginASTEngines == nil {
		r.pluginASTEngines = make(map[string]*PluginASTEngine)
	}
}

func (r *Runtime) getPluginASTEngine(pluginName string) *PluginASTEngine {
	if r == nil {
		return nil
	}
	r.ensurePluginASTEngines()
	if engine, ok := r.pluginASTEngines[pluginName]; ok {
		return engine
	}
	if r.PluginRegistry != nil {
		if plugin := r.PluginRegistry.Get(pluginName); plugin != nil && plugin.Program() != nil {
			engine := NewPluginASTEngine(r, pluginName)
			_ = engine.RegisterProgram(plugin.Program())
			r.pluginASTEngines[pluginName] = engine
			return engine
		}
	}
	return nil
}

// CallPluginAST implementa PluginAwareHost para ejecutar funciones AST en el runtime actual.
func (r *Runtime) CallPluginAST(pluginName, fnName string, args []interface{}) (interface{}, error) {
	engine := r.getPluginASTEngine(pluginName)
	if engine == nil {
		return nil, fmt.Errorf("plugin %s no tiene motor AST disponible en este runtime", pluginName)
	}
	return engine.CallFunction(fnName, args)
}

// InstantiatePluginAST implementa PluginAwareHost para instanciar clases AST en el runtime actual.
func (r *Runtime) InstantiatePluginAST(pluginName, className string, args []interface{}) (interface{}, error) {
	engine := r.getPluginASTEngine(pluginName)
	if engine == nil {
		return nil, fmt.Errorf("plugin %s no tiene motor AST disponible en este runtime", pluginName)
	}
	return engine.Instantiate(className, args)
}

// CallPluginASTMethod implementa PluginAwareHost para invocar metodos AST en el runtime actual.
func (r *Runtime) CallPluginASTMethod(pluginName, className, methodName string, instance interface{}, args []interface{}) (interface{}, error) {
	engine := r.getPluginASTEngine(pluginName)
	if engine == nil {
		return nil, fmt.Errorf("plugin %s no tiene motor AST disponible en este runtime", pluginName)
	}
	return engine.CallMethod(instance, methodName, args)
}

// LoadPluginPackage carga, verifica y registra un paquete .jp en el runtime.
func (r *Runtime) LoadPluginPackage(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("core: error leyendo archivo de plugin %s: %w", filePath, err)
	}
	return r.LoadPluginBytes(data)
}

// LoadPluginBytes carga, verifica y registra un plugin a partir de sus bytes .jp.
func (r *Runtime) LoadPluginBytes(data []byte) error {
	if r.PluginRegistry == nil {
		r.PluginRegistry = pluginruntime.NewPluginRegistry(r)
	}

	archive, err := pluginpkg.ReadVerified(data)
	if err != nil {
		return fmt.Errorf("core: error verificando plugin .jp: %w", err)
	}

	if r.PluginRegistry.Get(archive.Metadata.Name) != nil {
		// Plugin already loaded
		return nil
	}

	engine := NewPluginASTEngine(r, archive.Metadata.Name)
	plugin, err := pluginruntime.LoadPluginWithEngine(data, engine)
	if err != nil {
		return fmt.Errorf("core: error cargando plugin .jp: %w", err)
	}

	if err := r.PluginRegistry.Register(plugin); err != nil {
		return err
	}

	r.ensurePluginASTEngines()
	r.pluginASTEngines[archive.Metadata.Name] = engine

	r.registerPluginSymbols(plugin)
	return nil
}

func (r *Runtime) registerPluginSymbols(plugin *pluginruntime.Plugin) {
	// 1. Exponer namespace del plugin en variables (ej: MathService::calcular o $MathService.calcular)
	ns := &PluginNamespace{
		Name:    plugin.Name,
		Plugin:  plugin,
		Runtime: r,
	}
	r.Variables[plugin.Name] = ns

	// 2. Registrar funciones exportadas directas
	for _, fn := range plugin.Symbols.Functions {
		callable := &PluginCallable{
			PluginName: plugin.Name,
			Function:   fn.Name,
		}
		// Acceso calificado: plugin_name::func
		r.Variables[fmt.Sprintf("%s::%s", plugin.Name, fn.Name)] = callable

		// Acceso directo si no colisiona
		if _, exists := r.Functions[fn.Name]; !exists {
			if _, existsVar := r.Variables[fn.Name]; !existsVar {
				r.Variables[fn.Name] = callable
			}
		}
	}

	// 3. Registrar clases exportadas
	for _, cls := range plugin.Symbols.Classes {
		if _, exists := r.Classes[cls.Name]; !exists {
			syntheticClass := &parser.ClassStatement{
				Name: &parser.Identifier{Value: cls.Name},
				Body: &parser.BlockStatement{Statements: []parser.Statement{}},
			}
			r.Classes[cls.Name] = syntheticClass
		}
		// Exponer namespace de la clase del plugin (ej: AI::client o $AI.client)
		r.Variables[cls.Name] = &PluginNamespace{
			Name:    plugin.Name,
			Plugin:  plugin,
			Runtime: r,
		}
	}
}

// AutoloadPlugins escanea y carga automaticamente plugins .jp presentes en el proyecto.
func (r *Runtime) AutoloadPlugins(projectRoot string) {
	if projectRoot == "" {
		projectRoot = "."
	}

	pluginsDir := filepath.Join(projectRoot, "plugins")
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		return
	}

	_ = filepath.Walk(pluginsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".backup" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".jp") {
			if err := r.LoadPluginPackage(path); err != nil {
				if !strings.Contains(err.Error(), "ya registrado") {
					fmt.Printf("[Plugin Autoload] Error cargando %s: %v\n", path, err)
				}
			} else {
				fmt.Printf("[Plugin Autoload] Plugin cargado desde %s\n", path)
			}
		}
		return nil
	})
}
