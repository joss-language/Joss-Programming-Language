package analyzer

import (
	"testing"
)

func TestDefiniteAssignmentRejectsUseBeforeInit(t *testing.T) {
	items := analyzeSource(t, `
public func compute(): int {
    int $x
    return $x
}
`, NewEnvironment())
	if !hasCode(items, "JOSS-SYM-001") {
		t.Fatalf("expected JOSS-SYM-001 for use before initialization, got %#v", items)
	}
}

func TestDefiniteAssignmentAcceptsAfterAssignment(t *testing.T) {
	items := analyzeSource(t, `
public func compute(): int {
    int $x
    $x = 42
    return $x
}
`, NewEnvironment())
	if hasCode(items, "JOSS-SYM-001") {
		t.Fatalf("expected no JOSS-SYM-001 after assignment, got %#v", items)
	}
}

func TestDefiniteAssignmentRejectsPartialBranchInit(t *testing.T) {
	items := analyzeSource(t, `
public func compute(bool $cond): int {
    int $x
    $cond ? { $x = 10 } : { echo "not assigned" }
    return $x
}
`, NewEnvironment())
	if !hasCode(items, "JOSS-SYM-001") {
		t.Fatalf("expected JOSS-SYM-001 for partial branch initialization, got %#v", items)
	}
}

func TestDefiniteAssignmentAcceptsBothBranchesInit(t *testing.T) {
	items := analyzeSource(t, `
public func compute(bool $cond): int {
    int $x
    $cond ? { $x = 10 } : { $x = 20 }
    return $x
}
`, NewEnvironment())
	if hasCode(items, "JOSS-SYM-001") {
		t.Fatalf("expected no JOSS-SYM-001 when both branches initialize, got %#v", items)
	}
}

func TestDefiniteAssignmentAcceptsBranchInitWithEarlyReturn(t *testing.T) {
	items := analyzeSource(t, `
public func compute(bool $cond): int {
    int $x
    $cond ? { return 0 } : { $x = 20 }
    return $x
}
`, NewEnvironment())
	if hasCode(items, "JOSS-SYM-001") {
		t.Fatalf("expected no JOSS-SYM-001 when non-initializing branch terminates, got %#v", items)
	}
}

func TestNarrowingInvalidationOnReassignment(t *testing.T) {
	items := analyzeSource(t, `
public func testNarrowing(string|null $val): string {
    $val != null ? {
        $val = null
        return $val
    } : {}
    return "default"
}
`, NewEnvironment())
	// When $val is reassigned null, its narrowed type 'string' is invalidated,
	// returning $val which is now null when string is expected emits JOSS-TYPE-008.
	if !hasCode(items, "JOSS-TYPE-008") {
		t.Fatalf("expected JOSS-TYPE-008 because narrowed variable was reassigned to null, got %#v", items)
	}
}

func TestMatchEnumExhaustivenessWarning(t *testing.T) {
	src := `
public enum Status {
    case Pending;
    case Active;
    case Archived;
}

public func handleStatus(Status $s): string {
    return match ($s) {
        Status::Pending => "waiting",
        Status::Active => "running"
    }
}
`
	items := analyzeSource(t, src, NewEnvironment())
	if !hasCode(items, "JOSS-FLOW-002") {
		t.Fatalf("expected JOSS-FLOW-002 for non-exhaustive enum match, got %#v", items)
	}
}

func TestMatchEnumExhaustivenessWithDefaultOrAllCases(t *testing.T) {
	srcAll := `
public enum Status {
    case Pending;
    case Active;
    case Archived;
}

public func handleStatus(Status $s): string {
    return match ($s) {
        Status::Pending => "waiting",
        Status::Active => "running",
        Status::Archived => "done"
    }
}
`
	items := analyzeSource(t, srcAll, NewEnvironment())
	if hasCode(items, "JOSS-FLOW-002") {
		t.Fatalf("did not expect JOSS-FLOW-002 when all enum cases are covered, got %#v", items)
	}

	srcDefault := `
public enum Status {
    case Pending;
    case Active;
    case Archived;
}

public func handleStatus(Status $s): string {
    return match ($s) {
        Status::Pending => "waiting",
        default => "other"
    }
}
`
	itemsDef := analyzeSource(t, srcDefault, NewEnvironment())
	if hasCode(itemsDef, "JOSS-FLOW-002") {
		t.Fatalf("did not expect JOSS-FLOW-002 when default arm is present, got %#v", itemsDef)
	}
}
