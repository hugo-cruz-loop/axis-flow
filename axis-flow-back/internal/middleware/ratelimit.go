package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient is the minimal interface used by RateLimitMiddleware.
// Defined in the consumer package for easy mocking in tests.
type RedisClient interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
}

// RateLimitMiddleware returns a chi-compatible middleware that limits requests
// using a sliding window counter stored in Redis.
//
// Key pattern:
//   - login endpoint: "auth:login-fail:{ip}"
//   - general:        "ratelimit:{ip}:{path}"
//
// If Redis is unavailable, the middleware fails open: the request is allowed
// through and a warning is logged.
func RateLimitMiddleware(redisClient RedisClient, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
				ip = realIP
			} else if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = forwarded
			}

			var key string
			if r.URL.Path == "/api/auth/login/" {
				key = fmt.Sprintf("auth:login-fail:%s", ip)
			} else {
				key = fmt.Sprintf("ratelimit:%s:%s", ip, r.URL.Path)
			}

			ctx := r.Context()

			count, err := redisClient.Incr(ctx, key).Result()
			if err != nil {
				// Fail open: Redis unavailable — log and allow.
				slog.WarnContext(ctx, "rate limit: redis unavailable, failing open",
					slog.String("error", err.Error()),
					slog.String("key", key),
				)
				next.ServeHTTP(w, r)
				return
			}

			// Set expiry only on first increment.
			if count == 1 {
				if expErr := redisClient.Expire(ctx, key, window).Err(); expErr != nil {
					slog.WarnContext(ctx, "rate limit: failed to set key expiry",
						slog.String("error", expErr.Error()),
					)
				}
			}

			if count > int64(limit) {
				retryAfter := int(window.Seconds())
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
