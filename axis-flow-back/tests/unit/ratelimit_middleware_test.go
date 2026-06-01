package unit_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"axis-flow-back/internal/middleware"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// mockRedisClient implements middleware.RedisClient for tests.
type mockRedisClient struct {
	counter int64
	err     error
}

func (m *mockRedisClient) Incr(_ context.Context, _ string) *redis.IntCmd {
	if m.err != nil {
		cmd := redis.NewIntCmd(context.Background())
		cmd.SetErr(m.err)
		return cmd
	}
	m.counter++
	cmd := redis.NewIntCmd(context.Background())
	cmd.SetVal(m.counter)
	return cmd
}

func (m *mockRedisClient) Expire(_ context.Context, _ string, _ time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(context.Background())
	cmd.SetVal(true)
	return cmd
}

func TestRateLimitMiddleware_BlocksAfterLimit(t *testing.T) {
	mock := &mockRedisClient{}
	limit := 3
	mw := middleware.RateLimitMiddleware(mock, limit, time.Minute)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 1; i <= limit; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should pass", i)
	}

	// One over the limit — should be blocked.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

func TestRateLimitMiddleware_FailsOpenOnRedisError(t *testing.T) {
	mock := &mockRedisClient{err: errors.New("connection refused")}
	limit := 3
	mw := middleware.RateLimitMiddleware(mock, limit, time.Minute)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Even with Redis down, requests should go through.
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should pass (fail open)", i+1)
	}
}
