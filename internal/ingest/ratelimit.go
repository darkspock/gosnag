package ingest

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientState
	limit   int
	window  time.Duration
}

type clientState struct {
	count   int
	resetAt time.Time
}

func NewRateLimiter(limitPerWindow int, window time.Duration) func(http.Handler) http.Handler {
	if limitPerWindow <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	rl := &rateLimiter{
		clients: make(map[string]*clientState),
		limit:   limitPerWindow,
		window:  window,
	}

	go rl.cleanup()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			if ip == "" {
				ip = r.RemoteAddr
			}

			if !rl.allow(ip) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	c, ok := rl.clients[ip]
	if !ok || now.After(c.resetAt) {
		rl.clients[ip] = &clientState{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	c.count++
	return c.count <= rl.limit
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for k, v := range rl.clients {
			if now.After(v.resetAt) {
				delete(rl.clients, k)
			}
		}
		rl.mu.Unlock()
	}
}
