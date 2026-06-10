package handler

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// IPRateLimiter limits requests per IP using an in-memory sync.Map with sliding-window buckets.
type IPRateLimiter struct {
	limit  int
	window time.Duration
	mu     sync.Mutex
	hits   sync.Map // map[string][]time.Time
}

// NewIPRateLimiter constructs an IPRateLimiter.
func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	return &IPRateLimiter{limit: limit, window: window}
}

// Allow returns true if the IP is within the rate limit.
func (l *IPRateLimiter) Allow(ip string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	raw, _ := l.hits.LoadOrStore(ip, []time.Time{})
	times := raw.([]time.Time)

	// Prune expired entries
	valid := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.limit {
		l.hits.Store(ip, valid)
		return false
	}

	valid = append(valid, now)
	l.hits.Store(ip, valid)
	return true
}

// Middleware returns a chi-compatible middleware that enforces the rate limit.
func (l *IPRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := realIP(r)
			if !l.Allow(ip) {
				respondError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "too many requests — please try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// realIP extracts the client IP from X-Real-IP header, X-Forwarded-For, or RemoteAddr.
func realIP(r *http.Request) string {
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
