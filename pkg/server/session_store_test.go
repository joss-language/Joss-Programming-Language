package server

import (
	"path/filepath"
	"testing"
)

func TestFileSessionsSurviveMemoryReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	env := map[string]string{"SESSION_FILE": path}

	sessionMu.Lock()
	previousStore := sessionStore
	previousPath := loadedSessionPath
	sessionStore = map[string]map[string]interface{}{"abc": {"user_id": float64(7)}}
	loadedSessionPath = path
	if err := persistFileSessions(env); err != nil {
		sessionMu.Unlock()
		t.Fatal(err)
	}
	sessionStore["abc"]["user_id"] = float64(8)
	if err := persistFileSessions(env); err != nil {
		sessionMu.Unlock()
		t.Fatalf("second atomic replacement failed: %v", err)
	}
	sessionStore = make(map[string]map[string]interface{})
	loadedSessionPath = ""
	if err := ensureFileSessionsLoaded(env); err != nil {
		sessionMu.Unlock()
		t.Fatal(err)
	}
	got := sessionStore["abc"]["user_id"]
	sessionStore = previousStore
	loadedSessionPath = previousPath
	sessionMu.Unlock()

	if got != float64(8) {
		t.Fatalf("persisted user_id = %v", got)
	}
}

func TestSessionAndRateDefaults(t *testing.T) {
	if got := sessionDriver(map[string]string{}); got != "file" {
		t.Fatalf("default session driver = %q", got)
	}
	if got := envPositiveInt(map[string]string{"RATE_LIMIT_REQUESTS": "250"}, "RATE_LIMIT_REQUESTS", 60); got != 250 {
		t.Fatalf("configured limit = %d", got)
	}
}
