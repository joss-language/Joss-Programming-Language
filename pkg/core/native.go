package core

import (
	"fmt"
	"sort"
	"sync"

	"github.com/jossecurity/joss/pkg/parser"
)

var (
	nativeClassesMu  sync.RWMutex
	nativeClassesMap = make(map[string]bool)
)

// Helper to register a native class and its handler
func (r *Runtime) registerNative(name string, methods []string, handler NativeHandler) {
	nativeClassesMu.Lock()
	nativeClassesMap[name] = true
	nativeClassesMu.Unlock()

	// Build MethodStatements
	stmts := []parser.Statement{}
	for _, m := range methods {
		returnType := nativeMethodReturnType(name, m)
		stmts = append(stmts, &parser.MethodStatement{
			Name:       &parser.Identifier{Value: m},
			ReturnType: parser.Token{Type: parser.IDENT, Literal: returnType.String()},
		})
	}

	classStmt := &parser.ClassStatement{
		Name: &parser.Identifier{Value: name},
		Body: &parser.BlockStatement{Statements: stmts},
	}
	r.registerClass(classStmt)
	r.NativeHandlers[name] = handler
}

// IsNativeClass returns true if the class name is a registered native class in Joss.
func IsNativeClass(name string) bool {
	nativeClassesMu.RLock()
	defer nativeClassesMu.RUnlock()
	return nativeClassesMap[name]
}

// GetNativeClassMethods returns a detached catalog built by the same
// registration routine used by Runtime. Tooling generators consume it instead
// of maintaining a parallel native-class list.
func GetNativeClassMethods() map[string][]string {
	runtime := &Runtime{
		Variables:      make(map[string]interface{}),
		Classes:        make(map[string]*parser.ClassStatement),
		NativeHandlers: make(map[string]NativeHandler),
	}
	runtime.RegisterNativeClasses()
	result := make(map[string][]string, len(runtime.Classes))
	for name, class := range runtime.Classes {
		methods := []string{}
		if class != nil && class.Body != nil {
			for _, statement := range class.Body.Statements {
				if method, ok := statement.(*parser.MethodStatement); ok && method.Name != nil {
					methods = append(methods, method.Name.Value)
				}
			}
		}
		sort.Strings(methods)
		result[name] = methods
	}
	return result
}

// RegisterNativeClasses injects the native class definitions into the runtime
func (r *Runtime) RegisterNativeClasses() {
	// Note: We use (*Runtime).methodName to register UNBOUND methods as handlers.
	// This ensures that when they are called, we pass the *current* execution runtime 'r',
	// not the original 'r' that tried to register them.

	// Stack
	r.registerNative("Stack", []string{"push", "pop", "peek"}, (*Runtime).executeStackMethod)

	// Queue
	r.registerNative("Queue", []string{"enqueue", "dequeue", "peek"}, (*Runtime).executeQueueMethod)

	// GranDB
	granDBMethods := []string{
		"table", "select", "changeDB", "changedb", "connection", "use", "distinct",
		"where", "orWhere", "orwhere", "whereLike", "wherelike", "orWhereLike", "orwherelike",
		"whereColumn", "wherecolumn", "orWhereColumn", "orwherecolumn",
		"whereNot", "wherenot", "orWhereNot", "orwherenot",
		"whereIn", "wherein", "orWhereIn", "orwherein", "whereNotIn", "wherenotin", "orWhereNotIn", "orwherenotin",
		"whereNull", "wherenull", "orWhereNull", "orwherenull", "whereNotNull", "wherenotnull", "orWhereNotNull", "orwherenotnull",
		"whereBetween", "wherebetween", "orWhereBetween", "orwherebetween", "whereNotBetween", "wherenotbetween", "orWhereNotBetween", "orwherenotbetween",
		"whereDate", "wheredate", "orWhereDate", "orwheredate", "whereYear", "whereyear", "orWhereYear", "orwhereyear",
		"whereMonth", "wheremonth", "orWhereMonth", "orwheremonth", "whereDay", "whereday", "orWhereDay", "orwhereday",
		"whereTime", "wheretime", "orWhereTime", "orwheretime", "whereJsonContains", "wherejsoncontains", "orWhereJsonContains", "orwherejsoncontains",
		"join", "innerJoin", "leftJoin", "rightJoin", "crossJoin", "crossjoin",
		"get", "first", "firstOrFail", "firstofail", "firstWhere", "firstwhere", "sole", "find", "findMany", "findmany", "findOrFail", "findorfail",
		"value", "pluck", "exists", "doesntExist", "paginate", "chunk",
		"count", "sum", "avg", "min", "max",
		"insert", "insertGetId", "insertgetid", "update", "updateOrInsert", "updateorinsert", "upsert", "delete", "deleteAll", "truncate", "increment", "decrement", "touch",
		"orderBy", "orderby", "orderByDesc", "orderbydesc", "orderByAsc", "orderbyasc", "latest", "oldest", "inRandomOrder", "reorder", "limit", "take", "offset", "skip", "forPage", "forpage",
		"groupBy", "groupby", "having", "orHaving", "orhaving",
		"when", "unless", "transaction", "toSql", "tosql", "getBindings", "getbindings", "dump", "dd",
	}
	r.registerNative("GranDB", granDBMethods, (*Runtime).executeGranDBMethod)

	// Auth
	r.registerNative("Auth", []string{"hash", "complete2FA", "verify2FAChallenge", "login", "create", "attempt", "check", "verify", "forgotPassword", "resetPassword", "resendVerification", "verificationStatus", "user", "guest", "hasRole", "id", "refresh", "update", "delete", "logout", "validateToken"}, (*Runtime).executeAuthMethod)
	r.Variables["Auth"] = &Instance{Class: r.Classes["Auth"], Fields: make(map[string]interface{})}

	// AuthLoginResult
	r.registerNative("AuthLoginResult", []string{"require2FA", "onSuccess", "onChallenge", "onFail", "response"}, (*Runtime).executeAuthLoginResultMethod)

	// MFA
	r.registerNative("MFA", []string{"generateTOTP", "verifyTOTP", "generateRecoveryCodes", "verifyRecoveryCode"}, (*Runtime).executeMFAMethod)
	r.Variables["MFA"] = &Instance{Class: r.Classes["MFA"], Fields: make(map[string]interface{})}

	// TwoFactor
	r.registerNative("TwoFactor", []string{"verify", "required"}, (*Runtime).executeTwoFactorMethod)
	r.Variables["TwoFactor"] = &Instance{Class: r.Classes["TwoFactor"], Fields: make(map[string]interface{})}

	// System
	r.registerNative("System", []string{"env", "Run", "load_driver", "driver_call", "log", "sleep", "now"}, (*Runtime).executeSystemMethod)
	r.Variables["System"] = &Instance{Class: r.Classes["System"], Fields: make(map[string]interface{})}

	// Plugin (JP v2 plugin runtime bridge)
	r.registerNative("Plugin", []string{"call", "stream", "path", "platform"}, (*Runtime).executePluginMethod)
	r.Variables["Plugin"] = &Instance{Class: r.Classes["Plugin"], Fields: make(map[string]interface{})}

	// Cron
	r.registerNative("Cron", []string{"schedule"}, (*Runtime).executeCronMethod)
	r.Variables["Cron"] = &Instance{Class: r.Classes["Cron"], Fields: make(map[string]interface{})}

	// Task
	r.registerNative("Task", []string{"on_request"}, (*Runtime).executeTaskMethod)
	r.Variables["Task"] = &Instance{Class: r.Classes["Task"], Fields: make(map[string]interface{})}

	// View
	r.registerNative("View", []string{"render", "exists", "share"}, (*Runtime).executeViewMethod)
	r.Variables["View"] = &Instance{Class: r.Classes["View"], Fields: make(map[string]interface{})}

	// Router
	r.registerNative("Router", []string{"get", "post", "put", "patch", "delete", "head", "options", "query", "any", "match", "api", "ws", "group", "middleware", "registerMiddleware", "end"}, (*Runtime).executeRouterMethod)
	r.Variables["Router"] = &Instance{Class: r.Classes["Router"], Fields: make(map[string]interface{})}

	// Redirect (PHP-style convenience helper: Redirect::to("url", 302))
	r.registerNative("Redirect", []string{"to"}, (*Runtime).executeRedirectMethod)
	r.Variables["Redirect"] = &Instance{Class: r.Classes["Redirect"], Fields: make(map[string]interface{})}

	// Request
	r.registerNative("Request", []string{"input", "post", "all", "except", "file", "hasFile", "hasfile", "has", "cookie", "root", "header", "isMethod", "ismethod", "method", "path", "url", "ip", "userAgent", "useragent", "bearerToken", "bearertoken"}, (*Runtime).executeRequestMethod)
	r.Variables["Request"] = &Instance{Class: r.Classes["Request"], Fields: make(map[string]interface{})}

	// Response
	r.registerNative("Response", []string{"json", "error", "redirect", "back", "raw", "stream", "download"}, (*Runtime).executeResponseMethod)
	r.Variables["Response"] = &Instance{Class: r.Classes["Response"], Fields: make(map[string]interface{})}

	// WebResponse (replaces RedirectResponse)
	r.registerNative("WebResponse", []string{"with", "withCookie", "withHeader", "status"}, (*Runtime).executeWebResponseMethod)

	// WebSocket
	r.registerNative("WebSocket", []string{"broadcast", "send", "subscribe", "unsubscribe", "publish", "subscriberCount", "onMessage", "onClose", "close"}, (*Runtime).executeWebSocketMethod)
	r.Variables["WebSocket"] = &Instance{Class: r.Classes["WebSocket"], Fields: make(map[string]interface{})}

	// Schema
	r.registerNative("Schema", []string{"create", "table", "rename", "drop", "dropIfExists", "hasTable", "hasColumn"}, (*Runtime).executeSchemaMethod)
	r.Variables["Schema"] = &Instance{Class: r.Classes["Schema"], Fields: make(map[string]interface{})}

	// Blueprint
	blueprintMethods := []string{"id", "increments", "integer", "tinyInteger", "smallInteger", "mediumInteger", "bigInteger", "unsignedInteger", "unsignedBigInteger", "float", "double", "decimal", "char", "string", "text", "mediumText", "longText", "date", "dateTime", "time", "timestamp", "timestamps", "softDeletes", "boolean", "json", "enum", "nullable", "unsigned", "unique", "default", "comment", "dropColumn", "renameColumn", "index", "uniqueIndex", "dropIndex", "foreign", "references", "on", "onDelete", "onUpdate"}
	r.registerNative("Blueprint", blueprintMethods, (*Runtime).executeBlueprintMethod)

	// Redis
	r.registerNative("Redis", []string{"connect", "set", "get", "del", "has", "forget", "ttl", "flush"}, (*Runtime).executeRedisMethod)
	r.Variables["Redis"] = &Instance{Class: r.Classes["Redis"], Fields: make(map[string]interface{})}

	// Migration
	r.registerNative("Migration", []string{}, nil)

	// Middleware
	r.registerNative("Middleware", []string{}, nil)

	// Math
	r.registerNative("Math", []string{"random", "floor", "ceil", "abs"}, (*Runtime).executeMathMethod)
	r.Variables["Math"] = &Instance{Class: r.Classes["Math"], Fields: make(map[string]interface{})}

	// Session
	r.registerNative("Session", []string{"get", "put", "has", "forget", "all"}, (*Runtime).executeSessionMethod)
	r.Variables["Session"] = &Instance{Class: r.Classes["Session"], Fields: make(map[string]interface{})}

	// UUID
	r.registerNative("UUID", []string{"generate", "v4"}, (*Runtime).executeUUIDMethod)
	r.Variables["UUID"] = &Instance{Class: r.Classes["UUID"], Fields: make(map[string]interface{})}

	// Str
	r.registerNative("Str", []string{"length", "random", "startsWith", "substring", "indexOf", "contains", "trim", "replace"}, (*Runtime).executeStrMethod)
	r.Variables["Str"] = &Instance{Class: r.Classes["Str"], Fields: make(map[string]interface{})}

	// UserStorage
	r.registerNative("UserStorage", []string{"put", "get", "getToFile", "delete", "path"}, (*Runtime).executeUserStorageMethod)
	r.Variables["UserStorage"] = &Instance{Class: r.Classes["UserStorage"], Fields: make(map[string]interface{})}

	// SQLite (Native)
	r.registerNative("SQLite", []string{"open", "query", "close"}, (*Runtime).executeSQLiteMethod)
	r.Variables["SQLite"] = &Instance{Class: r.Classes["SQLite"], Fields: make(map[string]interface{})}

	// Zip (Native)
	r.registerNative("Zip", []string{"extract"}, (*Runtime).executeZipMethod)
	r.Variables["Zip"] = &Instance{Class: r.Classes["Zip"], Fields: make(map[string]interface{})}

	// JSON
	r.registerNative("JSON", []string{"parse", "stringify", "decode", "encode"}, (*Runtime).executeJSONMethod)
	r.Variables["JSON"] = &Instance{Class: r.Classes["JSON"], Fields: make(map[string]interface{})}

	// Markdown
	r.registerNative("Markdown", []string{"toHtml", "readFile"}, (*Runtime).executeMarkdownMethod)
	r.Variables["Markdown"] = &Instance{Class: r.Classes["Markdown"], Fields: make(map[string]interface{})}

	// Cache (Native)
	r.registerNative("Cache", []string{"put", "get", "has", "forget"}, (*Runtime).executeCacheMethod)
	r.Variables["Cache"] = &Instance{Class: r.Classes["Cache"], Fields: make(map[string]interface{})}

	// Http (General-purpose HTTP client)
	r.registerNative("Http", []string{"get", "post", "put", "patch", "delete", "head", "options", "json", "request"}, (*Runtime).executeHttpMethod)
	r.Variables["Http"] = &Instance{Class: r.Classes["Http"], Fields: make(map[string]interface{})}

	// SmtpClient (Native SMTP Engine)
	r.registerNative("SmtpClient", []string{"auth", "secure", "timeout", "send", "lastError"}, (*Runtime).executeSmtpClientMethod)
	r.Variables["SmtpClient"] = &Instance{Class: r.Classes["SmtpClient"], Fields: make(map[string]interface{})}

	// Stream (Native - Instantiated by Server)
	r.registerNative("Stream", []string{"send", "close"}, (*Runtime).executeStreamMethod)

	// Process (Native Execution)
	r.registerNative("Process", []string{"constructor", "start", "wait", "kill", "pid", "stdin", "stdout_chan", "stderr_chan"}, (*Runtime).executeProcessMethod)

	// Server Control
	r.registerNative("Server", []string{"start", "spawn"}, (*Runtime).executeServerControlMethod)
	r.Variables["Server"] = &Instance{Class: r.Classes["Server"], Fields: make(map[string]interface{})}

	// Lang (I18n)
	r.registerNative("Lang", []string{"get", "set", "locale", "locales"}, (*Runtime).executeLangMethod)
	r.Variables["Lang"] = &Instance{Class: r.Classes["Lang"], Fields: make(map[string]interface{})}

	// SEO
	r.registerNative("SEO", []string{"title", "description", "keywords", "og", "canonical", "meta", "render"}, (*Runtime).executeSEOMethod)
	r.Variables["SEO"] = &Instance{Class: r.Classes["SEO"], Fields: make(map[string]interface{})}

	// Sitemap
	r.registerNative("Sitemap", []string{"add", "provider", "exclude", "generate", "xsl"}, (*Runtime).executeSitemapMethod)
	r.Variables["Sitemap"] = &Instance{Class: r.Classes["Sitemap"], Fields: make(map[string]interface{})}

	// Console (Terminal styling & ANSI colors)
	consoleMethods := []string{"green", "red", "yellow", "blue", "cyan", "magenta", "gray", "bold", "clear", "color", "log"}
	r.registerNative("Console", consoleMethods, (*Runtime).executeConsoleMethod)
	r.Variables["Console"] = &Instance{Class: r.Classes["Console"], Fields: make(map[string]interface{})}

	// Exception
	r.registerNative("Exception", []string{"constructor", "getMessage", "getCode"}, (*Runtime).executeExceptionMethod)
}

func (r *Runtime) executeConsoleMethod(instance *Instance, method string, args []interface{}) interface{} {
	text := ""
	if len(args) > 0 {
		text = fmt.Sprint(args[0])
	}
	switch method {
	case "green":
		if len(args) == 0 {
			return "\033[32m"
		}
		return "\033[32m" + text + "\033[0m"
	case "red":
		if len(args) == 0 {
			return "\033[31m"
		}
		return "\033[31m" + text + "\033[0m"
	case "yellow":
		if len(args) == 0 {
			return "\033[33m"
		}
		return "\033[33m" + text + "\033[0m"
	case "blue":
		if len(args) == 0 {
			return "\033[34m"
		}
		return "\033[34m" + text + "\033[0m"
	case "cyan":
		if len(args) == 0 {
			return "\033[36m"
		}
		return "\033[36m" + text + "\033[0m"
	case "magenta":
		if len(args) == 0 {
			return "\033[35m"
		}
		return "\033[35m" + text + "\033[0m"
	case "gray":
		if len(args) == 0 {
			return "\033[90m"
		}
		return "\033[90m" + text + "\033[0m"
	case "bold":
		if len(args) == 0 {
			return "\033[1m"
		}
		return "\033[1m" + text + "\033[0m"
	case "clear":
		return "\033[2J\033[H"
	case "color":
		if len(args) >= 2 {
			code := fmt.Sprint(args[1])
			return "\033[" + code + "m" + text + "\033[0m"
		}
		return text
	case "log":
		fmt.Println(text)
		return nil
	}
	return text
}

func (r *Runtime) executeExceptionMethod(instance *Instance, method string, args []interface{}) interface{} {
	if instance == nil {
		return nil
	}
	switch method {
	case "constructor":
		if len(args) > 0 {
			instance.Fields["message"] = args[0]
		} else {
			instance.Fields["message"] = ""
		}
		if len(args) > 1 {
			instance.Fields["code"] = args[1]
		} else {
			instance.Fields["code"] = int64(0)
		}
		return instance
	case "getMessage":
		if msg, ok := instance.Fields["message"]; ok {
			return msg
		}
		return ""
	case "getCode":
		if code, ok := instance.Fields["code"]; ok {
			return code
		}
		return int64(0)
	default:
		return nil
	}
}

func (r *Runtime) executeNativeMethod(instance *Instance, method string, args []interface{}) interface{} {
	// Traverse class hierarchy (bottom-up)
	currentClass := instance.Class
	for currentClass != nil {
		className := currentClass.Name.Value

		// Optimize: Check simple map lookup
		if handler, ok := r.NativeHandlers[className]; ok {
			if handler != nil {
				// PASS 'r' (the current runtime) as the first argument
				return handler(r, instance, method, args)
			}
		}

		// Move to parent
		if currentClass.SuperClass != nil {
			if parent, ok := r.Classes[currentClass.SuperClass.Value]; ok {
				currentClass = parent
			} else {
				break
			}
		} else {
			break
		}
	}
	return nil
}
