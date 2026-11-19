package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/jossecurity/joss/pkg/crypto"
)

// EnvironmentFileCandidates defines all supported environment file variants in order of priority.
var EnvironmentFileCandidates = []string{
	"env.joss",
	".env",
	"joss.env",
	".env.local",
	"env.enc",
	".env.enc",
	"env.json",
	".env.json",
}

// EnvFileDetectResult contains detected file metadata and parsed environment key-value pairs.
type EnvFileDetectResult struct {
	FilePath    string            `json:"file_path"`
	IsEncrypted bool              `json:"is_encrypted"`
	IsJSON      bool              `json:"is_json"`
	EnvMap      map[string]string `json:"env_map"`
}

// DetectAndLoadEnv detects, decrypts, and parses environment files from VFS or local disk.
func DetectAndLoadEnv(fs http.FileSystem) EnvFileDetectResult {
	result := EnvFileDetectResult{
		EnvMap: make(map[string]string),
	}

	for _, name := range EnvironmentFileCandidates {
		var content []byte
		var err error

		if fs != nil {
			f, openErr := fs.Open(name)
			if openErr == nil {
				stat, _ := f.Stat()
				content = make([]byte, stat.Size())
				f.Read(content)
				f.Close()
			}
		} else {
			content, err = os.ReadFile(name)
		}

		if err == nil && len(content) > 0 {
			result.FilePath = name

			// 1. Decrypt if encrypted format (env.enc, .env.enc)
			if strings.HasSuffix(name, ".enc") {
				result.IsEncrypted = true
				if len(content) > 16 {
					salt := content[:16]
					ciphertext := content[16:]
					masterSecret, keyErr := crypto.ResolveEnvEncryptionKey()
					if keyErr != nil {
						fmt.Printf("[EnvLoader] Error desencriptando %s: %v\n", name, keyErr)
					} else {
						key := crypto.DeriveKey(masterSecret, salt)
						decrypted, decErr := crypto.DecryptAES(ciphertext, key)
						if decErr == nil {
							content = decrypted
						} else {
							fmt.Printf("[EnvLoader] Error desencriptando %s: %v\n", name, decErr)
						}
					}
				}
			}

			// 2. Parse JSON format if env.json / .env.json or starts with '{'
			if strings.HasSuffix(name, ".json") || (len(content) > 0 && bytes.HasPrefix(bytes.TrimSpace(content), []byte("{"))) {
				result.IsJSON = true
				var jsonMap map[string]interface{}
				if jsonErr := json.Unmarshal(content, &jsonMap); jsonErr == nil {
					for k, v := range jsonMap {
						result.EnvMap[k] = fmt.Sprintf("%v", v)
					}
					return result
				}
			}

			// 3. Standard Key=Value / Dotenv / Joss parser
			lines := bytes.Split(content, []byte("\n"))
			for _, line := range lines {
				line = bytes.TrimSpace(line)
				if len(line) == 0 || bytes.HasPrefix(line, []byte("#")) {
					continue
				}
				parts := bytes.SplitN(line, []byte("="), 2)
				if len(parts) == 2 {
					key := string(bytes.TrimSpace(parts[0]))
					val := string(bytes.TrimSpace(parts[1]))
					val = strings.Trim(val, "\"")
					val = strings.Trim(val, "'")
					result.EnvMap[key] = val
				}
			}
			return result
		}
	}

	return result
}

// GetEnvFile returns the active environment file path or "env.joss" as fallback.
func GetEnvFile() string {
	res := DetectAndLoadEnv(nil)
	if res.FilePath != "" {
		return res.FilePath
	}
	return "env.joss"
}

// ResolvePort retrieves the HTTP port from EnvMap, OS environment, or defaults to "8000".
func ResolvePort(envMap map[string]string) string {
	if val := os.Getenv("PORT"); val != "" {
		return val
	}
	if val := os.Getenv("JOSS_PORT"); val != "" {
		return val
	}
	if envMap != nil {
		for _, key := range []string{"PORT", "JOSS_PORT", "APP_PORT", "SERVER_PORT"} {
			if val, ok := envMap[key]; ok && val != "" {
				return val
			}
		}
	}
	return "8000"
}

// ReadEnvFile reads a file and returns its parsed environment key-value map.
func ReadEnvFile(path string) map[string]string {
	m := make(map[string]string)
	content, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"")
			val = strings.Trim(val, "'")
			m[strings.TrimSpace(parts[0])] = val
		}
	}
	return m
}

// UpdateEnvFile updates or appends a key=value pair in an environment file.
func UpdateEnvFile(path, key, value string) error {
	content, _ := os.ReadFile(path)
	lines := strings.Split(string(content), "\n")
	found := false
	var newLines []string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			newLines = append(newLines, fmt.Sprintf("%s=%s", key, value))
			found = true
		} else {
			newLines = append(newLines, line)
		}
	}
	if !found {
		newLines = append(newLines, fmt.Sprintf("%s=%s", key, value))
	}
	return os.WriteFile(path, []byte(strings.Join(newLines, "\n")), 0644)
}

// RemoveEnvKey removes a key from an environment file.
func RemoveEnvKey(path, key string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	var newLines []string
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			newLines = append(newLines, line)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(newLines, "\n")), 0644)
}
