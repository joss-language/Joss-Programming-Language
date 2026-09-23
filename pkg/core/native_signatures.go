package core

import "github.com/jossecurity/joss/pkg/typesystem"

// NativeParameterDefinition contains only a contract that the native runtime
// can state reliably. Runtime implementation details remain in NativeHandler.
type NativeParameterDefinition struct {
	Name        string
	Type        typesystem.Type
	HasDefault  bool
	ByReference bool
}

// NativeMethodDefinition is the canonical semantic metadata for a native
// method. ArityKnown distinguishes an intentionally unknown signature from a
// method that reliably accepts zero parameters.
type NativeMethodDefinition struct {
	Name       string
	ReturnType typesystem.Type
	Parameters []NativeParameterDefinition
	Effects    []string
	ArityKnown bool
	Variadic   bool
}

func withNativeEffects(definition NativeMethodDefinition, effects ...string) NativeMethodDefinition {
	definition.Effects = append([]string(nil), effects...)
	return definition
}

func nativeMethod(name, returnName string) NativeMethodDefinition {
	return NativeMethodDefinition{Name: name, ReturnType: typesystem.Parse(returnName)}
}

func nativeMethodWithArity(name, returnName string, params ...NativeParameterDefinition) NativeMethodDefinition {
	return NativeMethodDefinition{
		Name:       name,
		ReturnType: typesystem.Parse(returnName),
		Parameters: params,
		ArityKnown: true,
	}
}

func nativeParam(name, typeName string) NativeParameterDefinition {
	return NativeParameterDefinition{
		Name: name,
		Type: typesystem.Parse(typeName),
	}
}

func nativeOptionalParam(name, typeName string) NativeParameterDefinition {
	parameter := nativeParam(name, typeName)
	parameter.HasDefault = true
	return parameter
}

var migratedNativeMethods = map[string][]NativeMethodDefinition{
	"Stack": {
		nativeMethodWithArity("push", "mixed", nativeParam("item", "mixed")),
		nativeMethodWithArity("pop", "mixed"),
		nativeMethodWithArity("peek", "mixed"),
	},
	"Queue": {
		nativeMethodWithArity("enqueue", "mixed", nativeParam("item", "mixed")),
		nativeMethodWithArity("dequeue", "mixed"),
		nativeMethodWithArity("peek", "mixed"),
	},
	"Math": {
		nativeMethodWithArity("random", "int", nativeParam("min", "int"), nativeParam("max", "int")),
		nativeMethodWithArity("floor", "float", nativeParam("val", "float")),
		nativeMethodWithArity("ceil", "float", nativeParam("val", "float")),
		nativeMethodWithArity("abs", "mixed", nativeParam("val", "mixed")),
	},
	"JSON": {
		nativeMethodWithArity("parse", "mixed", nativeParam("json", "string")),
		nativeMethodWithArity("stringify", "string", nativeParam("value", "mixed")),
		nativeMethodWithArity("decode", "mixed", nativeParam("json", "string")),
		nativeMethodWithArity("encode", "string", nativeParam("value", "mixed")),
	},
	"Markdown": {
		nativeMethodWithArity("toHtml", "string", nativeParam("markdown", "string")),
		withNativeEffects(nativeMethodWithArity("readFile", "string", nativeParam("path", "string")), "fs", "io", "blocking"),
	},
	"Str": {
		nativeMethodWithArity("length", "int", nativeParam("str", "string")),
		nativeMethodWithArity("random", "string", nativeOptionalParam("length", "int")),
		nativeMethodWithArity("startsWith", "bool", nativeParam("haystack", "string"), nativeParam("needle", "string")),
		nativeMethodWithArity("endsWith", "bool", nativeParam("haystack", "string"), nativeParam("needle", "string")),
		nativeMethodWithArity("substring", "string", nativeParam("str", "string"), nativeParam("start", "int"), nativeOptionalParam("length", "int")),
		nativeMethodWithArity("substr", "string", nativeParam("str", "string"), nativeParam("start", "int"), nativeOptionalParam("length", "int")),
		nativeMethodWithArity("lower", "string", nativeParam("str", "string")),
		nativeMethodWithArity("indexOf", "int", nativeParam("haystack", "string"), nativeParam("needle", "string")),
		nativeMethodWithArity("contains", "bool", nativeParam("haystack", "string"), nativeParam("needle", "string")),
		nativeMethodWithArity("trim", "string", nativeParam("str", "string")),
		nativeMethodWithArity("replace", "string", nativeParam("search", "string"), nativeParam("replace", "string"), nativeParam("subject", "string")),
	},
	"UUID": {
		nativeMethodWithArity("generate", "string"),
		nativeMethodWithArity("v4", "string"),
	},
	"Lang": {
		nativeMethodWithArity("get", "mixed", nativeParam("key", "string"), nativeOptionalParam("replacements", "map")),
		nativeMethodWithArity("set", "bool", nativeParam("locale", "string")),
		nativeMethodWithArity("locale", "string"),
		nativeMethodWithArity("locales", "array"),
	},
	"Console": {
		nativeMethodWithArity("green", "string", nativeOptionalParam("text", "mixed")),
		nativeMethodWithArity("red", "string", nativeOptionalParam("text", "mixed")),
		nativeMethodWithArity("yellow", "string", nativeOptionalParam("text", "mixed")),
		nativeMethodWithArity("blue", "string", nativeOptionalParam("text", "mixed")),
		nativeMethodWithArity("cyan", "string", nativeOptionalParam("text", "mixed")),
		nativeMethodWithArity("magenta", "string", nativeOptionalParam("text", "mixed")),
		nativeMethodWithArity("gray", "string", nativeOptionalParam("text", "mixed")),
		nativeMethodWithArity("bold", "string", nativeOptionalParam("text", "mixed")),
		nativeMethodWithArity("clear", "string"),
		nativeMethodWithArity("color", "string", nativeParam("text", "mixed"), nativeParam("code", "mixed")),
		nativeMethodWithArity("log", "void", nativeOptionalParam("value", "mixed")),
	},
	"Zip": {
		withNativeEffects(nativeMethodWithArity("extract", "bool", nativeParam("src", "string"), nativeParam("dest", "string")), "fs", "io", "blocking"),
	},
	"FileStream": {
		withNativeEffects(nativeMethodWithArity("open", "FileStream", nativeParam("path", "string"), nativeParam("mode", "string")), "fs", "io", "blocking"),
		withNativeEffects(nativeMethodWithArity("read", "string", nativeParam("length", "int")), "io", "blocking"),
		withNativeEffects(nativeMethodWithArity("write", "int", nativeParam("data", "string")), "io", "blocking"),
		nativeMethodWithArity("seek", "int", nativeParam("offset", "int"), nativeParam("whence", "int")),
		nativeMethodWithArity("size", "int"),
		nativeMethodWithArity("flush", "void"),
		nativeMethodWithArity("close", "void"),
	},
	"StreamReader": {
		nativeMethodWithArity("open", "StreamReader", nativeParam("source", "mixed")),
		nativeMethodWithArity("readLine", "string|null"),
		nativeMethodWithArity("readToEnd", "string"),
		nativeMethodWithArity("close", "void"),
	},
	"StreamWriter": {
		nativeMethodWithArity("open", "StreamWriter", nativeParam("destination", "mixed"), nativeParam("append", "bool")),
		nativeMethodWithArity("write", "int", nativeParam("text", "string")),
		nativeMethodWithArity("writeLine", "int", nativeParam("text", "string")),
		nativeMethodWithArity("flush", "void"),
		nativeMethodWithArity("close", "void"),
	},
	"Socket": {
		withNativeEffects(nativeMethodWithArity("tcp", "Socket", nativeParam("host", "string"), nativeParam("port", "string")), "network", "io", "blocking"),
		withNativeEffects(nativeMethodWithArity("connect", "Socket", nativeParam("host", "string"), nativeParam("port", "string")), "network", "io", "blocking"),
		withNativeEffects(nativeMethodWithArity("listen", "Socket", nativeParam("host", "string"), nativeParam("port", "string")), "network", "io", "blocking"),
		withNativeEffects(nativeMethodWithArity("accept", "Socket"), "network", "io", "blocking"),
		withNativeEffects(nativeMethodWithArity("send", "int", nativeParam("data", "string")), "network", "io", "blocking"),
		withNativeEffects(nativeMethodWithArity("receive", "string", nativeParam("maxBytes", "int")), "network", "io", "blocking"),
		nativeMethodWithArity("port", "int"),
		nativeMethodWithArity("close", "void"),
	},
	"DateTime": {
		nativeMethodWithArity("now", "DateTime"),
		nativeMethodWithArity("create", "DateTime", nativeParam("year", "int"), nativeParam("month", "int"), nativeParam("day", "int")),
		nativeMethodWithArity("parse", "DateTime", nativeParam("str", "string")),
		nativeMethodWithArity("format", "string", nativeParam("layout", "string")),
		nativeMethodWithArity("timestamp", "int"),
		nativeMethodWithArity("iso", "string"),
		nativeMethodWithArity("year", "int"),
		nativeMethodWithArity("month", "int"),
		nativeMethodWithArity("day", "int"),
		nativeMethodWithArity("hour", "int"),
		nativeMethodWithArity("minute", "int"),
		nativeMethodWithArity("second", "int"),
		nativeMethodWithArity("add", "DateTime", nativeParam("interval", "mixed")),
		nativeMethodWithArity("sub", "DateTime", nativeParam("interval", "mixed")),
		nativeMethodWithArity("diff", "DateInterval", nativeParam("other", "DateTime")),
	},
	"DateInterval": {
		nativeMethodWithArity("days", "DateInterval", nativeParam("n", "int")),
		nativeMethodWithArity("hours", "DateInterval", nativeParam("n", "int")),
		nativeMethodWithArity("minutes", "DateInterval", nativeParam("n", "int")),
		nativeMethodWithArity("seconds", "DateInterval", nativeParam("n", "int")),
		nativeMethodWithArity("totalSeconds", "int"),
	},
	"Mutex": {
		nativeMethodWithArity("lock", "void"),
		nativeMethodWithArity("unlock", "void"),
		nativeMethodWithArity("tryLock", "bool"),
	},
	"RWMutex": {
		nativeMethodWithArity("lock", "void"),
		nativeMethodWithArity("unlock", "void"),
		nativeMethodWithArity("rLock", "void"),
		nativeMethodWithArity("rUnlock", "void"),
	},
	"WaitGroup": {
		nativeMethodWithArity("add", "void", nativeParam("delta", "int")),
		nativeMethodWithArity("done", "void"),
		nativeMethodWithArity("wait", "void"),
	},
}

// nativeMethodReturnType is the return-signature source of truth for core
// classes. Every registered method receives a declared type; mixed means the
// API is intentionally value-polymorphic, never that metadata is missing.
func nativeMethodReturnType(className, methodName string) typesystem.Type {
	if returnName, exists := preciseNativeReturns[className+"::"+methodName]; exists {
		return typesystem.Parse(returnName)
	}
	if fluentNativeClasses[className] || fluentNativeMethods[className+"::"+methodName] {
		return typesystem.Type{Kind: typesystem.Class, Name: className}
	}
	return typesystem.Type{Kind: typesystem.Mixed}
}

var fluentNativeClasses = map[string]bool{
	"Blueprint":   true,
	"WebResponse": true,
}

var fluentNativeMethods = nameSet(
	"AuthLoginResult::require2FA", "AuthLoginResult::onSuccess", "AuthLoginResult::onChallenge", "AuthLoginResult::onFail",
	"GranDB::table", "GranDB::select", "GranDB::changeDB", "GranDB::changedb", "GranDB::connection", "GranDB::use", "GranDB::distinct",
	"GranDB::where", "GranDB::orWhere", "GranDB::orwhere", "GranDB::whereLike", "GranDB::wherelike",
	"GranDB::orWhereLike", "GranDB::orwherelike", "GranDB::whereColumn", "GranDB::wherecolumn", "GranDB::orWhereColumn", "GranDB::orwherecolumn",
	"GranDB::whereNot", "GranDB::wherenot", "GranDB::orWhereNot", "GranDB::orwherenot", "GranDB::whereIn", "GranDB::wherein",
	"GranDB::orWhereIn", "GranDB::orwherein", "GranDB::whereNotIn", "GranDB::wherenotin", "GranDB::orWhereNotIn", "GranDB::orwherenotin",
	"GranDB::whereNull", "GranDB::wherenull", "GranDB::orWhereNull", "GranDB::orwherenull", "GranDB::whereNotNull", "GranDB::wherenotnull",
	"GranDB::orWhereNotNull", "GranDB::orwherenotnull", "GranDB::whereBetween", "GranDB::wherebetween", "GranDB::orWhereBetween", "GranDB::orwherebetween",
	"GranDB::whereNotBetween", "GranDB::wherenotbetween", "GranDB::orWhereNotBetween", "GranDB::orwherenotbetween", "GranDB::join", "GranDB::innerJoin",
	"GranDB::leftJoin", "GranDB::rightJoin", "GranDB::crossJoin", "GranDB::crossjoin", "GranDB::orderBy", "GranDB::orderby", "GranDB::orderByDesc", "GranDB::orderbydesc", "GranDB::orderByAsc",
	"GranDB::orderbyasc", "GranDB::latest", "GranDB::oldest", "GranDB::inRandomOrder", "GranDB::reorder", "GranDB::limit", "GranDB::take",
	"GranDB::offset", "GranDB::skip", "GranDB::forPage", "GranDB::forpage", "GranDB::groupBy", "GranDB::groupby", "GranDB::having",
	"GranDB::orHaving", "GranDB::orhaving", "GranDB::when", "GranDB::unless",
	"SEO::title", "SEO::description", "SEO::keywords", "SEO::og", "SEO::canonical", "SEO::meta",
	"Sitemap::add", "Sitemap::provider", "Sitemap::exclude", "Sitemap::xsl",
	"SmtpClient::auth", "SmtpClient::secure", "SmtpClient::timeout",
)

var preciseNativeReturns = map[string]string{
	"Auth::check":                "bool",
	"Auth::guest":                "bool",
	"Auth::hasRole":              "bool",
	"Auth::verify":               "bool",
	"Cache::has":                 "bool",
	"Exception::getCode":         "int",
	"Exception::getMessage":      "string",
	"GranDB::avg":                "float|null",
	"GranDB::count":              "int",
	"GranDB::doesntExist":        "bool",
	"GranDB::exists":             "bool",
	"GranDB::get":                "array",
	"GranDB::getBindings":        "array",
	"GranDB::getbindings":        "array",
	"GranDB::max":                "mixed",
	"GranDB::min":                "mixed",
	"GranDB::pluck":              "array",
	"GranDB::sum":                "float",
	"GranDB::toSql":              "string",
	"GranDB::tosql":              "string",
	"MFA::verifyRecoveryCode":    "bool",
	"MFA::verifyTOTP":            "bool",
	"Redirect::to":               "WebResponse",
	"Request::all":               "map",
	"Request::except":            "map",
	"Request::has":               "bool",
	"Request::hasFile":           "bool",
	"Request::hasfile":           "bool",
	"Request::isMethod":          "bool",
	"Request::ismethod":          "bool",
	"Request::method":            "string",
	"Request::path":              "string",
	"Request::root":              "string",
	"Request::url":               "string",
	"Request::uri":               "string",
	"Response::back":             "WebResponse",
	"Response::download":         "WebResponse",
	"Response::error":            "WebResponse",
	"Response::json":             "WebResponse",
	"Response::raw":              "WebResponse",
	"Response::redirect":         "WebResponse",
	"Response::stream":           "WebResponse",
	"Schema::hasColumn":          "bool",
	"Schema::hasTable":           "bool",
	"SEO::render":                "string",
	"Session::all":               "map",
	"Session::has":               "bool",
	"System::now":                "int",
	"IndexNow::enabled":          "bool",
	"IndexNow::key":              "string",
	"IndexNow::keyLocation":      "string",
	"IndexNow::submit":           "bool",
	"Sitemap::generate":          "string",
	"SmtpClient::lastError":      "string|null",
	"SmtpClient::send":           "bool",
	"TwoFactor::required":        "bool",
	"TwoFactor::verify":          "bool",
	"View::exists":               "bool",
	"View::render":               "string",
	"WebSocket::subscriberCount": "int",
}

func nameSet(names ...string) map[string]bool {
	result := make(map[string]bool, len(names))
	for _, name := range names {
		result[name] = true
	}
	return result
}
