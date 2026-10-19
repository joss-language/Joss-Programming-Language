//go:build windows

package native

import (
	"syscall"

	"github.com/jossecurity/joss/pkg/jit"
)

var (
	modKernel32        = syscall.NewLazyDLL("kernel32.dll")
	procGetStdHandle   = modKernel32.NewProc("GetStdHandle")
	procWriteFile      = modKernel32.NewProc("WriteFile")
	procGetProcessHeap = modKernel32.NewProc("GetProcessHeap")
	procHeapAlloc      = modKernel32.NewProc("HeapAlloc")
	procExitProcess    = modKernel32.NewProc("ExitProcess")
	procLoadLibraryA   = modKernel32.NewProc("LoadLibraryA")
	procGetProcAddress = modKernel32.NewProc("GetProcAddress")
	procHostBridgePtr  = syscall.NewCallback(jit.InProcessHostBridge)
)

func getPlatformIATPointers() [iatSlotCount]uintptr {
	return [iatSlotCount]uintptr{
		procGetStdHandle.Addr(),
		procWriteFile.Addr(),
		procGetProcessHeap.Addr(),
		procHeapAlloc.Addr(),
		procExitProcess.Addr(),
		procLoadLibraryA.Addr(),
		procGetProcAddress.Addr(),
		procHostBridgePtr,
	}
}
