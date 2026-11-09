//go:build windows

package jit

import (
	"syscall"
	"unsafe"
)

var (
	modKernel32        = syscall.NewLazyDLL("kernel32.dll")
	procGetProcessHeap = modKernel32.NewProc("GetProcessHeap")
	procHeapAlloc      = modKernel32.NewProc("HeapAlloc")
	procVirtualQuery   = modKernel32.NewProc("VirtualQuery")
)

type memoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionId       uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

func isReadablePointer(ptr uintptr) bool {
	if ptr < 0x10000 || ptr > 0x00007FFFFFFFFFFF {
		return false
	}
	var mbi memoryBasicInformation
	ret, _, _ := procVirtualQuery.Call(ptr, uintptr(unsafe.Pointer(&mbi)), unsafe.Sizeof(mbi))
	if ret == 0 {
		return false
	}
	if mbi.State != 0x1000 /* MEM_COMMIT */ {
		return false
	}
	const readMask = 0x02 /* PAGE_READONLY */ | 0x04 /* PAGE_READWRITE */ | 0x20 /* PAGE_EXECUTE_READ */ | 0x40 /* PAGE_EXECUTE_READWRITE */
	return (mbi.Protect & readMask) != 0
}

func allocNative(size int) uintptr {
	if size <= 0 {
		return 0
	}
	hHeap, _, _ := procGetProcessHeap.Call()
	ptr, _, _ := procHeapAlloc.Call(hHeap, 8 /* HEAP_ZERO_MEMORY */, uintptr(size))
	return ptr
}
