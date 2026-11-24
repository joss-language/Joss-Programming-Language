package native_test

import (
	"bytes"
	"debug/elf"
	"debug/pe"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/pluginpkg"
)

func buildNativeTestJPArchive(t *testing.T, pluginName string, source string) []byte {
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

func buildJossCLIBinaryForTest(t *testing.T) string {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	binDir := t.TempDir()
	binName := "joss"
	if runtime.GOOS == "windows" {
		binName = "joss.exe"
	}
	binPath := filepath.Join(binDir, binName)
	cmd := exec.Command("go", "build", "-tags", "nodb noredis nowebview2", "-o", binPath, "./cmd/joss")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build joss CLI failed: %v\n%s", err, string(out))
	}
	return binPath
}

func assertPureNativeBinaryWithoutGoOrTrailer(t *testing.T, exePath string) {
	t.Helper()
	data, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("read executable %s: %v", exePath, err)
	}
	if len(data) < 4 {
		t.Fatalf("executable %s is unexpectedly small: %d bytes", exePath, len(data))
	}
	switch runtime.GOOS {
	case "windows":
		peFile, err := pe.Open(exePath)
		if err != nil {
			t.Fatalf("%s is not a valid PE executable: %v", exePath, err)
		}
		_ = peFile.Close()
	case "linux":
		elfFile, err := elf.Open(exePath)
		if err != nil {
			t.Fatalf("%s is not a valid ELF executable: %v", exePath, err)
		}
		_ = elfFile.Close()
	}

	forbidden := []string{
		"JOSS_RUNNER_DATA",
		"go.buildid",
		"Go build ID:",
		"runtime.main",
		"runtime.gopanic",
		"JOSSBC2Z",
		"ExecutePrepared",
	}
	for _, marker := range forbidden {
		if bytes.Contains(data, []byte(marker)) {
			t.Fatalf("binary %s contains forbidden Go/interpreter/trailer marker %q", exePath, marker)
		}
	}
}

func TestNative_PluginJPCompilationAndCall(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang/zig not available on PATH")
	}

	jossBin := buildJossCLIBinaryForTest(t)

	projectDir := t.TempDir()
	pluginSrc := `
public func plugin_triple(int $x): int {
	return $x * 3
}

public func add_with_default(int $x, int $offset = 15): int {
	return $x + $offset
}

public class Greeter {
	private string $prefix = "Hola "
	public func hello(string $name): string {
		return $this->prefix . $name
	}
}
`
	jpBytes := buildNativeTestJPArchive(t, "math_greeter", pluginSrc)
	pluginDir := filepath.Join(projectDir, "plugins", "math_greeter", "latest")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "math_greeter.jp"), jpBytes, 0644); err != nil {
		t.Fatalf("write .jp archive: %v", err)
	}

	mainSrc := `
public func main(): int {
	int $a = intval(Plugin::call("math_greeter", "plugin_triple", [14]))
	int $b = intval(Plugin::call("math_greeter", "add_with_default", [7]))
	int $c = intval(Plugin::call("math_greeter", "add_with_default", [7, 3]))
	string $d = strval(Plugin::call("math_greeter", "Greeter::hello", ["Joss"]))
	string $e = strval(Plugin::call("math_greeter", "hello", ["Native"]))
	print($a)
	print($b)
	print($c)
	print($d)
	print($e)
	return 0
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "main.joss"), []byte(mainSrc), 0644); err != nil {
		t.Fatalf("write main.joss: %v", err)
	}

	// Default mode: `joss build main.joss` (no --backend=llvm, no --anterior)
	buildCmd := exec.Command(jossBin, "build", "main.joss")
	buildCmd.Dir = projectDir
	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("joss build main.joss failed: %v\noutput:\n%s", err, string(buildOut))
	}

	builtExe := filepath.Join(projectDir, "build", "main")
	if runtime.GOOS == "windows" {
		builtExe += ".exe"
	}
	assertPureNativeBinaryWithoutGoOrTrailer(t, builtExe)

	// Copy ONLY the compiled executable to an isolated empty directory with no .jp or .joss files
	cleanDir := t.TempDir()
	cleanExe := filepath.Join(cleanDir, filepath.Base(builtExe))
	exeBytes, err := os.ReadFile(builtExe)
	if err != nil {
		t.Fatalf("read built exe: %v", err)
	}
	if err := os.WriteFile(cleanExe, exeBytes, 0755); err != nil {
		t.Fatalf("copy built exe to clean dir: %v", err)
	}

	runCmd := exec.Command(cleanExe)
	runCmd.Dir = cleanDir
	outBytes, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("clean execution of plugin binary failed: %v\noutput:\n%s", err, string(outBytes))
	}
	got := strings.ReplaceAll(strings.TrimSpace(string(outBytes)), "\r\n", "\n")
	want := "42\n22\n10\nHola Joss\nHola Native"
	if got != want {
		t.Fatalf("plugin binary stdout mismatch:\ngot:\n%s\nwant:\n%s\nbuild output:\n%s", got, want, string(buildOut))
	}
}
