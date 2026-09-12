package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeRequestDataCombinesJSONQueryAndMetadata(t *testing.T) {
	request := httptest.NewRequest("POST", "https://example.test/items?page=2", strings.NewReader(`{"name":"Ada"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer token")
	request.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})

	data := decodeRequestData(request, "example.test", "https")
	if data["page"] != "2" || data["name"] != "Ada" || data["_method"] != "POST" {
		t.Fatalf("unexpected decoded request: %#v", data)
	}
	if data["Authorization"] != "Bearer token" || data["_host"] != "example.test" || data["_scheme"] != "https" {
		t.Fatalf("missing request metadata: %#v", data)
	}
	cookies := data["_cookies"].(map[string]interface{})
	if cookies["theme"] != "dark" {
		t.Fatalf("missing decoded cookie: %#v", cookies)
	}
}
