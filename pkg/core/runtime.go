package core

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/parser"
	runtimeplan "github.com/jossecurity/joss/pkg/runtime/plan"
	"github.com/jossecurity/joss/pkg/version"
	_ "modernc.org/sqlite"
)

var (
	// BroadcastFunc is a hook for WebSocket broadcasting
	BroadcastFunc func(msg interface{})

	// GlobalFileSystem is the VFS for the application
	GlobalFileSystem http.FileSystem

	runtimePool = sync.Pool{
		New: func() interface{} {
			r := &Runtime{
				Env:                make(map[string]string),
				Variables:          make(map[string]interface{}),
				VarTypes:           make(map[string]string),
				Constants:          make(map[string]bool),
				HostGlobals:        make(map[string]bool),
				Classes:            make(map[string]*parser.ClassStatement),
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
			r.Variables["cout"] = &Cout{}
			r.Variables["cin"] = &Cin{}
			r.Variables["cerr"] = &Cerr{}
			r.Variables["endl"] = "\n"
			r.Constants["endl"] = true
			r.Variables["JOSS_VERSION"] = version.Version
			r.RegisterNativeClasses()
			r.markCurrentVariablesAsHostGlobals()
			return r
		},
	}
)

// SetFileSystem sets the global file system
func SetFileSystem(fs http.FileSystem) {
	GlobalFileSystem = fs
}

// NewRuntime gets a runtime from the pool
func NewRuntime() *Runtime {
	// Initialize Logger globally once
	InitLogger()

	r := runtimePool.Get().(*Runtime)
	if r.Constants == nil {
		r.Constants = make(map[string]bool)
	}
	if r.NativePlugins == nil {
		r.NativePlugins = make(map[string]*NativePluginDefinition)
	}
	if r.NativeDrivers == nil {
		r.NativeDrivers = make(map[string]*NativeDriverDefinition)
	}
	if r.HostGlobals == nil {
		r.HostGlobals = make(map[string]bool)
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
	// Ensure native classes are registered (if recycled)
	if _, ok := r.Variables["View"]; !ok {
		r.Variables["cout"] = &Cout{}
		r.Variables["cin"] = &Cin{}
		r.Variables["cerr"] = &Cerr{}
		r.Variables["endl"] = "\n"
		r.Constants["endl"] = true
		r.Variables["JOSS_VERSION"] = version.Version
		r.RegisterNativeClasses()
		r.markCurrentVariablesAsHostGlobals()

		// Initialize GlobalAssetManager once
		am := GetAssetManager()
		am.Initialize()
	}
	r.AutoloadPlugins(".")
	r.markCurrentVariablesAsHostGlobals()
	return r
}

// FreeRuntime returns the runtime to the pool
func (r *Runtime) Free() {
	// Reset state
	for k := range r.Variables {
		delete(r.Variables, k)
	}
	for k := range r.VarTypes {
		delete(r.VarTypes, k)
	}
	for k := range r.Constants {
		delete(r.Constants, k)
	}
	for k := range r.HostGlobals {
		delete(r.HostGlobals, k)
	}
	for k := range r.Classes {
		delete(r.Classes, k)
	}
	for k := range r.Functions {
		delete(r.Functions, k)
	}
	for k := range r.Routes {
		delete(r.Routes, k)
	}
	for k := range r.CustomMiddlewares {
		delete(r.CustomMiddlewares, k)
	}
	for k := range r.NativeHandlers {
		delete(r.NativeHandlers, k)
	}
	for k := range r.NativePlugins {
		delete(r.NativePlugins, k)
	}
	for k := range r.NativeDrivers {
		delete(r.NativeDrivers, k)
	}
	for method := range r.callablePlans {
		delete(r.callablePlans, method)
	}
	for function := range r.functionPlans {
		delete(r.functionPlans, function)
	}
	// PluginRegistry owns symbol tables whose host context is this Runtime.
	// Keeping it while clearing Variables/Classes makes a recycled runtime skip
	// plugin registration because the archive still appears to be loaded.
	r.PluginRegistry = nil
	// Restore standard variables
	r.Variables["cout"] = &Cout{}
	r.Variables["cin"] = &Cin{}
	r.Variables["cerr"] = &Cerr{}
	r.Variables["endl"] = "\n"
	r.Constants["endl"] = true
	r.markCurrentVariablesAsHostGlobals()

	r.CurrentMiddleware = r.CurrentMiddleware[:0]
	r.ProjectRoot = ""
	r.callDepth = 0
	r.callStack = r.callStack[:0]
	r.currentFrame = nil
	r.MaxCallDepth = DefaultMaxCallDepth
	r.captureEnvironment = nil
	r.SitemapEntries = r.SitemapEntries[:0]
	r.SitemapProviders = r.SitemapProviders[:0]
	r.SitemapExclusions = r.SitemapExclusions[:0]

	runtimePool.Put(r)
}

// Fork creates a lightweight copy of the runtime for request isolation
func (r *Runtime) Fork() *Runtime {
	// fmt.Printf("[RUNTIME] Forking from %p\n", r)
	newR := &Runtime{
		Env:               make(map[string]string),
		Classes:           copyClassMap(r.Classes),
		Functions:         copyMethodMap(r.Functions),
		Routes:            make(map[string]map[string]interface{}),
		CurrentMiddleware: make([]string, 0),
		CustomMiddlewares: make(map[string]interface{}),
		DB:                r.DB, // Share DB Connection (Thread-Safe)
		Variables:         make(map[string]interface{}),
		VarTypes:          make(map[string]string),
		Constants:         copyBoolMap(r.Constants),
		HostGlobals:       copyBoolMap(r.HostGlobals),
		NativeHandlers:    copyNativeHandlerMap(r.NativeHandlers),
		NativePlugins:     copyNativePluginMap(r.NativePlugins),
		NativeDrivers:     copyNativeDriverMap(r.NativeDrivers),
		callablePlans:     copyCallablePlanMap(r.callablePlans),
		functionPlans:     copyFunctionPlanMap(r.functionPlans),
		MaxCallDepth:      r.MaxCallDepth,
		PluginRegistry:    r.PluginRegistry,
		ProjectRoot:       r.ProjectRoot,
	}
	// fmt.Println("[RUNTIME] Fork: Maps initialized")

	// Copy Env
	for k, v := range r.Env {
		newR.Env[k] = v
	}

	// Copy Routes (Deep Copy to allow dynamic route modification per request without race)
	for method, routes := range r.Routes {
		newR.Routes[method] = make(map[string]interface{})
		for path, handler := range routes {
			newR.Routes[method][path] = handler
		}
	}

	// Copy Custom Middlewares
	for name, handler := range r.CustomMiddlewares {
		newR.CustomMiddlewares[name] = handler
	}

	// Initialize standard variables
	newR.Variables["cout"] = &Cout{}
	newR.Variables["cin"] = &Cin{}
	newR.Variables["cerr"] = &Cerr{}
	newR.Variables["endl"] = "\n"
	newR.Constants["endl"] = true
	newR.Variables["JOSS_VERSION"] = version.Version

	// Deep Copy Global Variables
	for k, v := range r.Variables {
		if inst, ok := v.(*Instance); ok {
			newR.Variables[k] = inst.Clone()
		} else if m, ok := v.(map[string]interface{}); ok {
			// Deep copy maps
			newMap := make(map[string]interface{})
			for mk, mv := range m {
				newMap[mk] = mv
			}
			newR.Variables[k] = newMap
		} else if l, ok := v.([]interface{}); ok {
			// Deep copy slices
			newList := make([]interface{}, len(l))
			copy(newList, l)
			newR.Variables[k] = newList
		} else {
			newR.Variables[k] = v
		}
	}

	// Copy Functions and Classes
	for k, v := range r.Functions {
		newR.Functions[k] = v
	}
	for k, v := range r.Classes {
		newR.Classes[k] = v
	}
	for k, v := range r.VarTypes {
		newR.VarTypes[k] = v
	}

	// Copy Sitemap Entries, Providers & Exclusions
	if len(r.SitemapEntries) > 0 {
		newR.SitemapEntries = make([]SitemapEntry, len(r.SitemapEntries))
		copy(newR.SitemapEntries, r.SitemapEntries)
	}
	if len(r.SitemapProviders) > 0 {
		newR.SitemapProviders = make([]*CapturedFunction, len(r.SitemapProviders))
		copy(newR.SitemapProviders, r.SitemapProviders)
	}
	if len(r.SitemapExclusions) > 0 {
		newR.SitemapExclusions = make([]string, len(r.SitemapExclusions))
		copy(newR.SitemapExclusions, r.SitemapExclusions)
	}

	return newR
}

func copyBoolMap(source map[string]bool) map[string]bool {
	result := make(map[string]bool, len(source))
	for name, value := range source {
		result[name] = value
	}
	return result
}

func copyClassMap(source map[string]*parser.ClassStatement) map[string]*parser.ClassStatement {
	result := make(map[string]*parser.ClassStatement, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func copyMethodMap(source map[string]*parser.MethodStatement) map[string]*parser.MethodStatement {
	result := make(map[string]*parser.MethodStatement, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func copyNativeHandlerMap(source map[string]NativeHandler) map[string]NativeHandler {
	result := make(map[string]NativeHandler, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func copyNativePluginMap(source map[string]*NativePluginDefinition) map[string]*NativePluginDefinition {
	result := make(map[string]*NativePluginDefinition, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func copyNativeDriverMap(source map[string]*NativeDriverDefinition) map[string]*NativeDriverDefinition {
	result := make(map[string]*NativeDriverDefinition, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func copyCallablePlanMap(source map[*parser.MethodStatement]*runtimeplan.Callable) map[*parser.MethodStatement]*runtimeplan.Callable {
	result := make(map[*parser.MethodStatement]*runtimeplan.Callable, len(source))
	for method, compiled := range source {
		result[method] = compiled
	}
	return result
}

func copyFunctionPlanMap(source map[*parser.FunctionLiteral]*runtimeplan.Callable) map[*parser.FunctionLiteral]*runtimeplan.Callable {
	result := make(map[*parser.FunctionLiteral]*runtimeplan.Callable, len(source))
	for function, compiled := range source {
		result[function] = compiled
	}
	return result
}

// LoadEnv loads environment variables from env.joss
func (r *Runtime) LoadEnv(fs http.FileSystem) {
	fmt.Println("[Security] Cargando entorno...")

	// Initialize I18n
	i18n.GlobalManager.Load(fs)

	detect := DetectAndLoadEnv(fs)
	if detect.FilePath != "" {
		fmt.Printf("[Security] Entorno cargado desde '%s'\n", detect.FilePath)
	}

	for k, v := range detect.EnvMap {
		r.Env[k] = v
	}

	// Ensure APP_KEY exists
	if _, hasKey := r.Env["APP_KEY"]; !hasKey || r.Env["APP_KEY"] == "" {
		newKey := generateSecureKey()
		r.Env["APP_KEY"] = newKey
		r.writeEnvJoss()
	}

	// 4. Override with System Environment Variables (Docker/System Priority)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			k := parts[0]
			v := parts[1]
			r.Env[k] = v
		}
	}

	if r.Env["PREFIX"] == "" && r.Env["DB_PREFIX"] != "" {
		r.Env["PREFIX"] = r.Env["DB_PREFIX"]
	}
	if r.Env["DB_PREFIX"] == "" && r.Env["PREFIX"] != "" {
		r.Env["DB_PREFIX"] = r.Env["PREFIX"]
	}

	// 5. Autogenerate JWT_SECRET and APP_KEY if weak or missing
	updatedEnv := false
	jwtSec := r.Env["JWT_SECRET"]
	if jwtSec == "" || jwtSec == "joss_default_secret_change_in_production" || len(jwtSec) < 32 {
		r.Env["JWT_SECRET"] = generateSecureKey()
		updatedEnv = true
		fmt.Println("[Security] Advertencia: JWT_SECRET inexistente o debil. Autogenerando uno nuevo y seguro...")
	}

	appKey := r.Env["APP_KEY"]
	if appKey == "" || appKey == "joss_default_secret_change_in_production" || len(appKey) < 32 {
		r.Env["APP_KEY"] = generateSecureKey()
		updatedEnv = true
		fmt.Println("[Security] Advertencia: APP_KEY inexistente o debil. Autogenerando uno nuevo y seguro...")
	}

	if updatedEnv {
		r.writeEnvJoss()
	}
}

func generateSecureKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "joss_fallback_secret_secure_key_123456"
	}
	return hex.EncodeToString(bytes)
}

func (r *Runtime) writeEnvJoss() {
	filePath := "env.joss"
	if _, err := os.Stat("env.joss"); os.IsNotExist(err) {
		if _, errDot := os.Stat(".env"); errDot == nil {
			filePath = ".env"
		} else {
			f, errCreate := os.Create("env.joss")
			if errCreate != nil {
				return
			}
			f.Close()
		}
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	hasJWT := false
	hasKey := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "JWT_SECRET=") {
			lines[i] = fmt.Sprintf("JWT_SECRET=\"%s\"", r.Env["JWT_SECRET"])
			hasJWT = true
		} else if strings.HasPrefix(trimmed, "APP_KEY=") {
			lines[i] = fmt.Sprintf("APP_KEY=\"%s\"", r.Env["APP_KEY"])
			hasKey = true
		}
	}

	var newLines []string
	for _, l := range lines {
		newLines = append(newLines, l)
	}
	if !hasJWT {
		newLines = append(newLines, fmt.Sprintf("JWT_SECRET=\"%s\"", r.Env["JWT_SECRET"]))
	}
	if !hasKey {
		newLines = append(newLines, fmt.Sprintf("APP_KEY=\"%s\"", r.Env["APP_KEY"]))
	}

	os.WriteFile(filePath, []byte(strings.Join(newLines, "\n")), 0644)
	fmt.Printf("[Security] Archivo de entorno %s actualizado con claves de seguridad fuertes.\n", filePath)
}

// GetDB ensures the database connection is initialized and returns it.
func (r *Runtime) GetDB() *sql.DB {
	// If already connected, return it
	if r.DB != nil {
		return r.DB
	}

	// Connect to DB lazily
	dbDriver := "mysql"
	if val, ok := r.Env["DB"]; ok {
		dbDriver = normalizeDatabaseDriver(val)
	}

	db, err := OpenConfiguredDatabase(dbDriver, r.Env)
	if err == nil && db != nil {
		r.DB = db

		// Optimize SQLite for Concurrency
		if dbDriver == "sqlite" {
			_, err := db.Exec("PRAGMA journal_mode=WAL;")
			if err != nil {
				fmt.Printf("[Security] Error activando WAL: %v\n", err)
			}
			_, err = db.Exec("PRAGMA busy_timeout = 5000;")
			if err != nil {
				fmt.Printf("[Security] Error setting busy_timeout: %v\n", err)
			}
		}

		r.EnsureCronTable()
		r.EnsureMigrationTable()
		r.EnsureAuthTables()
		r.EnsureMFATables()

		// Connection Pooling Settings
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)
	} else if err != nil {
		fmt.Printf("[Security] Error fatal de conexión SQL (%s): %v\n", dbDriver, err)
	}

	return r.DB
}

// ChangeDB dynamically switches the database connection and driver at runtime.
func (r *Runtime) ChangeDB(driverName string, config ...map[string]string) error {
	normalized := normalizeDatabaseDriver(driverName)
	if r.Env == nil {
		r.Env = make(map[string]string)
	}
	r.Env["DB"] = normalized
	if len(config) > 0 && config[0] != nil {
		for k, v := range config[0] {
			r.Env[k] = v
		}
	}
	if r.DB != nil {
		_ = r.DB.Close()
		r.DB = nil
	}
	db, err := OpenConfiguredDatabase(normalized, r.Env)
	if err != nil {
		return err
	}
	r.DB = db
	if normalized == "sqlite" {
		_, _ = db.Exec("PRAGMA journal_mode=WAL;")
		_, _ = db.Exec("PRAGMA busy_timeout = 5000;")
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	return nil
}

// Server connection is now handled lazily via r.GetDB()

// NewInstance creates a new instance of a class
func NewInstance(class *parser.ClassStatement) *Instance {
	return &Instance{
		Class:     class,
		Fields:    make(map[string]interface{}),
		Constants: make(map[string]bool),
	}
}

// Clone creates a deep copy of the instance (for runtime forking)
func (i *Instance) Clone() *Instance {
	if i == nil {
		return nil
	}
	newI := &Instance{
		Class:     i.Class,
		Fields:    make(map[string]interface{}),
		Constants: make(map[string]bool),
	}
	for k, v := range i.Fields {
		newI.Fields[k] = v
	}
	for k, v := range i.Constants {
		newI.Constants[k] = v
	}
	return newI
}

// PreloadAppFiles strictly preloads .joss files recursively within domain folders defined in parser.StandardAppDomains.
func (r *Runtime) PreloadAppFiles(targetPath string) {
	if targetPath == "" || targetPath == "app" {
		for _, domain := range parser.StandardAppDomains {
			r.preloadSingleDir(filepath.FromSlash(domain))
		}
		return
	}
	r.preloadSingleDir(targetPath)
}

func (r *Runtime) preloadSingleDir(dirPath string) {
	info, err := os.Stat(dirPath)
	if err != nil || !info.IsDir() {
		return
	}

	_ = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if parser.IsIgnoredDirectory(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if parser.IsJossSourceFile(path) {
			content, readErr := os.ReadFile(path)
			if readErr == nil {
				l := parser.NewLexer(string(content))
				p := parser.NewParser(l)
				program := p.ParseProgram()
				if len(p.Errors()) == 0 {
					r.Execute(program)
				} else {
					fmt.Printf("[Runtime Warning] Parser errors in %s:\n", path)
					for _, msg := range p.Errors() {
						fmt.Printf("\t%s\n", msg)
					}
				}
			}
		}
		return nil
	})
}

// PreloadVFSAppFiles recursively preloads .joss files from VFS within domain-scoped subfolders.
func (r *Runtime) PreloadVFSAppFiles(fs http.FileSystem, targetPath string) {
	if fs == nil {
		r.PreloadAppFiles(targetPath)
		return
	}

	domains := parser.StandardAppDomains
	if targetPath != "" && targetPath != "app" {
		domains = []string{filepath.ToSlash(targetPath)}
	}

	var walkVFS func(dir string)
	walkVFS = func(dir string) {
		dirFile, err := fs.Open(dir)
		if err != nil {
			return
		}
		defer dirFile.Close()

		info, err := dirFile.Stat()
		if err != nil {
			return
		}
		if !info.IsDir() {
			if strings.HasSuffix(dir, ".joss") {
				content, err := io.ReadAll(dirFile)
				if err == nil {
					l := parser.NewLexer(string(content))
					p := parser.NewParser(l)
					program := p.ParseProgram()
					if len(p.Errors()) == 0 {
						r.Execute(program)
					}
				}
			}
			return
		}

		if readdir, ok := dirFile.(interface {
			Readdir(count int) ([]os.FileInfo, error)
		}); ok {
			infos, err := readdir.Readdir(-1)
			if err == nil {
				for _, childInfo := range infos {
					childPath := filepath.ToSlash(filepath.Join(dir, childInfo.Name()))
					if childInfo.IsDir() {
						walkVFS(childPath)
					} else if strings.HasSuffix(childInfo.Name(), ".joss") {
						cf, err := fs.Open(childPath)
						if err == nil {
							data, readErr := io.ReadAll(cf)
							cf.Close()
							if readErr == nil {
								l := parser.NewLexer(string(data))
								p := parser.NewParser(l)
								program := p.ParseProgram()
								if len(p.Errors()) == 0 {
									r.Execute(program)
								}
							}
						}
					}
				}
			}
		}
	}

	for _, domain := range domains {
		walkVFS(domain)
	}
}
