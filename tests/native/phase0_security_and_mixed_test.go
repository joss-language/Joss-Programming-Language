package native_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/crypto"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
)

func TestPhase0_MixedLargeIntegerAndTypesDifferentialParity(t *testing.T) {
	src := `
		mixed $x = 100000;
		echo $x;
		$x = "hello";
		echo $x;
		$x = true;
		echo $x;
		var $m = {};
		$m["count"] = 250000;
		$m["name"] = "Ada";
		echo $m["count"];
		echo $m["name"];
	`
	interpOut, ok, errInfo := runInterpreterWithUnits(t, src, nil)
	if !ok {
		t.Fatalf("interpreter failed: %s", errInfo)
	}

	nativeOut, exitCode, err := compileAndRunNative(t, src, "phase0_mixed")
	if err != nil {
		t.Fatalf("native compile/run error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("native exit code = %d, output = %q", exitCode, nativeOut)
	}

	if interpOut != nativeOut {
		t.Fatalf("differential mismatch:\ninterpreter:\n%s\nnative:\n%s", interpOut, nativeOut)
	}
}

func TestPhase0_NoBuildTimeEnvSecretEmbedding(t *testing.T) {
	tmpDir := t.TempDir()
	secret := "SUPER_SECRET_API_KEY_987654321"
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("API_KEY="+secret+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	src := `echo "ok";`
	p := parser.NewParser(parser.NewLexer(src))
	prog := p.ParseProgram()
	prep := analyzer.PrepareProgram([]analyzer.SourceUnit{{Path: "env_test.joss", Program: prog}}, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	irProg, err := ir.LowerAndVerify(prep, "env_test", nil)
	if err != nil {
		t.Fatal(err)
	}
	outBin := filepath.Join(tmpDir, "env_test")
	if _, err := native.BuildProgram(irProg, native.BuildOptions{
		Target:     native.HostTarget(),
		Backend:    native.BackendNative,
		OutputPath: outBin,
	}); err != nil {
		t.Fatal(err)
	}
	binBytes, err := os.ReadFile(outBin)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(binBytes, []byte(secret)) {
		t.Fatalf("security regression: build-time .env secret was embedded into native executable")
	}
}

func TestPhase0_NoHardcodedEnvMasterSecret(t *testing.T) {
	t.Setenv("JOSS_ENV_KEY", "")
	t.Setenv("JOSS_MASTER_KEY", "")
	if _, err := crypto.ResolveEnvEncryptionKey(); err == nil {
		t.Fatalf("expected ResolveEnvEncryptionKey to fail when JOSS_ENV_KEY and JOSS_MASTER_KEY are unset")
	}
}

func TestPhase0_ARM64RejectsSilentStubs(t *testing.T) {
	program := ir.NewProgram("arm64_stub_check")
	fn := ir.NewFunction("main", ir.TypeI64)
	dst := fn.NewTemp(ir.TypeBool, "exists")
	fn.EntryBlock.AddInstruction(&ir.CallBuiltinInst{
		Dest:      dst,
		BuiltinID: "file_exists",
		Args:      []ir.Value{ir.NewConstString("/etc/passwd")},
	})
	fn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	program.AddFunction(fn)

	emitter := native.SelectCodeEmitter(native.Target{OS: "linux", Arch: "arm64"}, false)
	if _, _, err := emitter.Compile(program); err == nil || !strings.Contains(err.Error(), "JOSS-NATIVE-001") {
		t.Fatalf("expected ARM64 to reject unimplemented file_exists with JOSS-NATIVE-001, got %v", err)
	}
}
