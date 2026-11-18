package analyzer

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
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

func TestReachabilityGraphDetectsMultiActionMatchRoutes(t *testing.T) {
	entrySrc := `print("Starting server")`
	routeSrc := `Router::match("GET|POST", "/register", "AuthController@showRegister@doRegister")`
	authCtrlSrc := `
public class AuthController {
    public static func showRegister(): void {
        print("show register form")
    }
    public static func doRegister(): void {
        print("do register submit")
    }
    public static func unused(): void {
        print("dead")
    }
}
`

	units := []SourceUnit{
		parseReachUnit("main.joss", entrySrc),
		parseReachUnit("routes.joss", routeSrc),
		parseReachUnit("app/controllers/AuthController.joss", authCtrlSrc),
	}

	prep := PrepareProgram(units, NewEnvironment())
	graph := BuildReachabilityGraph(prep, ReachabilityOptions{Entrypoint: "main.joss", RouteFiles: []string{"routes.joss"}})

	if !graph.IsClassReachable("AuthController") {
		t.Errorf("AuthController must be reachable via routes.joss")
	}
	if !graph.IsMethodReachable("AuthController", "showRegister") {
		t.Errorf("AuthController::showRegister must be reachable")
	}
	if !graph.IsMethodReachable("AuthController", "doRegister") {
		t.Errorf("AuthController::doRegister must be reachable")
	}
	if graph.IsMethodReachable("AuthController", "unused") {
		t.Errorf("AuthController::unused must NOT be reachable")
	}

	pruned := PruneProgramAST(units[2], graph)
	cls := pruned.Program.Statements[0].(*parser.ClassStatement)
	if len(cls.Body.Statements) != 2 {
		t.Errorf("expected 2 methods retained in AuthController, got %d", len(cls.Body.Statements))
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

func TestReachabilityKeepsJossOwnerForStaticHostService(t *testing.T) {
	main := parseReachUnit("main.joss", `echo Session::get("user_id");`)
	owner := parseReachUnit("std/http/session_host.joss", `private func __joss_host_Session_get(string $key): mixed { return session($key); }`)
	env := NewEnvironment()
	valueType := typesystem.Type{Kind: typesystem.Mixed}
	env.Classes["Session"] = Class{Name: "Session", Methods: map[string]Callable{
		"get": {Name: "get", Owner: "Session", ReturnType: valueType, Parameters: []Parameter{{Name: "key", Type: typesystem.Type{Kind: typesystem.String}}}},
	}}
	env.Builtins["session"] = Callable{Name: "session", ReturnType: valueType, Parameters: []Parameter{{Name: "key", Type: typesystem.Type{Kind: typesystem.String}}}}
	prep := PrepareProgram([]SourceUnit{main, owner}, env)
	if prep.HasErrors() {
		t.Fatalf("prepare source units: %v", prep.Diagnostics)
	}
	graph := BuildReachabilityGraph(prep, ReachabilityOptions{Entrypoint: "main.joss"})
	if !graph.IsFunctionReachable("__joss_host_Session_get") {
		t.Fatal("static Session::get must retain its Joss-owned host-service implementation")
	}
	if !graph.IsFileReachable("std/http/session_host.joss") {
		t.Fatal("the Joss host-service owner source unit must remain reachable")
	}
}

func TestReachabilityKeepsJossOwnerForNativeInstanceService(t *testing.T) {
	main := parseReachUnit("main.joss", `var $request = new Request(); var $safe = $request->except(["_token"]);`)
	owner := parseReachUnit("std/http/request.joss", `public func __joss_host_Request_except(mixed $excluded): map { return {}; }`)
	env := NewEnvironment()
	mapType := typesystem.Type{Kind: typesystem.Map}
	arrayType := typesystem.Type{Kind: typesystem.Array}
	env.Classes["Request"] = Class{Name: "Request", Methods: map[string]Callable{
		"except": {Name: "except", Owner: "Request", ReturnType: mapType, Parameters: []Parameter{{Name: "excluded", Type: arrayType}}},
	}}
	prepared := PrepareProgram([]SourceUnit{main, owner}, env)
	if prepared.HasErrors() {
		t.Fatalf("prepare source units: %v", prepared.Diagnostics)
	}
	graph := BuildReachabilityGraph(prepared, ReachabilityOptions{Entrypoint: "main.joss"})
	if !graph.IsFunctionReachable("__joss_host_Request_except") {
		t.Fatal("native Request instance calls must retain their Joss-owned method policy")
	}
	if !graph.IsFileReachable("std/http/request.joss") {
		t.Fatal("the Request Joss policy source unit must remain reachable")
	}
}

func TestReachabilityKeepsJossOwnerForJSONDecodeBuiltin(t *testing.T) {
	main := parseReachUnit("main.joss", `echo json_decode("{}");`)
	owner := parseReachUnit("std/json/decoder.joss", `public func __joss_builtin_json_decode(string $source): mixed { return null; }`)
	env := NewEnvironment()
	env.Builtins["json_decode"] = Callable{
		Name: "json_decode", ReturnType: typesystem.Type{Kind: typesystem.Mixed},
		Parameters: []Parameter{{Name: "source", Type: typesystem.Type{Kind: typesystem.String}}},
	}
	prepared := PrepareProgram([]SourceUnit{main, owner}, env)
	if prepared.HasErrors() {
		t.Fatalf("prepare source units: %v", prepared.Diagnostics)
	}
	graph := BuildReachabilityGraph(prepared, ReachabilityOptions{Entrypoint: "main.joss"})
	if !graph.IsFunctionReachable("__joss_builtin_json_decode") || !graph.IsFileReachable("std/json/decoder.joss") {
		t.Fatal("reachable json_decode calls must retain the canonical Joss decoder owner")
	}
}

func TestReachabilityKeepsSEORenderOwnerForCompiledViews(t *testing.T) {
	main := parseReachUnit("main.joss", `view("welcome");`)
	owner := parseReachUnit("std/http/seo_host.joss", `public func __joss_host_SEO_render(map $request): string { return ""; }`)
	env := NewEnvironment()
	env.Builtins["view"] = Callable{Name: "view", ReturnType: typesystem.Type{Kind: typesystem.String}, Parameters: []Parameter{{Name: "name", Type: typesystem.Type{Kind: typesystem.String}}}}
	prep := PrepareProgram([]SourceUnit{main, owner}, env)
	if prep.HasErrors() {
		t.Fatalf("prepare source units: %v", prep.Diagnostics)
	}
	graph := BuildReachabilityGraph(prep, ReachabilityOptions{Entrypoint: "main.joss"})
	if !graph.IsFunctionReachable("__joss_host_SEO_render") || !graph.IsFileReachable("std/http/seo_host.joss") {
		t.Fatal("compiled view use must retain the Pure Joss SEO render owner for template lowering")
	}
}

func TestReachabilityKeepsSEORenderOwnerForRoutedTemplates(t *testing.T) {
	main := parseReachUnit("main.joss", `Server::start();`)
	routes := parseReachUnit("routes.joss", `Router::get("/", "HomeController@index");`)
	controller := parseReachUnit("HomeController.joss", `public class HomeController { public func index(): string { return view("home"); } }`)
	owner := parseReachUnit("std/http/seo_host.joss", `public func __joss_host_SEO_render(map $request): string { return ""; }`)
	env := NewEnvironment()
	env.Builtins["view"] = Callable{Name: "view", ReturnType: typesystem.Type{Kind: typesystem.String}, Parameters: []Parameter{{Name: "name", Type: typesystem.Type{Kind: typesystem.String}}}}
	prep := PrepareProgram([]SourceUnit{main, routes, controller, owner}, env)
	graph := BuildReachabilityGraph(prep, ReachabilityOptions{Entrypoint: "main.joss"})
	if !graph.IsFunctionReachable("__joss_host_SEO_render") || !graph.IsFileReachable("std/http/seo_host.joss") {
		t.Fatal("routed view templates must retain the Pure Joss SEO owner before template lowering")
	}
}
