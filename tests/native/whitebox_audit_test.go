package native_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	wbJossOnce sync.Once
	wbJossBin  string
	wbJossErr  error
)

func wbRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root not found from %s: %v", wd, err)
	}
	return root
}

func wbBuildJossCLI(t *testing.T) string {
	t.Helper()
	wbJossOnce.Do(func() {
		root := wbRepoRoot(t)
		dir, err := os.MkdirTemp("", "joss-wb-cli-*")
		if err != nil {
			wbJossErr = err
			return
		}
		exeName := "joss"
		if runtime.GOOS == "windows" {
			exeName += ".exe"
		}
		out := filepath.Join(dir, exeName)
		cmd := exec.Command("go", "build", "-tags", "nodb noredis nowebview2", "-o", out, "./cmd/joss")
		cmd.Dir = root
		if b, err := cmd.CombinedOutput(); err != nil {
			wbJossErr = fmt.Errorf("go build ./cmd/joss failed: %w\n%s", err, string(b))
			return
		}
		wbJossBin = out
	})
	if wbJossErr != nil {
		t.Fatalf("%v", wbJossErr)
	}
	return wbJossBin
}

func wbBuildWithCLI(t *testing.T, workDir, srcFile, outName, backendFlag string, extraFlags ...string) (string, string, error) {
	t.Helper()
	jossBin := wbBuildJossCLI(t)
	exePath := filepath.Join(workDir, outName)
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(exePath), ".exe") {
		exePath += ".exe"
	}
	args := []string{"build", srcFile, "-o", exePath, "--native"}
	if backendFlag != "" {
		args = append(args, "--backend="+backendFlag)
	}
	args = append(args, extraFlags...)
	cmd := exec.Command(jossBin, args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	return exePath, string(out), err
}

func wbRunBinary(t *testing.T, exePath string, workDir string, extraEnv []string) (string, int, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exePath)
	cmd.WaitDelay = 2 * time.Second
	if workDir != "" {
		cmd.Dir = workDir
	} else {
		cmd.Dir = filepath.Dir(exePath)
	}
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return strings.TrimSpace(strings.ReplaceAll(string(out), "\r\n", "\n")), -1, fmt.Errorf("binary timed out after 10s (infinite loop/deadlock): %w", ctx.Err())
	}
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			return string(out), -1, err
		}
	}
	norm := strings.TrimSpace(strings.ReplaceAll(string(out), "\r\n", "\n"))
	return norm, exitCode, err
}

func wbNormalizeOutput(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}

func wbRunCLISourceAcrossBackends(t *testing.T, name, source string, backends []string, extraEnv []string) string {
	t.Helper()
	interpOut := wbNormalizeOutput(runGoInterpreter(t, source))
	for _, bk := range backends {
		t.Run("backend_"+bk, func(t *testing.T) {
			dir := t.TempDir()
			srcPath := filepath.Join(dir, name+".joss")
			if err := os.WriteFile(srcPath, []byte(source), 0644); err != nil {
				t.Fatalf("write %s: %v", srcPath, err)
			}
			exePath, buildOut, err := wbBuildWithCLI(t, dir, filepath.Base(srcPath), name+"_"+bk, bk)
			if err != nil {
				t.Fatalf("[%s/%s] joss build failed: %v\nbuild output:\n%s", name, bk, err, buildOut)
			}
			fi, statErr := os.Stat(exePath)
			if statErr != nil {
				t.Fatalf("[%s/%s] missing binary %s: %v", name, bk, exePath, statErr)
			}
			natOut, code, runErr := wbRunBinary(t, exePath, dir, extraEnv)
			if runErr != nil || code != 0 {
				t.Fatalf("[%s/%s] binary (%d bytes) exited code=%d err=%v\noutput:\n%s", name, bk, fi.Size(), code, runErr, natOut)
			}
			if natOut != interpOut {
				t.Fatalf("[%s/%s] PARITY MISMATCH (binary=%d bytes):\n--- INTERPRETER ---\n%s\n--- NATIVE (%s) ---\n%s",
					name, bk, fi.Size(), interpOut, bk, natOut)
			}
			t.Logf("[%s/%s] PASS (binary=%d bytes)\noutput:\n%s", name, bk, fi.Size(), natOut)
		})
	}
	return interpOut
}

func wbCopyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) ||
			rel == "build" || strings.HasPrefix(rel, "build"+string(filepath.Separator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
	if err != nil {
		t.Fatalf("copyDir %s -> %s: %v", src, dst, err)
	}
}

// ============================================================================
// AREA 1: Closures y capturas léxicas
// ============================================================================

func TestWhiteBoxAudit_01_ClosuresAndLexicalCaptures(t *testing.T) {
	t.Run("scalar_map_array_captures", func(t *testing.T) {
		src := `
int $base = 10;
string $prefix = "tag:";
array $nums = [3, 7, 11];
map $cfg = {"mode": "strict", "ver": 2};

var $fn = func(int $x): string {
	return $prefix . strval($base + $x + $nums[1]) . ":" . strval($cfg["mode"]) . ":" . strval($cfg["ver"]);
};
print($fn(5));
`
		wbRunCLISourceAcrossBackends(t, "closure_captures", src, []string{"auto", "native", "llvm"}, nil)
	})

	t.Run("multiple_closures_from_same_factory_literal", func(t *testing.T) {
		src := `
public func makeAdder(int $n): mixed {
	return func(int $x): int {
		return $x + $n;
	};
}

var $add5 = makeAdder(5);
var $add100 = makeAdder(100);
print("A:" . strval($add5(1)));
print("B:" . strval($add100(1)));
print("A2:" . strval($add5(2)));
`
		wbRunCLISourceAcrossBackends(t, "closure_factory", src, []string{"auto", "native", "llvm"}, nil)
	})

	t.Run("nested_closures_and_higher_order_calls", func(t *testing.T) {
		src := `
public func applyTwice(mixed $f, int $v): int {
	return $f($f($v));
}

public func makeChain(int $a): mixed {
	int $b = $a * 2;
	return func(int $c): mixed {
		return func(int $d): int {
			return $a + $b + $c + $d;
		};
	};
}

var $step1 = func(int $x): int { return $x + 3; };
print("APPLY:" . strval(applyTwice($step1, 10)));

var $c1 = makeChain(4);
var $c2 = $c1(5);
print("CHAIN:" . strval($c2(6)));
`
		wbRunCLISourceAcrossBackends(t, "closure_nested", src, []string{"auto", "native", "llvm"}, nil)
	})

	t.Run("immediate_invocation_of_returned_closure", func(t *testing.T) {
		src := `
public func makeMul(int $k): mixed {
	return func(int $x): int { return $k * $x; };
}
print(makeMul(6)(7));
`
		wbRunCLISourceAcrossBackends(t, "closure_immediate_call", src, []string{"auto", "native", "llvm"}, nil)
	})

	t.Run("many_closures_over_tls_64_slots", func(t *testing.T) {
		var sb strings.Builder
		for i := 1; i <= 70; i++ {
			sb.WriteString(fmt.Sprintf("int $k%d = %d;\n", i, i))
			sb.WriteString(fmt.Sprintf("var $f%d = func(int $x): int { return $x + $k%d; };\n", i, i))
		}
		sb.WriteString("print(\"F1:\" . strval($f1(0)));\n")
		sb.WriteString("print(\"F64:\" . strval($f64(0)));\n")
		sb.WriteString("print(\"F70:\" . strval($f70(0)));\n")
		sb.WriteString("print(\"SUM:\" . strval($f1(10) + $f70(10)));\n")
		wbRunCLISourceAcrossBackends(t, "closure_tls_70", sb.String(), []string{"auto", "native", "llvm"}, nil)
	})

	t.Run("closure_mutating_captured_map_and_scalar", func(t *testing.T) {
		src := `
int $counter = 1;
map $state = {"hits": 0};
var $inc = func(): string {
	$counter = $counter + 1;
	$state["hits"] = intval($state["hits"]) + 1;
	return strval($counter) . "/" . strval($state["hits"]);
};
print($inc());
print($inc());
print("OUT:" . strval($counter) . "/" . strval($state["hits"]));
`
		wbRunCLISourceAcrossBackends(t, "closure_mutation", src, []string{"auto", "native", "llvm"}, nil)
	})
}

// ============================================================================
// AREA 2: Colecciones y POO
// ============================================================================

func TestWhiteBoxAudit_02_CollectionsOOPAndTryCatch(t *testing.T) {
	t.Run("stack_and_queue_stdlib_growth_and_mixed", func(t *testing.T) {
		src := `
var $st = new Stack();
print("ST_EMPTY:" . strval($st->isEmpty()));
print("ST_EMPTY_POP:" . strval($st->pop() == null));
int $i = 1;
while ($i <= 20) {
	$st->push("item-" . strval($i));
	$i++;
}
print("ST_SIZE:" . strval($st->size()));
print("ST_PEEK:" . strval($st->peek()));
print("ST_POP1:" . strval($st->pop()));
print("ST_POP2:" . strval($st->pop()));
print("ST_SIZE_AFTER:" . strval($st->size()));

var $q = new Queue();
print("Q_EMPTY:" . strval($q->isEmpty()));
print("Q_EMPTY_DEQ:" . strval($q->dequeue() == null));
int $j = 100;
while ($j < 120) {
	$q->enqueue($j);
	$j++;
}
print("Q_SIZE:" . strval($q->size()));
print("Q_PEEK:" . strval($q->peek()));
print("Q_DEQ1:" . strval($q->dequeue()));
print("Q_DEQ2:" . strval($q->dequeue()));
print("Q_SIZE_AFTER:" . strval($q->size()));
`
		wbRunCLISourceAcrossBackends(t, "stack_queue", src, []string{"auto", "native", "llvm"}, nil)
	})

	t.Run("map_foreach_values_and_null_coalescing_and_shorthand_push", func(t *testing.T) {
		src := `
// 1. Foreach over map values (string and int)
map $langs = {"a": "Joss", "b": "LLVM"};
foreach ($langs as $k => $v) {
	print("MAP:" . strval($k) . "=" . strval($v));
}

// 2. Null-coalescing ?? MUST ONLY handle null, NOT 0 / false / ""
int $zero = 0;
bool $fval = false;
string $emptyStr = "";
mixed $nilVal = null;
print("COAL_ZERO:" . strval($zero ?? 99));
print("COAL_FALSE:" . strval($fval ?? true));
print("COAL_EMPTY:" . "[" . strval($emptyStr ?? "fallback") . "]");
print("COAL_NULL:" . strval($nilVal ?? "was_null"));

// 3. Array shorthand push $arr[] = val
array<int> $arr = [10];
$arr[] = 20;
$arr[] = 30;
print("ARR_LEN:" . strval(count($arr)) . ":" . strval($arr[0]) . "," . strval($arr[1]) . "," . strval($arr[2]));
`
		wbRunCLISourceAcrossBackends(t, "map_coal_push", src, []string{"auto", "native", "llvm"}, nil)
	})

	t.Run("oop_inheritance_constructors_polymorphism_and_visibility", func(t *testing.T) {
		src := `
public class BaseEntity {
	public string $origin = "base_origin";
	protected int $secretBase = 41;
	public int $ctorRan = 0;

	Init(int $seed = 7) {
		$this->ctorRan = $seed * 2;
	}

	public func describe(): string {
		return "Base:" . $this->origin . ":" . strval($this->ctorRan);
	}
}

public class ChildEntity extends BaseEntity {
	public string $tag = "child_tag";

	public func readProtected(): int {
		return $this->secretBase + 1;
	}

	public func describe(): string {
		return "Child:" . $this->origin . ":" . $this->tag . ":" . strval($this->ctorRan) . ":" . strval($this->readProtected());
	}
}

var $c = new ChildEntity(5);
print("DIRECT:" . $c->describe());
print("ORIGIN:" . $c->origin);
print("CTOR:" . strval($c->ctorRan));

// Polymorphic call via base-typed variable
BaseEntity $poly = new ChildEntity(9);
print("POLY:" . $poly->describe());
`
		wbRunCLISourceAcrossBackends(t, "oop_inherit", src, []string{"auto", "native", "llvm"}, nil)

		// Also verify visibility enforcement by semantic analyzer
		badVisSrc := `
public class SecretBox {
	private int $pin = 1234;
	private func hidden(): int { return $this->pin; }
}
var $b = new SecretBox();
print($b->pin);
`
		dir := t.TempDir()
		badFile := filepath.Join(dir, "bad_vis.joss")
		_ = os.WriteFile(badFile, []byte(badVisSrc), 0644)
		_, buildOut, err := wbBuildWithCLI(t, dir, "bad_vis.joss", "bad_vis", "llvm")
		if err == nil {
			t.Errorf("expected private field access to be rejected by analyzer, but build succeeded:\n%s", buildOut)
		} else {
			t.Logf("Private visibility correctly rejected:\n%s", strings.TrimSpace(buildOut))
		}
	})

	t.Run("try_catch_mutating_locals_nested_and_loop", func(t *testing.T) {
		src := `
public func mayFail(int $n): int {
	($n < 0) ? {
		throw "neg:" . strval($n);
	}
	return $n * 10;
}

int $acc = 5;
string $trace = "start";
try {
	$acc = $acc + 10;
	$trace = $trace . ">in_try";
	int $v = mayFail(-3);
	$acc = $acc + $v;
} catch ($e) {
	$trace = $trace . ">catch(" . strval($e) . ")";
	$acc = $acc + 100;
}
print("LOCAL_AFTER:" . strval($acc) . "|" . $trace);

// Nested try/catch inside loop + early return try balance check
int $sum = 0;
int $i = 0;
while ($i < 4) {
	try {
		$sum = $sum + 1;
		try {
			($i == 1) ? {
				throw "inner1";
			}
			$sum = $sum + 10;
		} catch ($ie) {
			$sum = $sum + 100;
			($i == 1) ? {
				throw "outer_rethrow";
			}
		}
		($i == 2) ? {
			throw "outer2";
		}
	} catch ($oe) {
		$sum = $sum + 1000;
	}
	$i++;
}
print("LOOP_NESTED_SUM:" . strval($sum));
`
		wbRunCLISourceAcrossBackends(t, "try_catch_mut", src, []string{"auto", "native", "llvm"}, nil)
	})

	t.Run("unboxed_int_pointer_threshold_crash_or_garbage", func(t *testing.T) {
		// In joss_rt.c:470-472, 535-540, 854-858, joss_print_mixed/strval/intval treat any
		// unboxed integer >= JOSS_MIN_PTR_ADDR (0x100000 = 1048576) as a C string pointer!
		src := `
int $below = 1048575;
int $atThresh = 1048576;
int $big = 16777216;
print("BELOW:" . strval($below));
print("AT_STR:" . strval($atThresh));
print("BIG_INT:" . strval(intval($big)));
mixed $mx = $atThresh;
print($mx);
`
		wbRunCLISourceAcrossBackends(t, "int_ptr_thresh", src, []string{"auto", "native", "llvm"}, nil)
	})
}

// ============================================================================
// AREA 3: Criptografía y buffers binarios
// ============================================================================

func TestWhiteBoxAudit_03_CryptoAndBinaryBuffers(t *testing.T) {
	// Precompute real $2b$ bcrypt hashes in Go for cost 4 and cost 10, including 72-byte and 73-byte passwords
	pwShort := "JossAudit!2026"
	hashCost4, err := bcrypt.GenerateFromPassword([]byte(pwShort), 4)
	if err != nil {
		t.Fatalf("bcrypt cost4: %v", err)
	}
	hashCost10, err := bcrypt.GenerateFromPassword([]byte(pwShort), 10)
	if err != nil {
		t.Fatalf("bcrypt cost10: %v", err)
	}
	hashEmpty, err := bcrypt.GenerateFromPassword([]byte(""), 4)
	if err != nil {
		t.Fatalf("bcrypt empty: %v", err)
	}
	pw72 := strings.Repeat("A", 72)
	pw73 := pw72 + "Z"
	hash72, err := bcrypt.GenerateFromPassword([]byte(pw72), 4)
	if err != nil {
		t.Fatalf("bcrypt 72: %v", err)
	}

	t.Run("hash_vectors_and_bcrypt_valid_invalid", func(t *testing.T) {
		src := fmt.Sprintf(`
print("SHA256_ABC:" . sha256("abc"));
print("SHA256_EMPTY:" . sha256(""));
print("MD5_ABC:" . md5("abc"));
print("SHA256_B64:" . sha256_base64("abc"));
print("SHA1_B64:" . sha1_base64("dGhlIHNhbXBsZSBub25jZQ==258EAFA5-E914-47DA-95CA-C5AB0DC85B11"));

string $pbk = password_hash("correct-horse-battery");
print("PBKDF2_PREFIX:" . strval(str_starts_with($pbk, "$pbkdf2-sha256$")));
print("PBKDF2_OK:" . strval(password_verify($pbk, "correct-horse-battery")));
print("PBKDF2_BAD:" . strval(password_verify($pbk, "wrong")));

print("BCRYPT_C4_OK:" . strval(password_verify(%q, %q)));
print("BCRYPT_C4_BAD:" . strval(password_verify(%q, "wrong")));
print("BCRYPT_C10_OK:" . strval(password_verify(%q, %q)));
print("BCRYPT_EMPTY_OK:" . strval(password_verify(%q, "")));
print("BCRYPT_EMPTY_BAD:" . strval(password_verify(%q, "x")));
print("BCRYPT_72_OK:" . strval(password_verify(%q, %q)));
print("BCRYPT_73_TRUNC_OK:" . strval(password_verify(%q, %q)));

// Malformed $2b$ hashes must return false without crash
print("BCRYPT_INV_SHORT:" . strval(password_verify("$2b$10$short", "x")));
print("BCRYPT_INV_COST03:" . strval(password_verify("$2b$03$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234", "x")));
print("BCRYPT_INV_COST32:" . strval(password_verify("$2b$32$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234", "x")));
print("BCRYPT_INV_CHAR:" . strval(password_verify("$2b$10$!!!!!!!!!!!!!!!!!!!!!!ABCDEFGHIJKLMNOPQRSTUVWXYZ01234", "x")));
`,
			string(hashCost4), pwShort,
			string(hashCost4),
			string(hashCost10), pwShort,
			string(hashEmpty),
			string(hashEmpty),
			string(hash72), pw72,
			string(hash72), pw73,
		)

		dir := t.TempDir()
		srcPath := filepath.Join(dir, "crypto_audit.joss")
		_ = os.WriteFile(srcPath, []byte(src), 0644)
		exePath, buildOut, err := wbBuildWithCLI(t, dir, "crypto_audit.joss", "crypto_audit", "llvm")
		if err != nil {
			t.Fatalf("joss build --backend=llvm failed: %v\n%s", err, buildOut)
		}
		out, code, runErr := wbRunBinary(t, exePath, dir, nil)
		if runErr != nil || code != 0 {
			t.Fatalf("crypto_audit exited code=%d err=%v\n%s", code, runErr, out)
		}

		// Verify standard test vectors
		wantPairs := map[string]string{
			"SHA256_ABC":         "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
			"SHA256_EMPTY":       "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			"MD5_ABC":            "900150983cd24fb0d6963f7d28e17f72",
			"SHA256_B64":         "ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0=",
			"SHA1_B64":           "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=",
			"PBKDF2_PREFIX":      "true",
			"PBKDF2_OK":          "true",
			"PBKDF2_BAD":         "false",
			"BCRYPT_C4_OK":       "true",
			"BCRYPT_C4_BAD":      "false",
			"BCRYPT_C10_OK":      "true",
			"BCRYPT_EMPTY_OK":    "true",
			"BCRYPT_EMPTY_BAD":   "false",
			"BCRYPT_72_OK":       "true",
			"BCRYPT_73_TRUNC_OK": "true",
			"BCRYPT_INV_SHORT":   "false",
			"BCRYPT_INV_COST03":  "false",
			"BCRYPT_INV_COST32":  "false",
			"BCRYPT_INV_CHAR":    "false",
		}
		gotMap := make(map[string]string)
		for _, line := range strings.Split(out, "\n") {
			if idx := strings.IndexByte(line, ':'); idx > 0 {
				gotMap[line[:idx]] = line[idx+1:]
			}
		}
		for k, want := range wantPairs {
			if gotMap[k] != want {
				t.Errorf("crypto vector %s = %q, want %q", k, gotMap[k], want)
			}
		}
		fi, _ := os.Stat(exePath)
		t.Logf("crypto_audit binary=%d bytes\n%s", fi.Size(), out)
	})

	t.Run("binary_null_bytes_and_bytebuffer", func(t *testing.T) {
		dir := t.TempDir()
		binPayload := []byte{'A', 0x00, 'B', 0x00, 'C', 0xFF, 0x00, 'D'}
		binFile := filepath.Join(dir, "payload.bin")
		if err := os.WriteFile(binFile, binPayload, 0644); err != nil {
			t.Fatalf("write payload.bin: %v", err)
		}
		wantShaSum := sha256.Sum256(binPayload)
		wantShaHex := hex.EncodeToString(wantShaSum[:])
		wantMd5Sum := md5.Sum(binPayload)
		wantMd5Hex := hex.EncodeToString(wantMd5Sum[:])
		wantByteStrSha := sha256.Sum256([]byte{'X', 0x00, 'Y'})
		wantByteStrHex := hex.EncodeToString(wantByteStrSha[:])

		src := fmt.Sprintf(`
mixed $raw = file_get_contents("payload.bin");
print("FILE_LEN:" . strval(strlen($raw)));
print("FILE_SHA256:" . sha256($raw));
print("FILE_MD5:" . md5($raw));

// __joss_byte_string(0) embedded null byte handling vs interpreter
string $nul = __joss_byte_string(0);
string $comb = "X" . $nul . "Y";
print("BYTESTR_LEN:" . strval(strlen($comb)));
print("BYTESTR_SHA256:" . sha256($comb));

// ByteBuffer C11 class
var $buf = new ByteBuffer(4);
$buf->appendByte(65);
$buf->appendByte(0);
$buf->appendByte(66);
$buf->appendString("CD");
print("BUF_LEN:" . strval($buf->len()));
print("BUF_B0:" . strval($buf->get(0)));
print("BUF_B1:" . strval($buf->get(1)));
print("BUF_B2:" . strval($buf->get(2)));
$buf->set(1, 90);
print("BUF_STR:" . $buf->toString());
$buf->free();
`)
		// First check interpreter output for __joss_byte_string(0)
		// (Run inside dir so payload.bin resolves)
		srcPath := filepath.Join(dir, "bin_audit.joss")
		_ = os.WriteFile(srcPath, []byte(src), 0644)
		exePath, buildOut, err := wbBuildWithCLI(t, dir, "bin_audit.joss", "bin_audit", "llvm")
		if err != nil {
			t.Fatalf("joss build bin_audit failed: %v\n%s", err, buildOut)
		}
		out, code, runErr := wbRunBinary(t, exePath, dir, nil)
		if runErr != nil || code != 0 {
			t.Fatalf("bin_audit exited code=%d err=%v\n%s", code, runErr, out)
		}
		got := make(map[string]string)
		for _, line := range strings.Split(out, "\n") {
			if idx := strings.IndexByte(line, ':'); idx > 0 {
				got[line[:idx]] = line[idx+1:]
			}
		}
		if got["FILE_LEN"] != "8" {
			t.Errorf("FILE_LEN = %q, want 8", got["FILE_LEN"])
		}
		if got["FILE_SHA256"] != wantShaHex {
			t.Errorf("FILE_SHA256 = %q, want %q", got["FILE_SHA256"], wantShaHex)
		}
		if got["FILE_MD5"] != wantMd5Hex {
			t.Errorf("FILE_MD5 = %q, want %q", got["FILE_MD5"], wantMd5Hex)
		}
		if got["BYTESTR_LEN"] != "3" {
			t.Errorf("BYTESTR_LEN with embedded \\x00 = %q, want 3 (bug: __joss_byte_string(0) returns C-string truncated at NUL!)", got["BYTESTR_LEN"])
		}
		if got["BYTESTR_SHA256"] != wantByteStrHex {
			t.Errorf("BYTESTR_SHA256 with embedded \\x00 = %q, want %q", got["BYTESTR_SHA256"], wantByteStrHex)
		}
		if got["BUF_LEN"] != "5" || got["BUF_B0"] != "65" || got["BUF_B1"] != "0" || got["BUF_B2"] != "66" || got["BUF_STR"] != "AZBCD" {
			t.Errorf("ByteBuffer mismatch: %v", got)
		}
		t.Logf("bin_audit output:\n%s", out)
	})
}

// ============================================================================
// AREA 4: Servidor HTTP concurrente, Multipart, Middlewares, Rate Limiting y Cookies/Sesiones
// ============================================================================

func TestWhiteBoxAudit_04_HTTPConcurrentMultipartMiddlewareSessions(t *testing.T) {
	dir := t.TempDir()
	// Create a sensitive file on disk in server working dir to test static path exposure
	_ = os.WriteFile(filepath.Join(dir, "env.joss"), []byte("APP_KEY=top_secret_audit_key\nJWT_SECRET=super_jwt_secret_123456\n"), 0644)

	routesSrc := `
Router::get("/ping", "AuditCtrl@ping")
Router::get("/json-types", "AuditCtrl@jsonTypes")
Router::get("/flash-set", "AuditCtrl@flashSet")
Router::get("/flash-read", "AuditCtrl@flashRead")
Router::get("/cookie-on-raw", "AuditCtrl@cookieOnRaw")
Router::post("/api/upload-bin", "AuditCtrl@uploadBin")
Router::get("/session-counter", "AuditCtrl@sessionCounter")

Router::group("auth", func() {
	Router::get("/protected", "AuditCtrl@protectedPage")
})

Router::group("auth_api", func() {
	Router::get("/api/whoami", "AuditCtrl@whoami")
})

Router::group("throttle:2,10", func() {
	Router::get("/rl", "AuditCtrl@ping")
})
`
	if err := os.WriteFile(filepath.Join(dir, "routes.joss"), []byte(routesSrc), 0644); err != nil {
		t.Fatalf("write routes.joss: %v", err)
	}

	mainSrc := `
public class AuditCtrl {
	public func ping(): string {
		return "PONG:" . Request::ip();
	}

	public func jsonTypes(): mixed {
		return json({
			"int_nonzero": 42,
			"int_zero": 0,
			"float_val": 1.5,
			"bool_true": true,
			"bool_false": false,
			"text": "ok\tline"
		}, 201);
	}

	public func flashSet(): mixed {
		return redirect("/flash-read", 302)->with("notice", "saved_ok");
	}

	public func flashRead(): string {
		return "FLASH:" . strval(session("notice", "none"));
	}

	public func cookieOnRaw(): mixed {
		return Response::raw("RAW_OK", 200, "text/plain")->withCookie("pref", "dark");
	}

	public func uploadBin(): mixed {
		mixed $f = Request::file("binfile");
		string $after = strval(Request::input("after_field", ""));
		int $sz = 0;
		string $sha = "";
		($f != null) ? {
			$sz = intval($f["size"]);
			mixed $data = file_get_contents(strval($f["path"]));
			$sha = sha256($data);
		}
		return json({"size": $sz, "sha": $sha, "after": $after}, 200);
	}

	public func sessionCounter(): string {
		int $c = intval(Session::get("hits", 0)) + 1;
		Session::put("hits", $c);
		return "HITS:" . strval($c);
	}

	public func protectedPage(): string {
		return "PROTECTED_OK";
	}

	public func whoami(): mixed {
		return json({"uid": Session::get("user_id", 0), "role": Session::get("user_role", "")}, 200);
	}
}

public class Main {
	Init main() {
		Server::start();
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.joss"), []byte(mainSrc), 0644); err != nil {
		t.Fatalf("write main.joss: %v", err)
	}

	exePath, buildOut, err := wbBuildWithCLI(t, dir, "main.joss", "wb_http_server", "llvm")
	if err != nil {
		t.Fatalf("joss build wb_http_server failed: %v\n%s", err, buildOut)
	}
	fi, _ := os.Stat(exePath)
	t.Logf("wb_http_server built (%d bytes)", fi.Size())

	// 1. Run with SESSION_DRIVER=file and valid JWT_SECRET
	port1 := allocFreePort(t)
	sessFile := filepath.Join(dir, "storage", "wb_sessions.json")
	cmd1, logs1 := startBuiltServer(t, exePath, port1, []string{
		"SESSION_DRIVER=file",
		"SESSION_FILE=" + sessFile,
		"JWT_SECRET=wb_audit_secret_987654321",
		"RATE_LIMIT_REQUESTS=0",
	})
	t.Cleanup(func() {
		if cmd1 != nil && cmd1.Process != nil {
			_ = cmd1.Process.Kill()
			_ = cmd1.Wait()
		}
	})
	base1 := fmt.Sprintf("http://127.0.0.1:%d", port1)
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	t.Run("json_types_float_and_zero_bug", func(t *testing.T) {
		resp, err := client.Get(base1 + "/json-types")
		if err != nil {
			t.Fatalf("GET /json-types: %v\n%s", err, logs1.String())
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != 201 {
			t.Errorf("status = %d, want 201", resp.StatusCode)
		}
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("invalid JSON %q: %v", string(body), err)
		}
		if fv, ok := parsed["float_val"].(float64); !ok || fv != 1.5 {
			t.Errorf("JSON float_val = %#v (raw=%s), want 1.5 (JOSS_TAG_FLOAT unbox bug in json_encode_value!)", parsed["float_val"], string(body))
		}
		if zv, ok := parsed["int_zero"].(float64); !ok || zv != 0 {
			t.Errorf("JSON int_zero = %#v (raw=%s), want 0 (val==0 emitted as null!)", parsed["int_zero"], string(body))
		}
		if bv, ok := parsed["bool_false"].(bool); !ok || bv {
			t.Errorf("JSON bool_false = %#v (raw=%s), want false", parsed["bool_false"], string(body))
		}
	})

	t.Run("flash_redirect_and_withCookie_on_raw", func(t *testing.T) {
		r1, err := client.Get(base1 + "/flash-set")
		if err != nil {
			t.Fatalf("GET /flash-set: %v", err)
		}
		_ = r1.Body.Close()
		if r1.StatusCode != 302 || r1.Header.Get("Location") != "/flash-read" {
			t.Errorf("redirect status=%d location=%q", r1.StatusCode, r1.Header.Get("Location"))
		}
		var sidCookie *http.Cookie
		for _, c := range r1.Cookies() {
			if c.Name == "joss_session" {
				sidCookie = c
			}
		}
		if sidCookie == nil {
			t.Fatalf("missing joss_session cookie on /flash-set")
		}
		req2, _ := http.NewRequest("GET", base1+"/flash-read", nil)
		req2.AddCookie(sidCookie)
		r2, err := client.Do(req2)
		if err != nil {
			t.Fatalf("GET /flash-read: %v", err)
		}
		b2, _ := io.ReadAll(r2.Body)
		_ = r2.Body.Close()
		if string(b2) != "FLASH:saved_ok" {
			t.Errorf("/flash-read = %q, want %q", string(b2), "FLASH:saved_ok")
		}
		// Flash should be consumed on second read
		req3, _ := http.NewRequest("GET", base1+"/flash-read", nil)
		req3.AddCookie(sidCookie)
		r3, err := client.Do(req3)
		if err != nil {
			t.Fatalf("GET /flash-read 2nd: %v", err)
		}
		b3, _ := io.ReadAll(r3.Body)
		_ = r3.Body.Close()
		if string(b3) != "FLASH:none" {
			t.Errorf("/flash-read 2nd = %q, want %q", string(b3), "FLASH:none")
		}

		// Test withCookie(Response::raw(...))
		rc, err := client.Get(base1 + "/cookie-on-raw")
		if err != nil {
			t.Fatalf("GET /cookie-on-raw: %v", err)
		}
		bc, _ := io.ReadAll(rc.Body)
		_ = rc.Body.Close()
		foundPref := false
		for _, c := range rc.Cookies() {
			if c.Name == "pref" && c.Value == "dark" {
				foundPref = true
			}
		}
		if !foundPref {
			t.Errorf("withCookie(Response::raw) missing pref=dark Set-Cookie; headers=%v body=%q", rc.Header, string(bc))
		}
	})

	t.Run("multipart_binary_null_bytes_and_trailing_field", func(t *testing.T) {
		binData := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00, 0x1A, 0x0A, 'E', 'N', 'D'}
		wantSum := sha256.Sum256(binData)
		wantSha := hex.EncodeToString(wantSum[:])

		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		fw, err := mw.CreateFormFile("binfile", "test.png")
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		_, _ = fw.Write(binData)
		_ = mw.WriteField("after_field", "trailing_ok")
		_ = mw.Close()

		req, _ := http.NewRequest("POST", base1+"/api/upload-bin", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST /upload-bin: %v\n%s", err, logs1.String())
		}
		rb, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		var res map[string]any
		if err := json.Unmarshal(rb, &res); err != nil {
			t.Fatalf("invalid json from /upload-bin: %q (%v)", string(rb), err)
		}
		if int(res["size"].(float64)) != len(binData) || res["sha"] != wantSha || res["after"] != "trailing_ok" {
			t.Errorf("multipart mismatch: got=%v, want size=%d sha=%s after=trailing_ok", res, len(binData), wantSha)
		}
	})

	t.Run("concurrent_sessions_and_throttle", func(t *testing.T) {
		const workers = 24
		var wg sync.WaitGroup
		errs := make(chan string, workers)
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				// Create own session and hit /session-counter twice
				r1, err := client.Get(base1 + "/session-counter")
				if err != nil {
					errs <- fmt.Sprintf("w%d req1: %v", id, err)
					return
				}
				b1, _ := io.ReadAll(r1.Body)
				_ = r1.Body.Close()
				var sc *http.Cookie
				for _, c := range r1.Cookies() {
					if c.Name == "joss_session" {
						sc = c
					}
				}
				if sc == nil || string(b1) != "HITS:1" {
					errs <- fmt.Sprintf("w%d req1 body=%q cookie=%v", id, string(b1), sc)
					return
				}
				req2, _ := http.NewRequest("GET", base1+"/session-counter", nil)
				req2.AddCookie(sc)
				r2, err := client.Do(req2)
				if err != nil {
					errs <- fmt.Sprintf("w%d req2: %v", id, err)
					return
				}
				b2, _ := io.ReadAll(r2.Body)
				_ = r2.Body.Close()
				if string(b2) != "HITS:2" {
					errs <- fmt.Sprintf("w%d req2 body=%q, want HITS:2", id, string(b2))
				}
			}(w)
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			t.Error(e)
		}

		// File session persistence check
		if _, err := os.Stat(sessFile); err != nil {
			t.Errorf("expected session file at %s: %v", sessFile, err)
		}

		// Auth middleware redirect check
		rProt, err := client.Get(base1 + "/protected")
		if err != nil {
			t.Fatalf("GET /protected: %v", err)
		}
		_ = rProt.Body.Close()
		if rProt.StatusCode != 302 || rProt.Header.Get("Location") != "/login" {
			t.Errorf("/protected unauth = %d %q, want 302 /login", rProt.StatusCode, rProt.Header.Get("Location"))
		}

		// Auth API JWT check
		tok := makeTestHS256JWT("wb_audit_secret_987654321", fmt.Sprintf(`{"user_id":88,"role":"auditor","exp":%d}`, time.Now().Add(time.Hour).Unix()))
		reqApi, _ := http.NewRequest("GET", base1+"/api/whoami", nil)
		reqApi.Header.Set("Authorization", "Bearer "+tok)
		rApi, err := client.Do(reqApi)
		if err != nil {
			t.Fatalf("GET /api/whoami: %v", err)
		}
		apiBody, _ := io.ReadAll(rApi.Body)
		_ = rApi.Body.Close()
		if rApi.StatusCode != 200 || !strings.Contains(string(apiBody), `"uid":88`) {
			t.Errorf("/api/whoami = %d %s", rApi.StatusCode, string(apiBody))
		}
	})

	t.Run("static_file_exposure_of_env_joss", func(t *testing.T) {
		rEnv, err := client.Get(base1 + "/env.joss")
		if err != nil {
			t.Fatalf("GET /env.joss: %v", err)
		}
		eb, _ := io.ReadAll(rEnv.Body)
		_ = rEnv.Body.Close()
		if rEnv.StatusCode == 200 && strings.Contains(string(eb), "top_secret_audit_key") {
			t.Errorf("SECURITY BUG: HTTP server publicly served /env.joss (status=%d, body=%q)", rEnv.StatusCode, string(eb))
		}
	})

	t.Run("invalid_session_driver_rejected", func(t *testing.T) {
		port2 := allocFreePort(t)
		cmd2, logs2 := startBuiltServer(t, exePath, port2, []string{
			"SESSION_DRIVER=invalid_backend",
			"RATE_LIMIT_REQUESTS=0",
		})
		t.Cleanup(func() {
			if cmd2 != nil && cmd2.Process != nil {
				_ = cmd2.Process.Kill()
				_ = cmd2.Wait()
			}
		})
		r, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", port2))
		if err != nil {
			t.Fatalf("GET /ping invalid driver: %v\n%s", err, logs2.String())
		}
		rb, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if r.StatusCode != 500 || !strings.Contains(string(rb), "not supported") {
			t.Errorf("expected 500 session driver error, got %d %q", r.StatusCode, string(rb))
		}
	})

	t.Run("session_driver_memory_mode", func(t *testing.T) {
		port3 := allocFreePort(t)
		cmd3, logs3 := startBuiltServer(t, exePath, port3, []string{
			"SESSION_DRIVER=memory",
			"RATE_LIMIT_REQUESTS=0",
		})
		t.Cleanup(func() {
			if cmd3 != nil && cmd3.Process != nil {
				_ = cmd3.Process.Kill()
				_ = cmd3.Wait()
			}
		})
		base3 := fmt.Sprintf("http://127.0.0.1:%d", port3)
		r1, err := client.Get(base3 + "/session-counter")
		if err != nil {
			t.Fatalf("GET memory session 1: %v\n%s", err, logs3.String())
		}
		b1, _ := io.ReadAll(r1.Body)
		_ = r1.Body.Close()
		var sc *http.Cookie
		for _, c := range r1.Cookies() {
			if c.Name == "joss_session" {
				sc = c
			}
		}
		if sc == nil || string(b1) != "HITS:1" {
			t.Fatalf("memory session 1 got %q cookie=%v", string(b1), sc)
		}
		req2, _ := http.NewRequest("GET", base3+"/session-counter", nil)
		req2.AddCookie(sc)
		r2, err := client.Do(req2)
		if err != nil {
			t.Fatalf("GET memory session 2: %v", err)
		}
		b2, _ := io.ReadAll(r2.Body)
		_ = r2.Body.Close()
		if string(b2) != "HITS:2" {
			t.Errorf("memory session 2 got %q, want HITS:2", string(b2))
		}
	})
}

// ============================================================================
// AREA 5: Auth / 2FA TOTP / JWT / OAuth / CSRF / Request facade
// ============================================================================

func TestWhiteBoxAudit_05_Auth2FATOTPJWTOAuthCSRFRequest(t *testing.T) {
	// Mock OAuth server
	oauthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"access_token":"mock-oauth-token-123"}`))
			return
		}
		if r.URL.Path == "/user" {
			_, _ = w.Write([]byte(`{"id":"9001","email":"oauth@joss.dev","name":"OAuthUser"}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer oauthSrv.Close()

	t.Run("totp_jwt_oauth_csrf_unit_parity", func(t *testing.T) {
		src := `
// 1. TOTP RFC 6238 edge cases
print("TOTP_59:" . totp_generate("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", 59));
print("TOTP_1111111109:" . totp_generate("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", 1111111109));
print("TOTP_DRIFT_P1:" . strval(totp_verify("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "287082", 65)));
print("TOTP_DRIFT_OUT:" . strval(totp_verify("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "287082", 120)));
print("TOTP_BAD_FMT:" . strval(totp_verify("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "12345", 59)));
print("TOTP_BAD_ALPHA:" . strval(totp_verify("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "12345a", 59)));

// 2. MFA::generateTOTP(null) & recovery codes
var $mfaInfo = MFA::generateTOTP(null);
print("MFA_SEC_LEN:" . strval(strlen(strval($mfaInfo["secret"]))));
print("MFA_QR:" . strval(str_contains(strval($mfaInfo["qr_url"]), "api.qrserver.com")));

// 3. JWT sign/verify & expired token check via __joss_jwt_parse_verified
string $validTok = jwt_sign("{\"sub\":7,\"exp\":4102444800}", "my-jwt-sec");
string $expTok = jwt_sign("{\"sub\":7,\"exp\":1000}", "my-jwt-sec");
print("JWT_VALID:" . strval(jwt_verify($validTok, "my-jwt-sec")));
map $claimsValid = __joss_jwt_parse_verified($validTok, "my-jwt-sec");
print("JWT_PARSE_VALID:" . strval($claimsValid != null));
map $claimsExp = __joss_jwt_parse_verified($expTok, "my-jwt-sec");
print("JWT_PARSE_EXP:" . strval($claimsExp == null));

// 4. OAuth socialRedirect
string $gUrl = Auth::socialRedirect("google", "https://app.test/cb", "st123");
print("OAUTH_GOOGLE:" . strval(str_contains($gUrl, "accounts.google.com") && str_contains($gUrl, "st123")));

// 5. CSRF token generation (calls rand() with 0 args in std/http/csrf.joss) & csrf_field()
var $sess = new Session();
$sess->id = "sess-audit-1";
string $t1 = Csrf::token($sess);
string $t2 = Csrf::token($sess);
print("CSRF_STABLE:" . strval(strlen($t1) == 64 && $t1 == $t2));
print("CSRF_VERIFY_OK:" . strval(Csrf::verify($t1, $t1)));
print("CSRF_VERIFY_EMPTY:" . strval(Csrf::verify("", $t1)));
`
		dir := t.TempDir()
		srcPath := filepath.Join(dir, "auth_audit.joss")
		_ = os.WriteFile(srcPath, []byte(src), 0644)
		exePath, buildOut, err := wbBuildWithCLI(t, dir, "auth_audit.joss", "auth_audit", "llvm")
		if err != nil {
			t.Fatalf("joss build auth_audit failed: %v\n%s", err, buildOut)
		}
		out, code, runErr := wbRunBinary(t, exePath, dir, []string{
			"GOOGLE_CLIENT_ID=gid-123",
			"GOOGLE_CLIENT_SECRET=gsec-456",
		})
		if runErr != nil || code != 0 {
			t.Fatalf("auth_audit exited code=%d err=%v\n%s", code, runErr, out)
		}
		got := make(map[string]string)
		for _, line := range strings.Split(out, "\n") {
			if idx := strings.IndexByte(line, ':'); idx > 0 {
				got[line[:idx]] = line[idx+1:]
			}
		}
		want := map[string]string{
			"TOTP_59":           "287082",
			"TOTP_1111111109":   "081804",
			"TOTP_DRIFT_P1":     "true",
			"TOTP_DRIFT_OUT":    "false",
			"TOTP_BAD_FMT":      "false",
			"TOTP_BAD_ALPHA":    "false",
			"MFA_SEC_LEN":       "16",
			"MFA_QR":            "true",
			"JWT_VALID":         "true",
			"JWT_PARSE_VALID":   "true",
			"JWT_PARSE_EXP":     "true",
			"OAUTH_GOOGLE":      "true",
			"CSRF_STABLE":       "true",
			"CSRF_VERIFY_OK":    "true",
			"CSRF_VERIFY_EMPTY": "false",
		}
		for k, w := range want {
			if got[k] != w {
				t.Errorf("%s = %q, want %q", k, got[k], w)
			}
		}
		t.Logf("auth_audit output:\n%s", out)
	})

	t.Run("csrf_field_builtin_llvm_link_mismatch", func(t *testing.T) {
		// runtime/native/joss_web_rt.c:1923 defines `csrf_field(void)` without `joss_` prefix,
		// whereas LLVM emitter calls `@joss_csrf_field` -> lld-link undefined symbol: joss_csrf_field
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "csrf_link.joss"), []byte("print(csrf_field());\n"), 0644)
		_, buildOut, err := wbBuildWithCLI(t, dir, "csrf_link.joss", "csrf_link", "llvm")
		if err != nil {
			t.Errorf("csrf_field() fails LLVM link (csrf_field vs joss_csrf_field mismatch): %v\n%s", err, buildOut)
		}
	})

	t.Run("oauth_callback_and_request_facade_http", func(t *testing.T) {
		dir := t.TempDir()
		routesSrc := `
Router::post("/api/facade-probe", "FacadeCtrl@probe")
Router::get("/oauth-cb", "FacadeCtrl@oauthCb")
`
		mainSrc := `
public class FacadeCtrl {
	public func probe(): mixed {
		return json({
			"root": Request::root(),
			"method": Request::method(),
			"is_post": Request::isMethod("POST"),
			"path": Request::path(),
			"cookie": Request::cookie("theme", "def"),
			"header_ci": Request::header("X-Custom-Header"),
			"bearer": Request::bearerToken(),
			"has_name": Request::has("name"),
			"all_count": count(Request::all()),
			"except_count": count(Request::except(["secret"]))
		}, 200);
	}

	public func oauthCb(): mixed {
		var $res = Auth::socialCallback("github", "code-xyz", "https://app.test/cb");
		return json({"ok": $res->success, "err": $res->error}, 200);
	}
}

public class Main {
	Init main() {
		Server::start();
	}
}
`
		_ = os.WriteFile(filepath.Join(dir, "routes.joss"), []byte(routesSrc), 0644)
		_ = os.WriteFile(filepath.Join(dir, "main.joss"), []byte(mainSrc), 0644)
		exePath, buildOut, err := wbBuildWithCLI(t, dir, "main.joss", "facade_srv", "llvm")
		if err != nil {
			t.Fatalf("joss build facade_srv failed: %v\n%s", err, buildOut)
		}
		port := allocFreePort(t)
		cmd, logs := startBuiltServer(t, exePath, port, []string{
			"GITHUB_CLIENT_ID=ghid",
			"GITHUB_CLIENT_SECRET=ghsec",
			"OAUTH_GITHUB_TOKEN_URL=" + oauthSrv.URL + "/token",
			"OAUTH_GITHUB_USERINFO_URL=" + oauthSrv.URL + "/user",
			"RATE_LIMIT_REQUESTS=0",
		})
		t.Cleanup(func() {
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		req, _ := http.NewRequest("POST", base+"/api/facade-probe", strings.NewReader(`{"name":"joss","secret":"123"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", "theme=ocean")
		req.Header.Set("X-Custom-Header", "AuditVal")
		req.Header.Set("Authorization", "Bearer tok_abc")
		resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err != nil {
			t.Fatalf("POST /api/facade-probe: %v\n%s", err, logs.String())
		}
		rb, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		var got map[string]any
		if err := json.Unmarshal(rb, &got); err != nil {
			t.Fatalf("invalid json /api/facade-probe: %q (%v)", string(rb), err)
		}
		if got["method"] != "POST" || got["is_post"] != true || got["path"] != "/api/facade-probe" ||
			got["cookie"] != "ocean" || got["header_ci"] != "AuditVal" || got["bearer"] != "tok_abc" ||
			got["has_name"] != true {
			t.Errorf("Request facade mismatch: %v", got)
		}
		if int(got["all_count"].(float64)) < 2 || int(got["except_count"].(float64)) != int(got["all_count"].(float64))-1 {
			t.Errorf("Request::all/except mismatch: %v", got)
		}

		// Also verify OAuth HTTP roundtrip reached mock token/userinfo server and failed cleanly at DB check
		ro, err := (&http.Client{Timeout: 5 * time.Second}).Get(base + "/oauth-cb")
		if err != nil {
			t.Fatalf("GET /oauth-cb: %v", err)
		}
		ob, _ := io.ReadAll(ro.Body)
		_ = ro.Body.Close()
		if !strings.Contains(string(ob), "Base de datos no disponible") {
			t.Errorf("OAuth callback expected clean DB-unavailable error after mock HTTP token/userinfo exchange, got: %s", string(ob))
		}
	})
}

// ============================================================================
// AREA 6: Base de datos y Schema::* DDL
// ============================================================================

func TestWhiteBoxAudit_06_DatabaseAndSchemaDDL(t *testing.T) {
	t.Run("grandb_sql_identifier_starting_with_A_strpos_bug", func(t *testing.T) {
		// In std/database/grandb.joss:69:
		// bool $valid = strpos("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_", $character) !== false
		// In C11 runtime (joss_web_rt.c:135), joss_strpos returns 0 for index-0 AND 0 for not-found!
		src := `
print("STRPOS_A:" . strval(strpos("ABC", "A") !== false));
print("STRPOS_Z:" . strval(strpos("ABC", "Z") !== false));
var $q = GranDB::table("Accounts")->select("Amount, id")->where("Active", 1);
print("SQL:" . $q->toSql());
`
		wbRunCLISourceAcrossBackends(t, "grandb_ident_A", src, []string{"llvm"}, nil)
	})

	t.Run("schema_create_mixed_closure_param_bug", func(t *testing.T) {
		dir := t.TempDir()
		src := `
bool $created = Schema::create("mix_tbl", func(mixed $t): void {
	$t->id("id");
	$t->string("name", 50);
});
print(strval($created));
`
		_ = os.WriteFile(filepath.Join(dir, "mix_ddl.joss"), []byte(src), 0644)
		_, buildOut, err := wbBuildWithCLI(t, dir, "mix_ddl.joss", "mix_ddl", "llvm")
		if err != nil {
			t.Errorf("Schema::create with func(mixed $t) fails link (Blueprint method lowered as naked @id/@string): %v\n%s", err, buildOut)
		}
	})

	t.Run("sqlite_schema_ddl_on_host", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "audit.sqlite")
		src := `
bool $conn = __joss_db_is_connected();
print("CONN:" . strval($conn));
bool $created = Schema::create("audit_users", func(Blueprint $t): void {
	$t->id("id");
	$t->string("email", 120)->unique();
	$t->integer("age")->nullable();
});
print("CREATED:" . strval($created));
print("HAS_TABLE:" . strval(Schema::hasTable("audit_users")));
print("HAS_COL_EMAIL:" . strval(Schema::hasColumn("audit_users", "email")));
print("HAS_COL_MISSING:" . strval(Schema::hasColumn("audit_users", "nope")));
print("DROP:" . strval(Schema::dropIfExists("audit_users")));
print("AFTER_DROP:" . strval(Schema::hasTable("audit_users")));
`
		srcPath := filepath.Join(dir, "sqlite_ddl.joss")
		_ = os.WriteFile(srcPath, []byte(src), 0644)
		exePath, buildOut, err := wbBuildWithCLI(t, dir, "sqlite_ddl.joss", "sqlite_ddl", "llvm")
		if err != nil {
			t.Fatalf("joss build sqlite_ddl failed: %v\n%s", err, buildOut)
		}
		out, code, runErr := wbRunBinary(t, exePath, dir, []string{
			"DB_CONNECTION=sqlite",
			"DB_DATABASE=" + dbPath,
		})
		if runErr != nil || code != 0 {
			t.Fatalf("sqlite_ddl exited code=%d err=%v\n%s", code, runErr, out)
		}
		want := "CONN:true\nCREATED:true\nHAS_TABLE:true\nHAS_COL_EMAIL:true\nHAS_COL_MISSING:false\nDROP:true\nAFTER_DROP:false"
		if out != want {
			t.Errorf("SQLite Schema DDL failed (missing sqlite3.h/libsqlite3 on Windows toolchain?):\ngot:\n%s\nwant:\n%s", out, want)
		}
	})

	t.Run("postgres_wire_schema_ddl_and_reconnect", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen pg mock: %v", err)
		}
		defer ln.Close()
		var queries []string
		var qmu sync.Mutex
		var connSeq int
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				connSeq++
				cid := connSeq
				go func(conn net.Conn, id int) {
					defer conn.Close()
					lenBuf := make([]byte, 4)
					if _, err := io.ReadFull(conn, lenBuf); err != nil {
						return
					}
					mlen := binary.BigEndian.Uint32(lenBuf)
					startup := make([]byte, mlen-4)
					if _, err := io.ReadFull(conn, startup); err != nil {
						return
					}
					_, _ = conn.Write([]byte{'R', 0, 0, 0, 8, 0, 0, 0, 0})
					rfq := []byte{'Z', 0, 0, 0, 5, 'I'}
					_, _ = conn.Write(rfq)
					for {
						hdr := make([]byte, 5)
						if _, err := io.ReadFull(conn, hdr); err != nil {
							return
						}
						qlen := binary.BigEndian.Uint32(hdr[1:5])
						qbody := make([]byte, qlen-4)
						if _, err := io.ReadFull(conn, qbody); err != nil {
							return
						}
						sqlStr := strings.TrimRight(string(qbody), "\x00")
						if id == 1 && strings.Contains(sqlStr, "CREATE TABLE") {
							// Drop connection on first CREATE TABLE to force wire reconnect
							return
						}
						qmu.Lock()
						queries = append(queries, sqlStr)
						qmu.Unlock()
						if strings.Contains(sqlStr, "information_schema.tables") || strings.Contains(sqlStr, "information_schema.columns") {
							col := "table_name\x00"
							tb := make([]byte, 2+len(col)+18)
							binary.BigEndian.PutUint16(tb[0:2], 1)
							copy(tb[2:], col)
							tm := make([]byte, 5+len(tb))
							tm[0] = 'T'
							binary.BigEndian.PutUint32(tm[1:5], uint32(4+len(tb)))
							copy(tm[5:], tb)
							dm := []byte{'D', 0, 0, 0, 17, 0, 1, 0, 0, 0, 7, 'i', 't', 'e', 'm', 's', '_', '1'}
							_, _ = conn.Write(tm)
							_, _ = conn.Write(dm)
						} else {
							cmdTag := "OK\x00"
							cm := make([]byte, 5+len(cmdTag))
							cm[0] = 'C'
							binary.BigEndian.PutUint32(cm[1:5], uint32(4+len(cmdTag)))
							copy(cm[5:], cmdTag)
							_, _ = conn.Write(cm)
						}
						_, _ = conn.Write(rfq)
					}
				}(c, cid)
			}
		}()

		dir := t.TempDir()
		src := `
bool $ok = __joss_db_is_connected();
bool $cr = Schema::create("items_1", func(Blueprint $t): void {
	$t->id("id");
	$t->string("title", 80);
});
bool $ht = Schema::hasTable("items_1");
bool $hc = Schema::hasColumn("items_1", "title");
bool $dr = Schema::dropIfExists("items_1");
print("PG_DDL:" . strval($ok) . "," . strval($cr) . "," . strval($ht) . "," . strval($hc) . "," . strval($dr));
`
		srcPath := filepath.Join(dir, "pg_ddl.joss")
		_ = os.WriteFile(srcPath, []byte(src), 0644)
		exePath, buildOut, err := wbBuildWithCLI(t, dir, "pg_ddl.joss", "pg_ddl", "llvm")
		if err != nil {
			t.Fatalf("joss build pg_ddl failed: %v\n%s", err, buildOut)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		out, code, runErr := wbRunBinary(t, exePath, dir, []string{
			"DB_CONNECTION=pgsql",
			"DB_HOST=127.0.0.1",
			fmt.Sprintf("DB_PORT=%d", port),
			"DB_USERNAME=pg",
			"DB_PASSWORD=pw",
			"DB_DATABASE=app",
		})
		if runErr != nil || code != 0 {
			t.Fatalf("pg_ddl exited code=%d err=%v\n%s", code, runErr, out)
		}
		if out != "PG_DDL:true,true,true,true,true" {
			t.Errorf("PG wire Schema DDL got %q, want PG_DDL:true,true,true,true,true\nqueries=%v", out, queries)
		}
		qmu.Lock()
		joined := strings.Join(queries, " | ")
		qmu.Unlock()
		if !strings.Contains(joined, "SERIAL PRIMARY KEY") || !strings.Contains(joined, "DROP TABLE IF EXISTS items_1") {
			t.Errorf("unexpected PG SQL dialect output: %s", joined)
		}
	})
}

// ============================================================================
// AREA 7: WebSockets RFC 6455, Cron::schedule, Sitemap, IndexNow, SmtpClient
// ============================================================================

func TestWhiteBoxAudit_07_WebSocketsCronSitemapIndexNowSMTP(t *testing.T) {
	t.Run("websocket_rfc6455_handshake_frames_ping_pong", func(t *testing.T) {
		dir := t.TempDir()
		routesSrc := `
Router::get("/ws-info", "WsCtrl@info")
Router::ws("/ws/chat", "WsCtrl@chat")
`
		mainSrc := `
public class WsCtrl {
	public func info(): string {
		return "WS_OK";
	}
	public func chat(): void {
	}
}

public class Main {
	Init main() {
		Server::start();
	}
}
`
		_ = os.WriteFile(filepath.Join(dir, "routes.joss"), []byte(routesSrc), 0644)
		_ = os.WriteFile(filepath.Join(dir, "main.joss"), []byte(mainSrc), 0644)
		exePath, buildOut, err := wbBuildWithCLI(t, dir, "main.joss", "ws_srv", "llvm")
		if err != nil {
			t.Fatalf("joss build ws_srv failed: %v\n%s", err, buildOut)
		}
		port := allocFreePort(t)
		cmd, logs := startBuiltServer(t, exePath, port, []string{"RATE_LIMIT_REQUESTS=0"})
		t.Cleanup(func() {
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})

		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		infoResp, err := (&http.Client{Timeout: 3 * time.Second}).Get(base + "/ws-info")
		if err != nil {
			t.Fatalf("GET /ws-info failed: %v\nlogs:\n%s", err, logs.String())
		}
		infoBytes, _ := io.ReadAll(infoResp.Body)
		_ = infoResp.Body.Close()
		t.Logf("GET /ws-info status=%d body=%q", infoResp.StatusCode, string(infoBytes))

		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
		if err != nil {
			t.Fatalf("dial ws: %v\n%s", err, logs.String())
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

		key := "dGhlIHNhbXBsZSBub25jZQ=="
		hs := "GET /ws/chat HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: " + key + "\r\nSec-WebSocket-Version: 13\r\n\r\n"
		_, _ = conn.Write([]byte(hs))

		br := bufio.NewReader(conn)
		var respLines []string
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				t.Fatalf("read ws handshake: %v\nlogs:\n%s\npartial lines=%q", err, logs.String(), respLines)
			}
			respLines = append(respLines, line)
			if line == "\r\n" {
				break
			}
		}
		hsResp := strings.Join(respLines, "")
		h := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		wantAccept := base64.StdEncoding.EncodeToString(h[:])
		if !strings.Contains(hsResp, "101 Switching Protocols") || !strings.Contains(hsResp, wantAccept) {
			t.Fatalf("invalid WS handshake:\n%s\nwant accept: %s", hsResp, wantAccept)
		}

		// Send masked Ping frame (opcode 0x9) and expect unmasked Pong frame (opcode 0xA)
		mask := []byte{0x12, 0x34, 0x56, 0x78}
		pingPayload := []byte("ping")
		pingFrame := []byte{0x89, 0x80 | byte(len(pingPayload))}
		pingFrame = append(pingFrame, mask...)
		for i, b := range pingPayload {
			pingFrame = append(pingFrame, b^mask[i&3])
		}
		_, _ = conn.Write(pingFrame)

		pongHdr := make([]byte, 2)
		if _, err := io.ReadFull(br, pongHdr); err != nil {
			t.Fatalf("read pong hdr: %v", err)
		}
		pongLen := int(pongHdr[1] & 0x7F)
		pongBody := make([]byte, pongLen)
		if _, err := io.ReadFull(br, pongBody); err != nil {
			t.Fatalf("read pong body: %v", err)
		}
		if pongHdr[0] != 0x8A || string(pongBody) != "ping" {
			t.Errorf("pong frame mismatch: hdr=%x body=%q", pongHdr, string(pongBody))
		}

		// Send masked text frame (opcode 0x1) and expect server echo/broadcast frame (opcode 0x81)
		txt := []byte("hello-ws-rfc6455")
		txtFrame := []byte{0x81, 0x80 | byte(len(txt))}
		txtFrame = append(txtFrame, mask...)
		for i, b := range txt {
			txtFrame = append(txtFrame, b^mask[i&3])
		}
		_, _ = conn.Write(txtFrame)

		echoHdr := make([]byte, 2)
		if _, err := io.ReadFull(br, echoHdr); err != nil {
			t.Fatalf("read ws echo hdr: %v", err)
		}
		echoLen := int(echoHdr[1] & 0x7F)
		echoBody := make([]byte, echoLen)
		if _, err := io.ReadFull(br, echoBody); err != nil {
			t.Fatalf("read ws echo body: %v", err)
		}
		if echoHdr[0] != 0x81 || string(echoBody) != "hello-ws-rfc6455" {
			t.Errorf("ws echo mismatch: hdr=%x body=%q", echoHdr, string(echoBody))
		}
	})

	t.Run("cron_sitemap_indexnow_smtp_services", func(t *testing.T) {
		var idxBody string
		idxSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			idxBody = string(b)
			w.WriteHeader(200)
		}))
		defer idxSrv.Close()

		smtpLn, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen smtp: %v", err)
		}
		defer smtpLn.Close()
		var smtpTranscript string
		var smtpMu sync.Mutex
		go func() {
			conn, err := smtpLn.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			_, _ = conn.Write([]byte("220 mock.smtp ESMTP\r\n"))
			r := bufio.NewReader(conn)
			var sb strings.Builder
			inData := false
			for {
				line, err := r.ReadString('\n')
				if err != nil {
					break
				}
				sb.WriteString(line)
				if inData {
					if line == ".\r\n" {
						inData = false
						_, _ = conn.Write([]byte("250 2.0.0 OK\r\n"))
					}
					continue
				}
				up := strings.ToUpper(strings.TrimSpace(line))
				switch {
				case strings.HasPrefix(up, "EHLO"):
					_, _ = conn.Write([]byte("250-mock.smtp\r\n250 OK\r\n"))
				case up == "DATA":
					inData = true
					_, _ = conn.Write([]byte("354 Start mail input\r\n"))
				case up == "QUIT":
					_, _ = conn.Write([]byte("221 Bye\r\n"))
					smtpMu.Lock()
					smtpTranscript = sb.String()
					smtpMu.Unlock()
					return
				default:
					_, _ = conn.Write([]byte("250 OK\r\n"))
				}
			}
		}()

		dir := t.TempDir()
		src := `
var $goodJob = Cron::schedule("* * * * *", func(): void {
	print("CRON_TICK");
});
var $badJob = Cron::schedule("invalid_cron_expr", func(): void {
	print("BAD_CRON");
});
print("CRON_RES:" . strval($goodJob != null) . "," . strval($badJob == null));

Sitemap::exclude(["/private*"]);
Sitemap::add("/about", "monthly", "0.7");
Sitemap::provider(func(): array {
	return ["/articles/1", "/private/secret", {"loc": "/pricing", "priority": "1.0"}];
});
string $xml = Sitemap::generate("https://audit.joss");
print("SITEMAP_ABOUT:" . strval(str_contains($xml, "https://audit.joss/about")));
print("SITEMAP_PRIV:" . strval(str_contains($xml, "/private/secret")));

print("IDX_KEY:" . IndexNow::key());
print("IDX_OK:" . strval(IndexNow::submit(["https://audit.joss/about"])));
print("SMTP_OK:" . strval(SmtpClient::send("qa@audit.joss", "WhiteBox Subject", "Body content")));
print("SMTP_EMPTY:" . strval(SmtpClient::send("", "Subj", "Body")));
`
		srcPath := filepath.Join(dir, "services_audit.joss")
		_ = os.WriteFile(srcPath, []byte(src), 0644)
		exePath, buildOut, err := wbBuildWithCLI(t, dir, "services_audit.joss", "services_audit", "llvm")
		if err != nil {
			t.Fatalf("joss build services_audit failed: %v\n%s", err, buildOut)
		}
		smtpPort := smtpLn.Addr().(*net.TCPAddr).Port
		out, code, runErr := wbRunBinary(t, exePath, dir, []string{
			"INDEXNOW_KEY=wb-idx-key",
			"INDEXNOW_ENDPOINT=" + idxSrv.URL,
			"APP_URL=https://audit.joss",
			"MAIL_HOST=127.0.0.1",
			fmt.Sprintf("MAIL_PORT=%d", smtpPort),
			"MAIL_FROM_ADDRESS=sender@audit.joss",
		})
		if runErr != nil || code != 0 {
			t.Fatalf("services_audit exited code=%d err=%v\n%s", code, runErr, out)
		}
		for _, want := range []string{
			"CRON_RES:true,true",
			"SITEMAP_ABOUT:true",
			"SITEMAP_PRIV:false",
			"IDX_KEY:wb-idx-key",
			"IDX_OK:true",
			"SMTP_OK:true",
			"SMTP_EMPTY:false",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in services_audit output:\n%s", want, out)
			}
		}
		if !strings.Contains(idxBody, `"wb-idx-key"`) || !strings.Contains(idxBody, `"https://audit.joss/about"`) {
			t.Errorf("unexpected IndexNow HTTP body: %s", idxBody)
		}
		smtpMu.Lock()
		tr := smtpTranscript
		smtpMu.Unlock()
		if !strings.Contains(tr, "MAIL FROM:<sender@audit.joss>") || !strings.Contains(tr, "RCPT TO:<qa@audit.joss>") || !strings.Contains(tr, "Body content") {
			t.Errorf("unexpected SMTP transcript:\n%s", tr)
		}
		t.Logf("services_audit output:\n%s", out)
	})

	t.Run("cron_rundue_and_sitemap_xsl_signature_gaps", func(t *testing.T) {
		dir := t.TempDir()
		// 1. joss_web_rt.c:3278 implements fn_Cron_runDue(void), yet pkg/core/native.go:195 omits "runDue"
		_ = os.WriteFile(filepath.Join(dir, "cron_gap.joss"), []byte("print(strval(Cron::runDue()));\n"), 0644)
		if _, out, err := wbBuildWithCLI(t, dir, "cron_gap.joss", "cron_gap", "llvm"); err != nil {
			t.Errorf("Cron::runDue() rejected despite C11 fn_Cron_runDue existing: %v\n%s", err, out)
		}
		// 2. pkg/core/native_signatures.go:262 lists Sitemap::xsl, but return-type map omits "Sitemap::xsl": "string"
		_ = os.WriteFile(filepath.Join(dir, "xsl_gap.joss"), []byte("print(strval(str_contains(Sitemap::xsl(), \"xsl:stylesheet\")));\n"), 0644)
		if _, out, err := wbBuildWithCLI(t, dir, "xsl_gap.joss", "xsl_gap", "llvm"); err != nil {
			t.Errorf("Sitemap::xsl() return type inferred as `Sitemap` instead of `string`: %v\n%s", err, out)
		}
	})
}

// ============================================================================
// AREA 8: Carga/despacho de plugins .jp y compilación nativa por defecto de Joss-Red-JosSecurity
// ============================================================================

func TestWhiteBoxAudit_08_PluginsJPAndJosSecurityDefaultBuild(t *testing.T) {
	jossBin := wbBuildJossCLI(t)

	t.Run("jp_plugin_package_build_and_dispatch", func(t *testing.T) {
		root := t.TempDir()
		pkgDir := filepath.Join(root, "plugins", "wb_math")
		if err := os.MkdirAll(filepath.Join(pkgDir, "src"), 0755); err != nil {
			t.Fatalf("mkdir plugin: %v", err)
		}
		manifest := `name: wb_math
version: 1.0.0
type: library
entry: src/plugin.joss
`
		pluginSrc := `
public func wb_mul_add(int $a, int $b, int $c): int {
	return ($a * $b) + $c;
}
`
		_ = os.WriteFile(filepath.Join(pkgDir, "joss.yaml"), []byte(manifest), 0644)
		_ = os.WriteFile(filepath.Join(pkgDir, "src", "plugin.joss"), []byte(pluginSrc), 0644)

		pkgCmd := exec.Command(jossBin, "build", "package", pkgDir)
		pkgCmd.Dir = root
		if out, err := pkgCmd.CombinedOutput(); err != nil {
			t.Fatalf("joss build package failed: %v\n%s", err, string(out))
		}
		jpPath := filepath.Join(pkgDir, "wb_math.jp")
		if _, err := os.Stat(jpPath); err != nil {
			t.Fatalf("missing generated .jp plugin: %v", err)
		}

		hostSrc := `
int $r1 = intval(Plugin::call("wb_math", "wb_mul_add", [6, 7, 5]));
print("JP_RES:" . strval($r1));
`
		_ = os.WriteFile(filepath.Join(root, "main.joss"), []byte(hostSrc), 0644)

		// Compare interpreter (`joss run --anterior`) vs native LLVM (`joss build --native --backend=llvm`)
		interpCmd := exec.Command(jossBin, "run", "--anterior", "main.joss")
		interpCmd.Dir = root
		interpBytes, err := interpCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("joss run --anterior failed: %v\n%s", err, string(interpBytes))
		}
		interpOut := wbNormalizeOutput(string(interpBytes))

		exePath, buildOut, err := wbBuildWithCLI(t, root, "main.joss", "jp_host", "llvm")
		if err != nil {
			t.Fatalf("joss build jp_host --backend=llvm failed: %v\n%s", err, buildOut)
		}
		natOut, code, runErr := wbRunBinary(t, exePath, root, nil)
		if runErr != nil || code != 0 {
			t.Fatalf("jp_host exited code=%d err=%v\n%s", code, runErr, natOut)
		}
		if !strings.Contains(natOut, "JP_RES:47") {
			t.Errorf("expected JP_RES:47 from native plugin dispatch, got:\n%s", natOut)
		}
		if !strings.Contains(interpOut, "JP_RES:47") {
			t.Errorf("expected JP_RES:47 from interpreter plugin dispatch, got:\n%s", interpOut)
		}
		t.Logf("jp_host native=%q interp=%q", natOut, interpOut)
	})

	t.Run("jossecurity_default_and_native_build", func(t *testing.T) {
		repoRoot := wbRepoRoot(t)
		srcDir := filepath.Join(repoRoot, "ejemplos", "Joss-Red-JosSecurity")
		if _, err := os.Stat(filepath.Join(srcDir, "main.joss")); err != nil {
			t.Skipf("external project %s not present in checkout", srcDir)
		}
		tmpDir := t.TempDir()
		wbCopyDir(t, srcDir, tmpDir)

		// 1. Strict native build (--native --backend=llvm)
		outExeStrict := filepath.Join(tmpDir, "build", "jossec_llvm.exe")
		cmdStrict := exec.Command(jossBin, "build", "main.joss", "-o", outExeStrict, "--native", "--backend=llvm")
		cmdStrict.Dir = tmpDir
		strictBytes, strictErr := cmdStrict.CombinedOutput()
		if strictErr != nil {
			t.Errorf("JosSecurity --native --backend=llvm build FAILED (err=%v):\n%s", strictErr, string(strictBytes))
		} else {
			t.Logf("JosSecurity --native --backend=llvm build SUCCEEDED:\n%s", string(strictBytes))
		}

		// 2. Default `joss build main.joss` (auto mode, without --native)
		outExeDefault := filepath.Join(tmpDir, "build", "jossec_default.exe")
		cmdDef := exec.Command(jossBin, "build", "main.joss", "-o", outExeDefault)
		cmdDef.Dir = tmpDir
		defBytes, defErr := cmdDef.CombinedOutput()
		if defErr != nil {
			t.Errorf("JosSecurity default `joss build main.joss` FAILED (err=%v, outputLen=%d):\n%s", defErr, len(defBytes), string(defBytes))
		} else {
			fi, _ := os.Stat(outExeDefault)
			t.Logf("JosSecurity default `joss build main.joss` SUCCEEDED (size=%d bytes):\n%s", fi.Size(), string(defBytes))
		}
	})
}
