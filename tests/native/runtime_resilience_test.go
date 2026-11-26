package native_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNative_ReadableStackTraceOnUncaughtThrow(t *testing.T) {
	jossBin := wbBuildJossCLI(t)
	tmpDir := t.TempDir()

	src := `public func nivel2(int $x): int {
	($x < 0) ? {
		throw "valor negativo no permitido"
	}
	return $x * 2
}

public func nivel1(int $v): int {
	return nivel2($v)
}

print(nivel1(-5))
`
	srcPath := filepath.Join(tmpDir, "trace_demo.joss")
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	binPath := filepath.Join(tmpDir, "trace_demo")

	buildCmd := exec.Command(jossBin, "build", srcPath, "--backend=llvm", "--release", "-o", binPath)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("joss build failed: %v\n%s", err, string(out))
	}

	runCmd := exec.Command(binPath)
	var stdout, stderr bytes.Buffer
	runCmd.Stdout = &stdout
	runCmd.Stderr = &stderr
	err := runCmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit on uncaught throw, got exit 0 (stdout=%q stderr=%q)", stdout.String(), stderr.String())
	}

	errText := stderr.String()
	t.Logf("Native stderr:\n%s", errText)
	for _, want := range []string{
		"Panic: valor negativo no permitido",
		"Stack trace:",
		"at nivel2 (trace_demo.joss:3)",
		"at nivel1 (trace_demo.joss:8)",
		"at main (trace_demo.joss:1)",
	} {
		if !strings.Contains(errText, want) {
			t.Errorf("expected stderr to contain %q, got:\n%s", want, errText)
		}
	}
}

func TestNative_NominalFlatSlotsTreeParity(t *testing.T) {
	src := `public class TreeNode {
	public int $item
	public mixed $left
	public mixed $right

	Init(int $item, mixed $left, mixed $right) {
		$this->item = $item
		$this->left = $left
		$this->right = $right
	}

	public func check(): int {
		($this->left == null) ? {
			return $this->item
		}
		TreeNode $l = $this->left
		TreeNode $r = $this->right
		return $this->item + $l->check() - $r->check()
	}
}

public func makeTree(int $item, int $depth): TreeNode {
	($depth == 0) ? {
		return new TreeNode($item, null, null)
	}
	int $item2 = $item * 2
	int $d = $depth - 1
	return new TreeNode($item, makeTree($item2 - 1, $d), makeTree($item2, $d))
}

TreeNode $t = makeTree(0, 10)
print("CHECK:" . strval($t->check()))
`
	wbRunCLISourceAcrossBackends(t, "nominal_tree", src, []string{"llvm"}, nil)
}

func TestNative_MiniGCAutoReclaimsGarbageAndPreservesLongLivedRoot(t *testing.T) {
	src := `public class Node {
	public int $val
	public mixed $next

	Init(int $val, mixed $next) {
		$this->val = $val
		$this->next = $next
	}
}

Node $longLived = new Node(42, new Node(99, null))
int $sum = 0
int $i = 0
while ($i < 4000) {
	Node $a = new Node($i, null)
	Node $b = new Node($i + 1, $a)
	$a->next = $b
	$sum = $sum + $a->val
	$i = $i + 1
}
Node $child = $longLived->next
print("GC_OK:" . strval($longLived->val) . ":" . strval($child->val) . ":" . strval($sum))
`
	wbRunCLISourceAcrossBackends(t, "mini_gc_cycles", src, []string{"llvm"}, nil)
}
