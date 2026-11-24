package native_test

import (
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
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
