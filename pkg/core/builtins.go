package core

import "sync"

var (
	builtinNamesOnce sync.Once
	builtinNamesMap  map[string]bool
)

// builtinList is the canonical public builtin catalog. Runtime dispatch and
// static analysis both consult this list; adding a builtin requires adding its
// implementation and its name here.
var builtinList = []string{
	// Arrays, maps and conversion.
	"isset", "empty", "is_string", "is_numeric", "is_int", "is_integer", "is_float", "is_double", "is_decimal",
	"intval", "floatval", "doubleval", "decimal", "strval", "boolval", "is_array", "is_null", "len", "count",
	"keys", "array_keys", "values", "array_values", "explode", "end", "append", "merge", "in_array",
	"array_key_exists", "array_merge", "array_push", "array_pop", "array_shift", "array_slice", "array_unique",
	"array_reverse", "array_column", "map", "filter", "reduce", "find", "any", "all", "sum",
	// Async and channels.
	"async", "await", "make_chan", "close", "send", "recv",
	// Date and time.
	"time", "microtime", "date", "strtotime", "now", "sleep", "usleep",
	// IO, framework helpers and serialization.
	"env", "config", "view", "json", "back", "response", "request", "session", "redirect", "file_exists",
	"file_get_contents", "file_put_contents", "unlink", "file_delete", "mkdir", "is_dir", "is_file",
	"toon_encode", "toon_decode", "toon_verify", "json_encode", "json_decode", "json_verify", "hive_read_box", "run",
	// Strings, formatting, hashing and numeric helpers.
	"html_escape", "__", "csrf_field", "print", "echo", "cout", "cerr", "printf", "str_contains", "contains",
	"str_starts_with", "starts_with", "str_ends_with", "ends_with", "str_replace", "strtolower", "to_lower",
	"strtoupper", "to_upper", "trim", "ltrim", "rtrim", "substr", "strpos", "implode", "join", "md5", "sha1",
	"sha256", "base64_encode", "base64_decode", "strlen", "ucfirst", "lcfirst", "ucwords", "str_pad",
	"str_repeat", "round", "floor", "ceil", "abs", "min", "max", "rand",
}

func initBuiltinMap() {
	builtinNamesOnce.Do(func() {
		builtinNamesMap = make(map[string]bool, len(builtinList))
		for _, name := range builtinList {
			builtinNamesMap[name] = true
		}
	})
}

// IsBuiltin returns true if the function name is a core built-in function in Joss.
func IsBuiltin(name string) bool {
	initBuiltinMap()
	return builtinNamesMap[name]
}

// GetBuiltinFunctionNames returns a list of all built-in function names.
func GetBuiltinFunctionNames() []string {
	initBuiltinMap()
	result := make([]string, len(builtinList))
	copy(result, builtinList)
	return result
}
