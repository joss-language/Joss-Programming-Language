package native

import (
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/ir"
)

func capabilityProbe(className, methodName string) *ir.Program {
	program := ir.NewProgram("capability_probe")
	function := ir.NewFunction("main", ir.TypeI64)
	function.EntryBlock.AddInstruction(&ir.CallHostServiceInst{
		ClassName:  className,
		MethodName: methodName,
	})
	program.AddFunction(function)
	return program
}

func TestValidateAOTCapabilitiesUsesNativeEmitterPolicy(t *testing.T) {
	for _, target := range []Target{
		{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"}, {OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"}, {OS: "darwin", Arch: "arm64"},
		{OS: "android", Arch: "arm64"},
	} {
		if err := ValidateAOTCapabilities(capabilityProbe("Request", "input"), target); err != nil {
			t.Errorf("Request::input should pass AOT preflight for %s: %v", target.DashString(), err)
		}
		if err := ValidateAOTCapabilities(capabilityProbe("Auth", "login"), target); err == nil || !strings.Contains(err.Error(), "Auth::login") {
			t.Errorf("Auth::login without a Joss owner must fail AOT preflight for %s; got %v", target.DashString(), err)
		}
	}
}
