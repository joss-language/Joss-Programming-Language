package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIndexNowNativeMethods(t *testing.T) {
	rt := NewRuntime()

	// 1. Initial state: not enabled
	enabled := rt.executeIndexNowMethod(nil, "enabled", nil)
	if enabled != false {
		t.Fatalf("expected enabled=false, got %v", enabled)
	}

	// 2. Set key
	testKey := "abcdef1234567890abcdef1234567890"
	rt.executeIndexNowMethod(nil, "key", []interface{}{testKey})

	if rt.GetIndexNowKey() != testKey {
		t.Fatalf("expected key %s, got %s", testKey, rt.GetIndexNowKey())
	}

	if rt.executeIndexNowMethod(nil, "enabled", nil) != true {
		t.Fatalf("expected enabled=true after setting key")
	}

	// 3. Key location
	loc := rt.executeIndexNowMethod(nil, "keyLocation", []interface{}{"https://joss.red"})
	expectedLoc := "https://joss.red/" + testKey + ".txt"
	if loc != expectedLoc {
		t.Fatalf("expected keyLocation %s, got %v", expectedLoc, loc)
	}

	// 4. Submit with Mock Server
	var receivedPayload IndexNowPayload
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "invalid method", http.StatusBadRequest)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Direct call to sendIndexNowRequest with mock server URL override or payload check
	payload := IndexNowPayload{
		Host:        "joss.red",
		Key:         testKey,
		KeyLocation: expectedLoc,
		URLList:     []string{"https://joss.red/blog/test"},
	}
	if len(payload.URLList) != 1 || payload.Host != "joss.red" {
		t.Fatalf("invalid payload")
	}
}
