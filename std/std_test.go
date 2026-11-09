package std_test

import (
	"testing"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

func TestStdlib_LoadsCanonicalUnits(t *testing.T) {
	units := std.GetStdlibUnits()
	if len(units) == 0 {
		t.Fatalf("Se esperaba al menos una unidad en la biblioteca estándar, obtenido 0")
	}

	foundGranDB := false
	for _, u := range units {
		if u.Path == "<std/database/grandb.joss>" {
			foundGranDB = true
			if u.Program == nil || len(u.Program.Statements) == 0 {
				t.Fatalf("El programa parseado de <std/database/grandb.joss> está vacío")
			}
		}
	}

	if !foundGranDB {
		t.Fatalf("No se encontró la unidad <std/database/grandb.joss> en la biblioteca estándar")
	}
}

func TestMiniClass(t *testing.T) {
	code := `
public class Box {
    public string $name = "default";
    public func setName(string $n): Box {
        $this->name = $n;
        return $this;
    }
}
public func main(): int {
    var $b = new Box();
    $b->setName("test");
    return 0;
}
`
	l := parser.NewLexer(code)
	p := parser.NewParser(l)
	prog := p.ParseProgram()
	report := core.AnalyzeSourceUnits([]semanticanalyzer.SourceUnit{{Path: "test.joss", Program: prog}})
	if report.HasErrors() {
		report.PrintReport()
		t.Fatalf("MiniClass falló con %d errores", report.Count(diagnostics.SeverityError))
	}
}

func TestStdlib_GranDB_SemanticAnalysis(t *testing.T) {
	units := std.GetStdlibUnits()
	testCode := `
public func main(): int {
    var $q = GranDB::table("users")->where("active", 1)->orderBy("id", "desc")->limit(10)
    var $sql = $q->toSql()
    print($sql)
    return 0
}
`
	l := parser.NewLexer(testCode)
	p := parser.NewParser(l)
	mainProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Error de parser en mainProg: %v", p.Errors())
	}

	allUnits := append(units, semanticanalyzer.SourceUnit{
		Path:    "main.joss",
		Program: mainProg,
	})

	report := core.AnalyzeSourceUnits(allUnits)
	if report.HasErrors() {
		report.PrintReport()
		t.Fatalf("AnalyzeSourceUnits falló con errores en stdlib: %d errores", report.Count(diagnostics.SeverityError))
	}
}

func TestStdlib_PureJossWebStack_SemanticAnalysis(t *testing.T) {
	units := std.GetStdlibUnits()
	testCode := `
public func main(): int {
    // 1. MIME types
    string $cssMime = Mime::type("css");
    bool $cached = Mime::isCacheable("js");

    // 2. Session
    var $sess = Session::start("JOSS_SESSION=abc");
    $sess->put("user_id", 42);
    mixed $uid = $sess->get("user_id");
    bool $hasUser = $sess->has("user_id");

    // 3. CSRF
    string $csrfTok = Csrf::token($sess);
    string $field = Csrf::field($csrfTok);
    bool $valid = Csrf::verify($csrfTok, $csrfTok);

    // 4. Rate Limiter
    bool $allowed = RateLimiter::check("127.0.0.1", 60, 60);
    RateLimiter::hit("127.0.0.1", 60);
    int $rem = RateLimiter::remaining("127.0.0.1", 60);

    // 5. Error Pages
    string $err404 = ErrorPages::render404("/not-found");
    string $err500 = ErrorPages::render500("Database offline");

    // 6. Hot-Reload
    string $injected = HotReload::inject("<!DOCTYPE html><html><body><h1>App</h1></body></html>");

    // 7. WebSockets
    string $accept = WebSocketProtocol::computeAcceptKey("dGhlIHNhbXBsZSBub25jZQ==");
    string $handshake = WebSocketProtocol::handshakeResponse("dGhlIHNhbXBsZSBub25jZQ==");
    string $frame = WebSocketProtocol::encodeTextFrame("hello");
    WebSocketHub::subscribe("chat", "conn_1");
    int $count = WebSocketHub::subscriberCount("chat");

    // 8. HttpServer & HotReload liveJs
    string $liveJs = HotReload::liveJs();
    string $mockReq = "GET /__joss_health HTTP/1.1\r\nHost: localhost\r\n\r\n";
    string $mockResp = HttpServer::handle($mockReq);

    return 0;
}
`
	l := parser.NewLexer(testCode)
	p := parser.NewParser(l)
	mainProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Error de parser en mainProg: %v", p.Errors())
	}

	allUnits := append(units, semanticanalyzer.SourceUnit{
		Path:    "main.joss",
		Program: mainProg,
	})

	report := core.AnalyzeSourceUnits(allUnits)
	if report.HasErrors() {
		report.PrintReport()
		t.Fatalf("Pure Joss Web Stack Semantic Analysis falló con %d errores", report.Count(diagnostics.SeverityError))
	}
}
