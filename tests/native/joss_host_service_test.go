package native_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

func TestAuthEnabledSocialProvidersUsesJossInInterpreterAndNative(t *testing.T) {
	for _, key := range []string{
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "APPLE_CLIENT_ID", "APPLE_CLIENT_SECRET",
		"FACEBOOK_CLIENT_ID", "FACEBOOK_CLIENT_SECRET", "X_CLIENT_ID", "X_CLIENT_SECRET",
		"TWITTER_CLIENT_ID", "TWITTER_CLIENT_SECRET", "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET",
		"TWITCH_CLIENT_ID", "TWITCH_CLIENT_SECRET", "YAHOO_CLIENT_ID", "YAHOO_CLIENT_SECRET",
		"MICROSOFT_CLIENT_ID", "MICROSOFT_CLIENT_SECRET",
	} {
		t.Setenv(key, "")
	}
	for key, value := range map[string]string{
		"GOOGLE_CLIENT_ID":      "google-id",
		"GOOGLE_CLIENT_SECRET":  "google-secret",
		"X_CLIENT_ID":           "",
		"X_CLIENT_SECRET":       "",
		"TWITTER_CLIENT_ID":     "twitter-id",
		"TWITTER_CLIENT_SECRET": "twitter-secret",
		"GITHUB_CLIENT_ID":      "partial-id",
		"GITHUB_CLIENT_SECRET":  "",
	} {
		t.Setenv(key, value)
	}

	source := `echo implode(",", Auth::enabledSocialProviders());`
	parsed := parser.NewParser(parser.NewLexer(source))
	programAST := parsed.ParseProgram()
	if issues := parsed.Diagnostics(); len(issues) > 0 {
		t.Fatalf("parse Auth provider test: %v", issues)
	}
	units := append([]analyzer.SourceUnit{{Path: "main.joss", Program: programAST}}, std.GetStdlibUnits()...)
	report := core.AnalyzeSourceUnits(units)
	if report.Prepared == nil || report.Prepared.HasErrors() {
		t.Fatalf("analyze Auth provider test: %#v", report.Diagnostics)
	}
	irProgram, err := ir.LowerAndVerify(report.Prepared, "auth_social_provider_owner", nil)
	if err != nil {
		t.Fatalf("lower Auth provider test: %v", err)
	}
	owner := irProgram.Functions["__joss_host_Auth_enabledSocialProviders"]
	if owner == nil {
		t.Fatal("lowered Joss Auth provider owner was pruned")
	}
	if err := backend.ValidateAOTCapabilities(irProgram, backend.HostTarget()); err != nil {
		t.Logf("lowered IR:\n%s", irProgram.Dump())
		t.Fatalf("Joss Auth provider owner should be AOT-capable: %v", err)
	}
	for _, target := range []backend.Target{
		{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"}, {OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"}, {OS: "darwin", Arch: "arm64"},
		{OS: "android", Arch: "arm64"},
	} {
		if _, _, err := backend.SelectCodeEmitter(target, false).Compile(irProgram); err != nil {
			t.Errorf("Auth provider Joss compilation for %s failed: %v", target.DashString(), err)
		}
	}

	interpreted, success, interpreterErr := runInterpreterWithUnits(t, source, std.GetStdlibUnits())
	if !success {
		t.Fatalf("interpreter Auth provider execution failed: %s", interpreterErr)
	}
	if got, want := interpreted, "google,x"; got != want {
		t.Fatalf("interpreter enabled providers = %q, want %q; stderr=%q", got, want, interpreterErr)
	}
	jitExecutable, err := backend.SelectCodeEmitter(backend.HostTarget(), true).CompileInMemory(irProgram)
	if err != nil {
		t.Fatalf("JIT compile Auth provider owner: %v", err)
	}
	defer jitExecutable.Free()
	if exitCode, err := jitExecutable.Execute(); err != nil || exitCode != 0 {
		t.Fatalf("JIT Auth provider owner exited %d, err=%v; want 0", exitCode, err)
	}

	outputPath := filepath.Join(t.TempDir(), "auth_social_provider_owner.exe")
	if _, err := backend.BuildProgram(irProgram, backend.BuildOptions{Target: backend.HostTarget(), OutputPath: outputPath}); err != nil {
		t.Fatalf("build Auth provider owner: %v", err)
	}
	output, err := exec.Command(outputPath).CombinedOutput()
	if err != nil {
		t.Fatalf("execute Auth provider owner: %v, output=%q", err, output)
	}
	nativeLines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	nativeResult := ""
	for _, line := range nativeLines {
		if strings.TrimSpace(line) != "" {
			nativeResult = strings.TrimSpace(line)
		}
	}
	if nativeResult != "google,x" {
		t.Fatalf("AOT enabled providers = %q, want final result %q; output=%q", nativeResult, "google,x", output)
	}
}

func TestNativeHostServiceBindsPureJossOwner(t *testing.T) {
	sources := []struct{ path, source string }{
		{path: "main.joss", source: `System::log("joss-owner")`},
	}
	units := make([]analyzer.SourceUnit, 0, len(sources))
	for _, source := range sources {
		p := parser.NewParser(parser.NewLexer(source.source))
		program := p.ParseProgram()
		if issues := p.Diagnostics(); len(issues) > 0 {
			t.Fatalf("parse %s: %v", source.path, issues)
		}
		units = append(units, analyzer.SourceUnit{Path: source.path, Program: program})
	}
	units = append(units, std.GetStdlibUnits()...)
	prepared := core.AnalyzeSourceUnits(units).Prepared
	if prepared == nil {
		t.Fatal("semantic analysis returned nil PreparedProgram")
	}
	if prepared.HasErrors() {
		t.Fatalf("semantic analysis failed: %v", prepared.Diagnostics)
	}
	program, err := ir.NewLowerer(prepared, nil).LowerProgram("joss_host_owner")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	if program.Functions["__joss_host_System_log"] == nil {
		t.Fatal("reachable Pure Joss host-service owner was pruned")
	}
	for _, function := range program.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instructions {
				if call, ok := instruction.(*ir.CallHostServiceInst); ok && call.ClassName == "System" && call.MethodName == "log" {
					owner := program.Functions["__joss_host_System_log"]
					t.Fatalf("System::log remained a platform host call: call=%s recv=%v args=%v argType=%s dest=%v ownerParamType=%s ownerReturn=%s", call, call.Recv, call.Args, call.Args[0].Type(), call.Dest, owner.Params[0].Type(), owner.ReturnType)
				}
			}
		}
	}
	if err := ir.NewVerifier().Verify(program); err != nil {
		t.Fatalf("IR verification failed: %v", err)
	}
	for _, target := range []backend.Target{
		{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"}, {OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"}, {OS: "darwin", Arch: "arm64"},
		{OS: "android", Arch: "arm64"},
	} {
		if _, _, err := backend.SelectCodeEmitter(target, false).Compile(program); err != nil {
			t.Errorf("AOT compile for %s failed: %v", target.DashString(), err)
		}
	}
	jitExecutable, err := backend.SelectCodeEmitter(backend.HostTarget(), false).CompileInMemory(program)
	if err != nil {
		t.Fatalf("JIT compile failed: %v", err)
	}
	defer jitExecutable.Free()
	if exitCode, err := jitExecutable.Execute(); err != nil || exitCode != 0 {
		t.Fatalf("JIT execution returned exit=%d, err=%v", exitCode, err)
	}
	outPath := filepath.Join(t.TempDir(), "joss_host_owner")
	if backend.HostTarget().OS == "windows" {
		outPath += ".exe"
	}
	if _, err := backend.BuildProgram(program, backend.BuildOptions{Target: backend.HostTarget(), OutputPath: outPath}); err != nil {
		t.Fatalf("native build failed: %v", err)
	}
	output, err := exec.Command(outPath).CombinedOutput()
	if err != nil {
		t.Fatalf("native executable failed: %v, output=%q", err, output)
	}
	if got, want := strings.ReplaceAll(string(output), "\r\n", "\n"), "[System Log] joss-owner\n"; got != want {
		t.Fatalf("native output = %q, want %q", got, want)
	}
}

func TestNativeMathCeilUsesPureJossOwner(t *testing.T) {
	source := `public func main(): int { echo intval(-1.25); echo intval(floatval(-1)); echo intval(Math::ceil(1.25)); echo intval(Math::ceil(-1.25)); return 0; }`
	p := parser.NewParser(parser.NewLexer(source))
	mainProgram := p.ParseProgram()
	if issues := p.Diagnostics(); len(issues) > 0 {
		t.Fatalf("parse Math::ceil program: %v", issues)
	}
	units := append([]analyzer.SourceUnit{{Path: "main.joss", Program: mainProgram}}, std.GetStdlibUnits()...)
	prepared := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prepared.HasErrors() {
		t.Fatalf("analyze Math::ceil program: %v", prepared.Diagnostics)
	}
	program, err := ir.LowerAndVerify(prepared, "math_ceil_owner", nil)
	if err != nil {
		t.Fatalf("lower Math::ceil program: %v", err)
	}
	if program.Functions["__joss_host_Math_ceil"] == nil {
		t.Fatal("Math::ceil Pure Joss owner was pruned")
	}
	for _, function := range program.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instructions {
				if call, ok := instruction.(*ir.CallHostServiceInst); ok && call.ClassName == "Math" && call.MethodName == "ceil" {
					t.Fatal("Math::ceil remained a host service despite its Pure Joss owner")
				}
			}
		}
	}
	for _, target := range []backend.Target{
		{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"}, {OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"}, {OS: "darwin", Arch: "arm64"},
		{OS: "android", Arch: "arm64"},
	} {
		if _, _, err := backend.SelectCodeEmitter(target, false).Compile(program); err != nil {
			t.Errorf("Math::ceil AOT compile for %s failed: %v", target.DashString(), err)
		}
	}

	jitExecutable, err := backend.SelectCodeEmitter(backend.HostTarget(), true).CompileInMemory(program)
	if err != nil {
		t.Fatalf("JIT compile Math::ceil program: %v", err)
	}
	defer jitExecutable.Free()
	if exitCode, err := jitExecutable.Execute(); err != nil || exitCode != 0 {
		t.Fatalf("JIT Math::ceil exit=%d, err=%v", exitCode, err)
	}

	outputPath := filepath.Join(t.TempDir(), "math_ceil_owner")
	if backend.HostTarget().OS == "windows" {
		outputPath += ".exe"
	}
	if _, err := backend.BuildProgram(program, backend.BuildOptions{Target: backend.HostTarget(), OutputPath: outputPath}); err != nil {
		t.Fatalf("AOT build Math::ceil program: %v", err)
	}
	output, err := exec.Command(outputPath).CombinedOutput()
	if err != nil {
		t.Fatalf("AOT Math::ceil executable failed: %v, output=%q", err, output)
	}
	if got, want := strings.ReplaceAll(string(output), "\r\n", "\n"), "-1\n-1\n2\n-1\n"; got != want {
		t.Fatalf("AOT Math::ceil output=%q, want %q", got, want)
	}
}

func TestNativeRequestInputUsesEmittedAMD64Intrinsic(t *testing.T) {
	source := `public func main(): int { echo Request::input("missing", "fallback"); return 0; }`
	p := parser.NewParser(parser.NewLexer(source))
	mainProgram := p.ParseProgram()
	if issues := p.Diagnostics(); len(issues) > 0 {
		t.Fatalf("parse Request::input program: %v", issues)
	}
	units := append([]analyzer.SourceUnit{{Path: "main.joss", Program: mainProgram}}, std.GetStdlibUnits()...)
	prepared := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prepared.HasErrors() {
		t.Fatalf("analyze Request::input program: %v", prepared.Diagnostics)
	}
	program, err := ir.LowerAndVerify(prepared, "request_input_intrinsic", nil)
	if err != nil {
		t.Fatalf("lower Request::input program: %v", err)
	}
	for _, target := range []backend.Target{
		{OS: "windows", Arch: "amd64"},
		{OS: "linux", Arch: "amd64"},
		{OS: "darwin", Arch: "amd64"},
	} {
		if _, _, err := backend.SelectCodeEmitter(target, false).Compile(program); err != nil {
			t.Errorf("AOT compile Request::input for %s: %v", target.DashString(), err)
		}
	}

	outputPath := filepath.Join(t.TempDir(), "request_input_intrinsic")
	if backend.HostTarget().OS == "windows" {
		outputPath += ".exe"
	}
	if _, err := backend.BuildProgram(program, backend.BuildOptions{Target: backend.HostTarget(), OutputPath: outputPath}); err != nil {
		t.Fatalf("build Request::input for host: %v", err)
	}
	output, err := exec.Command(outputPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run Request::input executable: %v, output=%q", err, output)
	}
	if got, want := strings.ReplaceAll(string(output), "\r\n", "\n"), "fallback\n"; got != want {
		t.Fatalf("Request::input output=%q, want %q", got, want)
	}
}

func TestNativeSystemNowUsesPureJossOwner(t *testing.T) {
	source := `public func main(): int { echo System::now(); echo System::now(2); return 0; }`
	p := parser.NewParser(parser.NewLexer(source))
	mainProgram := p.ParseProgram()
	if issues := p.Diagnostics(); len(issues) > 0 {
		t.Fatalf("parse System::now program: %v", issues)
	}
	units := append([]analyzer.SourceUnit{{Path: "main.joss", Program: mainProgram}}, std.GetStdlibUnits()...)
	prepared := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prepared.HasErrors() {
		t.Fatalf("analyze System::now program: %v", prepared.Diagnostics)
	}
	program, err := ir.LowerAndVerify(prepared, "system_now_owner", nil)
	if err != nil {
		t.Fatalf("lower System::now program: %v", err)
	}
	if program.Functions["__joss_host_System_now"] == nil {
		t.Fatal("System::now Pure Joss owner was pruned")
	}
	for _, function := range program.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instructions {
				if call, ok := instruction.(*ir.CallHostServiceInst); ok && call.ClassName == "System" && call.MethodName == "now" {
					t.Fatalf("System::now remained a platform host call: %s", call)
				}
			}
		}
	}
	for _, target := range []backend.Target{
		{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"}, {OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"}, {OS: "darwin", Arch: "arm64"},
		{OS: "android", Arch: "arm64"},
	} {
		if _, _, err := backend.SelectCodeEmitter(target, false).Compile(program); err != nil {
			t.Errorf("System::now AOT compile for %s failed: %v", target.DashString(), err)
		}
	}

	jitExecutable, err := backend.SelectCodeEmitter(backend.HostTarget(), true).CompileInMemory(program)
	if err != nil {
		t.Fatalf("JIT compile System::now program: %v", err)
	}
	defer jitExecutable.Free()
	if exitCode, err := jitExecutable.Execute(); err != nil || exitCode != 0 {
		t.Fatalf("JIT System::now exit=%d, err=%v", exitCode, err)
	}

	outputPath := filepath.Join(t.TempDir(), "system_now_owner")
	if backend.HostTarget().OS == "windows" {
		outputPath += ".exe"
	}
	if _, err := backend.BuildProgram(program, backend.BuildOptions{Target: backend.HostTarget(), OutputPath: outputPath}); err != nil {
		t.Fatalf("AOT build System::now program: %v", err)
	}
	output, err := exec.Command(outputPath).CombinedOutput()
	if err != nil {
		t.Fatalf("AOT System::now executable failed: %v, output=%q", err, output)
	}
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(output), "\r\n", "\n")), "\n")
	if len(lines) != 2 {
		t.Fatalf("AOT System::now output=%q, want two date lines", output)
	}
	first, err := time.ParseInLocation("2006-01-02 15:04:05", lines[0], time.Local)
	if err != nil {
		t.Fatalf("parse System::now() output %q: %v", lines[0], err)
	}
	second, err := time.ParseInLocation("2006-01-02 15:04:05", lines[1], time.Local)
	if err != nil {
		t.Fatalf("parse System::now(2) output %q: %v", lines[1], err)
	}
	expectedDate := first.AddDate(0, 0, 2)
	if second.Format("2006-01-02") != expectedDate.Format("2006-01-02") || second.Hour() != first.Hour() || second.Minute() != first.Minute() {
		t.Fatalf("System::now(2)=%q does not preserve local time two calendar days after %q", lines[1], lines[0])
	}
}

func TestNativeJSONDecodeUsesPureJossOwner(t *testing.T) {
	source := `public func main(): int {
    echo null;
    var $literal = {"message":"literal"};
    echo $literal["message"];
    echo JSONDecoder::decode("\"plain\"");
    echo JSONDecoder::decode("\"Jos\\u0073!\"");
    echo count(JSONDecoder::decode("[true,125]"));
    echo JSONDecoder::decode("[1,]") == null;
    echo JSONDecoder::decode("\"raw\ncontrol\"") == null;
    echo JSONDecoder::decode("01") == null;
    echo JSONDecoder::decode("-0.5E+2") != null;
    string $jsonSource = "{\"message\":\"Jos\\u0073!\",\"items\":[true,125]}";
    mixed $decoded = json_decode($jsonSource);
    echo count($decoded["items"]);
    echo JSON::parse("{\"ok\":true}")["ok"];
    return 0;
}`
	p := parser.NewParser(parser.NewLexer(source))
	mainProgram := p.ParseProgram()
	if issues := p.Diagnostics(); len(issues) > 0 {
		t.Fatalf("parse JSON owner program: %v", issues)
	}
	units := append([]analyzer.SourceUnit{{Path: "main.joss", Program: mainProgram}}, std.GetStdlibUnits()...)
	prepared := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prepared.HasErrors() {
		t.Fatalf("analyze JSON owner program: %v", prepared.Diagnostics)
	}
	program, err := ir.LowerAndVerify(prepared, "json_decode_owner", nil)
	if err != nil {
		t.Fatalf("lower JSON owner program: %v", err)
	}
	for _, function := range program.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instructions {
				if builtin, ok := instruction.(*ir.CallBuiltinInst); ok && builtin.BuiltinID == "json_decode" {
					owner := program.Functions["__joss_builtin_json_decode"]
					t.Fatalf("json_decode remained a backend builtin instead of a shared Joss call (owner=%v, dest=%v, args=%v)", owner, builtin.Dest, builtin.Args)
				}
				if call, ok := instruction.(*ir.CallHostServiceInst); ok && call.ClassName == "JSON" && (call.MethodName == "parse" || call.MethodName == "decode") {
					t.Fatalf("JSON::%s remained a platform host call", call.MethodName)
				}
			}
		}
	}
	for _, target := range []backend.Target{
		{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"}, {OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"}, {OS: "darwin", Arch: "arm64"},
		{OS: "android", Arch: "arm64"},
	} {
		if _, _, err := backend.SelectCodeEmitter(target, false).Compile(program); err != nil {
			t.Errorf("JSON decoder AOT compile for %s failed: %v", target.DashString(), err)
		}
	}

	jitExecutable, err := backend.SelectCodeEmitter(backend.HostTarget(), true).CompileInMemory(program)
	if err != nil {
		t.Fatalf("JIT compile JSON owner program: %v", err)
	}
	defer jitExecutable.Free()
	if exitCode, err := jitExecutable.Execute(); err != nil || exitCode != 0 {
		t.Fatalf("JIT JSON owner exit=%d, err=%v", exitCode, err)
	}

	outputPath := filepath.Join(t.TempDir(), "json_decode_owner")
	if backend.HostTarget().OS == "windows" {
		outputPath += ".exe"
	}
	if _, err := backend.BuildProgram(program, backend.BuildOptions{Target: backend.HostTarget(), OutputPath: outputPath}); err != nil {
		t.Fatalf("AOT build JSON owner program: %v", err)
	}
	output, err := exec.Command(outputPath).CombinedOutput()
	if err != nil {
		t.Fatalf("AOT JSON owner executable failed: %v, output=%q", err, output)
	}
	if got, want := strings.ReplaceAll(string(output), "\r\n", "\n"), "<nil>\nliteral\nplain\nJoss!\n2\ntrue\ntrue\ntrue\ntrue\n2\n1\n"; got != want {
		t.Fatalf("AOT JSON owner output=%q, want %q", got, want)
	}
}
