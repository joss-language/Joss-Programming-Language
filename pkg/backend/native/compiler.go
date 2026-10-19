package native

import (
	"fmt"
)

// NativeCompiler representa el estado de toolchain LLVM externo (deshabilitado por defecto
// en favor del generador de código máquina integrado de Joss).
type NativeCompiler struct {
	ClangPath string
}

func NewNativeCompiler() *NativeCompiler {
	// El compilador nativo oficial de Joss es 100% autocontenido y no invoca compiladores externos.
	return &NativeCompiler{ClangPath: ""}
}

// HasNativeToolchain devuelve false porque joss build no depende de clang/gcc externos.
func (c *NativeCompiler) HasNativeToolchain() bool {
	return false
}

// CompileLLVMWithOptions rechaza el uso de clang/gcc externos conforme a la arquitectura autocontenida.
func (c *NativeCompiler) CompileLLVMWithOptions(llvmIR string, outputPath string, opts BuildOptions) error {
	return fmt.Errorf("native compiler: la invocación de compiladores externos (clang o gcc) está deshabilitada; usa el backend nativo integrado de Joss")
}
