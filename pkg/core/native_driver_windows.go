//go:build windows && (amd64 || arm64)

package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/windows"
)

func loadNativeDriver(name, libraryPath string) (*NativeDriverDefinition, error) {
	abs, err := filepath.Abs(libraryPath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, err
	}
	handle, err := windows.LoadLibrary(abs)
	if err != nil {
		return nil, err
	}
	callSymbol, err := windows.GetProcAddress(handle, "joss_driver_call")
	if err != nil {
		_ = windows.FreeLibrary(handle)
		return nil, fmt.Errorf("falta simbolo joss_driver_call: %w", err)
	}
	var call func(string, string) *byte
	purego.RegisterFunc(&call, callSymbol)
	var free func(*byte)
	if freeSymbol, symbolErr := windows.GetProcAddress(handle, "joss_driver_free"); symbolErr == nil {
		purego.RegisterFunc(&free, freeSymbol)
	}
	return &NativeDriverDefinition{Name: name, Path: abs, Handle: uintptr(handle), Call: call, Free: free}, nil
}

func unloadNativeDriverHandle(handle uintptr) error {
	return windows.FreeLibrary(windows.Handle(handle))
}
