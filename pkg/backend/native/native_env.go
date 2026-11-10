package native

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jossecurity/joss/pkg/crypto"
)

// readBuildTimeEnv reads and returns the project environment from disk during compilation.
// It checks env.joss, env.enc (decrypting it with the master secret), and .env.
func readBuildTimeEnv() string {
	baseDirs := []string{"."}
	if envDir := os.Getenv("JOSS_PROJECT_DIR"); envDir != "" {
		baseDirs = append(baseDirs, envDir)
	}
	baseDirs = append(baseDirs, "..", "../..")

	fileNames := []string{
		"env.joss",
		"env.enc",
		".env",
	}

	masterSecret := []byte("JOSSECURITY_MASTER_SECRET_2025")

	for _, dir := range baseDirs {
		for _, name := range fileNames {
			fullPath := filepath.Join(dir, name)
			data, err := os.ReadFile(fullPath)
			if err != nil || len(data) == 0 {
				continue
			}
			if strings.HasSuffix(name, ".enc") {
				if len(data) > 16 {
					salt := data[:16]
					ciphertext := data[16:]
					key := crypto.DeriveKey(masterSecret, salt)
					decrypted, decErr := crypto.DecryptAES(ciphertext, key)
					if decErr == nil && len(decrypted) > 0 {
						return string(decrypted)
					}
				}
				continue
			}
			return string(data)
		}
	}
	return ""
}
