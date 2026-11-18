package core

import (
	"net/http"
	"testing"
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
