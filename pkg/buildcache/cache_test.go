package buildcache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCacheStoreAndRetrieve(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "sample.joss")
	if err := os.WriteFile(srcFile, []byte("print('hello')"), 0644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}

	key, err := CacheKey(srcFile, "release", "windows", "amd64")
	if err != nil {
		t.Fatalf("failed to calculate cache key: %v", err)
	}
	if len(key) != 64 {
		t.Errorf("expected 64-char sha256 key, got %d", len(key))
	}

	cacheDir := filepath.Join(tempDir, ".cache")
	bytecodeSample := []byte("JOSSBC2Z-TEST-BYTES")

	if err := StoreBytecode(cacheDir, key, bytecodeSample); err != nil {
		t.Fatalf("failed to store bytecode: %v", err)
	}

	retrieved, found := GetBytecode(cacheDir, key)
	if !found {
		t.Fatalf("expected bytecode to be found in cache")
	}
	if string(retrieved) != string(bytecodeSample) {
		t.Errorf("expected cached payload match")
	}
}
