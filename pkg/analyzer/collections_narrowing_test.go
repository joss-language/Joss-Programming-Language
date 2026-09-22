package analyzer

import (
	"testing"
)

func TestTypedCollectionsAnalyzerInference(t *testing.T) {
	items := analyzeSource(t, `
array<int> $numbers = [1, 2, 3]
int $_first = $numbers[0]
`, NewEnvironment())
	if len(items) > 0 {
		t.Fatalf("expected no errors for valid typed array index access, got %#v", items)
	}

	badItems := analyzeSource(t, `
array<int> $_nums = ["hello"]
`, NewEnvironment())
	if !hasCode(badItems, "JOSS-TYPE-002") {
		t.Fatalf("expected type mismatch error assigning array<string> to array<int>, got %#v", badItems)
	}
}

func TestTypeNarrowingInTernaryBlocks(t *testing.T) {
	items := analyzeSource(t, `
public class User {
    public string $name = "Alice"
}
public func getUser(): User|null {
    return null
}
public func test(): string {
    User|null $u = getUser()
    return $u != null ? {
        return $u->name
    } : {
        return "default"
    }
}
`, NewEnvironment())
	if len(items) > 0 {
		t.Fatalf("expected clean analysis with type narrowing, got diagnostics: %#v", items)
	}
}

func TestTypeNarrowingEqualityBranch(t *testing.T) {
	items := analyzeSource(t, `
public class Profile {
    public string $email = "test@example.com"
}
public func getProfile(): Profile|null {
    return null
}
public func run(): string {
    Profile|null $p = getProfile()
    return $p == null ? {
        return "no email"
    } : {
        return $p->email
    }
}
`, NewEnvironment())
	if len(items) > 0 {
		t.Fatalf("expected clean analysis with equality narrowing, got diagnostics: %#v", items)
	}
}

func TestCollectionCovarianceAndAliasingCharacterization(t *testing.T) {
	// Characterization test for Phase 2 collection soundness:
	// Untyped collections act as boundary for dynamic values.
	// Assigning array to array<int> is allowed, but direct typed mismatch is caught.
	typedAssign := analyzeSource(t, `
array<string> $strs = ["a", "b"]
array<int> $nums = $strs
`, NewEnvironment())
	if !hasCode(typedAssign, "JOSS-TYPE-002") {
		t.Fatalf("expected JOSS-TYPE-002 when assigning array<string> to array<int>, got %#v", typedAssign)
	}

	// Map key/element covariance characterization:
	mapAssign := analyzeSource(t, `
map<string, string> $m1 = {"k": "v"}
map<string, int> $m2 = $m1
`, NewEnvironment())
	if !hasCode(mapAssign, "JOSS-TYPE-002") {
		t.Fatalf("expected JOSS-TYPE-002 when assigning map<string, string> to map<string, int>, got %#v", mapAssign)
	}
}

