package native_test

import (
	"encoding/binary"
	"strings"
	"testing"

	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/ir"
)

func TestARM64_HelloWorld_InstructionsAndELF(t *testing.T) {
	prog := ir.NewProgram("hello_arm64")
	fn := ir.NewFunction("main", ir.TypeI64)

	cs := ir.NewConstString("Hello AArch64 World")
	fn.EntryBlock.AddInstruction(&ir.CallInst{
		Callee: "print",
		Args:   []ir.Value{cs},
	})
	fn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	prog.AddFunction(fn)

	// 1. Target Linux ARM64
	target := backend.Target{OS: "linux", Arch: "arm64"}
	builder := backend.NewARM64Builder(target, false)
	binBytes, asmListing, err := builder.Compile(prog)
	if err != nil {
		t.Fatalf("ARM64Builder.Compile falló: %v", err)
	}

	if len(binBytes) < 4096 {
		t.Fatalf("Binario generado demasiado pequeño: %d bytes", len(binBytes))
	}

	// Verificar cabecera ELF
	eIdentMagic := string(binBytes[0:4])
	if eIdentMagic != "\x7fELF" {
		t.Errorf("Cabecera ELF inválida: %q", eIdentMagic)
	}
	eMachine := binary.LittleEndian.Uint16(binBytes[18:20])
	if eMachine != 0x00B7 {
		t.Errorf("e_machine esperado 0x00B7 (EM_AARCH64), obtenido 0x%04X", eMachine)
	}

	// Verificar offset 0x1000 (.text)
	textOffset := 4096

	// En x86-64 era: 0x55 (push rbp), 0x48 0x89 0xE5 (mov rbp, rsp)
	if binBytes[textOffset] == 0x55 && binBytes[textOffset+1] == 0x48 {
		t.Fatalf("CRÍTICO: El binario ARM64 contiene instrucciones x86_64 en .text!")
	}

	// En AArch64 _start debe comenzar con STP x29, x30, [sp, #-32]! y MOV x29, sp
	inst1 := binary.LittleEndian.Uint32(binBytes[textOffset+4 : textOffset+8])
	if inst1 != 0x910003FD {
		t.Errorf("Instrucción 1 esperada 'mov x29, sp' (0x910003FD), obtenido 0x%08X", inst1)
	}

	// Verificar presencia de llamada al sistema Linux AArch64: svc #0 (0xD4000001)
	foundSVC0 := false
	for i := textOffset; i < len(binBytes)-4; i += 4 {
		inst := binary.LittleEndian.Uint32(binBytes[i : i+4])
		if inst == 0xD4000001 {
			foundSVC0 = true
			break
		}
	}
	if !foundSVC0 {
		t.Errorf("No se encontró instrucción 'svc #0' (0xD4000001) para syscalls de Linux ARM64")
	}

	// Verificar presencia de retorno AArch64: ret (0xD65F03C0)
	foundRET := false
	for i := textOffset; i < len(binBytes)-4; i += 4 {
		inst := binary.LittleEndian.Uint32(binBytes[i : i+4])
		if inst == 0xD65F03C0 {
			foundRET = true
			break
		}
	}
	if !foundRET {
		t.Errorf("No se encontró instrucción 'ret' (0xD65F03C0)")
	}

	_ = asmListing
}

func TestARM64_Darwin_MachO(t *testing.T) {
	prog := ir.NewProgram("hello_darwin_arm64")
	fn := ir.NewFunction("main", ir.TypeI64)

	cs := ir.NewConstString("Hello Apple Silicon")
	fn.EntryBlock.AddInstruction(&ir.CallInst{
		Callee: "print",
		Args:   []ir.Value{cs},
	})
	fn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	prog.AddFunction(fn)

	target := backend.Target{OS: "darwin", Arch: "arm64"}
	builder := backend.NewARM64Builder(target, false)
	binBytes, _, err := builder.Compile(prog)
	if err != nil {
		t.Fatalf("ARM64Builder.Compile Darwin falló: %v", err)
	}

	// Verificar Mach-O 64 magic (0xfeedfacf)
	magic := binary.LittleEndian.Uint32(binBytes[0:4])
	if magic != 0xfeedfacf {
		t.Errorf("Mach-O magic inválido: 0x%08X", magic)
	}

	cpuType := binary.LittleEndian.Uint32(binBytes[4:8])
	if cpuType != 0x0100000c {
		t.Errorf("Mach-O cpuType esperado 0x0100000c (CPU_TYPE_ARM64), obtenido 0x%08X", cpuType)
	}

	// Verificar presencia de syscall Darwin: svc #0x80 (0xD4001001)
	foundSVC80 := false
	for i := 0; i < len(binBytes)-4; i += 4 {
		inst := binary.LittleEndian.Uint32(binBytes[i : i+4])
		if inst == 0xD4001001 {
			foundSVC80 = true
			break
		}
	}
	if !foundSVC80 {
		t.Errorf("No se encontró instrucción 'svc #0x80' (0xD4001001) para syscalls de macOS Darwin ARM64")
	}
}

func TestARM64_ArithmeticAndFunctions(t *testing.T) {
	prog := ir.NewProgram("math_arm64")

	// Function: add(a, b) -> a + b
	fnAdd := ir.NewFunction("add", ir.TypeI64)
	pA := fnAdd.AddParam("a", ir.TypeI64)
	pB := fnAdd.AddParam("b", ir.TypeI64)
	tAdd := fnAdd.NewTemp(ir.TypeI64, "add_res")
	fnAdd.EntryBlock.AddInstruction(&ir.BinaryInst{
		Op:    ir.OpAdd,
		Dest:  tAdd,
		Left:  pA,
		Right: pB,
	})
	fnAdd.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: tAdd})
	prog.AddFunction(fnAdd)

	// Function: main()
	fnMain := ir.NewFunction("main", ir.TypeI64)
	c10 := ir.NewConstInt(10, ir.TypeI64)
	c20 := ir.NewConstInt(20, ir.TypeI64)
	tRes := fnMain.NewTemp(ir.TypeI64, "main_res")
	fnMain.EntryBlock.AddInstruction(&ir.CallInst{
		Callee: "add",
		Args:   []ir.Value{c10, c20},
		Dest:   tRes,
	})
	fnMain.EntryBlock.AddInstruction(&ir.CallInst{
		Callee: "print",
		Args:   []ir.Value{tRes},
	})
	fnMain.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: tRes})
	prog.AddFunction(fnMain)

	target := backend.Target{OS: "linux", Arch: "arm64"}
	builder := backend.NewARM64Builder(target, false)
	binBytes, asmListing, err := builder.Compile(prog)
	if err != nil {
		t.Fatalf("ARM64Builder.Compile math falló: %v", err)
	}

	if len(binBytes) == 0 {
		t.Fatal("Binario vacío")
	}

	// Verificar presencia de instrucción ADD en AArch64: add x2, x0, x1 = 0x8B010002
	foundADD := false
	for i := 4096; i < len(binBytes)-4; i += 4 {
		inst := binary.LittleEndian.Uint32(binBytes[i : i+4])
		if inst == 0x8B010002 {
			foundADD = true
			break
		}
	}
	if !foundADD {
		t.Errorf("No se encontró instrucción AArch64 'add x2, x0, x1' (0x8B010002)")
	}

	_ = asmListing
}

func TestARM64_StringLengthCountsUTF8Runes(t *testing.T) {
	prog := ir.NewProgram("strlen_arm64")
	fn := ir.NewFunction("main", ir.TypeI64)
	length := fn.NewTemp(ir.TypeI64, "length")
	fn.EntryBlock.AddInstruction(&ir.CallBuiltinInst{
		Dest:      length,
		BuiltinID: "strlen",
		Args:      []ir.Value{ir.NewConstString("Ñ🚀")},
	})
	fn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: length})
	prog.AddFunction(fn)

	for _, target := range []backend.Target{
		{OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "arm64"},
	} {
		t.Run(target.OS, func(t *testing.T) {
			builder := backend.NewARM64Builder(target, false)
			bin, listing, err := builder.Compile(prog)
			if err != nil {
				t.Fatalf("Compile(%s) failed: %v", target, err)
			}
			if len(bin) == 0 {
				t.Fatal("empty ARM64 binary")
			}
			if !strings.Contains(listing, "fn_strlen:") || !strings.Contains(listing, "cmp x0, x31") || !strings.Contains(listing, "and w6, w4, w5") || !strings.Contains(listing, ".strlen_rune_loop:") {
				t.Fatalf("%s strlen emitter does not count UTF-8 leading bytes:\n%s", target, listing)
			}
		})
	}
}

func TestARM64_TimeBuiltinHasPerOSClockPrimitive(t *testing.T) {
	prog := ir.NewProgram("time_arm64")
	fn := ir.NewFunction("main", ir.TypeI64)
	seconds := fn.NewTemp(ir.TypeI64, "seconds")
	fn.EntryBlock.AddInstruction(&ir.CallBuiltinInst{Dest: seconds, BuiltinID: "time"})
	fn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: seconds})
	prog.AddFunction(fn)

	targets := []struct {
		target backend.Target
		want   string
	}{
		{backend.Target{OS: "linux", Arch: "arm64"}, "CLOCK_REALTIME via clock_gettime syscall"},
		{backend.Target{OS: "android", Arch: "arm64"}, "CLOCK_REALTIME via clock_gettime syscall"},
		{backend.Target{OS: "darwin", Arch: "arm64"}, "gettimeofday syscall"},
		{backend.Target{OS: "windows", Arch: "arm64"}, "Windows KUSER_SHARED_DATA SystemTime"},
	}
	for _, tc := range targets {
		t.Run(tc.target.DashString(), func(t *testing.T) {
			builder := backend.NewARM64Builder(tc.target, false)
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

func TestARM64_WindowsPE32Plus_And_ObjectsCollections(t *testing.T) {
	prog := ir.NewProgram("full_arm64")
	fnMain := ir.NewFunction("main", ir.TypeI64)

	tObj := fnMain.NewTemp(ir.PtrType(ir.StructType("User", nil)), "obj")
	fnMain.EntryBlock.AddInstruction(&ir.NewObjectInst{
		Dest:      tObj,
		ClassName: "User",
	})
	fnMain.EntryBlock.AddInstruction(&ir.StoreFieldInst{
		Obj:       tObj,
		FieldName: "name",
		Val:       ir.NewConstString("Ada"),
	})
	tField := fnMain.NewTemp(ir.TypeString, "field")
	fnMain.EntryBlock.AddInstruction(&ir.LoadFieldInst{
		Dest:      tField,
		Obj:       tObj,
		FieldName: "name",
	})
	tConcat := fnMain.NewTemp(ir.TypeString, "concat")
	fnMain.EntryBlock.AddInstruction(&ir.CallRuntimeInst{
		Dest: tConcat,
		Func: "str_concat",
		Args: []ir.Value{ir.NewConstString("Hello "), tField},
	})
	fnMain.EntryBlock.AddInstruction(&ir.CallInst{
		Callee: "print",
		Args:   []ir.Value{tConcat},
	})
	fnMain.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	prog.AddFunction(fnMain)

	for _, target := range []backend.Target{
		{OS: "linux", Arch: "arm64"},
		{OS: "darwin", Arch: "arm64"},
		{OS: "windows", Arch: "arm64"},
	} {
		builder := backend.NewARM64Builder(target, false)
		binBytes, _, err := builder.Compile(prog)
		if err != nil {
			t.Fatalf("ARM64Builder.Compile(%s) falló: %v", target, err)
		}
		if builder.NeedsHostBridge() {
			t.Errorf("ARM64Builder(%s) no debe activar NeedsHostBridge", target)
		}
		if target.OS == "windows" {
			if string(binBytes[0:2]) != "MZ" {
				t.Errorf("Cabecera MZ inválida para windows/arm64")
			}
			machine := binary.LittleEndian.Uint16(binBytes[0x44:0x46])
			if machine != 0xAA64 {
				t.Errorf("PE Machine esperado 0xAA64 (IMAGE_FILE_MACHINE_ARM64), obtenido 0x%04X", machine)
			}
		}
	}
}

func TestARM64_FloatingPoint_TryCatch_And_GranDB(t *testing.T) {
	prog := ir.NewProgram("fp_try_grandb_arm64")

	fnMain := ir.NewFunction("main", ir.TypeI64)
	tSum := fnMain.NewTemp(ir.TypeF64, "fp_sum")
	fnMain.EntryBlock.AddInstruction(&ir.BinaryInst{
		Op:    ir.OpAdd,
		Dest:  tSum,
		Left:  ir.NewConstFloat(1.5, ir.TypeF64),
		Right: ir.NewConstFloat(2.5, ir.TypeF64),
	})
	tCmp := fnMain.NewTemp(ir.TypeBool, "fp_cmp")
	fnMain.EntryBlock.AddInstruction(&ir.CompareInst{
		Op:    ir.OpCmpEq,
		Dest:  tCmp,
		Left:  tSum,
		Right: ir.NewConstFloat(4.0, ir.TypeF64),
	})

	tryBlock := &ir.BasicBlock{Label: "try_body"}
	catchBlock := &ir.BasicBlock{Label: "catch_body"}
	excTemp := fnMain.NewTemp(ir.TypeString, "err")

	tTable := fnMain.NewTemp(ir.TypeI64, "tbl")
	tryBlock.AddInstruction(&ir.CallHostServiceInst{
		Dest:       tTable,
		ClassName:  "GranDB",
		MethodName: "table",
		Args:       []ir.Value{ir.NewConstString("users")},
	})
	tryBlock.AddInstruction(&ir.ThrowInst{Val: ir.NewConstString("boom")})

	catchBlock.AddInstruction(&ir.CallInst{
		Callee: "print",
		Args:   []ir.Value{excTemp},
	})

	fnMain.EntryBlock.AddInstruction(&ir.TryCatchInst{
		ID:          1,
		TryBlocks:   []*ir.BasicBlock{tryBlock},
		CatchBlocks: []*ir.BasicBlock{catchBlock},
		CatchVarPtr: excTemp,
	})
	fnMain.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	prog.AddFunction(fnMain)

	for _, target := range []backend.Target{
		{OS: "linux", Arch: "arm64"},
		{OS: "darwin", Arch: "arm64"},
		{OS: "windows", Arch: "arm64"},
	} {
		builder := backend.NewARM64Builder(target, false)
		binBytes, asmListing, err := builder.Compile(prog)
		if err != nil {
			t.Fatalf("ARM64Builder.Compile(%s) falló en FP/TryCatch/GranDB: %v", target, err)
		}
		if len(binBytes) < 4096 {
			t.Fatalf("Binario(%s) demasiado pequeño: %d", target, len(binBytes))
		}
		// Verificar presencia de fadd d2, d0, d1 (0x1E612802)
		foundFadd := false
		for i := 0; i <= len(binBytes)-4; i += 4 {
			if binary.LittleEndian.Uint32(binBytes[i:i+4]) == 0x1E612802 {
				foundFadd = true
				break
			}
		}
		if !foundFadd {
			t.Errorf("No se encontró instrucción hardware 'fadd d2, d0, d1' (0x1E612802) en %s\nASM:\n%s", target, asmListing)
		}
	}
}

func TestARM64_GranDB_MultiDialect_QueryBuilder(t *testing.T) {
	prog := ir.NewProgram("grandb_arm64")
	fnMain := ir.NewFunction("main", ir.TypeI64)

	tTable := fnMain.NewTemp(ir.TypeI64, "q1")
	fnMain.EntryBlock.AddInstruction(&ir.CallHostServiceInst{
		Dest:       tTable,
		ClassName:  "GranDB",
		MethodName: "table",
		Args:       []ir.Value{ir.NewConstString("users")},
	})

	tWhere := fnMain.NewTemp(ir.TypeI64, "q2")
	fnMain.EntryBlock.AddInstruction(&ir.CallHostServiceInst{
		Dest:       tWhere,
		ClassName:  "GranDB",
		MethodName: "where",
		Args:       []ir.Value{tTable, ir.NewConstString("role"), ir.NewConstString("="), ir.NewConstString("admin")},
	})

	tSql := fnMain.NewTemp(ir.TypeString, "sql")
	fnMain.EntryBlock.AddInstruction(&ir.CallHostServiceInst{
		Dest:       tSql,
		ClassName:  "GranDB",
		MethodName: "toSql",
		Args:       []ir.Value{tWhere},
	})

	fnMain.EntryBlock.AddInstruction(&ir.CallInst{
		Callee: "print",
		Args:   []ir.Value{tSql},
	})
	fnMain.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	prog.AddFunction(fnMain)

	for _, target := range []backend.Target{
		{OS: "linux", Arch: "arm64"},
		{OS: "darwin", Arch: "arm64"},
		{OS: "windows", Arch: "arm64"},
	} {
		builder := backend.NewARM64Builder(target, false)
		binBytes, asmListing, err := builder.Compile(prog)
		if err != nil {
			t.Fatalf("ARM64Builder.Compile(%s) falló en GranDB MultiDialect: %v", target, err)
		}
		if len(binBytes) < 4096 {
			t.Fatalf("Binario ARM64(%s) demasiado pequeño: %d", target, len(binBytes))
		}

		switch target.OS {
		case "linux":
			if string(binBytes[0:4]) != "\x7fELF" {
				t.Errorf("Cabecera ELF inválida en %s", target)
			}
			eMachine := binary.LittleEndian.Uint16(binBytes[18:20])
			if eMachine != 0x00B7 {
				t.Errorf("e_machine esperado 0x00B7 (EM_AARCH64), obtenido 0x%04X", eMachine)
			}
		case "darwin":
			magic := binary.LittleEndian.Uint32(binBytes[0:4])
			if magic != 0xfeedfacf {
				t.Errorf("Mach-O magic inválido en %s: 0x%08X", target, magic)
			}
		case "windows":
			if string(binBytes[0:2]) != "MZ" {
				t.Errorf("Cabecera MZ inválida para windows/arm64")
			}
			machine := binary.LittleEndian.Uint16(binBytes[0x44:0x46])
			if machine != 0xAA64 {
				t.Errorf("PE Machine esperado 0xAA64 (IMAGE_FILE_MACHINE_ARM64), obtenido 0x%04X", machine)
			}
		}

		_ = asmListing
	}
}

func TestARM64_ServerStart_FullEmission(t *testing.T) {
	prog := ir.NewProgram("server_arm64")
	fnMain := ir.NewFunction("main", ir.TypeI64)
	fnMain.EntryBlock.AddInstruction(&ir.CallHostServiceInst{
		ClassName:  "Server",
		MethodName: "start",
		Args:       nil,
	})
	fnMain.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	prog.AddFunction(fnMain)

	for _, target := range []backend.Target{
		{OS: "linux", Arch: "arm64"},
		{OS: "darwin", Arch: "arm64"},
		{OS: "windows", Arch: "arm64"},
	} {
		builder := backend.NewARM64Builder(target, false)
		binBytes, asmListing, err := builder.Compile(prog)
		if err != nil {
			t.Fatalf("ARM64Builder.Compile(%s) falló con Server.start: %v", target, err)
		}
		if len(binBytes) < 4096 {
			t.Fatalf("Binario ARM64(%s) demasiado pequeño: %d", target, len(binBytes))
		}
		if !strings.Contains(asmListing, "fn_Server_start:") {
			t.Errorf("asmListing(%s) no contiene la etiqueta fn_Server_start", target)
		}
		if !strings.Contains(asmListing, ".srv_accept_loop:") {
			t.Errorf("asmListing(%s) no contiene el bucle de accept de servidor nativo", target)
		}
	}
}
