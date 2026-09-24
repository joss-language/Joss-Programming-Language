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
