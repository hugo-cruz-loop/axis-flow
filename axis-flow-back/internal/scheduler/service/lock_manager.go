// Package service hosts the Scheduler orchestration layer (lock manager,
// cron runner, job interface, FCM client, M2M client, parametrizacion
// client, and structured logging).
//
// This file implements the distributed lock manager described in the
// "Datos > Distributed Locking Strategy (Redis SETNX)" section of the
// Scheduler Service specification. The contract is a thin wrapper around
// Redis SET NX PX that uses two Lua scripts — one to release the lock and
// one to renew its TTL — exactly as published in the spec.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"axis-flow-back/internal/scheduler"

	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// LockManager — public contract.
// ---------------------------------------------------------------------------

// LockManager serializes the execution of a single scheduled job across all
// scheduler replicas. Each replica trying to run a job must call Acquire
// first; only the replica that wins the SET NX PX can proceed. Replicas that
// fail to acquire must skip the run. The winner must call Release after the
// run terminates; long-running jobs call RenewHeartbeat periodically to
// extend the TTL and avoid an orphaned lock.
type LockManager interface {
	// Acquire attempts to take the lock for jobKey on behalf of
	// executionID. It returns acquired=true exactly once per lease; on a
	// collision it returns acquired=false and a nil error. A non-nil
	// error is reserved for transport or Redis-level failures.
	Acquire(ctx context.Context, jobKey, executionID string, ttl time.Duration) (acquired bool, err error)
	// Release deletes the lock for jobKey iff its current value matches
	// executionID. It is a no-op (returning nil) when the lock is absent
	// or owned by a different replica. A non-nil error indicates a
	// transport failure or that the lock is no longer held by this
	// execution, in which case ErrLockLost is returned.
	Release(ctx context.Context, jobKey, executionID string) error
	// RenewHeartbeat extends the TTL of the lock for jobKey iff its
	// current value matches executionID. It returns renewed=true when the
	// script updated the TTL. ErrLockLost is returned when the lock is
	// gone or owned by a different replica.
	RenewHeartbeat(ctx context.Context, jobKey, executionID string, ttl time.Duration) (renewed bool, err error)
}

// ---------------------------------------------------------------------------
// Sentinel errors. Use errors.Is to inspect.
// ---------------------------------------------------------------------------

var (
	// ErrLockHeld is returned by Acquire when another replica already
	// holds the lock for the requested job key. The caller should treat
	// this as "skip the run" rather than retry aggressively.
	ErrLockHeld = errors.New("scheduler: lock held by another replica")
	// ErrLockLost is returned by Release and RenewHeartbeat when the
	// lock is no longer held by the caller — either it expired or a
	// different replica has taken over. The caller should log it and
	// abort the in-flight run.
	ErrLockLost = errors.New("scheduler: lock lost")
)

// ---------------------------------------------------------------------------
// Lua scripts — VERBATIM from specification.md §"Distributed Locking Strategy".
//
// Keeping these as package-level *redis.Script values lets go-redis cache
// their SHA1 hash on the client (EVALSHA + EVAL fallback), avoiding the
// per-call bandwidth cost of resending the script body.
// ---------------------------------------------------------------------------

// releaseScript deletes the lock key only when its value still matches the
// caller's execution token. Returns 1 when the key was deleted, 0 when the
// token did not match.
var releaseScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
`)

// heartbeatScript extends the lock TTL only when the caller's execution
// token still owns the key. Returns 1 when the TTL was refreshed, 0
// otherwise.
var heartbeatScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("pexpire", KEYS[1], ARGV[2])
else
    return 0
end
`)

// lockKeyPrefix is the namespace prefix for all scheduler distributed
// locks. Matches the key shape described in the spec
// (`scheduler:lock:<job_key>`).
const lockKeyPrefix = "scheduler:lock:"

// ---------------------------------------------------------------------------
// redisLockManager — Redis-backed implementation.
// ---------------------------------------------------------------------------

// redisLockManager is the production implementation of LockManager. It
// holds a single *redis.Client and is safe for concurrent use.
type redisLockManager struct {
	client *redis.Client
}

// NewRedisLockManager returns a LockManager backed by the provided Redis
// client. The client is retained by reference and must be reachable for
// the lifetime of the manager.
func NewRedisLockManager(client *redis.Client) LockManager {
	return &redisLockManager{client: client}
}

// Acquire performs `SET <key> <execution_id> NX PX <ttl_ms>`. The SET
// command returns the OK status ("OK") on success and nil on collision;
// go-redis surfaces those as a non-nil string and a redis.Nil error
// respectively, which we translate into the LockManager contract.
func (m *redisLockManager) Acquire(ctx context.Context, jobKey, executionID string, ttl time.Duration) (bool, error) {
	if jobKey == "" {
		return false, fmt.Errorf("lock_manager.Acquire: %w: job_key is required", scheduler.ErrInvalidInput)
	}
	if executionID == "" {
		return false, fmt.Errorf("lock_manager.Acquire: %w: execution_id is required", scheduler.ErrInvalidInput)
	}
	if ttl <= 0 {
		return false, fmt.Errorf("lock_manager.Acquire: %w: ttl must be > 0", scheduler.ErrInvalidInput)
	}
	ok, err := m.client.SetNX(ctx, lockKeyFor(jobKey), executionID, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("lock_manager.Acquire: %w", err)
	}
	return ok, nil
}

// Release runs the releaseScript against the lock key. A 1 return value
// means the key was deleted (we owned the lock); a 0 return value means
// the token did not match — the lock was either lost to TTL or stolen
// by another replica, both surfaced as ErrLockLost.
func (m *redisLockManager) Release(ctx context.Context, jobKey, executionID string) error {
	if jobKey == "" {
		return fmt.Errorf("lock_manager.Release: %w: job_key is required", scheduler.ErrInvalidInput)
	}
	if executionID == "" {
		return fmt.Errorf("lock_manager.Release: %w: execution_id is required", scheduler.ErrInvalidInput)
	}
	res, err := releaseScript.Run(ctx, m.client, []string{lockKeyFor(jobKey)}, executionID).Int64()
	if err != nil {
		return fmt.Errorf("lock_manager.Release: %w", err)
	}
	if res != 1 {
		return fmt.Errorf("lock_manager.Release: %w", ErrLockLost)
	}
	return nil
}

// RenewHeartbeat runs the heartbeatScript against the lock key with the
// new TTL expressed in milliseconds. A 1 return value means the TTL was
// refreshed; a 0 return value means the lock is gone or no longer ours.
func (m *redisLockManager) RenewHeartbeat(ctx context.Context, jobKey, executionID string, ttl time.Duration) (bool, error) {
	if jobKey == "" {
		return false, fmt.Errorf("lock_manager.RenewHeartbeat: %w: job_key is required", scheduler.ErrInvalidInput)
	}
	if executionID == "" {
		return false, fmt.Errorf("lock_manager.RenewHeartbeat: %w: execution_id is required", scheduler.ErrInvalidInput)
	}
	if ttl <= 0 {
		return false, fmt.Errorf("lock_manager.RenewHeartbeat: %w: ttl must be > 0", scheduler.ErrInvalidInput)
	}
	ttlMs := ttl.Milliseconds()
	if ttlMs <= 0 {
		ttlMs = 1
	}
	res, err := heartbeatScript.Run(ctx, m.client, []string{lockKeyFor(jobKey)}, executionID, ttlMs).Int64()
	if err != nil {
		return false, fmt.Errorf("lock_manager.RenewHeartbeat: %w", err)
	}
	if res != 1 {
		return false, fmt.Errorf("lock_manager.RenewHeartbeat: %w", ErrLockLost)
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// Internal helpers.
// ---------------------------------------------------------------------------

// lockKeyFor returns the canonical Redis key for a job's distributed lock.
// It is intentionally not exported: callers must not assemble lock keys
// by hand.
func lockKeyFor(jobKey string) string {
	return lockKeyPrefix + jobKey
}
