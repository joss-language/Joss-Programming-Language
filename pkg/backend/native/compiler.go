package native

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	nativeruntime "github.com/jossecurity/joss/runtime/native"
)

// NativeCompiler orquesta la compilación de LLVM IR a código máquina y su enlace con joss_rt.
type NativeCompiler struct {
	ClangPath string
}

func NewNativeCompiler() *NativeCompiler {
	clang, _ := exec.LookPath("clang")
	return &NativeCompiler{ClangPath: clang}
}

// HasNativeToolchain indica si un compilador/linker C/LLVM está disponible en el PATH del sistema.
func (c *NativeCompiler) HasNativeToolchain() bool {
	return c.ClangPath != ""
}

// CompileLLVMToFile compila un módulo LLVM IR (.ll) directamente a un binario ejecutable nativo.
func (c *NativeCompiler) CompileLLVMToFile(llvmIR string, outputPath string, runtimeSourcePath string) error {
	return c.CompileLLVMWithOptions(llvmIR, outputPath, BuildOptions{
		OutputPath:    outputPath,
		RuntimeSource: runtimeSourcePath,
	})
}

// CompileLLVMWithOptions compila un módulo LLVM IR (.ll) usando las opciones completas de compilación.
func (c *NativeCompiler) CompileLLVMWithOptions(llvmIR string, outputPath string, opts BuildOptions) error {
	if !c.HasNativeToolchain() {
		return fmt.Errorf("native compiler: no se encontró clang o gcc en PATH")
	}

	tempDir, err := os.MkdirTemp("", "joss-native-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	llFile := filepath.Join(tempDir, "module.ll")
	if err := os.WriteFile(llFile, []byte(llvmIR), 0644); err != nil {
		return err
	}

	runtimeSourcePath := opts.RuntimeSource
	if runtimeSourcePath == "" {
		cFile := filepath.Join(tempDir, "joss_rt.c")
		hFile := filepath.Join(tempDir, "joss_rt.h")
		if err := os.WriteFile(cFile, nativeruntime.SourceC, 0644); err != nil {
			return err
		}
		if err := os.WriteFile(hFile, nativeruntime.HeaderH, 0644); err != nil {
			return err
		}
		runtimeSourcePath = cFile
	}

	if outputPath == "" {
		outputPath = "a.out"
		if runtime.GOOS == "windows" {
			outputPath = "a.exe"
		}
	}

	optFlag := "-O2"
	if opts.Release {
		optFlag = "-O3"
	} else if opts.Debug {
		optFlag = "-O0"
	}

	args := []string{
		optFlag,
		llFile,
		runtimeSourcePath,
		"-o", outputPath,
	}
	if opts.Debug {
		args = append([]string{"-g"}, args...)
	}

	cmd := exec.Command(c.ClangPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fallo enlace nativo con %s: %v\nSalida:\n%s", c.ClangPath, err, string(out))
	}

	return nil
}
