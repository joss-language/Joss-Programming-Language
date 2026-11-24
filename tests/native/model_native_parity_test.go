package native_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	_ "github.com/jossecurity/joss/pkg/backend/llvm"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	core "github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

func TestNativeParity_Phase4_ModelORM(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang not available on PATH")
	}
	if runtime.GOOS == "windows" {
		t.Skip("SQLite3 C development library not preinstalled on Windows CI runner")
	}

	source := `
public class Post extends Model {
	protected string $table = "posts"
	protected bool $timestamps = false
	protected array $fillable = ["user_id", "title"]
}

public class User extends Model {
	protected string $table = "users"
	protected bool $timestamps = false
	protected bool $softDeletes = true
	protected array $fillable = ["name", "email", "active"]
	protected array $hidden = ["email"]
	protected map $casts = {"active": "bool"}

	public func posts(): mixed {
		return $this->hasMany("Post", "user_id", "id")
	}

	public func scopeNamed(mixed $q, string $n): mixed {
		return $q->where("name", $n)
	}

	public func creating(): bool {
		return $this->name != "blocked"
	}
}

__joss_db_execute("CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, email TEXT, active INTEGER, deleted_at TEXT)", []);
__joss_db_execute("CREATE TABLE posts (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, title TEXT)", []);

// 1. Create + Casts + Dirty tracking + Partial Update
var $u1 = User::create({"name": "Ada", "email": "ada@joss.dev", "active": true});
print("C1:" . strval($u1->id) . "|" . strval($u1->name) . "|" . strval($u1->active) . "|" . strval($u1->isClean()));

$u1->name = "Ada Lovelace";
print("DIRTY:" . strval($u1->isDirty("name")) . "|" . strval($u1->isDirty("email")));
$u1->save();
print("SAVED:" . strval($u1->isClean()) . "|" . strval($u1->wasChanged("name")));

// 2. Hook cancel-on-false
var $blocked = new User();
$blocked->fill({"name": "blocked", "email": "b@joss.dev", "active": false});
bool $bOk = $blocked->save();
print("BLOCKED:" . strval($bOk) . "|CNT:" . strval(User::count()));

// 3. firstOrCreate & updateOrCreate
var $u2 = User::firstOrCreate({"email": "grace@joss.dev"}, {"name": "Grace", "active": true});
var $u2Again = User::firstOrCreate({"email": "grace@joss.dev"}, {"name": "Ignored", "active": false});
print("FOC:" . strval($u2->id == $u2Again->id) . "|" . strval($u2Again->name));

var $u2Upd = User::updateOrCreate({"email": "grace@joss.dev"}, {"name": "Grace Hopper"});
print("UOC:" . strval($u2Upd->name));

// 4. Local scope + Find
var $scoped = User::scope("named", "Ada Lovelace")->first();
print("SCOPE:" . strval($scoped->name));

// 5. Relations + Eager Loading (with)
Post::create({"user_id": $u1->id, "title": "First Note"});
Post::create({"user_id": $u1->id, "title": "Second Note"});
Post::create({"user_id": $u2->id, "title": "Compiler Note"});

var $loaded = User::with("posts")->orderBy("id", "asc")->get();
print("EAGER_USERS:" . strval(count($loaded)));
var $firstU = $loaded[0];
var $firstPosts = $firstU->posts;
print("EAGER_POSTS:" . strval(count($firstPosts)));

// 6. Soft Deletes + withTrashed + onlyTrashed + restore + forceDelete
$u2Upd->delete();
print("SOFT_CNT:" . strval(User::count()) . "|WITH:" . strval(User::withTrashed()->count()) . "|ONLY:" . strval(User::onlyTrashed()->count()));
$u2Upd->restore();
print("RESTORED_CNT:" . strval(User::count()));
$u2Upd->forceDelete();
print("FORCED_CNT:" . strval(User::withTrashed()->count()));

// 7. Serialization (hidden email excluded)
var $m = $u1->toMap();
print("HIDDEN_OMITTED:" . strval(!array_key_exists("email", $m)) . "|HAS_NAME:" . strval(array_key_exists("name", $m)));
`

	// 1. Compile and execute Native LLVM binary against real SQLite3
	tmpDir := t.TempDir()
	dbFileNative := filepath.Join(tmpDir, "model_native.sqlite")

	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: "model_parity.joss", Program: prog}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	irProg, err := ir.NewLowerer(prep, nil).LowerProgram("model_parity")
	if err != nil {
		t.Fatalf("IR lowering failed: %v", err)
	}
	if err := ir.NewVerifier().Verify(irProg); err != nil {
		t.Fatalf("IR verification failed: %v", err)
	}

	outExe := filepath.Join(tmpDir, "model_parity_bin")
	res, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    backend.BackendLLVM,
		OutputPath: outExe,
	})
	if err != nil {
		t.Fatalf("BuildProgram(LLVM) failed: %v", err)
	}

	cmd := exec.Command(outExe)
	cmd.Env = append(os.Environ(), "DB_CONNECTION=sqlite", "DB_DATABASE="+dbFileNative)
	nativeRaw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native binary failed: %v\noutput:\n%s", err, string(nativeRaw))
	}
	nativeOut := strings.TrimSpace(string(nativeRaw))

	expected := strings.Join([]string{
		"C1:1|Ada|true|true",
		"DIRTY:true|false",
		"SAVED:true|true",
		"BLOCKED:false|CNT:1",
		"FOC:true|Grace",
		"UOC:Grace Hopper",
		"SCOPE:Ada Lovelace",
		"EAGER_USERS:2",
		"EAGER_POSTS:2",
		"SOFT_CNT:1|WITH:2|ONLY:1",
		"RESTORED_CNT:2",
		"FORCED_CNT:1",
		"HIDDEN_OMITTED:true|HAS_NAME:true",
	}, "\n")

	if nativeOut != expected {
		t.Fatalf("Model ORM native output mismatch:\n--- GOT ---\n%s\n--- WANT ---\n%s", nativeOut, expected)
	}
	t.Logf("[Model ORM Native SQLite3] PASS (binary=%d bytes)\n%s", res.ExecutableSize, nativeOut)
}
