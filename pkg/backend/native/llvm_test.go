package native_test

import (
	"strings"
	"testing"

	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/ir"
)

func TestLLVM_EmptyMain(t *testing.T) {
	prog := ir.NewProgram("empty_main")
	fn := ir.NewFunction("main", ir.TypeI64)
	retVal := ir.NewConstInt(0, ir.TypeI64)
	fn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: retVal})
	prog.AddFunction(fn)

	emitter := backend.NewLLVMEmitter()
	ll, err := emitter.Emit(prog)
	if err != nil {
		t.Fatalf("failed to emit LLVM IR: %v", err)
	}

	if !strings.Contains(ll, "define i64 @main()") {
		t.Errorf("expected define i64 @main(), got:\n%s", ll)
	}
	if !strings.Contains(ll, "ret i64 0") {
		t.Errorf("expected ret i64 0, got:\n%s", ll)
	}
}

func TestLLVM_HelloWorld(t *testing.T) {
	prog := ir.NewProgram("hello_world")
	fn := ir.NewFunction("main", ir.TypeI64)
	cs := ir.NewConstString("Hello World")
	fn.EntryBlock.AddInstruction(&ir.CallRuntimeInst{Func: "print_string", Args: []ir.Value{cs}})
	fn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	prog.AddFunction(fn)

	emitter := backend.NewLLVMEmitter()
	ll, err := emitter.Emit(prog)
	if err != nil {
		t.Fatalf("failed to emit LLVM IR: %v", err)
	}

	if !strings.Contains(ll, "c\"Hello World\\00\"") {
		t.Errorf("expected global string constant, got:\n%s", ll)
	}
	if !strings.Contains(ll, "call void @joss_print_string") {
		t.Errorf("expected call to @joss_print_string, got:\n%s", ll)
	}
}

func TestLLVM_ArithmeticAndFunctionCall(t *testing.T) {
	prog := ir.NewProgram("calc")
	addFn := ir.NewFunction("add", ir.TypeI64)
	pA := addFn.AddParam("a", ir.TypeI64)
	pB := addFn.AddParam("b", ir.TypeI64)
	res := addFn.NewTemp(ir.TypeI64, "res")
	addFn.EntryBlock.AddInstruction(&ir.BinaryInst{Op: ir.OpAdd, Dest: res, Left: pA, Right: pB})
	addFn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: res})
	prog.AddFunction(addFn)

	mainFn := ir.NewFunction("main", ir.TypeI64)
	c10 := ir.NewConstInt(10, ir.TypeI64)
	c20 := ir.NewConstInt(20, ir.TypeI64)
	callRes := mainFn.NewTemp(ir.TypeI64, "call_res")
	mainFn.EntryBlock.AddInstruction(&ir.CallInst{Dest: callRes, Callee: "add", Args: []ir.Value{c10, c20}})
	mainFn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: callRes})
	prog.AddFunction(mainFn)

	emitter := backend.NewLLVMEmitter()
	ll, err := emitter.Emit(prog)
	if err != nil {
		t.Fatalf("failed to emit LLVM IR: %v", err)
	}

	if !strings.Contains(ll, "define i64 @add(i64 %a, i64 %b)") {
		t.Errorf("expected @add definition, got:\n%s", ll)
	}
	if !strings.Contains(ll, "add i64 %a, %b") {
		t.Errorf("expected add instruction, got:\n%s", ll)
	}
	if !strings.Contains(ll, "call i64 @add(i64 10, i64 20)") {
		t.Errorf("expected call @add instruction, got:\n%s", ll)
	}
}
