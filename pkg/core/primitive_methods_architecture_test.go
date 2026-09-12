package core

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

func TestPrimitiveMethodCatalogHasRuntimeImplementations(t *testing.T) {
	runtime := NewRuntime()
	tests := []struct {
		kind    typesystem.Kind
		resolve func(string) bool
	}{
		{
			kind: typesystem.String,
			resolve: func(name string) bool {
				_, ok := runtime.resolveStringMethod("sample", &parser.Identifier{Value: name})
				return ok
			},
		},
		{
			kind: typesystem.Array,
			resolve: func(name string) bool {
				_, ok := runtime.resolveArrayMethod([]interface{}{}, &parser.Identifier{Value: name})
				return ok
			},
		},
		{
			kind: typesystem.Map,
			resolve: func(name string) bool {
				_, ok := runtime.resolveMapMethod(map[string]interface{}{}, &parser.Identifier{Value: name})
				return ok
			},
		},
	}

	for _, test := range tests {
		for _, definition := range typesystem.PrimitiveMethodDefinitions(test.kind) {
			if !test.resolve(definition.Name) {
				t.Errorf("canonical %s method %q has no runtime implementation", test.kind, definition.Name)
			}
		}
	}
}
