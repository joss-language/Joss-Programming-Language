package server

import (
	"testing"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/vfs"
)

func TestPrepareHotReloadProjectAnalyzesAllRuntimeSources(t *testing.T) {
	previousFS := GlobalFileSystem
	defer func() { GlobalFileSystem = previousFS }()
	mem := vfs.NewMemFS()
	mem.Files = map[string][]byte{
		"app/services/user.joss": []byte(`public class User {}`),
		"routes.joss":            []byte(`$user = new MissingClass()`),
	}
	GlobalFileSystem = mem
	report := prepareHotReloadProject()
	if !report.HasErrors() {
		t.Fatalf("expected cross-file semantic error, got %#v", report.Diagnostics)
	}
}

func TestRejectedPreparedReloadKeepsActiveRuntime(t *testing.T) {
	previousFS := GlobalFileSystem
	previousRuntime := currentRuntime
	defer func() {
		GlobalFileSystem = previousFS
		currentRuntime = previousRuntime
	}()
	mem := vfs.NewMemFS()
	mem.Files = map[string][]byte{"routes.joss": []byte(`$value = new MissingClass()`)}
	GlobalFileSystem = mem
	active := core.NewRuntime()
	defer active.Free()
	currentRuntime = active
	if reloadPreparedJossRuntime() {
		t.Fatal("expected invalid project reload to be rejected")
	}
	if currentRuntime != active {
		t.Fatal("rejected reload replaced the active runtime")
	}
}
