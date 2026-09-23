package core

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func registerTestModel(t *testing.T, runtime *Runtime) *modelMetadata {
	t.Helper()
	source := `
public class User extends Model {
    protected string $table = "users"
    protected string $primaryKey = "uuid"
    protected string $keyType = "string"
    protected bool $incrementing = false
    protected bool $timestamps = false
    protected array $fillable = ["uuid", "name", "active", "profile"]
    protected array $guarded = ["password"]
    protected array $hidden = ["password"]
    protected map $casts = {"active": "bool", "profile": "json"}
}
`
	parsed := parser.NewParser(parser.NewLexer(source)).ParseProgram()
	runtime.Execute(parsed)
	metadata := runtime.lookupModelMetadata("User")
	if metadata == nil {
		t.Fatal("expected User to be recognized as a Model subclass")
	}
	return metadata
}

func TestModelMetadataIsCachedAndUsesExplicitConfiguration(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	metadata := registerTestModel(t, runtime)
	if metadata != runtime.lookupModelMetadata("User") {
		t.Fatal("model metadata was rebuilt instead of using class cache")
	}
	if metadata.Table != "users" || metadata.PrimaryKey != "uuid" || metadata.KeyType != "string" || metadata.Incrementing || metadata.Timestamps {
		t.Fatalf("unexpected model metadata: %#v", metadata)
	}
	if metadata.Casts["profile"] != "json" {
		t.Fatalf("casts = %#v", metadata.Casts)
	}
}

func TestModelHydrationPreservesNullAndAppliesCasts(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	metadata := registerTestModel(t, runtime)
	model := runtime.hydrateModel(metadata, map[string]interface{}{
		"uuid": "u-1", "active": int64(1), "profile": `{"role":"admin"}`, "name": nil,
	})
	if model.Fields["active"] != true || model.Fields["name"] != nil {
		t.Fatalf("cast/null result = %#v", model.Fields)
	}
	profile, ok := model.Fields["profile"].(map[string]interface{})
	if !ok || profile["role"] != "admin" {
		t.Fatalf("JSON profile = %#v", model.Fields["profile"])
	}
	if runtime.executeModelMethod(model, "isClean", nil) != true {
		t.Fatal("fresh hydrated model must be clean")
	}
	model.Fields["name"] = "Ada"
	if runtime.executeModelMethod(model, "isDirty", []interface{}{"name"}) != true {
		t.Fatal("changed hydrated attribute must be dirty")
	}
}

func TestModelMassAssignmentAndSerialization(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	metadata := registerTestModel(t, runtime)
	model := &Instance{Class: metadata.Class, Fields: map[string]interface{}{}, Constants: map[string]bool{}, model: newModelState(metadata, false, false)}
	runtime.fillModel(model, map[string]interface{}{"uuid": "u-1", "name": "Ada", "active": true}, false)

	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Fatal("guarded mass assignment was accepted")
			}
		}()
		runtime.fillModel(model, map[string]interface{}{"password": "secret"}, false)
	}()
	runtime.fillModel(model, map[string]interface{}{"password": "secret"}, true)
	encoded, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || containsJSONKey(encoded, "password") {
		t.Fatalf("hidden password leaked: %s", encoded)
	}
}

func TestModelSaveUsesInsertThenDirtyPartialUpdate(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = database.Exec(`
CREATE TABLE users (uuid TEXT PRIMARY KEY, name TEXT, active INTEGER, profile TEXT, password TEXT);
CREATE TABLE update_audit (count INTEGER NOT NULL);
INSERT INTO update_audit VALUES (0);
CREATE TRIGGER users_updated AFTER UPDATE ON users BEGIN UPDATE update_audit SET count = count + 1; END;
`)
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.DB = database
	runtime.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	metadata := registerTestModel(t, runtime)
	model := &Instance{Class: metadata.Class, Fields: map[string]interface{}{}, Constants: map[string]bool{}, model: newModelState(metadata, false, false)}
	runtime.fillModel(model, map[string]interface{}{"uuid": "u-1", "name": "Ada", "active": true, "profile": map[string]interface{}{"role": "admin"}}, false)
	if !runtime.saveModel(model) || !model.model.exists || len(modelChanges(model)) != 0 {
		t.Fatalf("model was not synchronized after insert: %#v", model.model)
	}
	if !runtime.saveModel(model) {
		t.Fatal("clean save failed")
	}
	var updates int
	if err := database.QueryRow(`SELECT count FROM update_audit`).Scan(&updates); err != nil || updates != 0 {
		t.Fatalf("clean save executed UPDATE: count=%d err=%v", updates, err)
	}
	model.Fields["name"] = "Grace"
	if !runtime.saveModel(model) {
		t.Fatal("dirty save failed")
	}
	if err := database.QueryRow(`SELECT count FROM update_audit`).Scan(&updates); err != nil || updates != 1 {
		t.Fatalf("dirty save did not execute one UPDATE: count=%d err=%v", updates, err)
	}
	var name, profile string
	if err := database.QueryRow(`SELECT name, profile FROM users WHERE uuid = 'u-1'`).Scan(&name, &profile); err != nil || name != "Grace" || profile != `{"role":"admin"}` {
		t.Fatalf("persisted row name=%q profile=%q err=%v", name, profile, err)
	}
}

func TestModelQueryHydratesConcreteInstancesAndCustomPrimaryKey(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(`CREATE TABLE users (uuid TEXT PRIMARY KEY, name TEXT, active INTEGER, profile TEXT); INSERT INTO users VALUES ('u-1','Ada',1,'{}')`); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.DB = database
	runtime.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	metadata := registerTestModel(t, runtime)
	query := runtime.newModelQuery(metadata)
	found := runtime.executeFindMethod(query, []interface{}{"u-1"})
	model, ok := found.(*Instance)
	if !ok || model.Class.Name.Value != "User" || model.Fields["active"] != true || !model.model.exists {
		t.Fatalf("find returned %#v", found)
	}
	query = runtime.newModelQuery(metadata)
	rows := runtime.executeGetMethod(query, nil).([]interface{})
	if len(rows) != 1 {
		t.Fatalf("get returned %d models", len(rows))
	}
}

func TestModelHasManyAndEagerLoadingMatchByIndexedKeys(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(`
CREATE TABLE users (uuid TEXT PRIMARY KEY, name TEXT);
CREATE TABLE posts (id INTEGER PRIMARY KEY, user_uuid TEXT, title TEXT);
INSERT INTO users VALUES ('u-1','Ada'), ('u-2','Grace');
INSERT INTO posts VALUES (1,'u-1','One'), (2,'u-1','Two'), (3,'u-2','Three');
`); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.DB = database
	runtime.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	source := `
public class User extends Model {
    protected string $table = "users"
    protected string $primaryKey = "uuid"
    protected bool $incrementing = false
    public func posts(): mixed { return $this->hasMany("Post", "user_uuid", "uuid") }
}
public class Post extends Model {
    protected string $table = "posts"
    public func user(): mixed { return $this->belongsTo("User", "user_uuid", "uuid") }
}
`
	runtime.Execute(parser.NewParser(parser.NewLexer(source)).ParseProgram())
	query := runtime.newModelQuery(runtime.lookupModelMetadata("User"))
	query.Fields["_with"] = []string{"posts"}
	users := runtime.executeGetMethod(query, nil).([]interface{})
	if len(users) != 2 {
		t.Fatalf("users=%d", len(users))
	}
	first := users[0].(*Instance)
	posts, ok := first.Fields["posts"].([]interface{})
	if !ok || len(posts) != 2 || !first.model.loaded["posts"] {
		t.Fatalf("eager posts = %#v", first.Fields["posts"])
	}
	post := posts[0].(*Instance)
	relation := runtime.CallMethodEvaluated(runtime.lookupClassMetadata("Post").Methods["user"].Method, post, nil).(*Instance)
	owner := runtime.executeFirstMethod(relation, nil).(*Instance)
	if owner.Fields["uuid"] != "u-1" {
		t.Fatalf("belongsTo owner = %#v", owner.Fields)
	}
}

func TestModelPublicAPIWorksThroughJossStaticAndInstanceDispatch(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(`CREATE TABLE users (uuid TEXT PRIMARY KEY, name TEXT); INSERT INTO users VALUES ('u-1','Ada')`); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.DB = database
	runtime.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	source := `
public class User extends Model {
    protected string $table = "users"
    protected string $primaryKey = "uuid"
    protected bool $incrementing = false
}
$user = User::find("u-1")
$clean = $user->isClean()
$name = $user->name
`
	program := parser.NewParser(parser.NewLexer(source)).ParseProgram()
	runtime.Execute(program)
	if runtime.Variables["clean"] != true || runtime.Variables["name"] != "Ada" {
		t.Fatalf("Joss Model dispatch failed: clean=%#v name=%#v", runtime.Variables["clean"], runtime.Variables["name"])
	}
}

func containsJSONKey(encoded []byte, key string) bool {
	var value map[string]interface{}
	_ = json.Unmarshal(encoded, &value)
	_, ok := value[key]
	return ok
}
