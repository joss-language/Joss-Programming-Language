//go:build !(amd64 || arm64) || !(darwin || freebsd || linux || netbsd || windows)

package core

import "fmt"

func loadNativeDriver(name, libraryPath string) (*NativeDriverDefinition, error) {
	return nil, fmt.Errorf("los drivers nativos C ABI v1 no estan soportados en esta arquitectura/plataforma")
}

func unloadNativeDriverHandle(handle uintptr) error {
	return nil
}
