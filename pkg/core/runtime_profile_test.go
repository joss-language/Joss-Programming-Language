package core

import "testing"

func TestRuntimeProfilesShareLanguageAndConstrainCapabilities(t *testing.T) {
	coreRuntime := NewRuntimeWithProfile(RuntimeProfileCore)
	defer coreRuntime.Free()
	if coreRuntime.Profile != RuntimeProfileCore || coreRuntime.CanAccessFS() || coreRuntime.CanAccessNetwork() {
		t.Fatalf("core profile capabilities = %#v", coreRuntime.Capabilities)
	}
	if coreRuntime.Classes["Math"] == nil || coreRuntime.Classes["Router"] == nil {
		t.Fatal("profiles must not create incompatible language catalogs")
	}

	serverRuntime := NewRuntimeWithProfile(RuntimeProfileServer)
	defer serverRuntime.Free()
	if !serverRuntime.CanAccessFS() || !serverRuntime.CanAccessNetwork() || serverRuntime.CanExecuteProcess() {
		t.Fatalf("server profile capabilities = %#v", serverRuntime.Capabilities)
	}
}
