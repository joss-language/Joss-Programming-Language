package main

import (
	"testing"
)

func TestPluginCommandsDiscoveryAndHelp(t *testing.T) {
	plugins := discoverPlugins()
	if len(plugins) == 0 {
		t.Fatalf("expected at least standard official plugins to be discovered")
	}

	// Verify joss_ai plugin
	ai, ok := plugins["joss_ai"]
	if !ok {
		t.Fatalf("expected joss_ai plugin to be discovered")
	}
	if _, ok := ai.Commands["ai:activate"]; !ok {
		t.Fatalf("expected ai:activate command in joss_ai")
	}

	// Verify joss_brevo plugin
	brevo, ok := plugins["joss_brevo"]
	if !ok {
		t.Fatalf("expected joss_brevo plugin to be discovered")
	}
	if _, ok := brevo.Commands["brevo:config"]; !ok {
		t.Fatalf("expected brevo:config command in joss_brevo")
	}

	// Verify unknown command returns false
	if tryDispatchPluginCommand("unknown:cmd", nil) {
		t.Fatalf("expected unknown:cmd to return false")
	}

	// Verify known plugin command returns true
	if !tryDispatchPluginCommand("notify:send", nil) {
		t.Fatalf("expected notify:send to be dispatched by discovered plugin")
	}
}

