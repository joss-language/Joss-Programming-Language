//go:build !windows

package jit

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// ExecutableBlock represents an allocated page of executable memory in RAM.
type ExecutableBlock struct {
	Addr uintptr
	Size uintptr
	data []byte
}

// AllocateExecutable allocates memory via mmap with PROT_READ|PROT_WRITE,
// copies the machine code into it, and changes permissions to PROT_READ|PROT_EXEC.
func AllocateExecutable(code []byte) (*ExecutableBlock, error) {
	if len(code) == 0 {
		return nil, errors.New("cannot allocate executable block for empty code")
	}

	size := len(code)
	// Allocate RW memory via mmap
	data, err := syscall.Mmap(
		-1,
		0,
		size,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_ANON|syscall.MAP_PRIVATE,
	)
	if err != nil {
		return nil, fmt.Errorf("mmap failed: %v", err)
	}

	copy(data, code)

	// Change protection to PROT_READ|PROT_EXEC via mprotect
	err = syscall.Mprotect(data, syscall.PROT_READ|syscall.PROT_EXEC)
	if err != nil {
		_ = syscall.Munmap(data)
		return nil, fmt.Errorf("mprotect failed: %v", err)
	}

	addr := uintptr(unsafe.Pointer(&data[0]))
	return &ExecutableBlock{
		Addr: addr,
		Size: uintptr(size),
		data: data,
	}, nil
}

// Free releases the mmap'd memory.
func (b *ExecutableBlock) Free() error {
	if b == nil || len(b.data) == 0 {
		return nil
	}
	err := syscall.Munmap(b.data)
	b.Addr = 0
	b.Size = 0
	b.data = nil
	return err
}
