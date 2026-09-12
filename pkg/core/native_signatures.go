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
	ArityKnown bool
	Variadic   bool
}

func nativeMethod(name, returnName string) NativeMethodDefinition {
	return NativeMethodDefinition{Name: name, ReturnType: typesystem.Parse(returnName)}
}

var migratedNativeMethods = map[string][]NativeMethodDefinition{
	"Stack": {
		nativeMethod("push", "mixed"), nativeMethod("pop", "mixed"), nativeMethod("peek", "mixed"),
	},
	"Queue": {
		nativeMethod("enqueue", "mixed"), nativeMethod("dequeue", "mixed"), nativeMethod("peek", "mixed"),
	},
	"Math": {
		nativeMethod("random", "int"), nativeMethod("floor", "float"), nativeMethod("ceil", "float"), nativeMethod("abs", "float"),
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
	"Console::green":             "string",
	"Console::red":               "string",
	"Console::yellow":            "string",
	"Console::blue":              "string",
	"Console::cyan":              "string",
	"Console::magenta":           "string",
	"Console::gray":              "string",
	"Console::bold":              "string",
	"Console::clear":             "string",
	"Console::color":             "string",
	"Console::log":               "void",
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
	"JSON::encode":               "string",
	"JSON::stringify":            "string",
	"Lang::locale":               "string",
	"Lang::locales":              "array",
	"Markdown::readFile":         "string",
	"Markdown::toHtml":           "string",
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
	"Str::contains":              "bool",
	"Str::indexOf":               "int",
	"Str::length":                "int",
	"Str::random":                "string",
	"Str::replace":               "string",
	"Str::startsWith":            "bool",
	"Str::substring":             "string",
	"Str::trim":                  "string",
	"System::now":                "int",
	"Sitemap::generate":          "string",
	"SmtpClient::lastError":      "string|null",
	"SmtpClient::send":           "bool",
	"TwoFactor::required":        "bool",
	"TwoFactor::verify":          "bool",
	"UUID::generate":             "string",
	"UUID::v4":                   "string",
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
