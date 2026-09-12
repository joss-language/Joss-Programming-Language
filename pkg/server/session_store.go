package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/redis/go-redis/v9"
)

var loadedSessionPath string

func initializeRedisSessions(env map[string]string) error {
	redisURL := strings.TrimSpace(env["REDIS_URL"])
	if redisURL == "" {
		redisURL = strings.TrimSpace(os.Getenv("REDIS_URL"))
	}
	if redisURL != "" {
		opts, err := redis.ParseURL(redisURL)
		if err == nil {
			core.GlobalRedis = redis.NewClient(opts)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := core.GlobalRedis.Ping(ctx).Err(); err != nil {
				_ = core.GlobalRedis.Close()
				core.GlobalRedis = nil
				return fmt.Errorf("redis %s no disponible: %w", redisURL, err)
			}
			return nil
		}
	}

	host := strings.TrimSpace(env["REDIS_HOST"])
	if host == "" {
		host = strings.TrimSpace(os.Getenv("REDIS_HOST"))
	}
	if host == "" {
		host = "127.0.0.1:6379"
	} else if !strings.Contains(host, ":") {
		port := strings.TrimSpace(env["REDIS_PORT"])
		if port == "" {
			port = strings.TrimSpace(os.Getenv("REDIS_PORT"))
		}
		if port == "" {
			port = "6379"
		}
		host = host + ":" + port
	}

	database, err := strconv.Atoi(strings.TrimSpace(env["REDIS_DB"]))
	if err != nil {
		database = 0
	}
	password := env["REDIS_PASSWORD"]
	if password == "" {
		password = os.Getenv("REDIS_PASSWORD")
	}

	core.InitRedis(host, password, database)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := core.GlobalRedis.Ping(ctx).Err(); err != nil {
		_ = core.GlobalRedis.Close()
		core.GlobalRedis = nil
		return fmt.Errorf("redis %s no disponible: %w", host, err)
	}
	return nil
}

func sessionDriver(env map[string]string) string {
	driver := strings.ToLower(strings.TrimSpace(env["SESSION_DRIVER"]))
	if driver == "" {
		return "file"
	}
	return driver
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
		result[key] = value
	}
	return result
}

// loadSession is the single backend-neutral read contract. A missing session is
// an empty map; backend unavailability and corrupt serialized data are errors.
func loadSession(env map[string]string, sessionID string) (map[string]interface{}, string, error) {
	driver := sessionDriver(env)
	if driver == "redis" {
		if core.GlobalRedis == nil {
			return nil, driver, fmt.Errorf("redis session storage is not available")
		}
		value, err := core.GlobalRedis.Get(core.Ctx, "session:"+sessionID).Result()
		if err == redis.Nil {
			return make(map[string]interface{}), driver, nil
		}
		if err != nil {
			return nil, driver, fmt.Errorf("read redis session: %w", err)
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
	if driver == "redis" {
		if core.GlobalRedis == nil {
			return fmt.Errorf("redis session storage is not available")
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("encode redis session: %w", err)
		}
		if err := core.GlobalRedis.Set(core.Ctx, "session:"+sessionID, encoded, 24*time.Hour).Err(); err != nil {
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
