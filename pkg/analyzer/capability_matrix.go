package analyzer

import "fmt"

// SupportStatus represents whether a feature is supported in a particular execution layer or backend.
type SupportStatus string

const (
	Supported   SupportStatus = "supported"
	Unsupported SupportStatus = "not currently supported"
	Planned     SupportStatus = "planned"
)

// CompletionState models the formal lifecycle status of a language feature across compiler tiers.
type CompletionState string

const (
	StateLanguageComplete    CompletionState = "LANGUAGE-COMPLETE"    // Joss understands it semantically
	StateInterpreterComplete CompletionState = "INTERPRETER-COMPLETE" // Interpreter can execute it
	StateServerComplete      CompletionState = "SERVER-COMPLETE"      // Server can execute it
	StateNativeComplete      CompletionState = "NATIVE-COMPLETE"      // Native can materialize it
	StateFullyComplete       CompletionState = "FULLY-COMPLETE"       // All applicable execution modes support it
)

// FeatureOwnership specifies the authoritative owner of each phase of a feature.
type FeatureOwnership struct {
	SemanticOwner           string // Component determining semantic truth (e.g. "pkg/analyzer")
	CanonicalRepresentation string // Canonical structure in semantic model (e.g. "PreparedProgram.Facts.ResolvedCalls")
	InterpreterOwner        string // Component materializing runtime execution (e.g. "pkg/core")
	ServerOwner             string // Component materializing server execution (e.g. "pkg/server")
	NativeOwner             string // Component materializing machine code (e.g. "pkg/ir -> pkg/backend/native")
	DifferentialTest        string // Test suite verifying byte-by-byte behavioral equivalence (e.g. "tests/native")
}

// FeatureCategory classifies a capability into its proper architectural layer.
type FeatureCategory string

const (
	CategoryCoreLanguage    FeatureCategory = "CORE LANGUAGE"
	CategoryLanguageRuntime FeatureCategory = "LANGUAGE RUNTIME"
	CategoryStandardLibrary FeatureCategory = "STANDARD LIBRARY"
	CategoryWebRuntime      FeatureCategory = "WEB RUNTIME"
	CategoryDatabaseORM     FeatureCategory = "DATABASE/ORM"
	CategoryPackaging       FeatureCategory = "PACKAGING"
	CategoryTooling         FeatureCategory = "TOOLING"
)

// CapabilityEntry represents the support status and architectural ownership of a Joss language feature across all tiers.
type CapabilityEntry struct {
	Feature        string
	Category       FeatureCategory
	Language       SupportStatus // Semantic model / language specification
	SemanticModel  SupportStatus // Analyzer / PreparedProgram
	Interpreter    SupportStatus // Interpreter backend (pkg/core)
	Server         SupportStatus // Server backend (pkg/server)
	Native         SupportStatus // Native compiler backend (pkg/backend/native)
	DiagnosticCode string        // Diagnostic emitted if unsupported in native (e.g. "JOSS-NATIVE-001")
	Description    string
	Ownership      FeatureOwnership
	State          CompletionState
}

// CanonicalCapabilityMatrix contains the authoritative matrix of features across all execution forms.
var CanonicalCapabilityMatrix = []CapabilityEntry{
	{
		Feature:       "Functions",
		Category:      CategoryCoreLanguage,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Supported,
		Description:   "Funciones globales tipadas y llamadas con argumentos posicionales",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Environment.Functions",
			InterpreterOwner:        "pkg/core/executor.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go (@function)",
			DifferentialTest:        "tests/native/differential_test.go",
		},
		State: StateFullyComplete,
	},
	{
		Feature:       "Primitives",
		Category:      CategoryCoreLanguage,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Supported,
		Description:   "Tipos primitivos int, float, bool, string y operaciones aritméticas nativas",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/typesystem",
			CanonicalRepresentation: "typesystem.Type (Int, Float, Bool, String)",
			InterpreterOwner:        "pkg/core/evaluator_numeric.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/types.go",
			DifferentialTest:        "tests/native/differential_test.go",
		},
		State: StateFullyComplete,
	},
	{
		Feature:       "ControlFlow",
		Category:      CategoryCoreLanguage,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Supported,
		Description:   "Bifurcaciones (guard, ternario ? :) y bucles (while, do-while)",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Units (AST)",
			InterpreterOwner:        "pkg/core/evaluator_control.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go (BranchTerminator, JumpTerminator)",
			DifferentialTest:        "tests/native/differential_test.go",
		},
		State: StateFullyComplete,
	},
	{
		Feature:       "Recursion",
		Category:      CategoryCoreLanguage,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Supported,
		Description:   "Llamadas recursivas a funciones",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Facts.ResolvedCalls",
			InterpreterOwner:        "pkg/core/call_method.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go (CallInst)",
			DifferentialTest:        "tests/native/differential_test.go",
		},
		State: StateFullyComplete,
	},
	{
		Feature:       "ConsoleIO",
		Category:      CategoryStandardLibrary,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Supported,
		Description:   "Impresión a salida estándar (echo, print)",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Units (*parser.EchoStatement)",
			InterpreterOwner:        "pkg/core/builtins_string.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go (CallRuntimeInst -> print_i64, print_string)",
			DifferentialTest:        "tests/native/differential_test.go",
		},
		State: StateFullyComplete,
	},
	{
		Feature:       "StaticClassMethods",
		Category:      CategoryCoreLanguage,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Supported,
		Description:   "Métodos estáticos y de utilidad en clases (Class::method)",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Environment.Classes + ResolvedCalls",
			InterpreterOwner:        "pkg/core/executor.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go (@ClassName_method)",
			DifferentialTest:        "tests/native/differential_test.go",
		},
		State: StateFullyComplete,
	},
	{
		Feature:       "SpaceshipOperator",
		Category:      CategoryCoreLanguage,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Supported,
		Description:   "Operador de comparación de tres vías (<=>)",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Facts.InferredTypes (<=> returns int)",
			InterpreterOwner:        "pkg/core/evaluator_compare.go (spaceshipCompare)",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go (OpSpaceship -> CompareInst)",
			DifferentialTest:        "tests/native/differential_test.go",
		},
		State: StateFullyComplete,
	},
	{
		Feature:       "DynamicClasses",
		Category:      CategoryCoreLanguage,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Supported,
		Description:   "Instanciación dinámica en heap (new), propiedades, métodos de instancia y mutación de estado",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Environment.Classes",
			InterpreterOwner:        "pkg/core/evaluator_member.go (Instance)",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go (NewObjectInst, LoadFieldInst, StoreFieldInst)",
			DifferentialTest:        "tests/native/differential_test.go",
		},
		State: StateFullyComplete,
	},
	{
		Feature:       "DynamicArrays",
		Category:      CategoryCoreLanguage,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Supported,
		Description:   "Literales de arrays dinámicos ([]), indexación y mutaciones en runtime",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Units (*parser.ArrayLiteral)",
			InterpreterOwner:        "pkg/core/builtins_array.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go (NewArrayInst, ArrayGetInst, ArraySetInst)",
			DifferentialTest:        "tests/native/differential_test.go",
		},
		State: StateFullyComplete,
	},
	{
		Feature:       "DynamicMaps",
		Category:      CategoryCoreLanguage,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Supported,
		Description:   "Literales de mapas asociativos ({}), indexación por clave y mutaciones en runtime",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Units (*parser.MapLiteral)",
			InterpreterOwner:        "pkg/core/builtins_array.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go (NewMapInst, MapGetInst, MapSetInst)",
			DifferentialTest:        "tests/native/differential_test.go",
		},
		State: StateFullyComplete,
	},
	{
		Feature:        "Exceptions",
		Category:       CategoryCoreLanguage,
		Language:       Supported,
		SemanticModel:  Supported,
		Interpreter:    Supported,
		Server:         Supported,
		Native:         Supported,
		DiagnosticCode: "",
		Description:    "Manejo estructurado de excepciones (try, catch, throw)",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Units (*parser.TryCatchStatement, *parser.ThrowStatement)",
			InterpreterOwner:        "pkg/core/executor.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go (TryCatchInst, ThrowInst), pkg/backend/native/standalone.go",
			DifferentialTest:        "tests/native/differential_test.go",
		},
		State: StateFullyComplete,
	},
	{
		Feature:        "Interfaces",
		Category:       CategoryCoreLanguage,
		Language:       Supported,
		SemanticModel:  Supported,
		Interpreter:    Supported,
		Server:         Supported,
		Native:         Supported,
		DiagnosticCode: "",
		Description:    "Contratos nominales de interfaces con validación estática y despacho uniforme",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Environment.Interfaces",
			InterpreterOwner:        "pkg/core/nominal_contracts.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go",
			DifferentialTest:        "tests/native/differential_test.go",
		},
		State: StateFullyComplete,
	},
	{
		Feature:        "Channels",
		Category:       CategoryLanguageRuntime,
		Language:       Supported,
		SemanticModel:  Supported,
		Interpreter:    Supported,
		Server:         Supported,
		Native:         Unsupported,
		DiagnosticCode: "JOSS-NATIVE-001",
		Description:    "Canales tipados de concurrencia y select",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Units (*parser.SelectStatement)",
			InterpreterOwner:        "pkg/core/builtins_async.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go (JOSS-NATIVE-001)",
			DifferentialTest:        "tests/native/differential_test.go (Rejection test)",
		},
		State: StateInterpreterComplete,
	},
	{
		Feature:        "Defer",
		Category:       CategoryCoreLanguage,
		Language:       Supported,
		SemanticModel:  Supported,
		Interpreter:    Supported,
		Server:         Supported,
		Native:         Unsupported,
		DiagnosticCode: "JOSS-NATIVE-001",
		Description:    "Ejecución diferida de sentencias (defer)",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Units (*parser.DeferStatement)",
			InterpreterOwner:        "pkg/core/executor.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "pkg/ir/lower.go (JOSS-NATIVE-001)",
			DifferentialTest:        "tests/native/differential_test.go (Rejection test)",
		},
		State: StateInterpreterComplete,
	},
	{
		Feature:       "RoutesAndMVC",
		Category:      CategoryWebRuntime,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Unsupported,
		Description:   "Enrutamiento HTTP, controladores MVC y plantillas de vista",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Units (Routes)",
			InterpreterOwner:        "pkg/server/router.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "No aplica a binario CLI nativo",
			DifferentialTest:        "pkg/server/router_test.go",
		},
		State: StateServerComplete,
	},
	{
		Feature:       "GranDBORM",
		Category:      CategoryDatabaseORM,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Unsupported,
		Description:   "Consultas SQL dinámicas, modelos ORM y migraciones",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Environment.Classes (Model subclasses)",
			InterpreterOwner:        "pkg/core/model.go",
			ServerOwner:             "pkg/server",
			NativeOwner:             "No aplica a binario CLI nativo",
			DifferentialTest:        "pkg/core/model_test.go",
		},
		State: StateServerComplete,
	},
	{
		Feature:       "WebSockets",
		Category:      CategoryWebRuntime,
		Language:      Supported,
		SemanticModel: Supported,
		Interpreter:   Supported,
		Server:        Supported,
		Native:        Unsupported,
		Description:   "Actualización de conexiones a WebSocket y callbacks de eventos",
		Ownership: FeatureOwnership{
			SemanticOwner:           "pkg/analyzer",
			CanonicalRepresentation: "PreparedProgram.Units",
			InterpreterOwner:        "pkg/core/websocket.go",
			ServerOwner:             "pkg/server/websocket.go",
			NativeOwner:             "No aplica a binario CLI nativo",
			DifferentialTest:        "pkg/core/websocket_test.go",
		},
		State: StateServerComplete,
	},
}

// LookupCapability returns capability information for a feature name if known.
func LookupCapability(feature string) (CapabilityEntry, bool) {
	for _, entry := range CanonicalCapabilityMatrix {
		if entry.Feature == feature {
			return entry, true
		}
	}
	return CapabilityEntry{}, false
}

// GetFeatureOwnership queries the formal architectural owner of a language feature.
func GetFeatureOwnership(feature string) (FeatureOwnership, bool) {
	entry, ok := LookupCapability(feature)
	if !ok {
		return FeatureOwnership{}, false
	}
	return entry.Ownership, true
}

// IsNativeSupported reports whether the feature can currently be lowered to Native IR and binary.
func IsNativeSupported(feature string) bool {
	entry, ok := LookupCapability(feature)
	if !ok {
		return false
	}
	return entry.Native == Supported
}

// FormatCapabilityError formats the standardized diagnostic message when a language feature
// is supported by the language/semantic model and interpreter/server but not currently supported
// by the native compiler lowering.
func FormatCapabilityError(feature string) string {
	return fmt.Sprintf("[JOSS-NATIVE-001] [JOSS-NATIVE-CAPABILITY] Característica semántica no materializable en backend nativo:\n\nFeature:\n    %s\n\nLanguage:\n    supported\n\nInterpreter:\n    supported\n\nServer:\n    supported\n\nNative compiler:\n    not currently supported\n\nMotivo:\n    La característica existe y es válida en la semántica de Joss, pero el compilador nativo todavía no cuenta con la infraestructura física de runtime para materializarla.\n\nSugerencia: Ejecute el programa con 'joss run' para ejecución interpretada.", feature)
}
