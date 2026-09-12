package pluginruntime

import (
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/pluginpkg"
)

// BytecodeFormat define el formato binario interno de main.jbc.
type BytecodeFormat string

const (
	FormatJossAST BytecodeFormat = "JOSSBC2Z" // Bytecode nativo Joss (AST Gob comprimido)
	FormatJPBC    BytecodeFormat = "JPBC"     // Bytecode multilenguaje (Joss Plugin ByteCode)
)

// ASTEngine define el contrato para que el runtime de Joss ejecute programas AST decodificados.
type ASTEngine interface {
	RegisterProgram(prog *parser.Program) error
	CallFunction(fnName string, args []interface{}) (interface{}, error)
	Instantiate(className string, args []interface{}) (interface{}, error)
	CallMethod(instance interface{}, methodName string, args []interface{}) (interface{}, error)
}

// PluginAwareHost allows host runtimes to execute AST plugins within their own
// execution frame and runtime instance, preventing cross-runtime pollution and
// avoiding ambiguous global symbol resolution when multiple plugins define matching names.
type PluginAwareHost interface {
	HostContext
	CallPluginAST(pluginName, fnName string, args []interface{}) (interface{}, error)
	InstantiatePluginAST(pluginName, className string, args []interface{}) (interface{}, error)
	CallPluginASTMethod(pluginName, className, methodName string, instance interface{}, args []interface{}) (interface{}, error)
}

// Plugin representa un paquete .jp cargado, verificado y preparado para ejecución.
type Plugin struct {
	Name        string
	Version     string
	Language    string
	Format      BytecodeFormat
	Metadata    pluginpkg.Metadata
	Symbols     pluginpkg.SymbolIndex
	Manifest    string
	RawBytecode []byte

	// Ejecutor para plugins nativos Joss (AST)
	jossProgram  *parser.Program
	jossExecutor *JossASTExecutor

	// Ejecutor para plugins multilenguaje (JPBC)
	jpbcModule *JPBCModule
}

// DetectBytecodeFormat identifica el formato de bytecode inspeccionando los primeros bytes.
func DetectBytecodeFormat(data []byte) (BytecodeFormat, error) {
	if len(data) >= 8 && string(data[:8]) == "JOSSBC2Z" {
		return FormatJossAST, nil
	}
	if len(data) >= 4 && string(data[:4]) == "JPBC" {
		return FormatJPBC, nil
	}
	return "", ErrInvalidBytecodeHeader
}

func (p *Plugin) Program() *parser.Program {
	return p.jossProgram
}

func (p *Plugin) JPBCModule() *JPBCModule {
	return p.jpbcModule
}
