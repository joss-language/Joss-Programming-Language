package native

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jossecurity/joss/pkg/ir"
)

// BackendKind define los tipos de backend nativo soportados.
type BackendKind string

const (
	BackendAuto       BackendKind = "auto"
	BackendLLVM       BackendKind = "llvm"
	BackendStandalone BackendKind = "standalone"
)

// BuildOptions configura la compilación nativa desde Joss Native IR.
type BuildOptions struct {
	Target        Target
	Backend       BackendKind
	OutputPath    string
	Trace         bool
	TraceDir      string
	RuntimeSource string
	Verbose       bool
	Debug         bool
	Release       bool
}

// BuildResult contiene la información del ejecutable generado y artefactos de trace.
type BuildResult struct {
	Target          Target
	BackendUsed     BackendKind
	IsBootstrap     bool
	ToolchainNotice string
	OutputPath      string
	ExecutableSize  int64
	IRDumpPath      string
	LLVMPath        string
	BootstrapPath   string
}

// BuildProgram compila un ir.Program al backend seleccionado, verificando invariantes y
// generando opcionalmente artefactos intermedios (.ir, .ll, .standalone.go) si Trace está activo.
func BuildProgram(prog *ir.Program, opts BuildOptions) (*BuildResult, error) {
	if prog == nil {
		return nil, fmt.Errorf("build native: programa IR nulo")
	}

	// 1. Verificación de la IR
	verifier := ir.NewVerifier()
	if err := verifier.Verify(prog); err != nil {
		return nil, fmt.Errorf("verificación de IR fallida: %w", err)
	}

	// 2. Resolver target y rutas de salida
	target := opts.Target
	if target.OS == "" {
		target = HostTarget()
	}

	baseName := prog.Name
	if baseName == "" {
		baseName = "app"
	}

	outExe := opts.OutputPath
	if outExe == "" {
		if target.OS == "windows" {
			outExe = baseName + ".exe"
		} else {
			outExe = baseName
		}
	} else if target.OS == "windows" && !strings.HasSuffix(strings.ToLower(outExe), ".exe") {
		outExe += ".exe"
	}

	traceDir := opts.TraceDir
	if traceDir == "" {
		traceDir = filepath.Dir(outExe)
		if traceDir == "" || traceDir == "." {
			traceDir = "."
		}
	}

	res := &BuildResult{
		Target:     target,
		OutputPath: outExe,
	}

	if baseName == "app" {
		baseName = strings.TrimSuffix(filepath.Base(outExe), filepath.Ext(outExe))
	}

	// 3. Generación y retención de artefactos intermedios si Trace está activo
	if opts.Trace {
		if err := os.MkdirAll(traceDir, 0755); err != nil {
			return nil, fmt.Errorf("no se pudo crear directorio de trace: %w", err)
		}

		// Volcado textual de Joss IR
		irText := prog.Dump()
		irFile := filepath.Join(traceDir, baseName+".ir")
		if err := os.WriteFile(irFile, []byte(irText), 0644); err != nil {
			return nil, fmt.Errorf("no se pudo guardar volcado de IR en trace: %w", err)
		}
		res.IRDumpPath = irFile

		// Emisión LLVM IR (.ll)
		llvmEmitter := NewLLVMEmitter()
		llvmText, err := llvmEmitter.Emit(prog)
		if err == nil {
			llFile := filepath.Join(traceDir, baseName+".ll")
			_ = os.WriteFile(llFile, []byte(llvmText), 0644)
			res.LLVMPath = llFile
		}
	}

	// 4. Selección del backend
	compiler := NewNativeCompiler()
	selectedBackend := opts.Backend
	if selectedBackend == "" || selectedBackend == BackendAuto {
		if compiler.HasNativeToolchain() {
			selectedBackend = BackendLLVM
		} else {
			selectedBackend = BackendStandalone
			res.IsBootstrap = true
			res.ToolchainNotice = "Toolchain nativo primario (LLVM/Clang) no encontrado en PATH; utilizando backend auxiliar bootstrap (Standalone)."
		}
	} else if selectedBackend == BackendStandalone {
		res.IsBootstrap = true
		res.ToolchainNotice = "Backend bootstrap (Standalone) solicitado explícitamente."
	}

	res.BackendUsed = selectedBackend

	// 5. Compilación según el backend seleccionado
	switch selectedBackend {
	case BackendLLVM:
		if !compiler.HasNativeToolchain() {
			return nil, fmt.Errorf("el backend 'llvm' requiere un compilador C/LLVM (clang o gcc) en PATH.\nPara compilar sin dependencias externas use '--backend=standalone'")
		}
		llvmEmitter := NewLLVMEmitter()
		llvmIR, err := llvmEmitter.Emit(prog)
		if err != nil {
			return nil, fmt.Errorf("error generando LLVM IR: %w", err)
		}
		if err := compiler.CompileLLVMWithOptions(llvmIR, outExe, opts); err != nil {
			return nil, err
		}

	case BackendStandalone:
		standalone := NewStandaloneBuilder()
		if opts.Trace {
			goSource := standalone.GenerateSource(prog)
			goFile := filepath.Join(traceDir, baseName+".standalone.go")
			_ = os.WriteFile(goFile, []byte(goSource), 0644)
			res.BootstrapPath = goFile
		}
		if err := standalone.BuildNativeExecutableWithOptions(prog, outExe, opts); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("backend desconocido: %s (opciones válidas: auto, llvm, standalone)", selectedBackend)
	}

	// 6. Obtener tamaño del ejecutable
	if fi, err := os.Stat(outExe); err == nil {
		res.ExecutableSize = fi.Size()
	}

	return res, nil
}
