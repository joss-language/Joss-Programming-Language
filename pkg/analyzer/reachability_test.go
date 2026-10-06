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
