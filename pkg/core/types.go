package core

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"sync"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/pluginruntime"
	runtimeerrors "github.com/jossecurity/joss/pkg/runtime/errors"
	runtimeplan "github.com/jossecurity/joss/pkg/runtime/plan"
)

// NativeHandler is a function that executes a native method
type NativeHandler func(r *Runtime, instance *Instance, method string, args []interface{}) interface{}

// SEOData stores metadata for search engine optimization
type SEOData struct {
	Title       string
	Description string
	Keywords    []string
	Canonical   string
	Meta        map[string]string
	OG          map[string]string
}

// SitemapEntry represents a URL in the sitemap
type SitemapEntry struct {
	URL        string
	LastMod    string
	ChangeFreq string
	Priority   float64
}

// Runtime manages the execution environment of a Joss program
type Runtime struct {
	Env               map[string]string
	Variables         map[string]interface{}
	VarTypes          map[string]string // For strict typing
	Constants         map[string]bool
	HostGlobals       map[string]bool // runtime/plugin bindings visible inside named callables
	Classes           map[string]*parser.ClassStatement
	Interfaces        map[string]*parser.InterfaceStatement
	Enums             map[string]*EnumDefinition
	Functions         map[string]*parser.MethodStatement
	DB                *sql.DB
	Routes            map[string]map[string]interface{} // HTTP Method -> Path -> Handler
	CurrentMiddleware []string
	CustomMiddlewares map[string]interface{} // Name -> Closure/Handler
	NativeHandlers    map[string]NativeHandler
	NativePlugins     map[string]*NativePluginDefinition
	NativeDrivers     map[string]*NativeDriverDefinition
	PluginRegistry    *pluginruntime.PluginRegistry
	ProjectRoot       string
	pluginASTEngines  map[string]*PluginASTEngine
	freed             bool

	// SEO & Sitemap
	SEO                *SEOData
	SitemapEntries     []SitemapEntry
	SitemapProviders   []*CapturedFunction
	SitemapExclusions  []string
	CurrentSource      string // "routes", "api", "app", etc.
	CurrentFile        string // Currently executing file path
	MaxCallDepth       int    // Guard against unbounded recursive calls
	callDepth          int
	currentClass       string
	callStack          []runtimeerrors.Frame
	callablePlans      map[*parser.MethodStatement]*runtimeplan.Callable
	functionPlans      map[*parser.FunctionLiteral]*runtimeplan.Callable
	classMetadataCache map[string]*classMetadata
	currentFrame       *executionFrame
	planMu             sync.Mutex

	captureEnvironment *ClosureEnvironment
	cinReader          *bufio.Reader
	currentGenerator   *Generator
	generatorIndex     int64
	cinTokens          []string
	topDefers          []*parser.DeferStatement
}

func (r *Runtime) markCurrentVariablesAsHostGlobals() {
	if r.HostGlobals == nil {
		r.HostGlobals = make(map[string]bool)
	}
	for name := range r.Variables {
		r.HostGlobals[name] = true
	}
}

func (r *Runtime) MarkHostGlobals(names ...string) {
	r.markHostGlobals(names...)
}

func (r *Runtime) markHostGlobals(names ...string) {
	if r.HostGlobals == nil {
		r.HostGlobals = make(map[string]bool)
	}
	for _, name := range names {
		r.HostGlobals[name] = true
	}
}

const DefaultMaxCallDepth = 1024

// ClosureEnvironment is the shared lexical state of callbacks captured during
// the same function or method invocation.
type ClosureEnvironment struct {
	Variables map[string]interface{}
	VarTypes  map[string]string
	Constants map[string]bool
	mu        sync.Mutex
}

// CapturedFunction is a function literal paired with the lexical environment
// that existed when a long-lived callback was registered.
//
// Function literals evaluate to captured functions so both immediate and
// deferred native consumers observe the same lexical environment.
type CapturedFunction struct {
	Function    *parser.FunctionLiteral
	Environment *ClosureEnvironment
}

// NativePluginDefinition describes a JP v2 plugin runtime payload.
type NativePluginDefinition struct {
	Name         string
	Version      string
	Root         string
	Protocol     string
	Executable   string
	Driver       string
	ArchiveFiles map[string][]byte // read-only, used by compiled VFS applications
	UseVFS       bool
}

// NativeDriverDefinition is a loaded C ABI v1 library.
type NativeDriverDefinition struct {
	Name     string
	Path     string
	Handle   uintptr
	Call     func(string, string) *byte
	Free     func(*byte)
	Mu       sync.Mutex
	unloaded bool
}

// Instance represents an instance of a class
type Instance struct {
	Class      *parser.ClassStatement
	Fields     map[string]interface{}
	Constants  map[string]bool
	Destroyed  bool
	Destroying bool
	Mu         sync.RWMutex
}

func (i *Instance) MarshalJSON() ([]byte, error) {
	if i == nil {
		return []byte("null"), nil
	}
	return json.Marshal(i.Fields)
}

func (i *Instance) String() string {
	if i == nil {
		return "null"
	}
	i.Mu.RLock()
	defer i.Mu.RUnlock()
	if msg, ok := i.Fields["message"].(string); ok && msg != "" {
		return msg
	}
	if i.Class != nil && i.Class.Name != nil {
		return "Instance of " + i.Class.Name.Value
	}
	return "Instance"
}

// BoundMethod represents a method bound to an instance
type BoundMethod struct {
	Method      *parser.MethodStatement
	Instance    *Instance
	StaticClass string // For static calls
}

// Future represents an asynchronous computation
type Future struct {
	done   chan bool
	result interface{}
	err    error
}

// EnumValue represents an individual case instance of an enum
type EnumValue struct {
	EnumName string
	Name     string
	Value    interface{}
}

func (ev *EnumValue) String() string {
	return ev.EnumName + "::" + ev.Name
}

func (ev *EnumValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"enum":  ev.EnumName,
		"name":  ev.Name,
		"value": ev.Value,
	})
}

// EnumDefinition stores the declaration metadata of an enum
type EnumDefinition struct {
	Name        string
	BackingType string
	Cases       map[string]*EnumValue
	CaseOrder   []*EnumValue
}

// Channel represents a Go channel
type Channel struct {
	Ch chan interface{}
}

func (c *Channel) String() string { return "channel" }

// ReturnPanic is used to bubble up ReturnStatements through the AST
type ReturnPanic struct {
	Value interface{}
}

// BreakPanic is used to exit a loop
type BreakPanic struct{}

// ContinuePanic is used to skip to the next loop iteration
type ContinuePanic struct{}

// Wait blocks until the Future completes and returns the result
func (f *Future) Wait() interface{} {
	<-f.done
	if f.err != nil {
		panic(f.err)
	}
	return f.result
}

// Cout represents standard output stream
type Cout struct{}

func (c *Cout) String() string { return "cout" }

// Cin represents standard input stream
type Cin struct{}

func (c *Cin) String() string { return "cin" }

// Cerr represents standard error stream
type Cerr struct{}

func (c *Cerr) String() string { return "cerr" }
