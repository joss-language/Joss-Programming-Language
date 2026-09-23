package core

import (
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
