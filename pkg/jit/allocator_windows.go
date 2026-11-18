package jit

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

var procFlushInstructionCache = windows.NewLazySystemDLL("kernel32.dll").NewProc("FlushInstructionCache")

// ExecutableBlock represents an allocated page of executable memory in RAM.
type ExecutableBlock struct {
	Addr      uintptr
	Size      uintptr
	codeReady bool
}

func AllocateJITImage(size int) (*ExecutableBlock, error) {
	if size <= 0 {
		return nil, errors.New("cannot allocate an empty JIT image")
	}
	addr, err := windows.VirtualAlloc(0, uintptr(size), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if err != nil {
		return nil, fmt.Errorf("VirtualAlloc writable JIT image failed: %w", err)
	}
	return &ExecutableBlock{Addr: addr, Size: uintptr(size)}, nil
}

func (b *ExecutableBlock) Write(image []byte) error {
	if b == nil || b.Addr == 0 || b.codeReady {
		return errors.New("JIT image is not writable")
	}
	if uintptr(len(image)) > b.Size {
		return fmt.Errorf("JIT image exceeds allocated size: %d > %d", len(image), b.Size)
	}
	if len(image) > 0 {
		var written uintptr
		err := windows.WriteProcessMemory(windows.CurrentProcess(), b.Addr, &image[0], uintptr(len(image)), &written)
		if err != nil {
			return fmt.Errorf("WriteProcessMemory JIT image failed: %w", err)
		}
		if written != uintptr(len(image)) {
			return fmt.Errorf("incomplete JIT image write: %d of %d bytes", written, len(image))
		}
	}
	return nil
}

func (b *ExecutableBlock) FinalizeCode(codeSize int) error {
	if b == nil || b.Addr == 0 || b.codeReady || codeSize <= 0 || uintptr(codeSize) > b.Size {
		return fmt.Errorf("invalid JIT code size %d for image size %d", codeSize, b.Size)
	}
	pageSize := os.Getpagesize()
	protectedSize := uintptr((codeSize + pageSize - 1) / pageSize * pageSize)
	if protectedSize > b.Size {
		return fmt.Errorf("JIT code page range exceeds image: %d > %d", protectedSize, b.Size)
	}
	flushed, _, flushErr := procFlushInstructionCache.Call(uintptr(windows.CurrentProcess()), b.Addr, uintptr(codeSize))
	if flushed == 0 {
		if flushErr != nil {
			return fmt.Errorf("FlushInstructionCache failed: %w", flushErr)
		}
		return errors.New("FlushInstructionCache failed without a Windows error")
	}
	var oldProtect uint32
	if err := windows.VirtualProtect(b.Addr, protectedSize, windows.PAGE_EXECUTE_READ, &oldProtect); err != nil {
		return fmt.Errorf("VirtualProtect JIT text RX failed: %w", err)
	}
	b.codeReady = true
	return nil
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
		Addr:      addr,
		Size:      size,
		codeReady: true,
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
