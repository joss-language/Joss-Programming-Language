package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/pluginpkg"
)

func TestExecutionModeIsAlwaysNative(t *testing.T) {
	mode, args := takeExecutionMode([]string{"main.joss"})
	if mode != executionModeNative || len(args) != 1 || args[0] != "main.joss" {
		t.Fatalf("default mode = %q, args = %v; want native mode and untouched args", executionModeName(mode), args)
	}

	mode, args = takeExecutionMode([]string{"main.joss", "--native"})
	if mode != executionModeNative || len(args) != 1 || args[0] != "main.joss" {
		t.Fatalf("native mode = %q, args = %v; want native mode and selector removed", executionModeName(mode), args)
	}

	mode, args = takeExecutionMode([]string{"main.joss", "--anterior"})
	if mode != executionModeNative || len(args) != 1 || args[0] != "main.joss" {
		t.Fatalf("deprecated anterior flag = %q, args = %v; want native mode with flag stripped", executionModeName(mode), args)
	}
}

func TestServerChildPreservesNativeModeThroughReload(t *testing.T) {
	got := serverChildArgs("main.joss", []string{"--port=9000"}, executionModeNative)
	want := []string{"run", "main.joss", "--native", "--port=9000"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("native child args = %v, want %v", got, want)
	}
}

func TestServerChildAlwaysSelectsNativeHTTP(t *testing.T) {
	if got := serverBackendForMode(executionModeNative); got != "native" {
		t.Fatalf("HTTP backend = %q, want native", got)
	}
	if shouldUseGoHTTPRuntime(executionModeNative, "go") {
		t.Fatal("native mode must never select Go HTTP")
	}
	t.Setenv("JOSS_HTTP_BACKEND", "go")
	var selected string
	for _, entry := range serverChildEnv(false, executionModeNative) {
		if strings.HasPrefix(strings.ToUpper(entry), "JOSS_HTTP_BACKEND=") {
			selected = strings.TrimPrefix(entry, "JOSS_HTTP_BACKEND=")
		}
	}
	if selected != "native" {
		t.Errorf("child HTTP backend = %q, want native", selected)
	}
}

func TestProjectHTTPServerDetectionUsesPreparedCallFacts(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	source := `public func main(): int {
    Server::start()
    return 0
}`
	if err := os.WriteFile("main.joss", []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if !projectUsesHTTPServer("main.joss") {
		t.Fatal("project with resolved Server::start should be detected as an HTTP server")
	}
	if err := os.WriteFile("main.joss", []byte(`public func main(): int {
    return 0
}`), 0600); err != nil {
		t.Fatal(err)
	}
	if projectUsesHTTPServer("main.joss") {
		t.Fatal("the Standard Library's own server definitions must not mark a CLI project as an HTTP server")
	}
}

func TestHTTPBuildIsStrictlyNative(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	if err := os.WriteFile("main.joss", []byte(`public func main(): int {
    Server::start()
    return 0
}`), 0600); err != nil {
		t.Fatal(err)
	}
	hostTarget := native.HostTarget()
	targetStr := hostTarget.DashString()
	ext := ""
	if hostTarget.OS == "windows" {
		ext = ".exe"
	}

	if ensureNativeToolchainReady() {
		nativeOutput, err := buildProgramForMode("main.joss", filepath.Join("build", "native_http"+ext), targetStr, false, false, false, false, "auto", executionModeNative)
		if err != nil {
			t.Fatalf("native HTTP build failed: %v", err)
		}
		contents, err := os.ReadFile(nativeOutput)
		if err != nil {
			t.Fatal(err)
		}
		validMagic := false
		switch hostTarget.OS {
		case "windows":
			validMagic = len(contents) >= 2 && string(contents[:2]) == "MZ"
		case "linux":
			validMagic = len(contents) >= 4 && string(contents[:4]) == "\x7fELF"
		case "darwin":
			validMagic = len(contents) >= 4 && (bytes.Equal(contents[:4], []byte{0xcf, 0xfa, 0xed, 0xfe}) || bytes.Equal(contents[:4], []byte{0xca, 0xfe, 0xba, 0xbe}))
		default:
			validMagic = len(contents) > 0
		}
		if !validMagic {
			t.Fatalf("native HTTP build did not emit a native %s executable", hostTarget.OS)
		}
		if bytes.Contains(contents, []byte("JOSS_RUNNER_DATA")) {
			t.Fatal("native HTTP build must never embed legacy runner data")
		}
	}

	if err := os.WriteFile("broken.joss", []byte(`public func main(): int {
    Server::start()
    return "wrong-type"
}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := buildProgramForMode("broken.joss", filepath.Join("build", "broken"+ext), targetStr, false, false, false, false, "auto", executionModeNative); err == nil {
		t.Fatal("build must fail on static semantic errors")
	}
}

func TestNativeCapabilityGapDetection(t *testing.T) {
	if !isNativeCapabilityGap(errors.New("JOSS-NATIVE-001: unsupported call")) {
		t.Fatal("native capability diagnostic should be detected")
	}
	if !isNativeCapabilityGap(errors.New("capacidades AOT no disponibles: Socket::listen")) {
		t.Fatal("AOT capability validation error should be detected")
	}
	for _, err := range []error{
		nil,
		errors.New("syntax error"),
		errors.New("IR verifier internal failure"),
	} {
		if isNativeCapabilityGap(err) {
			t.Errorf("%v must not be classified as a capability gap", err)
		}
	}
}

func TestBuildRejectsMissingAOTCapabilityBeforePreparingProjectAssets(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)

	if err := os.WriteFile("main.joss", []byte(`public func main(): int {
    Cache::get("probe")
    return 0
}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("assets", "css"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("assets", "css", "probe.scss"), []byte("body { color: red; }"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := buildNativeProgram("main.joss", "", "windows-amd64", false, false, false, false, "native")
	if err == nil || !strings.Contains(err.Error(), "Cache::get") {
		t.Fatalf("build should reject the missing AOT service capability, got %v", err)
	}
	if _, err := os.Stat(filepath.Join("public", "css", "probe.css")); !os.IsNotExist(err) {
		t.Fatalf("AOT preflight should reject before compiling project styles; stat error = %v", err)
	}
}

func TestBuildUsesCompactAOTWhenNoHostCapabilityIsNeeded(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	if err := os.WriteFile("main.joss", []byte(`public func main(): int {
    return 0
}`), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := buildProgramForMode("main.joss", "", "windows-amd64", true, false, false, false, "native", executionModeNative)
	if err != nil {
		t.Fatalf("native build should select AOT for a pure program: %v", err)
	}
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) < 2 || string(contents[:2]) != "MZ" {
		t.Fatalf("pure native build is not a PE executable: prefix=%q", contents[:min(2, len(contents))])
	}
	if bytes.Contains(contents, []byte("JOSS_RUNNER_DATA")) {
		t.Fatal("pure native build unexpectedly embeds a Go compatibility runner")
	}
	if len(contents) > 80*1024 {
		t.Fatalf("pure native build size = %d bytes, exceeds the 80 KiB compact profile goal", len(contents))
	}
}

func buildTestJPArchive(t *testing.T, pluginName, source string) []byte {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("plugin parse errors: %v", p.Errors())
	}
	bcBytes, err := bytecode.Encode(prog)
	if err != nil {
		t.Fatalf("bytecode.Encode failed: %v", err)
	}
	jpBytes, err := pluginpkg.Build(pluginpkg.Metadata{
		Name:     pluginName,
		Version:  "1.0.0",
		Bytecode: "bytecode/main.jbc",
	}, map[string][]byte{
		"bytecode/main.jbc": bcBytes,
	})
	if err != nil {
		t.Fatalf("pluginpkg.Build failed: %v", err)
	}
	return jpBytes
}

func TestAppendProjectPluginUnitsFindsSubdirPluginsAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	jpBytes := buildTestJPArchive(t, "math_ext", `public func plugin_double(int $x): int {
    return $x * 2
}`)

	subDir := filepath.Join(root, "subapp")
	subPluginDir := filepath.Join(subDir, "plugins", "math_ext", "latest")
	if err := os.MkdirAll(subPluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subPluginDir, "math_ext.jp"), jpBytes, 0644); err != nil {
		t.Fatal(err)
	}
	// Also place the same plugin at the workspace root to verify Path deduplication.
	rootPluginDir := filepath.Join(root, "plugins", "math_ext", "latest")
	if err := os.MkdirAll(rootPluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootPluginDir, "math_ext.jp"), jpBytes, 0644); err != nil {
		t.Fatal(err)
	}

	entryPath := filepath.Join("subapp", "main.joss")
	if err := os.WriteFile(entryPath, []byte(`public func main(): int { return 0 }`), 0644); err != nil {
		t.Fatal(err)
	}

	units := appendProjectPluginUnits([]semanticanalyzer.SourceUnit{{Path: entryPath}}, entryPath)
	pluginCount := 0
	for _, u := range units {
		if strings.HasPrefix(u.Path, "<plugin:math_ext/") {
			pluginCount++
		}
	}
	if pluginCount != 1 {
		t.Fatalf("expected exactly 1 deduplicated math_ext plugin unit, got %d (%+v)", pluginCount, units)
	}
}

func TestPluginDispatchIRAndNativeAOTExecution(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)

	pluginSrc := `public func plugin_double(int $x): int {
    return $x * 2
}

public func add_offset(int $x, int $offset = 10): int {
    return $x + $offset
}

public class Greeter {
    private string $prefix = "Hello "
    public func hello(string $name): string {
        return $this->prefix . $name
    }
}`
	jpBytes := buildTestJPArchive(t, "math_ext", pluginSrc)
	pluginDir := filepath.Join("sub", "plugins", "math_ext", "latest")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "math_ext.jp"), jpBytes, 0644); err != nil {
		t.Fatal(err)
	}

	entryPath := filepath.Join("sub", "main.joss")
	mainSrc := `public func main(): int {
    int $a = intval(Plugin::call("math_ext", "plugin_double", [21]))
    int $b = intval(Plugin::call("math_ext", "add_offset", [5]))
    string $c = strval(Plugin::call("math_ext", "Greeter::hello", ["Ana"]))
    string $d = strval(Plugin::call("math_ext", "hello", ["Bob"]))
    print($a)
    print($b)
    print($c)
    print($d)
    return 0
}`
	if err := os.WriteFile(entryPath, []byte(mainSrc), 0644); err != nil {
		t.Fatal(err)
	}

	units, parseDiags := semanticanalyzer.LoadProject(entryPath, "app")
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %v", parseDiags)
	}
	units = appendProjectPluginUnits(units, entryPath)
	report := core.AnalyzeSourceUnits(units)
	if report.HasErrors() {
		report.PrintReport()
		t.Fatal("semantic analysis failed")
	}
	irProg, err := lowerPreparedProject(report.Prepared, entryPath)
	if err != nil {
		t.Fatalf("lowerPreparedProject failed: %v", err)
	}
	if irProg.Functions["__joss_plugin_dispatch"] == nil {
		t.Fatal("expected __joss_plugin_dispatch to be synthesized and retained")
	}
	for _, fnName := range []string{"plugin_double", "add_offset", "Greeter_hello"} {
		if irProg.Functions[fnName] == nil {
			t.Fatalf("expected plugin function %q to be lowered and retained", fnName)
		}
	}
	mainFn := irProg.Functions["main"]
	if mainFn == nil || mainFn.EntryBlock == nil {
		t.Fatal("expected main function in IR")
	}
	foundReg := false
	for _, inst := range mainFn.EntryBlock.Instructions {
		if cr, ok := inst.(*ir.CallRuntimeInst); ok && cr.Func == "joss_register_plugin_dispatcher" {
			if len(cr.Args) == 1 {
				if fp, okFp := cr.Args[0].(*ir.ConstFnPtr); okFp && fp.FnName == "__joss_plugin_dispatch" {
					foundReg = true
				}
			}
		}
	}
	if !foundReg {
		t.Fatalf("expected main entry block to register __joss_plugin_dispatch via joss_register_plugin_dispatcher")
	}

	if ensureNativeToolchainReady() {
		outBin, err := buildNativeProgram(entryPath, filepath.Join("build", "plugin_test"), native.HostTarget().DashString(), false, false, false, false, "auto")
		if err != nil {
			t.Fatalf("native build with plugins failed: %v", err)
		}
		if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(outBin), ".exe") {
			outBin += ".exe"
		}
		outBytes, err := exec.Command(outBin).CombinedOutput()
		if err != nil {
			t.Fatalf("native plugin binary execution failed: %v\noutput:\n%s", err, string(outBytes))
		}
		got := strings.ReplaceAll(strings.TrimSpace(string(outBytes)), "\r\n", "\n")
		want := "42\n15\nHello Ana\nHello Bob"
		if got != want {
			t.Fatalf("native plugin binary stdout = %q, want %q", got, want)
		}
	}
}
