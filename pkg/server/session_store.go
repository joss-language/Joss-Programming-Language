package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jossecurity/joss/pkg/core"
)

var loadedSessionPath string

func initializeRedisSessions(env map[string]string) error {
	return core.InitRedisSessions(env)
}

func sessionDriver(env map[string]string) string {
	driver := strings.ToLower(strings.TrimSpace(env["SESSION_DRIVER"]))
	if driver == "" {
		return "file"
	}
	return driver
}

func validateSessionDriver(driver string) error {
	switch driver {
	case "memory", "file", "redis":
		return nil
	default:
		return fmt.Errorf("session driver %q is not supported", driver)
	}
}

func sessionFilePath(env map[string]string) string {
	path := strings.TrimSpace(env["SESSION_FILE"])
	if path == "" {
		return filepath.Join("storage", "sessions.json")
	}
	return filepath.Clean(path)
}

func cloneSessionData(source map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(source))
	for key, value := range source {
		result[key] = cloneSessionValue(value)
	}
	return result
}

func cloneSessionValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return cloneSessionData(typed)
	case []interface{}:
		result := make([]interface{}, len(typed))
		for index := range typed {
			result[index] = cloneSessionValue(typed[index])
		}
		return result
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

// loadSession is the single backend-neutral read contract. A missing session is
// an empty map; backend unavailability and corrupt serialized data are errors.
func loadSession(env map[string]string, sessionID string) (map[string]interface{}, string, error) {
	driver := sessionDriver(env)
	if err := validateSessionDriver(driver); err != nil {
		return nil, driver, err
	}
	if driver == "redis" {
		value, found, err := core.GetRedisSession(sessionID)
		if err != nil {
			if strings.Contains(err.Error(), "not available") {
				return nil, driver, err
			}
			return nil, driver, fmt.Errorf("read redis session: %w", err)
		}
		if !found {
			return make(map[string]interface{}), driver, nil
		}
		data := make(map[string]interface{})
		if err := json.Unmarshal([]byte(value), &data); err != nil {
			return nil, driver, fmt.Errorf("decode redis session: %w", err)
		}
		return data, driver, nil
	}

	sessionMu.Lock()
	defer sessionMu.Unlock()
	if driver == "file" {
		if err := ensureFileSessionsLoaded(env); err != nil {
			return nil, driver, fmt.Errorf("load file sessions: %w", err)
		}
	}
	if sessionStore[sessionID] == nil {
		sessionStore[sessionID] = make(map[string]interface{})
	}
	return cloneSessionData(sessionStore[sessionID]), driver, nil
}

// saveSession persists a complete session snapshot. Callers must surface its
// error; silently dropping authentication/session mutations is not permitted.
func saveSession(env map[string]string, driver, sessionID string, data map[string]interface{}) error {
	if err := validateSessionDriver(driver); err != nil {
		return err
	}
	if driver == "redis" {
		encoded, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("encode redis session: %w", err)
		}
		if err := core.SetRedisSession(sessionID, encoded, 24*time.Hour); err != nil {
			if strings.Contains(err.Error(), "not available") {
				return err
			}
			return fmt.Errorf("write redis session: %w", err)
		}
		return nil
	}

	sessionMu.Lock()
	defer sessionMu.Unlock()
	previous, existed := sessionStore[sessionID]
	sessionStore[sessionID] = cloneSessionData(data)
	if driver == "file" {
		if err := persistFileSessions(env); err != nil {
			if existed {
				sessionStore[sessionID] = previous
			} else {
				delete(sessionStore, sessionID)
			}
			return fmt.Errorf("write file sessions: %w", err)
		}
	}
	return nil
}

// ensureFileSessionsLoaded must be called while sessionMu is held.
func ensureFileSessionsLoaded(env map[string]string) error {
	path := sessionFilePath(env)
	if loadedSessionPath == path {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			sessionStore = make(map[string]map[string]interface{})
			loadedSessionPath = path
			return nil
		}
		return err
	}
	loaded := make(map[string]map[string]interface{})
	if len(content) > 0 {
		if err := json.Unmarshal(content, &loaded); err != nil {
			return fmt.Errorf("sesiones invalidas en %s: %w", path, err)
		}
	}
	sessionStore = loaded
	loadedSessionPath = path
	return nil
}

// persistFileSessions must be called while sessionMu is held.
func persistFileSessions(env map[string]string) error {
	path := sessionFilePath(env)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	content, err := json.Marshal(sessionStore)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".joss-sessions-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return replaceSessionFile(tempName, path)
}
