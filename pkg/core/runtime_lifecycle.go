package core

import (
	"sync"

	"github.com/jossecurity/joss/pkg/parser"
	runtimeplan "github.com/jossecurity/joss/pkg/runtime/plan"
	"github.com/jossecurity/joss/pkg/version"
)

var runtimePool = sync.Pool{New: newRuntimeState}

func newRuntimeState() interface{} {
	r := &Runtime{
		Env:                make(map[string]string),
		Variables:          make(map[string]interface{}),
		VarTypes:           make(map[string]string),
		Constants:          make(map[string]bool),
		HostGlobals:        make(map[string]bool),
		Classes:            make(map[string]*parser.ClassStatement),
		Interfaces:         make(map[string]*parser.InterfaceStatement),
		Enums:              make(map[string]*EnumDefinition),
		Functions:          make(map[string]*parser.MethodStatement),
		Routes:             make(map[string]map[string]interface{}),
		CurrentMiddleware:  make([]string, 0),
		CustomMiddlewares:  make(map[string]interface{}),
		NativeHandlers:     make(map[string]NativeHandler),
		NativePlugins:      make(map[string]*NativePluginDefinition),
		NativeDrivers:      make(map[string]*NativeDriverDefinition),
		callablePlans:      make(map[*parser.MethodStatement]*runtimeplan.Callable),
		functionPlans:      make(map[*parser.FunctionLiteral]*runtimeplan.Callable),
		classMetadataCache: make(map[string]*classMetadata),
		MaxCallDepth:       DefaultMaxCallDepth,
	}
	r.registerCanonicalHostState()
	return r
}

// NewRuntime acquires a clean runtime lifecycle from the pool.
func NewRuntime() *Runtime {
	InitLogger()
	r := runtimePool.Get().(*Runtime)
	r.ensureLifecycleMaps()
	if _, ok := r.Variables["View"]; !ok {
		r.registerCanonicalHostState()
		GetAssetManager().Initialize()
	}
	r.AutoloadPlugins(".")
	r.markCurrentVariablesAsHostGlobals()
	return r
}

func (r *Runtime) ensureLifecycleMaps() {
	if r.Env == nil {
		r.Env = make(map[string]string)
	}
	if r.Variables == nil {
		r.Variables = make(map[string]interface{})
	}
	if r.VarTypes == nil {
		r.VarTypes = make(map[string]string)
	}
	if r.Constants == nil {
		r.Constants = make(map[string]bool)
	}
	if r.HostGlobals == nil {
		r.HostGlobals = make(map[string]bool)
	}
	if r.Classes == nil {
		r.Classes = make(map[string]*parser.ClassStatement)
	}
	if r.Interfaces == nil {
		r.Interfaces = make(map[string]*parser.InterfaceStatement)
	}
	if r.Enums == nil {
		r.Enums = make(map[string]*EnumDefinition)
	}
	if r.Functions == nil {
		r.Functions = make(map[string]*parser.MethodStatement)
	}
	if r.Routes == nil {
		r.Routes = make(map[string]map[string]interface{})
	}
	if r.CustomMiddlewares == nil {
		r.CustomMiddlewares = make(map[string]interface{})
	}
	if r.NativeHandlers == nil {
		r.NativeHandlers = make(map[string]NativeHandler)
	}
	if r.NativePlugins == nil {
		r.NativePlugins = make(map[string]*NativePluginDefinition)
	}
	if r.NativeDrivers == nil {
		r.NativeDrivers = make(map[string]*NativeDriverDefinition)
	}
	if r.callablePlans == nil {
		r.callablePlans = make(map[*parser.MethodStatement]*runtimeplan.Callable)
	}
	if r.functionPlans == nil {
		r.functionPlans = make(map[*parser.FunctionLiteral]*runtimeplan.Callable)
	}
	if r.classMetadataCache == nil {
		r.classMetadataCache = make(map[string]*classMetadata)
	}
	if r.MaxCallDepth <= 0 {
		r.MaxCallDepth = DefaultMaxCallDepth
	}
}

func (r *Runtime) installStandardBindings() {
	r.Variables["cout"] = &Cout{}
	r.Variables["cin"] = &Cin{}
	r.Variables["cerr"] = &Cerr{}
	r.Variables["endl"] = "\n"
	r.Constants["endl"] = true
	r.Variables["JOSS_VERSION"] = version.Version
}

func (r *Runtime) registerCanonicalHostState() {
	r.installStandardBindings()
	r.RegisterNativeClasses()
	r.markCurrentVariablesAsHostGlobals()
}

// Free resets request/execution state and returns the runtime to the pool.
// External resources such as DB are not owned or closed by this lifecycle.
func (r *Runtime) Free() {
	clear(r.Env)
	clear(r.Variables)
	clear(r.VarTypes)
	clear(r.Constants)
	clear(r.HostGlobals)
	clear(r.Classes)
	clear(r.Interfaces)
	clear(r.Enums)
	clear(r.Functions)
	clear(r.Routes)
	clear(r.CustomMiddlewares)
	clear(r.NativeHandlers)
	clear(r.NativePlugins)
	clear(r.NativeDrivers)
	clear(r.callablePlans)
	clear(r.functionPlans)
	clear(r.classMetadataCache)

	// Registry symbol tables are bound to this Runtime. Retaining the registry
	// while clearing its exposed symbols would skip plugin registration on reuse.
	r.PluginRegistry = nil
	r.installStandardBindings()
	r.markCurrentVariablesAsHostGlobals()

	r.CurrentMiddleware = r.CurrentMiddleware[:0]
	r.ProjectRoot = ""
	r.CurrentSource = ""
	r.CurrentFile = ""
	r.callDepth = 0
	r.currentClass = ""
	r.callStack = r.callStack[:0]
	r.currentFrame = nil
	r.MaxCallDepth = DefaultMaxCallDepth
	r.captureEnvironment = nil
	r.SEO = nil
	r.SitemapEntries = r.SitemapEntries[:0]
	r.SitemapProviders = r.SitemapProviders[:0]
	r.SitemapExclusions = r.SitemapExclusions[:0]
	r.cinReader = nil
	r.cinTokens = r.cinTokens[:0]
	r.currentGenerator = nil
	r.generatorIndex = 0
	r.topDefers = r.topDefers[:0]

	runtimePool.Put(r)
}
