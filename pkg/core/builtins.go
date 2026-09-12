package core

import (
	"sync"

	"github.com/jossecurity/joss/pkg/typesystem"
)

type builtinDomain uint8

const (
	builtinCollections builtinDomain = iota
	builtinAsync
	builtinDate
	builtinIO
	builtinSerialization
	builtinString
)

type builtinDefinition struct {
	name       string
	domain     builtinDomain
	returnType typesystem.Type
}

func builtinGroup(domain builtinDomain, returnKind typesystem.Kind, names ...string) []builtinDefinition {
	definitions := make([]builtinDefinition, 0, len(names))
	for _, name := range names {
		definitions = append(definitions, builtinDefinition{name: name, domain: domain, returnType: typesystem.Type{Kind: returnKind}})
	}
	return definitions
}

func joinBuiltinGroups(groups ...[]builtinDefinition) []builtinDefinition {
	var definitions []builtinDefinition
	for _, group := range groups {
		definitions = append(definitions, group...)
	}
	return definitions
}

// builtinDefinitions is the canonical global-function catalog. Each name is
// declared once together with the metadata needed by runtime dispatch and
// semantic analysis.
var builtinDefinitions = joinBuiltinGroups(
	builtinGroup(builtinCollections, typesystem.Bool,
		"isset", "empty", "is_string", "is_numeric", "is_int", "is_integer", "is_float", "is_double", "is_decimal",
		"is_array", "is_null", "in_array", "array_key_exists", "any", "all"),
	builtinGroup(builtinCollections, typesystem.Int, "intval", "boolval", "len", "count"),
	builtinGroup(builtinCollections, typesystem.Float, "floatval", "doubleval"),
	builtinGroup(builtinCollections, typesystem.Decimal, "decimal"),
	builtinGroup(builtinCollections, typesystem.String, "strval"),
	builtinGroup(builtinCollections, typesystem.Array,
		"keys", "array_keys", "values", "array_values", "explode", "merge", "array_merge", "array_slice",
		"array_unique", "array_reverse", "array_column", "map", "filter"),
	builtinGroup(builtinCollections, typesystem.Mixed,
		"end", "append", "array_push", "array_pop", "array_shift", "reduce", "find", "sum"),

	builtinGroup(builtinAsync, typesystem.Mixed, "async", "await", "make_chan", "close", "send", "recv"),

	builtinGroup(builtinDate, typesystem.Int, "time"),
	builtinGroup(builtinDate, typesystem.Float, "microtime"),
	builtinGroup(builtinDate, typesystem.String, "date"),
	builtinGroup(builtinDate, typesystem.Mixed, "strtotime", "now", "sleep", "usleep"),

	builtinGroup(builtinIO, typesystem.Bool, "file_exists", "is_dir", "is_file", "toon_verify"),
	builtinGroup(builtinIO, typesystem.String, "env", "config", "view", "json", "toon_encode"),
	builtinGroup(builtinIO, typesystem.Mixed,
		"back", "response", "request", "session", "redirect", "file_get_contents", "file_put_contents", "unlink",
		"file_delete", "mkdir", "toon_decode", "hive_read_box", "run"),

	builtinGroup(builtinSerialization, typesystem.Bool, "json_verify"),
	builtinGroup(builtinSerialization, typesystem.String, "json_encode"),
	builtinGroup(builtinSerialization, typesystem.Mixed, "json_decode"),

	builtinGroup(builtinString, typesystem.Bool,
		"str_contains", "contains", "str_starts_with", "starts_with", "str_ends_with", "ends_with"),
	builtinGroup(builtinString, typesystem.Int, "strlen", "strpos", "rand"),
	builtinGroup(builtinString, typesystem.Float, "round", "floor", "ceil", "abs"),
	builtinGroup(builtinString, typesystem.String,
		"html_escape", "__", "csrf_field", "str_replace", "strtolower", "to_lower", "strtoupper", "to_upper",
		"trim", "ltrim", "rtrim", "substr", "implode", "join", "md5", "sha1", "sha256", "base64_encode",
		"base64_decode", "ucfirst", "lcfirst", "ucwords", "str_pad", "str_repeat"),
	builtinGroup(builtinString, typesystem.Mixed, "print", "echo", "cout", "cerr", "printf", "min", "max"),
)

var (
	builtinNamesOnce sync.Once
	builtinNamesMap  map[string]builtinDefinition
)

func initBuiltinMap() {
	builtinNamesOnce.Do(func() {
		builtinNamesMap = make(map[string]builtinDefinition, len(builtinDefinitions))
		for _, definition := range builtinDefinitions {
			if _, duplicate := builtinNamesMap[definition.name]; duplicate {
				panic("duplicate builtin definition: " + definition.name)
			}
			builtinNamesMap[definition.name] = definition
		}
	})
}

func builtinDefinitionFor(name string) (builtinDefinition, bool) {
	initBuiltinMap()
	definition, exists := builtinNamesMap[name]
	return definition, exists
}

// IsBuiltin returns true if the function name is a core built-in function in Joss.
func IsBuiltin(name string) bool {
	_, exists := builtinDefinitionFor(name)
	return exists
}

// GetBuiltinFunctionNames returns a copy of all built-in function names.
func GetBuiltinFunctionNames() []string {
	result := make([]string, 0, len(builtinDefinitions))
	for _, definition := range builtinDefinitions {
		result = append(result, definition.name)
	}
	return result
}
