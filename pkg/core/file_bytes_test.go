package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInternalFileGetBytesPreservesNULAndHighBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tzif")
	want := []byte{'T', 'Z', 'i', 'f', 0, 2, 0x80, 0xFF}
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}

	runtime := NewRuntime()
	gotValue, handled := runtime.callBuiltinIO("__joss_file_get_bytes", []interface{}{path})
	if !handled {
		t.Fatal("internal byte reader was not handled")
	}
	got, ok := gotValue.([]interface{})
	if !ok {
		t.Fatalf("result has type %T, want []interface{}", gotValue)
	}
	if len(got) != len(want) {
		t.Fatalf("result length = %d, want %d", len(got), len(want))
	}
	for index, expected := range want {
		actual, ok := got[index].(int64)
		if !ok || actual != int64(expected) {
			t.Errorf("byte %d = %#v, want %d", index, got[index], expected)
		}
	}
}
