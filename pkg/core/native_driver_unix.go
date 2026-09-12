//go:build (darwin || freebsd || linux || netbsd) && (amd64 || arm64)

package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ebitengine/purego"
)

func loadNativeDriver(name, libraryPath string) (*NativeDriverDefinition, error) {
	abs, err := filepath.Abs(libraryPath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, err
	}
	handle, err := purego.Dlopen(abs, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	callSymbol, err := purego.Dlsym(handle, "joss_driver_call")
	if err != nil {
		_ = purego.Dlclose(handle)
		return nil, fmt.Errorf("falta simbolo joss_driver_call: %w", err)
	}
	var call func(string, string) *byte
	purego.RegisterFunc(&call, callSymbol)
	var free func(*byte)
	if freeSymbol, symbolErr := purego.Dlsym(handle, "joss_driver_free"); symbolErr == nil {
		purego.RegisterFunc(&free, freeSymbol)
	}
	return &NativeDriverDefinition{Name: name, Path: abs, Handle: handle, Call: call, Free: free}, nil
}

func unloadNativeDriverHandle(handle uintptr) error {
	return purego.Dlclose(handle)
}
