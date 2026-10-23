package diagnostics

// Stable language error codes shared by static analysis and runtime defense.
// A runtime error may carry the same code as an analyzer diagnostic when both
// enforce the same semantic rule.
const (
	CodeArithmeticOverflow = "JOSS-ARITH-001"
	CodeDivisionByZero     = "JOSS-ARITH-002"
	CodePrecisionLoss      = "JOSS-ARITH-003"
	CodeIndexOutOfRange    = "JOSS-INDEX-001"
	CodeInvalidIndexType   = "JOSS-INDEX-002"
	CodeChannelClosed      = "JOSS-CHANNEL-001"
	CodeChannelCapacity    = "JOSS-CHANNEL-002"
)

// Códigos de `joss doctor storage`. Son hallazgos de configuración, no errores del lenguaje.
const (
	CodeStorageOK                  = "JOSS-STORAGE-000"
	CodeStorageUnknownProvider     = "JOSS-STORAGE-001"
	CodeStorageKey                 = "JOSS-STORAGE-002"
	CodeStorageFingerprint         = "JOSS-STORAGE-003"
	CodeStorageConfig              = "JOSS-STORAGE-004"
	CodeStorageAuth                = "JOSS-STORAGE-005"
	CodeStorageNamespace           = "JOSS-STORAGE-006"
	CodeStorageBucket              = "JOSS-STORAGE-007"
	CodeStorageLocalNotWritable    = "JOSS-STORAGE-008"
	CodeStorageProviderUnsupported = "JOSS-STORAGE-009"
)
