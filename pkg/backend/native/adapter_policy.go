package native

import (
	"strings"

	"github.com/jossecurity/joss/pkg/ir"
)

func programHasJossHostService(program *ir.Program, className, methodName string) bool {
	if program == nil {
		return false
	}
	for _, name := range []string{className + "_" + methodName, "fn_" + className + "_" + methodName} {
		if program.Functions[name] != nil {
			return true
		}
	}
	return false
}

// isFrameworkAdapterTarget limits pass-through adapters to class-method-shaped
// framework calls. A missing global runtime symbol (for example fn_strpos) must
// stay unresolved so compilation fails instead of silently emitting a no-op.
func isFrameworkAdapterTarget(label string) bool {
	return strings.HasPrefix(label, "fn_Router_") ||
		strings.HasPrefix(label, "fn_Schema_") ||
		strings.HasPrefix(label, "fn_Plugin_") ||
		strings.HasPrefix(label, "fn_GranDB_") ||
		strings.HasPrefix(label, "fn_HttpServer_")
}
