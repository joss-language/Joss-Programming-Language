package ir

import "testing"

func TestHostServiceOwnerBindingRequiresMatchingCallContract(t *testing.T) {
	program := NewProgram("owner_contract")
	owner := NewFunction("__joss_host_System_log", TypeVoid)
	owner.AddParam("value", TypeMixed)
	program.Functions[owner.Name] = owner

	main := NewFunction("main", TypeI64)
	main.EntryBlock.AddInstruction(&CallHostServiceInst{
		ClassName: "System", MethodName: "log", Args: []Value{NewConstString("ok")},
	})
	main.EntryBlock.SetTerminator(&ReturnTerminator{Val: NewConstInt(0, TypeI64)})
	program.Functions[main.Name] = main

	(&Lowerer{prog: program}).lowerJossHostServiceCalls()
	call, ok := main.EntryBlock.Instructions[0].(*CallInst)
	if !ok || call.Callee != owner.Name || len(call.Args) != 1 || call.Args[0].Type().Kind != TypeKindString {
		t.Fatalf("compatible service owner was not bound with the source argument: %#v", main.EntryBlock.Instructions[0])
	}

	wrongArity := NewFunction("__joss_host_Test_ping", TypeVoid)
	wrongArity.AddParam("value", TypeI64)
	program.Functions[wrongArity.Name] = wrongArity
	arityCall := &CallHostServiceInst{ClassName: "Test", MethodName: "ping"}
	if hostServiceOwnerMatches(wrongArity, arityCall) {
		t.Fatal("owner with a mismatched argument count must not replace a host service")
	}

	wrongResult := NewFunction("__joss_host_Test_count", TypeI64)
	resultCall := &CallHostServiceInst{
		Dest: &TempValue{ID: 0, ValueType: TypeBool}, ClassName: "Test", MethodName: "count",
	}
	if hostServiceOwnerMatches(wrongResult, resultCall) {
		t.Fatal("owner with a mismatched return type must not replace a host service")
	}
}

func TestRuntimeProductShimBindsMatchingJossOwner(t *testing.T) {
	program := NewProgram("auth_owner")
	owner := NewFunction("__joss_host_Auth_enabledSocialProviders", ArrayType(TypeString))
	program.Functions[owner.Name] = owner

	main := NewFunction("main", TypeI64)
	dest := main.NewTemp(ArrayType(TypeString), "providers")
	main.EntryBlock.AddInstruction(&CallRuntimeInst{
		Dest: dest, Func: "Auth_enabledSocialProviders",
	})
	main.EntryBlock.SetTerminator(&ReturnTerminator{Val: NewConstInt(0, TypeI64)})
	program.Functions[main.Name] = main

	(&Lowerer{prog: program}).lowerJossHostServiceCalls()
	call, ok := main.EntryBlock.Instructions[0].(*CallInst)
	if !ok || call.Callee != owner.Name || len(call.Args) != 0 {
		t.Fatalf("runtime product shim was not rebound to its Joss owner: %#v", main.EntryBlock.Instructions[0])
	}
}

func TestLowererPreservesExplicitMixedType(t *testing.T) {
	lowerer := NewLowerer(nil, nil)
	if got := lowerer.mapType("mixed"); !got.Equal(TypeMixed) {
		t.Fatalf("source mixed type lowered to %s, want %s", got, TypeMixed)
	}
	if got := lowerer.mapType("unknown"); !got.Equal(TypeUnknown) {
		t.Fatalf("unknown source type lowered to %s, want %s", got, TypeUnknown)
	}
	if got, want := lowerer.mapType("array<string>"), ArrayType(TypeString); !got.Equal(want) {
		t.Fatalf("typed string array lowered to %s, want %s", got, want)
	}
	if got, want := lowerer.mapType("array<array<int>>"), ArrayType(ArrayType(TypeI64)); !got.Equal(want) {
		t.Fatalf("nested typed array lowered to %s, want %s", got, want)
	}
}

func TestHostServiceOwnerBindingPassesInstanceReceiver(t *testing.T) {
	program := NewProgram("owner_receiver")
	owner := NewFunction("__joss_host_Session_get", TypeMixed)
	owner.AddParam("session", TypeI64)
	owner.AddParam("key", TypeString)
	program.Functions[owner.Name] = owner

	receiver := NewConstInt(7, TypeI64)
	call := &CallHostServiceInst{
		Dest: &TempValue{ID: 0, ValueType: TypeMixed}, ClassName: "Session", MethodName: "get",
		Recv: receiver, Args: []Value{NewConstString("user_id")},
	}
	main := NewFunction("main", TypeI64)
	main.EntryBlock.AddInstruction(call)
	main.EntryBlock.SetTerminator(&ReturnTerminator{Val: NewConstInt(0, TypeI64)})
	program.Functions[main.Name] = main

	(&Lowerer{prog: program}).lowerJossHostServiceCalls()
	rewritten, ok := main.EntryBlock.Instructions[0].(*CallInst)
	if !ok || rewritten.Callee != owner.Name || len(rewritten.Args) != 2 || rewritten.Args[0] != receiver {
		t.Fatalf("receiver was not preserved as the first Joss owner argument: %#v", main.EntryBlock.Instructions[0])
	}
}

func TestRequestFacadeOwnerDropsCompatibilityReceiver(t *testing.T) {
	program := NewProgram("request_owner_receiver")
	mapType := PtrType(StructType("map", nil))
	arrayType := Type{Kind: TypeKindArray}
	owner := NewFunction("__joss_host_Request_except", mapType)
	owner.AddParam("excluded", TypeMixed)
	program.Functions[owner.Name] = owner

	receiver := NewConstInt(7, TypeI64)
	main := NewFunction("main", TypeI64)
	arrayArg := main.AddParam("excluded", arrayType)
	call := &CallHostServiceInst{
		Dest: &TempValue{ID: 0, ValueType: mapType}, ClassName: "Request", MethodName: "except",
		Recv: receiver, Args: []Value{arrayArg},
	}
	main.EntryBlock.AddInstruction(call)
	main.EntryBlock.SetTerminator(&ReturnTerminator{Val: NewConstInt(0, TypeI64)})
	program.Functions[main.Name] = main

	(&Lowerer{prog: program}).lowerJossHostServiceCalls()
	rewritten, ok := main.EntryBlock.Instructions[0].(*CallInst)
	if !ok || rewritten.Callee != owner.Name || len(rewritten.Args) != 1 || rewritten.Args[0] != call.Args[0] {
		t.Fatalf("request compatibility receiver was not omitted for its context-backed owner: %#v", main.EntryBlock.Instructions[0])
	}
}

func TestHostServiceOwnerBindingUsesConstantOptionalDefault(t *testing.T) {
	owner := NewFunction("__joss_host_System_now", TypeString)
	owner.AddParam("days", TypeI64)
	owner.DefaultArgs = []Value{NewConstInt(0, TypeI64)}

	call := &CallHostServiceInst{
		Dest: &TempValue{ID: 0, ValueType: TypeString}, ClassName: "System", MethodName: "now",
	}
	args, ok := hostServiceOwnerArguments(owner, call)
	if !ok || len(args) != 1 {
		t.Fatalf("optional Joss owner default was not accepted: args=%v, ok=%v", args, ok)
	}
	if got, ok := args[0].(*ConstInt); !ok || got.Val != 0 {
		t.Fatalf("owner default argument = %v, want integer zero", args[0])
	}
}
