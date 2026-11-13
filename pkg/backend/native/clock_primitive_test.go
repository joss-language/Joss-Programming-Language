package native_test

import (
	"strings"
	"testing"

	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/ir"
)

func TestX8664_TimeBuiltinHasPerOSClockPrimitive(t *testing.T) {
	prog := ir.NewProgram("time_x86_64")
	fn := ir.NewFunction("main", ir.TypeI64)
	seconds := fn.NewTemp(ir.TypeI64, "seconds")
	fn.EntryBlock.AddInstruction(&ir.CallBuiltinInst{Dest: seconds, BuiltinID: "time"})
	fn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: seconds})
	prog.AddFunction(fn)

	targets := []struct {
		target backend.Target
		want   string
	}{
		{backend.Target{OS: "linux", Arch: "amd64"}, "CLOCK_REALTIME via clock_gettime syscall"},
		{backend.Target{OS: "darwin", Arch: "amd64"}, "gettimeofday syscall"},
		{backend.Target{OS: "windows", Arch: "amd64"}, "Windows KUSER_SHARED_DATA SystemTime"},
	}
	for _, tc := range targets {
		t.Run(tc.target.DashString(), func(t *testing.T) {
			builder := backend.NewMachineCodeBuilder(tc.target, false)
			binary, listing, err := builder.Compile(prog)
			if err != nil {
				t.Fatalf("Compile(%s) failed: %v", tc.target, err)
			}
			if len(binary) == 0 || !strings.Contains(listing, "fn_time:") || !strings.Contains(listing, tc.want) {
				t.Fatalf("%s did not emit its clock primitive", tc.target)
			}
		})
	}
}
