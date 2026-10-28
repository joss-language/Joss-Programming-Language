package native_test

import (
	"encoding/binary"
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
