package server

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rateLimitEntry struct {
	count       int
	windowStart time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateLimitEntry
}

var requestRateLimiter = rateLimiter{entries: make(map[string]rateLimitEntry)}

func (limiter *rateLimiter) allow(key string, limit int, window time.Duration, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	entry, exists := limiter.entries[key]
	if !exists || now.Sub(entry.windowStart) > window {
		entry = rateLimitEntry{windowStart: now}
	}
	entry.count++
	limiter.entries[key] = entry
	return entry.count <= limit
}

func enforceRateLimit(w http.ResponseWriter, request *http.Request, environment map[string]string) bool {
	limit := envPositiveInt(environment, "RATE_LIMIT_REQUESTS", 60)
	window := time.Duration(envPositiveInt(environment, "RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second
	if requestRateLimiter.allow(requestClientIP(request), limit, window, time.Now()) {
		return true
	}

	w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = fmt.Fprint(w, "<h1>429 Too Many Requests</h1>")
	return false
}

func requestClientIP(request *http.Request) string {
	if forwarded := request.Header.Get("X-Forwarded-For"); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}
