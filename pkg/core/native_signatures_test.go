package core

import (
	"testing"

	"github.com/jossecurity/joss/pkg/typesystem"
)

func TestCoreCallableReturnSignaturesAreExplicit(t *testing.T) {
	environment := buildAnalysisEnvironment()
	for name := range builtinNamesMap {
		if environment.Builtins[name].ReturnType.Kind == typesystem.Unknown {
			t.Fatalf("builtin %s has unknown return type", name)
		}
	}
	runtime := NewRuntime()
	defer runtime.Free()
	for className, classNode := range runtime.Classes {
		if !IsNativeClass(className) || classNode == nil || classNode.Body == nil {
			continue
		}
		for methodName, callable := range environment.Classes[className].Methods {
			if callable.ReturnType.Kind == typesystem.Unknown {
				t.Fatalf("native %s::%s has unknown return type", className, methodName)
			}
		}
	}
}

func TestBuiltinCatalogHasUniqueDefinitionsAndReachableHandlers(t *testing.T) {
	seen := make(map[string]bool, len(builtinDefinitions))
	runtime := NewRuntime()
	defer runtime.Free()

	for _, definition := range builtinDefinitions {
		if seen[definition.name] {
			t.Fatalf("builtin %s has more than one canonical definition", definition.name)
		}
		seen[definition.name] = true
		if definition.returnType.Kind == typesystem.Unknown || definition.returnType.Kind == "" {
			t.Fatalf("builtin %s has no declared return type", definition.name)
		}

		t.Run(definition.name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("catalogued builtin panicked during empty-argument reachability check: %v", recovered)
				}
			}()
			if _, handled := runtime.callBuiltin(definition.name, nil); !handled {
				t.Fatal("catalogued builtin has no reachable runtime handler")
			}
		})
	}

	if len(GetBuiltinFunctionNames()) != len(seen) {
		t.Fatalf("public builtin projection has %d names, canonical catalog has %d", len(GetBuiltinFunctionNames()), len(seen))
	}
}

func TestNativeMethodDefinitionsProjectToRuntimeAndAnalyzer(t *testing.T) {
	definitions := GetNativeMethodDefinitions()
	environment := buildAnalysisEnvironment()
	runtime := NewRuntime()
	defer runtime.Free()

	for _, className := range []string{"Stack", "Queue", "Math", "JSON", "Markdown", "Str", "UUID", "Lang", "Console", "Zip"} {
		classDefinitions := definitions[className]
		if len(classDefinitions) == 0 {
			t.Fatalf("migrated native class %s has no semantic definitions", className)
		}
		seen := make(map[string]bool, len(classDefinitions))
		for _, definition := range classDefinitions {
			if seen[definition.Name] {
				t.Fatalf("duplicate native definition %s::%s", className, definition.Name)
			}
			seen[definition.Name] = true
			callable, exists := environment.Classes[className].Methods[definition.Name]
			if !exists {
				t.Fatalf("analyzer projection is missing %s::%s", className, definition.Name)
			}
			if callable.ReturnType.String() != definition.ReturnType.String() {
				t.Fatalf("%s::%s return = %s, want %s", className, definition.Name, callable.ReturnType, definition.ReturnType)
			}
			if !definition.ArityKnown && !callable.Variadic {
				t.Fatalf("unknown arity for %s::%s was published as exact", className, definition.Name)
			}
		}
		if runtime.NativeHandlers[className] == nil {
			t.Fatalf("migrated native class %s has no runtime implementation", className)
		}
	}
}
