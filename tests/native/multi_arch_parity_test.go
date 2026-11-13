package native_test

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	core "github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

// TestMultiArchAndDistro_ComprehensiveParity tests that a rich Joss program covering
// arithmetic, recursion, strings, objects, arrays, maps, and try-catch compiles natively
// into valid binary formats across all supported OSes and architectures.
func TestMultiArchAndDistro_ComprehensiveParity(t *testing.T) {
	source := `
public class MathBox {
	public static int $seed = 4;
	public static func seeded(int $n): int {
		return $n + MathBox::$seed;
	}
	public static func fib(int $n): int {
		return ($n <= 1) ? $n : (MathBox::fib($n - 1) + MathBox::fib($n - 2));
	}
}

public class Item {
	public string $name = "default";
	public int $price = 100;
}

public func computeTotal(int $a, int $b): int {
	return $a + $b;
}

int $sum = computeTotal(10, 20);
int $f = MathBox::fib(6);
int $seeded = MathBox::seeded(6);
string $msg = "Total: " . $sum . ", Fib: " . $f;
echo $msg;
echo "Static: " . $seeded;
echo "Auth guest: " . Auth::guest();

var $item = new Item();
$item->name = "Widget";
$item->price = 250;
echo $item->name . " costs " . $item->price;

var $nums = [1, 2, 3, 4, 5];
int $total = 0;
foreach ($nums as $val) {
	$total = $total + $val;
}
echo "ArraySum: " . $total;

var $scores = {"alice": 90, "bob": 85};
$scores["charlie"] = 95;
echo "Score: " . $scores["charlie"];

var $dynamicValues = {};
$dynamicValues["email"] = "ada@example.test";
echo $dynamicValues["email"];

try {
	throw "TestException";
} catch ($e) {
	echo "Caught: " . $e;
}
`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: "parity.joss", Program: prog}}
	for _, unit := range std.GetStdlibUnits() {
		if unit.Path == "<std/auth/auth.joss>" || strings.HasPrefix(unit.Path, "<std/date/") {
			units = append(units, unit)
		}
	}
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("parity")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	verifier := ir.NewVerifier()
	if err := verifier.Verify(irProg); err != nil {
		t.Fatalf("IR verification failed: %v", err)
	}

	targets := []backend.Target{
		{OS: "linux", Arch: "amd64"},
		{OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"},
		{OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"},
		{OS: "darwin", Arch: "arm64"},
		{OS: "android", Arch: "arm64"},
	}

	for _, target := range targets {
		t.Run(target.DashString(), func(t *testing.T) {
			emitter := backend.SelectCodeEmitter(target, false)
			binBytes, asmListing, err := emitter.Compile(irProg)
			if err != nil {
				t.Fatalf("Compile(%s) failed: %v", target.DashString(), err)
			}
			if len(binBytes) < 512 {
				t.Fatalf("Binary size for %s too small: %d bytes", target.DashString(), len(binBytes))
			}

			switch target.OS {
			case "linux", "android":
				if string(binBytes[0:4]) != "\x7fELF" {
					t.Errorf("[%s] Invalid ELF magic: %q", target.DashString(), binBytes[0:4])
				}
				if binBytes[4] != 2 { // ELFCLASS64
					t.Errorf("[%s] Expected ELFCLASS64, got %d", target.DashString(), binBytes[4])
				}
				eMachine := binary.LittleEndian.Uint16(binBytes[18:20])
				if target.Arch == "amd64" && eMachine != 0x003E {
					t.Errorf("[%s] Expected EM_X86_64 (0x3E), got 0x%04X", target.DashString(), eMachine)
				} else if target.Arch == "arm64" && eMachine != 0x00B7 {
					t.Errorf("[%s] Expected EM_AARCH64 (0xB7), got 0x%04X", target.DashString(), eMachine)
				}
			case "windows":
				if string(binBytes[0:2]) != "MZ" {
					t.Errorf("[%s] Invalid DOS MZ header", target.DashString())
				}
				peOffset := binary.LittleEndian.Uint32(binBytes[0x3C:0x40])
				if string(binBytes[peOffset:peOffset+4]) != "PE\x00\x00" {
					t.Errorf("[%s] Invalid PE signature", target.DashString())
				}
				machine := binary.LittleEndian.Uint16(binBytes[peOffset+4 : peOffset+6])
				if target.Arch == "amd64" && machine != 0x8664 {
					t.Errorf("[%s] Expected IMAGE_FILE_MACHINE_AMD64 (0x8664), got 0x%04X", target.DashString(), machine)
				} else if target.Arch == "arm64" && machine != 0xAA64 {
					t.Errorf("[%s] Expected IMAGE_FILE_MACHINE_ARM64 (0xAA64), got 0x%04X", target.DashString(), machine)
				}
			case "darwin":
				magic := binary.LittleEndian.Uint32(binBytes[0:4])
				if magic != 0xfeedfacf {
					t.Errorf("[%s] Invalid Mach-O 64 magic: 0x%08X", target.DashString(), magic)
				}
			}

			_ = asmListing
		})
	}
}

// TestMultiArchAndDistro_WebServerParity ensures Server::start() emits the native HTTP server
// across all supported targets without any target throwing errors or emitting dummy stubs.
func TestMultiArchAndDistro_WebServerParity(t *testing.T) {
	source := `
public class Main {
	Init main() {
		echo "Iniciando servidor...";
		Server::start();
	}
}
`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: "main.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("server_app")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	targets := []backend.Target{
		{OS: "linux", Arch: "amd64"},
		{OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"},
		{OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"},
		{OS: "darwin", Arch: "arm64"},
		{OS: "android", Arch: "arm64"},
	}

	for _, target := range targets {
		t.Run(target.DashString(), func(t *testing.T) {
			emitter := backend.SelectCodeEmitter(target, false)
			binBytes, asmListing, err := emitter.Compile(irProg)
			if err != nil {
				t.Fatalf("Compile(%s) failed: %v", target.DashString(), err)
			}
			if len(binBytes) < 512 {
				t.Fatalf("Binary size for %s too small: %d bytes", target.DashString(), len(binBytes))
			}

			if !strings.Contains(asmListing, "fn_Server_start:") {
				t.Fatalf("[%s] asmListing does not define fn_Server_start", target.DashString())
			}

			// Ensure the accept loop exists (not a dummy exit stub!)
			if !strings.Contains(asmListing, "accept") && !strings.Contains(asmListing, "Accept") && !strings.Contains(asmListing, "srv_") {
				t.Fatalf("[%s] fn_Server_start appears to be a stub without a server loop", target.DashString())
			}
		})
	}
}

// TestMultiArch_PureJossHTTPRequestParser ensures the new Joss-owned request
// parser lowers and emits across the supported primary OS/architecture matrix.
func TestMultiArch_PureJossHTTPRequestParser(t *testing.T) {
	source := `
string $raw = "POST /login?next=%2Fdashboard HTTP/1.1\r\nHost: example.test\r\nCookie: joss_session=sid-1\r\nContent-Type: application/x-www-form-urlencoded\r\n\r\nemail=ada%40example.test";
var $request = HttpRequestParser::parse($raw);
echo $request["_path"];
echo $request["email"];
`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	units := append(std.GetStdlibUnits(), analyzer.SourceUnit{Path: "http_parser_parity.joss", Program: prog})
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("http_parser_parity")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	if err := (ir.NewVerifier()).Verify(irProg); err != nil {
		t.Fatalf("IR verification failed: %v", err)
	}

	targets := []backend.Target{
		{OS: "linux", Arch: "amd64"},
		{OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"},
		{OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"},
		{OS: "darwin", Arch: "arm64"},
	}
	for _, target := range targets {
		t.Run(target.DashString(), func(t *testing.T) {
			emitter := backend.SelectCodeEmitter(target, false)
			binBytes, _, err := emitter.Compile(irProg)
			if err != nil {
				t.Fatalf("compile failed for %s: %v", target.DashString(), err)
			}
			if len(binBytes) < 512 {
				t.Fatalf("binary too small for %s: %d bytes", target.DashString(), len(binBytes))
			}
		})
	}
}
