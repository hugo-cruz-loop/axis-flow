// Package service — locker_redis.go: the RedisLocker transport
// that satisfies the service.Locker port using the
// redis/go-redis/v9 client (already in go.mod).
//
// PR-6 (6.3) implementation. The lock uses the standard
// distributed-lock pattern: SET key value NX EX <ttl> stores a
// random token; the UnlockFn runs a Lua script that deletes the
// key only if the token matches (so a process that lost the lock
// due to TTL expiry cannot accidentally delete a NEW lock held
// by a different process).
//
// The lock key shape is the canonical formularios constant
// KeyReporteS3Lock (built internally — the previous Locker port
// leaked the key string; the new port takes uuid.UUID + duration
// only).
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"axis-flow-back/internal/formularios"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// Lock TTL default
// ---------------------------------------------------------------------------

// DefaultLockTTL is the TTL applied to a S3 report lock when the
// constructor receives a non-positive duration. The previous
// PDFService used 300 seconds (5 minutes) — that is preserved as
// the default for backward compat. The production wiring in
// main.go reads FORMULARIOS_LOCK_TTL (PR-6 6.4) and falls back
// to this default when the env var is unset.
const DefaultLockTTL = 5 * time.Minute

// ---------------------------------------------------------------------------
// RedisLocker
// ---------------------------------------------------------------------------

// RedisLocker is the Redis-backed implementation of the Locker
// port. The client is a redis.Cmdable so tests can inject a
// miniredis-backed client.
type RedisLocker struct {
	client redis.Cmdable
	ttl    time.Duration
}

// NewRedisLocker constructs a Redis-backed Locker with the
// configured TTL. A non-positive ttl falls back to
// DefaultLockTTL (5 minutes).
func NewRedisLocker(client redis.Cmdable, ttl time.Duration) *RedisLocker {
	if ttl <= 0 {
		ttl = DefaultLockTTL
	}
	return &RedisLocker{client: client, ttl: ttl}
}

// Acquire takes a Redis SET-NX-with-TTL lock on the canonical
// "formularios:reporte:lock:<iniciadoID>" key. The token is a
// random UUID v4; the UnlockFn runs a Lua script that deletes
// the key only if the token matches.
//
// Returns formularios.ErrConflict when the lock is already held
// (NX failed). Returns a wrapped formularios.ErrInternal for
// Redis I/O failures (network / timeout / MinIO) with a generic
// message; the underlying error is logged via slog at the
// service layer.
func (l *RedisLocker) Acquire(ctx context.Context, iniciadoID uuid.UUID) (UnlockFn, error) {
	key := buildLockKey(iniciadoID)
	token := uuid.NewString()

	ok, err := l.client.SetNX(ctx, key, token, l.ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("%w: redis SETNX: %s", formularios.ErrInternal, "lock acquire failed")
	}
	if !ok {
		return nil, formularios.ErrConflict
	}

	return func(ctx context.Context) error {
		// Safe-release Lua script: delete the key only if the
		// stored value still equals our token. This prevents a
		// process that lost the lock due to TTL expiry from
		// accidentally deleting a NEW lock held by a different
		// process.
		const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end`
		_, err := l.client.Eval(ctx, releaseScript, []string{key}, token).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return fmt.Errorf("%w: redis release: %s", formularios.ErrInternal, "lock release failed")
		}
		return nil
	}, nil
}

// AddToPending adds the iniciadoID to the canonical pending set
// (formularios.KeyReportePending). Errors are surfaced as
// formularios.ErrInternal with a generic message; the underlying
// error is logged at the service layer.
func (l *RedisLocker) AddToPending(ctx context.Context, iniciadoID uuid.UUID) error {
	if err := l.client.SAdd(ctx, formularios.KeyReportePending, iniciadoID.String()).Err(); err != nil {
		return fmt.Errorf("%w: redis SADD: %s", formularios.ErrInternal, "add pending failed")
	}
	return nil
}

// RemoveFromPending removes the iniciadoID from the pending
// set. Safe to call even if the member is not present (SREM
// returns 0 in that case — no error).
func (l *RedisLocker) RemoveFromPending(ctx context.Context, iniciadoID uuid.UUID) error {
	if err := l.client.SRem(ctx, formularios.KeyReportePending, iniciadoID.String()).Err(); err != nil {
		return fmt.Errorf("%w: redis SREM: %s", formularios.ErrInternal, "remove pending failed")
	}
	return nil
}

// ---------------------------------------------------------------------------
// buildLockKey
// ---------------------------------------------------------------------------

// buildLockKey builds the canonical lock key for an iniciado.
// Exposed (lowercase) so the test can compare against the actual
// key in Redis.
func buildLockKey(iniciadoID uuid.UUID) string {
	return fmt.Sprintf(formularios.KeyReporteS3Lock, iniciadoID)
}

// ---------------------------------------------------------------------------
// Test-only helpers (consumed by locker_redis_test.go).
// The leading "ForTest" suffix signals "do not use in
// production code" — these are kept in the production file
// (not _test.go) because the test in another package needs
// them, and Go's _test.go separation is per-package.
// ---------------------------------------------------------------------------

// LockKeyForTest returns the canonical lock key for the given
// iniciado. Test-only — production code MUST go through
// RedisLocker.Acquire.
func LockKeyForTest(iniciadoID uuid.UUID) string { return buildLockKey(iniciadoID) }

// ReleaseLockForTest runs the safe-release Lua script on the
// given miniredis instance with the given key+token. Returns
// nil on no-op (token mismatch — the key is NOT deleted) and on
// redis.Nil. Test-only — used to simulate a "wrong token"
// release attempt against a miniredis instance.
func ReleaseLockForTest(mr *miniredis.Miniredis, key, token string) error {
	const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end`
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	_, err := rdb.Eval(context.Background(), releaseScript, []string{key}, token).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	return nil
}
