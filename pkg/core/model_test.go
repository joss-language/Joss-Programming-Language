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

func TestBelongsToManyAttachDetachSyncEagerAndRollback(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = database.Exec(`
CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);
CREATE TABLE roles (id INTEGER PRIMARY KEY, name TEXT);
CREATE TABLE role_user (user_id INTEGER, role_id INTEGER, assigned_by TEXT, UNIQUE(user_id, role_id));
INSERT INTO users VALUES (1,'Ada');
INSERT INTO roles VALUES (1,'admin'),(2,'editor'),(3,'blocked');`)
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.DB = database
	runtime.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	source := `
public class User extends Model {
    protected string $table = "users"
    public func roles(): mixed { return $this->belongsToMany("Role", "role_user", "user_id", "role_id", "id", "id") }
}
public class Role extends Model { protected string $table = "roles" }
`
	runtime.Execute(parser.NewParser(parser.NewLexer(source)).ParseProgram())
	user := runtime.executeFindMethod(runtime.newModelQuery(runtime.lookupModelMetadata("User")), []interface{}{int64(1)}).(*Instance)
	relation := runtime.CallMethodEvaluated(runtime.lookupClassMetadata("User").Methods["roles"].Method, user, nil).(*Instance)
	if attached := runtime.executeModelMethod(relation, "attach", []interface{}{int64(1), map[string]interface{}{"assigned_by": "system"}}); attached != int64(1) {
		t.Fatalf("attach=%v", attached)
	}
	if duplicate := runtime.executeModelMethod(relation, "attach", []interface{}{int64(1)}); duplicate != int64(0) {
		t.Fatalf("duplicate attach=%v", duplicate)
	}
	items := runtime.executeModelMethod(relation, "get", nil).([]interface{})
	if len(items) != 1 || items[0].(*Instance).Fields["pivot"].(map[string]interface{})["assigned_by"] != "system" {
		t.Fatalf("pivot items=%#v", items)
	}
	if detached := runtime.executeModelMethod(relation, "detach", []interface{}{[]interface{}{}}); detached != int64(0) {
		t.Fatalf("empty detach=%v", detached)
	}
	result := runtime.executeModelMethod(relation, "sync", []interface{}{[]interface{}{int64(1), int64(2)}}).(map[string]interface{})
	if result["attached"] != int64(1) || result["detached"] != int64(0) {
		t.Fatalf("sync=%#v", result)
	}
	result = runtime.executeModelMethod(relation, "sync", []interface{}{map[string]interface{}{"1": map[string]interface{}{"assigned_by": "owner"}, "2": map[string]interface{}{"assigned_by": "owner"}}}).(map[string]interface{})
	if result["updated"] != int64(2) {
		t.Fatalf("sync pivot update=%#v", result)
	}
	var assigned string
	if err := database.QueryRow(`SELECT assigned_by FROM role_user WHERE user_id=1 AND role_id=2`).Scan(&assigned); err != nil || assigned != "owner" {
		t.Fatalf("pivot update assigned=%q err=%v", assigned, err)
	}
	query := runtime.newModelQuery(runtime.lookupModelMetadata("User"))
	query.Fields["_with"] = []string{"roles"}
	loaded := runtime.executeGetMethod(query, nil).([]interface{})[0].(*Instance)
	if len(loaded.Fields["roles"].([]interface{})) != 2 {
		t.Fatalf("eager roles=%#v", loaded.Fields["roles"])
	}
	if _, err := database.Exec(`CREATE TRIGGER reject_blocked BEFORE INSERT ON role_user WHEN NEW.role_id = 3 BEGIN SELECT RAISE(ABORT,'blocked'); END;`); err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("sync should fail and roll back")
			}
		}()
		runtime.executeModelMethod(relation, "sync", []interface{}{[]interface{}{int64(3)}})
	}()
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM role_user WHERE user_id=1 AND role_id IN (1,2)`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("rollback count=%d err=%v", count, err)
	}
	if detached := runtime.executeModelMethod(relation, "detach", nil); detached != int64(2) {
		t.Fatalf("detach all=%v", detached)
	}
}

func TestModelSoftDeletesAndExplicitLocalScope(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, deleted_at TEXT); INSERT INTO users VALUES (1,'Ada',NULL),(2,'Grace','2026-01-01 00:00:00')`); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.DB = database
	runtime.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	source := `
public class User extends Model {
    protected string $table = "users"
    protected bool $softDeletes = true
    public func scopeNamed(mixed $query, string $name): mixed { return $query->where("name", $name) }
}`
	runtime.Execute(parser.NewParser(parser.NewLexer(source)).ParseProgram())
	metadata := runtime.lookupModelMetadata("User")
	if rows := runtime.executeGetMethod(runtime.newModelQuery(metadata), nil).([]interface{}); len(rows) != 1 {
		t.Fatalf("default soft-delete scope returned %d", len(rows))
	}
	reused := runtime.newModelQuery(metadata)
	_ = runtime.executeGetMethod(reused, nil)
	if rows := runtime.executeGetMethod(reused, nil).([]interface{}); len(rows) != 1 {
		t.Fatalf("reused query lost soft-delete scope: %d", len(rows))
	}
	allQuery := runtime.executeModelMethod(&Instance{Class: metadata.Class, Fields: map[string]interface{}{}}, "withTrashed", nil).(*Instance)
	if rows := runtime.executeGetMethod(allQuery, nil).([]interface{}); len(rows) != 2 {
		t.Fatalf("withTrashed returned %d", len(rows))
	}
	onlyQuery := runtime.executeModelMethod(&Instance{Class: metadata.Class, Fields: map[string]interface{}{}}, "onlyTrashed", nil).(*Instance)
	if rows := runtime.executeGetMethod(onlyQuery, nil).([]interface{}); len(rows) != 1 || rows[0].(*Instance).Fields["name"] != "Grace" {
		t.Fatalf("onlyTrashed=%#v", rows)
	}
	scopeQuery := runtime.executeModelMethod(&Instance{Class: metadata.Class, Fields: map[string]interface{}{}}, "scope", []interface{}{"named", "Ada"}).(*Instance)
	user := runtime.executeFirstMethod(scopeQuery, nil).(*Instance)
	if !runtime.deleteModel(user) || !user.model.deleted {
		t.Fatal("soft delete failed")
	}
	if rows := runtime.executeGetMethod(runtime.newModelQuery(metadata), nil).([]interface{}); len(rows) != 0 {
		t.Fatalf("deleted model remained visible: %d", len(rows))
	}
	if !runtime.restoreModel(user) || user.model.deleted {
		t.Fatal("restore failed")
	}
	if !runtime.deleteModel(user) || !runtime.forceDeleteModel(user) {
		t.Fatal("forceDelete failed")
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM users WHERE id=1`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("forceDelete count=%d err=%v", count, err)
	}
}

func TestModelEventsRunInOrderAndCanCancelWrite(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, saving_seen INTEGER)`); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.DB = database
	runtime.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	source := `
public class User extends Model {
    protected string $table = "users"
    protected array $fillable = ["name"]
    public func saving(): bool { $this->saving_seen = true return true }
    public func creating(): bool { return $this->name != "blocked" }
    public func created(): bool { $this->created_seen = true return true }
}`
	runtime.Execute(parser.NewParser(parser.NewLexer(source)).ParseProgram())
	metadata := runtime.lookupModelMetadata("User")
	created := &Instance{Class: metadata.Class, Fields: map[string]interface{}{"name": "Ada"}, Constants: map[string]bool{}, model: newModelState(metadata, false, false)}
	if !runtime.saveModel(created) || created.Fields["saving_seen"] != true || created.Fields["created_seen"] != true {
		t.Fatalf("events=%#v", created.Fields)
	}
	blocked := &Instance{Class: metadata.Class, Fields: map[string]interface{}{"name": "blocked"}, Constants: map[string]bool{}, model: newModelState(metadata, false, false)}
	if runtime.saveModel(blocked) {
		t.Fatal("creating=false did not cancel insert")
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("event cancellation count=%d err=%v", count, err)
	}
}

func TestModelFirstOrCreateAndUpdateOrCreate(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT UNIQUE, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.DB = database
	runtime.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	runtime.Execute(parser.NewParser(parser.NewLexer(`public class User extends Model { protected string $table = "users" protected array $fillable = ["email","name"] }`)).ParseProgram())
	metadata := runtime.lookupModelMetadata("User")
	created := runtime.firstOrWriteModel(metadata, "firstorcreate", []interface{}{map[string]interface{}{"email": "ada@example.test"}, map[string]interface{}{"name": "Ada"}})
	if !created.model.exists {
		t.Fatal("firstOrCreate returned new state")
	}
	same := runtime.firstOrWriteModel(metadata, "firstorcreate", []interface{}{map[string]interface{}{"email": "ada@example.test"}, map[string]interface{}{"name": "Ignored"}})
	if same.Fields["name"] != "Ada" {
		t.Fatalf("firstOrCreate updated existing row: %#v", same.Fields)
	}
	updated := runtime.firstOrWriteModel(metadata, "updateorcreate", []interface{}{map[string]interface{}{"email": "ada@example.test"}, map[string]interface{}{"name": "Grace"}})
	if updated.Fields["name"] != "Grace" {
		t.Fatalf("updateOrCreate=%#v", updated.Fields)
	}
	pending := runtime.firstOrWriteModel(metadata, "firstornew", []interface{}{map[string]interface{}{"email": "new@example.test"}, map[string]interface{}{"name": "New"}})
	if pending.model.exists {
		t.Fatal("firstOrNew persisted model")
	}
}

func TestModelGlobalScopesAreDeferredAndRemovable(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, active INTEGER); INSERT INTO users VALUES (1,'Ada',1),(2,'Grace',0)`); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.DB = database
	runtime.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	source := `
public class User extends Model {
    protected string $table = "users"
    protected array $globalScopes = ["active"]
    public func scopeActive(mixed $query): mixed { return $query->where("active", 1) }
}`
	runtime.Execute(parser.NewParser(parser.NewLexer(source)).ParseProgram())
	metadata := runtime.lookupModelMetadata("User")
	if rows := runtime.executeGetMethod(runtime.newModelQuery(metadata), nil).([]interface{}); len(rows) != 1 || rows[0].(*Instance).Fields["name"] != "Ada" {
		t.Fatalf("global scope rows=%#v", rows)
	}
	reused := runtime.newModelQuery(metadata)
	if first := runtime.executeGetMethod(reused, nil).([]interface{}); len(first) != 1 {
		t.Fatalf("first execution rows=%d", len(first))
	}
	if second := runtime.executeGetMethod(reused, nil).([]interface{}); len(second) != 1 {
		t.Fatalf("reused query lost global scope: rows=%d", len(second))
	}
	query := runtime.executeModelMethod(&Instance{Class: metadata.Class, Fields: map[string]interface{}{}}, "withoutGlobalScope", []interface{}{"active"}).(*Instance)
	if rows := runtime.executeGetMethod(query, nil).([]interface{}); len(rows) != 2 {
		t.Fatalf("withoutGlobalScope rows=%d", len(rows))
	}
	query = runtime.executeModelMethod(&Instance{Class: metadata.Class, Fields: map[string]interface{}{}}, "withoutGlobalScopes", nil).(*Instance)
	if rows := runtime.executeGetMethod(query, nil).([]interface{}); len(rows) != 2 {
		t.Fatalf("withoutGlobalScopes rows=%d", len(rows))
	}
}

func TestModelEagerLoadingAvoidsNPlusOneQueries(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(`
CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);
CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER, title TEXT);
INSERT INTO users VALUES (1,'Ada'),(2,'Grace'),(3,'Linus');
INSERT INTO posts VALUES (1,1,'One'),(2,1,'Two'),(3,2,'Three');`); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.DB = database
	runtime.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	runtime.Execute(parser.NewParser(parser.NewLexer(`
public class User extends Model {
    protected string $table = "users"
    public func posts(): mixed { return $this->hasMany("Post", "user_id", "id") }
}
public class Post extends Model { protected string $table = "posts" }
`)).ParseProgram())
	userMetadata := runtime.lookupModelMetadata("User")
	postsMethod := runtime.lookupClassMetadata("User").Methods["posts"].Method
	runtime.startQueryCounting()
	users := runtime.executeGetMethod(runtime.newModelQuery(userMetadata), nil).([]interface{})
	for _, value := range users {
		relation := runtime.CallMethodEvaluated(postsMethod, value.(*Instance), nil).(*Instance)
		_ = runtime.executeGetMethod(relation, nil)
	}
	if count := runtime.stopQueryCounting(); count != 4 {
		t.Fatalf("lazy loading queries=%d, want 4", count)
	}
	runtime.startQueryCounting()
	query := runtime.newModelQuery(userMetadata)
	query.Fields["_with"] = []string{"posts"}
	users = runtime.executeGetMethod(query, nil).([]interface{})
	if count := runtime.stopQueryCounting(); count != 2 || len(users) != 3 {
		t.Fatalf("eager loading queries=%d users=%d, want 2 and 3", count, len(users))
	}
}

func TestModelAccessorsMutatorsAndSerializationOrder(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	source := `
public class User extends Model {
    protected bool $timestamps = false
    public func setNameAttribute(string $value): string { return $value->lower() }
    public func getNameAttribute(mixed $value): string { return "Hello " . $value }
}
$user = new User()
$user->name = "ADA"
$display = $user->name
`
	runtime.Execute(parser.NewParser(parser.NewLexer(source)).ParseProgram())
	user := runtime.Variables["user"].(*Instance)
	if user.Fields["name"] != "ada" || runtime.Variables["display"] != "Hello ada" {
		t.Fatalf("attribute order raw=%#v display=%#v", user.Fields["name"], runtime.Variables["display"])
	}
	serialized := runtime.serializeModel(user)
	if serialized["name"] != "Hello ada" {
		t.Fatalf("serialized accessor=%#v", serialized)
	}
}

func containsJSONKey(encoded []byte, key string) bool {
	var value map[string]interface{}
	_ = json.Unmarshal(encoded, &value)
	_, ok := value[key]
	return ok
}
