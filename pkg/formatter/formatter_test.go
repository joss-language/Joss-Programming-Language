package formatter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestFormatterIdempotence(t *testing.T) {
	sources := []string{
		`public func test(int $a): int {
    return $a + 1;
}`,
		`public class User {
    public string $name = "Alice";
    public func greet(): string {
        return "Hello " . $this->name;
    }
}`,
		`$isValid ? {
    save();
} : {
    reject();
}`,
		`array<int> $items = [1, 2, 3];
map $data = {"key": 5};`,
		`// Leading comment
public func compute(): int {
    // Inner comment
    int $x = 10;
    int $y = 20;
    return $x + $y;
}`,
	}

	for _, src := range sources {
		firstPass, err := FormatSource(src)
		if err != nil {
			t.Fatalf("first pass error: %v", err)
		}
		secondPass, err := FormatSource(firstPass)
		if err != nil {
			t.Fatalf("second pass error: %v", err)
		}
		if firstPass != secondPass {
			t.Fatalf("formatter is not idempotent!\nFirst pass:\n%s\nSecond pass:\n%s", firstPass, secondPass)
		}
	}
}

func TestFormatterNormalizesSpaces(t *testing.T) {
	input := `public   func   add( int $a,int $b ):int{return $a+$b;}`
	expected := `public func add(int $a, int $b): int {
    return $a + $b;
}
`
	got, err := FormatSource(input)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}
	if got != expected {
		t.Fatalf("formatting mismatch:\nGot:\n%q\nWant:\n%q", got, expected)
	}
}

func TestFormatterPreservesComments(t *testing.T) {
	input := `// Global comment
public func run(): void {
    // Step 1
    int $step = 1;
}
`
	got, err := FormatSource(input)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}
	if got != input {
		t.Fatalf("comments corrupted:\nGot:\n%s\nWant:\n%s", got, input)
	}
}

func TestFormatterMatchAndChaining(t *testing.T) {
	input := `public func handle(int $status): string {
    return match ($status) {
        200 => "ok",
        404 => "not found",
        default => "unknown",
    };
}
`
	got, err := FormatSource(input)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}
	if got != input {
		t.Fatalf("match formatting error:\nGot:\n%s\nWant:\n%s", got, input)
	}
}

func TestFormatterBlockTernary(t *testing.T) {
	input := `$active ? {
    activate();
} : {
    deactivate();
}
`
	got, err := FormatSource(input)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}
	if got != input {
		t.Fatalf("block ternary formatting error:\nGot:\n%s\nWant:\n%s", got, input)
	}
}

func TestFormatterWithoutSemicolons(t *testing.T) {
	input := `Router::group("JossRed", func() {
    Router::post("/api/register", "ApiAuthController@register")
    Router::post("/api/login", "ApiAuthController@login")
})
`
	got, err := FormatSource(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != input {
		t.Fatalf("mismatch:\nGot:\n%s\nWant:\n%s", got, input)
	}
}

func TestFormatterGuardAndTernary(t *testing.T) {
	input := `public func procesar(int $monto): string {
    guard ($monto > 0) : {
        return "Monto inválido"
    }
    return "Procesando: " . $monto
}
`
	got, err := FormatSource(input)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}
	if got != input {
		t.Fatalf("guard mismatch:\nGot:\n%s\nWant:\n%s", got, input)
	}
}

func TestFormatterMethodChainingAcrossLines(t *testing.T) {
	input := `public func test(): void {
    $posts = $items->orderBy("id", "DESC")
        ->limit($perPage)
        ->offset($offset)
}
`
	got, err := FormatSource(input)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}
	if got != input {
		t.Fatalf("chaining mismatch:\nGot:\n%s\nWant:\n%s", got, input)
	}
}

func TestFormatterNullSafeArrow(t *testing.T) {
	input := `public func getName(User|null $user): string|null {
    return $user?->profile?->name
}
`
	got, err := FormatSource(input)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}
	if got != input {
		t.Fatalf("null safe arrow mismatch:\nGot:\n%s\nWant:\n%s", got, input)
	}
}

func TestFormatterControllerSnippet(t *testing.T) {
	input := `public class AdminCmsController {
    public func posts() {
        $db = new GranDB()
        $q = Str::trim(Request::input("q") ?? "")
        $type = Str::trim(Request::input("type") ?? "")
        $pVal = Request::input("page")
        $page = $pVal ? JSON::parse($pVal) : 1
        ($page < 1) ? {
            $page = 1
        }
        $perPage = 8
        $offset = ($page - 1) * $perPage
    }
}
`
	got, err := FormatSource(input)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}
	if got != input {
		t.Fatalf("controller snippet mismatch:\nGot:\n%s\nWant:\n%s", got, input)
	}
}

func TestFormatterRealProjectFiles(t *testing.T) {
	root := "../../ejemplos/Joss-Red-JosSecurity"
	if _, err := os.Stat(root); err != nil {
		t.Skip("ejemplos directory not present")
		return
	}

	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if parser.IsIgnoredDirectory(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if parser.IsJossSourceFile(path) {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			formatted, err := FormatSource(string(data))
			if err != nil {
				t.Fatalf("error formatting %s: %v", path, err)
			}
			// 1. Must parse with 0 errors
			pParser := parser.NewParser(parser.NewLexer(formatted))
			_ = pParser.ParseProgram()
			if len(pParser.Errors()) > 0 {
				t.Fatalf("syntax error in formatted %s: %v", path, pParser.Errors())
			}

			// 2. Must be idempotent
			secondPass, err := FormatSource(formatted)
			if err != nil {
				t.Fatalf("error on second pass %s: %v", path, err)
			}
			if formatted != secondPass {
				t.Fatalf("formatter not idempotent on %s", path)
			}
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
	t.Logf("Successfully formatted and verified %d files from Joss-Red-JosSecurity", count)
}
