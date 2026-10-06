package buildcache

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/jossecurity/joss/pkg/version"
)

// CacheKey computes a content-addressable hash for a specific file and build context.
func CacheKey(filePath string, buildMode string, targetOS, targetArch string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	hasher.Write([]byte(version.Version + ":" + buildMode + ":" + targetOS + ":" + targetArch + ":"))
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// GetBytecode retrieves cached bytecode if valid.
func GetBytecode(cacheDir, key string) ([]byte, bool) {
	target := filepath.Join(cacheDir, "bytecode", key+".jbc")
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, false
	}
	return data, true
}

// StoreBytecode saves compiled bytecode into cache.
func StoreBytecode(cacheDir, key string, data []byte) error {
	dir := filepath.Join(cacheDir, "bytecode")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, key+".jbc"), data, 0644)
}
