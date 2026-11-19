package native

import "fmt"

// ExternalLLVMCompiler is an optional hook registered by pkg/backend/llvm
// so pkg/backend/native never imports os/exec or invokes external processes directly.
type ExternalLLVMCompiler func(llvmIR string, outputPath string, opts BuildOptions) error

var registeredLLVMCompiler ExternalLLVMCompiler

// RegisterExternalLLVMCompiler registers the optional external LLVM+LLD driver.
func RegisterExternalLLVMCompiler(fn ExternalLLVMCompiler) {
	registeredLLVMCompiler = fn
}

// NativeCompiler gestiona el estado del compilador LLVM externo opcional.
// El compilador nativo integrado de Joss es 100% autocontenido y no importa os/exec.
type NativeCompiler struct{}

func NewNativeCompiler() *NativeCompiler {
	return &NativeCompiler{}
}

// HasNativeToolchain devuelve true si el driver opcional pkg/backend/llvm registró clang.
func (c *NativeCompiler) HasNativeToolchain() bool {
	return registeredLLVMCompiler != nil
}

// CompileLLVM delega al compilador LLVM externo registrado si está disponible.
func (c *NativeCompiler) CompileLLVM(llvmIR string, outputPath string) error {
	return c.CompileLLVMWithOptions(llvmIR, outputPath, BuildOptions{})
}

// CompileLLVMWithOptions delega al compilador LLVM+LLD externo registrado.
func (c *NativeCompiler) CompileLLVMWithOptions(llvmIR string, outputPath string, opts BuildOptions) error {
	if registeredLLVMCompiler == nil {
		return fmt.Errorf("native compiler: el backend externo 'llvm' no está habilitado sin toolchain externo")
	}
	return registeredLLVMCompiler(llvmIR, outputPath, opts)
}
