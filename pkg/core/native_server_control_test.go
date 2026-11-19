package core

import (
	"net/http"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestServerStartUsesGoHTTPCallbackWhenSelected(t *testing.T) {
	previousCallback := ServerStartCallback
	previousFS := GlobalFileSystem
	t.Cleanup(func() {
		ServerStartCallback = previousCallback
		GlobalFileSystem = previousFS
	})

	fs := http.Dir(t.TempDir())
	GlobalFileSystem = fs
	called := false
	ServerStartCallback = func(got http.FileSystem) {
		called = true
		if got != fs {
			t.Errorf("Go HTTP callback filesystem = %v, want configured filesystem", got)
		}
	}

	runtime := NewRuntime()
	defer runtime.Free()
	runtime.Env["JOSS_HTTP_BACKEND"] = "go"

	result := runtime.executeServerControlMethod(nil, "start", nil)
	if result != true {
		t.Fatalf("Server::start result = %v, want true", result)
	}
	if !called {
		t.Fatal("Server::start did not dispatch to the configured Go HTTP callback")
	}
}

func TestServerStartEvaluatedPrefersGoHTTPWhenBackendIsGo(t *testing.T) {
	previousCallback := ServerStartCallback
	previousFS := GlobalFileSystem
	t.Cleanup(func() {
		ServerStartCallback = previousCallback
		GlobalFileSystem = previousFS
	})

	fs := http.Dir(t.TempDir())
	GlobalFileSystem = fs
	called := false
	ServerStartCallback = func(got http.FileSystem) {
		called = true
	}

	runtime := NewRuntime()
	defer runtime.Free()
	runtime.RegisterNativeClasses()
	runtime.Env["JOSS_HTTP_BACKEND"] = "go"

	// Register a shadow class Server with a start() method like std/http/server.joss does
	runtime.Classes["Server"] = &parser.ClassStatement{
		Name: &parser.Identifier{Value: "Server"},
		Body: &parser.BlockStatement{
			Statements: []parser.Statement{
				&parser.MethodStatement{
					Name:       &parser.Identifier{Value: "start"},
					IsStatic:   true,
					ReturnType: parser.Token{Literal: "void"},
					Body:       &parser.BlockStatement{},
				},
			},
		},
	}

	call := &parser.CallExpression{
		Function: &parser.MemberExpression{
			Left:     &parser.Identifier{Value: "Server"},
			Property: &parser.Identifier{Value: "start"},
		},
	}
	runtime.executeCall(call)

	if !called {
		t.Fatal("Server::start expression evaluation did not dispatch to Go HTTP callback when JOSS_HTTP_BACKEND=go")
	}
}
