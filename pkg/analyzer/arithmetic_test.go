package analyzer

import (
	"testing"

	"github.com/jossecurity/joss/pkg/diagnostics"
)

func TestAnalyzerDetectsConstantIntegerOverflow(t *testing.T) {
	items := analyzeSource(t, `$value = 9223372036854775807 + 1`, NewEnvironment())
	if !hasCode(items, diagnostics.CodeArithmeticOverflow) {
		t.Fatalf("expected constant overflow diagnostic, got %#v", items)
	}
}

func TestAnalyzerAllowsExactLargeIntegerArithmetic(t *testing.T) {
	items := analyzeSource(t, `$value = 9007199254740993 + 1 echo $value`, NewEnvironment())
	if hasCode(items, diagnostics.CodeArithmeticOverflow) {
		t.Fatalf("exact in-range integer was rejected: %#v", items)
	}
}

func TestAnalyzerDetectsConstantDivisionByZero(t *testing.T) {
	items := analyzeSource(t, `$value = 1 / 0`, NewEnvironment())
	if !hasCode(items, diagnostics.CodeDivisionByZero) {
		t.Fatalf("expected division-by-zero diagnostic, got %#v", items)
	}
}

func TestAnalyzerRejectsLossyImplicitNumericConversions(t *testing.T) {
	tests := []string{
		`float $value = 9007199254740993`,
		`decimal $value = 0.1`,
		`var $value = 1 + 0.5`,
		`var $value = 0.5 + 1.00m`,
		`var $value = 9007199254740993 / 1`,
		`var $value = 1 < 0.5`,
	}
	for _, source := range tests {
		items := analyzeSource(t, source, NewEnvironment())
		if !hasCode(items, diagnostics.CodePrecisionLoss) && !hasCode(items, "JOSS-TYPE-002") {
			t.Errorf("expected unsafe numeric conversion diagnostic for %q, got %#v", source, items)
		}
	}
}

func TestAnalyzerAllowsExactNumericContracts(t *testing.T) {
	items := analyzeSource(t, `decimal $money = 10 var $ratio = 1.5 + 0.5 var $total = $money + 2`, NewEnvironment())
	if hasCode(items, diagnostics.CodePrecisionLoss) || hasCode(items, "JOSS-TYPE-002") {
		t.Fatalf("safe numeric expressions were rejected: %#v", items)
	}
}
