package analyzer

import (
	"testing"
)

func TestAnalyzer_ValidInterfaceImplementation(t *testing.T) {
	source := `
public interface IGreeter {
    public func greet(string $name): string;
}

public class SpanishGreeter implements IGreeter {
    public func greet(string $name): string {
        return "Hola " . $name;
    }
}
`
	items := analyzeSource(t, source, NewEnvironment())
	for _, item := range items {
		if item.Severity == "error" {
			t.Fatalf("unexpected error: %s", item.String())
		}
	}
}

func TestAnalyzer_InterfaceMissingMethod(t *testing.T) {
	source := `
public interface IGreeter {
    public func greet(string $name): string;
    public func farewell(): string;
}

public class SpanishGreeter implements IGreeter {
    public func greet(string $name): string {
        return "Hola " . $name;
    }
}
`
	items := analyzeSource(t, source, NewEnvironment())
	if !hasCode(items, "JOSS-DECL-005") {
		t.Fatalf("expected JOSS-DECL-005 for missing method, got %#v", items)
	}
}

func TestAnalyzer_InterfaceMethodParameterMismatch(t *testing.T) {
	source := `
public interface ICalculator {
    public func add(int $a, int $b): int;
}

public class MyCalculator implements ICalculator {
    public func add(string $a, int $b): int {
        return 0;
    }
}
`
	items := analyzeSource(t, source, NewEnvironment())
	if !hasCode(items, "JOSS-DECL-005") {
		t.Fatalf("expected JOSS-DECL-005 for parameter type mismatch, got %#v", items)
	}
}

func TestAnalyzer_InterfaceMethodReturnTypeMismatch(t *testing.T) {
	source := `
public interface ICalculator {
    public func add(int $a, int $b): int;
}

public class MyCalculator implements ICalculator {
    public func add(int $a, int $b): string {
        return "zero";
    }
}
`
	items := analyzeSource(t, source, NewEnvironment())
	if !hasCode(items, "JOSS-DECL-005") {
		t.Fatalf("expected JOSS-DECL-005 for return type mismatch, got %#v", items)
	}
}

func TestAnalyzer_InterfaceMethodVisibilityNotPublic(t *testing.T) {
	source := `
public interface IGreeter {
    public func greet(string $name): string;
}

public class PrivateGreeter implements IGreeter {
    private func greet(string $name): string {
        return "Hola " . $name;
    }
}
`
	items := analyzeSource(t, source, NewEnvironment())
	if !hasCode(items, "JOSS-DECL-005") {
		t.Fatalf("expected JOSS-DECL-005 for non-public interface method, got %#v", items)
	}
}

func TestAnalyzer_InterfaceNotFound(t *testing.T) {
	source := `
public class User implements INonExistent {
}
`
	items := analyzeSource(t, source, NewEnvironment())
	if !hasCode(items, "JOSS-SYM-007") {
		t.Fatalf("expected JOSS-SYM-007 for unknown interface, got %#v", items)
	}
}

func TestAnalyzer_InterfaceNameCollisionWithClass(t *testing.T) {
	source := `
public class Animal {}
public interface Animal {}
`
	items := analyzeSource(t, source, NewEnvironment())
	if !hasCode(items, "JOSS-DECL-004") {
		t.Fatalf("expected JOSS-DECL-004 for name collision, got %#v", items)
	}
}

func TestAnalyzer_CyclicInterfaceInheritance(t *testing.T) {
	source := `
public interface IA extends IB {}
public interface IB extends IA {}
`
	items := analyzeSource(t, source, NewEnvironment())
	if !hasCode(items, "JOSS-SYM-008") {
		t.Fatalf("expected JOSS-SYM-008 for cyclic inheritance, got %#v", items)
	}
}

func TestAnalyzer_PolymorphicInterfaceUsage(t *testing.T) {
	source := `
public interface IWriter {
    public func write(string $data): string;
}

public class ConsoleWriter implements IWriter {
    public func write(string $data): string {
        return $data;
    }
}

public func logMessage(IWriter $writer, string $msg): string {
    return $writer->write($msg);
}

public func test(): string {
    $w = new ConsoleWriter();
    return logMessage($w, "hello");
}
`
	items := analyzeSource(t, source, NewEnvironment())
	for _, item := range items {
		if item.Severity == "error" {
			t.Fatalf("unexpected error in polymorphic usage: %s", item.String())
		}
	}
}
