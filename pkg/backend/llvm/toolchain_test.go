package llvm

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/ir"
)

func TestEnsureRuntimeC_ExtractsEmbeddedOutsideRepo(t *testing.T) {
	emptyDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(emptyDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	t.Setenv("HOME", emptyDir)
	rtPath, err := EnsureRuntimeC("")
	if err != nil {
		t.Fatalf("EnsureRuntimeC failed outside repo: %v", err)
	}
	rtDir := filepath.Dir(rtPath)
	for _, f := range []string{"joss_rt.h", "joss_rt.c", "joss_web_rt.c", "joss_concurrency_rt.c", "joss_db_rt.c", "joss_crypto_rt.c", "joss_gui_rt.c"} {
		if info, statErr := os.Stat(filepath.Join(rtDir, f)); statErr != nil || info.Size() == 0 {
			t.Fatalf("expected embedded runtime file %s in %s: %v", f, rtDir, statErr)
		}
	}
}

func TestResolveToolchain_PrefersBundledZigOverSystem(t *testing.T) {
	fakeDir := t.TempDir()
	zigName := "zig"
	if runtime.GOOS == "windows" {
		zigName = "zig.exe"
	}
	fakeZig := filepath.Join(fakeDir, zigName)
	if err := os.WriteFile(fakeZig, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JOSS_TOOLCHAIN_DIR", fakeDir)

	tc := ResolveToolchain()
	if !tc.Available() {
		t.Fatalf("expected bundled toolchain to be available")
	}
	if tc.Kind != "bundled-zig" || !tc.UseZigCC || tc.CompilerPath != fakeZig {
		t.Fatalf("unexpected toolchain info: %+v", tc)
	}
}

func TestEndToEnd_CompileFromEmptyDirUsingEmbeddedRuntime(t *testing.T) {
	if !ResolveToolchain().Available() {
		t.Skip("no local clang/zig toolchain available")
	}
	emptyDir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(emptyDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	t.Setenv("HOME", emptyDir)

	prog := ir.NewProgram("embedded_rt_test")
	mainFn := ir.NewFunction("main", ir.TypeI64)
	mainFn.EntryBlock.AddInstruction(&ir.CallRuntimeInst{
		Func: "joss_print_i64",
		Args: []ir.Value{ir.NewConstInt(2026, ir.TypeI64)},
	})
	mainFn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	prog.AddFunction(mainFn)

	outBin := filepath.Join(emptyDir, "app_standalone")
	if runtime.GOOS == "windows" {
		outBin += ".exe"
	}
	if _, err := native.BuildProgram(prog, native.BuildOptions{
		Target:     native.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    native.BackendLLVM,
		OutputPath: outBin,
	}); err != nil {
		t.Fatalf("BuildProgram outside repo failed: %v", err)
	}

	out, err := exec.Command(outBin).CombinedOutput()
	if err != nil {
		t.Fatalf("exec failed: %v\n%s", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "2026" {
		t.Fatalf("expected 2026, got %q", strings.TrimSpace(string(out)))
	}
}
