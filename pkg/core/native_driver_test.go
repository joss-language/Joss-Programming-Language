package core

import "testing"

func TestCallLoadedNativeDriverCopiesAndFreesCString(t *testing.T) {
	response := append([]byte(`{"result":42}`), 0)
	freed := false
	driver := &NativeDriverDefinition{
		Call: func(method, args string) *byte {
			if method != "sum" || args != "[20,22]" {
				t.Fatalf("unexpected ABI call: %s %s", method, args)
			}
			return &response[0]
		},
		Free: func(pointer *byte) {
			freed = pointer == &response[0]
		},
	}
	got, err := callLoadedNativeDriver(driver, "sum", "[20,22]")
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"result":42}` || !freed {
		t.Fatalf("result=%q freed=%v", got, freed)
	}
}

func TestInstallNativeDriverReleasesReplacedOwner(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	previous := &NativeDriverDefinition{Name: "demo", Call: func(_, _ string) *byte { return nil }}
	replacement := &NativeDriverDefinition{Name: "demo", Call: func(_, _ string) *byte { return nil }}
	if err := runtime.installNativeDriver("demo", previous); err != nil {
		t.Fatal(err)
	}
	if err := runtime.installNativeDriver("demo", replacement); err != nil {
		t.Fatal(err)
	}
	if !previous.unloaded {
		t.Fatal("replacing a native driver did not release the previous owner")
	}
	if replacement.unloaded {
		t.Fatal("replacement native driver was unloaded during installation")
	}
}
