package native_test

import (
	"strings"
	"testing"

	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/ir"
)

func TestFileGetBytesBuiltinEmitsForSupportedTargets(t *testing.T) {
	targets := []backend.Target{
		{OS: "linux", Arch: "amd64"},
		{OS: "darwin", Arch: "amd64"},
		{OS: "windows", Arch: "amd64"},
		{OS: "linux", Arch: "arm64"},
		{OS: "darwin", Arch: "arm64"},
		{OS: "windows", Arch: "arm64"},
	}
	for _, target := range targets {
		t.Run(target.DashString(), func(t *testing.T) {
			prog := ir.NewProgram("file_bytes")
			main := ir.NewFunction("main", ir.TypeI64)
			bytes := main.NewTemp(ir.ArrayType(ir.TypeI64), "bytes")
			main.EntryBlock.AddInstruction(&ir.CallBuiltinInst{
				Dest:      bytes,
				BuiltinID: "__joss_file_get_bytes",
				Args:      []ir.Value{ir.NewConstString("scratch/tzif-test")},
			})
			main.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
			prog.AddFunction(main)

			emitter := backend.SelectCodeEmitter(target, false)
			binary, _, err := emitter.Compile(prog)
			if err != nil {
				t.Fatalf("Compile(%s): %v", target, err)
			}
			if len(binary) == 0 {
				t.Fatal("compiler emitted an empty executable")
			}
			if !strings.Contains(prog.Dump(), "__joss_file_get_bytes") {
				t.Fatal("IR lost the byte-reader builtin")
			}
		})
	}
}
