package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPluginCommandsDiscoveryAndHelp(t *testing.T) {
	tempPluginsDir := t.TempDir()

	// 1. Create a mock plugin with joss.yaml
	testPluginDir := filepath.Join(tempPluginsDir, "joss_test_plug")
	if err := os.MkdirAll(testPluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	testYaml := `name: "joss_test_plug"
version: "1.0.0"
description: "Plugin de prueba para discovery"
repository: "https://github.com/joss-language/joss_test_plug"
commands:
  test:hello:
    name: "test:hello"
    description: "Comando de prueba"
    usage: "joss test:hello"
    protected: true
`
	if err := os.WriteFile(filepath.Join(testPluginDir, "joss.yaml"), []byte(testYaml), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Create a mock compiled .jp file
	if err := os.WriteFile(filepath.Join(tempPluginsDir, "dummy_compiled.jp"), []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Test discovery with extraDirs
	plugins := discoverPlugins(tempPluginsDir)
	if len(plugins) == 0 {
		t.Fatalf("expected plugins to be discovered from tempPluginsDir")
	}

	testPlug, ok := plugins["joss_test_plug"]
	if !ok {
		t.Fatalf("expected joss_test_plug to be discovered")
	}
	if testPlug.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", testPlug.Version)
	}
	cmd, ok := testPlug.Commands["test:hello"]
	if !ok {
		t.Fatalf("expected test:hello command in joss_test_plug")
	}
	if !cmd.Protected {
		t.Errorf("expected test:hello to be protected")
	}

	// Verify compiled .jp was discovered
	if _, ok := plugins["dummy_compiled"]; !ok {
		t.Fatalf("expected dummy_compiled.jp to be discovered")
	}

	// 4. Test dispatch with extraDirs
	if tryDispatchPluginCommand("unknown:cmd", nil, tempPluginsDir) {
		t.Fatalf("expected unknown:cmd to return false")
	}
	if !tryDispatchPluginCommand("test:hello", nil, tempPluginsDir) {
		t.Fatalf("expected test:hello to return true (command discovered)")
	}

	// 5. Test handlePluginHelp executes cleanly
	handlePluginHelp("", tempPluginsDir)
	handlePluginHelp("test_plug", tempPluginsDir)

	// 6. If local official plugins exist on disk, optionally assert them
	if ai, ok := plugins["joss_ai"]; ok {
		if _, ok := ai.Commands["ai:activate"]; !ok {
			t.Errorf("expected ai:activate command in joss_ai")
		}
	}
}
