package native

import (
	"strings"

	"github.com/jossecurity/joss/pkg/ir"
)

func hasProgramFunction(program *ir.Program, name string) bool {
	if program == nil {
		return false
	}
	name = strings.TrimPrefix(name, "fn_")
	return program.Functions[name] != nil
}

func isProductShimWithoutJossOwner(name string) bool {
	name = strings.TrimPrefix(name, "fn_")
	return strings.HasPrefix(name, "Auth_") ||
		strings.HasPrefix(name, "SEO_") ||
		strings.HasPrefix(name, "Turnstile_")
}

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

func hasJossHostServiceImplementation(program *ir.Program, className, methodName string) bool {
	if programHasJossHostService(program, className, methodName) {
		return true
	}
	if program != nil && program.Functions["__joss_host_"+className+"_"+methodName] != nil {
		return true
	}
	// Legacy Server::start delegates to the canonical Pure Joss HTTP server.
	return className == "Server" && methodName == "start" && programHasJossHostService(program, "HttpServer", "start")
}

func jossHostServiceFunctionName(program *ir.Program, className, methodName string) string {
	if program != nil {
		owner := "__joss_host_" + className + "_" + methodName
		if program.Functions[owner] != nil {
			return owner
		}
	}
	return className + "_" + methodName
}

// isUnimplementedFrameworkTarget identifies class-method-shaped framework
// references that have no source or native implementation. They must fail
// capability validation; emitting a pass-through/no-op stub hides missing work.
func isUnimplementedFrameworkTarget(label string) bool {
	return strings.HasPrefix(label, "fn_Router_") ||
		strings.HasPrefix(label, "fn_Schema_") ||
		strings.HasPrefix(label, "fn_Plugin_") ||
		strings.HasPrefix(label, "fn_GranDB_") ||
		strings.HasPrefix(label, "fn_HttpServer_")
}
