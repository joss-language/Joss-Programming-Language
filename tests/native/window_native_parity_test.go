package native_test

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	_ "github.com/jossecurity/joss/pkg/backend/llvm"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	core "github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

func TestNativeParity_WindowGUI(t *testing.T) {
	t.Setenv("JOSS_GUI_HEADLESS", "1")
	src := `
var $win = Window::open("Joss Desktop", 1280, 720);
print("INIT:" . $win->getTitle() . "|" . strval($win->getWidth()) . "x" . strval($win->getHeight()) . "|" . strval($win->isVisible()));

$win->setTitle("Joss Native Studio");
$win->setSize(1440, 900);
$win->setMinSize(800, 600);
$win->setResizable(true);
$win->setFrameless(false);
$win->setAlwaysOnTop(false);
$win->setTheme("dark");
print("UPDATED:" . $win->getTitle() . "|" . strval($win->getWidth()) . "x" . strval($win->getHeight()));

$win->setHtml("<h1>Hola Joss Native</h1>");
$win->eval("console.log('ready')");
$win->navigate("joss://app/index.html");

string $prefix = "Hola ";
$win->bind("saludar", func(string $name): string {
	return $prefix . $name . "!";
});
string $reply = $win->dispatch("saludar", "Mundo");
print("IPC:" . $reply);

$win->on("close", func(): void {
	print("EVT:closed");
});
$win->run();
$win->hide();
print("HIDDEN:" . strval($win->isVisible()));
$win->show();
print("SHOWN:" . strval($win->isVisible()));
$win->close();
print("CLOSED:" . strval($win->isVisible()));
`
	wbRunCLISourceAcrossBackends(t, "window_gui_parity", src, []string{"llvm"}, []string{
		"JOSS_GUI_HEADLESS=1",
	})
}

func TestNative_GUI_PrunedWhenUnused(t *testing.T) {
	source := `print("cli only");`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: "cli_only.joss", Program: prog}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	irProg, err := ir.NewLowerer(prep, nil).LowerProgram("cli_only")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	llvmIR, err := backend.NewLLVMEmitter().Emit(irProg)
	if err != nil {
		t.Fatalf("LLVM emit failed: %v", err)
	}
	if strings.Contains(llvmIR, "@fn_Window_") {
		t.Fatalf("CLI program without Window unexpectedly referenced @fn_Window_ in LLVM IR")
	}
}

func TestNative_GUI_ServerBuildsAndExitsCleanlyInHeadless(t *testing.T) {
	if !backend.NewNativeCompiler().HasNativeToolchain() {
		t.Skip("LLVM/clang toolchain not available")
	}
	source := `
HttpServer::run(0);
print("GUI_SERVER_DONE");
`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: "gui_server.joss", Program: prog}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	irProg, err := ir.NewLowerer(prep, nil).LowerProgram("gui_server")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	outExe := filepath.Join(t.TempDir(), "gui_server_app")
	if runtime.GOOS == "windows" {
		outExe += ".exe"
	}
	_, err = backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     backend.HostTarget(),
		Backend:    backend.BackendLLVM,
		OutputPath: outExe,
		GUI:        true,
	})
	if err != nil {
		t.Fatalf("BuildProgram(--gui) failed: %v", err)
	}

	if runtime.GOOS == "windows" {
		raw, readErr := os.ReadFile(outExe)
		if readErr != nil {
			t.Fatalf("failed reading built GUI exe: %v", readErr)
		}
		if len(raw) < 0x40 {
			t.Fatalf("built GUI exe too small: %d bytes", len(raw))
		}
		peOff := int(binary.LittleEndian.Uint32(raw[0x3C:0x40]))
		subsysOff := peOff + 4 + 20 + 68
		if subsysOff+2 > len(raw) {
			t.Fatalf("invalid PE header offset: %d", peOff)
		}
		subsys := binary.LittleEndian.Uint16(raw[subsysOff : subsysOff+2])
		if subsys != 2 {
			t.Fatalf("expected Windows GUI PE subsystem (2), got %d (3=CUI console)", subsys)
		}
	}

	cmd := exec.Command(outExe)
	cmd.Env = append(os.Environ(), "JOSS_GUI_HEADLESS=1", "APP_NAME=Joss GUI Test")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("GUI server binary failed in headless mode: %v\noutput:\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "GUI_SERVER_DONE") {
		t.Fatalf("expected GUI server binary to return cleanly after GUI window closes; got:\n%s", string(out))
	}
}
