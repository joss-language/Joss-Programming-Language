package jit_test

import (
	"runtime"
	"testing"

	"github.com/jossecurity/joss/pkg/jit"
)

func TestJIT_AllocateAndExecuteBasic(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("JIT basic test is x86_64 amd64 only")
	}

	// x86_64 Machine Code:
	// mov eax, 42   (B8 2A 00 00 00)
	// ret           (C3)
	code := []byte{
		0xB8, 0x2A, 0x00, 0x00, 0x00, // mov eax, 42
		0xC3, // ret
	}

	block, err := jit.AllocateExecutable(code)
	if err != nil {
		t.Fatalf("AllocateExecutable failed: %v", err)
	}
	defer func() {
		if err := block.Free(); err != nil {
			t.Errorf("Free failed: %v", err)
		}
	}()

	ret, err := jit.CallFunc0(block.Addr)
	if err != nil && err.Error() != "The operation completed successfully." {
		// Windows SyscallN returns errno 0 which formats as "The operation completed successfully."
		t.Logf("CallFunc0 notice: %v", err)
	}

	if ret != 42 {
		t.Fatalf("expected return 42, got %d", ret)
	}
}

func TestJIT_ArithmeticAdd(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("JIT basic test is x86_64 amd64 only")
	}

	var code []byte
	if runtime.GOOS == "windows" {
		// Windows x64 ABI: arg1 in RCX, arg2 in RDX, return in RAX
		// lea rax, [rcx + rdx]  -> 48 8D 04 11
		// ret                   -> C3
		code = []byte{
			0x48, 0x8D, 0x04, 0x11, // lea rax, [rcx + rdx]
			0xC3, // ret
		}
	} else {
		// System V AMD64 ABI: arg1 in RDI, arg2 in RSI, return in RAX
		// lea rax, [rdi + rsi]  -> 48 8D 04 37
		// ret                   -> C3
		code = []byte{
			0x48, 0x8D, 0x04, 0x37, // lea rax, [rdi + rsi]
			0xC3, // ret
		}
	}

	block, err := jit.AllocateExecutable(code)
	if err != nil {
		t.Fatalf("AllocateExecutable failed: %v", err)
	}
	defer block.Free()

	ret, _ := jit.CallFunc2(block.Addr, 15, 27)
	if ret != 42 {
		t.Fatalf("expected return 42 (15 + 27), got %d", ret)
	}
}
