package analyzer

import "testing"

func TestTypedCollectionIndexAssignmentChecksElement(t *testing.T) {
	bad := analyzeSource(t, `array<int> $items = [1]
$items[0] = "wrong"`, NewEnvironment())
	if !hasCode(bad, "JOSS-TYPE-001") {
		t.Fatalf("expected element mismatch, got %#v", bad)
	}
	good := analyzeSource(t, `array<int> $items = [1]
$items[0] = 2`, NewEnvironment())
	if hasCode(good, "JOSS-TYPE-001") {
		t.Fatalf("valid element mutation rejected: %#v", good)
	}

	nestedBad := analyzeSource(t, `array<array<int>> $matrix = [[1]]
$matrix[0][0] = "bad"`, NewEnvironment())
	if !hasCode(nestedBad, "JOSS-TYPE-001") {
		t.Fatalf("expected nested element mismatch, got %#v", nestedBad)
	}

	nestedGood := analyzeSource(t, `array<array<int>> $matrix = [[1]]
$matrix[0][0] = 42`, NewEnvironment())
	if hasCode(nestedGood, "JOSS-TYPE-001") {
		t.Fatalf("valid nested mutation rejected: %#v", nestedGood)
	}
}
