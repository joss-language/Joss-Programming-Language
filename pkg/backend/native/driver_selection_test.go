package native

import (
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/ir"
)

func TestProgramRequiresC11WebRuntimeOnlyForHTTPDispatch(t *testing.T) {
	console := ir.NewProgram("console")
	console.AddFunction(ir.NewFunction("main", ir.TypeI64))
	if programRequiresC11WebRuntime(console) {
		t.Fatal("a console program must remain eligible for the integrated backend")
	}

	web := ir.NewProgram("web")
	web.AddFunction(ir.NewFunction("__joss_http_dispatch", ir.TypeString))
	if !programRequiresC11WebRuntime(web) {
		t.Fatal("an HTTP dispatch program must require the C11 web runtime")
	}
}

func TestNativeBackendRejectsHTTPDispatchBeforeEmission(t *testing.T) {
	web := ir.NewProgram("web")
	dispatch := ir.NewFunction("__joss_http_dispatch", ir.TypeString)
	dispatch.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstString("")})
	web.AddFunction(dispatch)

	_, err := BuildProgram(web, BuildOptions{
		Target:  Target{OS: "linux", Arch: "amd64"},
		Backend: BackendNative,
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP runtime C11") {
		t.Fatalf("BackendNative must reject HTTP dispatch instead of emitting a partial server; got %v", err)
	}
}
