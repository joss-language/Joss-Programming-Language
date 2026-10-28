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
	BackendNative     BackendKind = "native"
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
	GUI           bool
}

// BuildResult contiene la información del ejecutable generado y artefactos de trace.
type BuildResult struct {
	Target          Target
	BackendUsed     BackendKind
	IsBootstrap     bool
	NeedsHostBridge bool
	ToolchainNotice string
	OutputPath      string
	ExecutableSize  int64
	IRDumpPath      string
	LLVMPath        string
	ASMPath         string
	BootstrapPath   string
}

// NativeCodeEmitter define el contrato que deben cumplir los generadores de código máquina por arquitectura.
type NativeCodeEmitter interface {
	Compile(prog *ir.Program) ([]byte, string, error)
	NeedsHostBridge() bool
}

// SelectCodeEmitter devuelve el generador nativo correspondiente a la arquitectura objetivo.
func SelectCodeEmitter(target Target, guiMode bool) NativeCodeEmitter {
	if target.Arch == "arm64" || target.Arch == "aarch64" {
		return NewARM64Builder(target, guiMode)
	}
	return NewMachineCodeBuilder(target, guiMode)
}

// BuildProgram compila un ir.Program directamente a código máquina nativo (PE32+ / ELF64)
// sin invocar compiladores externos (sin Go, sin GCC, sin Clang, sin MSVC).
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
			outExe = filepath.Join("build", baseName+".exe")
		} else {
			outExe = filepath.Join("build", baseName)
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

		// Emisión LLVM IR (.ll) para inspección
		llvmEmitter := NewLLVMEmitter()
		llvmText, err := llvmEmitter.Emit(prog)
		if err == nil {
			llFile := filepath.Join(traceDir, baseName+".ll")
			_ = os.WriteFile(llFile, []byte(llvmText), 0644)
			res.LLVMPath = llFile
		}
	}

	// 4. Selección del backend (por defecto: BackendNative autocontenido)
	selectedBackend := opts.Backend
	if selectedBackend == "" || selectedBackend == BackendAuto {
		selectedBackend = BackendNative
	}
	res.BackendUsed = selectedBackend

	// 5. Compilación según el backend seleccionado
	switch selectedBackend {
	case BackendNative, BackendStandalone:
		mcBuilder := SelectCodeEmitter(target, opts.GUI)
		binBytes, asmListing, err := mcBuilder.Compile(prog)
		if err != nil {
			return nil, err
		}
		if opts.Trace {
			asmFile := filepath.Join(traceDir, baseName+".asm")
			_ = os.WriteFile(asmFile, []byte(asmListing), 0644)
			res.ASMPath = asmFile
			if selectedBackend == BackendStandalone {
				res.BootstrapPath = asmFile
			}
		}
		if err := os.MkdirAll(filepath.Dir(outExe), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(outExe, binBytes, 0755); err != nil {
			return nil, fmt.Errorf("no se pudo escribir ejecutable nativo %s: %w", outExe, err)
		}

		res.NeedsHostBridge = mcBuilder.NeedsHostBridge()
		if mcBuilder.NeedsHostBridge() && target.OS == "windows" {
			// Ensure joss_host.dll is placed alongside the generated executable
			targetDll := filepath.Join(filepath.Dir(outExe), "joss_host.dll")
			// Try locating joss_host.dll from build directory or runtime location
			builtDllLocations := []string{
				"pkg/backend/native/abi/build/joss_host.dll",
				"../pkg/backend/native/abi/build/joss_host.dll",
				"../../pkg/backend/native/abi/build/joss_host.dll",
			}
			if selfExe, exeErr := os.Executable(); exeErr == nil {
				selfDir := filepath.Dir(selfExe)
				builtDllLocations = append(builtDllLocations,
					filepath.Join(selfDir, "joss_host.dll"),
					filepath.Join(selfDir, "pkg", "backend", "native", "abi", "build", "joss_host.dll"),
					filepath.Join(selfDir, "..", "pkg", "backend", "native", "abi", "build", "joss_host.dll"),
				)
			}
			var dllBytes []byte
			for _, loc := range builtDllLocations {
				if b, readErr := os.ReadFile(loc); readErr == nil {
					dllBytes = b
					break
				}
			}
			if len(dllBytes) > 0 {
				_ = os.WriteFile(targetDll, dllBytes, 0755)
			}
		}

	case BackendLLVM:
		compiler := NewNativeCompiler()
		if !compiler.HasNativeToolchain() {
			return nil, fmt.Errorf("el backend externo 'llvm' no está habilitado sin toolchain externo (clang/gcc); usa el compilador nativo integrado de 'joss build'")
		}
		llvmEmitter := NewLLVMEmitter()
		llvmIR, err := llvmEmitter.Emit(prog)
		if err != nil {
			return nil, fmt.Errorf("error generando LLVM IR: %w", err)
		}
		if err := compiler.CompileLLVMWithOptions(llvmIR, outExe, opts); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("backend desconocido: %s (opciones válidas: auto, native, standalone, llvm)", selectedBackend)
	}

	// 6. Obtener tamaño del ejecutable
	if fi, err := os.Stat(outExe); err == nil {
		res.ExecutableSize = fi.Size()
	}

	return res, nil
}
