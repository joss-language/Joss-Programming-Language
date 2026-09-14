package core

import "testing"

func TestFreeingForkDoesNotEraseParentPluginRegistries(t *testing.T) {
	parent := NewRuntime()
	defer parent.Free()
	parent.NativePlugins["demo"] = &NativePluginDefinition{Name: "demo", Version: "1.0.0"}
	parent.NativeDrivers["demo"] = &NativeDriverDefinition{Name: "demo"}
	fork := parent.Fork()
	fork.Free()
	if parent.NativePlugins["demo"] == nil {
		t.Fatal("freeing request fork erased parent native plugin")
	}
	if parent.NativeDrivers["demo"] == nil {
		t.Fatal("freeing request fork erased parent native driver")
	}
}
