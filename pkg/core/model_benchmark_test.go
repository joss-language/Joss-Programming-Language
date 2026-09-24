package core

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func benchmarkModelMetadata() *modelMetadata {
	class := &parser.ClassStatement{Name: &parser.Identifier{Value: "BenchUser"}}
	return &modelMetadata{Class: class, ClassName: "BenchUser", Table: "users", PrimaryKey: "id", Incrementing: true, Fillable: map[string]struct{}{}, Guarded: map[string]struct{}{}, Hidden: map[string]struct{}{}, Visible: map[string]struct{}{}, Casts: map[string]string{"active": "bool", "profile": "json"}}
}

func BenchmarkModelHydration(b *testing.B) {
	for _, count := range []int{1, 100, 1000} {
		b.Run(fmt.Sprintf("rows_%d", count), func(b *testing.B) {
			runtime := &Runtime{}
			metadata := benchmarkModelMetadata()
			rows := make([]map[string]interface{}, count)
			for index := range rows {
				rows[index] = map[string]interface{}{"id": int64(index + 1), "name": "Ada", "active": int64(1), "profile": `{"role":"admin"}`}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				_ = runtime.hydrateModelRows(metadata, rows)
			}
		})
	}
}

func BenchmarkModelRelationLoading(b *testing.B) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(`
CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);
CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER, title TEXT);
INSERT INTO users VALUES (1,'A'),(2,'B'),(3,'C'),(4,'D'),(5,'E');
INSERT INTO posts VALUES (1,1,'P1'),(2,2,'P2'),(3,3,'P3'),(4,4,'P4'),(5,5,'P5');`); err != nil {
		b.Fatal(err)
	}
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.DB = database
	runtime.Env = map[string]string{"DB": "sqlite", "PREFIX": ""}
	runtime.Execute(parser.NewParser(parser.NewLexer(`
public class BenchUser extends Model {
    protected string $table = "users"
    public func posts(): mixed { return $this->hasMany("BenchPost", "user_id", "id") }
}
public class BenchPost extends Model { protected string $table = "posts" }
`)).ParseProgram())
	metadata := runtime.lookupModelMetadata("BenchUser")
	postsMethod := runtime.lookupClassMetadata("BenchUser").Methods["posts"].Method

	b.Run("n_plus_one", func(b *testing.B) {
		runtime.startQueryCounting()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			users := runtime.executeGetMethod(runtime.newModelQuery(metadata), nil).([]interface{})
			for _, value := range users {
				relation := runtime.CallMethodEvaluated(postsMethod, value.(*Instance), nil).(*Instance)
				_ = runtime.executeGetMethod(relation, nil)
			}
		}
		b.StopTimer()
		b.ReportMetric(float64(runtime.stopQueryCounting())/float64(b.N), "queries/op")
	})
	b.Run("eager", func(b *testing.B) {
		runtime.startQueryCounting()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			query := runtime.newModelQuery(metadata)
			query.Fields["_with"] = []string{"posts"}
			_ = runtime.executeGetMethod(query, nil)
		}
		b.StopTimer()
		b.ReportMetric(float64(runtime.stopQueryCounting())/float64(b.N), "queries/op")
	})
}
