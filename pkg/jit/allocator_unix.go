//go:build !windows

package jit

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// ExecutableBlock represents an allocated page of executable memory in RAM.
type ExecutableBlock struct {
	Addr      uintptr
	Size      uintptr
	data      []byte
	codeReady bool
}

// AllocateJITImage reserves a writable image so a backend can resolve its
// relocations using the final address before making only the text pages executable.
func AllocateJITImage(size int) (*ExecutableBlock, error) {
	if size <= 0 {
		return nil, errors.New("cannot allocate an empty JIT image")
	}
	data, err := syscall.Mmap(-1, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		return nil, fmt.Errorf("mmap writable JIT image failed: %w", err)
	}
	return &ExecutableBlock{Addr: uintptr(unsafe.Pointer(&data[0])), Size: uintptr(size), data: data}, nil
}

// Write initializes an image before FinalizeCode changes text pages to RX.
func (b *ExecutableBlock) Write(image []byte) error {
	if b == nil || len(b.data) == 0 || b.codeReady {
		return errors.New("JIT image is not writable")
	}
	if len(image) > len(b.data) {
		return fmt.Errorf("JIT image exceeds allocated size: %d > %d", len(image), len(b.data))
	}
	copy(b.data, image)
	return nil
}

// FinalizeCode flushes the ARM instruction cache and protects only code pages
// as executable. The image's data pages stay writable and non-executable.
func (b *ExecutableBlock) FinalizeCode(codeSize int) error {
	if b == nil || len(b.data) == 0 || b.codeReady {
		return errors.New("JIT image cannot be finalized")
	}
	if codeSize <= 0 || uintptr(codeSize) > b.Size {
		return fmt.Errorf("invalid JIT code size %d for image size %d", codeSize, b.Size)
	}
	pageSize := os.Getpagesize()
	protectedSize := (codeSize + pageSize - 1) / pageSize * pageSize
	if protectedSize > len(b.data) {
		return fmt.Errorf("JIT code page range exceeds image: %d > %d", protectedSize, len(b.data))
	}
	flushInstructionCache(b.Addr, uintptr(codeSize))
	if err := syscall.Mprotect(b.data[:protectedSize], syscall.PROT_READ|syscall.PROT_EXEC); err != nil {
		return fmt.Errorf("mprotect JIT text RX failed: %w", err)
	}
	b.codeReady = true
	return nil
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
		Addr:      addr,
		Size:      uintptr(size),
		data:      data,
		codeReady: true,
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
