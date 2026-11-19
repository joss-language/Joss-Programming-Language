package native

import (
	"os"
	"path/filepath"

	"github.com/jossecurity/joss/pkg/ir"
)

// StandaloneBuilder produce un ejecutable nativo puro (PE32+ / ELF64) directamente
// desde Joss Native IR usando el generador de código máquina integrado, sin invocar
// 'go build', 'gcc', 'clang' ni ningún otro compilador externo.
type StandaloneBuilder struct{}

func NewStandaloneBuilder() *StandaloneBuilder {
	return &StandaloneBuilder{}
}

// GenerateSource devuelve el listado de ensamblador x86_64 nativo generado desde Joss IR
// para inspección y auditoría de trazabilidad.
func (b *StandaloneBuilder) GenerateSource(prog *ir.Program) string {
	mc := NewMachineCodeBuilder(HostTarget(), false)
	_, asmListing, err := mc.Compile(prog)
	if err != nil {
		return "; error generating assembly listing: " + err.Error()
	}
	return asmListing
}

// BuildNativeExecutable compila un ir.Program directamente a un binario nativo en la máquina host.
func (b *StandaloneBuilder) BuildNativeExecutable(prog *ir.Program, outputPath string) error {
	return b.BuildNativeExecutableWithTarget(prog, outputPath, HostTarget())
}

// BuildNativeExecutableWithTarget compila un ir.Program al target específico (cross-compilation).
func (b *StandaloneBuilder) BuildNativeExecutableWithTarget(prog *ir.Program, outputPath string, target Target) error {
	return b.BuildNativeExecutableWithOptions(prog, outputPath, BuildOptions{
		Target:     target,
		OutputPath: outputPath,
	})
}

// BuildNativeExecutableWithOptions compila un ir.Program directamente a código máquina nativo
// sin requerir herramientas externas instaladas.
func (b *StandaloneBuilder) BuildNativeExecutableWithOptions(prog *ir.Program, outputPath string, opts BuildOptions) error {
	target := opts.Target
	if target.OS == "" && target.Arch == "" {
		target = HostTarget()
	}

	if target.OS == "darwin" && NewNativeCompiler().HasNativeToolchain() {
		opts.Target = target
		opts.OutputPath = outputPath
		opts.Backend = BackendLLVM
		_, err := BuildProgram(prog, opts)
		return err
	}

	// Native output is a single standalone executable. Runtime calls must be
	// lowered to emitted code or rejected by the selected backend; no bridge DLL
	// is linked or inferred here.
	mc := SelectCodeEmitter(target, opts.GUI)
	binBytes, asmSource, err := mc.Compile(prog)
	if err != nil {
		return err
	}

	_ = asmSource
	if err != nil {
		return err
	}
	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, binBytes, 0755)
}
