package jit

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// ExecutableBlock represents an allocated page of executable memory in RAM.
type ExecutableBlock struct {
	Addr uintptr
	Size uintptr
}

// AllocateExecutable allocates memory with PAGE_EXECUTE_READWRITE permissions,
// copies the machine code into it, and returns an ExecutableBlock.
func AllocateExecutable(code []byte) (*ExecutableBlock, error) {
	if len(code) == 0 {
		return nil, errors.New("cannot allocate executable block for empty code")
	}

	size := uintptr(len(code))
	addr, err := windows.VirtualAlloc(
		0,
		size,
		windows.MEM_COMMIT|windows.MEM_RESERVE,
		windows.PAGE_EXECUTE_READWRITE,
	)
	if err != nil {
		return nil, fmt.Errorf("VirtualAlloc failed: %w", err)
	}

	// Copy machine code into allocated space via WriteProcessMemory
	if len(code) > 0 {
		var written uintptr
		currentProc := windows.CurrentProcess()
		err = windows.WriteProcessMemory(currentProc, addr, &code[0], size, &written)
		if err != nil {
			_ = windows.VirtualFree(addr, 0, windows.MEM_RELEASE)
			return nil, fmt.Errorf("WriteProcessMemory failed: %w", err)
		}
	}

	return &ExecutableBlock{
		Addr: addr,
		Size: size,
	}, nil
}

// Free releases the allocated virtual memory.
func (b *ExecutableBlock) Free() error {
	if b == nil || b.Addr == 0 {
		return nil
	}
	err := windows.VirtualFree(b.Addr, 0, windows.MEM_RELEASE)
	if err != nil {
		return fmt.Errorf("VirtualFree failed: %w", err)
	}
	b.Addr = 0
	b.Size = 0
	return nil
}
