package jit

import "testing"

func TestJSONDecodeHostBridgeConvertsDecodedValuesToJossMemory(t *testing.T) {
	input := nativeString(`{"ok":true,"items":["a",2],"nil":null}`)
	result := handleJsonDecode([]uintptr{input}...)
	if result == 0 {
		t.Fatal("valid JSON must produce a Joss value")
	}
	decoded, ok := readNativeValue(result).(map[string]interface{})
	if !ok {
		t.Fatalf("decoded JSON type = %T, want map", readNativeValue(result))
	}
	// The current native dynamic-value ABI represents JSON booleans as 0/1.
	if decoded["ok"] != int64(1) {
		t.Fatalf("decoded boolean representation = %#v, want native true (1)", decoded["ok"])
	}
	items, ok := decoded["items"].([]interface{})
	if !ok || len(items) != 2 || items[0] != "a" {
		t.Fatalf("decoded array = %#v", decoded["items"])
	}
	if decoded["nil"] != nil {
		t.Fatalf("decoded null = %#v, want nil", decoded["nil"])
	}

	if invalid := handleJsonDecode([]uintptr{nativeString("{")}...); invalid != 0 {
		t.Fatalf("invalid JSON result = %#x, want nil pointer", invalid)
	}
}
