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

func TestExecutionModeDefaultsToAutoAndNativeIsExplicit(t *testing.T) {
	mode, args := takeExecutionMode([]string{"main.joss"})
	if mode != executionModeAuto || len(args) != 1 || args[0] != "main.joss" {
		t.Fatalf("default mode = %q, args = %v; want auto mode and untouched args", executionModeName(mode), args)
	}

	mode, args = takeExecutionMode([]string{"main.joss", "--native"})
	if mode != executionModeNative || len(args) != 1 || args[0] != "main.joss" {
		t.Fatalf("native mode = %q, args = %v; want native mode and selector removed", executionModeName(mode), args)
	}

	mode, args = takeExecutionMode([]string{"main.joss", "--anterior"})
	if mode != executionModePrevious || len(args) != 1 || args[0] != "main.joss" {
		t.Fatalf("previous mode = %q, args = %v; want anterior mode and selector removed", executionModeName(mode), args)
	}
}

func TestServerChildPreservesNativeModeThroughReload(t *testing.T) {
	got := serverChildArgs("main.joss", []string{"--port=9000"}, executionModeNative)
	want := []string{"run", "main.joss", "--native", "--port=9000"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("native child args = %v, want %v", got, want)
	}
	got = serverChildArgs("main.joss", nil, executionModePrevious)
	if strings.Join(got, " ") != "run main.joss" {
		t.Fatalf("previous child args = %v, want no native selector", got)
	}
	got = serverChildArgs("main.joss", nil, executionModeAuto)
	if strings.Join(got, " ") != "run main.joss" {
		t.Fatalf("auto child args = %v, want auto selection via default", got)
	}
}

func TestServerChildSelectsNativeHTTPByDefault(t *testing.T) {
	if got := serverBackendForMode(executionModeAuto); got != "native" {
		t.Fatalf("auto HTTP backend = %q, want native", got)
	}
	if got := serverBackendForMode(executionModePrevious); got != "go" {
		t.Fatalf("previous HTTP backend = %q, want go", got)
	}
	if got := serverBackendForMode(executionModeNative); got != "native" {
		t.Fatalf("native HTTP backend = %q, want native", got)
	}
	if !shouldUseGoHTTPRuntime(executionModePrevious, "go") {
		t.Fatal("previous server mode should use the Go HTTP runtime")
	}
	if shouldUseGoHTTPRuntime(executionModeAuto, "native") || shouldUseGoHTTPRuntime(executionModeNative, "go") {
		t.Fatal("auto and native modes must not select Go HTTP")
	}
	t.Setenv("JOSS_HTTP_BACKEND", "inherited-invalid")
	for mode, want := range map[executionMode]string{
		executionModeAuto:     "native",
		executionModePrevious: "go",
		executionModeNative:   "native",
	} {
		var selected string
		for _, entry := range serverChildEnv(false, mode) {
			if strings.HasPrefix(strings.ToUpper(entry), "JOSS_HTTP_BACKEND=") {
				selected = strings.TrimPrefix(entry, "JOSS_HTTP_BACKEND=")
			}
		}
		if selected != want {
			t.Errorf("child HTTP backend for %s = %q, want %q", executionModeName(mode), selected, want)
		}
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
		t.Fatal("project with resolved Server::start should select the Go HTTP server in auto mode")
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

func TestAutomaticHTTPBuildAttemptsNativeAndPreviousUsesGoRunner(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	if err := os.WriteFile("main.joss", []byte(`public func main(): int {
    Server::start()
    return 0
}`), 0600); err != nil {
		t.Fatal(err)
	}
	runnerDir := filepath.Join(t.TempDir(), "runners")
	if err := os.MkdirAll(filepath.Join(runnerDir, "windows-amd64"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runnerDir, "windows-amd64", "joss-runner.exe"), []byte("test-runner"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JOSS_RUNNER_DIR", runnerDir)

	prevOutput, err := buildProgramForMode("main.joss", filepath.Join("build", "previous.exe"), "windows-amd64", false, false, false, false, "auto", executionModePrevious)
	if err != nil {
		t.Fatalf("previous (--anterior) HTTP build should select the Go runner: %v", err)
	}
	prevContents, err := os.ReadFile(prevOutput)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(prevContents, []byte(legacyRunnerTrailer)) {
		t.Fatal("previous (--anterior) HTTP build did not package the Go runner")
	}

	autoOutput, err := buildProgramForMode("main.joss", filepath.Join("build", "auto.exe"), "windows-amd64", false, false, false, false, "auto", executionModeAuto)
	if err != nil {
		t.Fatalf("auto HTTP build failed: %v", err)
	}
	autoContents, err := os.ReadFile(autoOutput)
	if err != nil {
		t.Fatal(err)
	}
	if ensureNativeToolchainReady() {
		if len(autoContents) < 2 || string(autoContents[:2]) != "MZ" {
			t.Fatalf("auto HTTP build did not emit a native PE executable: prefix=%q", autoContents[:min(2, len(autoContents))])
		}
		if bytes.Contains(autoContents, []byte(legacyRunnerTrailer)) {
			t.Fatal("auto HTTP build unexpectedly packaged the Go runner instead of compiling natively")
		}
	}

	// Verify auto mode enters the native frontend first rather than bypassing to buildLegacyProgram.
	if err := os.WriteFile("broken.joss", []byte(`public func main(): int {
    Server::start()
    return "wrong-type"
}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := buildProgramForMode("broken.joss", filepath.Join("build", "broken.exe"), "windows-amd64", false, false, false, false, "auto", executionModeAuto); err == nil {
		t.Fatal("auto mode must run native frontend analysis instead of bypassing HTTP projects straight to the Go runner")
	}
}

func TestNativeCapabilityGapIsTheOnlyAutomaticFallback(t *testing.T) {
	if !isNativeCapabilityGap(errors.New("JOSS-NATIVE-001: unsupported call")) {
		t.Fatal("native capability diagnostic should allow automatic compatibility selection")
	}
	if !isNativeCapabilityGap(errors.New("capacidades AOT no disponibles: Socket::listen")) {
		t.Fatal("AOT capability validation error should allow automatic compatibility selection")
	}
	for _, err := range []error{
		nil,
		errors.New("syntax error"),
		errors.New("IR verifier internal failure"),
	} {
		if isNativeCapabilityGap(err) {
			t.Errorf("%v must not trigger compatibility selection", err)
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

func TestAutomaticBuildUsesCompatibilityRunnerOnlyForNativeCapabilityGap(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	if err := os.WriteFile("custom.joss", []byte(`public func main(): int {
    Cache::get("probe")
    return 0
}`), 0600); err != nil {
		t.Fatal(err)
	}

	runnerDir := filepath.Join(t.TempDir(), "runners")
	targetRunnerDir := filepath.Join(runnerDir, "windows-amd64")
	if err := os.MkdirAll(targetRunnerDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetRunnerDir, "joss-runner.exe"), []byte("test-runner"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JOSS_RUNNER_DIR", runnerDir)

	output, err := buildProgramForMode("custom.joss", "", "windows-amd64", false, false, false, false, "native", executionModeAuto)
	if err != nil {
		t.Fatalf("auto build should select the compatibility runner for a known native gap: %v", err)
	}
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(contents, []byte(legacyRunnerTrailer)) {
		t.Fatalf("auto build output lacks the sealed compatibility payload marker")
	}
}

func TestAutomaticBuildUsesCompactAOTWhenNoHostCapabilityIsNeeded(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	if err := os.WriteFile("main.joss", []byte(`public func main(): int {
    return 0
}`), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := buildProgramForMode("main.joss", "", "windows-amd64", true, false, false, false, "native", executionModeAuto)
	if err != nil {
		t.Fatalf("auto build should select AOT for a pure program: %v", err)
	}
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) < 2 || string(contents[:2]) != "MZ" {
		t.Fatalf("pure auto build is not a PE executable: prefix=%q", contents[:min(2, len(contents))])
	}
	if bytes.Contains(contents, []byte(legacyRunnerTrailer)) {
		t.Fatal("pure auto build unexpectedly embeds a Go compatibility runner")
	}
	if len(contents) > 80*1024 {
		t.Fatalf("pure auto build size = %d bytes, exceeds the 80 KiB compact profile goal", len(contents))
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
