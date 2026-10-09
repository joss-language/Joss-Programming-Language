package analyzer

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func parseReachUnit(path, src string) SourceUnit {
	p := parser.NewParser(parser.NewLexer(src))
	return SourceUnit{Path: path, Program: p.ParseProgram()}
}

func TestReachabilityGraphPrunesDeadFilesAndMethods(t *testing.T) {
	entrySrc := `
var $r = Calculator::usedOp(5)
print($r)
`
	calcSrc := `
public class Calculator {
    public static func usedOp(int $x): int {
        return $x * 2
    }
    public static func deadOp1(int $x): int {
        return $x + 100
    }
}
`
	deadSrc := `
public class DeadService {
    public static func ignoreMe(): void {
        print("unused")
    }
}
`

	units := []SourceUnit{
		parseReachUnit("main.joss", entrySrc),
		parseReachUnit("app/services/Calculator.joss", calcSrc),
		parseReachUnit("app/services/DeadService.joss", deadSrc),
	}

	prep := PrepareProgram(units, NewEnvironment())
	graph := BuildReachabilityGraph(prep, ReachabilityOptions{Entrypoint: "main.joss"})

	// 1. Files
	if !graph.IsFileReachable("main.joss") {
		t.Errorf("main.joss must be reachable")
	}
	if !graph.IsFileReachable("app/services/Calculator.joss") {
		t.Errorf("Calculator.joss must be reachable")
	}
	if graph.IsFileReachable("app/services/DeadService.joss") {
		t.Errorf("DeadService.joss must NOT be reachable")
	}

	// 2. Classes
	if !graph.IsClassReachable("Calculator") {
		t.Errorf("Calculator class must be reachable")
	}
	if graph.IsClassReachable("DeadService") {
		t.Errorf("DeadService class must NOT be reachable")
	}

	// 3. Methods
	if !graph.IsMethodReachable("Calculator", "usedOp") {
		t.Errorf("Calculator::usedOp must be reachable")
	}
	if graph.IsMethodReachable("Calculator", "deadOp1") {
		t.Errorf("Calculator::deadOp1 must NOT be reachable")
	}

	// 4. AST Pruning
	prunedCalc := PruneProgramAST(units[1], graph)
	classStmt := prunedCalc.Program.Statements[0].(*parser.ClassStatement)
	if len(classStmt.Body.Statements) != 1 {
		t.Fatalf("expected 1 method in pruned Calculator, got %d", len(classStmt.Body.Statements))
	}
	method := classStmt.Body.Statements[0].(*parser.MethodStatement)
	if method.Name.Value != "usedOp" {
		t.Errorf("expected usedOp, got %s", method.Name.Value)
	}

	// 5. Capabilities
	if !graph.HasCapability(CapIO) {
		t.Errorf("IO capability must be active due to print()")
	}
	if graph.HasCapability(CapDatabase) {
		t.Errorf("Database capability must NOT be active")
	}
	if graph.HasCapability(CapServer) {
		t.Errorf("Server capability must NOT be active")
	}
}

func TestReachabilityGraphDetectsDynamicRoutes(t *testing.T) {
	entrySrc := `print("Starting server")`
	routeSrc := `Router::get("/users", "UserController@index")`
	userCtrlSrc := `
public class UserController {
    public static func index(): void {
        print("all users")
    }
    public static func unused(): void {
        print("dead")
    }
}
`

	units := []SourceUnit{
		parseReachUnit("main.joss", entrySrc),
		parseReachUnit("routes.joss", routeSrc),
		parseReachUnit("app/controllers/UserController.joss", userCtrlSrc),
	}

	prep := PrepareProgram(units, NewEnvironment())
	graph := BuildReachabilityGraph(prep, ReachabilityOptions{Entrypoint: "main.joss", RouteFiles: []string{"routes.joss"}})

	if !graph.IsClassReachable("UserController") {
		t.Errorf("UserController must be reachable via routes.joss")
	}
	if !graph.IsMethodReachable("UserController", "index") {
		t.Errorf("UserController::index must be reachable")
	}
	if graph.IsMethodReachable("UserController", "unused") {
		t.Errorf("UserController::unused must NOT be reachable")
	}
	if !graph.HasCapability(CapServer) {
		t.Errorf("Server capability must be active due to Router::get")
	}
}

func TestReachabilityGraphPrunesDeadFunctions(t *testing.T) {
	entrySrc := `
var $res = helper_1()
Plugin::call("joss_bg_remover", "preload", [])
print($res)
`
	// libSrc defines 12 functions, but only helper_1 is used
	libSrc := `
public func helper_1(): string {
    return "one"
}
public func helper_2(): string {
    return "two"
}
public func helper_3(): string {
    return "three"
}
public func helper_4(): string {
    return "four"
}
public func helper_5(): string {
    return "five"
}
public func helper_6(): string {
    return "six"
}
public func helper_7(): string {
    return "seven"
}
public func helper_8(): string {
    return "eight"
}
public func helper_9(): string {
    return "nine"
}
public func helper_10(): string {
    return "ten"
}
public func helper_11(): string {
    return "eleven"
}
public func helper_12(): string {
    return "twelve"
}
`
	units := []SourceUnit{
		parseReachUnit("main.joss", entrySrc),
		parseReachUnit("lib/helpers.joss", libSrc),
	}

	prep := PrepareProgram(units, NewEnvironment())
	graph := BuildReachabilityGraph(prep, ReachabilityOptions{Entrypoint: "main.joss"})

	// Verify helper_1 is live
	if !graph.IsFunctionReachable("helper_1") {
		t.Fatalf("expected helper_1 to be reachable")
	}

	// Verify helper_2 through helper_12 are dead
	for i := 2; i <= 12; i++ {
		fnName := "helper_" + string(rune('0'+i))
		if i >= 10 {
			fnName = "helper_1" + string(rune('0'+i-10))
		}
		if graph.IsFunctionReachable(fnName) {
			t.Errorf("expected %s to be dead, but was marked reachable", fnName)
		}
	}

	// Verify Plugin symbol tracking
	if !graph.IsPluginSymbolReachable("joss_bg_remover", "preload") {
		t.Errorf("expected joss_bg_remover::preload to be recorded as reachable")
	}

	// Prune lib/helpers.joss
	prunedLib := PruneProgramAST(units[1], graph)
	if len(prunedLib.Program.Statements) != 1 {
		t.Fatalf("expected exactly 1 function retained in pruned library, got %d", len(prunedLib.Program.Statements))
	}

	fnStmt, ok := prunedLib.Program.Statements[0].(*parser.MethodStatement)
	if !ok || fnStmt.Name.Value != "helper_1" {
		t.Fatalf("expected retained function to be helper_1")
	}
}
