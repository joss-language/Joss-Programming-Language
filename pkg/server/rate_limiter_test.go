package server

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterOwnsWindowState(t *testing.T) {
	limiter := rateLimiter{entries: make(map[string]rateLimitEntry)}
	now := time.Unix(100, 0)
	if !limiter.allow("client", 2, time.Minute, now) || !limiter.allow("client", 2, time.Minute, now) {
		t.Fatal("rate limiter rejected a request within the configured limit")
	}
	if limiter.allow("client", 2, time.Minute, now) {
		t.Fatal("rate limiter accepted a request above the configured limit")
	}
	if !limiter.allow("client", 2, time.Minute, now.Add(2*time.Minute)) {
		t.Fatal("rate limiter did not reset after the window elapsed")
	}
}

func TestRequestClientIPUsesFirstForwardedAddressAndSupportsIPv6(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "[2001:db8::1]:8080"
	if got := requestClientIP(request); got != "2001:db8::1" {
		t.Fatalf("IPv6 client IP = %q", got)
	}
	request.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := requestClientIP(request); got != "203.0.113.7" {
		t.Fatalf("forwarded client IP = %q", got)
	}
}
